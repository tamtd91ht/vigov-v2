package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// Integration test for migration 0021 through StoredFileStore.
//
// ⚠ READ THIS BEFORE BELIEVING A GREEN RUN: it SKIPS unless VIGOV_TEST_DSN is set, and the package
// still prints `ok`. The harness (TestMain, moKetNoi, xaRieng) lives in danh_muc_nhiem_vu_pg_test.go.
// This is the only place that proves the CHECKs and the three triggers do what 0021 says.

const pgSHA = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func pgFile(commune, id, task, uploader string, at time.Time) domain.StoredFile {
	return domain.StoredFile{
		ID: id, Bucket: domain.StoredFileBucketPrivate,
		ObjectKey: "records/t_" + strings.ToLower(commune) + "/2026/09/petitions/task-attachment/" +
			strings.ToLower(id) + "/original.pdf",
		RetentionClass: "records", Purpose: "task-attachment",
		SubjectType: domain.StoredFileSubjectTask, SubjectID: task,
		OriginalName: "tai-lieu.pdf", UploadedBy: uploader, CreatedAt: at,
	}
}

func pgEntry(id, task, author string, at time.Time) domain.NhatKyNhiemVu {
	return domain.NhatKyNhiemVu{ID: id, NhiemVuID: task, ThoiDiem: at, NguoiMa: author,
		TrangThaiTaiThoiDiem: domain.DangThucHien, NoiDung: "Đã gửi tài liệu."}
}

// pgStoredFile issues and completes one upload, in its own transactions — as the flow does.
func pgStoredFile(t *testing.T, h *pkgstore.DB, s *StoredFileStore, commune string, f domain.StoredFile) {
	t.Helper()
	ctx := ctxXa(tenant.ID(commune))
	steps := []func(*pkgstore.ScopedTx) error{
		func(tx *pkgstore.ScopedTx) error { return s.InsertPending(ctx, tx, f) },
		func(tx *pkgstore.ScopedTx) error {
			return s.Transition(ctx, tx, f.ID, domain.StoredFilePending, domain.StoredFileScanning, f.CreatedAt)
		},
		func(tx *pkgstore.ScopedTx) error {
			return s.MarkStored(ctx, tx, f.ID, domain.StoredFileFacts{MIMEType: "application/pdf",
				SizeBytes: 2048, SHA256: pgSHA}, f.CreatedAt)
		},
	}
	for i, step := range steps {
		if err := h.For(ctx).Tx(ctx, step); err != nil {
			t.Fatalf("upload %s step %d: %v", f.ID, i, err)
		}
	}
}

func TestPgTaskLogAttachmentFlowAndIsolation(t *testing.T) {
	db := moKetNoi(t)
	commune, other := xaRieng(t)
	h := pkgstore.New(db)
	s := NewStoredFileStore(h)
	logs := NewNhiemVuStore(h)
	at := time.Date(2026, 9, 29, 2, 0, 0, 0, time.UTC)
	ctx := ctxXa(tenant.ID(commune))

	pgStoredFile(t, h, s, commune, pgFile(commune, "01JBPG0000000000000000FA01", "nv-a1", "CB-00311", at))
	pgStoredFile(t, h, s, commune, pgFile(commune, "01JBPG0000000000000000FA02", "nv-a1", "CB-00311", at))
	// The SAME file id in another commune: a different row, never visible here (rule 1).
	pgStoredFile(t, h, s, other, pgFile(other, "01JBPG0000000000000000FA01", "nv-a1", "CB-00311", at))

	// The entry and its links, in ONE transaction.
	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		if err := logs.GhiNhatKy(ctx, tx, pgEntry("nk-a1", "nv-a1", "CB-00311", at)); err != nil {
			return err
		}
		return s.LinkToLogEntry(ctx, tx, "nk-a1", []string{"01JBPG0000000000000000FA01", "01JBPG0000000000000000FA02"})
	}); err != nil {
		t.Fatalf("write entry with attachments: %v", err)
	}

	got, err := s.AttachmentsByLogEntries(ctx, []string{"nk-a1", "nk-none"})
	if err != nil {
		t.Fatalf("AttachmentsByLogEntries: %v", err)
	}
	if len(got) != 1 || len(got["nk-a1"]) != 2 || got["nk-a1"][0].Status != domain.StoredFileStored ||
		got["nk-a1"][0].SizeBytes != 2048 {
		t.Errorf("attachments = %+v, want two stored files on nk-a1", got)
	}
	if g, err := s.AttachmentsByLogEntries(ctxXa(tenant.ID(other)), []string{"nk-a1"}); err != nil || len(g) != 0 {
		t.Errorf("other commune read %v (err %v), want nothing", g, err)
	}

	// Soft-deleting the file hides it from every read (rule 7 inv 2); the link stays.
	if _, err := db.Exec(`UPDATE stored_file SET deleted_at = now(), deleted_by = 'CB-00001',
		delete_reason = 'thu nghiem' WHERE tenant_id = $1 AND id = $2`, commune, "01JBPG0000000000000000FA02"); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	got, _ = s.AttachmentsByLogEntries(ctx, []string{"nk-a1"})
	if len(got["nk-a1"]) != 1 {
		t.Errorf("after soft delete: %d attachments, want 1", len(got["nk-a1"]))
	}
}

