// Package privacy masks citizen personal data.
//
// WHY ONE PACKAGE: the previous system had this helper in ten separate files. Ten copies of a
// masking function are ten masking behaviours that can differ — and if one masks 09****5678
// while another masks 0912***678, the amount of information disclosed differs and nobody
// knows which one is running where. Rule 3 depends on there being exactly one answer.
package privacy

import "strings"

// MaskPhone renders a Vietnamese mobile number as 09****5678.
func MaskPhone(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 8 {
		return strings.Repeat("*", len(s))
	}
	return s[:2] + strings.Repeat("*", len(s)-6) + s[len(s)-4:]
}

// MaskCccd renders a 12-digit national ID as 079*****789.
func MaskCccd(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 7 {
		return strings.Repeat("*", len(s))
	}
	return s[:3] + strings.Repeat("*", len(s)-6) + s[len(s)-3:]
}

// MaskName keeps the family name and initials: "Nguyễn Văn An" -> "Nguyễn V. A."
//
// The family name is kept because staff need to recognise a record at a glance; given names
// are what identify an individual.
func MaskName(s string) string {
	parts := strings.Fields(s)
	if len(parts) <= 1 {
		return s
	}
	out := []string{parts[0]}
	for _, p := range parts[1:] {
		r := []rune(p)
		if len(r) > 0 {
			out = append(out, string(r[0])+".")
		}
	}
	return strings.Join(out, " ")
}
