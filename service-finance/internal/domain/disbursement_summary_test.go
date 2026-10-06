package domain

import (
	"testing"
	"time"
)

var ict = time.FixedZone("ICT", 7*3600)

func TestRatioOfTowardZeroUnclampedAndAbsentOnZero(t *testing.T) {
	if _, ok := RatioOf(5, 0); ok {
		t.Fatal("ratio over a zero plan must have no value, not 0%")
	}
	if r, ok := RatioOf(1, 3); !ok || r != 3333 {
		t.Fatalf("1/3 = %d — want 3333 (toward zero)", r)
	}
	if r, _ := RatioOf(35_300_000, 20_000_000); r != 17650 {
		t.Fatalf("over-disbursed = %d — want 17650, never clamped to 10000", r)
	}
	if r, _ := RatioOf(-10, 100); r != -1000 {
		t.Fatalf("negative remainder = %d — want -1000", r)
	}
	// Past the int64 product: the big.Int path must give the same answer, not a wrapped one.
	big := Dong(2_000_000_000_000_000)
	if r, ok := RatioOf(big, big*2); !ok || r != 5000 {
		t.Fatalf("huge figures = %d — want 5000 (overflow would wrap)", r)
	}
}

func TestYearCurveLinearPlanAndActualStopsAtToday(t *testing.T) {
	var paid DisbursedByMonth
	paid.Months[0] = 10 // January (includes anything paid before the year — store folds it)
	paid.Months[2] = 5  // March
	paid.Months[8] = 7  // September
	paid.Months[9] = 99 // October: dated ahead, month not begun at `now`
	paid.AfterYear = 1000

	now := time.Date(2026, time.September, 17, 10, 0, 0, 0, ict)
	c := YearCurve(1200, paid, 2026, now)
	if len(c) != 12 {
		t.Fatalf("points = %d, want 12", len(c))
	}
	if c[0].Planned != 100 || c[11].Planned != 1200 {
		t.Fatalf("plan line = %d..%d, want 100..1200 (December exactly the plan)", c[0].Planned, c[11].Planned)
	}
	if c[1].Disbursed != 10 || c[2].Disbursed != 15 || c[8].Disbursed != 22 {
		t.Fatalf("cumulative = %d,%d,%d — want 10,15,22", c[1].Disbursed, c[2].Disbursed, c[8].Disbursed)
	}
	if !c[8].Observed || c[9].Observed {
		t.Fatal("September (current) must be observed, October (future) must not")
	}
	if c[11].Disbursed != 121 {
		t.Fatalf("December cumulative = %d — AfterYear must stay off every month", c[11].Disbursed)
	}

	if YearCurve(1200, paid, 2025, now)[11].Observed != true {
		t.Fatal("a closed year is wholly observed")
	}
	if YearCurve(1200, paid, 2027, now)[0].Observed {
		t.Fatal("a future year has no observed month")
	}
}

func TestYearCurveIndivisiblePlanEndsExactly(t *testing.T) {
	c := YearCurve(1001, DisbursedByMonth{}, 2026, time.Date(2026, 1, 1, 0, 0, 0, 0, ict))
	if c[11].Planned != 1001 {
		t.Fatalf("December plan = %d, want exactly 1001", c[11].Planned)
	}
}

func TestProjectScheduleMonthsPrototypeRules(t *testing.T) {
	d := func(y, m int) time.Time { return time.Date(y, time.Month(m), 10, 0, 0, 0, 0, ict) }
	cases := []struct {
		name         string
		start, end   time.Time
		wantS, wantE int
	}{
		{"no dates", time.Time{}, time.Time{}, 1, 12},
		{"inside the year", d(2026, 9), d(2026, 11), 9, 11},
		{"carried over, ends next year", d(2025, 6), d(2027, 3), 1, 12},
		{"starts next year", d(2027, 2), time.Time{}, 12, 12},
		{"ended last year", time.Time{}, d(2025, 5), 1, 1},
		{"start only", d(2026, 4), time.Time{}, 4, 12},
	}
	for _, tc := range cases {
		s, e := ProjectScheduleMonths(DuAn{Nam: 2026, NgayKhoiCong: tc.start, NgayHoanThanh: tc.end})
		if s != tc.wantS || e != tc.wantE {
			t.Errorf("%s: %d..%d, want %d..%d", tc.name, s, e, tc.wantS, tc.wantE)
		}
	}
}

