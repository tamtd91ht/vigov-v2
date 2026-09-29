package http

// The three routes of the `⇄ Các đợt thu, chi` dialog (docs/ui-ux/07-thu-chi-ngan-sach.md §5,
// migration 0008). Declared in routes.go with their permissions; mapped to errors by
// writeBudgetError, the one mapping for the whole budget board.
//
// AMOUNTS are JSON numbers of đồng, `null` for an empty amount (§9 rule 4) — the contract bangDayDuRa
// states for the sheet.
//
// `counterparty` ("Đơn vị, cá nhân", `don_vi_ca_nhan`) MAY NAME A PERSON (migration 0008), so it
// LEAVES THE API MASKED with core/privacy.MaskName (rule 3, invariant 3) — on the list and on the 201
// alike. No permission grants the full value: showing it would need an explicit full-view key, which
// the `quyen` table does not have (rule 3 stop condition #1; open question #27 — never an INSERT).
//
// NOT THE VOUCHER'S PRECEDENT, deliberately: chungTuRa emits its `counterparty` raw because that field
// is declared a COMPANY, never a person (domain/disbursement_voucher.go NormalizeCounterparty, 0004:283-284).
// 0008 declares the opposite for this column.
//
// THE COSTS, stated: (1) an ORGANISATION name is masked too ("Công ty TNHH ABC" -> "Công t. T. A.") —
// the text cannot tell a household from a company, and failing closed is the rule; (2) MaskName leaves
// a ONE-WORD value unchanged, which is the shared helper's behaviour and not re-decided here.
// Never logged and never put in an error message by anything in this file.

