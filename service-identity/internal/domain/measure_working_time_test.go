package domain

import (
	"errors"
	"testing"
	"time"
)

// WHAT THESE TESTS DEFEND: MeasureWorkingTime is the INVERSE of TienGioLamViec against the same
// calendar — measure(t, advance(t, h)) == h hours — on every hard case ADR 0007 names, and it never
// answers 0 for a commune with no calendar. Expected instants are written out in the fixed +07 oracle
// `luc`, never recomputed with the code under test.

func measureOne(t *testing.T, start, end time.Time, week []CaLamViec, cal *lichGiaNam) time.Duration {
	t.Helper()
	if cal == nil {
		cal = &lichGiaNam{}
	}
	got, err := MeasureWorkingTime([]WorkingSpan{{Start: start, End: end}}, week, cal.doc)
	if err != nil {
		t.Fatalf("MeasureWorkingTime: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("số kết quả = %d, muốn 1", len(got))
	}
	return got[0]
}

// THE INVERSE PROPERTY on every hard case. The advance is computed by TienGioLamViec and its result
// is ALSO checked against a written-out instant, so a bug shared by both directions cannot pass.
func TestMeasureIsInverseOfAdvance(t *testing.T) {
	saturdayOnly := []CaLamViec{{ID: "truc-7", Thu: 6, BatDau: gio(7, 30), KetThuc: gio(11, 30)}}
	holiday := func() *lichGiaNam {
		return &lichGiaNam{nghi: []NgayNghiLe{{ID: "le-1", Ngay: "2026-09-21", Ten: "Lễ giả định"}}}
	}
	swapDay := func() *lichGiaNam {
		return &lichGiaNam{lamBu: []CaLamBu{{
			ID: "bu-1", Ngay: "2026-09-19", BatDau: gio(7, 30), KetThuc: gio(11, 30), Ten: "Làm bù giả định",
		}}}
	}
	cases := []struct {
		name    string
		from    string
		hours   int
		week    []CaLamViec
		cal     func() *lichGiaNam
		reached string
	}{
		{"qua cuối tuần, từ 22:00 thứ Sáu", "2026-09-18 22:00", 2, tuanChuan(), nil, "2026-09-21 09:30"},
		{"thứ Hai nghỉ lễ", "2026-09-18 22:00", 2, tuanChuan(), holiday, "2026-09-22 09:30"},
		{"thứ Bảy làm bù", "2026-09-18 22:00", 2, tuanChuan(), swapDay, "2026-09-19 09:30"},
		{"vắt qua nghỉ trưa", "2026-09-21 08:00", 5, tuanChuan(), nil, "2026-09-21 15:00"},
		{"hết đúng 11:30", "2026-09-21 08:30", 3, tuanChuan(), nil, "2026-09-21 11:30"},
		{"hết đúng 17:00", "2026-09-21 08:00", 7, tuanChuan(), nil, "2026-09-21 17:00"},
		{"168 giờ", "2026-09-21 07:30", 168, tuanChuan(), nil, "2026-10-21 10:30"},
		{"xã chỉ làm sáng thứ Bảy", "2026-09-18 22:00", 6, saturdayOnly, nil, "2026-09-26 09:30"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			newCal := func() *lichGiaNam {
				if c.cal == nil {
					return &lichGiaNam{}
				}
				return c.cal()
			}
			from := luc(t, c.from)
			reached := mot(t, from, c.week, newCal(), c.hours)
			bang(t, reached, luc(t, c.reached), "mốc tiến giờ")

			got := measureOne(t, from, reached, c.week, newCal())
			if want := time.Duration(c.hours) * time.Hour; got != want {
				t.Errorf("đo(%s → %s) = %s, muốn %s", c.from, c.reached, got, want)
			}
		})
	}
}

// An endpoint outside working time is NOT snapped anywhere and contributes nothing: Friday 22:00 and
// Monday 07:30 measure the same to any later instant (ADR 0007 decision 8, by shape).
func TestMeasureFromClosedTimeEqualsFromNextOpening(t *testing.T) {
	end := luc(t, "2026-09-21 10:00")
	a := measureOne(t, luc(t, "2026-09-18 22:00"), end, tuanChuan(), nil)
	b := measureOne(t, luc(t, "2026-09-21 07:30"), end, tuanChuan(), nil)
	if a != b || a != 150*time.Minute {
		t.Errorf("từ 22:00 thứ Sáu = %s, từ 07:30 thứ Hai = %s, muốn cả hai 2h30m", a, b)
	}
}

func TestMeasureZeroCases(t *testing.T) {
	at := luc(t, "2026-09-21 09:00")
	if got := measureOne(t, at, at, tuanChuan(), nil); got != 0 {
		t.Errorf("start == end đo được %s, muốn 0", got)
	}
	// Saturday 08:00 → Sunday 20:00: a weekend holds no working time in the ordinary week.
	if got := measureOne(t, luc(t, "2026-09-19 08:00"), luc(t, "2026-09-20 20:00"), tuanChuan(), nil); got != 0 {
		t.Errorf("khoảng nằm trọn trong cuối tuần đo được %s, muốn 0", got)
	}
	// Inside the lunch break.
	if got := measureOne(t, luc(t, "2026-09-21 11:45"), luc(t, "2026-09-21 13:15"), tuanChuan(), nil); got != 0 {
		t.Errorf("khoảng nằm trọn trong nghỉ trưa đo được %s, muốn 0", got)
	}
}

