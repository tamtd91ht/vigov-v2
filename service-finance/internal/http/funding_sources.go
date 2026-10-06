package http

// The routes of §6 "Tiến độ theo nguồn vốn" and its "Quản lý nguồn vốn" dialog (docs/ui-ux/
// 06-giai-ngan.md §6, §11, §13 rule 6; migration 0013; user decisions 06/10/2026). Declared in
// routes.go with their permissions:
//
//	GET  /api/v1/funding-sources?year=                           the year's cards + §13 rule 6 warning
//	POST /api/v1/funding-sources                                 add a source to the commune's catalogue
//	PUT  /api/v1/funding-sources/{id}/annual-amounts/{year}      record / correct the year's granted amount
//	GET  /api/v1/funding-sources/{id}/projects?year=             the projects behind one card
//
// NOTHING HERE IS PERSONAL DATA (rule 3): source names, project codes and names, amounts of public money.
//
// EVERY AMOUNT IS A JSON NUMBER OF ĐỒNG and every ratio is in HUNDREDTHS OF A PERCENT (1033 = 10,33%),
// NULL when its denominator is zero — the conventions duAnRa states and the screen already reads.

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/app"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// FundingSourceReading is the read half, *fistore.NguonVonStore in production. An interface at the
// point of use for the reason DuAnTienDo gives: the permission declaration, the commune check ahead of
// any read and the refusal instead of a truncated list must be testable without a PostgreSQL.
type FundingSourceReading interface {
	// vi-name-ok: mirrors the existing store method NguonVonStore.TienDoTheoNguon (rule 12 inv 3, not renamed)
	TienDoTheoNguon(ctx context.Context, nam int) ([]domain.TienDoNguonVon, error)
	UnattributedDisbursed(ctx context.Context, year int) (domain.Dong, error)
	SourceProjects(ctx context.Context, sourceID string, year int) (domain.FundingSourceProjects, error)
}

// FundingSourceWriting is the write half, *app.FundingSources in production — separate from the read
// half for the reason GhiDuAn gives: each method opens a transaction and writes an audit entry inside
// it, and one interface would let a caller reach for the nearest method.
type FundingSourceWriting interface {
	AddFundingSource(ctx context.Context, req app.FundingSourceCreateRequest,
		actor audit.Actor) (domain.NguonVon, error)
	SetGrantedAmount(ctx context.Context, sourceID string, year int, amount domain.Dong,
		actor audit.Actor) (domain.FundingSourceAnnualAmount, error)
}

// fundingSourceOut is one source's card for one budget year.
type fundingSourceOut struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Order int    `json:"order"`
	Year  int    `json:"year"` // the year every figure below was read for

	// GrantedAmount is "vốn được giao" for Year — 0 when the commune has entered none (0013).
	GrantedAmount int64 `json:"granted_amount"`
	// AllocatedAmount — SUM of the allocation lines of Year's live projects naming this source.
	AllocatedAmount int64 `json:"allocated_amount"`
	// ProjectCount — DISTINCT projects of Year with a line on this source.
	ProjectCount int `json:"project_count"`
	// DisbursedAmount — Year's live vouchers drawn from this source, every state (§11).
	DisbursedAmount int64 `json:"disbursed_amount"`

	// UnallocatedAmount and OverallocatedAmount are never negative; at most one is non-zero. The
	// overrun is reported only when something was granted for Year (domain.OverallocatedAmount).
	UnallocatedAmount   int64 `json:"unallocated_amount"`
	OverallocatedAmount int64 `json:"overallocated_amount"`

	// §6's three bars, NOT CLAMPED (§13 rule 2), NULL when the denominator is 0.
	AllocatedRatio            *int64 `json:"allocated_ratio"`              // allocated / granted
	DisbursedOfAllocatedRatio *int64 `json:"disbursed_of_allocated_ratio"` // disbursed / allocated
	DisbursedOfGrantedRatio   *int64 `json:"disbursed_of_granted_ratio"`   // disbursed / granted
}

