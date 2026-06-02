# pindex — a Go rewrite of PageIndex (vectorless, reasoning-based RAG)

## Context

PageIndex (VectifyAI, Python) builds hierarchical *tree* indexes from PDFs/Markdown and
does **vectorless** RAG — retrieval by LLM reasoning over document structure, no embeddings,
no chunking, traceable page citations. Its published FinanceBench eval (Mafin 2.5) hits
**98.7%** answer accuracy. We are rebuilding the engine in Go as **pindex** to get: a
single-binary, low-memory, fast-startup, **CLI-first** tool; clean concurrency orchestration;
and a robustness layer the Python original lacks (it silently swallows failed LLM calls,
has no backoff/jitter, no concurrency cap, no response cache, untyped config, non-atomic
manifest writes). The OSS PageIndex is *index-only* — it has **no `ask` primitive**; you
roll your own. pindex bakes in a clean `ask`.

**Honest framing of the goal:** Indexing is **LLM-bound** (a 100-page PDF = 100–200 LLM
calls). Go will *not* make the LLM faster. What Go + good design buys us, and what "scale to
500k docs / performant" actually means here:
- **bounded concurrency + rate limiting** (don't get throttled / banned),
- **resumable indexing** (SQLite checkpoint — never re-index a completed doc after a crash),
- a **prompt-hash response cache** (re-runs, crash recovery, and eval iteration become nearly
  free — directly serves "don't burn retries"),
- **degradation** (circuit-break a dead provider without draining the retry budget),
- low memory + instant CLI an LLM can drive token-efficiently, and a **single binary**.
  *Caveat:* the default extractor (go-fitz/MuPDF) is CGo → the default release binary is
  platform-specific, not fully static. A **fully-static, cross-compilable build** is available
  via a build tag that swaps in the pure-Go `ledongthuc` extractor (quality trade for portability).

**North star (do not drift):** simplicity until stable. Single-process for v1. Defer the
corpus-scale File System and Search-as-Code to v2 (captured in `roadmap.md`).

---

## Locked decisions (from user)

| # | Decision | Choice |
|---|----------|--------|
| PDF extraction | **Pluggable `Extractor` adapter**, backend chosen via config. Accuracy-first; CGo/AGPL acceptable (project is OSS). | **Default: go-fitz / MuPDF** (CGo, the Go equivalent of PyMuPDF — same engine). Adapters: poppler `pdftotext -layout`; pure-Go `ledongthuc/pdf` (zero-dep fallback); vision-LLM (scanned/hard pages). A FinanceBench **bake-off** tunes the default; users can switch. CGo default ⇒ release binary is platform-specific; pure-Go build tag gives a static cross-compile. |
| Eval metric | Mirror PageIndex's own methodology **and** add an objective diagnostic. | **Primary (comparability):** LLM-judge answer accuracy mirroring Mafin 2.5 `check_answer_equivalence`. **Secondary (engineering):** retrieval **recall@page** on the 150-q public subset. |
| TOON | TOON's win (~58%) only appears rendering one node's children as a flat table; the nested tree (~32%) and raw page prose (N/A) are the heavy payloads. | **JSON canonical on disk (incl. `_pindex.json`)** + a `Renderer` seam at the LLM boundary. Ship JSON in v1; add a TOON renderer in v2 and A/B real token savings on FinanceBench. |
| v1 ambition | Keep retrieval simple. | **Simple iterative tree-search `ask` loop** (single-doc + multi-doc-by-description routing). **Defer** corpus File System (virtual nodes / query-dependent corpus tree) and Search-as-Code → `roadmap.md`. |

**Still to confirm at kickoff (non-blocking):** Go module path — proposed
`github.com/jenningsfantini/pindex`.

---

## Architecture (module layout)

```
pindex/
  cmd/pindex/main.go            # cobra root: index | ask | eval | extract (debug)
  internal/
    config/    # typed Config struct, YAML load, schema validation (defaults mirror config.yaml)
    tree/      # PURE types + ops: TreeNode, list_to_tree, write_node_id, remove_fields,
               #   page-range parser, JSON/TOON renderers  (heavy TDD, no LLM)
    prompts/   # the ~15 prompts as templates + typed output schemas (struct -> JSON Schema)
    llm/       # Provider interface; openai + anthropic adapters; validate-then-retry structured
               #   output; resilience (retry + circuit-breaker + rate-limit); prompt-hash cache;
               #   token counting; MockProvider/cassettes (same seam as the cache)
    extract/   # Extractor interface + adapters: mupdf(go-fitz, default), poppler, purego, vision
               #   NOTE: go-fitz cannot call Text() concurrently on the SAME doc handle —
               #   serialize page calls within a doc; parallelize ACROSS docs (see pipeline/)
    index/     # the engine state machine: TOC detect -> branch -> verify/fix -> list->tree ->
               #   large-node split -> enrich.  LLM-calling steps thin; pure logic lives in tree/
    store/     # SQLite catalog (modernc.org/sqlite, pure-Go) + per-doc JSON blobs;
               #   atomic writes; resumable indexing checkpoint/job store
    retrieve/  # get_document, get_document_structure (text stripped), get_page_content
    ask/       # iterative tree-search agent loop -> grounded answer w/ page citations
    pipeline/  # bounded worker pool (ants) batch indexing; resume; rate-limited; degradation-aware
  eval/financebench/  # download -> index -> ask -> score (LLM-judge + recall@page)
  testdata/           # golden small PDF/MD, LLM cassettes
  README.md  roadmap.md
```

**Library choices** (from research; all vetted): `spf13/cobra` (CLI), `openai-go` +
`anthropic-sdk-go` (thin `Provider` adapter over both), `avast/retry-go` + `sony/gobreaker` +
`golang.org/x/time/rate` (resilience), `panjf2000/ants` (worker pool),
`modernc.org/sqlite` (pure-Go SQLite catalog), `invopop/jsonschema` (struct→schema) +
`santhosh-tekuri/jsonschema` (validate LLM output), `pkoukk/tiktoken-go` (tokens),
`gen2brain/go-fitz` (MuPDF). **No** Temporal/River/asynq in v1 — keep job-store/worker-pool
*interfaces* clean so distribution can plug in later.

---

## What we port from PageIndex (reuse map)

The engine is a deterministic state machine around ~15 prompts. Port faithfully; quote and
re-implement, don't reinvent. Source = `/Users/jjfantini/github/PageIndex/pageindex/`.

| pindex target | PageIndex source | Notes |
|---|---|---|
| `tree.ListToTree`, node-id, field-strip, page-range parse | `page_index.py:list_to_tree/post_processing`, `utils.py:write_node_id/remove_fields`, `retrieve.py:_parse_pages` | **Pure → real TDD.** These were the fragile bits in Python; harden them. |
| `index/` state machine + 3 branches | `page_index.py:check_toc/find_toc_pages/process_toc_with_page_numbers/process_toc_no_page_numbers/process_no_toc/meta_processor` | Preserve the fallback chain (page-numbered → no-page → no-TOC). |
| verify + fix | `page_index.py:verify_toc/check_title_appearance/fix_incorrect_toc_with_retries/single_toc_item_index_fixer` | Sampling verify; bounded fix retries (3). |
| page-offset alignment | `page_index.py:extract_matching_page_pairs/calculate_page_offset/add_page_offset_to_toc_json` | **Critical for recall@page** (printed label ≠ internal index). |
| enrich | `utils.py:generate_node_summary/generate_doc_description`, `write_node_id` | config-gated (`if_add_*`). |
| retrieval | `retrieve.py` (whole file) | `get_document` / `get_document_structure` (text stripped) / `get_page_content`. |
| `ask/` loop | `examples/agentic_vectorless_rag_demo.py` | Reference for the agent loop; we make it a first-class CLI command. |
| config defaults | `config.yaml` | model, `toc_check_page_num:20`, `max_page_num_each_node:10`, `max_token_num_each_node:20000`, `if_add_node_id/summary/doc_description/node_text`. |
| extraction | `utils.py:get_page_tokens` (PyPDF2/PyMuPDF) | Our `Extractor` interface generalizes this; `(text, token_count)` per page. |

**All ~15 prompts** (toc_detector, toc_transformer, toc_index_extractor, add_page_number_to_toc,
generate_toc_init/continue, check_title_appearance(_in_start), single_toc_item_index_fixer,
generate_node_summary, generate_doc_description, completeness checks) port verbatim into
`prompts/`. **Keep the `{"thinking": ...}` CoT fields** — that field does accuracy work;
use validate-then-retry, **not** hard constrained-decoding. Keep **`temperature=0`** (index
determinism + cache hits).

## Robustness improvements over PageIndex (deliberate divergences)

1. **No silent failures** — failed LLM calls / unparseable JSON return typed errors, not `""`/`{}`.
2. **Real resilience** — exponential backoff + jitter, circuit-breaker per provider, rate limiter,
   bounded retry budget (degradation ≠ retry-storm).
3. **Prompt-hash response cache** — `(model, prompt) → response`; the same seam mocks LLMs in tests.
4. **Typed, validated config + schemas** (struct → JSON Schema → validate LLM output).
5. **Bounded concurrency** (worker pool/semaphore) instead of unbounded `asyncio.gather`.
6. **SQLite catalog, not a single growing `_pindex.json`** — a flat manifest re-read/rewritten
   per doc does not scale to 500k. Per-doc JSON blobs stay on disk (inspectable, git-friendly).
7. **Resumable indexing** — checkpoint store; skip completed docs on restart.
8. **Recursion depth guard** on large-node split; atomic catalog writes.

---

## Phased PR plan (each PR small, merges into `develop`)

> Dependency order: **P0 → P1** foundation. **P1 (tree), P2 (llm), P3 (extract)** then run
> largely in parallel. **P4** needs 1+2+3. **P5→P6→P7** are sequential. **P8** anytime.
> **One hard coupling:** P4 cassettes embed page text, which depends on the extractor, so
> **P3.3 must set the default extractor before P4 records cassettes** (re-record if it changes).

**P0 — Scaffold & infra**
- P0.1 repo scaffold (`go.mod`, cobra skeleton with stub `index/ask/eval`), GitHub Actions CI
  (build, `go vet`, `golangci-lint`, `go test`) — **CI installs the C toolchain + MuPDF for
  CGo/go-fitz**, plus a static pure-Go build-tag job, so P3.1 doesn't redden CI on arrival;
  `develop` branch via `setup-develop-branch`.
- P0.2 typed `config` (struct + YAML + validation) and core schema structs (TreeNode, Document,
  catalog entry) with struct→JSON-Schema + round-trip golden tests.

**P1 — Pure tree core (heavy TDD, no LLM)**
- P1.1 `tree`: node types, `ListToTree` (numeric-hierarchy nesting), node-id, field-strip,
  page-range parser, JSON renderer — exhaustive unit tests (these were Python's fragile spots).
- P1.2 (optional/low-priority) Markdown pure-parse path (header regex + stack nest + thinning),
  sharing tree-build. **Conscious call:** MD is *not* load-bearing for FinanceBench (PDF). Ship
  the cheap pure-parse path or defer summaries; note in roadmap.

**P2 — LLM client + resilience + cache (the seam)**
- P2.1 `llm.Provider` interface; OpenAI + Anthropic adapters; structured output (JSON Schema)
  with **validate-then-retry**; preserve CoT fields; `temperature=0`.
- P2.2 resilience: retry-go + gobreaker + rate limiter; typed errors; degradation policy.
- P2.3 prompt-hash response cache (SQLite/disk) + `MockProvider`/cassettes for tests.
- P2.4 token counting (tiktoken-go) + chunking util (`page_list_to_group_text`).

**P3 — Extractor adapters + bake-off**
- P3.1 `Extractor` interface + **go-fitz/MuPDF (default)** + pure-Go `ledongthuc` adapters
  (`(text, tokenCount)` per page; config-selectable). Pure-Go adapter doubles as the **static
  build-tag** path; serialize `Text()` within a doc handle (go-fitz concurrency constraint).
- P3.2 poppler `pdftotext -layout` adapter. Vision-LLM adapter (escape hatch) — **deferrable to
  roadmap** if we want a tighter v1; the pluggable interface makes adding it later free.
- P3.3 FinanceBench 3-PDF **extraction bake-off** → fidelity report on table pages → set default.

**P4 — Indexing engine (port the state machine)**
- P4.1 `prompts` package — all ~15 prompts + typed schemas, cassette tests.
- P4.2 TOC detect + 3 branches (page-numbered / no-page / no-TOC) against cassettes.
- P4.3 verify-by-sampling + fix-with-retries + **page-offset calc** + validate/truncate indices.
- P4.4 `list→tree` post-processing + recursive large-node split + enrichment (id/summary/desc).
- P4.5 e2e single-PDF index → tree JSON; **golden small-doc test** in CI (cassette-backed).

**P5 — Persistence + retrieval + ask**
- P5.1 `store`: SQLite catalog + per-doc JSON blob; atomic writes; resumable checkpoint store.
- P5.2 `retrieve`: port `retrieve.py` (3 tools + page-range semantics, PDF + MD).
- P5.3 `ask`: iterative tree-search loop (description-routing across docs → tree walk → tight
  page fetch → grounded answer **with page citations**). CLI `pindex ask`.

**P6 — Batch pipeline + scale**
- P6.1 bounded worker-pool batch indexing (ants) with **resume** (skip completed via checkpoint),
  rate-limited, degradation-aware. **Parallelize across docs; serialize page extraction within a
  single doc** (go-fitz handle constraint). CLI `pindex index <dir>`.
- P6.2 observability: structured logs, per-doc status, partial-failure report.

**P7 — FinanceBench eval harness**
- P7.1 fetch/prepare the **150-question public subset** (PDFs + questions + evidence pages).
- P7.2 run pipeline (index all 361/subset docs into one store, `ask` each question in
  **corpus-search** mode), score:
  - **answer accuracy** via LLM-judge mirroring Mafin 2.5 `check_answer_equivalence` (permissive:
    rounding/superset/semantic-equivalence) → directly comparable to the 98.7% claim;
  - **recall@page**: did fetched/cited pages include the gold evidence page, *after*
    physical_index ↔ printed-label alignment (hand-verify 2–3 first).
  - Report both + cost/query (cache makes re-runs cheap).

**P8 — Docs & examples**
- README (install, `index`/`ask`/`eval` usage), runnable examples, and **`roadmap.md`** (v2:
  corpus File System / virtual nodes / query-dependent corpus tree; Search-as-Code `ask`;
  TOON renderer A/B; full MD summaries; distributed job queue).

---

## Execution workflow (after approval)

- **Branching:** `setup-develop-branch` first. Every PR forks from `develop`, merges back into
  `develop` (I merge); you review `develop → main`.
- **Isolation:** each PR-chunk built by a subagent in its **own git worktree** (no toes stepped on).
- **Orchestration (ultracode):** use the Workflow tool to pipeline independent chunks (P1/P2/P3
  fan out after P0) and to **adversarially review** each PR (correctness + simplicity) before
  it merges. Small, atomic PRs.
- **TDD everywhere, with a stated taxonomy:**
  1. **pure functions** (tree ops, page-range, offset calc, renderers) → real red-green TDD;
  2. **LLM-calling funcs** → cassette/mock provider (deterministic, free, fast) — same seam as
     the response cache;
  3. **e2e** → one golden doc in CI (cassette-backed); FinanceBench is a **paid benchmark run
     outside CI**.
  Minimal implementation to pass tests; keep the codebase tight.

---

## Verification (end-to-end)

1. **Unit/CI:** `go test ./...` green; `golangci-lint` clean; pure-logic packages near-100% covered.
2. **Golden index:** `pindex index testdata/sample.pdf` reproduces the committed golden tree JSON.
   Determinism comes from the **frozen LLM cassette** (providers aren't guaranteed deterministic
   even at `temperature=0`); cassette text is extractor-specific, so it's pinned to the P3.3 default.
3. **ask smoke:** `pindex ask <doc> "..."` returns a grounded answer citing specific pages; verify
   the cited pages actually contain the evidence.
4. **Extraction bake-off (P3.3):** side-by-side fidelity on 3 FinanceBench table pages across
   adapters; default backend chosen on evidence.
5. **FinanceBench (P7):** report LLM-judge answer accuracy (vs Mafin 2.5's 98.7%) **and**
   recall@page on the 150-q subset; hand-verify 2–3 page alignments before trusting the harness.
6. **Resilience:** kill/200-error a provider mid-batch → circuit breaks, retry budget not drained,
   `pindex index` **resumes** without re-indexing completed docs; cache hits visible on re-run.
