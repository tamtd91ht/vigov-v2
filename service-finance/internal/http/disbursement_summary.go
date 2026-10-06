package http

// The aggregate reads of the disbursement screen (docs/ui-ux/06-giai-ngan.md §3, §4, §5, §8.3).
//
// EVERY FIGURE IS COMPUTED BY THE SERVER, in domain.SummariseProjects / YearCurve / ProjectCurve. The
// screen used to total the project list itself; two places that add the same money up are two places
// that can disagree, and the KPI card is the figure that is reported upward.
//
// UNITS: amounts are JSON numbers of ĐỒNG; ratios are HUNDREDTHS OF A PERCENT (1033 = 10,33%), NOT
// clamped (§13 rule 2), and NULL when the denominator is zero — the conventions of duAnRa.
//
// NO AUDIT ENTRY on either route, for the reason DanhSachDuAn gives: public money inside the commune
// the request arrived in, read under budget.read.

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// curvePointOut is one month of a cumulative chart.
type curvePointOut struct {
	Month             int   `json:"month"`              // 1..12
	PlannedCumulative int64 `json:"planned_cumulative"` // đồng, at the end of the month

	// DisbursedCumulative is NULL for a month that has not begun — the actual line stops at today
	// rather than running flat into the future, which would read as a forecast nobody made.
	DisbursedCumulative *int64 `json:"disbursed_cumulative"`
}

func curveOut(points []domain.CurvePoint) []curvePointOut {
	out := make([]curvePointOut, 0, len(points))
	for _, p := range points {
		o := curvePointOut{Month: p.Month, PlannedCumulative: int64(p.Planned)}
		if p.Observed {
			v := int64(p.Disbursed)
			o.DisbursedCumulative = &v
		}
		out = append(out, o)
	}
	return out
}

// categoryProgressOut is one row of §5's table, and its "Tổng cộng" row.
type categoryProgressOut struct {
	// CategoryID / Label / Order are empty on the total row. Label is "" also on a row whose
	// in_catalogue is false — projects classified under a category that is no longer a live catalogue
	// row; the screen names that row itself (they are shown so the rows add up to the total).
	CategoryID  string `json:"category_id,omitempty"`
	Label       string `json:"label,omitempty"`
	Order       int    `json:"order"`
	InCatalogue bool   `json:"in_catalogue"`

	ProjectCount int `json:"project_count"`

	// Planned is the SUM OF THE CATEGORY'S PROJECTS' year plans. There is no per-category plan column
	// (hang_muc_ke_hoach_von carries no amount by design); derived as the prototype derives it.
	Planned   int64 `json:"planned"`
	Disbursed int64 `json:"disbursed"`

	// Undisbursed = planned − disbursed, NOT clamped: negative is an over-disbursement.
	Undisbursed      int64  `json:"undisbursed"`
	DisbursedRatio   *int64 `json:"disbursed_ratio"`
	UndisbursedRatio *int64 `json:"undisbursed_ratio"`

	// DisbursementDeadline is the EARLIEST project deadline in the row (prototype list_categories:
	// "the day this block starts being late"), YYYY-MM-DD; absent when the row has no project.
	DisbursementDeadline string `json:"disbursement_deadline,omitempty"`
}

func categoryProgressOutOf(c domain.CategoryProgress) categoryProgressOut {
	return categoryProgressOut{
		CategoryID:           c.CategoryID,
		Label:                c.Label,
		Order:                c.Order,
		InCatalogue:          c.InCatalogue,
		ProjectCount:         c.ProjectCount,
		Planned:              int64(c.Planned),
		Disbursed:            int64(c.Disbursed),
		Undisbursed:          int64(c.Undisbursed()),
		DisbursedRatio:       ratioOut(c.DisbursedRatio()),
		UndisbursedRatio:     ratioOut(c.UndisbursedRatio()),
		DisbursementDeadline: ngayRa(c.EarliestDeadline),
	}
}

