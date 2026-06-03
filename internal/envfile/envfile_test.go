package envfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadParsesAndOverrides(t *testing.T) {
	t.Setenv("PINDEX_TEST_KEY", "stale")
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	content := "# a comment\n\nexport PINDEX_TEST_KEY=\"fresh\"\nOTHER='value with space'\nmalformed_line\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Load(p); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("PINDEX_TEST_KEY"); got != "fresh" {
		t.Errorf("override failed: PINDEX_TEST_KEY = %q want fresh", got)
	}
	if got := os.Getenv("OTHER"); got != "value with space" {
		t.Errorf("OTHER = %q want 'value with space'", got)
	}
}

func TestLoadMissingFileIsNoOp(t *testing.T) {
	if err := Load(filepath.Join(t.TempDir(), "nope.env")); err != nil {
		t.Errorf("missing file should be a no-op, got %v", err)
	}
}
