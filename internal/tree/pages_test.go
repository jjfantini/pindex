package tree

import (
	"reflect"
	"testing"
)

func TestParsePages(t *testing.T) {
	cases := []struct {
		in   string
		want []int
	}{
		{"12", []int{12}},
		{"5-7", []int{5, 6, 7}},
		{"3,8", []int{3, 8}},
		{"5-7,3,9-11", []int{3, 5, 6, 7, 9, 10, 11}},
		{" 5 - 7 , 3 ", []int{3, 5, 6, 7}},
		{"4-4", []int{4}},
		{"6,6,6", []int{6}}, // de-duplicated
	}
	for _, c := range cases {
		got, err := ParsePages(c.in)
		if err != nil {
			t.Errorf("ParsePages(%q) error: %v", c.in, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("ParsePages(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParsePagesErrors(t *testing.T) {
	for _, in := range []string{"7-5", "abc", "1,,2", "", "3-x", "-"} {
		if _, err := ParsePages(in); err == nil {
			t.Errorf("ParsePages(%q): expected error", in)
		}
	}
}
