package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0043 (ADR 0047 G4 — the two status-entry times). It reads the SQL
// this binary embeds; it does not prove PostgreSQL enforces any of it (tools/schema-smoke).
//
// THE MUTATIONS THAT MUST TURN THIS RED: a column made NOT NULL or given a default (every archival
// petition would be rewritten with an invented time); a BACKFILL (an UPDATE of phieu_phan_anh, or a read
// of nhat_ky_phan_anh, which G4 excludes); the "after intake" floor dropped; the status list or the
// archival guard touched; anything dropped or deleted.
func TestMigration0043PetitionStatusEnteredTimes(t *testing.T) {
	const file = "0043_petition_status_entered_times.sql"
	sql := maChay(t, file)
	for _, c := range []struct{ want, why string }{
		{"alter table phieu_phan_anh add column if not exists forwarded_for_processing_at timestamptz;", "nullable, no default"},
		{"alter table phieu_phan_anh add column if not exists awaiting_confirmation_at timestamptz;", "nullable, no default"},
		{"check (forwarded_for_processing_at is null or forwarded_for_processing_at >= vao_so_luc)", "entered after intake"},
		{"check (awaiting_confirmation_at is null or awaiting_confirmation_at >= vao_so_luc)", "entered after intake"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0043 lacks %q — %s", c.want, c.why)
		}
	}
	for _, c := range []struct{ banned, why string }{
		{"update phieu_phan_anh", "no backfill — old times cannot be reconstructed reliably"},
		{"nhat_ky_phan_anh", "G4: the internal log is not read"},
		{"trang_thai_hop_le", "the closed status list (ADR 0027) is untouched"},
		{"ho_so_luu_tru_bat_bien", "0004's archival guard is untouched"},
		{"not valid", "NOT VALID on a partitioned table is not portable"},
		{"default", "a default would invent a time on every archival petition"},
		{"drop table", "drops a table"},
		{"drop column", "drops a column"},
		{"drop constraint", "drops a constraint"},
		{"delete from", "deletes rows"},
		{"insert into", "writes rows"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0043 contains %q — %s", c.banned, c.why)
		}
	}
	if n := strings.Count(sql, "add constraint"); n != 2 {
		t.Errorf("0043 adds %d constraints, want 2", n)
	}
}