// THE LINK FLOOR: every refusal task_log_attachment_check promises, each from a real INSERT.
func TestPgTaskLogAttachmentRefusals(t *testing.T) {
	db := moKetNoi(t)
	commune, _ := xaRieng(t)
	h := pkgstore.New(db)
	s := NewStoredFileStore(h)
	logs := NewNhiemVuStore(h)
	at := time.Date(2026, 9, 29, 3, 0, 0, 0, time.UTC)
	ctx := ctxXa(tenant.ID(commune))

	pgStoredFile(t, h, s, commune, pgFile(commune, "01JBPG0000000000000000FB01", "nv-b1", "CB-00311", at))
	pgStoredFile(t, h, s, commune, pgFile(commune, "01JBPG0000000000000000FB02", "nv-b2", "CB-00311", at)) // other task
	pgStoredFile(t, h, s, commune, pgFile(commune, "01JBPG0000000000000000FB03", "nv-b1", "CB-00999", at)) // other uploader
	pending := pgFile(commune, "01JBPG0000000000000000FB04", "nv-b1", "CB-00311", at)
	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error { return s.InsertPending(ctx, tx, pending) }); err != nil {
		t.Fatalf("pending: %v", err)
	}

	// An entry written in an EARLIER transaction.
	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return logs.GhiNhatKy(ctx, tx, pgEntry("nk-b-old", "nv-b1", "CB-00311", at))
	}); err != nil {
		t.Fatalf("old entry: %v", err)
	}
	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.LinkToLogEntry(ctx, tx, "nk-b-old", []string{"01JBPG0000000000000000FB01"})
	}); err == nil {
		t.Error("attaching to an entry of an earlier transaction was accepted")
	}

	for i, c := range []struct{ file, why string }{
		{"01JBPG0000000000000000FB02", "a file uploaded for another task"},
		{"01JBPG0000000000000000FB03", "a file another officer uploaded"},
		{"01JBPG0000000000000000FB04", "a pending file"},
		{"01JBPG0000000000000000FB99", "a file that does not exist"},
	} {
		entry := "nk-b-" + string(rune('a'+i))
		if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
			if err := logs.GhiNhatKy(ctx, tx, pgEntry(entry, "nv-b1", "CB-00311", at)); err != nil {
				return err
			}
			return s.LinkToLogEntry(ctx, tx, entry, []string{c.file})
		}); err == nil {
			t.Errorf("%s was attached", c.why)
		}
	}

	// A good link, then the append-only guard.
	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		if err := logs.GhiNhatKy(ctx, tx, pgEntry("nk-b-ok", "nv-b1", "CB-00311", at)); err != nil {
			return err
		}
		return s.LinkToLogEntry(ctx, tx, "nk-b-ok", []string{"01JBPG0000000000000000FB01"})
	}); err != nil {
		t.Fatalf("good link: %v", err)
	}
	for _, stmt := range []string{
		`UPDATE task_log_attachment SET log_entry_id = 'nk-b-old' WHERE tenant_id = $1`,
		`DELETE FROM task_log_attachment WHERE tenant_id = $1`,
		`DELETE FROM stored_file WHERE tenant_id = $1`,
	} {
		if _, err := db.Exec(stmt, commune); err == nil {
			t.Errorf("accepted: %s", stmt)
		}
	}
	// One file, one entry.
	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		if err := logs.GhiNhatKy(ctx, tx, pgEntry("nk-b-twice", "nv-b1", "CB-00311", at)); err != nil {
			return err
		}
		return s.LinkToLogEntry(ctx, tx, "nk-b-twice", []string{"01JBPG0000000000000000FB01"})
	}); err == nil {
		t.Error("one file was attached to two entries")
	}
}

