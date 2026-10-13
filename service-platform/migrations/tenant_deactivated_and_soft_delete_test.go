package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0026 (tenant.deactivated_at + soft-delete triple, owner decisions
// 13/10/2026). It proves the DDL, the backfill and the trail are WRITTEN, not that PostgreSQL enforces
// them: this service's pg suites SKIP without VIGOV_TEST_DSN.

const file0026 = "0026_tenant_deactivated_and_soft_delete.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: a CHECK removed or loosened (deleted_by not a VH- code, a
// blank reason, an active commune removable, deactivated_at able to disagree with dang_hoat_dong); the
// backfill reading the wrong action, ignoring a later reactivation, losing its `deactivated_at IS NULL`
// filter (a re-run would overwrite values), or writing no trail; the iff CHECK moved above the backfill
// (every inactive row would break it); tenant_domain touched; anything dropped, deleted or bumped.
func TestMigration0026TenantDeactivatedAndSoftDelete(t *testing.T) {
	sql := executableSQL(t, file0026)
	for _, c := range []struct{ want, why string }{
		{"alter table tenant add column if not exists deactivated_at timestamptz null;", "nullable, no default"},
		{"alter table tenant add column if not exists deleted_at timestamptz null;", "soft delete (rule 7 inv 1)"},
		{"alter table tenant add column if not exists deleted_by text null;", "soft delete who"},
		{"alter table tenant add column if not exists delete_reason text null;", "soft delete why"},
		{"check ((deleted_at is null and deleted_by is null and delete_reason is null) " +
			"or (deleted_at is not null and deleted_by ~ '^vh-[0-9]{5,}$' and btrim(delete_reason) <> ''))", "all three or none, VH- code, reason"},
		{"check (deleted_at is null or not dang_hoat_dong)", "only an inactive commune is removed"},
		{"check ((deactivated_at is null) = dang_hoat_dong)", "deactivated_at iff inactive"},
		{"and d.action = 'ngung_hoat_dong_xa'", "the action SetActivation writes (operator_writes.go:73)"},
		{"and r.action = 'mo_lai_hoat_dong_xa' and r.at > d.at", "a later reactivation voids an earlier deactivation"},
		{"set deactivated_at = coalesce(s.entry_at, now())", "trail value, else migration time"},
		{"where t.id = s.id and not t.dang_hoat_dong and t.deactivated_at is null", "re-run safe, inactive only"},
		{"insert into audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta) " +
			"select c.id, 'system', 'system', '', 'deactivated_at.recorded', c.ten, now(),", "one system entry per commune, same statement"},
		{"from changed c;", "the trail reads the rows the UPDATE actually wrote"},
		{"where c.relkind = 'p' and n.nspname = current_schema()", "partition backstop"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("%s lacks %q — %s", file0026, c.want, c.why)
		}
	}
	for _, c := range []struct{ banned, why string }{
		{"drop ", "drops something"},
		{"delete from", "deletes rows"},
		{"truncate", "truncates"},
		{"tenant_domain", "domains stay reserved by their primary key; not this file's business"},
		{"cap_nhat_luc", "the last human change keeps its timestamp"},
		{"set dang_hoat_dong", "a commune's status is not this file's business"},
		{"set ten", "a commune's name is not this file's business"},
		{"default", "no column default: Go writes these values"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("%s contains %q — %s", file0026, c.banned, c.why)
		}
	}
	if n := strings.Count(sql, "update "); n != 1 {
		t.Errorf("%s runs %d UPDATE statements, want 1", file0026, n)
	}
	backfill := strings.Index(sql, "set deactivated_at =")
	iff := strings.Index(sql, "add constraint tenant_deactivated_iff_inactive")
	if backfill < 0 || iff < 0 || iff < backfill {
		t.Errorf("%s: tenant_deactivated_iff_inactive must be added AFTER the backfill", file0026)
	}
}
