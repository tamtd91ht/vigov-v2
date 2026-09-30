package domain

import (
	"errors"
	"testing"
	"time"
)

// The task lifecycle and the two derived deadline questions.
//
// WHAT THESE PROVE, AND IT IS THE HALF THAT NEVER REACHES A DATABASE: the seven codes are the seven
// of §6 and nothing else · the map admits exactly the moves of vigov-require 52ec9b5's table minus
// `chuyen-tiep` as a target (user decision 28/09/2026) · no status is terminal · late is DERIVED and a task finished late STAYS late · and the ONE
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

// requireTable is vigov-require's ALLOWED_TRANSITIONS at anchor 0053854
// (apps/api/app/modules/tasks/service.py:63-102, commit 52ec9b5), TRANSCRIBED BY HAND with the
// mapping NEW=moi-giao, ACCEPTED=da-tiep-nhan, IN_PROGRESS=dang-thuc-hien, PENDING_APPROVAL=cho-duyet,
// DONE=hoan-thanh, PAUSED=tam-dung, TRANSFERRED=chuyen-tiep — and with the ONE departure the user
// decided on 28/09/2026: `chuyen-tiep` removed as a TARGET (forwarding is the assignment act). A
// second copy of the table on purpose: an edit to the map that nobody meant fails here.
var requireTable = map[TrangThaiNhiemVu][]TrangThaiNhiemVu{
	MoiGiao:      {DaTiepNhanNV, DangThucHien, TamDung}, // require also: TRANSFERRED
	DaTiepNhanNV: {DangThucHien, TamDung},               // require also: TRANSFERRED
	DangThucHien: {HoanThanh, ChoDuyet, TamDung},        // require also: TRANSFERRED
	ChoDuyet:     {HoanThanh, DangThucHien},             //
	TamDung:      {DangThucHien, DaTiepNhanNV, MoiGiao}, //
	ChuyenTiep:   {DaTiepNhanNV, DangThucHien},          //
	HoanThanh:    {DangThucHien},                        // the reopen
}

// TestLifecycleIsRequireTable pins EVERY pair of the 7×7 grid against requireTable.
//
// ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026: this replaces TestLuongChinhTheoDungSoDoDacTa,
// TestTraLaiLamTiepLaMuiTenLuiDuyNhat, TestHaiTrangThaiKetThuc and TestTamDungChiVeDuocTrangThaiTruoc,
// which pinned the strict chain of §6: no skipping (moi-giao → dang-thuc-hien, dang-thuc-hien →
// hoan-thanh were refused), no reopen, `hoan-thanh` and `chuyen-tiep` terminal, resume only to the
// state before the pause. The user decided to follow vigov-require 52ec9b5 instead.
func TestLifecycleIsRequireTable(t *testing.T) {
	all := []TrangThaiNhiemVu{MoiGiao, DaTiepNhanNV, DangThucHien, ChoDuyet, HoanThanh, TamDung, ChuyenTiep}
	for _, from := range all {
		for _, to := range all {
			want := false
			for _, m := range requireTable[from] {
				if m == to {
					want = true
				}
			}
			if got := from.ChuyenSangDuoc(to); got != want {
				t.Errorf("%s -> %s: ChuyenSangDuoc = %v, bảng require (đã bỏ đích chuyen-tiep) = %v",
					from, to, got, want)
			}
		}
	}
}

// TestNewEdgesOfRequire names each edge the strict chain did not have, so a regression names the
// edge rather than a grid cell.
func TestNewEdgesOfRequire(t *testing.T) {
	for _, c := range []struct{ from, to TrangThaiNhiemVu }{
		{MoiGiao, DangThucHien},    // skip acknowledgement
		{DangThucHien, HoanThanh},  // skip review (no key since ADR 0065 NV1)
		{HoanThanh, DangThucHien},  // reopen (needs task.approve)
		{ChuyenTiep, DaTiepNhanNV}, // legacy forwarded row moves on
		{ChuyenTiep, DangThucHien}, // legacy forwarded row moves on
		{TamDung, MoiGiao},         // resume regardless of history
		{TamDung, DaTiepNhanNV},    //
		{TamDung, DangThucHien},    //
	} {
		if err := ChuyenTrangThaiDuoc(c.from, c.to); err != nil {
			t.Errorf("%s -> %s bị từ chối: %v", c.from, c.to, err)
		}
	}
	// And the edges require does NOT have, which the old map did.
	for _, c := range []struct{ from, to TrangThaiNhiemVu }{
		{ChoDuyet, TamDung}, // require's PENDING_APPROVAL has no PAUSED
		{TamDung, ChoDuyet}, // nothing resumes into review
	} {
		if err := ChuyenTrangThaiDuoc(c.from, c.to); !errors.Is(err, ErrChuyenTrangThaiNhiemVuSaiLuc) {
			t.Errorf("%s -> %s: lỗi = %v, muốn ErrChuyenTrangThaiNhiemVuSaiLuc", c.from, c.to, err)
		}
	}
}

