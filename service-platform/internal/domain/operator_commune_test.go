package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestParseCommuneHost(t *testing.T) {
	const opHost = "admin.vigov.vn"
	ok := map[string]string{
		"thangbinh-danang.vigov.vn":   "thangbinh-danang.vigov.vn",
		"  TanPhu.STG.vigov.vn ":      "tanphu.stg.vigov.vn",
		"xa.example.gov.vn":           "xa.example.gov.vn",
		"xn--thng-bnh-xxa.example.vn": "xn--thng-bnh-xxa.example.vn",
	}
	for in, want := range ok {
		got, err := ParseCommuneHost(in, opHost)
		if err != nil || got != want {
			t.Errorf("ParseCommuneHost(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	invalid := []string{"", "x.vigov.vn:443", "https://x.vigov.vn", "x.vigov.vn/a", "x.vigov.vn.",
		"localhost", "10.0.0.1", "thăngbình.vigov.vn", "-x.vigov.vn", "x_y.vigov.vn",
		strings.Repeat("a", 64) + ".vigov.vn"}
	for _, in := range invalid {
		if _, err := ParseCommuneHost(in, opHost); !errors.Is(err, ErrCommuneHostInvalid) {
			t.Errorf("ParseCommuneHost(%q) err = %v, want ErrCommuneHostInvalid", in, err)
		}
	}
	reserved := []string{"admin.vigov.vn", "ADMIN-STG.vigov.vn", "vigov.vn", "api.vigov.vn",
		"identity.api.vigov.vn", "www.stg.vigov.vn"}
	for _, in := range reserved {
		if _, err := ParseCommuneHost(in, opHost); !errors.Is(err, ErrCommuneHostReserved) {
			t.Errorf("ParseCommuneHost(%q) err = %v, want ErrCommuneHostReserved", in, err)
		}
	}
	// OPERATOR_HOST outside vigov.vn is still refused as a commune host.
	if _, err := ParseCommuneHost("ops.example.vn", "ops.example.vn"); !errors.Is(err, ErrCommuneHostReserved) {
		t.Errorf("OPERATOR_HOST accepted as a commune host: %v", err)
	}
}

func TestValidateCommuneNameAndReason(t *testing.T) {
	if s, err := ValidateCommuneName("  Xã Thăng Bình "); err != nil || s != "Xã Thăng Bình" {
		t.Errorf("name = %q, %v", s, err)
	}
	for _, bad := range []string{"", "   ", "Xã\x00Bình", "Xã\nBình", string([]byte{0xff, 0xfe}),
		strings.Repeat("x", MaxCommuneNameRunes+1)} {
		if _, err := ValidateCommuneName(bad); !errors.Is(err, ErrNameInvalid) {
			t.Errorf("ValidateCommuneName(%q) err = %v", bad, err)
		}
	}
	if s, err := ValidateReason("Sửa lỗi gõ\nTicket OPS-12"); err != nil || s == "" {
		t.Errorf("reason with newline refused: %v", err)
	}
	for _, bad := range []string{"", " \n ", "a\x00b", strings.Repeat("x", MaxReasonRunes+1)} {
		if _, err := ValidateReason(bad); !errors.Is(err, ErrReasonInvalid) {
			t.Errorf("ValidateReason(%q) err = %v", bad, err)
		}
	}
}

func TestNameAndProvinceKeys(t *testing.T) {
	if NameKey("XÃ  Thăng   Bình") != NameKey("xã thăng bình") {
		t.Error("case and inner whitespace must not distinguish two names")
	}
	if NameKey("Xã Tân Phú") == NameKey("Xã Tấn Phú") {
		t.Error("diacritics must distinguish two names")
	}
	if ProvinceKey("Thành phố Đà Nẵng") != ProvinceKey("Đà Nẵng") ||
		ProvinceKey("Tỉnh Lai Châu") != ProvinceKey("Lai Châu") {
		t.Error("legacy prefixed province spelling must compare equal to the catalogue spelling")
	}
}

func TestValidateMiniAppID(t *testing.T) {
	if err := ValidateMiniAppID("3291993990104489440"); err != nil {
		t.Errorf("real-shaped App ID refused: %v", err)
	}
	for _, bad := range []string{"", " 123", "12a", "https://zalo.me/s/1", strings.Repeat("1", 33)} {
		if err := ValidateMiniAppID(bad); !errors.Is(err, ErrMiniAppIDInvalid) {
			t.Errorf("ValidateMiniAppID(%q) err = %v", bad, err)
		}
	}
}
