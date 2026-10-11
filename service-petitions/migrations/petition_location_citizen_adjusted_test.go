package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0044 (owner decision 11/10/2026 — the citizen may move the map pin,
// and the petition records that it did). It reads the SQL this binary embeds; it does not prove
// PostgreSQL enforces any of it (tools/schema-smoke).
//
// THE MUTATIONS THAT MUST TURN THIS RED: the column made NOT NULL or given a default (every archival
// petition would be stamped "Zalo's location, untouched" — a fact nobody recorded); a BACKFILL (an
// UPDATE of phieu_phan_anh); the "true only with coordinates" CHECK dropped or reduced to one
// coordinate; the coordinate columns, the status list or the archival guard touched; anything dropped
// or deleted.
func TestMigration0044PetitionLocationCitizenAdjusted(t *testing.T) {
	const file = "0044_petition_location_citizen_adjusted.sql"
	sql := maChay(t, file)
	for _, c := range []struct{ want, why string }{
		{"alter table phieu_phan_anh add column if not exists location_citizen_adjusted boolean;", "nullable, no default"},
		{"check (location_citizen_adjusted is not true or (lat is not null and lng is not null))", "true only with both coordinates"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0044 lacks %q — %s", c.want, c.why)
		}
	}
	for _, c := range []struct{ banned, why string }{
		{"update phieu_phan_anh", "no backfill — how old coordinates were placed is not on disk"},
		{"alter column", "lat/lng (0004) are untouched"},
		{"trang_thai_hop_le", "the closed status list (ADR 0027) is untouched"},
		{"ho_so_luu_tru_bat_bien", "0004's archival guard is untouched"},
		{"not valid", "NOT VALID on a partitioned table is not portable"},
		{"default", "a default would invent a provenance on every archival petition"},
		{"drop table", "drops a table"},
		{"drop column", "drops a column"},
		{"drop constraint", "drops a constraint"},
		{"delete from", "deletes rows"},
		{"insert into", "writes rows"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0044 contains %q — %s", c.banned, c.why)
		}
	}
	if n := strings.Count(sql, "add column"); n != 1 {
		t.Errorf("0044 adds %d columns, want 1", n)
	}
	if n := strings.Count(sql, "add constraint"); n != 1 {
		t.Errorf("0044 adds %d constraints, want 1", n)
	}
}
