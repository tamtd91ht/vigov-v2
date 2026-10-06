package http

// The WRITE routes of the investment project register (docs/ui-ux/06-giai-ngan.md §9, §8).
//
// THREE ROUTES, ONE PERMISSION: `budget.update` enters, corrects AND — since the user's decision of
// 06/10/2026, following the prototype (vigov-require budget/router.py) — removes. routes.go records
// the change at the DELETE route.
//
//	POST   /api/v1/investment-projects        §9's "Thêm dự án" modal; blank `code` = auto-issued
//	PATCH  /api/v1/investment-projects/{id}   §8's `[✎ Sửa dự án]`
//	DELETE /api/v1/investment-projects/{id}   soft delete, reason optional (blank = fixed reason)
//
// THE PATH IS THE ONE THE READ ROUTES ALREADY OWN. `investment-projects` was decided by the user on
// 20/09/2026 and routes.go carries the argument for both halves of the noun; nothing is renamed or
// re-derived here.
//
// §9's `☑ Tự sinh mã` (user decision 06/10/2026): `code` omitted or blank → the server issues the next
// code of the commune's series DA01, DA02 … (app.issueProjectSerial). There is no separate `auto_code`
// flag: an absent code already says it, and a flag beside a typed code would be a contradiction to
// resolve.

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/app"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// `omitempty` ON EVERY OPTIONAL INPUT FIELD, AND IT IS NOT COSMETIC. tools/apidoc marks a field
// REQUIRED in kb/20-contracts/openapi.json unless it carries `omitempty`, so without it these bodies
// would tell every generated client that `year` MUST be sent on a PATCH — on a route that answers
// 400 to exactly that.

// phanBoVao is one line of §9's dynamic `Nguồn vốn` list.
//
// BOTH FIELDS ARE REQUIRED WITHIN A LINE. Unlike a voucher, where an absent source is the meaningful
// state §13 rule 6 defines, an allocation line NAMES a source by definition — `phan_bo_nguon_von
// .nguon_von_id` is NOT NULL with `CHECK (btrim(...) <> ”)` (0007). A line with no source is a row
// the chip would count in "N nguồn" and nothing could match to a card on §6.
type phanBoVao struct {
	FundingSourceID string `json:"funding_source_id"`
	Amount          int64  `json:"amount"` // đồng
}

