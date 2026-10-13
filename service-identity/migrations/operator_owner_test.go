package migrations

import (
	"io/fs"
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0030 (operator_account.is_owner, owner decision 13/10/2026). Same
// weaker-half caveat as danh_ba_mini_app_test.go: it proves the DDL is WRITTEN, not that PostgreSQL
// enforces it.

const file0030 = "0030_operator_owner.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: the unique index losing its WHERE (then only ONE ROW of the
// table could exist at all) or keyed on anything but a constant (two owners would differ in the key);
// the owner-not-disabled CHECK removed; the column nullable or defaulting to true; a backfill that
// designates an owner.
func TestMigration0030SingleOwner(t *testing.T) {
	sql := maChay(t, file0030)
	for _, c := range []struct{ want, why string }{
		{"alter table operator_account add column if not exists is_owner boolean not null default false;", "nobody is owner by default"},
		{"create unique index if not exists operator_account_single_owner on operator_account ((true)) where is_owner;", "at most one owner"},
		{"add constraint operator_account_owner_not_disabled check (not is_owner or disabled_at is null);", "the owner can sign in"},
		{"where c.relkind = 'p' and n.nspname = current_schema()", "partition backstop"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("%s lacks %q — %s", file0030, c.want, c.why)
		}
	}
	for _, bad := range []string{"insert into", "update ", "delete from", "truncate", "drop ", "set is_owner"} {
		if strings.Contains(sql, bad) {
			t.Errorf("%s contains %q — the owner is designated by operatorctl set-owner, never by a migration", file0030, bad)
		}
	}
}

// The index has no tenant_id on purpose; tools/check_khoa_duy_nhat.py accepts it only with an
// `@scope: platform` mark within the 40 lines above it.
func TestMigration0030ScopeMark(t *testing.T) {
	b, err := fs.ReadFile(FS, file0030)
	if err != nil {
		t.Fatalf("read %s: %v", file0030, err)
	}
	src := string(b)
	idx := strings.Index(src, "CREATE UNIQUE INDEX IF NOT EXISTS operator_account_single_owner")
	if idx < 0 {
		t.Fatalf("%s: unique index not found", file0030)
	}
	above := strings.Split(src[:idx], "\n")
	if len(above) > 40 {
		above = above[len(above)-40:]
	}
	for _, l := range above {
		if strings.Contains(l, "@scope:") && strings.Contains(l, "platform") {
			return
		}
	}
	t.Errorf("%s: `-- @scope:  platform` must sit within 40 lines above operator_account_single_owner", file0030)
}
