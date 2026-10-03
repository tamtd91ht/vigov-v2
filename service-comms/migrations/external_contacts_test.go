package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0014 (`external_contacts`). It asserts the keys, CHECKs, index and
// guard are WRITTEN; it does not prove PostgreSQL enforces them — the pg suites SKIP without
// VIGOV_TEST_DSN.

const file0014 = "0014_external_contacts.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: a key or index without tenant_id first; a missing partition
// loop; the guard removed or no longer refusing DELETE / edits of a deleted row; a blank name,
// category or phone accepted; the soft-delete trio no longer all-or-none; a publish flag or a
// catalogue foreign key sneaking in; a hard delete or drop.
func TestMigration0014ExternalContacts(t *testing.T) {
	sql := executableSQL(t, file0014)
	for _, c := range []struct{ want, why string }{
		{"create table if not exists external_contacts ( tenant_id text not null, id text not null,", "tenant_id and ULID id, both required"},
		{"name text not null, category text not null, phone text not null, address text, display_order int,", "the agreed columns, address and order nullable"},
		{"created_by text not null, updated_at timestamptz not null default now(), updated_by text not null,", "signed by business codes"},
		{"primary key (tenant_id, id),", "key composite with tenant_id (rule 1 inv 6)"},
		{"partition of external_contacts ' 'for values with (modulus 32, remainder %s)", "MODULUS 32 partition loop (ADR 0010)"},
		{"btrim(name) <> '' and char_length(name) <= 255", "name not blank, bounded"},
		{"btrim(category) <> '' and char_length(category) <= 100", "category not blank, bounded"},
		{"btrim(phone) <> '' and char_length(phone) <= 32 and phone ~ '^[0-9+(). -]+$'", "phone is dial characters only"},
		{"char_length(regexp_replace(phone, '[^0-9]', '', 'g')) between 3 and 15", "3 to 15 digits"},
		{"address is null or (btrim(address) <> '' and char_length(address) <= 500", "address null or not blank"},
		{"check ( display_order is null or display_order >= 0)", "order non-negative"},
		{"check (btrim(created_by) <> '' and btrim(updated_by) <> '')", "signed"},
		{"check ((deleted_at is null) = (deleted_by is null) and (deleted_at is null) = (delete_reason is null))", "soft-delete trio all or none (rule 7)"},
		{"on external_contacts (tenant_id, display_order, id) where deleted_at is null", "public read index leads with tenant_id, live rows only"},
		{"before update or delete on external_contacts for each row execute function external_contacts_guard()", "guard on update and delete"},
		{"if tg_op = 'delete' then raise exception", "hard delete refused"},
		{"if old.deleted_at is not null then raise exception", "deleted row not edited"},
		{"or new.created_by is distinct from old.created_by then", "identity columns frozen"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0014 lacks %q — %s", c.want, c.why)
		}
	}
	for _, c := range []struct{ banned, why string }{
		{"drop table", "drops a table"},
		{"drop column", "drops a column"},
		{"delete from", "deletes rows"},
		{"not valid", "NOT VALID on a partitioned table"},
		{"is_published", "no publish flag: every live row is public (header)"},
		{"references", "category is free text and nothing links to identity (rule 2)"},
		{"nguoi_dung", "not mixed into identity's staff table"},
		{"unique (", "no business unique key beyond the primary key (header)"},
		{"insert into", "nothing is seeded"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0014 contains %q — %s", c.banned, c.why)
		}
	}
}
