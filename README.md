# pindex

A fast, **vectorless** reasoning-RAG engine in Go — a rewrite of
[PageIndex](https://github.com/VectifyAI/PageIndex).

pindex builds a hierarchical **tree index** from a PDF and answers questions by having an LLM
**reason over that structure** — no embeddings, no fixed chunking. Retrieval walks the tree
(section summaries → relevant branch → tight page range) and answers cite the specific pages
they used.

> **Status: working v1.** `index`, `ask`, and `eval` all run live against OpenAI and Anthropic.
> Validated end-to-end on a real [FinanceBench](https://github.com/patronus-ai/financebench)
> earnings release (see [results](#financebench)). What's deferred is in [`roadmap.md`](roadmap.md);
> the design is in [`docs/PLAN.md`](docs/PLAN.md).

## Why vectorless

Indexing is LLM-bound, so Go doesn't make the model faster — it buys the engineering envelope the
Python original lacks: **bounded concurrency + rate limiting**, **resumable batch indexing** (never
re-index a finished doc), a **prompt-hash response cache** (re-runs and crash recovery are nearly
free; it doesn't burn the retry budget), graceful **degradation** when a provider stalls (a dead
account circuit-breaks instead of draining retries), and a single binary an LLM can drive.

## Install

```sh
go build -o pindex ./cmd/pindex      # default build uses go-fitz/MuPDF (needs a C toolchain)
go build -tags purego ./cmd/pindex   # not yet a separate tag; see Extractors for the static path
```

The default **MuPDF** extractor (`gen2brain/go-fitz`) links a bundled static libmupdf via cgo — no
system MuPDF needed, but you need a C compiler. For a fully-static (`CGO_ENABLED=0`) binary, build
with the pure-Go extractor and set `extractor: purego` (see [Extractors](#extractors)).

## API keys

`index`/`ask`/`eval` call a live LLM. Provide keys via the environment or a gitignored `.env`
(copy [`.env.example`](.env.example)). The `.env` **overrides** the process environment, so it's
the reliable channel:

```sh
cp .env.example .env   # then fill in OPENAI_API_KEY and/or ANTHROPIC_API_KEY
```

The model name picks the provider: `claude*` → Anthropic, otherwise OpenAI.

## Quickstart

```sh
# Index a PDF into a workspace (prints the tree; persists to .pindex/workspace)
pindex index report.pdf --model gpt-4o

# Ask a question — pindex tree-searches, fetches tight pages, answers with citations
pindex ask "What was full-year revenue?" --model gpt-4o
#  → "Full-year revenue was $10.2B."   cited pages: [7]

# Batch-index a whole directory, in parallel, resumable (skips already-indexed docs)
pindex index ./filings/ --model gpt-4o-mini --concurrency 8

# Debug: dump per-page extracted text
pindex extract report.pdf --backend mupdf
```

## Commands

| Command | What it does |
|---------|--------------|
| `pindex index <pdf\|dir>` | Build a tree index for a file (prints tree) or a directory (batch, resumable). |
| `pindex ask <question>` | Reason over an indexed doc's tree, fetch pages, answer with citations. |
| `pindex eval` | Run the FinanceBench harness over a pre-indexed workspace. |
| `pindex extract <pdf>` | Dump per-page extracted text (extractor debugging). |

Common flags: `--model`, `--workspace`, `--cache-dir`, `--env-file`, `--config`.

## Configuration

A YAML config (via `--config`) overrides the built-in defaults, which mirror PageIndex:

```yaml
model: gpt-4o-2024-11-20      # or claude-... ; routes provider by name
extractor: mupdf              # mupdf | poppler | purego
toc_check_page_num: 20
max_page_num_each_node: 10
max_token_num_each_node: 20000
if_add_node_id: true
if_add_node_summary: true
if_add_doc_description: false
if_add_node_text: false
```

## Extractors

Pluggable via `extractor:` (or `--backend`):

| Backend | Engine | Notes |
|---------|--------|-------|
| `mupdf` *(default)* | go-fitz / MuPDF | Highest fidelity. Needs a cgo build (bundled static libmupdf). |
| `poppler` | `pdftotext -layout` | Strong on tables; needs `poppler-utils` on PATH. |
| `purego` | ledongthuc/pdf | 100% Go, lightest binary, the `CGO_ENABLED=0` static path; lower table fidelity. |

## Architecture

```
cmd/pindex        CLI (index | ask | eval | extract)
internal/
  config          typed config (mirrors PageIndex defaults)
  extract         pluggable Extractor (mupdf/poppler/purego)
  tree            pure tree ops (list→tree, page ranges, renderer)
  prompts         the ~15 ported prompts + typed schemas
  llm             Provider seam: OpenAI/Anthropic HTTP, resilience, prompt-hash cache, structured output
  index           the indexing engine (TOC-less structure generation → tree → enrich)
  store           SQLite catalog + per-doc JSON blobs
  retrieve        get_document / get_structure / get_page_content
  ask             select-pages-then-answer retrieval loop
  pipeline        batch indexing (bounded concurrency + resume)
eval/financebench FinanceBench harness (LLM-judge accuracy + evidence recall)
```

## FinanceBench

Run the harness over a pre-indexed workspace:

```sh
pindex index ./financebench/pdfs/SOME_DOC.pdf --model gpt-4o-mini --workspace ws
pindex eval --questions financebench_open_source.jsonl --workspace ws \
  --model gpt-4o --judge-model gpt-4o --limit 50
```

It reports **LLM-judge answer accuracy** (the permissive Mafin 2.5 rubric, for comparability) and
**evidence recall** (does the cited page text contain the gold evidence — alignment-free). A
page-number recall is also printed but is *alignment-sensitive* (pindex's physical page index can
differ from a filing's printed page label).

Live result on a real earnings release (`ULTABEAUTY_2023Q4_EARNINGS`, 9 pages, 4 questions):

| ask/judge model | answer accuracy | evidence recall |
|---|---|---|
| gpt-4o-mini | 0% | 0% |
| **gpt-4o** *(same stored index)* | **50%** | **75%** |

Swapping only the model (no re-index) recovered accuracy — the pipeline is sound and model-bound
(PageIndex's published 98.7% used GPT-4o/DeepSeek, not mini).

## License

AGPL-3.0 — the default MuPDF extractor (go-fitz) is AGPL. Building with only the pure-Go `purego`
extractor avoids that dependency.
