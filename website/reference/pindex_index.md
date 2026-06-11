---
title: pindex index
---

## pindex index

Index a PDF (prints its tree) or a directory of PDFs (batch, resumable)

```
pindex index <pdf-or-dir> [flags]
```

### Options

```
      --backend string       extractor backend (default from config)
      --cache-dir string     prompt-hash response cache dir (empty to disable) (default ".pindex/cache")
      --concurrency int      parallel documents when indexing a directory (default 4)
      --env-file string      load API keys from this .env file (overrides the environment) (default ".env")
      --force                re-index documents already in the workspace
  -h, --help                 help for index
      --include-raw-text     include raw page text in the browsable <workspace>/pindex export (larger, less readable)
      --model string         LLM model (default from config; e.g. claude-haiku-4-5-20251001, gpt-4o)
      --rpm int              max requests/min to the LLM (0 = unlimited; set on low rate-limit tiers)
      --toc-page-limit int   leading pages to scan for a table of contents (0 disables TOC detection; -1 uses the config default of 10) (default -1)
      --workspace string     persist the index here (empty to only print) (default ".pindex/workspace")
```

### Options inherited from parent commands

```
      --config string   path to a pindex config YAML (optional)
      --plain           force plain line-oriented output: no colors or animations (also via PINDEX_PLAIN=1; auto when piped)
      --verbose         stream under-the-hood diagnostics to stderr (LLM calls, cache hits, retries, build stages)
```

### SEE ALSO

* [pindex](./pindex)	 - Vectorless, reasoning-based RAG over document trees

