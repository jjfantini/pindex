//go:build cgo

package extract

import (
	"fmt"

	fitz "github.com/gen2brain/go-fitz"
)

// MuPDFAvailable reports whether the MuPDF (go-fitz) backend is compiled in.
// go-fitz links a bundled static libmupdf via cgo; a CGO_ENABLED=0 build must
// exclude it (its nocgo path panics at init looking for a shared library), so
// the backend is only present in cgo builds.
const MuPDFAvailable = true

func newMUPDF() (Extractor, error) { return MuPDF{}, nil }

// MuPDF extracts text via gen2brain/go-fitz (MuPDF) — the highest-fidelity
// backend and the default. go-fitz cannot have Text() called concurrently on the
// SAME document handle, so pages are read sequentially here; callers parallelize
// across documents, not within one.
type MuPDF struct{}

// Name implements Extractor.
func (MuPDF) Name() string { return "mupdf" }

// Extract implements Extractor.
func (MuPDF) Extract(path string) ([]Page, error) {
	doc, err := fitz.New(path)
	if err != nil {
		return nil, fmt.Errorf("mupdf: open %q: %w", path, err)
	}
	defer func() { _ = doc.Close() }()

	n := doc.NumPage()
	pages := make([]Page, 0, n)
	for i := 0; i < n; i++ {
		text, err := doc.Text(i)
		if err != nil {
			return nil, fmt.Errorf("mupdf: page %d: %w", i+1, err)
		}
		pages = append(pages, Page{Index: i + 1, Text: text})
	}
	return pages, nil
}
