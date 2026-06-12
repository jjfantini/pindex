package tree

import (
	"reflect"
	"strconv"
	"testing"
)

func page(n int, text string) PageContent {
	return PageContent{Page: n, Content: text}
}

func withFooter(body string, printed int) string {
	return body + "\n\n" + itoa(printed)
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func TestBuildPageMapDetectsConstantOffset(t *testing.T) {
	pages := []PageContent{
		page(3, withFooter("Management discussion", 1)),
		page(4, withFooter("Revenue table", 2)),
		page(5, withFooter("Income tax note", 3)),
	}

	got := BuildPageMap(pages)
	want := PageMap{{PhysStart: 3, PhysEnd: 5, Offset: 2}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildPageMap() = %#v, want %#v", got, want)
	}

	if printed, ok := got.PrintedOf(4); !ok || printed != 2 {
		t.Fatalf("PrintedOf(4) = %d, %v; want 2, true", printed, ok)
	}
	if physical, ok := got.PhysicalOf(3); !ok || physical != 5 {
		t.Fatalf("PhysicalOf(3) = %d, %v; want 5, true", physical, ok)
	}
}

func TestBuildPageMapDetectsBoeingOffsetDrift(t *testing.T) {
	pages := []PageContent{
		page(50, withFooter("Critical Accounting Policies", 48)),
		page(51, withFooter("long-term production effort", 49)),
		page(52, withFooter("Pension Plans", 50)),
		page(53, withFooter("commodity purchase contracts", 51)),
		page(54, withFooter("Item 8. Financial Statements", 52)),
		page(55, withFooter("Consolidated Statements of Operations", 53)),
		page(56, withFooter("Consolidated Statements of Comprehensive Income", 54)),
		page(57, "Consolidated Statements of Financial Position\nSee Notes to the Consolidated Financial Statements"),
		page(58, "55"),
		page(59, withFooter("Consolidated Statements of Cash Flows", 56)),
		page(60, ""),
		page(61, withFooter("Consolidated Statements of Equity", 57)),
		page(62, withFooter("Note 1 - Basis of Presentation", 58)),
		page(63, withFooter("Note 1 - Revenue Recognition", 59)),
		page(64, withFooter("Long-term contracts", 60)),
	}

	got := BuildPageMap(pages)
	want := PageMap{
		{PhysStart: 50, PhysEnd: 56, Offset: 2},
		{PhysStart: 58, PhysEnd: 59, Offset: 3},
		{PhysStart: 61, PhysEnd: 64, Offset: 4},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildPageMap() = %#v, want %#v", got, want)
	}

	if _, ok := got.PrintedOf(57); ok {
		t.Fatal("PrintedOf(57) mapped an ambiguous gap page")
	}
	if _, ok := got.PrintedOf(60); ok {
		t.Fatal("PrintedOf(60) mapped a blank gap page")
	}
	if printed, ok := got.PrintedOf(61); !ok || printed != 57 {
		t.Fatalf("PrintedOf(61) = %d, %v; want 57, true", printed, ok)
	}
}

func TestBuildPageMapRejectsNoise(t *testing.T) {
	pages := []PageContent{
		page(10, withFooter("stable page", 8)),
		page(11, withFooter("stable page", 9)),
		page(12, "Table values\n2022"),
		page(13, withFooter("stable page", 11)),
		page(14, withFooter("stable page", 12)),
		page(15, withFooter("single-anchor drift", 7)),
		page(16, withFooter("stable page", 14)),
	}

	got := BuildPageMap(pages)
	want := PageMap{
		{PhysStart: 10, PhysEnd: 11, Offset: 2},
		{PhysStart: 13, PhysEnd: 14, Offset: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildPageMap() = %#v, want %#v", got, want)
	}
	if _, ok := got.PrintedOf(12); ok {
		t.Fatal("PrintedOf(12) mapped table-number noise")
	}
	if _, ok := got.PrintedOf(15); ok {
		t.Fatal("PrintedOf(15) mapped single-anchor drift")
	}
}

func TestBuildPageMapWithoutUsableFootersReturnsNil(t *testing.T) {
	got := BuildPageMap([]PageContent{
		page(1, "cover page without footer"),
		page(2, "table\n137100"),
		page(3, ""),
	})
	if got != nil {
		t.Fatalf("BuildPageMap() = %#v, want nil", got)
	}
}

func TestFormatCitationsShowsPrintedAndPhysicalPages(t *testing.T) {
	m := PageMap{
		{PhysStart: 50, PhysEnd: 56, Offset: 2},
		{PhysStart: 61, PhysEnd: 64, Offset: 4},
	}

	got := FormatCitations([]int{56, 60, 61}, m)
	want := "cited pages: 54, 57 (PDF 56, 61); unmapped PDF pages: 60"
	if got != want {
		t.Fatalf("FormatCitations() = %q, want %q", got, want)
	}
}

func TestPrintedPagesSkipsUnmappedPhysicalPages(t *testing.T) {
	m := PageMap{{PhysStart: 50, PhysEnd: 56, Offset: 2}}

	got := PrintedPages([]int{50, 57, 56}, m)
	want := []int{48, 54}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PrintedPages() = %#v, want %#v", got, want)
	}
}
