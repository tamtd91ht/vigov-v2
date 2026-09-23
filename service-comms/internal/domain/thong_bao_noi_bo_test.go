package domain

import (
	"errors"
	"strings"
	"testing"
)

// The rules of chapter 08 that are DECIDABLE WITHOUT INFRASTRUCTURE. Everything here runs
// everywhere, always — which is the point: the pg suite next door skips itself when no DSN is set,
// and a rule that only that suite checks is a rule nothing checks on most machines.

func TestChuanHoaTieuDeCatKhoangTrangVaTuChoiRong(t *testing.T) {
	ra, err := ChuanHoaTieuDe("  Mời họp giao ban tháng 9  ")
	if err != nil {
		t.Fatalf("tiêu đề hợp lệ bị từ chối: %v", err)
	}
	if ra != "Mời họp giao ban tháng 9" {
		t.Errorf("tiêu đề = %q, muốn đã cắt khoảng trắng", ra)
	}

	for _, xau := range []string{"", "   ", "\t\n"} {
		if _, err := ChuanHoaTieuDe(xau); !errors.Is(err, ErrTieuDeTrong) {
			t.Errorf("tiêu đề %q: lỗi = %v, muốn ErrTieuDeTrong", xau, err)
		}
	}
}

func TestChuanHoaTieuDeTuChoiKyTuDieuKhien(t *testing.T) {
	// A newline in a subject line is how one announcement becomes two lines in a log and a broken
	// card on the screen. The body is allowed them; the title is not.
	if _, err := ChuanHoaTieuDe("Mời họp\ngiao ban"); !errors.Is(err, ErrTieuDeTrong) {
		t.Errorf("tiêu đề có xuống dòng: lỗi = %v, muốn bị từ chối", err)
	}
}

func TestChuanHoaTieuDeTuChoiChuKhongCatBot(t *testing.T) {
	// REFUSED, NOT TRUNCATED. A title silently cut is an announcement whose subject changed on the
	// way into an archival record.
	dai := strings.Repeat("a", TieuDeToiDa+1)
	ra, err := ChuanHoaTieuDe(dai)
	if !errors.Is(err, ErrTieuDeQuaDai) {
		t.Fatalf("lỗi = %v, muốn ErrTieuDeQuaDai", err)
	}
	if ra != "" {
		t.Errorf("tiêu đề quá dài phải trả rỗng, nhận %d ký tự — trả về một phần là cắt bớt ngầm", len([]rune(ra)))
	}
}

func TestChuanHoaNoiDungGiuXuongDongNhungTuChoiKyTuDieuKhienKhac(t *testing.T) {
	ra, err := ChuanHoaNoiDungThongBao("Đoạn một.\r\n\nĐoạn hai.\tCó tab.")
	if err != nil {
		t.Fatalf("nội dung nhiều đoạn bị từ chối: %v", err)
	}
	if !strings.Contains(ra, "\n") {
		t.Error("xuống dòng bị mất — một thông báo là nhiều đoạn")
	}
	if _, err := ChuanHoaNoiDungThongBao("Có ký tự \x00 rỗng"); !errors.Is(err, ErrNoiDungTrong) {
		t.Errorf("ký tự NUL: lỗi = %v, muốn bị từ chối", err)
	}
}

func TestChuanHoaMaCanBoTuChoiKhoangTrangBenTrong(t *testing.T) {
	if _, err := ChuanHoaMaCanBo("  CB-2026-7K3M9Q "); err != nil {
		t.Errorf("mã hợp lệ có khoảng trắng hai đầu bị từ chối: %v", err)
	}
	// A code with a space inside is not a code; it is two things, or a name typed into the wrong
	// box. Accepting it would put a value in `nguoi_nhan_ma` that no identity lookup can resolve.
	if _, err := ChuanHoaMaCanBo("CB-2026 7K3M9Q"); !errors.Is(err, ErrMaCanBoTrong) {
		t.Errorf("mã có khoảng trắng bên trong: lỗi = %v, muốn bị từ chối", err)
	}
}