import (
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/privacy"
	"github.com/vihat/vigov/service-finance/internal/app"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// dotRa is one batch.
type dotRa struct {
	ID     string `json:"id"`
	LineID string `json:"line_id"`
	Date   string `json:"date"` // YYYY-MM-DD
	// Content is "Nội dung".
	Content string `json:"content"`
	// Counterparty is "Đơn vị, cá nhân", MASKED (privacy.MaskName) — omitted when not stated.
	Counterparty string `json:"counterparty,omitempty"`
	DocumentNo   string `json:"document_no,omitempty"`
	RecordedAt   string `json:"recorded_at,omitempty"` // RFC 3339; absent on the 201 of a create

	// Values is columnID -> đồng. ON THE LIST, EVERY LIVE NUMBER COLUMN OF THE SHEET IS A KEY, `null`
	// when this batch left it empty — the same shape dongRa.Values has. On the 201 of a create only
	// the stated amounts appear; the client refetches the list, as it refetches the sheet.
	Values map[string]*int64 `json:"values"`

	// UnavailableReasons is columnID -> sentence, for an amount STORED before the ceiling was lowered
	// (domain.ValueMax, 25/09/2026) that the browser could not read exactly. Such an amount is
	// `null` in Values and keyed here, so the accountant can find the batch and remove it. Omitted in
	// the ordinary case.
	UnavailableReasons map[string]string `json:"unavailable_reasons,omitempty"`
}

// danhSachDotRa is the dialog's list.
type danhSachDotRa struct {
	LineID string `json:"line_id"`

	// Method is the line's calculation mode. The batches move the line's DISPLAYED figure only when
	// this is `entries` (§4.2); the screen can say so instead of leaving the accountant to wonder why
	// a recorded batch changed nothing.
	Method string `json:"method"`

	Entries []dotRa `json:"entries"` // newest first: date DESC, then recording time DESC
}

// ghiDotVao is the body of POST /api/v1/budget-lines/{id}/entries.
//
// THERE IS NO `line_id`: the line is the path segment, and a second copy in the body is a second
// answer that could disagree. `values` is `map[string]*int64`: a number is an amount in đồng, `null`
// is "left empty" and stores nothing. At least one amount must be a number.
type ghiDotVao struct {
	Date         string            `json:"date"` // YYYY-MM-DD
	Content      string            `json:"content"`
	Counterparty string            `json:"counterparty,omitempty"`
	DocumentNo   string            `json:"document_no,omitempty"`
	Values       map[string]*int64 `json:"values"`
}

func entryToOut(d domain.BudgetEntry, column []domain.BudgetColumn) dotRa {
	result := dotRa{
		ID: d.ID, LineID: d.LineID, Date: formatDate(d.Date), Content: d.Content,
		Counterparty: privacy.MaskName(d.Counterparty), DocumentNo: d.VoucherNo, RecordedAt: formatTime(d.CreatedAt),
		Values: map[string]*int64{},
	}
	for _, c := range column {
		if c.Format != domain.ColumnFormatNumber {
			continue // a percentage column carries no amount (§9 rule 3)
		}
		result.Values[c.ID] = nil
	}
	for columnID, g := range d.Amounts {
		if err := domain.ValidateStoredValue(g); err != nil {
			result.Values[columnID] = nil
			if result.UnavailableReasons == nil {
				result.UnavailableReasons = map[string]string{}
			}
			result.UnavailableReasons[columnID] = err.Error()
			continue
		}
		v := int64(g)
		result.Values[columnID] = &v
	}
	return result
}

// ListBudgetEntries returns one line's live batches. GET /api/v1/budget-lines/{id}/entries
func (h *Handler) ListBudgetEntries(w http.ResponseWriter, r *http.Request) {
	list, err := h.d.Budget.LineEntries(r.Context(), r.PathValue("id"))
	if err != nil {
		h.writeBudgetError(w, r, "đọc đợt", err)
		return
	}
	result := danhSachDotRa{
		LineID: list.Line.ID, Method: string(list.Line.Method),
		Entries: make([]dotRa, 0, len(list.Entries)),
	}
	for _, d := range list.Entries {
		result.Entries = append(result.Entries, entryToOut(d, list.Columns))
	}
	writeJSON(w, http.StatusOK, result)
}

// RecordBudgetEntry records one batch. POST /api/v1/budget-lines/{id}/entries
func (h *Handler) RecordBudgetEntry(w http.ResponseWriter, r *http.Request) {
	var in ghiDotVao
	if !readBody(w, r, &in) {
		return
	}
	date, ok := parseDate(in.Date)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"`date` phải theo dạng YYYY-MM-DD, ví dụ 2026-08-25.", "")
		return
	}
	value := make(map[string]*domain.Dong, len(in.Values))
	for columnID, v := range in.Values {
		if v == nil {
			value[columnID] = nil
			continue
		}
		g := domain.Dong(*v)
		value[columnID] = &g
	}

	actor, ok := actorFrom(r)
	if !ok {
		h.noPrincipal(w, r)
		return
	}
	next, err := h.d.BudgetWriter.RecordEntry(r.Context(), app.RecordBudgetEntryRequest{
		LineID: r.PathValue("id"), Date: date, Content: in.Content,
		Counterparty: in.Counterparty, VoucherNo: in.DocumentNo, Amounts: value,
	}, actor)
	if err != nil {
		h.writeBudgetError(w, r, "ghi đợt", err)
		return
	}

	// The batch id, never the body: the body carries `counterparty` and would land in Redis, which is
	// a cache and not a record store.
	idem.RecordCode(r.Context(), next.ID)
	writeJSON(w, http.StatusCreated, entryToOut(next, nil))
}

// RemoveBudgetEntry soft deletes one batch. DELETE /api/v1/budget-entries/{id}
func (h *Handler) RemoveBudgetEntry(w http.ResponseWriter, r *http.Request) {
	var in goVao
	if !readBody(w, r, &in) {
		return
	}
	actor, ok := actorFrom(r)
	if !ok {
		h.noPrincipal(w, r)
		return
	}
	if err := h.d.BudgetWriter.RemoveEntry(r.Context(), r.PathValue("id"), in.Reason, actor); err != nil {
		h.writeBudgetError(w, r, "gỡ đợt", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
