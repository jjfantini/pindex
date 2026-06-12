package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jjfantini/pindex/internal/exportout"
)

func writeRecord(t *testing.T, root, effort, doc, id string, rec exportout.AnswerRecord) string {
	t.Helper()
	dir := filepath.Join(root, "test-model", effort, doc, "answers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, id+".json")
	if err := os.WriteFile(p, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func fixture(t *testing.T) (*server, string, string) {
	t.Helper()
	root := t.TempDir()
	miss := exportout.AnswerRecord{
		FinancebenchID: "financebench_id_test1", DocName: "DOC_A.pdf",
		Question: "How much?", GoldAnswer: "42", GoldPages: []int{7},
		Predicted: "41", CitedPages: []int{8}, Label: "NAL",
	}
	hit := miss
	hit.Label = "AL"
	hit.Predicted = "42"
	p1 := writeRecord(t, root, "low", "DOC_A", "financebench_id_test1", miss)
	writeRecord(t, root, "high", "DOC_A", "financebench_id_test1", miss)
	writeRecord(t, root, "ultra", "DOC_A", "financebench_id_test1", hit) // AL: must not surface or be touched
	s := &server{resultsRoot: root, pages: newPageSource(filepath.Join(root, "no-such-ws"))}
	if err := s.load(); err != nil {
		t.Fatal(err)
	}
	return s, root, p1
}

func TestLoadGroupsOnlyNonAL(t *testing.T) {
	s, _, _ := fixture(t)
	refs := s.byID["financebench_id_test1"]
	if len(refs) != 2 {
		t.Fatalf("got %d records, want 2 (the AL one excluded)", len(refs))
	}
	d := s.data()
	if len(d.Questions) != 1 || d.Questions[0].ID != "financebench_id_test1" {
		t.Fatalf("data = %+v", d.Questions)
	}
	if len(d.Questions[0].GoldText) != 0 {
		t.Error("gold text should be empty without a workspace")
	}
}

func TestApplyRelabelsAllEffortsAndWraps(t *testing.T) {
	s, _, p1 := fixture(t)
	called := ""
	old := runAggregate
	runAggregate = func(root string) (string, error) { called = root; return "SCOREBOARD", nil }
	defer func() { runAggregate = old }()

	resp, err := s.apply(applyRequest{Changes: []applyChange{
		{ID: "financebench_id_test1", Label: "SEDC", Reason: "same evidence, different fork."},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Updated) != 2 || resp.Scoreboard != "SCOREBOARD" || called == "" {
		t.Fatalf("resp = %+v (aggregate called with %q)", resp, called)
	}
	b, err := os.ReadFile(p1)
	if err != nil {
		t.Fatal(err)
	}
	var rec exportout.AnswerRecord
	if err := json.Unmarshal(b, &rec); err != nil {
		t.Fatal(err)
	}
	if rec.Label != "SEDC" {
		t.Errorf("label = %q", rec.Label)
	}
	if !strings.HasPrefix(rec.LabelReason, "Label: SEDC - Subjective Evaluation") ||
		!strings.Contains(rec.LabelReason, "same evidence, different fork.") {
		t.Errorf("reason not wrapped: %q", rec.LabelReason)
	}
}

func TestApplyRejectsALAndUnknown(t *testing.T) {
	s, _, _ := fixture(t)
	if _, err := s.apply(applyRequest{Changes: []applyChange{{ID: "financebench_id_test1", Label: "AL"}}}); err == nil {
		t.Error("AL must be rejected — judge-only label")
	}
	if _, err := s.apply(applyRequest{Changes: []applyChange{{ID: "nope", Label: "NAL"}}}); err == nil {
		t.Error("unknown id must be rejected")
	}
}

func TestWrapReasonPassThrough(t *testing.T) {
	if got := wrapReason("NAL", ""); got != "" {
		t.Errorf("empty reason should stay empty, got %q", got)
	}
	pre := "Label: MVA - Multiple Valid Approaches\n\nDetailed Reason: already formatted."
	if got := wrapReason("MVA", pre); got != pre {
		t.Errorf("pre-formatted reason should pass through, got %q", got)
	}
}
