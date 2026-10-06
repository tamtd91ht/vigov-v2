package domain

// The figures of the disbursement screen's upper half (docs/ui-ux/06-giai-ngan.md §3, §4, §5) and of
// one project's chart tab (§8.3) — all DERIVED, on every read, from the projects and their vouchers.
//
// WHY THEY ARE COMPUTED HERE AND NOT IN THE BROWSER: the screen used to total the project list itself.
// A second place that adds the same money up is a second place that can disagree with the first — a
// filter applied before the sum, a ratio rounded the other way — and the KPI card is the figure that
// is reported upward. One function, one answer, and the web only formats it.
//
// STANDARD LIBRARY ONLY (rule 4).

import (
	"math/big"
	"sort"
	"time"
)

// RatioOf is part / whole in parts per ten thousand, ROUNDED TOWARD ZERO — the same rounding
// TienDoDuAn.TyLeGiaiNgan uses, so a category row and the projects inside it are rounded alike.
//
// ok IS false WHEN THE WHOLE IS ZERO OR LESS: "no ratio" and "0%" are different statements (the reason
// TyLeGiaiNgan gives). NOT CLAMPED: §13 rule 2 shows a ratio above 100%, and a negative part (an
// over-disbursed remainder) gives a negative ratio.
//
// THE PRODUCT IS TAKEN IN big.Int ONLY WHEN int64 WOULD OVERFLOW (|part| > 9,2 × 10^14 đồng). A
// commune's year is ~3 × 10^10, so the path is never taken in practice; it exists because the
// alternative is a silent wrap to a plausible small number — the failure PhanTramThoiGianDaQua's
// first version had.
func RatioOf(part, whole Dong) (PhanVan, bool) {
	if whole <= 0 {
		return 0, false
	}
	const maxSafe = int64(^uint64(0)>>1) / 10000
	p := int64(part)
	if p <= maxSafe && p >= -maxSafe {
		return PhanVan(p * 10000 / int64(whole)), true
	}
	q := new(big.Int).Mul(big.NewInt(p), big.NewInt(10000))
	q.Quo(q, big.NewInt(int64(whole))) // Quo truncates toward zero, like Go's `/`
	return PhanVan(q.Int64()), true
}

// DisbursedByMonth is what a set of vouchers paid, bucketed by the calendar month of `payment_date`
// (`ngay_chi`) within ONE budget year.
//
// THE TWO EDGES ARE DECIDED HERE, ONCE, because a voucher of a 2026 project may carry a 2025 or 2027
// date (KiemTraNgayChi bounds only the year range, on purpose):
//
//	paid BEFORE 01/01   folded into January — that money was already out by then, so the cumulative
//	                    line at every month of the year is still the truth.
//	paid AFTER 31/12    AfterYear, on no month. The curve is this calendar year's; putting the money on
//	                    December would show it disbursed before it was. It is carried separately so the
//	                    screen can say why December's cumulative is below "đã giải ngân" (which counts
//	                    every live voucher of the year's projects, whatever its date).
//
// The prototype (vigov-require repository.monthly_disbursed) drops BOTH edges, which leaves the curve
// short of the KPI card with nothing explaining the gap.
type DisbursedByMonth struct {
	Months    [12]Dong // index 0 = January
	AfterYear Dong
}

// CurvePoint is one month of a cumulative chart.
type CurvePoint struct {
	Month     int  // 1..12
	Planned   Dong // cumulative plan at the END of this month
	Disbursed Dong // cumulative disbursed at the end of this month; meaningful only when Observed

	// Observed is false for a month that has not begun at `now`. The ACTUAL line stops there: drawing
	// it flat into the future reads as "nothing more will be paid", which is a forecast nobody made.
	// The current month IS observed — it is partial, and what is paid so far is real.
	Observed bool

	// ExpectedEnd marks the month of the project's expected completion (§8.3, prototype 03c1787) —
	// set only when the project HAS a completion date. The year curve never sets it.
	ExpectedEnd bool
}

// monthObserved: has `month` of `year` begun at `now`? A closed year is wholly observed, a future year
// not at all. `now` is compared in its own location — see PhanTramThoiGianDaQua on whose clock that is.
func monthObserved(year, month int, now time.Time) bool {
	switch {
	case now.Year() > year:
		return true
	case now.Year() < year:
		return false
	default:
		return month <= int(now.Month())
	}
}

