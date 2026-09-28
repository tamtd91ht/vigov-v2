package http

// "Lời hệ thống" — the finance half (14-cau-hinh §7, ADR 0024 §`loi_he_thong`, migration 0010).
//
// THREE ROUTES, NO FOURTH: list, reword, restore. There is no create (a commune-invented key has no
// code that raises it) and no delete of a shipped key (it cannot be removed — only reworded back).

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// SystemMessageService is the use case, declared at the point of use so the routes' refusals are
// testable without PostgreSQL. *app.SystemMessages satisfies it; every write opens one transaction
// carrying the row and its audit entry.
type SystemMessageService interface {
	Messages(ctx context.Context) ([]domain.SystemMessage, error)
	// Text is the sentence in force for one key — what a route that EMITS the sentence reads
	// (`scope_notice` on the investment-project reads, du_an.go).
	Text(ctx context.Context, key string) (string, error)
	Reword(ctx context.Context, key, text string, actor audit.Actor) (domain.SystemMessage, error)
	Restore(ctx context.Context, key string, actor audit.Actor) (domain.SystemMessage, error)
}

// systemMessageOut is one card of the tab.
//
// `default_text` IS SENT BESIDE `current_text` so the screen can show what "Khôi phục câu mặc
// định" would bring back without a second request. `updated_at` / `updated_by` are absent while
// the commune is on the default — nobody changed anything, so there is nobody to name.
type systemMessageOut struct {
	Key         string     `json:"key"`
	Description string     `json:"description"`
	DefaultText string     `json:"default_text"`
	CurrentText string     `json:"current_text"`
	Overridden  bool       `json:"overridden"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
	UpdatedBy   string     `json:"updated_by,omitempty"` // staff business code (rule 6, inv 8)
}

// systemMessageListOut wraps the list in an object, like every list route here, so it can grow.
// No cursor: the list is the catalogue, bounded by the number of shipped keys.
type systemMessageListOut struct {
	Items []systemMessageOut `json:"items"`
}

// rewordSystemMessageIn is the PUT body. `text` is REQUIRED; empty answers 400 and names the
// restore route — an empty wording is not a state this screen offers.
type rewordSystemMessageIn struct {
	Text string `json:"text"`
}

func systemMessageToOut(m domain.SystemMessage) systemMessageOut {
	return systemMessageOut{
		Key: m.Key, Description: m.Description, DefaultText: m.DefaultText,
		CurrentText: m.CurrentText, Overridden: m.Overridden,
		UpdatedAt: m.UpdatedAt, UpdatedBy: m.UpdatedBy,
	}
}

// ListSystemMessages serves GET /api/v1/finance-system-messages.
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
	vietJSON(w, http.StatusOK, out)
}

// RewordSystemMessage serves PUT /api/v1/finance-system-messages/{key}/override.
func (h *Handler) RewordSystemMessage(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.writeSystemMessageError(w, r, "sửa", errNoActor)
		return
	}
	var in rewordSystemMessageIn
	if !docThan(w, r, &in) {
		return
	}
	m, err := h.d.SystemMessages.Reword(r.Context(), r.PathValue("key"), in.Text, actor)
	if err != nil {
		h.writeSystemMessageError(w, r, "sửa", err)
		return
	}
	vietJSON(w, http.StatusOK, systemMessageToOut(m))
}

// RestoreSystemMessage serves DELETE /api/v1/finance-system-messages/{key}/override.
func (h *Handler) RestoreSystemMessage(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.writeSystemMessageError(w, r, "khôi phục", errNoActor)
		return
	}
	if _, err := h.d.SystemMessages.Restore(r.Context(), r.PathValue("key"), actor); err != nil {
		h.writeSystemMessageError(w, r, "khôi phục", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// errNoActor — the principal carries no business code. 500 and not a fallback to the internal id:
// rule 6, invariant 8 refuses a write whose trail cannot name who made it.
var errNoActor = errors.New("system_message: principal has no business code")

func (h *Handler) writeSystemMessageError(w http.ResponseWriter, r *http.Request, op string, err error) {
	switch {
	case errors.Is(err, domain.ErrUnknownMessageKey):
		// Same body for every unknown key, including another service's (`feedback.*`): the key is
		// not this service's to reword.
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không có câu hệ thống mang khoá này ở phân hệ Tài chính.", "")
	case domain.IsMessageInputError(err):
		// The domain's own sentence: it names the rule, holds no personal data, and a second copy
		// here would drift from it. The use case returns input refusals UNWRAPPED (they happen
		// before any transaction), so no commune id or operation leaks into the message.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
	default:
		h.d.Log.Error("lời hệ thống: "+op+" lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}
