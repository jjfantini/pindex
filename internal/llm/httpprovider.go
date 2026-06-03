package llm

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// DefaultHTTPTimeout bounds a single outbound LLM request (the Python original
// had no timeout). Exported so callers can tune it.
var DefaultHTTPTimeout = 120 * time.Second

func newHTTPClient() *http.Client { return &http.Client{Timeout: DefaultHTTPTimeout} }

// classifyStatus maps a non-2xx HTTP status to a transient (Retryable) error for
// 5xx and genuine rate limits, and a permanent error otherwise. A 429 caused by
// quota/billing exhaustion is permanent: retrying only burns the budget.
func classifyStatus(status int, body string) error {
	msg := fmt.Errorf("http %d: %s", status, truncate(strings.TrimSpace(body), 300))
	if status == http.StatusTooManyRequests {
		lb := strings.ToLower(body)
		if strings.Contains(lb, "insufficient_quota") || strings.Contains(lb, "billing") ||
			strings.Contains(lb, "exceeded your current quota") {
			return msg // permanent: a credits/billing problem won't fix itself
		}
		return RateLimited(msg) // backpressure: retry, but don't trip the breaker
	}
	if status >= 500 {
		return Retryable(msg)
	}
	return msg
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

// NewHTTPProvider returns a live Provider for the given model, reading the API key
// from the environment. It routes to Anthropic for "claude*"/"anthropic/*" models
// and to OpenAI otherwise. The key is never logged or returned.
func NewHTTPProvider(model string) (Provider, error) {
	if model == "" {
		return nil, fmt.Errorf("llm: empty model")
	}
	lower := strings.ToLower(model)
	switch {
	case strings.HasPrefix(lower, "anthropic/") || strings.Contains(lower, "claude"):
		key := os.Getenv("ANTHROPIC_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("llm: ANTHROPIC_API_KEY is not set (needed for model %q)", model)
		}
		return &AnthropicProvider{apiKey: key, client: newHTTPClient(), MaxTokens: 8192}, nil
	default:
		key := os.Getenv("OPENAI_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("llm: OPENAI_API_KEY is not set (needed for model %q)", model)
		}
		return &OpenAIProvider{apiKey: key, client: newHTTPClient()}, nil
	}
}
