// Command pindex is the CLI for a vectorless, reasoning-based RAG engine: it
// builds hierarchical tree indexes from PDFs and answers questions by LLM
// reasoning over that structure (no vectors, no fixed chunking). Subcommands:
// index, ask, eval, extract (plus a hidden docs command for reference
// generation).
package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/jjfantini/pindex/internal/ui"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "0.0.0-dev"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		ui.New(os.Stderr).Errorf("%v", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "pindex",
		Short: "Vectorless, reasoning-based RAG over document trees",
		Long: "pindex builds hierarchical tree indexes from PDFs/Markdown and answers\n" +
			"questions by LLM reasoning over that structure — no vectors, no fixed\n" +
			"chunking, traceable page citations.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().String("config", "", "path to a pindex config YAML (optional)")
	root.PersistentFlags().Bool("verbose", false, "stream under-the-hood diagnostics to stderr (LLM calls, cache hits, retries, build stages)")
	root.PersistentFlags().Bool("plain", false, "force plain line-oriented output: no colors or animations (also via PINDEX_PLAIN=1; auto when piped)")
	root.AddCommand(newIndexCmd(), newAskCmd(), newEvalCmd(), newExtractCmd(), newDocsCmd())
	return root
}
