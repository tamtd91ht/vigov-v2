package migrations

import (
	"io/fs"
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0005 (user decision 09/10/2026: Tắt/Bật on every shipped sentence). It
// reads the SQL this binary embeds; it does not prove PostgreSQL enforces any of it (tools/schema-smoke).
//
// THE MUTATIONS THAT MUST TURN THIS RED: the switched-off unworded row stored with a copy of the default
// (no DROP NOT NULL); the CHECK that forbids a live unworded row switched ON going missing; a destructive
// or backfilling statement.
func TestMigration0005(t *testing.T) {
	b, err := fs.ReadFile(FS, "0005_system_message_switch_without_wording.sql")
	if err != nil {
		t.Fatal(err)
	}
	// Executable part only: `--` comments removed FIRST (the REVERSAL block writes DDL in prose),
	// whitespace collapsed, lower-cased.
	var lines []string
	for _, l := range strings.Split(string(b), "\n") {
		if i := strings.Index(l, "--"); i >= 0 {
			l = l[:i]
		}
		lines = append(lines, l)
	}
	sql := strings.ToLower(strings.Join(strings.Fields(strings.Join(lines, " ")), " "))
	for _, c := range []struct{ want, why string }{
		{"alter table system_message_override alter column message_text drop not null;",
			"a switched-off sentence the commune never reworded stores no wording"},
		{"add constraint system_message_override_wording_or_off check (message_text is not null or not is_active);",
			"a live row without wording is only ever a switched-off one"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0005 lacks %q — %s", c.want, c.why)
		}
	}
	for _, banned := range []string{"drop table", "drop column", "delete from", "insert into", "update system_message_override", "truncate"} {
		if strings.Contains(sql, banned) {
			t.Errorf("0005 contains %q — this file only relaxes a column and adds a CHECK", banned)
		}
	}
}
