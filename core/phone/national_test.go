package phone

import (
	"errors"
	"strings"
	"testing"
)

// Every number here is built on the agreed fake 0900000000 (rule 3, invariant 5) or is not a real
// subscriber shape at all. The cases are the ones identity's directory fold has pinned since ADR 0097
// (service-identity/internal/domain/citizen_directory_test.go) — the same fold, so the same table.

const fakeNational = "0900000000"

func TestNormalizeNational_OneFormForEverySpelling(t *testing.T) {
	for _, raw := range []string{
		fakeNational,
		"0900 000 000",
		"+84900000000",
		"84900000000",
		"0900.000.000",
		"(+84) 900-000-000",
		"+84 900 000 000",
		"0084900000000",
		" 0900000000 ",
		"84 900 000 000",
	} {
		got, err := NormalizeNational(raw)
		if err != nil || got != fakeNational {
			t.Errorf("NormalizeNational(%q) = %q, %v; want %q", raw, got, err, fakeNational)
		}
	}
}

func TestNormalizeNational_KeepsALeading084(t *testing.T) {
	// Built by concatenation so no literal looks like a real subscriber.
	national := "084" + "0000000"
	if got, err := NormalizeNational(national); err != nil || got != national {
		t.Errorf("a number starting 084 was changed: %q, %v", got, err)
	}
	// The country code followed by an 084 number folds to the 084 number.
	if got, err := NormalizeNational("84" + national[1:]); err != nil || got != national {
		t.Errorf("84 + an 084 number must fold to the 084 number: %q, %v", got, err)
	}
}

func TestNormalizeNational_AcceptsLandlinesAndLongNumbers(t *testing.T) {
	// Unlike NormalizeVNMobile: a commune's directory holds landlines, and the CHECK allows 9..15 digits.
	for _, raw := range []string{"0240000000", "02400000000", "0" + strings.Repeat("9", 14), "0" + strings.Repeat("9", 8)} {
		if got, err := NormalizeNational(raw); err != nil || got != raw {
			t.Errorf("NormalizeNational(%q) = %q, %v; want it unchanged", raw, got, err)
		}
	}
}

func TestNormalizeNational_Refusals(t *testing.T) {
	for _, c := range []struct {
		name string
		raw  string
		want error
	}{
		{"empty", "", ErrMissing},
		{"whitespace only", "   ", ErrMissing},
		{"letters", "abc", ErrInvalid},
		{"letter among digits", "0900000000a", ErrInvalid},
		{"plus in the middle", "0900+000000", ErrInvalid},
		{"too short", "0900", ErrNotNational},
		// No guessed 0 — the departure from NormalizeVNMobile, on purpose.
		{"no leading 0", "900000000", ErrNotNational},
		{"more than 15 digits", "0" + strings.Repeat("9", 15), ErrNotNational},
		{"more than 32 characters", strings.Repeat("0 ", 17), ErrTooLong},
		{"separators only", "+()-.", ErrNotNational},
		// "00" is the international dialling prefix and is dropped, leaving seven digits.
		{"leading 00 eats into a short number", "000000000", ErrNotNational},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := NormalizeNational(c.raw)
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			if got != "" {
				t.Errorf("refused but returned %q", got)
			}
			if typed := strings.TrimSpace(c.raw); typed != "" && strings.Contains(err.Error(), typed) {
				t.Errorf("the error repeats the refused value: %q", err)
			}
		})
	}
}

func TestIsNational(t *testing.T) {
	for d, want := range map[string]bool{
		fakeNational:                  true,
		"090000000":                   true, // 9 digits: the CHECK's lower bound
		"0" + strings.Repeat("9", 14): true,
		"0" + strings.Repeat("9", 15): false,
		"09000000":                    false,
		"900000000":                   false,
		"0900 000 000":                false,
		"84900000000":                 false,
		"":                            false,
		"０９００００００００":                  false, // full-width digits are not ASCII digits
	} {
		if got := IsNational(d); got != want {
			t.Errorf("IsNational(%q) = %v, want %v", d, got, want)
		}
	}
}
