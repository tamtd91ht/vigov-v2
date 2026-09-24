package domain

import (
	"errors"
	"strings"
	"testing"
)

// The pure rules of this service's two reference catalogues. No database, no HTTP.

func TestTangCuaPhuHopVoiBangBaTang(t *testing.T) {
	// THE TABLE IS THE MIGRATION'S — migrations/0005_don_vi_dan_cu_va_danh_muc.sql:79-81.
	for ten, tc := range map[string]struct {
		nguon   string
		reNhanh bool
		muon    Tang
	}{
		"xã tự thêm":          {NguonDonVi, false, TangDonVi},
		"hệ thống cấp":        {NguonHeThong, false, TangHeThong},
		"hệ thống + rẽ nhánh": {NguonHeThong, true, TangReNhanh},
		// The CHECK refuses this combination; if it ever appeared, the safe reading is the stricter.
		"đơn vị + rẽ nhánh (lược đồ cấm)": {NguonDonVi, true, TangReNhanh},
		"nguồn lạ": {"khong-biet", false, TangDonVi},
	} {
		t.Run(ten, func(t *testing.T) {
			if got := TangCua(tc.nguon, tc.reNhanh); got != tc.muon {
				t.Errorf("TangCua(%q, %v) = %d, muốn %d", tc.nguon, tc.reNhanh, got, tc.muon)
			}
		})
	}
}

func TestChoXoaMemChiTang1(t *testing.T) {
	if err := TangDonVi.ChoXoaMem(); err != nil {
		t.Errorf("tầng 1 phải xoá mềm được: %v", err)
	}
	for _, tang := range []Tang{TangHeThong, TangReNhanh} {
		if err := tang.ChoXoaMem(); !errors.Is(err, ErrKhongXoaDuocMucHeThong) {
			t.Errorf("tầng %d: lỗi = %v, muốn ErrKhongXoaDuocMucHeThong", tang, err)
		}
	}
}

func TestChoTatMoiTangTruTang3(t *testing.T) {
	for _, tang := range []Tang{TangDonVi, TangHeThong} {
		if err := tang.ChoTat(); err != nil {
			t.Errorf("tầng %d phải tắt được: %v", tang, err)
		}
	}
	if err := TangReNhanh.ChoTat(); !errors.Is(err, ErrKhongTatDuocMucReNhanh) {
		t.Errorf("tầng 3: lỗi = %v, muốn ErrKhongTatDuocMucReNhanh", err)
	}
}

// BOTH ROW TYPES READ BOTH COLUMNS. A method reading only `nguon` would report tier 2 for a row the
// source code branches on — and the screen would then offer `Tắt` on `thon`.
func TestTangCuaMotDongDocCaHaiCot(t *testing.T) {
	if got := (LoaiDonViDanCu{Nguon: NguonHeThong, MaNguonReNhanh: true}).Tang(); got != TangReNhanh {
		t.Errorf("LoaiDonViDanCu.Tang() = %d, muốn %d", got, TangReNhanh)
	}
	if got := (KhoiNhiemVu{Nguon: NguonHeThong, MaNguonReNhanh: true}).Tang(); got != TangReNhanh {
		t.Errorf("KhoiNhiemVu.Tang() = %d, muốn %d", got, TangReNhanh)
	}
	if got := (KhoiNhiemVu{Nguon: NguonHeThong}).Tang(); got != TangHeThong {
		t.Errorf("KhoiNhiemVu hệ thống = %d, muốn %d", got, TangHeThong)
	}
}