type fundingSourcesOut struct {
	Year  int                `json:"year"`
	Items []fundingSourceOut `json:"items"`

	// UnattributedDisbursedAmount is §13 rule 6's warning: Year's vouchers that name NO source. Counted
	// by the same rule as a project's "đã giải ngân" (store.UnattributedDisbursed), so the cards' total
	// plus this equals the commune's disbursed total for Year. 0 = nothing to warn about.
	UnattributedDisbursedAmount int64 `json:"unattributed_disbursed_amount"`

	// ScopeNotice is `budget.scope_notice` as THIS commune words it — the banner §1 requires above
	// these figures, carried exactly as GET /api/v1/investment-projects carries it.
	ScopeNotice string `json:"scope_notice,omitempty"`
}

// fundingSourceCreateIn is the body of POST /api/v1/funding-sources.
type fundingSourceCreateIn struct {
	Name string `json:"name"` // trimmed; unique within the commune, never renamed
	// Year is REQUIRED: the reply is the source as read for this year, and nothing here defaults a year.
	Year int `json:"year"`
	// GrantedAmount is optional: absent/null = left blank (no figure for Year, read as 0); 0 = an
	// explicit "granted nothing", recorded.
	GrantedAmount *int64 `json:"granted_amount,omitempty"`
}

// grantedAmountIn is the body of PUT .../annual-amounts/{year}. REQUIRED — absent is 400, never 0.
type grantedAmountIn struct {
	GrantedAmount *int64 `json:"granted_amount"`
}

type grantedAmountOut struct {
	FundingSourceID string `json:"funding_source_id"`
	Year            int    `json:"year"`
	GrantedAmount   int64  `json:"granted_amount"`
}

// fundingSourceProjectOut is one row of the breakdown: THIS SOURCE'S share of one project.
type fundingSourceProjectOut struct {
	ID              string `json:"id"`
	Code            string `json:"code"`
	Name            string `json:"name"`
	PlannedAmount   int64  `json:"planned_amount"`   // the project's whole year plan, for context
	AllocatedAmount int64  `json:"allocated_amount"` // this source's allocation line
	DisbursedAmount int64  `json:"disbursed_amount"` // vouchers drawn from this source
	// DisbursedRatio = disbursed / allocated for this source; NULL when allocated is 0. Not clamped.
	DisbursedRatio *int64 `json:"disbursed_ratio"`
}

type fundingSourceProjectsOut struct {
	FundingSourceID string                    `json:"funding_source_id"`
	Name            string                    `json:"name"`
	Year            int                       `json:"year"`
	Items           []fundingSourceProjectOut `json:"items"`

	// DisbursedWithoutAllocationAmount — paid from this source in Year by projects with NO allocation
	// line for it. In the card's disbursed figure and in no row above; without it the rows would not
	// add up to the card.
	DisbursedWithoutAllocationAmount int64 `json:"disbursed_without_allocation_amount"`
}

func ratioOut(v domain.PhanVan, ok bool) *int64 {
	if !ok {
		return nil
	}
	r := int64(v)
	return &r
}

func fundingSourceOutOf(t domain.TienDoNguonVon) fundingSourceOut {
	return fundingSourceOut{
		ID: t.NguonVon.ID, Name: t.NguonVon.Ten, Order: t.NguonVon.ThuTu, Year: t.NguonVon.Nam,
		GrantedAmount:             int64(t.NguonVon.TongNguon),
		AllocatedAmount:           int64(t.DaPhanBo),
		ProjectCount:              t.SoDuAn,
		DisbursedAmount:           int64(t.DaGiaiNgan),
		UnallocatedAmount:         int64(t.UnallocatedAmount()),
		OverallocatedAmount:       int64(t.OverallocatedAmount()),
		AllocatedRatio:            ratioOut(t.TyLeDaPhanBo()),
		DisbursedOfAllocatedRatio: ratioOut(t.TyLeGiaiNganTrenPhanBo()),
		DisbursedOfGrantedRatio:   ratioOut(t.TyLeGiaiNganTrenTongNguon()),
	}
}

