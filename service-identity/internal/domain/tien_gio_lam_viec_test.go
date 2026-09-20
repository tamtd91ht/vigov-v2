package domain

import (
	"errors"
	"testing"
	"time"
)

// WHAT THESE TESTS DEFEND: the seven hard cases ADR 0007 §Hệ quả names itself — a count crossing
// the lunch gap, a receipt outside every session, a run spanning several weeks, a public holiday
// inside the run, a swap day inside the run, a count ending exactly at a session end, and a
// commune that works Saturday mornings only — plus the zone, which is the one thing here that a
// green test suite on one machine can be wrong about on another.
//
// EVERY EXPECTED INSTANT IS WRITTEN OUT AS A WALL-CLOCK TIME IN Asia/Ho_Chi_Minh and compared as
// an INSTANT. A test that recomputed the expected value with the same arithmetic would pass
// against any off-by-one this file exists to catch.

// The ordinary week most communes have: Monday–Friday, 07:30–11:30 and 13:30–17:00. Seven and a
// half hours a day, thirty-seven and a half a week — and a lunch gap, which is a session boundary
// every working day.
func tuanChuan() []CaLamViec {
	var ra []CaLamViec
	for thu := 1; thu <= 5; thu++ {
		ra = append(ra,
			CaLamViec{ID: "sang-" + string(rune('0'+thu)), Thu: thu, BatDau: gio(7, 30), KetThuc: gio(11, 30)},
			CaLamViec{ID: "chieu-" + string(rune('0'+thu)), Thu: thu, BatDau: gio(13, 30), KetThuc: gio(17, 0)},
		)
	}
	return ra
}

func gio(h, p int) GioTrongNgay { return GioTrongNgay(h*3600 + p*60) }

// muiDoiChung is the zone every expectation below is written in: a FIXED +07, built here.
//
// IT IS DELIBERATELY NOT MuiGio(). An expectation computed by the code under test agrees with that
// code by construction — the whole TZ property would then be green for the wrong reason, which is
// this repository's most repeated defect. Vietnam has been UTC+7 with no daylight saving since
// 1975, so a fixed offset is an INDEPENDENT oracle for every date used here, and an implementation
// that read time.Local or time.UTC would disagree with it.
var muiDoiChung = time.FixedZone("ICT-doi-chung", 7*3600)

// luc reads "2006-01-02 15:04" as a wall-clock time in the commune's zone — never in the
// process's. time.Parse would express every expectation in UTC and agree with an implementation
// that read time.Local.
func luc(t *testing.T, s string) time.Time {
	t.Helper()
	ra, err := time.ParseInLocation("2006-01-02 15:04", s, muiDoiChung)
	if err != nil {
		t.Fatalf("mốc %q không đọc được: %v", s, err)
	}
	return ra
}

// MuiGio phải THẬT SỰ là giờ Việt Nam, chứ không phải một *time.Location nào đó chạy được. Đây là
// chỗ duy nhất trong tệp này so hàm với một giá trị cố định bên ngoài nó.
func TestMuiGioLaUTC7(t *testing.T) {
	mui, err := MuiGio()
	if err != nil {
		t.Fatalf("MuiGio: %v", err)
	}
	for _, ngay := range []string{"2026-01-15 08:00", "2026-07-15 08:00", "2026-09-21 08:00"} {
		trong, err := time.ParseInLocation("2006-01-02 15:04", ngay, mui)
		if err != nil {
			t.Fatalf("mốc %q: %v", ngay, err)
		}
		if _, lech := trong.Zone(); lech != 7*3600 {
			t.Errorf("%s: độ lệch %d giây, muốn %d (UTC+7) — %q không phải giờ Việt Nam",
				ngay, lech, 7*3600, MuiGioHanhChinh)
		}
	}
}

// lichGiaNam stands in for the two per-year reads, and RECORDS WHICH YEARS WERE ASKED FOR — the
// laziness is a property worth a test of its own: a configuration fault in a year the count never
// reaches must not change the answer.
type lichGiaNam struct {
	nghi     []NgayNghiLe
	lamBu    []CaLamBu
	err      error
	namDaDoc []int
}

