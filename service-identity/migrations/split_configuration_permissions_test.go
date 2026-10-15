package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0032 (owner decision 15/10/2026, bug sheet web-admin-141026 row 22).
// The weaker half, as danh_ba_mini_app_test.go says: it proves the statements are WRITTEN this way,
// not that PostgreSQL ran them — the grant itself is exercised by
// internal/store/split_configuration_permissions_pg_test.go, which SKIPS without VIGOV_TEST_DSN.

const migration0032 = "0032_split_configuration_permissions.sql"

// The four keys are seeded under QUẢN TRỊ (the exact group string 0001 uses), at ranks 44-47, and the
// four QUẢN TRỊ keys that followed admin.lookup move to 48-51 — ONLY their rank is updated.
func TestMigration0032SeedsTheFourKeysAfterLookup(t *testing.T) {
	sql := maChay(t, migration0032)
	for _, want := range []string{
		"('admin.village', 'quản trị', 'cấu hình thôn/tổ dân phố', 44)",
		"('admin.message', 'quản trị', 'cấu hình lời hệ thống', 45)",
		"('admin.mail', 'quản trị', 'cấu hình máy chủ thư', 46)",
		"('admin.zalo', 'quản trị', 'cấu hình kênh zalo', 47)",
		"('admin.org', 'quản trị', 'quản lý sơ đồ tổ chức', 48)",
		"('admin.role', 'quản trị', 'phân quyền', 49)",
		"('admin.sla', 'quản trị', 'cấu hình thời hạn xử lý', 50)",
		"('admin.user', 'quản trị', 'quản lý người dùng', 51) on conflict (ma) do update set thu_tu = excluded.thu_tu;",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("0032 no longer contains %s", want)
		}
	}
}

// The grant follows the owner's rule — lookup ⇒ all four, org ⇒ residential_unit — live roles only,
// idempotently, and its audit entry is built from the grant's own RETURNING in the same statement
// (rule 6, invariant 3).
func TestMigration0032GrantsByHeldKeyWithAudit(t *testing.T) {
	sql := maChay(t, migration0032)
	for _, c := range []struct{ want, why string }{
		{"values ('admin.lookup', 'admin.village'), ('admin.lookup', 'admin.message'), " +
			"('admin.lookup', 'admin.mail'), ('admin.lookup', 'admin.zalo'), " +
			"('admin.org', 'admin.village')",
			"the owner's grant rule (15/10/2026): lookup ⇒ four keys, org ⇒ residential_unit only"},
		{"where vt.deleted_at is null", "live roles only — a soft-deleted role is the commune's decision"},
		{"on conflict (tenant_id, vai_tro_id, quyen_ma) do nothing returning tenant_id, vai_tro_id, quyen_ma",
			"idempotent: a key already held is neither re-granted nor re-audited"},
		{"insert into audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta) select tenant_id, 'system', 'system', '', 'luu_phan_quyen_vai_tro', vai_tro_id, now(),",
			"one system entry per role that gained a key, the Phân quyền verb, subject the role id"},
		{"from per_role;", "the entry is built from the INSERT's RETURNING"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0032 does not contain %q — %s", c.want, c.why)
		}
	}
	if strings.Count(sql, "insert into vai_tro_quyen") != 1 {
		t.Error("0032 must grant in exactly one statement")
	}
	for _, bad := range []string{"drop ", "delete from", "truncate", "alter table", "update vai_tro", "update quyen"} {
		if strings.Contains(sql, bad) {
			t.Errorf("0032 contains %q in its executable part", bad)
		}
	}
}
