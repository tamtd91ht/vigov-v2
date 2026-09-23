package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// Tests for the WRITE rules of the task register.
//
//	PROVED HERE   ADR 0037 decision 4's verdict, including that `chuyen-tiep` does NOT count as
//	              finished · ADR 0038's approval rule in every one of its branches, including the two
//	              empty-code branches that would otherwise open every task in the commune · §6's
//	              resume-to-the-previous-state rule and the refusal when the timeline cannot say ·
//	              the minted series · the bounds, measured in RUNES.
//
//	NOT PROVED    the TRAVERSAL of the tree and the cycle walk: both need the store and live in
//	              internal/app, over the real store on a fake driver. This file is the verdict, that
//	              one is the walk.

const maLanhDaoThu = "CB-00007"

func nhiemVuCoHan(han time.Time) NhiemVu {
	return NhiemVu{
		Ma:                "NV19",
		TrangThai:         DangThucHien,
		HanXuLy:           han,
		HanBanDau:         han,
		LanhDaoGiaoViecMa: maLanhDaoThu,
	}
}

// --- ADR 0037 decision 4 ---------------------------------------------------------------------

func TestConChuaXongChiTinhHoanThanh(t *testing.T) {
	// EVERY NON-`hoan-thanh` STATUS IS UNFINISHED, and the case that matters is `chuyen-tiep`: it is
	// TERMINAL, so a reader could reasonably think it counts as done. It does not, because §6 says a
	// forwarded task "sinh bản ghi liên kết" and NO COLUMN links the two rows — this service cannot
	// tell whether the work was ever finished somewhere else.
	con := []NhiemVuTomTat{
		{Ma: "NV20", TrangThai: HoanThanh},
		{Ma: "NV21", TrangThai: ChuyenTiep},
		{Ma: "NV22", TrangThai: HoanThanh},
		{Ma: "NV23", TrangThai: TamDung},
	}
	chua := ConChuaXong(con)
	if len(chua) != 2 || chua[0] != "NV21" || chua[1] != "NV23" {
		t.Fatalf("việc con chưa xong = %v, muốn [NV21 NV23] — `chuyen-tiep` KHÔNG phải hoàn thành", chua)
	}
}

func TestConChuaXongCaCayHoanThanhThiKhongCanTro(t *testing.T) {
	chua := ConChuaXong([]NhiemVuTomTat{
		{Ma: "NV20", TrangThai: HoanThanh},
		{Ma: "NV21", TrangThai: HoanThanh},
	})
	if len(chua) != 0 {
		t.Fatalf("việc con chưa xong = %v, muốn rỗng", chua)
	}
}

func TestConChuaXongKhongConThiKhongCanTro(t *testing.T) {
	if chua := ConChuaXong(nil); len(chua) != 0 {
		t.Fatalf("việc con chưa xong = %v, muốn rỗng — việc không có con thì hoàn thành được", chua)
	}
}

// TestLoiConChuaXongNoiRoConNao pins ADR 0037 decision 4's own wording: the refusal must LIST the
// remaining work, not merely refuse.
func TestLoiConChuaXongNoiRoConNao(t *testing.T) {
	err := LoiConChuaXong([]string{"NV20", "NV21"})
	if !errors.Is(err, ErrConChuaXong) {
		t.Fatalf("lỗi không bọc ErrConChuaXong: %v", err)
	}
	for _, muon := range []string{"NV20", "NV21", "2"} {
		if !strings.Contains(err.Error(), muon) {
			t.Errorf("câu từ chối không nhắc %q: %q", muon, err.Error())
		}
	}
}

// TestLoiConChuaXongChanDoDaiCau keeps one refusal from becoming a page of register numbers while
// still stating the exact count.
func TestLoiConChuaXongChanDoDaiCau(t *testing.T) {
	var ma []string
	for i := 0; i < MuoiMaDauTien+5; i++ {
		ma = append(ma, "NV"+string(rune('A'+i)))
	}
	err := LoiConChuaXong(ma)
	if !strings.Contains(err.Error(), "15") {
		t.Errorf("câu từ chối không nói đủ số việc con: %q", err.Error())
	}
	if strings.Contains(err.Error(), ma[MuoiMaDauTien]) {
		t.Errorf("câu từ chối liệt kê quá %d mã: %q", MuoiMaDauTien, err.Error())
	}
}

// TestLoiConChuaXoaNoiRoMayViec pins decision 3's wording — "kèm câu nói rõ còn mấy việc con".
func TestLoiConChuaXoaNoiRoMayViec(t *testing.T) {
	err := LoiConChuaXoa(3)
	if !errors.Is(err, ErrConChuaXoa) {
		t.Fatalf("lỗi không bọc ErrConChuaXoa: %v", err)
	}
	if !strings.Contains(err.Error(), "3") {
		t.Errorf("câu từ chối không nói còn mấy việc con: %q", err.Error())
	}
}

