package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0030 (the timeline act `nhap-ho`, owner decision 02/10/2026, ADR 0028
// Bổ sung 2026-10-02 row 6). Same standing as nhat_ky_phan_anh_test.go.

const file0030 = "0030_petition_log_staff_intake.sql"

// TestMigration0030WidensOnly — the list is 0023's ten codes plus `nhap-ho`, nothing dropped.
//
// THE MUTATIONS THAT MUST TURN THIS RED: an existing code dropped or misspelled (every row holding it
// fails ADD CONSTRAINT, or every later write of that act rolls back); `nhap-ho` missing or spelled
// differently from what the intake card will write; any extra code smuggled in.
func TestMigration0030WidensOnly(t *testing.T) {
	prev := danhSachMaTrongCheck(t, maChay(t, "0023_petition_log_task_created.sql"),
		"nhat_ky_phan_anh_hanh_vi_hop_le", "hanh_vi")
	got := danhSachMaTrongCheck(t, maChay(t, file0030), "nhat_ky_phan_anh_hanh_vi_hop_le", "hanh_vi")
	if len(prev) != 10 {
		t.Fatalf("0023 declares %d codes, want 10 — the parser went blind or 0023 changed: %v", len(prev), prev)
	}
	want := map[string]bool{"nhap-ho": true}
	for _, c := range prev {
		want[c] = true
	}
	if len(got) != len(want) {
		t.Errorf("0030 declares %v, want 0023's ten plus nhap-ho", got)
	}
	for _, c := range got {
		if !want[c] {
			t.Errorf("0030 declares unexpected code %q", c)
		}
		delete(want, c)
	}
	for c := range want {
		t.Errorf("0030 lacks code %q", c)
	}
}

func TestMigration0030TouchesOnlyTheCheck(t *testing.T) {
	sql := maChay(t, file0030)
	if !strings.Contains(sql, "alter table nhat_ky_phan_anh drop constraint if exists nhat_ky_phan_anh_hanh_vi_hop_le;") {
		t.Error("0030 must drop the old CHECK by name, in the same transaction as the new one")
	}
	for _, c := range []struct{ banned, why string }{
		{"drop table", "drops a table"},
		{"drop column", "drops a column"},
		{"drop trigger", "removes the append-only guard (0013)"},
		{"delete from", "deletes timeline rows — rule 7, forbidden #5"},
		{"update nhat_ky_phan_anh", "edits timeline rows — rule 7, forbidden #5"},
		{"insert into", "writes rows — schema only"},
		{"nhat_ky_phan_anh_phan_cong_du_truong", "0013's assignment binding is not this file's"},
		{"nhat_ky_phan_anh_noi_dung_hop_le", "0013's note binding is not this file's"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0030 contains %q — %s", c.banned, c.why)
		}
	}
}
