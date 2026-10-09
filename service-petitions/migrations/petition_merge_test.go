package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migrations 0037 (merging duplicate petitions as a link, ADR 0087) and 0038
// (per-commune suspected-duplicate thresholds).
//
// Same standing as petition_settings_test.go: it reads the SQL this binary embeds and asserts the
// constraints and guards are WRITTEN. It does not prove PostgreSQL enforces them — that needs
// VIGOV_TEST_DSN (tools/schema-smoke). It is the half that always runs.

const (
	file0037 = "0037_petition_merge.sql"
	file0038 = "0038_petition_duplicate_thresholds.sql"
)

// THE MUTATIONS THAT MUST TURN THIS RED: the FK losing tenant_id (a cross-commune merge, rule 1); the
// link's who/when becoming optional; the self-merge or `can-bo` CHECK dropped; the guard losing a no-chain
// arm, the status set widening to a resolved/closed status, the deadline floor, or the INSERT arm; the
// history table losing its append-only trigger, its mandatory unmerge reason or its never-later deadline;
// an index not led by tenant_id.
func TestMigration0037PetitionMerge(t *testing.T) {
	sql := maChay(t, file0037)
	for _, c := range []struct{ want, why string }{
		{"alter table phieu_phan_anh add column if not exists merged_into text;", "the link column"},
		{"alter table phieu_phan_anh add column if not exists merged_at timestamptz;", "when"},
		{"alter table phieu_phan_anh add column if not exists merged_by text;", "who"},
		{"foreign key (tenant_id, merged_into) references phieu_phan_anh (tenant_id, id)",
			"same commune by construction (rule 1, invariant 6)"},
		{"(merged_into is null and merged_at is null and merged_by is null) or (merged_into is not null and merged_at is not null and merged_by is not null and btrim(merged_by) <> '')",
			"who/when present while linked, cleared on unlink"},
		{"check (merged_into is null or merged_into <> id)", "no self-merge"},
		{"check (merged_into is null or linh_vuc is distinct from 'can-bo')", "can-bo never merged (merged side)"},
		{"if tg_op = 'insert' then if new.merged_into is not null then raise exception", "never born merged"},
		{"if old.merged_into is not null and new.merged_into is not null then raise exception", "no direct re-point"},
		{"if exists (select 1 from phieu_phan_anh c where c.tenant_id = new.tenant_id and c.merged_into = new.id) then raise exception",
			"no chain: a main petition with children is not merged"},
		{"if main_parent is not null then raise exception", "no chain: target is itself merged"},
		{"if main_field is not distinct from 'can-bo' then raise exception", "can-bo never a main petition"},
		{"where p.tenant_id = new.tenant_id and p.id = new.merged_into for update;", "main read in the same commune, locked"},
		{"if new.han_xu_ly_xong is not null and (main_deadline is null or main_deadline > new.han_xu_ly_xong) then raise exception",
			"main petition already carries the earlier deadline"},
		{"before insert or update of merged_into, merged_at, merged_by, linh_vuc on phieu_phan_anh for each row execute function phieu_phan_anh_merge_guard()",
			"the guard is attached"},
		{"on phieu_phan_anh (tenant_id, merged_into) where merged_into is not null", "children-of-main index"},
		{"on phieu_phan_anh (tenant_id, goc_dem_han, lat, lng) where deleted_at is null and merged_into is null",
			"duplicate-candidate index"},
		{"create table if not exists petition_merge_event (", "history table"},
		{"check (kind in ('gop-phieu', 'tach-phieu'))", "closed act list"},
		{"when 'tach-phieu' then reason is not null and btrim(reason) <> ''", "unmerge reason mandatory"},
		{"main_deadline_before is null or (main_deadline_after is not null and main_deadline_after <= main_deadline_before)",
			"a deadline never moves later (ADR 0087 stop #2)"},
		{") partition by hash (tenant_id);", "partitioned by commune (ADR 0010)"},
		{"before update or delete on petition_merge_event for each row execute function petition_merge_event_append_only()",
			"history append-only"},
		{"before truncate on %s ' 'for each statement execute function petition_merge_event_append_only()",
			"history TRUNCATE refused per partition"},
		{"on petition_merge_event (tenant_id, petition_id, performed_at desc, id desc)", "history by petition"},
		{"on petition_merge_event (tenant_id, main_petition_id, performed_at desc, id desc)", "history by main"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0037 lacks %q — %s", c.want, c.why)
		}
	}
	// The unresolved set appears three times (merged OLD, merged NEW, main) and must never name a
	// resolved or closed status.
	const open = "('da-tiep-nhan', 'dang-phan-loai', 'da-chuyen-xu-ly', 'dang-xu-ly')"
	if n := strings.Count(sql, "trang_thai not in "+open); n != 2 {
		t.Errorf("0037: merged petition's status checked %d times against the unresolved set, want 2 (OLD and NEW)", n)
	}
	if !strings.Contains(sql, "main_status not in "+open) {
		t.Error("0037: the main petition's status must be checked against the unresolved set")
	}
	for _, c := range []struct{ banned, why string }{
		{"drop table", "drops a table"},
		{"drop column", "drops a column"},
		{"delete from", "deletes rows"},
		{"insert into", "writes rows — schema only"},
		{"update phieu_phan_anh", "writes rows — schema only"},
		{"'da-xu-ly', 'cho-dan-xac-nhan'", "a resolved status inside the merge set"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0037 contains %q — %s", c.banned, c.why)
		}
	}
	b, err := FS.ReadFile(file0037)
	if err != nil {
		t.Fatalf("read %s: %v", file0037, err)
	}
	if !strings.Contains(strings.ReplaceAll(string(b), "\r\n", "\n"), "-- @entity: PetitionMergeEvent\n-- @scope:  tenant") {
		t.Error("0037 must declare the new table's ownership mark (tools/kb/ownership.go)")
	}
}

// THE MUTATIONS THAT MUST TURN THIS RED: a default other than ADR 0087's 50 m / 7 days (a commune with no
// row would read differently from a row written with the defaults); a nullable column; a missing range.
func TestMigration0038PetitionDuplicateThresholds(t *testing.T) {
	sql := maChay(t, file0038)
	for _, c := range []struct{ want, why string }{
		{"add column if not exists duplicate_radius_meters int not null default 50;", "ADR 0087 §6 default 50 m"},
		{"add column if not exists duplicate_window_days int not null default 7;", "ADR 0087 §6 default 7 days"},
		{"check (duplicate_radius_meters between 1 and 1000)", "radius sanity range"},
		{"check (duplicate_window_days between 1 and 90)", "window sanity range"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0038 lacks %q — %s", c.want, c.why)
		}
	}
	for _, banned := range []string{"drop table", "drop column", "delete from", "insert into", "update petition_settings"} {
		if strings.Contains(sql, banned) {
			t.Errorf("0038 contains %q — schema only, nothing dropped", banned)
		}
	}
}