// --- §6: pausing and resuming -------------------------------------------------------------------

func TestTrangThaiTruocTamDungBoQuaMoiDongTamDung(t *testing.T) {
	// NEWEST FIRST. The head of the timeline is the pause itself and an entry written while paused;
	// the first row that is not `tam-dung` is the state the task was paused FROM.
	truoc, co := TrangThaiTruocTamDung([]MocNhatKy{
		{TrangThai: TamDung},
		{TrangThai: TamDung},
		{TrangThai: DangThucHien},
		{TrangThai: DaTiepNhanNV},
	})
	if !co || truoc != DangThucHien {
		t.Fatalf("trạng thái trước tạm dừng = %q (%v), muốn %q", truoc, co, DangThucHien)
	}
}

func TestTrangThaiTruocTamDungNhatKyRongThiKhongDoan(t *testing.T) {
	if _, co := TrangThaiTruocTamDung(nil); co {
		t.Fatal("đoán ra trạng thái trước từ nhật ký rỗng — phải từ chối, không được mặc định")
	}
	if _, co := TrangThaiTruocTamDung([]MocNhatKy{{TrangThai: TamDung}}); co {
		t.Fatal("đoán ra trạng thái trước khi nhật ký chỉ có tạm dừng")
	}
}

func TestChuyenTrangThaiTiepTucVeDungTrangThaiTruoc(t *testing.T) {
	if err := ChuyenTrangThaiDuoc(TamDung, DangThucHien, DangThucHien); err != nil {
		t.Fatalf("tiếp tục về đúng trạng thái trước bị từ chối: %v", err)
	}
}

func TestChuyenTrangThaiTiepTucSaiTrangThaiThiTuChoi(t *testing.T) {
	// The task was paused from `moi-giao`; resuming it into `dang-thuc-hien` would skip a step
	// nobody took. ChuyenSangDuoc alone would allow it — which is why the resume rule is a second
	// function and not a looser map.
	err := ChuyenTrangThaiDuoc(TamDung, DangThucHien, MoiGiao)
	if !errors.Is(err, ErrChuyenTrangThaiNhiemVuSaiLuc) {
		t.Fatalf("lỗi = %v, muốn ErrChuyenTrangThaiNhiemVuSaiLuc", err)
	}
}

func TestChuyenTrangThaiTiepTucVeHoanThanhThiTuChoi(t *testing.T) {
	// A timeline CAN hold `hoan-thanh`, and resuming a pause into it would be finished work nobody
	// did. TamDungVeDuoc is what stops it.
	err := ChuyenTrangThaiDuoc(TamDung, HoanThanh, HoanThanh)
	if !errors.Is(err, ErrChuyenTrangThaiNhiemVuSaiLuc) {
		t.Fatalf("lỗi = %v, muốn ErrChuyenTrangThaiNhiemVuSaiLuc", err)
	}
}

func TestChuyenTrangThaiTamDungKhongBietTrangThaiTruocThiTuChoi(t *testing.T) {
	err := ChuyenTrangThaiDuoc(TamDung, DangThucHien, "")
	if !errors.Is(err, ErrTiepTucKhongBietTrangThaiTruoc) {
		t.Fatalf("lỗi = %v, muốn ErrTiepTucKhongBietTrangThaiTruoc", err)
	}
}

func TestChuyenTrangThaiTheoDungVongDoi(t *testing.T) {
	for _, ca := range []struct {
		tu, sang TrangThaiNhiemVu
		duoc     bool
	}{
		{MoiGiao, DaTiepNhanNV, true},
		{DaTiepNhanNV, DangThucHien, true},
		{DangThucHien, ChoDuyet, true},
		{ChoDuyet, HoanThanh, true},
		{DangThucHien, TamDung, true},
		{DangThucHien, ChuyenTiep, true},
		// The jumps §6 does not draw. A client able to skip steps makes every intermediate state
		// optional in practice while looking mandatory in the map.
		{MoiGiao, HoanThanh, false},
		{DaTiepNhanNV, ChoDuyet, false},
		{HoanThanh, DangThucHien, false},
		{ChuyenTiep, DangThucHien, false},
	} {
		err := ChuyenTrangThaiDuoc(ca.tu, ca.sang, "")
		if ca.duoc && err != nil {
			t.Errorf("%s → %s bị từ chối: %v", ca.tu, ca.sang, err)
		}
		if !ca.duoc && err == nil {
			t.Errorf("%s → %s được chấp nhận — vòng đời §6 không có bước này", ca.tu, ca.sang)
		}
	}
}

