// Package ask implements pindex's retrieval loop: reason over a document's
// structure to pick the tightest relevant page ranges, fetch them, and answer
// with page citations. This is the "ask" primitive PageIndex leaves to callers.
package ask

import (
	"context"
	"fmt"
	"strings"

	"github.com/jjfantini/pindex/internal/llm"
	"github.com/jjfantini/pindex/internal/prompts"
	"github.com/jjfantini/pindex/internal/retrieve"
	"github.com/jjfantini/pindex/internal/tree"
)

// Answer is the result of Ask.
type Answer struct {
	Text          string
	CitedPages    []int
	SelectedPages string
	Reasoning     string
}

// Effort dials the retrieval strategy + reasoning budget. low is the fast default
// (single-pass); medium adds a fetch-more re-select on a refusal; high/ultra (the
// agentic loop) are reserved for a later phase and currently behave like medium.
type Effort string

const (
	EffortLow    Effort = "low"
	EffortMedium Effort = "medium"
	EffortHigh   Effort = "high"
	EffortUltra  Effort = "ultra"
)

var effortRank = map[Effort]int{EffortLow: 0, EffortMedium: 1, EffortHigh: 2, EffortUltra: 3}

func (e Effort) atLeast(o Effort) bool { return effortRank[e] >= effortRank[o] }

// ParseEffort validates an effort string (empty == low).
func ParseEffort(s string) (Effort, error) {
	switch e := Effort(strings.ToLower(strings.TrimSpace(s))); e {
	case "":
		return EffortLow, nil
	case EffortLow, EffortMedium, EffortHigh, EffortUltra:
		return e, nil
	default:
		return "", fmt.Errorf("ask: unknown effort %q (want low|medium|high|ultra)", s)
	}
}

// Asker answers questions over a single document via select-pages-then-answer.
type Asker struct {
	provider llm.Provider
	model    string
	Attempts int
	Effort   Effort
}

// New returns an Asker bound to a provider and model (effort low by default).
func New(provider llm.Provider, model string) *Asker {
	return &Asker{provider: provider, model: model, Attempts: 3, Effort: EffortLow}
}

func isRefusal(s string) bool {
	a := strings.ToLower(s)
	for _, p := range []string{
		"cannot find", "can't find", "could not find", "not provided", "not found",
		"unable to", "does not provide", "not present", "not available",
		"not stated", "not disclosed", "insufficient information",
	} {
		if strings.Contains(a, p) {
			return true
		}
	}
	return false
}

func validPages(s prompts.PageSelection) error {
	if strings.TrimSpace(s.Pages) == "" {
		return fmt.Errorf("no pages selected")
	}
	if _, err := tree.ParsePages(s.Pages); err != nil {
		return fmt.Errorf("invalid page selector %q: %w", s.Pages, err)
	}
	return nil
}

// Ask answers question over doc: select page ranges from the structure, fetch
// them, then answer strictly from that content with citations.
func (a *Asker) Ask(ctx context.Context, doc tree.Document, question string) (Answer, error) {
	structure, err := retrieve.GetStructure(doc)
	if err != nil {
		return Answer{}, err
	}

	sp := prompts.AskSelectPages(structure, question)
	sel, err := llm.CompleteJSON[prompts.PageSelection](ctx, a.provider,
		llm.SystemUser(a.model, sp.System, sp.User), a.Attempts, validPages)
	if err != nil {
		return Answer{}, fmt.Errorf("ask: select pages: %w", err)
	}

	ans, err := a.answerFrom(ctx, doc, sel.Pages, question)
	if err != nil {
		return Answer{}, err
	}

	// medium+ effort: if the answer is an honest refusal, fetch a DIFFERENT set of
	// pages and try once more (recovers a wrong/too-narrow first selection).
	if a.Effort.atLeast(EffortMedium) && isRefusal(ans.Text) {
		mp := prompts.AskSelectMore(structure, question, sel.Pages)
		more, merr := llm.CompleteJSON[prompts.PageSelection](ctx, a.provider,
			llm.SystemUser(a.model, mp.System, mp.User), a.Attempts, validPages)
		if merr == nil {
			combined := sel.Pages + "," + more.Pages
			if retry, rerr := a.answerFrom(ctx, doc, combined, question); rerr == nil && !isRefusal(retry.Text) {
				return retry, nil
			}
		}
	}
	return ans, nil
}

// answerFrom fetches the given pages and produces an answer over them.
func (a *Asker) answerFrom(ctx context.Context, doc tree.Document, pages, question string) (Answer, error) {
	pagesJSON, err := retrieve.GetPageContent(doc, pages)
	if err != nil {
		return Answer{}, fmt.Errorf("ask: fetch pages %q: %w", pages, err)
	}
	ap := prompts.AskAnswer(question, pagesJSON)
	out, err := llm.CompleteJSON[prompts.AnswerOut](ctx, a.provider,
		llm.SystemUser(a.model, ap.System, ap.User), a.Attempts,
		func(o prompts.AnswerOut) error {
			if strings.TrimSpace(o.Answer) == "" {
				return fmt.Errorf("empty answer")
			}
			return nil
		})
	if err != nil {
		return Answer{}, fmt.Errorf("ask: answer: %w", err)
	}
	cited, _ := tree.ParsePages(out.PagesUsed) // best-effort
	return Answer{Text: out.Answer, CitedPages: cited, SelectedPages: pages, Reasoning: out.Thinking}, nil
}
