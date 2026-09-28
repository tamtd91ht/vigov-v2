package migrations

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0012 (operator realm, ADR 0048). The weaker half, as
// danh_ba_mini_app_test.go says: it proves the constraints are WRITTEN, not that PostgreSQL enforces
// them. The proof is internal/store/operatorstore/store_pg_test.go, which SKIPS without
// VIGOV_TEST_DSN — so a deleted constraint must turn red somewhere that always runs.

const file0012 = "0012_operator_accounts.sql"

func TestMigration0012Constraints(t *testing.T) {
	sql := maChay(t, file0012)
	for _, c := range []struct{ name, expr, why string }{
		{"operator_account_code_unique", "unique (code)", "a code issued twice is two people in one trail line"},
		{"operator_account_code_shape", "check (code ~ '^vh-[0-9]{5,}$')", "VH- prefix, five digits minimum"},
		{"operator_session_id_is_sha256", "check (id ~ '^[0-9a-f]{64}$')", "a raw sid in the registry is a replayable credential"},
		{"operator_recovery_code_hash_is_sha256", "check (code_hash ~ '^[0-9a-f]{64}$')", "recovery codes stored in the clear"},
		{"operator_recovery_code_used_xor_voided", "check (used_at is null or voided_at is null)", "a code both used and voided"},
		{"operator_audit_log_actor_shape", "check (actor = 'system' or actor ~ '^vh-[0-9]{5,}$')", "rule 6 invariant 8"},
		{"operator_audit_log_subject_shape", "check (subject ~ '^vh-[0-9]{5,}$')", "an email as subject is personal data in the trail"},
	} {
		if !strings.Contains(sql, "constraint "+c.name+" "+c.expr) {
			t.Errorf("0012 NO LONGER has %s %q — %s", c.name, c.expr, c.why)
		}
	}
	for _, want := range []string{
		"create unique index if not exists operator_account_email_unique on operator_account (lower(email))",
		"create unique index if not exists operator_permission_grant_live_unique on operator_permission_grant (operator_account_id, permission_key) where revoked_at is null",
		"create sequence if not exists operator_code_seq as bigint start with 1 minvalue 1 no cycle",
		"before update or delete on operator_audit_log for each row execute function operator_audit_log_append_only()",
		"before truncate on operator_audit_log for each statement execute function operator_audit_log_append_only()",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("0012 NO LONGER contains %q", want)
		}
	}
}

func TestMigration0012NoTenantIDAndEveryTablePlatformScoped(t *testing.T) {
	b, err := fs.ReadFile(FS, file0012)
	if err != nil {
		t.Fatal(err)
	}
	// String literals removed first: the COMMENT ON texts SAY "no tenant_id", which is the point.
	code := regexp.MustCompile(`'[^']*'`).ReplaceAllString(maChay(t, file0012), "''")
	if strings.Contains(code, "tenant_id") {
		t.Error("0012 declares tenant_id — the operator realm has no commune by decision (ADR 0048 §Chốt #1)")
	}
	lines := strings.Split(string(b), "\n")
	reTable := regexp.MustCompile(`^CREATE TABLE IF NOT EXISTS (\w+)`)
	tables := 0
	for i, l := range lines {
		m := reTable.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		tables++
		if i < 2 || !strings.HasPrefix(lines[i-2], "-- @entity: ") ||
			strings.Join(strings.Fields(lines[i-1]), " ") != "-- @scope: platform" {
			t.Errorf("table %s lacks `-- @entity:` + `-- @scope: platform` directly above it", m[1])
		}
	}
	if tables != 5 {
		t.Errorf("found %d tables, want 5 — this check is reading the wrong shape", tables)
	}
}

func TestMigration0012NoDestructiveStatement(t *testing.T) {
	sql := maChay(t, file0012)
	for _, bad := range []string{"drop table", "drop column", "delete from", "truncate table", "drop sequence"} {
		if strings.Contains(sql, bad) {
			t.Errorf("0012 contains %q in its executable part", bad)
		}
	}
}

const file0013 = "0013_operator_session_lifetime_cap.sql"

// The 8-hour cap is written, added only if absent, and refuses (never rewrites) rows already over
// it. Agreement with domain.SessionLifetime is checked in internal/store/operatorstore, which may
// import domain; this package may not reach into internal/.
func TestMigration0013SessionLifetimeCap(t *testing.T) {
	sql := maChay(t, file0013)
	for _, want := range []string{
		"add constraint operator_session_lifetime_cap check (expires_at <= created_at + interval '8 hours')",
		"where conname = 'operator_session_lifetime_cap' and conrelid = 'operator_session'::regclass",
		"raise exception 'operator_session has % row(s) whose expiry exceeds created_at + 8 hours'",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("0013 NO LONGER contains %q", want)
		}
	}
	for _, bad := range []string{"update operator_session", "delete from", "drop table", "drop column", "drop constraint"} {
		if strings.Contains(sql, bad) {
			t.Errorf("0013 contains %q in its executable part — it must refuse, never rewrite", bad)
		}
	}
}
