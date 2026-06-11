package llm

import (
	"context"
	"errors"
	"testing"
	"time"
)

// collect returns an Observer appending events to the returned slice pointer.
func collect() (*[]Event, Observer) {
	events := &[]Event{}
	return events, func(e Event) { *events = append(*events, e) }
}

func kinds(events []Event) []EventKind {
	out := make([]EventKind, len(events))
	for i, e := range events {
		out[i] = e.Kind
	}
	return out
}

func TestCachingProviderEmitsMissThenHit(t *testing.T) {
	events, obs := collect()
	cp := NewCaching(NewMock("m", MockResponse{Content: "hi", Finish: "stop"}), NewMemoryCache())
	cp.Observer = obs
	req := UserPrompt("model-x", "question")

	if _, err := cp.Complete(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if _, err := cp.Complete(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	got := kinds(*events)
	want := []EventKind{EventCacheMiss, EventCacheHit}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("events = %v, want %v", got, want)
	}
	if (*events)[0].Model != "model-x" {
		t.Errorf("event model = %q, want model-x", (*events)[0].Model)
	}
}

func TestResilientEmitsErrorRetryThenOK(t *testing.T) {
	events, obs := collect()
	transient := Retryable(errors.New("boom"))
	p := NewResilient(FailThenSucceed(1, transient, "ok"),
		RetryPolicy{MaxAttempts: 3}, WithObserver(obs))

	if _, err := p.Complete(context.Background(), UserPrompt("m", "q")); err != nil {
		t.Fatal(err)
	}
	got := kinds(*events)
	want := []EventKind{EventCallError, EventRetry, EventCallOK}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("events = %v, want %v", got, want)
	}
	if (*events)[1].Attempt != 1 {
		t.Errorf("retry attempt = %d, want 1", (*events)[1].Attempt)
	}
	if (*events)[2].Attempt != 2 {
		t.Errorf("success attempt = %d, want 2", (*events)[2].Attempt)
	}
}

func TestResilientEmitsBreakerOpen(t *testing.T) {
	events, obs := collect()
	dead := NewMock("dead")
	dead.Default = MockResponse{Err: errors.New("permanent")}
	p := NewResilient(dead, RetryPolicy{MaxAttempts: 1},
		WithBreaker(1, time.Hour), WithObserver(obs))

	if _, err := p.Complete(context.Background(), UserPrompt("m", "q")); err == nil {
		t.Fatal("want error from dead provider")
	}
	if _, err := p.Complete(context.Background(), UserPrompt("m", "q")); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("want ErrCircuitOpen, got %v", err)
	}
	got := kinds(*events)
	want := []EventKind{EventCallError, EventBreakerOpen}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("events = %v, want %v", got, want)
	}
}

func TestNilObserverIsSafe(t *testing.T) {
	p := NewResilient(NewMock("m", MockResponse{Content: "ok", Finish: "stop"}), RetryPolicy{MaxAttempts: 1})
	if _, err := p.Complete(context.Background(), UserPrompt("m", "q")); err != nil {
		t.Fatal(err)
	}
	cp := NewCaching(NewMock("m", MockResponse{Content: "ok", Finish: "stop"}), NewMemoryCache())
	if _, err := cp.Complete(context.Background(), UserPrompt("m", "q")); err != nil {
		t.Fatal(err)
	}
}
