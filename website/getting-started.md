---
title: Getting Started
---

# Getting Started

Index your first PDF and ask it a question — about 5 minutes.

## 1. Install

```sh
brew install jjfantini/humbl/pindex
```

Other paths (release binaries, `go install`, source) are on the [installation page](/installation).

## 2. Set an API key

pindex talks to a live LLM provider. Create a `.env` file in your working directory:

```sh
echo 'OPENAI_API_KEY=sk-...' > .env
```

Or use `ANTHROPIC_API_KEY` and pass a Claude model (`--model claude-...`) — the provider is routed by model name. The default model is `gpt-4o-2024-11-20`.

::: warning
Values in `.env` **override** the process environment — an updated `.env` always wins. Use `--env-file` to point at a different file.
:::

## 3. Index a PDF

```sh
pindex index report.pdf
```

This extracts the pages, has the LLM build a hierarchical tree index (sections, page ranges, summaries), and prints the tree as JSON to stdout. On stderr you'll see where it was saved:

```
saved to .pindex/workspace (doc id 3fa1c2d4e5b6a798)
wrote tree to .pindex/workspace/pindex/report/report_pindex.json
```

Indexing is LLM-bound: a 100-page PDF means roughly 100–200 LLM calls, so expect minutes, not seconds. Responses are cached in `.pindex/cache`, so re-runs and crash recovery are nearly free.

## 4. Ask a question

```sh
pindex ask "What was total revenue in fiscal 2023?"
```

pindex walks the tree index, selects a tight page range, reads only those pages, and answers. The answer goes to stdout; the citation line goes to stderr:

```
Total revenue in fiscal 2023 was $10.2 billion, up 4% year over year.
cited pages: [42 43]  (doc: report.pdf)
```

With one indexed document, `ask` targets it automatically. With several, pick one with `--doc` (a document id or the original file path).

## Where your data went

Everything lives under the **workspace**, `.pindex/workspace` by default:

- `catalog.db` — SQLite catalog of indexed documents
- `docs/<id>.json` — the full tree index per document, including page text
- `pindex/<doc>/<doc>_pindex.json` — a browsable export of the tree

The prompt cache lives separately at `.pindex/cache`. See [how it works](/how-it-works) for the full data flow.
