package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0033 (ADR 0079 Q2/Q5: system-message switch, commune sentences;
// row 5: catalogue colour). It reads the SQL this binary embeds; it does not prove PostgreSQL enforces
// any of it — that needs VIGOV_TEST_DSN (tools/schema-smoke).

const file0033 = "0033_system_message_switch_custom_messages_catalogue_color.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: the switch nullable or defaulting to false (every existing
// override would silently switch off); the unique key losing tenant_id or becoming live-rows-only (a
// deleted key reissued — rule 7 invariant 3); the key no longer bound to its group prefix (a commune key
// able to equal a shipped one); the delete CHECK admitting a NULL who; the guards losing DELETE refusal;
// the table without partitions; a destructive statement.
func TestMigration0033(t *testing.T) {
	sql := maChay(t, file0033)
	for _, c := range []struct{ want, why string }{
		{"alter table system_message_override add column if not exists is_active boolean not null default true;",
			"every existing override stays in force"},
		{"before delete on system_message_override for each row execute function system_message_override_no_delete()",
			"override hard delete refused"},
		{"create table if not exists custom_system_message (", "commune sentences table"},
		{"primary key (tenant_id, id), unique (tenant_id, message_key),",
			"keys composite with tenant_id; the unique key counts deleted rows"},
		{"check (group_code in ('phan-anh', 'chung'))", "ADR 0079 Q5a: petitions holds Phản ánh + Dùng chung"},
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
		{"on custom_system_message (tenant_id, group_code, message_key) where deleted_at is null",
			"list index starts with tenant_id"},
		{"alter table loai_nhiem_vu add column if not exists color text constraint loai_nhiem_vu_color_shape check (color ~ '^#[0-9a-fa-f]{6}$');",
			"task type colour"},
		{"alter table muc_uu_tien_nhiem_vu add column if not exists color text constraint muc_uu_tien_nhiem_vu_color_shape check (color ~ '^#[0-9a-fa-f]{6}$');",
			"task priority colour"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0033 lacks %q — %s", c.want, c.why)
		}
	}
	for _, banned := range []string{"drop table", "drop column", "delete from", "insert into", "update system_message_override set",
		"update loai_nhiem_vu set", "update muc_uu_tien_nhiem_vu set", "truncate"} {
		if strings.Contains(sql, banned) {
			t.Errorf("0033 contains %q — this file only adds", banned)
		}
	}
	b, err := FS.ReadFile(file0033)
	if err != nil {
		t.Fatalf("read %s: %v", file0033, err)
	}
	if !strings.Contains(strings.ReplaceAll(string(b), "\r\n", "\n"),
		"-- @entity: PetitionsCustomSystemMessage\n-- @scope:  tenant") {
		t.Error("0033 must declare the new table's ownership mark (tools/kb/ownership.go)")
	}
}
