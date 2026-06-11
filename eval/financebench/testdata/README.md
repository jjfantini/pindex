# Dataset cache & diagnostic set

This directory is the **gitignored cache** for the FinanceBench dataset (CC-BY-NC-4.0 — fetched
at eval time, never committed): `fb.jsonl` (all 150 questions), `pdfs/` (source documents), and
`ws/` (the indexed benchmark workspace). Populate it with:

```sh
../fetch.sh                    # questions only
../fetch.sh AMD_2022_10K       # questions + one PDF
../fetch.sh --all              # the full 84-document set (hundreds of MB)
```

For benchmark runs you normally don't call `fetch.sh` directly —
`../bench.sh <DOC_NAME>` fetches, indexes, evals every effort level, and folds the results into
[`../results/`](../results/README.md) in one command.

## `diagnostic_set.json`

15 short FinanceBench documents (4–72 pages, 29 questions) used during development. The `train`
list marks documents that were used while **tuning prompts** — that is its only meaning (nothing
is trained; this is contamination tracking, not an ML split). Tune against `train`, sanity-check
on `heldout`, and flag any `train` doc that gets added to the public benchmark. The file stores
only doc names; the questions themselves are fetched.

The eval's stage funnel (extraction / retrieval / answer / hallucination) localizes where
accuracy is lost — see the eval guide on the docs site for reading it.
