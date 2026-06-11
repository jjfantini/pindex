---
title: pindex extract
---

## pindex extract

Debug: dump per-page extracted text for a PDF

```
pindex extract <pdf> [flags]
```

### Options

```
      --backend string   extractor backend: mupdf|poppler|purego (default: from config)
  -h, --help             help for extract
```

### Options inherited from parent commands

```
      --config string   path to a pindex config YAML (optional)
      --plain           force plain line-oriented output: no colors or animations (also via PINDEX_PLAIN=1; auto when piped)
      --verbose         stream under-the-hood diagnostics to stderr (LLM calls, cache hits, retries, build stages)
```

### SEE ALSO

* [pindex](./pindex)	 - Vectorless, reasoning-based RAG over document trees

