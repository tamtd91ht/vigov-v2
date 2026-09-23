package domain

import (
	"testing"
	"time"
)

// The task lifecycle and the two derived deadline questions.
//
// WHAT THESE PROVE, AND IT IS THE HALF THAT NEVER REACHES A DATABASE: the seven codes are the seven
// of §6 and nothing else · the map admits exactly the moves §6 draws · `hoan-thanh` and
// `chuyen-tiep` are terminal · late is DERIVED and a task finished late STAYS late · and the ONE
// distinction most likely to produce a plausible wrong figure — `han_xu_ly` for "is it late now"
// against `han_ban_dau` for the on-time ratio.

// The fixture instants are FIXED, not relative to time.Now(): a deadline expressed as "two hours
// ago" makes the assertions depend on when the suite runs.
var (
	mocHanNV    = time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	mocGiaHanNV = time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	mocTruocHan = time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
	mocSauHan   = time.Date(2026, 6, 25, 8, 0, 0, 0, time.UTC)
)

func TestBayMaTrangThaiVaKhongThemMaNao(t *testing.T) {
	// THE LIST IS THE CONTRACT. Open question #21 decided a commune may change the label and the
	// order and NEVER the codes (ADR 0035 §C), so an eighth code appearing here is a decision
	// somebody took without the customer — and the schema's CHECK would refuse it at the first
	// write, on a screen, rather than here.
	muon := []TrangThaiNhiemVu{
		"moi-giao", "da-tiep-nhan", "dang-thuc-hien", "cho-duyet", "hoan-thanh",
		"tam-dung", "chuyen-tiep",
	}
	if len(chuyenDuocSangNhiemVu) != len(muon) {
		t.Fatalf("có %d trạng thái, muốn %d — danh sách mã là mã ĐÓNG (câu hỏi mở #21)",
			len(chuyenDuocSangNhiemVu), len(muon))
	}
	for _, m := range muon {
		if !m.HopLe() {
			t.Errorf("thiếu mã %q", m)
		}
	}
	for _, xau := range []TrangThaiNhiemVu{
		// Spellings that look right and are not. `chua-thuc-hien` is what the KANBAN BOARD calls
		// `moi-giao` (§4.1) — a LABEL, and the one most likely to be typed as a code by somebody
		// reading the screen instead of §6.
		"chua-thuc-hien", "moi_giao", "hoan-thanh-tre-han", "cho-duyet-lui-han", "",
	} {
		if xau.HopLe() {
			t.Errorf("nhận mã %q ngoài bảy mã đã chốt", xau)
		}
	}
}

// TestChoDuyetLuiHanKhongPhaiTrangThai is the one the BA/PM note paid for
// (kb/50-doi-chieu/2026-09-23-feat-m8-multitenant-foundation.md §M1): folding "Chờ duyệt lùi hạn"
// into the status set makes the progress board COUNT WRONG, because every task awaiting a decision
// leaves the column it is actually in.
func TestChoDuyetLuiHanKhongPhaiTrangThai(t *testing.T) {
	for _, m := range []TrangThaiNhiemVu{"cho-duyet-lui-han", "cho-duyet-gia-han", "de-nghi-lui-han"} {
		if m.HopLe() {
			t.Errorf("%q là một trạng thái nhiệm vụ — nó phải là NHÃN đọc từ bảng de_nghi_lui_han, "+
				"nếu không bảng tiến độ đếm sai", m)
		}
	}
}

func TestLuongChinhTheoDungSoDoDacTa(t *testing.T) {
	// §6, :227 read left to right. Each step and NOTHING ELSE on the main flow.
	for _, c := range []struct{ tu, den TrangThaiNhiemVu }{
		{MoiGiao, DaTiepNhanNV},
		{DaTiepNhanNV, DangThucHien},
		{DangThucHien, ChoDuyet},
		{ChoDuyet, HoanThanh},
	} {
		if !c.tu.ChuyenSangDuoc(c.den) {
			t.Errorf("%s -> %s bị từ chối, sơ đồ §6 có bước này", c.tu, c.den)
		}
	}

	// SKIPPING A STEP IS REFUSED. A task that jumped to `hoan-thanh` is finished work nobody did,
	// and §11.3's on-time ratio would count it.
	for _, c := range []struct{ tu, den TrangThaiNhiemVu }{
		{MoiGiao, DangThucHien},
		{MoiGiao, HoanThanh},
		{DaTiepNhanNV, HoanThanh},
		{DangThucHien, HoanThanh},
		// And backwards: §6 draws no arrow the other way on the main flow.
		{DangThucHien, DaTiepNhanNV},
		{HoanThanh, DangThucHien},
	} {
		if c.tu.ChuyenSangDuoc(c.den) {
			t.Errorf("%s -> %s được nhận, sơ đồ §6 không có bước này", c.tu, c.den)
		}
	}
}

