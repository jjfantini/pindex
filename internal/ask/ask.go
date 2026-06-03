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

// Asker answers questions over a single document via select-pages-then-answer.
type Asker struct {
	provider llm.Provider
	model    string
	Attempts int
}

// New returns an Asker bound to a provider and model.
func New(provider llm.Provider, model string) *Asker {
	return &Asker{provider: provider, model: model, Attempts: 3}
}

// Ask answers question over doc: select page ranges from the structure, fetch
// them, then answer strictly from that content with citations.
func (a *Asker) Ask(ctx context.Context, doc tree.Document, question string) (Answer, error) {
	structure, err := retrieve.GetStructure(doc)
	if err != nil {
		return Answer{}, err
	}

	sel, err := llm.CompleteJSON[prompts.PageSelection](ctx, a.provider,
		llm.UserPrompt(a.model, prompts.AskSelectPages(structure, question)),
		a.Attempts, func(s prompts.PageSelection) error {
			if strings.TrimSpace(s.Pages) == "" {
				return fmt.Errorf("no pages selected")
			}
			if _, perr := tree.ParsePages(s.Pages); perr != nil {
				return fmt.Errorf("invalid page selector %q: %w", s.Pages, perr)
			}
			return nil
		})
	if err != nil {
		return Answer{}, fmt.Errorf("ask: select pages: %w", err)
	}

	pagesJSON, err := retrieve.GetPageContent(doc, sel.Pages)
	if err != nil {
		return Answer{}, fmt.Errorf("ask: fetch pages %q: %w", sel.Pages, err)
	}

	out, err := llm.CompleteJSON[prompts.AnswerOut](ctx, a.provider,
		llm.UserPrompt(a.model, prompts.AskAnswer(question, pagesJSON)),
		a.Attempts, func(o prompts.AnswerOut) error {
			if strings.TrimSpace(o.Answer) == "" {
				return fmt.Errorf("empty answer")
			}
			return nil
		})
	if err != nil {
		return Answer{}, fmt.Errorf("ask: answer: %w", err)
	}

	cited, _ := tree.ParsePages(out.PagesUsed) // best-effort
	return Answer{
		Text:          out.Answer,
		CitedPages:    cited,
		SelectedPages: sel.Pages,
		Reasoning:     out.Thinking,
	}, nil
}