func TestChuyenTrangThaiLaKhongBietThiTuChoi(t *testing.T) {
	err := ChuyenTrangThaiDuoc(DangThucHien, TrangThaiNhiemVu("dang-lam-do"), "")
	if !errors.Is(err, ErrTrangThaiNhiemVuKhongBiet) {
		t.Fatalf("lỗi = %v, muốn ErrTrangThaiNhiemVuKhongBiet", err)
	}
}

// --- ADR 0038: who approves an extension ----------------------------------------------------------

func deNghiThu() DeNghiLuiHan {
	return DeNghiLuiHan{
		ID:            "dn-001",
		NguoiDeNghiMa: "CB-00311",
		HanMoi:        time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC),
		LyDo:          "Chờ số liệu từ ba chi bộ chưa gửi về.",
		TrangThai:     ChoDuyetLuiHan,
	}
}

func TestDuyetLuiHanDungNguoiTrenBanGhiThiQua(t *testing.T) {
	n := nhiemVuCoHan(time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC))
	if err := DuocDuyetLuiHan(n, deNghiThu(), maLanhDaoThu); err != nil {
		t.Fatalf("lãnh đạo giao việc ghi trên bản ghi bị từ chối: %v", err)
	}
}

// TestDuyetLuiHanNguoiKhacThiTuChoi is ADR 0038's whole reason for existing: holding `task.extend`
// is NOT the same question as "is this your task".
func TestDuyetLuiHanNguoiKhacThiTuChoi(t *testing.T) {
	n := nhiemVuCoHan(time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC))
	err := DuocDuyetLuiHan(n, deNghiThu(), "CB-00999")
	if !errors.Is(err, ErrKhongPhaiLanhDaoGiaoViec) {
		t.Fatalf("lỗi = %v, muốn ErrKhongPhaiLanhDaoGiaoViec — "+
			"một lãnh đạo khác cầm `task.extend` KHÔNG được duyệt việc của người khác", err)
	}
}

// TestDuyetLuiHanChuaGhiLanhDaoThiTuChoiChuKhongRoiVeNguoiTao is ADR 0038's OPEN QUESTION, failing
// closed. Falling back to `nguoi_tao_ma` would hand the decision to the clerk who typed the row on
// somebody else's behalf — which is exactly why migration 0006 put the two codes in two columns.
func TestDuyetLuiHanChuaGhiLanhDaoThiTuChoiChuKhongRoiVeNguoiTao(t *testing.T) {
	n := nhiemVuCoHan(time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC))
	n.LanhDaoGiaoViecMa = ""
	n.NguoiTaoMa = "CB-00123"

	err := DuocDuyetLuiHan(n, deNghiThu(), "CB-00123")
	if !errors.Is(err, ErrChuaGhiLanhDaoGiaoViec) {
		t.Fatalf("lỗi = %v, muốn ErrChuaGhiLanhDaoGiaoViec — "+
			"người tạo KHÔNG được thay lãnh đạo giao việc duyệt lùi hạn", err)
	}
	if !strings.Contains(err.Error(), "lãnh đạo giao việc") {
		t.Errorf("câu từ chối không nói rõ thiếu gì: %q", err.Error())
	}
}

// TestDuyetLuiHanHaiMaRongKhongKhopNhau is the narrowest omission with the widest failure: without
// the explicit guards, `"" == ""` is true and every account holding the key approves every
// extension on every task that names no leader.
func TestDuyetLuiHanHaiMaRongKhongKhopNhau(t *testing.T) {
	n := nhiemVuCoHan(time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC))
	n.LanhDaoGiaoViecMa = ""

	if err := DuocDuyetLuiHan(n, deNghiThu(), ""); err == nil {
		t.Fatal("chủ thể rỗng duyệt được đề nghị trên nhiệm vụ không ghi lãnh đạo — " +
			`"" == "" mở toàn bộ sổ nhiệm vụ của xã`)
	}
}

// TestDuyetLuiHanKhongTuDuyetDeNghiCuaMinh is the SECOND, independent rule — it bites even when the
// person IS the named leader, which is the case the first rule cannot catch.
func TestDuyetLuiHanKhongTuDuyetDeNghiCuaMinh(t *testing.T) {
	n := nhiemVuCoHan(time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC))
	dn := deNghiThu()
	dn.NguoiDeNghiMa = maLanhDaoThu // the leader asked for the extension themselves

	err := DuocDuyetLuiHan(n, dn, maLanhDaoThu)
	if !errors.Is(err, ErrTuDuyetDeNghiCuaMinh) {
		t.Fatalf("lỗi = %v, muốn ErrTuDuyetDeNghiCuaMinh", err)
	}
}

