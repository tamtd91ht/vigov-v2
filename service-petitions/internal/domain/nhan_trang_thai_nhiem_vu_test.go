package domain

import (
	"errors"
	"strings"
	"testing"
)

// The default table, the merge and the validators of nhan_trang_thai_nhiem_vu.go.

func TestMacDinhTrangThaiDungBayMaCuaVongDoi(t *testing.T) {
	// #21 IN ONE ASSERTION: the default table names EXACTLY the lifecycle's codes. An eighth entry
	// would be a label for a state nothing transitions into; a missing one would be a status the read
	// route never returns, i.e. a Kanban column with no header.
	md := MacDinhTrangThaiNhiemVu()
	if len(md) != len(chuyenDuocSangNhiemVu) {
		t.Fatalf("bảng mặc định có %d mã, vòng đời có %d", len(md), len(chuyenDuocSangNhiemVu))
	}
	daThay := map[TrangThaiNhiemVu]bool{}
	thuTu := map[int]bool{}
	for _, m := range md {
		if !m.Ma.HopLe() {
			t.Errorf("mã mặc định %q không có trong vòng đời", m.Ma)
		}
		if daThay[m.Ma] {
			t.Errorf("mã %q lặp", m.Ma)
		}
		daThay[m.Ma] = true
		// DISTINCT default positions are what makes the read's tie-break total.
		if thuTu[m.ThuTu] || m.ThuTu < 1 {
			t.Errorf("thứ tự mặc định %d lặp hoặc < 1", m.ThuTu)
		}
		thuTu[m.ThuTu] = true
		if _, err := ChuanHoaNhanTrangThai(m.Nhan); err != nil {
			t.Errorf("nhãn mặc định %q không qua được chính bộ kiểm của mình: %v", m.Nhan, err)
		}
	}
}

func TestMacDinhTrangThaiKhopDacTa(t *testing.T) {
	// docs/ui-ux/02-nhiem-vu.md §6, :218-224. Written out so a "tidy-up" of the wording is a red test
	// rather than a silent change on every commune's screen.
	muon := []struct {
		ma     TrangThaiNhiemVu
		nhan   string
		thuTu  int
		vaiTro VaiTroTrangThai
	}{
		{MoiGiao, "Mới giao", 1, VaiTroChinh},
		{DaTiepNhanNV, "Đã tiếp nhận", 2, VaiTroChinh},
		{DangThucHien, "Đang thực hiện", 3, VaiTroChinh},
		{ChoDuyet, "Chờ duyệt", 4, VaiTroChinh},
		{HoanThanh, "Hoàn thành", 5, VaiTroChinh},
		{TamDung, "Tạm dừng", 6, VaiTroReNhanh},
		{ChuyenTiep, "Chuyển tiếp", 7, VaiTroReNhanh},
	}
	md := MacDinhTrangThaiNhiemVu()
	for i, m := range muon {
		if md[i].Ma != m.ma || md[i].Nhan != m.nhan || md[i].ThuTu != m.thuTu || md[i].Ma.VaiTro() != m.vaiTro {
			t.Errorf("dòng %d = %+v (vai trò %q), muốn %+v", i, md[i], md[i].Ma.VaiTro(), m)
		}
	}
}

func TestMacDinhTrangThaiTraBanSao(t *testing.T) {
	md := MacDinhTrangThaiNhiemVu()
	md[0].Nhan = "đã bị sửa"
	if MacDinhTrangThaiNhiemVu()[0].Nhan != "Mới giao" {
		t.Fatal("người gọi sửa được bảng mặc định dùng chung cho mọi xã")
	}
}

func TestTimMacDinhTrangThaiMaLaLaKhongCo(t *testing.T) {
	for _, ma := range []string{"da-huy", "", "Moi-giao", "moi-giao "} {
		if _, ok := TimMacDinhTrangThai(ma); ok {
			t.Errorf("mã %q được nhận là mã trạng thái", ma)
		}
	}
	if md, ok := TimMacDinhTrangThai("cho-duyet"); !ok || md.Nhan != "Chờ duyệt" {
		t.Errorf("không tìm thấy cho-duyet: %+v %v", md, ok)
	}
}

