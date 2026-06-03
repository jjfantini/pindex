// Command pindex is the CLI for a vectorless, reasoning-based RAG engine: it
// builds hierarchical tree indexes from PDFs and answers questions by LLM
// reasoning over that structure (no vectors, no fixed chunking). Subcommands:
// index, ask, eval, extract.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "0.0.0-dev"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
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
	root.AddCommand(newIndexCmd(), newAskCmd(), newEvalCmd(), newExtractCmd())
	return root
}
