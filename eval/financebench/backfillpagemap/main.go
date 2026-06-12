// Command backfillpagemap recomputes piecewise printed-page maps from already
// extracted FinanceBench workspace docs and refreshes derived page-alignment
// fields in answer records and tree exports. It does not re-index or call LLMs.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jjfantini/pindex/eval/financebench"
	"github.com/jjfantini/pindex/internal/exportout"
	"github.com/jjfantini/pindex/internal/tree"
)

func main() {
	workspace := flag.String("workspace", "eval/financebench/testdata/ws", "FinanceBench pindex workspace")
	results := flag.String("results", "eval/financebench/results", "FinanceBench results directory")
	skipAggregate := flag.Bool("skip-aggregate", false, "skip go run ./eval/financebench/aggregate after backfill")
	flag.Parse()

	docs, err := backfillWorkspaceDocs(*workspace)
	if err != nil {
		fatal(err)
	}
	if err := backfillTreeExports(*results, docs); err != nil {
		fatal(err)
	}
	if err := backfillAnswerRecords(*results, docs); err != nil {
		fatal(err)
	}
	if !*skipAggregate {
		if err := runAggregate(*results); err != nil {
			fatal(err)
		}
	}
}

func backfillWorkspaceDocs(workspace string) (map[string]tree.Document, error) {
	files, err := filepath.Glob(filepath.Join(workspace, "docs", "*.json"))
	if err != nil {
		return nil, err
	}
	docs := make(map[string]tree.Document, len(files))
	for _, path := range files {
		var doc tree.Document
		if err := readJSONFile(path, &doc); err != nil {
			return nil, fmt.Errorf("read doc %s: %w", path, err)
		}
		doc.PageMap = tree.BuildPageMap(doc.Pages)
		if err := writeJSONFile(path, doc); err != nil {
			return nil, fmt.Errorf("write doc %s: %w", path, err)
		}
		docs[docKey(doc.DocName)] = doc
	}
	return docs, nil
}

func backfillAnswerRecords(results string, docs map[string]tree.Document) error {
	return filepath.WalkDir(results, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".json" || filepath.Base(filepath.Dir(path)) != "answers" {
			return nil
		}
		var rec exportout.AnswerRecord
		if err := readJSONFile(path, &rec); err != nil {
			return fmt.Errorf("read answer %s: %w", path, err)
		}
		doc, ok := docs[docKey(rec.DocName)]
		if !ok {
			return fmt.Errorf("answer %s references unknown doc %q", path, rec.DocName)
		}
		rec.CitedPagesPrinted = tree.PrintedPages(rec.CitedPages, doc.PageMap)
		rec.PageHit = financebench.RecallAtPageMap(rec.GoldPages, rec.CitedPages, doc.PageMap, doc.PageOffset)
		if err := writeJSONFile(path, rec); err != nil {
			return fmt.Errorf("write answer %s: %w", path, err)
		}
		return nil
	})
}

func backfillTreeExports(results string, docs map[string]tree.Document) error {
	return filepath.WalkDir(results, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".json" || filepath.Base(filepath.Dir(path)) != "trees" {
			return nil
		}
		base := strings.TrimSuffix(filepath.Base(path), "_pindex.json")
		doc, ok := docs[docKey(base)]
		if !ok {
			return fmt.Errorf("tree export %s references unknown doc %q", path, base)
		}
		te := exportout.TreeExport{
			ID:          doc.ID,
			DocName:     doc.DocName,
			Type:        string(doc.Type),
			Description: doc.DocDescription,
			PageCount:   doc.PageCount,
			LineCount:   doc.LineCount,
			PageOffset:  doc.PageOffset,
			PageMap:     doc.PageMap,
			Structure:   tree.StripText(doc.Structure),
		}
		if err := writeJSONFile(path, te); err != nil {
			return fmt.Errorf("write tree export %s: %w", path, err)
		}
		return nil
	})
}

func docKey(name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	return exportout.Sanitize(base)
}

func runAggregate(results string) error {
	cmd := exec.Command("go", "run", "./eval/financebench/aggregate", results)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func readJSONFile(path string, v any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}

func writeJSONFile(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "backfillpagemap:", err)
	os.Exit(1)
}
