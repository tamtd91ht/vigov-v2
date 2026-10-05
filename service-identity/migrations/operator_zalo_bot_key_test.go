package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT check for migration 0025. Same weaker-half caveat as danh_ba_mini_app_test.go: it
// proves the DDL is WRITTEN, not that PostgreSQL ran it. The list itself is compared with
// domain.OperatorPermissions() by internal/store/operatorstore TestPermissionCheckMatchesDomainList.

const migration0025 = "0025_operator_zalo_bot_key.sql"

// The eighth key is in the replaced CHECK, and the replacement is ONE statement (drop + add), so a
// half-applied state with no constraint at all cannot exist between two statements.
func TestMigration0025AddsZaloBotKeyInOneStatement(t *testing.T) {
	sql := maChay(t, migration0025)
	want := "alter table operator_permission_grant drop constraint if exists operator_permission_grant_key_known, " +
		"add constraint operator_permission_grant_key_known check (permission_key in ("
	if !strings.Contains(sql, want) {
		t.Fatal("0025 no longer replaces operator_permission_grant_key_known in one ALTER TABLE")
	}
	if !strings.Contains(sql, "'ops.zalo_bot.manage'") {
		t.Error("0025 does not list ops.zalo_bot.manage (ADR 0074 #3)")
	}
	// Widening only: the seventh key of 0023 must still be there.
	if !strings.Contains(sql, "'ops.petition_field.manage'") {
		t.Error("0025 dropped ops.petition_field.manage — the CHECK must only widen")
	}
}

// GRANTED TO NOBODY: a key reaches an operator only through operatorctl grant, which is trailed.
func TestMigration0025GrantsNobody(t *testing.T) {
	sql := maChay(t, migration0025)
	if strings.Contains(sql, "insert into") || strings.Contains(sql, "quyen") {
		t.Error("0025 writes a row — the operator key is granted by operatorctl, and it is never a row of quyen")
	}
}
