package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0017 (`map_frame` radius ceiling and the "Về mặc định" column,
// ADR 0072 §"Sửa đổi 04/10/2026 (lần 2)", K2/K4). It asserts the DDL is WRITTEN; it does not prove
// PostgreSQL enforces it — the pg suites SKIP without VIGOV_TEST_DSN. 0016's own text stays pinned by
// map_frame_test.go: that file never changes.

const file0017 = "0017_map_frame_ceiling_and_reset.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: the ceiling raised past 50 km or the lower bound admitting 0
// (K2, stop condition 1 of lần 2); the old CHECK left in place (it would still refuse 30–50 km); the
// old CHECK dropped BEFORE the new one exists; a nullable or false-by-default `is_enabled` (the first
// changes the meaning of every older row, the second switches every saved frame off at deploy); a
// cleared_at / notice_version copy of what the audit trail records; a backfill, a delete, a drop of
// anything but the replaced CHECK; NOT VALID on the partitioned table; the mainland box touched.
func TestMigration0017MapFrameCeilingAndReset(t *testing.T) {
	sql := executableSQL(t, file0017)
	for _, c := range []struct{ want, why string }{
		{"alter table map_frame add column if not exists is_enabled boolean not null default true;", "reset is a column; existing frames stay applied"},
		{"alter table map_frame add constraint map_frame_radius_ceiling check (radius_km > 0 and radius_km <= 50);", "K2 hard ceiling"},
		{"conname = 'map_frame_radius_ceiling'", "constraint added behind a pg_constraint lookup (retry-safe)"},
		{"alter table map_frame drop constraint if exists map_frame_radius_range;", "0016's 1–30 CHECK replaced"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0017 lacks %q — %s", c.want, c.why)
		}
	}

	add := strings.Index(sql, "add constraint map_frame_radius_ceiling")
	drop := strings.Index(sql, "drop constraint if exists map_frame_radius_range")
	if add < 0 || drop < 0 || add > drop {
		t.Error("0017 must add map_frame_radius_ceiling BEFORE dropping map_frame_radius_range — the radius is never unbounded")
	}

	for _, c := range []struct{ banned, why string }{
		{"drop table", "drops a table"},
		{"drop column", "drops a column"},
		{"delete from", "deletes rows"},
		{"update map_frame", "backfills — every existing row stays as saved"},
		{"insert into", "nothing is seeded — the default frame is service-platform's (K3)"},
		{"not valid", "NOT VALID on a partitioned table; a widening CHECK cannot fail validation anyway"},
		{"truncate", "empties the table"},
		{"cleared_at", "who/when of a reset is updated_* plus the audit entry"},
		{"notice_version", "the acknowledgement lives in the audit entry (K4)"},
		{"deleted_at", "no soft delete on a one-row-per-commune setting (0008)"},
		{"map_frame_center_in_mainland_box", "the mainland box is unchanged (K2) — widening it is a stop condition"},
		{"radius_km >= 0", "a zero radius admitted"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0017 contains %q — %s", c.banned, c.why)
		}
	}
	// Exactly one upper bound on the radius, and it is 50 km (K2).
	if n, want := strings.Count(sql, "radius_km <="), strings.Count(sql, "radius_km <= 50)"); n != 1 || want != 1 {
		t.Errorf("0017 has %d radius upper bounds, %d of them `<= 50` — K2 allows exactly one, 50 km", n, want)
	}
}

// CREATE OR REPLACE IS A FULL REWRITE: a clause 0017 leaves out of map_frame_guard is a rule of 0016
// silently repealed. Mutation: drop the DELETE refusal or any frozen identity column from 0017.
func TestMigration0017KeepsEvery0016GuardRule(t *testing.T) {
	cur := functionBody(t, executableSQL(t, file0017), "map_frame_guard")
	for _, c := range []struct{ want, why string }{
		{"if tg_op = 'delete' then raise exception", "delete still refused"},
		{"new.tenant_id is distinct from old.tenant_id", "commune frozen"},
		{"or new.created_at is distinct from old.created_at", "creation time frozen"},
		{"or new.created_by is distinct from old.created_by then", "creator frozen"},
		{"return new;", "every other column, is_enabled included, editable"},
	} {
		if !strings.Contains(cur, c.want) {
			t.Errorf("0017's map_frame_guard lacks %q — %s", c.want, c.why)
		}
	}

	// The identity block is 0016's verbatim (only the DELETE hint text changed).
	old := functionBody(t, executableSQL(t, file0016), "map_frame_guard")
	const from = "if new.tenant_id"
	i := strings.Index(old, from)
	if i < 0 {
		t.Fatal("0016's identity block not found")
	}
	if !strings.Contains(cur, old[i:]) {
		t.Error("0017's map_frame_guard no longer carries 0016's identity block and RETURN verbatim")
	}
}
