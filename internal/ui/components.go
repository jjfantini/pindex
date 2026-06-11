package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	ltree "github.com/charmbracelet/lipgloss/tree"

	ptree "github.com/jjfantini/pindex/internal/tree"
)

// SummaryBox renders a rounded, brand-bordered box of key→value rows — the
// end-of-command receipt (doc id, pages, sections, output paths). Returns the
// rendered block; the caller prints it to the right stream.
func (u *UI) SummaryBox(title string, rows [][2]string) string {
	keyW := 0
	for _, r := range rows {
		if len(r[0]) > keyW {
			keyW = len(r[0])
		}
	}
	lines := make([]string, 0, len(rows)+1)
	if title != "" {
		lines = append(lines, u.st.BoxTitle.Render(title))
	}
	for _, r := range rows {
		lines = append(lines, u.st.Dim.Render(fmt.Sprintf("%-*s", keyW, r[0]))+"  "+r[1])
	}
	return u.st.BoxBorder.Render(strings.Join(lines, "\n"))
}

// Table renders headers+rows with rounded borders and a brand header row.
func (u *UI) Table(headers []string, rows [][]string) string {
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(u.st.TableBorder).
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == table.HeaderRow {
				return u.st.TableHeader
			}
			return u.re.NewStyle().Padding(0, 1)
		}).
		Headers(headers...).
		Rows(rows...)
	return t.Render()
}

// DocTree renders a compact preview of an indexed document's section tree:
// the doc name as root, section titles with their page spans, truncated after
// maxNodes sections so a 500-section filing stays a glanceable receipt.
func (u *UI) DocTree(docName string, nodes []ptree.TreeNode, maxNodes int) string {
	root := ltree.Root(u.st.TreeRoot.Render(docName)).
		EnumeratorStyle(u.st.TreeBranches)
	budget := maxNodes
	total := countNodes(nodes)
	u.addTreeChildren(root, nodes, &budget)
	if total > maxNodes {
		root.Child(u.st.Dim.Render(fmt.Sprintf("… +%d more sections", total-maxNodes)))
	}
	return root.String()
}

func (u *UI) addTreeChildren(parent *ltree.Tree, nodes []ptree.TreeNode, budget *int) {
	for i := range nodes {
		if *budget <= 0 {
			return
		}
		*budget--
		label := u.st.TreeItem.Render(nodes[i].Title) + " " + u.st.Dim.Render(pageSpan(nodes[i]))
		if len(nodes[i].Nodes) == 0 {
			parent.Child(label)
			continue
		}
		sub := ltree.Root(label).EnumeratorStyle(u.st.TreeBranches)
		u.addTreeChildren(sub, nodes[i].Nodes, budget)
		parent.Child(sub)
	}
}

func pageSpan(n ptree.TreeNode) string {
	if n.StartIndex == n.EndIndex {
		return fmt.Sprintf("p.%d", n.StartIndex)
	}
	return fmt.Sprintf("p.%d–%d", n.StartIndex, n.EndIndex)
}

func countNodes(nodes []ptree.TreeNode) int {
	n := len(nodes)
	for i := range nodes {
		n += countNodes(nodes[i].Nodes)
	}
	return n
}
