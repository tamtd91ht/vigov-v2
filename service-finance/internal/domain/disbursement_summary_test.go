package domain

import (
	"testing"
	"time"
)

var ict = time.FixedZone("ICT", 7*3600)

func TestRatioOfRoundsHalfAwayFromZeroUnclampedAndAbsentOnZero(t *testing.T) {
	if _, ok := RatioOf(5, 0); ok {
		t.Fatal("ratio over a zero plan must have no value, not 0%")
	}
	if r, ok := RatioOf(1, 3); !ok || r != 3333 {
		t.Fatalf("1/3 = %d — want 3333 (3333,3 rounds down)", r)
	}
	if r, ok := RatioOf(2, 3); !ok || r != 6667 {
		t.Fatalf("2/3 = %d — want 6667 (6666,7 rounds up; truncation gave 6666)", r)
	}
	if r, _ := RatioOf(-2, 3); r != -6667 {
		t.Fatalf("-2/3 = %d — want -6667 (half AWAY from zero, symmetric)", r)
	}
	// Exactly half a unit: 1 / 20000 = 0,5 phần vạn → 1 (away from zero), -1 for the negative.
	if r, _ := RatioOf(1, 20000); r != 1 {
		t.Fatalf("1/20000 = %d — want 1 (a half rounds away from zero)", r)
	}
	if r, _ := RatioOf(-1, 20000); r != -1 {
		t.Fatalf("-1/20000 = %d — want -1", r)
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
	// The big.Int path rounds the same way: 2 × 10^18 / (3 × 10^18) = 6666,67 → 6667.
	if r, _ := RatioOf(big, big/2*3); r != 6667 {
		t.Fatalf("huge 2/3 = %d — want 6667 (big path must round, not truncate)", r)
	}
	if r, _ := RatioOf(-big, big/2*3); r != -6667 {
		t.Fatalf("huge -2/3 = %d — want -6667", r)
	}
}

// The brief's own row (giai-ngan-update-theo-prototype.md §2.1, "Các công trình chuyển tiếp"):
// truncated, the two ratio columns read 32,58% + 67,41% = 99,99%. Rounded, 32,59% + 67,41% = 100%.
func TestCategoryRatiosOfBriefRowAddUpToHundredPercent(t *testing.T) {
	row := CategoryProgress{Planned: 800_000_000, Disbursed: 260_690_000}
	d, ok := row.DisbursedRatio()
	if !ok || d != 3259 {
		t.Fatalf("disbursed ratio = %d, want 3259 (32,59%%)", d)
	}
	u, ok := row.UndisbursedRatio()
	if !ok || u != 6741 {
		t.Fatalf("undisbursed ratio (539.310.000 / 800.000.000) = %d, want 6741 (67,41%%)", u)
	}
	if d+u != 10000 {
		t.Fatalf("columns sum to %d, want 10000", d+u)
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

	// No dates: twelve points, linear — and the marker sits on December (ADR 0080 #3, prototype).
	plain := ProjectCurve(DuAn{Nam: 2026, KeHoachVonNam: 1200}, DisbursedByMonth{}, now)
	if len(plain) != 12 || plain[0].Planned != 100 || plain[11].Planned != 1200 {
		t.Fatalf("undated project: %+v", plain)
	}
	for i, pt := range plain {
		if pt.ExpectedEnd != (i == 11) {
			t.Fatalf("undated project: marker on month %d = %v — want December only", pt.Month, pt.ExpectedEnd)
		}
	}
}

// ADR 0080 #3: every project curve carries exactly one "Hoàn thành dự kiến" point, placed as the
// prototype places it — and the plan line does not move because of it.
func TestProjectExpectedEndMarkerPrototypeRules(t *testing.T) {
	d := func(y, m int) time.Time { return time.Date(y, time.Month(m), 10, 0, 0, 0, 0, ict) }
	deadline := d(2026, 6) // a disbursement deadline that must NOT be consulted
	cases := []struct {
		name       string
		start, end time.Time
		want       int
		firstMonth int
		planAtWant Dong // the plan at the marker month: the full plan, as before this change
	}{
		{"completion in the year", d(2026, 3), d(2026, 9), 9, 3, 1200},
		{"completion in a later year", d(2026, 3), d(2027, 2), 12, 3, 1200},
		{"completion in an earlier year", d(2026, 4), d(2025, 11), 4, 4, 1200},
		{"completion in an earlier year, no start", time.Time{}, d(2025, 11), 1, 1, 1200},
		{"no completion date", d(2026, 5), time.Time{}, 12, 5, 1200},
		{"no dates at all", time.Time{}, time.Time{}, 12, 1, 1200},
	}
	now := time.Date(2026, time.October, 5, 0, 0, 0, 0, ict)
	for _, tc := range cases {
		p := DuAn{Nam: 2026, KeHoachVonNam: 1200, NgayKhoiCong: tc.start, NgayHoanThanh: tc.end,
			ThoiHanGiaiNgan: deadline}
		if got := ProjectExpectedEndMonth(p); got != tc.want {
			t.Errorf("%s: marker month %d, want %d", tc.name, got, tc.want)
			continue
		}
		c := ProjectCurve(p, DisbursedByMonth{}, now)
		if c[0].Month != tc.firstMonth {
			t.Errorf("%s: curve starts at %d, want %d", tc.name, c[0].Month, tc.firstMonth)
		}
		marked := 0
		for _, pt := range c {
			if pt.ExpectedEnd {
				marked++
				if pt.Month != tc.want || pt.Planned != tc.planAtWant {
					t.Errorf("%s: marker on %d with plan %d, want %d with %d", tc.name, pt.Month, pt.Planned,
						tc.want, tc.planAtWant)
				}
			}
		}
		if marked != 1 {
			t.Errorf("%s: %d marked points, want exactly 1", tc.name, marked)
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
	if s.AtRiskCount != 0 {
		t.Fatalf("at risk = %d, want 0 — no project is flagged, and nothing is inferred", s.AtRiskCount)
	}
	// The flag is counted as set, independent of delay: one delayed project and one ahead, both flagged.
	flagged := append([]TienDoDuAn(nil), projects...)
	flagged[0].DuAn.AtRisk = true // "a": ahead of schedule — still counted, the flag is a judgement
	flagged[1].DuAn.AtRisk = true // "b": delayed
	if got := SummariseProjects(flagged, catalogue, now, NguongCanhBaoChamMacDinh).AtRiskCount; got != 2 {
		t.Fatalf("at risk = %d, want 2", got)
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
