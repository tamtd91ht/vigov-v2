package migrations

import (
	"io/fs"
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0020 (commune_map_frame_default, ADR 0072 amendment 2 K1–K2).
//
// It reads the SQL this binary embeds and asserts the key, the CHECKs and the guard are WRITTEN. It does
// not prove PostgreSQL enforces them: that needs a database, and this service's pg suites SKIP without
// VIGOV_TEST_DSN (store/map_frame_default_pg_test.go is the enforcing half).

const file0020 = "0020_commune_map_frame_default.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: the key without tenant_id or the FK to the registry dropped;
// the mainland box widened or removed; the 50 km ceiling raised or the "> 0" floor dropped; the
// operator-code shape loosened; the guard stopping only DELETE, or not freezing the identity columns;
// a seed row.
func TestMigration0020KeyConstraintsGuard(t *testing.T) {
	sql := executableSQL(t, file0020)
	for _, clause := range []string{
		"create table if not exists commune_map_frame_default (",
		"tenant_id text not null references tenant (id)",
		"center_lat numeric(10,6) not null",
		"center_lng numeric(10,6) not null",
		"radius_km numeric(4,1) not null",
		"primary key (tenant_id)",
		"constraint commune_map_frame_default_center_in_mainland_box check ( center_lat between 8.4 and 23.4 and center_lng between 102.1 and 109.5)",
		"constraint commune_map_frame_default_radius_ceiling check (radius_km > 0 and radius_km <= 50)",
		"created_by ~ '^vh-[0-9]{5,}$' and updated_by ~ '^vh-[0-9]{5,}$'",
		"if tg_op = 'delete' then raise exception",
		"new.tenant_id is distinct from old.tenant_id or new.created_at is distinct from old.created_at or new.created_by is distinct from old.created_by",
		"before update or delete on commune_map_frame_default for each row execute function commune_map_frame_default_guard()",
	} {
		if !strings.Contains(sql, clause) {
			t.Errorf("%s lacks %q", file0020, clause)
		}
	}
	// Rule 7: nothing is dropped, retyped or deleted by the statements (the REVERSAL prose may name
	// them), and no default centre is seeded (rule 1, invariant 10).
	for _, forbidden := range []string{"drop column", "drop table", "delete from", "alter column", "truncate",
		"insert into"} {
		if strings.Contains(sql, forbidden) {
			t.Errorf("%s executes %q", file0020, forbidden)
		}
	}
}

// The ownership marks tools/kb reads must sit directly above the CREATE TABLE.
func TestMigration0020OwnershipMarks(t *testing.T) {
	b, err := fs.ReadFile(FS, file0020)
	if err != nil {
		t.Fatalf("read %s: %v", file0020, err)
	}
	src := string(b)
	mark := strings.Index(src, "-- @entity: CommuneMapFrameDefault\n-- @scope:  tenant\n")
	table := strings.Index(src, "CREATE TABLE IF NOT EXISTS commune_map_frame_default (")
	if mark < 0 || table < 0 || mark > table {
		t.Errorf("@entity: CommuneMapFrameDefault / @scope: tenant must precede CREATE TABLE commune_map_frame_default")
	}
	if strings.Count(src, "-- @entity:") != 1 {
		t.Errorf("exactly one @entity mark expected in %s", file0020)
	}
}
