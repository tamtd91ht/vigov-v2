package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0016 (`map_frame`). It asserts the key, CHECKs and guard are
// WRITTEN; it does not prove PostgreSQL enforces them — the pg suites SKIP without VIGOV_TEST_DSN.

const file0016 = "0016_map_frame.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: a key other than tenant_id alone; a missing partition loop;
// the mainland box widened east toward Hoàng Sa / Trường Sa or dropped; the radius bound raised or
// dropped (both ADR 0072 H3 stop condition 2); the guard no longer refusing DELETE or freezing who
// created the row; a seed row (a default centre is one commune's value — rule 1, invariant 10); soft
// delete columns that no read path would know to exclude; a hard delete or drop.
func TestMigration0016MapFrame(t *testing.T) {
	sql := executableSQL(t, file0016)
	for _, c := range []struct{ want, why string }{
		{"create table if not exists map_frame ( tenant_id text not null, center_lat numeric(10,6) not null, center_lng numeric(10,6) not null, radius_km numeric(4,1) not null,", "centre and radius required, map_asset's coordinate type"},
		{"created_at timestamptz not null default now(), created_by text not null, updated_at timestamptz not null default now(), updated_by text not null,", "signed by business codes"},
		{"primary key (tenant_id),", "one row per commune"},
		{"partition of map_frame ' 'for values with (modulus 32, remainder %s)", "MODULUS 32 partition loop (ADR 0010)"},
		{"check ( center_lat between 8.4 and 23.4 and center_lng between 102.1 and 109.5)", "centre inside the mainland box (H3), east edge short of Hoàng Sa / Trường Sa"},
		{"check (radius_km between 1 and 30)", "radius proposal 1–30 km (H3)"},
		{"check (btrim(created_by) <> '' and btrim(updated_by) <> '')", "signed"},
		{"before update or delete on map_frame for each row execute function map_frame_guard()", "guard on update and delete"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0016 lacks %q — %s", c.want, c.why)
		}
	}

	guard := functionBody(t, sql, "map_frame_guard")
	for _, c := range []struct{ want, why string }{
		{"if tg_op = 'delete' then raise exception", "delete refused"},
		{"new.tenant_id is distinct from old.tenant_id", "commune frozen"},
		{"or new.created_at is distinct from old.created_at", "creation time frozen"},
		{"or new.created_by is distinct from old.created_by then", "creator frozen"},
	} {
		if !strings.Contains(guard, c.want) {
			t.Errorf("map_frame_guard lacks %q — %s", c.want, c.why)
		}
	}

	for _, c := range []struct{ banned, why string }{
		{"drop table", "drops a table"},
		{"drop column", "drops a column"},
		{"delete from", "deletes rows"},
		{"not valid", "NOT VALID on a partitioned table"},
		{"insert into", "nothing is seeded — no default centre"},
		{"deleted_at", "no soft delete on a one-row-per-commune setting (0008)"},
		{"geometry", "no PostGIS"},
		{"create extension", "no extension is added by a migration"},
		{"create unique index", "the only key is the primary key"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0016 contains %q — %s", c.banned, c.why)
		}
	}
}
