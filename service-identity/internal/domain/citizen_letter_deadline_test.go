package domain

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

// WHAT THESE TESTS DEFEND: the unit lock (every cell of the owner's 08/10 table, and every unit that
// is not the cell's), and the Civil-Code day count — day one not counted, a last date on a closure
// moved forward, the end of the commune's own working hours, the date taken in Vietnam's zone, the
// year boundary, a swap Saturday, and every refusal the working-hours walk already gives.
//
// Expectations are written as wall-clock times in a FIXED +07 (muiDoiChung, tien_gio_lam_viec_test.go)
// so they are an oracle independent of the code under test. 2026-09-21 is a Monday.

func TestRequiredCitizenLetterUnitIsTheOwnersTable(t *testing.T) {
	cases := []struct {
		letter CitizenLetterType
		kind   CitizenLetterDeadlineKind
		want   CitizenLetterDeadlineUnit
		err    error
	}{
		{CitizenLetterKienNghiPhanAnh, CitizenLetterProcessing, UnitWorkingDays, nil},
		{CitizenLetterKienNghiPhanAnh, CitizenLetterResolution, "", ErrCitizenLetterNoResolutionDeadline},
		{CitizenLetterDeNghi, CitizenLetterProcessing, UnitWorkingDays, nil},
		{CitizenLetterDeNghi, CitizenLetterResolution, "", ErrCitizenLetterNoResolutionDeadline},
		{CitizenLetterKhieuNai, CitizenLetterProcessing, UnitCalendarDays, nil},
		{CitizenLetterKhieuNai, CitizenLetterResolution, UnitCalendarDays, nil},
		{CitizenLetterToCao, CitizenLetterProcessing, UnitWorkingDays, nil},
		{CitizenLetterToCao, CitizenLetterResolution, UnitCalendarDays, nil},
		{"phan-anh", CitizenLetterProcessing, "", ErrCitizenLetterTypeUnknown},
		{CitizenLetterKhieuNai, "tiep-nhan", "", ErrCitizenLetterDeadlineKindUnknown},
	}
	for _, c := range cases {
		got, err := RequiredCitizenLetterUnit(c.letter, c.kind)
		if got != c.want || !errors.Is(err, c.err) || (c.err == nil && err != nil) {
			t.Errorf("%s/%s = %q, %v — muốn %q, %v", c.letter, c.kind, got, err, c.want, c.err)
		}
	}
}

// A ROW WHOSE UNIT IS NOT ITS CELL'S IS REFUSED — including working HOURS on every cell, which the
// schema admits and the owner did not choose.
func TestCheckCitizenLetterDeadlineRuleRefusesEveryOtherUnit(t *testing.T) {
	cells := []struct {
		letter CitizenLetterType
		kind   CitizenLetterDeadlineKind
	}{
		{CitizenLetterKienNghiPhanAnh, CitizenLetterProcessing},
		{CitizenLetterDeNghi, CitizenLetterProcessing},
		{CitizenLetterKhieuNai, CitizenLetterProcessing},
		{CitizenLetterKhieuNai, CitizenLetterResolution},
		{CitizenLetterToCao, CitizenLetterProcessing},
		{CitizenLetterToCao, CitizenLetterResolution},
	}
	for _, c := range cells {
		want, _ := RequiredCitizenLetterUnit(c.letter, c.kind)
		for _, u := range []CitizenLetterDeadlineUnit{UnitWorkingHours, UnitWorkingDays, UnitCalendarDays} {
			err := CheckCitizenLetterDeadlineRule(CitizenLetterDeadlineRule{
				LetterType: c.letter, Kind: c.kind, Amount: 10, Unit: u})
			if u == want && err != nil {
				t.Errorf("%s/%s/%s bị từ chối: %v", c.letter, c.kind, u, err)
			}
			if u != want && !errors.Is(err, ErrCitizenLetterDeadlineUnitLocked) {
				t.Errorf("%s/%s/%s được nhận (lỗi %v) — khoá đơn vị bị thủng", c.letter, c.kind, u, err)
			}
		}
	}
}

