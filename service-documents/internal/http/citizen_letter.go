package http

// The edge of SỔ ĐƠN THƯ CÔNG DÂN (`/api/v1/citizen-letters…`, ADR 0078). Routes and their
// permission declarations: routes_citizen_letter.go.
//
// WHAT THIS FILE OWNS IS MASKING. Every letter leaves through letterItemOut or letterOut, and both
// mask from ONE domain.LetterDisclosure — the list's (domain.ListDisclosure) or the drawer's
// (domain.DetailDisclosure). The rules, said once:
//
//	sender phone    ALWAYS masked (privacy.MaskPhone). No full-view key exists for letters, and rule
//	                5 invariant 3c forbids inventing one (ADR 0078 #4) — fail closed.
//	sender address  NEVER returned, only `has_sender_address`. core/privacy has no address mask, and
//	                inventing a masking behaviour here would be a second copy of rule 3's one answer.
//	sender name     in full for the three ordinary types; for a DENUNCIATION only to its assignee, in
//	                the drawer — never on a list, never in the duplicate warning, never in the report.
//	summary         in full for the three ordinary types; for a denunciation only in the drawer, to its
//	                assignee and to holders of `petition.create` (domain.DetailDisclosure says why).
//
// THE COMMUNE is the request's, resolved from Host by httpx.TenantMiddleware; the use case and the
// store bind that tenant_id. Nothing here reads a commune or a person from the request.
//
// NOTHING HERE LOGS A LETTER FIELD. Errors carry the commune and the operation only (rule 3).

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/privacy"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-documents/internal/app"
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

// --- output -----------------------------------------------------------------------------------------

// citizenLetterItemOut is one row of the register list.
//
// THERE IS NO `overdue` FIELD (rule 10, invariant 3 — vanBanDenRa says why a boolean beside the
// deadline is a second copy of one fact). The two due instants are the facts; both are null this run
// (ADR 0078 #3), which the screen renders as "Không đặt hạn".
//
// `days_open` IS NOT A DEADLINE: it counts days already spent (C16/C17), so it is computed on read.
type citizenLetterItemOut struct {
	ID           string `json:"id"`
	Number       int    `json:"number"`
	Year         int    `json:"year"`
	ReceivedDate string `json:"received_date"` // YYYY-MM-DD
	LetterType   string `json:"letter_type"`   // kien-nghi-phan-anh · khieu-nai · to-cao · de-nghi

	// null for a denunciation (identity_withheld) and when no name was given.
	SenderName *string `json:"sender_name"`
	// MASKED, always (09****0000). null for a denunciation and when no phone was given.
	SenderPhone *string `json:"sender_phone"`
	// true when the sender's identity is deliberately withheld (Luật Tố cáo 2018 Đ.8): the client shows
	// a fixed sentence instead of "Không rõ người gửi".
	IdentityWithheld bool `json:"identity_withheld"`

	// null for a denunciation (summary_withheld).
	Summary         *string `json:"summary"`
	SummaryWithheld bool    `json:"summary_withheld"`

	HoldingUnitID string `json:"holding_unit_id,omitempty"` // identity's `bo_phan.id`
	AssigneeCode  string `json:"assignee_code,omitempty"`   // a staff business code
	Status        string `json:"status"`

	ProcessingDueAt *string `json:"processing_due_at"` // RFC 3339; null = no deadline set
	ResolutionDueAt *string `json:"resolution_due_at"` // RFC 3339; null = no deadline set

	DaysOpen        int    `json:"days_open"`
	IsResolved      bool   `json:"is_resolved"` // da-giai-quyet / dinh-chi
	IsClosed        bool   `json:"is_closed"`   // any finishing status
	RelatedLetterID string `json:"related_letter_id,omitempty"`
}