func (f *lichGiaNam) doc(nam int) ([]NgayNghiLe, []CaLamBu, error) {
	f.namDaDoc = append(f.namDaDoc, nam)
	if f.err != nil {
		return nil, nil, f.err
	}
	return f.nghi, f.lamBu, nil
}

// mot runs the arithmetic for ONE amount and returns the instant, failing the test on any refusal.
func mot(t *testing.T, tuMoc time.Time, tuan []CaLamViec, lich *lichGiaNam, gioLam int) time.Time {
	t.Helper()
	if lich == nil {
		lich = &lichGiaNam{}
	}
	ra, err := TienGioLamViec(tuMoc, tuan, lich.doc, []int{gioLam})
	if err != nil {
		t.Fatalf("TienGioLamViec(%d giờ): %v", gioLam, err)
	}
	if len(ra) != 1 {
		t.Fatalf("số mốc trả về = %d, muốn 1", len(ra))
	}
	if ra[0].Gio != gioLam {
		t.Fatalf("mốc trả về %d giờ, muốn %d — item phải mang chính con số được hỏi", ra[0].Gio, gioLam)
	}
	return ra[0].DatLuc
}

func bang(t *testing.T, duoc, muon time.Time, viec string) {
	t.Helper()
	if !duoc.Equal(muon) {
		t.Errorf("%s: đạt lúc %s, muốn %s", viec,
			duoc.Format(time.RFC3339), muon.Format(time.RFC3339))
	}
}

// ---------------------------------------------------------------- the seven hard cases

// Nghỉ trưa là một ranh giới ca MỖI NGÀY LÀM VIỆC: 08:00 + 5 giờ làm việc = 15:00, không phải
// 13:00 — hai tiếng nghỉ trưa không được tính.
func TestVatQuaNghiTrua(t *testing.T) {
	// 2026-09-21 is a Monday.
	dat := mot(t, luc(t, "2026-09-21 08:00"), tuanChuan(), nil, 5)
	bang(t, dat, luc(t, "2026-09-21 15:00"), "5 giờ làm việc từ 08:00 thứ Hai")
}

// THE OFF-BY-ONE-SESSION THE CONTRACT WRITES DOWN. The count runs out exactly at 11:30 and the
// afternoon opens at 13:30: the answer is 11:30. Returning 13:30 would hand the commune two hours
// nobody granted it — on every deadline landing on a boundary, and lunch is one every day.
func TestHetGioDungRANHGIOICaThiLayMocSOMNHAT(t *testing.T) {
	// Hết đúng lúc TAN CA SÁNG: 08:30 + 3 giờ = 11:30. Nếu trả 13:30 thì xã được cộng không hai
	// tiếng nghỉ trưa.
	dat := mot(t, luc(t, "2026-09-21 08:30"), tuanChuan(), nil, 3)
	if dat.Equal(luc(t, "2026-09-21 13:30")) {
		t.Fatal("trả về đầu ca chiều — xã được cộng không hai tiếng nghỉ trưa")
	}
	bang(t, dat, luc(t, "2026-09-21 11:30"), "3 giờ từ 08:30, hết đúng lúc tan ca sáng")

	// Cũng là ranh giới ấy, đi từ đầu ca sáng: 07:30 + 4 giờ = 11:30.
	dat = mot(t, luc(t, "2026-09-21 07:30"), tuanChuan(), nil, 4)
	bang(t, dat, luc(t, "2026-09-21 11:30"), "4 giờ từ đầu ca sáng")

	// Hết đúng lúc TAN CA CHIỀU — tức hết giờ làm của cả ngày: 08:00 + 3,5 giờ sáng + 3,5 giờ
	// chiều = 17:00, chứ không phải 07:30 sáng hôm sau.
	dat = mot(t, luc(t, "2026-09-21 08:00"), tuanChuan(), nil, 7)
	if dat.Equal(luc(t, "2026-09-22 07:30")) {
		t.Fatal("trả về đầu ca sáng hôm sau — xã được cộng không cả một đêm")
	}
	bang(t, dat, luc(t, "2026-09-21 17:00"), "7 giờ từ 08:00, hết đúng lúc tan ca chiều")

	// Và mốc vẫn chạy bình thường khi không rơi vào ranh giới nào.
	dat = mot(t, luc(t, "2026-09-21 08:00"), tuanChuan(), nil, 3)
	bang(t, dat, luc(t, "2026-09-21 11:00"), "3 giờ — vẫn trong ca sáng")
	dat = mot(t, luc(t, "2026-09-21 09:30"), tuanChuan(), nil, 5)
	bang(t, dat, luc(t, "2026-09-21 16:30"), "5 giờ từ 09:30")
}

