// Package extract turns a PDF into per-page text behind a pluggable Extractor
// interface, so backends can be swapped via config and A/B-tested on FinanceBench.
//
// Backends:
//   - mupdf  (default) — gen2brain/go-fitz; high fidelity. Notably it uses purego
//     (FFI), NOT cgo, so it needs no C toolchain or system MuPDF and cross-compiles.
//   - purego — ledongthuc/pdf; pure Go, lightest binary, lower table fidelity.
//   - poppler — shells out to `pdftotext -layout`; needs poppler-utils installed.
//   - vision — deferred to v2 (page images -> vision model) for scanned/hard PDFs.
package extract

import "fmt"

// Page is one extracted page: a 1-based physical index and its text.
type Page struct {
	Index int
	Text  string
}

// Extractor turns a PDF file into per-page text.
type Extractor interface {
	Extract(path string) ([]Page, error)
	Name() string
}

// New returns the extractor for a backend name (empty == the default, "mupdf").
func New(backend string) (Extractor, error) {
	switch backend {
	case "", "mupdf":
		return newMUPDF()
	case "purego":
		return PureGo{}, nil
	case "poppler":
		return Poppler{}, nil
	case "vision":
		return nil, fmt.Errorf("extract: vision backend is deferred to v2 (see roadmap.md)")
	default:
		return nil, fmt.Errorf("extract: unknown backend %q (want mupdf|poppler|purego|vision)", backend)
	}
}
