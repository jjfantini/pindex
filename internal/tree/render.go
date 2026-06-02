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
