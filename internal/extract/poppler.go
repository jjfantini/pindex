package extract

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Poppler extracts text by shelling out to `pdftotext -layout` (poppler-utils).
// -layout preserves column/table geometry, which helps dense financial tables.
// pdftotext separates pages with a form-feed (\f).
type Poppler struct{}

// Name implements Extractor.
func (Poppler) Name() string { return "poppler" }

// Available reports whether the pdftotext binary is on PATH.
func (Poppler) Available() bool {
	_, err := exec.LookPath("pdftotext")
	return err == nil
}

// Extract implements Extractor.
func (Poppler) Extract(path string) ([]Page, error) {
	var out, stderr bytes.Buffer
	cmd := exec.Command("pdftotext", "-layout", path, "-")
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("poppler: pdftotext failed (%w): %s — is poppler-utils installed?", err, strings.TrimSpace(stderr.String()))
	}
	chunks := strings.Split(out.String(), "\f")
	pages := make([]Page, 0, len(chunks))
	for i, c := range chunks {
		// pdftotext emits a trailing form-feed after the last page.
		if i == len(chunks)-1 && strings.TrimSpace(c) == "" {
			break
		}
		pages = append(pages, Page{Index: i + 1, Text: c})
	}
	return pages, nil
}