// ADR 0007 QUYẾT ĐỊNH 8: tiếp nhận ngoài mọi ca thì đếm từ ĐẦU CA KẾ TIẾP, và không có ca nào
// được tính một phần cho quãng cơ quan đóng cửa. Hệ quả người dùng đã chấp nhận và phải nói được
// với dân: phiếu gửi 22:00 thứ Sáu có hạn ĐÚNG BẰNG phiếu gửi 07:30 thứ Hai.
func TestNgoaiGioThiDemTuDauCaKeTiep(t *testing.T) {
	// 2026-09-18 is a Friday.
	ngoaiGio := mot(t, luc(t, "2026-09-18 22:00"), tuanChuan(), nil, 2)
	bang(t, ngoaiGio, luc(t, "2026-09-21 09:30"), "2 giờ từ 22:00 thứ Sáu")

	trongGio := mot(t, luc(t, "2026-09-21 07:30"), tuanChuan(), nil, 2)
	if !ngoaiGio.Equal(trongGio) {
		t.Errorf("phiếu 22:00 thứ Sáu hạn %s, phiếu 07:30 thứ Hai hạn %s — phải BẰNG NHAU, "+
			"đêm thứ Sáu và cuối tuần không được cộng chút nào",
			ngoaiGio.Format(time.RFC3339), trongGio.Format(time.RFC3339))
	}
}

// Nhận giữa nghỉ trưa: đồng hồ chạy từ đầu ca chiều của CHÍNH NGÀY HÔM ĐÓ (bảng trong ADR 0007,
// quyết định 8).
func TestNhanGiuaNghiTruaThiDemTuDauCaChieu(t *testing.T) {
	dat := mot(t, luc(t, "2026-09-21 12:00"), tuanChuan(), nil, 1)
	bang(t, dat, luc(t, "2026-09-21 14:30"), "1 giờ từ 12:00 (giữa nghỉ trưa)")
}

// Nhận GIỮA ca thì đếm ngay tại chỗ nhận, không dời tới đầu ca sau.
func TestNhanTrongCaThiDemNgayTaiCho(t *testing.T) {
	dat := mot(t, luc(t, "2026-09-21 10:00"), tuanChuan(), nil, 1)
	bang(t, dat, luc(t, "2026-09-21 11:00"), "1 giờ từ 10:00")
}

// Ngày nghỉ lễ nằm trong quãng đếm: thứ Hai nghỉ, đồng hồ chạy từ sáng thứ Ba.
func TestNgayNghiLeTrongQuangDem(t *testing.T) {
	lich := &lichGiaNam{nghi: []NgayNghiLe{{ID: "le-1", Ngay: "2026-09-21", Ten: "Lễ giả định"}}}
	dat := mot(t, luc(t, "2026-09-18 22:00"), tuanChuan(), lich, 2)
	bang(t, dat, luc(t, "2026-09-22 09:30"), "2 giờ từ 22:00 thứ Sáu, thứ Hai nghỉ lễ")
}

