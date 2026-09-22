package domain

import (
	"errors"
	"strings"
	"testing"
)

// WHAT THIS FILE IS FOR: the parsing and the refusals the calendar write path owes, with no database
// and no HTTP in sight. Every case here is a value that LOOKS fine and is not — which is the only
// kind that survives to production.

// 24:00:00 IS A LEGAL TIME AND 7:30 IS NOT. The first is what PostgreSQL's TIME allows for a session
// that closes at midnight (and what a time.Time cannot hold at all); the second is a shape that
// strconv.Atoi would happily read as 07:30 while "7:3" would become 07:03 — a time the commune did
// not type, 27 minutes from the one they meant.
func TestDocGioTrongNgay(t *testing.T) {
	nhan := []struct {
		vao  string
		muon GioTrongNgay
	}{
		{"07:30", 7*3600 + 30*60},
		{"07:30:15", 7*3600 + 30*60 + 15},
		{"00:00", 0},
		{"24:00:00", GiayTrongNgay}, // a session closing at midnight
		{" 17:00 ", 17 * 3600},      // trimmed
	}
	for _, c := range nhan {
		got, err := DocGioTrongNgay(c.vao)
		if err != nil {
			t.Errorf("DocGioTrongNgay(%q) lỗi: %v", c.vao, err)
			continue
		}
		if got != c.muon {
			t.Errorf("DocGioTrongNgay(%q) = %d, muốn %d", c.vao, got, c.muon)
		}
	}

	tuChoi := []string{
		"", "7:30", "07:3", "0730", "07h30", "07:30:", "07:30:00:00",
		"24:00:01", "24:01", "07:60", "07:30:60", "-1:00", "+7:30", "07:30 sáng",
	}
	for _, v := range tuChoi {
		if _, err := DocGioTrongNgay(v); !errors.Is(err, ErrGioKhongDocDuoc) {
			t.Errorf("DocGioTrongNgay(%q) = %v, muốn ErrGioKhongDocDuoc", v, err)
		}
	}
}

// THE ROUND TRIP IS THE CHECK. time.Parse accepts "2026-02-30" by rolling it into 2 March, so a
// commune entering a date that does not exist would get a holiday on a different day from the one
// they typed — and the deadline function looks a date up as a STRING, so "2026-9-2" would be a
// holiday that silently does nothing.
func TestChuanHoaNgay(t *testing.T) {
	for _, v := range []string{"2026-01-01", "2026-09-02", "2024-02-29", " 2026-12-31 "} {
		got, err := ChuanHoaNgay(v)
		if err != nil {
			t.Errorf("ChuanHoaNgay(%q) lỗi: %v", v, err)
			continue
		}
		if got != strings.TrimSpace(v) {
			t.Errorf("ChuanHoaNgay(%q) = %q — ngày phải giữ NGUYÊN cách viết", v, got)
		}
	}
	for _, v := range []string{
		"", "2026-9-2", "2026-02-30", "2025-02-29", "2026-13-01", "02/09/2026",
		"2026-09-02T00:00:00Z", "26-09-02",
	} {
		if _, err := ChuanHoaNgay(v); !errors.Is(err, ErrNgayKhongDocDuoc) {
			t.Errorf("ChuanHoaNgay(%q) = %v, muốn ErrNgayKhongDocDuoc", v, err)
		}
	}
}

// THE ISO WEEKDAY, AND IT DECIDES WHETHER A SWAP DAY IS REFUSED UNDER ADR 0007 DECISION 9. An
// off-by-one here would let a swap day onto a working weekday, or refuse one on a genuinely closed
// day — and it would be invisible for a year.
func TestThuCuaNgay(t *testing.T) {
	for _, c := range []struct {
		ngay string
		muon int
	}{
		{"2026-02-21", 6}, // thứ Bảy
		{"2026-02-22", 7}, // Chủ nhật — 7, NEVER 0 (that is JavaScript's getDay())
		{"2026-02-23", 1}, // thứ Hai
		{"2026-09-02", 3}, // thứ Tư — Quốc khánh 2026
	} {
		got, err := ThuCuaNgay(c.ngay)
		if err != nil {
			t.Fatalf("ThuCuaNgay(%q) lỗi: %v", c.ngay, err)
		}
		if got != c.muon {
			t.Errorf("ThuCuaNgay(%q) = %d (%s), muốn %d (%s)",
				c.ngay, got, TenThuISO(got), c.muon, TenThuISO(c.muon))
		}
	}
}

