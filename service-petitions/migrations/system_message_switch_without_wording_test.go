package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0035 (user decision 09/10/2026: Tắt/Bật on every shipped sentence). It
// reads the SQL this binary embeds; it does not prove PostgreSQL enforces any of it (tools/schema-smoke).
//
// THE MUTATIONS THAT MUST TURN THIS RED: the switched-off unworded row stored with a copy of the default
// (no DROP NOT NULL); the CHECK that forbids a live unworded row switched ON going missing; a destructive
// or backfilling statement.
func TestMigration0035(t *testing.T) {
	const file = "0035_system_message_switch_without_wording.sql"
	sql := maChay(t, file)
	for _, c := range []struct{ want, why string }{
		{"alter table system_message_override alter column message_text drop not null;",
			"a switched-off sentence the commune never reworded stores no wording"},
		{"add constraint system_message_override_wording_or_off check (message_text is not null or not is_active);",
			"a live row without wording is only ever a switched-off one"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0035 lacks %q — %s", c.want, c.why)
		}
	}
	for _, banned := range []string{"drop table", "drop column", "delete from", "insert into", "update system_message_override", "truncate"} {
		if strings.Contains(sql, banned) {
			t.Errorf("0035 contains %q — this file only relaxes a column and adds a CHECK", banned)
		}
	}
}
