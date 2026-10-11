package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0042 (owner rule 11/10/2026, bug sheet row 60 — the staff-recorded
// rating). It reads the SQL this binary embeds; it does not prove PostgreSQL enforces any of it
// (tools/schema-smoke).
//
// THE MUTATIONS THAT MUST TURN THIS RED: a key without tenant_id (rule 1, invariant 6); a second staff
// record per petition; `source` able to hold anything but 'staff_recorded' (a row could then pass for the
// citizen's own verdict); the table left unpartitioned; the append-only trigger missing; and — the one that
// matters most — the file touching `diem_hai_long`, the citizen's column every satisfaction figure reads.
func TestMigration0042(t *testing.T) {
	const file = "0042_petition_staff_recorded_rating.sql"
	sql := maChay(t, file)
	for _, c := range []struct{ want, why string }{
		{"create table if not exists petition_staff_recorded_rating (", "the table itself"},
		{"primary key (tenant_id, id)", "the primary key is composite with the commune"},
		{"unique (tenant_id, petition_id)", "one staff record per petition, per commune"},
		{"check (source = 'staff_recorded')", "a row here can never claim to be the citizen's own verdict"},
		{"check (stars between 1 and 5)", "the range of the citizen's rating"},
		{"check (channel in ('dien-thoai', 'tai-quay'))", "the two channels the prototype names"},
		{") partition by hash (tenant_id);", "hash-partitioned by commune"},
		{"for values with (modulus 32, remainder %s)", "the 32 partitions exist"},
		{"before update or delete on petition_staff_recorded_rating", "append-only"},
		{"before truncate on %s", "truncate refused per partition"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0042 lacks %q — %s", c.want, c.why)
		}
	}
	for _, banned := range []string{"diem_hai_long", "alter table phieu_phan_anh", "drop table", "drop column",
		"delete from", "insert into", "update phieu_phan_anh", "truncate petition"} {
		if strings.Contains(sql, banned) {
			t.Errorf("0042 contains %q — this file only adds a separate table and never touches the citizen's rating", banned)
		}
	}
}
