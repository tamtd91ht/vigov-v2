package store

import (
	"net/url"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/page"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// Integration test for `sort=priority` and `sort=title` against the real schema.
//
// ⚠ SKIPS unless VIGOV_TEST_DSN is set (see nhiem_vu_van_ban_pg_test.go's header). What only
// PostgreSQL can show: the catalogue join and its COALESCE keys, the scalar-subquery anchor of a
// KindRef sort, and that a two-row-per-page walk visits every task exactly once in both directions —
// no-priority tasks last both ways, and ties broken by id. Titles are ASCII so the expected order does
// not depend on the database collation.
func TestPgPriorityAndTitleSortWalk(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	danhMucChoXa(t, db, xa) // seeds `cao` at thu_tu 2
	themUuTien(t, db, xa, "uu-khan-"+xa[:6], "khan", "Khẩn", 1)
	themUuTien(t, db, xa, "uu-thuong-"+xa[:6], "thuong", "Thường", 3)

	rows := []struct{ id, ma, priority, title string }{
		{"nv-ps-a", "NV81", "khan", "Bravo"},
		{"nv-ps-b", "NV82", "cao", "Alpha"},
		{"nv-ps-c", "NV83", "thuong", "Charlie"},
		{"nv-ps-d", "NV84", "", "Alpha"},
		{"nv-ps-e", "NV85", "khan", "Delta"},
	}
	for _, r := range rows {
		if err := themNhiemVu(db, xa, r.id, r.ma, "theo-van-ban", "dang-thuc-hien", nil, nil, nil); err != nil {
			t.Fatalf("thêm %s: %v", r.id, err)
		}
		var pr any
		if r.priority != "" {
			pr = r.priority
		}
		if _, err := db.Exec(`UPDATE nhiem_vu SET muc_uu_tien = $3, tieu_de = $4 WHERE tenant_id = $1 AND id = $2`,
			xa, r.id, pr, r.title); err != nil {
			t.Fatalf("đặt ưu tiên/tiêu đề %s: %v", r.id, err)
		}
	}

	s := NewNhiemVuStore(pkgstore.New(db))
	ctx := ctxXa(tenant.ID(xa))
	walk := func(sortParam, order string) string {
		var got []string
		cursor := ""
		for guard := 0; guard < 10; guard++ {
			q := url.Values{"sort": {sortParam}, "order": {order}, "limit": {"2"}}
			if cursor != "" {
				q.Set("cursor", cursor)
			}
			yc, err := page.Parse(q, SapXepNhiemVu)
			if err != nil {
				t.Fatalf("page.Parse: %v", err)
			}
			kq, err := s.DanhSach(ctx, LocNhiemVu{}, yc)
			if err != nil {
				t.Fatalf("DanhSach %s %s: %v", sortParam, order, err)
			}
			for _, n := range kq.Items {
				got = append(got, n.Ma)
			}
			if !kq.HasMore {
				break
			}
			cursor = kq.NextCursor
		}
		return strings.Join(got, ",")
	}

	for _, tc := range []struct{ sort, order, want string }{
		{"priority", "asc", "NV81,NV85,NV82,NV83,NV84"},
		{"priority", "desc", "NV83,NV82,NV85,NV81,NV84"},
		{"title", "asc", "NV82,NV84,NV81,NV83,NV85"},
		{"title", "desc", "NV85,NV83,NV81,NV84,NV82"},
	} {
		if got := walk(tc.sort, tc.order); got != tc.want {
			t.Errorf("sort=%s order=%s: %s, muốn %s", tc.sort, tc.order, got, tc.want)
		}
	}
}

// pgSortWalk pages the register two rows at a time under one sort and returns the codes in order.
func pgSortWalk(t *testing.T, s *NhiemVuStore, xa, sortParam, order string, loc LocNhiemVu) string {
	t.Helper()
	ctx := ctxXa(tenant.ID(xa))
	var got []string
	cursor := ""
	for guard := 0; guard < 10; guard++ {
		q := url.Values{"sort": {sortParam}, "order": {order}, "limit": {"2"}}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		yc, err := page.Parse(q, SapXepNhiemVu)
		if err != nil {
			t.Fatalf("page.Parse: %v", err)
		}
		kq, err := s.DanhSach(ctx, loc, yc)
		if err != nil {
			t.Fatalf("DanhSach %s %s: %v", sortParam, order, err)
		}
		for _, n := range kq.Items {
			got = append(got, n.Ma)
		}
		if !kq.HasMore {
			break
		}
		cursor = kq.NextCursor
	}
	return strings.Join(got, ",")
}

// TestPgStatusSortWalk — `sort=status` (ADR 0082) against the real schema: the commune's own order
// (an override moves `tam-dung` to position 1, TYING with `moi-giao`'s default 1, which has no row),
// the tie broken by the DEFAULT position (moi-giao 1 before tam-dung 6), equal statuses by id, and a
// two-row-per-page walk that visits every task exactly once both ways. Alphabetical code order would
// be cho-duyet, dang-thuc-hien, moi-giao, tam-dung — asserted to be NOT what comes back.
func TestPgStatusSortWalk(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	danhMucChoXa(t, db, xa)
	if err := themNhanTrangThai(db, xa, "tam-dung", "Tạm dừng", 1, "CB-00123"); err != nil {
		t.Fatalf("ghi đè thứ tự tam-dung: %v", err)
	}

	for _, r := range []struct{ id, ma, status string }{
		{"nv-ss-a", "NV91", "dang-thuc-hien"}, // 3*16+3
		{"nv-ss-b", "NV92", "tam-dung"},       // 1*16+6 (override)
		{"nv-ss-c", "NV93", "moi-giao"},       // 1*16+1
		{"nv-ss-d", "NV94", "cho-duyet"},      // 4*16+4
		{"nv-ss-e", "NV95", "moi-giao"},       // 1*16+1, tie with NV93 → id
	} {
		if err := themNhiemVu(db, xa, r.id, r.ma, "theo-van-ban", r.status, nil, nil, nil); err != nil {
			t.Fatalf("thêm %s: %v", r.id, err)
		}
	}

	s := NewNhiemVuStore(pkgstore.New(db))
	for _, tc := range []struct{ order, want string }{
		{"asc", "NV93,NV95,NV92,NV91,NV94"},
		{"desc", "NV94,NV91,NV92,NV95,NV93"},
	} {
		if got := pgSortWalk(t, s, xa, "status", tc.order, LocNhiemVu{}); got != tc.want {
			t.Errorf("sort=status order=%s: %s, muốn %s", tc.order, got, tc.want)
		}
	}
	// The same key under `roots=true` — every task above is a root, so the order is unchanged.
	if got := pgSortWalk(t, s, xa, "status", "asc", LocNhiemVu{Roots: true}); got != "NV93,NV95,NV92,NV91,NV94" {
		t.Errorf("sort=status roots=true: %s", got)
	}
}
