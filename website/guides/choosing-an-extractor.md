---
title: Choosing an Extractor
---

# Choosing an Extractor

## Goal

Pick the right PDF text-extraction backend, and run pindex as a fully-static binary when you need one.

## The backends

| Extractor | How it works | When to use |
|---|---|---|
| `mupdf` (default) | go-fitz / bundled MuPDF, cgo build | Highest fidelity; the default if you installed the regular `pindex` binary |
| `poppler` | Shells out to `pdftotext -layout` | Good table geometry; needs `poppler-utils` on PATH |
| `purego` | ledongthuc/pdf, 100% Go | Static builds and `pindex-lite`; lower table fidelity |

(`vision` is accepted by config validation but deferred to v2.)

## Switching extractors

Per run, on `index` and `extract`:

```sh
pindex index report.pdf --backend poppler
```

Or persistently in a config file (passed with `--config`):

```yaml
extractor: purego
```

## Compare extractors on your PDF

`pindex extract` dumps per-page text without any LLM calls — cheap to compare:

```sh
pindex extract report.pdf --backend mupdf | head -40
pindex extract report.pdf --backend purego | head -40
```

Each page prints as `===== page N (<backend>) =====` followed by its text.

## The fully-static build

The default build is cgo (links a bundled static libmupdf, needs a C toolchain). For a portable, fully-static binary:

```sh
CGO_ENABLED=0 go build -o pindex ./cmd/pindex
```

There is no `-tags purego` build tag — go-fitz is compiled out automatically under `CGO_ENABLED=0`. Portability is a **runtime** extractor choice: set `extractor: purego` in config or pass `--backend purego`. Selecting `mupdf` in a static build errors with a message telling you to rebuild with `CGO_ENABLED=1` or switch to purego/poppler.

The prebuilt `pindex-lite` Homebrew package is exactly this build.

## What you should see

- `pindex extract` output whose page headers name the backend you selected.
- In a static/lite build, `--backend mupdf` fails fast with a clear error instead of producing bad output.
