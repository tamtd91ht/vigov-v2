package domain

import (
	"strings"
	"testing"
)

// WHAT THIS FILE GUARDS: the two seed sets, and above all WHAT IS NOT IN THEM.
//
// The expensive failure here is not a wrong hour — a commune edits an hour on day one. It is a
// LUNAR DATE computed in Go. Tết Nguyên đán and Giỗ Tổ Hùng Vương move every year against the solar
// calendar, every hand-rolled conversion differs from the official calendar in some year, and a
// holiday that is one day out does not fail: it produces a deadline counted through a day the office
// was shut, in the direction that reports an authority late when it was not. That figure goes into a
// report sent upward, and the citizen who was told "within 2 hours" is the one who finds out.

// TEN SESSIONS, MONDAY TO FRIDAY, MORNING AND AFTERNOON — and nothing for Saturday or Sunday. A
// commune that does not work a day has NO ROW for it (migration 0006:88), so a seven-day seed would
// be writing two days the commune does not work.
func TestBoGieoCaLamViecLaMuoiCaTuThuHaiDenThuSau(t *testing.T) {
	bo := BoGieoCaLamViec()
	if len(bo) != 10 {
		t.Fatalf("bộ gieo có %d ca, muốn 10", len(bo))
	}
	theoThu := map[int]int{}
	for _, g := range bo {
		theoThu[g.Thu]++
		// EVERY SEED ROW MUST PASS THE SAME CHECK A TYPED ROW DOES. A seed set that could not be
		// entered by hand is a seed set nobody can reproduce or correct.
		if err := KiemTraCaLamViec(CaLamViec{
			Thu: g.Thu, BatDau: g.BatDau, KetThuc: g.KetThuc,
		}); err != nil {
			t.Errorf("%s %s–%s: dòng gieo không hợp lệ: %v",
				TenThuISO(g.Thu), g.BatDau.Chuoi(), g.KetThuc.Chuoi(), err)
		}
	}
	for thu := 1; thu <= 5; thu++ {
		if theoThu[thu] != 2 {
			t.Errorf("%s có %d ca, muốn 2", TenThuISO(thu), theoThu[thu])
		}
	}
	if theoThu[6] != 0 || theoThu[7] != 0 {
		t.Errorf("bộ gieo có ca thứ Bảy (%d) hoặc Chủ nhật (%d) — xã không làm việc hai ngày đó thì "+
			"KHÔNG có dòng nào", theoThu[6], theoThu[7])
	}
}

// THE SEED SET NEVER OVERLAPS ITSELF. Ten rows written in one transaction that clashed with each
// other would leave the commune's calendar uncomputable the moment it was created.
func TestBoGieoCaLamViecKhongTuChongLenChinhNo(t *testing.T) {
	var ks []Khoang
	for i, g := range BoGieoCaLamViec() {
		ks = append(ks, Khoang{
			ID: string(rune('a' + i)), Nhom: TenThuISO(g.Thu), BatDau: g.BatDau, KetThuc: g.KetThuc,
		})
	}
	if cap := TimChongNhau(ks); len(cap) > 0 {
		t.Errorf("bộ gieo tự chồng giờ: %+v", cap)
	}
}

// THE LUNCH BREAK IS A GAP, NOT A FIELD. Over a 40-hour deadline a mishandled lunch break is a full
// working day of drift, in whichever direction happens to be wrong.
func TestBoGieoCaLamViecCoKhoangNghiTrua(t *testing.T) {
	if GioDongSang >= GioMoChieu {
		t.Fatalf("ca sáng kết thúc %s, ca chiều bắt đầu %s — không có khoảng nghỉ trưa",
			GioDongSang.Chuoi(), GioMoChieu.Chuoi())
	}
	if GioMoSang >= GioDongSang || GioMoChieu >= GioDongChieu {
		t.Fatal("một trong hai ca mặc định có độ dài âm hoặc bằng 0")
	}
}

// ⚠ THE CASE THIS WHOLE FILE EXISTS FOR. Four fixed solar dates, and NOTHING that moves with the
// lunar calendar or with an annual announcement.
//
// MUTATION THAT MUST TURN THIS RED: add any Tết row, any Giỗ Tổ row, or 01/9 or 03/9, to
// BoGieoNgayNghiLe.
func TestBoGieoNgayNghiLeChiCoNgayCoDinhTheoDuongLich(t *testing.T) {
	for _, nam := range []int{2026, 2027, 2030} {
		bo := BoGieoNgayNghiLe(nam)
		if len(bo) != 4 {
			t.Fatalf("năm %d: bộ gieo có %d ngày, muốn 4", nam, len(bo))
		}
		muon := map[string]bool{
			"01-01": true, // Tết Dương lịch
			"04-30": true, // Ngày Chiến thắng
			"05-01": true, // Ngày Quốc tế lao động
			"09-02": true, // Quốc khánh
		}
		for _, g := range bo {
			ngay, err := ChuanHoaNgay(g.Ngay)
			if err != nil {
				t.Errorf("năm %d: ngày %q không hợp khuôn: %v", nam, g.Ngay, err)
				continue
			}
			if !strings.HasPrefix(ngay, itoa4(nam)+"-") {
				t.Errorf("năm %d: gieo ngày %s của năm khác", nam, ngay)
			}
			thang := ngay[5:]
			if !muon[thang] {
				t.Errorf("năm %d: gieo ngày %s — bộ gieo CHỈ được có ngày lễ CỐ ĐỊNH theo dương lịch. "+
					"Tết Nguyên đán và Giỗ Tổ Hùng Vương theo ÂM LỊCH, và ngày liền kề 02/9 do Thủ "+
					"tướng chọn từng năm giữa 01/9 và 03/9 — xã tự nhập cả ba", nam, ngay)
			}
			delete(muon, thang)
			if err := KiemTraNgayNghiLe(NgayNghiLe{Ngay: g.Ngay, Ten: g.Ten}); err != nil {
				t.Errorf("năm %d: dòng gieo %s không hợp lệ: %v", nam, ngay, err)
			}
		}
		if len(muon) != 0 {
			t.Errorf("năm %d: thiếu các ngày %v", nam, muon)
		}
	}
}

// A LEAP YEAR CHANGES NOTHING HERE, and that is worth one case: all four dates sit after 29 February
// or on 01/01, so none of them can shift. A seed that broke on a leap year would break once every
// four years, in a year nobody tested.
func TestBoGieoNgayNghiLeHopLeCaTrongNamNhuan(t *testing.T) {
	for _, nam := range []int{2024, 2028} {
		for _, g := range BoGieoNgayNghiLe(nam) {
			if _, err := ChuanHoaNgay(g.Ngay); err != nil {
				t.Errorf("năm nhuận %d: ngày %q không hợp khuôn: %v", nam, g.Ngay, err)
			}
		}
	}
}

// itoa4 renders a year as four digits, matching the seed's own formatting. Written here rather than
// imported from strconv so the assertion fails on a year the seed would render differently.
func itoa4(n int) string {
	s := ""
	for i := 0; i < 4; i++ {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
