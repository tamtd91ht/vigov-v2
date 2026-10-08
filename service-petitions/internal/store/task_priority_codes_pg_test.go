package store

import (
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// Against a real PostgreSQL: StatesByCode answers live rows of the context's commune only, carries
// `dang_dung` through, and matches exactly. SKIPS WITHOUT VIGOV_TEST_DSN; the harness is in
// danh_muc_nhiem_vu_pg_test.go.

func TestPgTaskPriorityStatesByCode(t *testing.T) {
	db := moKetNoi(t)
	tenantA, tenantB := xaRieng(t)

	themUuTien(t, db, tenantA, "uu-on", "khan", "Khẩn", 10)
	themUuTien(t, db, tenantA, "uu-off", "cao", "Cao", 20)
	if _, err := db.Exec(`UPDATE muc_uu_tien_nhiem_vu SET dang_dung = false WHERE tenant_id = $1 AND id = $2`,
		tenantA, "uu-off"); err != nil {
		t.Fatalf("switch off: %v", err)
	}
	themUuTien(t, db, tenantA, "uu-gone", "thuong", "Thường", 30)
	if _, err := db.Exec(
		`UPDATE muc_uu_tien_nhiem_vu SET deleted_at = now(), deleted_by = $3, delete_reason = $4
		 WHERE tenant_id = $1 AND id = $2`,
		tenantA, "uu-gone", "CB-001", "gộp vào mức khác"); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	// Another commune carries a code commune A does not.
	themUuTien(t, db, tenantB, "uu-b", "rat-khan", "Rất khẩn", 10)

	got, err := NewMucUuTienNhiemVuStore(pkgstore.New(db)).StatesByCode(ctxXa(tenant.ID(tenantA)),
		[]string{"khan", "cao", "thuong", "rat-khan", "Khan"})
	if err != nil {
		t.Fatal(err)
	}
	byCode := map[string]bool{}
	for _, s := range got {
		byCode[s.Code] = s.Active
	}
	if len(got) != 2 || !byCode["khan"] {
		t.Errorf("= %+v, want khan active and cao inactive only", got)
	}
	if active, ok := byCode["cao"]; !ok || active {
		t.Errorf("switched-off priority: present=%v active=%v, want present and inactive", ok, active)
	}
	for _, absent := range []string{"thuong", "rat-khan", "Khan"} {
		if _, ok := byCode[absent]; ok {
			t.Errorf("%q answered — soft-deleted, another commune's, or a non-exact match must be absent", absent)
		}
	}
}