func TestHaiNhanhRoiDuocTuMoiBuocChuaXong(t *testing.T) {
	// §6, :229-231: the fan-in. Both branches leave from every unfinished main state.
	for _, tu := range []TrangThaiNhiemVu{MoiGiao, DaTiepNhanNV, DangThucHien, ChoDuyet} {
		if !tu.ChuyenSangDuoc(TamDung) {
			t.Errorf("%s không tạm dừng được", tu)
		}
		if !tu.ChuyenSangDuoc(ChuyenTiep) {
			t.Errorf("%s không chuyển tiếp được", tu)
		}
	}
}

func TestHaiTrangThaiKetThuc(t *testing.T) {
	for _, m := range []TrangThaiNhiemVu{HoanThanh, ChuyenTiep} {
		if !m.KetThuc() {
			t.Errorf("%s không phải trạng thái kết thúc", m)
		}
	}
	for _, m := range []TrangThaiNhiemVu{MoiGiao, DaTiepNhanNV, DangThucHien, ChoDuyet, TamDung} {
		if m.KetThuc() {
			t.Errorf("%s bị coi là kết thúc — việc rơi vào đó sẽ nằm lại vĩnh viễn", m)
		}
	}
}

// TestTamDungChiVeDuocTrangThaiTruoc guards the ONE entry that is not a plain lookup.
//
// §6 says a paused task resumes at "(trạng thái trước)". `ChuyenSangDuoc` answers SHAPE and is
// deliberately loose; `TamDungVeDuoc` is the rule, and it is what stops a pause from becoming a
// shortcut to `hoan-thanh`.
func TestTamDungChiVeDuocTrangThaiTruoc(t *testing.T) {
	for _, m := range []TrangThaiNhiemVu{MoiGiao, DaTiepNhanNV, DangThucHien, ChoDuyet} {
		if !TamDungVeDuoc(m) {
			t.Errorf("tạm dừng không về được %s", m)
		}
	}
	for _, m := range []TrangThaiNhiemVu{HoanThanh, ChuyenTiep, TamDung, "moi-nghi-ra"} {
		if TamDungVeDuoc(m) {
			t.Errorf("tạm dừng về được %s — việc chưa ai làm sẽ thành việc đã xong", m)
		}
	}
	// And the shape half really is looser than the rule, or the two functions would be one.
	if TamDung.ChuyenSangDuoc(HoanThanh) {
		t.Error("sơ đồ cho tạm dừng nhảy thẳng sang hoàn thành")
	}
}

func TestNamCotKanban(t *testing.T) {
	// §4.1: the two branch states have no column of their own.
	for _, m := range []TrangThaiNhiemVu{MoiGiao, DaTiepNhanNV, DangThucHien, ChoDuyet, HoanThanh} {
		if !m.LaTrangThaiChinh() {
			t.Errorf("%s không phải cột Kanban, §4.1 vẽ năm cột", m)
		}
	}
	for _, m := range []TrangThaiNhiemVu{TamDung, ChuyenTiep} {
		if m.LaTrangThaiChinh() {
			t.Errorf("%s có cột riêng trên Kanban — §4.1 nói hai nhánh rẽ KHÔNG có cột", m)
		}
	}
}

func TestBonNguonGiao(t *testing.T) {
	for _, m := range []NguonGiao{NguonTrucTiep, NguonKetLuanHop, NguonVanBanDen, NguonPhanAnh} {
		if !m.HopLe() {
			t.Errorf("thiếu nguồn giao %q", m)
		}
	}
	for _, xau := range []NguonGiao{"", "excel", "don-thu", "truc_tiep"} {
		if xau.HopLe() {
			t.Errorf("nhận nguồn giao %q ngoài bốn mã", xau)
		}
	}
}

// --- the derived deadline questions ---------------------------------------------------------------

