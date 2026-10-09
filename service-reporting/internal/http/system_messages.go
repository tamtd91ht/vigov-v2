package http

// "Lời hệ thống" — the reporting half (14-cau-hinh §7, ADR 0024 §Phụ, *Bổ sung 29/09/2026*,
// migration 0003). A copy of service-finance/internal/http/system_messages.go, never an import
// (rule 2).
//
// FOUR ROUTES: list, reword, restore, and "Tắt / Bật lại" (PATCH …/override, ADR 0079 Q2, migration
// 0004). NO COMMUNE SENTENCES HERE: ADR 0079 Q5a keeps "Báo cáo điều hành" a group of its own with
// none, so there is no create and no delete of a key (a shipped key is only ever reworded back).

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-reporting/internal/domain"
)

// SystemMessageService is the use case, declared at the point of use so the routes' refusals are
// testable without PostgreSQL. *app.SystemMessages satisfies it; every write opens one transaction
// carrying the row and its audit entry.
type SystemMessageService interface {
	Messages(ctx context.Context) ([]domain.SystemMessage, error)
	Reword(ctx context.Context, key, text string, actor audit.Actor) (domain.SystemMessage, error)
	Restore(ctx context.Context, key string, actor audit.Actor) (domain.SystemMessage, error)
	SetActive(ctx context.Context, key string, active bool, actor audit.Actor) (domain.SystemMessage, error)
}

// systemMessageOut is one card of the tab — the SAME shape finance and petitions answer, so the
// web-admin tab reads all three services with one type.
//
// THE KEY TRAVELS AS `code`, NOT `key`: tools/apidoc's credential guard refuses a response field
// whose name holds the word `key`.
//
// `default_text` IS SENT BESIDE `current_text` so the screen can show what "Khôi phục câu mặc
// định" would bring back without a second request. `updated_at` / `updated_by` are absent while
// the commune is on the default — nobody changed anything, so there is nobody to name.
//
// SINCE ADR 0079 Q2 (owner answers of 08/10/2026):
//
//	group_code     `bao-cao` — the web's group code, which section of the tab the card is listed under
//	origin         always `shipped` here (no commune sentences in this group, ADR 0079 Q5a); kept so the
//	               three services answer one shape
//	is_active      false for a switched-off sentence — any shipped key since 09/10/2026, reworded or
//	               not (then overridden false, current_text the default), or a commune sentence
//	override_text  the commune's stored wording of a shipped key, present while overridden, ALSO while
//	               switched off (then current_text is the default) — what "Bật lại" brings back
type systemMessageOut struct {
	Code         string     `json:"code"`
	GroupCode    string     `json:"group_code"`
	Origin       string     `json:"origin"`
	Description  string     `json:"description"`
	DefaultText  string     `json:"default_text,omitempty"`
	CurrentText  string     `json:"current_text"`
	OverrideText string     `json:"override_text,omitempty"`
	Overridden   bool       `json:"overridden"`
	IsActive     bool       `json:"is_active"`
	UpdatedAt    *time.Time `json:"updated_at,omitempty"`
	UpdatedBy    string     `json:"updated_by,omitempty"` // staff business code (rule 6, inv 8)
}

// systemMessageListOut wraps the list in an object so it can grow. No cursor: the list is the
// catalogue, bounded by the number of shipped keys (38).
type systemMessageListOut struct {
	Items []systemMessageOut `json:"items"`
}

// rewordSystemMessageIn is the PUT body. `text` is REQUIRED; empty answers 400 and names the
// restore route — an empty wording is not a state this screen offers.
type rewordSystemMessageIn struct {
	Text string `json:"text"`
}

// switchSystemMessageIn is the PATCH …/override body: the switch, and nothing else. REQUIRED — a
// PATCH that names no state is refused rather than read as "on".
type switchSystemMessageIn struct {
	IsActive *bool `json:"is_active"`
}

