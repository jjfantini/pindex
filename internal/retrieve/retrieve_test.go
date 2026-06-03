package retrieve

import (
	"strings"
	"testing"

	"github.com/jjfantini/pindex/internal/tree"
)

func doc() tree.Document {
	return tree.Document{
		ID: "id1", Type: tree.DocPDF, DocName: "n.pdf", DocDescription: "desc", PageCount: 3,
		Structure: []tree.TreeNode{{Title: "A", Text: "SECRET-TEXT", StartIndex: 1, EndIndex: 2}},
		Pages: []tree.PageContent{
			{Page: 1, Content: "one"}, {Page: 2, Content: "two"}, {Page: 3, Content: "three"},
		},
	}
}

func TestGetDocumentMetadata(t *testing.T) {
	md, err := GetDocument(doc())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"doc_id":"id1"`, `"page_count":3`, `"status":"completed"`} {
		if !strings.Contains(md, want) {
			t.Errorf("metadata missing %s: %s", want, md)
		}
	}
}

func TestGetStructureStripsText(t *testing.T) {
	st, err := GetStructure(doc())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(st, "SECRET-TEXT") {
		t.Errorf("structure must not include page text: %s", st)
	}
	if !strings.Contains(st, `"title":"A"`) {
		t.Errorf("structure should include titles: %s", st)
	}
}

func TestGetPageContentSelects(t *testing.T) {
	pc, err := GetPageContent(doc(), "1,3")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pc, "one") || !strings.Contains(pc, "three") || strings.Contains(pc, "two") {
		t.Errorf("page content selection wrong: %s", pc)
	}
}

func TestPagesReturnsTyped(t *testing.T) {
	pcs, err := Pages(doc(), "2")
	if err != nil {
		t.Fatal(err)
	}
	if len(pcs) != 1 || pcs[0].Page != 2 || pcs[0].Content != "two" {
		t.Errorf("Pages = %+v", pcs)
	}
	if _, err := Pages(doc(), "bad"); err == nil {
		t.Error("invalid selector should error")
	}
}