func TestDuyetLuiHanDeNghiDaQuyetDinhThiTuChoi(t *testing.T) {
	n := nhiemVuCoHan(time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC))
	dn := deNghiThu()
	dn.TrangThai = DaDuyetLuiHan

	err := DuocDuyetLuiHan(n, dn, maLanhDaoThu)
	if !errors.Is(err, ErrDeNghiDaQuyetDinh) {
		t.Fatalf("lỗi = %v, muốn ErrDeNghiDaQuyetDinh", err)
	}
}

// TestLaLoiThamQuyenLuiHanTachDungHaiNhom pins which refusals are about WHO is acting (403) and
// which are about the state of the record (409). Folding them together sends an officer to the
// Phân quyền screen for a problem no permission can fix, or the reverse.
func TestLaLoiThamQuyenLuiHanTachDungHaiNhom(t *testing.T) {
	for _, err := range []error{
		ErrKhongPhaiLanhDaoGiaoViec, ErrTuDuyetDeNghiCuaMinh, ErrChuaGhiLanhDaoGiaoViec,
	} {
		if !LaLoiThamQuyenLuiHan(err) {
			t.Errorf("%v không được xếp vào nhóm thẩm quyền", err)
		}
	}
	for _, err := range []error{ErrDeNghiDaQuyetDinh, ErrNhiemVuChuaCoHan, ErrHanMoiKhongLui} {
		if LaLoiThamQuyenLuiHan(err) {
			t.Errorf("%v bị xếp nhầm vào nhóm thẩm quyền — đây là lỗi trạng thái bản ghi", err)
		}
	}
}

// --- filing an extension request ------------------------------------------------------------------

func TestDeNghiLuiHanPhaiMuonHonHanHienTai(t *testing.T) {
	han := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	n := nhiemVuCoHan(han)

	if err := DuocDeNghiLuiHan(n, han.Add(72*time.Hour)); err != nil {
		t.Fatalf("hạn mới muộn hơn bị từ chối: %v", err)
	}
	for _, xau := range []time.Time{han, han.Add(-time.Hour)} {
		if err := DuocDeNghiLuiHan(n, xau); !errors.Is(err, ErrHanMoiKhongLui) {
			t.Errorf("hạn mới %v: lỗi = %v, muốn ErrHanMoiKhongLui", xau, err)
		}
	}
}

