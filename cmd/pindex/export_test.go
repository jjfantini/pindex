package main

import (
	"path/filepath"
	"testing"
)

func TestExportDir(t *testing.T) {
	// a workspace gets a trees/ subdir
	if got := exportDir("/ws"); got != filepath.Join("/ws", "pindex") {
		t.Errorf("workspace: got %q want /ws/pindex", got)
	}
	// no workspace -> no export (print-only)
	if got := exportDir(""); got != "" {
		t.Errorf("no workspace: got %q want empty", got)
	}
}
