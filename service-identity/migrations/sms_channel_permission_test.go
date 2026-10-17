package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0035 (owner decision 17/10/2026, ADR 0099 #3). The weaker half, as
// split_configuration_permissions_test.go says: it proves the statements are WRITTEN this way, not that
// PostgreSQL ran them — that needs VIGOV_TEST_DSN.

const migration0035 = "0035_sms_channel_permission.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: the key seeded under another group or label (the matrix would
// file it in a second band); the grant keyed on anything but admin.zalo (ADR 0099 #3 "đúng các vai trò
// đang có admin.zalo"); soft-deleted roles granted; the grant made non-idempotent or split from its audit
// entry; anything removed, rewritten or re-ranked.
func TestMigration0035SeedsAdminSMSAndGrantsItToZaloHolders(t *testing.T) {
	sql := maChay(t, migration0035)
	for _, c := range []struct{ want, why string }{
		{"insert into quyen (ma, nhom, nhan, thu_tu) values ('admin.sms', 'quản trị', 'cấu hình kênh sms', 52) on conflict (ma) do update set",
			"one key, QUẢN TRỊ band, the owner's label, a free rank"},
		{"values ('admin.zalo', 'admin.sms')", "granted to exactly the holders of admin.zalo"},
		{"where vt.deleted_at is null", "live roles only — a soft-deleted role is the commune's decision"},
		{"on conflict (tenant_id, vai_tro_id, quyen_ma) do nothing returning tenant_id, vai_tro_id, quyen_ma",
			"idempotent: a key already held is neither re-granted nor re-audited"},
		{"insert into audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta) select tenant_id, 'system', 'system', '', 'luu_phan_quyen_vai_tro', vai_tro_id, now(),",
			"one system entry per role that gained the key, the Phân quyền verb"},
		{"from per_role;", "the entry is built from the INSERT's RETURNING"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0035 does not contain %q — %s", c.want, c.why)
		}
	}
	if n := strings.Count(sql, "insert into quyen"); n != 1 {
		t.Errorf("0035 has %d catalogue inserts, want 1 — no other row's rank moves", n)
	}
	if n := strings.Count(sql, "insert into vai_tro_quyen"); n != 1 {
		t.Errorf("0035 grants in %d statements, want 1", n)
	}
	for _, bad := range []string{"drop ", "delete from", "truncate", "alter table", "update vai_tro", "update quyen", "admin.lookup"} {
		if strings.Contains(sql, bad) {
			t.Errorf("0035 contains %q in its executable part", bad)
		}
	}
}
