package domain

import (
	"errors"
	"strings"
	"testing"
)

// The note bound is counted in CHARACTERS, as migration 0013's char_length counts: a Vietnamese note
// of 2000 characters is 4000+ bytes and must still be accepted.
func TestGhiChuDemTheoKyTuKhongTheoByte(t *testing.T) {
	vuaDu := strings.Repeat("ệ", GhiChuToiDa)
	if s, err := KiemGhiChuTuyChon(vuaDu); err != nil || s != vuaDu {
		t.Fatalf("ghi chú đúng %d ký tự bị từ chối: %v", GhiChuToiDa, err)
	}
	if _, err := KiemGhiChuTuyChon(vuaDu + "ệ"); !errors.Is(err, ErrGhiChuQuaDai) {
		t.Errorf("ghi chú %d ký tự lọt qua, lỗi = %v", GhiChuToiDa+1, err)
	}
}

// Blank after trim is ABSENT on the optional form — migration 0013 refuses an empty string on a
// non-note row —
// and a REFUSAL on the mandatory form.
func TestGhiChuRongSauKhiCatLaVang(t *testing.T) {
	if s, err := KiemGhiChuTuyChon(" \n\t "); err != nil || s != "" {
		t.Errorf("ghi chú toàn khoảng trắng = %q, %v; muốn \"\" và không lỗi", s, err)
	}
	if _, err := KiemGhiChu(" \n\t "); !errors.Is(err, ErrThieuGhiChu) {
		t.Errorf("ghi chú bắt buộc toàn khoảng trắng: lỗi = %v, muốn ErrThieuGhiChu", err)
	}
	if s, err := KiemGhiChu("  Đã gọi tổ trưởng.  "); err != nil || s != "Đã gọi tổ trưởng." {
		t.Errorf("ghi chú hợp lệ = %q, %v", s, err)
	}
}

// The refusal never echoes the note (rule 3, forbidden #3): it may hold a citizen's number.
func TestLoiGhiChuKhongLapLaiNoiDung(t *testing.T) {
	dai := "0900000000" + strings.Repeat("a", GhiChuToiDa)
	_, err := KiemGhiChuTuyChon(dai)
	if err == nil || strings.Contains(err.Error(), "0900000000") {
		t.Errorf("lỗi = %v — không được lặp lại nội dung ghi chú", err)
	}
	if !LaLoiXuLyPhanAnh(err) || !LaLoiXuLyPhanAnh(ErrThieuGhiChu) {
		t.Error("hai lỗi ghi chú phải là lỗi của người gửi (400), không phải lỗi hệ thống")
	}
}
