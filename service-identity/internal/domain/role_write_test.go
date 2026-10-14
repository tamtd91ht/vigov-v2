package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeRoleName(t *testing.T) {
	ok := map[string]string{
		"  Bí thư  ":                     "Bí thư",
		strings.Repeat("ạ", 100):         strings.Repeat("ạ", 100),
		"Trưởng thôn, Tổ trưởng dân phố": "Trưởng thôn, Tổ trưởng dân phố",
	}
	for in, want := range ok {
		got, err := NormalizeRoleName(in)
		if err != nil || got != want {
			t.Errorf("NormalizeRoleName(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "   ", strings.Repeat("ạ", 101), "Bí\tthư", "Bí\u0000thư"} {
		if _, err := NormalizeRoleName(in); !errors.Is(err, ErrRoleNameInvalid) {
			t.Errorf("NormalizeRoleName(%q) err = %v, want ErrRoleNameInvalid", in, err)
		}
	}
}

func TestRoleCodeBaseAndPick(t *testing.T) {
	base, err := RoleCodeBase("Phó Chủ tịch UBND")
	if err != nil || base != "pho-chu-tich-ubnd" {
		t.Fatalf("base = %q, %v", base, err)
	}
	if _, err := RoleCodeBase("!!!"); !errors.Is(err, ErrRoleNameInvalid) {
		t.Fatalf("a name with no letter must be an invalid name, got %v", err)
	}
	if c, ok := PickRoleCode("bi-thu", nil); !ok || c != "bi-thu" {
		t.Errorf("free base: %q %v", c, ok)
	}
	if c, ok := PickRoleCode("bi-thu", []string{"bi-thu", "bi-thu-2"}); !ok || c != "bi-thu-3" {
		t.Errorf("taken base: %q %v", c, ok)
	}
	// The administrator code is never issued, even when free.
	if c, ok := PickRoleCode(CommuneAdminRoleCode, nil); !ok || c != CommuneAdminRoleCode+"-2" {
		t.Errorf("administrator code: %q %v", c, ok)
	}
	var all []string
	for n := 1; n <= roleCodeSuffixes; n++ {
		all = append(all, MaBoPhanThuN("x", n))
	}
	if _, ok := PickRoleCode("x", all); ok {
		t.Error("every suffix taken must be reported, not wrapped round")
	}
}

func TestCheckRoleNameFreeAndOrder(t *testing.T) {
	live := []ExistingRole{{ID: "a", Name: "Kế toán", Order: 6}, {ID: "b", Name: "Chủ tịch UBND", Order: 1}}
	if err := CheckRoleNameFree(" kế TOÁN ", "", live); !errors.Is(err, ErrRoleNameTaken) {
		t.Errorf("case-insensitive duplicate not refused: %v", err)
	}
	if err := CheckRoleNameFree("KẾ TOÁN", "a", live); err != nil {
		t.Errorf("renaming a role to its own name in another case refused: %v", err)
	}
	if NextRoleOrder(live) != 7 || NextRoleOrder(nil) != 1 {
		t.Errorf("NextRoleOrder = %d / %d", NextRoleOrder(live), NextRoleOrder(nil))
	}
}

func TestNormalizeRoleDeleteReason(t *testing.T) {
	if got, err := NormalizeRoleDeleteReason("  Gộp chức danh\r\nvào Văn phòng "); err != nil || got != "Gộp chức danh\r\nvào Văn phòng" {
		t.Errorf("got %q, %v", got, err)
	}
	for _, in := range []string{"", "  ", strings.Repeat("ạ", MaxRoleDeleteReason+1), "a\u0007b"} {
		if _, err := NormalizeRoleDeleteReason(in); !errors.Is(err, ErrRoleDeleteReasonInvalid) {
			t.Errorf("%q: err = %v", in, err)
		}
	}
}
