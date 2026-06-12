// Package ask implements pindex's retrieval loop: reason over a document's
// structure to pick the tightest relevant page ranges, fetch them, and answer
// with page citations. This is the "ask" primitive PageIndex leaves to callers.
package ask

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
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
	// Verification is the ultra-effort fact-check verdict: "" (not run),
	// "supported" (every claim grounded in the cited pages), or "unsupported"
	// (some claim lacks support — surfaced, never silently replaced).
	Verification string
	// Steps is the number of agentic-loop turns used (0 for the fixed
	// low/medium pipeline).
	Steps int
}

// Effort dials the retrieval strategy. low is the fast default (single-pass
// select → fetch → answer); medium adds a fetch-more re-select on a refusal;
// high replaces the fixed pipeline with an agentic loop — the model navigates
// the tree itself, fetching tight page ranges until it can answer; ultra runs
// the same agentic loop and then fact-checks the final answer against its cited
// pages, with one corrective continuation on an unsupported verdict.
type Effort string

const (
	EffortLow    Effort = "low"
	EffortMedium Effort = "medium"
	EffortHigh   Effort = "high"
	EffortUltra  Effort = "ultra"
)

// agentMaxIterations caps the agentic loop before an answer is forced;
// agentCorrectiveIterations caps the ultra corrective continuation.
const (
	agentMaxIterations        = 8
	agentCorrectiveIterations = 3
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

// Asker answers questions over a single document.
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

// Ask answers question over doc. At low/medium effort it runs the fixed
// pipeline: select page ranges from the structure, fetch them, then answer
// strictly from that content with citations (medium retries once with a broader
// selection on a refusal). At high/ultra effort the fixed pipeline is replaced
// by an agentic loop (see askAgentic); ultra additionally fact-checks the final
// answer against its cited pages.
func (a *Asker) Ask(ctx context.Context, doc tree.Document, question string) (Answer, error) {
	if a.Effort.atLeast(EffortHigh) {
		return a.askAgentic(ctx, doc, question)
	}

	structure, _, err := retrieve.GetStructureWithin(doc, llm.StructureBudget(a.model))
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

	// medium effort: if the answer is an honest refusal, fetch a DIFFERENT set of
	// pages and try once more (recovers a wrong/too-narrow first selection).
	if a.Effort.atLeast(EffortMedium) && isRefusal(ans.Text) {
		mp := prompts.AskSelectMore(structure, question, sel.Pages)
		more, merr := llm.CompleteJSON[prompts.PageSelection](ctx, a.provider,
			llm.SystemUser(a.model, mp.System, mp.User), a.Attempts, validPages)
		if merr == nil {
			combined := sel.Pages + "," + more.Pages
			if retry, rerr := a.answerFrom(ctx, doc, combined, question); rerr == nil && !isRefusal(retry.Text) {
				ans = retry
			}
		}
	}
	return ans, nil
}

// askAgentic answers question via the high/ultra agentic loop: the model is
// given the document's metadata and text-stripped structure and navigates it
// with JSON actions — get_pages to fetch tight ranges, answer to finish — for up
// to agentMaxIterations turns (then an answer is forced; if even that fails a
// typed error is returned, never an empty answer). At ultra the final non-refusal
// answer is fact-checked against its cited pages: a "supported" verdict is
// returned as-is; on "unsupported" the SAME conversation continues with the
// fact-checker's findings for up to agentCorrectiveIterations more turns, the
// corrected answer is re-verified, and if still unsupported the ORIGINAL answer
// comes back with Verification="unsupported" (surfaced, never silent). A
// transport error in either verification call fails the whole call.
func (a *Asker) askAgentic(ctx context.Context, doc tree.Document, question string) (Answer, error) {
	structure, _, err := retrieve.GetStructureWithin(doc, llm.StructureBudget(a.model))
	if err != nil {
		return Answer{}, err
	}
	meta, err := retrieve.GetDocument(doc)
	if err != nil {
		return Answer{}, err
	}

	p := prompts.AskAgent(question, structure, meta)
	s := &agentSession{
		asker: a,
		doc:   doc,
		msgs: []llm.Message{
			{Role: llm.RoleSystem, Content: p.System, Cache: true},
			{Role: llm.RoleUser, Content: p.User},
		},
		fetched: make(map[int]bool),
	}

	ans, err := s.run(ctx, agentMaxIterations)
	if err != nil {
		return Answer{}, err
	}

	// ultra effort: fact-check a non-refusal answer against its cited pages
	// (refusals are skipped — an honest abstention is not a hallucination).
	if a.Effort != EffortUltra || isRefusal(ans.Text) {
		return ans, nil
	}
	v, err := a.verify(ctx, doc, question, ans)
	if err != nil {
		return Answer{}, err
	}
	if v.Verdict == "supported" {
		ans.Verification = "supported"
		return ans, nil
	}

	// One corrective continuation on the SAME conversation: the agent re-examines
	// the document with the fact-checker's findings, then the new answer is
	// re-verified. A failed continuation falls through to the original answer —
	// marked unsupported, never silently replaced by an unverified rewrite.
	s.msgs = append(s.msgs, llm.Message{Role: llm.RoleUser, Content: fmt.Sprintf(
		"A fact-checker reviewed your answer and found unsupported claims: %s. "+
			"Re-examine the document (you may fetch more pages) and produce a corrected, fully supported answer.",
		v.Missing)})
	if retry, rerr := s.run(ctx, agentCorrectiveIterations); rerr == nil && !isRefusal(retry.Text) {
		rv, verr := a.verify(ctx, doc, question, retry)
		if verr != nil {
			return Answer{}, verr
		}
		if rv.Verdict == "supported" {
			retry.Verification = "supported"
			return retry, nil
		}
	}
	ans.Verification = "unsupported"
	return ans, nil
}

// agentSession is the running state of one agentic conversation: the message
// transcript, the union of pages fetched so far, and the turn count. It persists
// across the ultra corrective continuation so the agent keeps its context.
type agentSession struct {
	asker   *Asker
	doc     tree.Document
	msgs    []llm.Message
	fetched map[int]bool
	steps   int
}

// validAgentAction accepts a well-formed get_pages or answer action.
func validAgentAction(act prompts.AgentAction) error {
	switch act.Action {
	case "get_pages":
		if strings.TrimSpace(act.Pages) == "" {
			return fmt.Errorf("get_pages requires non-empty pages")
		}
		if _, err := tree.ParsePages(act.Pages); err != nil {
			return fmt.Errorf("invalid page selector %q: %w", act.Pages, err)
		}
		return nil
	case "answer":
		if strings.TrimSpace(act.Answer) == "" {
			return fmt.Errorf("answer action requires non-empty answer")
		}
		return nil
	default:
		return fmt.Errorf("unknown action %q (want get_pages or answer)", act.Action)
	}
}

// validAgentAnswer accepts ONLY a well-formed answer action (the forced final turn).
func validAgentAnswer(act prompts.AgentAction) error {
	if act.Action != "answer" {
		return fmt.Errorf("action %q not allowed: the iteration budget is exhausted, you must answer", act.Action)
	}
	if strings.TrimSpace(act.Answer) == "" {
		return fmt.Errorf("answer action requires non-empty answer")
	}
	return nil
}

// run drives the loop for up to budget turns. Each turn the agent either fetches
// pages (the page JSON is appended as the next user turn) or answers. If the
// budget runs out without an answer, one final turn is forced that accepts only
// an answer action; if that also fails, a typed error is returned.
func (s *agentSession) run(ctx context.Context, budget int) (Answer, error) {
	for i := 0; i < budget; i++ {
		act, err := s.step(ctx, validAgentAction)
		if err != nil {
			return Answer{}, fmt.Errorf("ask: agent: %w", err)
		}
		if act.Action == "answer" {
			// Grounding is enforced mechanically, not just by the prompt: an
			// answer before any get_pages is redirected back into the loop
			// (haiku-class models otherwise answer from the structure summaries
			// and hallucinate). The turn budget still bounds the loop.
			if len(s.fetched) == 0 {
				s.msgs = append(s.msgs, llm.Message{Role: llm.RoleUser, Content: "You answered without " +
					"reading any pages — never answer from the structure summaries alone. Fetch the relevant " +
					"pages with get_pages first, then answer again grounded in their text."})
				continue
			}
			return s.answer(act), nil
		}
		pagesJSON, err := retrieve.GetPageContent(s.doc, act.Pages)
		if err != nil {
			return Answer{}, fmt.Errorf("ask: agent: fetch pages %q: %w", act.Pages, err)
		}
		nums, err := tree.ParsePages(act.Pages) // validated in validAgentAction
		if err != nil {
			return Answer{}, fmt.Errorf("ask: agent: parse pages %q: %w", act.Pages, err)
		}
		for _, n := range nums {
			s.fetched[n] = true
		}
		s.msgs = append(s.msgs, llm.Message{Role: llm.RoleUser, Content: pagesJSON})
	}

	// Budget exhausted: force a final answer from what was read.
	s.msgs = append(s.msgs, llm.Message{Role: llm.RoleUser, Content: "Iteration budget exhausted. " +
		"You MUST answer now from what you have read (or state the document does not contain it)."})
	act, err := s.step(ctx, validAgentAnswer)
	if err != nil {
		return Answer{}, fmt.Errorf("ask: agent: no answer after %d iterations: %w", budget, err)
	}
	return s.answer(act), nil
}

// step runs one agent turn: complete over the running transcript, validate the
// action, and append the canonical assistant JSON to the transcript.
func (s *agentSession) step(ctx context.Context, validate func(prompts.AgentAction) error) (prompts.AgentAction, error) {
	act, err := llm.CompleteJSON[prompts.AgentAction](ctx, s.asker.provider,
		llm.Request{Model: s.asker.model, Messages: s.msgs}, s.asker.Attempts, validate)
	if err != nil {
		return prompts.AgentAction{}, err
	}
	s.steps++
	raw, err := json.Marshal(act)
	if err != nil {
		return prompts.AgentAction{}, fmt.Errorf("marshal action: %w", err)
	}
	s.msgs = append(s.msgs, llm.Message{Role: llm.RoleAssistant, Content: string(raw)})
	return act, nil
}

// answer builds the Answer for a finishing action: cited pages parse best-effort
// from pages_used, SelectedPages is the union of everything fetched.
func (s *agentSession) answer(act prompts.AgentAction) Answer {
	cited, _ := tree.ParsePages(act.PagesUsed) // best-effort
	return Answer{
		Text:          act.Answer,
		CitedPages:    cited,
		SelectedPages: s.selected(),
		Reasoning:     act.Thinking,
		Steps:         s.steps,
	}
}

// selected renders the union of fetched pages as a selector string ("" if the
// agent answered without fetching anything).
func (s *agentSession) selected() string {
	pages := make([]int, 0, len(s.fetched))
	for p := range s.fetched {
		pages = append(pages, p)
	}
	sort.Ints(pages)
	return joinPages(pages)
}

// verify fact-checks ans against the content of its cited pages (falling back to
// the selected pages when no citations parsed; an answer that read no pages at
// all is checked against an empty page list) via the AskVerify prompt.
func (a *Asker) verify(ctx context.Context, doc tree.Document, question string, ans Answer) (prompts.Verification, error) {
	pages := ans.SelectedPages
	if len(ans.CitedPages) > 0 {
		pages = joinPages(ans.CitedPages)
	}
	pagesJSON := "[]"
	if pages != "" {
		var err error
		pagesJSON, err = retrieve.GetPageContent(doc, pages)
		if err != nil {
			return prompts.Verification{}, fmt.Errorf("ask: verify: fetch pages %q: %w", pages, err)
		}
	}
	vp := prompts.AskVerify(question, ans.Text, pagesJSON)
	v, err := llm.CompleteJSON[prompts.Verification](ctx, a.provider,
		llm.SystemUser(a.model, vp.System, vp.User), a.Attempts,
		func(v prompts.Verification) error {
			if v.Verdict != "supported" && v.Verdict != "unsupported" {
				return fmt.Errorf("verdict %q (want supported or unsupported)", v.Verdict)
			}
			return nil
		})
	if err != nil {
		return prompts.Verification{}, fmt.Errorf("ask: verify: %w", err)
	}
	return v, nil
}

// joinPages renders page numbers back into a page selector string ("2,7").
func joinPages(pages []int) string {
	parts := make([]string, len(pages))
	for i, p := range pages {
		parts[i] = strconv.Itoa(p)
	}
	return strings.Join(parts, ",")
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