// Ngày làm bù nằm trong quãng đếm: thứ Bảy có ca làm bù nên hạn SỚM HƠN, đúng bảng của quyết
// định 8. Thứ Bảy vốn không có dòng nào trong `lich_lam_viec`, nên đây là trường hợp bình thường
// của ngày làm bù chứ không phải lỗi cấu hình.
func TestNgayLamBuTrongQuangDem(t *testing.T) {
	lich := &lichGiaNam{lamBu: []CaLamBu{{
		ID: "bu-1", Ngay: "2026-09-19", BatDau: gio(7, 30), KetThuc: gio(11, 30),
		Ten: "Làm bù theo thông báo giả định",
	}}}
	dat := mot(t, luc(t, "2026-09-18 22:00"), tuanChuan(), lich, 2)
	bang(t, dat, luc(t, "2026-09-19 09:30"), "2 giờ từ 22:00 thứ Sáu, thứ Bảy có làm bù")
}

// Hồ sơ vắt qua NHIỀU TUẦN — 168 giờ là con số lớn nhất trong bảng SLA của ADR 0007.
// 22 ngày làm việc trọn (165 giờ) + 3 giờ sáng ngày thứ 23.
func TestVatQuaNhieuTuan(t *testing.T) {
	dat := mot(t, luc(t, "2026-09-21 07:30"), tuanChuan(), nil, 168)
	bang(t, dat, luc(t, "2026-10-21 10:30"), "168 giờ làm việc từ sáng thứ Hai 21/09")
}

// Xã chỉ trực SÁNG THỨ BẢY — một trong bốn câu ADR 0007 để mở, và nó là DỮ LIỆU chứ không phải
// quy tắc: một dòng thu = 6. Ca này cũng kiểm luôn thứ ISO 6.
func TestXaChiLamSangThuBay(t *testing.T) {
	tuan := []CaLamViec{{ID: "truc-7", Thu: 6, BatDau: gio(7, 30), KetThuc: gio(11, 30)}}

	dat := mot(t, luc(t, "2026-09-18 22:00"), tuan, nil, 2)
	bang(t, dat, luc(t, "2026-09-19 09:30"), "2 giờ, xã chỉ làm sáng thứ Bảy")

	// Vắt qua trọn một tuần: 4 giờ của thứ Bảy này + 2 giờ của thứ Bảy sau.
	dat = mot(t, luc(t, "2026-09-18 22:00"), tuan, nil, 6)
	bang(t, dat, luc(t, "2026-09-26 09:30"), "6 giờ, vắt sang thứ Bảy tuần sau")
}

// CHỦ NHẬT LÀ 7 THEO ISO, KHÔNG PHẢI 0. time.Weekday đếm Chủ nhật là 0 và cột `thu` thì không
// bao giờ — đây là chỗ một off-by-one sống cả năm trước khi có người nhận ra hạn lệch một ngày.
func TestChuNhatLaThuISO7(t *testing.T) {
	tuan := []CaLamViec{{ID: "cn", Thu: 7, BatDau: gio(7, 30), KetThuc: gio(11, 30)}}
	// 2026-09-20 is a Sunday.
	dat := mot(t, luc(t, "2026-09-19 00:00"), tuan, nil, 1)
	bang(t, dat, luc(t, "2026-09-20 08:30"), "1 giờ, xã chỉ làm Chủ nhật")
}

// ---------------------------------------------------------------- the zone

// MÚI GIỜ LÀ CỦA XÃ, KHÔNG PHẢI CỦA TIẾN TRÌNH. Một pod chạy TZ=UTC phải cho cùng một câu trả
// lời với pod không đặt TZ — nếu không thì cam kết nói với dân phụ thuộc vào một biến môi trường
// của container, và không màn hình nào cho thấy điều đó.
//
// ĐẶT THẲNG time.Local LÀ ĐÚNG THỨ TZ LÀM lúc khởi động tiến trình, nên đây là phép kiểm thật
// chứ không phải phép kiểm giả vờ: nếu hàm đọc time.Local, hai vòng dưới sẽ ra hai kết quả khác
// nhau.
func TestKhongPhuThuocTZCuaTienTrinh(t *testing.T) {
	goc := time.Local
	t.Cleanup(func() { time.Local = goc })

	var dapAn []time.Time
	for _, mui := range []*time.Location{time.UTC, time.FixedZone("GIA", 7*3600), time.FixedZone("GIA-TAY", -8*3600)} {
		time.Local = mui
		// Mốc bắt đầu cũng được dựng lại trong vòng lặp, bằng luc() — nó luôn đọc theo múi giờ
		// của xã, nên giá trị không đổi theo time.Local.
		dapAn = append(dapAn, mot(t, luc(t, "2026-09-21 08:00"), tuanChuan(), nil, 5))
	}
	for i := 1; i < len(dapAn); i++ {
		if !dapAn[i].Equal(dapAn[0]) {
			t.Fatalf("TZ của tiến trình đổi thì đáp án đổi: %s so với %s — hàm đang đọc time.Local "+
				"thay vì %s", dapAn[i].Format(time.RFC3339), dapAn[0].Format(time.RFC3339), MuiGioHanhChinh)
		}
	}
	bang(t, dapAn[0], luc(t, "2026-09-21 15:00"), "5 giờ làm việc, bất kể TZ")
}

