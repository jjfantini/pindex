package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// newDocsCmd returns the hidden "docs" subcommand, which generates markdown
// reference pages for the whole command tree (one file per command) into a
// directory. The output is VitePress-ready: each page gets a frontmatter
// title derived from its filename, and cross-command links are relative and
// extension-free so they work with VitePress cleanUrls.
func newDocsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "docs",
		Short:  "Generate markdown CLI reference docs",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			dir, _ := c.Flags().GetString("dir")
			if dir == "" {
				return fmt.Errorf("docs: --dir is required")
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("docs: create output dir: %w", err)
			}
			root := c.Root()
			root.DisableAutoGenTag = true
			// Keep the reference to real subcommands: hide the auto-added
			// shell-completion command so it (and its per-shell children)
			// are not documented. This only affects docs generation.
			for _, sub := range root.Commands() {
				if sub.Name() == "completion" {
					sub.Hidden = true
				}
			}

			filePrepender := func(filename string) string {
				name := filepath.Base(filename)
				name = strings.TrimSuffix(name, filepath.Ext(name))
				title := strings.ReplaceAll(name, "_", " ")
				return fmt.Sprintf("---\ntitle: %s\n---\n\n", title)
			}
			linkHandler := func(name string) string {
				return "./" + strings.TrimSuffix(name, ".md")
			}
			if err := doc.GenMarkdownTreeCustom(root, dir, filePrepender, linkHandler); err != nil {
				return fmt.Errorf("docs: generate markdown tree: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().String("dir", "", "output directory for generated markdown (required)")
	return cmd
}
