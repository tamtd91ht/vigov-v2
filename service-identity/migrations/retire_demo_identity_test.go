package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0024 (owner decision 05/10/2026, ADR 0066 §Sửa đổi). The weaker
// half: it proves the statements are WRITTEN this way. What PostgreSQL does with them — the guard
// trigger admitting the retirement, idempotency, one entry per row — is
// internal/store/mini_app_secret_pg_test.go TestMigration0024RetiresDemoOnlyRowsPG.

const file0024 = "0024_retire_demo_identity_settings.sql"

func TestMigration0024RetiresOnlyDemoOnlyLiveRows(t *testing.T) {
	sql := maChay(t, file0024)
	for _, c := range []struct{ want, why string }{
		{"where deleted_at is null and app_secret_sealed is null and demo_identity_enabled",
			"only LIVE rows with NO secret and demo on — a row with a secret keeps an app on the air"},
		{"set deleted_at = now(), deleted_by = 'system', delete_reason = 'gỡ danh tính demo theo quyết định 05/10/2026', updated_at = now()",
			"retire = the soft-delete trio + updated_at, the one mutation mini_app_secret_guard allows"},
		{"and m.deleted_at is null returning m.tenant_id, m.id, m.app_id",
			"idempotent: an already-retired row is never matched again"},
		{"insert into audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta) select tenant_id, 'system', 'system', '', 'ngung_cau_hinh_app_rieng', app_id, now(),",
			"one system entry per retired row, the retire verb, subject the App ID (rule 6)"},
		{"from changed;",
			"the entry is written from the UPDATE's own RETURNING — same statement, same transaction"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0024 does not contain %q — %s", c.want, c.why)
		}
	}
	for _, bad := range []string{"drop ", "delete from", "truncate", "alter table", "set demo_identity_enabled", "set app_secret_sealed"} {
		if strings.Contains(sql, bad) {
			t.Errorf("0024 contains %q in its executable part — rule 7: retire, never drop or rewrite a version", bad)
		}
	}
}
