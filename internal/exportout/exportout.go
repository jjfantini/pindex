// Package exportout writes a browsable, human-readable view of an eval/ask run on
// top of pindex's scalable store (which keeps opaque hash-named blobs). It serves
// two purposes:
//
//   - Diagnosis: per-document folders with the generated tree and a rich per-question
//     record (reasoning, selected/cited pages, stage flags).
//   - Benchmarking & adjudication: a Mafin2.5-compatible result_<model>.json
//     ([{id, question, label, benchmark_answer, pindex_answer}]) plus a human-review
//     CSV of the judge-disagreements, using the AL/MVA/BE/NAL/SEDC label taxonomy so
//     "multiple valid approaches" and "benchmark errors" can be excused (the way
//     Mafin reaches its headline accuracy).
//
// It is pure I/O: no LLM provider and no store coupling, so every writer is
// unit-testable against a t.TempDir().
package exportout

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jjfantini/pindex/eval/financebench"
	"github.com/jjfantini/pindex/internal/tree"
)

// Adjudication labels (Mafin2.5's human-evaluation taxonomy). AL/MVA/BE/SEDC count as
// correct under the adjusted metric; only NAL counts as wrong.
const (
	LabelAL   = "AL"   // Answers Aligned: matches benchmark in conclusion and methodology
	LabelMVA  = "MVA"  // Multiple Valid Approaches: both valid, different methods — excused
	LabelBE   = "BE"   // Benchmark Error: gold answer is wrong, model is right — excused
	LabelNAL  = "NAL"  // Not Aligned: a genuine miss
	LabelSEDC = "SEDC" // Same Evidence, Different Conclusion
)

// AutoLabel assigns the default label from the automated judge verdict: a correct
// answer is AL (aligned); a wrong one is NAL (pending human review into MVA/BE/…).
func AutoLabel(correct bool) string {
	if correct {
		return LabelAL
	}
	return LabelNAL
}

// AdjustedCorrect reports whether a label counts as correct under the adjusted
// metric — aligned, multiple-valid-approach, benchmark error, or same-evidence
// different valid conclusion (SEDC).
func AdjustedCorrect(label string) bool {
	switch label {
	case LabelAL, LabelMVA, LabelBE, LabelSEDC:
		return true
	default:
		return false
	}
}

// TreeExport is the browsable per-document tree ({doc_name}_pindex.json): metadata
// plus the structure. Page text is stripped by default for readability.
type TreeExport struct {
	ID          string             `json:"id"`
	DocName     string             `json:"doc_name"`
	Type        string             `json:"type"`
	Description string             `json:"doc_description,omitempty"`
	PageCount   int                `json:"page_count,omitempty"`
	LineCount   int                `json:"line_count,omitempty"`
	PageOffset  int                `json:"page_offset,omitempty"`
	PageMap     tree.PageMap       `json:"page_map,omitempty"`
	Structure   []tree.TreeNode    `json:"structure"`
	Pages       []tree.PageContent `json:"pages,omitempty"`
}

// AnswerRecord is the rich per-question diagnostic record (<doc>/answers/<id>.json):
// the full reasoning, the pages selected and cited, and the per-stage outcome.
type AnswerRecord struct {
	FinancebenchID    string `json:"financebench_id,omitempty"`
	Company           string `json:"company,omitempty"`
	DocName           string `json:"doc_name"`
	Question          string `json:"question"`
	GoldAnswer        string `json:"gold_answer,omitempty"`
	Predicted         string `json:"predicted"`
	Reasoning         string `json:"reasoning,omitempty"`
	Verification      string `json:"verification,omitempty"`
	Steps             int    `json:"steps,omitempty"`
	SelectedPages     string `json:"selected_pages,omitempty"`
	CitedPages        []int  `json:"cited_pages,omitempty"`
	CitedPagesPrinted []int  `json:"cited_pages_printed,omitempty"`
	GoldPages         []int  `json:"gold_pages,omitempty"`
	ExtractionOK      bool   `json:"extraction_ok"`
	RetrievalOK       bool   `json:"retrieval_ok"`
	AnswerOK          bool   `json:"answer_ok"`
	Hallucinated      bool   `json:"hallucinated"`
	PageHit           bool   `json:"page_hit"`
	Label             string `json:"label,omitempty"`
	LabelReason       string `json:"label_reason,omitempty"`
	Error             string `json:"error,omitempty"`
}