// YearCurve is §4's chart for one commune's budget year: the plan spread LINEARLY over twelve months
// (KeHoachTuyenTinh — December is exactly the plan), against the cumulative of what was paid.
//
// LINEAR BECAUSE COMMUNES PUBLISH NO MONTHLY PLAN (§4 "đi tuyến tính", prototype overview): the same
// yardstick the delay score uses, so the chart and the "N dự án chậm" card cannot tell two stories.
func YearCurve(plan Dong, paid DisbursedByMonth, year int, now time.Time) []CurvePoint {
	planned := KeHoachTuyenTinh(plan, 12)
	out := make([]CurvePoint, 12)
	var running Dong
	for i := range out {
		running += paid.Months[i]
		out[i] = CurvePoint{
			Month:     i + 1,
			Planned:   planned[i],
			Disbursed: running,
			Observed:  monthObserved(year, i+1, now),
		}
	}
	return out
}

// ProjectScheduleMonths is the window, in months of the project's budget year, its plan line rises
// over — the prototype's _schedule_months (vigov-require 03c1787), ported rule for rule:
//
//	start   the month of ngay_khoi_cong when it falls in the budget year; January when it is earlier
//	        or unset (a carried-over project's tracked money is THIS year's share); December when it
//	        is in a later year.
//	end     the month of ngay_hoan_thanh when it falls in the budget year; December when it is later
//	        or unset; `start` when it is in an earlier year.
//	        end is never before start.
func ProjectScheduleMonths(d DuAn) (start, end int) {
	year := d.Nam
	start = 1
	if !d.NgayKhoiCong.IsZero() {
		switch y := d.NgayKhoiCong.Year(); {
		case y == year:
			start = int(d.NgayKhoiCong.Month())
		case y > year:
			start = 12
		}
	}
	end = 12
	if !d.NgayHoanThanh.IsZero() {
		switch y := d.NgayHoanThanh.Year(); {
		case y == year:
			end = int(d.NgayHoanThanh.Month())
		case y < year:
			end = start
		}
	}
	if end < start {
		end = start
	}
	return start, end
}

// ProjectCurve is §8.3's chart for ONE project, following the project's own calendar (prototype
// 03c1787): points from its start month to December; the plan rises evenly to the full year plan at
// the expected-completion month and stays flat after it — money still goes out after works finish,
// and cutting the axis at completion would hide that tail.
//
// A project with no dates runs January..December, i.e. the same linear line as YearCurve.
//
// ONE DEPARTURE FROM THE PROTOTYPE, stated: the prototype starts its running total at zero in the
// start month, so a payment dated before the works began (an advance) vanished from the curve. Here
// such payments are folded into the first point, so the actual line never reads below what was paid.
//
// The plan at month m is plan × done / span, computed from the plan directly (KeHoachTuyenTinh's
// reason): the completion month is EXACTLY the plan, never a few đồng short.
func ProjectCurve(d DuAn, paid DisbursedByMonth, now time.Time) []CurvePoint {
	start, end := ProjectScheduleMonths(d)
	span := int64(end - start + 1)

	var running Dong
	for i := 0; i < start-1; i++ {
		running += paid.Months[i]
	}
	out := make([]CurvePoint, 0, 13-start)
	for m := start; m <= 12; m++ {
		running += paid.Months[m-1]
		done := int64(m - start + 1)
		if done > span {
			done = span
		}
		out = append(out, CurvePoint{
			Month:       m,
			Planned:     Dong(int64(d.KeHoachVonNam) * done / span),
			Disbursed:   running,
			Observed:    monthObserved(d.Nam, m, now),
			ExpectedEnd: !d.NgayHoanThanh.IsZero() && m == end,
		})
	}
	return out
}