func TestGopNhanTrangThaiKhongCoDongNaoLaBayMacDinh(t *testing.T) {
	// TODAY'S ANSWER FOR EVERY COMMUNE.
	ra, err := GopNhanTrangThai(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(ra) != 7 {
		t.Fatalf("trả %d trạng thái, muốn 7", len(ra))
	}
	for i, tt := range ra {
		if tt.DaTuyChinh || tt.Nhan != tt.NhanMacDinh || tt.ThuTu != i+1 {
			t.Errorf("dòng %d: %+v — không có dòng ghi đè thì phải là mặc định", i, tt)
		}
	}
}

func TestGopNhanTrangThaiMotDongGhiDeVaSapXep(t *testing.T) {
	// `tam-dung` moved to position 1 and renamed: it ties with `moi-giao`'s default 1, and the tie
	// is broken by DEFAULT order, so `moi-giao` (default 1) stays ahead of `tam-dung` (default 6).
	ra, err := GopNhanTrangThai([]NhanTrangThaiNhiemVu{
		{Ma: TamDung, Nhan: "Đang treo", ThuTu: 1, CapNhatBoi: "CB-00123"},
	})
	if err != nil {
		t.Fatal(err)
	}
	thuTuMa := make([]string, 0, 7)
	for _, tt := range ra {
		thuTuMa = append(thuTuMa, string(tt.Ma))
	}
	muon := "moi-giao,tam-dung,da-tiep-nhan,dang-thuc-hien,cho-duyet,hoan-thanh,chuyen-tiep"
	if got := strings.Join(thuTuMa, ","); got != muon {
		t.Fatalf("thứ tự = %s, muốn %s", got, muon)
	}
	td := ra[1]
	if td.Nhan != "Đang treo" || td.ThuTu != 1 || !td.DaTuyChinh ||
		td.NhanMacDinh != "Tạm dừng" || td.ThuTuMacDinh != 6 || td.VaiTro != VaiTroReNhanh {
		t.Errorf("tam-dung sau gộp = %+v", td)
	}
	if ra[0].DaTuyChinh {
		t.Error("moi-giao không có dòng mà bị báo đã tuỳ chỉnh")
	}
}

func TestGopNhanTrangThaiDongBangMacDinhKhongLaTuyChinh(t *testing.T) {
	// "Back to the default" is a WRITE of the default values (migration 0010 refuses DELETE). After
	// it the row exists and nothing is customised.
	ra, err := GopNhanTrangThai([]NhanTrangThaiNhiemVu{{Ma: ChoDuyet, Nhan: "Chờ duyệt", ThuTu: 4}})
	if err != nil {
		t.Fatal(err)
	}
	if ra[3].Ma != ChoDuyet || ra[3].DaTuyChinh {
		t.Errorf("dòng mang đúng giá trị mặc định mà bị báo tuỳ chỉnh: %+v", ra[3])
	}
}

func TestGopNhanTrangThaiMaLaThiTuChoi(t *testing.T) {
	_, err := GopNhanTrangThai([]NhanTrangThaiNhiemVu{{Ma: "da-huy", Nhan: "Đã huỷ", ThuTu: 8}})
	if !errors.Is(err, ErrMaTrangThaiLa) {
		t.Fatalf("lỗi = %v, muốn ErrMaTrangThaiLa — bỏ qua im lặng là cài đặt của xã không được áp", err)
	}
}

func TestChuanHoaNhanTrangThaiDemKyTuKhongDemByte(t *testing.T) {
	// "ơ" is 2 bytes. 100 of them = 200 bytes: accepted. 101 = refused. A byte count would refuse
	// the first already (at 51 characters).
	vua := strings.Repeat("ơ", 100)
	if _, err := ChuanHoaNhanTrangThai(vua); err != nil {
		t.Fatalf("nhãn đúng 100 ký tự (200 byte) bị từ chối — đang đếm byte: %v", err)
	}
	if _, err := ChuanHoaNhanTrangThai(vua + "ơ"); !errors.Is(err, ErrNhanQuaDai) {
		t.Fatalf("nhãn 101 ký tự: lỗi = %v, muốn ErrNhanQuaDai", err)
	}
	// A 150-character Vietnamese label of 3-byte letters (450 bytes): refused.
	if _, err := ChuanHoaNhanTrangThai(strings.Repeat("ệ", 150)); !errors.Is(err, ErrNhanQuaDai) {
		t.Fatalf("nhãn 150 ký tự: lỗi = %v", err)
	}
}

func TestChuanHoaNhanTrangThaiTrongVaCatKhoangTrang(t *testing.T) {
	for _, s := range []string{"", "   ", "\t\n", "Mới\x00giao"} {
		if _, err := ChuanHoaNhanTrangThai(s); !errors.Is(err, ErrNhanTrong) {
			t.Errorf("%q: lỗi = %v, muốn ErrNhanTrong", s, err)
		}
	}
	if got, err := ChuanHoaNhanTrangThai("  Chưa thực hiện  "); err != nil || got != "Chưa thực hiện" {
		t.Errorf("cắt khoảng trắng: %q %v", got, err)
	}
}

func TestKiemTraThuTuTrangThai(t *testing.T) {
	for _, n := range []int{0, -1, ThuTuToiDa + 1} {
		if err := KiemTraThuTuTrangThai(n); !errors.Is(err, ErrThuTuNgoaiKhoang) {
			t.Errorf("%d: lỗi = %v", n, err)
		}
	}
	for _, n := range []int{1, 7, ThuTuToiDa} {
		if err := KiemTraThuTuTrangThai(n); err != nil {
			t.Errorf("%d bị từ chối: %v", n, err)
		}
	}
}
