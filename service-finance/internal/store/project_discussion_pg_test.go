package store

// Integration tests for §8.1 issues and §8.4 discussion (migration 0015) against a real PostgreSQL.
// SKIPPED UNLESS VIGOV_TEST_DSN IS SET — a green run without it means the code COMPILES.
//
// WHAT ONLY A SERVER CAN SAY: that DISTINCT ON really picks the latest issue per project, that a
// soft-deleted project's issues leave every read, that a colliding id of another commune is never
// read, that the triggers refuse an edit and a reopen, and that the JSONB mentions round-trip.

import (
	"context"
	"errors"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

func TestPgProjectDiscussionReadsWritesAndIsolation(t *testing.T) {
	db := moKetNoi(t)
	commune, other := xaRieng(t)
	scoped := pkgstore.New(db)
	r := NewProjectDiscussionStore(scoped)
	w := NewProjectDiscussionWriteStore(scoped)
	ctx := ctxXa(tenant.ID(commune))
	otherCtx := ctxXa(tenant.ID(other))

	themDuAnToiThieu(t, db, commune, "da-1", "DA01", 2026, 100)
	themDuAnToiThieu(t, db, commune, "da-2", "DA02", 2026, 100)
	// The SAME project id in another commune, with an issue of its own.
	themDuAnToiThieu(t, db, other, "da-1", "DA01", 2026, 100)

	tx := func(c context.Context, f func(tx *pkgstore.ScopedTx) error) {
		t.Helper()
		if err := scoped.For(c).Tx(c, f); err != nil {
			t.Fatalf("tx: %v", err)
		}
	}
	tx(ctx, func(tx *pkgstore.ScopedTx) error {
		code, err := w.LiveProjectCode(ctx, tx, "da-1")
		if err != nil || code != "DA01" {
			t.Fatalf("mã dự án = %q, %v", code, err)
		}
		for _, i := range []domain.ProjectIssue{
			{ID: "01JISSUE0000000000000000A1", ProjectID: "da-1", Title: "Cũ", RecordedBy: "CB-00001"},
			{ID: "01JISSUE0000000000000000A2", ProjectID: "da-1", Title: "Mới", Description: "chi tiết", RecordedBy: "CB-00002"},
		} {
			if _, err := w.InsertIssue(ctx, tx, i); err != nil {
				return err
			}
		}
		_, err = w.InsertComment(ctx, tx, domain.ProjectComment{
			ID: "01JCOMMENT00000000000000A1", ProjectID: "da-1", Body: "Ý kiến", AuthorCode: "CB-00001",
			MentionedStaffCodes: []string{"CB-00011", "CB-00012"},
		})
		return err
	})
	tx(otherCtx, func(tx *pkgstore.ScopedTx) error {
		_, err := w.InsertIssue(otherCtx, tx, domain.ProjectIssue{
			ID: "01JISSUE0000000000000000B1", ProjectID: "da-1", Title: "Của xã khác", RecordedBy: "CB-99999"})
		return err
	})

	issues, err := r.IssuesOfProject(ctx, "da-1")
	if err != nil || len(issues) != 2 || issues[0].Title != "Mới" || issues[0].Description != "chi tiết" {
		t.Fatalf("timeline = %+v, %v", issues, err)
	}
	latest, err := r.LatestIssuesOfYear(ctx, LocDuAn{Nam: 2026})
	if err != nil || len(latest) != 1 || latest["da-1"].Title != "Mới" {
		t.Fatalf("mới nhất = %+v, %v", latest, err)
	}
	if n, err := r.OpenIssueCount(ctx, 2026); err != nil || n != 2 {
		t.Fatalf("chưa gỡ = %d, %v; muốn 2 (không đếm xã khác)", n, err)
	}
	comments, err := r.CommentsOfProject(ctx, "da-1")
	if err != nil || len(comments) != 1 || len(comments[0].MentionedStaffCodes) != 2 {
		t.Fatalf("trao đổi = %+v, %v", comments, err)
	}
	if _, err := r.IssuesOfProject(ctx, "da-unknown"); !errors.Is(err, ErrKhongThayDuAn) {
		t.Fatalf("dự án lạ: %v", err)
	}

	// Resolve once; the second is refused, and the trigger refuses a reopen underneath.
	tx(ctx, func(tx *pkgstore.ScopedTx) error {
		if _, err := w.IssueForUpdate(ctx, tx, "01JISSUE0000000000000000A2"); err != nil {
			return err
		}
		_, err := w.ResolveIssue(ctx, tx, "01JISSUE0000000000000000A2", "CB-00003")
		return err
	})
	err = scoped.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		_, err := w.ResolveIssue(ctx, tx, "01JISSUE0000000000000000A2", "CB-00004")
		return err
	})
	if !errors.Is(err, domain.ErrIssueAlreadyResolved) {
		t.Fatalf("gỡ lần hai: %v", err)
	}
	if _, err := db.Exec(`UPDATE project_issues SET resolved_at = NULL, resolved_by = NULL
		WHERE tenant_id = $1 AND id = $2`, commune, "01JISSUE0000000000000000A2"); err == nil {
		t.Fatal("trigger để mở lại một vướng mắc đã gỡ")
	}
	if _, err := db.Exec(`UPDATE project_comments SET body = 'sửa' WHERE tenant_id = $1`, commune); err == nil {
		t.Fatal("trigger để sửa một ý kiến đã gửi")
	}
	if n, _ := r.OpenIssueCount(ctx, 2026); n != 1 {
		t.Fatalf("chưa gỡ sau khi gỡ một = %d, muốn 1", n)
	}

	// Another commune cannot reach this commune's issue, even with the id.
	err = scoped.For(otherCtx).Tx(otherCtx, func(tx *pkgstore.ScopedTx) error {
		_, err := w.IssueForUpdate(otherCtx, tx, "01JISSUE0000000000000000A1")
		return err
	})
	if !errors.Is(err, ErrIssueNotFound) {
		t.Fatalf("xã khác khoá được vướng mắc của xã này: %v", err)
	}

	// A soft-deleted project leaves every read.
	if _, err := db.Exec(`UPDATE du_an SET deleted_at = now(), deleted_by = 'CB-1', delete_reason = 'x'
		WHERE tenant_id = $1 AND id = 'da-1'`, commune); err != nil {
		t.Fatalf("xoá mềm dự án: %v", err)
	}
	if n, _ := r.OpenIssueCount(ctx, 2026); n != 0 {
		t.Fatalf("dự án đã xoá vẫn đếm %d vướng mắc", n)
	}
	if latest, _ := r.LatestIssuesOfYear(ctx, LocDuAn{Nam: 2026}); len(latest) != 0 {
		t.Fatalf("dự án đã xoá vẫn có vướng mắc mới nhất: %+v", latest)
	}
}
