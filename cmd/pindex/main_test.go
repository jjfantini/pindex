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

func TestStubsReturnNotImplemented(t *testing.T) {
	stubs := map[string]bool{"index": true, "ask": true, "eval": true}
	for _, cmd := range newRootCmd().Commands() {
		if !stubs[cmd.Name()] || cmd.RunE == nil {
			continue
		}
		if err := cmd.RunE(cmd, nil); err == nil {
			t.Errorf("%s: expected a not-implemented error from scaffold stub", cmd.Name())
		}
	}
}
