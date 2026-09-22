package domain

import (
	"sort"
	"strings"
	"testing"
)

// WHAT THIS FILE IS FOR: the seed set is a TABLE OF NUMBERS TRANSCRIBED BY HAND from
// docs/ui-ux/14-cau-hinh.md §8, and a transcription defect here is invisible in every other way.
// The code compiles, the seeding route works, the screen renders — and one commune promises its
// citizens something the authority never agreed to. There is no constraint that could catch it and
// no other test that would go red.
//
// THE CASES BELOW ARE THEREFORE ABOUT THE SPECIFICATION, NOT ABOUT THE CODE: the count, the twelve
// field codes, the dropped dead code, the default rows, and one full row checked figure by figure.

// laBoGieoTheoKhoa indexes the seed set by (kind of work, field code).
func laBoGieoTheoKhoa(t *testing.T) map[string]GieoSLA {
	t.Helper()
	ra := map[string]GieoSLA{}
	for _, g := range BoGieoSLA() {
		k := string(g.LoaiViec) + "/" + g.LinhVuc
		if _, trung := ra[k]; trung {
			// A DUPLICATE PAIR WOULD NOT FAIL AT COMPILE TIME and would reach the database as a
			// unique-key violation at a commune's very first configuration — the worst moment to
			// find out. It is checked here, where the list is.
			t.Fatalf("bộ gieo có hai dòng cùng (loại việc, lĩnh vực) %q — khoá duy nhất sẽ từ chối dòng thứ hai", k)
		}
		ra[k] = g
	}
	return ra
}

// FIFTEEN ROWS AND NOT SIXTEEN. The specification's table has sixteen; :308 is `ve-sinh-moi-truong`,
// which it labels itself as a dead code.
//
// MUTATION THAT MUST TURN THIS RED: add the `ve-sinh-moi-truong` row back to BoGieoSLA.
func TestBoGieoSLACoDungMuoiLamDong(t *testing.T) {
	if n := len(BoGieoSLA()); n != 15 {
		t.Fatalf("bộ gieo có %d dòng, muốn 15 (bảng 14-cau-hinh.md §8 có 16 dòng, trừ dòng mã cũ :308)", n)
	}
}

// THE DEAD CODE IS ABSENT. Seeding it would manufacture on day one the exact defect ADR 0026 §2 and
// ADR 0024 cite the specification as evidence of: a live SLA row pointing at a code no catalogue
// holds, rendered raw on the screen.
func TestBoGieoSLAKhongGieoMaCu(t *testing.T) {
	for _, g := range BoGieoSLA() {
		if g.LinhVuc == "ve-sinh-moi-truong" {
			t.Fatal("bộ gieo còn `ve-sinh-moi-truong` — mã cũ, 14-cau-hinh.md:308 ghi rõ không còn " +
				"trong danh mục; nó là mã cũ của `rac-thai` (cùng nhãn, cùng năm con số)")
		}
	}
}

// ALL TWELVE TIER-1 FIELD CODES APPEAR, EXACTLY ONCE EACH.
//
// THIS IS THE CASE THAT CATCHES A MAPPING THAT IS ONE SHORT — the specification lists fields by
// LABEL and the codes come from docs/ui-ux/09-phan-anh-nguoi-dan.md §5, so the transcription is a
// label-to-code lookup done by hand. A field missing here is a field a commune silently has no
// deadline for: DongTheoLinhVuc falls back to the default row, so nothing errors and the promise is
// simply a different one.
//
// MUTATION THAT MUST TURN THIS RED: drop any one `phan-anh` row, or misspell a code.
func TestBoGieoSLAPhuDuMuoiHaiLinhVuc(t *testing.T) {
	// The closed tier-1 set, docs/ui-ux/09-phan-anh-nguoi-dan.md §5. Written out rather than
	// imported: there is nothing to import — ADR 0026 places the code set in service `platform` and
	// that table does not exist yet. See the report of the turn that added this file.
	muon := []string{
		"an-ninh", "an-toan-thuc-pham", "can-bo", "cap-thoat-nuoc",
		"dien", "giao-thong", "khac", "o-nhiem",
		"rac-thai", "trat-tu-do-thi", "xay-dung", "y-te-giao-duc",
	}

	var co []string
	for _, g := range BoGieoSLA() {
		if g.LoaiViec == LoaiViecPhanAnh && g.LinhVuc != "" {
			co = append(co, g.LinhVuc)
		}
	}
	sort.Strings(co)

	if strings.Join(co, ",") != strings.Join(muon, ",") {
		t.Fatalf("mã lĩnh vực của bộ gieo:\n  có  = %v\n  muốn = %v", co, muon)
	}
}

