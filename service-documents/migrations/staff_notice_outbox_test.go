package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0009 (ADR 0086 A1, the staff-notice outbox). It reads the SQL this
// binary embeds; it does not prove PostgreSQL enforces any of it (the pg suites skip without
// VIGOV_TEST_DSN).
//
// THE MUTATIONS THAT MUST TURN THIS RED: a key or primary key without tenant_id (rule 1, invariant 6 —
// the second commune's first notice would collide); the table left unpartitioned or without partitions
// (every routing's transaction would roll back on the INSERT); the partial index growing with every
// notice ever sent; an archival trigger attached (delivered rows could never be pruned); a destructive
// or backfilling statement in a file that only adds.
func TestMigration0009StaffNoticeOutbox(t *testing.T) {
	sql := executableSQL(t, "0009_staff_notice_outbox.sql")
	for _, c := range []struct{ want, why string }{
		{"create table if not exists staff_notice_outbox (", "the table itself"},
		{"primary key (tenant_id, id)", "the primary key is composite with the commune (rule 1, invariant 6)"},
		{"unique (tenant_id, idempotency_key)", "one row per act per commune — never a single-column key"},
		{"payload jsonb not null", "the protojson of comms.v1.StaffNotification"},
		{"delivered_at timestamptz,", "NULL until delivered: the queue state"},
		{"attempts integer not null default 0", "failed attempts are counted"},
		{"failed_class text,", "a refused row is set aside, not retried for ever"},
		{") partition by hash (tenant_id);", "hash-partitioned by commune like every table here"},
		{"partition of staff_notice_outbox", "the 32 partitions exist — a partitioned table without them rejects every insert"},
		{"for values with (modulus 32, remainder %s)", "the modulus every partitioned table here uses"},
		{"where delivered_at is null and failed_class is null", "the relay's index is partial and shrinks as rows leave"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0009 lacks %q — %s", c.want, c.why)
		}
	}
	for _, banned := range []string{"drop table", "drop column", "delete from", "insert into", "update ",
		"truncate", "create trigger"} {
		if strings.Contains(sql, banned) {
			t.Errorf("0009 contains %q — this file only adds an infrastructure table", banned)
		}
	}
}
