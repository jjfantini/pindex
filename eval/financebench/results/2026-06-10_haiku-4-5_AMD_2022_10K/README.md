# FinanceBench AMD_2022_10K: effort=low vs medium vs high vs ultra (claude-haiku-4-5)

Second installment of the incremental FinanceBench benchmark (one document at a time — see
[`../README.md`](../README.md) for the running scoreboard). All 7 open-source questions for
`AMD_2022_10K` (a fresh document: not in the diagnostic train split, never used for prompt
tuning), at all four effort levels.

- **Date:** 2026-06-10
- **Document:** AMD_2022_10K (FY2022 10-K, indexed once with haiku)
- **Questions:** 7 (all FinanceBench open-source questions for this doc)
- **Index + generation model:** `claude-haiku-4-5-20251001` · **Judge:** `gpt-4o-2024-11-20`
- All four runs share the same workspace.

## Four-way comparison

| Metric | low | medium | high (agentic) | ultra (agentic + verify) |
|---|---|---|---|---|
| Extraction rate | 100.0% | 100.0% | 100.0% | 100.0% |
| Evidence recall | 100.0% | 100.0% | 100.0% | 100.0% |
| Answer accuracy | **100.0% (7/7)** | **100.0% (7/7)** | 85.7% (6/7) | 85.7% (6/7) |
| Hallucination rate | 0.0% | 0.0% | 14.3% (1/7) | 14.3% (1/7) |
| Verification supported | n/a | n/a | n/a | 7/7 non-refusals |

Reading the result (the inversion vs the heldout subset is informative):

- **This document is easy for the fixed pipeline**: every question's evidence sits in obvious
  sections, so `low`/`medium` go 7/7. `medium` never fired its refusal retry (no refusals).
- **The one high/ultra miss is a formula-choice disagreement, not a retrieval failure**: for the
  FY22 quick-ratio question the agent computed **1.77** from the balance sheet on page 56 while
  the gold answer computes **1.57** with a different quick-ratio variant — same page, same
  "yes, liquidity is healthy" conclusion. The `ultra` verifier correctly returned `supported`
  (every figure used is on the cited page); the judge scored it wrong because the ratio differs.
  Under the Mafin adjudication taxonomy this is a candidate for **MVA** (multiple valid
  approaches); it is left auto-labeled `NAL` here — relabel via
  `pindex eval --rescore` after human review rather than self-grading.
- Takeaway for the scoreboard: effort levels are not strictly ordered per document — `high`
  trades the fixed pipeline's section heuristics for autonomous navigation, which wins on hard
  documents (heldout: +33 points) and can lose a derivation coin-flip on easy ones.

## Funnel outputs (verbatim)

`--effort low` and `--effort medium` (identical):

```
=== stage funnel (scored 7/7) ===
  extraction (evidence in extracted text): 100.0%
  retrieval  (cited page holds evidence):  100.0%   [evidence-recall]
  answer     (judged correct):             100.0%   [answer-accuracy]
  hallucination (confident-wrong):           0.0%
  (page-number recall 0.0%, alignment-sensitive)
```

`--effort high` and `--effort ultra` (identical funnels):

```
=== stage funnel (scored 7/7) ===
  extraction (evidence in extracted text): 100.0%
  retrieval  (cited page holds evidence):  100.0%   [evidence-recall]
  answer     (judged correct):              85.7%   [answer-accuracy]
  hallucination (confident-wrong):          14.3%
  (page-number recall 14.3%, alignment-sensitive)
```

## Contents & reproduction

Same curation and layout as the heldout subset (see its README): per-effort `summary.json`,
Mafin-compatible `result_<model>.json`, human-eval CSV, per-question `answers/*.json`, and the
text-stripped tree. No `questions.jsonl`, no PDFs, no raw page text (FinanceBench is
CC-BY-NC-4.0; see the license note in `../2026-06-10_haiku-4-5_heldout-subset/README.md`).

```sh
go build -o /tmp/pindex-bench ./cmd/pindex
cd /tmp/run && curl -sSL -o fb.jsonl https://raw.githubusercontent.com/patronus-ai/financebench/main/data/financebench_open_source.jsonl
jq -c 'select(.doc_name=="AMD_2022_10K")' fb.jsonl > amd.jsonl
curl -sSL -o AMD_2022_10K.pdf https://raw.githubusercontent.com/patronus-ai/financebench/main/pdfs/AMD_2022_10K.pdf
/tmp/pindex-bench index AMD_2022_10K.pdf --workspace ws --model claude-haiku-4-5-20251001
for eff in low medium high ultra; do
  /tmp/pindex-bench eval --questions amd.jsonl --workspace ws \
    --model claude-haiku-4-5-20251001 --judge-model gpt-4o-2024-11-20 \
    --effort $eff --out out-$eff
done
```
