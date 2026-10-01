package store

import (
	"context"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/vihat/vigov/core/migrate"
	"github.com/vihat/vigov/service-platform/migrations"
)

// migrationsBefore returns the embedded migrations whose name sorts before stop — the schema as it
// stood before that file was released, so a test can put a row into a state 0012 must respect.
func migrationsBefore(t *testing.T, stop string) fs.FS {
	t.Helper()
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	out := fstest.MapFS{}
	for _, e := range entries {
		if e.IsDir() || e.Name() >= stop {
			continue
		}
		b, err := fs.ReadFile(migrations.FS, e.Name())
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		out[e.Name()] = &fstest.MapFile{Data: b}
	}
	return out
}

// A content-image limit an operator set before 0012 ran is a limit somebody chose and signed: 0012
// must leave it — values, signer and trail — exactly as it found it.
func TestPgContentImageCoverKeepsOperatorEdit(t *testing.T) {
	db, _ := moKetNoi(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if _, err := migrate.Chay(ctx, db, migrationsBefore(t, contentImageCoverMigration), "platform"); err != nil {
		t.Fatalf("migrate up to 0011: %v", err)
	}
	if _, err := db.Exec(`UPDATE upload_policy SET max_bytes = 20971520, updated_by = 'VH-00001'
		WHERE purpose = 'content-image'`); err != nil {
		t.Fatalf("operator edit: %v", err)
	}
	chayMigration(t, db)

	var maxBytes int64
	var mimes, by string
	if err := db.QueryRow(`SELECT max_bytes, allowed_mime_types::text, updated_by FROM upload_policy
		WHERE purpose = 'content-image'`).Scan(&maxBytes, &mimes, &by); err != nil {
		t.Fatalf("read row: %v", err)
	}
	if maxBytes != 20971520 || by != "VH-00001" || !strings.Contains(mimes, "image/heic") {
		t.Errorf("0012 overwrote an operator's edit: max_bytes %d, mimes %s, by %s", maxBytes, mimes, by)
	}
	var changed int
	if err := db.QueryRow(`SELECT count(*) FROM platform_audit_log
		WHERE action = 'upload_policy.changed' AND subject = 'content-image'`).Scan(&changed); err != nil {
		t.Fatalf("count change trail: %v", err)
	}
	if changed != 0 {
		t.Errorf("change trail entries = %d, want 0 — nothing was changed", changed)
	}
}

// The same guarantee for 0014 (G3): a petition-photo limit an operator set before it ran is kept.
func TestPgPetitionPhotoNoHEICKeepsOperatorEdit(t *testing.T) {
	db, _ := moKetNoi(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if _, err := migrate.Chay(ctx, db, migrationsBefore(t, petitionPhotoNoHEICMigration), "platform"); err != nil {
		t.Fatalf("migrate up to 0013: %v", err)
	}
	if _, err := db.Exec(`UPDATE upload_policy SET max_files_per_subject = 3, updated_by = 'VH-00001'
		WHERE purpose = 'petition-photo'`); err != nil {
		t.Fatalf("operator edit: %v", err)
	}
	chayMigration(t, db)

	var maxFiles int
	var mimes, by string
	if err := db.QueryRow(`SELECT max_files_per_subject, allowed_mime_types::text, updated_by FROM upload_policy
		WHERE purpose = 'petition-photo'`).Scan(&maxFiles, &mimes, &by); err != nil {
		t.Fatalf("read row: %v", err)
	}
	if maxFiles != 3 || by != "VH-00001" || !strings.Contains(mimes, "image/heic") {
		t.Errorf("0014 overwrote an operator's edit: max_files %d, mimes %s, by %s", maxFiles, mimes, by)
	}
	var changed int
	if err := db.QueryRow(`SELECT count(*) FROM platform_audit_log
		WHERE action = 'upload_policy.changed' AND subject = 'petition-photo'`).Scan(&changed); err != nil {
		t.Fatalf("count change trail: %v", err)
	}
	if changed != 0 {
		t.Errorf("change trail entries = %d, want 0 — nothing was changed", changed)
	}
}

// Integration tests for migration 0008 against a real PostgreSQL — skipped without VIGOV_TEST_DSN,
// like every *_pg_test.go here. The CHECKs, triggers and seed are behaviour the database owns.

func TestPgUploadPolicySeedAndRead(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)

	ps, err := NewUploadPolicyStore(db).ListUploadPolicies(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	// 0008 seeds six, 0010 seeds task-attachment.
	if len(ps) != 7 {
		t.Fatalf("policies = %d, want the 7 seeded", len(ps))
	}
	for _, p := range ps {
		switch p.Purpose {
		case "petition-photo":
			// 0014 (G3, ADR 0047): jpeg/png/webp, no HEIC; 10 MiB and 5 files kept from 0008.
			if !p.FileCountLimited || p.MaxFilesPerSubject != 5 || p.MaxBytes != 10485760 ||
				!slices.Equal(p.AllowedMIMETypes, []string{"image/jpeg", "image/png", "image/webp"}) {
				t.Errorf("petition-photo = %+v", p)
			}
		case "content-video":
			if p.FileCountLimited || p.MaxBytes != 2147483648 || len(p.AllowedMIMETypes) != 2 {
				t.Errorf("content-video = %+v", p)
			}
		case "content-image":
			// 0012 (người dùng chốt 01/10/2026): 50 MiB, jpeg/png/webp, no HEIC, no count limit.
			if p.FileCountLimited || p.MaxBytes != 52428800 ||
				!slices.Equal(p.AllowedMIMETypes, []string{"image/jpeg", "image/png", "image/webp"}) {
				t.Errorf("content-image = %+v", p)
			}
		}
	}

	// 0012 rewrote content-image once, with one trail entry carrying 0008's values as before.
	var changed int
	var before, after string
	if err := db.QueryRow(`SELECT count(*), min(before::text), min(after::text) FROM platform_audit_log
		WHERE action = 'upload_policy.changed' AND actor = 'system' AND subject = 'content-image'`).
		Scan(&changed, &before, &after); err != nil {
		t.Fatalf("count change trail: %v", err)
	}
	if changed != 1 || !strings.Contains(before, `10485760`) || !strings.Contains(before, `image/heic`) ||
		!strings.Contains(after, `52428800`) || strings.Contains(after, `image/heic`) {
		t.Errorf("content-image change trail: %d entries, before %s, after %s", changed, before, after)
	}

	// 0014 rewrote petition-photo once: before holds HEIC, after does not, size and count unchanged.
	if err := db.QueryRow(`SELECT count(*), min(before::text), min(after::text) FROM platform_audit_log
		WHERE action = 'upload_policy.changed' AND actor = 'system' AND subject = 'petition-photo'`).
		Scan(&changed, &before, &after); err != nil {
		t.Fatalf("count petition-photo change trail: %v", err)
	}
	if changed != 1 || !strings.Contains(before, `image/heic`) || strings.Contains(after, `image/heic`) ||
		!strings.Contains(after, `10485760`) || !strings.Contains(after, `"max_files_per_subject": 5`) {
		t.Errorf("petition-photo change trail: %d entries, before %s, after %s", changed, before, after)
	}

	var entries int
	if err := db.QueryRow(`SELECT count(*) FROM platform_audit_log
		WHERE action = 'upload_policy.seeded' AND actor = 'system'`).Scan(&entries); err != nil {
		t.Fatalf("count trail: %v", err)
	}
	if entries != 7 {
		t.Errorf("seed trail entries = %d, want 7 — one per seeded policy, same transaction", entries)
	}
}

func TestPgUploadPolicySoftDeletedIsAbsent(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	if _, err := db.Exec(`UPDATE upload_policy SET deleted_at = now(), deleted_by = 'system',
		delete_reason = 'test' WHERE purpose = 'tenant-logo'`); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	ps, err := NewUploadPolicyStore(db).ListUploadPolicies(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, p := range ps {
		if p.Purpose == "tenant-logo" {
			t.Fatal("soft-deleted policy still served")
		}
	}
	if len(ps) != 6 {
		t.Errorf("policies = %d, want 6", len(ps))
	}
}

func TestPgUploadPolicyConstraints(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	// Each statement breaks exactly one rule on an otherwise valid seeded row. An UPDATE that matched
	// no row would return no error and be reported as accepted — a false red, never a false green.
	for name, stmt := range map[string]string{
		"max_bytes zero":         `UPDATE upload_policy SET max_bytes = 0 WHERE purpose = 'content-image'`,
		"max_bytes over 5 GiB":   `UPDATE upload_policy SET max_bytes = 5368709121 WHERE purpose = 'content-image'`,
		"empty MIME list":        `UPDATE upload_policy SET allowed_mime_types = '{}' WHERE purpose = 'content-image'`,
		"max files zero":         `UPDATE upload_policy SET max_files_per_subject = 0 WHERE purpose = 'content-image'`,
		"half a soft delete":     `UPDATE upload_policy SET deleted_at = now() WHERE purpose = 'content-image'`,
		"blank signer":           `UPDATE upload_policy SET updated_by = ' ' WHERE purpose = 'content-image'`,
		"unknown purpose":        `INSERT INTO upload_policy (purpose, max_bytes, allowed_mime_types, created_by, updated_by) VALUES ('comms', 1, '{image/png}', 'system', 'system')`,
		"hard delete refused":    `DELETE FROM upload_policy WHERE purpose = 'content-image'`,
		"trail update refused":   `UPDATE platform_audit_log SET actor = 'x'`,
		"trail delete refused":   `DELETE FROM platform_audit_log`,
		"trail truncate refused": `TRUNCATE platform_audit_log`,
	} {
		if _, err := db.Exec(stmt); err == nil {
			t.Errorf("%s: the database accepted it", name)
		}
	}
}
