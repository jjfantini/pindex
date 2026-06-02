//go:build cgo

package extract

import "testing"

func TestMuPDFExtract(t *testing.T) { assertSample(t, MuPDF{}) }
