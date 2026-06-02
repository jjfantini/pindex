package tree

import (
	"reflect"
	"testing"
)

func TestListToTreeNestsByStructure(t *testing.T) {
	got := ListToTree([]FlatItem{
		{Structure: "1", Title: "A", StartIndex: 1, EndIndex: 5},
		{Structure: "1.1", Title: "B", StartIndex: 1, EndIndex: 2},
		{Structure: "1.2", Title: "C", StartIndex: 3, EndIndex: 5},
		{Structure: "2", Title: "D", StartIndex: 6, EndIndex: 10},
	})
	want := []TreeNode{
		{Title: "A", StartIndex: 1, EndIndex: 5, Nodes: []TreeNode{
			{Title: "B", StartIndex: 1, EndIndex: 2},
			{Title: "C", StartIndex: 3, EndIndex: 5},
		}},
		{Title: "D", StartIndex: 6, EndIndex: 10},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tree mismatch:\n got=%+v\nwant=%+v", got, want)
	}
}

func TestListToTreeDeepNesting(t *testing.T) {
	got := ListToTree([]FlatItem{
		{Structure: "1", Title: "A"},
		{Structure: "1.1", Title: "B"},
		{Structure: "1.1.1", Title: "C"},
	})
	if len(got) != 1 || len(got[0].Nodes) != 1 || len(got[0].Nodes[0].Nodes) != 1 {
		t.Fatalf("expected A>B>C chain, got %+v", got)
	}
	if got[0].Nodes[0].Nodes[0].Title != "C" {
		t.Errorf("deepest title = %q, want C", got[0].Nodes[0].Nodes[0].Title)
	}
}

func TestListToTreeOrphanBecomesRoot(t *testing.T) {
	// "2.1" appears before its parent "2" exists -> it must become a root.
	got := ListToTree([]FlatItem{
		{Structure: "1", Title: "A"},
		{Structure: "2.1", Title: "orphan"},
		{Structure: "2", Title: "B"},
	})
	if len(got) != 3 {
		t.Fatalf("expected 3 roots (A, orphan, B), got %d: %+v", len(got), got)
	}
	if got[1].Title != "orphan" {
		t.Errorf("root[1] = %q, want orphan", got[1].Title)
	}
}

func TestListToTreeLeafHasNilNodes(t *testing.T) {
	got := ListToTree([]FlatItem{{Structure: "1", Title: "A"}})
	if got[0].Nodes != nil {
		t.Errorf("leaf Nodes should be nil for omitempty, got %v", got[0].Nodes)
	}
}

func TestListToTreeEmpty(t *testing.T) {
	if got := ListToTree(nil); len(got) != 0 {
		t.Errorf("empty input should yield empty tree, got %+v", got)
	}
}

func TestPostProcessSpans(t *testing.T) {
	// B starts at the top of its page (AppearStart) so A ends at 4 (= 5-1);
	// C does not, so B ends at 8 (= C's page). Last item ends at endPhysicalIndex.
	got := PostProcess([]PostItem{
		{Structure: "1", Title: "A", PhysicalIndex: 1},
		{Structure: "2", Title: "B", PhysicalIndex: 5, AppearStart: true},
		{Structure: "3", Title: "C", PhysicalIndex: 8},
	}, 12)
	want := []TreeNode{
		{Title: "A", StartIndex: 1, EndIndex: 4},
		{Title: "B", StartIndex: 5, EndIndex: 8},
		{Title: "C", StartIndex: 8, EndIndex: 12},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("spans mismatch:\n got=%+v\nwant=%+v", got, want)
	}
}

func TestWriteNodeIDsPreOrder(t *testing.T) {
	tree := []TreeNode{
		{Title: "A", Nodes: []TreeNode{{Title: "B"}, {Title: "C"}}},
		{Title: "D"},
	}
	WriteNodeIDs(tree)
	if tree[0].NodeID != "0000" {
		t.Errorf("A id = %q, want 0000", tree[0].NodeID)
	}
	if tree[0].Nodes[0].NodeID != "0001" || tree[0].Nodes[1].NodeID != "0002" {
		t.Errorf("B,C ids = %q,%q want 0001,0002", tree[0].Nodes[0].NodeID, tree[0].Nodes[1].NodeID)
	}
	if tree[1].NodeID != "0003" {
		t.Errorf("D id = %q, want 0003", tree[1].NodeID)
	}
}

func TestStripTextDeepCopies(t *testing.T) {
	in := []TreeNode{{Title: "A", Text: "secret", Nodes: []TreeNode{{Title: "B", Text: "more"}}}}
	out := StripText(in)
	if out[0].Text != "" || out[0].Nodes[0].Text != "" {
		t.Errorf("text not stripped: %+v", out)
	}
	if in[0].Text != "secret" || in[0].Nodes[0].Text != "more" {
		t.Errorf("input was mutated: %+v", in)
	}
}
