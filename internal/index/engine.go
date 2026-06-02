// Package index is pindex's indexing engine: it turns extracted pages into an
// enriched hierarchical tree. This file implements the no-TOC path (generate
// structure over token-bounded page groups -> resolve spans -> nest -> split
// oversized nodes -> enrich), which is both the general case and the fallback.
// TOC-detection branches and verify/fix land in a follow-up.
package index

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/jjfantini/pindex/internal/config"
	"github.com/jjfantini/pindex/internal/extract"
	"github.com/jjfantini/pindex/internal/llm"
	"github.com/jjfantini/pindex/internal/prompts"
	"github.com/jjfantini/pindex/internal/tree"
)

// Builder turns extracted pages into an enriched hierarchical tree index.
type Builder struct {
	cfg      config.Config
	provider llm.Provider
	counter  llm.TokenCounter
	renderer tree.Renderer

	// Concurrency bounds parallel LLM calls (appear-start checks, summaries).
	Concurrency int
	// MaxRecursionDepth guards large-node splitting (the Python original had none).
	MaxRecursionDepth int
	// StructuredAttempts is the validate-then-retry budget per structured call.
	StructuredAttempts int
	// SummaryTokenThreshold: nodes whose text is below this keep their text as the
	// summary instead of spending an LLM call (mirrors PageIndex's 200-token rule).
	SummaryTokenThreshold int
}

// NewBuilder returns a Builder with sensible defaults.
func NewBuilder(cfg config.Config, p llm.Provider) *Builder {
	return &Builder{
		cfg:                   cfg,
		provider:              p,
		counter:               llm.HeuristicCounter{},
		renderer:              tree.JSONRenderer{},
		Concurrency:           8,
		MaxRecursionDepth:     4,
		StructuredAttempts:    3,
		SummaryTokenThreshold: 200,
	}
}

// Result is the output of Build.
type Result struct {
	Structure   []tree.TreeNode
	Description string
}

// item is a working TOC entry during the build.
type item struct {
	structure   string
	title       string
	physicalIdx int
	appearStart bool
}

// Build runs the no-TOC indexing path end to end.
func (b *Builder) Build(ctx context.Context, pages []extract.Page) (Result, error) {
	if len(pages) == 0 {
		return Result{}, fmt.Errorf("index: no pages to index")
	}

	raw, err := b.generateStructure(ctx, pages)
	if err != nil {
		return Result{}, err
	}
	items := addPreface(b.resolveAndFilter(raw, pages))
	if len(items) == 0 {
		return Result{}, fmt.Errorf("index: no valid sections extracted from %d pages", len(pages))
	}
	if err := b.markAppearStart(ctx, items, pages); err != nil {
		return Result{}, err
	}

	nodes := tree.PostProcess(toPostItems(items), maxIndex(pages))
	if err := b.splitLargeNodes(ctx, nodes, pages, 0); err != nil {
		return Result{}, err
	}
	tree.WriteNodeIDs(nodes)

	if b.cfg.AddNodeSummary {
		if err := b.addSummaries(ctx, nodes, pages); err != nil {
			return Result{}, err
		}
	}
	desc := ""
	if b.cfg.AddDocDescription {
		if desc, err = b.docDescription(ctx, nodes); err != nil {
			return Result{}, err
		}
	}
	return Result{Structure: nodes, Description: desc}, nil
}

// generateStructure runs generate_toc_init then generate_toc_continue across
// token-bounded page groups, returning the raw (string physical_index) items.
func (b *Builder) generateStructure(ctx context.Context, pages []extract.Page) ([]prompts.TOCItem, error) {
	groups := llm.GroupPages(toLLMPages(pages), b.counter, b.cfg.MaxTokenNumEachNode, 1)
	if len(groups) == 0 {
		return nil, fmt.Errorf("index: no page groups produced")
	}
	nonEmpty := func(items []prompts.TOCItem) error {
		if len(items) == 0 {
			return fmt.Errorf("structure was empty")
		}
		return nil
	}
	all, err := llm.CompleteJSON(ctx, b.provider,
		llm.UserPrompt(b.cfg.Model, prompts.GenerateTOCInit(groups[0].Text)),
		b.StructuredAttempts, nonEmpty)
	if err != nil {
		return nil, fmt.Errorf("index: generate structure: %w", err)
	}
	for _, g := range groups[1:] {
		prev, _ := json.Marshal(all)
		cont, err := llm.CompleteJSON[[]prompts.TOCItem](ctx, b.provider,
			llm.UserPrompt(b.cfg.Model, prompts.GenerateTOCContinue(string(prev), g.Text)),
			b.StructuredAttempts, nil) // a continuation may legitimately add nothing
		if err != nil {
			return nil, fmt.Errorf("index: continue structure: %w", err)
		}
		all = append(all, cont...)
	}
	return all, nil
}

var physRe = regexp.MustCompile(`physical_index_(\d+)`)

// parsePhysical extracts N from "<physical_index_N>" (or a bare integer string).
func parsePhysical(s string) (int, bool) {
	if m := physRe.FindStringSubmatch(s); m != nil {
		n, err := strconv.Atoi(m[1])
		return n, err == nil
	}
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n, true
	}
	return 0, false
}

// resolveAndFilter converts string physical indices to ints and drops items that
// are unparseable or out of range (the validate_and_truncate + filter step).
func (b *Builder) resolveAndFilter(raw []prompts.TOCItem, pages []extract.Page) []item {
	max := maxIndex(pages)
	out := make([]item, 0, len(raw))
	for _, it := range raw {
		n, ok := parsePhysical(it.PhysicalIndex)
		if !ok || n < 1 || n > max {
			continue
		}
		out = append(out, item{structure: it.Structure, title: it.Title, physicalIdx: n})
	}
	return out
}

// addPreface prepends a Preface section when the first section starts past page 1.
func addPreface(items []item) []item {
	if len(items) > 0 && items[0].physicalIdx > 1 {
		return append([]item{{structure: "0", title: "Preface", physicalIdx: 1}}, items...)
	}
	return items
}

func toPostItems(items []item) []tree.PostItem {
	out := make([]tree.PostItem, len(items))
	for i, it := range items {
		out[i] = tree.PostItem{
			Structure:     it.structure,
			Title:         it.title,
			PhysicalIndex: it.physicalIdx,
			AppearStart:   it.appearStart,
		}
	}
	return out
}

func toLLMPages(pages []extract.Page) []llm.Page {
	out := make([]llm.Page, len(pages))
	for i, p := range pages {
		out[i] = llm.Page{Index: p.Index, Text: p.Text}
	}
	return out
}

// maxIndex returns the highest 1-based page index in the slice.
func maxIndex(pages []extract.Page) int {
	m := 0
	for _, p := range pages {
		if p.Index > m {
			m = p.Index
		}
	}
	return m
}

// pageText returns the text of the page with the given 1-based index.
func pageText(pages []extract.Page, idx int) (string, bool) {
	for _, p := range pages {
		if p.Index == idx {
			return p.Text, true
		}
	}
	return "", false
}
