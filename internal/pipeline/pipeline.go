// Package pipeline indexes documents: a single-file orchestration (extract ->
// build -> persist) and a bounded-concurrency, resumable batch over a directory.
// Parallelism is ACROSS documents (a single doc's extraction stays sequential,
// per the go-fitz handle constraint).
package pipeline

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/jjfantini/pindex/internal/extract"
	"github.com/jjfantini/pindex/internal/index"
	"github.com/jjfantini/pindex/internal/store"
	"github.com/jjfantini/pindex/internal/tree"
)

// FileIndexer indexes one file end to end and (if Store is set) persists it.
type FileIndexer struct {
	Builder   *index.Builder
	Extractor extract.Extractor
	Store     *store.Store
}

// IndexOne extracts, builds, assembles, and saves one PDF, returning the Document.
func (f *FileIndexer) IndexOne(ctx context.Context, path string) (tree.Document, error) {
	pages, err := f.Extractor.Extract(path)
	if err != nil {
		return tree.Document{}, fmt.Errorf("extract %s: %w", path, err)
	}
	res, err := f.Builder.Build(ctx, pages)
	if err != nil {
		return tree.Document{}, fmt.Errorf("build %s: %w", path, err)
	}
	doc := assembleDoc(path, pages, res)
	if f.Store != nil {
		if err := f.Store.Save(doc); err != nil {
			return tree.Document{}, fmt.Errorf("save %s: %w", path, err)
		}
	}
	return doc, nil
}

func assembleDoc(path string, pages []extract.Page, res index.Result) tree.Document {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	pcs := make([]tree.PageContent, len(pages))
	for i, p := range pages {
		pcs[i] = tree.PageContent{Page: p.Index, Content: p.Text}
	}
	return tree.Document{
		ID:             store.DocID(path),
		Type:           tree.DocPDF,
		Path:           abs,
		DocName:        filepath.Base(path),
		DocDescription: res.Description,
		PageCount:      len(pages),
		Structure:      res.Structure,
		Pages:          pcs,
		PageOffset:     res.PageOffset,
		PageMap:        tree.BuildPageMap(pcs),
	}
}

// Result reports the outcome of indexing one file in a batch.
type Result struct {
	Path    string
	DocID   string
	Skipped bool
	Err     error
}

// BatchIndex indexes paths with bounded concurrency, skipping already-indexed
// documents unless force is set (resumability). One file's failure does not abort
// the batch — errors are collected per Result. progress, if non-nil, is invoked
// per file as it finishes (serialized, safe to print from).
func BatchIndex(ctx context.Context, f *FileIndexer, paths []string, concurrency int, force bool, progress func(Result)) []Result {
	if concurrency < 1 {
		concurrency = 1
	}
	results := make([]Result, len(paths))
	var mu sync.Mutex
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrency)
	for i := range paths {
		g.Go(func() error {
			r := Result{Path: paths[i], DocID: store.DocID(paths[i])}
			switch {
			case !force && f.Store != nil && f.Store.Has(r.DocID):
				r.Skipped = true
			default:
				_, r.Err = f.IndexOne(ctx, paths[i])
			}
			results[i] = r
			if progress != nil {
				mu.Lock()
				progress(r)
				mu.Unlock()
			}
			return nil // never abort the whole batch on a single failure
		})
	}
	_ = g.Wait()
	return results
}

// FindPDFs returns all .pdf files under dir (recursive), sorted.
func FindPDFs(dir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(path), ".pdf") {
			out = append(out, path)
		}
		return nil
	})
	return out, err
}

// Summarize counts batch outcomes.
func Summarize(results []Result) (indexed, skipped, failed int) {
	for _, r := range results {
		switch {
		case r.Err != nil:
			failed++
		case r.Skipped:
			skipped++
		default:
			indexed++
		}
	}
	return
}