func systemMessageToOut(m domain.SystemMessage) systemMessageOut {
	return systemMessageOut{
		Code: m.Key, GroupCode: m.Group, Origin: m.Origin, Description: m.Description,
		DefaultText: m.DefaultText, CurrentText: m.CurrentText, OverrideText: m.OverrideText,
		Overridden: m.Overridden, IsActive: m.Active,
		UpdatedAt: m.UpdatedAt, UpdatedBy: m.UpdatedBy,
	}
}

// ListSystemMessages serves GET /api/v1/reporting-system-messages.
func (h *Handler) ListSystemMessages(w http.ResponseWriter, r *http.Request) {
	all, err := h.d.SystemMessages.Messages(r.Context())
	if err != nil {
		h.writeSystemMessageError(w, r, "đọc", err)
		return
	}
	out := systemMessageListOut{Items: make([]systemMessageOut, 0, len(all))}
	for _, m := range all {
		out.Items = append(out.Items, systemMessageToOut(m))
	}
	writeJSON(w, http.StatusOK, out)
}

// RewordSystemMessage serves PUT /api/v1/reporting-system-messages/{code}/override.
func (h *Handler) RewordSystemMessage(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r)
	if !ok {
		h.writeSystemMessageError(w, r, "sửa", errNoMessageActor)
		return
	}
	var in rewordSystemMessageIn
	if !decodeBody(w, r, &in) {
		return
	}
	m, err := h.d.SystemMessages.Reword(r.Context(), r.PathValue("code"), in.Text, actor)
	if err != nil {
		h.writeSystemMessageError(w, r, "sửa", err)
		return
	}
	writeJSON(w, http.StatusOK, systemMessageToOut(m))
}

// RestoreSystemMessage serves DELETE /api/v1/reporting-system-messages/{code}/override.
func (h *Handler) RestoreSystemMessage(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r)
	if !ok {
		h.writeSystemMessageError(w, r, "khôi phục", errNoMessageActor)
		return
	}
	if _, err := h.d.SystemMessages.Restore(r.Context(), r.PathValue("code"), actor); err != nil {
		h.writeSystemMessageError(w, r, "khôi phục", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SwitchSystemMessage serves PATCH /api/v1/reporting-system-messages/{code}/override — "Tắt / Bật lại"
// of the commune's wording of a shipped key.
func (h *Handler) SwitchSystemMessage(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r)
	if !ok {
		h.writeSystemMessageError(w, r, "tắt/bật", errNoMessageActor)
		return
	}
	var in switchSystemMessageIn
	if !decodeBody(w, r, &in) {
		return
	}
	if in.IsActive == nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Thiếu trường is_active (true = bật, false = tắt).", "")
		return
	}
	m, err := h.d.SystemMessages.SetActive(r.Context(), r.PathValue("code"), *in.IsActive, actor)
	if err != nil {
		h.writeSystemMessageError(w, r, "tắt/bật", err)
		return
	}
	writeJSON(w, http.StatusOK, systemMessageToOut(m))
}

// errNoMessageActor — the principal carries no business code. 500 and not a fallback to the internal
// id: rule 6, invariant 8 refuses a write whose trail cannot name who made it.
var errNoMessageActor = errors.New("system_message: principal has no business code")

func (h *Handler) writeSystemMessageError(w http.ResponseWriter, r *http.Request, op string, err error) {
	switch {
	case errors.Is(err, domain.ErrUnknownMessageKey):
		// Same body for every unknown key, including another service's (`budget.scope_notice`,
		// `feedback.*`): the key is not this service's to reword.
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không có câu hệ thống mang mã này ở phân hệ Báo cáo.", "")
	case domain.IsMessageInputError(err):
		// The domain's own sentence: it names the rule and holds no personal data. The use case
		// returns input refusals UNWRAPPED (they happen before any transaction), so no commune id or
		// operation leaks into the message.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
	default:
		h.d.Log.Error("lời hệ thống: "+op+" lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}
