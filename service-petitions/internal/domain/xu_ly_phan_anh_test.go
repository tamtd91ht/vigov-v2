package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// The staff-side vocabulary and shape checks — pure functions over values, tested without a database,
// a network or a commune's configuration.

// TestChinLabelDayDuVaKhongTrung — every one of the nine has a label, no two share one, and a string
// that is not one of the nine gets "" rather than itself.
//
// EMPTY AND NOT THE CODE: a caller that fell back to the raw code would put `cho-dan-xac-nhan` in
// front of a citizen as though it were Vietnamese. The receiving side refuses an empty
// `status_label` outright, which is a loud failure instead of a message nobody can read.
func TestChinLabelDayDuVaKhongTrung(t *testing.T) {
	chin := []TrangThai{
		DaTiepNhan, DangPhanLoai, DaChuyenXuLy, DangXuLy, DaXuLy,
		ChoDanXacNhan, DaDong, KhongTiepNhan, ChuyenCapTren,
	}
	thay := map[string]TrangThai{}
	for _, t2 := range chin {
		nhan := NhanTrangThai(t2)
		if nhan == "" {
			t.Errorf("trạng thái %q không có nhãn — bên nhận từ chối một nhãn rỗng, và người dân "+
				"không đọc được mã", t2)
			continue
		}
		if cu, co := thay[nhan]; co {
			t.Errorf("nhãn %q dùng cho cả %q và %q — báo cáo xuyên xã gộp theo trạng thái sẽ trộn "+
				"hai bước làm một", nhan, cu, t2)
		}
		thay[nhan] = t2
	}
	if len(thay) != 9 {
		t.Errorf("có %d nhãn, muốn 9", len(thay))
	}
	if NhanTrangThai(TrangThai("received")) != "" {
		t.Error("một mã KHÔNG thuộc chín trạng thái vẫn có nhãn — `received` là tên ĐÃ BỊ THAY " +
			"(ADR 0027), không phải cách gọi thứ hai đang song song")
	}
}

// The instants a notification may name. +07:00 on purpose: 02:30Z is 09:30 in Vietnam, so a sentence
// rendered in UTC would read "09:30" wrong by seven hours and this file would catch it.
var (
	hanTiepNhanMau = time.Date(2026, 9, 23, 2, 30, 0, 0, time.UTC) // 09:30 ngày 23/09/2026 giờ VN
	hanXuLyXongMau = time.Date(2026, 9, 30, 4, 20, 0, 0, time.UTC) // 11:20 ngày 30/09/2026 giờ VN
)

func phieuBaoMau(linhVuc string) PhieuPhanAnh {
	return PhieuPhanAnh{
		MaTraCuu:    "PA-9WDN-3HQK-72FM",
		LinhVuc:     linhVuc,
		NoiDung:     "Đống rác ở đầu ngõ đã ba ngày chưa ai dọn.",
		DiaChi:      "Đầu ngõ thôn Hà Lam",
		BoPhanID:    "bp-001",
		CanBoXuLyID: "CB-00999",
		KetQuaXuLy:  "Đội vệ sinh đã thu gom toàn bộ rác tại đầu ngõ.",
		HanTiepNhan: hanTiepNhanMau,
		HanXuLyXong: hanXuLyXongMau,
	}
}

