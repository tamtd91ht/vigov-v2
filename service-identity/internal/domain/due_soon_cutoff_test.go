package domain

import (
	"errors"
	"testing"
	"time"
)

// WHAT THESE TESTS DEFEND: DueSoonCutoff answers "is at most N working hours left before D" for
// every D by one comparison, `D <= cutoff`. The property test below checks that equivalence against
// an INDEPENDENT oracle — working time counted minute by minute from the fixed week, in a fixed +07
// zone — never against the walk under test, which would agree with itself by construction.
//
// Expectations are wall-clock times in the commune's zone, compared as instants (luc, bang).

// eightHourWeek is Monday–Friday 08:00–12:00 and 13:00–17:00: whole-hour sessions, so a whole
// number of working hours lands EXACTLY on a session boundary — the case where "latest" and
// "earliest" differ.
func eightHourWeek() []CaLamViec {
	var out []CaLamViec
	for weekday := 1; weekday <= 5; weekday++ {
		out = append(out,
			CaLamViec{ID: "am-" + string(rune('0'+weekday)), Thu: weekday, BatDau: gio(8, 0), KetThuc: gio(12, 0)},
			CaLamViec{ID: "pm-" + string(rune('0'+weekday)), Thu: weekday, BatDau: gio(13, 0), KetThuc: gio(17, 0)},
		)
	}
	return out
}

func cutoffOf(t *testing.T, asOf time.Time, week []CaLamViec, cal *lichGiaNam, hours int) time.Time {
	t.Helper()
	if cal == nil {
		cal = &lichGiaNam{}
	}
	out, err := DueSoonCutoff(asOf, week, cal.doc, hours)
	if err != nil {
		t.Fatalf("DueSoonCutoff(%d giờ): %v", hours, err)
	}
	return out
}

// The N-th hour ends INSIDE a session: latest and earliest coincide. 2026-09-21 is a Monday.
func TestDueSoonCutoffInsideSessionEqualsEarliest(t *testing.T) {
	asOf := luc(t, "2026-09-21 08:00")
	got := cutoffOf(t, asOf, tuanChuan(), nil, 5)
	bang(t, got, luc(t, "2026-09-21 15:00"), "5 giờ từ 08:00 thứ Hai")
	bang(t, got, mot(t, asOf, tuanChuan(), nil, 5), "khớp TienGioLamViec khi không rơi vào ranh giới ca")
}

// The N-th hour ends EXACTLY at the lunch boundary: every instant up to 13:30 still has 3 hours left,
// so the cutoff is 13:30, not the 11:30 a deadline would get.
func TestDueSoonCutoffOnBoundaryIsNextSessionStart(t *testing.T) {
	asOf := luc(t, "2026-09-21 08:30")
	bang(t, mot(t, asOf, tuanChuan(), nil, 3), luc(t, "2026-09-21 11:30"), "mốc SỚM NHẤT (hạn)")
	bang(t, cutoffOf(t, asOf, tuanChuan(), nil, 3), luc(t, "2026-09-21 13:30"), "mốc MUỘN NHẤT (sắp đến hạn)")
}

// THE CASE THE "LATEST" RULE EXISTS FOR: counted from a weekend with N a whole number of days, the
// count ends at 17:00 — and a hand-entered deadline that evening still has exactly N hours left.
func TestDueSoonCutoffFromWeekendKeepsEveningDeadline(t *testing.T) {
	asOf := luc(t, "2026-09-26 10:00") // Saturday
	got := cutoffOf(t, asOf, eightHourWeek(), nil, 16)
	bang(t, got, luc(t, "2026-09-30 08:00"), "16 giờ từ thứ Bảy: hết 17:00 thứ Ba, mốc là 08:00 thứ Tư")
	if tuesdayNight := luc(t, "2026-09-29 23:59"); tuesdayNight.After(got) {
		t.Errorf("hạn 23:59 thứ Ba còn đúng 16 giờ làm việc nhưng nằm ngoài ngưỡng %s", got.Format(time.RFC3339))
	}
}

// A public holiday between the boundary and the next session is skipped, exactly as the deadline
// walk skips it — one walk, one calendar.
func TestDueSoonCutoffSkipsHolidayAfterBoundary(t *testing.T) {
	cal := &lichGiaNam{nghi: []NgayNghiLe{{ID: "le-1", Ngay: "2026-09-22", Ten: "Lễ giả định"}}}
	got := cutoffOf(t, luc(t, "2026-09-21 08:00"), eightHourWeek(), cal, 8)
	bang(t, got, luc(t, "2026-09-23 08:00"), "8 giờ từ 08:00 thứ Hai, thứ Ba nghỉ lễ")
}