// yearParam parses a REQUIRED budget year. Validated BEFORE any store call, and never defaulted: a
// default decides which money is reported, invisibly (§13 rule 8).
func yearParam(w http.ResponseWriter, raw string) (int, bool) {
	year, err := strconv.Atoi(raw)
	if raw == "" || err != nil || domain.CheckFundingSourceYear(year) != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Thiếu hoặc sai năm ngân sách (2000..2100). Ví dụ: ?year=2026", "")
		return 0, false
	}
	return year, true
}

// ListFundingSources — GET /api/v1/funding-sources?year=2026
//
// NO AUDIT ENTRY, for the reason DanhSachDuAn gives: public money inside the request's own commune,
// read under `budget.read`. The commune reaches both store reads through r.Context() only.
func (h *Handler) ListFundingSources(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()
	year, ok := yearParam(w, query.Get("year"))
	if !ok {
		return
	}
	cards, err := h.d.FundingSources.TienDoTheoNguon(ctx, year)
	if err != nil {
		h.writeFundingSourceError(w, r, "đọc nguồn vốn", err)
		return
	}
	unattributed, err := h.d.FundingSources.UnattributedDisbursed(ctx, year)
	if err != nil {
		h.writeFundingSourceError(w, r, "đọc chi chưa ghi nguồn", err)
		return
	}
	notice, ok := h.scopeNotice(w, r)
	if !ok {
		return
	}
	// make(..., 0, ...): `items` marshals as [] and never as null — a commune with no source yet is
	// the ordinary state.
	out := fundingSourcesOut{
		Year: year, Items: make([]fundingSourceOut, 0, len(cards)),
		UnattributedDisbursedAmount: int64(unattributed), ScopeNotice: notice,
	}
	for _, c := range cards {
		out.Items = append(out.Items, fundingSourceOutOf(c))
	}
	vietJSON(w, http.StatusOK, out)
}

// CreateFundingSource — POST /api/v1/funding-sources
//
// 201 WITH THE SOURCE AS A CARD FOR `year` — the shape of one list item, so the screen inserts it
// without a second read. A new source has no allocation and no voucher, so every derived figure is 0.
func (h *Handler) CreateFundingSource(w http.ResponseWriter, r *http.Request) {
	var in fundingSourceCreateIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	req := app.FundingSourceCreateRequest{Name: in.Name, Year: in.Year}
	if in.GrantedAmount != nil {
		v := domain.Dong(*in.GrantedAmount)
		req.GrantedAmount = &v
	}
	added, err := h.d.FundingSourceWrites.AddFundingSource(r.Context(), req, actor)
	if err != nil {
		h.writeFundingSourceError(w, r, "thêm nguồn vốn", err)
		return
	}
	// What a retry carrying the same Idempotency-Key is told about. The source has no issued code
	// (§11), so its id — the same identifier the PUT and the breakdown are addressed by.
	idem.RecordCode(r.Context(), added.ID)
	vietJSON(w, http.StatusCreated, fundingSourceOutOf(domain.TienDoNguonVon{NguonVon: added}))
}

// SetFundingSourceGrantedAmount — PUT /api/v1/funding-sources/{id}/annual-amounts/{year}
func (h *Handler) SetFundingSourceGrantedAmount(w http.ResponseWriter, r *http.Request) {
	year, ok := yearParam(w, r.PathValue("year"))
	if !ok {
		return
	}
	var in grantedAmountIn
	if !docThan(w, r, &in) {
		return
	}
	if in.GrantedAmount == nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", domain.ErrGrantedAmountMissing.Error(), "")
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	got, err := h.d.FundingSourceWrites.SetGrantedAmount(r.Context(), r.PathValue("id"), year,
		domain.Dong(*in.GrantedAmount), actor)
	if err != nil {
		h.writeFundingSourceError(w, r, "ghi vốn được giao", err)
		return
	}
	vietJSON(w, http.StatusOK, grantedAmountOut{
		FundingSourceID: got.FundingSourceID, Year: got.Year, GrantedAmount: int64(got.GrantedAmount),
	})
}

