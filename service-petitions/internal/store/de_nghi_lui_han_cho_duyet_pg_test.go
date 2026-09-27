package store

import (
	"database/sql"
	"net/url"
	"testing"

	"github.com/vihat/vigov/core/page"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// Integration test for the extension approval-queue READ (§5.8, GET /api/v1/task-extensions).
//
// ⚠ READ THIS BEFORE BELIEVING A GREEN RUN: it SKIPS unless VIGOV_TEST_DSN is set, and the package
// still prints `ok`. The harness (TestMain, moKetNoi, xaRieng) lives in danh_muc_nhiem_vu_pg_test.go.
// Worth writing while it cannot run: the derived table and its column list are strings the fake
// driver accepts whatever they say, and only the real schema can refuse a misspelt column or a join
// that does not bind.

// themNhiemVuCoLanhDao inserts one live task naming (or not) a leader. Its own statement rather than
// themNhiemVu plus an UPDATE, so the test does not depend on which columns nhiem_vu_bat_bien freezes.
func themNhiemVuCoLanhDao(t *testing.T, db *sql.DB, xa, id, ma string, lanhDao any) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO nhiem_vu
		 (tenant_id, id, ma, loai, tieu_de, trang_thai, nguon_giao,
		  han_xu_ly, han_ban_dau, tien_do, nguoi_tao_ma, lanh_dao_giao_viec_ma)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		xa, id, ma, "theo-van-ban", "Nhiệm vụ dùng cho phép kiểm.", "dang-thuc-hien", "truc-tiep",
		mocHanGocPg, mocHanGocPg, 0, "CB-00007", lanhDao); err != nil {
		t.Fatalf("thêm nhiệm vụ %s: %v", id, err)
	}
}

// TestPgHangChoLuiHanChiDangChoCuaXaVaNhiemVuConSong — pending only, this commune only (on BOTH
// tables), soft-deleted requests and soft-deleted tasks excluded, oldest first across a cursor, and
// the leader filter.
func TestPgHangChoLuiHanChiDangChoCuaXaVaNhiemVuConSong(t *testing.T) {
	db := moKetNoi(t)
	xa, xaKhac := xaRieng(t)
	danhMucChoXa(t, db, xa)
	danhMucChoXa(t, db, xaKhac)

	themNhiemVuCoLanhDao(t, db, xa, "nv-q1", "NV81", "CB-00123")
	themNhiemVuCoLanhDao(t, db, xa, "nv-q2", "NV82", nil)
	themNhiemVuCoLanhDao(t, db, xa, "nv-q3", "NV83", "CB-00123")
	themNhiemVuCoLanhDao(t, db, xa, "nv-q4", "NV84", "CB-00123")
	// THE SAME INTERNAL TASK ID in another commune, with a pending request: a join on id alone would
	// attach it (rule 1).
	themNhiemVuCoLanhDao(t, db, xaKhac, "nv-q1", "NV81", "CB-00123")

	for _, dn := range []struct{ xa, id, nv string }{
		{xa, "dn-q1", "nv-q1"}, {xa, "dn-q2", "nv-q2"}, {xa, "dn-q3", "nv-q3"},
		{xa, "dn-q5", "nv-q4"}, {xaKhac, "dn-qk", "nv-q1"},
	} {
		if err := themDeNghi(db, dn.xa, dn.id, dn.nv, "cho-duyet", nil, nil); err != nil {
			t.Fatalf("gửi đề nghị %s: %v", dn.id, err)
		}
	}
	// dn-q5: soft-deleted request → out. Then a DECIDED request on the same task → out.
	if _, err := db.Exec(`UPDATE de_nghi_lui_han SET deleted_at = now(), deleted_by = $3, delete_reason = $4
		WHERE tenant_id = $1 AND id = $2`, xa, "dn-q5", "CB-00007", "gửi nhầm"); err != nil {
		t.Fatalf("xoá mềm đề nghị: %v", err)
	}
	if err := themDeNghi(db, xa, "dn-q4", "nv-q4", "tu-choi", "CB-00123", mocHanGocPg); err != nil {
		t.Fatalf("đề nghị đã quyết: %v", err)
	}
	// nv-q3: soft-deleted TASK → its pending request leaves the queue.
	if _, err := db.Exec(`UPDATE nhiem_vu SET deleted_at = now(), deleted_by = $3, delete_reason = $4
		WHERE tenant_id = $1 AND id = $2`, xa, "nv-q3", "CB-00007", "trùng nhiệm vụ"); err != nil {
		t.Fatalf("xoá mềm nhiệm vụ: %v", err)
	}

	s := NewDeNghiLuiHanStore(pkgstore.New(db))
	ctx := ctxXa(tenant.ID(xa))

	yc, err := page.Parse(url.Values{"limit": {"1"}}, SapXepDeNghiChoDuyet)
	if err != nil {
		t.Fatalf("page.Parse: %v", err)
	}
	trang1, err := s.ChoDuyet(ctx, LocDeNghiChoDuyet{}, yc)
	if err != nil {
		t.Fatalf("trang 1: %v", err)
	}
	yc2, err := page.Parse(url.Values{"limit": {"1"}, "cursor": {trang1.NextCursor}}, SapXepDeNghiChoDuyet)
	if err != nil {
		t.Fatalf("page.Parse trang 2: %v", err)
	}
	trang2, err := s.ChoDuyet(ctx, LocDeNghiChoDuyet{}, yc2)
	if err != nil {
		t.Fatalf("trang 2: %v", err)
	}
	var ids []string
	for _, d := range append(trang1.Items, trang2.Items...) {
		ids = append(ids, d.DeNghi.ID)
	}
	if len(ids) != 2 || ids[0] != "dn-q1" || ids[1] != "dn-q2" || trang2.HasMore {
		t.Fatalf("hàng chờ = %v (còn trang: %v), muốn [dn-q1 dn-q2] cũ nhất trước", ids, trang2.HasMore)
	}
	if a := trang1.Items[0]; a.NhiemVuMa != "NV81" || a.LanhDaoGiaoViecMa != "CB-00123" ||
		!a.HanXuLyHienTai.Equal(mocHanGocPg) || !a.DeNghi.HanMoi.Equal(mocHanPg) {
		t.Errorf("dòng 1 = %+v", a)
	}

	chiToi, err := s.ChoDuyet(ctx, LocDeNghiChoDuyet{LanhDaoGiaoViecMa: "CB-00123"}, yc)
	if err != nil {
		t.Fatalf("lọc lãnh đạo: %v", err)
	}
	if len(chiToi.Items) != 1 || chiToi.Items[0].DeNghi.ID != "dn-q1" || chiToi.HasMore {
		t.Errorf("lọc lãnh đạo = %+v, muốn đúng [dn-q1]", chiToi.Items)
	}
}
