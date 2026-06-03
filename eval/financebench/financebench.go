// Package financebench is the evaluation harness for Patronus AI's FinanceBench.
// It loads the question set, runs each question through pindex's ask loop over a
// pre-indexed workspace, and scores two metrics: LLM-judge answer accuracy (the
// permissive rubric PageIndex's Mafin 2.5 used, for comparability) and retrieval
// recall@page (did the cited pages include the gold evidence page).
package financebench

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jjfantini/pindex/internal/ask"
	"github.com/jjfantini/pindex/internal/llm"
	"github.com/jjfantini/pindex/internal/prompts"
	"github.com/jjfantini/pindex/internal/tree"
)

// Question is one FinanceBench item (a subset of the JSONL fields).
type Question struct {
	ID       string     `json:"financebench_id"`
	Company  string     `json:"company"`
	DocName  string     `json:"doc_name"`
	Question string     `json:"question"`
	Answer   string     `json:"answer"`
	Evidence []Evidence `json:"evidence"`
}

// Evidence is one supporting passage with its printed page number.
type Evidence struct {
	Text string  `json:"evidence_text"`
	Page flexInt `json:"evidence_page_num"`
}

// flexInt tolerates a JSON number or numeric string (FinanceBench mixes both).
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	var n int
	if json.Unmarshal(b, &n) == nil {
		*f = flexInt(n)
		return nil
	}
	var s string
	if json.Unmarshal(b, &s) == nil {
		if v, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			*f = flexInt(v)
		}
	}
	return nil // tolerate anything else (leave zero)
}

// LoadQuestions reads a FinanceBench JSONL file (one question per line).
func LoadQuestions(path string) ([]Question, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var out []Question
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 8*1024*1024) // long lines
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var q Question
		if err := json.Unmarshal([]byte(line), &q); err != nil {
			return nil, fmt.Errorf("financebench: parse line: %w", err)
		}
		out = append(out, q)
	}
	return out, sc.Err()
}

// GoldPages returns the distinct evidence page numbers for a question.
func GoldPages(q Question) []int {
	seen := map[int]bool{}
	var out []int
	for _, e := range q.Evidence {
		p := int(e.Page)
		if p > 0 && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// RecallAtPage reports whether any gold page was among the retrieved pages.
// (Caveat: pindex's physical page index may differ from FinanceBench's printed
// page label; align before trusting this on a full run — see docs/PLAN.md.)
func RecallAtPage(gold, retrieved []int) bool {
	if len(gold) == 0 {
		return false
	}
	set := make(map[int]bool, len(retrieved))
	for _, p := range retrieved {
		set[p] = true
	}
	for _, g := range gold {
		if set[g] {
			return true
		}
	}
	return false
}

// EvidenceHit reports whether the cited pages' text actually contains the gold
// evidence (word-overlap >= 60% for any evidence passage). This is alignment-free
// recall — unlike RecallAtPage it does not depend on the physical page index
// matching FinanceBench's printed page label, so a correct retrieval is not
// penalised by page-numbering differences.
func EvidenceHit(doc tree.Document, citedPages []int, q Question) bool {
	if len(q.Evidence) == 0 || len(citedPages) == 0 {
		return false
	}
	byPage := make(map[int]string, len(doc.Pages))
	for _, p := range doc.Pages {
		byPage[p.Page] = p.Content
	}
	var cited strings.Builder
	for _, p := range citedPages {
		cited.WriteString(byPage[p])
		cited.WriteByte(' ')
	}
	citedWords := wordSet(cited.String())
	for _, e := range q.Evidence {
		ew := words(e.Text)
		if len(ew) == 0 {
			continue
		}
		hits := 0
		for _, w := range ew {
			if citedWords[w] {
				hits++
			}
		}
		if float64(hits)/float64(len(ew)) >= 0.6 {
			return true
		}
	}
	return false
}

func isAlnum(r rune) bool { return r >= 'a' && r <= 'z' || r >= '0' && r <= '9' }

// words returns the lowercased alphanumeric tokens (length >= 4) of s.
func words(s string) []string {
	var out []string
	for _, f := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !isAlnum(r) }) {
		if len(f) >= 4 {
			out = append(out, f)
		}
	}
	return out
}

func wordSet(s string) map[string]bool {
	set := map[string]bool{}
	for _, w := range words(s) {
		set[w] = true
	}
	return set
}

// Judge grades predicted against the gold answer via the permissive equivalence
// rubric.
func Judge(ctx context.Context, judge llm.Provider, model string, q Question, predicted string) (bool, error) {
	out, err := llm.CompleteJSON[prompts.Equivalence](ctx, judge,
		llm.UserPrompt(model, prompts.JudgeEquivalence(q.Question, q.Answer, predicted)), 3, nil)
	if err != nil {
		return false, err
	}
	return out.Correct, nil
}

// RunResult is the per-question outcome.
type RunResult struct {
	Question    Question
	Predicted   string
	Cited       []int
	GoldPages   []int
	Correct     bool
	PageHit     bool // gold printed page == cited physical page (alignment-sensitive)
	EvidenceHit bool // cited page text contains the gold evidence (alignment-free)
	Err         error
}

// Aggregate holds run-level metrics.
type Aggregate struct {
	Total, Scored, CorrectCount, PageHitCount, EvidenceHitCount int
}

// AnswerAccuracy is correct/scored.
func (a Aggregate) AnswerAccuracy() float64 {
	if a.Scored == 0 {
		return 0
	}
	return float64(a.CorrectCount) / float64(a.Scored)
}

// RecallAtPage is page-number-hits/scored (alignment-sensitive; see PageHit).
func (a Aggregate) RecallAtPage() float64 {
	if a.Scored == 0 {
		return 0
	}
	return float64(a.PageHitCount) / float64(a.Scored)
}

// EvidenceRecall is evidence-text-hits/scored (alignment-free; the trustworthy
// retrieval metric).
func (a Aggregate) EvidenceRecall() float64 {
	if a.Scored == 0 {
		return 0
	}
	return float64(a.EvidenceHitCount) / float64(a.Scored)
}

// Run answers and scores each question against a pre-indexed corpus. lookup maps
// a FinanceBench doc_name to its indexed Document.
func Run(ctx context.Context, asker *ask.Asker, judge llm.Provider, judgeModel string, questions []Question, lookup func(docName string) (tree.Document, bool)) ([]RunResult, Aggregate) {
	results := make([]RunResult, 0, len(questions))
	agg := Aggregate{Total: len(questions)}
	for _, q := range questions {
		r := RunResult{Question: q, GoldPages: GoldPages(q)}
		doc, ok := lookup(q.DocName)
		if !ok {
			r.Err = fmt.Errorf("document %q not indexed in workspace", q.DocName)
			results = append(results, r)
			continue
		}
		ans, err := asker.Ask(ctx, doc, q.Question)
		if err != nil {
			r.Err = err
			results = append(results, r)
			continue
		}
		r.Predicted = ans.Text
		r.Cited = ans.CitedPages
		r.PageHit = RecallAtPage(r.GoldPages, ans.CitedPages)
		r.EvidenceHit = EvidenceHit(doc, ans.CitedPages, q)
		correct, jerr := Judge(ctx, judge, judgeModel, q, ans.Text)
		if jerr != nil {
			r.Err = jerr
			results = append(results, r)
			continue
		}
		r.Correct = correct
		agg.Scored++
		if correct {
			agg.CorrectCount++
		}
		if r.PageHit {
			agg.PageHitCount++
		}
		if r.EvidenceHit {
			agg.EvidenceHitCount++
		}
		results = append(results, r)
	}
	return results, agg
}
