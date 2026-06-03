package main

import "testing"

func TestRootHasSubcommands(t *testing.T) {
	root := newRootCmd()
	want := map[string]bool{"index": false, "ask": false, "eval": false, "extract": false}
	for _, c := range root.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("root is missing subcommand %q", name)
		}
	}
}