// LỊCH TRỐNG KHÔNG PHẢI "0 GIỜ": 0 đọc thành "không trễ" trên mọi dòng của hàng đợi quá hạn. Từ chối,
// kể cả khi mọi khoảng đều rỗng.
func TestMeasureEmptyCalendarIsRefusedNotZero(t *testing.T) {
	for name, end := range map[string]string{"khoảng thường": "2026-09-22 09:00", "start == end": "2026-09-21 09:00"} {
		t.Run(name, func(t *testing.T) {
			got, err := MeasureWorkingTime([]WorkingSpan{{Start: luc(t, "2026-09-21 09:00"), End: luc(t, end)}},
				nil, (&lichGiaNam{}).doc)
			var refusal *LoiKhongTinhDuocHan
			if !errors.As(err, &refusal) || refusal.Loai != LoiLichTrong {
				t.Fatalf("lịch trống trả %v, lỗi %v — muốn LoiLichTrong", got, err)
			}
		})
	}
}

// Many spans, one walk: each measured from the same origin, duplicates and order irrelevant, and the
// horizon fault never produced even for a span wider than 366 days.
func TestMeasureManySpansOneWalkNoHorizon(t *testing.T) {
	cal := &lichGiaNam{}
	spans := []WorkingSpan{
		{Start: luc(t, "2026-09-21 08:00"), End: luc(t, "2026-09-21 15:00")}, // 5h
		{Start: luc(t, "2026-09-18 22:00"), End: luc(t, "2026-09-21 09:30")}, // 2h
		{Start: luc(t, "2026-09-21 08:00"), End: luc(t, "2026-09-21 15:00")}, // duplicate
		{Start: luc(t, "2026-09-21 07:30"), End: luc(t, "2028-09-21 07:30")}, // two years
	}
	got, err := MeasureWorkingTime(spans, tuanChuan(), cal.doc)
	if err != nil {
		t.Fatalf("MeasureWorkingTime: %v", err)
	}
	if got[0] != 5*time.Hour || got[1] != 2*time.Hour || got[2] != 5*time.Hour {
		t.Errorf("kết quả = %v, muốn [5h 2h 5h …]", got[:3])
	}
	// 2026-09-21 → 2028-09-21 is 731 days = 104 weeks + 3 days (Mon, Tue, Wed): 523 working days.
	if want := 523 * (7*time.Hour + 30*time.Minute); got[3] != want {
		t.Errorf("hai năm đo được %s, muốn %s", got[3], want)
	}
	// One read per year entered, never a year twice.
	if len(cal.namDaDoc) != 3 {
		t.Errorf("đã đọc các năm %v, muốn đúng 2026, 2027, 2028 mỗi năm một lần", cal.namDaDoc)
	}
}

// END IS EXCLUSIVE: a span ending at 00:00 on 1 January never enters the new year, so a fault there
// cannot change the answer.
func TestMeasureDoesNotReadYearItNeverEnters(t *testing.T) {
	cal := &lichGiaNam{}
	measureOne(t, luc(t, "2026-12-30 08:00"), luc(t, "2027-01-01 00:00"), tuanChuan(), cal)
	for _, y := range cal.namDaDoc {
		if y == 2027 {
			t.Fatalf("đã đọc năm 2027 cho một khoảng kết thúc lúc 00:00 ngày 01/01/2027: %v", cal.namDaDoc)
		}
	}
}

// Calendar and store faults come out unchanged.
func TestMeasureReturnsWalkErrorsUnchanged(t *testing.T) {
	store := errors.New("giả lập: kho hỏng")
	_, err := MeasureWorkingTime([]WorkingSpan{{Start: luc(t, "2026-09-21 08:00"), End: luc(t, "2026-09-22 08:00")}},
		tuanChuan(), (&lichGiaNam{err: store}).doc)
	if !errors.Is(err, store) {
		t.Errorf("lỗi kho bị đổi thành %v", err)
	}
	if _, err := MeasureWorkingTime([]WorkingSpan{{Start: luc(t, "2026-09-22 08:00"), End: luc(t, "2026-09-21 08:00")}},
		tuanChuan(), (&lichGiaNam{}).doc); err == nil {
		t.Error("end trước start mà không lỗi")
	}
}

// MÚI GIỜ LÀ CỦA XÃ: đổi time.Local không được đổi kết quả.
func TestMeasureIndependentOfProcessTZ(t *testing.T) {
	orig := time.Local
	t.Cleanup(func() { time.Local = orig })
	for _, z := range []*time.Location{time.UTC, time.FixedZone("GIA-TAY", -8*3600)} {
		time.Local = z
		if got := measureOne(t, luc(t, "2026-09-21 08:00"), luc(t, "2026-09-21 15:00"), tuanChuan(), nil); got != 5*time.Hour {
			t.Errorf("TZ=%s: đo được %s, muốn 5h", z, got)
		}
	}
}
