package domain

import (
	"errors"
	"math"
	"strings"
	"testing"
)

// The shape checks mirror migration 0014's CHECKs; each case is one the database would refuse (or, for
// the accepted ones, one it would store).

func TestExternalContactPhoneShape(t *testing.T) {
	for _, ok := range []string{"113", " 0900000000 ", "+84 900 000 000", "(0236) 3.822.115", "0900-000-000"} {
		if _, err := NormalizeExternalContactPhone(ok); err != nil {
			t.Errorf("%q refused: %v", ok, err)
		}
	}
	for in, want := range map[string]error{
		"":                                      ErrExternalContactPhoneEmpty,
		"   ":                                   ErrExternalContactPhoneEmpty,
		"11":                                    ErrExternalContactPhoneShape, // two digits
		"0900000000 máy lẻ 12":                  ErrExternalContactPhoneShape, // a letter breaks the tel: link
		"0900000000/1":                          ErrExternalContactPhoneShape,
		"1234567890123456":                      ErrExternalContactPhoneShape, // 16 digits
		"0900000000\n":                          nil,                          // trailing newline trimmed
		"+84 (" + strings.Repeat("9", 15) + ")": ErrExternalContactPhoneShape, // 17 digits
		"1" + strings.Repeat("-", 31) + "11":    ErrExternalContactPhoneShape, // 3 digits, 34 characters
	} {
		_, err := NormalizeExternalContactPhone(in)
		if want == nil {
			if err != nil {
				t.Errorf("%q refused: %v", in, err)
			}
			continue
		}
		if !errors.Is(err, want) {
			t.Errorf("%q: err = %v, want %v", in, err, want)
		}
	}
}

func TestExternalContactTextFields(t *testing.T) {
	if got, err := NormalizeExternalContactName("  Trạm Y tế xã  "); err != nil || got != "Trạm Y tế xã" {
		t.Errorf("name = %q, %v", got, err)
	}
	if _, err := NormalizeExternalContactName(strings.Repeat("ạ", ExternalContactNameMaxLen+1)); !errors.Is(err, ErrExternalContactNameTooLong) {
		t.Errorf("long name: %v", err)
	}
	if _, err := NormalizeExternalContactName("Trạm\tY tế"); !errors.Is(err, ErrExternalContactNameEmpty) {
		t.Errorf("control char in name: %v", err)
	}
	if _, err := NormalizeExternalContactCategory(" "); !errors.Is(err, ErrExternalContactCategoryEmpty) {
		t.Errorf("blank category: %v", err)
	}
	if _, err := NormalizeExternalContactCategory(strings.Repeat("a", ExternalContactCategoryMaxLen+1)); !errors.Is(err, ErrExternalContactCategoryTooLong) {
		t.Errorf("long category: %v", err)
	}
	// Address: blank is NULL, never an all-blank string.
	if got, err := NormalizeExternalContactAddress("   "); err != nil || got != "" {
		t.Errorf("blank address = %q, %v", got, err)
	}
	if _, err := NormalizeExternalContactAddress("Thôn 1\x00"); !errors.Is(err, ErrExternalContactAddressInvalid) {
		t.Errorf("control char in address: %v", err)
	}
	if _, err := NormalizeExternalContactAddress(strings.Repeat("a", ExternalContactAddressMaxLen+1)); !errors.Is(err, ErrExternalContactAddressTooLong) {
		t.Errorf("long address: %v", err)
	}
}

func TestExternalContactDisplayOrder(t *testing.T) {
	zero, neg, big := 0, -1, math.MaxInt32+1
	if CheckExternalContactDisplayOrder(nil) != nil || CheckExternalContactDisplayOrder(&zero) != nil {
		t.Error("nil / 0 refused")
	}
	if !errors.Is(CheckExternalContactDisplayOrder(&neg), ErrExternalContactOrderInvalid) ||
		!errors.Is(CheckExternalContactDisplayOrder(&big), ErrExternalContactOrderInvalid) {
		t.Error("negative / past INT accepted")
	}
}
