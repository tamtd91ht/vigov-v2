package http

// Wave 2 of the operator area (ADR 0073): the upload limits (#5, ADR 0052 §10) and the operator log
// (#2). Both read and write ONLY this service's own tables — upload_policy, platform_audit_log and
// audit_log — and both are mounted on the operator edge alone (operator_routes.go).

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"golang.org/x/text/unicode/norm"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/service-platform/internal/app"
	"github.com/vihat/vigov/service-platform/internal/domain"
	"github.com/vihat/vigov/service-platform/internal/store"
)

// UploadPolicyEditor is the upload_policy store (*store.UploadPolicyStore).
type UploadPolicyEditor interface {
	ListUploadPolicies(ctx context.Context) ([]domain.UploadPolicy, error)
	ChangeUploadPolicy(ctx context.Context, next domain.UploadPolicy, reason string, by domain.OperatorActor) (domain.UploadPolicy, bool, error)
}

// OperatorLogReader is the operator log (*store.OperatorLog). It trails every page it returns.
type OperatorLogReader interface {
	Read(ctx context.Context, q store.OperatorLogQuery, reader domain.OperatorActor) (page.Result[domain.OperatorLogEntry], error)
}

// --- upload policies --------------------------------------------------------------------------------

// uploadPolicyView is one purpose's limits, plus what the console may offer for it (app.ChoicesFor):
// the types and the byte cap this service would accept on a PUT. Empty choices = a purpose this
// build does not class, which no PUT can change.
type uploadPolicyView struct {
	Purpose          string   `json:"purpose"`
	MaxBytes         int64    `json:"max_bytes"`
	AllowedMIMETypes []string `json:"allowed_mime_types"`
	// MaxFilesPerSubject null = Vihat deliberately set NO count limit (migration 0008), never "zero".
	MaxFilesPerSubject *int32    `json:"max_files_per_subject"`
	UpdatedAt          time.Time `json:"updated_at"`
	UpdatedBy          string    `json:"updated_by"`
	MIMEChoices        []string  `json:"mime_choices"`
	MaxBytesCap        int64     `json:"max_bytes_cap"`
}

type uploadPolicyListView struct {
	Items []uploadPolicyView `json:"items"`
}

// countInput tells an ABSENT max_files_per_subject (400 — nobody removes a count limit by leaving a
// field out) from an explicit null (no count limit, a deliberate choice) and from a number.
type countInput struct {
	set, limited bool
	n            int32
}

func (c *countInput) UnmarshalJSON(b []byte) error {
	c.set = true
	if string(b) == "null" {
		return nil
	}
	if err := json.Unmarshal(b, &c.n); err != nil {
		return errors.New("max_files_per_subject: not an integer") // never quotes the input
	}
	c.limited = true
	return nil
}

type uploadPolicyBody struct {
	// A pointer so an absent field is a 400, never a silent 0.
	MaxBytes           *int64     `json:"max_bytes"`
	AllowedMIMETypes   []string   `json:"allowed_mime_types"`
	MaxFilesPerSubject countInput `json:"max_files_per_subject"`
	Reason             string     `json:"reason"`
}

func toUploadPolicyView(p domain.UploadPolicy) uploadPolicyView {
	v := uploadPolicyView{Purpose: p.Purpose, MaxBytes: p.MaxBytes, AllowedMIMETypes: p.AllowedMIMETypes,
		UpdatedAt: p.UpdatedAt.UTC(), UpdatedBy: p.UpdatedBy, MIMEChoices: []string{}}
	if v.AllowedMIMETypes == nil {
		v.AllowedMIMETypes = []string{}
	}
	if p.FileCountLimited {
		n := p.MaxFilesPerSubject
		v.MaxFilesPerSubject = &n
	}
	if c, ok := app.ChoicesFor(p.Purpose); ok {
		v.MIMEChoices, v.MaxBytesCap = c.MIMETypes, c.MaxBytesCap
	}
	return v
}

