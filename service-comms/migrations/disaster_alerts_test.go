package migrations

import (
	"io/fs"
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0026 (`disaster_alerts`). It asserts the key, CHECKs, index and guard
// are WRITTEN; it does not prove PostgreSQL enforces them — the pg suites SKIP without VIGOV_TEST_DSN.

const file0026 = "0026_disaster_alerts.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: a key or index without tenant_id first; the table
// partitioned (ADR 0096); a kind dropped or spelled in English (ADR 0011); a JSONB list allowed to be a non-array or
// unbounded; the ended pair no longer both-or-neither; the guard no longer refusing DELETE, edits of a
// deleted row, or edits of an ended alert; the soft-delete trio no longer all-or-none; a row seeded.
func TestMigration0026DisasterAlerts(t *testing.T) {
	sql := executableSQL(t, file0026)
	requirePlainTable(t, file0026, sql, "disaster_alerts")
	for _, c := range []struct{ want, why string }{
		{"create table if not exists disaster_alerts ( tenant_id text not null, id text not null,", "tenant_id and ULID id, both required"},
		{"kind text not null, title text not null, level text, headline text, detail text not null,", "the agreed text columns; level and headline nullable"},
		{"effective_from timestamptz not null, effective_to timestamptz, ended_at timestamptz, ended_by text,", "the time window and the early end"},
		{"danger_zone jsonb not null default '[]'::jsonb,", "danger zone, never NULL"},
		{"evacuation_points jsonb not null default '[]'::jsonb,", "evacuation points, never NULL"},
		{"emergency_contacts jsonb not null default '[]'::jsonb,", "emergency contacts, never NULL"},
		{"created_by text not null, updated_at timestamptz not null default now(), updated_by text not null,", "signed by business codes"},
		{"deleted_at timestamptz, deleted_by text, delete_reason text,", "soft-delete trio (rule 7)"},
		{"primary key (tenant_id, id),", "key composite with tenant_id (rule 1 inv 6)"},
		{"check (kind in ('bao', 'lu', 'sat-lo', 'mua-lon', 'dich-benh', 'chay'))", "the six kinds, Vietnamese values (ADR 0011)"},
		{"check ( effective_to is null or effective_to > effective_from)", "window not inverted"},
		{"check ( (ended_at is null) = (ended_by is null) and (ended_by is null or btrim(ended_by) <> ''))", "ended both or neither, signed"},
		{"jsonb_typeof(danger_zone) = 'array' and jsonb_array_length(danger_zone) <= 200", "danger zone is a bounded array"},
		{"jsonb_typeof(evacuation_points) = 'array' and jsonb_array_length(evacuation_points) <= 50", "evacuation points a bounded array"},
		{"jsonb_typeof(emergency_contacts) = 'array' and jsonb_array_length(emergency_contacts) <= 50", "contacts a bounded array"},
		{"check (btrim(created_by) <> '' and btrim(updated_by) <> '')", "signed"},
		{"check ((deleted_at is null) = (deleted_by is null) and (deleted_at is null) = (delete_reason is null))", "soft-delete trio all or none (rule 7)"},
		{"on disaster_alerts (tenant_id, effective_from) where deleted_at is null", "public read index leads with tenant_id, live rows only"},
		{"before update or delete on disaster_alerts for each row execute function disaster_alerts_guard()", "guard on update and delete"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0026 lacks %q — %s", c.want, c.why)
		}
	}

	guard := functionBody(t, sql, "disaster_alerts_guard")
	for _, want := range []string{
		"if tg_op = 'delete' then raise exception",
		"if old.deleted_at is not null then raise exception",
		"or new.created_by is distinct from old.created_by then",
		"if old.ended_at is not null and (to_jsonb(new) - array['deleted_at', 'deleted_by', 'delete_reason', 'updated_at', 'updated_by']) is distinct from (to_jsonb(old) - array['deleted_at', 'deleted_by', 'delete_reason', 'updated_at', 'updated_by']) then",
	} {
		if !strings.Contains(guard, want) {
			t.Errorf("disaster_alerts_guard lacks %q", want)
		}
	}

	for _, c := range []struct{ banned, why string }{
		{"not valid", "NOT VALID on a partitioned table"},
		{"references", "nothing links across to another table or service"},
		{"latitude", "no geographic coordinates — relative polygon only, no resident location (ADR 0047 G2)"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0026 contains %q — %s", c.banned, c.why)
		}
	}
	noDestruction(t, file0026, sql)
}

// The ownership marks tools/kb reads must precede the CREATE TABLE and be the only ones in the file.
func TestMigration0026OwnershipMarks(t *testing.T) {
	ownershipMarks(t, file0026, "DisasterAlert", "disaster_alerts")
}

// ownershipMarks asserts exactly one `@entity: <entity>` / `@scope: tenant` pair, above the CREATE TABLE.
func ownershipMarks(t *testing.T, file, entity, table string) {
	t.Helper()
	b, err := fs.ReadFile(FS, file)
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	src := string(b)
	mark := strings.Index(src, "-- @entity: "+entity+"\n-- @scope:  tenant\n")
	create := strings.Index(src, "CREATE TABLE IF NOT EXISTS "+table+" (")
	if mark < 0 || create < 0 || mark > create {
		t.Errorf("%s: @entity: %s / @scope: tenant must precede CREATE TABLE %s", file, entity, table)
	}
	if n := strings.Count(src, "-- @entity:"); n != 1 {
		t.Errorf("%s: %d @entity marks, want exactly 1", file, n)
	}
}
