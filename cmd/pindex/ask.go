package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jjfantini/pindex/internal/ask"
	"github.com/jjfantini/pindex/internal/config"
	"github.com/jjfantini/pindex/internal/envfile"
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

			rpm, _ := c.Flags().GetInt("rpm")
			provider, err := buildProvider(cfg.RetrieveModelOrDefault(), cacheDir, rpm)
			if err != nil {
				return err
			}

			ans, err := ask.New(provider, cfg.RetrieveModelOrDefault()).Ask(c.Context(), doc, args[0])
			if err != nil {
				return err
			}

			_, _ = fmt.Fprintln(c.OutOrStdout(), ans.Text)
			if len(ans.CitedPages) > 0 {
				_, _ = fmt.Fprintf(c.ErrOrStderr(), "cited pages: %v  (doc: %s)\n", ans.CitedPages, doc.DocName)
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
