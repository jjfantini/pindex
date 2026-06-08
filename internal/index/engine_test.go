package index

import (
	"context"
	"strings"
	"testing"

	"github.com/jjfantini/pindex/internal/config"
	"github.com/jjfantini/pindex/internal/extract"
	"github.com/jjfantini/pindex/internal/llm"
	"github.com/jjfantini/pindex/internal/prompts"
)

// tocItems builds []prompts.TOCItem from (structure, title, physical_index) triples.
func tocItems(triples ...string) []prompts.TOCItem {
	var out []prompts.TOCItem
	for i := 0; i+2 < len(triples); i += 3 {
		out = append(out, prompts.TOCItem{Structure: triples[i], Title: triples[i+1], PhysicalIndex: triples[i+2]})
	}
	return out
}

func twoPages() []extract.Page {
	return []extract.Page{
		{Index: 1, Text: "Introduction. A short opening section."},
		{Index: 2, Text: "Methods. A short methods section with revenue 1234."},
	}
}

const initTwoSections = `[{"structure":"1","title":"Introduction","physical_index":"<physical_index_1>"},` +
	`{"structure":"2","title":"Methods","physical_index":"<physical_index_2>"}]`

func newTestBuilder(cfg config.Config, p llm.Provider) *Builder {
	b := NewBuilder(cfg, p)
	b.Concurrency = 1         // deterministic call ordering for scripted mocks
	b.cfg.TOCCheckPageNum = 0 // disable TOC detection to test the generation path
	// in isolation here; the TOC path is exercised in toc_test.go.
	return b
}

func TestBuildNoTOCEndToEnd(t *testing.T) {
	cfg := config.Default() // AddNodeSummary=true, AddDocDescription=false
	mock := llm.NewMock("m",
		llm.MockResponse{Content: initTwoSections},         // generate_toc_init
		llm.MockResponse{Content: `{"start_begin":"yes"}`}, // appear-start: Introduction
		llm.MockResponse{Content: `{"start_begin":"yes"}`}, // appear-start: Methods
	)
	res, err := newTestBuilder(cfg, mock).Build(context.Background(), twoPages())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Structure) != 2 {
		t.Fatalf("roots=%d want 2: %+v", len(res.Structure), res.Structure)
	}
	intro, methods := res.Structure[0], res.Structure[1]
	if intro.Title != "Introduction" || intro.StartIndex != 1 || intro.EndIndex != 1 {
		t.Errorf("intro = %+v", intro)
	}
	if methods.Title != "Methods" || methods.StartIndex != 2 || methods.EndIndex != 2 {
		t.Errorf("methods = %+v", methods)
	}
	if intro.NodeID != "0000" || methods.NodeID != "0001" {
		t.Errorf("node ids = %q,%q want 0000,0001", intro.NodeID, methods.NodeID)
	}
	// short pages -> summary is the page text verbatim (no LLM call)
	if !strings.Contains(intro.Summary, "Introduction") || !strings.Contains(methods.Summary, "Methods") {
		t.Errorf("summaries not set from text: %q / %q", intro.Summary, methods.Summary)
	}
	if res.Description != "" {
		t.Errorf("description should be empty when AddDocDescription=false, got %q", res.Description)
	}
	if mock.CallCount() != 3 {
		t.Errorf("llm calls=%d want 3 (init + 2 appear-start; summaries local)", mock.CallCount())
	}
}

func TestBuildWithDocDescription(t *testing.T) {
	cfg := config.Default()
	cfg.AddDocDescription = true
	mock := llm.NewMock("m",
		llm.MockResponse{Content: initTwoSections},
		llm.MockResponse{Content: `{"start_begin":"yes"}`},
		llm.MockResponse{Content: `{"start_begin":"yes"}`},
		llm.MockResponse{Content: "A short paper introducing methods."}, // doc description (plain text)
	)
	res, err := newTestBuilder(cfg, mock).Build(context.Background(), twoPages())
	if err != nil {
		t.Fatal(err)
	}
	if res.Description != "A short paper introducing methods." {
		t.Errorf("description = %q", res.Description)
	}
	if mock.CallCount() != 4 {
		t.Errorf("llm calls=%d want 4", mock.CallCount())
	}
}

func TestBuildEmptyPagesErrors(t *testing.T) {
	if _, err := newTestBuilder(config.Default(), llm.NewMock("m")).Build(context.Background(), nil); err == nil {
		t.Error("expected error for empty pages")
	}
}

func TestParsePhysical(t *testing.T) {
	cases := map[string]struct {
		n  int
		ok bool
	}{
		"<physical_index_5>": {5, true},
		"physical_index_12":  {12, true},
		"7":                  {7, true},
		"":                   {0, false},
		"none":               {0, false},
	}
	for in, want := range cases {
		n, ok := parsePhysical(in)
		if n != want.n || ok != want.ok {
			t.Errorf("parsePhysical(%q) = (%d,%v) want (%d,%v)", in, n, ok, want.n, want.ok)
		}
	}
}

func TestResolveAndFilterDropsInvalid(t *testing.T) {
	b := NewBuilder(config.Default(), nil)
	pages := twoPages()
	raw := tocItems(
		"1", "A", "<physical_index_1>", // valid
		"2", "B", "<physical_index_9>", // out of range -> dropped
		"3", "C", "garbage", // unparseable -> dropped
	)
	got := b.resolveAndFilter(raw, pages)
	if len(got) != 1 || got[0].title != "A" || got[0].physicalIdx != 1 {
		t.Errorf("resolveAndFilter = %+v want one item A@1", got)
	}
}

func TestAddPreface(t *testing.T) {
	out := addPreface([]item{{structure: "1", title: "Body", physicalIdx: 3}})
	if len(out) != 2 || out[0].title != "Preface" || out[0].physicalIdx != 1 {
		t.Errorf("expected Preface prepended, got %+v", out)
	}
	out = addPreface([]item{{structure: "1", title: "Body", physicalIdx: 1}})
	if len(out) != 1 {
		t.Errorf("no preface expected when first starts at page 1, got %+v", out)
	}
}
