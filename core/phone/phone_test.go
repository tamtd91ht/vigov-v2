package phone

import "testing"

// Every number here is built on the agreed fake 0900000000 (rule 3, invariant 5) or is not a real
// subscriber shape at all.
func TestNormalizeVNMobile(t *testing.T) {
	for _, c := range []struct {
		raw  string
		want string
		ok   bool
	}{
		{"0900000000", "0900000000", true},
		{" 0900 000 000 ", "0900000000", true},
		{"090.000.0000", "0900000000", true},
		{"090-000-0000", "0900000000", true},
		{"(090) 000 0000", "0900000000", true},
		{"+84900000000", "0900000000", true},
		{"+84 900 000 000", "0900000000", true},
		{"84900000000", "0900000000", true},
		{"0084900000000", "0900000000", true},
		{"0300000000", "0300000000", true},
		{"0500000000", "0500000000", true},
		{"0700000000", "0700000000", true},
		{"0800000000", "0800000000", true},
		// Already national: "084…" is never re-folded, so it is eleven digits and refused.
		{"084900000000", "", false},
		// Landline, wrong length, wrong prefix, letters, a "+" in the middle, empty.
		{"0240000000", "", false},
		{"090000000", "", false},
		{"09000000000", "", false},
		{"0100000000", "", false},
		{"0600000000", "", false},
		// A spreadsheet's numeric cell dropped the leading 0: read back as the mobile number it was.
		{"900000000", "0900000000", true},
		{"300000000", "0300000000", true},
		{"100000000", "", false},
		{"400000000", "", false},
		{"09000O0000", "", false},
		{"0900+000000", "", false},
		{"", "", false},
		{"abc", "", false},
	} {
		got, ok := NormalizeVNMobile(c.raw)
		if got != c.want || ok != c.ok {
			t.Errorf("NormalizeVNMobile(%q) = %q, %v; want %q, %v", c.raw, got, ok, c.want, c.ok)
		}
	}
}
