package store

import (
	"context"
	"database/sql"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
)

// Against a real PostgreSQL: the automation read returns exactly the open, live documents of ONE
// commune, and the hold anchor is the first routing entry of the CURRENT unassigned hold.
//
// ⚠ SKIPS WITHOUT VIGOV_TEST_DSN, and the package still prints `ok`. The harness is in
// document_type_pg_test.go; insertHeldIncoming in org_unit_holdings_pg_test.go.

func insertRouting(t *testing.T, db *sql.DB, tenantID, id, docID string, at time.Time, unit, holder string) {
	t.Helper()
	var h any
	if holder != "" {
		h = holder
	}
	if _, err := db.ExecContext(context.Background(), `INSERT INTO lich_su_chuyen_van_ban
		(tenant_id, id, van_ban_den_id, thoi_diem, nguoi_ma, trang_thai_tai_thoi_diem, den_bo_phan_id,
		 can_bo_xu_ly_ma, noi_dung)
		VALUES ($1, $2, $3, $4, 'CB-00123', 'da-phan-cong', $5, $6, 'Chuyển xử lý')`,
		tenantID, id, docID, at, unit, h); err != nil {
		t.Fatalf("insert routing %s: %v", id, err)
	}
}

func TestPgOpenIncomingForAutomationHoldAnchor(t *testing.T) {
	db := openDB(t)
	tenantA, tenantB := uniqueTenants(t)
	insertHeldIncoming(t, db, tenantA, 1, "vb-held", "da-phan-cong", "bp-1", false)
	insertHeldIncoming(t, db, tenantA, 2, "vb-done", "da-giai-quyet", "bp-1", false)
	insertHeldIncoming(t, db, tenantA, 3, "vb-deleted", "dang-xu-ly", "bp-1", true)
	insertHeldIncoming(t, db, tenantB, 1, "vb-b", "dang-xu-ly", "bp-1", false)

	// Held by bp-1 unassigned from t1, given to an officer at t2, back to bp-1 unassigned at t3, and
	// re-routed to bp-1 unassigned again at t4 — the episode still began at t3.
	t1 := time.Date(2026, 9, 1, 2, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 9, 3, 2, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 9, 5, 2, 0, 0, 0, time.UTC)
	insertRouting(t, db, tenantA, "ls-1", "vb-held", t1, "bp-1", "")
	insertRouting(t, db, tenantA, "ls-2", "vb-held", t2, "bp-1", "CB-00999")
	insertRouting(t, db, tenantA, "ls-3", "vb-held", t3, "bp-1", "")
	insertRouting(t, db, tenantA, "ls-4", "vb-held", t3.Add(24*time.Hour), "bp-1", "")

	s := NewIncomingDocumentStore(pkgstore.New(db))
	recs, err := s.OpenIncomingForAutomation(tenantCtx(tenantA))
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].ID != "vb-held" {
		t.Fatalf("xã A: %+v, muốn đúng vb-held (đã giải quyết, đã xoá, xã B bị loại)", recs)
	}
	if !recs[0].HoldStartedAt.Equal(t3) {
		t.Errorf("mốc bắt đầu giữ = %s, muốn %s (đợt giữ HIỆN TẠI)", recs[0].HoldStartedAt, t3)
	}
	if recs[0].Code != "VB-DEN-2026-0001" || recs[0].OrgUnitID != "bp-1" || recs[0].AssigneeCode != "" ||
		recs[0].Deadline.IsZero() {
		t.Errorf("dòng = %+v", recs[0])
	}
}