func TestCheckCitizenLetterDeadlineRuleAmountAndUnknowns(t *testing.T) {
	base := CitizenLetterDeadlineRule{LetterType: CitizenLetterKhieuNai, Kind: CitizenLetterProcessing,
		Amount: 10, Unit: UnitCalendarDays}
	cases := []struct {
		name string
		edit func(*CitizenLetterDeadlineRule)
		err  error
	}{
		{"zero", func(r *CitizenLetterDeadlineRule) { r.Amount = 0 }, ErrCitizenLetterAmountNotPositive},
		{"negative", func(r *CitizenLetterDeadlineRule) { r.Amount = -3 }, ErrCitizenLetterAmountNotPositive},
		{"over ceiling", func(r *CitizenLetterDeadlineRule) { r.Amount = CitizenLetterDeadlineMaxAmount + 1 }, ErrCitizenLetterAmountTooLarge},
		{"ceiling", func(r *CitizenLetterDeadlineRule) { r.Amount = CitizenLetterDeadlineMaxAmount }, nil},
		{"unknown unit", func(r *CitizenLetterDeadlineRule) { r.Unit = "ngay" }, ErrCitizenLetterDeadlineUnitUnknown},
		{"unknown type", func(r *CitizenLetterDeadlineRule) { r.LetterType = "" }, ErrCitizenLetterTypeUnknown},
	}
	for _, c := range cases {
		r := base
		c.edit(&r)
		err := CheckCitizenLetterDeadlineRule(r)
		if (c.err == nil && err != nil) || !errors.Is(err, c.err) {
			t.Errorf("%s: lỗi %v, muốn %v", c.name, err, c.err)
		}
	}
}

func TestNormalizeCitizenLetterDeleteReason(t *testing.T) {
	if r, err := NormalizeCitizenLetterDeleteReason("  Sai số  "); err != nil || r != "Sai số" {
		t.Errorf("= %q, %v", r, err)
	}
	if _, err := NormalizeCitizenLetterDeleteReason("   "); !errors.Is(err, ErrCitizenLetterDeleteReasonMissing) {
		t.Errorf("lý do trống: %v", err)
	}
	long := make([]rune, CitizenLetterDeleteReasonMaxLen+1)
	for i := range long {
		long[i] = 'ă'
	}
	if _, err := NormalizeCitizenLetterDeleteReason(string(long)); !errors.Is(err, ErrCitizenLetterDeleteReasonTooLong) {
		t.Errorf("lý do dài: %v", err)
	}
}

// --- the day count -------------------------------------------------------------------------------

// weekClosing1630 is a week whose afternoon ends at 16:30 — the commune's own end of day.
func weekClosing1630() []CaLamViec {
	var out []CaLamViec
	for weekday := 1; weekday <= 5; weekday++ {
		out = append(out,
			CaLamViec{ID: "am", Thu: weekday, BatDau: gio(7, 0), KetThuc: gio(11, 0)},
			CaLamViec{ID: "pm", Thu: weekday, BatDau: gio(13, 0), KetThuc: gio(16, 30)},
		)
	}
	return out
}

