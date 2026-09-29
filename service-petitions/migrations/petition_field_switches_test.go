package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0022 (tier-2 switches on `nhan_linh_vuc`). Runs always; the
// PostgreSQL behaviour needs VIGOV_TEST_DSN.

const file0022 = "0022_petition_field_switches.sql"

// TestMigration0022AddsSwitchesWithoutLosingAnything — THE MUTATIONS THAT MUST TURN THIS RED: a default
// of false (every existing commune would lose every field on its citizens' form); making the column
// nullable (three states for an on/off switch); bounding the label without allowing NULL (the inherit
// spelling); dropping or deleting anything.
func TestMigration0022AddsSwitchesWithoutLosingAnything(t *testing.T) {
	sql := maChay(t, file0022)
	for _, c := range []struct{ want, why string }{
		{"alter table nhan_linh_vuc add column if not exists enabled boolean not null default true",
			"no row = ON, and existing rows stay ON"},
		{"alter table nhan_linh_vuc alter column nhan drop not null", "NULL is the inherit-the-default spelling"},
		{"check (nhan is null or (btrim(nhan) <> '' and char_length(nhan) <= 100))",
			"a present label is non-blank and within the write path's 100 characters"},
		{"check (thu_tu between 0 and 9999)", "0 = inherit, 1..9999 a position"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0022 lacks %q — %s", c.want, c.why)
		}
	}
	for _, c := range []struct{ banned, why string }{
		{"drop column", "nothing is dropped"},
		{"drop table", "nothing is dropped"},
		{"delete from", "configuration is never deleted"},
		{"create table", "this file adds no table"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0022 contains %q — %s", c.banned, c.why)
		}
	}
}
