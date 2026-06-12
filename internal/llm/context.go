package llm

import "strings"

// contextWindowDefault is the conservative assumption for models not in the
// table — small enough to be safe on every provider pindex routes to.
const contextWindowDefault = 128_000

// ContextWindow returns the model's context window in tokens, matched by
// model-name prefix (the same convention the provider router uses). Unknown
// models get a conservative 128k default. The table only needs entries where
// the right answer differs from the default — it is a sizing hint for prompt
// budgets, not a capability registry.
func ContextWindow(model string) int {
	m := strings.ToLower(strings.TrimSpace(model))
	for prefix, window := range contextWindows {
		if strings.HasPrefix(m, prefix) {
			return window
		}
	}
	if strings.HasPrefix(m, "claude") {
		return 200_000 // every served Claude model has at least 200k
	}
	return contextWindowDefault
}

// structureBudgetFloorChars is the minimum structure-prompt budget regardless
// of model (~75k tokens at ~4 chars/token) — ample for any normal filing while
// leaving room for instructions and the reply inside a 128k-token context.
const structureBudgetFloorChars = 300_000

// StructureBudget sizes the rendered-structure cap (in characters) for a
// prompt to the given model: half the model's context window at a conservative
// ~4 chars/token, never below the 300k-char floor. On a 1M-context model a
// 1MB tree fits untouched; the degradation ladder is only a safety floor
// there. Without a budget a 500-page tree's summaries overflow the window
// outright (PEPSICO_2022_10K's asks all died with "prompt is too long: 205330
// tokens > 200000 maximum").
func StructureBudget(model string) int {
	b := ContextWindow(model) / 2 * 4
	if b < structureBudgetFloorChars {
		return structureBudgetFloorChars
	}
	return b
}

// contextWindows maps model-name prefixes to context sizes (tokens).
var contextWindows = map[string]int{
	// 1M-context Claude models (Fable/Mythos 5, Opus 4.6+, Sonnet 4.6).
	"claude-fable":      1_000_000,
	"claude-mythos":     1_000_000,
	"claude-opus-4-6":   1_000_000,
	"claude-opus-4-7":   1_000_000,
	"claude-opus-4-8":   1_000_000,
	"claude-sonnet-4-6": 1_000_000,
	// OpenAI.
	"gpt-4.1": 1_000_000,
	"gpt-4o":  128_000,
	"o1":      200_000,
	"o3":      200_000,
}
