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
	// TOCVerifyThreshold is the minimum verified-title fraction to trust the TOC
	// branch; below it the build falls back to the structure-generation path.
	TOCVerifyThreshold float64
	// Progress, when set, receives human-readable build-stage updates
	// (stage key + message). It may be called from concurrent goroutines and
	// must be cheap; nil disables progress reporting.
	Progress func(stage, msg string)
}

// progressf reports a build stage to the optional Progress hook (nil-safe).
func (b *Builder) progressf(stage, format string, args ...any) {
	if b.Progress != nil {
		b.Progress(stage, fmt.Sprintf(format, args...))
	}
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

// Build runs the indexing pipeline end to end: derive the section list (TOC path
// first, generation as fallback), resolve spans, nest, split oversized nodes,
// then enrich.
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
	b.progressf("verify", "verifying %d section starts", len(items))
	if err := b.markAppearStart(ctx, items, pages); err != nil {
		return Result{}, err
	}

	nodes := tree.PostProcess(toPostItems(items), maxIndex(pages))
	b.progressf("split", "splitting oversized sections")
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
		b.progressf("describe", "writing document description")
		if desc, err = b.docDescription(ctx, nodes); err != nil {
			return Result{}, err
		}
	}
	return Result{Structure: nodes, Description: desc, PageOffset: offset}, nil
}

// sections produces the flat, resolved section list plus a page offset. The
// table-of-contents path is the primary route: it reads the document's own
// printed TOC, recovers the printed->physical offset, and is cheaper than
// generating structure from scratch. It runs whenever TOC detection is enabled
// (cfg.TOCCheckPageNum > 0 bounds how many leading pages are scanned — the
// --toc-page-limit knob). When no page-numbered TOC is detected, or too few
// sections verify after repair, it falls back to the structure-generation path.
// structureFromTOC does the per-section verify+repair and gates on the threshold.
func (b *Builder) sections(ctx context.Context, pages []extract.Page) ([]item, int, error) {
	if b.cfg.TOCCheckPageNum > 0 {
		b.progressf("toc", "scanning %d leading pages for a table of contents", min(b.cfg.TOCCheckPageNum, len(pages)))
		if toc, found, err := b.detectTOC(ctx, pages); err == nil && found && toc.hasPageNumbers {
			b.progressf("toc", "page-numbered TOC found — using the fast path")
			if items, offset, terr := b.structureFromTOC(ctx, pages, toc); terr == nil {
				return items, offset, nil
			}
			b.progressf("toc", "TOC sections failed verification — falling back to structure generation")
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
	b.progressf("structure", "generating structure · group 1/%d", len(groups))
	nonEmpty := func(items []prompts.TOCItem) error {
		if len(items) == 0 {
			return fmt.Errorf("structure was empty")
		}
		return nil
	}
	initP := prompts.GenerateTOCInit(groups[0].Text)
	all, err := llm.CompleteJSON(ctx, b.provider,
		llm.SystemUser(b.cfg.Model, initP.System, initP.User),
		b.StructuredAttempts, nonEmpty)
	if err != nil {
		return nil, fmt.Errorf("index: generate structure: %w", err)
	}
	for gi, g := range groups[1:] {
		b.progressf("structure", "generating structure · group %d/%d", gi+2, len(groups))
		prev, _ := json.Marshal(all)
		contP := prompts.GenerateTOCContinue(string(prev), g.Text)
		cont, err := llm.CompleteJSON[[]prompts.TOCItem](ctx, b.provider,
			llm.SystemUser(b.cfg.Model, contP.System, contP.User),
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
