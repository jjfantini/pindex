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

func TestRecallAtPageOffset(t *testing.T) {
	// printed page 5 -> physical 7 with a +2 offset; the cited physical page 7 hits.
	if !RecallAtPageOffset([]int{5}, []int{7}, 2) {
		t.Error("offset-aligned gold should hit the cited physical page")
	}
	// without the alignment the same comparison misses.
	if RecallAtPage([]int{5}, []int{7}) {
		t.Error("raw (unaligned) comparison should miss")
	}
	// offset 0 degrades to the raw comparison.
	if !RecallAtPageOffset([]int{7}, []int{7}, 0) {
		t.Error("offset 0 should behave like RecallAtPage")
	}
}

func TestRecallAtPageMapHandlesPiecewiseDrift(t *testing.T) {
	m := tree.PageMap{
		{PhysStart: 50, PhysEnd: 56, Offset: 2},
		{PhysStart: 61, PhysEnd: 64, Offset: 4},
	}

	if !RecallAtPageMap([]int{57}, []int{61}, m, 0) {
		t.Error("page map should align printed page 57 to physical page 61")
	}
	if RecallAtPageMap([]int{57}, []int{59}, m, 0) {
		t.Error("page map should not hit the wrong physical page")
	}
	if !RecallAtPageMap([]int{5}, []int{7}, nil, 2) {
		t.Error("nil page map should fall back to legacy page offset")
	}
}

