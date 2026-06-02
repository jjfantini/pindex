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

// Message is one chat turn.
type Message struct {
	Role    Role
	Content string
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

// IsRetryable reports whether err, or anything it wraps, is a RetryableError.
func IsRetryable(err error) bool {
	var r *RetryableError
	return errors.As(err, &r)
}

// ErrCircuitOpen is returned when a provider's circuit breaker is open, so the
// caller can degrade instead of burning the retry budget on a dead provider.
var ErrCircuitOpen = errors.New("llm: circuit breaker open")
