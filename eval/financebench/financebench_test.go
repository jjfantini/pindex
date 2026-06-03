package financebench

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jjfantini/pindex/internal/ask"
	"github.com/jjfantini/pindex/internal/llm"
	"github.com/jjfantini/pindex/internal/tree"
)

func TestLoadQuestionsAndGoldPages(t *testing.T) {
	p := filepath.Join(t.TempDir(), "q.jsonl")
	content := `{"financebench_id":"q1","doc_name":"ACME_2023_10K","question":"What was revenue?","answer":"$1,234","evidence":[{"evidence_text":"a","evidence_page_num":42},{"evidence_text":"b","evidence_page_num":"42"}]}

{"financebench_id":"q2","doc_name":"ACME_2023_10K","question":"x","answer":"y","evidence":[{"evidence_page_num":"7"}]}
`
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	qs, err := LoadQuestions(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(qs) != 2 {
		t.Fatalf("loaded %d questions, want 2 (blank line skipped)", len(qs))
	}
	if gp := GoldPages(qs[0]); len(gp) != 1 || gp[0] != 42 {
		t.Errorf("gold pages = %v want [42] (int + string deduped)", gp)
	}
	if gp := GoldPages(qs[1]); len(gp) != 1 || gp[0] != 7 {
		t.Errorf("q2 gold = %v want [7]", gp)
	}
}

func TestRecallAtPage(t *testing.T) {
	if !RecallAtPage([]int{42}, []int{40, 42, 43}) {
		t.Error("should report a hit")
	}
	if RecallAtPage([]int{42}, []int{1, 2}) {
		t.Error("should report a miss")
	}
	if RecallAtPage(nil, []int{1}) {
		t.Error("no gold pages -> false")
	}
}

func TestRunScoresAccuracyAndRecall(t *testing.T) {
	doc := tree.Document{
		Type: tree.DocPDF, DocName: "ACME_2023_10K.pdf", PageCount: 50,
		Structure: []tree.TreeNode{{Title: "Financials", StartIndex: 42, EndIndex: 42}},
		Pages:     []tree.PageContent{{Page: 42, Content: "Total revenue increased to 1234 million dollars."}},
	}
	askProvider := llm.NewMock("ask",
		llm.MockResponse{Content: `{"pages":"42"}`},
		llm.MockResponse{Content: `{"answer":"Revenue was $1,234.","pages_used":"42"}`},
	)
	judge := llm.NewMock("judge", llm.MockResponse{Content: `{"correct":true}`})

	qs := []Question{{
		ID: "q1", DocName: "ACME_2023_10K", Question: "What was revenue?", Answer: "$1,234",
		Evidence: []Evidence{{Text: "revenue increased to 1234 million", Page: 42}},
	}}
	lookup := func(string) (tree.Document, bool) { return doc, true }

	results, agg := Run(context.Background(), ask.New(askProvider, "m"), judge, "j", qs, lookup)
	if len(results) != 1 || !results[0].Correct || !results[0].PageHit || !results[0].EvidenceHit {
		t.Fatalf("result = %+v", results[0])
	}
	if agg.AnswerAccuracy() != 1.0 || agg.RecallAtPage() != 1.0 || agg.EvidenceRecall() != 1.0 {
		t.Errorf("accuracy=%.2f page=%.2f evidence=%.2f want 1.0/1.0/1.0",
			agg.AnswerAccuracy(), agg.RecallAtPage(), agg.EvidenceRecall())
	}
}

func TestEvidenceHit(t *testing.T) {
	doc := tree.Document{Pages: []tree.PageContent{
		{Page: 5, Content: "The company reported net sales of 1,234 million dollars in fiscal 2022."},
	}}
	q := Question{Evidence: []Evidence{{Text: "net sales of 1234 million dollars fiscal 2022", Page: 5}}}
	if !EvidenceHit(doc, []int{5}, q) {
		t.Error("should hit: cited page contains the evidence words")
	}
	if EvidenceHit(doc, []int{1}, q) {
		t.Error("should miss: page 1 has no content")
	}
	if EvidenceHit(doc, []int{5}, Question{Evidence: []Evidence{{Page: 5}}}) {
		t.Error("no evidence text -> no hit")
	}
}

func TestRunMissingDocIsNotScored(t *testing.T) {
	qs := []Question{{ID: "q", DocName: "NOPE"}}
	lookup := func(string) (tree.Document, bool) { return tree.Document{}, false }
	results, agg := Run(context.Background(), ask.New(llm.NewMock("a"), "m"), llm.NewMock("j"), "j", qs, lookup)
	if results[0].Err == nil {
		t.Error("missing doc should produce an error result")
	}
	if agg.Scored != 0 {
		t.Errorf("missing doc should not be scored, got scored=%d", agg.Scored)
	}
}
