package main

import (
	"io"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"github.com/jjfantini/pindex/internal/llm"
	"github.com/jjfantini/pindex/internal/ui"
)

// newUI builds the stderr presentation layer for a command run from the
// persistent --plain / --verbose flags, plus its diagnostic logger. Verbose
// mode disables the spinner (streaming log lines and in-place redraws would
// fight for the same line) and lowers the logger to debug level.
func newUI(c *cobra.Command) (*ui.UI, *log.Logger, bool) {
	verbose, _ := c.Flags().GetBool("verbose")
	u := newUIWriter(c, c.ErrOrStderr())
	return u, u.NewLogger(verbose), verbose
}

// newUIWriter builds a UI bound to an arbitrary writer (e.g. stdout for the
// eval funnel table) under the same --plain / --verbose flags.
func newUIWriter(c *cobra.Command, w io.Writer) *ui.UI {
	plain, _ := c.Flags().GetBool("plain")
	verbose, _ := c.Flags().GetBool("verbose")
	var opts []ui.Option
	if plain {
		opts = append(opts, ui.Plain())
	}
	if verbose {
		opts = append(opts, ui.NoAnimation())
	}
	return ui.New(w, opts...)
}

// llmObserver adapts engine events to log lines so an operator — human or
// agent — can watch the resilience envelope work: prompt-cache hits, per-call
// timings, retry backoff, and circuit-breaker trips. Cache and call events
// are debug (visible with --verbose); backpressure and degradation always show.
func llmObserver(l *log.Logger) llm.Observer {
	return func(e llm.Event) {
		switch e.Kind {
		case llm.EventCacheHit:
			l.Debug("prompt-cache hit — no API call", "model", e.Model)
		case llm.EventCacheMiss:
			l.Debug("prompt-cache miss — calling provider", "model", e.Model)
		case llm.EventCallOK:
			l.Debug("llm call ok", "provider", e.Provider, "model", e.Model, "took", e.Duration.Round(time.Millisecond), "attempt", e.Attempt)
		case llm.EventCallError:
			l.Debug("llm call failed", "provider", e.Provider, "model", e.Model, "took", e.Duration.Round(time.Millisecond), "attempt", e.Attempt, "err", e.Err)
		case llm.EventRetry:
			if llm.IsRateLimited(e.Err) {
				l.Warn("rate limited — backing off", "model", e.Model, "retry_in", e.Delay, "attempt", e.Attempt)
			} else {
				l.Warn("transient failure — retrying", "model", e.Model, "retry_in", e.Delay, "attempt", e.Attempt, "err", e.Err)
			}
		case llm.EventBreakerOpen:
			l.Error("circuit breaker open — provider degraded, failing fast", "provider", e.Provider)
		}
	}
}