const msgPolicyNotFound = "Không có giới hạn tải lên đang dùng cho mục đích này."

func (h *operatorHandlers) writePolicyError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, app.ErrUnknownPurpose), errors.Is(err, store.ErrUploadPolicyNotFound):
		httpx.WriteError(w, http.StatusNotFound, "upload_policy_not_found", msgPolicyNotFound, "")
	case errors.Is(err, app.ErrMaxBytes):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid_max_bytes",
			"Dung lượng tối đa phải lớn hơn 0 và không vượt mức trần của mục đích này.", "")
	case errors.Is(err, app.ErrMIMETypes):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid_mime_types",
			"Danh sách kiểu tệp không hợp lệ: cần ít nhất một kiểu, không lặp, và chỉ các kiểu mục đích này xử lý được.", "")
	case errors.Is(err, app.ErrMaxFilesInvalid):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid_max_files",
			"Số tệp tối đa phải từ 1 đến 100, hoặc để trống (null) nếu không giới hạn.", "")
	case errors.Is(err, domain.ErrReasonInvalid):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid_reason", "Cần ghi lý do (tối đa 500 ký tự).", "")
	default:
		h.d.Log.ErrorContext(r.Context(), "khu vận hành: đọc/ghi giới hạn tải lên thất bại", "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", msgInternal, "")
	}
}

func (h *operatorHandlers) listUploadPolicies(w http.ResponseWriter, r *http.Request) {
	ps, err := h.d.Policies.ListUploadPolicies(r.Context())
	if err != nil {
		h.writePolicyError(w, r, err)
		return
	}
	out := uploadPolicyListView{Items: []uploadPolicyView{}}
	for _, p := range ps {
		out.Items = append(out.Items, toUploadPolicyView(p))
	}
	writeJSON(w, http.StatusOK, out)
}

// changeUploadPolicy sets one purpose's limits, platform-wide. Live in this database at once; every
// service reading the limits (core/platformclient/uploadpolicy) sees it within its 60-second TTL.
func (h *operatorHandlers) changeUploadPolicy(w http.ResponseWriter, r *http.Request) {
	purpose := r.PathValue("purpose")
	if _, ok := app.ChoicesFor(purpose); !ok {
		h.writePolicyError(w, r, app.ErrUnknownPurpose)
		return
	}
	var b uploadPolicyBody
	if !decodeBody(w, r, &b) {
		return
	}
	if b.MaxBytes == nil || !b.MaxFilesPerSubject.set {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", msgInvalidBody, "")
		return
	}
	next := domain.UploadPolicy{Purpose: purpose, MaxBytes: *b.MaxBytes, AllowedMIMETypes: b.AllowedMIMETypes,
		FileCountLimited: b.MaxFilesPerSubject.limited, MaxFilesPerSubject: b.MaxFilesPerSubject.n}
	if err := app.ValidateUploadPolicy(next); err != nil {
		h.writePolicyError(w, r, err)
		return
	}
	reason, err := domain.ValidateReason(norm.NFC.String(b.Reason))
	if err != nil {
		h.writePolicyError(w, r, err)
		return
	}
	out, _, err := h.d.Policies.ChangeUploadPolicy(r.Context(), next, reason, actorOf(r))
	if err != nil {
		h.writePolicyError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toUploadPolicyView(out))
}

// --- the operator log -------------------------------------------------------------------------------

type operatorAuditCommuneView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// operatorAuditEntryView is one act on the operator log. Before / After are the JSON objects the
// trail stored (typed `any` like core/audit.EntryView: the shape differs per action).
type operatorAuditEntryView struct {
	At     time.Time `json:"at"`
	Actor  string    `json:"actor"`
	Action string    `json:"action"`
	// Commune is null for a platform-wide change (an upload limit).
	Commune *operatorAuditCommuneView `json:"commune"`
	Subject string                    `json:"subject"`
	Before  any                       `json:"before"`
	After   any                       `json:"after"`
	Reason  string                    `json:"reason"`
}

