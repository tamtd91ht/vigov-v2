package migrations

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// Schema-TEXT checks for migrations 0039 (timeline acts `gop-phieu`, `tach-phieu`, `go-tep`, added NOT
// VALID) and 0040 (their VALIDATE). Same standing as petition_log_staff_intake_test.go: it reads the SQL
// this binary embeds and asserts what is WRITTEN. That PostgreSQL accepts a NOT VALID CHECK on the
// partitioned parent and validates every partition needs VIGOV_TEST_DSN (tools/schema-smoke).

const (
	file0039 = "0039_petition_log_merge_and_attachment_removal.sql"
	file0040 = "0040_petition_log_actions_validate.sql"
)

// TestMigration0039WidensOnly — the list is 0030's eleven codes plus exactly the three new ones.
//
// THE MUTATIONS THAT MUST TURN THIS RED: an existing code dropped or misspelled (every later write of that
// act rolls back, and 0040's VALIDATE fails on the rows already holding it); a new code spelled differently
// from petition_merge_event.kind (0037) or from what the Go card writes; any extra code smuggled in.
func TestMigration0039WidensOnly(t *testing.T) {
	prev := danhSachMaTrongCheck(t, maChay(t, file0030), "nhat_ky_phan_anh_hanh_vi_hop_le", "hanh_vi")
	got := danhSachMaTrongCheck(t, maChay(t, file0039), "nhat_ky_phan_anh_hanh_vi_hop_le", "hanh_vi")
	if len(prev) != 11 {
		t.Fatalf("0030 declares %d codes, want 11 — the parser went blind or 0030 changed: %v", len(prev), prev)
	}
	want := map[string]bool{"gop-phieu": true, "tach-phieu": true, "go-tep": true}
	for _, c := range prev {
		want[c] = true
	}
	if len(got) != len(want) {
		t.Errorf("0039 declares %v, want 0030's eleven plus gop-phieu, tach-phieu, go-tep", got)
	}
	for _, c := range got {
		if !want[c] {
			t.Errorf("0039 declares unexpected code %q", c)
		}
		delete(want, c)
	}
	for c := range want {
		t.Errorf("0039 lacks code %q", c)
	}
}

// TestMigration0039MergeCodesMatchMergeHistory — one act, one name, in both records it is written to.
func TestMigration0039MergeCodesMatchMergeHistory(t *testing.T) {
	history := danhSachMaTrongCheck(t, maChay(t, file0037), "petition_merge_event_kind_valid", "kind")
	timeline := map[string]bool{}
	for _, c := range danhSachMaTrongCheck(t, maChay(t, file0039), "nhat_ky_phan_anh_hanh_vi_hop_le", "hanh_vi") {
		timeline[c] = true
	}
	for _, k := range history {
		if !timeline[k] {
			t.Errorf("petition_merge_event.kind %q has no timeline code of the same spelling in 0039", k)
		}
	}
}