// themDuAnVao is the body of POST /api/v1/investment-projects — §9's modal, field for field.
//
// THE ORDER BELOW IS §9's OWN AND THE ORDER IS BUSINESS: `planned_amount` comes before
// `funding_allocations` because the split is compared against the amount, not the other way round.
//
// OVER-ALLOCATION IS REFUSED, UNDER-ALLOCATION IS NOT (user decision 06/10/2026, following the
// prototype — budget/service.py:584-610): a total above `planned_amount` answers 409
// `allocation_exceeds_plan` naming the overrun; a total below it is a normal project whose chip shows
// the shortfall. The same source twice answers 409 `duplicate_source`.
//
// THERE IS NO `disbursed_amount` AND THERE MUST NEVER BE ONE. It is SUM over the project's live
// vouchers, derived on every read and stored nowhere (0004:33-42) — a field here would be a client
// naming a figure the commune reports upward.
type themDuAnVao struct {
	// Code is OPTIONAL. Omitted or blank: the server issues the next code of this commune's series,
	// DA01, DA02 … (§9 `☑ Tự sinh mã`). Typed: it must never have been used in this commune, removed
	// projects included (409 `code_taken`). The issued code is in the 201 reply's `code`.
	Code string `json:"code,omitempty"`
	Year int    `json:"year"`

	CategoryID  string `json:"category_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	PlannedAmount  int64 `json:"planned_amount"`            // `ke_hoach_von_nam`, đồng
	ApprovedAmount int64 `json:"approved_amount,omitempty"` // 0 = "same as this year's plan" (§9)

	OrgUnitID  string `json:"org_unit_id,omitempty"`
	AssigneeID string `json:"assignee_id,omitempty"`

	StartDate      string `json:"start_date,omitempty"`      // YYYY-MM-DD
	CompletionDate string `json:"completion_date,omitempty"` // YYYY-MM-DD

	// DisbursementDeadline is optional here and defaults to 31/12 of `year` (§11). §9 is explicit
	// that it is NOT the completion date: works finished in March may still have to be disbursed
	// before 31/12.
	DisbursementDeadline string `json:"disbursement_deadline,omitempty"` // YYYY-MM-DD

	FundingAllocations []phanBoVao `json:"funding_allocations,omitempty"`
}

// suaDuAnVao is the body of PATCH /api/v1/investment-projects/{id}.
//
// EVERY EDITABLE FIELD IS A POINTER, and that is the whole reason this is a PATCH and not a PUT:
// several fields have a meaningful zero — a description cleared to "", an officer unassigned back to
// "Chưa phân công", a plan revised down to 0 — and a body of plain values cannot tell "not
// mentioned" from "cleared". A dialog editing only the name would silently unassign the officer.
//
// `Code` AND `Year` ARE REFUSED, NOT IGNORED, and the two reasons are different:
//
//	code   rule 7, forbidden #4 — a code that has been issued is never renumbered. It is printed on
//	       §7.2's row, quoted in the decisions filed against the project, and carried as the
//	       `subject` of every audit entry its vouchers have written.
//	year   §13 rule 8 — each budget year is its own set of projects. Moving one takes its whole plan
//	       and every voucher filed against it out of one year's totals and into another's.
//
// REFUSED RATHER THAN IGNORED because ignoring leaves the client believing it just moved a project
// to another year, while every screen still shows it where it was.
//
// `funding_allocations` IS THE FULL REPLACEMENT SET (decision 06/10/2026): absent or null = the lines
// are left alone; present = exactly these lines afterwards (`[]` removes every line). A pointer to a
// slice, because "absent" and "empty" mean opposite things here. Refusals: 409 `duplicate_source`,
// 409 `allocation_exceeds_plan` (also when `planned_amount` is revised below what is allocated), 409
// `source_has_disbursements` when a dropped source already has vouchers of this project.
type suaDuAnVao struct {
	CategoryID  *string `json:"category_id,omitempty"`
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`

	PlannedAmount  *int64 `json:"planned_amount,omitempty"`
	ApprovedAmount *int64 `json:"approved_amount,omitempty"`

	OrgUnitID  *string `json:"org_unit_id,omitempty"`
	AssigneeID *string `json:"assignee_id,omitempty"`

	StartDate            *string `json:"start_date,omitempty"`
	CompletionDate       *string `json:"completion_date,omitempty"`
	DisbursementDeadline *string `json:"disbursement_deadline,omitempty"`

	FundingAllocations *[]phanBoVao `json:"funding_allocations,omitempty"`

	Code *string `json:"code,omitempty"`
	Year *int    `json:"year,omitempty"`
}

// xoaDuAnVao is the body of DELETE /api/v1/investment-projects/{id}.
//
// A DELETE WITH AN OPTIONAL BODY. The reason is OPTIONAL since 06/10/2026 (user decision, following the
// prototype): omitted, blank, or no body at all → `delete_reason` is the fixed "Rút khỏi danh sách dự
// án" (domain.ProjectRemovalDefaultReason), so rule 7 invariant 1's column is never empty. When given,
// it travels in the body and never in the query string, where free text about a public authority's
// spending would land in every access log and proxy cache.
type xoaDuAnVao struct {
	Reason string `json:"reason,omitempty"`
}

