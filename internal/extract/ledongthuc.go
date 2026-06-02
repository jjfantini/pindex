package extract

import (
	"fmt"

	"github.com/ledongthuc/pdf"
)

// PureGo extracts text via ledongthuc/pdf — 100% Go, no embedded native library,
// the lightest binary. Table fidelity is lower than MuPDF/poppler; it is the
// zero-dependency fallback and the cross-compile-friendly option.
type PureGo struct{}

// Name implements Extractor.
func (PureGo) Name() string { return "purego" }

// Extract implements Extractor.
func (PureGo) Extract(path string) ([]Page, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("purego: open %q: %w", path, err)
	}
	defer f.Close()

	n := r.NumPage()
	pages := make([]Page, 0, n)
	for i := 1; i <= n; i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			pages = append(pages, Page{Index: i})
			continue
		}
		text, err := p.GetPlainText(make(map[string]*pdf.Font))
		if err != nil {
			return nil, fmt.Errorf("purego: page %d: %w", i, err)
		}
		pages = append(pages, Page{Index: i, Text: text})
	}
	return pages, nil
}
