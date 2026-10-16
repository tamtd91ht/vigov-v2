package migrations

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/textfold"
)

// Schema-TEXT checks for migration 0033 (citizen contact directory, owner decisions 16/10/2026).
// Same weaker-half caveat as danh_ba_mini_app_test.go: it proves the DDL is WRITTEN this way, not that
// PostgreSQL enforces it. No pg suite exists yet for these tables; this file is what always runs.

const file0033 = "0033_citizen_directory.sql"

// embeddedRaw is the file as embedded, comments kept, case kept: the fold expression carries capital
// letters (`Đ`, `U&'\031B'`) that maChay's lower-casing would erase from both sides of the comparison.
func embeddedRaw(t *testing.T, name string) string {
	t.Helper()
	b, err := fs.ReadFile(FS, name)
	if err != nil {
		t.Fatalf("đọc %s: %v", name, err)
	}
	// A checkout with CRLF line endings must compare like one with LF.
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

// THE STORED FOLD AND THE TERM'S FOLD ARE ONE FUNCTION (the service-petitions 0041 precedent). The
// store will fold the search term with textfold.SQL at query time; the columns were folded with the
// text in this file at migration time. If core/textfold's table changes, this turns red: the stored
// columns were computed with the OLD table and a new migration must recompute them.
func TestMigration0033FoldedColumnsAreTextfoldSQL(t *testing.T) {
	sql := embeddedRaw(t, file0033)
	for _, c := range []struct{ column, source string }{
		{"full_name_folded", "full_name"},
		{"name_folded", "name"},
	} {
		// The column name is matched separately from the expression: the DDL pads names to a
		// column, and a padding change must not read as a fold change.
		want := "GENERATED ALWAYS AS (\n        " + textfold.SQL(c.source) + "\n    ) STORED,"
		if !strings.Contains(sql, c.column+" ") || !strings.Contains(sql, want) {
			t.Errorf("0033 không còn khai %s đúng bằng textfold.SQL(%q) — cột lưu và từ khoá tìm sẽ gập khác nhau",
				c.column, c.source)
		}
	}
}

// Keys, live-row uniqueness, foreign keys and the row-shape CHECKs of all three tables.
//
// THE MUTATIONS THAT MUST TURN THIS RED: a key without tenant_id; a foreign key on the id alone
// (another commune's unit / contact / group accepted); phone uniqueness that counts removed rows (a
// contact removed by mistake can never be re-added) or none at all; a phone CHECK that admits
// anything but 9–15 digits; a soft delete that is not all-three-or-none on contact / membership;
// a membership PK without created_at (re-adding after removal would be an edit of the old row).
func TestMigration0033KeysAndConstraints(t *testing.T) {
	sql := maChay(t, file0033)
	for _, c := range []struct{ want, why string }{
		// citizen_contact
		{"create table if not exists citizen_contact (", "the contact table"},
		{"tenant_id text not null, id text not null,", "contact: tenant_id and id NOT NULL"},
		{"full_name text not null,", "a contact has a name"},
		{"phone text not null,", "a contact has a phone"},
		{"tags text[] not null default '{}',", "free labels, never NULL"},
		{"is_active boolean not null default true,", "active by default"},
		{"primary key (tenant_id, id), constraint citizen_contact_phone_live_unique unique (tenant_id, phone, live_key),",
			"contact PK composite with tenant_id; ONE LIVE row per phone (rule 7 inv 3 does not apply: a phone is not an issued code)"},
		{"constraint citizen_contact_residential_unit_fkey foreign key (tenant_id, residential_unit_id) references thon_to_dan_pho (tenant_id, id),",
			"the residential unit, in THIS commune"},
		{"constraint citizen_contact_id_ulid check (length(id) = 26),", "contact id is a ULID"},
		{"constraint citizen_contact_phone_digits check (phone ~ '^[0-9]{9,15}$'),", "phone is 9–15 digits, normalised"},
		{"constraint citizen_contact_import_batch_ulid check (import_batch_id is null or length(import_batch_id) = 26),",
			"import batch id is a ULID when present"},
		{"check ((deleted_at is null and deleted_by is null and delete_reason is null) or (deleted_at is not null and deleted_by is not null and btrim(deleted_by) <> '' and delete_reason is not null and btrim(delete_reason) <> '' and char_length(delete_reason) <= 500))",
			"contact soft delete all-three-or-none (rule 7 inv 1)"},
		{"on citizen_contact (tenant_id, is_active) where deleted_at is null;", "the list index starts with tenant_id and drops removed rows"},
		{"on citizen_contact (tenant_id, residential_unit_id) where deleted_at is null;", "the unit filter index starts with tenant_id"},
		// citizen_group
		{"create table if not exists citizen_group (", "the group table"},
		{"sort_order integer not null default 0,", "groups have an order"},
		{"primary key (tenant_id, id), constraint citizen_group_name_live_unique unique (tenant_id, name_folded, live_key),",
			"group PK composite with tenant_id; ONE LIVE group per folded name"},
		{"constraint citizen_group_id_ulid check (length(id) = 26),", "group id is a ULID"},
		{"and (delete_reason is null or (btrim(delete_reason) <> '' and char_length(delete_reason) <= 500))))",
			"group delete reason is optional (owner 16/10/2026) but never blank"},
		{"on citizen_group (tenant_id, sort_order, name_folded) where deleted_at is null;", "the group list index starts with tenant_id"},
		// citizen_contact_group
		{"create table if not exists citizen_contact_group (", "the membership table"},
		{"primary key (tenant_id, contact_id, group_id, created_at), constraint citizen_contact_group_live_unique unique (tenant_id, contact_id, group_id, live_key),",
			"membership: composite with tenant_id, created_at in the PK (re-add is a new row), ONE LIVE row per person and group"},
		{"constraint citizen_contact_group_contact_fkey foreign key (tenant_id, contact_id) references citizen_contact (tenant_id, id),",
			"the contact, in THIS commune"},
		{"constraint citizen_contact_group_group_fkey foreign key (tenant_id, group_id) references citizen_group (tenant_id, id),",
			"the group, in THIS commune"},
		{"constraint citizen_contact_group_created_by_not_blank check (btrim(created_by) <> ''),", "who added it (rule 6 inv 8)"},
		{"on citizen_contact_group (tenant_id, group_id) where deleted_at is null;", "the members-of-group index starts with tenant_id"},
		// shared
		{"where c.relkind = 'p' and n.nspname = current_schema() and not exists (select 1 from pg_inherits where inhparent = c.oid);",
			"partition backstop"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0033 is missing %q — %s", c.want, c.why)
		}
	}
	// Every table carries the live marker in the same shape (sla.linh_vuc_khoa, 0008).
	if n := strings.Count(sql, "live_key boolean generated always as (case when deleted_at is null then true end) stored,"); n != 3 {
		t.Errorf("0033 declares live_key on %d tables, want 3", n)
	}
	if got := codesOf(t, sql, "citizen_contact_source_valid", "source"); got != "nhap-excel,nhap-tay" {
		t.Errorf("citizen_contact_source_valid = %s, want nhap-excel,nhap-tay (a third value is a decision, not a drift)", got)
	}
}

// ADR 0096: the two tables that grow with citizens are MODULUS 8; the group catalogue is plain.
// The MUTATIONS THAT MUST TURN THIS RED: citizen_group partitioned "to be safe"; a contact table made
// plain; a leaf loop missing or 32-way.
func TestMigration0033PartitioningFollowsADR0096(t *testing.T) {
	sql := maChay(t, file0033)
	requireHashPartitionedBy8(t, file0033, sql, "citizen_contact")
	requireHashPartitionedBy8(t, file0033, sql, "citizen_contact_group")
	requirePlainTable(t, file0033, sql, "citizen_group")
}

// Entity marks directly above each CREATE TABLE (ADR 0021, read by tools/kb into data-ownership.json).
func TestMigration0033EntityAnnotations(t *testing.T) {
	lines := strings.Split(embeddedRaw(t, file0033), "\n")
	for _, c := range []struct{ table, entity string }{
		{"citizen_contact", "CitizenContact"},
		{"citizen_group", "CitizenGroup"},
		{"citizen_contact_group", "CitizenContactGroup"},
	} {
		found := false
		for i, l := range lines {
			if !strings.HasPrefix(l, "CREATE TABLE IF NOT EXISTS "+c.table+" (") {
				continue
			}
			found = true
			if i < 2 || strings.TrimSpace(lines[i-2]) != "-- @entity: "+c.entity ||
				strings.Join(strings.Fields(lines[i-1]), " ") != "-- @scope: tenant" {
				t.Errorf("%s lacks `-- @entity: %s` + `-- @scope: tenant` directly above it", c.table, c.entity)
			}
		}
		if !found {
			t.Errorf("CREATE TABLE %s not found", c.table)
		}
	}
}

// PERSONAL DATA IS SAID ON THE COLUMN (rule 3), as 0004 does for so_dien_thoai — the one place a
// person running `\d+` in psql reads before writing a query.
func TestMigration0033MarksPersonalDataColumns(t *testing.T) {
	sql := maChay(t, file0033)
	for _, col := range []string{"full_name", "full_name_folded", "phone", "address"} {
		if !strings.Contains(sql, "comment on column citizen_contact."+col+" is 'du lieu ca nhan (luat 3)") {
			t.Errorf("citizen_contact.%s has no COMMENT saying it is personal data (rule 3)", col)
		}
	}
}

// Seeds nothing and destroys nothing: three new empty tables only.
func TestMigration0033SeedsAndDestroysNothing(t *testing.T) {
	sql := maChay(t, file0033)
	for _, bad := range []string{"insert into", "update ", "delete from", "truncate", "drop ", "alter table", "create extension", "unaccent"} {
		if strings.Contains(sql, bad) {
			t.Errorf("0033 contains %q in its executable part — new empty tables only", bad)
		}
	}
	for _, index := range []string{"create unique index"} {
		if strings.Contains(sql, index) {
			t.Errorf("0033 contains %q — uniqueness is a composite UNIQUE with live_key, never a partial unique index", index)
		}
	}
}