func TestCitizenLetterDueAt(t *testing.T) {
	nationalDay := []NgayNghiLe{{ID: "nd", Ngay: "2026-09-02"}}
	newYearsDay := []NgayNghiLe{{ID: "ny", Ngay: "2027-01-01"}}
	swapSaturday := []CaLamBu{{ID: "sw", Ngay: "2026-10-03", BatDau: gio(8, 0), KetThuc: gio(11, 0)}}

	cases := []struct {
		name   string
		from   time.Time
		amount int
		unit   CitizenLetterDeadlineUnit
		week   []CaLamViec
		cal    *lichGiaNam
		want   string
	}{
		{"ngày lịch, rơi vào ngày làm việc", luc(t, "2026-09-21 15:00"), 10, UnitCalendarDays,
			tuanChuan(), nil, "2026-10-01 17:00"},
		// Day one is NOT counted: one calendar day from a Monday morning is Tuesday, never Monday.
		{"không tính ngày đầu", luc(t, "2026-09-21 07:00"), 1, UnitCalendarDays,
			tuanChuan(), nil, "2026-09-22 17:00"},
		{"ngày lịch rơi thứ Bảy dời sang thứ Hai", luc(t, "2026-09-21 09:00"), 5, UnitCalendarDays,
			tuanChuan(), nil, "2026-09-28 17:00"},
		{"ngày lịch rơi ngày nghỉ lễ dời sang hôm sau", luc(t, "2026-08-28 09:00"), 5, UnitCalendarDays,
			tuanChuan(), &lichGiaNam{nghi: nationalDay}, "2026-09-03 17:00"},
		{"ngày làm việc bỏ cuối tuần", luc(t, "2026-09-21 09:00"), 7, UnitWorkingDays,
			tuanChuan(), nil, "2026-09-30 17:00"},
		{"ngày làm việc bỏ ngày nghỉ lễ", luc(t, "2026-08-31 09:00"), 3, UnitWorkingDays,
			tuanChuan(), &lichGiaNam{nghi: nationalDay}, "2026-09-04 17:00"},
		{"nhận thứ Bảy, một ngày làm việc", luc(t, "2026-09-26 10:00"), 1, UnitWorkingDays,
			tuanChuan(), nil, "2026-09-28 17:00"},
		// A swap Saturday is a working date, and its own session decides the end of day.
		{"thứ Bảy làm bù là ngày làm việc, hết giờ theo ca làm bù", luc(t, "2026-09-28 09:00"), 5, UnitWorkingDays,
			tuanChuan(), &lichGiaNam{lamBu: swapSaturday}, "2026-10-03 11:00"},
		{"ngày lịch rơi đúng thứ Bảy làm bù thì không dời", luc(t, "2026-09-28 09:00"), 5, UnitCalendarDays,
			tuanChuan(), &lichGiaNam{lamBu: swapSaturday}, "2026-10-03 11:00"},
		{"qua năm, ngày lịch rơi thứ Bảy sau Tết dương", luc(t, "2026-12-28 09:00"), 5, UnitCalendarDays,
			tuanChuan(), &lichGiaNam{nghi: newYearsDay}, "2027-01-04 17:00"},
		{"qua năm, ngày làm việc bỏ Tết dương", luc(t, "2026-12-30 09:00"), 2, UnitWorkingDays,
			tuanChuan(), &lichGiaNam{nghi: newYearsDay}, "2027-01-04 17:00"},
		{"hết giờ là giờ đóng cửa của xã, không phải 17:00", luc(t, "2026-09-21 09:00"), 2, UnitWorkingDays,
			weekClosing1630(), nil, "2026-09-23 16:30"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cal := c.cal
			if cal == nil {
				cal = &lichGiaNam{}
			}
			got, err := CitizenLetterDueAt(c.from, c.amount, c.unit, c.week, cal.doc)
			if err != nil {
				t.Fatalf("lỗi: %v", err)
			}
			bang(t, got, luc(t, c.want), c.name)
		})
	}
}

// THE DATE IS VIETNAM'S. 00:30 on 22/09 in Vietnam is 17:30 on 21/09 in UTC; counting from the UTC date
// would put the deadline a day early on every letter received between midnight and 07:00.
func TestCitizenLetterDueAtTakesTheVietnameseDate(t *testing.T) {
	from := time.Date(2026, 9, 21, 17, 30, 0, 0, time.UTC) // 2026-09-22 00:30 +07
	got, err := CitizenLetterDueAt(from, 1, UnitCalendarDays, tuanChuan(), (&lichGiaNam{}).doc)
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	bang(t, got, luc(t, "2026-09-23 17:00"), "ngày nhận theo giờ Việt Nam")
}

