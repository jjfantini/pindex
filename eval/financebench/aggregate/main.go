// Command aggregate rebuilds every derived artifact in the accumulating
// FinanceBench results tree (eval/financebench/results/) from its single source
// of truth: the per-question answer records.
//
// Layout it operates on:
//
//	results/<model>/<effort>/<DOC>/answers/<id>.json   per-question records (source of truth)
//	results/<model>/<effort>/<DOC>/run.json            per-doc-run provenance (date, judge)
//
// For each <model>/<effort> it (re)writes:
//
//	summary.json            aggregate funnel + raw/adjusted accuracy over ALL docs
//	result_<model>.json     Mafin2.5-compatible aggregate record list
//	human_evaluations.csv   every non-AL row (judge-wrong or human-relabelled) for review
//
// and prints a per-model markdown scoreboard. Human adjudication happens by
// editing label (and label_reason) in the per-question record, then re-running
// this tool; the CSV and summaries are regenerated views, never hand-edited.
//
// Usage: go run ./eval/financebench/aggregate [resultsDir]
package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jjfantini/pindex/internal/exportout"
)

// runMeta is the per-doc-run provenance file (run.json).
type runMeta struct {
	DocName     string `json:"doc_name"`
	GeneratedAt string `json:"generated_at"`
	Model       string `json:"model"`
	JudgeModel  string `json:"judge_model"`
	Effort      string `json:"effort"`
	Questions   int    `json:"questions"`
	Note        string `json:"note,omitempty"`
}

func main() {
	root := "eval/financebench/results"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	models, err := subdirs(root)
	if err != nil {
		fatal(err)
	}
	for _, model := range models {
		fmt.Printf("## %s\n\n", model)
		fmt.Println("| Effort | Docs | Questions | Raw accuracy | Adjusted accuracy | Evidence recall | Hallucination |")
		fmt.Println("|---|---|---|---|---|---|---|")
		for _, effort := range []string{"low", "medium", "high", "ultra"} {
			dir := filepath.Join(root, model, effort)
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				continue
			}
			if err := aggregateEffort(dir, model, effort); err != nil {
				fatal(fmt.Errorf("%s/%s: %w", model, effort, err))
			}
		}
		fmt.Println()
	}
}

// aggregateEffort rebuilds the derived artifacts for one <model>/<effort> dir
// and prints its scoreboard row.
func aggregateEffort(dir, model, effort string) error {
	docs, err := subdirs(dir)
	if err != nil {
		return err
	}
	var records []exportout.AnswerRecord
	judge := ""
	latest := ""
	for _, doc := range docs {
		var meta runMeta
		if err := readJSON(filepath.Join(dir, doc, "run.json"), &meta); err != nil {
			return fmt.Errorf("doc %s: %w (every doc dir needs run.json provenance)", doc, err)
		}
		// Mixing judges silently would corrupt the pooled number — fail loudly.
		if judge != "" && meta.JudgeModel != judge {
			return fmt.Errorf("doc %s judged by %q but earlier docs by %q — a pooled benchmark needs one judge", doc, meta.JudgeModel, judge)
		}
		judge = meta.JudgeModel
		if meta.GeneratedAt > latest {
			latest = meta.GeneratedAt
		}
		files, err := filepath.Glob(filepath.Join(dir, doc, "answers", "*.json"))
		if err != nil {
			return err
		}
		if len(files) == 0 {
			return fmt.Errorf("doc %s has no answer records", doc)
		}
		sort.Strings(files)
		for _, f := range files {
			var r exportout.AnswerRecord
			if err := readJSON(f, &r); err != nil {
				return fmt.Errorf("record %s: %w", f, err)
			}
			records = append(records, r)
		}
	}

	sum := exportout.AggregateRecords(records)
	sum.GeneratedAt = latest
	sum.Model = model
	sum.JudgeModel = judge
	sum.Effort = effort
	if err := writeJSON(filepath.Join(dir, "summary.json"), sum); err != nil {
		return err
	}

	// Mafin2.5-compatible aggregate result file.
	mafin := make([]exportout.MafinRecord, 0, len(records))
	for _, r := range records {
		if r.Error != "" {
			continue
		}
		label := r.Label
		if label == "" {
			label = exportout.AutoLabel(r.AnswerOK)
		}
		mafin = append(mafin, exportout.MafinRecord{
			ID:              r.FinancebenchID,
			Question:        r.Question,
			Label:           label,
			BenchmarkAnswer: r.GoldAnswer,
			PindexAnswer:    r.Predicted,
		})
	}
	sort.Slice(mafin, func(i, j int) bool { return mafin[i].ID < mafin[j].ID })
	if err := writeJSON(filepath.Join(dir, "result_"+exportout.Sanitize(model)+".json"), mafin); err != nil {
		return err
	}

	// Human-review worksheet: every non-AL row (graded rows keep their reason).
	if err := writeReviewCSV(filepath.Join(dir, "human_evaluations.csv"), records); err != nil {
		return err
	}

	fmt.Printf("| %s | %d | %d | %.2f%% (%d/%d) | %.2f%% | %.2f%% | %.2f%% |\n",
		effort, len(docs), sum.Scored,
		sum.AnswerAccuracyRaw*100, int(sum.AnswerAccuracyRaw*float64(sum.Scored)+0.5), sum.Scored,
		sum.AnswerAccuracyAdjusted*100, sum.RetrievalRate*100, sum.HallucinationRate*100)
	return nil
}

func writeReviewCSV(path string, records []exportout.AnswerRecord) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	w := csv.NewWriter(f)
	if err := w.Write([]string{"id", "doc_name", "question", "label", "label reason", "benchmark answer", "pindex answer"}); err != nil {
		return err
	}
	for _, r := range records {
		label := r.Label
		if label == "" {
			label = exportout.AutoLabel(r.AnswerOK)
		}
		if r.Error != "" || label == exportout.LabelAL {
			continue
		}
		if err := w.Write([]string{r.FinancebenchID, r.DocName, r.Question, label, r.LabelReason, r.GoldAnswer, r.Predicted}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func subdirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && e.Name() != "trees" {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

func readJSON(path string, v any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}

func writeJSON(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "aggregate:", err)
	os.Exit(1)
}
