package store

import (
	"reflect"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// StaffCounts against a REAL PostgreSQL. Skipped without VIGOV_TEST_DSN (rule 8); the statement half
// always runs, in staff_counts_test.go.
//
// EACH ROW OF COMMUNE A EXISTS TO MAKE ONE DEFECT VISIBLE:
//
//	pub-u1        u1, published + consent           counts in total AND published
//	unpub-u1      u1, not published                 total only — drop `hien_tren_mini_app` → published 2
//	locked-u1     u1, published + consent, locked   total only — drop `dang_hoat_dong` → published 2
//	deleted-u1    u1, published, soft-deleted       neither — drop `deleted_at IS NULL` → total 4
//	pub-none      no unit, published + consent      the no-department bucket
//
// Commune B holds a published row in a unit with the SAME id, so a lost commune shows as u1 = 4.
func TestPgStaffCounts(t *testing.T) {
	db := moKetNoi(t)
	a, b := xaRieng(t)
	themBoPhanCongKhaiPg(t, db, a, "u1", "VĂN PHÒNG A")
	themBoPhanCongKhaiPg(t, db, b, "u1", "VĂN PHÒNG B")

	themCongKhaiPg(t, db, a, "pub-u1", "u1", true, false, false, nil)
	themCongKhaiPg(t, db, a, "unpub-u1", "u1", false, false, false, nil)
	themCongKhaiPg(t, db, a, "locked-u1", "u1", true, true, false, nil)
	themCongKhaiPg(t, db, a, "deleted-u1", "u1", true, false, true, nil)
	themCongKhaiPg(t, db, a, "pub-none", "", true, false, false, nil)
	themCongKhaiPg(t, db, b, "pub-u1", "u1", true, false, false, nil)

	got, err := NewCanBoStore(pkgstore.New(db)).StaffCounts(ctxXa(a))
	if err != nil {
		t.Fatalf("StaffCounts: %v", err)
	}
	want := domain.StaffCounts{
		StaffTally:   domain.StaffTally{Total: 4, Published: 2},
		NoDepartment: domain.StaffTally{Total: 1, Published: 1},
		Departments: []domain.DepartmentStaffTally{
			{DepartmentID: "u1", StaffTally: domain.StaffTally{Total: 3, Published: 1}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("xã A = %+v\nmuốn    %+v — RÒ RỈ GIỮA HAI XÃ nếu u1 có 4", got, want)
	}

	// The KPI must equal what citizens see: the public directory of A, read by its own route's store.
	public, err := NewCanBoStore(pkgstore.New(db)).DanhBaCongKhai(ctxXa(a))
	if err != nil {
		t.Fatalf("DanhBaCongKhai: %v", err)
	}
	if len(public) != got.Published {
		t.Fatalf("published = %d nhưng danh bạ công khai hiện %d người", got.Published, len(public))
	}
}
