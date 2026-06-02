//go:build !cgo

package extract

import "errors"

// MuPDFAvailable is false in CGO_ENABLED=0 (static) builds: go-fitz is excluded
// so the binary stays static and does not panic at init. Use the purego or
// poppler backend in such builds.
const MuPDFAvailable = false

var errMUPDFUnavailable = errors.New(
	"extract: the mupdf backend needs a cgo build; rebuild with CGO_ENABLED=1, " +
		"or set the extractor to purego/poppler")

func newMUPDF() (Extractor, error) { return nil, errMUPDFUnavailable }
