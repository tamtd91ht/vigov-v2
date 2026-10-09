package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0028 (user decision 2026-10-09). Same weaker-half caveat as
// danh_ba_mini_app_test.go: it proves the statements are WRITTEN this way, not that PostgreSQL ran them.

const migration0028 = "0028_citizen_dossier_notice_permissions.sql"

// The six keys are seeded, each under its group.
func TestMigration0028SeedsTheSixKeys(t *testing.T) {
	sql := maChay(t, migration0028)
	for _, want := range []string{
		"('citizen.read', 'danh bạ người dân',",
		"('citizen.update', 'danh bạ người dân',",
		"('dossier.import', 'hồ sơ công dân',",
		"('dossier.read', 'hồ sơ công dân',",
		"('dossier.update', 'hồ sơ công dân',",
		"('notice.send', 'gửi thông báo',",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("0028 no longer seeds %s", want)
		}
	}
}

// The grant goes to the two leader roles and the administrator role BY CODE, live roles only, idempotently, and its audit entry is
// written from the grant's own RETURNING — same statement, same transaction (rule 6, invariant 3).
func TestMigration0028GrantsLeaderAndAdminRolesWithAudit(t *testing.T) {
	sql := maChay(t, migration0028)
	for _, c := range []struct{ want, why string }{
		{"where vt.deleted_at is null and vt.ma in ('chu-tich-ubnd', 'pho-chu-tich-ubnd', 'quan-tri-he-thong')",
			"the two leader roles plus the administrator role, by stable code, live only (user decision 2026-10-09)"},
		{"on conflict (tenant_id, vai_tro_id, quyen_ma) do nothing returning tenant_id, vai_tro_id, quyen_ma",
			"idempotent: a key already held is neither re-granted nor re-audited"},
		{"insert into audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta) select tenant_id, 'system', 'system', '', 'luu_phan_quyen_vai_tro', vai_tro_id, now(),",
			"one system entry per role that gained a key, the Phân quyền verb, subject the role id"},
		{"from per_role;", "the entry is built from the INSERT's RETURNING"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0028 does not contain %q — %s", c.want, c.why)
		}
	}
	if strings.Count(sql, "insert into vai_tro_quyen") != 1 {
		t.Error("0028 must grant in exactly one statement")
	}
	for _, bad := range []string{"drop ", "delete from", "truncate", "alter table", "update vai_tro"} {
		if strings.Contains(sql, bad) {
			t.Errorf("0028 contains %q in its executable part", bad)
		}
	}
}
