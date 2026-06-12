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

Regenerate with `go run ./eval/financebench/aggregate`. As of 2026-06-12 (9/84 docs, 32/150 questions):

| Effort | Raw accuracy | Adjusted accuracy | Evidence recall | Hallucination |
|---|---|---|---|---|
| low | 87.50% (28/32) | 96.88% | 81.25% | 12.50% |
| medium | 87.50% (28/32) | 96.88% | 84.38% | 12.50% |
| **high** | **90.62% (29/32)** | **93.75%** | 87.50% | 9.38% |
| ultra | 87.50% (28/32) | 90.62% | 87.50% | 12.50% |

Adjusted numbers are provisional: the three misses introduced by the 2026-06-12 installment
(see below) are auto-labelled `NAL` and still await human adjudication.

### Documents in the pool so far

| Document | Questions |
|---|---|
| AMD_2022_10K | 7 |
| AMERICANEXPRESS_2022_10K | 7 |
| BESTBUY_2024Q2_10Q | 3 |
| BOEING_2022_10K | 7 |
| FOOTLOCKER_2022_8K_dated-2022-05-20 | 1 |
| FOOTLOCKER_2022_8K_dated_2022-08-19 | 1 |
| JOHNSON_JOHNSON_2023Q2_EARNINGS | 1 |
| JOHNSON_JOHNSON_2023_8K_dated-2023-08-30 | 3 |
| PEPSICO_2023_8K_dated-2023-05-30 | 2 |

All are outside the diagnostic **train** split (no prompt-tuning contamination).

### Human adjudications (low/medium + high/ultra)

| ID | Effort(s) | Label | Summary |
|---|---|---|---|
| financebench_id_00460 | low, medium | BE | Gold uses total store count; question asks Best Buy-branded stores |
| financebench_id_01902 | low, medium | BE | Gold uses comp-sales growth; question asks top-line revenue category |
| financebench_id_00839 | low, medium | SEDC | Same CEO/Ulta evidence; interpretive split on "similar company" |
| financebench_id_00222 | high, ultra | MVA | AMD quick ratio — alternate valid formula, same conclusion |

### Pending human review (2026-06-12 installment, auto-labelled `NAL`)

| ID | Effort(s) | Question |
|---|---|---|
| financebench_id_00585 | all four | Boeing FY2022 vs FY2021 effective tax rate — gold says 0.62% vs -14.76%; pindex read (0.6)% vs 14.7% off the page-77 reconciliation (sign convention differs) |
| financebench_id_00476 | high, ultra | Amex debt securities registered on a national exchange — gold "There are none"; pindex listed Note 8 parent-company debt instead |
| financebench_id_00494 | ultra | Boeing FY2023 production rate forecasts — pindex cited the 737/787 increases but hedged on specificity |

- **Raw** is judge-only; **adjusted** also counts human-adjudicated `MVA`/`BE`/`SEDC` relabels (the
  process behind Mafin 2.5's published 98.7%). See each answer record's `label_reason` for detail.
- `medium` has matched `low` on accuracy on every doc so far: its refusal retry has never fired (all
  misses were confident-wrong, not refusals). As of the 2026-06-12 installment their evidence-recall
  paths diverge slightly (84.38% vs 81.25%) while scoring the same questions right.

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
