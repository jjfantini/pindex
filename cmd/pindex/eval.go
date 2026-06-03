package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jjfantini/pindex/eval/financebench"
	"github.com/jjfantini/pindex/internal/ask"
	"github.com/jjfantini/pindex/internal/config"
	"github.com/jjfantini/pindex/internal/envfile"
	"github.com/jjfantini/pindex/internal/exportout"
	"github.com/jjfantini/pindex/internal/store"
	"github.com/jjfantini/pindex/internal/tree"
)

func newEvalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "eval",
		Short: "Run the FinanceBench evaluation over a pre-indexed workspace",
		RunE: func(c *cobra.Command, _ []string) error {
			// --rescore: read back a (possibly human-edited) result_<model>.json,
			// recompute adjusted accuracy from the labels, and return. No API needed.
			if rescore, _ := c.Flags().GetString("rescore"); rescore != "" {
				raw, adjusted, counts, rawKnown, rerr := exportout.Rescore(rescore)
				if rerr != nil {
					return rerr
				}
				out := c.OutOrStdout()
				_, _ = fmt.Fprintf(out, "=== rescore %s ===\n", rescore)
				if rawKnown {
					_, _ = fmt.Fprintf(out, "  raw answer accuracy (judge only): %5.1f%%\n", raw*100)
				} else {
					_, _ = fmt.Fprintln(out, "  raw answer accuracy (judge only): n/a (no sibling summary.json)")
				}
				_, _ = fmt.Fprintf(out, "  adjusted accuracy (AL+MVA+BE):    %5.1f%%\n", adjusted*100)
				_, _ = fmt.Fprintf(out, "  labels: %v\n", counts)
				return nil
			}

			envFile, _ := c.Flags().GetString("env-file")
			if err := envfile.Load(envFile); err != nil {
				return err
			}
			cfgPath, _ := c.Flags().GetString("config")
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			if m, _ := c.Flags().GetString("model"); m != "" {
				cfg.Model = m
			}
			qpath, _ := c.Flags().GetString("questions")
			ws, _ := c.Flags().GetString("workspace")
			cacheDir, _ := c.Flags().GetString("cache-dir")
			limit, _ := c.Flags().GetInt("limit")
			judgeModel, _ := c.Flags().GetString("judge-model")
			if judgeModel == "" {
				judgeModel = cfg.RetrieveModelOrDefault()
			}

			if qpath == "" {
				return fmt.Errorf("eval: --questions is required (or use --rescore <file>)")
			}
			questions, err := financebench.LoadQuestions(qpath)
			if err != nil {
				return err
			}
			if limit > 0 && limit < len(questions) {
				questions = questions[:limit]
			}

			s, err := store.Open(ws)
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			lookup, err := buildLookup(s)
			if err != nil {
				return err
			}

			rpm, _ := c.Flags().GetInt("rpm")
			retrieveProvider, err := buildProvider(cfg.RetrieveModelOrDefault(), cacheDir, rpm)
			if err != nil {
				return err
			}
			judgeProvider, err := buildProvider(judgeModel, cacheDir, rpm)
			if err != nil {
				return err
			}

			effStr, _ := c.Flags().GetString("effort")
			effort, err := ask.ParseEffort(effStr)
			if err != nil {
				return err
			}
			asker := ask.New(retrieveProvider, cfg.RetrieveModelOrDefault())
			asker.Effort = effort
			results, agg := financebench.Run(c.Context(), asker, judgeProvider, judgeModel, questions, lookup)

			flag := func(b bool, yes string) string {
				if b {
					return yes
				}
				return "-"
			}
			for _, r := range results {
				if r.Err != nil {
					_, _ = fmt.Fprintf(c.ErrOrStderr(), "[err] %s: %v\n", r.Question.ID, r.Err)
					continue
				}
				hal := ""
				if r.Hallucinated {
					hal = " HALLUC"
				}
				// Per-question stage flags: extraction / retrieval / answer.
				_, _ = fmt.Fprintf(c.ErrOrStderr(), "[ext:%s ret:%s ans:%s%s] %s  gold=%q pred=%q cited=%v\n",
					flag(r.EvidenceInDoc, "Y"), flag(r.EvidenceHit, "Y"), flag(r.Correct, "Y"), hal,
					r.Question.ID, clip(r.Question.Answer, 50), clip(r.Predicted, 80), r.Cited)
			}

			ext, ret, ansr, hal := agg.Funnel()
			out := c.OutOrStdout()
			_, _ = fmt.Fprintf(out, "\n=== stage funnel (scored %d/%d) ===\n", agg.Scored, agg.Total)
			_, _ = fmt.Fprintf(out, "  extraction (evidence in extracted text): %5.1f%%\n", ext*100)
			_, _ = fmt.Fprintf(out, "  retrieval  (cited page holds evidence):  %5.1f%%   [evidence-recall]\n", ret*100)
			_, _ = fmt.Fprintf(out, "  answer     (judged correct):             %5.1f%%   [answer-accuracy]\n", ansr*100)
			_, _ = fmt.Fprintf(out, "  hallucination (confident-wrong):         %5.1f%%\n", hal*100)
			_, _ = fmt.Fprintf(out, "  (page-number recall %.1f%%, alignment-sensitive)\n", agg.RecallAtPage()*100)

			if outDir, _ := c.Flags().GetString("out"); outDir != "" {
				inclPages, _ := c.Flags().GetBool("include-pages")
				model := cfg.RetrieveModelOrDefault()
				sum := exportout.Summary{
					GeneratedAt:       time.Now().UTC().Format(time.RFC3339),
					Model:             model,
					JudgeModel:        judgeModel,
					Effort:            string(effort),
					RPM:               rpm,
					QuestionsTotal:    agg.Total,
					Scored:            agg.Scored,
					ExtractionRate:    ext,
					RetrievalRate:     ret,
					AnswerAccuracyRaw: ansr,
					HallucinationRate: hal,
					RecallAtPage:      agg.RecallAtPage(),
				}
				if err := exportout.ExportEval(outDir, sum, questions, results, lookup, inclPages, model); err != nil {
					return err
				}
				_, _ = fmt.Fprintf(c.ErrOrStderr(), "wrote browsable output to %s\n", outDir)
			}
			return nil
		},
	}
	cmd.Flags().String("questions", "", "path to a FinanceBench JSONL file (required)")
	cmd.Flags().String("workspace", ".pindex/workspace", "workspace with the docs pre-indexed")
	cmd.Flags().String("model", "", "retrieval model (default from config)")
	cmd.Flags().String("judge-model", "", "LLM-judge model (default: retrieval model)")
	cmd.Flags().String("cache-dir", ".pindex/cache", "prompt-hash response cache dir")
	cmd.Flags().String("env-file", ".env", "load API keys from this .env file")
	cmd.Flags().Int("limit", 0, "only run the first N questions (0 = all)")
	cmd.Flags().Int("rpm", 0, "max requests/min to the LLM (0 = unlimited; set on low rate-limit tiers)")
	cmd.Flags().String("effort", "low", "ask reasoning effort: low|medium|high|ultra")
	cmd.Flags().String("out", "", "write a browsable output dir (per-doc trees, questions, answers, Mafin-compatible result_<model>.json + human-eval CSV, summary)")
	cmd.Flags().Bool("include-pages", false, "include raw page text in exported trees (larger, less readable)")
	cmd.Flags().String("rescore", "", "recompute adjusted accuracy from a (human-edited) result_<model>.json and exit")
	return cmd
}

// buildLookup maps a FinanceBench doc_name to an indexed Document by matching the
// catalog's doc_name ignoring case and the .pdf extension.
func buildLookup(s *store.Store) (func(string) (tree.Document, bool), error) {
	list, err := s.List()
	if err != nil {
		return nil, err
	}
	byName := make(map[string]string, len(list))
	for _, r := range list {
		byName[normalizeName(r.DocName)] = r.ID
	}
	return func(docName string) (tree.Document, bool) {
		id, ok := byName[normalizeName(docName)]
		if !ok {
			return tree.Document{}, false
		}
		doc, err := s.Load(id)
		return doc, err == nil
	}, nil
}

func normalizeName(s string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), ".pdf")
}

func clip(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
