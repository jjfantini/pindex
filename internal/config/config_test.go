package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultMirrorsUpstream(t *testing.T) {
	d := Default()
	if d.Model == "" {
		t.Fatal("default model is empty")
	}
	if d.Extractor != "mupdf" {
		t.Errorf("extractor = %q, want mupdf", d.Extractor)
	}
	if d.TOCCheckPageNum != 10 {
		t.Errorf("toc_check_page_num = %d, want 10", d.TOCCheckPageNum)
	}
	if d.MaxPageNumEachNode != 10 || d.MaxTokenNumEachNode != 20000 {
		t.Errorf("node limits = (%d,%d), want (10,20000)", d.MaxPageNumEachNode, d.MaxTokenNumEachNode)
	}
	if !d.AddNodeID || !d.AddNodeSummary {
		t.Error("node id/summary should default to true")
	}
	if d.AddDocDescription || d.AddNodeText {
		t.Error("doc description / node text should default to false (verified upstream default)")
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("default config invalid: %v", err)
	}
}

func TestLoadOverlaysDefaults(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.yaml")
	if err := os.WriteFile(p, []byte("model: claude-x\nmax_page_num_each_node: 5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model != "claude-x" {
		t.Errorf("model = %q, want claude-x", cfg.Model)
	}
	if cfg.MaxPageNumEachNode != 5 {
		t.Errorf("max_page_num_each_node = %d, want 5", cfg.MaxPageNumEachNode)
	}
	if cfg.TOCCheckPageNum != 10 {
		t.Errorf("unspecified key should keep default 10, got %d", cfg.TOCCheckPageNum)
	}
}

func TestLoadEmptyOrMissingReturnsDefaults(t *testing.T) {
	for _, p := range []string{"", filepath.Join(t.TempDir(), "nope.yaml")} {
		cfg, err := Load(p)
		if err != nil {
			t.Fatalf("Load(%q): %v", p, err)
		}
		if cfg.Model != Default().Model {
			t.Errorf("Load(%q) should yield defaults", p)
		}
	}
}

func TestLoadRejectsInvalid(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(p, []byte("extractor: bogus\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Error("expected Load to reject an unknown extractor")
	}
}

func TestRetrieveModelFallback(t *testing.T) {
	c := Default()
	if got := c.RetrieveModelOrDefault(); got != c.Model {
		t.Errorf("fallback = %q, want %q", got, c.Model)
	}
	c.RetrieveModel = "retrieve-x"
	if got := c.RetrieveModelOrDefault(); got != "retrieve-x" {
		t.Errorf("override = %q, want retrieve-x", got)
	}
}

func TestValidateCatchesBadValues(t *testing.T) {
	cases := map[string]func(*Config){
		"empty model":   func(c *Config) { c.Model = "" },
		"bad extractor": func(c *Config) { c.Extractor = "nope" },
		"zero maxpage":  func(c *Config) { c.MaxPageNumEachNode = 0 },
		"neg toc":       func(c *Config) { c.TOCCheckPageNum = -1 },
	}
	for name, mutate := range cases {
		c := Default()
		mutate(&c)
		if err := c.Validate(); err == nil {
			t.Errorf("%s: expected validation error", name)
		}
	}
}