// AggregateRecords sums per-question AnswerRecords (the single source of truth in
// an accumulating benchmark tree) into a Summary. Records with an Error are
// counted in QuestionsTotal but excluded from Scored and every rate, matching the
// live eval funnel. Rates are over Scored.
func AggregateRecords(records []AnswerRecord) Summary {
	var s Summary
	s.QuestionsTotal = len(records)
	s.LabelCounts = map[string]int{}
	var ext, ret, ans, adj, hal, hit int
	for _, r := range records {
		if r.Error != "" {
			continue
		}
		s.Scored++
		if r.ExtractionOK {
			ext++
		}
		if r.RetrievalOK {
			ret++
		}
		if r.AnswerOK {
			ans++
		}
		if r.Hallucinated {
			hal++
		}
		if r.PageHit {
			hit++
		}
		label := r.Label
		if label == "" {
			label = AutoLabel(r.AnswerOK)
		}
		s.LabelCounts[label]++
		if AdjustedCorrect(label) {
			adj++
		}
	}
	if s.Scored == 0 {
		return s
	}
	n := float64(s.Scored)
	s.ExtractionRate = float64(ext) / n
	s.RetrievalRate = float64(ret) / n
	s.AnswerAccuracyRaw = float64(ans) / n
	s.AnswerAccuracyAdjusted = float64(adj) / n
	s.HallucinationRate = float64(hal) / n
	s.RecallAtPage = float64(hit) / n
	return s
}

// MafinRecord mirrors Mafin2.5's result_<model>.json schema exactly, with
// mafin_answer renamed to pindex_answer. PindexAnswer is the concise final answer
// only (the full reasoning lives in the per-document AnswerRecord).
type MafinRecord struct {
	ID              string `json:"id"`
	Question        string `json:"question"`
	Label           string `json:"label"`
	BenchmarkAnswer string `json:"benchmark_answer"`
	PindexAnswer    string `json:"pindex_answer"`
}

// Summary is the run-level summary.json: configuration plus the funnel and both the
// raw (judge-only) and adjusted (MVA/BE/SEDC excused) answer accuracy.
type Summary struct {
	GeneratedAt            string         `json:"generated_at,omitempty"`
	Model                  string         `json:"model"`
	JudgeModel             string         `json:"judge_model"`
	Effort                 string         `json:"effort"`
	RPM                    int            `json:"rpm"`
	QuestionsTotal         int            `json:"questions_total"`
	Scored                 int            `json:"scored"`
	ExtractionRate         float64        `json:"extraction_rate"`
	RetrievalRate          float64        `json:"retrieval_rate"`
	AnswerAccuracyRaw      float64        `json:"answer_accuracy_raw"`
	AnswerAccuracyAdjusted float64        `json:"answer_accuracy_adjusted"`
	HallucinationRate      float64        `json:"hallucination_rate"`
	RecallAtPage           float64        `json:"recall_at_page"`
	LabelCounts            map[string]int `json:"label_counts,omitempty"`
}

// Sanitize turns a document or model name into a safe filename component: a trailing
// ".pdf" is dropped, any rune outside [A-Za-z0-9._-] becomes "_", runs of inserted
// "_" collapse, and surrounding "_"/"." are trimmed. Empty input yields "doc".
// Filename safety is an export concern, so it lives here (not in tree/).
func Sanitize(name string) string {
	s := strings.TrimSpace(name)
	if len(s) >= 4 && strings.EqualFold(s[len(s)-4:], ".pdf") {
		s = s[:len(s)-4]
	}
	var b strings.Builder
	pendingUnderscore := false
	for _, r := range s {
		if r == '_' || r == '.' || r == '-' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			pendingUnderscore = false
			continue
		}
		if !pendingUnderscore {
			b.WriteByte('_')
			pendingUnderscore = true
		}
	}
	out := strings.Trim(b.String(), "_.")
	if out == "" {
		return "doc"
	}
	return out
}

// EvalRunDir picks the default output directory for an eval run when --out is
// not given: <workspace-parent>/evals/<YYYY-MM-DD>_<model>_<effort> — a sibling
// of the workspace, so the workspace stays a purely regenerable index catalog
// while run artifacts (which cost API calls to reproduce) live next to it,
// alongside the cache. Same-day re-runs of the same model and effort are never
// overwritten: the first free of <name>, <name>-2, <name>-3, … is chosen. The
// directory itself is created later by ExportEval.
func EvalRunDir(workspace, model, effort string, now time.Time) (string, error) {
	base := filepath.Join(filepath.Dir(filepath.Clean(workspace)), "evals")
	name := fmt.Sprintf("%s_%s_%s", now.Format("2006-01-02"), Sanitize(model), Sanitize(effort))
	for i := 1; ; i++ {
		candidate := name
		if i > 1 {
			candidate = fmt.Sprintf("%s-%d", name, i)
		}
		p := filepath.Join(base, candidate)
		_, err := os.Stat(p)
		switch {
		case os.IsNotExist(err):
			return p, nil
		case err != nil:
			return "", fmt.Errorf("exportout: probe eval dir %s: %w", p, err)
		}
	}
}

