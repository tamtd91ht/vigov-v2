package domain

import (
	"testing"
	"time"
)

// The DERIVED conclusion status of user decision 4 (25/09/2026), and the card's `x/y kết luận`.
//
//	PROVED HERE   every arm of the precedence qua-han > dang-thuc-hien > hoan-thanh > chua-giao · the
//	              `khong_phat_sinh` mark counts as done, and wins · a task NOT in `hoan-thanh` — including
//	              `chuyen-tiep` / `tam-dung` — is not done, by the counts the store sends · the card
//	              figure counts done conclusions over all live ones.
//
//	NOT PROVED    which tasks the store counts as late. That is store.dieuKienTreHan — the one SQL
//	              spelling of NhiemVu.TreHan — and TestPgTrangThaiKetLuanKhopTreHanCuaDomain compares
//	              the two against a real server (skipped without VIGOV_TEST_DSN).

func TestTrangThaiKetLuanThuTuUuTien(t *testing.T) {
	for _, c := range []struct {
		ten  string
		k    KetLuanHop
		muon TrangThaiKetLuan
	}{
		{"chưa có nhiệm vụ nào", KetLuanHop{}, KetLuanChuaGiao},
		{"đánh dấu không phát sinh, không nhiệm vụ", KetLuanHop{KhongPhatSinh: true}, KetLuanHoanThanh},
		// The mark wins even over a late task — the user's precedence puts it first. The write path
		// refuses the combination; if data ever holds it, the recorded human act is what shows.
		{"đánh dấu không phát sinh thắng cả việc trễ",
			KetLuanHop{KhongPhatSinh: true, SoNhiemVu: 2, SoNhiemVuXong: 0, SoNhiemVuTreHan: 1}, KetLuanHoanThanh},
		{"một việc chưa xong, không trễ", KetLuanHop{SoNhiemVu: 1}, KetLuanDangThucHien},
		{"một xong một chưa, không trễ", KetLuanHop{SoNhiemVu: 2, SoNhiemVuXong: 1}, KetLuanDangThucHien},
		// qua-han OUTRANKS dang-thuc-hien: one late task among three running is the thing the card
		// must show.
		{"một trễ trong ba việc đang làm", KetLuanHop{SoNhiemVu: 3, SoNhiemVuTreHan: 1}, KetLuanQuaHan},
		{"một trễ, còn lại xong", KetLuanHop{SoNhiemVu: 3, SoNhiemVuXong: 2, SoNhiemVuTreHan: 1}, KetLuanQuaHan},
		{"mọi việc xong", KetLuanHop{SoNhiemVu: 2, SoNhiemVuXong: 2}, KetLuanHoanThanh},
		// A task in `chuyen-tiep` (or `tam-dung`) is NOT in SoNhiemVuXong, so the conclusion is not
		// finished. Open question for the customer; the literal reading is what is implemented.
		{"một xong một chuyển tiếp", KetLuanHop{SoNhiemVu: 2, SoNhiemVuXong: 1}, KetLuanDangThucHien},
		{"chuyển tiếp đã quá hạn", KetLuanHop{SoNhiemVu: 1, SoNhiemVuTreHan: 1}, KetLuanQuaHan},
	} {
		if got := c.k.TrangThai(); got != c.muon {
			t.Errorf("%s: TrangThai = %q, muốn %q", c.ten, got, c.muon)
		}
	}
}

func TestTienDoKetLuanDemXongGomDauKhongPhatSinh(t *testing.T) {
	b := BienBanHop{KetLuan: []KetLuanHop{
		{ThuTu: 1, SoNhiemVu: 2, SoNhiemVuXong: 2},   // hoan-thanh
		{ThuTu: 2, KhongPhatSinh: true},              // hoan-thanh by the mark
		{ThuTu: 3},                                   // chua-giao
		{ThuTu: 4, SoNhiemVu: 1, SoNhiemVuTreHan: 1}, // qua-han
		{ThuTu: 5, SoNhiemVu: 3, SoNhiemVuXong: 1},   // dang-thuc-hien
	}}
	xong, tong := b.TienDoKetLuan()
	if xong != 2 || tong != 5 {
		t.Errorf("TienDoKetLuan = %d/%d, muốn 2/5", xong, tong)
	}
	if x, y := (BienBanHop{}).TienDoKetLuan(); x != 0 || y != 0 {
		t.Errorf("biên bản không kết luận: %d/%d, muốn 0/0", x, y)
	}
}

// TestTrangThaiKetLuanKhongDocDongHo — the status is a function of the counts ONLY. The clock enters
// once, in the store's SQL (`now()`), so the same row cannot read two statuses in one response.
func TestTrangThaiKetLuanKhongDocDongHo(t *testing.T) {
	k := KetLuanHop{SoNhiemVu: 1, TaoLuc: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)}
	if k.TrangThai() != KetLuanDangThucHien {
		t.Errorf("TrangThai phụ thuộc thứ khác ngoài bộ đếm: %q", k.TrangThai())
	}
}
