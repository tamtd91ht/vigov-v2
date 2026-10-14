package xlsx

import "testing"

func TestChoiceValueAndCodeRoundTrip(t *testing.T) {
	for _, c := range []struct{ code, label, value string }{
		{"van-phong", "Văn phòng", "van-phong · Văn phòng"},
		{"CB-00123", "Nguyễn Văn A", "CB-00123 · Nguyễn Văn A"},
		{"khan", "", "khan"},
		{"khan", "   ", "khan"},
	} {
		if got := ChoiceValue(c.code, c.label); got != c.value {
			t.Errorf("ChoiceValue(%q, %q) = %q, want %q", c.code, c.label, got, c.value)
		}
		if got := ChoiceCode(c.value); got != c.code {
			t.Errorf("ChoiceCode(%q) = %q, want %q", c.value, got, c.code)
		}
	}
}

// A bare code typed by hand — the only shape before dropdowns — keeps working; a label that itself
// contains the separator does not leak into the code.
func TestChoiceCodeOfTypedCells(t *testing.T) {
	for cell, want := range map[string]string{
		"  van-phong  ":             "van-phong",
		"":                          "",
		"a · b · c":                 "a",
		" CB-00123 ·  Nguyễn Văn A": "CB-00123",
		"không có dấu ngăn":         "không có dấu ngăn",
		"x·y":                       "x·y", // the separator is " · " with spaces, not the bare dot
	} {
		if got := ChoiceCode(cell); got != want {
			t.Errorf("ChoiceCode(%q) = %q, want %q", cell, got, want)
		}
	}
}
