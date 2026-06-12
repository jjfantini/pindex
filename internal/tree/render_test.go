package tree

import (
	"strings"
	"testing"
)

func TestJSONRendererCompact(t *testing.T) {
	nodes := []TreeNode{{Title: "A", StartIndex: 1, EndIndex: 2}}
	got, err := JSONRenderer{}.Render(nodes)
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"title":"A","start_index":1,"end_index":2}]`
	if got != want {
		t.Errorf("render = %s, want %s", got, want)
	}
}

func TestJSONRendererIndentParses(t *testing.T) {
	got, err := JSONRenderer{Indent: true}.Render([]TreeNode{{Title: "A"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "\n") {
		t.Errorf("indented output should contain newlines: %q", got)
	}
}

func TestRenderStructureStripsText(t *testing.T) {
	nodes := []TreeNode{{Title: "A", Text: "page text here", StartIndex: 1, EndIndex: 1}}
	got, err := RenderStructure(JSONRenderer{}, nodes)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "page text here") {
		t.Errorf("structure view should not include text: %s", got)
	}
	if nodes[0].Text != "page text here" {
		t.Error("RenderStructure must not mutate the input")
	}
}

func bigStructure(n, summaryRunes int) []TreeNode {
	nodes := make([]TreeNode, n)
	for i := range nodes {
		nodes[i] = TreeNode{
			Title: "Section", StartIndex: i + 1, EndIndex: i + 1,
			Summary: strings.Repeat("s", summaryRunes),
			Text:    "page text",
		}
	}
	return nodes
}

func TestRenderStructureWithinFullFit(t *testing.T) {
	nodes := bigStructure(3, 100)
	got, fit, err := RenderStructureWithin(JSONRenderer{}, nodes, 1_000_000)
	if err != nil {
		t.Fatal(err)
	}
	if fit != FitFull {
		t.Errorf("fit = %v, want FitFull", fit)
	}
	full, _ := RenderStructure(JSONRenderer{}, nodes)
	if got != full {
		t.Error("within-budget render must equal the full structure render")
	}
}

func TestRenderStructureWithinTruncatesSummaries(t *testing.T) {
	nodes := bigStructure(10, 2_000) // ~20k chars of summaries
	got, fit, err := RenderStructureWithin(JSONRenderer{}, nodes, 10_000)
	if err != nil {
		t.Fatal(err)
	}
	if fit != FitTruncatedSummaries {
		t.Errorf("fit = %v, want FitTruncatedSummaries", fit)
	}
	if len(got) > 10_000 {
		t.Errorf("render = %d chars, exceeds 10k budget", len(got))
	}
	if !strings.Contains(got, "…") {
		t.Error("truncated summaries should carry the ellipsis marker")
	}
	if nodes[0].Summary != strings.Repeat("s", 2_000) {
		t.Error("RenderStructureWithin must not mutate the input")
	}
}

func TestRenderStructureWithinFallsBackToTitlesOnly(t *testing.T) {
	nodes := bigStructure(100, 2_000)
	got, fit, err := RenderStructureWithin(JSONRenderer{}, nodes, 12_000)
	if err != nil {
		t.Fatal(err)
	}
	if fit != FitTitlesOnly {
		t.Errorf("fit = %v, want FitTitlesOnly", fit)
	}
	if len(got) > 12_000 {
		t.Errorf("render = %d chars, exceeds 12k budget", len(got))
	}
	if strings.Contains(got, "summary") {
		t.Error("titles-only render must not contain summaries")
	}
}

func TestRenderStructureWithinTitlesOnlyFloorMayExceedBudget(t *testing.T) {
	nodes := bigStructure(100, 0)
	got, fit, err := RenderStructureWithin(JSONRenderer{}, nodes, 10)
	if err != nil {
		t.Fatal(err)
	}
	if fit != FitTitlesOnly {
		t.Errorf("fit = %v, want FitTitlesOnly", fit)
	}
	if len(got) <= 10 {
		t.Error("floor render should be returned as-is even over budget")
	}
}