// ListFundingSourceProjects — GET /api/v1/funding-sources/{id}/projects?year=2026
//
// A source of another commune answers the same 404 as one that does not exist — the store cannot
// reach it at all.
func (h *Handler) ListFundingSourceProjects(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	year, ok := yearParam(w, query.Get("year"))
	if !ok {
		return
	}
	got, err := h.d.FundingSources.SourceProjects(r.Context(), r.PathValue("id"), year)
	if err != nil {
		h.writeFundingSourceError(w, r, "đọc dự án theo nguồn", err)
		return
	}
	out := fundingSourceProjectsOut{
		FundingSourceID: got.Source.ID, Name: got.Source.Ten, Year: year,
		Items:                            make([]fundingSourceProjectOut, 0, len(got.Projects)),
		DisbursedWithoutAllocationAmount: int64(got.DisbursedWithoutAllocation),
	}
	for _, p := range got.Projects {
		out.Items = append(out.Items, fundingSourceProjectOut{
			ID: p.ProjectID, Code: p.Code, Name: p.Name,
			PlannedAmount: int64(p.PlannedAmount), AllocatedAmount: int64(p.AllocatedAmount),
			DisbursedAmount: int64(p.DisbursedAmount), DisbursedRatio: ratioOut(p.DisbursedRatio()),
		})
	}
	vietJSON(w, http.StatusOK, out)
}

// fundingSourceInputErrors are refusals of what the client sent. LISTED, never "anything unknown is a
// 400", for the reason laLoiDauVaoDuAn gives.
var fundingSourceInputErrors = []error{
	domain.ErrFundingSourceNameMissing, domain.ErrFundingSourceNameTooLong, domain.ErrFundingSourceNameInvalid,
	domain.ErrFundingSourceYearMissing, domain.ErrFundingSourceYearOutOfRange,
	domain.ErrGrantedAmountMissing, domain.ErrGrantedAmountNegative, domain.ErrGrantedAmountTooLarge,
}

// writeFundingSourceError maps one failure onto a status and a sentence — ONE mapping for the four
// routes.
//
// THE SENTENCE IS ALWAYS A FIXED STRING OR A SENTINEL'S OWN TEXT, NEVER err.Error() OF THE CHAIN: a
// failure from inside the transaction is wrapped with the commune id (app.wrapFundingSource), which is
// not for a clerk, and a PostgreSQL exception must never reach a client (rule 3, forbidden #3).
func (h *Handler) writeFundingSourceError(w http.ResponseWriter, r *http.Request, op string, err error) {
	switch {
	case errors.Is(err, fistore.ErrFundingSourceNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found",
			"Không tìm thấy nguồn vốn này trong danh mục nguồn vốn của xã.", "")
		return
	case errors.Is(err, fistore.ErrFundingSourceNameTaken):
		// 409, NOT 403: the caller may manage sources; what is refused is this value. The sentence
		// names the way out — a source is declared once and serves every year, so the figure the clerk
		// wanted is entered on the existing source.
		httpx.WriteError(w, http.StatusConflict, "funding_source_name_taken",
			"`name`: xã đã có nguồn vốn mang tên này. Mỗi nguồn vốn chỉ khai một lần và dùng chung cho "+
				"mọi năm — hãy ghi vốn được giao của năm vào nguồn vốn đã có.", "")
		return
	case errors.Is(err, fistore.ErrFundingSourceCatalogueFull):
		httpx.WriteError(w, http.StatusConflict, "funding_source_catalogue_full",
			"Danh mục nguồn vốn của xã đã đủ số nguồn tối đa nên chưa thêm được.", "")
		return
	}
	for _, e := range fundingSourceInputErrors {
		if errors.Is(err, e) {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", e.Error(), "")
			return
		}
	}
	if errors.Is(err, fistore.ErrQuaNhieuNguonVon) || errors.Is(err, fistore.ErrQuaNhieuDuAn) {
		// REFUSED, NOT TRUNCATED: these figures are added up on the screen.
		h.d.Log.Error("nguồn vốn: "+op+" vượt trần — TỪ CHỐI thay vì cắt bớt",
			"xa", string(tenant.MustFrom(r.Context())))
	} else {
		// The wrapped error never reaches the client; no name, no amount in the log line.
		h.d.Log.Error("nguồn vốn: "+op+" lỗi hệ thống",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
	}
	httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
}
