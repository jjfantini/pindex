package tree

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestTreeNodeJSONRoundTrip locks the on-disk JSON shape (interchangeable with
// upstream PageIndex) and verifies a nested structure survives marshal/unmarshal.
func TestTreeNodeJSONRoundTrip(t *testing.T) {
	in := TreeNode{
		Title: "Financial Stability", NodeID: "0006", StartIndex: 21, EndIndex: 22,
		Summary: "The Federal Reserve ...",
		Nodes: []TreeNode{
			{Title: "Risks", NodeID: "0007", StartIndex: 21, EndIndex: 21},
		},
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out TreeNode
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round-trip mismatch:\n in=%+v\nout=%+v", in, out)
	}
}

func TestTreeNodeOmitsEmptyOptionalFields(t *testing.T) {
	data, err := json.Marshal(TreeNode{Title: "X", StartIndex: 1, EndIndex: 2})
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	want := `{"title":"X","start_index":1,"end_index":2}`
	if got != want {
		t.Errorf("json = %s, want %s", got, want)
	}
}

func TestDocumentJSONRoundTrip(t *testing.T) {
	in := Document{
		ID: "abc", Type: DocPDF, Path: "/x.pdf", DocName: "x.pdf", PageCount: 2,
		Structure: []TreeNode{{Title: "Intro", StartIndex: 1, EndIndex: 1}},
		Pages:     []PageContent{{Page: 1, Content: "hello"}},
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out Document
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round-trip mismatch:\n in=%+v\nout=%+v", in, out)
	}
}
