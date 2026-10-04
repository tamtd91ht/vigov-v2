package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0015 (`map_asset`). It asserts the keys, CHECKs, indexes and guard
// are WRITTEN; it does not prove PostgreSQL enforces them — the pg suites SKIP without VIGOV_TEST_DSN.

const file0015 = "0015_map_asset.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: a key or index without tenant_id first; a missing partition
// loop; the tax-code key turned into a partial unique index (tools/check_khoa_duy_nhat.py) or into a
// key that ignores soft delete; the type foreign key losing tenant_id; a coordinate, status, VSIC or
// verified CHECK dropped; the guard no longer refusing DELETE / edits of a deleted row; a seed row; a
// hard delete or drop; PostGIS sneaking in.
func TestMigration0015MapAsset(t *testing.T) {
	sql := executableSQL(t, file0015)
	for _, c := range []struct{ want, why string }{
		{"create table if not exists map_asset ( tenant_id text not null, id text not null, asset_type_code text not null, name text not null,", "tenant_id, ULID id, type and name required"},
		{"lat numeric(10,6) not null, lng numeric(10,6) not null,", "coordinates required, spec §10 type"},
		{"status text not null default 'dang-hoat-dong',", "status defaults to active (spec §8.1)"},
		{"custom_values jsonb not null default '{}'::jsonb,", "custom values default to an empty object"},
		{"created_by text not null, updated_at timestamptz not null default now(), updated_by text not null,", "signed by business codes"},
		{"live_tax_code text generated always as (case when deleted_at is null then tax_code end) stored,", "tax-code marker is NULL on soft-deleted rows"},
		{"primary key (tenant_id, id), unique (tenant_id, live_tax_code),", "keys composite with tenant_id (rule 1 inv 6)"},
		{"foreign key (tenant_id, asset_type_code) references loai_tai_nguyen_ban_do (tenant_id, ma)", "type issued in THIS commune's catalogue, as 0007"},
		{"partition of map_asset ' 'for values with (modulus 32, remainder %s)", "MODULUS 32 partition loop (ADR 0010)"},
		{"btrim(name) <> '' and char_length(name) <= 255", "name not blank, bounded"},
		{"check (lat between -90 and 90)", "latitude range"},
		{"check (lng between -180 and 180)", "longitude range"},
		{"check ( status in ('dang-hoat-dong', 'tam-ngung', 'da-giai-the'))", "spec's three statuses (ADR 0011 values)"},
		{"verified = (verified_at is not null) and verified = (verified_by is not null)", "verified ⇔ who and when"},
		{"tax_code is null or (char_length(tax_code) <= 20 and tax_code ~ '^[0-9]+(-[0-9]+)?$')", "tax code digits, bounded"},
		{"industry_code is null or industry_code ~ '^[0-9]{2}$'", "VSIC level 2"},
		{"employee_count is null or employee_count >= 0", "employee count non-negative"},
		{"phone is null or (btrim(phone) <> '' and char_length(phone) <= 32 and phone ~ '^[0-9+(). -]+$'", "phone is dial characters only (0014)"},
		{"jsonb_typeof(custom_values) = 'object'", "custom values are an object keyed by field_code"},
		{"check (btrim(created_by) <> '' and btrim(updated_by) <> '')", "signed"},
		{"check ((deleted_at is null) = (deleted_by is null) and (deleted_at is null) = (delete_reason is null))", "soft-delete trio all or none (rule 7)"},
		{"on map_asset (tenant_id, asset_type_code, id) where deleted_at is null", "list index leads with tenant_id, live rows only"},
		{"on map_asset (tenant_id, residential_unit_id) where deleted_at is null and residential_unit_id is not null", "hamlet index leads with tenant_id"},
		{"before update or delete on map_asset for each row execute function map_asset_guard()", "guard on update and delete"},
		{"if tg_op = 'delete' then raise exception", "hard delete refused"},
		{"if old.deleted_at is not null then raise exception", "deleted row not edited"},
		{"or new.created_by is distinct from old.created_by then", "identity columns frozen"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0015 lacks %q — %s", c.want, c.why)
		}
	}
	for _, c := range []struct{ banned, why string }{
		{"drop table", "drops a table"},
		{"drop column", "drops a column"},
		{"delete from", "deletes rows"},
		{"not valid", "NOT VALID on a partitioned table"},
		{"create unique index", "a partial unique index on tax_code is what check_khoa_duy_nhat refuses; use the marker"},
		{"unique (tenant_id, tax_code)", "a plain key would block re-entering a soft-deleted enterprise (header)"},
		{"insert into", "nothing is seeded"},
		{"geometry", "no PostGIS"},
		{"create extension", "no extension is added by a migration"},
		{"references nguoi_dung", "residential unit / staff are identity's rows, held as values (rule 2)"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0015 contains %q — %s", c.banned, c.why)
		}
	}
}