// The year after is read only when the count reaches it.
func TestCitizenLetterDueAtReadsYearsLazily(t *testing.T) {
	cal := &lichGiaNam{nghi: []NgayNghiLe{{ID: "ny", Ngay: "2027-01-01"}}}
	if _, err := CitizenLetterDueAt(luc(t, "2026-12-30 09:00"), 2, UnitWorkingDays, tuanChuan(), cal.doc); err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if !reflect.DeepEqual(cal.namDaDoc, []int{2026, 2027}) {
		t.Errorf("năm đã đọc = %v, muốn [2026 2027]", cal.namDaDoc)
	}
	cal = &lichGiaNam{}
	if _, err := CitizenLetterDueAt(luc(t, "2026-09-21 09:00"), 10, UnitCalendarDays, tuanChuan(), cal.doc); err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if !reflect.DeepEqual(cal.namDaDoc, []int{2026}) {
		t.Errorf("năm đã đọc = %v, muốn [2026]", cal.namDaDoc)
	}
}

// EVERY REFUSAL IS THE CONFIGURATION'S (LoiKhongTinhDuocHan → FAILED_PRECONDITION at the boundary),
// never an instant — for calendar days too, which still need the week for the move and the end hour.
func TestCitizenLetterDueAtRefusals(t *testing.T) {
	// A swap day on a Tuesday, which already works: ADR 0007 decision 9.
	swapOnWeekday := &lichGiaNam{lamBu: []CaLamBu{{ID: "x", Ngay: "2026-09-22", BatDau: gio(8, 0), KetThuc: gio(9, 0)}}}
	cases := []struct {
		name   string
		amount int
		unit   CitizenLetterDeadlineUnit
		week   []CaLamViec
		cal    *lichGiaNam
		kind   LoaiLoiTinhHan
	}{
		{"lịch trống, ngày lịch", 10, UnitCalendarDays, nil, &lichGiaNam{}, LoiLichTrong},
		{"lịch trống, ngày làm việc", 10, UnitWorkingDays, nil, &lichGiaNam{}, LoiLichTrong},
		{"làm bù trên ngày đã làm", 1, UnitWorkingDays, tuanChuan(), swapOnWeekday, LoiLamBuTrungNgayDaLam},
		{"vượt chân trời", CitizenLetterDeadlineMaxAmount, UnitWorkingDays, tuanChuan(), &lichGiaNam{}, LoiVuotChanTroi},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := CitizenLetterDueAt(luc(t, "2026-09-21 09:00"), c.amount, c.unit, c.week, c.cal.doc)
			var configErr *LoiKhongTinhDuocHan
			if !errors.As(err, &configErr) || configErr.Loai != c.kind {
				t.Fatalf("= %s, %v — muốn từ chối loại %s", got, err, c.kind)
			}
		})
	}

	// A holiday that is also a swap day is refused by the read itself; it travels out unchanged.
	conflict := &LoiNgayVuaNghiVuaLamBu{Ngay: []string{"2026-09-22"}}
	_, err := CitizenLetterDueAt(luc(t, "2026-09-21 09:00"), 1, UnitWorkingDays, tuanChuan(),
		(&lichGiaNam{err: conflict}).doc)
	if !errors.As(err, new(*LoiNgayVuaNghiVuaLamBu)) {
		t.Errorf("xung đột lịch: %v", err)
	}
}

// Working hours, or a non-positive amount, reaching the arithmetic is this service's own fault.
func TestCitizenLetterDueAtRefusesWhatTheLockRefuses(t *testing.T) {
	for _, c := range []struct {
		amount int
		unit   CitizenLetterDeadlineUnit
	}{{10, UnitWorkingHours}, {0, UnitCalendarDays}, {10, "ngay"}} {
		_, err := CitizenLetterDueAt(luc(t, "2026-09-21 09:00"), c.amount, c.unit, tuanChuan(), (&lichGiaNam{}).doc)
		if err == nil || errors.As(err, new(*LoiKhongTinhDuocHan)) {
			t.Errorf("%d %s: lỗi %v — muốn lỗi thường, không phải lỗi cấu hình", c.amount, c.unit, err)
		}
	}
}
