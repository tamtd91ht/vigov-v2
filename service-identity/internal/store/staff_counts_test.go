package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"reflect"
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// StaffCounts — the STATEMENT, where it reaches the driver, and the folding of its rows. The fake
// does not execute SQL; which rows each predicate admits is proven against PostgreSQL in
// staff_counts_pg_test.go.

func openStaffCountsStore(t *testing.T, rows [][]driver.Value) (*CanBoStore, *ckGhi) {
	t.Helper()
	g := &ckGhi{hang: rows, columns: []string{"bo_phan_id", "count", "count"}}
	db := sql.OpenDB(ckConnector{g: g})
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return NewCanBoStore(pkgstore.New(db)), g
}

func TestStaffCountsSharesBothPredicatesAndBindsCommune(t *testing.T) {
	repo, rec := openStaffCountsStore(t, nil)
	if _, err := repo.StaffCounts(tenant.Into(context.Background(), ckXa)); err != nil {
		t.Fatalf("StaffCounts: %v", err)
	}
	if len(rec.lenh) != 1 {
		t.Fatalf("số câu lệnh = %d, muốn 1 — mọi con số phải cùng một thời điểm", len(rec.lenh))
	}
	stmt := rec.lenh[0]
	if len(stmt.args) != 1 || stmt.args[0] != ckXa {
		t.Fatalf("tham số = %v, muốn đúng [xã của ngữ cảnh %s]", stmt.args, ckXa)
	}
	// MUTATIONS THAT MUST TURN THIS RED: retype either predicate instead of sharing it, or drop the
	// commune. Containment of the CONSTANTS, not of a copy of their text — a copy would agree with an
	// edited constant just as happily.
	for name, want := range map[string]string{
		"commune":                    "WHERE nd.tenant_id = $1",
		"register (locTomTat)":       locTomTat,
		"public (locDanhBaCongKhai)": "FILTER (WHERE true\n" + locDanhBaCongKhai + ")",
		"grouping":                   "GROUP BY 1",
	} {
		if !strings.Contains(stmt.sql, want) {
			t.Errorf("câu lệnh thiếu %s %q:\n%s", name, want, stmt.sql)
		}
	}
	// One table only: the unqualified `deleted_at` of locTomTat must not become ambiguous.
	if strings.Contains(strings.ToUpper(stmt.sql), "JOIN") {
		t.Errorf("câu lệnh đếm có JOIN:\n%s", stmt.sql)
	}
	// Counts only: no personal column is selected.
	cols := stmt.sql[strings.Index(stmt.sql, "SELECT"):strings.Index(stmt.sql, "FROM nguoi_dung")]
	for _, banned := range []string{"ho_ten", "dien_thoai", "di_dong", "email", "nd.id", "nd.ma"} {
		if strings.Contains(cols, banned) {
			t.Errorf("câu lệnh đếm chọn cột %q: %s", banned, cols)
		}
	}
}

func TestStaffCountsFoldsRowsIntoTotalsAndNoDepartment(t *testing.T) {
	repo, _ := openStaffCountsStore(t, [][]driver.Value{
		{"", int64(2), int64(1)},
		{"bp-001", int64(5), int64(3)},
		{"bp-002", int64(4), int64(0)},
	})
	got, err := repo.StaffCounts(tenant.Into(context.Background(), ckXa))
	if err != nil {
		t.Fatalf("StaffCounts: %v", err)
	}
	want := domain.StaffCounts{
		StaffTally:   domain.StaffTally{Total: 11, Published: 4},
		NoDepartment: domain.StaffTally{Total: 2, Published: 1},
		Departments: []domain.DepartmentStaffTally{
			{DepartmentID: "bp-001", StaffTally: domain.StaffTally{Total: 5, Published: 3}},
			{DepartmentID: "bp-002", StaffTally: domain.StaffTally{Total: 4, Published: 0}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("StaffCounts = %+v\nmuốn          %+v", got, want)
	}
}

func TestStaffCountsEmptyCommuneIsZeroWithEmptyDepartments(t *testing.T) {
	repo, _ := openStaffCountsStore(t, nil)
	got, err := repo.StaffCounts(tenant.Into(context.Background(), ckXa))
	if err != nil {
		t.Fatalf("StaffCounts: %v", err)
	}
	if got.Total != 0 || got.Published != 0 || got.Departments == nil || len(got.Departments) != 0 {
		t.Fatalf("xã rỗng = %+v, muốn 0/0 và departments rỗng (không nil)", got)
	}
}

func TestStaffCountsWithoutCommunePanics(t *testing.T) {
	repo, rec := openStaffCountsStore(t, nil)
	defer func() {
		if recover() == nil {
			t.Fatal("đếm danh bạ không có xã trong ngữ cảnh mà không panic")
		}
		if len(rec.lenh) != 0 {
			t.Fatalf("không có xã mà vẫn gửi %d câu lệnh", len(rec.lenh))
		}
	}()
	repo.StaffCounts(context.Background()) //nolint:errcheck // must panic before returning
}
