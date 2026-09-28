package store

import (
	"sort"
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
)

// The two org-unit reads behind ResolveLiveOrgUnits and ResolveStaffOrgUnits.
//
// THE FIRST TEST RUNS EVERYWHERE and pins the join's isolation clauses in its TEXT — a weak check
// (a string is not a plan), stated as such, so the one-word edits that leak across communes cannot
// happen with no test red on a machine without PostgreSQL. THE REST NEED VIGOV_TEST_DSN (harness in
// checker_pg_test.go) and skip without one.

func TestStaffOrgUnitsQueryKeepsBothRowsInTheCommune(t *testing.T) {
	for _, want := range []string{
		"bp.tenant_id = nd.tenant_id", // the join repeats the commune (rule 1)
		"bp.id        = nd.bo_phan_id",
		"bp.deleted_at IS NULL", // a removed unit holds no work
		"WHERE nd.tenant_id = $1",
		"nd.ma = $2", // the business code, bound — never spliced
		"nd.deleted_at IS NULL",
	} {
		if !strings.Contains(staffOrgUnitsQuery, want) {
			t.Errorf("câu đọc bộ phận của cán bộ thiếu %q", want)
		}
	}
	if strings.Contains(staffOrgUnitsQuery, "LEFT JOIN") {
		t.Error("LEFT JOIN trả một dòng NULL cho người không có bộ phận — phải là JOIN")
	}
}

func TestPgLiveOrgUnitsOnlyLiveUnitsOfThisCommune(t *testing.T) {
	db := moKetNoi(t)
	xa, otherXa := xaRieng(t)
	themBoPhanPg(t, db, xa, "bp-live", "van-phong", "", false)
	themBoPhanPg(t, db, xa, "bp-gone", "da-xoa", "", true)
	themBoPhanPg(t, db, otherXa, "bp-other", "van-phong", "", false)

	got, err := NewBoPhanStore(pkgstore.New(db)).LiveIDs(ctxXa(xa),
		[]string{"bp-live", "bp-gone", "bp-other", "bp-none"})
	if err != nil {
		t.Fatalf("LiveIDs: %v", err)
	}
	sort.Strings(got)
	if strings.Join(got, ",") != "bp-live" {
		t.Fatalf("bộ phận còn hiệu lực = %v, muốn chỉ bp-live (đã xoá / xã khác / không tồn tại phải vắng)", got)
	}
}

func TestPgUnitsOfStaff(t *testing.T) {
	db := moKetNoi(t)
	xa, otherXa := xaRieng(t)
	themBoPhanPg(t, db, xa, "bp-1", "van-phong", "", false)
	themBoPhanPg(t, db, xa, "bp-gone", "da-xoa", "", true)
	themNguoiBoPhanPg(t, db, xa, "nd-1", "bp-1", true, false)       // CB-nd-1 in bp-1
	themNguoiBoPhanPg(t, db, xa, "nd-locked", "bp-1", false, false) // locked: still in bp-1
	themNguoiBoPhanPg(t, db, xa, "nd-deleted", "bp-1", true, true)  // removed record
	themNguoiBoPhanPg(t, db, xa, "nd-gone-unit", "bp-gone", true, false)
	// The same code in another commune, in a unit of that commune.
	themBoPhanPg(t, db, otherXa, "bp-9", "van-phong", "", false)
	themNguoiBoPhanPg(t, db, otherXa, "nd-1", "bp-9", true, false)

	kho := NewBoPhanStore(pkgstore.New(db))
	for code, want := range map[string]string{
		"CB-nd-1":         "bp-1",
		"CB-nd-locked":    "bp-1",
		"CB-nd-deleted":   "",
		"CB-nd-gone-unit": "",
		"CB-unknown":      "",
	} {
		got, err := kho.UnitsOfStaff(ctxXa(xa), code)
		if err != nil {
			t.Fatalf("UnitsOfStaff(%s): %v", code, err)
		}
		if strings.Join(got, ",") != want {
			t.Errorf("UnitsOfStaff(%s) = %v, muốn %q", code, got, want)
		}
	}
}