// Mốc bắt đầu gửi tới bằng MÚI GIỜ NÀO CŨNG ĐƯỢC — nó là một INSTANT. Cùng một khoảnh khắc viết
// bằng UTC và viết bằng giờ Việt Nam phải cho cùng một hạn.
func TestMocBatDauLaINSTANTChuKhongPhaiGioTuong(t *testing.T) {
	vn := luc(t, "2026-09-21 08:00")
	dat := mot(t, vn.UTC(), tuanChuan(), nil, 5)
	bang(t, dat, luc(t, "2026-09-21 15:00"), "mốc gửi bằng UTC")
}

// ---------------------------------------------------------------- nhiều mốc trong một lời gọi

// ADR 0007 cho mỗi dòng HAI hạn — `Tiếp nhận` và `Xử lý xong` — đếm từ CÙNG một mốc trên CÙNG một
// lịch. Trùng nhau thì gộp, nên số item có thể ít hơn số mốc được hỏi và vị trí không bao giờ là
// khoá.
func TestNhieuMocMotLanDiVaGopTrung(t *testing.T) {
	lich := &lichGiaNam{}
	ra, err := TienGioLamViec(luc(t, "2026-09-21 07:30"), tuanChuan(), lich.doc, []int{16, 2, 16, 2})
	if err != nil {
		t.Fatalf("TienGioLamViec: %v", err)
	}
	if len(ra) != 2 {
		t.Fatalf("số item = %d, muốn 2 — mốc trùng phải gộp", len(ra))
	}
	theoGio := map[int]time.Time{}
	for _, m := range ra {
		theoGio[m.Gio] = m.DatLuc
	}
	bang(t, theoGio[2], luc(t, "2026-09-21 09:30"), "mốc 2 giờ")
	bang(t, theoGio[16], luc(t, "2026-09-23 08:30"), "mốc 16 giờ")

	// MỘT LẦN ĐỌC LỊCH cho cả hai mốc, chứ không phải một lần mỗi mốc: hai lần đọc là hai lần có
	// thể rơi vào hai trạng thái cấu hình khác nhau.
	if len(lich.namDaDoc) != 1 || lich.namDaDoc[0] != 2026 {
		t.Errorf("năm đã đọc = %v, muốn đúng một lần năm 2026", lich.namDaDoc)
	}
}

// Quãng đếm vắt qua giao thừa dương lịch thì đọc HAI năm, và chỉ hai năm.
func TestVatQuaNamThiDocDungHaiNam(t *testing.T) {
	lich := &lichGiaNam{}
	// 2026-12-31 là thứ Năm; 01/01 là thứ Sáu; thứ Hai kế tiếp là 04/01/2027.
	ra, err := TienGioLamViec(luc(t, "2026-12-31 07:30"), tuanChuan(), lich.doc, []int{16})
	if err != nil {
		t.Fatalf("TienGioLamViec: %v", err)
	}
	bang(t, ra[0].DatLuc, luc(t, "2027-01-04 08:30"), "16 giờ từ 31/12")
	if len(lich.namDaDoc) != 2 || lich.namDaDoc[0] != 2026 || lich.namDaDoc[1] != 2027 {
		t.Errorf("năm đã đọc = %v, muốn [2026 2027]", lich.namDaDoc)
	}
}

