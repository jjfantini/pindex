---
title: pindex eval
---

## pindex eval

Run the FinanceBench evaluation over a pre-indexed workspace

```
pindex eval [flags]
```

### Options

```
      --cache-dir string     prompt-hash response cache dir (default ".pindex/cache")
      --effort string        retrieval effort: low|medium|high|ultra (medium retries on refusal; high uses an agentic tree-search loop; ultra adds an answer-verification pass) (default "low")
      --env-file string      load API keys from this .env file (default ".env")
  -h, --help                 help for eval
      --include-pages        include raw page text in exported trees (larger, less readable)
      --judge-model string   LLM-judge model (default: retrieval model)
      --limit int            only run the first N questions (0 = all)
      --model string         retrieval model (default from config)
      --out string           output dir for the browsable results (default: <workspace-parent>/evals/<date>_<model>_<effort>; same-day re-runs get a -2, -3, … suffix)
      --questions string     path to a FinanceBench JSONL file (required)
      --rescore string       recompute adjusted accuracy from a (human-edited) result_<model>.json and exit
      --rpm int              max requests/min to the LLM (0 = unlimited; set on low rate-limit tiers)
      --workspace string     workspace with the docs pre-indexed (default ".pindex/workspace")
```

### Options inherited from parent commands

```
      --config string   path to a pindex config YAML (optional)
      --plain           force plain line-oriented output: no colors or animations (also via PINDEX_PLAIN=1; auto when piped)
      --verbose         stream under-the-hood diagnostics to stderr (LLM calls, cache hits, retries, build stages)
```

### SEE ALSO

* [pindex](./pindex)	 - Vectorless, reasoning-based RAG over document trees

