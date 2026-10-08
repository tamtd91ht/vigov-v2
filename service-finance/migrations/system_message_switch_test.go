package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0017 (ADR 0079 Q2/Q5: system-message switch, commune sentences of
// group "Giải ngân"; row 5: catalogue colour). Reads the embedded SQL only; PostgreSQL enforcement needs
// VIGOV_TEST_DSN (tools/schema-smoke). Mutations that must turn it red: as petitions' 0033 test.

const file0017 = "0017_system_message_switch_custom_messages_catalogue_color.sql"

func TestMigration0017(t *testing.T) {
	sql := maChay(t, file0017)
	for _, c := range []struct{ want, why string }{
		{"alter table system_message_override add column if not exists is_active boolean not null default true;",
			"every existing override stays in force"},
		{"before delete on system_message_override for each row execute function system_message_override_no_delete()",
			"override hard delete refused"},
		{"create table if not exists custom_system_message (", "commune sentences table"},
		{"primary key (tenant_id, id), unique (tenant_id, message_key),",
			"keys composite with tenant_id; the unique key counts deleted rows"},
		{"check (group_code in ('giai-ngan'))", "ADR 0079 Q5a: finance holds Giải ngân only"},
		{"starts_with(message_key, group_code || '.')", "commune keys cannot collide with shipped keys"},
		{"check ((deleted_at is null) = (deleted_by is null) and (deleted_at is null) = (delete_reason is null) and (deleted_by is null or (btrim(deleted_by) <> '' and char_length(deleted_by) <= 64)) and (delete_reason is null or (btrim(delete_reason) <> '' and char_length(delete_reason) <= 200)))",
			"all three delete facts or none, both directions — no half-delete, no blank who/why"},
		{"check (btrim(created_by) <> '' and char_length(created_by) <= 64 and btrim(updated_by) <> '' and char_length(updated_by) <= 64)",
			"actors present and bounded (rule 6, invariant 8)"},
		{") partition by hash (tenant_id);", "partitioned by commune (ADR 0010)"},
		{"partition of custom_system_message ' 'for values with (modulus 32, remainder %s)", "32 partitions"},
		{"before update or delete on custom_system_message for each row execute function custom_system_message_guard()",
			"guard attached"},
		{"if new.message_key is distinct from old.message_key or new.group_code is distinct from old.group_code then raise exception",
			"issued key immutable"},
		{"if old.deleted_at is not null then raise exception", "deleted sentence closed"},
		{"alter table hang_muc_ke_hoach_von add column if not exists color text constraint hang_muc_ke_hoach_von_color_shape check (color ~ '^#[0-9a-fa-f]{6}$');",
			"funding-plan category colour"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0017 lacks %q — %s", c.want, c.why)
		}
	}
	for _, banned := range []string{"drop table", "drop column", "delete from", "insert into",
		"update system_message_override set", "update hang_muc_ke_hoach_von set", "truncate"} {
		if strings.Contains(sql, banned) {
			t.Errorf("0017 contains %q — this file only adds", banned)
		}
	}
	b, err := FS.ReadFile(file0017)
	if err != nil {
		t.Fatalf("read %s: %v", file0017, err)
	}
	if !strings.Contains(strings.ReplaceAll(string(b), "\r\n", "\n"),
		"-- @entity: FinanceCustomSystemMessage\n-- @scope:  tenant") {
		t.Error("0017 must declare the new table's ownership mark (tools/kb/ownership.go)")
	}
}
