package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnthropicProviderParses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") == "" || r.Header.Get("anthropic-version") == "" {
			t.Error("missing anthropic headers")
		}
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"hello world"}],"stop_reason":"end_turn"}`))
	}))
	defer srv.Close()

	p := &AnthropicProvider{apiKey: "k", client: srv.Client(), BaseURL: srv.URL}
	resp, err := p.Complete(context.Background(), UserPrompt("claude-x", "hi"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "hello world" || resp.FinishReason != "stop" {
		t.Errorf("resp = %+v", resp)
	}
}

func TestOpenAIProviderParses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("authorization") == "" {
			t.Error("missing authorization header")
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hi there"},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()

	p := &OpenAIProvider{apiKey: "k", client: srv.Client(), BaseURL: srv.URL}
	resp, err := p.Complete(context.Background(), UserPrompt("gpt-x", "hi"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "hi there" {
		t.Errorf("resp = %+v", resp)
	}
}

func TestProviderErrorClassification(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		retry  bool
	}{
		{"5xx is transient", 500, `{"error":"x"}`, true},
		{"rate limit is transient", 429, `{"error":{"type":"rate_limit_error"}}`, true},
		{"quota exhaustion is permanent", 429, `{"error":{"type":"insufficient_quota"}}`, false},
		{"bad request is permanent", 400, `{"error":"bad"}`, false},
		{"auth error is permanent", 401, `{"error":"invalid x-api-key"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			p := &AnthropicProvider{apiKey: "k", client: srv.Client(), BaseURL: srv.URL}
			_, err := p.Complete(context.Background(), UserPrompt("claude-x", "hi"))
			if err == nil {
				t.Fatal("expected error")
			}
			if IsRetryable(err) != tc.retry {
				t.Errorf("retryable=%v want %v", IsRetryable(err), tc.retry)
			}
		})
	}
}

func TestNewHTTPProviderRouting(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "a")
	t.Setenv("OPENAI_API_KEY", "o")

	for model, want := range map[string]string{
		"claude-haiku-4-5":            "anthropic",
		"anthropic/claude-sonnet-4-6": "anthropic",
		"gpt-4o":                      "openai",
		"openai/gpt-4o-mini":          "openai",
	} {
		p, err := NewHTTPProvider(model)
		if err != nil {
			t.Fatalf("NewHTTPProvider(%q): %v", model, err)
		}
		if p.Name() != want {
			t.Errorf("NewHTTPProvider(%q).Name() = %s want %s", model, p.Name(), want)
		}
	}

	t.Setenv("ANTHROPIC_API_KEY", "")
	if _, err := NewHTTPProvider("claude-x"); err == nil {
		t.Error("missing ANTHROPIC_API_KEY should error")
	}
}
