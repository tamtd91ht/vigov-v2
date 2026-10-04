package store

import (
	"database/sql"
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/page"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// Integration tests for the Sổ tay lãnh đạo's filters (ADR 0071) against the real schema.
//
// ⚠ SKIPS unless VIGOV_TEST_DSN is set, and the package still prints `ok`. Harness: TestMain,
// moKetNoi, xaRieng in danh_muc_nhiem_vu_pg_test.go. What only PostgreSQL can show: the OR over two
// nullable columns, `<> 'hoan-thanh'` over the real status column, and that another commune's row
// with the SAME staff code never reaches this commune's page, count or badge.

// insertNotebookTask books one live task with explicit creator, assigner and assignee. Its own
// statement rather than themNhiemVu plus an UPDATE, for the reason themNhiemVuCoLanhDao gives.
func insertNotebookTask(t *testing.T, db *sql.DB, commune, id, code, status, creator string,
	assigner, assignee any) {
	t.Helper()
	var done any
	if status == string(domain.HoanThanh) {
		done = mocXongPg // nhiem_vu's CHECK ties ngay_hoan_thanh to `hoan-thanh`
	}
	if _, err := db.Exec(
		`INSERT INTO nhiem_vu
		 (tenant_id, id, ma, loai, tieu_de, trang_thai, nguon_giao,
		  han_xu_ly, han_ban_dau, ngay_hoan_thanh, tien_do, nguoi_tao_ma, lanh_dao_giao_viec_ma, nguoi_thuc_hien_ma)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		commune, id, code, "theo-van-ban", "Nhiệm vụ dùng cho phép kiểm.", status, "truc-tiep",
		mocHanPg, mocHanGocPg, done, 0, creator, assigner, assignee); err != nil {
		t.Fatalf("thêm nhiệm vụ %s: %v", id, err)
	}
}

func TestPgLeaderNotebookAssignedByMe(t *testing.T) {
	db := moKetNoi(t)
	commune, other := xaRieng(t)
	danhMucChoXa(t, db, commune)
	danhMucChoXa(t, db, other)

	const me = "CB-LN-001"
	insertNotebookTask(t, db, commune, "nv-ln-1", "NV91", "dang-thuc-hien", me, nil, "CB-LN-777")     // I created it
	insertNotebookTask(t, db, commune, "nv-ln-2", "NV92", "tam-dung", "CB-LN-500", me, "CB-LN-777")   // I am its leader; suspended
	insertNotebookTask(t, db, commune, "nv-ln-3", "NV93", "hoan-thanh", me, nil, nil)                 // mine, but finished
	insertNotebookTask(t, db, commune, "nv-ln-4", "NV94", "moi-giao", "CB-LN-500", nil, me)           // I only HOLD it
	insertNotebookTask(t, db, commune, "nv-ln-5", "NV95", "cho-duyet", "CB-LN-500", "CB-LN-600", nil) // nobody's mine
	// THE OTHER COMMUNE: same staff code as creator — must never surface (rule 1).
	insertNotebookTask(t, db, other, "nv-ln-k", "NV91", "dang-thuc-hien", me, me, nil)

	s := NewNhiemVuStore(pkgstore.New(db))
	ctx := ctxXa(tenant.ID(commune))
	codes := func(loc LocNhiemVu, sortParam string) string {
		v := url.Values{"limit": {"100"}}
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
	total := func(loc LocNhiemVu) int {
		c, err := s.CountByStatus(ctx, loc)
		if err != nil {
			t.Fatalf("CountByStatus: %v", err)
		}
		n := 0
		for _, v := range c {
			n += v
		}
		return n
	}

	assigned := LocNhiemVu{AssignedByStaffCode: me}
	for _, srt := range []string{"", "due_at"} {
		if got := codes(assigned, srt); got != "NV91,NV92,NV93" {
			t.Errorf("tôi đã giao (sort=%q) = %q, muốn NV91,NV92,NV93", srt, got)
		}
	}
	column := LocNhiemVu{AssignedByStaffCode: me, Incomplete: true}
	if got := codes(column, ""); got != "NV91,NV92" {
		t.Errorf("cột Việc tôi đã giao = %q, muốn NV91,NV92 (tạm dừng vẫn hiện, hoàn thành thì không)", got)
	}
	// LIST AND COUNT AGREE, for the column and for the scope alone.
	if n := total(column); n != 2 {
		t.Errorf("đếm cột = %d, muốn 2 — bằng số dòng danh sách", n)
	}
	if n := total(assigned); n != 3 {
		t.Errorf("đếm phạm vi = %d, muốn 3", n)
	}
	if got := codes(LocNhiemVu{Incomplete: true}, ""); got != "NV91,NV92,NV94,NV95" {
		t.Errorf("chưa hoàn thành = %q, muốn mọi việc trừ NV93", got)
	}
}

// TestPgPendingExtensionCountEqualsQueue — the badge equals the queue length for the same filter, and
// another commune's request on a task naming the same leader is in neither.
func TestPgPendingExtensionCountEqualsQueue(t *testing.T) {
	db := moKetNoi(t)
	commune, other := xaRieng(t)
	danhMucChoXa(t, db, commune)
	danhMucChoXa(t, db, other)

	themNhiemVuCoLanhDao(t, db, commune, "nv-ec-1", "NV96", "CB-00123")
	themNhiemVuCoLanhDao(t, db, commune, "nv-ec-2", "NV97", nil)
	themNhiemVuCoLanhDao(t, db, other, "nv-ec-1", "NV96", "CB-00123")
	for _, dn := range []struct{ commune, id, task string }{
		{commune, "dn-ec-1", "nv-ec-1"}, {commune, "dn-ec-2", "nv-ec-2"}, {other, "dn-ec-k", "nv-ec-1"},
	} {
		if err := themDeNghi(db, dn.commune, dn.id, dn.task, "cho-duyet", nil, nil); err != nil {
			t.Fatalf("gửi đề nghị %s: %v", dn.id, err)
		}
	}

	s := NewDeNghiLuiHanStore(pkgstore.New(db))
	ctx := ctxXa(tenant.ID(commune))
	yc, err := page.Parse(url.Values{"limit": {"100"}}, SapXepDeNghiChoDuyet)
	if err != nil {
		t.Fatalf("page.Parse: %v", err)
	}
	for name, loc := range map[string]LocDeNghiChoDuyet{
		"cả xã":       {},
		"approver=me": {LanhDaoGiaoViecMa: "CB-00123"},
	} {
		kq, err := s.ChoDuyet(ctx, loc, yc)
		if err != nil {
			t.Fatalf("%s: ChoDuyet: %v", name, err)
		}
		n, err := s.CountPending(ctx, loc)
		if err != nil {
			t.Fatalf("%s: CountPending: %v", name, err)
		}
		if n != len(kq.Items) {
			t.Errorf("%s: đếm = %d, hàng chờ có %d dòng", name, n, len(kq.Items))
		}
	}
	if n, _ := s.CountPending(ctx, LocDeNghiChoDuyet{LanhDaoGiaoViecMa: "CB-00123"}); n != 1 {
		t.Errorf("approver=me đếm = %d, muốn 1 (đề nghị của xã khác không được tính)", n)
	}
}
