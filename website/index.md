---
layout: home

hero:
  name: pindex
  text: Vectorless reasoning-RAG for PDFs
  tagline: A single binary that builds a tree index from a PDF and answers questions with exact page citations. No embeddings, no vector DB.
  actions:
    - theme: brand
      text: Get Started
      link: /getting-started
    - theme: alt
      text: Why not vector RAG?
      link: /features
    - theme: alt
      text: GitHub
      link: https://github.com/jjfantini/pindex

features:
  - title: Tree index, not chunks
    details: Documents are indexed as a hierarchical tree of sections with page ranges — no fixed chunking that slices through tables and arguments.
  - title: Page-cited answers
    details: Every answer cites the exact pages it used, so you can verify the claim against the source PDF.
  - title: No vector DB
    details: Retrieval is an LLM reasoning over the tree structure. There are no embeddings to compute and no vector store to host.
  - title: Resumable, cached indexing
    details: Batch indexing checkpoints to SQLite and a prompt-hash cache makes re-runs and crash recovery nearly free.
  - title: OpenAI and Anthropic
    details: Provider is routed by model name — claude* goes to Anthropic, everything else to OpenAI. Keys come from a .env file.
  - title: Single static binary option
    details: A CGO_ENABLED=0 build produces a fully-static pure-Go binary with the purego extractor — no C toolchain at runtime.
---

## Quickstart

```sh
brew install jjfantini/humbl/pindex

# put your API key in .env (OPENAI_API_KEY or ANTHROPIC_API_KEY)
echo 'OPENAI_API_KEY=sk-...' > .env

pindex index report.pdf
pindex ask "What was total revenue in 2023?"
```

`index` builds the tree index and saves it to the workspace (`.pindex/workspace`); `ask` walks the tree, reads a tight page range, and prints an answer plus `cited pages: [...]`.

See [Installation](/installation) for all install paths and [Getting Started](/getting-started) for a full walkthrough.
