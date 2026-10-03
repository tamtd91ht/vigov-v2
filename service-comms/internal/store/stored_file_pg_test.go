package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// THE 20-BODY-IMAGE CAP UNDER CONCURRENCY, for an article with NO ROW yet (a reserved id): nothing but
// LockSubjectCount serialises the count. A holds the lock, counts 19 and keeps its transaction open while
// B starts; B must wait for A's commit and then count 20. Drop the lock and B counts 19 against A's
// uncommitted insert under READ COMMITTED — both insert, the article holds 21, and this test sees it.
func TestPgBodyImageCountLockAdmitsExactlyOneAt19(t *testing.T) {
	xa1, _ := xaRieng(t)
	db := pkgstore.New(moKetNoi(t))
	s := NewStoredFileStore(db)
	const subject, purpose, max = "01JRESERVEDAAAAAAAAAAAAAAA", "content-body-image", 20
	ctx := ctxXa(tenant.ID(xa1))
	now := time.Now().UTC()

	row := func(id string) domain.StoredFile {
		return domain.StoredFile{ID: id, Bucket: domain.StoredFileBucketPrivate,
			ObjectKey:      "content-source/t_" + strings.ToLower(xa1) + "/2026/10/comms/" + purpose + "/" + strings.ToLower(id) + "/original.jpg",
			RetentionClass: "content-source", Purpose: purpose, SubjectType: domain.StoredFileSubjectContentItem,
			SubjectID: subject, OriginalName: "anh.jpg", UploadedBy: "CB-A", CreatedAt: now, UpdatedAt: now}
	}
	for i := 0; i < max-1; i++ {
		f := row(fmt.Sprintf("01JSEED%019d", i))
		if err := db.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error { return s.InsertPending(ctx, tx, f) }); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	// admit is admitSubject's count half followed by the insert, in ONE transaction. counted, when set, is
	// closed once admit has counted; it then waits for release before it inserts.
	admit := func(id string, counted chan<- struct{}, release <-chan struct{}) error {
		return db.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
			if err := s.LockSubjectCount(ctx, tx, subject, purpose); err != nil {
				return err
			}
			n, err := s.CountForSubjectTx(ctx, tx, subject, purpose, now.Add(-time.Hour))
			if err != nil {
				return err
			}
			if counted != nil {
				close(counted)
				<-release
			}
			if n >= max {
				return errCapReached
			}
			return s.InsertPending(ctx, tx, row(id))
		})
	}

	counted, release := make(chan struct{}), make(chan struct{})
	errA := make(chan error, 1)
	go func() { errA <- admit("01JRACEAAAAAAAAAAAAAAAAAAA", counted, release) }()
	<-counted // A holds the lock and has counted 19
	errB := make(chan error, 1)
	go func() { errB <- admit("01JRACEBBBBBBBBBBBBBBBBBBB", nil, nil) }()
	time.Sleep(300 * time.Millisecond) // B reaches the lock (and, without it, would count 19 here)
	close(release)

	a, b := <-errA, <-errB
	if a != nil {
		t.Fatalf("A (first to the lock): %v", a)
	}
	if !errors.Is(b, errCapReached) {
		t.Errorf("B: err = %v, want the cap — two admits at 19 both passed", b)
	}
	n, err := s.CountForSubject(ctx, subject, purpose)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	var pending int
	if err := db.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		pending, err = s.CountForSubjectTx(ctx, tx, subject, purpose, now.Add(-time.Hour))
		return err
	}); err != nil {
		t.Fatalf("count tx: %v", err)
	}
	if pending != max || n != 0 {
		t.Errorf("live = %d (stored %d), want exactly %d pending", pending, n, max)
	}
}

var errCapReached = errors.New("test: cap reached")

// walkToReady takes a pending row to `ready` along the guarded edges, as a completion does.
func walkToReady(t *testing.T, ctx context.Context, tx *pkgstore.ScopedTx, s *StoredFileStore, id string, at time.Time) error {
	t.Helper()
	if err := s.Transition(ctx, tx, id, domain.StoredFilePending, domain.StoredFileScanning, at); err != nil {
		return err
	}
	if err := s.MarkStored(ctx, tx, id, domain.StoredFileFacts{MIMEType: "image/jpeg", SizeBytes: 10,
		SHA256: strings.Repeat("ab", 32)}, at); err != nil {
		return err
	}
	if err := s.Transition(ctx, tx, id, domain.StoredFileStored, domain.StoredFileProcessing, at); err != nil {
		return err
	}
	return s.Transition(ctx, tx, id, domain.StoredFileProcessing, domain.StoredFileReady, at)
}

