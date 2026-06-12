# pindex

> A fast, **vectorless** reasoning-RAG engine in Go — a single-binary CLI that builds a hierarchical tree index from PDFs and answers questions by having an LLM reason over that structure, citing the exact pages it used. A rewrite of [PageIndex](https://github.com/VectifyAI/PageIndex).

## Project Intent

- **What it does:** Builds a hierarchical **tree index** from a PDF (no embeddings, no fixed chunking) and answers questions by walking the tree (section summaries → relevant branch → tight page range), grounding answers in cited pages. `index` / `ask` / `eval` run live against OpenAI and Anthropic.
- **Who it's for:** Developers and teams doing document QA over PDFs (validated on financial filings / FinanceBench) who want traceable, vectorless retrieval driven from one binary an LLM can operate.
- **Non-goals (deliberately out of scope):** No embeddings / vector store (vectorless by design). No hosted API or MCP server in v1 (CLI > API > MCP — those come after the CLI is strong). No web UI / chat surface. No corpus-scale routing, virtual nodes, or Search-as-Code (v2). Single-process only — no distributed indexing in v1. PDF-only for now (Markdown ingestion is deferred).

## Architecture & Stack

- **Language(s) / framework(s):** Go (go.mod `go 1.25`; CI builds on Go 1.26). `spf13/cobra` (CLI), `modernc.org/sqlite` (pure-Go catalog), `gen2brain/go-fitz`/MuPDF (default extractor, cgo), `ledongthuc/pdf` (pure-Go extractor), `golang.org/x/sync` errgroup (bounded concurrency), `yaml.v3` (config). LLM access is **hand-rolled HTTP adapters** for OpenAI + Anthropic behind a `Provider` seam — no vendor SDKs.
- **Runtime / target:** Single-binary CLI. Default build is **cgo** (`go build -o pindex ./cmd/pindex`) and links a bundled static libmupdf — needs a C toolchain. Fully-static path: `CGO_ENABLED=0 go build -o pindex ./cmd/pindex` (go-fitz is build-tagged out) plus `extractor: purego` at runtime. There is **no `-tags purego` build tag** — portability is a runtime extractor choice, not a tag.
- **Repo layout & entry points:** `cmd/pindex/` — cobra root + subcommands `index` | `ask` | `eval` | `extract`. `internal/` — `config` (typed YAML), `extract` (pluggable Extractor: mupdf/poppler/purego), `tree` (pure ops: list→tree, page ranges, JSON renderer), `prompts` (~15 ported prompts + typed schemas), `llm` (Provider seam, resilience, prompt-hash cache, structured output), `index` (TOC-less structure generation → tree → enrich), `store` (SQLite catalog + per-doc JSON), `retrieve`, `ask`, `pipeline` (batch), `envfile`. Also `eval/financebench/`, `docs/PLAN.md` (the design), `roadmap.md`, `system_design/`, `testdata/`.
- **Data flow / external services:** extract pages → `index` (LLM structure gen → tree) → enrich (optional summaries) → `store` (catalog at `.pindex/workspace/catalog.db` + per-doc JSON blobs) → `retrieve` (`get_document` / `get_structure` / `get_page_content`) → `ask` (tree-search → select tight page range → answer with page citations). Provider is routed by **model name**: `claude*` → Anthropic, otherwise OpenAI. API keys come from a gitignored `.env` (which **overrides** the process environment) — copy `.env.example`.

## Engineering Preferences

- **Planning:** Plan-first for non-trivial work (3+ steps or architectural impact); just-do-it for small, well-scoped changes. State the plan, then implement.
- **Subagents:** Use subagents for broad search, research, and parallel exploration to keep the main context clean — keep the findings, not the file dumps.
- **Development philosophy:** *Simplicity until stable* (the North Star). The rewrite's value is the **engineering envelope** around LLM-bound work — bounded concurrency, resumable indexing, prompt-hash cache, graceful degradation — not raw Go speed. Defer corpus-scale / v2 complexity (`roadmap.md`).
- **Self-improvement loop:** After a correction, capture the rule (a memory note or a CLAUDE.md tweak) so the same mistake doesn't recur.

## Code Quality

- **Formatter / linter:** `gofmt` + `goimports` (run before commit). Lint with `golangci-lint run` (v2 config: errcheck, govet, ineffassign, staticcheck, unused, misspell) to match the CI `lint` job; also `go vet ./...`.
- **Naming & comments:** Idiomatic Go — `New*` constructors, interface seams (`Provider`, `Extractor`, `Renderer`, `Cache`), uppercase acronyms (`LLM`, `PDF`, `TOC`). Every package carries a doc comment stating its purpose. JSON is canonical on disk; any alternate rendering goes behind the `Renderer` seam.
- **Simplicity bar:** Ship the simplest change that fully solves the problem; flag any hack for review. Don't add abstraction ahead of a second concrete use.
- **Size limits:** No hard line/function cap, but keep **pure logic in `tree/`** and **thin LLM steps in `index/`** — if a function mixes pure logic with an LLM call, split it so the pure half stays testable.

## Testing

- **Framework & run command:** Standard Go `testing` (no testify). Run `go test -race ./...` (matches CI) and `go vet ./...`. Validate the static path with `CGO_ENABLED=0 go test ./...`.
- **Coverage / test rule:** Pure-logic packages (`tree/`, `config`, page parsing) are near-100% covered via real red-green TDD. LLM-calling code is tested with `MockProvider` / cassettes (the same seam as the prompt-hash cache — deterministic, free, fast). Every bug fix ships a regression test that fails before the fix.
- **Verify before done:** Run `go test -race ./...`, `go vet ./...`, and `golangci-lint run` before claiming done. For behavior changes, exercise the real CLI path (e.g. `pindex extract` / `pindex ask`) — never mark a task complete on a green typecheck alone.

## Evaluation & Benchmarking (FinanceBench)

- **Incremental by design:** the benchmark accumulates **one document at a time** (the full 150-question suite in one shot is too expensive). `./eval/financebench/bench.sh <DOC_NAME>` is the whole loop: fetch → index → eval at every effort level → fold into `eval/financebench/results/<model>/<effort>/<DOC>/` → re-aggregate. Existing results are never recomputed.
- **Source of truth = per-question answer records** (`<DOC>/answers/<id>.json`). Every `summary.json` (the per-effort sum over all docs), Mafin-compatible `result_<model>.json`, `human_evaluations.csv`, and the scoreboard are **derived** — regenerated by `go run ./eval/financebench/aggregate` — never hand-edited.
- **Pooling rules:** one judge per `<model>/<effort>` (the aggregator fails loudly on a mix; standard judge is `gpt-4o-2024-11-20`, Mafin 2.5's, for comparability) and each doc appears at most once — so summed accuracy is a clean micro-average. Trees live once per doc in `<model>/trees/` (identical across efforts).
- **Raw vs adjusted accuracy:** raw = LLM-judge only; adjusted additionally counts human-relabelled `MVA`/`BE`/`SEDC` as correct (Mafin's 98.7% is an adjusted number). **Only the user adjudicates** — wrong answers auto-label `NAL` and are never self-graded up. Relabel in the answer record (`label` + `label_reason`), then re-aggregate; every non-AL question surfaces in `human_evaluations.csv` for review.
- **Current scoreboard (2026-06-12, 9/84 docs, 32/150 q, haiku + gpt-4o judge):** low/medium raw 87.50%, adjusted 96.88%; high raw 90.62%, adjusted 93.75%; ultra raw 87.50%, adjusted 90.62% (three 2026-06-12 misses still NAL, pending user adjudication). Regenerate via `go run ./eval/financebench/aggregate` — canonical table in `eval/financebench/results/README.md`.
- **No ML train/test split** — nothing is trained. `testdata/diagnostic_set.json`'s `train` list exists only to track **prompt-tuning contamination** (those docs were used while iterating on prompts; flag them if benchmarked). Everything else is fresh.
- **Dataset is CC-BY-NC-4.0 and never committed:** `fetch.sh` pulls questions/PDFs into the gitignored `testdata/`; committed results carry question text + gold answers only in the Mafin-style result files, with attribution in `results/README.md`.

## Performance

- **Budgets:** None — indexing is **LLM-bound** (a 100-page PDF ≈ 100–200 LLM calls); Go does not make the model faster. Do not micro-optimize Go hot paths.
- **Hot paths to protect:** The resilience/cost envelope — the **prompt-hash response cache** (re-runs and crash recovery are nearly free), **bounded concurrency + rate limiting**, **resumable batch indexing** (SQLite checkpoint — never re-index a finished doc), and **circuit-breaker degradation** (a dead provider breaks instead of draining the retry budget). Note: go-fitz cannot call `Text()` concurrently on the same doc handle — serialize page calls within a doc, parallelize **across** docs.
- **Do NOT optimize:** Go-level micro-performance, and never trade away simplicity or the resilience layer for speed. Token-efficiency work (the `Renderer` seam / TOON A/B) is a v2 measurement, not a v1 concern.

## Bug Protocol

- **Autonomy:** Fix clear bugs end-to-end from failing tests, logs, and errors — no hand-holding or confirmation needed.
- **Root-cause discipline:** Fix the cause, not the symptom. No temporary patches and no silent-failure band-aids — surfacing a typed error beats swallowing one or returning empty (the original sin this rewrite exists to fix).
- **Regression tests:** Every fix lands with a regression test that fails before the fix and passes after.

## Task Management & Core Principles

- **How work is tracked:** The P0–P8 build phases are done (v1 ships). Going forward: **small / low-risk changes commit directly to `develop`; larger or riskier features go on a feature branch → PR into `develop`.** `develop` is the integration branch (CI runs on it), `master` is stable/release (`develop` → `master` for releases). Use a checkable plan for multi-step work and mark items done as you go. Commit messages follow Conventional Commits (e.g. `feat(scope): …`, `docs: …`), matching history.
- **Core principles (non-negotiable):**
  - **Simplicity until stable** — single-process, CLI-first; defer v2 complexity until the core is solid.
  - **No silent failures** — surface typed errors; never swallow an error or return empty (`""` / `{}`) on failure.
  - **It's LLM-bound** — protect the resilience envelope (cache, bounded concurrency, resume, circuit-breaker); don't micro-optimize Go.
  - **Pure logic stays LLM-free and heavily tested** — keep `tree/`-style logic pure and near-100% covered behind the provider seam; every bug fix ships a regression test.
