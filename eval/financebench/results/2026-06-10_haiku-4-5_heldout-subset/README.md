# FinanceBench heldout subset: effort=low vs high vs ultra (claude-haiku-4-5)

Small public benchmark of the pindex retrieval effort levels on the **heldout** split of the
pindex accuracy diagnostic set (`eval/financebench/testdata/diagnostic_set.json`). This is a
**subset** test, not the full 150-question FinanceBench run.

Effort semantics (current):

- **`low`** — fixed pipeline: tree-search → select a tight page range → answer with citations.
- **`high`** — **agentic loop**: the model navigates the text-stripped tree with JSON actions
  (`get_pages` to fetch tight ranges, `answer` to finish), up to 8 turns. Grounding is enforced
  mechanically: an answer issued before any pages were read is redirected back into the loop
  (the model must fetch and read page text before finishing — it cannot answer from the tree
  summaries alone).
- **`ultra`** — agentic loop **plus** an answer-verification pass: every non-refusal answer is
  fact-checked against its cited pages; on "unsupported" the same conversation continues with the
  fact-checker's findings and the corrected answer is re-verified.

> Note: an earlier revision of this directory benchmarked a previous meaning of `--effort high`
> (low pipeline + verification pass). That semantics now lives at `ultra` (on top of the agentic
> loop), and the `high/` results here are from the new agentic-loop implementation. The `low/`
> results are unchanged from the original run (low semantics did not change).

- **Date:** 2026-06-10
- **Docs (5, heldout split, indexed once with haiku):** BESTBUY_2024Q2_10Q,
  JOHNSON_JOHNSON_2023_8K_dated-2023-08-30, JOHNSON_JOHNSON_2023Q2_EARNINGS,
  FOOTLOCKER_2022_8K_dated-2022-05-20, FOOTLOCKER_2022_8K_dated_2022-08-19
- **Questions:** 9 (all FinanceBench open-source questions whose `doc_name` is in the heldout
  split — no trimming needed: 5 docs / 9 questions is within the ≤5-doc / ≤12-question budget)
