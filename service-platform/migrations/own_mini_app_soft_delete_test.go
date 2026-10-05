package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0021 (switched-off own Mini App rows → soft-deleted, ADR 0070
// §Sửa đổi 05/10/2026 #2).
//
// It reads the SQL this binary embeds and asserts the filter, the write and the trail are WRITTEN.
// It does not prove PostgreSQL runs them as intended: that needs a database, and this service's pg
// suites SKIP without VIGOV_TEST_DSN (store/own_mini_app_soft_delete_pg_test.go is the enforcing half).

const file0021 = "0021_own_mini_app_switched_off_to_soft_delete.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: a filter clause dropped (che_do — the shared app would be
// removed; dang_hoat_dong — a RUNNING app would be removed; deleted_at IS NULL — a re-run would
// overwrite who removed it and write a second trail entry; the commune-active or succession guard —
// a merged commune's registry would change); the actor stops being the system principal; the action
// or the delta keys drift from store/operator_writes.go; the reason (the reversal's key) changes; a
// DELETE, DROP, TRUNCATE or trigger switch-off appears.
func TestMigration0021FilterWriteTrail(t *testing.T) {
	sql := executableSQL(t, file0021)
	for _, clause := range []string{
		// target
		"where m.che_do = 'rieng' and m.dang_hoat_dong = false and m.deleted_at is null and t.dang_hoat_dong " +
			"and not exists (select 1 from tenant_succession s where s.tu_id = m.tenant_id)",
		// the UPDATE re-checks the whole filter on the row it locks
		"update mini_app m set deleted_at = now(), deleted_by = 'system', delete_reason = p.reason",
		"where m.app_id = x.app_id and m.tenant_id = x.tenant_id and m.che_do = 'rieng' " +
			"and m.dang_hoat_dong = false and m.deleted_at is null returning",
		// reason, written once
		"'chuyển sang xoá mềm theo quyết định chủ dự án 05/10/2026 (adr 0070 §sửa đổi)'::text as reason",
		// trail: same statement, row's commune, system principal, Go shape
		"insert into audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta) " +
			"select c.tenant_id, 'system', 'system', '', 'tat_mini_app', 'miniapp ' || c.app_id, now(),",
		"'truoc', jsonb_build_object('dang_hoat_dong', false, 'da_xoa_mem', false)",
		"'sau', jsonb_build_object('dang_hoat_dong', false, 'da_xoa_mem', true)",
		"'ly_do', c.delete_reason",
		"'xa', c.commune_name",
		"from changed c;",
	} {
		if !strings.Contains(sql, clause) {
			t.Errorf("%s lacks %q", file0021, clause)
		}
	}
	// Rule 7: nothing deleted, dropped, retyped; no trigger switched off; the shared app never written.
	for _, forbidden := range []string{"delete from", "drop ", "truncate", "alter ", "disable trigger",
		"session_replication_role", "che_do = 'chinh'", "dang_hoat_dong = true", "set dang_hoat_dong",
		"cap_nhat_boi ="} {
		if strings.Contains(sql, forbidden) {
			t.Errorf("%s executes %q", file0021, forbidden)
		}
	}
	// Exactly one UPDATE, and it is the one above.
	if n := strings.Count(sql, "update "); n != 1 {
		t.Errorf("%s runs %d UPDATE statements, want 1", file0021, n)
	}
}