// operatorAuditPageView — no total, by core/page's rule.
type operatorAuditPageView struct {
	Items      []operatorAuditEntryView `json:"items"`
	NextCursor string                   `json:"next_cursor"`
	HasMore    bool                     `json:"has_more"`
}

func rawOrNil(b []byte) any {
	if b == nil {
		return nil
	}
	return json.RawMessage(b)
}

// parseLogQuery reads from / to (RFC 3339, half-open) and the page. No sort, no commune: the commune
// filter is the PATH of the per-commune route, never a query parameter (rule 1 forbidden #2's spirit —
// a scope is never a client string beside the request).
func parseLogQuery(w http.ResponseWriter, r *http.Request) (store.OperatorLogQuery, bool) {
	qs := r.URL.Query()
	var q store.OperatorLogQuery
	for _, f := range []struct {
		name string
		dst  *time.Time
	}{{"from", &q.From}, {"to", &q.To}} {
		if v := qs.Get(f.name); v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_range",
					"Khoảng thời gian không hợp lệ: from và to theo RFC 3339, from phải trước to.", "")
				return q, false
			}
			*f.dst = t
		}
	}
	if !q.From.IsZero() && !q.To.IsZero() && !q.From.Before(q.To) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_range",
			"Khoảng thời gian không hợp lệ: from và to theo RFC 3339, from phải trước to.", "")
		return q, false
	}
	req, err := page.New(store.OperatorLogOrder, "", "", qs.Get(page.ParamLimit), qs.Get(page.ParamCursor))
	if err != nil {
		status, code, msg := page.HTTPError(err)
		httpx.WriteError(w, status, code, msg, "")
		return q, false
	}
	q.Page = req
	return q, true
}

func (h *operatorHandlers) respondLog(w http.ResponseWriter, r *http.Request, q store.OperatorLogQuery) {
	res, err := h.d.OperatorLog.Read(r.Context(), q, actorOf(r))
	if err != nil {
		if errors.Is(err, page.ErrCursor) {
			status, code, msg := page.HTTPError(err)
			httpx.WriteError(w, status, code, msg, "")
			return
		}
		h.d.Log.ErrorContext(r.Context(), "khu vận hành: đọc nhật ký vận hành thất bại", "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", msgInternal, "")
		return
	}
	out := operatorAuditPageView{Items: []operatorAuditEntryView{}, NextCursor: res.NextCursor, HasMore: res.HasMore}
	for _, e := range res.Items {
		v := operatorAuditEntryView{At: e.At.UTC(), Actor: e.ActorCode, Action: e.Action, Subject: e.Subject,
			Before: rawOrNil(e.Before), After: rawOrNil(e.After), Reason: e.Reason}
		if e.CommuneID != "" {
			v.Commune = &operatorAuditCommuneView{ID: e.CommuneID, Name: e.CommuneName}
		}
		out.Items = append(out.Items, v)
	}
	writeJSON(w, http.StatusOK, out)
}

// listOperatorLog is every commune's operator acts plus the platform-wide changes.
func (h *operatorHandlers) listOperatorLog(w http.ResponseWriter, r *http.Request) {
	q, ok := parseLogQuery(w, r)
	if !ok {
		return
	}
	h.respondLog(w, r, q)
}

// listCommuneOperatorLog is one commune's operator acts — the commune detail's log. An unknown or
// malformed id is 404, like every other commune route.
func (h *operatorHandlers) listCommuneOperatorLog(w http.ResponseWriter, r *http.Request) {
	_, id, ok := targetCommune(w, r)
	if !ok {
		return
	}
	if _, _, err := h.d.Registry.Commune(r.Context(), id); err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	q, ok := parseLogQuery(w, r)
	if !ok {
		return
	}
	q.CommuneID = id
	h.respondLog(w, r, q)
}