// ISOLATION #2 (03/10/2026): abandoned pending uploads and rejected files accumulate on an article and are
// retired by nothing. LiveForSubject reads READY rows only, so 250 of them beside 3 ready images are not
// read at all — before the status clause they passed MaxStoredFileBatch and the read refused (a 500 on the
// public detail and on every save of the article).
func TestPgLiveForSubjectReadsReadyRowsOnly(t *testing.T) {
	xa1, _ := xaRieng(t)
	db := pkgstore.New(moKetNoi(t))
	s := NewStoredFileStore(db)
	const item, purpose = "01JITEMCCCCCCCCCCCCCCCCCCC", "content-body-image"
	ctx := ctxXa(tenant.ID(xa1))
	now := time.Now().UTC()

	var ready []string
	err := db.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		for i := 0; i < 253; i++ {
			id := fmt.Sprintf("01JROW%020d", i)
			f := domain.StoredFile{ID: id, Bucket: domain.StoredFileBucketPrivate,
				ObjectKey:      "content-source/t_" + strings.ToLower(xa1) + "/2026/10/comms/" + purpose + "/" + strings.ToLower(id) + "/original.jpg",
				RetentionClass: "content-source", Purpose: purpose, SubjectType: domain.StoredFileSubjectContentItem,
				SubjectID: item, OriginalName: "anh.jpg", UploadedBy: "CB-A", CreatedAt: now, UpdatedAt: now}
			if err := s.InsertPending(ctx, tx, f); err != nil {
				return err
			}
			switch {
			case i < 125: // rejected at completion
				if err := s.Transition(ctx, tx, id, domain.StoredFilePending, domain.StoredFileRejected, now); err != nil {
					return err
				}
			case i < 250: // abandoned: left pending past its form's lifetime
			default:
				if err := walkToReady(t, ctx, tx, s, id, now); err != nil {
					return err
				}
				ready = append(ready, id)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	got, err := s.LiveForSubject(ctx, item, purpose)
	if err != nil {
		t.Fatalf("LiveForSubject over 250 non-ready rows: %v", err)
	}
	var ids []string
	for _, f := range got {
		ids = append(ids, f.ID)
	}
	if strings.Join(ids, ",") != strings.Join(ready, ",") {
		t.Errorf("LiveForSubject = %v, want only the ready %v", ids, ready)
	}
}

// The body-image reads of stored_file (ADR 0067 §Sửa đổi 03/10/2026) against a REAL PostgreSQL. Skipped
// without VIGOV_TEST_DSN (shared harness: loai_tai_nguyen_ban_do_pg_test.go) — a green run without the DSN
// proves nothing about these statements. EACH ROW MAKES ONE DEFECT VISIBLE:
//
//	body-1, body-2  xã 1, item, content-body-image, by CB-A    must appear, in id order
//	cover           xã 1, item, content-image                  drop the purpose clause → appears
//	other-item      xã 1, another item, content-body-image     drop the subject clause → appears
//	deleted         xã 1, item, content-body-image, deleted    drop `deleted_at IS NULL` → appears
//	pending         xã 1, item, content-body-image, pending    drop `status = 'ready'` → appears
//	xa-2            xã 2, item (same id), content-body-image   drop the commune → appears in xã 1
//	on-deleted      xã 1, a SOFT-DELETED article's id, by CB-A drop the NOT EXISTS → reserved again
func TestPgStoredFileBodyImageReads(t *testing.T) {
	xa1, xa2 := xaRieng(t)
	db := pkgstore.New(moKetNoi(t))
	s := NewStoredFileStore(db)
	const item, otherItem = "01JITEMAAAAAAAAAAAAAAAAAAA", "01JITEMBBBBBBBBBBBBBBBBBBB"
	now := time.Now().UTC()

	insert := func(xa, id, purpose, subject, by string) {
		t.Helper()
		f := domain.StoredFile{ID: id, Bucket: domain.StoredFileBucketPrivate,
			ObjectKey:      "content-source/t_" + strings.ToLower(xa) + "/2026/10/comms/" + purpose + "/" + strings.ToLower(id) + "/original.jpg",
			RetentionClass: "content-source", Purpose: purpose, SubjectType: domain.StoredFileSubjectContentItem,
			SubjectID: subject, OriginalName: "anh.jpg", UploadedBy: by, CreatedAt: now, UpdatedAt: now}
		ctx := ctxXa(tenant.ID(xa))
		if err := db.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error { return s.InsertPending(ctx, tx, f) }); err != nil {
			t.Fatalf("insert %s: %v", id, err)
		}
	}
	insert(xa1, "01JBODY2AAAAAAAAAAAAAAAAAA", "content-body-image", item, "CB-A")
	insert(xa1, "01JBODY1AAAAAAAAAAAAAAAAAA", "content-body-image", item, "CB-A")
	insert(xa1, "01JCOVERAAAAAAAAAAAAAAAAAA", "content-image", item, "CB-A")
	insert(xa1, "01JOTHERAAAAAAAAAAAAAAAAAA", "content-body-image", otherItem, "CB-A")
	insert(xa1, "01JDELETEDAAAAAAAAAAAAAAAA", "content-body-image", item, "CB-A")
	insert(xa1, "01JPENDINGAAAAAAAAAAAAAAAA", "content-body-image", item, "CB-A") // never made ready
	insert(xa2, "01JXA2AAAAAAAAAAAAAAAAAAAA", "content-body-image", item, "CB-A")
	// Every row but the pending one is READY, so that each other clause is what keeps it out.
	for xa, ids := range map[string][]string{
		xa1: {"01JBODY2AAAAAAAAAAAAAAAAAA", "01JBODY1AAAAAAAAAAAAAAAAAA", "01JCOVERAAAAAAAAAAAAAAAAAA",
			"01JOTHERAAAAAAAAAAAAAAAAAA", "01JDELETEDAAAAAAAAAAAAAAAA"},
		xa2: {"01JXA2AAAAAAAAAAAAAAAAAAAA"},
	} {
		ctx := ctxXa(tenant.ID(xa))
		if err := db.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
			for _, id := range ids {
				if err := walkToReady(t, ctx, tx, s, id, now); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			t.Fatalf("ready: %v", err)
		}
	}
	ctx1 := ctxXa(tenant.ID(xa1))
	if err := db.For(ctx1).Tx(ctx1, func(tx *pkgstore.ScopedTx) error {
		return s.SoftDelete(ctx1, tx, "01JDELETEDAAAAAAAAAAAAAAAA", "CB-A", "gỡ khỏi thân bài", now)
	}); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	// A SOFT-DELETED ARTICLE whose author still holds a live file on its id (ISOLATION #3, rule 7 inv 3).
	const deletedItem = "01JITEMDELETEDAAAAAAAAAAAA"
	insert(xa1, "01JONDELETEDAAAAAAAAAAAAAA", "content-body-image", deletedItem, "CB-A")
	kho := NewNoiDungMiniAppStore(db)
	if err := db.For(ctx1).Tx(ctx1, func(tx *pkgstore.ScopedTx) error {
		if err := kho.Chen(ctx1, tx, domain.NoiDungMiniApp{ID: deletedItem, Loai: domain.LoaiTinTuc, TieuDe: "Đăng nhầm",
			NgayDang: now, TrangThai: domain.TrangThaiAn, NguoiTaoMa: "CB-A"}); err != nil {
			return err
		}
		return kho.SoftDelete(ctx1, tx, deletedItem, "CB-A", "Đăng nhầm bài", now)
	}); err != nil {
		t.Fatalf("deleted article: %v", err)
	}

	got, err := s.LiveForSubject(ctx1, item, "content-body-image")
	if err != nil {
		t.Fatalf("LiveForSubject: %v", err)
	}
	var ids []string
	for _, f := range got {
		ids = append(ids, f.ID)
	}
	if strings.Join(ids, ",") != "01JBODY1AAAAAAAAAAAAAAAAAA,01JBODY2AAAAAAAAAAAAAAAAAA" {
		t.Errorf("live body images of the item in xã 1 = %v", ids)
	}

	err = db.For(ctx1).Tx(ctx1, func(tx *pkgstore.ScopedTx) error {
		for _, tc := range []struct {
			subject, by string
			want        bool
		}{
			{item, "CB-A", true},
			{item, "CB-B", false},                         // another officer's reservation
			{"01JNEVERMINTEDAAAAAAAAAAAA", "CB-A", false}, // nobody's
			{deletedItem, "CB-A", false},                  // an issued id, its article soft-deleted
		} {
			ok, err := s.SubjectReservedBy(ctx1, tx, tc.subject, tc.by)
			if err != nil {
				return err
			}
			if ok != tc.want {
				t.Errorf("SubjectReservedBy(%s, %s) = %v, want %v", tc.subject, tc.by, ok, tc.want)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("SubjectReservedBy: %v", err)
	}
	// xã 2 holds a file under the same subject id, by the same code: never a reservation in xã 1's sense,
	// and xã 1's files never one in xã 2's.
	ctx2 := ctxXa(tenant.ID(xa2))
	got2, err := s.LiveForSubject(ctx2, item, "content-body-image")
	if err != nil || len(got2) != 1 || got2[0].ID != "01JXA2AAAAAAAAAAAAAAAAAAAA" {
		t.Errorf("xã 2 sees %v (err %v), want only its own file", got2, err)
	}
}