// EVERY KIND OF WORK HAS ITS DEFAULT ROW, and this is the one that is invisible on the screen: a
// commune whose `phan-anh` rows are all present except the default LOOKS configured, and every
// petition a citizen sends gets no deadline at all — ADR 0028 decision E reads `gio_tiep_nhan` from
// the default row precisely because the field is not known at that instant.
//
// MUTATION THAT MUST TURN THIS RED: drop the `{LoaiViecPhanAnh, "", ...}` row.
func TestBoGieoSLACoDongMacDinhChoCaBaLoaiViec(t *testing.T) {
	theo := laBoGieoTheoKhoa(t)
	for _, lv := range []LoaiViec{LoaiViecVanBanDen, LoaiViecPhanAnh, LoaiViecNhiemVu} {
		if _, co := theo[string(lv)+"/"]; !co {
			t.Errorf("bộ gieo thiếu dòng mặc định của loại việc %q — lĩnh vực nào không có dòng "+
				"riêng sẽ không tính được hạn, và kênh công dân thì KHÔNG BAO GIỜ tính được (ADR 0028 quyết định E)", lv)
		}
	}
}

// THE SEED SET PASSES THE SAME VALIDATION A TYPED FIGURE DOES. A row the CHECK constraint would
// refuse turns a commune's first configuration into a failed transaction with no sentence saying
// which row.
func TestBoGieoSLAMoiDongHopLe(t *testing.T) {
	for _, g := range BoGieoSLA() {
		if !g.LoaiViec.HopLe() {
			t.Errorf("dòng %q/%q: loại việc không nằm trong ba giá trị được phép", g.LoaiViec, g.LinhVuc)
		}
		d := DongSLA{
			GioTiepNhan: g.GioTiepNhan, GioXuLyXong: g.GioXuLyXong,
			GioSapDenHan: g.GioSapDenHan, GioBaoLanhDao: g.GioBaoLanhDao,
			GioBaoChuTich: g.GioBaoChuTich,
		}
		if err := KiemTraDongSLA(d); err != nil {
			t.Errorf("dòng %q/%q: %v", g.LoaiViec, g.LinhVuc, err)
		}
	}
}

// THE FIGURES THEMSELVES, ROW BY ROW, AGAINST 14-cau-hinh.md:297-312.
//
// EVERY ROW IS LISTED RATHER THAN A SAMPLE, and the five numbers are given in the column order of
// the specification's own table. This is the only place a transposition of two adjacent figures —
// the defect class migration 0008 names for the five adjacent INTEGER columns — can be caught: a
// swap produces no error anywhere, only a different promise.
//
// MUTATION THAT MUST TURN THIS RED: swap any two figures on any row of BoGieoSLA.
func TestBoGieoSLAKhopBangDacTa(t *testing.T) {
	// tiếp nhận · xử lý xong · sắp đến hạn · báo lãnh đạo · báo chủ tịch — ALL WORKING HOURS.
	muon := map[string][5]int{
		"van-ban-den/": {8, 40, 24, 24, 48}, // :297 Mặc định cho mọi lĩnh vực

		"phan-anh/an-ninh":           {2, 16, 4, 8, 16},    // :298 An ninh trật tự
		"phan-anh/an-toan-thuc-pham": {2, 24, 6, 12, 24},   // :299 An toàn thực phẩm
		"phan-anh/can-bo":            {8, 120, 24, 24, 48}, // :300 Thái độ / tác phong cán bộ
		"phan-anh/cap-thoat-nuoc":    {4, 48, 8, 16, 32},   // :301 Cấp thoát nước
		"phan-anh/dien":              {2, 24, 6, 12, 24},   // :302 Điện
		"phan-anh/giao-thong":        {8, 168, 24, 24, 48}, // :303 Hạ tầng giao thông
		"phan-anh/khac":              {8, 72, 24, 24, 48},  // :304 Khác
		"phan-anh/o-nhiem":           {8, 72, 24, 24, 48},  // :305 Ô nhiễm
		"phan-anh/rac-thai":          {4, 24, 8, 16, 32},   // :306 Rác thải – Vệ sinh môi trường
		"phan-anh/trat-tu-do-thi":    {6, 40, 12, 24, 48},  // :307 Trật tự đô thị
		"phan-anh/xay-dung":          {4, 72, 12, 12, 24},  // :309 Xây dựng không phép
		"phan-anh/y-te-giao-duc":     {8, 72, 24, 24, 48},  // :310 Y tế – Giáo dục
		"phan-anh/":                  {8, 56, 24, 24, 48},  // :311 Mặc định cho mọi lĩnh vực

		"nhiem-vu/": {8, 40, 72, 24, 48}, // :312 — sắp đến hạn (72) LỚN HƠN xử lý xong (40), đúng đặc tả
	}

	theo := laBoGieoTheoKhoa(t)
	if len(theo) != len(muon) {
		t.Fatalf("bộ gieo có %d dòng, bảng đối chiếu có %d", len(theo), len(muon))
	}
	for k, m := range muon {
		g, co := theo[k]
		if !co {
			t.Errorf("bộ gieo thiếu dòng %q", k)
			continue
		}
		duoc := [5]int{g.GioTiepNhan, g.GioXuLyXong, g.GioSapDenHan, g.GioBaoLanhDao, g.GioBaoChuTich}
		if duoc != m {
			t.Errorf("dòng %q: số giờ = %v, đặc tả 14-cau-hinh.md §8 ghi %v "+
				"(thứ tự: tiếp nhận · xử lý xong · sắp đến hạn · báo lãnh đạo · báo chủ tịch)", k, duoc, m)
		}
	}
}

