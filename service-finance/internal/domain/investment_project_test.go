package domain

import (
	"testing"
	"time"
)

// WHAT THIS FILE PROVES: the arithmetic of §3 and §13, against the specification's OWN worked
// figures. Where a case uses a number from docs/ui-ux/06-giai-ngan.md it says so, because a test
// that invents its own expected value only proves the code agrees with itself.
//
// WHAT IT DOES NOT PROVE: nothing here touches a database, a route or a permission. The commune
// does not appear at all — this package imports the standard library and nothing else (rule 4 for
// services), so isolation is not a property it can hold or break.

// vnTime is the commune's clock. Every date in this file is built in it, because
// ElapsedYearShare compares against year boundaries in the location of the time it is given
// and a UTC clock would move those boundaries by seven hours.
var vnTime = time.FixedZone("ICT", 7*3600)

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 12, 0, 0, 0, vnTime)
}

// atElapsed7096 is the instant at which the 2026 budget year is 70,96% elapsed — the figure §3
// uses in its worked examples ("thời gian đã qua 70,96%", "chậm 70,96 điểm"). It is a datetime and
// not a date because 70,96% of a 365-day year falls partway through 17/09; a date alone cannot
// reproduce the specification's own number, and rounding one to make it fit would be testing the
// test rather than the code.
var atElapsed7096 = time.Date(2026, time.September, 17, 0, 6, 0, 0, vnTime)

func TestDisbursedRatioMatchesSpecExample(t *testing.T) {
	// §3, the KPI card: 3.433.990.000 out of 33.230.000.000 is reported as 10,33%.
	progress := InvestmentProjectProgress{
		InvestmentProject: InvestmentProject{Year: 2026, PlannedAmount: 33_230_000_000},
		DisbursedAmount:   3_433_990_000,
	}
	ratio, ok := progress.DisbursedRatio()
	if !ok {
		t.Fatal("tỷ lệ phải tính được khi có kế hoạch vốn")
	}
	if ratio != 1033 {
		t.Fatalf("tỷ lệ = %d phần vạn, muốn 1033 (10,33%% — §3)", ratio)
	}
}

func TestDisbursedRatioAboveHundredIsNotCapped(t *testing.T) {
	// §13 rule 2: the ratio may exceed 100% and must be shown, not blocked.
	progress := InvestmentProjectProgress{
		InvestmentProject: InvestmentProject{Year: 2026, PlannedAmount: 100_000_000},
		DisbursedAmount:   176_500_000,
	}
	ratio, ok := progress.DisbursedRatio()
	if !ok || ratio != 17650 {
		t.Fatalf("tỷ lệ = %d phần vạn (ok=%v), muốn 17650 (176,5%% — §13 quy tắc 2)", ratio, ok)
	}
	// And the remainder is negative, not clamped: an over-disbursement must be visible.
	if remaining := progress.RemainingAmount(); remaining != -76_500_000 {
		t.Fatalf("còn phải giải ngân = %d, muốn -76500000 — kẹp về 0 là giấu mất khoản vượt", remaining)
	}
}

func TestNoPlanMeansNoRatio(t *testing.T) {
	// 0% and "no ratio" are different statements. A project nobody has allocated money to must not
	// be reported as the worst performer in the commune.
	progress := InvestmentProjectProgress{InvestmentProject: InvestmentProject{Year: 2026, PlannedAmount: 0}, DisbursedAmount: 0}
	if _, ok := progress.DisbursedRatio(); ok {
		t.Fatal("kế hoạch vốn 0 phải trả ok=false, không phải 0%")
	}
	if _, ok := DelayScore(progress, date(2026, time.September, 20)); ok {
		t.Fatal("không có mẫu số thì không có điểm chậm")
	}
	if IsDelayed(progress, date(2026, time.September, 20), DefaultDelayThreshold) {
		t.Fatal("dự án chưa bố trí vốn KHÔNG bị đếm là chậm — xem chú thích trên LaCham")
	}
}

func TestElapsedYearShareAtBothEndsOfYear(t *testing.T) {
	cases := []struct {
		name string
		at   time.Time
		want BasisPoints
	}{
		{"đầu ngày 01/01", time.Date(2026, time.January, 1, 0, 0, 0, 0, vnTime), 0},
		{"trước năm", date(2025, time.December, 31), 0},
		// 2026 is not a leap year: 01/07 00:00 is day 181 of 365.
		{"giữa năm", time.Date(2026, time.July, 1, 0, 0, 0, 0, vnTime), 4958},
		{"đúng 01/01 năm sau", time.Date(2027, time.January, 1, 0, 0, 0, 0, vnTime), 10000},
		// The last day of the year must NOT read as more than 100% elapsed — that is what the
		// literal "31/12 − 01/01" denominator of §3 would produce.
		{"ngày cuối năm", time.Date(2026, time.December, 31, 23, 0, 0, 0, vnTime), 9998},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ElapsedYearShare(c.at, 2026); got != c.want {
				t.Fatalf("= %d phần vạn, muốn %d", got, c.want)
			}
		})
	}
}

