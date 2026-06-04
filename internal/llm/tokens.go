package llm

import (
	"fmt"
	"strings"
)

// TokenCounter estimates the token length of text.
type TokenCounter interface {
	Count(text string) int
}

// HeuristicCounter approximates tokens as ceil(runes/4). It is offline and
// deterministic; a tiktoken-backed counter can replace it without changing
// callers (it satisfies the same interface).
type HeuristicCounter struct{}

// Count implements TokenCounter.
func (HeuristicCounter) Count(text string) int {
	n := len([]rune(text))
	return (n + 3) / 4
}

// Page is one source page and its extracted text.
type Page struct {
	Index int // 1-based physical index
	Text  string
}

// PageGroup is a contiguous run of pages rendered for the LLM, with each page
// wrapped in <physical_index_N> markers so the model can cite physical indices.
type PageGroup struct {
	Start int // inclusive 1-based
	End   int // inclusive 1-based
	Text  string
}

// WrapPage renders one page with its physical-index markers.
func WrapPage(p Page) string {
	return fmt.Sprintf("<physical_index_%d>\n%s\n<physical_index_%d>\n\n", p.Index, p.Text, p.Index)
}

// GroupPages splits pages into token-bounded groups (each <= maxTokens where a
// single page allows it), overlapping consecutive groups by `overlap` pages.
// Mirrors PageIndex page_list_to_group_text. Always makes forward progress, so a
// single oversized page forms its own group.
func GroupPages(pages []Page, tc TokenCounter, maxTokens, overlap int) []PageGroup {
	if len(pages) == 0 {
		return nil
	}
	if overlap < 0 {
		overlap = 0
	}
	var groups []PageGroup
	for i := 0; i < len(pages); {
		start := i
		var sb strings.Builder
		tokens := 0
		j := i
		for j < len(pages) {
			wrapped := WrapPage(pages[j])
			t := tc.Count(wrapped)
			if j > start && tokens+t > maxTokens {
				break
			}
			sb.WriteString(wrapped)
			tokens += t
			j++
		}
		groups = append(groups, PageGroup{Start: pages[start].Index, End: pages[j-1].Index, Text: sb.String()})
		if j >= len(pages) {
			break
		}
		next := j - overlap
		if next <= start {
			next = start + 1 // guarantee progress
		}
		i = next
	}
	return groups
}
