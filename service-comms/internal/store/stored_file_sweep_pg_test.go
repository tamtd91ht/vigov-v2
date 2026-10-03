package store

import (
	"strings"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// THE ABANDONED-DRAFT SWEEP'S TWO STATEMENTS (ADR 0067 K11) against a REAL PostgreSQL — skipped without
// VIGOV_TEST_DSN (shared harness). EACH ROW MAKES ONE DEFECT VISIBLE:
//
//	old         xã 1, never-saved id, ready, completed 8 days ago   must be retired
//	young       xã 1, never-saved id, ready, 6 days                 drop the cut-off → retired
//	live        xã 1, a LIVE article's id                           drop NOT EXISTS → retired
//	of-deleted  xã 1, a SOFT-DELETED article's id                   filter n.deleted_at → retired
//	pending     xã 1, never-saved id, pending, created 9 days ago   drop `status = 'ready'` → retired
//	cover       xã 1, never-saved id, content-image                 drop the purpose → retired
//	xa-2        xã 2, never-saved id, ready, 8 days                 drop the commune → retired by xã 1
func TestPgAbandonedBodyImageSweepStatements(t *testing.T) {
	xa1, xa2 := xaRieng(t)
	db := pkgstore.New(moKetNoi(t))
	s := NewStoredFileStore(db)
	kho := NewNoiDungMiniAppStore(db)
	now := time.Now().UTC()
	const body, cover = "content-body-image", "content-image"

	add := func(xa, id, purpose, subject string, completedAgo time.Duration, ready bool) {
		t.Helper()
		created := now.Add(-completedAgo - time.Hour)
		f := domain.StoredFile{ID: id, Bucket: domain.StoredFileBucketPrivate,
			ObjectKey:      "content-source/t_" + strings.ToLower(xa) + "/2026/10/comms/" + purpose + "/" + strings.ToLower(id) + "/original.jpg",
			RetentionClass: "content-source", Purpose: purpose, SubjectType: domain.StoredFileSubjectContentItem,
			SubjectID: subject, OriginalName: "anh.jpg", UploadedBy: "CB-A", CreatedAt: created, UpdatedAt: created}
		ctx := ctxXa(tenant.ID(xa))
		if err := db.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
			if err := s.InsertPending(ctx, tx, f); err != nil {
				return err
			}
			if !ready {
				return nil
			}
			return walkToReady(t, ctx, tx, s, id, now.Add(-completedAgo))
		}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	const day = 24 * time.Hour
	add(xa1, "01JSWEEPOLDAAAAAAAAAAAAAAA", body, "01JNEVERSAVEDAAAAAAAAAAAAA", 8*day, true)
	add(xa1, "01JSWEEPYOUNGAAAAAAAAAAAAA", body, "01JNEVERSAVEDBBBBBBBBBBBBB", 6*day, true)
	add(xa1, "01JSWEEPLIVEAAAAAAAAAAAAAA", body, "01JITEMLIVESWEEPAAAAAAAAAA", 30*day, true)
	add(xa1, "01JSWEEPOFDELETEDAAAAAAAAA", body, "01JITEMDELSWEEPAAAAAAAAAAA", 30*day, true)
	add(xa1, "01JSWEEPPENDINGAAAAAAAAAAA", body, "01JNEVERSAVEDAAAAAAAAAAAAA", 8*day, false)
	add(xa1, "01JSWEEPCOVERAAAAAAAAAAAAA", cover, "01JNEVERSAVEDAAAAAAAAAAAAA", 8*day, true)
	add(xa2, "01JSWEEPXA2AAAAAAAAAAAAAAA", body, "01JNEVERSAVEDAAAAAAAAAAAAA", 8*day, true)

	ctx1 := ctxXa(tenant.ID(xa1))
	if err := db.For(ctx1).Tx(ctx1, func(tx *pkgstore.ScopedTx) error {
		for _, id := range []string{"01JITEMLIVESWEEPAAAAAAAAAA", "01JITEMDELSWEEPAAAAAAAAAAA"} {
			if err := kho.Chen(ctx1, tx, domain.NoiDungMiniApp{ID: id, Loai: domain.LoaiTinTuc, TieuDe: "Bài",
				NgayDang: now, TrangThai: domain.TrangThaiAn, NguoiTaoMa: "CB-A"}); err != nil {
				return err
			}
		}
		return kho.SoftDelete(ctx1, tx, "01JITEMDELSWEEPAAAAAAAAAAA", "CB-A", "Đăng nhầm bài", now)
	}); err != nil {
		t.Fatalf("articles: %v", err)
	}

	cutoff := now.Add(-7 * day)
	var retired []string
	if err := db.For(ctx1).Tx(ctx1, func(tx *pkgstore.ScopedTx) error {
		locked, err := s.LockAbandonedBodyImages(ctx1, tx, body, cutoff, MaxStoredFileBatch)
		if err != nil {
			return err
		}
		if strings.Join(locked, ",") != "01JSWEEPOLDAAAAAAAAAAAAAAA" {
			t.Errorf("locked = %v, want only the old abandoned image", locked)
		}
		retired, err = s.RetireAbandonedBodyImages(ctx1, tx, locked, body, cutoff, "system", "bài nháp bỏ dở", now)
		return err
	}); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if strings.Join(retired, ",") != "01JSWEEPOLDAAAAAAAAAAAAAAA" {
		t.Errorf("retired = %v", retired)
	}
	if f, err := s.ByID(ctx1, "01JSWEEPOLDAAAAAAAAAAAAAAA"); err != nil || f != nil {
		t.Errorf("the retired row is still live (%v, %v)", f, err)
	}
	for _, id := range []string{"01JSWEEPYOUNGAAAAAAAAAAAAA", "01JSWEEPLIVEAAAAAAAAAAAAAA",
		"01JSWEEPOFDELETEDAAAAAAAAA", "01JSWEEPPENDINGAAAAAAAAAAA", "01JSWEEPCOVERAAAAAAAAAAAAA"} {
		if f, err := s.ByID(ctx1, id); err != nil || f == nil {
			t.Errorf("%s: retired or unreadable (%v)", id, err)
		}
	}
	ctx2 := ctxXa(tenant.ID(xa2))
	if f, err := s.ByID(ctx2, "01JSWEEPXA2AAAAAAAAAAAAAAA"); err != nil || f == nil {
		t.Errorf("xã 2's image was touched by xã 1's sweep (%v)", err)
	}
}
