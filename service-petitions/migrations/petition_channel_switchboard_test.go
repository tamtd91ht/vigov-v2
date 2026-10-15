package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0045 (channel `tong-dai`, owner decision 15/10/2026). It reads the SQL
// this binary embeds; it does not prove PostgreSQL enforces any of it (tools/schema-smoke).

const file0045 = "0045_petition_channel_switchboard.sql"

// TestMigration0045WidensTheChannelListOnly — 0004's four channels plus `tong-dai`, nothing dropped.
//
// THE MUTATIONS THAT MUST TURN THIS RED: an existing channel dropped (every row holding it fails ADD
// CONSTRAINT); `tong-dai` missing or misspelled (every staff intake picking it rolls back); any extra code.
func TestMigration0045WidensTheChannelListOnly(t *testing.T) {
	prev := danhSachMaTrongCheck(t, maChay(t, "0004_phieu_phan_anh.sql"), "phieu_phan_anh_kenh_hop_le", "kenh_tiep_nhan")
	got := danhSachMaTrongCheck(t, maChay(t, file0045), "phieu_phan_anh_kenh_hop_le", "kenh_tiep_nhan")
	if len(prev) != 4 {
		t.Fatalf("0004 declares %d channels, want 4 — the parser went blind or 0004 changed: %v", len(prev), prev)
	}
	want := map[string]bool{"tong-dai": true}
	for _, c := range prev {
		want[c] = true
	}
	if len(got) != len(want) {
		t.Errorf("0045 declares %v, want 0004's four plus tong-dai", got)
	}
	for _, c := range got {
		if !want[c] {
			t.Errorf("0045 declares unexpected channel %q", c)
		}
		delete(want, c)
	}
	for c := range want {
		t.Errorf("0045 lacks channel %q", c)
	}
}

// TestMigration0045StaffShapeCoversTheSwitchboard — the two staff-shape CHECKs are re-declared under their
// own names for BOTH staff-only channels, and NOT for `web-xa` / `zalo-oa` (the file header says why).
func TestMigration0045StaffShapeCoversTheSwitchboard(t *testing.T) {
	sql := maChay(t, file0045)
	for _, c := range []struct{ want, why string }{
		{"alter table phieu_phan_anh drop constraint if exists phieu_phan_anh_kenh_hop_le;", "old list dropped by name"},
		{"add constraint phieu_phan_anh_nhap_ho_co_linh_vuc check (kenh_tiep_nhan not in ('can-bo-nhap-ho', 'tong-dai') or linh_vuc is not null)",
			"field at booking on both staff-only channels"},
		{"add constraint phieu_phan_anh_nhap_ho_khong_co_han_tiep_nhan check (kenh_tiep_nhan not in ('can-bo-nhap-ho', 'tong-dai') or han_tiep_nhan is null)",
			"no acknowledge clock on both staff-only channels (ADR 0028 F5)"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0045 lacks %q — %s", c.want, c.why)
		}
	}
	for _, c := range []struct{ banned, why string }{
		{"update phieu_phan_anh", "no backfill — no existing channel changes"},
		{"ho_so_luu_tru_bat_bien", "0004's archival guard (channel immutable) is untouched"},
		{"trang_thai_hop_le", "the closed status list (ADR 0027) is untouched"},
		{"not valid", "NOT VALID on a partitioned table is not portable"},
		{"drop table", "drops a table"},
		{"drop column", "drops a column"},
		{"delete from", "deletes rows"},
		{"insert into", "writes rows"},
		{"'web-xa', 'tong-dai'", "web-xa must not be bound to the staff shape"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0045 contains %q — %s", c.banned, c.why)
		}
	}
	if n := strings.Count(sql, "add constraint"); n != 3 {
		t.Errorf("0045 adds %d constraints, want 3", n)
	}
}
