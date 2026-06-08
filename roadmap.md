# pindex roadmap

v1 ships a tight, single-process, CLI-first core (see `docs/PLAN.md`): `index`/`ask`/`eval` work
live across OpenAI and Anthropic. North star: *simplicity until stable.*

## v1.x — near-term refinements

These were consciously deferred during the v1 build; none block the working tool.

- **TOC-detection branches.** *(done)* TOC detection is now the **always-on primary path**: `check_toc`
  + the page-numbered branch + per-section `verifyAndRepair` (verify_toc/fix_incorrect_toc/
  single_toc_item_index_fixer) recover the printed→physical `PageOffset` and a faithful item hierarchy,
  with the structure-generation path kept as the internal fallback for docs without a usable TOC.
  Detection depth is tunable via `--toc-page-limit` (default 10; 0 disables).
- **recall@page alignment.** Align pindex's physical page index to a filing's *printed* page label
  (PageIndex's `calculate_page_offset`) so page-number recall is trustworthy. Until then the eval
  harness reports the alignment-free **evidence recall** as the primary retrieval metric.
- **Vision extractor.** The `vision` backend (render page images → vision model) for scanned/hard
  pages is stubbed (returns unimplemented).
- **Real tokenizer.** v1 uses a chars/4 heuristic token counter; swap in tiktoken for tighter
  chunk sizing.
- **Markdown ingestion.** v1 is PDF-only; add the `#`-header → tree path.

## v2 — deferred features

### Corpus-scale File System (the enterprise tier)
Route a query across a 500k-document corpus, not just within one document.
- **Virtual nodes** — synthesized topic / metadata / entity / cluster nodes layered over the raw
  document trees so the corpus itself is searchable as a tree.
- **Query-dependent corpus tree** — build the corpus view on demand, conditioned on the query
  (vendor-first vs. year-first vs. status-first), without re-ingesting or re-embedding.
- **Adaptive traversal** — layer-wise descent where labels are informative; recursive flattening
  where intermediate labels are weak.

### Search-as-Code `ask` primitive
Perplexity-style "search as code generation": the model emits a small traversal program executed
in one shot (sandbox + codegen validation), instead of the v1 iterative tool-by-tool loop. Adopt
only if v1 measurements show LLM round-trips — not tree traversal — are the bottleneck.

### TOON renderer + A/B
A `Renderer` seam exists at the LLM boundary from v1 (JSON is canonical on disk). v2 adds a TOON
renderer and A/B-tests its real token savings on FinanceBench. Expected win is narrow: TOON's ~58%
edge only shows when rendering one node's *children as a flat table*; the nested tree (~32%) and
raw page prose (N/A) don't benefit.

### Distributed indexing
v1 is single-process (bounded `errgroup` worker pool + SQLite checkpoint for resumability). The
job-store and worker-pool *interfaces* are kept clean so a durable/distributed backend
(River / asynq / Temporal) can plug in later without touching the engine.

## Out of scope (for now)
- Hosted API / MCP server (CLI > API > MCP — API and MCP come after the CLI is strong).
- Web UI / chat surface.
