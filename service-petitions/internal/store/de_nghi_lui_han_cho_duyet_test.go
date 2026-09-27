package store

import (
	"database/sql/driver"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/page"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The extension approval-queue READ, over the fake driver: the ONE statement it builds and the row it
// scans. What PostgreSQL does with it (real columns, real join, the commune boundary on data) is
// de_nghi_lui_han_cho_duyet_pg_test.go, which SKIPS without VIGOV_TEST_DSN.

var (
	mocDeNghiCho1 = time.Date(2026, 9, 24, 2, 0, 0, 0, time.UTC)
	mocDeNghiCho2 = time.Date(2026, 9, 25, 3, 0, 0, 0, time.UTC)
	// The two TIMESTAMPTZs a positional swap would confuse are a WEEK apart, so a swap is wrong data.
	mocHanMoiCho = time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)
	mocHanCuCho  = time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
)

func hangDeNghiCho() []map[string]driver.Value {
	return []map[string]driver.Value{
		{
			"id": "dn-001", "nhiem_vu_id": "nv-001", "nguoi_de_nghi_ma": "CB-00311",
			"han_moi": mocHanMoiCho, "ly_do": "Chờ số liệu của thôn.", "thoi_diem": mocDeNghiCho1,
			"nhiem_vu_ma": "NV19", "nhiem_vu_tieu_de": "Rà soát tuyến đường",
			"nhiem_vu_han_xu_ly": mocHanCuCho, "lanh_dao_giao_viec_ma": "CB-00123",
		},
		{
			"id": "dn-002", "nhiem_vu_id": "nv-002", "nguoi_de_nghi_ma": "CB-00412",
			"han_moi": mocHanMoiCho, "ly_do": "Thiếu nhân lực.", "thoi_diem": mocDeNghiCho2,
			"nhiem_vu_ma": "NV20", "nhiem_vu_tieu_de": "Tổng hợp báo cáo",
			// NULL leader and NULL deadline must scan to "" and the zero time — never an error.
			"nhiem_vu_han_xu_ly": nil, "lanh_dao_giao_viec_ma": nil,
		},
	}
}

func parseCho(t *testing.T, q url.Values) page.Request {
	t.Helper()
	yc, err := page.Parse(q, SapXepDeNghiChoDuyet)
	if err != nil {
		t.Fatalf("page.Parse: %v", err)
	}
	return yc
}

// TestChoDuyetBuocXaCaHaiBangChiDangChoVaLoaiDaXoa — the scoping of the ONE statement: commune $1 on
// BOTH tables (the joined one in its ON clause), pending only through a bound $2, soft-deleted
// requests AND soft-deleted tasks excluded, oldest first with the id tie-break, and the row scanned
// by name.
func TestChoDuyetBuocXaCaHaiBangChiDangChoVaLoaiDaXoa(t *testing.T) {
	xa := tenant.ID("01JA" + strings.Repeat("A", 22))
	k := &khoGia{hangTheoCot: hangDeNghiCho()}
	s := NewDeNghiLuiHanStore(pkgstore.New(moKhoGia(k)))

	kq, err := s.ChoDuyet(ctxXa(xa), LocDeNghiChoDuyet{}, parseCho(t, url.Values{}))
	if err != nil {
		t.Fatalf("ChoDuyet: %v", err)
	}

	// ONE statement per page — no N+1 over the tasks.
	if len(k.lenh) != 1 {
		t.Fatalf("chạy %d câu lệnh, muốn 1", len(k.lenh))
	}
	l := k.lenh[0]
	if len(l.args) < 2 || l.args[0] != string(xa) || l.args[1] != string(domain.ChoDuyetLuiHan) {
		t.Fatalf("tham số = %v — $1 phải là xã từ ngữ cảnh, $2 là mã trạng thái chờ duyệt", l.args)
	}
	// Commune, status, limit — and nothing else when no leader filter is asked for.
	if len(l.args) != 3 {
		t.Errorf("tham số = %v, muốn đúng 3 (xã, trạng thái, limit+1)", l.args)
	}
	for _, muon := range []string{
		"FROM de_nghi_lui_han d",
		// THE JOINED TABLE IS BOUND TO THE COMMUNE IN ITS ON CLAUSE — not joined on id alone.
		"JOIN nhiem_vu n ON n.tenant_id = $1 AND n.id = d.nhiem_vu_id AND n.deleted_at IS NULL",
		"d.tenant_id = $1",
		"d.trang_thai = $2",
		"d.deleted_at IS NULL",
		"WHERE tenant_id = $1",
		"ORDER BY thoi_diem ASC, id ASC",
		"LIMIT $3",
	} {
		if !strings.Contains(l.sql, muon) {
			t.Errorf("câu lệnh thiếu %q: %s", muon, l.sql)
		}
	}
	if strings.Contains(l.sql, "lanh_dao_giao_viec_ma = $") {
		t.Errorf("không lọc lãnh đạo mà câu lệnh vẫn có điều kiện ấy: %s", l.sql)
	}
	// The status is BOUND, never a second spelling of the code in the text.
	if strings.Contains(l.sql, "'cho-duyet'") {
		t.Errorf("mã trạng thái viết cứng trong câu lệnh: %s", l.sql)
	}

	if len(kq.Items) != 2 {
		t.Fatalf("đọc %d dòng, muốn 2", len(kq.Items))
	}
	a, b := kq.Items[0], kq.Items[1]
	if a.DeNghi.ID != "dn-001" || a.DeNghi.NhiemVuID != "nv-001" || a.DeNghi.NguoiDeNghiMa != "CB-00311" ||
		!a.DeNghi.HanMoi.Equal(mocHanMoiCho) || a.DeNghi.LyDo != "Chờ số liệu của thôn." ||
		!a.DeNghi.ThoiDiem.Equal(mocDeNghiCho1) || a.DeNghi.TrangThai != domain.ChoDuyetLuiHan ||
		a.NhiemVuMa != "NV19" || a.NhiemVuTieuDe != "Rà soát tuyến đường" ||
		!a.HanXuLyHienTai.Equal(mocHanCuCho) || a.LanhDaoGiaoViecMa != "CB-00123" {
		t.Errorf("dòng 1 quét sai: %+v", a)
	}
	if b.LanhDaoGiaoViecMa != "" || !b.HanXuLyHienTai.IsZero() {
		t.Errorf("dòng 2 quét sai (NULL phải thành rỗng/zero): %+v", b)
	}
}