// duAnGhiRa is one project as a WRITE route returns it.
//
// A SEPARATE TYPE FROM duAnRa, AND THE DIFFERENCE IS WHAT THE SERVER CAN HONESTLY SAY. duAnRa
// carries `disbursed_amount`, `disbursed_ratio`, `delay_score` and `is_delayed` — all DERIVED from
// the project's vouchers and from the commune's delay threshold, which the write path has neither
// read nor locked. Returning them here would mean either reading them a second time inside a
// response (two sources for one figure) or reporting zeros that read as real numbers: a project
// created a second ago would answer `is_delayed: false` and `disbursed_ratio: 0`, which is
// indistinguishable from a project the commune has been ignoring all year.
//
// THE CLIENT RE-READS THE PROJECT TO GET THOSE. That is one extra GET after a create, and it is the
// honest shape: the read route owns every derived figure and owns the threshold's provenance with
// it (`delay_threshold_source`).
//
// NOTHING HERE IS PERSONAL DATA (rule 3): a code, a name, amounts of public money, and two ids of
// rows owned by another service — not the names behind them.
type duAnGhiRa struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Year int    `json:"year"`

	CategoryID  string `json:"category_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	PlannedAmount int64 `json:"planned_amount"`

	// ApprovedAmount already has §9's rule for a blank field applied (domain.DuAn.TongMucHieuLuc), so
	// a client never has to know the rule. ApprovedAmountSet says whether the commune actually typed
	// a figure — without it, "approved for the same as this year" and "approved for exactly this
	// number by coincidence" are the same reply, and only the first may follow a revised plan.
	ApprovedAmount    int64 `json:"approved_amount"`
	ApprovedAmountSet bool  `json:"approved_amount_set"`

	OrgUnitID      string `json:"org_unit_id,omitempty"`
	AssigneeID     string `json:"assignee_id,omitempty"`
	StartDate      string `json:"start_date,omitempty"`
	CompletionDate string `json:"completion_date,omitempty"`

	DisbursementDeadline string `json:"disbursement_deadline"`

	// FundingAllocations is the project's live lines after the write — on create what was written, on
	// an edit what the project draws on afterwards — so the client does not have to guess which lines
	// landed. `omitempty`, because "Chưa gắn nguồn" is a normal project (§9, §11).
	FundingAllocations []phanBoRa `json:"funding_allocations,omitempty"`

	// FundingAllocatedTotal is SUM of the lines above, in đồng. The chip itself is on the READ routes
	// (`funding_status`); a write reply carries the raw figure.
	//
	// NOT `omitempty`. Zero is the answer for a project with no allocation, and it is a fact rather
	// than an absence — a field that vanished at zero would make a client unable to tell "nothing
	// allocated" from "this server does not report it".
	FundingAllocatedTotal int64 `json:"funding_allocated_total"`
}

type phanBoRa struct {
	ID              string `json:"id"`
	FundingSourceID string `json:"funding_source_id"`
	Amount          int64  `json:"amount"`
}

func duAnGhiRaNgoai(d domain.DuAn, phanBo []domain.PhanBoNguonVon) duAnGhiRa {
	ra := duAnGhiRa{
		ID:                   d.ID,
		Code:                 d.Ma,
		Year:                 d.Nam,
		CategoryID:           d.HangMucID,
		Name:                 d.Ten,
		Description:          d.MoTa,
		PlannedAmount:        int64(d.KeHoachVonNam),
		ApprovedAmount:       int64(d.TongMucHieuLuc()),
		ApprovedAmountSet:    d.TongMucDuocDuyet > 0,
		OrgUnitID:            d.DonViThucHienID,
		AssigneeID:           d.CanBoPhuTrachID,
		StartDate:            ngayRa(d.NgayKhoiCong),
		CompletionDate:       ngayRa(d.NgayHoanThanh),
		DisbursementDeadline: ngayRa(d.ThoiHanGiaiNgan),
	}
	for _, pb := range phanBo {
		ra.FundingAllocations = append(ra.FundingAllocations, phanBoRa{
			ID:              pb.ID,
			FundingSourceID: pb.NguonVonID,
			Amount:          int64(pb.SoTien),
		})
		ra.FundingAllocatedTotal += int64(pb.SoTien)
	}
	return ra
}

// ngayDuAnVao parses an optional YYYY-MM-DD project date.
//
// `time.DateOnly` AND NOT RFC 3339: all three of these are DATES — a start, a completion, a deadline
// — and a client sending an instant would be choosing a timezone for a fact that has none. The
// columns are `DATE`.
//
// THE ZERO VALUE IS RETURNED FOR AN EMPTY STRING rather than an error, so that PATCH can tell
// "absent" (a nil pointer) from "cleared" (a pointer to "") from "malformed".
func ngayDuAnVao(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, true
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// ThemDuAn records one new investment project. POST /api/v1/investment-projects
func (h *Handler) ThemDuAn(w http.ResponseWriter, r *http.Request) {
	var vao themDuAnVao
	if !docThan(w, r, &vao) {
		return
	}

	khoiCong, ok := ngayDuAnVao(vao.StartDate)
	if !ok {
		h.ngaySai(w, "start_date")
		return
	}
	hoanThanh, ok := ngayDuAnVao(vao.CompletionDate)
	if !ok {
		h.ngaySai(w, "completion_date")
		return
	}
	hanGiaiNgan, ok := ngayDuAnVao(vao.DisbursementDeadline)
	if !ok {
		h.ngaySai(w, "disbursement_deadline")
		return
	}

	phanBo := make([]domain.DongPhanBoMoi, 0, len(vao.FundingAllocations))
	for _, mot := range vao.FundingAllocations {
		phanBo = append(phanBo, domain.DongPhanBoMoi{
			NguonVonID: mot.FundingSourceID,
			SoTien:     domain.Dong(mot.Amount),
		})
	}

	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}

	kq, err := h.d.GhiDuAn.Them(r.Context(), app.YeuCauThemDuAn{
		Ma:               vao.Code,
		Nam:              vao.Year,
		HangMucID:        vao.CategoryID,
		Ten:              vao.Name,
		MoTa:             vao.Description,
		KeHoachVonNam:    domain.Dong(vao.PlannedAmount),
		TongMucDuocDuyet: domain.Dong(vao.ApprovedAmount),
		DonViThucHienID:  vao.OrgUnitID,
		CanBoPhuTrachID:  vao.AssigneeID,
		NgayKhoiCong:     khoiCong,
		NgayHoanThanh:    hoanThanh,
		ThoiHanGiaiNgan:  hanGiaiNgan,
		PhanBo:           phanBo,
	}, nguoi)
	if err != nil {
		h.traLoiLoiDuAn(w, r, "thêm", err)
		return
	}

	// What a retry carrying the same Idempotency-Key is told about. THE PROJECT'S BUSINESS CODE and
	// not the internal id: unlike a voucher, a project HAS a code of its own, it is the value §7.2
	// prints and the value the audit trail files the act under, and it is stable across everything
	// that can happen to the row afterwards.
	idem.RecordCode(r.Context(), kq.DuAn.Ma)
	vietJSON(w, http.StatusCreated, duAnGhiRaNgoai(kq.DuAn, kq.PhanBo))
}

// SuaDuAn corrects one project. PATCH /api/v1/investment-projects/{id}
func (h *Handler) SuaDuAn(w http.ResponseWriter, r *http.Request) {
	var vao suaDuAnVao
	if !docThan(w, r, &vao) {
		return
	}
	// BEFORE ANYTHING ELSE, so the caller learns that neither value is theirs to change rather than
	// watching the field disappear.
	if vao.Code != nil {
		h.traLoiLoiDuAn(w, r, "sửa", domain.ErrMaDuAnBatBien)
		return
	}
	if vao.Year != nil {
		h.traLoiLoiDuAn(w, r, "sửa", domain.ErrNamBatBien)
		return
	}

	yc := app.YeuCauSuaDuAn{
		// THE POINTERS ARE PASSED THROUGH UNTOUCHED, including pointers to "". Dereferencing one here
		// to "decide" whether the client meant it would collapse "leave alone" and "clear" into one
		// value at the only layer that can still tell them apart.
		HangMucID:       vao.CategoryID,
		Ten:             vao.Name,
		MoTa:            vao.Description,
		DonViThucHienID: vao.OrgUnitID,
		CanBoPhuTrachID: vao.AssigneeID,
	}
	if vao.PlannedAmount != nil {
		so := domain.Dong(*vao.PlannedAmount)
		yc.KeHoachVonNam = &so
	}
	if vao.ApprovedAmount != nil {
		so := domain.Dong(*vao.ApprovedAmount)
		yc.TongMucDuocDuyet = &so
	}
	if vao.FundingAllocations != nil {
		// NON-NIL EVEN WHEN EMPTY: `[]` is "remove every line", and only a nil pointer means "leave the
		// lines alone".
		phanBo := make([]domain.DongPhanBoMoi, 0, len(*vao.FundingAllocations))
		for _, mot := range *vao.FundingAllocations {
			phanBo = append(phanBo, domain.DongPhanBoMoi{
				NguonVonID: mot.FundingSourceID,
				SoTien:     domain.Dong(mot.Amount),
			})
		}
		yc.PhanBo = &phanBo
	}
	for _, mot := range []struct {
		vao    *string
		truong string
		ra     **time.Time
	}{
		{vao.StartDate, "start_date", &yc.NgayKhoiCong},
		{vao.CompletionDate, "completion_date", &yc.NgayHoanThanh},
		{vao.DisbursementDeadline, "disbursement_deadline", &yc.ThoiHanGiaiNgan},
	} {
		if mot.vao == nil {
			continue
		}
		// A POINTER TO "" IS A CLEAR, NOT A MALFORMED DATE, and it reaches the use case as a pointer
		// to the zero time. `start_date` and `completion_date` go back to NULL; the deadline cannot,
		// because its column is NOT NULL — the use case sends it back to §11's 31/12 default, in the
		// one function that owns that rule.
		ngay, ok := ngayDuAnVao(*mot.vao)
		if !ok {
			h.ngaySai(w, mot.truong)
			return
		}
		t := ngay
		*mot.ra = &t
	}

	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}

	kq, err := h.d.GhiDuAn.Sua(r.Context(), r.PathValue("id"), yc, nguoi)
	if err != nil {
		h.traLoiLoiDuAn(w, r, "sửa", err)
		return
	}
	// THE LIVE LINES AFTER THE EDIT, read inside the edit's own transaction (app.Sua), so
	// `funding_allocated_total` is the real figure on every PATCH reply — whether or not this PATCH
	// named `funding_allocations`.
	vietJSON(w, http.StatusOK, duAnGhiRaNgoai(kq.DuAn, kq.Allocations))
}

// XoaDuAn soft deletes one project. DELETE /api/v1/investment-projects/{id}
//
// 204 AND NO BODY. The row is still there — it carries `deleted_at`, `deleted_by` and
// `delete_reason`, and its code stays taken forever — but there is nothing the caller can do with
// it, and returning it would invite a client to display a project it has just taken off the screen.
func (h *Handler) XoaDuAn(w http.ResponseWriter, r *http.Request) {
	// AN EMPTY BODY IS ACCEPTED HERE, unlike docThan: the reason is optional, and a client removing a
	// project without one may well send no body at all. Anything that IS sent must still be valid JSON.
	var vao xoaDuAnVao
	r.Body = http.MaxBytesReader(w, r.Body, thanToiDa)
	if err := json.NewDecoder(r.Body).Decode(&vao); err != nil && !errors.Is(err, io.EOF) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Nội dung gửi lên không phải JSON hợp lệ hoặc quá lớn.", "")
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	if err := h.d.GhiDuAn.Xoa(r.Context(), r.PathValue("id"), vao.Reason, nguoi); err != nil {
		h.traLoiLoiDuAn(w, r, "xoá", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ngaySai answers a malformed date, naming the field that carried it IN THE MESSAGE.
//
// ⚠ THE FIFTH PARAMETER OF httpx.WriteError IS `traceID`, AND THERE IS NO `field` PARAMETER AT ALL —
// httpx.Error carries exactly `code`, `message` and `trace_id` (core/httpx/edge.go:88-92). A field
// name passed in that position lands in `trace_id`, where it looks precisely like a real trace id
// and sends an operator searching centralised logging for something that was never written. The same
// defect shipped in service-petitions and was found on 2026-09-23.
//
// SO THE FIELD NAME GOES IN THE SENTENCE, backtick-quoted, which is the convention every other
// handler in this service already follows ("`payment_date` phải theo dạng YYYY-MM-DD"). A client
// reading the message learns which box to fix; nothing pretends to be a trace id.
func (h *Handler) ngaySai(w http.ResponseWriter, truong string) {
	httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
		"`"+truong+"` phải theo dạng YYYY-MM-DD, ví dụ 2026-09-07.", "")
}

// traLoiLoiDuAn maps one use-case failure onto a status and a sentence.
//
// ONE FUNCTION FOR ALL THREE ROUTES, because three copies of this mapping would drift and the copy
// that drifts is the one answering 500 where it meant 409 — which reads to an operator as a broken
// server rather than as a rule doing its job.
//
// WHY 409 AND NOT 403 FOR EVERY BUSINESS REFUSAL: the caller HOLDS the permission and is allowed to
// perform the operation. What is refused is this operation on THIS project, because of a value the
// commune has already used or a state the record is in. 403 would send an accountant to the Phân
// quyền screen to be granted a right they already have.
//
// ⚠ EVERY CALL BELOW PASSES "" AS THE FIFTH ARGUMENT, AND THAT IS NOT AN OMISSION. It is `traceID`
// — httpx.Error has no `field` at all (core/httpx/edge.go:88-92) — so a field name there lands in
// `trace_id` and reads as a real one. Which field is at fault is said in the SENTENCE instead.
//
// THE DOMAIN'S AND THE STORE'S OWN SENTENCES ARE RETURNED ON EVERY REFUSAL, deliberately. They name
// the operation and the way out, hold no personal data and no internal detail, and a second sentence
// written here would drift from them. What must NEVER reach a client is the PostgreSQL exception
// underneath — its text is English, it names a constraint, and it says nothing an accountant in a
// commune can act on (rule 3, forbidden #3).
func (h *Handler) traLoiLoiDuAn(w http.ResponseWriter, r *http.Request, viec string, err error) {
	var overPlan *domain.AllocationExceedsPlanError
	switch {
	case errors.Is(err, fistore.ErrKhongThayDuAn):
		// 404 COVERS "no such project" AND "a project of another commune" as ONE answer, because the
		// store cannot reach another commune's row at all. Two different answers would tell a caller
		// that a record exists inside an authority they have no business knowing about (rule 4,
		// forbidden #2, applied between communes).
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy dự án.", "")
	case errors.Is(err, domain.ErrProjectSerialExhausted):
		// 409: the caller may create projects; what blocks the AUTO code is the state of the commune's
		// codes (a long run of hand-typed DA codes). Typing a code is the way out, and the sentence says so.
		httpx.WriteError(w, http.StatusConflict, "code_series_blocked",
			"`code`: không cấp được mã tự sinh vì quá nhiều mã DA liên tiếp đã được nhập tay. "+
				"Hãy nhập mã dự án.", "")
	case errors.Is(err, fistore.ErrMaDuAnDaTonTai):
		// The message says WHY a code that is nowhere on the screen is nonetheless taken: a removed
		// project keeps its code forever (§9, rule 7, invariant 3). Without that sentence this reads
		// as a bug in front of somebody who has just checked the list.
		httpx.WriteError(w, http.StatusConflict, "code_taken",
			"`code`: mã dự án này đã được dùng trong xã — kể cả khi dự án mang mã đó đã rút khỏi "+
				"danh sách. Mã đã cấp thì không cấp lại. Hãy chọn một mã khác.", "")
	case errors.Is(err, fistore.ErrKhongThayHangMuc):
		// 404 NAMING THE FIELD, not the project: the project may well exist. A category of another
		// commune is indistinguishable from one that does not exist, because the query cannot reach
		// it at all (rule 1) — so one answer must cover both, or the status code itself would tell a
		// caller which communes hold which categories.
		httpx.WriteError(w, http.StatusNotFound, "not_found",
			"`category_id`: không tìm thấy hạng mục kế hoạch vốn này trong xã.", "")
	case errors.Is(err, fistore.ErrKhongThayNguonVonPhanBo):
		// Since migration 0013 a source belongs to the commune's catalogue and serves every year, so
		// the sentence names the catalogue — never "the project's budget year", which is no longer a
		// property a source has.
		httpx.WriteError(w, http.StatusNotFound, "not_found",
			"`funding_allocations`: không tìm thấy nguồn vốn này trong danh mục nguồn vốn của xã.", "")
	case errors.Is(err, fistore.ErrDuAnConChungTu):
		// 409 AND THE SENTENCE NAMES THE WAY OUT, because otherwise the screen looks broken: the
		// project is right there and the Delete button did nothing. What the commune has to do first
		// is remove the vouchers — each with its own reason and its own audit entry, which is the
		// point.
		httpx.WriteError(w, http.StatusConflict, "project_has_vouchers",
			"Dự án này còn chứng từ giải ngân nên chưa xoá được. "+
				"Hãy gỡ các chứng từ kèm lý do trước, rồi xoá dự án.", "")
	case errors.As(err, &overPlan):
		// 409, NOT 400: the body is well formed; what is refused is this total against this project's
		// plan. The sentence is the domain's own and names the overrun — amounts of public money only,
		// never the commune id bocDuAn wraps around it.
		httpx.WriteError(w, http.StatusConflict, "allocation_exceeds_plan", overPlan.Sentence(), "")
	case errors.Is(err, domain.ErrPhanBoTrungNguon):
		httpx.WriteError(w, http.StatusConflict, "duplicate_source",
			"`funding_allocations`: mỗi nguồn vốn chỉ khai một dòng trong một dự án — hãy gộp số tiền "+
				"vào một dòng.", "")
	case errors.Is(err, domain.ErrSourceHasDisbursements):
		// The way out is in the sentence: the vouchers drawn from that source have to be moved to another
		// source or removed (each with its own reason and audit entry) before the source can go.
		httpx.WriteError(w, http.StatusConflict, "source_has_disbursements",
			"`funding_allocations`: nguồn vốn muốn gỡ đã có chứng từ giải ngân của dự án này. Hãy chuyển "+
				"các chứng từ ấy sang nguồn khác hoặc gỡ chúng trước, rồi mới gỡ nguồn vốn.", "")
	case errors.Is(err, domain.ErrProjectStartAfterCompletion):
		// A FIXED SENTENCE AND NOT err.Error(): on the edit path the refusal comes out of the
		// transaction wrapped by bocDuAn, whose text carries the commune's id — not for a clerk.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Ngày khởi công không được sau ngày hoàn thành.", "")
	case laLoiDauVaoDuAn(err):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
	default:
		// The wrapped error carries the store failure and never reaches the client. The commune is
		// logged because it is the only thing an operator can act on; the project name and the
		// amounts are NOT, because a log line travels into centralised logging across every commune
		// at once.
		h.d.Log.Error("dự án đầu tư: "+viec+" lỗi hệ thống",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}

// laLoiDauVaoDuAn reports whether this is a refusal of what the client sent, as opposed to a failure.
//
// LISTED EXPLICITLY RATHER THAN DEFAULTING TO 400. A default of "anything I do not recognise is the
// client's fault" turns a database outage into a 400, and a client that believes its input is wrong
// retries with different input forever while nobody is told the server is broken.
func laLoiDauVaoDuAn(err error) bool {
	for _, mot := range []error{
		domain.ErrThieuMaDuAn, domain.ErrMaDuAnQuaDai, domain.ErrMaDuAnSaiDinhDang,
		domain.ErrMaDuAnBatBien, domain.ErrNamBatBien,
		domain.ErrThieuNamDuAn, domain.ErrNamDuAnNgoaiLich,
		domain.ErrThieuHangMuc, domain.ErrHangMucIDSai,
		domain.ErrThieuTenDuAn, domain.ErrTenDuAnQuaDai, domain.ErrMoTaQuaDai,
		domain.ErrKeHoachVonAm, domain.ErrKeHoachVonQuaLon,
		domain.ErrTongMucAm, domain.ErrTongMucQuaLon,
		domain.ErrNgayDuAnNgoaiLich, domain.ErrThamChieuQuaDai,
		domain.ErrThieuLyDoXoaDuAn, domain.ErrLyDoXoaDuAnQuaDai,
		domain.ErrThieuNguonVonPhanBo,
		domain.ErrPhanBoAm, domain.ErrPhanBoQuaLon, domain.ErrQuaNhieuDongPhanBo,
	} {
		if errors.Is(err, mot) {
			return true
		}
	}
	return false
}
