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