// TestChoDuyetLocLanhDaoLaThamSoRangBuoc — the leader filter is ONE bound placeholder after the
// status, on the joined relation's leader column.
func TestChoDuyetLocLanhDaoLaThamSoRangBuoc(t *testing.T) {
	xa := tenant.ID("01JA" + strings.Repeat("A", 22))
	k := &khoGia{hangTheoCot: hangDeNghiCho()[:1]}
	s := NewDeNghiLuiHanStore(pkgstore.New(moKhoGia(k)))

	if _, err := s.ChoDuyet(ctxXa(xa), LocDeNghiChoDuyet{LanhDaoGiaoViecMa: "CB-00123"},
		parseCho(t, url.Values{})); err != nil {
		t.Fatalf("ChoDuyet: %v", err)
	}
	l := k.lenh[0]
	if !strings.Contains(l.sql, "AND lanh_dao_giao_viec_ma = $3") || !strings.Contains(l.sql, "LIMIT $4") {
		t.Errorf("điều kiện lãnh đạo không đúng chỗ: %s", l.sql)
	}
	if len(l.args) != 4 || l.args[2] != "CB-00123" {
		t.Errorf("tham số = %v, muốn $3 = mã lãnh đạo", l.args)
	}
	if strings.Contains(l.sql, "CB-00123") {
		t.Errorf("mã cán bộ nằm trong văn bản câu lệnh thay vì tham số: %s", l.sql)
	}
}

// TestChoDuyetConTroTrangSau — one row more than the limit gives a cursor, and the cursor comes back
// as the `(thoi_diem, id) > (…)` anchor in the next statement.
func TestChoDuyetConTroTrangSau(t *testing.T) {
	xa := tenant.ID("01JA" + strings.Repeat("A", 22))
	k := &khoGia{hangTheoCot: hangDeNghiCho()}
	s := NewDeNghiLuiHanStore(pkgstore.New(moKhoGia(k)))

	trang1, err := s.ChoDuyet(ctxXa(xa), LocDeNghiChoDuyet{}, parseCho(t, url.Values{"limit": {"1"}}))
	if err != nil {
		t.Fatalf("trang 1: %v", err)
	}
	if !trang1.HasMore || trang1.NextCursor == "" || len(trang1.Items) != 1 {
		t.Fatalf("trang 1: %d dòng, has_more=%v cursor=%q", len(trang1.Items), trang1.HasMore, trang1.NextCursor)
	}

	k.lenh = nil
	if _, err := s.ChoDuyet(ctxXa(xa), LocDeNghiChoDuyet{},
		parseCho(t, url.Values{"limit": {"1"}, "cursor": {trang1.NextCursor}})); err != nil {
		t.Fatalf("trang 2: %v", err)
	}
	l := k.lenh[0]
	if !strings.Contains(l.sql, "AND (thoi_diem, id) > ($3, $4)") {
		t.Errorf("trang 2 thiếu mốc con trỏ: %s", l.sql)
	}
	if len(l.args) < 4 || l.args[3] != "dn-001" {
		t.Errorf("mốc id = %v, muốn dn-001 (dòng cuối trang 1)", l.args)
	}
}
