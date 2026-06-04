package tree

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ParsePages parses a page selector into a sorted, de-duplicated list of page
// numbers. Accepted forms (comma-separated, mixable): "5-7" (inclusive range),
// "3,8" (list), "12" (single). Mirrors PageIndex retrieve._parse_pages.
func ParsePages(pages string) ([]int, error) {
	set := make(map[int]struct{})
	for _, part := range strings.Split(pages, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("invalid pages %q: empty segment", pages)
		}
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			start, err := strconv.Atoi(strings.TrimSpace(bounds[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid range %q: %w", part, err)
			}
			end, err := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid range %q: %w", part, err)
			}
			if start > end {
				return nil, fmt.Errorf("invalid range %q: start must be <= end", part)
			}
			for p := start; p <= end; p++ {
				set[p] = struct{}{}
			}
			continue
		}
		p, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid page %q: %w", part, err)
		}
		set[p] = struct{}{}
	}
	out := make([]int, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Ints(out)
	return out, nil
}