// A swap-day session is the next working session when it comes first.
func TestDueSoonCutoffLandsOnSwapDay(t *testing.T) {
	cal := &lichGiaNam{lamBu: []CaLamBu{{
		ID: "bu-1", Ngay: "2026-09-26", BatDau: gio(8, 0), KetThuc: gio(12, 0), Ten: "Làm bù giả định",
	}}}
	got := cutoffOf(t, luc(t, "2026-09-25 13:00"), eightHourWeek(), cal, 4)
	bang(t, got, luc(t, "2026-09-26 08:00"), "4 giờ từ 13:00 thứ Sáu, thứ Bảy có làm bù")
}

// workedMinutes is the ORACLE: working minutes in [from, to) of a plain week (no holidays), counted
// minute by minute in a fixed +07. Deliberately naive — it shares no code with the walk.
func workedMinutes(week []CaLamViec, from, to time.Time) int {
	n := 0
	for m := from.In(muiDoiChung); m.Before(to); m = m.Add(time.Minute) {
		weekday := int(m.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		sec := GioTrongNgay(m.Hour()*3600 + m.Minute()*60)
		for _, c := range week {
			if c.Thu == weekday && sec >= c.BatDau && sec < c.KetThuc {
				n++
				break
			}
		}
	}
	return n
}

// THE EQUIVALENCE THE CONTRACT PROMISES: for every deadline D after asOf,
// (working hours in [asOf, D] <= N) ⟺ (D <= cutoff). Checked every 15 minutes over three weeks,
// from starts inside a session, on a boundary, at lunch, at night and on a weekend.
func TestDueSoonCutoffEquivalentToRemainingHours(t *testing.T) {
	weeks := map[string][]CaLamViec{"7.5h": tuanChuan(), "8h": eightHourWeek()}
	starts := []string{
		"2026-09-21 08:30", "2026-09-21 11:30", "2026-09-21 12:15",
		"2026-09-23 21:00", "2026-09-26 10:00", "2026-09-25 16:00",
	}
	for name, week := range weeks {
		for _, s := range starts {
			asOf := luc(t, s)
			for _, hours := range []int{1, 3, 4, 8, 16, 40} {
				cutoff := cutoffOf(t, asOf, week, nil, hours)
				// Accumulated step by step, so the oracle stays linear; it still counts every minute.
				worked, prev := 0, asOf
				for d := asOf.Add(15 * time.Minute); d.Before(asOf.Add(21 * 24 * time.Hour)); d = d.Add(15 * time.Minute) {
					worked += workedMinutes(week, prev, d)
					prev = d
					dueSoon := worked <= hours*60
					if dueSoon != !d.After(cutoff) {
						t.Fatalf("tuần %s, từ %s, %d giờ, hạn %s: còn-≤-N=%v nhưng D<=mốc=%v (mốc %s)",
							name, s, hours, d.In(muiDoiChung).Format("2006-01-02 15:04"), dueSoon,
							!d.After(cutoff), cutoff.In(muiDoiChung).Format("2006-01-02 15:04"))
					}
				}
			}
		}
	}
}

// Refused, never a default week.
func TestDueSoonCutoffEmptyCalendarRefuses(t *testing.T) {
	_, err := DueSoonCutoff(luc(t, "2026-09-21 08:00"), nil, (&lichGiaNam{}).doc, 72)
	if got := loiCauHinh(t, err).Loai; got != LoiLichTrong {
		t.Errorf("loại = %q, muốn %q", got, LoiLichTrong)
	}
}

// Past the horizon is a refusal, not an instant computed from a year nobody entered holidays for.
func TestDueSoonCutoffBeyondHorizonRefuses(t *testing.T) {
	_, err := DueSoonCutoff(luc(t, "2026-09-21 08:00"), tuanChuan(), (&lichGiaNam{}).doc, 2000)
	if got := loiCauHinh(t, err).Loai; got != LoiVuotChanTroi {
		t.Errorf("loại = %q, muốn %q", got, LoiVuotChanTroi)
	}
}

// A non-positive threshold is this service's own fault (the handler refuses the commune's value
// first), so it must NOT come back as a configuration refusal that sends an operator to a screen.
func TestDueSoonCutoffNonPositiveHoursIsPlainError(t *testing.T) {
	for _, h := range []int{0, -4} {
		_, err := DueSoonCutoff(luc(t, "2026-09-21 08:00"), tuanChuan(), (&lichGiaNam{}).doc, h)
		var cfg *LoiKhongTinhDuocHan
		if err == nil || errors.As(err, &cfg) {
			t.Errorf("%d giờ: lỗi = %v, muốn một lỗi thường, không phải lỗi cấu hình", h, err)
		}
	}
	if _, err := DueSoonCutoff(luc(t, "2026-09-21 08:00"), tuanChuan(), nil, 4); err == nil {
		t.Error("thiếu hàm đọc năm mà vẫn trả lời")
	}
}
