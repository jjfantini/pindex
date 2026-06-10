---
title: Batch Indexing
---

# Batch Indexing

## Goal

Index a whole folder of PDFs with bounded concurrency, and resume safely after an interruption.

## Prerequisites

- `pindex` installed, API key in `.env`
- A directory of PDFs, e.g. `filings/` (discovery is recursive, `*.pdf` case-insensitive)

## Steps

**1. Index the directory.**

```sh
pindex index filings/ --concurrency 4
```

Documents are processed in parallel across files (`--concurrency`, default 4); pages within one document are always extracted sequentially. Batch mode requires a workspace — the default `.pindex/workspace` is fine, but `--workspace ""` is an error here.

**2. Watch the progress.**

Each file gets a line on stderr as it finishes:

```
[indexed] filings/AMD_2022_10K.pdf
[indexed] filings/INTEL_2022_10K.pdf
[FAILED: <error>] filings/corrupt.pdf
```

One file failing never aborts the batch.

**3. Interrupt it, then re-run.**

Kill the run (Ctrl-C) partway, then run the same command again:

```sh
pindex index filings/
```

Finished documents are checkpointed in the workspace catalog (SQLite) and are skipped — they print as `skipped`. Partially-indexed documents restart, but their completed LLM calls come back from the prompt-hash cache, so little money or time is lost. A finished doc is never re-indexed unless you pass `--force`.

## Options worth knowing

- `--rpm 60` — rate-limit LLM requests across the batch.
- `--force` — re-index documents already in the workspace.
- `--include-raw-text` — keep raw page text in the browsable exports.

## What you should see

- stderr: one `[indexed|skipped|FAILED: err] <path>` line per file, then `wrote trees to <workspace>/pindex` (and `error: N document(s) failed to index` if anything failed).
- stdout: a final summary like `indexed=10 skipped=2 failed=0 total=12`.
- Exit code is non-zero if any file failed.