// TestBangBaoChoDanDungQuyetDinh2409 — the owner's decision of 2026-09-24, over ALL NINE codes: which
// transitions owe the citizen a message and which do not. One table, so BaoChoDan and ViecTiepTheo
// must agree with it and with each other.
//
// ĐỘT BIẾN: thêm lại `DangXuLy` vào loiNhanChoDan, hoặc bỏ `ChoDanXacNhan`, và ca này ĐỎ.
func TestBangBaoChoDanDungQuyetDinh2409(t *testing.T) {
	muon := map[TrangThai]bool{
		DaTiepNhan:    true,
		DangPhanLoai:  false,
		DaChuyenXuLy:  true,
		DangXuLy:      false, // REMOVED 2026-09-24 — the unit+deadline message moved to da-chuyen-xu-ly
		DaXuLy:        false, // da-xu-ly -> cho-dan-xac-nhan carries the message
		ChoDanXacNhan: true,
		DaDong:        true,
		KhongTiepNhan: true, // no route yet; listed so it owes a message the day it is built
		ChuyenCapTren: true, // same
	}
	if len(muon) != 9 {
		t.Fatalf("bảng thử có %d mã, muốn đủ 9", len(muon))
	}
	p := phieuBaoMau("rac-thai")
	for t2, bao := range muon {
		if BaoChoDan(t2) != bao {
			t.Errorf("BaoChoDan(%q) = %v, muốn %v", t2, BaoChoDan(t2), bao)
		}
		viec := ViecTiepTheo(p, t2)
		if !bao {
			if viec != "" {
				t.Errorf("%q không được báo cho dân nhưng vẫn mang câu: %q", t2, viec)
			}
			continue
		}
		if strings.TrimSpace(viec) == "" {
			t.Errorf("%q phải báo cho dân nhưng câu rỗng — bên nhận bỏ tin, người dân không được báo", t2)
		}
		// THE RECEIVING SIDE REFUSES A `next_step` THAT MERELY REPEATS THE LABEL.
		if strings.EqualFold(strings.TrimSpace(viec), strings.TrimSpace(NhanTrangThai(t2))) {
			t.Errorf("%q: việc tiếp theo chỉ lặp lại nhãn", t2)
		}
	}
	if BaoChoDan(TrangThai("received")) || ViecTiepTheo(p, TrangThai("received")) != "" {
		t.Error("một mã không thuộc chín trạng thái lại được báo cho dân")
	}
}

// TestDangXuLyKhongConSinhLoiNhan — the one entry REMOVED by the decision, asserted on its own so the
// removal cannot be undone by a table edit that keeps the count right.
func TestDangXuLyKhongConSinhLoiNhan(t *testing.T) {
	if v := ViecTiepTheo(phieuBaoMau("rac-thai"), DangXuLy); v != "" {
		t.Errorf("dang-xu-ly vẫn sinh lời nhắn %q — quyết định 24/09 chuyển lời nhắn bộ phận + hạn sang "+
			"da-chuyen-xu-ly", v)
	}
}

// TestLoiNhanTiepNhanMangHanVaSoKhanCap — intake names the ACKNOWLEDGE deadline, in Vietnam time, and
// reminds the citizen that emergencies go to 113/114/115 (ADR 0028: the 2-hour fields are unreachable
// on this channel).
func TestLoiNhanTiepNhanMangHanVaSoKhanCap(t *testing.T) {
	v := ViecTiepTheo(phieuBaoMau(""), DaTiepNhan)
	for _, can := range []string{"09:30 ngày 23/09/2026", "113", "114", "115", "mã tra cứu"} {
		if !strings.Contains(v, can) {
			t.Errorf("lời nhắn tiếp nhận thiếu %q: %q", can, v)
		}
	}
	if strings.Contains(v, "02:30") {
		t.Errorf("hạn viết theo giờ UTC chứ không theo giờ Việt Nam: %q", v)
	}
	// THE RESOLVE DEADLINE DOES NOT EXIST AT INTAKE (ADR 0028 decision E) — naming it would invent it.
	if strings.Contains(v, "30/09/2026") {
		t.Errorf("lời nhắn tiếp nhận nêu hạn xử lý xong: %q", v)
	}
}

// TestLoiNhanChuyenXuLyMangHanXuLyXongKhongTenBoPhan — handed to a department: the resolve deadline,
// and "bộ phận chuyên môn" rather than an id (this service stores bo_phan_id only; see loiNhanChoDan).
func TestLoiNhanChuyenXuLyMangHanXuLyXongKhongTenBoPhan(t *testing.T) {
	v := ViecTiepTheo(phieuBaoMau("rac-thai"), DaChuyenXuLy)
	if !strings.Contains(v, "11:20 ngày 30/09/2026") {
		t.Errorf("thiếu hạn xử lý xong theo giờ Việt Nam: %q", v)
	}
	if !strings.Contains(v, "bộ phận chuyên môn") {
		t.Errorf("không nói phiếu đã chuyển bộ phận: %q", v)
	}
	if strings.Contains(v, "bp-001") || strings.Contains(v, "CB-00999") {
		t.Errorf("lời nhắn mang định danh nội bộ của bộ phận / cán bộ: %q", v)
	}
}

