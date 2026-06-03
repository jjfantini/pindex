package ask

import (
	"context"
	"strings"
	"testing"

	"github.com/jjfantini/pindex/internal/llm"
	"github.com/jjfantini/pindex/internal/tree"
)

func sampleDoc() tree.Document {
	return tree.Document{
		Type: tree.DocPDF, PageCount: 3,
		Structure: []tree.TreeNode{
			{Title: "Intro", NodeID: "0000", StartIndex: 1, EndIndex: 1},
			{Title: "Financials", NodeID: "0001", StartIndex: 2, EndIndex: 3},
		},
		Pages: []tree.PageContent{
			{Page: 1, Content: "intro text"},
			{Page: 2, Content: "Revenue was $1,234 in 2023."},
			{Page: 3, Content: "more financials"},
		},
	}
}

func TestAskSelectsThenAnswers(t *testing.T) {
	mock := llm.NewMock("m",
		llm.MockResponse{Content: `{"thinking":"financials live on p2","pages":"2"}`},
		llm.MockResponse{Content: `{"thinking":"found it","answer":"Revenue was $1,234.","pages_used":"2"}`},
	)
	ans, err := New(mock, "m").Ask(context.Background(), sampleDoc(), "What was revenue?")
	if err != nil {
		t.Fatal(err)
	}
	if ans.Text != "Revenue was $1,234." {
		t.Errorf("answer = %q", ans.Text)
	}
	if len(ans.CitedPages) != 1 || ans.CitedPages[0] != 2 {
		t.Errorf("cited = %v want [2]", ans.CitedPages)
	}
	if ans.SelectedPages != "2" {
		t.Errorf("selected = %q want 2", ans.SelectedPages)
	}
	if mock.CallCount() != 2 {
		t.Errorf("calls = %d want 2", mock.CallCount())
	}
	// The answer prompt must contain the fetched page-2 content (grounding).
	calls := mock.Calls()
	if !strings.Contains(calls[1].Messages[0].Content, "Revenue was $1,234") {
		t.Error("answer prompt should embed the fetched page content")
	}
}

func TestAskRetriesInvalidPageSelector(t *testing.T) {
	mock := llm.NewMock("m",
		llm.MockResponse{Content: `{"pages":"garbage"}`}, // invalid selector -> retry
		llm.MockResponse{Content: `{"pages":"2"}`},       // valid
		llm.MockResponse{Content: `{"answer":"ok","pages_used":"2"}`},
	)
	ans, err := New(mock, "m").Ask(context.Background(), sampleDoc(), "q")
	if err != nil {
		t.Fatal(err)
	}
	if ans.Text != "ok" {
		t.Errorf("answer = %q", ans.Text)
	}
	if mock.CallCount() != 3 {
		t.Errorf("calls = %d want 3 (one select retry)", mock.CallCount())
	}
}
