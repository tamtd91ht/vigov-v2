package domain

import (
	"testing"
	"time"
)

// The current period's first day, in Viet Nam (ADR 0086 B1). A Sunday belongs to the week that began
// the Monday BEFORE it (ISO weeks), and an instant late in the UTC evening is already the next
// Vietnamese day — the two ways a period key silently names the wrong period.
func TestReportPeriodStart(t *testing.T) {
	for _, c := range []struct {
		p    ReportPeriod
		at   time.Time
		want string
	}{
		{ReportWeek, time.Date(2026, 10, 4, 3, 0, 0, 0, time.UTC), "2026-09-28"},     // Sunday 10:00 VN
		{ReportWeek, time.Date(2026, 10, 4, 17, 30, 0, 0, time.UTC), "2026-10-05"},   // Monday 00:30 VN
		{ReportWeek, time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC), "2026-09-28"},     // Monday 07:00 VN
		{ReportMonth, time.Date(2026, 9, 30, 18, 0, 0, 0, time.UTC), "2026-10-01"},   // 01/10 01:00 VN
		{ReportMonth, time.Date(2026, 12, 31, 16, 59, 0, 0, time.UTC), "2026-12-01"}, // 31/12 23:59 VN
	} {
		got, ok := ReportPeriodStart(c.p, c.at)
		if !ok || got != c.want {
			t.Errorf("%s @ %s = %q (%v), muốn %q", c.p, c.at, got, ok, c.want)
		}
	}
	if _, ok := ReportPeriodStart("quarter", time.Now()); ok {
		t.Error("kỳ không biết vẫn được nhận")
	}
}

func TestLetterAssignedNoticeCarriesNothingButTheNumber(t *testing.T) {
	n := LetterAssignedNotice(CitizenLetter{Number: 12, Year: 2026, Type: LetterTypeDenunciation,
		Summary: "nội dung tố cáo", SenderName: "Người gửi"}, "log-1", "CB-1")
	if n.Body != "" || n.Title != "Bạn được giao xử lý đơn thư số 12/2026" || n.Key() != "van-ban.chuyen-toi:log-1" ||
		len(n.Recipients) != 1 || n.Recipients[0] != "CB-1" {
		t.Errorf("thông báo đơn thư = %+v", n)
	}
}
