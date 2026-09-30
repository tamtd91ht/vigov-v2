package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0020 (open question #39). The weaker half: it proves the columns
// and the constraint are WRITTEN, not that PostgreSQL enforces them — but it always runs, so a
// deleted line turns red somewhere.
func TestMigration0020SignInLockout(t *testing.T) {
	sql := maChay(t, "0020_staff_sign_in_lockout.sql")
	for _, want := range []string{
		"alter table nguoi_dung add column if not exists failed_sign_in_count int not null default 0",
		"alter table nguoi_dung add column if not exists sign_in_locked_until timestamptz",
		"add constraint nguoi_dung_failed_sign_in_count_range check (failed_sign_in_count >= 0)",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("0020 no longer contains %q", want)
		}
	}
	// Additive only: this file must never drop or rewrite what the manual lock (#10) owns.
	for _, banned := range []string{"drop ", "alter column", "delete from", "update nguoi_dung"} {
		if strings.Contains(sql, banned) {
			t.Errorf("0020 contains %q — the automatic lock is additive and separate from dang_hoat_dong", banned)
		}
	}
}