// TestLoiNhanHanDeTrongKhongInNamMot — a zero deadline must never render as the year 1, and must never
// empty the sentence (an empty sentence drops the whole message at the consumer).
func TestLoiNhanHanDeTrongKhongInNamMot(t *testing.T) {
	p := phieuBaoMau("rac-thai")
	p.HanTiepNhan, p.HanXuLyXong = time.Time{}, time.Time{}
	for _, t2 := range []TrangThai{DaTiepNhan, DaChuyenXuLy} {
		v := ViecTiepTheo(p, t2)
		if v == "" || strings.Contains(v, "0001") {
			t.Errorf("%q với hạn rỗng: %q", t2, v)
		}
	}
}

// TestLoiNhanLinhVucCanBoChiMaVaTrangThai — a report ABOUT a member of staff: code + status only. No
// deadline, no department, no result — at EVERY transition that owes a message.
//
// ĐỘT BIẾN: bỏ nhánh `LinhVucHanChe` khỏi ViecTiepTheo và ca này ĐỎ.
func TestLoiNhanLinhVucCanBoChiMaVaTrangThai(t *testing.T) {
	p := phieuBaoMau(LinhVucHanChe)
	for _, t2 := range []TrangThai{DaTiepNhan, DaChuyenXuLy, ChoDanXacNhan, DaDong, KhongTiepNhan, ChuyenCapTren} {
		v := ViecTiepTheo(p, t2)
		if v != loiNhanHanChe {
			t.Errorf("%q, lĩnh vực can-bo: %q, muốn đúng câu trung tính %q", t2, v, loiNhanHanChe)
		}
		for _, cam := range []string{"bộ phận", "bp-001", "CB-00999", "thu gom", "kết quả", "2026", "trước"} {
			if strings.Contains(v, cam) {
				t.Errorf("%q, lĩnh vực can-bo: lời nhắn mang %q", t2, cam)
			}
		}
	}
}

// TestLoiNhanKhongMangDuLieuCaNhanHayKetQua — no entry may draw on anything but the two deadlines: not
// the petition text, the address, the officer, the department id, or the staff-written result (the
// event contract forbids forwarding it; rule 3, invariant 6).
func TestLoiNhanKhongMangDuLieuCaNhanHayKetQua(t *testing.T) {
	p := phieuBaoMau("rac-thai")
	for t2 := range loiNhanChoDan {
		v := ViecTiepTheo(p, t2)
		for _, cam := range []string{"Đống rác", "Hà Lam", "bp-001", "CB-00999", "thu gom", "PA-9WDN"} {
			if strings.Contains(v, cam) {
				t.Errorf("%q: lời nhắn mang %q: %q", t2, cam, v)
			}
		}
	}
}

// TestTienTrinhChinhLaTapConCuaMayTrangThai — the plain advance may never name a move the lifecycle
// does not have. Two separate declarations, and the one that would drift is the narrow one.
func TestTienTrinhChinhLaTapConCuaMayTrangThai(t *testing.T) {
	for tu, sang := range tienTrinhChinh {
		if !tu.ChuyenSangDuoc(sang) {
			t.Errorf("tiến trình chính có %q -> %q, nhưng máy trạng thái không cho", tu, sang)
		}
	}
	// THE FOUR ABSENCES ARE THE DESIGN: each carries a decision and a permission of its own, so the
	// plain advance route must not be able to perform any of them.
	for _, tu := range []TrangThai{DaTiepNhan, DangPhanLoai, ChoDanXacNhan, DaDong,
		KhongTiepNhan, ChuyenCapTren} {
		if _, co := TienTrinhChinh(tu); co {
			t.Errorf("%q tiến được bằng tuyến chuyển trạng thái trống — bước ấy có quyền và dữ "+
				"liệu riêng", tu)
		}
	}
}

func TestSauKhiPhanCong(t *testing.T) {
	for tu, muon := range map[TrangThai]TrangThai{
		DangPhanLoai:  DaChuyenXuLy, // the FIRST routing IS the transition
		DaChuyenXuLy:  DaChuyenXuLy, // re-assignment leaves the status where it is (§8.5)
		DangXuLy:      DangXuLy,
		DaXuLy:        DaXuLy,
		ChoDanXacNhan: ChoDanXacNhan,
	} {
		sang, err := SauKhiPhanCong(tu)
		if err != nil {
			t.Errorf("SauKhiPhanCong(%q): %v", tu, err)
			continue
		}
		if sang != muon {
			t.Errorf("SauKhiPhanCong(%q) = %q, muốn %q", tu, sang, muon)
		}
	}

	// REFUSED FROM `da-tiep-nhan`: assigning an unclassified petition hands a department work whose
	// resolve deadline has not been fixed yet. REFUSED FROM the three end states: there is no
	// commitment left, and editing a closed petition is editing an archival record.
	for _, tu := range []TrangThai{DaTiepNhan, DaDong, KhongTiepNhan, ChuyenCapTren,
		TrangThai("khong-ton-tai")} {
		if _, err := SauKhiPhanCong(tu); !errors.Is(err, ErrPhanCongSaiLuc) {
			t.Errorf("SauKhiPhanCong(%q) không từ chối", tu)
		}
	}
}