// projectSummaryOut is GET /api/v1/investment-project-summary.
//
// NO "nguy cơ không giải ngân hết" COUNT, on purpose. §3's fourth card prints it, but in the prototype
// it is a HAND-SET flag on each project (`budget_items.at_risk`, ticked on the project form), not a
// figure derivable from anything this service stores — and inventing a rule for it would decide the
// commune's warning for them. A 0 would be a plausible figure that is simply false. It is added
// (optional) the day the flag, or a decided rule, exists.
type projectSummaryOut struct {
	Year int `json:"year"`

	PlannedTotal   int64  `json:"planned_total"`   // §3 card 1: sum of the year's project plans
	ProjectCount   int    `json:"project_count"`   // §3 card 1, sub-line
	DisbursedTotal int64  `json:"disbursed_total"` // §3 card 2: every live voucher, every state (§11)
	DisbursedRatio *int64 `json:"disbursed_ratio"` // disbursed / planned; null when planned is 0

	// TimeElapsedRatio is §3's thoi_gian_da_qua_pct — the share of the budget year gone (0 for a
	// future year, 10000 for a closed one).
	TimeElapsedRatio int64 `json:"time_elapsed_ratio"`

	// RemainingTotal = planned − disbursed, NOT clamped (§3 card 3).
	RemainingTotal int64 `json:"remaining_total"`

	// DelayThreshold / DelayThresholdSource: the commune's threshold for this year and who chose it —
	// the same pair, in the same unit and under the same names, as GET /api/v1/investment-projects.
	DelayThreshold       int64  `json:"delay_threshold"`
	DelayThresholdSource string `json:"delay_threshold_source"`

	// DelayedProjectCount is §3 card 4: projects domain.LaCham flags — the rule behind each project's
	// `is_delayed` and the list's `delayed_only=true`, so the three cannot disagree.
	DelayedProjectCount int `json:"delayed_project_count"`

	// OpenIssueCount is §3 card 4's "N vướng mắc đang theo dõi": unresolved issues of THIS YEAR's live
	// projects (the prototype counted every year at once; the card is about one year). Optional in the
	// contract because it was added to a published reply; this route always fills it.
	OpenIssueCount *int `json:"open_issue_count,omitempty"`

	// Monthly is §4's curve, January..December: linear plan against cumulative paid by `payment_date`.
	Monthly []curvePointOut `json:"monthly"`

	// DisbursedAfterYear is what this year's projects paid with a `payment_date` AFTER 31/12 — counted
	// in disbursed_total but on no month of the curve, so December's cumulative can be below the card.
	// (A payment dated before 01/01 is folded into January instead.) Usually 0.
	DisbursedAfterYear int64 `json:"disbursed_after_year"`

	// ByCategory is §5's table, in the catalogue's own order; Total is its "Tổng cộng" row.
	ByCategory []categoryProgressOut `json:"by_category"`
	Total      categoryProgressOut   `json:"total"`

	// ScopeNotice is §1's banner, as on the list route.
	ScopeNotice string `json:"scope_notice,omitempty"`
}

// projectCurveOut is GET /api/v1/investment-projects/{id}/disbursement-curve.
type projectCurveOut struct {
	ProjectID string `json:"project_id"`
	Year      int    `json:"year"`

	// Points run from the project's start month to December (domain.ProjectScheduleMonths): twelve
	// points for a project with no dates, fewer for one that breaks ground mid-year.
	Points []curvePointOut `json:"points"`

	// ExpectedEndMonth is the month of the expected completion, where the plan line reaches the full
	// year plan and flattens; absent when the project has no completion date.
	ExpectedEndMonth *int `json:"expected_end_month,omitempty"`

	// DisbursedAfterYear: as on the summary — paid after 31/12, on no point.
	DisbursedAfterYear int64 `json:"disbursed_after_year"`
}

// internalError answers the one 500 every read here uses; the cause goes to the log, never the client.
func (h *Handler) internalError(w http.ResponseWriter, r *http.Request, msg string, err error, args ...any) {
	h.d.Log.Error(msg, append([]any{"xa", string(tenant.MustFrom(r.Context())), "err", err}, args...)...)
	httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
}

