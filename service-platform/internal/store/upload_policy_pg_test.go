package store

import (
	"context"
	"testing"
)

// Integration tests for migration 0008 against a real PostgreSQL — skipped without VIGOV_TEST_DSN,
// like every *_pg_test.go here. The CHECKs, triggers and seed are behaviour the database owns.

func TestPgUploadPolicySeedAndRead(t *testing.T) {
	db, _ := openTestDB(t)
	runMigrations(t, db)

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
			if !p.FileCountLimited || p.MaxFilesPerSubject != 5 || p.MaxBytes != 10485760 ||
				len(p.AllowedMIMETypes) != 4 {
				t.Errorf("petition-photo = %+v", p)
			}
		case "content-video":
			if p.FileCountLimited || p.MaxBytes != 2147483648 || len(p.AllowedMIMETypes) != 2 {
				t.Errorf("content-video = %+v", p)
			}
		}
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
	db, _ := openTestDB(t)
	runMigrations(t, db)
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
	db, _ := openTestDB(t)
	runMigrations(t, db)
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