// THE METADATA GUARD AND CHECKS, each from a real statement.
func TestPgStoredFileGuard(t *testing.T) {
	db := moKetNoi(t)
	commune, other := xaRieng(t)
	h := pkgstore.New(db)
	s := NewStoredFileStore(h)
	at := time.Date(2026, 9, 29, 4, 0, 0, 0, time.UTC)
	ctx := ctxXa(tenant.ID(commune))
	const id = "01JBPG0000000000000000FC01"
	pgStoredFile(t, h, s, commune, pgFile(commune, id, "nv-c1", "CB-00311", at))
	pgStoredFile(t, h, s, other, pgFile(other, id, "nv-c1", "CB-00311", at))

	// Status only along the edges; the store method refuses before SQL, the trigger refuses raw SQL.
	if _, err := db.Exec(`UPDATE stored_file SET status = 'pending' WHERE tenant_id = $1 AND id = $2`, commune, id); err == nil {
		t.Error("stored → pending accepted")
	}
	for _, stmt := range []string{
		`UPDATE stored_file SET sha256 = '` + strings.Repeat("f", 64) + `' WHERE tenant_id = $1 AND id = $2`,
		`UPDATE stored_file SET subject_id = 'nv-other' WHERE tenant_id = $1 AND id = $2`,
		`UPDATE stored_file SET object_key = object_key || 'x' WHERE tenant_id = $1 AND id = $2`,
		`UPDATE stored_file SET uploaded_by = 'CB-00999' WHERE tenant_id = $1 AND id = $2`,
		// a records file that reached the destination is never purged
		`UPDATE stored_file SET status = 'purged', purged_at = now(), purged_by = 'system', purge_reason = 'x' WHERE tenant_id = $1 AND id = $2`,
		// a soft delete with no reason
		`UPDATE stored_file SET deleted_at = now(), deleted_by = 'CB-00001' WHERE tenant_id = $1 AND id = $2`,
	} {
		if _, err := db.Exec(stmt, commune, id); err == nil {
			t.Errorf("accepted: %s", stmt)
		}
	}

	// A key built for another commune's tree cannot be recorded under this one.
	bad := pgFile(other, "01JBPG0000000000000000FC02", "nv-c1", "CB-00311", at)
	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error { return s.InsertPending(ctx, tx, bad) }); err == nil {
		t.Error("a key naming another commune was accepted")
	}
	// The same object key twice in one commune.
	dup := pgFile(commune, id, "nv-c1", "CB-00311", at)
	dup.ID = "01JBPG0000000000000000FC03"
	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error { return s.InsertPending(ctx, tx, dup) }); err == nil {
		t.Error("a second row for one object key was accepted")
	}

	// ForUpdate + a stale Transition: the second move of one row finds nothing to move.
	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		f, err := s.ForUpdate(ctx, tx, id)
		if err != nil || f == nil || f.Status != domain.StoredFileStored || f.SHA256 != pgSHA {
			t.Errorf("ForUpdate = %+v, %v", f, err)
		}
		return s.Transition(ctx, tx, id, domain.StoredFileStored, domain.StoredFileProcessing, at)
	}); err != nil {
		t.Fatalf("stored → processing: %v", err)
	}
	err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.Transition(ctx, tx, id, domain.StoredFileStored, domain.StoredFileProcessing, at)
	})
	if !errors.Is(err, ErrStoredFileMoved) {
		t.Errorf("stale transition: err = %v, want ErrStoredFileMoved", err)
	}
	if f, err := s.forUpdateOutsideTest(ctxXa(tenant.ID(other)), h, id); err != nil || f == nil || f.Status != domain.StoredFileStored {
		t.Errorf("the other commune's row of the same id moved too: %+v, %v", f, err)
	}
}

