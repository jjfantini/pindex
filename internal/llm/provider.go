// Package llm defines the provider seam pindex's engine calls: a small Provider
// interface plus the resilience (retry + circuit breaker + rate limit), caching,
// structured-output, and token-counting wrappers around it. Concrete OpenAI /
// Anthropic adapters implement Provider; tests use MockProvider.
//
// Deliberate divergences from the Python original: failures surface as typed
// errors (never the silent "" / {} of llm_completion/extract_json), retries use
// bounded backoff, and a dead provider trips a breaker instead of draining the
// retry budget.
package llm

import (
	"context"
	"errors"
)

// Role is a chat message role.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message is one chat turn. Cache marks the message's content as a provider-side
// prompt-cache breakpoint: the Anthropic adapter renders a cacheable system block
// for it, while OpenAI (which caches prefixes automatically) ignores it. It is a
// transport hint, not part of the prompt identity, so it is excluded from CacheKey.
type Message struct {
	Role    Role
	Content string
	Cache   bool `json:"-"`
}

// Request is a completion request. pindex uses Temperature 0 for index
// determinism and cache hits.
type Request struct {
	Model       string
	Messages    []Message
	Temperature float64
}

// Response is a completion result. FinishReason is normalized ("stop", "length",
// "error", ...).
type Response struct {
	Content      string
	FinishReason string
}

// Provider is the minimal LLM surface the engine depends on.
type Provider interface {
	Complete(ctx context.Context, req Request) (Response, error)
	Name() string
}

// UserPrompt builds a single-user-message request at temperature 0.
func UserPrompt(model, prompt string) Request {
	return Request{
		Model:       model,
		Messages:    []Message{{Role: RoleUser, Content: prompt}},
		Temperature: 0,
	}
}

// SystemUser builds a two-message request at temperature 0: a stable system
// prompt followed by the per-request user content. The system block is flagged as
// a prompt-cache breakpoint, so providers that support caching reuse it across
// requests that share the same system text (see prompts.Prompt). Temperature 0
// keeps indexing deterministic and cache-friendly.
func SystemUser(model, system, user string) Request {
	return Request{
		Model: model,
		Messages: []Message{
			{Role: RoleSystem, Content: system, Cache: true},
			{Role: RoleUser, Content: user},
		},
		Temperature: 0,
	}
}

// RetryableError marks a provider failure as transient (timeout, rate limit, 5xx)
// and therefore worth retrying. Permanent failures (bad request, auth) are left
// unwrapped so the resilience layer fails fast.
type RetryableError struct{ Err error }

func (e *RetryableError) Error() string { return "retryable: " + e.Err.Error() }
func (e *RetryableError) Unwrap() error { return e.Err }

// Retryable wraps err as transient (nil stays nil).
func Retryable(err error) error {
	if err == nil {
		return nil
	}
	return &RetryableError{Err: err}
}

// RateLimitedError marks a 429 rate-limit: expected backpressure, not provider
// death. It is retryable, but the circuit breaker must NOT count it toward
// opening — otherwise a busy provider looks like a dead one and cascades.
type RateLimitedError struct{ Err error }

func (e *RateLimitedError) Error() string { return "rate limited: " + e.Err.Error() }
func (e *RateLimitedError) Unwrap() error { return e.Err }

// RateLimited wraps err as a rate-limit (nil stays nil).
func RateLimited(err error) error {
	if err == nil {
		return nil
	}
	return &RateLimitedError{Err: err}
}

// IsRateLimited reports whether err, or anything it wraps, is a RateLimitedError.
func IsRateLimited(err error) bool {
	var r *RateLimitedError
	return errors.As(err, &r)
}

// IsRetryable reports whether err is transient — a RetryableError or a
// RateLimitedError (both worth retrying).
func IsRetryable(err error) bool {
	var r *RetryableError
	if errors.As(err, &r) {
		return true
	}
	return IsRateLimited(err)
}

// ErrCircuitOpen is returned when a provider's circuit breaker is open, so the
// caller can degrade instead of burning the retry budget on a dead provider.
var ErrCircuitOpen = errors.New("llm: circuit breaker open")
