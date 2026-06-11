package index

import (
	"context"
	"sync"
	"testing"

	"github.com/jjfantini/pindex/internal/config"
	"github.com/jjfantini/pindex/internal/llm"
)

// TestBuildReportsProgress locks in the Progress hook: every build stage on the
// no-TOC path announces itself, so the CLI can stream "what is happening now".
func TestBuildReportsProgress(t *testing.T) {
	cfg := config.Default() // AddNodeSummary=true (short pages summarize without LLM calls)
	mock := llm.NewMock("m",
		llm.MockResponse{Content: initTwoSections},         // generate_toc_init
		llm.MockResponse{Content: `{"start_begin":"yes"}`}, // appear-start: Introduction
		llm.MockResponse{Content: `{"start_begin":"yes"}`}, // appear-start: Methods
	)
	b := newTestBuilder(cfg, mock)

	var mu sync.Mutex
	seen := map[string][]string{}
	b.Progress = func(stage, msg string) {
		mu.Lock()
		defer mu.Unlock()
		seen[stage] = append(seen[stage], msg)
	}

	if _, err := b.Build(context.Background(), twoPages()); err != nil {
		t.Fatal(err)
	}
	for _, stage := range []string{"structure", "verify", "split", "enrich"} {
		if len(seen[stage]) == 0 {
			t.Errorf("no progress reported for stage %q (got %v)", stage, seen)
		}
	}
	if got := seen["structure"][0]; got != "generating structure · group 1/1" {
		t.Errorf("structure msg = %q", got)
	}
	if got := seen["verify"][0]; got != "verifying 2 section starts" {
		t.Errorf("verify msg = %q", got)
	}
}

// TestBuildNilProgressIsSafe: a nil hook (the default) must not panic anywhere.
func TestBuildNilProgressIsSafe(t *testing.T) {
	cfg := config.Default()
	mock := llm.NewMock("m",
		llm.MockResponse{Content: initTwoSections},
		llm.MockResponse{Content: `{"start_begin":"yes"}`},
		llm.MockResponse{Content: `{"start_begin":"yes"}`},
	)
	if _, err := newTestBuilder(cfg, mock).Build(context.Background(), twoPages()); err != nil {
		t.Fatal(err)
	}
}
