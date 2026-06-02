package llm

import (
	"context"
	"errors"
	"testing"
	"time"
)

func noWait(p *ResilientProvider) { p.wait = func(context.Context, time.Duration) error { return nil } }

func TestRetrySucceedsAfterTransient(t *testing.T) {
	inner := FailThenSucceed(2, Retryable(errors.New("boom")), "ok")
	p := NewResilient(inner, RetryPolicy{MaxAttempts: 5, BaseDelay: time.Millisecond})
	noWait(p)
	resp, err := p.Complete(context.Background(), UserPrompt("m", "hi"))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.Content != "ok" {
		t.Errorf("content=%q want ok", resp.Content)
	}
	if inner.CallCount() != 3 {
		t.Errorf("calls=%d want 3", inner.CallCount())
	}
}

func TestPermanentErrorNotRetried(t *testing.T) {
	inner := NewMock("m", MockResponse{Err: errors.New("bad request")})
	inner.Default = MockResponse{Content: "ok"}
	p := NewResilient(inner, RetryPolicy{MaxAttempts: 5})
	noWait(p)
	if _, err := p.Complete(context.Background(), UserPrompt("m", "hi")); err == nil {
		t.Fatal("expected error")
	}
	if inner.CallCount() != 1 {
		t.Errorf("calls=%d want 1 (no retry on permanent)", inner.CallCount())
	}
}

func TestRetryExhausts(t *testing.T) {
	inner := NewMock("m")
	inner.Default = MockResponse{Err: Retryable(errors.New("always"))}
	p := NewResilient(inner, RetryPolicy{MaxAttempts: 3})
	noWait(p)
	if _, err := p.Complete(context.Background(), UserPrompt("m", "hi")); err == nil {
		t.Fatal("expected error")
	}
	if inner.CallCount() != 3 {
		t.Errorf("calls=%d want 3", inner.CallCount())
	}
}

func TestBreakerOpensAndFailsFast(t *testing.T) {
	inner := NewMock("m")
	inner.Default = MockResponse{Err: Retryable(errors.New("down"))}
	p := NewResilient(inner, RetryPolicy{MaxAttempts: 10}, WithBreaker(2, time.Minute))
	noWait(p)
	p.breaker.now = func() time.Time { return time.Unix(0, 0) } // frozen: never recovers
	_, err := p.Complete(context.Background(), UserPrompt("m", "hi"))
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("want ErrCircuitOpen, got %v", err)
	}
	if inner.CallCount() != 2 {
		t.Errorf("calls=%d want 2 (breaker trips after 2)", inner.CallCount())
	}
}

func TestBreakerRecoversAfterCooldown(t *testing.T) {
	inner := FailThenSucceed(2, Retryable(errors.New("down")), "ok")
	p := NewResilient(inner, RetryPolicy{MaxAttempts: 1}, WithBreaker(2, time.Minute))
	noWait(p)
	var clock time.Time
	p.breaker.now = func() time.Time { return clock }

	if _, err := p.Complete(context.Background(), UserPrompt("m", "x")); err == nil {
		t.Fatal("call 1 should fail")
	}
	if _, err := p.Complete(context.Background(), UserPrompt("m", "x")); err == nil {
		t.Fatal("call 2 should fail (and trip breaker)")
	}
	if _, err := p.Complete(context.Background(), UserPrompt("m", "x")); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("call 3 should be blocked, got %v", err)
	}
	clock = clock.Add(2 * time.Minute)
	resp, err := p.Complete(context.Background(), UserPrompt("m", "x"))
	if err != nil {
		t.Fatalf("after cooldown: %v", err)
	}
	if resp.Content != "ok" {
		t.Errorf("content=%q want ok", resp.Content)
	}
}

func TestBackoffDelayCapsAndGrows(t *testing.T) {
	base := 10 * time.Millisecond
	max := 80 * time.Millisecond
	got := []time.Duration{
		backoffDelay(1, base, max),
		backoffDelay(2, base, max),
		backoffDelay(4, base, max),
		backoffDelay(100, base, max),
	}
	want := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 80 * time.Millisecond, 80 * time.Millisecond}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("backoff[%d]=%v want %v", i, got[i], want[i])
		}
	}
}
