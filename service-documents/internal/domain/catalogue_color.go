package domain

import (
	"errors"
	"strings"
)

// The display colour of a catalogue row — `loai_van_ban` (migration 0007, ADR 0079 #5 and lô 2 Q1 #9).
// A COPY of service-identity/internal/domain/catalogue_color.go, never an import (rule 2, forbidden #1). Presentation
// only: editable on every tier, "Hệ thống" rows included, and nothing branches on it.
//
// ONE SHAPE, `#RRGGBB`. The value reaches a style attribute on the admin web, so a closed shape is the
// floor under it (rule 13 #3) — the CHECK in 0007 holds the same shape against every writer.
//
// STORED LOWER-CASE. The CHECK admits either case; folding here means one colour has one spelling, so
// "did the colour change" in the audit delta is a string comparison and not a case-insensitive one.

// ErrCatalogueColorInvalid — not `#` followed by six hex digits.
var ErrCatalogueColorInvalid = errors.New("danh_muc: `color` phải có dạng #RRGGBB, ví dụ #1f6feb")

// NormalizeCatalogueColor validates a colour and returns it lower-cased. The empty string is NOT "no
// colour" here — clearing is said with JSON null, so an empty string is a form that failed to fill.
func NormalizeCatalogueColor(color string) (string, error) {
	color = strings.TrimSpace(color)
	if len(color) != 7 || color[0] != '#' {
		return "", ErrCatalogueColorInvalid
	}
	for i := 1; i < 7; i++ {
		c := color[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return "", ErrCatalogueColorInvalid
		}
	}
	return strings.ToLower(color), nil
}
