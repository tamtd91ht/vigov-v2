package domain

import (
	"errors"
	"strings"
	"testing"
)

// Migration 0022's CHECK, answered before the database: "" is a clear (NULL), anything else must be
// non-blank, ≤ 500 characters, one line.
func TestValidateDescriptionPlaceholder(t *testing.T) {
	if got, err := ValidateDescriptionPlaceholder(""); err != nil || got != "" {
		t.Errorf("\"\" = %q, %v — want an explicit clear", got, err)
	}
	if got, err := ValidateDescriptionPlaceholder("  Ví dụ: đèn đường hỏng  "); err != nil || got != "Ví dụ: đèn đường hỏng" {
		t.Errorf("trimmed = %q, %v", got, err)
	}
	if _, err := ValidateDescriptionPlaceholder(strings.Repeat("ă", MaxPetitionFieldPlaceholderRunes)); err != nil {
		t.Errorf("500 runes refused: %v (the bound counts characters, not bytes)", err)
	}
	for name, raw := range map[string]string{
		"blank":    "   ",
		"too long": strings.Repeat("a", MaxPetitionFieldPlaceholderRunes+1),
		"newline":  "một\nhai",
		"control":  "một\x00hai",
		"bad utf8": "\xff",
	} {
		if _, err := ValidateDescriptionPlaceholder(raw); !errors.Is(err, ErrPetitionFieldPlaceholderInvalid) {
			t.Errorf("%s: %v, want ErrPetitionFieldPlaceholderInvalid", name, err)
		}
	}
}

func TestSamePresentationSeesThePlaceholder(t *testing.T) {
	a := PetitionField{DefaultLabel: "Điện", SortOrder: 4}
	b := a
	b.DescriptionPlaceholder = "Ví dụ"
	if SamePetitionFieldPresentation(a, b) {
		t.Error("a placeholder-only edit would be dropped as a no-op")
	}
}
