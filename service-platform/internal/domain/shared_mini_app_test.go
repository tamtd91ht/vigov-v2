package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestMiniAppLaunchLink(t *testing.T) {
	got, err := MiniAppLaunchLink("1234567890123456789", "thangbinh-danang.vigov.vn")
	if err != nil || got != "https://zalo.me/s/1234567890123456789/?d=thangbinh-danang.vigov.vn&src=qr" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := MiniAppLaunchLink("12a", "xa.vigov.vn"); !errors.Is(err, ErrMiniAppIDInvalid) {
		t.Errorf("a non-numeric App ID: %v", err)
	}
	// Each of these would make citizen-app's laTenMien drop `d`, so the QR would suggest no commune.
	for _, bad := range []string{"", "xa", "XA.vigov.vn", "xa.vigov.vn:443", "xa.vigov.vn/x", " xa.vigov.vn",
		"api.vigov.vn", "xa.vigov.vn."} {
		if _, err := MiniAppLaunchLink("1", bad); !errors.Is(err, ErrLaunchHostInvalid) {
			t.Errorf("host %q: %v, want ErrLaunchHostInvalid", bad, err)
		}
	}
}

// The own app's QR carries no commune host: the commune is the App ID's (ADR 0070 §Sửa đổi 05/10/2026 #4).
func TestOwnMiniAppLaunchLink(t *testing.T) {
	got, err := OwnMiniAppLaunchLink("3291993990104489440")
	if err != nil || got != "https://zalo.me/s/3291993990104489440/?src=qr" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := OwnMiniAppLaunchLink("12a"); !errors.Is(err, ErrMiniAppIDInvalid) {
		t.Errorf("a non-numeric App ID: %v", err)
	}
}

func TestPetitionFieldValidation(t *testing.T) {
	for _, ok := range []string{"dien", "rac-thai", "an-toan-thuc-pham", "a1", strings.Repeat("a", 64)} {
		if err := ValidatePetitionFieldCode(ok); err != nil {
			t.Errorf("code %q refused: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "Dien", "điện", "-dien", "dien-", "di--en", "di en", "di_en", strings.Repeat("a", 65)} {
		if err := ValidatePetitionFieldCode(bad); !errors.Is(err, ErrPetitionFieldCodeInvalid) {
			t.Errorf("code %q: %v", bad, err)
		}
	}

	good := PetitionField{DefaultLabel: "  Điện  ", SortOrder: 4, Icon: "Zap", Tone: "orange"}
	f, err := ValidatePetitionFieldPresentation(good)
	if err != nil || f.DefaultLabel != "Điện" {
		t.Fatalf("got %+v, %v", f, err)
	}
	if _, err := ValidatePetitionFieldPresentation(PetitionField{DefaultLabel: "Khác", SortOrder: 1}); err != nil {
		t.Errorf("empty icon and tone mean 'not declared': %v", err)
	}
	for name, c := range map[string]struct {
		f    PetitionField
		want error
	}{
		"blank label":    {PetitionField{DefaultLabel: "  ", SortOrder: 1}, ErrPetitionFieldLabelInvalid},
		"newline label":  {PetitionField{DefaultLabel: "a\nb", SortOrder: 1}, ErrPetitionFieldLabelInvalid},
		"zero order":     {PetitionField{DefaultLabel: "a", SortOrder: 0}, ErrPetitionFieldSortOrderInvalid},
		"huge order":     {PetitionField{DefaultLabel: "a", SortOrder: 10001}, ErrPetitionFieldSortOrderInvalid},
		"url icon":       {PetitionField{DefaultLabel: "a", SortOrder: 1, Icon: "https://x/y.svg"}, ErrPetitionFieldIconInvalid},
		"lowercase icon": {PetitionField{DefaultLabel: "a", SortOrder: 1, Icon: "zap"}, ErrPetitionFieldIconInvalid},
		"hex tone":       {PetitionField{DefaultLabel: "a", SortOrder: 1, Tone: "#ff0000"}, ErrPetitionFieldToneInvalid},
	} {
		if _, err := ValidatePetitionFieldPresentation(c.f); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", name, err, c.want)
		}
	}
}