// BoGieoSLA HANDS BACK A FRESH SLICE EVERY CALL. A package-level slice would be mutable by any
// caller in the process, and one commune's seeding run editing the set the next commune gets is a
// defect that only appears under concurrency.
func TestBoGieoSLAKhongChiaSeTrangThai(t *testing.T) {
	a := BoGieoSLA()
	a[0].GioTiepNhan = 999

	if b := BoGieoSLA(); b[0].GioTiepNhan == 999 {
		t.Fatal("sửa kết quả của một lần gọi làm đổi lần gọi sau — bộ gieo đang dùng chung bộ nhớ")
	}
}

// --- KiemTraGio / KiemTraDongSLA ---------------------------------------------------------------

func TestKiemTraGioTuChoiSoKhongDuong(t *testing.T) {
	for _, gio := range []int{0, -1, -24} {
		if err := KiemTraGio(gio); err == nil {
			t.Errorf("KiemTraGio(%d) không lỗi — 0 giờ là hạn vỡ ngay lúc đặt ra, số âm là lỗi gõ", gio)
		}
	}
}

func TestKiemTraGioTuChoiSoQuaLon(t *testing.T) {
	if err := KiemTraGio(GioToiDa); err != nil {
		t.Errorf("KiemTraGio(%d) phải chấp nhận — trần là bao gồm: %v", GioToiDa, err)
	}
	if err := KiemTraGio(GioToiDa + 1); err == nil {
		t.Errorf("KiemTraGio(%d) không lỗi — một chữ số gõ thừa là một cam kết dài gấp mười", GioToiDa+1)
	}
}

// KiemTraDongSLA CHECKS ALL FIVE, and this case walks each position in turn. A loop that stopped at
// the first field, or that checked one field twice, would pass a row with a bad figure in the
// position it skipped — and there is no constraint downstream that distinguishes the five.
func TestKiemTraDongSLAKiemDuCaNamCot(t *testing.T) {
	tot := DongSLA{GioTiepNhan: 8, GioXuLyXong: 40, GioSapDenHan: 24, GioBaoLanhDao: 24, GioBaoChuTich: 48}
	if err := KiemTraDongSLA(tot); err != nil {
		t.Fatalf("dòng hợp lệ bị từ chối: %v", err)
	}

	for ten, hong := range map[string]func(*DongSLA){
		"gio_tiep_nhan":    func(d *DongSLA) { d.GioTiepNhan = 0 },
		"gio_xu_ly_xong":   func(d *DongSLA) { d.GioXuLyXong = 0 },
		"gio_sap_den_han":  func(d *DongSLA) { d.GioSapDenHan = 0 },
		"gio_bao_lanh_dao": func(d *DongSLA) { d.GioBaoLanhDao = 0 },
		"gio_bao_chu_tich": func(d *DongSLA) { d.GioBaoChuTich = 0 },
	} {
		d := tot
		hong(&d)
		if err := KiemTraDongSLA(d); err == nil {
			t.Errorf("%s = 0 mà không bị từ chối", ten)
		}
	}
}

// NO ORDERING RULE BETWEEN THE FIGURES. The specification's own `nhiem-vu` row has
// gio_sap_den_han (72) exceeding gio_xu_ly_xong (40), so a rule that "the warning window fits
// inside the deadline" would refuse configuration the customer already uses.
//
// THIS CASE EXISTS TO STOP THAT RULE BEING ADDED LATER as an obvious improvement.
func TestKiemTraDongSLAKhongApRangBuocThuTu(t *testing.T) {
	d := DongSLA{GioTiepNhan: 8, GioXuLyXong: 40, GioSapDenHan: 72, GioBaoLanhDao: 24, GioBaoChuTich: 48}
	if err := KiemTraDongSLA(d); err != nil {
		t.Fatalf("dòng `nhiem-vu` của chính đặc tả (14-cau-hinh.md:312) bị từ chối: %v — "+
			"một ràng buộc từ chối cấu hình khách đang dùng thì tệ hơn là không có", err)
	}
}
