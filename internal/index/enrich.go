package index

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/jjfantini/pindex/internal/extract"
	"github.com/jjfantini/pindex/internal/llm"
	"github.com/jjfantini/pindex/internal/prompts"
	"github.com/jjfantini/pindex/internal/tree"
)

func (b *Builder) concurrency() int {
	if b.Concurrency < 1 {
		return 1
	}
	return b.Concurrency
}

// markAppearStart sets each item's appearStart by asking, per section, whether it
// begins at the top of its page. Bounded-concurrent; each goroutine writes a
// distinct slice element.
func (b *Builder) markAppearStart(ctx context.Context, items []item, pages []extract.Page) error {
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(b.concurrency())
	for i := range items {
		g.Go(func() error {
			txt, ok := pageText(pages, items[i].physicalIdx)
			if !ok {
				items[i].appearStart = false
				return nil
			}
			resp, err := llm.CompleteJSON[prompts.StartBegin](ctx, b.provider,
				llm.UserPrompt(b.cfg.Model, prompts.CheckTitleAppearanceInStart(items[i].title, txt)),
				b.StructuredAttempts, nil)
			if err != nil {
				return fmt.Errorf("index: appear-start %q: %w", items[i].title, err)
			}
			items[i].appearStart = strings.EqualFold(strings.TrimSpace(resp.StartBegin), "yes")
			return nil
		})
	}
	return g.Wait()
}

// splitLargeNodes recursively re-indexes nodes that span too many pages and tokens,
// guarded by MaxRecursionDepth (the Python original had no guard).
func (b *Builder) splitLargeNodes(ctx context.Context, nodes []tree.TreeNode, pages []extract.Page, depth int) error {
	for i := range nodes {
		if depth < b.MaxRecursionDepth && len(nodes[i].Nodes) == 0 {
			sub := pagesInRange(pages, nodes[i].StartIndex, nodes[i].EndIndex)
			span := nodes[i].EndIndex - nodes[i].StartIndex
			if span > b.cfg.MaxPageNumEachNode && b.tokensOf(sub) >= b.cfg.MaxTokenNumEachNode && len(sub) > 1 {
				children, err := b.buildSubtree(ctx, &nodes[i], sub)
				if err != nil {
					return err
				}
				nodes[i].Nodes = children
			}
		}
		if len(nodes[i].Nodes) > 0 {
			if err := b.splitLargeNodes(ctx, nodes[i].Nodes, pages, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

// buildSubtree re-runs structure generation on a single node's page range and
// returns its children, mirroring process_large_node_recursively (including the
// "first child repeats the parent title" de-duplication).
func (b *Builder) buildSubtree(ctx context.Context, node *tree.TreeNode, sub []extract.Page) ([]tree.TreeNode, error) {
	raw, err := b.generateStructure(ctx, sub)
	if err != nil {
		return nil, err
	}
	items := b.resolveAndFilter(raw, sub)
	if len(items) == 0 {
		return nil, nil
	}
	if err := b.markAppearStart(ctx, items, sub); err != nil {
		return nil, err
	}

	origEnd := node.EndIndex
	matchFirst := strings.TrimSpace(node.Title) == strings.TrimSpace(items[0].title)
	childItems := items
	if matchFirst {
		childItems = items[1:]
	}
	children := tree.PostProcess(toPostItems(childItems), origEnd)

	switch {
	case matchFirst && len(items) > 1:
		node.EndIndex = items[1].physicalIdx
	case !matchFirst:
		node.EndIndex = items[0].physicalIdx
	}
	return children, nil
}

// addSummaries sets a Summary on every node: its text verbatim when short, else
// an LLM-generated description. Bounded-concurrent over a flattened node list.
func (b *Builder) addSummaries(ctx context.Context, nodes []tree.TreeNode, pages []extract.Page) error {
	flat := flattenPtrs(nodes)
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(b.concurrency())
	for _, n := range flat {
		g.Go(func() error {
			text := concatPages(pages, n.StartIndex, n.EndIndex)
			if b.counter.Count(text) < b.SummaryTokenThreshold {
				n.Summary = text
				return nil
			}
			resp, err := b.provider.Complete(ctx, llm.UserPrompt(b.cfg.Model, prompts.NodeSummary(text)))
			if err != nil {
				return fmt.Errorf("index: summary %q: %w", n.Title, err)
			}
			n.Summary = strings.TrimSpace(resp.Content)
			return nil
		})
	}
	return g.Wait()
}

// docDescription asks for a one-sentence document description from the
// text-stripped structure.
func (b *Builder) docDescription(ctx context.Context, nodes []tree.TreeNode) (string, error) {
	structure, err := b.renderer.Render(tree.StripText(nodes))
	if err != nil {
		return "", err
	}
	resp, err := b.provider.Complete(ctx, llm.UserPrompt(b.cfg.Model, prompts.DocDescription(structure)))
	if err != nil {
		return "", fmt.Errorf("index: doc description: %w", err)
	}
	return strings.TrimSpace(resp.Content), nil
}

// --- helpers ---------------------------------------------------------------

func (b *Builder) tokensOf(pages []extract.Page) int {
	total := 0
	for _, p := range pages {
		total += b.counter.Count(p.Text)
	}
	return total
}

func pagesInRange(pages []extract.Page, start, end int) []extract.Page {
	var out []extract.Page
	for _, p := range pages {
		if p.Index >= start && p.Index <= end {
			out = append(out, p)
		}
	}
	return out
}

func concatPages(pages []extract.Page, start, end int) string {
	var sb strings.Builder
	for _, p := range pages {
		if p.Index >= start && p.Index <= end {
			sb.WriteString(p.Text)
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// flattenPtrs returns pre-order pointers into the tree so callers can mutate nodes
// in place. Valid as long as the tree's slices are not appended to concurrently.
func flattenPtrs(nodes []tree.TreeNode) []*tree.TreeNode {
	var out []*tree.TreeNode
	var dfs func([]tree.TreeNode)
	dfs = func(ns []tree.TreeNode) {
		for i := range ns {
			out = append(out, &ns[i])
			dfs(ns[i].Nodes)
		}
	}
	dfs(nodes)
	return out
}