func TestChuanHoaDanhSachBoTrungVaGiuThuTu(t *testing.T) {
	// DEDUPLICATION HAPPENS BEFORE THE COUNT IS TAKEN. The primary key would collapse the duplicate
	// anyway, so without this the announcement would report a recipient count larger than its own
	// list — and §3's `{y}` would be wrong for the life of the record.
	ra, err := ChuanHoaDanhSachMaCanBo(
		[]string{"CB-B", " CB-A ", "CB-B", "CB-C"}, NguoiNhanToiDa, ErrQuaNhieuNguoiNhan)
	if err != nil {
		t.Fatalf("danh sách hợp lệ bị từ chối: %v", err)
	}
	muon := []string{"CB-B", "CB-A", "CB-C"}
	if len(ra) != len(muon) {
		t.Fatalf("danh sách = %v, muốn %v", ra, muon)
	}
	for i := range muon {
		if ra[i] != muon[i] {
			t.Fatalf("danh sách = %v, muốn %v — thứ tự phải giữ nguyên, §4 vẽ đúng danh sách này", ra, muon)
		}
	}
}

func TestChuanHoaDanhSachTuChoiQuaTran(t *testing.T) {
	tho := make([]string, 0, NguoiNhanToiDa+1)
	for i := 0; i <= NguoiNhanToiDa; i++ {
		tho = append(tho, "CB-"+strings.Repeat("A", 1)+itoaTest(i))
	}
	if _, err := ChuanHoaDanhSachMaCanBo(tho, NguoiNhanToiDa, ErrQuaNhieuNguoiNhan); !errors.Is(err, ErrQuaNhieuNguoiNhan) {
		t.Errorf("lỗi = %v, muốn ErrQuaNhieuNguoiNhan", err)
	}
}

func TestYeuCauSoanThongBaoKhongMangTrangThaiHayNguoiSoan(t *testing.T) {
	// A STRUCTURAL ASSERTION, and it is the one worth having: a `TrangThai` or a `NguoiSoanMa`
	// field here would be a field the HTTP body could fill — an announcement posted already marked
	// issued with no recipients behind it, or issued over a colleague's name. The test fails to
	// COMPILE if either is added, which is earlier than any runtime check.
	yc := YeuCauSoanThongBao{
		TieuDe:      "Mời họp",
		NoiDung:     "Nội dung",
		NguoiNhanMa: []string{"CB-2026-7K3M9Q"},
	}
	sach, err := yc.KiemTra()
	if err != nil {
		t.Fatalf("yêu cầu hợp lệ bị từ chối: %v", err)
	}
	if len(sach.NguoiNhanMa) != 1 || len(sach.BoPhanIDs) != 0 {
		t.Errorf("kết quả = %+v", sach)
	}
}

func TestYeuCauSoanThongBaoGiuCoBatBuocXacNhanVaGhim(t *testing.T) {
	// The three flags decide what the commune was ASKED to do. A KiemTra that dropped one would
	// issue an announcement demanding no acknowledgement on a screen that showed the box ticked.
	sach, err := YeuCauSoanThongBao{
		TieuDe: "T", NoiDung: "N", NguoiNhanMa: []string{"CB-1"},
		Ghim: true, BatBuocXacNhan: true, GuiThuDienTu: true,
	}.KiemTra()
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if !sach.Ghim || !sach.BatBuocXacNhan || !sach.GuiThuDienTu {
		t.Errorf("cờ bị mất: %+v", sach)
	}
}

func TestDaPhatHanhBaoGomCaThongBaoDaGo(t *testing.T) {
	// THE DISTINCTION THIS METHOD EXISTS FOR. A withdrawn announcement WAS issued: §4 keeps its
	// recipients and their acknowledgements. Anything asking "has this left the author's hands"
	// that compared against `da-phat-hanh` alone would answer wrongly for exactly those rows.
	cases := []struct {
		trang TrangThaiThongBao
		muon  bool
	}{
		{ThongBaoNhap, false},
		{ThongBaoDaPhatHanh, true},
		{ThongBaoDaGo, true},
	}
	for _, c := range cases {
		if got := (ThongBaoNoiBo{TrangThai: c.trang}).DaPhatHanh(); got != c.muon {
			t.Errorf("DaPhatHanh(%q) = %v, muốn %v", c.trang, got, c.muon)
		}
	}
}

func TestNguoiNhanTrangThaiDocVaXacNhan(t *testing.T) {
	var chuaMo NguoiNhanThongBao
	if chuaMo.DaMo() || chuaMo.DaXacNhan() {
		t.Error("người nhận mới phải là `chưa mở`")
	}
}

// itoaTest keeps strconv out of this file for one loop.
func itoaTest(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
