package store

import (
	"database/sql"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// Against a real PostgreSQL: the automation reads return exactly the open, live rows of ONE commune,
// and the hold anchor is the first timeline entry of the CURRENT unassigned hold.
//
// ⚠ SKIPS WITHOUT VIGOV_TEST_DSN, and the package still prints `ok`. The harness (TestMain,
// moKetNoi, xaRieng) lives in danh_muc_nhiem_vu_pg_test.go; the row helpers in
// org_unit_holdings_pg_test.go.

func insertTaskLog(t *testing.T, db *sql.DB, tenantID, id, taskID string, at time.Time, unit, holder string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO nhat_ky_nhiem_vu
		(tenant_id, id, nhiem_vu_id, thoi_diem, nguoi_ma, trang_thai_tai_thoi_diem, bo_phan_id, nguoi_phu_trach_ma, noi_dung)
		VALUES ($1,$2,$3,$4,'CB-00123','moi-giao',$5,$6,'Giao việc')`,
		tenantID, id, taskID, at, nullHoac(unit), nullHoac(holder)); err != nil {
		t.Fatalf("insert task log %s: %v", id, err)
	}
}

func TestPgOpenTasksForAutomationHoldAnchor(t *testing.T) {
	db := moKetNoi(t)
	tenantA, tenantB := xaRieng(t)
	danhMucChoXa(t, db, tenantA)
	danhMucChoXa(t, db, tenantB)

	insertHeldTask(t, db, tenantA, "nv-held", "moi-giao", "bp-1", "", false)
	insertHeldTask(t, db, tenantA, "nv-done", "hoan-thanh", "bp-1", "", false)
	insertHeldTask(t, db, tenantA, "nv-paused", "tam-dung", "bp-1", "", false)
	insertHeldTask(t, db, tenantA, "nv-deleted", "moi-giao", "bp-1", "", true)
	insertHeldTask(t, db, tenantB, "nv-b", "moi-giao", "bp-1", "", false)

	// Held by bp-1 unassigned from t1, handed to an officer at t2, back to bp-1 unassigned at t3.
	t1 := time.Date(2026, 9, 1, 2, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 9, 3, 2, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 9, 5, 2, 0, 0, 0, time.UTC)
	insertTaskLog(t, db, tenantA, "nk-1", "nv-held", t1, "bp-1", "")
	insertTaskLog(t, db, tenantA, "nk-2", "nv-held", t2, "bp-1", "CB-00999")
	insertTaskLog(t, db, tenantA, "nk-3", "nv-held", t3, "bp-1", "")
	insertTaskLog(t, db, tenantA, "nk-4", "nv-held", t3.Add(24*time.Hour), "bp-1", "")

	s := NewNhiemVuStore(pkgstore.New(db))
	recs, err := s.OpenTasksForAutomation(ctxXa(tenant.ID(tenantA)))
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].ID != "nv-held" {
		t.Fatalf("xã A: %+v, muốn đúng nv-held (hoàn thành, tạm dừng, đã xoá, xã B bị loại)", recs)
	}
	if !recs[0].HoldStartedAt.Equal(t3) {
		t.Errorf("mốc bắt đầu giữ = %s, muốn %s (đợt giữ HIỆN TẠI, không phải lần đầu)", recs[0].HoldStartedAt, t3)
	}
	if !recs[0].Deadline.Equal(mocHanGocPg) || recs[0].OrgUnitID != "bp-1" || recs[0].AssigneeMa != "" {
		t.Errorf("dòng = %+v", recs[0])
	}
}

func TestPgCitizenReportAutomationReads(t *testing.T) {
	db := moKetNoi(t)
	tenantA, tenantB := xaRieng(t)
	insertHeldPetition(t, db, tenantA, "pa-open", "dang-xu-ly", "bp-1", false)
	insertHeldPetition(t, db, tenantA, "pa-done", "da-xu-ly", "bp-1", false)
	insertHeldPetition(t, db, tenantA, "pa-deleted", "dang-xu-ly", "bp-1", true)
	insertHeldPetition(t, db, tenantB, "pa-b", "dang-xu-ly", "bp-1", false)

	s := NewPhieuPhanAnhStore(pkgstore.New(db))
	recs, err := s.OpenCitizenReportsForAutomation(ctxXa(tenant.ID(tenantA)))
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].ID != "pa-open" || recs[0].HoldStartedAt.IsZero() {
		t.Fatalf("xã A: %+v", recs)
	}

	now := time.Now().UTC()
	c, err := s.CitizenReportDigestCounts(ctxXa(tenant.ID(tenantA)), now, now.AddDate(0, 0, -7))
	if err != nil {
		t.Fatal(err)
	}
	if c.Examined != 2 { // pa-open + pa-done; soft-deleted and the other commune excluded
		t.Errorf("số phiếu xét = %d, muốn 2", c.Examined)
	}
}
