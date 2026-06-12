package tree

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// PageSegment maps a contiguous physical page range to printed page labels.
//
// Offset follows the existing pindex convention: physical = printed + Offset.
type PageSegment struct {
	PhysStart int `json:"phys_start"`
	PhysEnd   int `json:"phys_end"`
	Offset    int `json:"offset"`
}

// PageMap is a piecewise printed-page to physical-page map.
type PageMap []PageSegment

type pageAnchor struct {
	physical int
	printed  int
	offset   int
}

var printedFooterRE = regexp.MustCompile(`^\d{1,4}$`)

// BuildPageMap recovers a conservative piecewise printed-to-physical page map
// from extracted page text. Regex only extracts candidates; mappings require
// corroborated monotonic runs so stray table values do not silently become
// citations.
func BuildPageMap(pages []PageContent) PageMap {
	anchors := make([]pageAnchor, 0, len(pages))
	for _, p := range pages {
		printed, ok := printedFooterCandidate(p.Content)
		if !ok {
			continue
		}
		anchors = append(anchors, pageAnchor{
			physical: p.Page,
			printed:  printed,
			offset:   p.Page - printed,
		})
	}
	if len(anchors) < 2 {
		return nil
	}
	sort.Slice(anchors, func(i, j int) bool { return anchors[i].physical < anchors[j].physical })

	var out PageMap
	run := []pageAnchor{anchors[0]}
	for _, a := range anchors[1:] {
		if compatibleAnchor(run[len(run)-1], a) {
			run = append(run, a)
			continue
		}
		out = appendValidRun(out, run)
		run = []pageAnchor{a}
	}
	out = appendValidRun(out, run)
	if len(out) == 0 {
		return nil
	}
	return out
}

func compatibleAnchor(prev, next pageAnchor) bool {
	if prev.offset != next.offset {
		return false
	}
	physicalDelta := next.physical - prev.physical
	printedDelta := next.printed - prev.printed
	return physicalDelta > 0 && physicalDelta == printedDelta
}

func appendValidRun(out PageMap, run []pageAnchor) PageMap {
	if len(run) < 2 {
		return out
	}
	first := run[0]
	last := run[len(run)-1]
	return append(out, PageSegment{
		PhysStart: first.physical,
		PhysEnd:   last.physical,
		Offset:    first.offset,
	})
}

func printedFooterCandidate(text string) (int, bool) {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 1 || !printedFooterRE.MatchString(fields[0]) {
			return 0, false
		}
		n, err := strconv.Atoi(fields[0])
		return n, err == nil
	}
	return 0, false
}

// PrintedOf converts a physical PDF page to its printed page label.
func (m PageMap) PrintedOf(physical int) (int, bool) {
	for _, seg := range m {
		if physical >= seg.PhysStart && physical <= seg.PhysEnd {
			return physical - seg.Offset, true
		}
	}
	return 0, false
}

// PhysicalOf converts a printed page label to a physical PDF page. If a printed
// label appears in more than one segment, the longest validated segment wins.
func (m PageMap) PhysicalOf(printed int) (int, bool) {
	bestPhysical := 0
	bestLen := -1
	for _, seg := range m {
		physical := printed + seg.Offset
		if physical < seg.PhysStart || physical > seg.PhysEnd {
			continue
		}
		segLen := seg.PhysEnd - seg.PhysStart
		if segLen > bestLen {
			bestPhysical = physical
			bestLen = segLen
		}
	}
	return bestPhysical, bestLen >= 0
}

// FormatCitations renders human-facing citations while preserving physical PDF
// pages for lookup/debugging.
func FormatCitations(cited []int, m PageMap) string {
	if len(cited) == 0 {
		return ""
	}
	printed := make([]int, 0, len(cited))
	printedPhysical := make([]int, 0, len(cited))
	unmapped := make([]int, 0)
	for _, physical := range cited {
		if p, ok := m.PrintedOf(physical); ok {
			printed = append(printed, p)
			printedPhysical = append(printedPhysical, physical)
			continue
		}
		unmapped = append(unmapped, physical)
	}
	if len(printed) == 0 {
		return fmt.Sprintf("cited pages (PDF file): %s", joinInts(unmapped))
	}
	msg := fmt.Sprintf("cited pages: %s (PDF %s)", joinInts(printed), joinInts(printedPhysical))
	if len(unmapped) > 0 {
		msg += fmt.Sprintf("; unmapped PDF pages: %s", joinInts(unmapped))
	}
	return msg
}

// PrintedPages converts physical citations to printed labels, omitting physical
// pages that the PageMap deliberately left unmapped.
func PrintedPages(cited []int, m PageMap) []int {
	out := make([]int, 0, len(cited))
	for _, physical := range cited {
		printed, ok := m.PrintedOf(physical)
		if ok {
			out = append(out, printed)
		}
	}
	return out
}

func joinInts(nums []int) string {
	parts := make([]string, len(nums))
	for i, n := range nums {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ", ")
}
