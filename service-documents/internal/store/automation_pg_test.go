package store

import (
	"context"
	"database/sql"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

// Against a real PostgreSQL: the automation read returns exactly the open, live documents of ONE
// commune, and the hold anchor is the first routing entry of the CURRENT unassigned hold.
//
// ⚠ SKIPS WITHOUT VIGOV_TEST_DSN, and the package still prints `ok`. The harness is in
// loai_van_ban_pg_test.go; insertHeldIncoming in org_unit_holdings_pg_test.go.

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
	db := moKetNoi(t)
	tenantA, tenantB := xaRieng(t)
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

	s := NewVanBanDenStore(pkgstore.New(db))
	recs, err := s.OpenIncomingForAutomation(ctxXa(tenantA))
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].ID != "vb-held" {
		t.Fatalf("xã A: %+v, muốn đúng vb-held (đã giải quyết, đã xoá, xã B bị loại)", recs)
	}
	if !recs[0].HoldStartedAt.Equal(t3) {
		t.Errorf("mốc bắt đầu giữ = %s, muốn %s (đợt giữ HIỆN TẠI)", recs[0].HoldStartedAt, t3)
	}
	if recs[0].Code != "VB-DEN-2026-0001" || recs[0].OrgUnitID != "bp-1" || recs[0].AssigneeMa != "" ||
		recs[0].Deadline.IsZero() {
		t.Errorf("dòng = %+v", recs[0])
	}
}

// Against a real PostgreSQL: the clerk's deadline round-trips exactly through UpdateDeadline, and the
// letter read returns only open, live letters of ONE commune whose CURRENT phase has a deadline, with
// the hold anchor taken from the routing rows of the letter log.
func TestPgCitizenLetterDeadlineAndAutomationRead(t *testing.T) {
	db := moKetNoi(t)
	a, b := xaRieng(t)
	ctxA, ctxB := ctxXa(a), ctxXa(b)
	kho := pkgstore.New(db)
	s := NewCitizenLetterStore(kho)
	series := NewDaySoStore(kho)
	due := time.Date(2026, 10, 20, 9, 45, 30, 0, time.UTC)
	t1 := time.Date(2026, 10, 2, 2, 0, 0, 0, time.UTC)

	book := func(ctx context.Context, id, unit string) {
		t.Helper()
		err := kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
			n, err := series.CapSo(ctx, tx, SeriesCitizenLetter, 2026)
			if err != nil {
				return err
			}
			l := domain.CitizenLetter{ID: id, Number: n, Year: 2026, ReceivedDate: t1, Type: domain.LetterTypeFeedback,
				Summary: "Đề nghị sửa đường", HoldingUnitID: unit, CreatedByCode: "CB-TEST01"}
			if err := s.Insert(ctx, tx, l); err != nil {
				return err
			}
			if unit != "" {
				return s.InsertLog(ctx, tx, domain.LetterLogEntry{ID: id + "-r", LetterID: id, At: t1,
					ActorCode: "CB-TEST01", Kind: domain.LetterLogRouting, ToUnitID: unit, Content: "Chuyển ngay khi vào sổ"})
			}
			return nil
		})
		mustAccept(t, err, "vào sổ "+id)
	}
	setDue := func(ctx context.Context, id string, d time.Time) {
		t.Helper()
		err := kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
			l, err := s.ForUpdate(ctx, tx, id)
			if err != nil {
				return err
			}
			if l, err = l.SetActiveDue(d); err != nil {
				return err
			}
			return s.UpdateDeadline(ctx, tx, l, "CB-TEST01")
		})
		mustAccept(t, err, "đặt hạn "+id)
	}
	book(ctxA, a+"-due", "bp-1")
	book(ctxA, a+"-none", "")
	book(ctxB, b+"-due", "")
	setDue(ctxA, a+"-due", due)
	setDue(ctxB, b+"-due", due)

	_ = kho.For(ctxA).Tx(ctxA, func(tx *pkgstore.ScopedTx) error {
		l, err := s.ByID(ctxA, tx, a+"-due")
		if err != nil || !l.ProcessingDueAt.Equal(due) || !l.ResolutionDueAt.IsZero() {
			t.Errorf("hạn đọc lại = %v / %v (%v), muốn đúng %v", l.ProcessingDueAt, l.ResolutionDueAt, err, due)
		}
		return nil
	})

	recs, err := s.OpenLettersForAutomation(ctxA)
	mustAccept(t, err, "đọc cho việc nền")
	if len(recs) != 1 || recs[0].ID != a+"-due" {
		t.Fatalf("xã A: %+v, muốn đúng đơn có hạn (đơn không hạn và xã B bị loại)", recs)
	}
	r := recs[0]
	if r.Code != "DT-2026-0001" || !r.Deadline.Equal(due) || r.OrgUnitID != "bp-1" || r.AssigneeMa != "" ||
		!r.HoldStartedAt.Equal(t1) {
		t.Errorf("dòng = %+v", r)
	}

	// Cleared → no longer a record of the run.
	setDue(ctxA, a+"-due", time.Time{})
	if recs, _ := s.OpenLettersForAutomation(ctxA); len(recs) != 0 {
		t.Errorf("bỏ hạn mà việc nền vẫn đọc: %+v", recs)
	}
}
