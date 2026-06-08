package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/time/rate"

	"github.com/jjfantini/pindex/internal/config"
	"github.com/jjfantini/pindex/internal/envfile"
	"github.com/jjfantini/pindex/internal/exportout"
	"github.com/jjfantini/pindex/internal/extract"
	"github.com/jjfantini/pindex/internal/index"
	"github.com/jjfantini/pindex/internal/llm"
	"github.com/jjfantini/pindex/internal/pipeline"
	"github.com/jjfantini/pindex/internal/store"
	"github.com/jjfantini/pindex/internal/tree"
)

func newIndexCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index <pdf-or-dir>",
		Short: "Index a PDF (prints its tree) or a directory of PDFs (batch, resumable)",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			envFile, _ := c.Flags().GetString("env-file")
			if err := envfile.Load(envFile); err != nil {
				return err
			}
			cfgPath, _ := c.Flags().GetString("config")
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			if m, _ := c.Flags().GetString("model"); m != "" {
				cfg.Model = m
			}
			if b, _ := c.Flags().GetString("backend"); b != "" {
				cfg.Extractor = b
			}
			if lim, _ := c.Flags().GetInt("toc-page-limit"); lim >= 0 {
				cfg.TOCCheckPageNum = lim
			}
			cacheDir, _ := c.Flags().GetString("cache-dir")
			ws, _ := c.Flags().GetString("workspace")

			ex, err := extract.New(cfg.Extractor)
			if err != nil {
				return err
			}
			rpm, _ := c.Flags().GetInt("rpm")
			provider, err := buildProvider(cfg.Model, cacheDir, rpm)
			if err != nil {
				return err
			}

			var st *store.Store
			if ws != "" {
				if st, err = store.Open(ws); err != nil {
					return err
				}
				defer func() { _ = st.Close() }()
			}
			fi := &pipeline.FileIndexer{
				Builder:   index.NewBuilder(cfg, provider),
				Extractor: ex,
				Store:     st,
			}

			info, err := os.Stat(args[0])
			if err != nil {
				return err
			}
			if info.IsDir() {
				return runBatch(c, fi, args[0])
			}

			doc, err := fi.IndexOne(c.Context(), args[0])
			if err != nil {
				return err
			}
			out, err := (tree.JSONRenderer{Indent: true}).Render(doc.Structure)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(c.OutOrStdout(), out)
			if st != nil {
				_, _ = fmt.Fprintf(c.ErrOrStderr(), "saved to %s (doc id %s)\n", ws, doc.ID)
			}
			if doc.DocDescription != "" {
				_, _ = fmt.Fprintln(c.ErrOrStderr(), "description:", doc.DocDescription)
			}
			if outDir := exportDir(ws); outDir != "" {
				inclPages, _ := c.Flags().GetBool("include-raw-text")
				path, werr := exportout.WriteTree(outDir, doc, inclPages)
				if werr != nil {
					return werr
				}
				_, _ = fmt.Fprintf(c.ErrOrStderr(), "wrote tree to %s\n", path)
			}
			return nil
		},
	}
	cmd.Flags().String("model", "", "LLM model (default from config; e.g. claude-haiku-4-5-20251001, gpt-4o)")
	cmd.Flags().String("backend", "", "extractor backend (default from config)")
	cmd.Flags().String("cache-dir", ".pindex/cache", "prompt-hash response cache dir (empty to disable)")
	cmd.Flags().String("env-file", ".env", "load API keys from this .env file (overrides the environment)")
	cmd.Flags().Int("rpm", 0, "max requests/min to the LLM (0 = unlimited; set on low rate-limit tiers)")
	cmd.Flags().String("workspace", ".pindex/workspace", "persist the index here (empty to only print)")
	cmd.Flags().Int("concurrency", 4, "parallel documents when indexing a directory")
	cmd.Flags().Bool("force", false, "re-index documents already in the workspace")
	cmd.Flags().Int("toc-page-limit", -1, "leading pages to scan for a table of contents (0 disables TOC detection; -1 uses the config default of 10)")
	cmd.Flags().Bool("include-raw-text", false, "include raw page text in the browsable <workspace>/pindex export (larger, less readable)")
	return cmd
}

func runBatch(c *cobra.Command, fi *pipeline.FileIndexer, dir string) error {
	if fi.Store == nil {
		return fmt.Errorf("indexing a directory requires a --workspace to persist into")
	}
	paths, err := pipeline.FindPDFs(dir)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("no .pdf files found under %s", dir)
	}
	conc, _ := c.Flags().GetInt("concurrency")
	force, _ := c.Flags().GetBool("force")
	results := pipeline.BatchIndex(c.Context(), fi, paths, conc, force, func(r pipeline.Result) {
		status := "indexed"
		switch {
		case r.Err != nil:
			status = "FAILED: " + r.Err.Error()
		case r.Skipped:
			status = "skipped"
		}
		_, _ = fmt.Fprintf(c.ErrOrStderr(), "[%s] %s\n", status, r.Path)
	})
	indexed, skipped, failed := pipeline.Summarize(results)
	_, _ = fmt.Fprintf(c.OutOrStdout(), "indexed=%d skipped=%d failed=%d total=%d\n", indexed, skipped, failed, len(results))

	ws, _ := c.Flags().GetString("workspace")
	if outDir := exportDir(ws); outDir != "" {
		inclPages, _ := c.Flags().GetBool("include-raw-text")
		for _, r := range results {
			if r.Err != nil {
				continue // never indexed (or failed) — nothing to export
			}
			doc, lerr := fi.Store.Load(r.DocID)
			if lerr != nil {
				return lerr
			}
			if _, werr := exportout.WriteTree(outDir, doc, inclPages); werr != nil {
				return werr
			}
		}
		_, _ = fmt.Fprintf(c.ErrOrStderr(), "wrote trees to %s\n", outDir)
	}
	if failed > 0 {
		return fmt.Errorf("%d document(s) failed to index", failed)
	}
	return nil
}

// exportDir is where the browsable {doc_name}_pindex.json trees are written: the
// workspace's pindex/ subdir. Empty workspace means no export (print-only).
func exportDir(workspace string) string {
	if workspace != "" {
		return filepath.Join(workspace, "pindex")
	}
	return ""
}

// buildProvider returns a live provider wrapped in resilience and (optionally) a
// read-through cache. Cache is outermost so a hit avoids the network entirely.
// rpm > 0 enables a request-rate limiter (useful on low TPM tiers); the deeper
// retry budget + rate-limit-aware breaker ride out 429s without cascading.
func buildProvider(model, cacheDir string, rpm int) (llm.Provider, error) {
	base, err := llm.NewHTTPProvider(model)
	if err != nil {
		return nil, err
	}
	opts := []llm.Option{llm.WithBreaker(5, 30*time.Second)}
	if rpm > 0 {
		opts = append(opts, llm.WithLimiter(rate.NewLimiter(rate.Limit(float64(rpm)/60.0), 1)))
	}
	var p llm.Provider = llm.NewResilient(base,
		llm.RetryPolicy{MaxAttempts: 8, BaseDelay: time.Second, MaxDelay: 60 * time.Second},
		opts...)
	if cacheDir != "" {
		fc, err := llm.NewFileCache(cacheDir)
		if err != nil {
			return nil, err
		}
		p = llm.NewCaching(p, fc)
	}
	return p, nil
}
