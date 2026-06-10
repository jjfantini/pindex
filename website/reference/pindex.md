---
title: pindex
---

## pindex

Vectorless, reasoning-based RAG over document trees

### Synopsis

pindex builds hierarchical tree indexes from PDFs/Markdown and answers
questions by LLM reasoning over that structure — no vectors, no fixed
chunking, traceable page citations.

### Options

```
      --config string   path to a pindex config YAML (optional)
  -h, --help            help for pindex
```

### SEE ALSO

* [pindex ask](./pindex_ask)	 - Answer a question over an indexed document (cites pages)
* [pindex eval](./pindex_eval)	 - Run the FinanceBench evaluation over a pre-indexed workspace
* [pindex extract](./pindex_extract)	 - Debug: dump per-page extracted text for a PDF
* [pindex index](./pindex_index)	 - Index a PDF (prints its tree) or a directory of PDFs (batch, resumable)

