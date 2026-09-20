package domain

import (
	"strings"
	"testing"
)

// THE THREE THINGS THE SCHEMA CANNOT ENFORCE, TWO OF WHICH LIVE HERE. Migration 0006 states them
// and hands them to the code:
//
//	empty calendar is not a default   a commune with no session has NO working hours, so nothing
//	                                  may fall back to "Mon–Fri 08:00–17:00" (0006:56)
//	overlapping sessions              the EXCLUDE constraint needs btree_gist and a migration that
//	                                  fails for a missing extension stops the service, so the read
//	                                  side has to surface the overlap (0006:109)
//
// The third — a date that is both a holiday and a swap day — spans two tables and is therefore
// caught in the store, where the join is.

const (
	sang0730  GioTrongNgay = 7*3600 + 30*60
	sang1130  GioTrongNgay = 11*3600 + 30*60
	chieu1330 GioTrongNgay = 13*3600 + 30*60
	chieu1700 GioTrongNgay = 17 * 3600
)

func TestGioTrongNgayHienThiCoDinhBaPhan(t *testing.T) {
	// ONE SHAPE ON THE WIRE. A field that dropped ":00" when the seconds were zero would make a
	// client parse two shapes, and a client that handles one handles the other wrong.
	for _, tc := range []struct {
		g    GioTrongNgay
		muon string
	}{
		{0, "00:00:00"},
		{sang0730, "07:30:00"},
		{chieu1700, "17:00:00"},
		// PostgreSQL's TIME accepts 24:00:00 and the CHECK (ket_thuc > bat_dau) permits a session
		// ending there. It is a legal value, not an overflow — the reason this type is an integer
		// rather than a time.Time, which pgx refuses to scan it into at all.
		{GiayTrongNgay, "24:00:00"},
		{7*3600 + 30*60 + 45, "07:30:45"},
	} {
		if got := tc.g.Chuoi(); got != tc.muon {
			t.Errorf("Chuoi(%d) = %q, muốn %q", tc.g, got, tc.muon)
		}
	}
}

func TestLichRongLaMotVanDeCoTenChuKhongPhaiDanhSachRong(t *testing.T) {
	// AN EMPTY CALENDAR IS NOT A DEFAULT. This is the assertion that stops the next person from
	// reading `len(items) == 0` as "the ordinary office week" — a default here is a commitment
	// invented by software and then told to a citizen (rule 10).
	vd := VanDeCuaLich(nil)
	if len(vd) != 1 {
		t.Fatalf("nhận %d vấn đề, muốn 1", len(vd))
	}
	if vd[0].Loai != VanDeLichTrong {
		t.Errorf("loại = %q, muốn %q", vd[0].Loai, VanDeLichTrong)
	}
	if vd[0].Thu != 0 {
		t.Errorf("thu = %d — lịch rỗng không thuộc thứ nào, và tuyến đọc dịch 0 thành null", vd[0].Thu)
	}
	if vd[0].CaID == nil {
		t.Error("session_ids phải là lát rỗng, không phải nil")
	}
}

func TestLichDayDuKhongCoVanDe(t *testing.T) {
	// The ordinary week: a morning and an afternoon with a lunch break between them. TOUCHING OR
	// SEPARATE SESSIONS ARE NOT AN OVERLAP — a day with no lunch break would have 07:30–11:30 and
	// 11:30–17:00, and reporting that as a clash would bury the real ones under noise.
	cas := []CaLamViec{
		{ID: "a", Thu: 1, BatDau: sang0730, KetThuc: sang1130},
		{ID: "b", Thu: 1, BatDau: sang1130, KetThuc: chieu1700},
		{ID: "c", Thu: 2, BatDau: sang0730, KetThuc: sang1130},
		{ID: "d", Thu: 2, BatDau: chieu1330, KetThuc: chieu1700},
	}
	if vd := VanDeCuaLich(cas); len(vd) != 0 {
		t.Fatalf("tuần hợp lệ mà bị báo vấn đề: %+v", vd)
	}
}