func TestRunCarriesPrintedCitations(t *testing.T) {
	doc := tree.Document{
		Type:    tree.DocPDF,
		DocName: "ACME_2023_10K.pdf",
		PageMap: tree.PageMap{{PhysStart: 7, PhysEnd: 7, Offset: 2}},
		Structure: []tree.TreeNode{{
			Title:      "Financials",
			StartIndex: 7,
			EndIndex:   7,
		}},
		Pages: []tree.PageContent{{Page: 7, Content: "Revenue was 1234 million."}},
	}
	askProvider := llm.NewMock("ask",
		llm.MockResponse{Content: `{"pages":"7"}`},
		llm.MockResponse{Content: `{"thinking":"Page 7 shows revenue.","answer":"Revenue was $1,234.","pages_used":"7"}`},
	)
	judge := llm.NewMock("judge", llm.MockResponse{Content: `{"correct":true}`})
	qs := []Question{{
		ID:       "q1",
		DocName:  "ACME_2023_10K",
		Question: "What was revenue?",
		Answer:   "$1,234",
		Evidence: []Evidence{{Text: "revenue was 1234 million", Page: 5}},
	}}

	results, _ := Run(context.Background(), ask.New(askProvider, "m"), judge, "j", qs, func(string) (tree.Document, bool) {
		return doc, true
	}, nil)

	if got := results[0].CitedPrinted; len(got) != 1 || got[0] != 5 {
		t.Fatalf("CitedPrinted = %v, want [5]", got)
	}
	if !results[0].PageHit {
		t.Fatal("piecewise page map should make printed gold page 5 hit physical citation 7")
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
		llm.MockResponse{Content: `{"thinking":"Page 42 shows total revenue of 1234 million.","answer":"Revenue was $1,234.","pages_used":"42"}`},
	)
	judge := llm.NewMock("judge", llm.MockResponse{Content: `{"correct":true}`})

	qs := []Question{{
		ID: "q1", DocName: "ACME_2023_10K", Question: "What was revenue?", Answer: "$1,234",
		Evidence: []Evidence{{Text: "revenue increased to 1234 million", Page: 42}},
	}}
	lookup := func(string) (tree.Document, bool) { return doc, true }

	results, agg := Run(context.Background(), ask.New(askProvider, "m"), judge, "j", qs, lookup, nil)
	r := results[0]
	if len(results) != 1 || !r.Correct || !r.PageHit || !r.EvidenceHit || !r.EvidenceInDoc || r.Hallucinated {
		t.Fatalf("result = %+v", r)
	}
	// The pages selected and the answer reasoning must be surfaced (not discarded).
	if r.SelectedPages != "42" {
		t.Errorf("SelectedPages = %q, want %q", r.SelectedPages, "42")
	}
	if r.Reasoning == "" {
		t.Error("Reasoning should be populated from the answer's thinking field")
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
	results, agg := Run(context.Background(), ask.New(llm.NewMock("a"), "m"), llm.NewMock("j"), "j", qs, lookup, nil)
	if results[0].Err == nil {
		t.Error("missing doc should produce an error result")
	}
	if agg.Scored != 0 {
		t.Errorf("missing doc should not be scored, got scored=%d", agg.Scored)
	}
}

func TestEvidenceInDoc(t *testing.T) {
	doc := tree.Document{Pages: []tree.PageContent{
		{Page: 1, Content: "intro text"},
		{Page: 7, Content: "net sales of 1234 million dollars in fiscal 2022"},
	}}
	if !EvidenceInDoc(doc, Question{Evidence: []Evidence{{Text: "net sales of 1234 million dollars fiscal 2022"}}}) {
		t.Error("evidence on page 7 should be found in the doc")
	}
	if EvidenceInDoc(doc, Question{Evidence: []Evidence{{Text: "unrelated phrase about widgets gadgets sprockets"}}}) {
		t.Error("absent evidence should not be in the doc")
	}
}

func TestIsRefusal(t *testing.T) {
	for _, s := range []string{"I cannot find it.", "The document does not provide this.", "Unable to determine."} {
		if !isRefusal(s) {
			t.Errorf("%q should be a refusal", s)
		}
	}
	if isRefusal("Revenue was $1,234 million.") {
		t.Error("a real answer must not count as a refusal")
	}
}

func TestFunnelAndHallucination(t *testing.T) {
	doc := tree.Document{
		Type: tree.DocPDF, DocName: "D.pdf",
		Structure: []tree.TreeNode{{Title: "A", StartIndex: 1, EndIndex: 1}},
		Pages:     []tree.PageContent{{Page: 1, Content: "net sales were 1234 million dollars this fiscal year"}},
	}
	lookup := func(string) (tree.Document, bool) { return doc, true }
	ev := []Evidence{{Text: "net sales were 1234 million dollars fiscal year", Page: 1}}

	// q1 correct; q2 confident-wrong (hallucination); q3 honest refusal (not a hallucination).
	askProvider := llm.NewMock("ask",
		llm.MockResponse{Content: `{"pages":"1"}`}, llm.MockResponse{Content: `{"answer":"Net sales were 1234 million.","pages_used":"1"}`},
		llm.MockResponse{Content: `{"pages":"1"}`}, llm.MockResponse{Content: `{"answer":"Profit was 500 million.","pages_used":"1"}`},
		llm.MockResponse{Content: `{"pages":"1"}`}, llm.MockResponse{Content: `{"answer":"I cannot find it.","pages_used":"1"}`},
	)
	judge := llm.NewMock("judge",
		llm.MockResponse{Content: `{"correct":true}`},
		llm.MockResponse{Content: `{"correct":false}`},
		llm.MockResponse{Content: `{"correct":false}`},
	)
	qs := []Question{
		{ID: "q1", DocName: "D", Question: "sales?", Answer: "1234m", Evidence: ev},
		{ID: "q2", DocName: "D", Question: "profit?", Answer: "300m", Evidence: ev},
		{ID: "q3", DocName: "D", Question: "capex?", Answer: "42m", Evidence: ev},
	}
	results, agg := Run(context.Background(), ask.New(askProvider, "m"), judge, "j", qs, lookup, nil)

	if !results[1].Hallucinated {
		t.Error("q2 (confident wrong) should be flagged hallucinated")
	}
	if results[2].Hallucinated {
		t.Error("q3 (honest refusal) must NOT be flagged hallucinated")
	}
	ext, ret, ans, hal := agg.Funnel()
	if ext != 1.0 || ret != 1.0 {
		t.Errorf("extraction/retrieval funnel = %.2f/%.2f want 1.0/1.0", ext, ret)
	}
	if ans < 0.33 || ans > 0.34 {
		t.Errorf("answer rate = %.2f want ~0.33 (1 of 3)", ans)
	}
	if hal < 0.33 || hal > 0.34 {
		t.Errorf("hallucination rate = %.2f want ~0.33 (1 of 3)", hal)
	}
}
