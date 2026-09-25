package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// Tests for the lifecycle rules of migration 0012 that are pure functions (user decisions
// 25/09/2026): the notice travels as a pair and is a calendar day, a soft delete names a reason, and
// the 400 / 409 line — every refusal of WHAT WAS SENT is an input error, and no refusal about the
// STATE of the record is one.

func TestKiemThongBaoKetLuan_CapSoVaNgay(t *testing.T) {
	ngay := time.Date(2026, 8, 12, 15, 4, 0, 0, time.FixedZone("ICT", 7*3600))

	so, d, err := KiemThongBaoKetLuan("  12/TB-UBND ", ngay)
	if err != nil {
		t.Fatalf("thông báo hợp lệ bị từ chối: %v", err)
	}
	if so != "12/TB-UBND" {
		t.Errorf("số = %q, muốn cắt khoảng trắng", so)
	}
	// THE DAY IS READ IN ITS OWN ZONE AND PINNED TO UTC MIDNIGHT — ChuanHoaNgayHop's rule.
	if !d.Equal(time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("ngày = %v, muốn 2026-08-12 nửa đêm UTC", d)
	}

	for _, ca := range []struct {
		ten  string
		so   string
		ngay time.Time
		loi  error
	}{
		{"thiếu số", "  ", ngay, ErrThieuSoThongBao},
		{"số quá dài", strings.Repeat("a", SoHieuBienBanToiDa+1), ngay, ErrSoThongBaoQuaDai},
		{"thiếu ngày", "12/TB-UBND", time.Time{}, ErrThieuNgayThongBao},
	} {
		t.Run(ca.ten, func(t *testing.T) {
			if _, _, err := KiemThongBaoKetLuan(ca.so, ca.ngay); !errors.Is(err, ca.loi) {
				t.Errorf("lỗi = %v, muốn %v", err, ca.loi)
			}
		})
	}
}

func TestKiemLyDoXoaBienBan_BatBuocVaCoTran(t *testing.T) {
	if got, err := KiemLyDoXoaBienBan("  nhập trùng "); err != nil || got != "nhập trùng" {
		t.Errorf("KiemLyDoXoaBienBan = %q, %v", got, err)
	}
	if _, err := KiemLyDoXoaBienBan(" "); !errors.Is(err, ErrThieuLyDoXoaBienBan) {
		t.Errorf("lỗi = %v, muốn ErrThieuLyDoXoaBienBan — xoá mềm phải ghi vì sao (luật 7)", err)
	}
	if _, err := KiemLyDoXoaBienBan(strings.Repeat("ạ", LyDoXoaBienBanToiDa+1)); !errors.Is(err, ErrLyDoXoaBienBanQuaDai) {
		t.Errorf("lỗi = %v, muốn ErrLyDoXoaBienBanQuaDai", err)
	}
}

// TestLoiTrangThaiBienBanKhongPhaiLoiDauVao keeps the 400 / 409 line honest. A state refusal answered
// as 400 tells a clerk to fix a form that has nothing wrong with it.
func TestLoiTrangThaiBienBanKhongPhaiLoiDauVao(t *testing.T) {
	for _, e := range []error{ErrBienBanDaKy, ErrDaCoThongBao, ErrKetLuanDaCoNhiemVu,
		LoiBienBanConNhiemVu(2), ErrKetLuanKhongPhatSinh} {
		if LaLoiDauVaoBienBan(e) {
			t.Errorf("%v bị xếp là lỗi đầu vào (400) — đây là trạng thái của bản ghi (409)", e)
		}
	}
	for _, e := range []error{ErrThuKyQuaDai, ErrThieuSoThongBao, ErrSoThongBaoQuaDai,
		ErrThieuNgayThongBao, ErrNgayThongBaoKhongDocDuoc, ErrThieuLyDoXoaBienBan,
		ErrLyDoXoaBienBanQuaDai, ErrBoSungChoBanNhap} {
		if !LaLoiDauVaoBienBan(e) {
			t.Errorf("%v không được nhận là lỗi đầu vào — sẽ thành 500 ở client", e)
		}
	}
}

func TestLoiBienBanConNhiemVu_MangSoLuong(t *testing.T) {
	err := LoiBienBanConNhiemVu(3)
	if !errors.Is(err, ErrBienBanConNhiemVu) {
		t.Fatalf("lỗi không bọc ErrBienBanConNhiemVu: %v", err)
	}
	if !strings.Contains(err.Error(), "3 nhiệm vụ") {
		t.Errorf("câu lỗi không nói còn bao nhiêu nhiệm vụ: %q", err)
	}
}
