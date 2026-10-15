package app

import (
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-petitions/internal/domain"
	"github.com/vihat/vigov/service-petitions/migrations"
)

// TestStaffChannelsAreAllowedByTheSchema — every channel an officer may pick (domain.StaffMayRecord) is in
// the LATEST `phieu_phan_anh_kenh_hop_le`. Without it a pick the domain accepts rolls the whole intake back
// on PostgreSQL — the row, the timeline row and the trail — with every fake-store test still green.
func TestStaffChannelsAreAllowedByTheSchema(t *testing.T) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatalf("đọc migrations nhúng: %v", err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	// THE DECLARATION, not a mention: a rollback comment naming the constraint must not be read as a list.
	decl := regexp.MustCompile(`(?is)phieu_phan_anh_kenh_hop_le\s+CHECK\s*\(\s*kenh_tiep_nhan\s+IN\s*\(([^)]*)\)`)
	var list string
	for _, n := range names {
		b, err := fs.ReadFile(migrations.FS, n)
		if err != nil {
			t.Fatalf("đọc %s: %v", n, err)
		}
		if ms := decl.FindAllStringSubmatch(string(b), -1); len(ms) > 0 {
			list = ms[len(ms)-1][1]
		}
	}
	if list == "" {
		t.Fatal("không tìm thấy CHECK phieu_phan_anh_kenh_hop_le — bộ đọc đã mù")
	}
	allowed := map[string]bool{}
	for _, m := range regexp.MustCompile(`'([^']*)'`).FindAllStringSubmatch(list, -1) {
		allowed[m[1]] = true
	}
	for _, k := range []domain.KenhTiepNhan{domain.KenhCanBoNhapHo, domain.ChannelSwitchboard, domain.KenhWebXa,
		domain.KenhZaloOA, domain.KenhZaloMiniApp} {
		if !k.HopLe() {
			t.Errorf("%q không HopLe trong domain", k)
		}
		if !allowed[string(k)] {
			t.Errorf("kênh %q không có trong CHECK phieu_phan_anh_kenh_hop_le mới nhất", k)
		}
	}
}
