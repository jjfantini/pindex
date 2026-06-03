package index

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jjfantini/pindex/internal/extract"
	"github.com/jjfantini/pindex/internal/llm"
	"github.com/jjfantini/pindex/internal/prompts"
)

// tocResult is the outcome of TOC detection.
type tocResult struct {
	content        string
	hasPageNumbers bool
	endPos         int // 0-based slice position of the last TOC page
}

var (
	dotsRun    = regexp.MustCompile(`\.{5,}`)
	dotsSpaced = regexp.MustCompile(`(?:\. ){5,}\.?`)
)

func transformDotsToColon(s string) string {
	return dotsSpaced.ReplaceAllString(dotsRun.ReplaceAllString(s, ": "), ": ")
}

func (b *Builder) tocDetectPage(ctx context.Context, text string) (bool, error) {
	out, err := llm.CompleteJSON[prompts.TOCDetected](ctx, b.provider,
		llm.UserPrompt(b.cfg.Model, prompts.TOCDetector(text)), b.StructuredAttempts, nil)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(out.TOCDetected), "yes"), nil
}

// findTOCPages scans from startPos for consecutive table-of-contents pages,
// stopping past TOCCheckPageNum unless still inside a TOC (mirrors find_toc_pages).
func (b *Builder) findTOCPages(ctx context.Context, pages []extract.Page, startPos int) ([]int, error) {
	var toc []int
	lastYes := false
	for i := startPos; i < len(pages); i++ {
		if i >= b.cfg.TOCCheckPageNum && !lastYes {
			break
		}
		yes, err := b.tocDetectPage(ctx, pages[i].Text)
		if err != nil {
			return nil, err
		}
		switch {
		case yes:
			toc = append(toc, i)
			lastYes = true
		case lastYes:
			return toc, nil
		}
	}
	return toc, nil
}

func (b *Builder) detectPageIndex(ctx context.Context, content string) (bool, error) {
	out, err := llm.CompleteJSON[prompts.PageIndexGiven](ctx, b.provider,
		llm.UserPrompt(b.cfg.Model, prompts.DetectPageIndex(content)), b.StructuredAttempts, nil)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(out.PageIndexGivenInTOC), "yes"), nil
}

func (b *Builder) tocExtract(ctx context.Context, pages []extract.Page, tocPos []int) (string, bool, error) {
	var sb strings.Builder
	for _, pos := range tocPos {
		sb.WriteString(pages[pos].Text)
	}
	content := transformDotsToColon(sb.String())
	has, err := b.detectPageIndex(ctx, content)
	return content, has, err
}

// detectTOC mirrors check_toc: find the TOC, extract it, and report whether it
// lists page numbers (rescanning later pages when the first TOC lacks them).
// Returns found=false when there is no TOC at all.
func (b *Builder) detectTOC(ctx context.Context, pages []extract.Page) (res tocResult, found bool, err error) {
	tocPos, err := b.findTOCPages(ctx, pages, 0)
	if err != nil || len(tocPos) == 0 {
		return tocResult{}, false, err
	}
	content, has, err := b.tocExtract(ctx, pages, tocPos)
	if err != nil {
		return tocResult{}, false, err
	}
	if has {
		return tocResult{content: content, hasPageNumbers: true, endPos: tocPos[len(tocPos)-1]}, true, nil
	}
	for start := tocPos[len(tocPos)-1] + 1; start < len(pages) && start < b.cfg.TOCCheckPageNum; {
		more, err := b.findTOCPages(ctx, pages, start)
		if err != nil {
			return tocResult{}, false, err
		}
		if len(more) == 0 {
			break
		}
		c2, has2, err := b.tocExtract(ctx, pages, more)
		if err != nil {
			return tocResult{}, false, err
		}
		if has2 {
			return tocResult{content: c2, hasPageNumbers: true, endPos: more[len(more)-1]}, true, nil
		}
		start = more[len(more)-1] + 1
	}
	return tocResult{content: content, hasPageNumbers: false, endPos: tocPos[len(tocPos)-1]}, true, nil
}

type tocPhys struct {
	title    string
	physical int
}

// structureFromTOC builds the flat section list from a page-numbered TOC plus the
// printed-page -> physical-index offset. Mirrors process_toc_with_page_numbers.
func (b *Builder) structureFromTOC(ctx context.Context, pages []extract.Page, toc tocResult) ([]item, int, error) {
	trans, err := llm.CompleteJSON[prompts.TOCTransformOut](ctx, b.provider,
		llm.UserPrompt(b.cfg.Model, prompts.TOCTransform(toc.content)), b.StructuredAttempts,
		func(o prompts.TOCTransformOut) error {
			if len(o.TableOfContents) == 0 {
				return fmt.Errorf("empty table of contents")
			}
			return nil
		})
	if err != nil {
		return nil, 0, fmt.Errorf("toc transform: %w", err)
	}
	entries := trans.TableOfContents

	// Body pages right after the TOC, tagged with physical-index markers.
	startPos := toc.endPos + 1
	endPos := min(startPos+b.cfg.TOCCheckPageNum, len(pages))
	var tagged strings.Builder
	for i := startPos; i < endPos; i++ {
		tagged.WriteString(llm.WrapPage(llm.Page{Index: pages[i].Index, Text: pages[i].Text}))
	}

	noPage, _ := json.Marshal(stripPages(entries))
	physRaw, err := llm.CompleteJSON[[]prompts.TOCItem](ctx, b.provider,
		llm.UserPrompt(b.cfg.Model, prompts.TOCIndexExtract(string(noPage), tagged.String())), b.StructuredAttempts, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("toc index extract: %w", err)
	}

	startIdx := 1
	if startPos < len(pages) {
		startIdx = pages[startPos].Index
	}
	pairs := matchPairs(entries, resolveTOCPhys(physRaw, maxIndex(pages)), startIdx)
	offset, ok := calcOffset(pairs)
	if !ok {
		return nil, 0, fmt.Errorf("toc: could not compute a page offset")
	}
	items := applyOffset(entries, offset, maxIndex(pages))
	if len(items) == 0 {
		return nil, 0, fmt.Errorf("toc: no resolvable sections after offset")
	}
	// Verify each section against its page and repair the mis-mapped ones (the
	// global offset drifts across front/back matter); fall back to the no-TOC path
	// if too few sections still verify.
	items, frac := b.verifyAndRepair(ctx, items, pages)
	if frac < b.TOCVerifyThreshold {
		return nil, 0, fmt.Errorf("toc: only %.0f%% of sections verified after repair", frac*100)
	}
	return items, offset, nil
}

