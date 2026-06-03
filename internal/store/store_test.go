package store

import (
	"testing"

	"github.com/jjfantini/pindex/internal/tree"
)

func sampleDoc(id string) tree.Document {
	return tree.Document{
		ID: id, Type: tree.DocPDF, Path: "/x.pdf", DocName: "x.pdf",
		DocDescription: "a test doc", PageCount: 2,
		Structure: []tree.TreeNode{{Title: "A", StartIndex: 1, EndIndex: 2}},
		Pages:     []tree.PageContent{{Page: 1, Content: "hi"}},
	}
}

func TestStoreSaveLoadListHas(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	if s.Has("abc123") {
		t.Error("should not have doc before save")
	}
	if err := s.Save(sampleDoc("abc123")); err != nil {
		t.Fatal(err)
	}
	if !s.Has("abc123") {
		t.Error("should have doc after save")
	}

	got, err := s.Load("abc123")
	if err != nil {
		t.Fatal(err)
	}
	if got.DocName != "x.pdf" || len(got.Pages) != 1 || got.Pages[0].Content != "hi" {
		t.Errorf("loaded = %+v", got)
	}

	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != "abc123" || list[0].PageCount != 2 || list[0].Description != "a test doc" {
		t.Errorf("list = %+v", list)
	}
}

func TestStoreReopenPersists(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Save(sampleDoc("k")); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s2.Close() }()
	if !s2.Has("k") {
		t.Error("catalog should persist across reopen")
	}
	if _, err := s2.Load("k"); err != nil {
		t.Errorf("blob should persist across reopen: %v", err)
	}
}

func TestDocIDStableAndDistinct(t *testing.T) {
	a := DocID("/some/path.pdf")
	if a == "" || a != DocID("/some/path.pdf") {
		t.Errorf("DocID should be stable, got %q", a)
	}
	if a == DocID("/other/path.pdf") {
		t.Error("different paths should produce different ids")
	}
}
