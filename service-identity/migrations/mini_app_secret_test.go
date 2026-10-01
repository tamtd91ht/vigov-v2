package migrations

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0021 (ADR 0066). The weaker half, as danh_ba_mini_app_test.go
// says: it proves the constraints are WRITTEN, not that PostgreSQL enforces them — but it always
// runs, so a deleted constraint turns red somewhere.

const file0021 = "0021_mini_app_secret.sql"

func TestMigration0021Constraints(t *testing.T) {
	sql := maChay(t, file0021)
	for _, c := range []struct{ want, why string }{
		{"demo_identity_enabled boolean not null default false",
			"--demo identity must be OFF unless an operator turns it on (ADR 0066)"},
		{"constraint mini_app_secret_sealed_or_demo check (app_secret_sealed is not null or demo_identity_enabled)",
			"a NULL secret is admitted only for an app running --demo"},
		{"constraint mini_app_secret_app_id_shape check (app_id ~ '^[0-9]{1,32}$')",
			"an App ID is digits; a pasted secret must be refused"},
		{"constraint mini_app_secret_sealed_length check (app_secret_sealed is null or octet_length(app_secret_sealed) > 29)",
			"shorter than the envelope overhead cannot open"},
		{"set_at timestamptz not null", "a version with no date answers nothing"},
		{"set_by text not null", "rule 6 invariant 8 — unsigned is refused"},
		{"constraint mini_app_secret_soft_delete_complete check ((deleted_at is null and deleted_by is null and delete_reason is null)",
			"soft delete is all three or none"},
		{"live_app_id text generated always as (case when deleted_at is null then app_id end) stored",
			"one LIVE row per (commune, App ID)"},
		{"primary key (tenant_id, id), unique (tenant_id, live_app_id),",
			"rule 1 invariant 6"},
		{"before update or delete on mini_app_secret for each row execute function mini_app_secret_guard()",
			"a version row is immutable and never deleted"},
		{"or new.app_secret_sealed is distinct from old.app_secret_sealed",
			"replacing a secret in place erases who set the old one"},
		{"or new.demo_identity_enabled is distinct from old.demo_identity_enabled",
			"toggling demo in place erases who turned it on"},
		{"if old.deleted_at is not null then raise exception",
			"a retired version is history"},
		{"constraint data_encryption_key_wrapped_length check (octet_length(wrapped) = 65)",
			"version-1 wrapped DEK format"},
		{"primary key (tenant_id), constraint data_encryption_key_kek_id_shape",
			"one DEK per commune"},
		{"before update or delete on data_encryption_key for each row execute function data_encryption_key_guard()",
			"a deleted DEK destroys every secret of that commune"},
		{"create table if not exists mini_app_secret_p%s partition of mini_app_secret",
			"HASH ×32 (ADR 0010)"},
		{"create table if not exists data_encryption_key_p%s partition of data_encryption_key",
			"HASH ×32 (ADR 0010)"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0021 NO LONGER contains %q — %s", c.want, c.why)
		}
	}
	// The checker in tools/check_khoa_duy_nhat.py refuses this shape; the generated column replaces it.
	if regexp.MustCompile(`create unique index[^;]*where[^;]*deleted_at is null`).MatchString(sql) {
		t.Error("0021 uses a partial unique index on deleted_at — use live_app_id")
	}
	for _, bad := range []string{"drop table", "drop column", "delete from", "truncate", "update mini_app_secret", "update data_encryption_key"} {
		if strings.Contains(sql, bad) {
			t.Errorf("0021 contains %q in its executable part", bad)
		}
	}
}

// Every table is declared tenant-scoped, and the DEK entity name is NOT comms' `DataEncryptionKey`:
// one entity name owned by two services is what rule 2 invariant 1 forbids.
func TestMigration0021EntityDeclarations(t *testing.T) {
	b, err := fs.ReadFile(FS, file0021)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(b), "\n")
	reTable := regexp.MustCompile(`^CREATE TABLE IF NOT EXISTS (\w+) \(`)
	want := map[string]string{
		"data_encryption_key": "-- @entity: IdentityDataEncryptionKey",
		"mini_app_secret":     "-- @entity: MiniAppSecret",
	}
	seen := 0
	for i, l := range lines {
		m := reTable.FindStringSubmatch(strings.TrimRight(l, "\r"))
		if m == nil {
			continue
		}
		seen++
		if i < 2 || strings.TrimRight(lines[i-2], "\r") != want[m[1]] ||
			strings.Join(strings.Fields(lines[i-1]), " ") != "-- @scope: tenant" {
			t.Errorf("table %s lacks %q + `-- @scope: tenant` directly above it", m[1], want[m[1]])
		}
	}
	if seen != len(want) {
		t.Errorf("found %d tables, want %d — this check is reading the wrong shape", seen, len(want))
	}
}