// ProjectSummary serves §3, §4 and §5 for one budget year.
// GET /api/v1/investment-project-summary?year=2026
//
// FIVE READS, NONE PER PROJECT: the year's projects, the threshold, the category catalogue, the month
// buckets, the scope notice.
func (h *Handler) ProjectSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Required and never defaulted — DanhSachDuAn's reason (§13 rule 8). Validated before any read.
	params := r.URL.Query()
	year, err := strconv.Atoi(params.Get("year"))
	if err != nil || year < 2000 || year > 2100 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_argument",
			"Thiếu hoặc sai năm ngân sách. Ví dụ: ?year=2026", "")
		return
	}

	projects, err := h.d.DuAn.DanhSach(ctx, fistore.LocDuAn{Nam: year})
	if err != nil {
		// ErrQuaNhieuDuAn included: REFUSED, not truncated — a short list is a total too small.
		h.internalError(w, r, "tổng hợp giải ngân: đọc dự án", err, "nam", year)
		return
	}
	threshold, err := h.d.Nguong.NguongCanhBaoCham(ctx, year)
	if err != nil {
		// Fail closed: never fall back to the software's default (DanhSachDuAn's reason).
		h.internalError(w, r, "tổng hợp giải ngân: ngưỡng cảnh báo chậm", err, "nam", year)
		return
	}
	catalogue, err := h.d.HangMuc.DanhSach(ctx)
	if err != nil {
		h.internalError(w, r, "tổng hợp giải ngân: danh mục hạng mục", err, "nam", year)
		return
	}
	byMonth, err := h.d.DuAn.DisbursedByMonth(ctx, year, "")
	if err != nil {
		h.internalError(w, r, "tổng hợp giải ngân: luỹ kế theo tháng", err, "nam", year)
		return
	}
	// A failure is a 500, never 0: "0 vướng mắc" reads as "none" (fail closed).
	openIssues, err := h.d.ProjectDiscussion.OpenIssueCount(ctx, year)
	if err != nil {
		h.internalError(w, r, "tổng hợp giải ngân: đếm vướng mắc chưa gỡ", err, "nam", year)
		return
	}
	notice, ok := h.scopeNotice(w, r)
	if !ok {
		return
	}

	now := h.nay()
	s := domain.SummariseProjects(projects, catalogue, now, threshold.Gia)
	total := categoryProgressOutOf(s.Total)

	out := projectSummaryOut{
		Year:                 year,
		PlannedTotal:         total.Planned,
		ProjectCount:         total.ProjectCount,
		DisbursedTotal:       total.Disbursed,
		DisbursedRatio:       total.DisbursedRatio,
		TimeElapsedRatio:     int64(domain.PhanTramThoiGianDaQua(now, year)),
		RemainingTotal:       total.Undisbursed,
		DelayThreshold:       int64(threshold.Gia),
		DelayThresholdSource: string(threshold.Nguon),
		DelayedProjectCount:  s.DelayedCount,
		OpenIssueCount:       &openIssues,
		Monthly:              curveOut(domain.YearCurve(s.Total.Planned, byMonth, year, now)),
		DisbursedAfterYear:   int64(byMonth.AfterYear),
		ByCategory:           make([]categoryProgressOut, 0, len(s.ByCategory)),
		Total:                total,
		ScopeNotice:          notice,
	}
	for _, c := range s.ByCategory {
		out.ByCategory = append(out.ByCategory, categoryProgressOutOf(c))
	}
	vietJSON(w, http.StatusOK, out)
}

// ProjectDisbursementCurve serves §8.3 for one project of the commune in the context.
// GET /api/v1/investment-projects/{id}/disbursement-curve
//
// ChiTiet FIRST, so a project of another commune is 404 — the same answer as one that does not exist
// (ChiTietDuAn's reason) — and the month read never runs for it.
func (h *Handler) ProjectDisbursementCurve(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id := r.PathValue("id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_argument", "Thiếu mã dự án.", "")
		return
	}
	project, err := h.d.DuAn.ChiTiet(ctx, id)
	if err != nil {
		if errors.Is(err, fistore.ErrKhongThayDuAn) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy dự án.", "")
			return
		}
		h.internalError(w, r, "luỹ kế dự án: đọc dự án", err)
		return
	}
	byMonth, err := h.d.DuAn.DisbursedByMonth(ctx, project.DuAn.Nam, project.DuAn.ID)
	if err != nil {
		h.internalError(w, r, "luỹ kế dự án: đọc theo tháng", err)
		return
	}

	points := domain.ProjectCurve(project.DuAn, byMonth, h.nay())
	out := projectCurveOut{
		ProjectID:          project.DuAn.ID,
		Year:               project.DuAn.Nam,
		Points:             curveOut(points),
		DisbursedAfterYear: int64(byMonth.AfterYear),
	}
	for _, p := range points {
		if p.ExpectedEnd {
			m := p.Month
			out.ExpectedEndMonth = &m
		}
	}
	vietJSON(w, http.StatusOK, out)
}