// TestNoStatusIsTerminal: every status has a way out since 28/09/2026 — which is why callers that
// mean "finished" name `hoan-thanh` (CheckAssignable, ConChuaXong).
func TestNoStatusIsTerminal(t *testing.T) {
	for m := range chuyenDuocSangNhiemVu {
		if len(m.AllowedTransitions()) == 0 {
			t.Errorf("%s không có bước ra — ngõ cụt", m)
		}
	}
	// The code itself stays VALID and is never a target (rule 7 for the rows that hold it).
	if !ChuyenTiep.HopLe() {
		t.Error("chuyen-tiep phải còn là mã hợp lệ cho các dòng cũ")
	}
	for m := range chuyenDuocSangNhiemVu {
		if m.ChuyenSangDuoc(ChuyenTiep) {
			t.Errorf("%s vẫn chuyển sang chuyen-tiep được — chuyển tiếp nay là thao tác giao lại", m)
		}
	}
}

// TestAllowedTransitionsIsTheMap: the list a screen draws is the map the write path enforces — for
// every status, the list holds exactly the targets ChuyenTrangThaiDuoc accepts.
func TestAllowedTransitionsIsTheMap(t *testing.T) {
	all := []TrangThaiNhiemVu{MoiGiao, DaTiepNhanNV, DangThucHien, ChoDuyet, HoanThanh, TamDung, ChuyenTiep}
	for _, from := range all {
		listed := map[TrangThaiNhiemVu]bool{}
		for _, m := range from.AllowedTransitions() {
			listed[m] = true
		}
		for _, to := range all {
			accepted := ChuyenTrangThaiDuoc(from, to) == nil
			if accepted != listed[to] {
				t.Errorf("%s -> %s: danh sách = %v, đường ghi chấp nhận = %v", from, to, listed[to], accepted)
			}
		}
	}
	// A fresh slice: editing it must not edit the lifecycle.
	l := MoiGiao.AllowedTransitions()
	l[0] = HoanThanh
	if MoiGiao.AllowedTransitions()[0] != DaTiepNhanNV {
		t.Error("AllowedTransitions trả lát cắt của chính bản đồ — sửa nó là sửa vòng đời")
	}
	// Unknown status: an empty list, never nil.
	if l := TrangThaiNhiemVu("ma-cu-la").AllowedTransitions(); l == nil || len(l) != 0 {
		t.Errorf("trạng thái lạ: %v, muốn [] không nil", l)
	}
}

// TestNeedsApproval names the three moves that need `task.approve` and pins that the ordinary steps
// do not (user decision 28/09/2026, the owner's return decision of 27/09, and ADR 0065 NV1 of 30/09).
func TestNeedsApproval(t *testing.T) {
	for _, c := range []struct {
		from, to TrangThaiNhiemVu
		want     bool
	}{
		{ChoDuyet, HoanThanh, true},
		// ĐỔI CHIỀU CÓ CHỦ Ý 30/09/2026 (ADR 0065 NV1): review is optional, the direct completion needs
		// no key. Work already AT cho-duyet still waits for a holder of task.approve (row above).
		{DangThucHien, HoanThanh, false},
		{HoanThanh, DangThucHien, true}, // reopen
		{ChoDuyet, DangThucHien, true},  // return
		{MoiGiao, DangThucHien, false},
		{DaTiepNhanNV, DangThucHien, false},
		{TamDung, DangThucHien, false},
		{ChuyenTiep, DangThucHien, false},
		{DangThucHien, ChoDuyet, false},
		{DangThucHien, TamDung, false},
	} {
		if got := NeedsApproval(c.from, c.to); got != c.want {
			t.Errorf("NeedsApproval(%s, %s) = %v, muốn %v", c.from, c.to, got, c.want)
		}
	}
	if !IsReopen(HoanThanh, DangThucHien) || IsReopen(ChoDuyet, DangThucHien) || IsReopen(TamDung, DangThucHien) {
		t.Error("IsReopen phải nhận đúng một bước hoan-thanh -> dang-thuc-hien")
	}
	if !LaTraLaiLamTiep(ChoDuyet, DangThucHien) || LaTraLaiLamTiep(HoanThanh, DangThucHien) {
		t.Error("LaTraLaiLamTiep phải nhận đúng một bước cho-duyet -> dang-thuc-hien")
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
