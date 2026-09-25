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

// The logbook READ, over the fake driver: the statement it builds and the row it scans. The
// PostgreSQL half (trigger, CHECKs, real ordering) is nhat_ky_phan_anh_pg_test.go.

func TestNhatKyCuaPhieuBuocXaPhieuVaMoiNhatTruoc(t *testing.T) {
	xa := tenant.ID("01JA" + strings.Repeat("A", 22))
	luc := time.Date(2026, 9, 26, 3, 4, 5, 0, time.UTC)
	k := &khoGia{hangTheoCot: []map[string]driver.Value{
		{
			"id": "nk-2", "phieu_phan_anh_id": "pa-001", "thoi_diem": luc, "nguoi_ma": "CB-00123",
			"hanh_vi": "phan-cong", "trang_thai_tai_thoi_diem": "da-chuyen-xu-ly",
			"bo_phan_id": "bp-001", "can_bo_xu_ly_ma": "CB-00777", "noi_dung": nil,
		},
		{
			"id": "nk-1", "phieu_phan_anh_id": "pa-001", "thoi_diem": luc, "nguoi_ma": "CB-00123",
			"hanh_vi": "ghi-chu", "trang_thai_tai_thoi_diem": "dang-phan-loai",
			"bo_phan_id": nil, "can_bo_xu_ly_ma": nil, "noi_dung": "Đã gọi tổ trưởng.",
		},
	}}
	s := NewPhieuPhanAnhStore(pkgstore.New(moKhoGia(k)))

	yc, err := page.Parse(url.Values{}, SapXepNhatKyPhieu)
	if err != nil {
		t.Fatalf("page.Parse: %v", err)
	}
	kq, err := s.NhatKyCuaPhieu(ctxXa(xa), "pa-001", yc)
	if err != nil {
		t.Fatalf("NhatKyCuaPhieu: %v", err)
	}

	if len(k.lenh) != 1 {
		t.Fatalf("chạy %d câu lệnh, muốn 1", len(k.lenh))
	}
	l := k.lenh[0]
	if l.args[0] != string(xa) || l.args[1] != "pa-001" {
		t.Errorf("tham số = %v — $1 phải là xã từ ngữ cảnh, $2 là id phiếu", l.args)
	}
	for _, muon := range []string{"FROM nhat_ky_phan_anh", "phieu_phan_anh_id = $2",
		"ORDER BY thoi_diem DESC, id DESC"} {
		if !strings.Contains(l.sql, muon) {
			t.Errorf("câu lệnh thiếu %q: %s", muon, l.sql)
		}
	}

	if len(kq.Items) != 2 {
		t.Fatalf("đọc %d dòng, muốn 2", len(kq.Items))
	}
	pc, gc := kq.Items[0], kq.Items[1]
	if pc.HanhVi != domain.NhatKyPhanCong || pc.BoPhanID != "bp-001" || pc.CanBoXuLyMa != "CB-00777" ||
		pc.NoiDung != "" || pc.TrangThai != domain.DaChuyenXuLy || !pc.ThoiDiem.Equal(luc) {
		t.Errorf("dòng phân công quét sai: %+v", pc)
	}
	if gc.HanhVi != domain.NhatKyGhiChu || gc.BoPhanID != "" || gc.CanBoXuLyMa != "" ||
		gc.NoiDung != "Đã gọi tổ trưởng." || gc.NguoiMa != "CB-00123" {
		t.Errorf("dòng ghi chú quét sai: %+v", gc)
	}
}
