package tree

import "encoding/json"

// Renderer serializes a node tree for handing to an LLM. It is the seam where a
// future TOON renderer can replace JSON without touching callers (v2). JSON is
// canonical on disk and the default on-wire format for v1.
type Renderer interface {
	Render(nodes []TreeNode) (string, error)
}

// JSONRenderer renders the tree as JSON. With Indent set, output is pretty-printed.
type JSONRenderer struct {
	Indent bool
}

// Render implements Renderer.
func (r JSONRenderer) Render(nodes []TreeNode) (string, error) {
	var (
		b   []byte
		err error
	)
	if r.Indent {
		b, err = json.MarshalIndent(nodes, "", "  ")
	} else {
		b, err = json.Marshal(nodes)
	}
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// RenderStructure renders the tree for the LLM with page text stripped
// (the get_document_structure view), using the given renderer.
func RenderStructure(r Renderer, nodes []TreeNode) (string, error) {
	return r.Render(StripText(nodes))
}

// StructureFit reports how much summary detail survived a budgeted render.
type StructureFit int

const (
	FitFull               StructureFit = iota // everything fit untouched
	FitTruncatedSummaries                     // summaries cut to truncatedSummaryRunes
	FitTitlesOnly                             // summaries stripped entirely
)

// truncatedSummaryRunes is the per-node summary length tried before summaries
// are dropped entirely — enough to keep each section recognizable while
// shrinking multi-paragraph summaries by an order of magnitude.
const truncatedSummaryRunes = 280

// RenderStructureWithin renders the text-stripped tree within budget
// characters, degrading gracefully instead of overflowing the model context:
// full summaries first, then summaries truncated to truncatedSummaryRunes
// runes, then titles only. The titles-only floor is returned even if it still
// exceeds budget — that pathological case is left to surface as the provider's
// typed too-long error rather than being masked here.
func RenderStructureWithin(r Renderer, nodes []TreeNode, budget int) (string, StructureFit, error) {
	stripped := StripText(nodes)
	out, err := r.Render(stripped)
	if err != nil || len(out) <= budget {
		return out, FitFull, err
	}
	out, err = r.Render(TruncateSummaries(stripped, truncatedSummaryRunes))
	if err != nil || len(out) <= budget {
		return out, FitTruncatedSummaries, err
	}
	out, err = r.Render(StripSummaries(stripped))
	return out, FitTitlesOnly, err
}