func stripPages(entries []prompts.TOCPageEntry) []map[string]string {
	out := make([]map[string]string, len(entries))
	for i, e := range entries {
		out[i] = map[string]string{"structure": e.Structure, "title": e.Title}
	}
	return out
}

func resolveTOCPhys(raw []prompts.TOCItem, maxPage int) []tocPhys {
	var out []tocPhys
	for _, it := range raw {
		if n, ok := parsePhysical(it.PhysicalIndex); ok && n >= 1 && n <= maxPage {
			out = append(out, tocPhys{title: it.Title, physical: n})
		}
	}
	return out
}

// matchPairs pairs printed page <-> physical index by matching titles
// (extract_matching_page_pairs).
func matchPairs(entries []prompts.TOCPageEntry, phys []tocPhys, startIdx int) [][2]int {
	var pairs [][2]int
	for _, p := range phys {
		if p.physical < startIdx {
			continue
		}
		for _, e := range entries {
			if e.Page != nil && strings.TrimSpace(e.Title) == strings.TrimSpace(p.title) {
				pairs = append(pairs, [2]int{*e.Page, p.physical})
			}
		}
	}
	return pairs
}

// calcOffset returns the most common (physical - printed) difference
// (calculate_page_offset).
func calcOffset(pairs [][2]int) (int, bool) {
	if len(pairs) == 0 {
		return 0, false
	}
	counts := map[int]int{}
	best, bestN := 0, -1
	for _, pr := range pairs {
		d := pr[1] - pr[0]
		counts[d]++
		if counts[d] > bestN {
			best, bestN = d, counts[d]
		}
	}
	return best, true
}

func applyOffset(entries []prompts.TOCPageEntry, offset, maxPage int) []item {
	var out []item
	for _, e := range entries {
		if e.Page == nil {
			continue
		}
		if phys := *e.Page + offset; phys >= 1 && phys <= maxPage {
			out = append(out, item{structure: e.Structure, title: e.Title, physicalIdx: phys})
		}
	}
	return out
}

// titleAt reports whether a section title actually appears on the given physical
// page (check_title_appearance).
func (b *Builder) titleAt(ctx context.Context, title string, idx int, pages []extract.Page) bool {
	txt, ok := pageText(pages, idx)
	if !ok {
		return false
	}
	out, err := llm.CompleteJSON[prompts.Appearance](ctx, b.provider,
		llm.UserPrompt(b.cfg.Model, prompts.CheckTitleAppearance(title, txt)), b.StructuredAttempts, nil)
	return err == nil && strings.EqualFold(strings.TrimSpace(out.Answer), "yes")
}

// verifyAndRepair checks each section's title against its offset-derived page and
// re-locates the ones that don't match by searching the window bounded by the
// surrounding verified sections (PageIndex verify_toc + fix_incorrect_toc +
// single_toc_item_index_fixer). Items that can't be verified or repaired keep
// their original page (never dropped — that would lose coverage). Returns the
// page-sorted items and the post-repair correct fraction (the fallback gate).
func (b *Builder) verifyAndRepair(ctx context.Context, items []item, pages []extract.Page) ([]item, float64) {
	if len(items) == 0 {
		return items, 0
	}
	maxP := maxIndex(pages)
	correct := make([]bool, len(items))
	for i := range items {
		correct[i] = b.titleAt(ctx, items[i].title, items[i].physicalIdx, pages)
	}
	for i := range items {
		if correct[i] {
			continue
		}
		lo, hi := 1, maxP
		for j := i - 1; j >= 0; j-- {
			if correct[j] {
				lo = items[j].physicalIdx
				break
			}
		}
		for j := i + 1; j < len(items); j++ {
			if correct[j] {
				hi = items[j].physicalIdx
				break
			}
		}
		if lo > hi {
			continue
		}
		var tagged strings.Builder
		for _, p := range pages {
			if p.Index >= lo && p.Index <= hi {
				tagged.WriteString(llm.WrapPage(llm.Page{Index: p.Index, Text: p.Text}))
			}
		}
		out, err := llm.CompleteJSON[prompts.PhysicalIndexFix](ctx, b.provider,
			llm.UserPrompt(b.cfg.Model, prompts.SingleTOCItemIndex(items[i].title, tagged.String())), b.StructuredAttempts, nil)
		if err != nil {
			continue
		}
		if n, ok := parsePhysical(out.PhysicalIndex); ok && n >= lo && n <= hi {
			items[i].physicalIdx = n
			correct[i] = b.titleAt(ctx, items[i].title, n, pages)
		}
	}
	nc := 0
	for _, c := range correct {
		if c {
			nc++
		}
	}
	sort.SliceStable(items, func(a, b int) bool { return items[a].physicalIdx < items[b].physicalIdx })
	return items, float64(nc) / float64(len(items))
}
