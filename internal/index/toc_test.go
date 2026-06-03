package index

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/jjfantini/pindex/internal/config"
	"github.com/jjfantini/pindex/internal/extract"
	"github.com/jjfantini/pindex/internal/llm"
	"github.com/jjfantini/pindex/internal/prompts"
)

// --- pure offset/structure logic -------------------------------------------

func TestTransformDotsToColon(t *testing.T) {
	if got := transformDotsToColon("Introduction........5"); got != "Introduction: 5" {
		t.Errorf("dot run: got %q want %q", got, "Introduction: 5")
	}
	// spaced dot leaders collapse to a colon (exact surrounding spaces don't matter).
	if got := transformDotsToColon("Methods. . . . . . 12"); !strings.Contains(got, ":") || strings.Contains(got, ". .") {
		t.Errorf("spaced dots not collapsed: %q", got)
	}
	if got := transformDotsToColon("no dots here"); got != "no dots here" {
		t.Errorf("untouched text changed: %q", got)
	}
}

func TestCalcOffset(t *testing.T) {
	// diffs: 1, 1, 89 -> mode is 1.
	off, ok := calcOffset([][2]int{{1, 2}, {3, 4}, {10, 99}})
	if !ok || off != 1 {
		t.Errorf("calcOffset = %d,%v want 1,true", off, ok)
	}
	if _, ok := calcOffset(nil); ok {
		t.Error("calcOffset(nil) should report ok=false")
	}
}

func TestMatchPairs(t *testing.T) {
	entries := []prompts.TOCPageEntry{
		{Structure: "1", Title: "Intro", Page: ptr(1)},
		{Structure: "2", Title: "Methods", Page: ptr(3)},
		{Structure: "3", Title: "NoPage", Page: nil},
	}
	phys := []tocPhys{
		{title: "Intro", physical: 2},
		{title: "Methods", physical: 4},
		{title: "Early", physical: 1}, // below startIdx -> dropped
	}
	pairs := matchPairs(entries, phys, 2)
	want := [][2]int{{1, 2}, {3, 4}}
	if fmt.Sprint(pairs) != fmt.Sprint(want) {
		t.Errorf("matchPairs = %v want %v", pairs, want)
	}
}

func TestApplyOffset(t *testing.T) {
	entries := []prompts.TOCPageEntry{
		{Structure: "1", Title: "Intro", Page: ptr(1)},
		{Structure: "2", Title: "Methods", Page: ptr(3)},
		{Structure: "3", Title: "OffEnd", Page: ptr(999)}, // out of range -> dropped
		{Structure: "4", Title: "NoPage", Page: nil},      // no page -> dropped
	}
	items := applyOffset(entries, 1, 10)
	if len(items) != 2 {
		t.Fatalf("applyOffset kept %d items want 2: %+v", len(items), items)
	}
	if items[0].physicalIdx != 2 || items[1].physicalIdx != 4 {
		t.Errorf("physical = %d,%d want 2,4", items[0].physicalIdx, items[1].physicalIdx)
	}
}

// --- dispatch (sections) ----------------------------------------------------

const tocMarker = "TABLE-OF-CONTENTS-MARKER"

// pageNumberedDoc: a TOC on physical page 1 (offset +1), body on pages 2-5.
func pageNumberedDoc() []extract.Page {
	return []extract.Page{
		{Index: 1, Text: tocMarker + " Introduction: 1 Methods: 3"},
		{Index: 2, Text: "Introduction. Opening remarks about the company and its year."},
		{Index: 3, Text: "Introduction continued with more background detail."},
		{Index: 4, Text: "Methods. The procedures and approach we used this period."},
		{Index: 5, Text: "Methods continued, then a short concluding paragraph."},
	}
}

// classifyPrompt maps a prompt to a stable tag (most specific anchors first).
func classifyPrompt(p string) string {
	switch {
	case strings.Contains(p, "beginning of the given page_text"):
		return "appear_start"
	case strings.Contains(p, "section appears or starts in the given page_text"):
		return "verify_appear"
	case strings.Contains(p, "there is a table of content provided"):
		return "toc_detect"
	case strings.Contains(p, "page numbers/indices given within"):
		return "detect_page_index"
	case strings.Contains(p, "transform the whole table of contents"):
		return "toc_transform"
	case strings.Contains(p, "add the physical_index to the table of contents"):
		return "toc_index_extract"
	case strings.Contains(p, "generate the tree structure of the document"):
		return "generate_init"
	case strings.Contains(p, "continue the tree structure"):
		return "generate_continue"
	default:
		return "unknown"
	}
}

// routeMock answers by matching prompt content, so order-independent multi-step
// flows can be scripted, and records a per-tag call count.
type routeMock struct {
	mu     sync.Mutex
	counts map[string]int
	reply  map[string]string
}

func newRouteMock(reply map[string]string) *routeMock {
	return &routeMock{counts: map[string]int{}, reply: reply}
}

func (r *routeMock) Name() string { return "route" }

