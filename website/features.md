---
title: Features
---

# Features

## What pindex does

- **Tree index from any PDF.** `pindex index` builds a hierarchical tree of sections with page ranges. A TOC fast path detects and verifies a printed table of contents (with repair); TOC-less documents fall back to LLM structure generation.
- **Question answering with citations.** `pindex ask` walks the tree, selects a tight page range, answers from those pages, and prints `cited pages: [...]` to stderr.
- **Resumable batch indexing.** Point `pindex index` at a directory: documents already in the workspace catalog are skipped, one file's failure never aborts the batch, and `--concurrency` bounds parallelism across documents.
- **Prompt-hash response cache.** Every LLM response is cached by a hash of {model, messages, temperature} in `.pindex/cache`. Re-running after a crash replays cached responses instead of hitting the network.
- **Resilience envelope.** Retries with backoff (8 attempts, 1s–60s), a circuit breaker (trips after 5 failures, 30s cooldown), and an optional rate limiter (`--rpm`). Quota/billing 429s fail fast instead of draining the retry budget.
- **Pluggable extractors.** `mupdf` (default, highest fidelity), `purego` (pure Go, static builds), `poppler` (shells out to `pdftotext -layout`). See [Choosing an extractor](/guides/choosing-an-extractor).
- **Two providers, one seam.** Model names containing `claude` route to Anthropic; everything else goes to OpenAI. No vendor SDKs — hand-rolled HTTP adapters behind a `Provider` interface.
- **Built-in evaluation.** `pindex eval` runs FinanceBench questions over a pre-indexed workspace and reports a stage funnel: extraction, evidence recall, answer accuracy (raw and adjusted), hallucination rate. An accumulating open-source benchmark lives in [`eval/financebench/results/`](https://github.com/jjfantini/pindex/tree/develop/eval/financebench/results) — as of 2026-06-11, **7/84 docs and 18/150 questions** scored for `claude-haiku-4-5-20251001` at all four effort levels (100% adjusted accuracy after human adjudication; 94.44% raw at `high`/`ultra` vs 83.33% at `low`/`medium`).

## Why this beats vector RAG

**Chunking destroys document structure; a tree preserves it.** Vector RAG slices a document into fixed-size chunks, cutting through tables, sections, and multi-page arguments. pindex keeps the document whole: sections, their hierarchy, and their page ranges. Retrieval works the way a person does — scan the table of contents, descend into the relevant branch, read a tight page range.

**Embeddings are opaque; reasoning over a table of contents is traceable.** When a vector search returns the wrong chunk, there is nothing to inspect — a cosine score is not an explanation. In pindex, retrieval is an LLM reading section titles and summaries and choosing pages. The selected pages, the reasoning, and the final citations are all visible (`pindex ask --out` writes them to disk).

**No vector DB to host.** No embedding model, no index server, no sync job. The entire state is a workspace directory: a SQLite catalog plus one inspectable JSON file per document.

**Citations are exact pages.** Answers cite the pages they used, and the eval harness measures evidence recall — whether the cited pages actually contain the gold evidence. pindex is accumulating scores on the full [FinanceBench](https://github.com/patronus-ai/financebench) open-source set (see the repo's `eval/financebench/results/README.md` scoreboard): with Claude Haiku 4.5, the agentic `high`/`ultra` effort levels reach **94.44% raw** answer accuracy on 18 questions so far, vs **83.33%** for fixed select-then-answer at `low`/`medium` — same trees, same judge. Early smoke tests on a single earnings release (`ULTABEAUTY_2023Q4_EARNINGS`, 4 questions) showed the same pattern: gpt-4o scored 50% where gpt-4o-mini scored 0% on an identical index — the pipeline is sound and model-bound. (PageIndex's published 98.7% on FinanceBench/Mafin 2.5 used GPT-4o/DeepSeek with human-adjusted scoring.)

**Re-runs are nearly free.** You pay for indexing once: the prompt-hash cache means a crashed or repeated run replays cached responses, and the SQLite checkpoint means a batch never re-indexes a finished document.

## Tradeoffs

Honest costs of the approach:

- **LLM-bound latency and cost.** Indexing a 100-page PDF is roughly 100–200 LLM calls; asking a question is several more. Go does not make the model faster, and there is no offline mode — live OpenAI or Anthropic API keys are required.
- **PDF-only today.** Markdown ingestion is deferred; the vision extractor for scanned pages is a stub. The pure-Go extractor trades table fidelity for portability, and the default high-fidelity build is platform-specific (cgo).
- **Per-document scale in v1.** pindex is single-process and answers questions against one document at a time. Corpus-scale routing across many documents is a v2 concern.
