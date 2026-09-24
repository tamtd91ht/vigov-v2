package domain

import (
	"errors"
	"strings"
	"testing"
)

// TestKetThucNhanhDuocChiTuDangPhanLoai — the branches leave from `dang-phan-loai` and from nowhere
// else, and only to one of the two branch statuses.
func TestKetThucNhanhDuocChiTuDangPhanLoai(t *testing.T) {
	for tu := range chuyenDuocSang {
		for _, nhanh := range []TrangThai{KhongTiepNhan, ChuyenCapTren} {
			err := KetThucNhanhDuoc(tu, nhanh)
			if tu == DangPhanLoai && err != nil {
				t.Errorf("%s -> %s bị từ chối: %v", tu, nhanh, err)
			}
			if tu != DangPhanLoai && !errors.Is(err, ErrKetThucNhanhSaiLuc) {
				t.Errorf("%s -> %s được phép — nhánh chỉ rời từ dang-phan-loai", tu, nhanh)
			}
		}
	}
	// A target that is not a branch is refused even from the right status: this function must not be
	// usable as a general transition check.
	for _, dich := range []TrangThai{DaChuyenXuLy, DaDong, "khong-co-that"} {
		if !errors.Is(KetThucNhanhDuoc(DangPhanLoai, dich), ErrKetThucNhanhSaiLuc) {
			t.Errorf("dang-phan-loai -> %s được coi là một nhánh kết thúc", dich)
		}
	}
}

// TestKiemLyDoDemTheoKyTu — the bounds are counted in RUNES, like the database's char_length. A byte
// count would refuse a 2000-character Vietnamese reason PostgreSQL accepts.
func TestKiemLyDoDemTheoKyTu(t *testing.T) {
	duToiDa := strings.Repeat("ồ", LyDoToiDa) // 2000 runes, 6000 bytes
	if got, err := KiemLyDoKetThucNhanh(duToiDa); err != nil || got != duToiDa {
		t.Errorf("lý do đúng %d ký tự có dấu bị từ chối: %v", LyDoToiDa, err)
	}
	if _, err := KiemLyDoKetThucNhanh(duToiDa + "ồ"); !errors.Is(err, ErrLyDoQuaDai) {
		t.Errorf("lý do %d ký tự: lỗi = %v, muốn ErrLyDoQuaDai", LyDoToiDa+1, err)
	}
	duToiThieu := strings.Repeat("ệ", LyDoToiThieu)
	if _, err := KiemLyDoKetThucNhanh(duToiThieu); err != nil {
		t.Errorf("lý do đúng %d ký tự bị từ chối: %v", LyDoToiThieu, err)
	}
	if _, err := KiemLyDoKetThucNhanh(duToiThieu[:len(duToiThieu)-len("ệ")]); !errors.Is(err, ErrLyDoQuaNgan) {
		t.Errorf("lý do %d ký tự: lỗi = %v, muốn ErrLyDoQuaNgan", LyDoToiThieu-1, err)
	}
	for _, rong := range []string{"", "   ", "\n\t\n"} {
		if _, err := KiemLyDoKetThucNhanh(rong); !errors.Is(err, ErrThieuLyDo) {
			t.Errorf("lý do %q: lỗi = %v, muốn ErrThieuLyDo", rong, err)
		}
	}
}

func TestKiemCoQuanNhan(t *testing.T) {
	if got, err := KiemCoQuanNhan("  Điện lực  "); err != nil || got != "Điện lực" {
		t.Errorf("tên ngắn hợp lệ: got=%q err=%v — tên cơ quan thật có thể rất ngắn", got, err)
	}
	if _, err := KiemCoQuanNhan(strings.Repeat("ệ", CoQuanNhanToiDa)); err != nil {
		t.Errorf("đúng %d ký tự bị từ chối: %v", CoQuanNhanToiDa, err)
	}
	if _, err := KiemCoQuanNhan(strings.Repeat("ệ", CoQuanNhanToiDa+1)); !errors.Is(err, ErrCoQuanNhanQuaDai) {
		t.Errorf("lỗi = %v, muốn ErrCoQuanNhanQuaDai", err)
	}
	if _, err := KiemCoQuanNhan(" \t"); !errors.Is(err, ErrThieuCoQuanNhan) {
		t.Errorf("lỗi = %v, muốn ErrThieuCoQuanNhan", err)
	}
}

// TestLoiKetThucNhanhPhanLoaiDung — input refusals are 400 material, the state refusal is not.
func TestLoiKetThucNhanhPhanLoaiDung(t *testing.T) {
	for _, e := range []error{ErrThieuLyDo, ErrLyDoQuaNgan, ErrLyDoQuaDai, ErrThieuCoQuanNhan,
		ErrCoQuanNhanQuaDai} {
		if !LaLoiXuLyPhanAnh(e) {
			t.Errorf("%v không được coi là lỗi đầu vào", e)
		}
	}
	if LaLoiXuLyPhanAnh(ErrKetThucNhanhSaiLuc) {
		t.Error("ErrKetThucNhanhSaiLuc bị coi là lỗi đầu vào — nó là lỗi TRẠNG THÁI (409)")
	}
}

// TestLoiNhanNhanhKhongMangLyDo — the sentence on the event is the fixed one; it cannot carry the
// reason or the body because ViecTiepTheo never reads those fields.
func TestLoiNhanNhanhKhongMangLyDo(t *testing.T) {
	p := PhieuPhanAnh{LinhVuc: "rac-thai", LyDoKetThucNhanh: "lý do nêu tên ông Bình",
		CoQuanNhan: "Toà án huyện"}
	for _, tt := range []TrangThai{KhongTiepNhan, ChuyenCapTren} {
		s := ViecTiepTheo(p, tt)
		if s == "" {
			t.Errorf("%s không báo gì cho dân", tt)
		}
		if strings.Contains(s, "Bình") || strings.Contains(s, "Toà án") {
			t.Errorf("%s: lời nhắn mang văn bản cán bộ viết: %q", tt, s)
		}
	}
}
