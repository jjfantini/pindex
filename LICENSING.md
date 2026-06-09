# Licensing

pindex is **dual-licensed**. Which license applies to a binary depends on **how
that binary is built** — specifically, whether it links MuPDF.

## TL;DR

| Artifact | How to get it | License |
| --- | --- | --- |
| pindex's first-party source code | this repository | **Apache-2.0** |
| `pindex` (default build, cgo + MuPDF extractor) | `brew install jjfantini/humbl/pindex` | **AGPL-3.0-or-later** |
| `pindex-lite` (pure-Go, `CGO_ENABLED=0`) | `brew install jjfantini/humbl/pindex-lite` | **Apache-2.0** |

- `LICENSE` — Apache License 2.0 (the first-party / `pindex-lite` license)
- `LICENSE.AGPL` — GNU Affero General Public License v3.0 (the default `pindex` license)
- `NOTICE` — Apache-2.0 attribution notice

## Why two licenses

pindex's own source code is licensed under the **Apache License 2.0**.

The **default build** links **MuPDF** (via `github.com/gen2brain/go-fitz`) for
high-fidelity PDF/text extraction. MuPDF is licensed under the **GNU Affero
General Public License v3.0**. The AGPL is a strong copyleft license, so **any
binary that links MuPDF must itself be distributed under the AGPL-3.0** — that
is the default `pindex` binary.

Apache-2.0 is one-way compatible with AGPL-3.0, so combining Apache-2.0
first-party code with AGPL-licensed MuPDF and distributing the combined work
under AGPL-3.0 is permitted. (You cannot relicense MuPDF; you can only
distribute the combined binary under the AGPL.)

## The permissive path: `pindex-lite`

Building with `CGO_ENABLED=0` excludes go-fitz/MuPDF entirely — it lives behind
a `//go:build cgo` tag (`internal/extract/mupdf.go`), with a no-cgo fallback
(`internal/extract/mupdf_nocgo.go`). The resulting `pindex-lite` binary links
**no AGPL-covered code** — only permissively-licensed dependencies — so it is
distributed under the **Apache License 2.0**.

`pindex-lite` uses the pure-Go `purego` extractor (`ledongthuc/pdf`); set
`extractor: purego` in your config. (Selecting the `mupdf` backend in a lite
build returns a clear error rather than failing silently.) Table-extraction
fidelity is lower than the MuPDF build.

## Dependency provenance (pure-Go / `pindex-lite` build)

Every dependency compiled into the `CGO_ENABLED=0` binary is permissively
licensed and AGPL-compatible:

| Dependency | License |
| --- | --- |
| github.com/spf13/cobra, spf13/pflag | Apache-2.0 / BSD-3 |
| github.com/ledongthuc/pdf | BSD-3-Clause |
| modernc.org/sqlite (+ libc, mathutil, memory) | BSD-3-Clause |
| github.com/ebitengine/purego | Apache-2.0 |
| github.com/jupiterrider/ffi | MIT |
| gopkg.in/yaml.v3 | MIT / Apache-2.0 |
| golang.org/x/sync, x/time, x/sys | BSD-3-Clause |
| github.com/google/uuid | BSD-3-Clause |
| github.com/dustin/go-humanize, mattn/go-isatty | MIT |
| github.com/inconshreveable/mousetrap | Apache-2.0 |

`github.com/gen2brain/go-fitz` (AGPL, bundling MuPDF) is **excluded** from this
build — verify with `CGO_ENABLED=0 go list -deps ./cmd/pindex | grep go-fitz`
(expected: no output).

## If you redistribute

- **Default `pindex` (MuPDF) binary:** comply with AGPL-3.0, including the
  obligation to offer the corresponding source to recipients and remote users.
- **Need a permissive binary:** build and distribute `pindex-lite`
  (`CGO_ENABLED=0`), which carries only Apache-2.0 obligations.
