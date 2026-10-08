package store

import (
	"testing"
)

// Against a real PostgreSQL: StatesByCode answers live rows of the context's commune only, carries
// `dang_dung` through, and matches exactly. SKIPS WITHOUT VIGOV_TEST_DSN; the harness is in
// loai_van_ban_pg_test.go.

func TestPgStatesByCode(t *testing.T) {
	db := moKetNoi(t)
	tenantA, tenantB := xaRieng(t)

	themLoai(t, db, tenantA, "lvb-on", "cong-van", "Công văn", 10, true, false)
	themLoai(t, db, tenantA, "lvb-off", "to-trinh", "Tờ trình", 20, false, false)
	themLoai(t, db, tenantA, "lvb-gone", "thong-bao", "Thông báo", 30, true, false)
	if _, err := db.Exec(
		`UPDATE loai_van_ban SET deleted_at = now(), deleted_by = $3, delete_reason = $4
		 WHERE tenant_id = $1 AND id = $2`,
		tenantA, "lvb-gone", "CB-001", "gộp vào loại khác"); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	// Another commune carries a code commune A does not.
	themLoai(t, db, tenantB, "lvb-b", "quyet-dinh", "Quyết định", 10, true, false)

	got, err := dungLoaiVanBanStore(db).StatesByCode(ctxXa(tenantA),
		[]string{"cong-van", "to-trinh", "thong-bao", "quyet-dinh", "Cong-Van"})
	if err != nil {
		t.Fatal(err)
	}
	byCode := map[string]bool{}
	for _, s := range got {
		byCode[s.Code] = s.Active
	}
	if len(got) != 2 || !byCode["cong-van"] {
		t.Errorf("= %+v, want cong-van active and to-trinh inactive only", got)
	}
	if active, ok := byCode["to-trinh"]; !ok || active {
		t.Errorf("switched-off type: present=%v active=%v, want present and inactive", ok, active)
	}
	for _, absent := range []string{"thong-bao", "quyet-dinh", "Cong-Van"} {
		if _, ok := byCode[absent]; ok {
			t.Errorf("%q answered — soft-deleted, another commune's, or a non-exact match must be absent", absent)
		}
	}
}
