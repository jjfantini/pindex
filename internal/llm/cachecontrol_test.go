package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// providerPayload runs one Complete against a stub server and returns the decoded
// JSON body the adapter sent — used to assert the on-the-wire prompt-cache shape.
func providerPayload(t *testing.T, p Provider, req Request) map[string]any {
	t.Helper()
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &got); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		// A reply both adapters can parse.
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn",` +
			`"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()
	switch v := p.(type) {
	case *AnthropicProvider:
		v.client, v.BaseURL = srv.Client(), srv.URL
	case *OpenAIProvider:
		v.client, v.BaseURL = srv.Client(), srv.URL
	}
	if _, err := p.Complete(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestAnthropicSystemCacheControl(t *testing.T) {
	got := providerPayload(t, &AnthropicProvider{apiKey: "k"}, SystemUser("claude-x", "INSTRUCTIONS", "DATA"))

	// system must be the array-of-blocks form carrying an ephemeral cache breakpoint.
	sysArr, ok := got["system"].([]any)
	if !ok || len(sysArr) != 1 {
		t.Fatalf("system should be a 1-element array, got %T: %v", got["system"], got["system"])
	}
	block, _ := sysArr[0].(map[string]any)
	if block["type"] != "text" || block["text"] != "INSTRUCTIONS" {
		t.Errorf("system block = %v", block)
	}
	cc, ok := block["cache_control"].(map[string]any)
	if !ok || cc["type"] != "ephemeral" {
		t.Errorf("cache_control = %v, want {type: ephemeral}", block["cache_control"])
	}

	// the user turn carries the data and is NOT inside the cached system block.
	msgs, _ := got["messages"].([]any)
	if len(msgs) != 1 {
		t.Fatalf("want 1 user message, got %v", got["messages"])
	}
	if m, _ := msgs[0].(map[string]any); m["role"] != "user" || m["content"] != "DATA" {
		t.Errorf("user message = %v", msgs[0])
	}
}

func TestAnthropicSystemPlainWhenNotCached(t *testing.T) {
	// A system message without the cache flag renders as a plain string (no array,
	// no cache_control) — the pre-caching wire shape, preserved for compatibility.
	req := Request{Model: "claude-x", Messages: []Message{
		{Role: RoleSystem, Content: "SYS"},
		{Role: RoleUser, Content: "U"},
	}}
	got := providerPayload(t, &AnthropicProvider{apiKey: "k"}, req)
	if s, ok := got["system"].(string); !ok || s != "SYS" {
		t.Errorf("system = %T %v, want plain string \"SYS\"", got["system"], got["system"])
	}
}

func TestAnthropicNoSystemKeyWhenAbsent(t *testing.T) {
	got := providerPayload(t, &AnthropicProvider{apiKey: "k"}, UserPrompt("claude-x", "hi"))
	if _, present := got["system"]; present {
		t.Errorf("system key must be absent when there is no system message, got %v", got["system"])
	}
}

// TestOpenAISystemUserSplit confirms the split reaches OpenAI as two role-tagged
// messages (OpenAI caches prefixes automatically, so there is no cache_control).
func TestOpenAISystemUserSplit(t *testing.T) {
	got := providerPayload(t, &OpenAIProvider{apiKey: "k"}, SystemUser("gpt-x", "SYS", "USR"))
	msgs, ok := got["messages"].([]any)
	if !ok || len(msgs) != 2 {
		t.Fatalf("want 2 messages, got %v", got["messages"])
	}
	first, _ := msgs[0].(map[string]any)
	second, _ := msgs[1].(map[string]any)
	if first["role"] != "system" || first["content"] != "SYS" {
		t.Errorf("first message = %v, want system/SYS", first)
	}
	if second["role"] != "user" || second["content"] != "USR" {
		t.Errorf("second message = %v, want user/USR", second)
	}
}
