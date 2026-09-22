package domain

import (
	"errors"
	"strings"
	"testing"
)

// The pure rules of a reference catalogue. No database, no HTTP — this package imports nothing but
// the standard library (doc.go), which is exactly what makes these testable at all.

func TestTangCuaPhuHopVoiBangBaTang(t *testing.T) {
	// THE TABLE IS COPIED FROM THE MIGRATION, not restated from memory —
	// migrations/0003_danh_muc_loai_tai_nguyen_ban_do.sql:75-77. A tier read wrongly here offers a button the
	// database will refuse, or hides one that would have worked.
	for ten, tc := range map[string]struct {
		nguon   string
		reNhanh bool
		muon    Tang
	}{
		"xã tự thêm":          {NguonDonVi, false, TangDonVi},
		"hệ thống cấp":        {NguonHeThong, false, TangHeThong},
		"hệ thống + rẽ nhánh": {NguonHeThong, true, TangReNhanh},
		// The schema's CHECK refuses this combination outright (`..._re_nhanh_thi_he_thong`), so it
		// cannot come out of the database. It is asserted anyway, and asserted as the STRICTER
		// reading: if a row ever held it, the safe answer is "do not touch it".
		"đơn vị + rẽ nhánh (lược đồ cấm)": {NguonDonVi, true, TangReNhanh},
		// An unknown `nguon` also cannot come out of the database, and the safe reading here is the
		// OPPOSITE one: treat it as the commune's own row rather than as untouchable, because a row
		// nobody can reach is a row an administrator cannot clean up.
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

func TestChuanHoaMaChiNhanKebabKhongDau(t *testing.T) {
	// ADR 0011: catalogue VALUES stay Vietnamese without diacritics while the surrounding contract
	// is English. The code goes straight into business records as a value nothing ever rewrites, so
	// this is the last moment it can be refused.
	for _, hop := range []string{"cong-van", "quyet-dinh", "bao-cao-2026", "a1"} {
		if got, err := ChuanHoaMa(hop); err != nil || got != hop {
			t.Errorf("ChuanHoaMa(%q) = %q, %v — phải nhận", hop, got, err)
		}
	}
	// Whitespace is trimmed rather than refused: it is invisible on a form, and a code that differs
	// from another only by a trailing space is the kind of duplicate nobody can see.
	if got, err := ChuanHoaMa("  cong-van  "); err != nil || got != "cong-van" {
		t.Errorf("ChuanHoaMa cắt khoảng trắng: %q, %v", got, err)
	}
	for ten, xau := range map[string]string{
		"chữ hoa":        "Cong-Van",
		"dấu tiếng Việt": "công-văn",
		"gạch dưới":      "cong_van",
		"khoảng trắng":   "cong van",
		"gạch đầu":       "-cong-van",
		"gạch cuối":      "cong-van-",
		"gạch đôi":       "cong--van",
		"rỗng":           "   ",
		"quá dài":        strings.Repeat("a", MaToiDa+1),
	} {
		t.Run(ten, func(t *testing.T) {
			if _, err := ChuanHoaMa(xau); err == nil {
				t.Errorf("ChuanHoaMa(%q) phải từ chối", xau)
			}
		})
	}
}

func TestChuanHoaMaKhongTuHaChu(t *testing.T) {
	// NO CASE FOLDING, and it is asserted rather than left to the refusal above: lower-casing what
	// the person typed means the code stored is not the code they saw, on the one column rule 7
	// never lets us correct afterwards.
	if _, err := ChuanHoaMa("Cong-Van"); !errors.Is(err, ErrMaSaiDinhDang) {
		t.Errorf("lỗi = %v, muốn ErrMaSaiDinhDang — không được tự hạ chữ", err)
	}
}

func TestChuanHoaNhanGiuDauTiengViet(t *testing.T) {
	// The label is a sentence a person reads, so it carries diacritics — the opposite rule to the
	// code. It is also the ONE field every tier may change.
	if got, err := ChuanHoaNhan("  Quyết định  "); err != nil || got != "Quyết định" {
		t.Errorf("ChuanHoaNhan = %q, %v", got, err)
	}
	if _, err := ChuanHoaNhan("   "); !errors.Is(err, ErrNhanTrong) {
		t.Errorf("nhãn rỗng: lỗi = %v, muốn ErrNhanTrong", err)
	}
	if _, err := ChuanHoaNhan(strings.Repeat("a", NhanToiDa+1)); !errors.Is(err, ErrNhanQuaDai) {
		t.Error("nhãn quá dài phải bị từ chối")
	}
	// A control character corrupts a screen and a log line alike.
	if _, err := ChuanHoaNhan("Công\nvăn"); err == nil {
		t.Error("nhãn chứa ký tự điều khiển phải bị từ chối")
	}
	// Counted in RUNES, not bytes: "Quyết định" is 10 characters and 14 bytes, and a byte bound
	// would cut a Vietnamese label off at two thirds of the length an English one gets.
	if _, err := ChuanHoaNhan(strings.Repeat("ế", NhanToiDa)); err != nil {
		t.Errorf("nhãn %d ký tự tiếng Việt bị từ chối — giới hạn đang đếm byte: %v", NhanToiDa, err)
	}
}

func TestKiemTraThuTuTuChoiAmVaQuaLon(t *testing.T) {
	// NEGATIVE IS REFUSED, NOT CLAMPED. A client sending -1 to mean "first" works until a second
	// client sends -2, and the order a commune arranged its own catalogue in then depends on who
	// edited last.
	for _, hop := range []int{0, 1, ThuTuToiDa} {
		if err := KiemTraThuTu(hop); err != nil {
			t.Errorf("thứ tự %d phải hợp lệ: %v", hop, err)
		}
	}
	for _, xau := range []int{-1, ThuTuToiDa + 1} {
		if err := KiemTraThuTu(xau); !errors.Is(err, ErrThuTuNgoaiKhoang) {
			t.Errorf("thứ tự %d: lỗi = %v, muốn ErrThuTuNgoaiKhoang", xau, err)
		}
	}
}

func TestChuanHoaLyDoXoaBatBuoc(t *testing.T) {
	// Rule 7, invariant 1 names `delete_reason` beside `deleted_at` and `deleted_by`. A row that
	// vanished from every screen with no reason attached is a row nobody can explain — and the row
	// is still there, so the question will be asked.
	if got, err := ChuanHoaLyDoXoa("  gộp vào loại khác "); err != nil || got != "gộp vào loại khác" {
		t.Errorf("ChuanHoaLyDoXoa = %q, %v", got, err)
	}
	if _, err := ChuanHoaLyDoXoa(" "); !errors.Is(err, ErrThieuLyDoXoa) {
		t.Errorf("lỗi = %v, muốn ErrThieuLyDoXoa", err)
	}
	if _, err := ChuanHoaLyDoXoa(strings.Repeat("a", LyDoXoaToiDa+1)); !errors.Is(err, ErrLyDoXoaQuaDai) {
		t.Error("lý do quá dài phải bị từ chối")
	}
}

func TestTangCuaMotDongDocCaHaiCot(t *testing.T) {
	// The method on the row, rather than the free function, because that is what the HTTP layer
	// calls. A method reading only `nguon` would report tier 2 for a row the source code branches
	// on — and the screen would then offer `Tắt` on the one row that must never be disabled.
	l := LoaiTaiNguyenBanDo{Nguon: NguonHeThong, MaNguonReNhanh: true}
	if l.Tang() != TangReNhanh {
		t.Errorf("Tang() = %d, muốn %d", l.Tang(), TangReNhanh)
	}
}
