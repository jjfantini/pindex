package extract

import (
	"strings"
	"testing"
)

const sample = "../../testdata/sample.pdf"

// wantTerms are strings the fixture PDF is known to contain (page 1 + page 2).
var wantTerms = []string{"pindex", "extraction", "second", "1234"}

func assertSample(t *testing.T, ex Extractor) {
	t.Helper()
	pages, err := ex.Extract(sample)
	if err != nil {
		t.Fatalf("%s: %v", ex.Name(), err)
	}
	if len(pages) != 2 {
		t.Fatalf("%s: pages=%d want 2", ex.Name(), len(pages))
	}
	if pages[0].Index != 1 || pages[1].Index != 2 {
		t.Errorf("%s: page indices = %d,%d want 1,2", ex.Name(), pages[0].Index, pages[1].Index)
	}
	joined := strings.ToLower(pages[0].Text + " " + pages[1].Text)
	for _, w := range wantTerms {
		if !strings.Contains(joined, w) {
			t.Errorf("%s: extracted text missing %q\ngot: %q", ex.Name(), w, joined)
		}
	}
}

func TestPureGoExtract(t *testing.T) { assertSample(t, PureGo{}) }

func TestPopplerExtract(t *testing.T) {
	if !(Poppler{}).Available() {
		t.Skip("pdftotext (poppler-utils) not installed")
	}
	assertSample(t, Poppler{})
}

func TestNewBackends(t *testing.T) {
	for _, b := range []string{"purego", "poppler"} {
		if _, err := New(b); err != nil {
			t.Errorf("New(%q): unexpected error %v", b, err)
		}
	}
	// mupdf (and the "" default, which maps to mupdf) is only present in cgo builds.
	if _, err := New("mupdf"); (err == nil) != MuPDFAvailable {
		t.Errorf("New(\"mupdf\") err=%v but MuPDFAvailable=%v", err, MuPDFAvailable)
	}
	if _, err := New(""); (err == nil) != MuPDFAvailable {
		t.Errorf("New(\"\") err=%v but MuPDFAvailable=%v", err, MuPDFAvailable)
	}
	if _, err := New("vision"); err == nil {
		t.Error("vision backend should report unimplemented")
	}
	if _, err := New("bogus"); err == nil {
		t.Error("unknown backend should error")
	}
}
