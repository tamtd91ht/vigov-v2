package migrations

import (
	"io/fs"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0011 (comms `stored_file`, the six new `noi_dung_mini_app`
// columns, G1 in `noi_dung_mini_app_bat_bien`).
//
// It reads the SQL this binary embeds and asserts the keys, CHECKs and triggers are WRITTEN. It does
// not prove PostgreSQL enforces any of it: that needs a database, and this service's pg suites SKIP
// without VIGOV_TEST_DSN.

const (
	file0006 = "0006_noi_dung_mini_app.sql"
	file0011 = "0011_content_item_media_and_event.sql"
)

// executableSQL returns the file with `--` comments removed, whitespace collapsed, lower-cased, so
// the assertions read the statements and never the prose that explains them (the REVERSAL block
// names DROP TABLE on purpose).
func executableSQL(t *testing.T, name string) string {
	t.Helper()
	b, err := fs.ReadFile(FS, name)
	if err != nil {
		t.Fatalf("read %s from the embedded FS: %v", name, err)
	}
	var lines []string
	for _, l := range strings.Split(string(b), "\n") {
		if i := strings.Index(l, "--"); i >= 0 {
			l = l[:i]
		}
		lines = append(lines, l)
	}
	return strings.ToLower(strings.Join(strings.Fields(strings.Join(lines, " ")), " "))
}

// functionBody returns the text between `create or replace function <name>()` and its closing `end $$;`.
func functionBody(t *testing.T, sql, name string) string {
	t.Helper()
	head := "create or replace function " + name + "()"
	i := strings.Index(sql, head)
	if i < 0 {
		t.Fatalf("function %s not found", name)
	}
	j := strings.Index(sql[i:], "end $$;")
	if j < 0 {
		t.Fatalf("end of function %s not found", name)
	}
	return sql[i : i+j]
}

// THE MUTATIONS THAT MUST TURN THIS RED: a key or index without tenant_id first; a missing partition
// loop; removing the guard or the cover check; a per-type CHECK dropped; a single-column key.
func TestMigration0011KeysPartitionsConstraints(t *testing.T) {
	sql := executableSQL(t, file0011)
	for _, c := range []struct{ want, why string }{
		{"primary key (tenant_id, id), unique (tenant_id, object_key), unique (tenant_id, public_object_key),", "stored_file keys composite with tenant_id (rule 1 inv 6)"},
		{"partition of stored_file ' 'for values with (modulus 32, remainder %s)", "stored_file partition loop"},
		{"on stored_file (tenant_id, subject_type, subject_id, created_at desc) where deleted_at is null", "subject index leads with tenant_id"},
		{"before update or delete on stored_file for each row execute function stored_file_guard()", "metadata guard"},
		{"check (subject_type in ('content-item'))", "subject is a content item"},
		{"check (bucket in ('private'))", "originals are private"},
		{"strpos(object_key, '/comms/' || purpose || '/' || lower(id) || '/') > 0", "key names this service and row"},
		{"starts_with(object_key, retention_class || '/t_' || lower(tenant_id) || '/')", "key names this commune"},
		{"starts_with(public_object_key, 'public-media/t_' || lower(tenant_id) || '/')", "public key names this commune"},
		{"public_object_key !~ '/original\\.[a-z0-9]+$'", "the original never goes public"},
		{"check ( public_object_key is null or (status = 'ready' and deleted_at is null))", "public only when ready and live"},
		{"foreign key (tenant_id, cover_image_file_id) references stored_file (tenant_id, id)", "cover FK composite with tenant_id"},
		{"check (loai = 'su-kien' or (event_starts_at is null and event_ends_at is null and event_place is null))", "event columns only on su-kien"},
		{"check (event_ends_at is null or (event_starts_at is not null and event_ends_at >= event_starts_at))", "event window"},
		{"check (loai = 'video' or video_url is null)", "video_url only on video"},
		{"video_url ~* '^https?://[^[:space:][:cntrl:]]+$' and char_length(video_url) <= 2048", "http(s)-only bounded link"},
		{"char_length(event_place) <= 500", "bounded place"},
		{"before insert or update of cover_image_file_id on noi_dung_mini_app for each row execute function noi_dung_mini_app_cover_image_check()", "cover floor"},
		{"or file_row.subject_id <> new.id or file_row.purpose <> 'content-image' then", "cover uploaded for this item as an image"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0011 lacks %q — %s", c.want, c.why)
		}
	}
	for _, col := range []string{"cover_image_file_id text", "published_at timestamptz", "event_starts_at timestamptz",
		"event_ends_at timestamptz", "event_place text", "video_url text"} {
		if !strings.Contains(sql, "alter table noi_dung_mini_app add column if not exists "+col+";") {
			t.Errorf("0011 does not add the NULLABLE column %q", col)
		}
	}
	for _, c := range []struct{ banned, why string }{
		{"drop table", "drops a table"},
		{"drop column", "drops a column (anh_dai_dien_url stays, rule 7)"},
		{"delete from", "deletes rows"},
		{"update noi_dung_mini_app", "backfills — G1 says existing rows keep NULL"},
		{"not null default", "a new column on noi_dung_mini_app must be nullable"},
		{"not valid", "NOT VALID on a partitioned table (see the file's constraint note)"},
		{"unique (object_key)", "single-column unique key"},
		{"audio", "audio is a later card"},
	} {
		if strings.Contains(sql[strings.Index(sql, "alter table noi_dung_mini_app"):], c.banned) {
			t.Errorf("0011 (noi_dung_mini_app part) contains %q — %s", c.banned, c.why)
		}
	}
	for _, banned := range []string{"drop table", "drop column", "delete from", "not valid", "audio"} {
		if strings.Contains(sql, banned) {
			t.Errorf("0011 contains %q", banned)
		}
	}
}

// THE REPLACED FUNCTION MUST STILL CARRY EVERY 0006 RULE. CREATE OR REPLACE is a full rewrite: a
// clause left out here is a rule silently repealed. Mutation: delete any 0006 IF-block from 0011.
func TestMigration0011KeepsEvery0006ImmutabilityRule(t *testing.T) {
	old := functionBody(t, executableSQL(t, file0006), "noi_dung_mini_app_bat_bien")
	cur := functionBody(t, executableSQL(t, file0011), "noi_dung_mini_app_bat_bien")
	oldBody := strings.TrimSpace(old[:strings.LastIndex(old, "return new;")])
	if !strings.Contains(cur, oldBody) {
		t.Fatal("0011's noi_dung_mini_app_bat_bien no longer contains 0006's body verbatim — a rule was repealed")
	}
	for _, want := range []string{
		"if old.published_at is not null and new.published_at is distinct from old.published_at then raise exception",
		"if old.published_at is null and new.published_at is not null and not (new.trang_thai = 'dang-hien' and old.trang_thai is distinct from 'dang-hien') then raise exception",
	} {
		if !strings.Contains(cur, want) {
			t.Errorf("G1 clause missing: %q", want)
		}
	}
}

// ADR 0052 §5's machine — the same edge set as service-petitions 0021 (written out: this module does
// not import petitions). Mutation: add or drop one pair.
func TestMigration0011StatusEdgesAreADR0052s(t *testing.T) {
	sql := executableSQL(t, file0011)
	const head = "(old.status, new.status) not in ("
	i := strings.Index(sql, head)
	if i < 0 {
		t.Fatal("transition list not found in stored_file_guard")
	}
	j := strings.Index(sql[i:], ") then raise exception")
	if j < 0 {
		t.Fatal("end of transition list not found")
	}
	var got []string
	for _, m := range regexp.MustCompile(`\('([a-z]+)', '([a-z]+)'\)`).FindAllStringSubmatch(sql[i+len(head):i+j], -1) {
		got = append(got, m[1]+"->"+m[2])
	}
	want := []string{
		"pending->scanning", "pending->failed", "pending->rejected",
		"scanning->stored", "scanning->failed", "scanning->rejected", "scanning->pending",
		"stored->processing", "stored->purged",
		"processing->ready", "processing->failed",
		"ready->purged", "failed->purged", "rejected->purged",
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("trigger edges %v\nADR 0052 edges %v", got, want)
	}
}