func TestProjectCurveFollowsOwnCalendar(t *testing.T) {
	p := DuAn{Nam: 2026, KeHoachVonNam: 300,
		NgayKhoiCong:  time.Date(2026, time.September, 1, 0, 0, 0, 0, ict),
		NgayHoanThanh: time.Date(2026, time.November, 20, 0, 0, 0, 0, ict)}
	var paid DisbursedByMonth
	paid.Months[6] = 40 // July: an advance before the works began
	paid.Months[8] = 60 // September

	now := time.Date(2026, time.October, 5, 0, 0, 0, 0, ict)
	c := ProjectCurve(p, paid, now)
	if len(c) != 4 || c[0].Month != 9 || c[3].Month != 12 {
		t.Fatalf("points = %+v, want September..December", c)
	}
	if c[0].Planned != 100 || c[1].Planned != 200 || c[2].Planned != 300 || c[3].Planned != 300 {
		t.Fatalf("plan = %d,%d,%d,%d — want 100,200,300,300 (flat after completion)",
			c[0].Planned, c[1].Planned, c[2].Planned, c[3].Planned)
	}
	if c[0].Disbursed != 100 {
		t.Fatalf("first point = %d — the July advance must be folded in, want 100", c[0].Disbursed)
	}
	if !c[2].ExpectedEnd || c[3].ExpectedEnd || c[0].ExpectedEnd {
		t.Fatal("only November (completion month) is marked")
	}
	if !c[1].Observed || c[2].Observed {
		t.Fatal("October observed, November not")
	}

	// No dates: twelve points, linear — and no completion month is invented.
	plain := ProjectCurve(DuAn{Nam: 2026, KeHoachVonNam: 1200}, DisbursedByMonth{}, now)
	if len(plain) != 12 || plain[0].Planned != 100 || plain[11].Planned != 1200 {
		t.Fatalf("undated project: %+v", plain)
	}
	for _, pt := range plain {
		if pt.ExpectedEnd {
			t.Fatal("an undated project has no expected-completion month")
		}
	}
}

func TestSummariseProjectsRollsUpByCategory(t *testing.T) {
	now := time.Date(2026, time.September, 17, 0, 6, 0, 0, ict) // 70,96% of 2026
	dl := func(m int) time.Time { return time.Date(2026, time.Month(m), 30, 0, 0, 0, 0, ict) }
	p := func(id, cat string, plan, paid Dong, deadline time.Time) TienDoDuAn {
		return TienDoDuAn{DuAn: DuAn{ID: id, Nam: 2026, HangMucID: cat, KeHoachVonNam: plan,
			ThoiHanGiaiNgan: deadline}, DaGiaiNgan: paid}
	}
	projects := []TienDoDuAn{
		p("a", "hm-1", 100, 90, dl(12)),   // ahead
		p("b", "hm-1", 100, 0, dl(6)),     // 0% at 70,96% -> delayed by exactly the elapsed share
		p("c", "hm-off", 200, 300, dl(9)), // >100%, in an out-of-use category
		p("d", "hm-gone", 50, 0, dl(11)),  // category no longer in the catalogue
		p("e", "hm-1", 0, 0, dl(3)),       // no plan: no score, never delayed
	}
	catalogue := []HangMucKeHoachVon{
		{ID: "hm-1", Nhan: "Chuyển tiếp", ThuTu: 1, DangDung: true},
		{ID: "hm-empty", Nhan: "Xây dựng mới", ThuTu: 2, DangDung: true},
		{ID: "hm-off", Nhan: "Đã tắt", ThuTu: 3},
		{ID: "hm-off-empty", Nhan: "Đã tắt, không dự án", ThuTu: 4},
	}
	s := SummariseProjects(projects, catalogue, now, NguongCanhBaoChamMacDinh)

	if s.Total.ProjectCount != 5 || s.Total.Planned != 450 || s.Total.Disbursed != 390 {
		t.Fatalf("total = %+v", s.Total)
	}
	if s.DelayedCount != 2 {
		t.Fatalf("delayed = %d, want 2 (b at 70,96 points, d likewise; e has no plan)", s.DelayedCount)
	}
	if !s.Total.EarliestDeadline.Equal(dl(3)) {
		t.Fatalf("total deadline = %v, want the earliest", s.Total.EarliestDeadline)
	}

	var ids []string
	for _, r := range s.ByCategory {
		ids = append(ids, r.CategoryID)
	}
	want := []string{"hm-1", "hm-empty", "hm-off", "hm-gone"}
	if len(ids) != len(want) {
		t.Fatalf("rows = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("rows = %v, want %v", ids, want)
		}
	}

	first := s.ByCategory[0]
	if first.ProjectCount != 3 || first.Planned != 200 || first.Disbursed != 90 || !first.EarliestDeadline.Equal(dl(3)) {
		t.Fatalf("hm-1 = %+v", first)
	}
	if r, _ := first.DisbursedRatio(); r != 4500 {
		t.Fatalf("hm-1 ratio = %d, want 4500", r)
	}
	if r, _ := first.UndisbursedRatio(); r != 5500 {
		t.Fatalf("hm-1 undisbursed ratio = %d, want 5500", r)
	}
	if _, ok := s.ByCategory[1].DisbursedRatio(); ok {
		t.Fatal("an empty category has no ratio")
	}
	over := s.ByCategory[2]
	if r, _ := over.DisbursedRatio(); r != 15000 || over.Undisbursed() != -100 {
		t.Fatalf("over-disbursed row: ratio %d, undisbursed %d — want 15000, -100 (unclamped)", r, over.Undisbursed())
	}
	if gone := s.ByCategory[3]; gone.InCatalogue || gone.ProjectCount != 1 {
		t.Fatalf("orphan row = %+v", gone)
	}
}
