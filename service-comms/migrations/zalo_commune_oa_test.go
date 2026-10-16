package migrations

import (
	"io/fs"
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0031 (a commune's Zalo OA authorized to the shared app, ADR 0098
// block A). They assert the keys, CHECKs, guards and the two declared cross-tenant indexes are WRITTEN;
// they do not prove PostgreSQL enforces them — tools/schema-smoke and the pg suites need VIGOV_TEST_DSN
// for that.

const file0031 = "0031_zalo_commune_oa.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: any of the three tables partitioned (ADR 0096; the two
// platform-wide keys could not exist then); the one-live-per-commune key losing tenant_id or its
// live-only predicate; the OA floor or the state lookup gaining tenant_id (a second commune could then
// hold the same OA live, or a `state` could be reissued); the state lookup made partial (a replayed
// state would meet no row and read as fresh); a guard no longer refusing DELETE, no longer freezing a
// retired OA or a used request, or no longer demanding a new token_refreshed_at / milestone_set_at; the
// OA guard regaining 0022's set_at rule (the refresh job would then be refused); a CHECK tying milestone
// to Zalo's status (a sync could no longer record Zalo's verdict); a soft-delete trio on templates; an
// enum of OUR OWN translated to English, or Zalo's values leaking into any column but the two named for
// them; a seed row.
func TestMigration0031ZaloCommuneOAKeysAndGuards(t *testing.T) {
	sql := executableSQL(t, file0031)
	requirePlainTable(t, file0031, sql, "zalo_commune_oa")
	requirePlainTable(t, file0031, sql, "zalo_oa_authorization")
	requirePlainTable(t, file0031, sql, "zalo_oa_template")

	for _, c := range []struct{ want, why string }{
		// zalo_commune_oa
		{"primary key (tenant_id, id), constraint zalo_commune_oa_oa_id_shape check (oa_id ~ '^[0-9]{1,32}$'),", "OA row key composite with tenant_id; oa_id is Zalo's digits"},
		{"access_token_sealed bytea not null, refresh_token_sealed bytea not null,", "both tokens sealed, never nullable"},
		{"check (octet_length(access_token_sealed) > 29)", "a sealed envelope, never a bare token"},
		{"check (octet_length(refresh_token_sealed) > 29)", "a sealed envelope, never a bare token"},
		{"access_expires_at timestamptz not null, refresh_issued_at timestamptz not null, token_refreshed_at timestamptz not null, authorized_at timestamptz not null, authorized_by text not null,", "the two clocks of ADR 0098 §Hai cột thời điểm, plus the token lifetimes"},
		{"check (btrim(authorized_by) <> '' and char_length(authorized_by) <= 64)", "who connected is a staff code (rule 6 inv 8)"},
		{"check ((last_check_at is null) = (last_check_result is null))", "check pair"},
		{"'thanh-cong', 'chua-cau-hinh', 'token-bi-tu-choi', 'gioi-han-tan-suat', 'khong-kha-dung', 'bi-tu-choi', 'phan-hoi-sai-dang'", "0022's seven classes, ADR 0011 spelling"},
		{"check ((needs_reauthorization_since is null) = (needs_reauthorization_reason is null))", "re-authorization pair"},
		{"needs_reauthorization_reason in ('refresh-token-bi-tu-choi', 'refresh-token-het-han')", "ADR 0098 #6's two causes"},
		{"check ((retired_at is null) = (retired_by is null) and (retired_at is null) = (retire_reason is null))", "retire trio all or none (rule 7)"},
		{"check (retire_reason is null or (btrim(retire_reason) <> '' and char_length(retire_reason) <= 200))", "a reason, bounded"},
		{"check (retired_at is null or retired_at >= created_at)", "retired after created"},
		{"on zalo_commune_oa (tenant_id) where retired_at is null", "one live OA per commune"},
		{"on zalo_commune_oa (oa_id) where retired_at is null", "one live place per OA, platform-wide (floor)"},
		// zalo_oa_authorization
		{"check (octet_length(state_hash) = 32)", "a hash of the state, never the state"},
		{"check (octet_length(code_verifier_sealed) > 29)", "the verifier is sealed, not hashed"},
		{"check (btrim(requested_by) <> '' and char_length(requested_by) <= 64)", "who asked is a staff code"},
		{"check (expires_at > requested_at)", "a link expires after it is made"},
		{"check ((used_at is null) = (outcome is null))", "spent has an outcome"},
		{"outcome in ('thanh-cong', 'zalo-tu-choi', 'khong-kha-dung', 'phan-hoi-sai-dang', 'het-han', 'oa-dang-o-xa-khac', 'sai-khuon')", "the seven outcomes, ADR 0011 spelling"},
		{"check ((outcome is not distinct from 'thanh-cong') = (oa_row_id is not null))", "success records the OA row it made; nothing else does"},
		{"create unique index if not exists zalo_oa_authorization_state_hash on zalo_oa_authorization (state_hash);", "the callback's lookup: platform-wide, total (ADR 0098 #4)"},
		{"on zalo_oa_authorization (tenant_id, expires_at) where used_at is null", "the sweep's read, per commune"},
		// zalo_oa_template
		{"unique (tenant_id, oa_row_id, template_id), foreign key (tenant_id, oa_row_id) references zalo_commune_oa (tenant_id, id),", "one row per template per OA row; a template names a real OA row"},
		{"check (template_id ~ '^[0-9a-za-z_-]{1,64}$')", "Zalo's template id shape"},
		{"zalo_status text not null, zalo_quality text not null default 'undefined',", "Zalo's values live in the two columns named for them"},
		{"check (zalo_status in ('enable', 'pending_review', 'reject', 'disable', 'delete'))", "Zalo's five statuses, verbatim (named ADR 0011 exception)"},
		{"check (zalo_quality in ('high', 'medium', 'low', 'undefined'))", "Zalo's four qualities, verbatim"},
		{"check (jsonb_typeof(params) = 'array')", "params is the declared array"},
		{"params_match boolean not null default false,", "Go's verdict; false until proven"},
		{"milestone in ('da-tiep-nhan', 'da-chuyen-xu-ly', 'cho-dan-xac-nhan', 'da-dong', 'khong-tiep-nhan', 'chuyen-cap-tren', 'gop-phieu', 'tach-phieu')", "ADR 0041 targets + ADR 0087 merge/split"},
		{"check ((milestone is null) = (milestone_set_at is null) and (milestone is null) = (milestone_set_by is null))", "milestone trio all or none"},
		{"on zalo_oa_template (tenant_id, milestone) where milestone is not null", "one template per milestone per commune"},
		{"and not exists (select 1 from pg_inherits where inhparent = c.oid)", "the BACKSTOP is present"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0031 lacks %q — %s", c.want, c.why)
		}
	}

	// The two platform-wide keys must stay platform-wide, and the state lookup total.
	for _, c := range []struct{ banned, why string }{
		{"on zalo_commune_oa (tenant_id, oa_id)", "the OA floor gained tenant_id — two communes could hold one OA live"},
		{"on zalo_oa_authorization (tenant_id, state_hash)", "the state lookup gained tenant_id — the callback has no commune to filter by (ADR 0098 #4)"},
		{"on zalo_oa_authorization (state_hash) where", "the state lookup became partial — a replayed state would meet no row"},
		{"unique (tenant_id, state_hash)", "a per-commune state key is not the callback's lookup"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0031 contains %q — %s", c.banned, c.why)
		}
	}

	// No CHECK ties the commune's milestone to Zalo's status or to params_match: the read checks it.
	tpl := createTableStatement(t, file0031, sql, "zalo_oa_template")
	for _, banned := range []string{"milestone is null or zalo_status", "milestone is null or params_match",
		"zalo_status = 'enable' or milestone", "params_match or milestone", "deleted_at"} {
		if strings.Contains(tpl, banned) {
			t.Errorf("zalo_oa_template contains %q — milestone is checked at read time, and templates have no soft delete (0031 header)", banned)
		}
	}

	for _, c := range []struct{ fn, table string }{
		{"zalo_commune_oa_guard", "zalo_commune_oa"},
		{"zalo_oa_authorization_guard", "zalo_oa_authorization"},
		{"zalo_oa_template_guard", "zalo_oa_template"},
	} {
		if !strings.Contains(functionBody(t, sql, c.fn), "if tg_op = 'delete' then raise exception") {
			t.Errorf("%s no longer refuses DELETE", c.fn)
		}
		if !strings.Contains(sql, "before update or delete on "+c.table+" for each row execute function "+c.fn+"()") {
			t.Errorf("%s is not attached to %s", c.fn, c.table)
		}
	}
	for _, c := range []struct{ want, fn, why string }{
		{"if old.retired_at is not null then raise exception", "zalo_commune_oa_guard", "a retired OA is history"},
		{"or new.oa_id is distinct from old.oa_id", "zalo_commune_oa_guard", "a different OA is a different row"},
		{"or new.authorized_at is distinct from old.authorized_at or new.authorized_by is distinct from old.authorized_by", "zalo_commune_oa_guard", "who connected is fixed (rule 6 inv 8)"},
		{"if (new.access_token_sealed is distinct from old.access_token_sealed or new.refresh_token_sealed is distinct from old.refresh_token_sealed) and new.token_refreshed_at is not distinct from old.token_refreshed_at then raise exception", "zalo_commune_oa_guard", "resealed tokens leave a time (ADR 0098 §Hai cột thời điểm)"},
		{"if old.used_at is not null then raise exception", "zalo_oa_authorization_guard", "a spent request is frozen"},
		{"or new.state_hash is distinct from old.state_hash or new.code_verifier_sealed is distinct from old.code_verifier_sealed or new.requested_at is distinct from old.requested_at or new.requested_by is distinct from old.requested_by or new.expires_at is distinct from old.expires_at or new.created_at is distinct from old.created_at", "zalo_oa_authorization_guard", "an issued request is immutable"},
		{"or new.oa_row_id is distinct from old.oa_row_id or new.template_id is distinct from old.template_id or new.created_at is distinct from old.created_at", "zalo_oa_template_guard", "a template belongs to its OA row"},
		{"if new.milestone is distinct from old.milestone and new.milestone_set_at is not distinct from old.milestone_set_at then raise exception", "zalo_oa_template_guard", "a changed milestone leaves who and when"},
	} {
		if !strings.Contains(functionBody(t, sql, c.fn), c.want) {
			t.Errorf("%s lacks %q — %s", c.fn, c.want, c.why)
		}
	}
	// The OA guard must NOT be 0022's: a reseal is keyed to token_refreshed_at, never to set_at, or the
	// background refresh job is refused at its first run (ADR 0098 §Hai cột thời điểm).
	if strings.Contains(functionBody(t, sql, "zalo_commune_oa_guard"), "set_at") {
		t.Error("zalo_commune_oa_guard mentions set_at — 0022's rule would refuse the refresh job")
	}
	// The template guard must NOT freeze Zalo's columns: every sync rewrites them.
	if g := functionBody(t, sql, "zalo_oa_template_guard"); strings.Contains(g, "new.zalo_status is distinct") || strings.Contains(g, "new.params is distinct") {
		t.Error("zalo_oa_template_guard freezes Zalo's columns — a sync could then never record Zalo's verdict")
	}

	// Enumerations of OUR OWN stay Vietnamese (ADR 0011); Zalo's values appear exactly once each — in the
	// two CHECKs asserted above — and nowhere else.
	for _, banned := range []string{"'success'", "'pending'", "'expired'", "'failed'", "'rejected'", "'refused'",
		"'unavailable'", "'received'", "'closed'", "'merged'", "'split'", "'escalated'"} {
		if strings.Contains(sql, banned) {
			t.Errorf("0031 contains %s — enum values of our own stay Vietnamese (ADR 0011)", banned)
		}
	}
	for _, c := range []struct {
		value  string
		inside int
	}{
		{"'enable'", 1}, {"'pending_review'", 1}, {"'reject'", 1}, {"'disable'", 1}, {"'delete'", 1},
		{"'high'", 1}, {"'medium'", 1}, {"'low'", 1},
		{"'undefined'", 2}, // the DEFAULT and the CHECK
	} {
		if n := strings.Count(tpl, c.value); n != c.inside {
			t.Errorf("%s appears %d times in zalo_oa_template, want %d (zalo_status / zalo_quality only)", c.value, n, c.inside)
		}
		outside := strings.Count(sql, c.value) - strings.Count(tpl, c.value)
		// `tg_op = 'delete'` is every guard's DELETE refusal, not an enum value.
		if c.value == "'delete'" {
			outside -= strings.Count(sql, "tg_op = 'delete'")
		}
		if outside != 0 {
			t.Errorf("%s appears %d times outside zalo_oa_template — Zalo's values belong to the two columns named for them", c.value, outside)
		}
	}

	noDestruction(t, file0031, sql)
}

// The ownership marks tools/kb reads: three entities, each `@scope: tenant`, each above its own table;
// and EXACTLY TWO `@scope: cross-tenant` marks, each above the one index it declares — the OA floor and
// the state lookup. A third would be an undeclared cross-commune read; one fewer would hide one.
func TestMigration0031OwnershipAndCrossTenantMarks(t *testing.T) {
	b, err := fs.ReadFile(FS, file0031)
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	last := -1
	for _, c := range []struct{ entity, table string }{
		{"ZaloCommuneOA", "zalo_commune_oa"},
		{"ZaloOAAuthorization", "zalo_oa_authorization"},
		{"ZaloOATemplate", "zalo_oa_template"},
	} {
		mark := strings.Index(src, "-- @entity: "+c.entity+"\n-- @scope:  tenant\n")
		create := strings.Index(src, "CREATE TABLE IF NOT EXISTS "+c.table+" (")
		if mark < 0 || create < 0 || mark > create || mark < last {
			t.Errorf("@entity: %s / @scope: tenant must precede CREATE TABLE %s, in order", c.entity, c.table)
		}
		if next := strings.Index(src[mark+1:], "-- @entity:"); next >= 0 && mark+1+next < create {
			t.Errorf("another @entity mark sits between %s and its table", c.entity)
		}
		last = create
	}
	if n := strings.Count(src, "-- @entity:"); n != 3 {
		t.Errorf("%d @entity marks, want 3", n)
	}

	if n := strings.Count(src, "-- @scope: cross-tenant"); n != 2 {
		t.Errorf("%d @scope: cross-tenant marks, want exactly 2 (the OA floor and the state lookup)", n)
	}
	for _, c := range []struct{ index, why string }{
		{"CREATE UNIQUE INDEX IF NOT EXISTS zalo_commune_oa_one_live_per_oa\n", "the OA floor is a cross-commune uniqueness check"},
		{"CREATE UNIQUE INDEX IF NOT EXISTS zalo_oa_authorization_state_hash\n", "the state lookup is a cross-commune read (ADR 0098 #4)"},
	} {
		i := strings.Index(src, c.index)
		if i < 0 {
			t.Errorf("0031 lacks %q", strings.TrimSpace(c.index))
			continue
		}
		// The mark opens the comment block directly above the statement: no other STATEMENT between
		// (comment prose may hold a semicolon; executable text may not).
		mark := strings.LastIndex(src[:i], "-- @scope: cross-tenant")
		if mark < 0 || strings.Contains(withoutComments(src[mark:i]), ";") {
			t.Errorf("%q is not directly under a @scope: cross-tenant mark — %s", strings.TrimSpace(c.index), c.why)
		}
	}
}

// withoutComments drops every `--` comment, as executableSQL does, but keeps case and line breaks.
func withoutComments(s string) string {
	var lines []string
	for _, l := range strings.Split(s, "\n") {
		if i := strings.Index(l, "--"); i >= 0 {
			l = l[:i]
		}
		lines = append(lines, l)
	}
	return strings.Join(lines, "\n")
}
