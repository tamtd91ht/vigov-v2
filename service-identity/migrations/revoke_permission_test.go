package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT check for migration 0022. Same weaker-half caveat as danh_ba_mini_app_test.go: it
// proves the seed is WRITTEN, not that PostgreSQL ran it.

const migration0022 = "0022_revoke_sign_in_account_permission.sql"

// `admin.user.revoke` is seeded (ADR 0035 / #27) — TASK-02b's route checks it, and a key no
// migration seeds is a route that answers 403 to every account forever (rule 5, invariant 3c).
func TestMigration0022SeedsAdminUserRevoke(t *testing.T) {
	sql := maChay(t, migration0022)
	if !strings.Contains(sql, "insert into quyen (ma, nhom, nhan, thu_tu) values ('admin.user.revoke', 'quản trị',") {
		t.Error("0022 no longer seeds admin.user.revoke into group QUẢN TRỊ")
	}
}

// GRANTED TO NO ROLE (0022 header): the migration must not decide who may cut a colleague off.
// A future edit that adds a grant here must go red and be a person's decision.
func TestMigration0022GrantsNoRole(t *testing.T) {
	sql := maChay(t, migration0022)
	if strings.Contains(sql, "vai_tro_quyen") {
		t.Error("0022 writes vai_tro_quyen — granting admin.user.revoke is the commune administrator's act, not a migration's")
	}
}
