package domain

import (
	"errors"
	"strings"
	"testing"
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

// TestChiBaBuocBaoChoDan — `skills/petition-lifecycle` settles which acts owe the citizen a message:
// intake, entering processing, and closing. The other six are internal steps, and the citizen sees
// status and result, never staff notes or routing history (rule 4, forbidden #5).
//
// ĐỘT BIẾN: thêm `DangPhanLoai` vào vietTiepTheo và ca này ĐỎ — phân loại là bước nội bộ, và một tin
// nhắn cho mỗi bước nội bộ là cách một kênh thông báo trở thành thứ người dân tắt đi.
func TestChiBaBuocBaoChoDan(t *testing.T) {
	baoTin := map[TrangThai]bool{DaTiepNhan: true, DangXuLy: true, DaDong: true}
	for _, t2 := range []TrangThai{
		DaTiepNhan, DangPhanLoai, DaChuyenXuLy, DangXuLy, DaXuLy,
		ChoDanXacNhan, DaDong, KhongTiepNhan, ChuyenCapTren,
	} {
		if BaoChoDan(t2) != baoTin[t2] {
			t.Errorf("BaoChoDan(%q) = %v, muốn %v", t2, BaoChoDan(t2), baoTin[t2])
		}
		viec := ViecTiepTheo(t2)
		if baoTin[t2] {
			if viec == "" {
				t.Errorf("%q phải báo cho dân nhưng không có câu việc tiếp theo", t2)
			}
			// THE RECEIVING SIDE REFUSES A `next_step` THAT MERELY REPEATS THE LABEL. A notification
			// naming a state and no consequence — "Đã xử lý." — tells the citizen nothing they can act
			// on, and a channel that answers like that is a channel people stop reading.
			if strings.TrimSpace(viec) == strings.TrimSpace(NhanTrangThai(t2)) {
				t.Errorf("%q: việc tiếp theo chỉ lặp lại nhãn", t2)
			}
		} else if viec != "" {
			t.Errorf("%q là bước nội bộ nhưng vẫn mang câu cho dân: %q", t2, viec)
		}
	}
}

// TestViecTiepTheoKhongHuaMotNgayNao — at intake no resolve deadline exists yet (ADR 0028), so a
// sentence that named a date would have to invent it. Rule 10, forbidden #3.
func TestViecTiepTheoKhongHuaMotNgayNao(t *testing.T) {
	for _, t2 := range []TrangThai{DaTiepNhan, DangXuLy, DaDong} {
		viec := ViecTiepTheo(t2)
		for _, cam := range []string{"ngày", "/20", "giờ nữa", "trước ngày"} {
			if strings.Contains(viec, cam) {
				t.Errorf("%q: câu cho dân chứa %q — một ngày hẹn hiện ra rồi đổi là đúng thứ "+
					"ADR 0027 quyết định C tồn tại để chặn, chỉ khác là nó xảy ra ở tầng giao diện",
					t2, cam)
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

func TestDongDuocChiTuChoDanXacNhan(t *testing.T) {
	if err := DongDuoc(ChoDanXacNhan); err != nil {
		t.Fatalf("không đóng được từ cho-dan-xac-nhan: %v", err)
	}
	for _, tu := range []TrangThai{DaTiepNhan, DangPhanLoai, DaChuyenXuLy, DangXuLy, DaXuLy,
		DaDong, KhongTiepNhan, ChuyenCapTren} {
		if err := DongDuoc(tu); !errors.Is(err, ErrDongSaiLuc) {
			t.Errorf("đóng được từ %q — phiếu bị đóng trước khi người dân được hỏi", tu)
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