func (r *routeMock) Complete(_ context.Context, req llm.Request) (llm.Response, error) {
	p := req.Messages[len(req.Messages)-1].Content
	tag := classifyPrompt(p)
	r.mu.Lock()
	r.counts[tag]++
	r.mu.Unlock()
	if tag == "toc_detect" {
		if strings.Contains(p, tocMarker) {
			return llm.Response{Content: `{"toc_detected":"yes"}`, FinishReason: "stop"}, nil
		}
		return llm.Response{Content: `{"toc_detected":"no"}`, FinishReason: "stop"}, nil
	}
	if c, ok := r.reply[tag]; ok {
		return llm.Response{Content: c, FinishReason: "stop"}, nil
	}
	return llm.Response{}, fmt.Errorf("routeMock: no reply for tag %q: %.70s", tag, p)
}

func (r *routeMock) count(tag string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.counts[tag]
}

func ptr(i int) *int { return &i }

func tocTestBuilder(p llm.Provider) *Builder {
	b := NewBuilder(config.Default(), p)
	b.Concurrency = 1 // DetectTOC stays true
	return b
}

// happyReplies drives the with-page-numbers branch to a +1 offset.
func happyReplies() map[string]string {
	return map[string]string{
		"detect_page_index": `{"page_index_given_in_toc":"yes"}`,
		"toc_transform":     `{"table_of_contents":[{"structure":"1","title":"Introduction","page":1},{"structure":"2","title":"Methods","page":3}]}`,
		"toc_index_extract": `[{"structure":"1","title":"Introduction","physical_index":"<physical_index_2>"},{"structure":"2","title":"Methods","physical_index":"<physical_index_4>"}]`,
		"verify_appear":     `{"answer":"yes"}`,
		"appear_start":      `{"start_begin":"no"}`,
	}
}

func TestSectionsUsesPageNumberedTOC(t *testing.T) {
	route := newRouteMock(happyReplies())
	items, offset, err := tocTestBuilder(route).sections(context.Background(), pageNumberedDoc())
	if err != nil {
		t.Fatal(err)
	}
	if offset != 1 {
		t.Errorf("offset = %d want 1", offset)
	}
	if len(items) != 2 || items[0].physicalIdx != 2 || items[1].physicalIdx != 4 {
		t.Fatalf("items = %+v want Introduction@2, Methods@4", items)
	}
	if route.count("generate_init") != 0 {
		t.Errorf("no-TOC fallback should NOT run when the TOC branch succeeds (generate_init=%d)", route.count("generate_init"))
	}
	if route.count("toc_transform") != 1 || route.count("toc_index_extract") != 1 {
		t.Errorf("TOC branch calls: transform=%d index=%d want 1,1", route.count("toc_transform"), route.count("toc_index_extract"))
	}
}

func TestSectionsFallsBackWhenVerifyFails(t *testing.T) {
	replies := happyReplies()
	replies["verify_appear"] = `{"answer":"no"}` // titles don't verify -> fall back
	replies["generate_init"] = initTwoSections
	route := newRouteMock(replies)
	items, offset, err := tocTestBuilder(route).sections(context.Background(), pageNumberedDoc())
	if err != nil {
		t.Fatal(err)
	}
	if offset != 0 {
		t.Errorf("offset = %d want 0 (fell back to no-TOC)", offset)
	}
	if route.count("generate_init") != 1 {
		t.Errorf("expected no-TOC fallback (generate_init=%d want 1)", route.count("generate_init"))
	}
	if len(items) != 2 {
		t.Errorf("fallback produced %d items want 2", len(items))
	}
}

func TestSectionsNoTOCFallsBack(t *testing.T) {
	// A doc with no TOC marker: every page detects "no", so detection finds none.
	pages := []extract.Page{
		{Index: 1, Text: "Introduction. Opening section with content."},
		{Index: 2, Text: "Methods. The procedures used in this study."},
	}
	route := newRouteMock(map[string]string{"generate_init": initTwoSections})
	items, offset, err := tocTestBuilder(route).sections(context.Background(), pages)
	if err != nil {
		t.Fatal(err)
	}
	if offset != 0 || len(items) != 2 {
		t.Errorf("offset=%d items=%d want 0,2", offset, len(items))
	}
	if route.count("toc_transform") != 0 {
		t.Errorf("no-TOC doc should not reach the transform step (got %d)", route.count("toc_transform"))
	}
	if route.count("generate_init") != 1 {
		t.Errorf("generate_init=%d want 1", route.count("generate_init"))
	}
}

func TestBuildPropagatesPageOffset(t *testing.T) {
	route := newRouteMock(happyReplies())
	res, err := tocTestBuilder(route).Build(context.Background(), pageNumberedDoc())
	if err != nil {
		t.Fatal(err)
	}
	if res.PageOffset != 1 {
		t.Errorf("Result.PageOffset = %d want 1", res.PageOffset)
	}
	// first section starts at physical 2 -> a Preface is prepended at page 1.
	if len(res.Structure) == 0 || res.Structure[0].Title != "Preface" {
		t.Fatalf("expected a Preface root, got %+v", res.Structure)
	}
}
