---
title: Effort Levels
---

# Effort levels

`pindex ask` (and `pindex eval`) take an `--effort` flag that dials how much reasoning is spent on each question:

```sh
pindex ask "What was total revenue in fiscal 2022?" --effort high
```

| Effort | What it does | Typical LLM calls / question |
|---|---|---|
| `low` (default) | Single pass: select pages → fetch → answer. | ~2 |
| `medium` | `low` + one broader re-select when the answer is a refusal. | ~2–4 |
| `high` | Agentic loop: the model navigates the tree itself, fetching pages turn by turn until it can answer. | ~3–9 |
| `ultra` | `high` + a verification pass on the final answer, with one corrective continuation if it fails. | ~5–12 |

## What each level does

**`low`** — the fast default. The LLM reads the tree structure and picks a tight page selector (like `"5-7,12"`), pindex fetches just those pages, and the LLM answers strictly from them with page citations. Two LLM calls, done.

**`medium`** — everything `low` does. Then, if the answer is an honest refusal ("I can't find that in the provided pages"), the LLM selects a *different* set of pages once, and pindex re-answers from the combined set. This recovers a wrong or too-narrow first selection.

**`high`** — replaces the fixed select-then-answer pipeline with an **agentic loop**, mirroring the agentic tree traversal of the original [PageIndex](https://github.com/VectifyAI/PageIndex) demo — vectorless, no embeddings. The model is handed the document's metadata and tree structure and acts turn by turn: fetch a tight page range (`get_pages`), read it, fetch more if the evidence points elsewhere, then answer — grounded only in the pages it actually read. The loop runs up to 8 turns; if the model still hasn't answered, one final turn forces an answer from what it has read (or an honest "not found"). If even that fails, you get a typed error — never a silent empty answer.

**`ultra`** — everything `high` does. Then, if the final answer is *not* a refusal, a verification pass fact-checks it: pindex fetches the pages the answer cited (falling back to everything the agent read when no citations parsed) and asks the LLM whether **every claim** is directly supported by them.

- **Supported** → the answer is returned, marked `supported`.
- **Unsupported** → one corrective continuation: the fact-checker's findings are fed back into the *same* agentic conversation and the agent gets up to 3 more turns to re-examine the document and produce a corrected answer, which is re-verified. If it verifies, you get it (marked `supported`). If not, you get the *original* answer marked `unsupported` — surfaced, never silently replaced. An `ultra` answer is never silently wrong: it is either verified or flagged.

Refusals skip verification — an honest "not found" is not a hallucination.

## What you see

The answer goes to stdout. Citations go to stderr at every level:

```
cited pages: [12 13 14]  (doc: report.pdf)
```

At `ultra`, a verdict line follows:

```
verification: supported
```

or, when the corrective continuation could not produce a verified answer:

```
verification: UNSUPPORTED — treat with caution (missing support for some claims)
```

The verdict also lands in `--out` exports and eval results as a `"verification"` key (omitted when verification did not run). At `high`/`ultra` the records also carry a `"steps"` key — the number of agentic turns used.

## When to use which

- **`low`** — the cheap default: interactive exploration, bulk evals, questions where a wrong answer is cheap to spot.
- **`medium`** — when the document is large or oddly structured and the first page selection sometimes misses.
- **`high`** — hard questions in big documents: multi-hop questions, scattered evidence, or documents where the right section only becomes clear after reading another one.
- **`ultra`** — defensible answers: financial figures, compliance, anything you would have to stand behind. It tells you explicitly when an answer could not be grounded in its sources.

## On FinanceBench so far

The repo accumulates scores on the open-source FinanceBench set via `./eval/financebench/bench.sh`
(see [`eval/financebench/results/README.md`](https://github.com/jjfantini/pindex/blob/develop/eval/financebench/results/README.md)).
With **Claude Haiku 4.5** (`claude-haiku-4-5-20251001`) and a **gpt-4o-2024-11-20** judge, on
**18 questions across 7 documents** (2026-06-11):

| Effort | Raw answer accuracy | Adjusted accuracy |
|---|---|---|
| `low` / `medium` | 83.33% | 100.0% |
| `high` / `ultra` | 94.44% | 100.0% |

The jump from `low`/`medium` to `high`/`ultra` is the agentic loop finding evidence that fixed
page selection misses. `medium` has matched `low` on every doc benchmarked so far — all misses were
confident-wrong, not refusals, so the refusal retry never fired.
