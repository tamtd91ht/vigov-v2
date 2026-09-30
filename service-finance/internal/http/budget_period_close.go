package http

// The three routes of budget period close (chốt kỳ), migration 0012. Declared in routes.go with
// their permissions; mapped to errors by traLoiLoiNganSach, the one mapping for the whole budget board.
//
//	GET  /api/v1/budget-period-closes?year=          the close history of one year
//	POST /api/v1/budget-period-closes                close a month, or the whole year
//	POST /api/v1/budget-period-closes/{code}/reopening   reopen one close, with a reason
//
// Nothing here is personal data: a close carries a period, a staff business code and a reason.

import (
	"net/http"
	"strconv"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/service-finance/internal/app"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// budgetPeriodCloseOut is one close act.
type budgetPeriodCloseOut struct {
	ID string `json:"id"`
	// Code is the business code — `CK-2026-09-01` (month 09, revision 1), `CK-2026-CN-01` (whole year).
	Code string `json:"code"`
	Year int    `json:"year"`
	// Month is 1..12, or null for a WHOLE-YEAR close. `scope` says the same on sight.
	Month *int `json:"month"`
	// Scope is `month` | `year`.
	Scope    string `json:"scope"`
	Revision int    `json:"revision"`
	// Active is true until the close is reopened. A reopened close stays in the list — it is history.
	Active bool `json:"active"`

	ClosedAt string `json:"closed_at,omitempty"` // RFC 3339; absent on the 201 of a create
	ClosedBy string `json:"closed_by"`           // staff business code

	ReopenedAt   string `json:"reopened_at,omitempty"` // RFC 3339
	ReopenedBy   string `json:"reopened_by,omitempty"` // staff business code
	ReopenReason string `json:"reopen_reason,omitempty"`
}

// budgetPeriodClosesOut is the close history of one year: year close first, then month by month,
// each by revision.
type budgetPeriodClosesOut struct {
	Year   int                    `json:"year"`
	Closes []budgetPeriodCloseOut `json:"closes"`
}

// budgetPeriodCloseIn is the body of POST /api/v1/budget-period-closes.
//
// `month` ABSENT (or null) = THE WHOLE YEAR; present, it must be 1..12. `0` is refused rather than
// read as "the year": a client that sent 0 meant something, and guessing which is how a whole year
// gets closed by accident.
type budgetPeriodCloseIn struct {
	Year  int  `json:"year"`
	Month *int `json:"month,omitempty"`
}

// budgetPeriodReopeningIn is the body of POST /api/v1/budget-period-closes/{code}/reopening.
type budgetPeriodReopeningIn struct {
	Reason string `json:"reason"` // required, trimmed, ≤500 characters
}

func budgetPeriodCloseOutOf(c domain.BudgetPeriodClose) budgetPeriodCloseOut {
	ra := budgetPeriodCloseOut{
		ID: c.ID, Code: c.Code, Year: c.Year, Scope: "year", Revision: c.Revision,
		Active: c.Active(), ClosedAt: lucRa(c.ClosedAt), ClosedBy: c.ClosedBy,
		ReopenedAt: lucRa(c.ReopenedAt), ReopenedBy: c.ReopenedBy, ReopenReason: c.ReopenReason,
	}
	if c.Month != 0 {
		m := c.Month
		ra.Month, ra.Scope = &m, "month"
	}
	return ra
}

// ListBudgetPeriodCloses — GET /api/v1/budget-period-closes?year=2026
func (h *Handler) ListBudgetPeriodCloses(w http.ResponseWriter, r *http.Request) {
	yearRaw := r.URL.Query().Get("year")
	year, err := strconv.Atoi(yearRaw)
	if yearRaw == "" || err != nil || domain.KiemTraNamNganSach(year) != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Cần `year` trong khoảng 2000..2100.", "year")
		return
	}
	closes, err := h.d.NganSach.BudgetPeriodClosesOfYear(r.Context(), year)
	if err != nil {
		h.traLoiLoiNganSach(w, r, "đọc chốt kỳ", err)
		return
	}
	ra := budgetPeriodClosesOut{Year: year, Closes: make([]budgetPeriodCloseOut, 0, len(closes))}
	for _, c := range closes {
		ra.Closes = append(ra.Closes, budgetPeriodCloseOutOf(c))
	}
	vietJSON(w, http.StatusOK, ra)
}

// CreateBudgetPeriodClose — POST /api/v1/budget-period-closes
func (h *Handler) CreateBudgetPeriodClose(w http.ResponseWriter, r *http.Request) {
	var in budgetPeriodCloseIn
	if !docThan(w, r, &in) {
		return
	}
	month := 0
	if in.Month != nil {
		if *in.Month < 1 || *in.Month > 12 {
			h.traLoiLoiNganSach(w, r, "chốt kỳ", domain.ErrCloseMonthInvalid)
			return
		}
		month = *in.Month
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	c, err := h.d.GhiNganSach.CloseBudgetPeriod(r.Context(),
		app.BudgetPeriodCloseRequest{Year: in.Year, Month: month}, nguoi)
	if err != nil {
		h.traLoiLoiNganSach(w, r, "chốt kỳ", err)
		return
	}
	idem.RecordCode(r.Context(), c.Code)
	vietJSON(w, http.StatusCreated, budgetPeriodCloseOutOf(c))
}

// CreateBudgetPeriodReopening — POST /api/v1/budget-period-closes/{code}/reopening
//
// 200 WITH THE CLOSE, now carrying its reopen trio — the shape `headline` answers with: the act
// changes the state of an existing record, and that record is what the caller needs back.
func (h *Handler) CreateBudgetPeriodReopening(w http.ResponseWriter, r *http.Request) {
	var in budgetPeriodReopeningIn
	if !docThan(w, r, &in) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	c, err := h.d.GhiNganSach.ReopenBudgetPeriodClose(r.Context(), r.PathValue("code"), in.Reason, nguoi)
	if err != nil {
		h.traLoiLoiNganSach(w, r, "mở chốt kỳ", err)
		return
	}
	idem.RecordCode(r.Context(), c.Code)
	vietJSON(w, http.StatusOK, budgetPeriodCloseOutOf(c))
}
