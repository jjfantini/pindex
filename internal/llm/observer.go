package llm

import "time"

// EventKind classifies an Observer event.
type EventKind string

const (
	// EventCacheHit: the prompt-hash cache answered; no network call was made.
	EventCacheHit EventKind = "cache_hit"
	// EventCacheMiss: the request goes through to the live provider.
	EventCacheMiss EventKind = "cache_miss"
	// EventCallOK: one provider attempt succeeded (Duration is its wall time).
	EventCallOK EventKind = "call_ok"
	// EventCallError: one provider attempt failed (Err, Duration set).
	EventCallError EventKind = "call_error"
	// EventRetry: a transient failure will be retried after Delay.
	EventRetry EventKind = "retry"
	// EventBreakerOpen: the circuit breaker refused the call — provider degraded.
	EventBreakerOpen EventKind = "breaker_open"
)

// Event is one diagnostic event from the resilience/cache envelope. Rate-limit
// retries are distinguishable via IsRateLimited(Err).
type Event struct {
	Kind     EventKind
	Provider string
	Model    string
	Attempt  int           // 1-based attempt the event refers to
	Delay    time.Duration // EventRetry: backoff before the next attempt
	Duration time.Duration // EventCallOK/EventCallError: attempt wall time
	Err      error
}

// Observer receives engine events for logging/diagnostics. It must be safe
// for concurrent use and cheap — it is called on the request path. A nil
// Observer is a no-op everywhere one is accepted.
type Observer func(Event)

// emit calls o with e when o is non-nil.
func (o Observer) emit(e Event) {
	if o != nil {
		o(e)
	}
}
