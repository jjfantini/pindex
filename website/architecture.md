---
title: Architecture
---

# Architecture

pindex is a single-process Go binary: thin cobra commands over a small set of packages, with all LLM access behind one seam.

## Data flow

```
PDF ──> extract ──> index ──────> store ──────> retrieve ──> ask
        (pages)     (LLM:         (catalog.db   (structure,  (LLM: select
                    structure      + docs/      page          pages, then
                    gen → tree     <id>.json)   content)      answer +
                    → enrich)                                 citations)
                       │                                        │
                       └────────── llm (Provider seam) ─────────┘
                            resilience + prompt-hash cache
```

## Package map

| Package | Role |
|---|---|
| `cmd/pindex` | cobra root + subcommands `index`, `ask`, `eval`, `extract` |
| `internal/config` | typed YAML config with defaults and validation |
| `internal/extract` | pluggable `Extractor`: mupdf / poppler / purego |
| `internal/tree` | pure tree ops — list→tree, page ranges, JSON renderer; no LLM |
| `internal/prompts` | the ported prompts plus their typed output schemas |
| `internal/llm` | `Provider` seam, resilience wrapper, prompt-hash cache, structured output |
| `internal/index` | structure generation → tree assembly → optional enrichment |
| `internal/store` | SQLite catalog (`catalog.db`) + per-doc JSON blobs |
| `internal/retrieve` | `get_document` / `get_structure` / `get_page_content` |
| `internal/ask` | tree search → page selection → answer with citations |
| `internal/pipeline` | batch indexing over directories, resumable |
| `internal/envfile` | `.env` loading (overrides the process environment) |

The dividing rule: **pure logic lives in `tree/` and stays near-100% tested; `index/` holds the thin LLM steps.** If a function would mix both, it gets split.

## The seams

Four small interfaces keep everything swappable and testable:

- **`Provider`** — one interface over OpenAI and Anthropic. The adapters are hand-rolled HTTP, not vendor SDKs: requests are simple enough that two SDKs would add dependency weight and hide the retry/caching behavior pindex needs to control. Routing is by model name (`claude*` → Anthropic, everything else → OpenAI).
- **`Extractor`** — mupdf (cgo, highest fidelity), poppler (shells out to `pdftotext`), purego (pure Go, fully static builds).
- **`Renderer`** — JSON is canonical on disk; any alternate tree rendering goes behind this seam.
- **`Cache`** — the prompt-hash response cache, and the same seam used to test LLM-calling code deterministically with mocks and cassettes.

## The resilience envelope

Indexing is LLM-bound — a 100-page PDF is 100–200 LLM calls — so the rewrite's engineering value is the envelope around those calls, not Go speed:

- **Bounded concurrency** — errgroup limits at both levels: across documents (`--concurrency`, default 4) and within a document (builder default 8). Page extraction is serialized per document (a go-fitz handle constraint); parallelism is across documents.
- **Rate limiting** — `--rpm` caps LLM requests per minute.
- **Retries + circuit breaker** — up to 8 attempts with backoff (1s base, 60s max); the breaker trips after 5 failures with a 30s cooldown, so a dead provider breaks fast instead of draining the retry budget. Quota/billing 429s are treated as permanent.
- **Prompt-hash cache** — outermost layer; key = `sha256(model, messages, temperature)`. A hit skips the network, making re-runs and crash recovery nearly free.
- **Resumable batch indexing** — the SQLite catalog is the checkpoint: a finished document is never re-indexed unless `--force`, and one file's failure never aborts the batch.

No silent failures anywhere in the stack: errors are typed and surfaced, never swallowed into `""` or `{}`.

For the full design rationale, see [docs/PLAN.md](https://github.com/jjfantini/pindex/blob/master/docs/PLAN.md) on GitHub.