// Ca kết thúc lúc 24:00:00 là giá trị HỢP LỆ (TIME của PostgreSQL nhận, GiayTrongNgay = 86400) —
// nửa đêm là ranh giới ngày, và cộng 86400 giây vào nửa đêm phải rơi đúng vào nửa đêm hôm sau.
func TestCaKetThucLucNuaDem(t *testing.T) {
	tuan := []CaLamViec{{ID: "dem", Thu: 1, BatDau: gio(20, 0), KetThuc: GiayTrongNgay}}
	dat := mot(t, luc(t, "2026-09-21 22:00"), tuan, nil, 1)
	bang(t, dat, luc(t, "2026-09-21 23:00"), "1 giờ trong ca đêm")

	dat = mot(t, luc(t, "2026-09-21 22:00"), tuan, nil, 3)
	bang(t, dat, luc(t, "2026-09-28 21:00"), "3 giờ — 2 giờ đêm thứ Hai này, 1 giờ thứ Hai sau")
}

// ---------------------------------------------------------------- những lệnh từ chối

func loiCauHinh(t *testing.T, err error) *LoiKhongTinhDuocHan {
	t.Helper()
	var e *LoiKhongTinhDuocHan
	if !errors.As(err, &e) {
		t.Fatalf("lỗi = %v (%T), muốn *LoiKhongTinhDuocHan", err, err)
	}
	return e
}

// LỊCH TRỐNG KHÔNG PHẢI LÀ "KHÔNG CÓ GIỜ LÀM VIỆC, HẠN BẰNG CHÍNH MỐC TIẾP NHẬN", và cũng không
// phải "Thứ Hai–Sáu 08:00–17:00". Nó là LỆNH TỪ CHỐI. Hôm nay đây là trạng thái của MỌI xã:
// migration 0006 không gieo dòng nào và bước onboarding chưa tồn tại.
func TestLichTrongThiTuChoiChuKhongMacDinh(t *testing.T) {
	ra, err := TienGioLamViec(luc(t, "2026-09-21 08:00"), nil, (&lichGiaNam{}).doc, []int{2})
	if err == nil {
		t.Fatalf("lịch trống mà vẫn trả hạn: %+v — đây là cam kết do phần mềm bịa ra", ra)
	}
	if got := loiCauHinh(t, err).Loai; got != LoiLichTrong {
		t.Errorf("loại = %q, muốn %q", got, LoiLichTrong)
	}
}

// Hai ca chồng giờ nhau làm giờ chồng bị đếm HAI LẦN, tức hạn SỚM hơn giờ thật của xã — đúng
// chiều báo cáo một cơ quan là trễ trong khi nó không trễ. Cơ sở dữ liệu không chặn được
// (0006:109), nên đường đọc phải chặn.
func TestCaChongNhauThiTuChoi(t *testing.T) {
	tuan := append(tuanChuan(), CaLamViec{
		ID: "chong", Thu: 1, BatDau: gio(9, 0), KetThuc: gio(12, 0),
	})
	_, err := TienGioLamViec(luc(t, "2026-09-21 08:00"), tuan, (&lichGiaNam{}).doc, []int{2})
	if err == nil {
		t.Fatal("hai ca chồng nhau mà vẫn trả hạn")
	}
	if got := loiCauHinh(t, err).Loai; got != LoiCaChongNhau {
		t.Errorf("loại = %q, muốn %q", got, LoiCaChongNhau)
	}
}

// Hai ca LÀM BÙ chồng nhau trên một ngày: cùng một lỗi, cùng một hướng hỏng, cùng một lệnh từ
// chối — `ngay_lam_bu` chỉ có UNIQUE (tenant_id, ngay, bat_dau) (0006:283).
func TestCaLamBuChongNhauThiTuChoi(t *testing.T) {
	lich := &lichGiaNam{lamBu: []CaLamBu{
		{ID: "bu-1", Ngay: "2026-09-19", BatDau: gio(7, 30), KetThuc: gio(11, 30), Ten: "Làm bù giả định"},
		{ID: "bu-2", Ngay: "2026-09-19", BatDau: gio(9, 0), KetThuc: gio(12, 0), Ten: "Làm bù giả định"},
	}}
	_, err := TienGioLamViec(luc(t, "2026-09-18 22:00"), tuanChuan(), lich.doc, []int{2})
	if err == nil {
		t.Fatal("hai ca làm bù chồng nhau mà vẫn trả hạn")
	}
	if got := loiCauHinh(t, err).Loai; got != LoiCaChongNhau {
		t.Errorf("loại = %q, muốn %q", got, LoiCaChongNhau)
	}
}