// QuestionSlug is a stable, collision-resistant filename for an ad-hoc (ask) answer:
// a sanitized, truncated form of the question plus a short content hash.
func QuestionSlug(q string) string {
	sum := sha256.Sum256([]byte(q))
	h := hex.EncodeToString(sum[:])[:8]
	base := Sanitize(q)
	if len(base) > 40 {
		base = strings.Trim(base[:40], "_.")
	}
	if base == "" {
		base = "q"
	}
	return base + "_" + h
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func treeExport(doc tree.Document, includePages bool) TreeExport {
	te := TreeExport{
		ID:          doc.ID,
		DocName:     doc.DocName,
		Type:        string(doc.Type),
		Description: doc.DocDescription,
		PageCount:   doc.PageCount,
		LineCount:   doc.LineCount,
		PageOffset:  doc.PageOffset,
		PageMap:     doc.PageMap,
	}
	if includePages {
		te.Structure = doc.Structure
		te.Pages = doc.Pages
	} else {
		te.Structure = tree.StripText(doc.Structure)
	}
	return te
}

// WriteTree writes <outDir>/<doc>/<doc>_pindex.json and returns its path.
func WriteTree(outDir string, doc tree.Document, includePages bool) (string, error) {
	name := Sanitize(doc.DocName)
	dir := filepath.Join(outDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, name+"_pindex.json")
	if err := writeJSON(path, treeExport(doc, includePages)); err != nil {
		return "", err
	}
	return path, nil
}

// WriteAnswer writes a rich record to <outDir>/<doc>/answers/<id-or-slug>.json.
func WriteAnswer(outDir string, rec AnswerRecord) (string, error) {
	docName := Sanitize(rec.DocName)
	dir := filepath.Join(outDir, docName, "answers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	base := Sanitize(rec.FinancebenchID)
	if rec.FinancebenchID == "" {
		base = QuestionSlug(rec.Question)
	}
	path := filepath.Join(dir, base+".json")
	if err := writeJSON(path, rec); err != nil {
		return "", err
	}
	return path, nil
}

// WriteQuestions writes the exact question set as <outDir>/questions.jsonl.
func WriteQuestions(outDir string, qs []financebench.Question) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(outDir, "questions.jsonl"))
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	enc := json.NewEncoder(f)
	for _, q := range qs {
		if err := enc.Encode(q); err != nil {
			return err
		}
	}
	return nil
}

// WriteSummary writes <outDir>/summary.json.
func WriteSummary(outDir string, s Summary) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	return writeJSON(filepath.Join(outDir, "summary.json"), s)
}

// WriteMafinResult writes the Mafin-compatible <outDir>/result_<model>.json array.
func WriteMafinResult(outDir, model string, recs []MafinRecord) (string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(outDir, "result_"+Sanitize(model)+".json")
	if err := writeJSON(path, recs); err != nil {
		return "", err
	}
	return path, nil
}

// WriteHumanEvalCSV writes <outDir>/human_evaluations/<model>.csv with only the
// judge-disagreements (label NAL) for a human to re-label MVA/BE/NAL/SEDC. Columns
// mirror Mafin's human_evaluation CSV.
func WriteHumanEvalCSV(outDir, model string, recs []MafinRecord) (string, error) {
	dir := filepath.Join(outDir, "human_evaluations")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, Sanitize(model)+".csv")
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	w := csv.NewWriter(f)
	if err := w.Write([]string{"id", "question", "label", "label reason", "benchmark answer", "pindex answer"}); err != nil {
		return "", err
	}
	for _, r := range recs {
		if r.Label != LabelNAL {
			continue
		}
		if err := w.Write([]string{r.ID, r.Question, r.Label, "", r.BenchmarkAnswer, r.PindexAnswer}); err != nil {
			return "", err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}
	return path, nil
}

