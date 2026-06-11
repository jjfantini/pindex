---
title: pindex ask
---

## pindex ask

Answer a question over an indexed document (cites pages)

```
pindex ask <question> [flags]
```

### Options

```
      --cache-dir string   prompt-hash response cache dir (empty to disable) (default ".pindex/cache")
      --doc string         document id or path (default: the only indexed doc)
      --effort string      retrieval effort: low|medium|high|ultra (medium retries on refusal; high uses an agentic tree-search loop; ultra adds an answer-verification pass) (default "low")
      --env-file string    load API keys from this .env file (default ".env")
  -h, --help               help for ask
      --include-pages      include raw page text in the exported tree
      --model string       LLM model (default from config)
      --out string         append this Q&A (and the doc's tree) to a browsable output directory
      --rpm int            max requests/min to the LLM (0 = unlimited)
      --workspace string   workspace directory (default ".pindex/workspace")
```

### Options inherited from parent commands

```
      --config string   path to a pindex config YAML (optional)
      --plain           force plain line-oriented output: no colors or animations (also via PINDEX_PLAIN=1; auto when piped)
      --verbose         stream under-the-hood diagnostics to stderr (LLM calls, cache hits, retries, build stages)
```

### SEE ALSO

* [pindex](./pindex)	 - Vectorless, reasoning-based RAG over document trees

