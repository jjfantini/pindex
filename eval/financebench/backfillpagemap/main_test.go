package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jjfantini/pindex/internal/exportout"
	"github.com/jjfantini/pindex/internal/tree"
)

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readJSON[T any](t *testing.T, path string) T {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestBackfillWorkspaceDocsWritesPageMap(t *testing.T) {
	root := t.TempDir()
	ws := filepath.Join(root, "testdata", "ws")
	docPath := filepath.Join(ws, "docs", "d1.json")
	writeJSON(t, docPath, tree.Document{
		ID:        "d1",
		DocName:   "ACME_2023_10K.pdf",
		Type:      tree.DocPDF,
		PageCount: 4,
		Pages: []tree.PageContent{
			{Page: 3, Content: "body\n\n1"},
			{Page: 4, Content: "body\n\n2"},
		},
	})

	docs, err := backfillWorkspaceDocs(ws)
	if err != nil {
		t.Fatal(err)
	}

	got := readJSON[tree.Document](t, docPath)
	want := tree.PageMap{{PhysStart: 3, PhysEnd: 4, Offset: 2}}
	if !reflect.DeepEqual(got.PageMap, want) {
		t.Fatalf("stored PageMap = %#v, want %#v", got.PageMap, want)
	}
	if _, ok := docs["ACME_2023_10K"]; !ok {
		t.Fatalf("doc index missing sanitized doc key: %#v", docs)
	}
}

func TestBackfillAnswerRecordsRecomputesPrintedCitationsAndPageHit(t *testing.T) {
	root := t.TempDir()
	answerPath := filepath.Join(root, "results", "model", "low", "ACME_2023_10K", "answers", "q1.json")
	writeJSON(t, answerPath, exportout.AnswerRecord{
		FinancebenchID: "q1",
		DocName:        "ACME_2023_10K.pdf",
		Question:       "Q?",
		Predicted:      "A",
		CitedPages:     []int{3},
		GoldPages:      []int{1},
	})
	docs := map[string]tree.Document{
		"ACME_2023_10K": {
			DocName: "ACME_2023_10K.pdf",
			PageMap: tree.PageMap{{PhysStart: 3, PhysEnd: 4, Offset: 2}},
		},
	}

	if err := backfillAnswerRecords(filepath.Join(root, "results"), docs); err != nil {
		t.Fatal(err)
	}

	got := readJSON[exportout.AnswerRecord](t, answerPath)
	if !reflect.DeepEqual(got.CitedPagesPrinted, []int{1}) {
		t.Fatalf("CitedPagesPrinted = %#v, want [1]", got.CitedPagesPrinted)
	}
	if !got.PageHit {
		t.Fatal("PageHit should be recomputed through the page map")
	}
}

func TestBackfillTreesWritesFlatTreeExports(t *testing.T) {
	root := t.TempDir()
	results := filepath.Join(root, "results")
	treePath := filepath.Join(results, "model", "trees", "ACME_2023_10K_pindex.json")
	writeJSON(t, treePath, map[string]any{"doc_name": "ACME_2023_10K.pdf"})
	docs := map[string]tree.Document{
		"ACME_2023_10K": {
			ID:        "d1",
			DocName:   "ACME_2023_10K.pdf",
			Type:      tree.DocPDF,
			PageCount: 4,
			PageMap:   tree.PageMap{{PhysStart: 3, PhysEnd: 4, Offset: 2}},
			Structure: []tree.TreeNode{{Title: "S1", StartIndex: 3, EndIndex: 4, Text: "strip me"}},
		},
	}

	if err := backfillTreeExports(results, docs); err != nil {
		t.Fatal(err)
	}

	got := readJSON[exportout.TreeExport](t, treePath)
	if !reflect.DeepEqual(got.PageMap, docs["ACME_2023_10K"].PageMap) {
		t.Fatalf("tree PageMap = %#v, want %#v", got.PageMap, docs["ACME_2023_10K"].PageMap)
	}
	if got.Structure[0].Text != "" {
		t.Fatal("flat tree export should remain text-stripped")
	}
}