// TestDongDuocTheoPhieu — owner's decision of 2026-09-24: a petition WITH a citizen closes only from
// `cho-dan-xac-nhan`; a petition with NOBODY to confirm (empty `cong_dan_id`) also closes from
// `da-xu-ly`, and that closing is reported as skipping the confirmation.
func TestDongDuocTheoPhieu(t *testing.T) {
	coDan := func(tt TrangThai) PhieuPhanAnh { return PhieuPhanAnh{TrangThai: tt, CongDanID: "cd-001"} }
	khongDan := func(tt TrangThai) PhieuPhanAnh { return PhieuPhanAnh{TrangThai: tt} }

	for ten, p := range map[string]PhieuPhanAnh{
		"có dân, chờ xác nhận":    coDan(ChoDanXacNhan),
		"không dân, chờ xác nhận": khongDan(ChoDanXacNhan),
	} {
		bo, err := DongDuoc(p)
		if err != nil || bo {
			t.Errorf("%s: bỏ qua=%v lỗi=%v, muốn false/nil", ten, bo, err)
		}
	}
	if bo, err := DongDuoc(khongDan(DaXuLy)); err != nil || !bo {
		t.Errorf("không dân, da-xu-ly: bỏ qua=%v lỗi=%v, muốn true/nil", bo, err)
	}
	if _, err := DongDuoc(coDan(DaXuLy)); !errors.Is(err, ErrDongSaiLuc) {
		t.Error("đóng được phiếu CÓ công dân từ da-xu-ly — phiếu bị đóng trước khi người dân được hỏi")
	}
	for _, tu := range []TrangThai{DaTiepNhan, DangPhanLoai, DaChuyenXuLy, DangXuLy,
		DaDong, KhongTiepNhan, ChuyenCapTren, TrangThai("khong-ton-tai")} {
		for _, p := range []PhieuPhanAnh{coDan(tu), khongDan(tu)} {
			if _, err := DongDuoc(p); !errors.Is(err, ErrDongSaiLuc) {
				t.Errorf("đóng được từ %q (công dân=%q)", tu, p.CongDanID)
			}
		}
	}
}

// TestKiemKetQuaTuChoiCaiGiongMotKetQua is rule 10, invariant 6 in its sharp form: the refusals that
// matter are not the empty string, they are the strings that LOOK like a result.
func TestKiemKetQuaTuChoiCaiGiongMotKetQua(t *testing.T) {
	for ten, vao := range map[string]string{
		"rỗng":     "",
		"toàn dấu": "  \n\t ",
		"ok":       "ok",
		"xong":     "xong",
		"đã xử lý": "đã xử lý",
	} {
		t.Run(ten, func(t *testing.T) {
			if _, err := KiemKetQua(vao); err == nil {
				t.Fatalf("chấp nhận %q làm kết quả cho người dân đọc — một người dân chỉ được báo "+
					"rằng phiếu đã đóng thì không phân biệt được được giúp với bị cho qua", vao)
			}
		})
	}

	// A real sentence is accepted AND TRIMMED. Trimmed before it is measured, so a textarea holding
	// newlines around a real answer is not refused.
	ra, err := KiemKetQua("  Đội vệ sinh đã thu gom toàn bộ rác tại đầu ngõ ngày 24/9.\n")
	if err != nil {
		t.Fatalf("từ chối một kết quả thật: %v", err)
	}
	if strings.HasPrefix(ra, " ") || strings.HasSuffix(ra, "\n") {
		t.Errorf("kết quả chưa được cắt khoảng trắng: %q", ra)
	}

	if _, err := KiemKetQua(strings.Repeat("a", KetQuaToiDa+1)); !errors.Is(err, ErrKetQuaQuaDai) {
		t.Errorf("không chặn kết quả quá dài: %v", err)
	}
}