// THE OVERLAP CHECK GOES THROUGH TimChongNhau, THE SAME FUNCTION THE READ PATH PUBLISHES `problems`
// FROM. These cases pin the three answers that matter, including the one that must NOT be an
// overlap: two sessions touching at one instant are a working day with no lunch break.
func TestKhongChongCaNao(t *testing.T) {
	sang := Khoang{ID: "ca-sang", Nhom: "1", BatDau: 7*3600 + 30*60, KetThuc: 11*3600 + 30*60}

	// (a) genuinely overlapping
	err := KhongChongCaNao(Khoang{Nhom: "1", BatDau: 9 * 3600, KetThuc: 12 * 3600}, []Khoang{sang})
	var chong *LoiCaChongCaKhac
	if !errors.As(err, &chong) {
		t.Fatalf("lỗi = %v, muốn *LoiCaChongCaKhac", err)
	}
	if chong.CaID != "ca-sang" {
		t.Errorf("lỗi nêu ca %q, muốn ca ĐÃ CÓ %q", chong.CaID, "ca-sang")
	}
	if !strings.Contains(chong.Error(), "thứ Hai") {
		t.Errorf("thông điệp không nêu thứ bằng tiếng Việt: %q", chong.Error())
	}

	// (b) touching at exactly one instant — NOT an overlap
	if err := KhongChongCaNao(
		Khoang{Nhom: "1", BatDau: 11*3600 + 30*60, KetThuc: 17 * 3600}, []Khoang{sang}); err != nil {
		t.Errorf("ca chạm nhau ở 11:30 bị coi là chồng: %v", err)
	}

	// (c) another group — two weekdays, or two dates, can never overlap
	if err := KhongChongCaNao(
		Khoang{Nhom: "2", BatDau: 9 * 3600, KetThuc: 12 * 3600}, []Khoang{sang}); err != nil {
		t.Errorf("ca của thứ khác bị coi là chồng: %v", err)
	}

	// (d) TWO EXISTING ROWS THAT OVERLAP EACH OTHER ARE NOT THIS WRITE'S PROBLEM. Refusing here
	// would leave a commune unable to add a Tuesday session until they had fixed a Monday one, and
	// the read route already names that fault.
	hong := []Khoang{
		{ID: "cu-1", Nhom: "1", BatDau: 7 * 3600, KetThuc: 12 * 3600},
		{ID: "cu-2", Nhom: "1", BatDau: 8 * 3600, KetThuc: 13 * 3600},
	}
	if err := KhongChongCaNao(Khoang{Nhom: "2", BatDau: 7 * 3600, KetThuc: 11 * 3600}, hong); err != nil {
		t.Errorf("hai ca cũ chồng nhau làm hỏng một lần ghi không liên quan: %v", err)
	}
}

// THE DATE FORM IS WHAT TELLS THE TWO SENTENCES APART: a swap day's group is a date, a weekly
// session's group is a weekday number. A message saying "ca này chồng giờ với một ca đã có của
// ngày không hợp lệ" is a message nobody can act on.
func TestLoiCaChongCaKhacNoiDungNhomLaNgayHayThu(t *testing.T) {
	theoNgay := (&LoiCaChongCaKhac{CaID: "x", Nhom: "2026-02-21", LaNgay: true}).Error()
	if !strings.Contains(theoNgay, "2026-02-21") {
		t.Errorf("thông điệp theo ngày không nêu ngày: %q", theoNgay)
	}
	theoThu := (&LoiCaChongCaKhac{CaID: "x", Nhom: "6"}).Error()
	if !strings.Contains(theoThu, "thứ Bảy") {
		t.Errorf("thông điệp theo thứ không nêu thứ: %q", theoThu)
	}
}

// ADR 0007 DECISION 9 USES THE SAME ERROR TYPE AND THE SAME `Loai` AS THE COMPUTE PATH. One fault,
// one vocabulary: a commune meeting the rule when they save and a service meeting it when it
// computes a deadline must be told the same thing in the same words.
func TestLoiLamBuVaoNgayDaLamViecDungChungTuVungVoiDuongTinhHan(t *testing.T) {
	err := LoiLamBuVaoNgayDaLamViec("2026-02-21", 6)
	var khongTinh *LoiKhongTinhDuocHan
	if !errors.As(err, &khongTinh) {
		t.Fatalf("lỗi = %v, muốn *LoiKhongTinhDuocHan", err)
	}
	if khongTinh.Loai != LoiLamBuTrungNgayDaLam {
		t.Errorf("loại = %q, muốn %q", khongTinh.Loai, LoiLamBuTrungNgayDaLam)
	}
	for _, phai := range []string{"2026-02-21", "thứ Bảy", "THAY", "CỘNG THÊM"} {
		if !strings.Contains(khongTinh.ChiTiet, phai) {
			t.Errorf("thông điệp thiếu %q: %q", phai, khongTinh.ChiTiet)
		}
	}
}

