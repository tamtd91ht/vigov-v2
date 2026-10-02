package migrations

import (
	"io/fs"
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0017 (platform `stored_file`, the two ho_so_hien_thi_xa references
// of ADR 0069).
//
// It reads the SQL this binary embeds and asserts the keys, CHECKs and triggers are WRITTEN. It does
// not prove PostgreSQL enforces any of it: that needs a database, and this service's pg suites SKIP
// without VIGOV_TEST_DSN.

const file0017 = "0017_tenant_branding_files.sql"

// executableSQL returns the file with `--` comments removed, whitespace collapsed, lower-cased, so the
// assertions read the statements and never the prose that explains them (the REVERSAL block names
// DROP COLUMN and DROP TABLE on purpose).
func executableSQL(t *testing.T, name string) string {
	t.Helper()
	b, err := fs.ReadFile(FS, name)
	if err != nil {
		t.Fatalf("read %s from the embedded FS: %v", name, err)
	}
	var lines []string
	for _, l := range strings.Split(string(b), "\n") {
		if i := strings.Index(l, "--"); i >= 0 {
			l = l[:i]
		}
		lines = append(lines, l)
	}
	return strings.ToLower(strings.Join(strings.Fields(strings.Join(lines, " ")), " "))
}

// THE MUTATIONS THAT MUST TURN THIS RED: a key without tenant_id; the FK without tenant_id (a profile
// pointing at another commune's file); the guard or the branding check removed; the published-only
// rule loosened; logo_url dropped.
func TestMigration0017KeysConstraintsTriggers(t *testing.T) {
	sql := executableSQL(t, file0017)
	for _, clause := range []string{
		"create table if not exists stored_file (",
		"tenant_id text not null references tenant (id)",
		"primary key (tenant_id, id)",
		"unique (tenant_id, object_key)",
		"unique (tenant_id, public_object_key)",
		"check (purpose in ('tenant-logo', 'tenant-banner'))",
		"check (subject_type in ('tenant-display-profile'))",
		"check (subject_id = tenant_id)",
		"check (retention_class in ('content-source'))",
		"check (bucket in ('private'))",
		"'/platform/' || purpose || '/' || lower(id) || '/'",
		"starts_with(public_object_key, 'public-media/t_' || lower(tenant_id) || '/')",
		"public_object_key is null or (status = 'ready' and deleted_at is null)",
		"on stored_file (tenant_id, subject_type, subject_id, purpose, created_at desc) where deleted_at is null",
		"before update or delete on stored_file for each row execute function stored_file_guard()",
		"add column if not exists logo_file_id text",
		"add column if not exists web_admin_banner_file_id text",
		"foreign key (tenant_id, logo_file_id) references stored_file (tenant_id, id)",
		"foreign key (tenant_id, web_admin_banner_file_id) references stored_file (tenant_id, id)",
		"before insert or update of logo_file_id, web_admin_banner_file_id on ho_so_hien_thi_xa",
		"platform_branding_file_assert(new.tenant_id, new.logo_file_id, 'tenant-logo')",
		"platform_branding_file_assert(new.tenant_id, new.web_admin_banner_file_id, 'tenant-banner')",
		"file_row.status <> 'ready' or file_row.public_object_key is null",
		"for share",
	} {
		if !strings.Contains(sql, clause) {
			t.Errorf("%s lacks %q", file0017, clause)
		}
	}
	// Rule 7: nothing is dropped, retyped or deleted by the statements (the prose may name them).
	for _, forbidden := range []string{"drop column", "drop table", "delete from", "alter column", "truncate"} {
		if strings.Contains(sql, forbidden) {
			t.Errorf("%s executes %q", file0017, forbidden)
		}
	}
}

// The ownership marks tools/kb reads must sit above the CREATE TABLE, with the service in the entity
// name — a bare `StoredFile` would be one entity with three owners in data-ownership.json.
func TestMigration0017OwnershipMarks(t *testing.T) {
	b, err := fs.ReadFile(FS, file0017)
	if err != nil {
		t.Fatalf("read %s: %v", file0017, err)
	}
	src := string(b)
	mark := strings.Index(src, "-- @entity: PlatformStoredFile\n-- @scope:  tenant\n")
	table := strings.Index(src, "CREATE TABLE IF NOT EXISTS stored_file (")
	if mark < 0 || table < 0 || mark > table {
		t.Errorf("@entity: PlatformStoredFile / @scope: tenant must precede CREATE TABLE stored_file")
	}
	if strings.Count(src, "-- @entity:") != 1 {
		t.Errorf("exactly one @entity mark expected in %s", file0017)
	}
}
