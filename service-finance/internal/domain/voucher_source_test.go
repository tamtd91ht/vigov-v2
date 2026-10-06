package domain

import (
	"errors"
	"testing"
)

func TestCheckVoucherSource(t *testing.T) {
	const a, b, other = "01JSOURCEA0000000000000000", "01JSOURCEB0000000000000000", "01JSOURCEZ0000000000000000"
	cases := []struct {
		name      string
		allocated []string
		source    string
		want      error
	}{
		{"no lines, no source — §13 rule 6 stays legal", nil, "", nil},
		{"no lines, a source named — refused", nil, a, ErrSourceNotAllocated},
		{"lines, no source — required", []string{a, b}, "", ErrSourceRequired},
		{"lines, an allocated source", []string{a, b}, b, nil},
		{"lines, a source the project does not draw on", []string{a, b}, other, ErrSourceNotAllocated},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := CheckVoucherSource(c.allocated, c.source)
			if !errors.Is(got, c.want) || (c.want == nil && got != nil) {
				t.Fatalf("CheckVoucherSource(%v, %q) = %v, want %v", c.allocated, c.source, got, c.want)
			}
		})
	}
}
