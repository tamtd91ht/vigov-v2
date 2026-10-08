package migrations

import (
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0027 (citizen-letter deadline rules, ADR 0084 #3 / ADR 0085 B).
// Same weaker-half caveat as danh_ba_mini_app_test.go: it proves the DDL is WRITTEN, not that
// PostgreSQL enforces it — a deleted key or a widened CHECK must turn red somewhere that always runs.

const file0027 = "0027_citizen_letter_deadline_rule.sql"

// codesOf returns the quoted codes of `CONSTRAINT <name> CHECK (<col> IN (...))`, sorted, and FAILS
// when the constraint is not found — a parser gone blind must not read as an empty, equal list.
func codesOf(t *testing.T, sql, name, col string) string {
	t.Helper()
	m := regexp.MustCompile(`constraint ` + regexp.QuoteMeta(name) + ` check \(` +
		regexp.QuoteMeta(col) + ` in \(([^)]*)\)\)`).FindStringSubmatch(sql)
	if m == nil {
		t.Fatalf("constraint %s CHECK (%s IN (...)) not found", name, col)
	}
	var codes []string
	for _, x := range regexp.MustCompile(`'([^']*)'`).FindAllStringSubmatch(m[1], -1) {
		codes = append(codes, x[1])
	}
	sort.Strings(codes)
	return strings.Join(codes, ",")
}

// The closed sets. letter_type must equal service-documents 0006 `citizen_letter_type_valid` — the
// literal below is that list; a type added there and not here is a type no commune can configure.
//
// THE MUTATION THAT MUST TURN THIS RED: add, drop or misspell one code in any of the three lists.
func TestMigration0027ClosedLists(t *testing.T) {
	sql := maChay(t, file0027)
	for _, c := range []struct{ name, col, want string }{
		{"citizen_letter_deadline_rule_letter_type_valid", "letter_type", "de-nghi,khieu-nai,kien-nghi-phan-anh,to-cao"},
		{"citizen_letter_deadline_rule_kind_valid", "deadline_kind", "giai-quyet,xu-ly-don"},
		{"citizen_letter_deadline_rule_unit_valid", "unit", "gio-lam-viec,ngay-lam-viec,ngay-lich"},
	} {
		if got := codesOf(t, sql, c.name, c.col); got != c.want {
			t.Errorf("%s = %s, want %s", c.name, got, c.want)
		}
	}
}

// Keys, live-row uniqueness, partitioning and the row-shape CHECKs.
//
// THE MUTATIONS THAT MUST TURN THIS RED: a key without tenant_id; uniqueness that counts removed rows
// (or none at all); dropping the partition loop; letting `giai-quyet` attach to a non-statutory type;
// allowing amount 0; a soft delete that is not all-three-or-none.
func TestMigration0027KeysAndConstraints(t *testing.T) {
	sql := maChay(t, file0027)
	for _, c := range []struct{ want, why string }{
		{"create table if not exists citizen_letter_deadline_rule (", "the table"},
		{"tenant_id text not null, id text not null,", "tenant_id and id are NOT NULL"},
		{"live_key boolean generated always as (case when deleted_at is null then true end) stored,",
			"live marker NULL once soft-deleted (sla.linh_vuc_khoa shape)"},
		{"primary key (tenant_id, id),", "primary key composite with tenant_id (rule 1 inv 6)"},
		{"constraint citizen_letter_deadline_rule_live_unique unique (tenant_id, letter_type, deadline_kind, live_key),",
			"one LIVE rule per commune, type, kind"},
		{"constraint citizen_letter_deadline_rule_id_ulid check (length(id) = 26),", "id is a ULID"},
		{"constraint citizen_letter_deadline_rule_resolution_type check (deadline_kind <> 'giai-quyet' or letter_type in ('khieu-nai', 'to-cao')),",
			"giai-quyet only for the two types that pass through thu-ly"},
		{"constraint citizen_letter_deadline_rule_amount_positive check (amount > 0),", "zero is refused"},
		{"check ((deleted_at is null and deleted_by is null and delete_reason is null) or (deleted_at is not null and deleted_by is not null",
			"soft delete all-three-or-none (rule 7 inv 1)"},
		{") partition by hash (tenant_id);", "hash-partitioned by tenant_id (ADR 0010)"},
		{"partition of citizen_letter_deadline_rule ' 'for values with (modulus 32, remainder %s)", "32 partitions"},
		{"on citizen_letter_deadline_rule (tenant_id, letter_type, deadline_kind) where deleted_at is null;",
			"the read index starts with tenant_id and drops removed rows"},
		{"where c.relkind = 'p' and n.nspname = current_schema() and not exists (select 1 from pg_inherits where inhparent = c.oid);",
			"partition backstop"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0027 is missing %q — %s", c.want, c.why)
		}
	}
	// The unit-per-type table is a DOMAIN rule (legal table unverified, ADR 0064): a CHECK tying unit
	// to letter_type would have to be replaced by a migration the day the legal review moves a cell.
	ddl := sql[:strings.Index(sql, ") partition by hash (tenant_id);")+1]
	constraints := strings.Split(ddl, "constraint ")
	if len(constraints) < 9 {
		t.Fatalf("found %d constraint clauses — the parser went blind", len(constraints)-1)
	}
	for _, c := range constraints[1:] {
		if strings.Contains(c, "unit") && strings.Contains(c, "letter_type") {
			t.Errorf("0027 binds unit to letter_type in a CHECK — that rule belongs to the domain (TASK-06): %.80s", c)
		}
	}
}

// Scope annotation directly above the CREATE TABLE (ADR 0021, read by tools/kb).
func TestMigration0027EntityAnnotation(t *testing.T) {
	b, err := fs.ReadFile(FS, file0027)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(b), "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, "CREATE TABLE IF NOT EXISTS citizen_letter_deadline_rule (") {
			if i < 2 || strings.TrimSpace(lines[i-2]) != "-- @entity: CitizenLetterDeadlineRule" ||
				strings.Join(strings.Fields(lines[i-1]), " ") != "-- @scope: tenant" {
				t.Error("citizen_letter_deadline_rule lacks `-- @entity: CitizenLetterDeadlineRule` + `-- @scope: tenant` directly above it")
			}
			return
		}
	}
	t.Fatal("CREATE TABLE citizen_letter_deadline_rule not found")
}

// Seeds nothing (owner, 08/10/2026) and destroys nothing.
func TestMigration0027SeedsAndDestroysNothing(t *testing.T) {
	sql := maChay(t, file0027)
	for _, bad := range []string{"insert into", "update ", "delete from", "truncate", "drop ", "alter table"} {
		if strings.Contains(sql, bad) {
			t.Errorf("0027 contains %q in its executable part — a new empty table only", bad)
		}
	}
}