func TestKiemTraCaLamViec(t *testing.T) {
	ok := CaLamViec{Thu: 1, BatDau: 7*3600 + 30*60, KetThuc: 11*3600 + 30*60}
	if err := KiemTraCaLamViec(ok); err != nil {
		t.Fatalf("ca hợp lệ bị từ chối: %v", err)
	}
	for _, c := range []struct {
		ten string
		ca  CaLamViec
		muc error
	}{
		{"thứ 0 (getDay() của JavaScript)", CaLamViec{Thu: 0, BatDau: 1, KetThuc: 2}, ErrThuKhongHopLe},
		{"thứ 8", CaLamViec{Thu: 8, BatDau: 1, KetThuc: 2}, ErrThuKhongHopLe},
		{"kết thúc trước bắt đầu", CaLamViec{Thu: 1, BatDau: 11 * 3600, KetThuc: 7 * 3600}, ErrCaKhongCoDoDai},
		{"ca dài 0 phút", CaLamViec{Thu: 1, BatDau: 7 * 3600, KetThuc: 7 * 3600}, ErrCaKhongCoDoDai},
	} {
		if err := KiemTraCaLamViec(c.ca); !errors.Is(err, c.muc) {
			t.Errorf("%s: lỗi = %v, muốn %v", c.ten, err, c.muc)
		}
	}
}

// A HOLIDAY WITH NO NAME IS A ROW NOBODY CAN IDENTIFY; a SESSION with no note is an ordinary
// session. The asymmetry is the schema's (`ghi_chu` has DEFAULT ”, `ten` has a CHECK) and it is
// asserted so a later tidy-up does not make them the same.
func TestTenBatBuocChoNgayNghiNhungGhiChuThiKhong(t *testing.T) {
	if _, err := ChuanHoaTenLich("   "); !errors.Is(err, ErrThieuTenLich) {
		t.Errorf("tên rỗng: lỗi = %v, muốn ErrThieuTenLich", err)
	}
	if got, err := ChuanHoaGhiChuCa("   "); err != nil || got != "" {
		t.Errorf("ghi chú rỗng = (%q, %v), muốn (\"\", nil)", got, err)
	}
	dai := strings.Repeat("a", TenLichToiDa+1)
	if _, err := ChuanHoaTenLich(dai); !errors.Is(err, ErrTenLichQuaDai) {
		t.Errorf("tên quá dài: lỗi = %v, muốn ErrTenLichQuaDai", err)
	}
	// COUNTED IN RUNES, NOT BYTES: a Vietnamese sentence is roughly three bytes a character, and a
	// byte bound would cut a commune's own language off at a third of the room it gives English.
	viet := strings.Repeat("ế", TenLichToiDa)
	if _, err := ChuanHoaTenLich(viet); err != nil {
		t.Errorf("tên %d ký tự tiếng Việt bị từ chối: %v", TenLichToiDa, err)
	}
}

// RULE 7, INVARIANT 1: a removal with no reason is a removal nobody can be asked about.
func TestChuanHoaLyDoXoaLich(t *testing.T) {
	if _, err := ChuanHoaLyDoXoaLich("  "); !errors.Is(err, ErrThieuLyDoXoaLich) {
		t.Errorf("lý do rỗng: lỗi = %v, muốn ErrThieuLyDoXoaLich", err)
	}
	if got, err := ChuanHoaLyDoXoaLich("  Xã đổi khung giờ  "); err != nil || got != "Xã đổi khung giờ" {
		t.Errorf("= (%q, %v), muốn (\"Xã đổi khung giờ\", nil)", got, err)
	}
}

// EVERY REFUSAL OF WHAT THE CLIENT SENT IS LISTED, and nothing else is. A default of "anything I do
// not recognise is the client's fault" turns a database outage into a 400.
func TestLaLoiDauVaoLich(t *testing.T) {
	for _, err := range []error{
		ErrThuKhongHopLe, ErrGioKhongDocDuoc, ErrCaKhongCoDoDai, ErrNgayKhongDocDuoc,
		ErrThieuTenLich, ErrTenLichQuaDai, ErrThieuLyDoXoaLich, ErrLyDoXoaLichQuaDai,
	} {
		if !LaLoiDauVaoLich(err) {
			t.Errorf("%v không được nhận là lỗi đầu vào", err)
		}
	}
	for _, err := range []error{
		errors.New("pq: connection refused"),
		&LoiNgayVuaNghiVuaLamBu{Ngay: []string{"2026-02-21"}},
		&LoiCaChongCaKhac{CaID: "x", Nhom: "1"},
	} {
		if LaLoiDauVaoLich(err) {
			t.Errorf("%v bị nhận nhầm là lỗi đầu vào — sẽ trả 400 cho thứ không phải lỗi của client", err)
		}
	}
}
