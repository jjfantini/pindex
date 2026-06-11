package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jjfantini/pindex/internal/llm"
	"github.com/jjfantini/pindex/internal/ui"
)

func TestRootHasUIFlags(t *testing.T) {
	root := newRootCmd()
	for _, name := range []string{"verbose", "plain", "config"} {
		if root.PersistentFlags().Lookup(name) == nil {
			t.Errorf("missing persistent flag --%s", name)
		}
	}
}

// TestExtractStdoutStaysPure locks the agent contract: machine-readable page
// dumps go to stdout untouched (exact headers, no ANSI); decorations go to
// stderr only.
func TestExtractStdoutStaysPure(t *testing.T) {
	root := newRootCmd()
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{"extract", "--backend", "purego", "../../testdata/sample.pdf"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "===== page 1 (purego) =====") {
		t.Errorf("stdout lost the page header contract:\n%s", out.String())
	}
	if strings.Contains(out.String(), "\x1b") {
		t.Errorf("stdout must never carry ANSI codes:\n%q", out.String())
	}
	if !strings.Contains(errBuf.String(), "extracted 2 pages") {
		t.Errorf("stderr should carry the extraction receipt:\n%s", errBuf.String())
	}
	if strings.Contains(errBuf.String(), "\x1b") {
		t.Errorf("stderr to a non-TTY must be ANSI-free:\n%q", errBuf.String())
	}
}

func TestLLMObserverMapsEventsToLogs(t *testing.T) {
	var buf bytes.Buffer
	u := ui.New(&buf, ui.Plain())
	obs := llmObserver(u.NewLogger(true))

	obs(llm.Event{Kind: llm.EventCacheHit, Model: "m"})
	obs(llm.Event{Kind: llm.EventCallOK, Provider: "openai", Model: "m", Attempt: 1, Duration: 1200 * time.Millisecond})
	obs(llm.Event{Kind: llm.EventRetry, Model: "m", Attempt: 2, Delay: time.Second, Err: llm.RateLimited(errors.New("429"))})
	obs(llm.Event{Kind: llm.EventBreakerOpen, Provider: "openai"})

	out := buf.String()
	for _, want := range []string{
		"prompt-cache hit",
		"llm call ok",
		"rate limited — backing off",
		"circuit breaker open",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("observer log missing %q:\n%s", want, out)
		}
	}
}

// TestVerboseSuppressesDebugByDefault: without --verbose the observer's debug
// stream stays quiet; warnings still surface.
func TestObserverDebugHiddenWithoutVerbose(t *testing.T) {
	var buf bytes.Buffer
	u := ui.New(&buf, ui.Plain())
	obs := llmObserver(u.NewLogger(false))
	obs(llm.Event{Kind: llm.EventCacheHit, Model: "m"})
	obs(llm.Event{Kind: llm.EventRetry, Model: "m", Attempt: 1, Delay: time.Second, Err: llm.Retryable(errors.New("boom"))})
	out := buf.String()
	if strings.Contains(out, "prompt-cache hit") {
		t.Errorf("debug events must be hidden without verbose:\n%s", out)
	}
	if !strings.Contains(out, "transient failure — retrying") {
		t.Errorf("warnings must always surface:\n%s", out)
	}
}