// TestMigration0039IsCatalogueOnly — the swap is DROP + ADD … NOT VALID, and NOTHING scans or writes here.
//
// THE MUTATIONS THAT MUST TURN THIS RED: `NOT VALID` removed (the scan returns under ACCESS EXCLUSIVE);
// a VALIDATE moved into this file (same, the ADD's lock is held to COMMIT); the constraint renamed (the
// app's latestLogActionCheck keeps reading 0030's list); the old CHECK left in place beside a new one
// (both enforced — the new codes still refused); a row deleted or edited.
func TestMigration0039IsCatalogueOnly(t *testing.T) {
	sql := maChay(t, file0039)
	for _, c := range []struct{ want, why string }{
		{"alter table nhat_ky_phan_anh drop constraint if exists nhat_ky_phan_anh_hanh_vi_hop_le; " +
			"alter table nhat_ky_phan_anh add constraint nhat_ky_phan_anh_hanh_vi_hop_le check (hanh_vi in (",
			"the old CHECK is dropped and the new one added under the SAME name, adjacent, in one transaction"},
		{"'gop-phieu', 'tach-phieu', 'go-tep')) not valid;", "added NOT VALID — catalogue-only, no scan"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0039 lacks %q — %s", c.want, c.why)
		}
	}
	if n := strings.Count(sql, "add constraint"); n != 1 {
		t.Errorf("0039 adds %d constraints, want 1", n)
	}
	for _, c := range []struct{ banned, why string }{
		{"validate constraint", "validation belongs to 0040, outside 0039's ACCESS EXCLUSIVE"},
		{"drop table", "drops a table"},
		{"drop column", "drops a column"},
		{"drop trigger", "removes the append-only guard (0013)"},
		{"delete from", "deletes timeline rows — rule 7, forbidden #5"},
		{"update nhat_ky_phan_anh", "edits timeline rows — rule 7, forbidden #5"},
		{"insert into", "writes rows — schema only; no backfill of old removals"},
		{"nhat_ky_phan_anh_phan_cong_du_truong", "0013's assignment binding is not this file's"},
		{"nhat_ky_phan_anh_noi_dung_hop_le", "0013's note binding is not this file's"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0039 contains %q — %s", c.banned, c.why)
		}
	}
}

// TestMigration0040ValidatesOnly — the scan, and the post-check that it reached every partition.
//
// THE MUTATIONS THAT MUST TURN THIS RED: the VALIDATE missing (the list stays NOT VALID forever — old rows
// never proven); a constraint re-declared here (0040 would become the "latest list"); the post-check
// dropped or no longer counting partitions or `convalidated`.
func TestMigration0040ValidatesOnly(t *testing.T) {
	sql := maChay(t, file0040)
	for _, c := range []struct{ want, why string }{
		{"alter table nhat_ky_phan_anh validate constraint nhat_ky_phan_anh_hanh_vi_hop_le;",
			"validates 0039's constraint on the parent (recurses into the partitions)"},
		{"from pg_inherits where inhparent = 'nhat_ky_phan_anh'::regclass", "counts the partitions"},
		{"count(*) filter (where not c.convalidated)", "refuses a partition left unvalidated"},
		{"if present <> expected or pending <> 0 then raise exception", "the post-check refuses, loudly"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0040 lacks %q — %s", c.want, c.why)
		}
	}
	for _, c := range []struct{ banned, why string }{
		{"add constraint", "0040 only validates"},
		{"drop constraint", "0040 only validates"},
		{") not valid", "0040 only validates — a constraint added NOT VALID here would never be scanned"},
		{"delete from", "deletes rows"},
		{"update nhat_ky_phan_anh", "edits timeline rows"},
		{"insert into", "writes rows"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0040 contains %q — %s", c.banned, c.why)
		}
	}
}

// TestRawFilesDeclareTheListOnce — the RAW files, comments included, as internal/app
// latestLogActionCheck reads them (it takes the LAST match by file name, comments and all): 0039 declares
// the list exactly once, 0040 not at all.
//
// THE MUTATION THAT MUST TURN THIS RED: the reversal comment rewritten as SQL — "re-add 0030's eleven
// codes" spelled as a CHECK. The app test would then read the narrow list as current, and the Go card's
// codes as refused.
func TestRawFilesDeclareTheListOnce(t *testing.T) {
	decl := regexp.MustCompile(`(?is)nhat_ky_phan_anh_hanh_vi_hop_le\s+CHECK\s*\(\s*hanh_vi\s+IN\s*\(`)
	for _, c := range []struct {
		file string
		want int
	}{{file0039, 1}, {file0040, 0}} {
		b, err := fs.ReadFile(FS, c.file)
		if err != nil {
			t.Fatalf("read %s: %v", c.file, err)
		}
		if got := len(decl.FindAllIndex(b, -1)); got != c.want {
			t.Errorf("%s declares the act list %d times (comments included), want %d", c.file, got, c.want)
		}
	}
}