// TestKiemLinhVucChiKiemHINHDANG pins the KNOWN HOLE rather than hiding it: the code's SHAPE is
// checked and its EXISTENCE is not, because ADR 0026 stop condition #2 leaves `petitions` with no way
// to read the tier-1 set from `platform`.
//
// THE CASE THAT ACCEPTS `khong-co-that` IS DELIBERATE AND IS THE FINDING. Change it the day a read
// path for the code set exists; until then a test asserting the opposite would be a test claiming a
// check that is not there.
func TestKiemLinhVucChiKiemHINHDANG(t *testing.T) {
	for _, hopLe := range []string{"rac-thai", "an-toan-thuc-pham", "khac", "khong-co-that"} {
		if _, err := KiemLinhVuc(hopLe); err != nil {
			t.Errorf("từ chối mã đúng hình dạng %q: %v", hopLe, err)
		}
	}
	for ten, sai := range map[string]string{
		"rỗng":      "",
		"toàn dấu":  "   ",
		"chữ hoa":   "Rac-Thai",
		"gạch đầu":  "-rac-thai",
		"gạch cuối": "rac-thai-",
		"gạch đôi":  "rac--thai",
		"gạch dưới": "rac_thai",
		"có dấu":    "rác-thải",
		"có khoảng": "rac thai",
		"quá dài":   strings.Repeat("a", LinhVucToiDa+1),
	} {
		t.Run(ten, func(t *testing.T) {
			if _, err := KiemLinhVuc(sai); err == nil {
				t.Fatalf("chấp nhận %q làm mã lĩnh vực", sai)
			}
		})
	}
}

func TestKiemPhanCongBatBuocBoPhanVaChoPhepTrongCanBo(t *testing.T) {
	// "— Để bộ phận phân công —" is a real choice on the screen (docs/ui-ux/09 §8.5).
	bp, cb, err := KiemPhanCong("  bp-001 ", "")
	if err != nil {
		t.Fatalf("từ chối phân công không nêu cán bộ: %v", err)
	}
	if bp != "bp-001" || cb != "" {
		t.Errorf("bộ phận=%q cán bộ=%q", bp, cb)
	}

	// A petition assigned to NO DEPARTMENT is a petition nobody is answerable for, with a deadline
	// already running.
	if _, _, err := KiemPhanCong("   ", "CB-1"); !errors.Is(err, ErrThieuBoPhan) {
		t.Errorf("chấp nhận phân công không có bộ phận: %v", err)
	}
	if _, _, err := KiemPhanCong(strings.Repeat("a", BoPhanToiDa+1), ""); !errors.Is(err, ErrBoPhanQuaDai) {
		t.Errorf("không chặn bộ phận quá dài: %v", err)
	}
}

// TestLaLoiXuLyPhanAnhKhongGomLoiTrangThai — a refusal about the STATE OF THE RECORD is a 409 and a
// refusal of what the caller TYPED is a 400. Folding them together would tell an officer to fix a
// form that is perfectly correct.
func TestLaLoiXuLyPhanAnhKhongGomLoiTrangThai(t *testing.T) {
	for _, dungLa400 := range []error{ErrThieuLinhVuc, ErrLinhVucSaiDang, ErrThieuBoPhan,
		ErrThieuKetQua, ErrKetQuaQuaNgan, ErrKetQuaQuaDai} {
		if !LaLoiXuLyPhanAnh(dungLa400) {
			t.Errorf("%v không được nhận là lỗi đầu vào", dungLa400)
		}
	}
	for _, dungLa409 := range []error{ErrPhanCongSaiLuc, ErrDongSaiLuc, ErrKhongConCamKet} {
		if LaLoiXuLyPhanAnh(dungLa409) {
			t.Errorf("%v bị nhận là lỗi đầu vào — nó là lỗi TRẠNG THÁI, và 400 ở đó bảo cán bộ "+
				"sửa một biểu mẫu vốn đã đúng", dungLa409)
		}
	}
	// A failure of the SYSTEM must never be read as the caller's fault: a default of "anything I do
	// not recognise is a 400" turns a database outage into a form error.
	if LaLoiXuLyPhanAnh(errors.New("kết nối cơ sở dữ liệu hỏng")) {
		t.Error("một lỗi hệ thống bị nhận là lỗi đầu vào")
	}
}
