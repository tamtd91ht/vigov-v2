package store

import (
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/page"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// Integration test for `soon=true` and `scope=related` against the real schema.
//
// ⚠ SKIPS unless VIGOV_TEST_DSN is set (see nhiem_vu_van_ban_pg_test.go's header). What only
// PostgreSQL can show: the `id IN (SELECT … nhat_ky_nhiem_vu …)` clause binds to the task relation
// under BOTH shapes DanhSach reads (the plain table and the due-sort derived table), and the window
// predicates are valid over TIMESTAMPTZ.
func TestPgTaskListSoonAndRelated(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	danhMucChoXa(t, db, xa)

	now := time.Now().UTC().Truncate(time.Second)
	in := now.Add(3 * time.Hour)
	out := now.Add(48 * time.Hour)
	for _, r := range []struct {
		id, ma string
		due    time.Time
	}{{"nv-sr-1", "NV71", in}, {"nv-sr-2", "NV72", out}, {"nv-sr-3", "NV73", in}} {
		if err := themNhiemVu(db, xa, r.id, r.ma, "theo-van-ban", "dang-thuc-hien", r.due, r.due, nil); err != nil {
			t.Fatalf("thêm %s: %v", r.id, err)
		}
	}
	// NV72 is related through the TIMELINE only; NV73 through its unit only.
	if _, err := db.Exec(`INSERT INTO nhat_ky_nhiem_vu
		(tenant_id, id, nhiem_vu_id, thoi_diem, nguoi_ma, trang_thai_tai_thoi_diem, noi_dung)
		VALUES ($1, 'nk-sr-1', 'nv-sr-2', now(), 'CB-SR-001', 'dang-thuc-hien', 'Đã liên hệ')`, xa); err != nil {
		t.Fatalf("thêm nhật ký: %v", err)
	}
	if _, err := db.Exec(`UPDATE nhiem_vu SET bo_phan_id = 'bp-sr' WHERE tenant_id = $1 AND id = 'nv-sr-3'`, xa); err != nil {
		t.Fatalf("gán bộ phận: %v", err)
	}

	s := NewNhiemVuStore(pkgstore.New(db))
	ctx := ctxXa(tenant.ID(xa))
	codes := func(loc LocNhiemVu, sortParam string) string {
		v := url.Values{}
		if sortParam != "" {
			v.Set("sort", sortParam)
		}
		yc, err := page.Parse(v, SapXepNhiemVu)
		if err != nil {
			t.Fatalf("page.Parse: %v", err)
		}
		kq, err := s.DanhSach(ctx, loc, yc)
		if err != nil {
			t.Fatalf("DanhSach: %v", err)
		}
		var out []string
		for _, n := range kq.Items {
			out = append(out, n.Ma)
		}
		sort.Strings(out)
		return strings.Join(out, ",")
	}

	soon := LocNhiemVu{DueSoonFrom: now, DueSoonUntil: now.Add(24 * time.Hour)}
	if got := codes(soon, ""); got != "NV71,NV73" {
		t.Errorf("sắp đến hạn = %q, muốn NV71,NV73", got)
	}
	related := LocNhiemVu{Related: &TaskRelatedScope{StaffCode: "CB-SR-001", OrgUnits: []string{"bp-sr"}}}
	for _, srt := range []string{"", "due_at"} {
		if got := codes(related, srt); got != "NV72,NV73" {
			t.Errorf("liên quan (sort=%q) = %q, muốn NV72,NV73", srt, got)
		}
	}
}
