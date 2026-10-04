package store

import (
	"database/sql"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// PostgreSQL proof that the per-department table (/bao-cao) counts the intended rows, and that its
// department rows add up to the /tong-quan tiles for the same period.
//
// ⚠ SKIPS unless VIGOV_TEST_DSN is set, and the package still prints `ok`. The fake-driver suite
// (task_unit_summary_test.go) proves the statement reuses the tile predicates; only this file proves
// GROUPING SETS, the COALESCE of NULL and '', and the in-hand predicate on a real server.

// insertTaskWithUnit books one task with a department and an explicit creation instant — the two
// columns themNhiemVu leaves to their defaults, and the two this table is about.
func insertTaskWithUnit(t *testing.T, db *sql.DB, tid, id, code, status string, unit any,
	createdAt time.Time, due, completedAt any) {

	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO nhiem_vu
		 (tenant_id, id, ma, loai, tieu_de, trang_thai, nguon_giao,
		  han_xu_ly, han_ban_dau, ngay_hoan_thanh, tien_do, nguoi_tao_ma, bo_phan_id, tao_luc)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$8,$9,$10,$11,$12,$13)`,
		tid, id, code, "theo-van-ban", "Nhiệm vụ dùng cho phép kiểm.", status, "truc-tiep",
		due, completedAt, 0, "CB-00123", unit, createdAt); err != nil {
		t.Fatalf("thêm %s: %v", code, err)
	}
}

func TestPgTaskUnitSummaryGroupsAndAddsUpToTheTiles(t *testing.T) {
	db := moKetNoi(t)
	tenantA, tenantB := xaRieng(t)
	danhMucChoXa(t, db, tenantA)
	danhMucChoXa(t, db, tenantB)

	now := time.Now().UTC()
	past, future := now.Add(-72*time.Hour), now.Add(72*time.Hour)
	// THE PERIOD ENDS AN HOUR AGO, so a task can be created after `to` and still be overdue now with a
	// deadline later than its creation (bp-d). Every deadline below is after its row's creation.
	period := domain.Period{From: now.Add(-7 * 24 * time.Hour), To: now.Add(-time.Hour)}
	inPeriod := now.Add(-24 * time.Hour)
	beforePeriod := period.From.Add(-time.Hour)
	afterPeriod := now.Add(-30 * time.Minute)

	id := func(s string) string { return tenantA[:10] + s }
	// bp-a
	insertTaskWithUnit(t, db, tenantA, id("a1"), "NV01", "dang-thuc-hien", "bp-a", beforePeriod, past, nil)    // total, overdue
	insertTaskWithUnit(t, db, tenantA, id("a2"), "NV02", "hoan-thanh", "bp-a", beforePeriod, future, inPeriod) // total, completed, sample, on_time (carried in)
	insertTaskWithUnit(t, db, tenantA, id("a3"), "NV03", "hoan-thanh", "bp-a", beforePeriod, past,
		beforePeriod.Add(time.Minute)) // finished before the period: nothing
	insertTaskWithUnit(t, db, tenantA, id("a4"), "NV04", "tam-dung", "bp-a", beforePeriod, nil, nil) // total (suspended is still in hand)
	// bp-b
	insertTaskWithUnit(t, db, tenantA, id("b1"), "NV05", "hoan-thanh", "bp-b", inPeriod, nil, inPeriod) // total, completed (nothing promised: no sample)
	// bp-c: only work finished before the period — the department is NOT listed.
	insertTaskWithUnit(t, db, tenantA, id("c1"), "NV06", "hoan-thanh", "bp-c", beforePeriod, nil,
		beforePeriod.Add(time.Minute))
	// bp-d: created after `to` yet overdue now — overdue is stock, total is not.
	insertTaskWithUnit(t, db, tenantA, id("d1"), "NV07", "dang-thuc-hien", "bp-d", afterPeriod,
		now.Add(-10*time.Minute), nil)
	// No department: NULL and '' are ONE row.
	insertTaskWithUnit(t, db, tenantA, id("n1"), "NV08", "moi-giao", nil, inPeriod, future, nil)    // total
	insertTaskWithUnit(t, db, tenantA, id("n2"), "NV09", "cho-duyet", nil, beforePeriod, past, nil) // total, overdue
	insertTaskWithUnit(t, db, tenantA, id("n3"), "NV10", "moi-giao", "", inPeriod, nil, nil)        // total
	// Soft-deleted overdue task in bp-a: in no figure.
	insertTaskWithUnit(t, db, tenantA, id("x1"), "NV11", "dang-thuc-hien", "bp-a", beforePeriod, past, nil)
	if _, err := db.Exec(`UPDATE nhiem_vu SET deleted_at = now(), deleted_by = 'CB-00123',
		delete_reason = 'phép kiểm' WHERE tenant_id = $1 AND ma = 'NV11'`, tenantA); err != nil {
		t.Fatal(err)
	}
	// Another commune's task in a department of the same id.
	insertTaskWithUnit(t, db, tenantB, tenantB[:10]+"a1", "NV01", "dang-thuc-hien", "bp-a", beforePeriod, past, nil)

	s := NewNhiemVuStore(pkgstore.New(db))
	ctx := ctxXa(tenant.ID(tenantA))
	before := time.Now()
	got, err := s.TaskUnitSummary(ctx, period)
	if err != nil {
		t.Fatalf("TaskUnitSummary: %v", err)
	}
	if got.AsOf.Before(before.Add(-time.Minute)) || got.AsOf.After(time.Now().Add(time.Minute)) {
		t.Errorf("AsOf = %v, không phải now() của câu lệnh", got.AsOf)
	}

	want := []domain.TaskUnitFigures{
		{OrgUnitID: "", Total: 3, Overdue: 1},
		{OrgUnitID: "bp-a", Total: 3, Completed: 1, OnTimeSample: 1, OnTime: 1, Overdue: 1},
		{OrgUnitID: "bp-b", Total: 1, Completed: 1},
		{OrgUnitID: "bp-d", Overdue: 1},
	}
	if len(got.Units) != len(want) {
		t.Fatalf("= %+v, muốn %+v", got.Units, want)
	}
	for i := range want {
		if got.Units[i] != want[i] {
			t.Errorf("dòng %d = %+v, muốn %+v", i, got.Units[i], want[i])
		}
	}

	// THE ROWS ADD UP TO THE TILES — the reason the predicates are shared.
	tiles, err := s.TaskSummary(ctx, period)
	if err != nil {
		t.Fatalf("TaskSummary: %v", err)
	}
	var sum domain.TaskUnitFigures
	for _, u := range got.Units {
		sum.Completed += u.Completed
		sum.OnTimeSample += u.OnTimeSample
		sum.OnTime += u.OnTime
		sum.Overdue += u.Overdue
	}
	if sum.Completed != tiles.Completed || sum.OnTimeSample != tiles.OnTimeSample ||
		sum.OnTime != tiles.OnTime || sum.Overdue != tiles.Overdue {
		t.Errorf("tổng các bộ phận = %+v, ô tổng quan = %+v", sum, tiles)
	}
}

// TestPgTaskUnitSummaryEmptyCommuneStillHasAsOf — no task at all: no unit, and the grand row still
// carries now().
func TestPgTaskUnitSummaryEmptyCommuneStillHasAsOf(t *testing.T) {
	db := moKetNoi(t)
	tenantA, _ := xaRieng(t)
	now := time.Now().UTC()
	got, err := NewNhiemVuStore(pkgstore.New(db)).TaskUnitSummary(ctxXa(tenant.ID(tenantA)),
		domain.Period{From: now.Add(-time.Hour), To: now})
	if err != nil {
		t.Fatalf("TaskUnitSummary: %v", err)
	}
	if len(got.Units) != 0 || got.AsOf.IsZero() {
		t.Errorf("= %+v", got)
	}
}
