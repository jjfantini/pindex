package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/jjfantini/pindex/internal/config"
	"github.com/jjfantini/pindex/internal/envfile"
	"github.com/jjfantini/pindex/internal/extract"
	"github.com/jjfantini/pindex/internal/index"
	"github.com/jjfantini/pindex/internal/llm"
	"github.com/jjfantini/pindex/internal/tree"
)

func newIndexCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index <pdf>",
		Short: "Index a PDF into a hierarchical tree (prints the tree JSON)",
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
			cacheDir, _ := c.Flags().GetString("cache-dir")

			ex, err := extract.New(cfg.Extractor)
			if err != nil {
				return err
			}
			pages, err := ex.Extract(args[0])
			if err != nil {
				return err
			}

			provider, err := buildProvider(cfg.Model, cacheDir)
			if err != nil {
				return err
			}

			res, err := index.NewBuilder(cfg, provider).Build(c.Context(), pages)
			if err != nil {
				return err
			}

			out, err := (tree.JSONRenderer{Indent: true}).Render(res.Structure)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(c.OutOrStdout(), out)
			if res.Description != "" {
				_, _ = fmt.Fprintln(c.ErrOrStderr(), "description:", res.Description)
			}
			return nil
		},
	}
	cmd.Flags().String("model", "", "LLM model (default from config; e.g. claude-haiku-4-5-20251001, gpt-4o)")
	cmd.Flags().String("backend", "", "extractor backend (default from config)")
	cmd.Flags().String("cache-dir", ".pindex/cache", "prompt-hash response cache dir (empty to disable)")
	cmd.Flags().String("env-file", ".env", "load API keys from this .env file (overrides the environment)")
	return cmd
}

// buildProvider returns a live provider wrapped in resilience and (optionally) a
// read-through cache. Cache is outermost so a hit avoids the network entirely.
func buildProvider(model, cacheDir string) (llm.Provider, error) {
	base, err := llm.NewHTTPProvider(model)
	if err != nil {
		return nil, err
	}
	var p llm.Provider = llm.NewResilient(base,
		llm.RetryPolicy{MaxAttempts: 4, BaseDelay: time.Second, MaxDelay: 30 * time.Second},
		llm.WithBreaker(5, 30*time.Second))
	if cacheDir != "" {
		fc, err := llm.NewFileCache(cacheDir)
		if err != nil {
			return nil, err
		}
		p = llm.NewCaching(p, fc)
	}
	return p, nil
}