// citizenLetterOut is one letter for the detail drawer and the answer of every write.
type citizenLetterOut struct {
	ID           string `json:"id"`
	Number       int    `json:"number"`
	Year         int    `json:"year"`
	ReceivedDate string `json:"received_date"`
	LetterType   string `json:"letter_type"`

	SenderName       *string `json:"sender_name"`
	SenderPhone      *string `json:"sender_phone"` // MASKED, always
	HasSenderAddress bool    `json:"has_sender_address"`
	// C7's "Không rõ người gửi", DERIVED. null when the identity is withheld — answering it would say
	// whether a whistleblower signed the letter.
	SenderUnknown    *bool `json:"sender_unknown"`
	IdentityWithheld bool  `json:"identity_withheld"`

	Summary         *string `json:"summary"`
	SummaryWithheld bool    `json:"summary_withheld"`

	Status       string   `json:"status"`
	NextStatuses []string `json:"next_statuses"` // C3's arrows out of the current status

	HoldingUnitID string `json:"holding_unit_id,omitempty"`
	AssigneeCode  string `json:"assignee_code,omitempty"`

	ProcessingDueAt *string `json:"processing_due_at"`
	ResolutionDueAt *string `json:"resolution_due_at"`
	AcceptedAt      *string `json:"accepted_at"`
	ResolvedAt      *string `json:"resolved_at"`
	ClosedAt        *string `json:"closed_at"`
	DaysOpen        int     `json:"days_open"`
	IsResolved      bool    `json:"is_resolved"`
	IsClosed        bool    `json:"is_closed"`

	RelatedLetterID string `json:"related_letter_id,omitempty"`

	ResultDocumentNo   string  `json:"result_document_no,omitempty"`
	ResultDocumentDate string  `json:"result_document_date,omitempty"` // YYYY-MM-DD
	ResultSigner       string  `json:"result_signer,omitempty"`
	ResultIssuer       string  `json:"result_issuer,omitempty"`
	ResultSummary      *string `json:"result_summary"` // null when none, or withheld with the summary

	CreatedByCode string `json:"created_by_code"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func instantPtr(t time.Time) *string {
	if t.IsZero() {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

func maskedPhone(s string) *string {
	if s == "" {
		return nil
	}
	m := privacy.MaskPhone(s)
	return &m
}

func letterItemOut(l domain.CitizenLetter, now time.Time) citizenLetterItemOut {
	show := domain.ListDisclosure(l.Type)
	out := citizenLetterItemOut{
		ID: l.ID, Number: l.Number, Year: l.Year, ReceivedDate: ngayRa(l.ReceivedDate),
		LetterType: string(l.Type), Status: string(l.Status),
		IdentityWithheld: !show.Identity, SummaryWithheld: !show.Summary,
		HoldingUnitID: l.HoldingUnitID, AssigneeCode: l.AssigneeCode,
		ProcessingDueAt: instantPtr(l.ProcessingDueAt), ResolutionDueAt: instantPtr(l.ResolutionDueAt),
		DaysOpen: l.DaysOpen(now), IsResolved: l.Status.EndsResolution(), IsClosed: l.Status.Finished(),
		RelatedLetterID: l.RelatedLetterID,
	}
	if show.Identity {
		out.SenderName = strPtr(l.SenderName)
		out.SenderPhone = maskedPhone(l.SenderPhone)
	}
	if show.Summary {
		out.Summary = strPtr(l.Summary)
	}
	return out
}

func letterOut(l domain.CitizenLetter, show domain.LetterDisclosure, now time.Time) citizenLetterOut {
	next := make([]string, 0, 5)
	for _, s := range l.Status.NextStatuses() {
		next = append(next, string(s))
	}
	out := citizenLetterOut{
		ID: l.ID, Number: l.Number, Year: l.Year, ReceivedDate: ngayRa(l.ReceivedDate),
		LetterType: string(l.Type), Status: string(l.Status), NextStatuses: next,
		IdentityWithheld: !show.Identity, SummaryWithheld: !show.Summary,
		HoldingUnitID: l.HoldingUnitID, AssigneeCode: l.AssigneeCode,
		ProcessingDueAt: instantPtr(l.ProcessingDueAt), ResolutionDueAt: instantPtr(l.ResolutionDueAt),
		AcceptedAt: instantPtr(l.AcceptedAt), ResolvedAt: instantPtr(l.ResolvedAt), ClosedAt: instantPtr(l.ClosedAt),
		DaysOpen: l.DaysOpen(now), IsResolved: l.Status.EndsResolution(), IsClosed: l.Status.Finished(),
		RelatedLetterID:  l.RelatedLetterID,
		ResultDocumentNo: l.ResultDocumentNo, ResultDocumentDate: ngayRa(l.ResultDocumentDate),
		ResultSigner: l.ResultSigner, ResultIssuer: l.ResultIssuer,
		CreatedByCode: l.CreatedByCode, CreatedAt: lucRa(l.CreatedAt), UpdatedAt: lucRa(l.UpdatedAt),
	}
	if show.Identity {
		out.SenderName = strPtr(l.SenderName)
		out.SenderPhone = maskedPhone(l.SenderPhone)
		out.HasSenderAddress = l.SenderAddress != ""
		unknown := l.SenderUnknown()
		out.SenderUnknown = &unknown
	}
	if show.Summary {
		out.Summary = strPtr(l.Summary)
		out.ResultSummary = strPtr(l.ResultSummary)
	}
	return out
}

// letterLogEntryOut is one log row. STAFF-INTERNAL (rule 4, forbidden #5). `content` is returned to
// every `petition.read` holder, including on a denunciation: by construction it holds no sender value
// (a correction row says only that one happened, a result row names the issued document), EXCEPT the
// free text of a note, a routing reason or a status note — which the officer wrote for colleagues.
type letterLogEntryOut struct {
	ID           string `json:"id"`
	LetterID     string `json:"letter_id"`
	At           string `json:"at"` // RFC 3339
	ActorCode    string `json:"actor_code"`
	Kind         string `json:"kind"` // chuyen-trang-thai · luan-chuyen · ghi-chu · ket-qua · sua-nguoi-gui
	FromStatus   string `json:"from_status,omitempty"`
	ToStatus     string `json:"to_status,omitempty"`
	FromUnitID   string `json:"from_unit_id,omitempty"`
	ToUnitID     string `json:"to_unit_id,omitempty"`
	AssigneeCode string `json:"assignee_code,omitempty"`
	Content      string `json:"content,omitempty"`
}

type letterLogOut struct {
	Items []letterLogEntryOut `json:"items"`
}

func logEntryOut(e domain.LetterLogEntry) letterLogEntryOut {
	return letterLogEntryOut{
		ID: e.ID, LetterID: e.LetterID, At: lucRa(e.At), ActorCode: e.ActorCode, Kind: string(e.Kind),
		FromStatus: string(e.FromStatus), ToStatus: string(e.ToStatus),
		FromUnitID: e.FromUnitID, ToUnitID: e.ToUnitID, AssigneeCode: e.AssigneeCode, Content: e.Content,
	}
}

// duplicateCandidateOut is one candidate of the warning. NO sender field at all, and NO letter type.
// A denunciation is never a candidate. `summary` is null when the letter being booked is a denunciation.
type duplicateCandidateOut struct {
	ID           string  `json:"id"`
	Number       int     `json:"number"`
	Year         int     `json:"year"`
	ReceivedDate string  `json:"received_date"`
	Similarity   float64 `json:"similarity"` // 0..1
	Summary      *string `json:"summary"`
}

type duplicatesOut struct {
	Items []duplicateCandidateOut `json:"items"`
}

// letterReportOut is the Báo cáo tab. Definitions: domain.LetterReport. The Go field is `PastDue` and
// the wire name `overdue`: a count DERIVED on read, never stored (rule 10, invariant 3).
//
// ⚠ NO UNIT NAMES: the incoming register resolves none either, and the web already holds the org chart
// (GET /api/v1/org-units). `unit_id` null = held by no unit.
type letterReportOut struct {
	Year               int                    `json:"year"`
	Received           int                    `json:"received"`
	Resolved           int                    `json:"resolved"`
	ClosedInProcessing int                    `json:"closed_in_processing"`
	InProgress         int                    `json:"in_progress"`
	PastDue            int                    `json:"overdue"`
	OnTimePercent      *float64               `json:"on_time_percent"`
	AverageDays        *float64               `json:"average_days"`
	ByType             []letterReportTypeOut  `json:"by_type"`
	ByUnit             []letterReportUnitOut  `json:"by_unit"`
	ByMonth            []letterReportMonthOut `json:"by_month"`
}

type letterReportTypeOut struct {
	LetterType string `json:"letter_type"`
	Total      int    `json:"total"`
	Resolved   int    `json:"resolved"`
	InProgress int    `json:"in_progress"`
	PastDue    int    `json:"overdue"`
}

type letterReportUnitOut struct {
	UnitID        *string  `json:"unit_id"`
	Total         int      `json:"total"`
	InProgress    int      `json:"in_progress"`
	Resolved      int      `json:"resolved"`
	PastDue       int      `json:"overdue"`
	OnTimePercent *float64 `json:"on_time_percent"`
}

type letterReportMonthOut struct {
	Month    int `json:"month"`
	Received int `json:"received"`
	Resolved int `json:"resolved"`
}

func reportOut(r domain.LetterReport) letterReportOut {
	out := letterReportOut{
		Year: r.Year, Received: r.Received, Resolved: r.Resolved, ClosedInProcessing: r.ClosedInProcessing,
		InProgress: r.InProgress, PastDue: r.PastDue, OnTimePercent: r.OnTimePercent, AverageDays: r.AverageDays,
		ByType:  make([]letterReportTypeOut, 0, len(r.ByType)),
		ByUnit:  make([]letterReportUnitOut, 0, len(r.ByUnit)),
		ByMonth: make([]letterReportMonthOut, 0, len(r.ByMonth)),
	}
	for _, t := range r.ByType {
		out.ByType = append(out.ByType, letterReportTypeOut{LetterType: string(t.Type), Total: t.Total,
			Resolved: t.Resolved, InProgress: t.InProgress, PastDue: t.PastDue})
	}
	for _, u := range r.ByUnit {
		out.ByUnit = append(out.ByUnit, letterReportUnitOut{UnitID: strPtr(u.UnitID), Total: u.Total,
			InProgress: u.InProgress, Resolved: u.Resolved, PastDue: u.PastDue, OnTimePercent: u.OnTimePercent})
	}
	for _, m := range r.ByMonth {
		out.ByMonth = append(out.ByMonth, letterReportMonthOut{Month: m.Month, Received: m.Received, Resolved: m.Resolved})
	}
	return out
}

// --- input ------------------------------------------------------------------------------------------

// bookLetterIn is the body of POST /api/v1/citizen-letters. `number`, `status` and the two due fields
// are declared ONLY to be refused (400) — vanBanDenRa's argument: a clerk must learn these are not
// theirs to set rather than watch them vanish. No `created_by`: the author is the session.
type bookLetterIn struct {
	ReceivedDate    string `json:"received_date"` // YYYY-MM-DD
	LetterType      string `json:"letter_type"`
	SenderName      string `json:"sender_name,omitempty"`
	SenderPhone     string `json:"sender_phone,omitempty"`
	SenderAddress   string `json:"sender_address,omitempty"`
	Summary         string `json:"summary"`
	RelatedLetterID string `json:"related_letter_id,omitempty"` // the duplicate the clerk CONFIRMED
	HoldingUnitID   string `json:"holding_unit_id,omitempty"`   // "Chuyển ngay cho bộ phận"

	Number          *int    `json:"number,omitempty"`
	Status          *string `json:"status,omitempty"`
	ProcessingDueAt *string `json:"processing_due_at,omitempty"`
	ResolutionDueAt *string `json:"resolution_due_at,omitempty"`
}

// duplicateCheckIn is the body of POST /api/v1/citizen-letters/duplicates. A POST BODY, NEVER A QUERY
// STRING: a sender's name in a URL lands in every access log, proxy and browser history (rule 3,
// forbidden #4; the prototype's `?sender_name=` is deliberately not copied — ADR 0078 consequence 1).
type duplicateCheckIn struct {
	SenderName string `json:"sender_name,omitempty"`
	Summary    string `json:"summary"`
	LetterType string `json:"letter_type,omitempty"` // when `to-cao`, no candidate summary is returned
}

type letterRoutingIn struct {
	ToUnit   string `json:"to_unit"`
	Assignee string `json:"assignee,omitempty"`
	Reason   string `json:"reason"`
}

type letterStatusIn struct {
	Status string `json:"status"`
	Note   string `json:"note,omitempty"`
}

type letterResultIn struct {
	ResultDocumentNo   string `json:"result_document_no"`
	ResultDocumentDate string `json:"result_document_date"` // YYYY-MM-DD
	ResultSigner       string `json:"result_signer"`
	ResultIssuer       string `json:"result_issuer"`
	ResultSummary      string `json:"result_summary"`
}

// senderCorrectionIn documents PATCH /api/v1/citizen-letters/{id}/sender. A field ABSENT is left
// alone; a field present as null or "" is CLEARED. Parsed by key presence (readSenderCorrection),
// because a *string cannot tell absent from null.
type senderCorrectionIn struct {
	SenderName    *string `json:"sender_name,omitempty"`
	SenderPhone   *string `json:"sender_phone,omitempty"`
	SenderAddress *string `json:"sender_address,omitempty"`
}

type letterNoteIn struct {
	Content string `json:"content"`
}

// --- handlers ---------------------------------------------------------------------------------------

// letterCaller builds the acting member of staff: the audit actor from Principal.Ma (nguoiThucHien —
// never the internal id, rule 6 invariant 8) and whether the principal holds `petition.create` IN THIS
// COMMUNE, asked of the same checker the route guard used (rule 5, invariant 3).
func (h *Handler) letterCaller(r *http.Request) (app.LetterCaller, bool) {
	actor, ok := nguoiThucHien(r)
	if !ok {
		return app.LetterCaller{}, false
	}
	p, _ := authz.From(r.Context())
	return app.LetterCaller{Actor: actor, CanBook: h.d.Checker.Allows(r.Context(), p, "petition.create")}, true
}

// BookCitizenLetter — POST /api/v1/citizen-letters
func (h *Handler) BookCitizenLetter(w http.ResponseWriter, r *http.Request) {
	var in bookLetterIn
	if !docThan(w, r, &in) {
		return
	}
	switch {
	case in.Number != nil:
		h.letterError(w, r, "vào sổ", domain.ErrLetterNumberFromClient)
		return
	case in.Status != nil:
		h.letterError(w, r, "vào sổ", domain.ErrLetterStatusFromClient)
		return
	case in.ProcessingDueAt != nil || in.ResolutionDueAt != nil:
		h.letterError(w, r, "vào sổ", domain.ErrLetterDueFromClient)
		return
	}
	received, ok := ngayVao(in.ReceivedDate)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"`received_date` phải theo dạng YYYY-MM-DD, ví dụ 2026-10-07.", "")
		return
	}
	caller, ok := h.letterCaller(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	l, err := h.d.CitizenLetters.Book(r.Context(), app.BookLetterRequest{
		ReceivedDate: received, Type: domain.LetterType(in.LetterType),
		SenderName: in.SenderName, SenderPhone: in.SenderPhone, SenderAddress: in.SenderAddress,
		Summary: in.Summary, RelatedLetterID: in.RelatedLetterID, HoldingUnitID: in.HoldingUnitID,
	}, caller)
	if err != nil {
		h.letterError(w, r, "vào sổ", err)
		return
	}
	// What a retry with the same Idempotency-Key is told about: the id, never the body (Redis is a
	// cache, not a record store — and the body holds personal data).
	idem.RecordCode(r.Context(), l.ID)
	vietJSON(w, http.StatusCreated, letterOut(l, domain.DetailDisclosure(l, caller.Actor.ID, caller.CanBook), h.now()))
}

// CitizenLetterDuplicates — POST /api/v1/citizen-letters/duplicates
func (h *Handler) CitizenLetterDuplicates(w http.ResponseWriter, r *http.Request) {
	var in duplicateCheckIn
	if !docThan(w, r, &in) {
		return
	}
	inputType := domain.LetterType(in.LetterType)
	if in.LetterType != "" && !inputType.Valid() {
		h.letterError(w, r, "kiểm trùng", domain.ErrLetterTypeInvalid)
		return
	}
	found, err := h.d.CitizenLetters.Duplicates(r.Context(), app.DuplicateQuery{SenderName: in.SenderName, Summary: in.Summary})
	if err != nil {
		h.letterError(w, r, "kiểm trùng", err)
		return
	}
	out := duplicatesOut{Items: make([]duplicateCandidateOut, 0, len(found))}
	for _, c := range found {
		// A DENUNCIATION IS NEVER LISTED — its presence alone would say the named sender filed one
		// (ADR 0078 #4). The store and domain.RankDuplicates exclude it already; this is the last wall.
		if c.Letter.Type.ProtectsIdentity() {
			continue
		}
		item := duplicateCandidateOut{ID: c.Letter.ID, Number: c.Letter.Number, Year: c.Letter.Year,
			ReceivedDate: ngayRa(c.Letter.ReceivedDate), Similarity: c.Similarity}
		// No summary when the letter being booked is a denunciation (ADR 0078 #4).
		if !inputType.ProtectsIdentity() {
			item.Summary = strPtr(c.Letter.Summary)
		}
		out.Items = append(out.Items, item)
	}
	vietJSON(w, http.StatusOK, out)
}

// ListCitizenLetters — GET /api/v1/citizen-letters
func (h *Handler) ListCitizenLetters(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	req, err := page.Parse(q, docstore.SortCitizenLetters)
	if err != nil {
		status, code, msg := page.HTTPError(err)
		httpx.WriteError(w, status, code, msg, "")
		return
	}
	f, scope, err := letterFilterFromQuery(q)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
		return
	}
	// THE CALLER'S CODE FROM THE SESSION, never from a parameter (rule 4, invariant 2 in staff form).
	p, _ := authz.From(r.Context())
	// tenant_id: the context's, from Host; the store binds it through store.DB.For(ctx).
	res, err := h.d.CitizenLetters.List(r.Context(), app.LetterListQuery{Filter: f, Scope: scope, CallerCode: p.Ma}, req)
	if err != nil {
		h.letterError(w, r, "đọc sổ", err)
		return
	}
	now := h.now()
	out := page.Result[citizenLetterItemOut]{
		Items:      make([]citizenLetterItemOut, 0, len(res.Items)),
		NextCursor: res.NextCursor,
		HasMore:    res.HasMore,
	}
	for _, l := range res.Items {
		out.Items = append(out.Items, letterItemOut(l, now))
	}
	vietJSON(w, http.StatusOK, out)
}

// CitizenLetterDetail — GET /api/v1/citizen-letters/{id}
func (h *Handler) CitizenLetterDetail(w http.ResponseWriter, r *http.Request) {
	caller, ok := h.letterCaller(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	l, show, err := h.d.CitizenLetters.Detail(r.Context(), r.PathValue("id"), caller)
	if err != nil {
		h.letterError(w, r, "đọc chi tiết", err)
		return
	}
	vietJSON(w, http.StatusOK, letterOut(l, show, h.now()))
}

// CitizenLetterLog — GET /api/v1/citizen-letters/{id}/log
func (h *Handler) CitizenLetterLog(w http.ResponseWriter, r *http.Request) {
	entries, err := h.d.CitizenLetters.Log(r.Context(), r.PathValue("id"))
	if err != nil {
		h.letterError(w, r, "đọc nhật ký", err)
		return
	}
	out := letterLogOut{Items: make([]letterLogEntryOut, 0, len(entries))}
	for _, e := range entries {
		out.Items = append(out.Items, logEntryOut(e))
	}
	vietJSON(w, http.StatusOK, out)
}

// RouteCitizenLetter — POST /api/v1/citizen-letters/{id}/routings
func (h *Handler) RouteCitizenLetter(w http.ResponseWriter, r *http.Request) {
	var in letterRoutingIn
	if !docThan(w, r, &in) {
		return
	}
	caller, ok := h.letterCaller(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	l, err := h.d.CitizenLetters.Route(r.Context(), r.PathValue("id"),
		app.RouteLetterRequest{ToUnitID: in.ToUnit, AssigneeCode: in.Assignee, Reason: in.Reason}, caller)
	if err != nil {
		h.letterError(w, r, "chuyển", err)
		return
	}
	vietJSON(w, http.StatusOK, letterOut(l, domain.DetailDisclosure(l, caller.Actor.ID, caller.CanBook), h.now()))
}

// MoveCitizenLetter — POST /api/v1/citizen-letters/{id}/status
func (h *Handler) MoveCitizenLetter(w http.ResponseWriter, r *http.Request) {
	var in letterStatusIn
	if !docThan(w, r, &in) {
		return
	}
	caller, ok := h.letterCaller(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	l, err := h.d.CitizenLetters.Move(r.Context(), r.PathValue("id"),
		app.MoveLetterRequest{Status: domain.LetterStatus(in.Status), Note: in.Note}, caller)
	if err != nil {
		h.letterError(w, r, "đổi trạng thái", err)
		return
	}
	vietJSON(w, http.StatusOK, letterOut(l, domain.DetailDisclosure(l, caller.Actor.ID, caller.CanBook), h.now()))
}

// RecordCitizenLetterResult — PUT /api/v1/citizen-letters/{id}/result
func (h *Handler) RecordCitizenLetterResult(w http.ResponseWriter, r *http.Request) {
	var in letterResultIn
	if !docThan(w, r, &in) {
		return
	}
	date, ok := ngayVao(in.ResultDocumentDate)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"`result_document_date` phải theo dạng YYYY-MM-DD, ví dụ 2026-10-07.", "")
		return
	}
	caller, ok := h.letterCaller(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	l, err := h.d.CitizenLetters.RecordResult(r.Context(), r.PathValue("id"), app.LetterResultRequest{
		DocumentNo: in.ResultDocumentNo, DocumentDate: date, Signer: in.ResultSigner,
		Issuer: in.ResultIssuer, Summary: in.ResultSummary,
	}, caller)
	if err != nil {
		h.letterError(w, r, "ghi kết quả", err)
		return
	}
	vietJSON(w, http.StatusOK, letterOut(l, domain.DetailDisclosure(l, caller.Actor.ID, caller.CanBook), h.now()))
}

// CorrectCitizenLetterSender — PATCH /api/v1/citizen-letters/{id}/sender
func (h *Handler) CorrectCitizenLetterSender(w http.ResponseWriter, r *http.Request) {
	corr, ok := readSenderCorrection(w, r)
	if !ok {
		return
	}
	caller, ok := h.letterCaller(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	l, err := h.d.CitizenLetters.CorrectSender(r.Context(), r.PathValue("id"), corr, caller)
	if err != nil {
		h.letterError(w, r, "sửa người gửi", err)
		return
	}
	vietJSON(w, http.StatusOK, letterOut(l, domain.DetailDisclosure(l, caller.Actor.ID, caller.CanBook), h.now()))
}

// AddCitizenLetterNote — POST /api/v1/citizen-letters/{id}/log-entries
func (h *Handler) AddCitizenLetterNote(w http.ResponseWriter, r *http.Request) {
	var in letterNoteIn
	if !docThan(w, r, &in) {
		return
	}
	caller, ok := h.letterCaller(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	e, err := h.d.CitizenLetters.AddNote(r.Context(), r.PathValue("id"), in.Content, caller)
	if err != nil {
		h.letterError(w, r, "ghi nhật ký", err)
		return
	}
	idem.RecordCode(r.Context(), e.ID)
	vietJSON(w, http.StatusCreated, logEntryOut(e))
}

// CitizenLetterReport — GET /api/v1/citizen-letter-report?year=
func (h *Handler) CitizenLetterReport(w http.ResponseWriter, r *http.Request) {
	// `year` IS REQUIRED: a report silently defaulting to "this year" is a figure a person may file
	// under the wrong year without noticing which one they were shown.
	year, err := strconv.Atoi(r.URL.Query().Get("year"))
	if err != nil || year < 2000 || year > 2200 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", errNamKhongHopLe.Error(), "")
		return
	}
	rep, err := h.d.CitizenLetters.Report(r.Context(), year)
	if err != nil {
		h.letterError(w, r, "lập báo cáo", err)
		return
	}
	vietJSON(w, http.StatusOK, reportOut(rep))
}

// --- parsing ----------------------------------------------------------------------------------------

var (
	errLetterStatusFilter = errors.New("`status` không phải một trạng thái của sổ đơn thư")
	errLetterTypeFilter   = errors.New("`letter_type` phải là một trong kien-nghi-phan-anh, khieu-nai, to-cao, de-nghi")
	errLetterScope        = errors.New("`scope` phải là all, mine hoặc related")
	errLetterDateFilter   = errors.New("`received_from` / `received_to` phải theo dạng YYYY-MM-DD và from không sau to")
	errLetterCodeFilter   = errors.New("`holding_unit` / `assignee` quá dài")
)

// letterFilterFromQuery validates the list filters. An unknown value is REFUSED, not ignored: a filter
// silently dropped returns the whole register to a screen that asked for a slice of it. Nothing here
// reads a commune or a person from the query (rule 1 forbidden #2; rule 4 invariant 2).
func letterFilterFromQuery(q url.Values) (docstore.CitizenLetterFilter, string, error) {
	var f docstore.CitizenLetterFilter
	if s := q.Get("year"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 2000 || n > 2200 {
			return f, "", errNamKhongHopLe
		}
		f.Year = n
	}
	if s := q.Get("status"); s != "" {
		if !domain.LetterStatus(s).Valid() {
			return f, "", errLetterStatusFilter
		}
		f.Status = domain.LetterStatus(s)
	}
	if s := q.Get("letter_type"); s != "" {
		if !domain.LetterType(s).Valid() {
			return f, "", errLetterTypeFilter
		}
		f.Type = domain.LetterType(s)
	}
	f.HoldingUnitID = strings.TrimSpace(q.Get("holding_unit"))
	f.AssigneeCode = strings.TrimSpace(q.Get("assignee"))
	if utf8.RuneCountInString(f.HoldingUnitID) > domain.MaxLetterUnitID || utf8.RuneCountInString(f.AssigneeCode) > domain.MaxStaffCode {
		return f, "", errLetterCodeFilter
	}
	from, okFrom := ngayVao(q.Get("received_from"))
	to, okTo := ngayVao(q.Get("received_to"))
	if !okFrom || !okTo || (!from.IsZero() && !to.IsZero() && from.After(to)) {
		return f, "", errLetterDateFilter
	}
	f.ReceivedFrom, f.ReceivedTo = from, to
	f.Search = strings.TrimSpace(q.Get("q"))
	if utf8.RuneCountInString(f.Search) > domain.MaxLetterSearch {
		return f, "", errTimQuaDai
	}
	scope := q.Get("scope")
	switch scope {
	case "", "all", "mine", "related":
	default:
		return f, "", errLetterScope
	}
	return f, scope, nil
}

// readSenderCorrection reads the PATCH body by KEY PRESENCE: absent = unchanged, null or "" = clear.
// A key outside the three is refused, so a client cannot believe it corrected the summary here.
func readSenderCorrection(w http.ResponseWriter, r *http.Request) (app.SenderCorrection, bool) {
	var corr app.SenderCorrection
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, thanToiDa))
	var raw map[string]json.RawMessage
	if err == nil {
		err = json.Unmarshal(body, &raw)
	}
	if err != nil || raw == nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Nội dung gửi lên không phải JSON hợp lệ hoặc quá lớn.", "")
		return corr, false
	}
	for key, val := range raw {
		var target **string
		switch key {
		case "sender_name":
			target = &corr.Name
		case "sender_phone":
			target = &corr.Phone
		case "sender_address":
			target = &corr.Address
		default:
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
				"Chỉ sửa được sender_name, sender_phone và sender_address ở đây.", "")
			return corr, false
		}
		s := ""
		if !bytes.Equal(bytes.TrimSpace(val), []byte("null")) {
			if err := json.Unmarshal(val, &s); err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
					"Mỗi trường người gửi phải là chuỗi hoặc null.", "")
				return corr, false
			}
		}
		*target = &s
	}
	return corr, true
}

// --- errors -----------------------------------------------------------------------------------------

// letterError maps one failure onto ONE status and ONE sentence. A refusal returns the domain's own
// sentence (LetterError.Msg — never err.Error() of the chain, which carries the commune id); anything
// unrecognised is a 500 whose cause is logged with the commune and nothing about the letter (rule 3).
func (h *Handler) letterError(w http.ResponseWriter, r *http.Request, op string, err error) {
	var (
		le *domain.LetterError
		te *domain.LetterTransitionError
	)
	switch {
	case errors.Is(err, docstore.ErrCitizenLetterNotFound):
		// One sentence for "never existed", "removed" and "another commune's" (rule 1).
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy đơn thư này.", "")
	case errors.As(err, &te):
		httpx.WriteError(w, http.StatusConflict, "letter_state", te.Error(), "")
	case errors.As(err, &le):
		switch le.Refusal {
		case domain.RefusalNotPermitted:
			httpx.WriteError(w, http.StatusForbidden, "forbidden", le.Msg, "")
		case domain.RefusalState:
			httpx.WriteError(w, http.StatusConflict, "letter_state", le.Msg, "")
		default:
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", le.Msg, "")
		}
	case errors.Is(err, docstore.ErrDaySoDayTran):
		httpx.WriteError(w, http.StatusConflict, "so_don_thu_day",
			"Dãy số của sổ đơn thư năm nay đã đạt mức tối đa. Hãy báo quản trị hệ thống.", "")
	case errors.Is(err, app.ErrLetterDirectoryUnavailable):
		httpx.WriteError(w, http.StatusServiceUnavailable, "directory_unavailable",
			"Chưa kiểm được bộ phận / cán bộ với danh bạ của xã. Vui lòng thử lại sau ít phút.", "")
	default:
		h.d.Log.Error("đơn thư: "+op+" lỗi hệ thống",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}
