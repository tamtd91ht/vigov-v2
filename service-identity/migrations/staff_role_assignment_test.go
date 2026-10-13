package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0031 (owner decision 13/10/2026: a member of staff holds several
// roles). The weaker half: it proves the statements are WRITTEN this way, not that PostgreSQL ran
// them — the backfill itself is exercised by store/staff_role_assignment_pg_test.go, which needs
// VIGOV_TEST_DSN.

const migration0031 = "0031_staff_role_assignment.sql"

// THE KEYS THAT CARRY RULE 1 AND RULE 7.
//
// MUTATIONS THAT MUST TURN THIS RED: a foreign key on the id alone (another commune's role accepted),
// a unique key without live_key (a removed role blocks its own re-grant), a missing soft-delete CHECK.
func TestMigration0031KeysAreCompositeWithTheCommune(t *testing.T) {
	sql := maChay(t, migration0031)
	for _, c := range []struct{ want, why string }{
		{"primary key (tenant_id, staff_id, role_id, created_at)", "composite with tenant_id; a re-grant is a new row"},
		{"unique (tenant_id, staff_id, role_id, live_key)", "one LIVE row per person and role"},
		{"foreign key (tenant_id, staff_id) references nguoi_dung (tenant_id, id)", "the person, in THIS commune"},
		{"foreign key (tenant_id, role_id) references vai_tro (tenant_id, id)", "the role, in THIS commune"},
		{"live_key boolean generated always as (case when deleted_at is null then true end) stored", "live_key is NULL once removed"},
		{"partition by hash (tenant_id)", "partitioned like every business table (ADR 0010)"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0031 does not contain %q — %s", c.want, c.why)
		}
	}
}

// THE BACKFILL copies the existing column, keeps it, and writes its system entry in the SAME statement.
//
// MUTATIONS THAT MUST TURN THIS RED: drop or null nguoi_dung.vai_tro_id (lossy, rule 7); backfill
// without its audit entry (rule 6, invariant 6); a second, separate INSERT for the entry.
func TestMigration0031BackfillsWithAuditAndKeepsTheColumn(t *testing.T) {
	sql := maChay(t, migration0031)
	for _, c := range []struct{ want, why string }{
		{"select nd.tenant_id, nd.id, nd.vai_tro_id, now(), 'system' from nguoi_dung nd where nd.vai_tro_id is not null on conflict do nothing returning tenant_id",
			"one row per existing role, idempotent"},
		{"insert into audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta) select tenant_id, 'system', 'system', '', 'gan_vai_tro_ban_dau'",
			"one system entry per commune, from the INSERT's RETURNING"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0031 does not contain %q — %s", c.want, c.why)
		}
	}
	for _, bad := range []string{"drop ", "delete from", "truncate", "alter table nguoi_dung", "update nguoi_dung"} {
		if strings.Contains(sql, bad) {
			t.Errorf("0031 contains %q — the role column is kept as the primary role, never rewritten", bad)
		}
	}
}
