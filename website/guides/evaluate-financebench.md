---
title: Evaluate on FinanceBench
---

# Evaluate on FinanceBench

## Goal

Measure retrieval and answer accuracy on FinanceBench questions against documents you have already indexed.

## Accumulating benchmark (repo)

The pindex repo maintains an **incremental** FinanceBench scoreboard under
[`eval/financebench/results/`](https://github.com/jjfantini/pindex/tree/develop/eval/financebench/results).
One document at a time:

```sh
./eval/financebench/bench.sh AMD_2022_10K
./eval/financebench/bench.sh BOEING_2022_10K --model gpt-4o-mini --efforts "low high"
```

`bench.sh` fetches questions + PDFs, indexes into a gitignored workspace, evals each effort level,
folds per-question records into the results tree, and re-aggregates. Existing results are never
recomputed.

Regenerate the scoreboard anytime:

```sh
go run ./eval/financebench/aggregate
```

**Current snapshot (2026-06-11)** — `claude-haiku-4-5-20251001`, judge `gpt-4o-2024-11-20`,
7/84 docs, 18/150 questions:

| Effort | Raw accuracy | Adjusted accuracy | Evidence recall |
|---|---|---|---|
| low / medium | 83.33% | 100.0% | 88.89% |
| high / ultra | 94.44% | 100.0% | 94.44% |

- **Raw** = automated LLM judge only.
- **Adjusted** = after human relabels (`MVA`, `BE`, `SEDC` count correct; only `NAL` counts wrong) —
  the process behind Mafin 2.5's published 98.7%.

See [`eval/financebench/results/README.md`](https://github.com/jjfantini/pindex/blob/develop/eval/financebench/results/README.md)
for layout, adjudication workflow, document list, and label definitions.

## Ad-hoc eval (your workspace)

For a one-off run over PDFs you have already indexed:

### Prerequisites

- The referenced PDFs **already indexed** into the workspace — `pindex eval` never indexes; un-indexed docs produce per-question errors
- A FinanceBench JSONL file: one question per line with `financebench_id`, `company`, `doc_name`, `question`, `answer`, and `evidence[]` (`evidence_text`, `evidence_page_num`)
- API key in `.env`

Questions are matched to indexed docs by `doc_name`, case-insensitively and ignoring the `.pdf` extension.

### Steps

**1. Run a small slice first.**

```sh
pindex eval --questions financebench.jsonl --limit 5
```

::: warning Cost
Every question is several LLM calls (page selection or agentic loop, answering, judging). Always start with `--limit` before running a full set.
:::

**2. Read the per-question lines (stderr).**

```
[ext:Y ret:- ans:Y HALLUC] financebench_id_123  gold="..." pred="..." cited=[...]
```

`ext` = gold evidence found in the extracted pages, `ret` = cited pages contain the evidence, `ans` = LLM-judge correctness, `HALLUC` = wrong and not an honest refusal.

**3. Read the summary (stdout).**

A stage funnel: extraction %, retrieval % (evidence recall), answer % (raw accuracy), hallucination %, plus alignment-sensitive page-number recall. `summary.json` in the output dir also carries **adjusted** accuracy after human label edits.

### Options worth knowing

- `--model` / `--judge-model` — retrieval model and judge LLM (judge defaults to the retrieval model; Mafin 2.5 comparability uses `gpt-4o-2024-11-20`).
- `--effort low|medium|high|ultra` — see [Effort levels](/effort-levels). `medium` retries on refusal; `high` uses the agentic tree loop; `ultra` adds verification.
- `--out results/` — choose where the output dir goes. Results are **always saved**: without `--out` they land in `.pindex/evals/<date>_<model>_<effort>/` next to the workspace, and same-day re-runs of the same model and effort get a `-2`, `-3`, … suffix so runs can be compared. Contents: `questions.jsonl`, per-doc trees and answers, a Mafin-compatible `result_<model>.json`, a human-eval CSV, and `summary.json`.
- `--rescore results/result_<model>.json` — recompute adjusted accuracy from a human-edited result file (labels `AL`/`MVA`/`BE`/`SEDC` count correct), offline, no API key needed.
- `--rpm 60` — rate-limit requests.

### Human adjudication

When the judge marks a question wrong, the answer record auto-labels `NAL`. A human may relabel:

| Label | Meaning | Counts adjusted-correct? |
|---|---|---|
| AL | Aligned with benchmark | yes |
| MVA | Multiple valid approaches | yes |
| BE | Benchmark error | yes |
| SEDC | Same evidence, different valid conclusion | yes |
| NAL | Genuine miss | no |

Edit `label` and `label_reason` in the per-question answer JSON (source of truth), then re-run the aggregator or `pindex eval --rescore`.

## What you should see

- One bracketed funnel line per question on stderr.
- A funnel summary on stdout.
- A final `wrote results to <dir>` line on stderr; the dir's `summary.json` carries config, funnel, raw plus adjusted accuracy.
