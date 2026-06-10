---
title: Evaluate on FinanceBench
---

# Evaluate on FinanceBench

## Goal

Measure retrieval and answer accuracy on FinanceBench questions against documents you have already indexed.

## Prerequisites

- The referenced PDFs **already indexed** into the workspace — `pindex eval` never indexes; un-indexed docs produce per-question errors
- A FinanceBench JSONL file: one question per line with `financebench_id`, `company`, `doc_name`, `question`, `answer`, and `evidence[]` (`evidence_text`, `evidence_page_num`)
- API key in `.env`

Questions are matched to indexed docs by `doc_name`, case-insensitively and ignoring the `.pdf` extension.

## Steps

**1. Run a small slice first.**

```sh
pindex eval --questions financebench.jsonl --limit 5
```

::: warning Cost
Every question is several LLM calls (page selection, answering, judging). Always start with `--limit` before running a full set.
:::

**2. Read the per-question lines (stderr).**

```
[ext:Y ret:- ans:Y HALLUC] financebench_id_123  gold="..." pred="..." cited=[...]
```

`ext` = gold evidence found in the extracted pages, `ret` = cited pages contain the evidence, `ans` = LLM-judge correctness, `HALLUC` = wrong and not an honest refusal.

**3. Read the summary (stdout).**

A stage funnel: extraction %, retrieval % (evidence recall), answer % (answer accuracy), hallucination %, plus alignment-sensitive page-number recall.

## Options worth knowing

- `--model` / `--judge-model` — retrieval model and judge LLM (judge defaults to the retrieval model).
- `--effort medium` — retry with a different page set on refusals (default `low`).
- `--out results/` — choose where the output dir goes. Results are **always saved**: without `--out` they land in `.pindex/evals/<date>_<model>_<effort>/` next to the workspace, and same-day re-runs of the same model and effort get a `-2`, `-3`, … suffix so runs can be compared. Contents: `questions.jsonl`, per-doc trees and answers, a Mafin-compatible `result_<model>.json`, a human-eval CSV, and `summary.json`.
- `--rescore results/result_<model>.json` — recompute adjusted accuracy from a human-edited result file, offline, no API key needed.
- `--rpm 60` — rate-limit requests.

## What you should see

- One bracketed funnel line per question on stderr.
- A funnel summary on stdout.
- A final `wrote results to <dir>` line on stderr; the dir's `summary.json` carries config, funnel, and raw plus adjusted accuracy.
