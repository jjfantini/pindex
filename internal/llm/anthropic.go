package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// AnthropicProvider calls the Anthropic Messages API directly (no SDK). The API
// key is held privately and never logged.
type AnthropicProvider struct {
	apiKey    string
	client    *http.Client
	BaseURL   string // defaults to https://api.anthropic.com
	MaxTokens int    // defaults to 4096
}

// Name implements Provider.
func (p *AnthropicProvider) Name() string { return "anthropic" }

// Complete implements Provider against POST /v1/messages.
func (p *AnthropicProvider) Complete(ctx context.Context, req Request) (Response, error) {
	base := p.BaseURL
	if base == "" {
		base = "https://api.anthropic.com"
	}
	maxTok := p.MaxTokens
	if maxTok <= 0 {
		maxTok = 4096
	}

	// Anthropic carries system instructions in a top-level field, not in messages.
	var systemParts []string
	cacheSystem := false
	msgs := make([]map[string]string, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == RoleSystem {
			systemParts = append(systemParts, m.Content)
			cacheSystem = cacheSystem || m.Cache
			continue
		}
		msgs = append(msgs, map[string]string{"role": string(m.Role), "content": m.Content})
	}

	payload := map[string]any{
		"model":       strings.TrimPrefix(req.Model, "anthropic/"),
		"max_tokens":  maxTok,
		"temperature": req.Temperature,
		"messages":    msgs,
	}
	if len(systemParts) > 0 {
		systemText := strings.Join(systemParts, "\n")
		if cacheSystem {
			// Array form is required to attach cache_control. Marking the system
			// block ephemeral caches the tools+system prefix; Anthropic silently
			// skips it when the prefix is below the model's minimum (~4096 tokens
			// for Opus, ~2048 for Sonnet 4.6) — no error, just no cache write.
			// Prompt caching is GA, so no anthropic-beta header is needed.
			payload["system"] = []map[string]any{{
				"type":          "text",
				"text":          systemText,
				"cache_control": map[string]string{"type": "ephemeral"},
			}}
		} else {
			payload["system"] = systemText
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return Response{}, Retryable(fmt.Errorf("anthropic: %w", err)) // network/timeout: transient
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, Retryable(fmt.Errorf("anthropic: read body: %w", err))
	}
	if resp.StatusCode != http.StatusOK {
		return Response{}, classifyStatus(resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return Response{}, fmt.Errorf("anthropic: parse response: %w", err)
	}
	var sb strings.Builder
	for _, c := range parsed.Content {
		if c.Type == "text" {
			sb.WriteString(c.Text)
		}
	}
	finish := "stop"
	if parsed.StopReason == "max_tokens" {
		finish = "length"
	}
	return Response{Content: sb.String(), FinishReason: finish}, nil
}
