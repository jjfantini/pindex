package main

import (
	"fmt"

	"github.com/jjfantini/pindex/internal/config"
	"github.com/jjfantini/pindex/internal/extract"
	"github.com/spf13/cobra"
)

func newExtractCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extract <pdf>",
		Short: "Debug: dump per-page extracted text for a PDF",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			cfgPath, _ := c.Flags().GetString("config")
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			backend, _ := c.Flags().GetString("backend")
			if backend == "" {
				backend = cfg.Extractor
			}
			ex, err := extract.New(backend)
			if err != nil {
				return err
			}
			u, _, _ := newUI(c)
			u.Header("extract", args[0])
			pages, err := ex.Extract(args[0])
			if err != nil {
				return err
			}
			out := c.OutOrStdout()
			for _, p := range pages {
				_, _ = fmt.Fprintf(out, "===== page %d (%s) =====\n%s\n\n", p.Index, ex.Name(), p.Text)
			}
			u.Successf("extracted %d pages · backend %s", len(pages), ex.Name())
			return nil
		},
	}
	cmd.Flags().String("backend", "", "extractor backend: mupdf|poppler|purego (default: from config)")
	return cmd
}
