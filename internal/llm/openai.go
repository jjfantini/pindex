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

// OpenAIProvider calls the OpenAI Chat Completions API directly (no SDK). The API
// key is held privately and never logged.
type OpenAIProvider struct {
	apiKey  string
	client  *http.Client
	BaseURL string // defaults to https://api.openai.com
}

// Name implements Provider.
func (p *OpenAIProvider) Name() string { return "openai" }

// Complete implements Provider against POST /v1/chat/completions.
func (p *OpenAIProvider) Complete(ctx context.Context, req Request) (Response, error) {
	base := p.BaseURL
	if base == "" {
		base = "https://api.openai.com"
	}
	msgs := make([]map[string]string, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = map[string]string{"role": string(m.Role), "content": m.Content}
	}
	payload := map[string]any{
		"model":       strings.TrimPrefix(req.Model, "openai/"),
		"messages":    msgs,
		"temperature": req.Temperature,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return Response{}, Retryable(fmt.Errorf("openai: %w", err))
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, Retryable(fmt.Errorf("openai: read body: %w", err))
	}
	if resp.StatusCode != http.StatusOK {
		return Response{}, classifyStatus(resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return Response{}, fmt.Errorf("openai: parse response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return Response{}, fmt.Errorf("openai: empty choices in response")
	}
	finish := "stop"
	if parsed.Choices[0].FinishReason == "length" {
		finish = "length"
	}
	return Response{Content: parsed.Choices[0].Message.Content, FinishReason: finish}, nil
}
