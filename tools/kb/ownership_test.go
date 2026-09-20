package main

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// These tests run against the REAL migrations, not a fixture. A fixture would have proved the
// regex works on text shaped the way the test author imagined; what actually broke was the
// distance between the mark and the table in the repository's own house style.

func khoRoot(t *testing.T) string {
	t.Helper()
	_, tep, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("không xác định được đường dẫn tệp test")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(tep)))
}

func quet(t *testing.T) ([]soHuu, []string) {
	t.Helper()
	root := khoRoot(t)
	sv, err := scanServices(root)
	if err != nil {
		t.Fatalf("scanServices: %v", err)
	}
	rows, canhBao, err := quetSoHuu(root, sv)
	if err != nil {
		t.Fatalf("quetSoHuu: %v", err)
	}
	return rows, canhBao
}

func TestChiMucSoHuuKHONGDuocRONG(t *testing.T) {
	// THE CASE THAT WOULD HAVE CAUGHT THE ORIGINAL DEFECT. For months this index said
	// "no schemas defined yet" while the marks were there, and CLAUDE.md step 4 sends every
	// session here FIRST with "do NOT read every schema". An empty index does not look broken,
	// so nobody asked — which is why the floor is asserted rather than trusted.
	rows, _ := quet(t)
	if len(rows) < 15 {
		t.Fatalf("chỉ mục sở hữu có %d thực thể — quá ít, bộ đọc nhiều khả năng đã chết", len(rows))
	}
}

func TestMoiDauEntityDeuTimDuocBang(t *testing.T) {
	// THE EXACT REGRESSION. The first version stopped looking 12 lines after the mark and lost
	// 11 of 20 entities, because this repo puts a long WHY comment between the mark and the
	// table. It produced an index that looked populated and was missing over half — worse than
	// the placeholder, because a populated-looking index gets believed.
	_, canhBao := quet(t)
	for _, c := range canhBao {
		if strings.Contains(c, "không có CREATE TABLE") {
			t.Errorf("dấu @entity không tìm được bảng: %s", c)
		}
	}
}

func TestKhoangCachXaGiuaDauVaBangVanDoc(t *testing.T) {
	// Petition's mark sits 13 lines above its CREATE TABLE — one line past the window that
	// broke. Named explicitly so that shortening the walk fails here with the reason, not
	// somewhere downstream with a smaller number.
	rows, _ := quet(t)
	for _, r := range rows {
		if r.Entity == "Petition" {
			if r.Table != "phieu_phan_anh" || r.Service != "petitions" {
				t.Errorf("Petition: got %s/%s, want petitions/phieu_phan_anh", r.Service, r.Table)
			}
			return
		}
	}
	t.Error("không thấy thực thể Petition — bộ đọc bỏ sót đúng ca có khoảng cách xa nhất")
}

func TestMoiThucTheCoDUNGMOTChuSoHuu(t *testing.T) {
	// Rule 2, invariant 1. Two services declaring one entity is a distributed monolith forming,
	// and this generated index is the only place it is visible at a glance.
	rows, _ := quet(t)
	for _, c := range trung(rows) {
		t.Error(c)
	}
}

func TestMoiThucTheKhaiScope(t *testing.T) {
	// A missing scope is never defaulted (see quetSoHuu). Declaring a commune's table as
	// platform-wide, or the reverse, is rule 1's entire subject matter.
	rows, _ := quet(t)
	for _, r := range rows {
		switch r.Scope {
		case "tenant", "platform", "cross-tenant":
		default:
			t.Errorf("%s (%s): @scope %q không hợp lệ", r.Entity, r.Source, r.Scope)
		}
	}
}