// ADR 0007 QUYẾT ĐỊNH 9: dòng làm bù rơi vào ngày mà THỨ CỦA NÓ vốn đã có ca là LỖI CẤU HÌNH.
// THAY hay CỘNG THÊM đều đọc được, hai cách lệch nhau đúng bằng phần chồng, và không gì chọn
// giùm — nên không đoán. Nới ra về sau là thêm tính năng; gỡ một hạn đã tính từ giờ đếm hai lần
// thì không.
func TestLamBuRoiVaoNgayVonDaLamViecThiTuChoi(t *testing.T) {
	lich := &lichGiaNam{lamBu: []CaLamBu{{
		ID: "bu-1", Ngay: "2026-09-21", BatDau: gio(18, 0), KetThuc: gio(20, 0), Ten: "Làm bù giả định",
	}}}
	_, err := TienGioLamViec(luc(t, "2026-09-18 22:00"), tuanChuan(), lich.doc, []int{2})
	if err == nil {
		t.Fatal("làm bù rơi vào thứ Hai vốn đã có ca mà vẫn trả hạn")
	}
	if got := loiCauHinh(t, err).Loai; got != LoiLamBuTrungNgayDaLam {
		t.Errorf("loại = %q, muốn %q", got, LoiLamBuTrungNgayDaLam)
	}
}

// Lỗi cấu hình ở một ngày quãng đếm KHÔNG chạm tới thì không đổi được câu trả lời này — nó thuộc
// về đường ghi. Cùng một dòng làm bù sai của ca trên, nhưng đặt ở một ngày sau khi hạn đã xong.
func TestLoiCauHinhONgayKhongChamToiThiKhongChan(t *testing.T) {
	lich := &lichGiaNam{lamBu: []CaLamBu{{
		ID: "bu-1", Ngay: "2026-12-14", BatDau: gio(18, 0), KetThuc: gio(20, 0), Ten: "Làm bù giả định",
	}}}
	dat := mot(t, luc(t, "2026-09-21 08:00"), tuanChuan(), lich, 2)
	bang(t, dat, luc(t, "2026-09-21 10:00"), "2 giờ, lỗi cấu hình nằm ở tháng 12")
}

// Ngày vừa nghỉ lễ vừa làm bù: KHÔNG có quy tắc ưu tiên. Hai đường đọc trong store bắt lỗi này
// bằng chính câu SQL của chúng, nên nhánh dưới là hàng rào cuối — và nó trả về ĐÚNG KIỂU LỖI của
// store, để phía dưới không có bộ từ vựng thứ hai cho một lỗi.
func TestNgayVuaNghiVuaLamBuThiTuChoi(t *testing.T) {
	lich := &lichGiaNam{
		nghi:  []NgayNghiLe{{ID: "le-1", Ngay: "2026-09-19", Ten: "Lễ giả định"}},
		lamBu: []CaLamBu{{ID: "bu-1", Ngay: "2026-09-19", BatDau: gio(7, 30), KetThuc: gio(11, 30), Ten: "Làm bù giả định"}},
	}
	_, err := TienGioLamViec(luc(t, "2026-09-18 22:00"), tuanChuan(), lich.doc, []int{2})
	var xungDot *LoiNgayVuaNghiVuaLamBu
	if !errors.As(err, &xungDot) {
		t.Fatalf("lỗi = %v (%T), muốn *LoiNgayVuaNghiVuaLamBu", err, err)
	}
	if len(xungDot.Ngay) != 1 || xungDot.Ngay[0] != "2026-09-19" {
		t.Errorf("ngày xung đột = %v, muốn [2026-09-19]", xungDot.Ngay)
	}
}

