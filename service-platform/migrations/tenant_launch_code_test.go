package migrations

import (
	"io/fs"
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0024 (tenant.launch_code, owner decision 12/10/2026).
//
// It reads the SQL this binary embeds and asserts the column, the backfill, the trail and the keys
// are WRITTEN. It does not prove PostgreSQL enforces any of it: that needs a database, and this
// service's pg suites SKIP without VIGOV_TEST_DSN.

const file0024 = "0024_tenant_launch_code.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: a column DEFAULT (a second, silent minting path that hides
// a forgotten Go mint); a code drawn from anything but gen_random_uuid's random digits (a hash of the
// name/domain/id is derivable — owner decision; rule 13 forbidden #2), or a slice that reaches the
// version nibble (digit 13, always '4'); the backfill losing its `IS NULL` filter (a re-run would
// re-draw and kill printed QRs); the trail dropped or its actor no longer the system principal; NOT
// NULL, the shape CHECK or the unique index removed; a WHERE on the unique index (an inactive
// commune's code could be drawn again — rule 7 invariant 3); anything dropped, deleted or retyped.
func TestMigration0024TenantLaunchCode(t *testing.T) {
	sql := executableSQL(t, file0024)
	for _, c := range []struct{ want, why string }{
		{"alter table tenant add column if not exists launch_code text;", "nullable until backfilled, no default"},
		{"check (launch_code ~ '^[0-9a-f]{12}$')", "12 lowercase hex"},
		{"set launch_code = substr(replace(gen_random_uuid()::text, '-', ''), 1, 12) where t.launch_code is null", "CSPRNG digits 1-12, re-run safe"},
		{"insert into audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta) " +
			"select c.id, 'system', 'system', '', 'launch_code.issued', c.ten, now(),", "one system entry per commune, same statement"},
		{"'launch_code', c.launch_code", "the trail keeps the code (reversal recovers it from here)"},
		{"from changed c;", "the trail reads the rows the UPDATE actually wrote"},
		{"alter table tenant alter column launch_code set not null;", "a forgotten Go mint fails loudly"},
		{"create unique index if not exists tenant_launch_code_unique on tenant (launch_code);", "one code, one commune, no WHERE"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("%s lacks %q — %s", file0024, c.want, c.why)
		}
	}
	for _, c := range []struct{ banned, why string }{
		{"default", "a default is a second, silent minting path"},
		{"md5(", "derivable / weak (rule 13)"},
		{"sha1(", "derivable / weak (rule 13)"},
		{"random()", "not a CSPRNG (rule 13)"},
		{"digest(", "a hash of something is derivable"},
		{", 1, 13)", "slice reaches the UUID version nibble"},
		{"unique index if not exists tenant_launch_code_unique on tenant (launch_code) where", "partial unique index lets a code be reissued"},
		{"drop ", "drops something"},
		{"delete from", "deletes rows"},
		{"truncate", "truncates"},
		{"cap_nhat_luc", "the last human change keeps its timestamp"},
		{"set ten", "a commune's name is not this file's business"},
		{"set dang_hoat_dong", "a commune's status is not this file's business"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("%s contains %q — %s", file0024, c.banned, c.why)
		}
	}
	if n := strings.Count(sql, "update "); n != 1 {
		t.Errorf("%s runs %d UPDATE statements, want 1", file0024, n)
	}
}

// The unique index is single-column on purpose; tools/check_khoa_duy_nhat.py accepts it only with an
// `@scope: platform` mark within the 40 lines above it.
func TestMigration0024ScopeMark(t *testing.T) {
	b, err := fs.ReadFile(FS, file0024)
	if err != nil {
		t.Fatalf("read %s: %v", file0024, err)
	}
	src := string(b)
	idx := strings.Index(src, "CREATE UNIQUE INDEX IF NOT EXISTS tenant_launch_code_unique")
	if idx < 0 {
		t.Fatalf("%s: unique index not found", file0024)
	}
	above := strings.Split(src[:idx], "\n")
	if len(above) > 40 {
		above = above[len(above)-40:]
	}
	found := false
	for _, l := range above {
		if strings.Contains(l, "@scope:") && strings.Contains(l, "platform") {
			found = true
		}
	}
	if !found {
		t.Errorf("%s: `-- @scope:  platform` must sit within 40 lines above tenant_launch_code_unique", file0024)
	}
	if strings.Contains(src, "-- @entity:") {
		t.Errorf("%s adds a column, not an entity: no @entity mark", file0024)
	}
}