func TestHaiCaChongNhauTrongMotThuThiBiNeuTen(t *testing.T) {
	// THE DOUBLE COUNT THE DATABASE CANNOT REFUSE. UNIQUE (tenant_id, thu, bat_dau) stops two
	// sessions STARTING at the same minute and nothing more, so 07:30–11:30 beside 09:00–12:00 is
	// a perfectly acceptable row pair that counts two and a half hours twice.
	cas := []CaLamViec{
		{ID: "sang", Thu: 3, BatDau: sang0730, KetThuc: sang1130},
		{ID: "chong", Thu: 3, BatDau: 9 * 3600, KetThuc: 12 * 3600},
	}
	vd := VanDeCuaLich(cas)
	if len(vd) != 1 || vd[0].Loai != VanDeCaChongNhau {
		t.Fatalf("nhận %+v, muốn đúng một vấn đề chồng ca", vd)
	}
	if vd[0].Thu != 3 {
		t.Errorf("thu = %d, muốn 3 — màn hình cấu hình cần biết sửa ngày nào", vd[0].Thu)
	}
	// The earlier session first, so the pair reads the way a person scans the screen.
	if len(vd[0].CaID) != 2 || vd[0].CaID[0] != "sang" || vd[0].CaID[1] != "chong" {
		t.Errorf("session_ids = %v, muốn [sang chong]", vd[0].CaID)
	}
}

func TestCaChongNhauKhacThuThiKhongPhaiVanDe(t *testing.T) {
	// THE GROUP IS WHAT MAKES THE CHECK MEAN ANYTHING. Without it every Monday morning would be
	// reported as clashing with every Tuesday morning, and the real clashes would be unreadable.
	cas := []CaLamViec{
		{ID: "hai", Thu: 1, BatDau: sang0730, KetThuc: chieu1700},
		{ID: "ba", Thu: 2, BatDau: sang0730, KetThuc: chieu1700},
	}
	if vd := VanDeCuaLich(cas); len(vd) != 0 {
		t.Fatalf("hai ca khác thứ bị báo chồng nhau: %+v", vd)
	}
}

func TestMotCaDaiNuotHaiCaNganThiNeuCaHai(t *testing.T) {
	// A SWEEP COMPARING ONLY NEIGHBOURS MISSES THIS, which is why the check is pairwise: the long
	// session swallows both short ones, and a "compare with the previous row" implementation would
	// report only the first pair.
	cas := []CaLamViec{
		{ID: "cangay", Thu: 4, BatDau: sang0730, KetThuc: chieu1700},
		{ID: "sang", Thu: 4, BatDau: 8 * 3600, KetThuc: 9 * 3600},
		{ID: "chieu", Thu: 4, BatDau: 14 * 3600, KetThuc: 15 * 3600},
	}
	vd := VanDeCuaLich(cas)
	if len(vd) != 2 {
		t.Fatalf("nhận %d vấn đề, muốn 2 — ca dài chồng lên cả hai ca ngắn: %+v", len(vd), vd)
	}
}

func TestCaLamBuChongNhauTinhTheoNgayChuKhongTheoThu(t *testing.T) {
	// The same defect on the swap-day table, grouped by DATE. Two sessions on two different swap
	// days cannot overlap, however similar their hours.
	cas := []CaLamBu{
		{ID: "a", Ngay: "2026-02-21", BatDau: sang0730, KetThuc: sang1130},
		{ID: "b", Ngay: "2026-02-21", BatDau: 9 * 3600, KetThuc: 12 * 3600},
		{ID: "c", Ngay: "2026-03-07", BatDau: sang0730, KetThuc: sang1130},
	}
	vd := CaLamBuChongNhau(cas)
	if len(vd) != 1 {
		t.Fatalf("nhận %d vấn đề, muốn 1: %+v", len(vd), vd)
	}
	if vd[0].Ngay != "2026-02-21" {
		t.Errorf("ngày = %q, muốn 2026-02-21", vd[0].Ngay)
	}
	if len(vd[0].CaID) != 2 || vd[0].CaID[0] != "a" || vd[0].CaID[1] != "b" {
		t.Errorf("session_ids = %v, muốn [a b]", vd[0].CaID)
	}
}

func TestThongBaoXungDotNeuNgayVaDemPhanConLai(t *testing.T) {
	// The message is read by a person who has to go and correct one of two rows, so it names the
	// days. A date is not personal data (rule 3). Past five it counts the rest rather than printing
	// a wall of text nobody reads to the end.
	e := &LoiNgayVuaNghiVuaLamBu{Ngay: []string{
		"2026-01-01", "2026-02-14", "2026-03-07", "2026-04-30", "2026-05-01", "2026-09-02",
	}}
	s := e.Error()
	if !strings.Contains(s, "2026-01-01") || !strings.Contains(s, "2026-05-01") {
		t.Errorf("thông báo không nêu năm ngày đầu: %q", s)
	}
	if strings.Contains(s, "2026-09-02") {
		t.Errorf("thông báo nêu quá %d ngày: %q", SoNgayXungDotNeuTen, s)
	}
	if !strings.Contains(s, "1 ngày khác") {
		t.Errorf("thông báo không đếm phần còn lại: %q", s)
	}
}
