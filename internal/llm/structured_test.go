package llm

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type answer struct {
	Thinking string `json:"thinking"`
	Answer   string `json:"answer"`
}

func TestExtractJSONHandlesFences(t *testing.T) {
	cases := map[string]string{
		"```json\n{\"a\":1}\n```":             `{"a":1}`,
		"```\n{\"a\":1}\n```":                 `{"a":1}`,
		"{\"a\":1}":                           `{"a":1}`,
		"prefix ```json {\"a\":1} ``` suffix": `{"a":1}`,
	}
	for in, want := range cases {
		got, err := ExtractJSON(in)
		if err != nil {
			t.Errorf("ExtractJSON(%q): %v", in, err)
			continue
		}
		if strings.TrimSpace(got) != want {
			t.Errorf("ExtractJSON(%q) = %q want %q", in, strings.TrimSpace(got), want)
		}
	}
	if _, err := ExtractJSON("   "); err == nil {
		t.Error("empty content should error")
	}
}

func TestUnmarshalToleratesTrailingComma(t *testing.T) {
	var v map[string]int
	if err := Unmarshal("```json\n{\"a\":1,}\n```", &v); err != nil {
		t.Fatal(err)
	}
	if v["a"] != 1 {
		t.Errorf("v=%v want a:1", v)
	}
	if err := Unmarshal("not json at all", &v); err == nil {
		t.Error("expected error on non-JSON")
	}
}

func TestCompleteJSONRetriesUntilValid(t *testing.T) {
	inner := NewMock("m",
		MockResponse{Content: `{"answer":"no"}`},                       // fails validation
		MockResponse{Content: `{"thinking":"because","answer":"yes"}`}, // passes
	)
	validate := func(a answer) error {
		if a.Answer != "yes" {
			return errors.New("want yes")
		}
		return nil
	}
	got, err := CompleteJSON(context.Background(), inner, UserPrompt("m", "q"), 3, validate)
	if err != nil {
		t.Fatal(err)
	}
	if got.Answer != "yes" || got.Thinking != "because" {
		t.Errorf("got %+v (thinking field must be preserved)", got)
	}
	if inner.CallCount() != 2 {
		t.Errorf("calls=%d want 2", inner.CallCount())
	}
	calls := inner.Calls()
	if len(calls[1].Messages) <= len(calls[0].Messages) {
		t.Error("retry should append correction messages to re-prompt")
	}
}

func TestCompleteJSONFailsAfterAttempts(t *testing.T) {
	inner := NewMock("m")
	inner.Default = MockResponse{Content: "garbage not json"}
	_, err := CompleteJSON[answer](context.Background(), inner, UserPrompt("m", "q"), 2, nil)
	if err == nil {
		t.Error("expected failure after exhausting attempts")
	}
	if inner.CallCount() != 2 {
		t.Errorf("calls=%d want 2", inner.CallCount())
	}
}
