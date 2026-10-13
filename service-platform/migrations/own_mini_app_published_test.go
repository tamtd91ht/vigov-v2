package migrations

import (
	"io/fs"
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0025 (mini_app.published_at / published_by, owner decision
// 13/10/2026).
//
// It reads the SQL this binary embeds and asserts the columns and checks are WRITTEN. It does not
// prove PostgreSQL enforces them: that needs a database, and this service's pg suites SKIP without
// VIGOV_TEST_DSN.

const file0025 = "0025_own_mini_app_published.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: a column DEFAULT or a backfill (an app marked published that
// no operator confirmed sends citizens to an app they cannot open); a NOT NULL on either column; the
// both-or-neither check loosened (an unsigned mark — rule 6 invariant 8); the own-app-only check
// removed (a mark on the shared app); anything dropped, deleted, truncated or rewritten.
func TestMigration0025OwnMiniAppPublished(t *testing.T) {
	sql := executableSQL(t, file0025)
	for _, c := range []struct{ want, why string }{
		{"alter table mini_app add column if not exists published_at timestamptz null;", "nullable, NULL = not published"},
		{"alter table mini_app add column if not exists published_by text null;", "nullable, set with published_at"},
		{"check ((published_at is null) = (published_by is null) and (published_by is null or btrim(published_by) <> ''))", "both or neither, never blank"},
		{"check (published_at is null or che_do = 'rieng')", "only a commune's own app is marked"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("%s lacks %q — %s", file0025, c.want, c.why)
		}
	}
	for _, c := range []struct{ banned, why string }{
		{"default", "every existing row must start unpublished"},
		{"update ", "no backfill: an operator marks, a migration never guesses"},
		{"timestamptz not null", "NULL is the meaning of \"not published\""},
		{"text not null", "published_by is NULL while unpublished"},
		{"set not null", "NULL is the meaning of \"not published\""},
		{"drop ", "drops something"},
		{"delete from", "deletes rows"},
		{"truncate", "truncates"},
		{"cap_nhat_luc", "the last human change keeps its timestamp"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("%s contains %q — %s", file0025, c.banned, c.why)
		}
	}
}

// A column on an existing entity, not a new one: no @entity mark (the entity index would list
// MiniApp twice).
func TestMigration0025NoEntityMark(t *testing.T) {
	b, err := fs.ReadFile(FS, file0025)
	if err != nil {
		t.Fatalf("read %s: %v", file0025, err)
	}
	if strings.Contains(string(b), "-- @entity:") {
		t.Errorf("%s adds columns, not an entity: no @entity mark", file0025)
	}
}