// SoftDelete against the real CHECK and trigger: the three columns land together, the row vanishes from
// every read, a second removal finds nothing, the link stays, the other commune's same id is untouched.
func TestPgStoredFileSoftDelete(t *testing.T) {
	db := moKetNoi(t)
	commune, other := xaRieng(t)
	h := pkgstore.New(db)
	s := NewStoredFileStore(h)
	logs := NewNhiemVuStore(h)
	at := time.Date(2026, 10, 7, 2, 0, 0, 0, time.UTC)
	ctx := ctxXa(tenant.ID(commune))
	const id = "01JBPG0000000000000000FD01"
	pgStoredFile(t, h, s, commune, pgFile(commune, id, "nv-d1", "CB-00311", at))
	pgStoredFile(t, h, s, other, pgFile(other, id, "nv-d1", "CB-00311", at))
	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		if err := logs.GhiNhatKy(ctx, tx, pgEntry("nk-d1", "nv-d1", "CB-00311", at)); err != nil {
			return err
		}
		return s.LinkToLogEntry(ctx, tx, "nk-d1", []string{id})
	}); err != nil {
		t.Fatalf("entry with attachment: %v", err)
	}

	// A blank reason is refused by the CHECK (stored_file_delete_complete).
	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.SoftDelete(ctx, tx, id, "CB-00311", "  ", at)
	}); err == nil {
		t.Error("a soft delete with a blank reason was accepted")
	}

	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		if linked, err := s.LinkedLogEntryTx(ctx, tx, id); err != nil || linked != "nk-d1" {
			t.Errorf("LinkedLogEntryTx = %q, %v", linked, err)
		}
		return s.SoftDelete(ctx, tx, id, "CB-00311", "tải nhầm tệp", at)
	}); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	var by, reason string
	if err := db.QueryRow(`SELECT deleted_by, delete_reason FROM stored_file WHERE tenant_id = $1 AND id = $2`,
		commune, id).Scan(&by, &reason); err != nil || by != "CB-00311" || reason != "tải nhầm tệp" {
		t.Errorf("row = %q / %q, %v", by, reason, err)
	}
	if f, err := s.ByID(ctx, id); err != nil || f != nil {
		t.Errorf("ByID after removal = %+v, %v — want nothing", f, err)
	}
	if got, err := s.AttachmentsByLogEntries(ctx, []string{"nk-d1"}); err != nil || len(got) != 0 {
		t.Errorf("timeline after removal = %+v, %v", got, err)
	}
	if linked, err := s.LinkedLogEntry(ctx, id); err != nil || linked != "nk-d1" {
		t.Errorf("the link went with the file: %q, %v", linked, err)
	}
	err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.SoftDelete(ctx, tx, id, "CB-00999", "lần hai", at)
	})
	if !errors.Is(err, ErrStoredFileMoved) {
		t.Errorf("second removal: err = %v, want ErrStoredFileMoved", err)
	}
	if f, err := s.ByID(ctxXa(tenant.ID(other)), id); err != nil || f == nil {
		t.Errorf("the other commune's row of the same id was removed too: %+v, %v", f, err)
	}
}

// forUpdateOutsideTest reads one row in its own transaction — for assertions only.
func (s *StoredFileStore) forUpdateOutsideTest(ctx context.Context, h *pkgstore.DB, id string) (*domain.StoredFile, error) {
	var f *domain.StoredFile
	err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		var err error
		f, err = s.ForUpdate(ctx, tx, id)
		return err
	})
	return f, err
}