func TestDelayScoreEqualsElapsedShareWhenNothingDisbursed(t *testing.T) {
	// §3, stated explicitly: "Dự án giải ngân 0% giữa năm ⇒ chậm 70,96 điểm (bằng đúng % thời gian
	// đã qua)". See atElapsed7096 for why that share of 2026 falls partway through 17/09.
	at := atElapsed7096
	if elapsed := ElapsedYearShare(at, 2026); elapsed != 7096 {
		t.Fatalf("thời gian đã qua = %d phần vạn, muốn 7096 — mốc lấy từ §3", elapsed)
	}

	progress := InvestmentProjectProgress{InvestmentProject: InvestmentProject{Year: 2026, PlannedAmount: 2_130_000_000}, DisbursedAmount: 0}
	score, ok := DelayScore(progress, at)
	if !ok || score != 7096 {
		t.Fatalf("điểm chậm = %d (ok=%v), muốn 7096", score, ok)
	}
	if !IsDelayed(progress, at, DefaultDelayThreshold) {
		t.Fatal("chậm 70,96 điểm vượt ngưỡng 10 điểm — phải bị đánh dấu chậm")
	}
}

func TestInvestmentProjectAheadOfScheduleHasNegativeDelayScore(t *testing.T) {
	at := atElapsed7096 // 70,96% đã qua
	progress := InvestmentProjectProgress{
		InvestmentProject: InvestmentProject{Year: 2026, PlannedAmount: 100_000_000},
		DisbursedAmount:   90_000_000, // §8, dự án mẫu: 90%
	}
	score, ok := DelayScore(progress, at)
	if !ok || score != 7096-9000 {
		t.Fatalf("điểm chậm = %d (ok=%v), muốn %d", score, ok, 7096-9000)
	}
	if IsDelayed(progress, at, DefaultDelayThreshold) {
		t.Fatal("dự án đi trước lịch không phải dự án chậm")
	}
}

func TestThresholdComparisonIsStrictlyGreater(t *testing.T) {
	// §3: "dự án CHẬM khi diem_cham > nguong". Exactly on the threshold is NOT behind — the
	// difference is invisible until a commune sets the threshold to 0.
	at := atElapsed7096 // 7096
	progress := InvestmentProjectProgress{
		InvestmentProject: InvestmentProject{Year: 2026, PlannedAmount: 10_000_000},
		DisbursedAmount:   6_096_000, // 60,96% -> điểm chậm đúng 1000 = 10 điểm
	}
	score, _ := DelayScore(progress, at)
	if score != DefaultDelayThreshold {
		t.Fatalf("dựng sai ca: điểm chậm = %d, cần đúng bằng ngưỡng %d", score, DefaultDelayThreshold)
	}
	if IsDelayed(progress, at, DefaultDelayThreshold) {
		t.Fatal("đúng BẰNG ngưỡng thì chưa chậm — so sánh phải là > chứ không phải >=")
	}
}

func TestBlankApprovedAmountFallsBackToPlannedAmount(t *testing.T) {
	// §9: "Để trống thì lấy bằng số tiền bố trí năm nay."
	if got := (InvestmentProject{PlannedAmount: 7_500_000_000}).EffectiveApprovedAmount(); got != 7_500_000_000 {
		t.Fatalf("= %d, muốn 7500000000", got)
	}
	if got := (InvestmentProject{PlannedAmount: 100, ApprovedAmount: 250}).EffectiveApprovedAmount(); got != 250 {
		t.Fatalf("= %d, muốn 250 — số đã khai phải thắng mặc định", got)
	}
}

func TestRunningTotalAccumulates(t *testing.T) {
	got := RunningTotal([]Dong{100, 0, 250, 50})
	want := []Dong{100, 100, 350, 400}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("luỹ kế[%d] = %d, muốn %d (cả dãy: %v)", i, got[i], want[i], got)
		}
	}
	if len(RunningTotal(nil)) != 0 {
		t.Fatal("luỹ kế của dãy rỗng phải rỗng")
	}
}

func TestLinearPlanEndsEXACTLYAtPlan(t *testing.T) {
	// 33.230.000.000 across 12 months does not divide evenly. The last point must still be the
	// plan exactly: a planning line stopping a few đồng short raises the question of which figure
	// is wrong, every time somebody looks at the chart.
	line := LinearPlan(33_230_000_000, 12)
	if len(line) != 12 {
		t.Fatalf("số mốc = %d, muốn 12", len(line))
	}
	if line[11] != 33_230_000_000 {
		t.Fatalf("mốc cuối = %d, muốn 33230000000 ĐÚNG BẰNG kế hoạch", line[11])
	}
	for i := 1; i < len(line); i++ {
		if line[i] < line[i-1] {
			t.Fatalf("đường kế hoạch giảm tại mốc %d: %v", i, line)
		}
	}
	if LinearPlan(100, 0) != nil {
		t.Fatal("0 mốc phải trả nil, không phải một dãy rỗng giả vờ là biểu đồ")
	}
}
