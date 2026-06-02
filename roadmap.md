# pindex roadmap

v1 deliberately ships a tight, single-process, CLI-first core (see `docs/PLAN.md`). The items
below are **explicitly deferred to v2** — they are wanted, but only after v1 is stable and proven
on FinanceBench. North star: *simplicity until stable.*

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

### Full Markdown pipeline
v1 ships only the cheap pure-parse MD path (header regex + stack nesting + thinning). v2 adds async
LLM node summaries for Markdown, matching the PDF enrichment path. (MD is not load-bearing for the
FinanceBench/PDF benchmark, hence the deferral.)

### Distributed indexing
v1 is single-process (bounded `ants` worker pool + SQLite checkpoint for resumability). The
job-store and worker-pool *interfaces* are kept clean so a durable/distributed backend
(River / asynq / Temporal) can plug in later without touching the engine.

## Out of scope (for now)
- Hosted API / MCP server (CLI > API > MCP — API and MCP come after the CLI is strong).
- Web UI / chat surface.