// CategoryProgress is one row of §5's "Tiến độ theo hạng mục" table, or its "Tổng cộng" row.
//
// THERE IS NO PER-CATEGORY PLAN OR DEADLINE COLUMN in this schema (§5 and §2's `☰ Hạng mục` describe
// one; `hang_muc_ke_hoach_von` deliberately carries no amount — see HangMucKeHoachVon). Both are
// DERIVED from the category's projects, as the prototype's list_categories does: the plan is the sum
// of the projects' year plans, the deadline the EARLIEST project deadline in the block ("the day this
// block starts being late").
type CategoryProgress struct {
	CategoryID string
	Label      string
	Order      int

	// InCatalogue is false for a row grouping projects whose category is no longer a live catalogue
	// row. Such projects are SHOWN, not dropped: dropping them makes the rows sum to less than the
	// KPI card with nothing saying where the difference went (prototype list_categories' reasoning).
	InCatalogue bool

	ProjectCount int
	Planned      Dong
	Disbursed    Dong

	// EarliestDeadline is the earliest thoi_han_giai_ngan among the rows' projects; zero when none.
	EarliestDeadline time.Time
}

// Undisbursed is planned − disbursed, NOT CLAMPED — negative is an over-disbursement, the figure
// somebody needs to see (ConPhaiGiaiNgan's reason; the prototype clamps it, this API never has).
func (c CategoryProgress) Undisbursed() Dong { return c.Planned - c.Disbursed }

// DisbursedRatio and UndisbursedRatio are over Planned; no value when Planned is zero.
func (c CategoryProgress) DisbursedRatio() (PhanVan, bool) { return RatioOf(c.Disbursed, c.Planned) }
func (c CategoryProgress) UndisbursedRatio() (PhanVan, bool) {
	return RatioOf(c.Undisbursed(), c.Planned)
}

func (c *CategoryProgress) add(p TienDoDuAn) {
	c.ProjectCount++
	c.Planned += p.DuAn.KeHoachVonNam
	c.Disbursed += p.DaGiaiNgan
	if dl := p.DuAn.ThoiHanGiaiNgan; !dl.IsZero() && (c.EarliestDeadline.IsZero() || dl.Before(c.EarliestDeadline)) {
		c.EarliestDeadline = dl
	}
}

// ProjectsSummary is §3's four cards and §5's table for one budget year.
type ProjectsSummary struct {
	Total        CategoryProgress // CategoryID/Label empty — the "Tổng cộng" row and the KPI totals
	DelayedCount int              // projects LaCham flags against the commune's threshold
	ByCategory   []CategoryProgress
}

// SummariseProjects rolls one budget year's projects up, in ONE pass, with the SAME delay rule as the
// project chip (LaCham) — the "N dự án chậm" card and the `delayed_only` filter can never disagree.
//
// ROWS, IN THIS ORDER: every catalogue row IN USE, in the catalogue's own order, even with no project
// (a block of the capital plan with nothing in it is still a line of the report — prototype); a row
// OUT OF USE only when it still has projects this year; then one row per category id that matches no
// live catalogue row, InCatalogue false, by id.
func SummariseProjects(projects []TienDoDuAn, catalogue []HangMucKeHoachVon,
	now time.Time, threshold PhanVan) ProjectsSummary {

	var s ProjectsSummary
	byID := make(map[string]*CategoryProgress, len(catalogue))
	rows := make([]*CategoryProgress, 0, len(catalogue))
	for _, c := range catalogue {
		row := &CategoryProgress{CategoryID: c.ID, Label: c.Nhan, Order: c.ThuTu, InCatalogue: true}
		byID[c.ID] = row
		rows = append(rows, row)
	}
	var orphans []*CategoryProgress
	for _, p := range projects {
		s.Total.add(p)
		if LaCham(p, now, threshold) {
			s.DelayedCount++
		}
		row, ok := byID[p.DuAn.HangMucID]
		if !ok {
			row = &CategoryProgress{CategoryID: p.DuAn.HangMucID}
			byID[p.DuAn.HangMucID] = row
			orphans = append(orphans, row)
		}
		row.add(p)
	}
	sort.Slice(orphans, func(i, j int) bool { return orphans[i].CategoryID < orphans[j].CategoryID })

	inUse := make(map[string]bool, len(catalogue))
	for _, c := range catalogue {
		inUse[c.ID] = c.DangDung
	}
	s.ByCategory = make([]CategoryProgress, 0, len(rows)+len(orphans))
	for _, r := range rows {
		if inUse[r.CategoryID] || r.ProjectCount > 0 {
			s.ByCategory = append(s.ByCategory, *r)
		}
	}
	for _, r := range orphans {
		s.ByCategory = append(s.ByCategory, *r)
	}
	return s
}
