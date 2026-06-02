# pindex

A fast, **vectorless** reasoning-RAG engine in Go — a rewrite of
[PageIndex](https://github.com/VectifyAI/PageIndex).

pindex builds a hierarchical **tree index** from PDFs/Markdown and answers questions by having an
LLM **reason over that structure** — no embeddings, no fixed chunking, and answers cite specific
pages. Retrieval walks the tree (section summaries → relevant branch → tight page range) instead of
ranking vector similarities.

> **Status: early development (v1 in progress).** The CLI subcommands are scaffolding stubs today;
> they are filled in phase by phase. See [`docs/PLAN.md`](docs/PLAN.md) for the build plan and
> [`roadmap.md`](roadmap.md) for what's deferred to v2.

## Why a rewrite

Indexing is LLM-bound, so Go doesn't make the model faster — it buys the engineering envelope the
Python original lacks: bounded concurrency + rate limiting, **resumable indexing** (never re-index
a finished doc after a crash), a **prompt-hash response cache** (cheap re-runs; doesn't burn the
retry budget), graceful **degradation** when a provider stalls, and a single binary an LLM can
drive token-efficiently.

## Planned CLI

```text
pindex index <path>      # build a tree index for a PDF/Markdown file or directory
pindex ask <question>    # answer by reasoning over indexed trees, citing pages
pindex eval              # run the FinanceBench evaluation harness
pindex extract <pdf>     # debug: dump per-page extracted text
```

## Build

```sh
go build ./...
go test ./...
```

## License

TBD.