// recordFromResult maps a scored RunResult to a rich AnswerRecord (pure).
func recordFromResult(r financebench.RunResult) AnswerRecord {
	rec := AnswerRecord{
		FinancebenchID:    r.Question.ID,
		Company:           r.Question.Company,
		DocName:           r.Question.DocName,
		Question:          r.Question.Question,
		GoldAnswer:        r.Question.Answer,
		Predicted:         r.Predicted,
		Reasoning:         r.Reasoning,
		Verification:      r.Verification,
		Steps:             r.Steps,
		SelectedPages:     r.SelectedPages,
		CitedPages:        r.Cited,
		CitedPagesPrinted: r.CitedPrinted,
		GoldPages:         r.GoldPages,
		ExtractionOK:      r.EvidenceInDoc,
		RetrievalOK:       r.EvidenceHit,
		AnswerOK:          r.Correct,
		Hallucinated:      r.Hallucinated,
		PageHit:           r.PageHit,
		Label:             labelFor(r),
	}
	if r.Err != nil {
		rec.Error = r.Err.Error()
	}
	return rec
}

// mafinFromResult maps a RunResult to the Mafin-compatible record (pure).
func mafinFromResult(r financebench.RunResult) MafinRecord {
	return MafinRecord{
		ID:              r.Question.ID,
		Question:        r.Question.Question,
		Label:           labelFor(r),
		BenchmarkAnswer: r.Question.Answer,
		PindexAnswer:    r.Predicted,
	}
}

// labelFor auto-labels a result; an errored (unscored) question has no label.
func labelFor(r financebench.RunResult) string {
	if r.Err != nil {
		return ""
	}
	return AutoLabel(r.Correct)
}

// ExportEval writes the full browsable + Mafin-compatible output for an eval run.
// It dedups trees by resolved document id (so two doc_name spellings resolving to
// one document are written once) and fills the adjusted accuracy + label histogram
// onto sum before writing summary.json.
func ExportEval(outDir string, sum Summary, qs []financebench.Question, results []financebench.RunResult,
	lookup func(docName string) (tree.Document, bool), includePages bool, model string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	if err := WriteQuestions(outDir, qs); err != nil {
		return err
	}

	seen := map[string]bool{}
	for _, q := range qs {
		doc, ok := lookup(q.DocName)
		if !ok || seen[doc.ID] {
			continue
		}
		seen[doc.ID] = true
		if _, err := WriteTree(outDir, doc, includePages); err != nil {
			return err
		}
	}

	mafin := make([]MafinRecord, 0, len(results))
	counts := map[string]int{}
	scored, adjCorrect := 0, 0
	for _, r := range results {
		rec := recordFromResult(r)
		// File the answer under the resolved doc's folder so it sits next to that
		// doc's tree, even if the question's doc_name spelling differs.
		if doc, ok := lookup(r.Question.DocName); ok {
			rec.DocName = doc.DocName
		}
		if _, err := WriteAnswer(outDir, rec); err != nil {
			return err
		}
		mr := mafinFromResult(r)
		mafin = append(mafin, mr)
		counts[mr.Label]++
		if mr.Label == "" { // errored / unscored
			continue
		}
		scored++
		if AdjustedCorrect(mr.Label) {
			adjCorrect++
		}
	}
	if _, err := WriteMafinResult(outDir, model, mafin); err != nil {
		return err
	}
	if _, err := WriteHumanEvalCSV(outDir, model, mafin); err != nil {
		return err
	}

	sum.LabelCounts = counts
	if scored > 0 {
		sum.AnswerAccuracyAdjusted = float64(adjCorrect) / float64(scored)
	}
	return WriteSummary(outDir, sum)
}

// Rescore reads a (possibly human-edited) result_<model>.json and recomputes the
// adjusted accuracy (AL+MVA+BE+SEDC over labeled records) and the label histogram. The
// raw (judge-only) accuracy is read from a sibling summary.json when present
// (rawKnown is false otherwise, since edited labels no longer carry the raw signal).
func Rescore(resultPath string) (raw, adjusted float64, counts map[string]int, rawKnown bool, err error) {
	data, err := os.ReadFile(resultPath)
	if err != nil {
		return 0, 0, nil, false, err
	}
	var recs []MafinRecord
	if err := json.Unmarshal(data, &recs); err != nil {
		return 0, 0, nil, false, err
	}
	counts = map[string]int{}
	scored, adjCorrect := 0, 0
	for _, r := range recs {
		counts[r.Label]++
		if r.Label == "" {
			continue
		}
		scored++
		if AdjustedCorrect(r.Label) {
			adjCorrect++
		}
	}
	if scored > 0 {
		adjusted = float64(adjCorrect) / float64(scored)
	}
	if sumData, serr := os.ReadFile(filepath.Join(filepath.Dir(resultPath), "summary.json")); serr == nil {
		var s Summary
		if json.Unmarshal(sumData, &s) == nil {
			raw, rawKnown = s.AnswerAccuracyRaw, true
		}
	}
	return raw, adjusted, counts, rawKnown, nil
}
