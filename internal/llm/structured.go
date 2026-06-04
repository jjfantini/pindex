package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var trailingCommaRe = regexp.MustCompile(`,\s*([}\]])`)

// ExtractJSON pulls the JSON payload out of an LLM response, tolerating ```json
// (or bare ```) fences. Unlike PageIndex's extract_json it returns an error
// instead of silently yielding {}.
func ExtractJSON(content string) (string, error) {
	s := content
	if i := strings.Index(s, "```json"); i != -1 {
		s = s[i+len("```json"):]
		if j := strings.LastIndex(s, "```"); j != -1 {
			s = s[:j]
		}
	} else if i := strings.Index(s, "```"); i != -1 {
		s = s[i+3:]
		if j := strings.LastIndex(s, "```"); j != -1 {
			s = s[:j]
		}
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("llm: no JSON found in response")
	}
	return s, nil
}

// Unmarshal extracts JSON from content into v, retrying once after stripping
// trailing commas. Returns a typed error on failure (never silently empty).
func Unmarshal(content string, v any) error {
	s, err := ExtractJSON(content)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(s), v); err == nil {
		return nil
	}
	cleaned := trailingCommaRe.ReplaceAllString(s, "$1")
	if err := json.Unmarshal([]byte(cleaned), v); err != nil {
		return fmt.Errorf("llm: invalid JSON: %w", err)
	}
	return nil
}

// CompleteJSON calls the provider, parses the reply into a fresh T, runs validate,
// and on parse/validation failure re-prompts (appending the bad reply plus a
// correction message) up to attempts times. This is validate-then-retry, not
// constrained decoding, so chain-of-thought ("thinking") fields are preserved.
// Provider/transport errors are returned immediately (they are retried, if at
// all, inside the Provider).
func CompleteJSON[T any](ctx context.Context, p Provider, req Request, attempts int, validate func(T) error) (T, error) {
	var zero T
	if attempts < 1 {
		attempts = 1
	}
	msgs := append([]Message(nil), req.Messages...)
	var lastErr error
	for i := 0; i < attempts; i++ {
		attemptReq := req
		attemptReq.Messages = msgs
		resp, err := p.Complete(ctx, attemptReq)
		if err != nil {
			return zero, err
		}
		var out T
		if perr := Unmarshal(resp.Content, &out); perr != nil {
			lastErr = perr
		} else if validate != nil {
			if verr := validate(out); verr != nil {
				lastErr = fmt.Errorf("llm: validation failed: %w", verr)
			} else {
				return out, nil
			}
		} else {
			return out, nil
		}
		// Re-prompt with the error so a temperature-0 model can correct itself.
		msgs = append(msgs,
			Message{Role: RoleAssistant, Content: resp.Content},
			Message{Role: RoleUser, Content: "Your previous reply was not valid for the required JSON schema: " +
				lastErr.Error() + ". Reply again with ONLY the corrected JSON, no prose."},
		)
	}
	return zero, fmt.Errorf("llm: structured output failed after %d attempt(s): %w", attempts, lastErr)
}
