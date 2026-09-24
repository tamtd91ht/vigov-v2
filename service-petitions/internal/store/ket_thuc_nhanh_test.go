package store

import (
	"database/sql/driver"
	"testing"
	"time"

	"github.com/vihat/vigov/core/store"
)

// TestTheoMaTraCuuDocBaCotNhanh — the three branch columns of migration 0011 are read BY NAME from the
// store's own SELECT list, so a Scan out of step with cotPhieu shows up as wrong data, not as zeros.
func TestTheoMaTraCuuDocBaCotNhanh(t *testing.T) {
	luc := time.Date(2026, 9, 9, 10, 17, 0, 0, time.UTC)
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongPhieu(map[string]driver.Value{
		"trang_thai":           "chuyen-cap-tren",
		"ly_do_ket_thuc_nhanh": "Thuộc thẩm quyền của điện lực huyện.",
		"co_quan_nhan":         "Điện lực huyện",
		"ket_thuc_nhanh_luc":   luc,
	})}}
	s := NewPhieuPhanAnhStore(store.New(moKhoGia(k)))

	p, err := s.TheoMaTraCuu(ctxXa(xaThu), maThu)
	if err != nil {
		t.Fatalf("đọc phiếu: %v", err)
	}
	if p.LyDoKetThucNhanh != "Thuộc thẩm quyền của điện lực huyện." || p.CoQuanNhan != "Điện lực huyện" ||
		!p.KetThucNhanhLuc.Equal(luc) {
		t.Errorf("ba cột nhánh đọc sai: lý do=%q cơ quan=%q lúc=%v", p.LyDoKetThucNhanh, p.CoQuanNhan,
			p.KetThucNhanhLuc)
	}
	// The columns AROUND them are unchanged — an inserted destination would have shifted these.
	if p.SoLanMoLai != 0 || p.HienCongKhai {
		t.Errorf("cột lân cận bị lệch: so_lan_mo_lai=%d hien_cong_khai=%v", p.SoLanMoLai, p.HienCongKhai)
	}
}

// TestTheoMaTraCuuBaCotNhanhNullThanhRong — NULL on every other status, read as empty / zero.
func TestTheoMaTraCuuBaCotNhanhNullThanhRong(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongPhieu(nil)}}
	s := NewPhieuPhanAnhStore(store.New(moKhoGia(k)))

	p, err := s.TheoMaTraCuu(ctxXa(xaThu), maThu)
	if err != nil {
		t.Fatalf("đọc phiếu: %v", err)
	}
	if p.LyDoKetThucNhanh != "" || p.CoQuanNhan != "" || !p.KetThucNhanhLuc.IsZero() {
		t.Errorf("phiếu không ở nhánh mang giá trị nhánh: %q %q %v", p.LyDoKetThucNhanh, p.CoQuanNhan,
			p.KetThucNhanhLuc)
	}
}
