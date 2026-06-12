package llm

import "testing"

func TestContextWindow(t *testing.T) {
	cases := []struct {
		model string
		want  int
	}{
		{"claude-haiku-4-5-20251001", 200_000},
		{"claude-sonnet-4-6", 1_000_000},
		{"claude-opus-4-8", 1_000_000},
		{"claude-fable-5", 1_000_000},
		{"claude-sonnet-4-5", 200_000}, // older claude falls back to 200k
		{"gpt-4o-2024-11-20", 128_000},
		{"gpt-4o-mini", 128_000},
		{"gpt-4.1", 1_000_000},
		{"o3-mini", 200_000},
		{"some-unknown-model", 128_000},
		{"", 128_000},
	}
	for _, c := range cases {
		if got := ContextWindow(c.model); got != c.want {
			t.Errorf("ContextWindow(%q) = %d, want %d", c.model, got, c.want)
		}
	}
}

func TestStructureBudget(t *testing.T) {
	// Floor: small-context and unknown models keep the shipped 300k budget.
	if got := StructureBudget("gpt-4o"); got != 300_000 {
		t.Errorf("gpt-4o budget = %d, want 300000 (floor)", got)
	}
	if got := StructureBudget("unknown"); got != 300_000 {
		t.Errorf("unknown budget = %d, want 300000 (floor)", got)
	}
	// Haiku's 200k window clears the floor: 200k/2*4 = 400k chars.
	if got := StructureBudget("claude-haiku-4-5-20251001"); got != 400_000 {
		t.Errorf("haiku budget = %d, want 400000", got)
	}
	// 1M-context models fit a 1MB tree untouched.
	if got := StructureBudget("claude-sonnet-4-6"); got != 2_000_000 {
		t.Errorf("sonnet 4.6 budget = %d, want 2000000", got)
	}
}