// Quá CHÂN TRỜI thì từ chối, vì qua đó lịch nghỉ lễ chưa ai khai — và câu trả lời tự tin tính
// qua một quãng không có ngày lễ nào là câu tệ nhất trong các câu có thể.
func TestQuaChanTroiThiTuChoi(t *testing.T) {
	// 2 000 giờ với tuần 37,5 giờ ≈ 53 tuần > 366 ngày.
	_, err := TienGioLamViec(luc(t, "2026-09-21 08:00"), tuanChuan(), (&lichGiaNam{}).doc, []int{2000})
	if err == nil {
		t.Fatal("2 000 giờ làm việc mà vẫn trả hạn — quá chân trời phải từ chối")
	}
	if got := loiCauHinh(t, err).Loai; got != LoiVuotChanTroi {
		t.Errorf("loại = %q, muốn %q", got, LoiVuotChanTroi)
	}
}

// CHÂN TRỜI ĐO TỪ `count_from`, KHÔNG PHẢI TỪ `now`, nên một mốc tiếp nhận CŨ vẫn được trả lời
// bình thường — ADR 0007 quyết định 5: văn bản đến hôm qua, vào sổ hôm nay, đếm từ hôm qua. Một
// máy chủ kẹp mốc ấy về `now` sẽ âm thầm dời một cam kết bên gọi đã chốt.
//
// ĐIỀU NÀY CÓ GIÁ CỦA NÓ, và nó được nói ra chứ không giấu: quãng đếm của một mốc rất cũ chạy
// qua những năm mà bảng ngày nghỉ lễ có thể chưa ai khai. Chân trời không che được chiều đó, và
// không hợp đồng nào che được — nó thuộc về người vận hành màn hình cấu hình.
func TestMocTiepNhanCuVanDuocTraLoi(t *testing.T) {
	// 2020-01-02 là thứ Năm.
	dat := mot(t, luc(t, "2020-01-02 08:00"), tuanChuan(), nil, 2)
	bang(t, dat, luc(t, "2020-01-02 10:00"), "2 giờ từ một mốc tiếp nhận năm 2020")
}

// Kho hỏng đi RA NGUYÊN VẸN, không bị đổi thành một câu trả lời nghiệp vụ và cũng không bị bọc
// thành lỗi cấu hình: phía gRPC phân biệt hai thứ đó bằng kiểu.
func TestKhoHongThiTraLoiKhoHong(t *testing.T) {
	loiKho := errors.New("giả lập: cơ sở dữ liệu không tới được")
	_, err := TienGioLamViec(luc(t, "2026-09-21 08:00"), tuanChuan(),
		(&lichGiaNam{err: loiKho}).doc, []int{2})
	if !errors.Is(err, loiKho) {
		t.Fatalf("lỗi = %v, muốn bọc được lỗi kho", err)
	}
	var cauHinh *LoiKhongTinhDuocHan
	if errors.As(err, &cauHinh) {
		t.Error("lỗi kho bị đọc thành lỗi cấu hình của xã — operator sẽ bị gửi tới màn hình cấu hình đang đúng")
	}
}

// Mốc 0 giờ không bao giờ tới được đây (gRPC đã chặn bằng INVALID_ARGUMENT), nên tới được nghĩa
// là hai nửa của chính dịch vụ này bất đồng — và nó không được trả về chính mốc tiếp nhận.
func TestMocKhongGioKhongBaoGioTraVeChinhMocTiepNhan(t *testing.T) {
	ra, err := TienGioLamViec(luc(t, "2026-09-21 08:00"), tuanChuan(), (&lichGiaNam{}).doc, []int{0})
	if err == nil {
		t.Fatalf("mốc 0 giờ trả về %+v — hồ sơ đến hạn ngay lúc vừa tiếp nhận", ra)
	}
	var cauHinh *LoiKhongTinhDuocHan
	if errors.As(err, &cauHinh) {
		t.Error("lỗi của bên gọi bị đọc thành lỗi cấu hình của xã")
	}
}
