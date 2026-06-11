# FinanceBench running scoreboard (incremental, one document at a time)

pindex is benchmarked against the full [FinanceBench](https://github.com/patronus-ai/financebench)
open-source set (150 questions, 84 documents) **incrementally**: one document (or small subset)
per run, indexed once, evaluated at every effort level, committed here. The final accuracy per
(model, effort) is the **micro-average** over all committed runs — pool every scored question
and compute `sum(correct) / sum(scored)`. This is valid because every run uses the same
generation model, the same judge (`gpt-4o-2024-11-20`, Mafin 2.5's judge), and no document
appears in two runs.

## Scoreboard — claude-haiku-4-5 (generation + indexing), gpt-4o judge

**Coverage: 16/150 questions (6/84 documents)** across
[`2026-06-10_haiku-4-5_heldout-subset`](2026-06-10_haiku-4-5_heldout-subset/) (5 docs, 9 q) and
[`2026-06-10_haiku-4-5_AMD_2022_10K`](2026-06-10_haiku-4-5_AMD_2022_10K/) (1 doc, 7 q).

| Effort | Raw accuracy | Adjusted accuracy (AL+MVA+BE) | Evidence recall | Hallucination rate |
|---|---|---|---|---|
| low | 81.25% (13/16) | 81.25% (13/16) | 87.5% (14/16) | 18.75% (3/16) |
| medium | 81.25% (13/16) | 81.25% (13/16) | 87.5% (14/16) | 18.75% (3/16) |
| **high** | **93.75% (15/16)** | **100.0% (16/16)** | **93.75% (15/16)** | **6.25% (1/16)** |
| **ultra** | **93.75% (15/16)** | **100.0% (16/16)** | **93.75% (15/16)** | **6.25% (1/16)** |

Notes:

- **Raw** is judge-only; **adjusted** additionally counts human-adjudicated BE/MVA/SEDC
  relabels as correct — the same process behind Mafin 2.5's published 98.7%. One adjudication
  so far: the AMD quick-ratio question (`financebench_id_00222`) at high/ultra is labeled
  **MVA** (verifier-supported alternative quick-ratio formula, same conclusion as gold; the
  same model at low/medium independently used the gold formula). The three low/medium misses
  remain auto-labeled `NAL` (confident-wrong answers, not yet human-reviewed). Adjusted numbers
  are recomputable per run via `pindex eval --rescore <result_file>`.
- `medium` equals `low` so far because no run has produced a refusal (its retry never fires).
- Per-document effort ordering varies (high wins big on the hard heldout docs, low wins the easy
  AMD doc) — the pooled number is the meaningful comparison.

## Recompute from the committed files

```sh
python3 - <<'EOF'
import json, glob, collections
agg = collections.defaultdict(lambda: [0, 0])
for f in glob.glob('eval/financebench/results/*/*/summary.json'):
    s = json.load(open(f))
    k = (s['model'], s['effort'])
    agg[k][0] += round(s['answer_accuracy_raw'] * s['scored'])
    agg[k][1] += s['scored']
for (m, e), (c, n) in sorted(agg.items()):
    print(f'{m} {e:6s} {c}/{n} = {c/n:.1%}')
EOF
```

## Adding the next document

1. Pick an uncovered `doc_name` (prefer high question count; check it is not in the diagnostic
   train split if you care about tuning-contamination).
2. Index it once with the target model, eval at all four effort levels, curate per the existing
   run READMEs (no `questions.jsonl`, no PDFs, no raw page text — CC-BY-NC-4.0).
3. Add the run directory here and update the scoreboard table (or re-run the snippet above).