- **Index + generation model:** `claude-haiku-4-5-20251001` (full-haiku: indexing, retrieval, and answering)
- **Judge model:** `gpt-4o-2024-11-20` (Mafin 2.5's exact judge, for comparability)
- **All three runs share the same workspace** (each doc indexed exactly once).

## Three-way comparison

| Metric | effort=low | effort=high (agentic) | effort=ultra (agentic + verify) |
|---|---|---|---|
| Extraction rate | 100.0% | 100.0% | 100.0% |
| Evidence recall (retrieval) | 77.8% | 88.9% | 88.9% |
| Answer accuracy (judged) | 66.7% (6/9) | **100.0% (9/9)** | **100.0% (9/9)** |
| Hallucination rate (confident-wrong) | 33.3% (3/9) | **0.0%** | **0.0%** |
| Avg agentic steps / question | n/a (fixed pipeline) | 2.44 (max 3 of 8) | 2.78 (max 4) |
| Verification: supported | n/a (no pass) | n/a (no pass) | 9 |
| Verification: unsupported | n/a (no pass) | n/a (no pass) | 0 |

Reading the deltas:

- **The agentic loop with enforced grounding fixes both failure modes of `low`**: evidence recall
  rises 77.8% → 88.9%, and all three of `low`'s confident-wrong answers become correct ones. The
  enforcement matters: in an earlier revision of this run *without* the read-before-answering
  redirect, haiku answered from the tree summaries alone on 5 of 9 questions and `high` actually
  scored **below** `low` (55.6%, with hallucinations). Forcing the agent to read pages before
  finishing took the same model, same workspace, and same questions to 9/9.
- **`ultra`'s verification pass found nothing to correct here** — all 9 answers came back
  `supported` on the first check. Its value on this subset is the audit trail (every answer
  carries a fact-check verdict) rather than extra accuracy; on harder corpora the corrective
  continuation is the safety net for whatever slips past `high`.
- An honest abstention is never auto-counted as a hallucination, and verification is skipped for
  refusals by design — this run simply had no refusals.

## Funnel outputs (verbatim)

`--effort low`:

```
=== stage funnel (scored 9/9) ===
  extraction (evidence in extracted text): 100.0%
  retrieval  (cited page holds evidence):   77.8%   [evidence-recall]
  answer     (judged correct):              66.7%   [answer-accuracy]
  hallucination (confident-wrong):          33.3%
  (page-number recall 0.0%, alignment-sensitive)
```

`--effort high`:

```
=== stage funnel (scored 9/9) ===
  extraction (evidence in extracted text): 100.0%
  retrieval  (cited page holds evidence):   88.9%   [evidence-recall]
  answer     (judged correct):             100.0%   [answer-accuracy]
  hallucination (confident-wrong):           0.0%
  (page-number recall 0.0%, alignment-sensitive)
```

`--effort ultra`:

```
=== stage funnel (scored 9/9) ===
  extraction (evidence in extracted text): 100.0%
  retrieval  (cited page holds evidence):   88.9%   [evidence-recall]
  answer     (judged correct):             100.0%   [answer-accuracy]
  hallucination (confident-wrong):           0.0%
  (page-number recall 11.1%, alignment-sensitive)
```

## Contents

- `low/`, `high/`, `ultra/` — per-effort eval output: `summary.json`,
  `result_claude-haiku-4-5-20251001.json` (Mafin-compatible),
  `human_evaluations/claude-haiku-4-5-20251001.csv`, per-doc `answers/*.json` records, and
  per-doc `*_pindex.json` trees (text-stripped — exported **without** `--include-pages`, so they
  contain section titles/summaries/page ranges only, no raw document text).
- `questions.jsonl` and the source PDFs are deliberately **not** committed (see License below).
- `high/` and `ultra/` `answers/*.json` records carry `steps` (agentic turns) and
  `selected_pages` (the union of every page the agent fetched — always non-empty, since the
  agent must read pages before answering). `ultra` records additionally carry the
  `verification` verdict on non-refusal answers; `high` and `low` records never do.

## Reproduce

```sh
go build -o /tmp/pindex-bench ./cmd/pindex
mkdir -p /tmp/pindex-bench-run/pdfs && cd /tmp/pindex-bench-run

# Questions: filter the FinanceBench open-source JSONL to the heldout docs
curl -sSL -o fb.jsonl https://raw.githubusercontent.com/patronus-ai/financebench/main/data/financebench_open_source.jsonl
jq -c 'select(.doc_name as $d | ["BESTBUY_2024Q2_10Q","JOHNSON_JOHNSON_2023_8K_dated-2023-08-30","JOHNSON_JOHNSON_2023Q2_EARNINGS","FOOTLOCKER_2022_8K_dated-2022-05-20","FOOTLOCKER_2022_8K_dated_2022-08-19"] | index($d))' fb.jsonl > heldout.jsonl

# PDFs
for d in BESTBUY_2024Q2_10Q JOHNSON_JOHNSON_2023_8K_dated-2023-08-30 JOHNSON_JOHNSON_2023Q2_EARNINGS FOOTLOCKER_2022_8K_dated-2022-05-20 FOOTLOCKER_2022_8K_dated_2022-08-19; do
  curl -sSL -o "pdfs/$d.pdf" "https://raw.githubusercontent.com/patronus-ai/financebench/main/pdfs/$d.pdf"
done

# Index with haiku, one doc per command (resumable; rerun on timeout)
for d in pdfs/*.pdf; do
  /tmp/pindex-bench index "$d" --workspace ws --model claude-haiku-4-5-20251001 --env-file <repo>/.env
done

# Eval three times on the same workspace
/tmp/pindex-bench eval --questions heldout.jsonl --workspace ws \
  --model claude-haiku-4-5-20251001 --judge-model gpt-4o-2024-11-20 \
  --effort low   --out out-low   --env-file <repo>/.env
/tmp/pindex-bench eval --questions heldout.jsonl --workspace ws \
  --model claude-haiku-4-5-20251001 --judge-model gpt-4o-2024-11-20 \
  --effort high  --out out-high  --env-file <repo>/.env
/tmp/pindex-bench eval --questions heldout.jsonl --workspace ws \
  --model claude-haiku-4-5-20251001 --judge-model gpt-4o-2024-11-20 \
  --effort ultra --out out-ultra --env-file <repo>/.env
```

## License & attribution

Questions, gold answers, and source PDFs come from
[FinanceBench](https://github.com/patronus-ai/financebench) (Patronus AI;
[arXiv:2311.11944](https://arxiv.org/abs/2311.11944)). The dataset is licensed
**CC-BY-NC-4.0** (per the [Hugging Face dataset card](https://huggingface.co/datasets/PatronusAI/financebench);
the GitHub repository itself carries no LICENSE file). Accordingly, the raw question set
(`questions.jsonl`) and PDFs are never committed to this repository. The
`result_claude-haiku-4-5-20251001.json` files contain question text and gold answers for the 9
evaluated questions — the same per-question result format that upstream
[VectifyAI/Mafin2.5-FinanceBench](https://github.com/VectifyAI/Mafin2.5-FinanceBench) publishes —
and are included here with attribution. **These results are published for research /
non-commercial purposes only.**
