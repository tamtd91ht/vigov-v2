package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0028 (per-commune petition settings, ADR 0008 decision 3).
//
// Same standing as petition_staff_files_test.go: it reads the SQL this binary embeds and asserts the
// table, its key, its default and its guard are WRITTEN. It does not prove PostgreSQL enforces any of
// it — that needs VIGOV_TEST_DSN (tools/schema-smoke).

const file0028 = "0028_petition_settings.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: the key losing tenant_id or gaining a second column (two rows
// per commune — which one is in force?); the switch becoming nullable or its default flipping to false
// (a commune with no row would then read differently from a row written with the default, and the
// decided default of ADR 0008 would be silently dropped); the who-columns becoming optional; the guard
// losing its DELETE refusal or its tenant / creation-stamp immutability; the table without partitions.
func TestMigration0028PetitionSettings(t *testing.T) {
	sql := maChay(t, file0028)
	for _, c := range []struct{ want, why string }{
		{"create table if not exists petition_settings (", "the table exists"},
		{"verification_photo_required boolean not null default true",
			"ADR 0008 decision 3: default TRUE, equal to the reader's answer for no row"},
		{"created_by text not null", "who created it — a business code (rule 6, invariant 8)"},
		{"updated_by text not null", "who last changed it — a business code (rule 6, invariant 8)"},
		{"primary key (tenant_id),", "one row per commune (rule 1, invariant 6)"},
		{"constraint petition_settings_created_by_present check (btrim(created_by) <> '')", "no blank who"},
		{"constraint petition_settings_updated_by_present check (btrim(updated_by) <> '')", "no blank who"},
		{") partition by hash (tenant_id);", "partitioned by commune (ADR 0010)"},
		{"partition of petition_settings ' 'for values with (modulus 32, remainder %s)", "32 partitions"},
		{"if tg_op = 'delete' then raise exception", "DELETE refused — no silent return to the default"},
		{"if new.tenant_id is distinct from old.tenant_id then raise exception", "commune immutable"},
		{"if new.created_at is distinct from old.created_at or new.created_by is distinct from old.created_by then raise exception",
			"creation stamp immutable"},
		{"before update or delete on petition_settings for each row execute function petition_settings_guard()",
			"the guard is attached"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0028 lacks %q — %s", c.want, c.why)
		}
	}
	for _, c := range []struct{ banned, why string }{
		{"drop table", "drops a table"},
		{"drop column", "drops a column"},
		{"alter table", "alters an existing table — this file only creates"},
		{"delete from", "deletes rows"},
		{"insert into", "writes rows — schema only; a seeded row would hide the no-row default"},
		{"update petition_settings", "writes rows — schema only"},
		{"deleted_at", "configuration has no soft delete; DELETE is refused instead (header)"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0028 contains %q — %s", c.banned, c.why)
		}
	}
	// FS directly, not petition_staff_files_test.go's mustRead: that file is uncommitted work of another
	// card, and this test must not stop compiling if it moves.
	b, err := FS.ReadFile(file0028)
	if err != nil {
		t.Fatalf("read %s: %v", file0028, err)
	}
	raw := strings.ReplaceAll(string(b), "\r\n", "\n")
	if !strings.Contains(raw, "-- @entity: PetitionSettings\n-- @scope:  tenant") {
		t.Error("0028 must declare the new table's ownership mark (tools/kb/ownership.go)")
	}
}
