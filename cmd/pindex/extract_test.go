package main

import (
	"bytes"
	"strings"
	"testing"
)

// purego works in every build mode, so this command test is build-agnostic.
func TestExtractCommandPureGo(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"extract", "--backend", "purego", "../../testdata/sample.pdf"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := strings.ToLower(buf.String())
	for _, w := range []string{"page 1", "page 2", "purego", "pindex"} {
		if !strings.Contains(got, w) {
			t.Errorf("extract output missing %q\n---\n%s", w, buf.String())
		}
	}
}

func TestExtractCommandUnknownBackend(t *testing.T) {
	root := newRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"extract", "--backend", "bogus", "../../testdata/sample.pdf"})
	if err := root.Execute(); err == nil {
		t.Error("expected error for unknown backend")
	}
}
