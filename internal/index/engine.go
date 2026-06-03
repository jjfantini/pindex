// Package index is pindex's indexing engine: it turns extracted pages into an
// enriched hierarchical tree. This file implements the build pipeline and the
// general no-TOC path (generate structure over token-bounded page groups ->
// resolve spans -> nest -> split oversized nodes -> enrich). The cheaper
// table-of-contents fast path (toc.go) runs first when a page-numbered TOC is
// present; this path is its fallback. Span resolution, nesting, splitting and
// enrichment are shared by both.
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
	// DetectTOC enables the table-of-contents fast path before falling back to the
	// general structure-generation path. Off by default: it changes the derived
	// tree for TOC-bearing docs, so it stays opt-in until accuracy parity is
	// re-measured against the no-TOC baseline.
	DetectTOC bool
	// TOCMinPages gates TOC detection to documents with at least this many pages.
	// Short docs rarely carry a formal page-numbered TOC, and detection costs up to
	// TOCCheckPageNum probe calls before falling back — so we skip it for them.
	TOCMinPages int
	// TOCVerifyThreshold is the minimum verified-title fraction to trust the TOC
	// branch; below it the build falls back to the no-TOC path.
	TOCVerifyThreshold float64
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
		DetectTOC:             false, // opt-in until TOC-path accuracy parity is measured
		TOCMinPages:           25,
		TOCVerifyThreshold:    0.6,
	}
}

// Result is the output of Build.
type Result struct {
	Structure   []tree.TreeNode
	Description string
	// PageOffset is the printed-label -> physical-index offset recovered from a
	// page-numbered table of contents (0 when the no-TOC path was used).
	PageOffset int
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

	rawItems, offset, err := b.sections(ctx, pages)
	if err != nil {
		return Result{}, err
	}
	items := addPreface(rawItems)
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
	tree.CoverChildren(nodes) // keep parent spans covering split-added children
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
	return Result{Structure: nodes, Description: desc, PageOffset: offset}, nil
}

// sections produces the flat, resolved section list plus a page offset. It tries
// the table-of-contents fast path (cheaper, and it recovers the printed->physical
// offset); if there is no page-numbered TOC, the branch fails, or too few sections
// verify after repair, it falls back to the general structure-generation path.
// structureFromTOC does the per-section verify+repair and gates on the threshold.
func (b *Builder) sections(ctx context.Context, pages []extract.Page) ([]item, int, error) {
	if b.DetectTOC && b.cfg.TOCCheckPageNum > 0 && len(pages) >= b.TOCMinPages {
		if toc, found, err := b.detectTOC(ctx, pages); err == nil && found && toc.hasPageNumbers {
			if items, offset, terr := b.structureFromTOC(ctx, pages, toc); terr == nil {
				return items, offset, nil
			}
		}
	}
	raw, err := b.generateStructure(ctx, pages)
	if err != nil {
		return nil, 0, err
	}
	return b.resolveAndFilter(raw, pages), 0, nil
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