func TestChuanHoaMaDanhMucChiNhanKebabKhongDau(t *testing.T) {
	for _, hop := range []string{"thon", "to-dan-pho", "khoi-uy-ban", "a1"} {
		if got, err := ChuanHoaMaDanhMuc(hop); err != nil || got != hop {
			t.Errorf("ChuanHoaMaDanhMuc(%q) = %q, %v — phải nhận", hop, got, err)
		}
	}
	if got, err := ChuanHoaMaDanhMuc("  khu-pho  "); err != nil || got != "khu-pho" {
		t.Errorf("cắt khoảng trắng: %q, %v", got, err)
	}
	for ten, xau := range map[string]string{
		"chữ hoa":        "Khoi-Dang",
		"dấu tiếng Việt": "khối-đảng",
		"gạch dưới":      "khoi_dang",
		"khoảng trắng":   "khoi dang",
		"gạch đầu":       "-khoi",
		"gạch cuối":      "khoi-",
		"gạch đôi":       "khoi--dang",
		"rỗng":           "   ",
		"quá dài":        strings.Repeat("a", MaDanhMucToiDa+1),
	} {
		t.Run(ten, func(t *testing.T) {
			if _, err := ChuanHoaMaDanhMuc(xau); err == nil {
				t.Errorf("ChuanHoaMaDanhMuc(%q) phải từ chối", xau)
			}
		})
	}
	if _, err := ChuanHoaMaDanhMuc("Khoi-Dang"); !errors.Is(err, ErrMaDanhMucSaiDinhDang) {
		t.Errorf("lỗi = %v, muốn ErrMaDanhMucSaiDinhDang — không được tự hạ chữ", err)
	}
}

func TestChuanHoaNhanDanhMucGiuDauTiengViet(t *testing.T) {
	if got, err := ChuanHoaNhanDanhMuc("  Khối Uỷ ban  "); err != nil || got != "Khối Uỷ ban" {
		t.Errorf("ChuanHoaNhanDanhMuc = %q, %v", got, err)
	}
	if _, err := ChuanHoaNhanDanhMuc("   "); !errors.Is(err, ErrNhanDanhMucTrong) {
		t.Errorf("nhãn rỗng: lỗi = %v", err)
	}
	if _, err := ChuanHoaNhanDanhMuc(strings.Repeat("a", NhanDanhMucToiDa+1)); !errors.Is(err, ErrNhanDanhMucQuaDai) {
		t.Error("nhãn quá dài phải bị từ chối")
	}
	if _, err := ChuanHoaNhanDanhMuc("Khối\nĐảng"); err == nil {
		t.Error("nhãn chứa ký tự điều khiển phải bị từ chối")
	}
	// Counted in RUNES, not bytes.
	if _, err := ChuanHoaNhanDanhMuc(strings.Repeat("ế", NhanDanhMucToiDa)); err != nil {
		t.Errorf("nhãn %d ký tự tiếng Việt bị từ chối: %v", NhanDanhMucToiDa, err)
	}
}

func TestKiemTraThuTuDanhMucTuChoiAmVaQuaLon(t *testing.T) {
	for _, hop := range []int{0, 1, ThuTuDanhMucToiDa} {
		if err := KiemTraThuTuDanhMuc(hop); err != nil {
			t.Errorf("thứ tự %d phải hợp lệ: %v", hop, err)
		}
	}
	for _, xau := range []int{-1, ThuTuDanhMucToiDa + 1} {
		if err := KiemTraThuTuDanhMuc(xau); !errors.Is(err, ErrThuTuDanhMucNgoaiKhoang) {
			t.Errorf("thứ tự %d: lỗi = %v", xau, err)
		}
	}
}

func TestChuanHoaLyDoXoaDanhMucBatBuoc(t *testing.T) {
	if got, err := ChuanHoaLyDoXoaDanhMuc("  nhập nhầm "); err != nil || got != "nhập nhầm" {
		t.Errorf("= %q, %v", got, err)
	}
	if _, err := ChuanHoaLyDoXoaDanhMuc(" "); !errors.Is(err, ErrThieuLyDoXoaDanhMuc) {
		t.Errorf("lỗi = %v, muốn ErrThieuLyDoXoaDanhMuc", err)
	}
	if _, err := ChuanHoaLyDoXoaDanhMuc(strings.Repeat("a", LyDoXoaDanhMucToiDa+1)); !errors.Is(err, ErrLyDoXoaDanhMucQuaDai) {
		t.Error("lý do quá dài phải bị từ chối")
	}
	// NOT the staff directory's sentinel — that one's message names the directory.
	if _, err := ChuanHoaLyDoXoaDanhMuc(""); errors.Is(err, ErrThieuLyDoXoa) {
		t.Error("danh mục dùng nhầm lỗi của danh bạ cán bộ")
	}
}
