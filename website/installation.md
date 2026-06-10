---
title: Installation
---

# Installation

pindex ships as a single binary in two flavors:

- **`pindex`** — the default build. Links a bundled static libmupdf (via go-fitz) for the highest-fidelity PDF extraction. Licensed AGPL-3.0.
- **`pindex-lite`** — pure Go, fully portable, no MuPDF. Uses the `purego` extractor (lower table fidelity). Licensed Apache-2.0.

## Homebrew (recommended)

```sh
brew install jjfantini/humbl/pindex        # default: MuPDF, full fidelity (AGPL-3.0)
brew install jjfantini/humbl/pindex-lite   # pure-Go, portable, no MuPDF (Apache-2.0)
```

## Release binaries

Prebuilt archives are attached to each [GitHub release](https://github.com/jjfantini/pindex/releases) for:

- darwin/amd64, darwin/arm64
- linux/amd64, linux/arm64

(No Windows builds.) Download `pindex_<version>_<os>_<arch>.tar.gz` or `pindex-lite_<version>_<os>_<arch>.tar.gz`, verify against the matching `pindex_checksums.txt` / `pindex-lite_checksums.txt`, and put the binary on your `PATH`.

## go install

```sh
go install github.com/jjfantini/pindex/cmd/pindex@latest
```

This compiles the default cgo/MuPDF build, so you need a C toolchain (no system MuPDF — go-fitz bundles a static libmupdf). For a pure-Go install:

```sh
CGO_ENABLED=0 go install github.com/jjfantini/pindex/cmd/pindex@latest
```

then set `extractor: purego` at runtime (see below).

::: tip
A `go install` binary may report a default dev version string, since releases inject the version at build time via `-ldflags`.
:::

## Build from source

```sh
# Default build: go-fitz/MuPDF, needs a C compiler
go build -o pindex ./cmd/pindex

# Fully-static pure-Go build
CGO_ENABLED=0 go build -o pindex ./cmd/pindex
```

::: warning No `-tags purego`
There is no `purego` build tag. Under `CGO_ENABLED=0`, go-fitz is excluded automatically by its `cgo` build constraint. The pure-Go extractor is a **runtime** choice: pass `--backend purego` or set `extractor: purego` in your config. Selecting `mupdf` in a static build returns a clear error.
:::

See the [choosing an extractor guide](/guides/choosing-an-extractor) for the fidelity tradeoffs.

## Verify

```sh
pindex --version
```

```
pindex version 0.1.1
```
