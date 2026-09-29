package http

// The routes of SỔ VĂN BẢN ĐI.
//
// ⚠ NO SPECIFICATION EXISTS FOR THIS REGISTER — see migration 0004. The four routes mirror the
// incoming register's, minus routing (an outgoing document is not sent between departments; it
// leaves the commune) and minus the deadline (there is no commitment to meet: issuing IS the act).
//
// FOUR ROUTES, TWO PERMISSIONS, THE SAME KEYS AS THE INCOMING REGISTER: `document.create` issues and
// corrects, `document.read` lists. No key was invented (rule 5, invariant 3c), and the same finding
// applies — there is no `document.delete`, so removing an entry is guarded by `document.create`.
//
// THE URL RESOURCE IS `outgoing-documents`, settled at kb/00-foundation/ubiquitous-language.md:143
// and not translated on the spot (ADR 0011).
//
// THE ERROR MAPPING IS writeDocumentError, SHARED WITH THE INCOMING REGISTER. One mapping, because
// the two books must answer the same question the same way; the sentences differ where the books
// differ, which is what the two separate `Err…NotFound` sentinels are for.

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"unicode/utf8"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-documents/internal/app"
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

// vanBanDiRa is one outgoing document as it leaves the API.
//
// ⚠ `Recipient` MAY NAME A CITIZEN (rule 3) — "Ông Nguyễn Văn A, thôn Bình Trị" is what a commune
// writes on a reply. It is returned unmasked to that commune's own staff, which is what the register
// is for; what is guaranteed is that it is never logged and never appears in an error message.
//
// THERE IS NO `status` AND NO `due_at`. An outgoing document has no lifecycle and no commitment in
// any source — see domain.OutgoingDocument. Inventing either would invent a workflow a commune then
// has to follow, or a promise nobody made.
//
// THE TYPE NAME STAYS VIETNAMESE: it is a component name of kb/20-contracts/openapi.json (routes.go).
type vanBanDiRa struct {
	ID string `json:"id"`

	// Number and Year are the ISSUED NUMBER — the one that is printed on the document, sealed and
	// sent out. Output only: a request carrying either is refused with 400.
	Number int `json:"number"`
	Year   int `json:"year"`

	DocumentDate string `json:"document_date"` // YYYY-MM-DD, the day it was signed and issued
	DocumentType string `json:"document_type"` // the catalogue CODE
	Summary      string `json:"summary"`
	Recipient    string `json:"recipient"`
	Signer       string `json:"signer,omitempty"`

	CreatedBy string `json:"created_by"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func renderOutgoing(d domain.OutgoingDocument) vanBanDiRa {
	return vanBanDiRa{
		ID:           d.ID,
		Number:       d.IssuedNo,
		Year:         d.Year,
		DocumentDate: formatDate(d.DocumentDate),
		DocumentType: d.DocumentType,
		Summary:      d.Summary,
		Recipient:    d.Recipient,
		Signer:       d.Signer,
		CreatedBy:    d.CreatedBy,
		CreatedAt:    formatInstant(d.CreatedAt),
		UpdatedAt:    formatInstant(d.UpdatedAt),
	}
}

// capSoVanBanDiVao is the body of POST /api/v1/outgoing-documents.
//
// `Number` IS HERE ONLY SO IT CAN BE REFUSED, and on this register that refusal is the heaviest one
// in the file: the number goes onto paper, under a seal, out of the building. A client that could
// name it could put a number already carried by a signed document onto a second one.
type capSoVanBanDiVao struct {
	DocumentDate string `json:"document_date"` // YYYY-MM-DD
	DocumentType string `json:"document_type"`
	Summary      string `json:"summary"`
	Recipient    string `json:"recipient"`
	Signer       string `json:"signer,omitempty"`

	Number *int `json:"number,omitempty"`
}

// suaVanBanDiVao is the body of PATCH /api/v1/outgoing-documents/{id}. Pointers for the same reason
// as the incoming register: `signer` is optional and its empty value is a statement.
type suaVanBanDiVao struct {
	DocumentDate *string `json:"document_date,omitempty"`
	DocumentType *string `json:"document_type,omitempty"`
	Summary      *string `json:"summary,omitempty"`
	Recipient    *string `json:"recipient,omitempty"`
	Signer       *string `json:"signer,omitempty"`

	Number *int `json:"number,omitempty"`
}

// IssueOutgoingDocument issues one outgoing document number. POST /api/v1/outgoing-documents
func (h *Handler) IssueOutgoingDocument(w http.ResponseWriter, r *http.Request) {
	var in capSoVanBanDiVao
	if !decodeBody(w, r, &in) {
		return
	}
	if err := refuseClientSupplied(in.Number, nil, nil); err != nil {
		h.writeDocumentError(w, r, "cấp số", err)
		return
	}

	documentDate, ok := parseDate(in.DocumentDate)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"`document_date` phải theo dạng YYYY-MM-DD, ví dụ 2026-09-22.", "")
		return
	}

	actor, ok := actorFrom(r)
	if !ok {
		h.missingPrincipal(w, r)
		return
	}

	created, err := h.d.OutgoingDocumentWriter.IssueNumber(r.Context(), app.IssueOutgoingDocumentRequest{
		DocumentDate: documentDate,
		DocumentType: in.DocumentType,
		Summary:      in.Summary,
		Recipient:    in.Recipient,
		Signer:       in.Signer,
	}, actor)
	if err != nil {
		h.writeDocumentError(w, r, "cấp số", err)
		return
	}

	idem.RecordCode(r.Context(), created.ID)
	writeJSON(w, http.StatusCreated, renderOutgoing(created))
}

// UpdateOutgoingDocument corrects one issued entry. PATCH /api/v1/outgoing-documents/{id}
func (h *Handler) UpdateOutgoingDocument(w http.ResponseWriter, r *http.Request) {
	var in suaVanBanDiVao
	if !decodeBody(w, r, &in) {
		return
	}
	if err := refuseClientSupplied(in.Number, nil, nil); err != nil {
		h.writeDocumentError(w, r, "sửa", err)
		return
	}

	req := app.UpdateOutgoingDocumentRequest{
		DocumentType: in.DocumentType,
		Summary:      in.Summary,
		Recipient:    in.Recipient,
		Signer:       in.Signer,
	}
	if in.DocumentDate != nil {
		t, ok := parseDate(*in.DocumentDate)
		if !ok || t.IsZero() {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
				"`document_date` phải theo dạng YYYY-MM-DD, ví dụ 2026-09-22.", "")
			return
		}
		req.DocumentDate = &t
	}

	actor, ok := actorFrom(r)
	if !ok {
		h.missingPrincipal(w, r)
		return
	}

	// Scoped: the use case opens uc.db.For(ctx).Tx — tenant_id is $1 of every statement.
	after, err := h.d.OutgoingDocumentWriter.Update(r.Context(), r.PathValue("id"), req, actor)
	if err != nil {
		h.writeDocumentError(w, r, "sửa", err)
		return
	}
	writeJSON(w, http.StatusOK, renderOutgoing(after))
}

// RemoveOutgoingDocument soft deletes one entry. DELETE /api/v1/outgoing-documents/{id}
//
// 204 AND NO BODY, and the row stays with its three removal columns. THE NUMBER STAYS TAKEN — the
// document was issued under it and has left the commune; taking the row off the screen cannot take
// the number off the paper.
func (h *Handler) RemoveOutgoingDocument(w http.ResponseWriter, r *http.Request) {
	var in goVanBanVao
	if !decodeBody(w, r, &in) {
		return
	}
	actor, ok := actorFrom(r)
	if !ok {
		h.missingPrincipal(w, r)
		return
	}
	if err := h.d.OutgoingDocumentWriter.Remove(r.Context(), r.PathValue("id"), in.Reason, actor); err != nil {
		h.writeDocumentError(w, r, "gỡ", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListOutgoingDocuments serves one page of the outgoing register. GET /api/v1/outgoing-documents
func (h *Handler) ListOutgoingDocuments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// THE COMMUNE IS FIXED BY httpx.TenantMiddleware FROM Host, and the store binds `tenant_id` to
	// $1 from the context on every statement (rule 1, invariant 5). NOTHING BELOW READS `tenant_id`
	// from the query string — a client naming its own commune grants itself access (rule 1, #2).
	params := r.URL.Query()

	req, err := page.Parse(params, docstore.OutgoingDocumentSorts)
	if err != nil {
		status, code, message := page.HTTPError(err)
		httpx.WriteError(w, status, code, message, "")
		return
	}

	filter, err := outgoingFilterFromQuery(params)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
		return
	}

	// Scoped: the store reads through s.db.For(ctx) — tenant_id is $1 of the page query.
	res, err := h.d.OutgoingDocuments.List(ctx, filter, req)
	if err != nil {
		// The wrapped error carries the store failure. It does NOT carry a summary or a recipient,
		// and it never reaches the client (rule 3, forbidden #3).
		h.d.Log.Error("danh sách văn bản đi: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	out := page.Result[vanBanDiRa]{
		Items:      make([]vanBanDiRa, 0, len(res.Items)),
		NextCursor: res.NextCursor,
		HasMore:    res.HasMore,
	}
	for _, d := range res.Items {
		out.Items = append(out.Items, renderOutgoing(d))
	}
	writeJSON(w, http.StatusOK, out)
}

// The two filter refusals, shared with the incoming register so that one mistake reads the same on
// both screens. A second wording is a second answer to one question.
var (
	errInvalidYear   = errors.New("`year` phải là một năm hợp lệ, ví dụ 2026")
	errSearchTooLong = errors.New("`q` quá dài")
	errInvalidStatus = errors.New("`status` không phải một trạng thái của sổ văn bản đến")
)

// outgoingFilterFromQuery validates the filters of the outgoing list. `params` is the handler's
// r.URL.Query(), passed through under one name.
func outgoingFilterFromQuery(params url.Values) (docstore.OutgoingDocumentFilter, error) {
	var filter docstore.OutgoingDocumentFilter
	if s := params.Get("year"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 2000 || n > 2200 {
			return filter, errInvalidYear
		}
		filter.Year = n
	}
	filter.DocumentType = params.Get("document_type")
	filter.Search = params.Get("q")
	// Runes, not bytes — the same reason as incomingFilterFromQuery.
	if utf8.RuneCountInString(filter.Search) > 200 {
		return filter, errSearchTooLong
	}
	return filter, nil
}
