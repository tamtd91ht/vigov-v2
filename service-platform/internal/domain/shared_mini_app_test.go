package domain

import (
	"errors"
	"strings"
	"testing"
)

// profileCodeFake is a fake MINI_APP_PROFILE_CODE (never the real dev code).
const profileCodeFake = "AbCdEfGhIjKlMnOpQrStUvWx"

func TestMiniAppLaunchLink(t *testing.T) {
	got, err := MiniAppLaunchLink("1234567890123456789", "0a1b2c3d4e5f", "")
	if err != nil || got != "https://zalo.me/s/1234567890123456789/?src=qr&x=0a1b2c3d4e5f" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := MiniAppLaunchLink("12a", "0a1b2c3d4e5f", ""); !errors.Is(err, ErrMiniAppIDInvalid) {
		t.Errorf("a non-numeric App ID: %v", err)
	}
	// Each of these would make citizen-app's isLaunchCode refuse `x`, so the QR would open no commune.
	for _, bad := range []string{"", "0A1B2C3D4E5F", "0a1b2c3d4e5", "0a1b2c3d4e5f0", "xa.vigov.vn", " 0a1b2c3d4e5f"} {
		if _, err := MiniAppLaunchLink("1", bad, profileCodeFake); !errors.Is(err, ErrLaunchCodeInvalid) {
			t.Errorf("code %q: %v, want ErrLaunchCodeInvalid", bad, err)
		}
	}
}

// With a profile code (dev cluster, ADR 0046 §Sửa đổi 14/10/2026 #9) the link carries `pf`; keys sorted.
func TestMiniAppLaunchLinkWithProfileCode(t *testing.T) {
	got, err := MiniAppLaunchLink("1234567890123456789", "0a1b2c3d4e5f", profileCodeFake)
	want := "https://zalo.me/s/1234567890123456789/?pf=" + profileCodeFake + "&src=qr&x=0a1b2c3d4e5f"
	if err != nil || got != want {
		t.Fatalf("got %q, %v; want %q", got, err, want)
	}
}

// The own app's QR carries no commune host: the commune is the App ID's (ADR 0070 §Sửa đổi 05/10/2026 #4).
func TestOwnMiniAppLaunchLink(t *testing.T) {
	got, err := OwnMiniAppLaunchLink("3291993990104489440", "")
	if err != nil || got != "https://zalo.me/s/3291993990104489440/?src=qr" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := OwnMiniAppLaunchLink("12a", profileCodeFake); !errors.Is(err, ErrMiniAppIDInvalid) {
		t.Errorf("a non-numeric App ID: %v", err)
	}
}

func TestOwnMiniAppLaunchLinkWithProfileCode(t *testing.T) {
	got, err := OwnMiniAppLaunchLink("3291993990104489440", profileCodeFake)
	want := "https://zalo.me/s/3291993990104489440/?pf=" + profileCodeFake + "&src=qr"
	if err != nil || got != want {
		t.Fatalf("got %q, %v; want %q", got, err, want)
	}
}

// Whatever reaches the builder, it stays inside `pf`: no second parameter can be smuggled in.
func TestLaunchProfileCodeIsEscaped(t *testing.T) {
	got, err := OwnMiniAppLaunchLink("3291993990104489440", "a&x=0a1b2c3d4e5f")
	if err != nil || strings.Contains(got, "&x=") {
		t.Fatalf("got %q, %v — the code broke out of its parameter", got, err)
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
