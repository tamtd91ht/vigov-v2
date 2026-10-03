package store

import (
	"strings"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// The body-image reads of stored_file (ADR 0067 §Sửa đổi 03/10/2026) against a REAL PostgreSQL. Skipped
// without VIGOV_TEST_DSN (shared harness: loai_tai_nguyen_ban_do_pg_test.go) — a green run without the DSN
// proves nothing about these statements. EACH ROW MAKES ONE DEFECT VISIBLE:
//
//	body-1, body-2  xã 1, item, content-body-image, by CB-A    must appear, in id order
//	cover           xã 1, item, content-image                  drop the purpose clause → appears
//	other-item      xã 1, another item, content-body-image     drop the subject clause → appears
//	deleted         xã 1, item, content-body-image, deleted    drop `deleted_at IS NULL` → appears
//	xa-2            xã 2, item (same id), content-body-image   drop the commune → appears in xã 1
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
	insert(xa2, "01JXA2AAAAAAAAAAAAAAAAAAAA", "content-body-image", item, "CB-A")
	ctx1 := ctxXa(tenant.ID(xa1))
	if err := db.For(ctx1).Tx(ctx1, func(tx *pkgstore.ScopedTx) error {
		return s.SoftDelete(ctx1, tx, "01JDELETEDAAAAAAAAAAAAAAAA", "CB-A", "gỡ khỏi thân bài", now)
	}); err != nil {
		t.Fatalf("soft delete: %v", err)
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
