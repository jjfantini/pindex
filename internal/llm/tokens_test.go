package llm

import (
	"strings"
	"testing"
)

func TestHeuristicCounter(t *testing.T) {
	c := HeuristicCounter{}
	if c.Count("") != 0 {
		t.Errorf("empty=%d want 0", c.Count(""))
	}
	if c.Count("abcd") != 1 {
		t.Errorf("abcd=%d want 1", c.Count("abcd"))
	}
	if c.Count("abcde") != 2 {
		t.Errorf("abcde=%d want 2", c.Count("abcde"))
	}
}

func TestGroupPagesBoundsAndOverlap(t *testing.T) {
	pages := []Page{{1, "aaaa"}, {2, "bbbb"}, {3, "cccc"}, {4, "dddd"}}
	groups := GroupPages(pages, HeuristicCounter{}, 30, 1)
	if len(groups) < 2 {
		t.Fatalf("expected multiple groups, got %d", len(groups))
	}
	if !strings.Contains(groups[0].Text, "<physical_index_1>") {
		t.Error("group text should carry physical-index markers")
	}
	if groups[1].Start != groups[0].End {
		t.Errorf("overlap=1: group1.Start=%d should equal group0.End=%d", groups[1].Start, groups[0].End)
	}
	if groups[len(groups)-1].End != 4 {
		t.Errorf("last group End=%d want 4 (full coverage)", groups[len(groups)-1].End)
	}
}

func TestGroupPagesSingleOversizedPage(t *testing.T) {
	pages := []Page{{1, strings.Repeat("x", 1000)}}
	groups := GroupPages(pages, HeuristicCounter{}, 5, 1)
	if len(groups) != 1 || groups[0].Start != 1 || groups[0].End != 1 {
		t.Errorf("oversized single page should form one group, got %+v", groups)
	}
}

func TestGroupPagesEmpty(t *testing.T) {
	if GroupPages(nil, HeuristicCounter{}, 100, 1) != nil {
		t.Error("empty input should yield nil")
	}
}
