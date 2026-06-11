# FinanceBench results — the accumulating benchmark

pindex is benchmarked against the full [FinanceBench](https://github.com/patronus-ai/financebench)
open-source set (150 questions, 84 documents) **incrementally**: one document at a time, each
indexed once and evaluated at every effort level, folded into this tree. Running the full suite
in one shot would be expensive; accumulating it document by document costs about a dollar per
installment and never recomputes what's already done.

## Layout

```
results/
  <model>/                      e.g. claude-haiku-4-5-20251001
    trees/<DOC>_pindex.json     one text-stripped tree per doc (shared by all efforts)
    low|medium|high|ultra/
      summary.json              aggregate over ALL docs — regenerated on every run
      result_<model>.json       Mafin2.5-compatible aggregate record list
      human_evaluations.csv     every non-AL question, for human review
      <DOC>/
        run.json                provenance: date, model, judge, question count
        answers/<id>.json       per-question records — THE SOURCE OF TRUTH
```

Everything except `<DOC>/answers/` and `<DOC>/run.json` is **derived**: the aggregator
(`go run ./eval/financebench/aggregate`) rebuilds every `summary.json`, `result_<model>.json`,
and `human_evaluations.csv` from the per-question records and prints the scoreboard. Never
hand-edit the derived files.

## Scoreboard — claude-haiku-4-5-20251001 (generation + indexing), gpt-4o-2024-11-20 judge

Regenerate with `go run ./eval/financebench/aggregate`. As of 2026-06-11 (7/84 docs, 18/150 questions):

| Effort | Raw accuracy | Adjusted accuracy | Evidence recall | Hallucination |
|---|---|---|---|---|
| low | 83.33% (15/18) | 100.0% | 88.89% | 16.67% |
| medium | 83.33% (15/18) | 100.0% | 88.89% | 16.67% |
| **high** | **94.44% (17/18)** | **100.0%** | 94.44% | 5.56% |
| **ultra** | **94.44% (17/18)** | **100.0%** | 94.44% | 5.56% |

- **Raw** is judge-only; **adjusted** also counts human-adjudicated `MVA`/`BE`/`SEDC` relabels (the
  process behind Mafin 2.5's published 98.7%). Adjudications so far: 4 on low/medium (2 BE, 1 SEDC,
  plus the AMD quick-ratio MVA on high/ultra — see each `label_reason`).
- `medium` has matched `low` on every doc so far: its refusal retry has never fired (all misses
  were confident-wrong, not refusals).

## Adding a document (one command)

```sh
./eval/financebench/bench.sh BOEING_2022_10K                # all four efforts, haiku, gpt-4o judge
./eval/financebench/bench.sh PEPSICO_2022_10K --model gpt-4o-mini --efforts "low high"
```

`bench.sh` fetches the questions + PDF (`fetch.sh`), indexes into the gitignored
`testdata/ws` workspace, evals each effort, folds the records into this tree, and re-aggregates.
Pooling rule: one judge per `<model>/<effort>` directory (the aggregator fails loudly on a mix),
and each document appears at most once — so the summed numbers are a clean micro-average.

## Human adjudication workflow

1. Open `<model>/<effort>/human_evaluations.csv` — every question the judge scored wrong (or
   that was already relabelled) is there.
2. To relabel: edit `label` (and add a `label_reason`) in the per-question record
   `<DOC>/answers/<id>.json` — the source of truth. `AL`/`MVA`/`BE`/`SEDC` count correct under the
   adjusted metric; only `NAL` counts wrong. Only a human relabels; the pipeline never
   self-grades above `NAL`.
3. Re-run the aggregator; `pindex eval --rescore <effort>/result_<model>.json` cross-checks.

## Provenance notes

- Documents in the diagnostic **train** split (`../testdata/diagnostic_set.json`) were used while
  tuning prompts; if one is added to the benchmark its scores carry that caveat. There is no
  ML-style train/test split — nothing is trained — the split exists purely to track
  prompt-tuning contamination. All docs benchmarked so far are untainted.
- Key historical finding (PR #31): without mechanically enforced read-before-answering grounding,
  haiku at `high` answered from tree summaries on 5/9 heldout questions and scored *below* `low`
  (55.6%). The enforced agentic loop took the same questions to 9/9.

## License & attribution

Questions, gold answers, and source PDFs are FinanceBench content (Patronus AI,
[arXiv:2311.11944](https://arxiv.org/abs/2311.11944)), licensed **CC-BY-NC-4.0** (per the
[Hugging Face dataset card](https://huggingface.co/datasets/PatronusAI/financebench)). The raw
dataset (`fb.jsonl`, PDFs, page text) is therefore fetched at eval time and never committed.
The committed `result_<model>.json` files contain question text and gold answers for evaluated
questions — the same format upstream
[VectifyAI/Mafin2.5-FinanceBench](https://github.com/VectifyAI/Mafin2.5-FinanceBench) publishes —
included with attribution, **for research / non-commercial purposes only**.
