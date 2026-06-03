package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jjfantini/pindex/internal/config"
	"github.com/jjfantini/pindex/internal/extract"
	"github.com/jjfantini/pindex/internal/index"
	"github.com/jjfantini/pindex/internal/llm"
	"github.com/jjfantini/pindex/internal/store"
)

// routingProvider answers by request content, so it is correct under any
// concurrency or call order (unlike an ordered scripted mock).
type routingProvider struct {
	mu    sync.Mutex
	calls int
}

func (p *routingProvider) Name() string { return "routing" }

func (p *routingProvider) Complete(_ context.Context, req llm.Request) (llm.Response, error) {
	p.mu.Lock()
	p.calls++
	p.mu.Unlock()
	content := req.Messages[len(req.Messages)-1].Content
	switch {
	case strings.Contains(content, "tree structure"):
		return llm.Response{Content: `[{"structure":"1","title":"sample","physical_index":"<physical_index_1>"}]`}, nil
	case strings.Contains(content, "start_begin"):
		return llm.Response{Content: `{"start_begin":"yes"}`}, nil
	default:
		return llm.Response{Content: `{}`}, nil
	}
}

func copyPDF(t *testing.T, dst string) {
	t.Helper()
	data, err := os.ReadFile("../../testdata/sample.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func newFileIndexer(t *testing.T) (*FileIndexer, *store.Store) {
	t.Helper()
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &FileIndexer{
		Builder:   index.NewBuilder(config.Default(), &routingProvider{}),
		Extractor: extract.PureGo{}, // pure-Go: safe across docs and all build modes
		Store:     s,
	}, s
}

func TestBatchIndexConcurrentResumeAndFailure(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.pdf")
	b := filepath.Join(dir, "b.pdf")
	copyPDF(t, a)
	copyPDF(t, b)
	bad := filepath.Join(dir, "missing.pdf")

	fi, s := newFileIndexer(t)
	defer func() { _ = s.Close() }()

	results := BatchIndex(context.Background(), fi, []string{a, b, bad}, 2, false, nil)
	indexed, skipped, failed := Summarize(results)
	if indexed != 2 || skipped != 0 || failed != 1 {
		t.Fatalf("first run: indexed=%d skipped=%d failed=%d want 2/0/1", indexed, skipped, failed)
	}
	if !s.Has(store.DocID(a)) || !s.Has(store.DocID(b)) {
		t.Error("indexed docs should be in the catalog")
	}

	// Re-run without force: already-indexed docs are skipped (resume).
	results = BatchIndex(context.Background(), fi, []string{a, b, bad}, 2, false, nil)
	indexed, skipped, failed = Summarize(results)
	if indexed != 0 || skipped != 2 || failed != 1 {
		t.Fatalf("resume run: indexed=%d skipped=%d failed=%d want 0/2/1", indexed, skipped, failed)
	}
}

func TestFindPDFs(t *testing.T) {
	dir := t.TempDir()
	copyPDF(t, filepath.Join(dir, "x.pdf"))
	copyPDF(t, filepath.Join(dir, "y.PDF"))
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := FindPDFs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("FindPDFs found %d, want 2 (case-insensitive .pdf, ignore .txt)", len(got))
	}
}
