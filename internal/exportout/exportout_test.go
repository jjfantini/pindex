package exportout

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jjfantini/pindex/eval/financebench"
	"github.com/jjfantini/pindex/internal/tree"
)

func TestSanitize(t *testing.T) {
	cases := map[string]string{
		"ACME 2023/10-K.pdf":   "ACME_2023_10-K",
		"BESTBUY_2024Q2_10Q":   "BESTBUY_2024Q2_10Q",
		"a b  c.PDF":           "a_b_c",
		"":                     "doc",
		"///":                  "doc",
		"weird:*?name":         "weird_name",
		"trailing_._":          "trailing",
		"keep.dots.and-dashes": "keep.dots.and-dashes",
	}
	for in, want := range cases {
		if got := Sanitize(in); got != want {
			t.Errorf("Sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestQuestionSlug(t *testing.T) {
	a := QuestionSlug("What is revenue?")
	b := QuestionSlug("What is net income?")
	if a == b {
		t.Fatalf("distinct questions produced the same slug: %q", a)
	}
	if a != QuestionSlug("What is revenue?") {
		t.Fatal("QuestionSlug is not deterministic")
	}
	if strings.ContainsAny(a, "/\\ ?") {
		t.Errorf("slug %q contains unsafe chars", a)
	}
}

func TestAutoLabelAndAdjusted(t *testing.T) {
	if AutoLabel(true) != LabelAL {
		t.Error("correct should auto-label AL")
	}
	if AutoLabel(false) != LabelNAL {
		t.Error("wrong should auto-label NAL")
	}
	for _, l := range []string{LabelAL, LabelMVA, LabelBE} {
		if !AdjustedCorrect(l) {
			t.Errorf("%s should count as adjusted-correct", l)
		}
	}
	for _, l := range []string{LabelNAL, LabelSEDC, ""} {
		if AdjustedCorrect(l) {
			t.Errorf("%s should NOT count as adjusted-correct", l)
		}
	}
}

func sampleDoc() tree.Document {
	return tree.Document{
		ID:         "d1",
		DocName:    "ACME 2023 10-K.pdf",
		Type:       tree.DocPDF,
		PageCount:  3,
		PageOffset: 1,
		Structure: []tree.TreeNode{{
			Title: "S1", StartIndex: 1, EndIndex: 2, Summary: "sum", Text: "NODE_TEXT",
			Nodes: []tree.TreeNode{{Title: "S1.1", StartIndex: 1, EndIndex: 1, Text: "CHILD_TEXT"}},
		}},
		Pages: []tree.PageContent{{Page: 1, Content: "PAGE_ONE_CONTENT"}},
	}
}

func TestWriteTreeStripsTextByDefault(t *testing.T) {
	dir := t.TempDir()
	path, err := WriteTree(dir, sampleDoc(), false)
	if err != nil {
		t.Fatal(err)
	}
	if base := filepath.Base(path); base != "ACME_2023_10-K_pindex.json" {
		t.Errorf("tree file name = %q", base)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, leak := range []string{"NODE_TEXT", "CHILD_TEXT", "PAGE_ONE_CONTENT"} {
		if strings.Contains(s, leak) {
			t.Errorf("default tree must not contain page text, found %q", leak)
		}
	}
	var te TreeExport
	if err := json.Unmarshal(data, &te); err != nil {
		t.Fatal(err)
	}
	if te.DocName != "ACME 2023 10-K.pdf" || te.PageOffset != 1 {
		t.Errorf("metadata not preserved: %+v", te)
	}
	if len(te.Structure) != 1 || te.Pages != nil {
		t.Errorf("expected structure kept, pages omitted: %+v", te)
	}
}

func TestWriteTreeIncludePages(t *testing.T) {
	dir := t.TempDir()
	path, err := WriteTree(dir, sampleDoc(), true)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	s := string(data)
	if !strings.Contains(s, "NODE_TEXT") || !strings.Contains(s, "PAGE_ONE_CONTENT") {
		t.Error("include-pages tree should retain node text and page content")
	}
	var te TreeExport
	if err := json.Unmarshal(data, &te); err != nil {
		t.Fatal(err)
	}
	if len(te.Pages) != 1 {
		t.Errorf("expected 1 page, got %d", len(te.Pages))
	}
}

func TestAnswerRecordCarriesVerification(t *testing.T) {
	r := financebench.RunResult{
		Question:     financebench.Question{ID: "fb_1", Question: "Q?", DocName: "doc.pdf"},
		Predicted:    "42",
		Verification: "unsupported",
	}
	rec := recordFromResult(r)
	if rec.Verification != "unsupported" {
		t.Errorf("verification = %q want unsupported", rec.Verification)
	}
	path, err := WriteAnswer(t.TempDir(), rec)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"verification": "unsupported"`) {
		t.Error("answer record JSON should carry the verification verdict")
	}
	// omitempty: an unverified record must not emit an empty verification key.
	rec.Verification = ""
	path2, err := WriteAnswer(t.TempDir(), rec)
	if err != nil {
		t.Fatal(err)
	}
	data2, err := os.ReadFile(path2)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data2), "verification") {
		t.Error("empty verification should be omitted from the record")
	}
}

func TestMafinRecordSchemaAndAnswerOnly(t *testing.T) {
	r := financebench.RunResult{
		Question:  financebench.Question{ID: "fb_1", Question: "Q?", Answer: "GOLD"},
		Predicted: "42",
		Reasoning: "SECRET_REASONING_should_not_appear",
		Correct:   true,
	}
	mr := mafinFromResult(r)
	if mr.PindexAnswer != "42" {
		t.Errorf("pindex_answer should be the concise answer, got %q", mr.PindexAnswer)
	}
	if mr.Label != LabelAL {
		t.Errorf("label = %q, want AL", mr.Label)
	}

	dir := t.TempDir()
	path, err := WriteMafinResult(dir, "gpt-4o", []MafinRecord{mr})
	if err != nil {
		t.Fatal(err)
	}
	if base := filepath.Base(path); base != "result_gpt-4o.json" {
		t.Errorf("result file name = %q", base)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "SECRET_REASONING") {
		t.Error("Mafin result must not contain reasoning")
	}
	var recs []map[string]any
	if err := json.Unmarshal(data, &recs); err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	want := map[string]bool{"id": true, "question": true, "label": true, "benchmark_answer": true, "pindex_answer": true}
	for k := range recs[0] {
		if !want[k] {
			t.Errorf("unexpected key %q in Mafin record", k)
		}
		delete(want, k)
	}
	if len(want) != 0 {
		t.Errorf("missing keys: %v", want)
	}
}

func TestWriteHumanEvalCSVOnlyDisagreements(t *testing.T) {
	dir := t.TempDir()
	recs := []MafinRecord{
		{ID: "1", Question: "Qa", Label: LabelAL, BenchmarkAnswer: "Ga", PindexAnswer: "Pa"},
		{ID: "2", Question: "Qb", Label: LabelNAL, BenchmarkAnswer: "Gb", PindexAnswer: "Pb"},
		{ID: "3", Question: "Qc", Label: LabelMVA, BenchmarkAnswer: "Gc", PindexAnswer: "Pc"},
	}
	path, err := WriteHumanEvalCSV(dir, "gpt-4o", recs)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if lines[0] != "id,question,label,label reason,benchmark answer,pindex answer" {
		t.Errorf("header = %q", lines[0])
	}
	if len(lines) != 2 { // header + only the NAL row
		t.Fatalf("expected header + 1 disagreement row, got %d lines: %v", len(lines), lines)
	}
	if !strings.HasPrefix(lines[1], "2,Qb,NAL,") {
		t.Errorf("disagreement row = %q", lines[1])
	}
}

func TestRescoreExcusesMVABE(t *testing.T) {
	dir := t.TempDir()
	if err := WriteSummary(dir, Summary{Scored: 2, AnswerAccuracyRaw: 0.5}); err != nil {
		t.Fatal(err)
	}
	mk := func(label string) []MafinRecord {
		return []MafinRecord{
			{ID: "1", Question: "Qa", Label: LabelAL, BenchmarkAnswer: "Ga", PindexAnswer: "Pa"},
			{ID: "2", Question: "Qb", Label: label, BenchmarkAnswer: "Gb", PindexAnswer: "Pb"},
		}
	}
	path, err := WriteMafinResult(dir, "gpt-4o", mk(LabelNAL))
	if err != nil {
		t.Fatal(err)
	}

	raw, adjusted, counts, rawKnown, err := Rescore(path)
	if err != nil {
		t.Fatal(err)
	}
	if !rawKnown || raw != 0.5 {
		t.Errorf("raw from summary = %v (known=%v), want 0.5", raw, rawKnown)
	}
	if adjusted != 0.5 {
		t.Errorf("adjusted with NAL = %v, want 0.5", adjusted)
	}
	if counts[LabelAL] != 1 || counts[LabelNAL] != 1 {
		t.Errorf("label counts = %v", counts)
	}

	// Human flips the disagreement to a benchmark error → it's excused.
	if _, err := WriteMafinResult(dir, "gpt-4o", mk(LabelBE)); err != nil {
		t.Fatal(err)
	}
	_, adjusted, _, _, err = Rescore(path)
	if err != nil {
		t.Fatal(err)
	}
	if adjusted != 1.0 {
		t.Errorf("adjusted after NAL->BE = %v, want 1.0", adjusted)
	}
}

func TestExportEvalLayout(t *testing.T) {
	dir := t.TempDir()
	doc := tree.Document{
		ID: "d1", DocName: "BESTBUY_2024Q2_10Q", Type: tree.DocPDF, PageCount: 30,
		Structure: []tree.TreeNode{{Title: "Stores", StartIndex: 17, EndIndex: 17, Summary: "store table"}},
	}
	// Two doc_name spellings that both resolve to the same document.
	lookup := func(name string) (tree.Document, bool) { return doc, true }
	qs := []financebench.Question{
		{ID: "q1", DocName: "BESTBUY", Question: "Qa", Answer: "Ga"},
		{ID: "q2", DocName: "bestbuy", Question: "Qb", Answer: "Gb"},
	}
	results := []financebench.RunResult{
		{Question: qs[0], Predicted: "Pa", Correct: true},
		{Question: qs[1], Predicted: "Pb", Correct: false},
	}
	sum := Summary{Model: "gpt-4o", JudgeModel: "claude", QuestionsTotal: 2, Scored: 2, AnswerAccuracyRaw: 0.5}
	if err := ExportEval(dir, sum, qs, results, lookup, false, "gpt-4o"); err != nil {
		t.Fatal(err)
	}

	// Exactly one tree folder (deduped by doc.ID), with the tree + both answers in it.
	docDir := filepath.Join(dir, "BESTBUY_2024Q2_10Q")
	if _, err := os.Stat(filepath.Join(docDir, "BESTBUY_2024Q2_10Q_pindex.json")); err != nil {
		t.Errorf("missing tree file: %v", err)
	}
	for _, id := range []string{"q1", "q2"} {
		if _, err := os.Stat(filepath.Join(docDir, "answers", id+".json")); err != nil {
			t.Errorf("missing answer %s: %v", id, err)
		}
	}

	qlines, _ := os.ReadFile(filepath.Join(dir, "questions.jsonl"))
	if n := len(strings.Split(strings.TrimSpace(string(qlines)), "\n")); n != 2 {
		t.Errorf("questions.jsonl lines = %d, want 2", n)
	}

	resData, err := os.ReadFile(filepath.Join(dir, "result_gpt-4o.json"))
	if err != nil {
		t.Fatal(err)
	}
	var recs []MafinRecord
	if err := json.Unmarshal(resData, &recs); err != nil || len(recs) != 2 {
		t.Fatalf("result records = %d (err %v)", len(recs), err)
	}

	sumData, err := os.ReadFile(filepath.Join(dir, "summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got Summary
	if err := json.Unmarshal(sumData, &got); err != nil {
		t.Fatal(err)
	}
	if got.AnswerAccuracyAdjusted != 0.5 {
		t.Errorf("adjusted = %v, want 0.5 (1 AL of 2 scored)", got.AnswerAccuracyAdjusted)
	}
	if got.LabelCounts[LabelAL] != 1 || got.LabelCounts[LabelNAL] != 1 {
		t.Errorf("label counts = %v", got.LabelCounts)
	}
	if _, err := os.Stat(filepath.Join(dir, "human_evaluations", "gpt-4o.csv")); err != nil {
		t.Errorf("missing human-eval CSV: %v", err)
	}
}

func TestExportEvalErrorResult(t *testing.T) {
	dir := t.TempDir()
	lookup := func(name string) (tree.Document, bool) { return tree.Document{}, false }
	qs := []financebench.Question{{ID: "q1", DocName: "missing.pdf", Question: "Q", Answer: "G"}}
	results := []financebench.RunResult{{Question: qs[0], Err: errString("document not indexed")}}
	if err := ExportEval(dir, Summary{Model: "m"}, qs, results, lookup, false, "m"); err != nil {
		t.Fatal(err)
	}
	// An errored question still produces a record (filed under the question's doc_name).
	data, err := os.ReadFile(filepath.Join(dir, "missing", "answers", "q1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rec AnswerRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatal(err)
	}
	if rec.Error == "" {
		t.Error("errored result should carry an error message")
	}
	if rec.Label != "" {
		t.Errorf("errored result should be unlabeled, got %q", rec.Label)
	}
}

// errString is a tiny error helper so the test needs no extra imports.
type errString string

func (e errString) Error() string { return string(e) }