// TestDeNghiLuiHanViecKhongCoHanThiTuChoi mirrors what the schema would refuse anyway — but as a
// sentence naming what is missing rather than a constraint error.
func TestDeNghiLuiHanViecKhongCoHanThiTuChoi(t *testing.T) {
	var n NhiemVu
	n.TrangThai = DangThucHien

	err := DuocDeNghiLuiHan(n, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	if !errors.Is(err, ErrNhiemVuChuaCoHan) {
		t.Fatalf("lỗi = %v, muốn ErrNhiemVuChuaCoHan", err)
	}
}

// --- the minted series ------------------------------------------------------------------------------

func TestMaNhiemVuTiepTheoTheoDayNV(t *testing.T) {
	for _, ca := range []struct {
		lonNhat int
		muon    string
	}{{0, "NV01"}, {1, "NV02"}, {18, "NV19"}, {99, "NV100"}} {
		if got := MaNhiemVuTiepTheo(ca.lonNhat); got != ca.muon {
			t.Errorf("MaNhiemVuTiepTheo(%d) = %q, muốn %q", ca.lonNhat, got, ca.muon)
		}
	}
}

func TestSoTrongMaNhiemVuChiDocDayDaCap(t *testing.T) {
	for _, ca := range []struct {
		ma  string
		so  int
		duo bool
	}{
		{"NV19", 19, true},
		{"NV01", 1, true},
		{"NV100", 100, true},
		// A commune that typed its own code contributes nothing to the series — §7.1 offers the
		// checkbox, and `KH-2026-07` is a perfectly good task number.
		{"KH-2026-07", 0, false},
		{"NV", 0, false},
		{"NVA1", 0, false},
	} {
		so, duoc := SoTrongMaNhiemVu(ca.ma)
		if duoc != ca.duo || (duoc && so != ca.so) {
			t.Errorf("SoTrongMaNhiemVu(%q) = %d,%v — muốn %d,%v", ca.ma, so, duoc, ca.so, ca.duo)
		}
	}
}

// --- the bounds -------------------------------------------------------------------------------------

// TestKiemTieuDeDemBangRuneChuKhongPhaiByte is the bound that would be wrong in exactly one
// language: Vietnamese is three bytes per accented character, so a byte bound cuts a Vietnamese
// title at a third of the length it cuts an English one.
func TestKiemTieuDeDemBangRuneChuKhongPhaiByte(t *testing.T) {
	vua := strings.Repeat("ề", TieuDeNhiemVuToiDa)
	if _, err := KiemTieuDeNhiemVu(vua); err != nil {
		t.Fatalf("tiêu đề đúng %d ký tự bị từ chối: %v", TieuDeNhiemVuToiDa, err)
	}
	if _, err := KiemTieuDeNhiemVu(vua + "ề"); !errors.Is(err, ErrTieuDeNhiemVuQuaDai) {
		t.Fatalf("lỗi = %v, muốn ErrTieuDeNhiemVuQuaDai", err)
	}
}

func TestKiemTieuDeCatKhoangTrangVaTuChoiRong(t *testing.T) {
	s, err := KiemTieuDeNhiemVu("  Báo cáo tổng kết  ")
	if err != nil || s != "Báo cáo tổng kết" {
		t.Fatalf("KiemTieuDeNhiemVu = %q, %v", s, err)
	}
	if _, err := KiemTieuDeNhiemVu("   "); !errors.Is(err, ErrThieuTieuDeNhiemVu) {
		t.Fatalf("lỗi = %v, muốn ErrThieuTieuDeNhiemVu", err)
	}
}

// TestKiemMaNhiemVuChanKyTuKhongGoLaiDuoc keeps out of the register a number that cannot be typed
// back, searched for reliably, or carried in a URL path segment.
func TestKiemMaNhiemVuChanKyTuKhongGoLaiDuoc(t *testing.T) {
	for _, tot := range []string{"NV19", "KH-2026-07", "NV_01"} {
		if _, err := KiemMaNhiemVu(tot); err != nil {
			t.Errorf("mã hợp lệ %q bị từ chối: %v", tot, err)
		}
	}
	for _, xau := range []string{"NV 19", "nv19", "NV/19", "NV19?", "Nhiệm-vụ-19", ""} {
		if _, err := KiemMaNhiemVu(xau); err == nil {
			t.Errorf("mã %q được nhận — mã này in lên Sổ theo dõi và nằm trên URL", xau)
		}
	}
}

func TestKiemTienDoTrongKhoang(t *testing.T) {
	for _, tot := range []int{0, 40, TienDoToiDa} {
		if err := KiemTienDo(tot); err != nil {
			t.Errorf("tiến độ %d bị từ chối: %v", tot, err)
		}
	}
	for _, xau := range []int{-1, TienDoToiDa + 1} {
		if err := KiemTienDo(xau); !errors.Is(err, ErrTienDoNgoaiKhoang) {
			t.Errorf("tiến độ %d: lỗi = %v, muốn ErrTienDoNgoaiKhoang", xau, err)
		}
	}
}

func TestKiemLyDoXoaBatBuoc(t *testing.T) {
	if _, err := KiemLyDoXoaNhiemVu("  "); !errors.Is(err, ErrThieuLyDoXoaNhiemVu) {
		t.Fatalf("lỗi = %v, muốn ErrThieuLyDoXoaNhiemVu — xoá mềm phải ghi vì sao (luật 7)", err)
	}
}

func TestNoiDungChuyenTrangThaiNoiMaChuKhongNoiNhan(t *testing.T) {
	// The LABELS belong to the commune (open question #21). A label frozen into an append-only
	// timeline row would still be there after the commune re-worded it.
	s := NoiDungChuyenTrangThai(DangThucHien, ChoDuyet)
	if !strings.Contains(s, string(DangThucHien)) || !strings.Contains(s, string(ChoDuyet)) {
		t.Fatalf("nội dung nhật ký không nêu đủ hai mã trạng thái: %q", s)
	}
}

func TestLaLoiDauVaoNhiemVuKhongNuotLoiHeThong(t *testing.T) {
	// A default of "anything I do not recognise is the client's fault" turns a database outage into
	// a 400, and the client retries with different input for ever.
	if LaLoiDauVaoNhiemVu(errors.New("kết nối cơ sở dữ liệu hỏng")) {
		t.Fatal("lỗi hệ thống bị xếp thành lỗi đầu vào — sẽ trả 400 cho một sự cố máy chủ")
	}
	if !LaLoiDauVaoNhiemVu(ErrThieuTieuDeNhiemVu) {
		t.Fatal("ErrThieuTieuDeNhiemVu không được xếp là lỗi đầu vào")
	}
}
