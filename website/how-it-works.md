---
title: How It Works
---

# How it works

pindex never computes an embedding. It builds a **tree index** — a hierarchical outline of your PDF with page ranges and optional summaries — and answers questions by having an LLM walk that tree, read a tight page range, and cite the pages it used.

This page explains exactly what `pindex index` and `pindex ask` do to your disk and your API bill.

## Indexing: PDF in, tree out

`pindex index report.pdf` runs this pipeline:

1. **Extract** — the extractor (default: `mupdf`) pulls plain text from every page. No LLM calls yet.
2. **Find the structure** — first a TOC fast path: the LLM scans the leading pages (default 10, `--toc-page-limit`) for a table of contents with page numbers, recovers the printed-to-physical page offset, and verifies each section title against its page. If there is no usable TOC, pindex falls back to full structure generation: the LLM reads token-bounded page groups and emits the section list.
3. **Build the tree** — pure Go code turns the flat section list into a nested tree with page ranges. Oversized leaf nodes (more than 10 pages *and* roughly 20k tokens) are recursively re-indexed, then node IDs are written.
4. **Enrich (optional)** — with `if_add_node_summary: true` (the default) each node gets an LLM-written summary. Tiny nodes (under ~200 tokens) keep their text verbatim — no LLM call.
5. **Store** — the document (pages + tree) is saved to the workspace.

A finished index looks like this:

```
AMD_2022_10K.pdf
├── Part I                       pages 4–33
│   ├── Item 1. Business         pages 4–18
│   └── Item 1A. Risk Factors    pages 19–33
├── Part II                      pages 34–88
│   ├── Item 7. MD&A             pages 34–52
│   └── Item 8. Financials       pages 53–88
└── ...                          (each node: title, page range, summary)
```

## Where data lives

Everything is under `.pindex/` in your current directory (configurable via flags):

| Path | What it is | Safe to delete? |
|---|---|---|
| `.pindex/workspace/catalog.db` | SQLite catalog: one row per indexed doc (id, name, page count, indexed-at). Used for listing, lookup, and batch resume. | Only with the whole workspace — deleting it means re-indexing everything. |
| `.pindex/workspace/docs/<id>.json` | The full document: raw page text plus the tree. One inspectable JSON file per doc. | Same — this *is* your index. |
| `.pindex/workspace/pindex/<doc>/<doc>_pindex.json` | Browsable export of the tree (text-stripped unless `--include-raw-text`). | Yes — regenerated on the next index run. |
| `.pindex/cache/` | Prompt-hash response cache: one JSON file per LLM response. | Yes — you just pay for those LLM calls again on the next run. |

The document ID is the first 16 hex characters of `sha256(absolute file path)` — stable per path, so moving or renaming a PDF changes its ID and it will be re-indexed.

## Asking: tree search, then a tight page range

`pindex ask "What was 2022 revenue?"` does four steps:

1. **Get the structure** — the tree with all page text stripped, rendered as JSON. This token-cheap view is what the LLM reasons over.
2. **Select pages** — the LLM picks a page selector like `"5-7,12"` from the tree.
3. **Fetch** — pindex pulls just those pages' text from the stored document.
4. **Answer** — the LLM answers from those pages and reports which it actually used.

The answer goes to stdout; citations go to stderr:

```
cited pages: [12 13 14]  (doc: AMD_2022_10K.pdf)
```

With `--effort medium` or higher, an honest "I can't find that" triggers one retry with a different page set.

## What costs LLM calls — and what doesn't

Indexing is **LLM-bound**: a 100-page PDF is roughly 100–200 LLM calls (structure generation, per-section checks, per-node summaries). Go does not make the model faster; the engineering is in not paying twice:

- **Prompt-hash cache** (`--cache-dir`, default `.pindex/cache`) — every response is cached under `sha256(model, messages, temperature)`. A hit skips the network entirely, so re-runs and crash recovery are nearly free.
- **Resumable batches** — `pindex index <dir>` skips any document already in the catalog unless you pass `--force`. A crash mid-batch costs only the in-flight document.
- **Asking is cheap** — one structure read, one page selection, one answer (plus at most one retry at higher effort).

Pure tree operations (nesting, page ranges, splitting, IDs) never touch the network.

::: tip
Same question, same model, same pages → the second `ask` is a cache hit and costs nothing.
:::
