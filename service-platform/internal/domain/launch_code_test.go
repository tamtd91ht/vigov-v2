package domain

import (
	"strings"
	"testing"
)

func TestNewLaunchCodeHasTheShape(t *testing.T) {
	seen := map[string]bool{}
	for range 200 {
		c, err := NewLaunchCode()
		if err != nil {
			t.Fatal(err)
		}
		if !ValidLaunchCode(c) {
			t.Fatalf("minted %q, which ValidLaunchCode (and 0024's CHECK) refuses", c)
		}
		seen[c] = true
	}
	// 200 draws of 48 bits: a repeat means the source is not random (a constant, a seeded PRNG).
	if len(seen) != 200 {
		t.Fatalf("200 draws gave %d distinct codes", len(seen))
	}
}

func TestValidLaunchCode(t *testing.T) {
	for _, ok := range []string{"0123456789ab", "ffffffffffff", "000000000000"} {
		if !ValidLaunchCode(ok) {
			t.Errorf("%q refused", ok)
		}
	}
	for _, bad := range []string{"", "0123456789a", "0123456789abc", "0123456789AB", "0123456789ag",
		" 0123456789ab", "0123456789ab\n", "xa.vigov.vn", strings.Repeat("a", 26)} {
		if ValidLaunchCode(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}
