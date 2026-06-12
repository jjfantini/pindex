// Package retrieve implements the three retrieval tools the ask loop drives over
// a stored document: metadata, the text-stripped structure, and targeted page
// content. Ported from PageIndex's retrieve.py.
package retrieve

import (
	"encoding/json"

	"github.com/jjfantini/pindex/internal/tree"
)

// GetDocument returns metadata JSON: id, name, description, type, status, and
// page_count (PDF) or line_count (Markdown).
func GetDocument(doc tree.Document) (string, error) {
	meta := map[string]any{
		"doc_id":          doc.ID,
		"doc_name":        doc.DocName,
		"doc_description": doc.DocDescription,
		"type":            string(doc.Type),
		"status":          "completed",
	}
	if doc.Type == tree.DocMD {
		meta["line_count"] = doc.LineCount
	} else {
		pc := doc.PageCount
		if pc == 0 {
			pc = len(doc.Pages)
		}
		meta["page_count"] = pc
	}
	b, err := json.Marshal(meta)
	return string(b), err
}

// GetStructure returns the tree with all text stripped, rendered as JSON — the
// token-cheap view the LLM reasons over.
func GetStructure(doc tree.Document) (string, error) {
	return (tree.JSONRenderer{}).Render(tree.StripText(doc.Structure))
}

// GetStructureWithin is GetStructure under a size budget in characters
// (~4 per token): an oversized structure degrades its node summaries
// (truncated, then stripped) instead of overflowing the model's context
// window. The returned StructureFit reports the degradation applied.
func GetStructureWithin(doc tree.Document, budget int) (string, tree.StructureFit, error) {
	return tree.RenderStructureWithin(tree.JSONRenderer{}, doc.Structure, budget)
}

// Pages returns the content of the requested pages (parsed from a selector like
// "5-7,12"), in page order, skipping any not present.
func Pages(doc tree.Document, pages string) ([]tree.PageContent, error) {
	nums, err := tree.ParsePages(pages)
	if err != nil {
		return nil, err
	}
	byPage := make(map[int]string, len(doc.Pages))
	for _, p := range doc.Pages {
		byPage[p.Page] = p.Content
	}
	out := make([]tree.PageContent, 0, len(nums))
	for _, n := range nums {
		if c, ok := byPage[n]; ok {
			out = append(out, tree.PageContent{Page: n, Content: c})
		}
	}
	return out, nil
}

// GetPageContent returns the requested pages as JSON [{page, content}].
func GetPageContent(doc tree.Document, pages string) (string, error) {
	pcs, err := Pages(doc, pages)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(pcs)
	return string(b), err
}