func TestTreHanLaSuyRa(t *testing.T) {
	t.Run("chưa có hạn thì KHÔNG trễ", func(t *testing.T) {
		// A statement about the COMMITMENT, not about the work: §4.1 renders it `Hạn —`, and nothing
		// was promised to anybody.
		var n NhiemVu
		if n.TreHan(mocSauHan) {
			t.Error("nhiệm vụ không có hạn bị tính là trễ")
		}
	})

	t.Run("chưa xong: so với đồng hồ", func(t *testing.T) {
		n := NhiemVu{HanXuLy: mocHanNV, HanBanDau: mocHanNV}
		if n.TreHan(mocTruocHan) {
			t.Error("trước hạn mà bị tính là trễ")
		}
		if !n.TreHan(mocSauHan) {
			t.Error("sau hạn mà không bị tính là trễ")
		}
	})

	t.Run("đã xong TRỄ thì VẪN trễ mãi", func(t *testing.T) {
		// Otherwise a commune's late work quietly stops being late as soon as it is done, and last
		// quarter's figure changes every time somebody opens the screen.
		n := NhiemVu{
			HanXuLy: mocHanNV, HanBanDau: mocHanNV,
			TrangThai: HoanThanh, NgayHoanThanh: mocSauHan,
		}
		// `now` is years later and must change nothing.
		if !n.TreHan(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)) {
			t.Error("việc làm trễ rồi xong lại hết trễ")
		}
	})

	t.Run("đã xong ĐÚNG HẠN thì không trễ", func(t *testing.T) {
		n := NhiemVu{
			HanXuLy: mocHanNV, HanBanDau: mocHanNV,
			TrangThai: HoanThanh, NgayHoanThanh: mocTruocHan,
		}
		if n.TreHan(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)) {
			t.Error("việc xong trước hạn bị tính là trễ")
		}
	})
}

// TestHaiCauHoiHanKhacNhau is the assertion this file exists for.
//
// A task whose deadline was EXTENDED and then finished before the NEW date is:
//
//	TreHan                  false — it is not late as things stand today
//	HoanThanhDungHanBanDau  false — §11.3's ratio measures against the date FIRST promised
//
// Reading either answer as the other produces a figure that is plausible, wrong, and reported
// upward: measuring the on-time ratio against `han_xu_ly` would make every granted extension read
// as a deadline met, which is exactly what §5.8 promises on the screen will NOT happen.
func TestHaiCauHoiHanKhacNhau(t *testing.T) {
	n := NhiemVu{
		HanBanDau:     mocHanNV,    // promised 20/6
		HanXuLy:       mocGiaHanNV, // extended to 15/7
		TrangThai:     HoanThanh,
		NgayHoanThanh: time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC), // finished 10/7
	}

	if n.TreHan(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Error("TreHan = true — việc xong TRƯỚC hạn hiện hành thì không trễ")
	}
	dungHan, laMau := n.HoanThanhDungHanBanDau()
	if !laMau {
		t.Fatal("không được tính là mẫu dù đã hoàn thành và có hạn ban đầu")
	}
	if dungHan {
		t.Error("tỷ lệ đúng hạn đo theo hạn ĐÃ LÙI — §11.3 đo theo `han_ban_dau`, và §5.8 hứa " +
			"với người dùng rằng hạn gốc không bị lùi theo")
	}
	if !n.DaGiaHan() {
		t.Error("DaGiaHan = false dù hai mốc hạn khác nhau")
	}
}

func TestChuaHoanThanhThiChuaPhaiMauCuaTyLeDungHan(t *testing.T) {
	// A running task belongs in NEITHER the numerator nor the denominator. Folding it into either
	// is how a commune with a lot of unfinished work reports a better ratio than one that finishes
	// things late.
	n := NhiemVu{HanXuLy: mocHanNV, HanBanDau: mocHanNV, TrangThai: DangThucHien}
	if _, laMau := n.HoanThanhDungHanBanDau(); laMau {
		t.Error("việc chưa xong được tính vào mẫu tỷ lệ đúng hạn")
	}
	// And a finished task with no deadline at all is not a sample either: nothing was promised.
	k := NhiemVu{TrangThai: HoanThanh, NgayHoanThanh: mocTruocHan}
	if _, laMau := k.HoanThanhDungHanBanDau(); laMau {
		t.Error("việc không có hạn ban đầu được tính vào mẫu tỷ lệ đúng hạn")
	}
}

func TestDaGiaHanVaChuaPhanCong(t *testing.T) {
	if (NhiemVu{HanXuLy: mocHanNV, HanBanDau: mocHanNV}).DaGiaHan() {
		t.Error("hai mốc bằng nhau lại báo đã gia hạn")
	}
	if (NhiemVu{}).DaGiaHan() {
		t.Error("nhiệm vụ không có hạn lại báo đã gia hạn")
	}
	// ChuaPhanCong asks about the PERSON, not the department: §11.1 makes "handed to a department
	// with nobody named" the case the system has to report, so the two states must stay distinct.
	if !(NhiemVu{BoPhanID: "bp-001"}).ChuaPhanCong() {
		t.Error("giao cho bộ phận mà chưa có người lại bị coi là đã phân công")
	}
	if (NhiemVu{NguoiThucHienMa: "CB-00123"}).ChuaPhanCong() {
		t.Error("đã có người thực hiện mà vẫn báo chưa phân công")
	}
}
