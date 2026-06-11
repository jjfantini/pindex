package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jjfantini/pindex/internal/ask"
	"github.com/jjfantini/pindex/internal/config"
	"github.com/jjfantini/pindex/internal/envfile"
	"github.com/jjfantini/pindex/internal/exportout"
	"github.com/jjfantini/pindex/internal/store"
)

func newAskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ask <question>",
		Short: "Answer a question over an indexed document (cites pages)",
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
			ws, _ := c.Flags().GetString("workspace")
			cacheDir, _ := c.Flags().GetString("cache-dir")
			docRef, _ := c.Flags().GetString("doc")

			s, err := store.Open(ws)
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()

			id, err := resolveDoc(s, docRef)
			if err != nil {
				return err
			}
			doc, err := s.Load(id)
			if err != nil {
				return err
			}

			u, logger, _ := newUI(c)
			rpm, _ := c.Flags().GetInt("rpm")
			provider, err := buildProvider(cfg.RetrieveModelOrDefault(), cacheDir, rpm, llmObserver(logger))
			if err != nil {
				return err
			}

			effStr, _ := c.Flags().GetString("effort")
			effort, err := ask.ParseEffort(effStr)
			if err != nil {
				return err
			}
			u.Header("ask", clip(args[0], 60))
			step := u.Step(fmt.Sprintf("searching %s · %s · effort %s", doc.DocName, cfg.RetrieveModelOrDefault(), effort))
			asker := ask.New(provider, cfg.RetrieveModelOrDefault())
			asker.Effort = effort
			ans, err := asker.Ask(c.Context(), doc, args[0])
			if err != nil {
				step.Fail("ask failed")
				return err
			}
			step.Done("answered · effort %s", effort)

			_, _ = fmt.Fprintln(c.OutOrStdout(), ans.Text)
			if len(ans.CitedPages) > 0 {
				u.Infof("cited pages: %s  (doc: %s)", u.Styles().Accent.Render(fmt.Sprint(ans.CitedPages)), doc.DocName)
			}
			switch ans.Verification {
			case "supported":
				u.Successf("verification: supported")
			case "unsupported":
				u.Warnf("verification: UNSUPPORTED — treat with caution (missing support for some claims)")
			}

			if outDir, _ := c.Flags().GetString("out"); outDir != "" {
				inclPages, _ := c.Flags().GetBool("include-pages")
				if _, werr := exportout.WriteTree(outDir, doc, inclPages); werr != nil {
					return werr
				}
				path, werr := exportout.WriteAnswer(outDir, exportout.AnswerRecord{
					DocName:       doc.DocName,
					Question:      args[0],
					Predicted:     ans.Text,
					Reasoning:     ans.Reasoning,
					Verification:  ans.Verification,
					Steps:         ans.Steps,
					SelectedPages: ans.SelectedPages,
					CitedPages:    ans.CitedPages,
				})
				if werr != nil {
					return werr
				}
				u.Notef("wrote answer to %s", path)
			}
			return nil
		},
	}
	cmd.Flags().String("model", "", "LLM model (default from config)")
	cmd.Flags().String("workspace", ".pindex/workspace", "workspace directory")
	cmd.Flags().String("doc", "", "document id or path (default: the only indexed doc)")
	cmd.Flags().String("cache-dir", ".pindex/cache", "prompt-hash response cache dir (empty to disable)")
	cmd.Flags().String("env-file", ".env", "load API keys from this .env file")
	cmd.Flags().Int("rpm", 0, "max requests/min to the LLM (0 = unlimited)")
	cmd.Flags().String("effort", "low", "retrieval effort: low|medium|high|ultra (medium retries on refusal; high uses an agentic tree-search loop; ultra adds an answer-verification pass)")
	cmd.Flags().String("out", "", "append this Q&A (and the doc's tree) to a browsable output directory")
	cmd.Flags().Bool("include-pages", false, "include raw page text in the exported tree")
	return cmd
}

// resolveDoc maps a --doc reference (a stored id or a file path) to a document
// id, or selects the only document when the reference is empty.
func resolveDoc(s *store.Store, ref string) (string, error) {
	if ref != "" {
		if _, err := os.Stat(ref); err == nil {
			id := store.DocID(ref)
			if s.Has(id) {
				return id, nil
			}
			return "", fmt.Errorf("ask: %q is not indexed yet — run: pindex index %q", ref, ref)
		}
		if s.Has(ref) {
			return ref, nil
		}
		return "", fmt.Errorf("ask: no document with id %q in workspace", ref)
	}
	list, err := s.List()
	if err != nil {
		return "", err
	}
	switch len(list) {
	case 0:
		return "", fmt.Errorf("ask: workspace is empty — index a document first")
	case 1:
		return list[0].ID, nil
	default:
		names := make([]string, len(list))
		for i, r := range list {
			names[i] = fmt.Sprintf("%s (%s)", r.ID, r.DocName)
		}
		return "", fmt.Errorf("ask: multiple documents; pass --doc <id>:\n  %s", strings.Join(names, "\n  "))
	}
}
