// Package tree defines pindex's core index data structures and (in later phases)
// the pure tree operations that build and transform them. The JSON shapes here
// mirror the verified upstream PageIndex schema so indexes are interchangeable.
package tree

// TreeNode is one section in a document's hierarchical index. start_index and
// end_index are 1-based physical page numbers (PDF) or line numbers (Markdown).
type TreeNode struct {
	Title      string     `json:"title"`
	NodeID     string     `json:"node_id,omitempty"`
	StartIndex int        `json:"start_index"`
	EndIndex   int        `json:"end_index"`
	Summary    string     `json:"summary,omitempty"`
	Text       string     `json:"text,omitempty"`
	Nodes      []TreeNode `json:"nodes,omitempty"`
}

// PageContent is the extracted text for a single page (PDF) or header line (MD).
type PageContent struct {
	Page    int    `json:"page"`
	Content string `json:"content"`
}

// DocType enumerates the supported source document kinds.
type DocType string

const (
	DocPDF DocType = "pdf"
	DocMD  DocType = "md"
)

// Document is the full persisted record for one indexed file
// (the per-doc {doc_id}.json blob).
type Document struct {
	ID             string        `json:"id"`
	Type           DocType       `json:"type"`
	Path           string        `json:"path"`
	DocName        string        `json:"doc_name"`
	DocDescription string        `json:"doc_description,omitempty"`
	PageCount      int           `json:"page_count,omitempty"`
	LineCount      int           `json:"line_count,omitempty"`
	Structure      []TreeNode    `json:"structure"`
	Pages          []PageContent `json:"pages,omitempty"`
	// PageOffset maps a printed page label to its physical page index
	// (physical = printed + PageOffset), recovered from a page-numbered table of
	// contents. Zero when unknown / no TOC was used.
	PageOffset int `json:"page_offset,omitempty"`
	// PageMap is a piecewise physical-to-printed page map recovered from printed
	// footer anchors. It is omitted when no reliable runs were found.
	PageMap PageMap `json:"page_map,omitempty"`
}

// CatalogEntry is the lightweight per-document record kept in the workspace
// catalog for lazy loading and corpus-level routing.
type CatalogEntry struct {
	Type           DocType `json:"type"`
	DocName        string  `json:"doc_name"`
	DocDescription string  `json:"doc_description,omitempty"`
	Path           string  `json:"path"`
	PageCount      int     `json:"page_count,omitempty"`
	LineCount      int     `json:"line_count,omitempty"`
}
