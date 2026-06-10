---
title: Index a PDF
---

# Index a PDF

## Goal

Build a tree index from a single PDF, inspect the result, and see why re-runs are nearly free.

## Prerequisites

- `pindex` installed ([Installation](/installation))
- An API key in `.env` in your working directory (`OPENAI_API_KEY` or `ANTHROPIC_API_KEY`)
- A PDF, e.g. `report.pdf`

## Steps

**1. Index the PDF.**

```sh
pindex index report.pdf
```

The tree index prints to stdout as indented JSON. By default the document is also saved to the workspace at `.pindex/workspace`.

**2. Inspect the result.**

Alongside the stdout JSON, a browsable export is written to `<workspace>/pindex/<doc>/<doc>_pindex.json`. To include the raw page text in that export:

```sh
pindex index report.pdf --include-raw-text
```

**3. Re-run it.**

```sh
pindex index report.pdf
```

Every LLM response is cached by prompt hash in `.pindex/cache`, so the re-run hits the cache instead of the network and finishes almost instantly. The same mechanism makes crash recovery cheap — interrupted runs resume from cached responses.

## Options worth knowing

- `--model gpt-4o-2024-11-20` — pick the indexing LLM (default comes from config).
- `--toc-page-limit N` — pindex first looks for a table of contents in the leading pages (default 10, on by default) and uses it as a fast path; `--toc-page-limit 0` disables TOC detection and forces full structure generation.
- `--workspace ""` — print the tree only, persist nothing.
- `--rpm 60` — cap LLM requests per minute.

::: warning Cost
Indexing is LLM-bound: a 100-page PDF is roughly 100–200 LLM calls. The cache only saves you on repeats.
:::

## What you should see

- stdout: the tree index as JSON (sections, page ranges, summaries).
- stderr: `saved to .pindex/workspace (doc id <16-hex-id>)` and `wrote tree to .pindex/workspace/pindex/<doc>/<doc>_pindex.json`.

To see a finished run without spending tokens — the input PDF, the tree index it produced, and a real Q&A transcript — see [`examples/`](https://github.com/jjfantini/pindex/tree/master/examples) in the repo.

Next: [Ask questions](/guides/ask-questions).
