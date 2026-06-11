package llm

import (
	"context"
	"sync"
	"time"
)

// Limiter gates request rate. *golang.org/x/time/rate.Limiter satisfies it, so
// callers inject one without this package depending on x/time/rate.
type Limiter interface {
	Wait(ctx context.Context) error
}

type breakerState int

const (
	stClosed breakerState = iota
	stOpen
	stHalfOpen
)

// circuitBreaker is a minimal failure breaker with an injectable clock (so the
// open -> half-open recovery is deterministically testable).
type circuitBreaker struct {
	mu          sync.Mutex
	maxFailures int
	cooldown    time.Duration
	now         func() time.Time
	state       breakerState
	failures    int
	openedAt    time.Time
}

func (b *circuitBreaker) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == stOpen {
		if b.now().Sub(b.openedAt) >= b.cooldown {
			b.state = stHalfOpen
			return true
		}
		return false
	}
	return true
}

func (b *circuitBreaker) onSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.state = stClosed
}

func (b *circuitBreaker) onFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	if b.state == stHalfOpen || b.failures >= b.maxFailures {
		b.state = stOpen
		b.openedAt = b.now()
	}
}

// RetryPolicy bounds retries with exponential backoff.
type RetryPolicy struct {
	MaxAttempts int // total attempts, >= 1
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

// ResilientProvider wraps a Provider with rate limiting, a circuit breaker, and
// bounded retry-with-backoff on transient (RetryableError) failures.
type ResilientProvider struct {
	inner   Provider
	policy  RetryPolicy
	breaker *circuitBreaker
	limiter Limiter
	obs     Observer
	wait    func(ctx context.Context, d time.Duration) error
}

// Option configures a ResilientProvider.
type Option func(*ResilientProvider)

// WithLimiter sets the rate limiter.
func WithLimiter(l Limiter) Option { return func(p *ResilientProvider) { p.limiter = l } }

// WithObserver streams resilience events (attempt timings, retries, breaker
// trips) to obs for diagnostics. A nil obs is a no-op.
func WithObserver(obs Observer) Option { return func(p *ResilientProvider) { p.obs = obs } }

// WithBreaker enables a circuit breaker that opens after maxFailures and recovers
// after cooldown.
func WithBreaker(maxFailures int, cooldown time.Duration) Option {
	return func(p *ResilientProvider) {
		p.breaker = &circuitBreaker{maxFailures: maxFailures, cooldown: cooldown, now: time.Now}
	}
}

// NewResilient wraps inner with the given retry policy and options.
func NewResilient(inner Provider, policy RetryPolicy, opts ...Option) *ResilientProvider {
	if policy.MaxAttempts < 1 {
		policy.MaxAttempts = 1
	}
	p := &ResilientProvider{inner: inner, policy: policy, wait: sleepWait}
	for _, o := range opts {
		o(p)
	}
	return p
}

func sleepWait(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Name implements Provider.
func (p *ResilientProvider) Name() string { return p.inner.Name() }

// Complete implements Provider with rate limiting, breaker, and retry.
func (p *ResilientProvider) Complete(ctx context.Context, req Request) (Response, error) {
	if p.limiter != nil {
		if err := p.limiter.Wait(ctx); err != nil {
			return Response{}, err
		}
	}
	var lastErr error
	for attempt := 1; attempt <= p.policy.MaxAttempts; attempt++ {
		if p.breaker != nil && !p.breaker.allow() {
			p.obs.emit(Event{Kind: EventBreakerOpen, Provider: p.inner.Name(), Model: req.Model, Attempt: attempt})
			return Response{}, ErrCircuitOpen
		}
		start := time.Now()
		resp, err := p.inner.Complete(ctx, req)
		if err == nil {
			if p.breaker != nil {
				p.breaker.onSuccess()
			}
			p.obs.emit(Event{Kind: EventCallOK, Provider: p.inner.Name(), Model: req.Model, Attempt: attempt, Duration: time.Since(start)})
			return resp, nil
		}
		p.obs.emit(Event{Kind: EventCallError, Provider: p.inner.Name(), Model: req.Model, Attempt: attempt, Duration: time.Since(start), Err: err})
		// Rate-limit 429s are backpressure, not provider death — retry them but
		// do NOT count them toward opening the breaker (else a busy provider
		// cascades into ErrCircuitOpen across a whole batch).
		if p.breaker != nil && !IsRateLimited(err) {
			p.breaker.onFailure()
		}
		lastErr = err
		if !IsRetryable(err) || attempt == p.policy.MaxAttempts {
			return Response{}, err
		}
		delay := backoffDelay(attempt, p.policy.BaseDelay, p.policy.MaxDelay)
		p.obs.emit(Event{Kind: EventRetry, Provider: p.inner.Name(), Model: req.Model, Attempt: attempt, Delay: delay, Err: err})
		if werr := p.wait(ctx, delay); werr != nil {
			return Response{}, werr
		}
	}
	return Response{}, lastErr
}

// backoffDelay is base * 2^(attempt-1), capped at max, overflow-safe.
func backoffDelay(attempt int, base, max time.Duration) time.Duration {
	if base <= 0 {
		return 0
	}
	d := base
	for i := 1; i < attempt; i++ {
		d *= 2
		if d <= 0 { // overflow
			if max > 0 {
				return max
			}
			return base
		}
		if max > 0 && d >= max {
			return max
		}
	}
	if max > 0 && d > max {
		return max
	}
	return d
}
