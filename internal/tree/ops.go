package tree

import (
	"fmt"
	"strings"
)

// FlatItem is one entry of a flat, pre-tree section list: a dotted hierarchy
// code (e.g. "1", "1.2", "1.2.3") plus the resolved page span.
type FlatItem struct {
	Structure  string
	Title      string
	StartIndex int
	EndIndex   int
}

// PostItem is a verified TOC entry before page spans are resolved. PhysicalIndex
// is the section's start page; AppearStart reports whether the section begins at
// the very top of that page (which shifts the previous section's end by one).
type PostItem struct {
	Structure     string
	Title         string
	PhysicalIndex int
	AppearStart   bool
}

// parentStructure returns the dotted code of the parent ("1.2.3" -> "1.2"),
// or "" for a top-level code.
func parentStructure(s string) string {
	if s == "" {
		return ""
	}
	parts := strings.Split(s, ".")
	if len(parts) > 1 {
		return strings.Join(parts[:len(parts)-1], ".")
	}
	return ""
}

// ListToTree nests a flat list into a hierarchy using the dotted Structure codes.
// A node is attached to its parent only if the parent was already seen; otherwise
// it becomes a root (faithful to PageIndex's order-dependent list_to_tree). Leaf
// nodes carry a nil Nodes slice (which omits "nodes" from JSON).
func ListToTree(items []FlatItem) []TreeNode {
	type bnode struct {
		node     TreeNode
		children []*bnode
	}
	index := make(map[string]*bnode, len(items))
	var roots []*bnode

	for _, it := range items {
		bn := &bnode{node: TreeNode{Title: it.Title, StartIndex: it.StartIndex, EndIndex: it.EndIndex}}
		index[it.Structure] = bn
		if p := parentStructure(it.Structure); p != "" {
			if parent, ok := index[p]; ok {
				parent.children = append(parent.children, bn)
				continue
			}
		}
		roots = append(roots, bn)
	}

	var build func(*bnode) TreeNode
	build = func(b *bnode) TreeNode {
		n := b.node
		if len(b.children) > 0 {
			n.Nodes = make([]TreeNode, len(b.children))
			for i, c := range b.children {
				n.Nodes[i] = build(c)
			}
		}
		return n
	}
	out := make([]TreeNode, 0, len(roots))
	for _, r := range roots {
		out = append(out, build(r))
	}
	return out
}

// PostProcess resolves each item's [StartIndex, EndIndex] page span from the
// physical indices, then nests via ListToTree. An item ends one page before the
// next when the next section starts at the top of its page, else on that page;
// the last item ends at endPhysicalIndex. Mirrors PageIndex post_processing.
func PostProcess(items []PostItem, endPhysicalIndex int) []TreeNode {
	flat := make([]FlatItem, len(items))
	for i, it := range items {
		end := endPhysicalIndex
		if i < len(items)-1 {
			next := items[i+1]
			if next.AppearStart {
				end = next.PhysicalIndex - 1
			} else {
				end = next.PhysicalIndex
			}
		}
		flat[i] = FlatItem{Structure: it.Structure, Title: it.Title, StartIndex: it.PhysicalIndex, EndIndex: end}
	}
	return ListToTree(flat)
}

// WriteNodeIDs assigns sequential zero-padded 4-digit IDs in pre-order DFS
// starting at 0 ("0000", "0001", ...), mutating the tree in place. Mirrors
// PageIndex write_node_id.
func WriteNodeIDs(nodes []TreeNode) {
	n := 0
	var dfs func([]TreeNode)
	dfs = func(ns []TreeNode) {
		for i := range ns {
			ns[i].NodeID = fmt.Sprintf("%04d", n)
			n++
			dfs(ns[i].Nodes)
		}
	}
	dfs(nodes)
}

// StripText returns a deep copy of the tree with every Text field cleared,
// leaving the input untouched. This is the get_document_structure view
// (remove_fields(['text'])) used to hand the LLM structure without page text.
func StripText(nodes []TreeNode) []TreeNode {
	if nodes == nil {
		return nil
	}
	out := make([]TreeNode, len(nodes))
	for i, n := range nodes {
		n.Text = ""
		n.Nodes = StripText(n.Nodes)
		out[i] = n
	}
	return out
}
