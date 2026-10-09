package http

// "Lời hệ thống" — the finance half (14-cau-hinh §7, ADR 0024 §`loi_he_thong`, migration 0010).
//
// SEVEN ROUTES SINCE ADR 0079 Q2 ("Làm đúng prototype", migration 0017): list; reword, restore and
// "Tắt / Bật lại" (PATCH …/override) of the commune's wording of a SHIPPED key; create, edit and soft
// delete of a sentence the COMMUNE added to "Giải ngân" (POST, PATCH / DELETE …/{code}). A shipped key
// is still never deleted, and a commune sentence is resolved nowhere (ADR 0079 Q5b).

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/app"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// SystemMessageService is the use case, declared at the point of use so the routes' refusals are
// testable without PostgreSQL. *app.SystemMessages satisfies it; every write opens one transaction
// carrying the row and its audit entry.
type SystemMessageService interface {
	Messages(ctx context.Context) ([]domain.SystemMessage, error)
	// Text is the sentence in force for one key — what a route that EMITS the sentence reads
	// (`scope_notice` on the investment-project reads, du_an.go).
	Resolve(ctx context.Context, key string) (domain.SystemMessage, error)
	Reword(ctx context.Context, key, text string, actor audit.Actor) (domain.SystemMessage, error)
	Restore(ctx context.Context, key string, actor audit.Actor) (domain.SystemMessage, error)
	SetActive(ctx context.Context, key string, active bool, actor audit.Actor) (domain.SystemMessage, error)
	CreateCustom(ctx context.Context, in app.NewCustomMessage, actor audit.Actor) (domain.SystemMessage, error)
	EditCustom(ctx context.Context, key string, in app.CustomMessageEdit, actor audit.Actor) (domain.SystemMessage, error)
	DeleteCustom(ctx context.Context, key, reason string, actor audit.Actor) error
}

// systemMessageOut is one card of the tab.
//
// `default_text` IS SENT BESIDE `current_text` so the screen can show what "Khôi phục câu mặc
// định" would bring back without a second request. `updated_at` / `updated_by` are absent while
// the commune is on the default — nobody changed anything, so there is nobody to name.
//
// `code` AND NOT `key`: apidoc refuses credential-looking response fields (tools/apidoc/schema.go:257).
//
// SINCE ADR 0079 Q2 (owner answers of 08/10/2026):
//
//	group_code     the web's group code — which section of the tab the card is listed under
//	origin         `shipped` (đi kèm phần mềm) or `commune` (xã tự thêm) — which buttons a card has
//	is_active      false for a switched-off sentence — any shipped key since 09/10/2026, reworded or
//	               not (then overridden false, current_text the default), or a commune sentence
//	override_text  the commune's stored wording of a shipped key, present while overridden, ALSO while
//	               switched off (then current_text is the default) — what "Bật lại" brings back
//	default_text   absent on a commune sentence: there is no software sentence to fall back to
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

// switchSystemMessageIn is the PATCH …/override body: the switch, and nothing else. REQUIRED — a
// PATCH that names no state is refused rather than read as "on".
type switchSystemMessageIn struct {
	IsActive *bool `json:"is_active"`
}

// createCustomMessageIn is the POST body of a commune sentence.
//
// `code` AND NOT `message_key`: tools/apidoc's credential guard refuses a field holding the word `key`
// (see systemMessageOut), and `code` is what the list, the path and every other route call it. `text`
// as on the reword PUT. The key must start with `<group_code>.` (domain.NormalizeCustomKey).
type createCustomMessageIn struct {
	GroupCode   string  `json:"group_code"`
	Code        string  `json:"code"`
	Text        string  `json:"text"`
	Description *string `json:"description,omitempty"`
}

// editCustomMessageIn is the PATCH body of a commune sentence. Every field optional; `description` has
// three states (absent = keep, null or blank = clear, text = set).
type editCustomMessageIn struct {
	Text        *string             `json:"text,omitempty"`
	Description optionalDescription `json:"description,omitempty"`
	IsActive    *bool               `json:"is_active,omitempty"`
}

// optionalDescription records whether `description` was present at all — encoding/json calls
// UnmarshalJSON for a present field INCLUDING an explicit null, never for an absent one.
type optionalDescription struct {
	present bool
	value   *string
}

func (o *optionalDescription) UnmarshalJSON(b []byte) error {
	o.present = true
	if string(b) == "null" {
		o.value = nil
		return nil
	}
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	o.value = &v
	return nil
}

func (o optionalDescription) change() *app.DescriptionChange {
	if !o.present {
		return nil
	}
	return &app.DescriptionChange{Value: o.value}
}

// deleteCustomMessageIn — a DELETE with a body: the reason is mandatory (rule 7, invariant 1) and a
// query string would put free text into every access log.
type deleteCustomMessageIn struct {
	Reason string `json:"reason"`
}

func systemMessageToOut(m domain.SystemMessage) systemMessageOut {
	return systemMessageOut{
		Code: m.Key, GroupCode: m.Group, Origin: m.Origin, Description: m.Description,
		DefaultText: m.DefaultText, CurrentText: m.CurrentText, OverrideText: m.OverrideText,
		Overridden: m.Overridden, IsActive: m.Active,
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

// RewordSystemMessage serves PUT /api/v1/finance-system-messages/{code}/override.
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
	m, err := h.d.SystemMessages.Reword(r.Context(), r.PathValue("code"), in.Text, actor)
	if err != nil {
		h.writeSystemMessageError(w, r, "sửa", err)
		return
	}
	vietJSON(w, http.StatusOK, systemMessageToOut(m))
}

// RestoreSystemMessage serves DELETE /api/v1/finance-system-messages/{code}/override.
func (h *Handler) RestoreSystemMessage(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.writeSystemMessageError(w, r, "khôi phục", errNoActor)
		return
	}
	if _, err := h.d.SystemMessages.Restore(r.Context(), r.PathValue("code"), actor); err != nil {
		h.writeSystemMessageError(w, r, "khôi phục", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SwitchSystemMessage serves PATCH /api/v1/finance-system-messages/{code}/override — "Tắt / Bật lại"
// of the commune's wording of a shipped key.
func (h *Handler) SwitchSystemMessage(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.writeSystemMessageError(w, r, "tắt/bật", errNoActor)
		return
	}
	var in switchSystemMessageIn
	if !docThan(w, r, &in) {
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
	vietJSON(w, http.StatusOK, systemMessageToOut(m))
}

// CreateCustomMessage serves POST /api/v1/finance-system-messages — a sentence the commune adds.
func (h *Handler) CreateCustomMessage(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.writeSystemMessageError(w, r, "thêm câu", errNoActor)
		return
	}
	var in createCustomMessageIn
	if !docThan(w, r, &in) {
		return
	}
	m, err := h.d.SystemMessages.CreateCustom(r.Context(), app.NewCustomMessage{
		Group: in.GroupCode, Key: in.Code, Text: in.Text, Description: in.Description,
	}, actor)
	if err != nil {
		h.writeSystemMessageError(w, r, "thêm câu", err)
		return
	}
	idem.RecordCode(r.Context(), m.Key)
	vietJSON(w, http.StatusCreated, systemMessageToOut(m))
}

// EditCustomMessage serves PATCH /api/v1/finance-system-messages/{code} — a commune sentence's
// words, description or switch.
func (h *Handler) EditCustomMessage(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.writeSystemMessageError(w, r, "sửa câu", errNoActor)
		return
	}
	var in editCustomMessageIn
	if !docThan(w, r, &in) {
		return
	}
	m, err := h.d.SystemMessages.EditCustom(r.Context(), r.PathValue("code"), app.CustomMessageEdit{
		Text: in.Text, Description: in.Description.change(), Active: in.IsActive,
	}, actor)
	if err != nil {
		h.writeSystemMessageError(w, r, "sửa câu", err)
		return
	}
	vietJSON(w, http.StatusOK, systemMessageToOut(m))
}

// DeleteCustomMessage serves DELETE /api/v1/finance-system-messages/{code} — soft delete of a commune
// sentence, with a reason. A shipped key answers 409.
func (h *Handler) DeleteCustomMessage(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.writeSystemMessageError(w, r, "xoá câu", errNoActor)
		return
	}
	var in deleteCustomMessageIn
	if !docThan(w, r, &in) {
		return
	}
	if err := h.d.SystemMessages.DeleteCustom(r.Context(), r.PathValue("code"), in.Reason, actor); err != nil {
		h.writeSystemMessageError(w, r, "xoá câu", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// errNoActor — the principal carries no business code. 500 and not a fallback to the internal id:
// rule 6, invariant 8 refuses a write whose trail cannot name who made it.
var errNoActor = errors.New("system_message: principal has no business code")

func (h *Handler) writeSystemMessageError(w http.ResponseWriter, r *http.Request, op string, err error) {
	switch {
	case errors.Is(err, domain.ErrUnknownMessageKey), errors.Is(err, domain.ErrCustomMessageNotFound):
		// Same body for every unknown key, including another service's (`feedback.*`) and a deleted
		// commune sentence: the key is not this service's to change. "mã", as the screen and the
		// petitions half say — the field is `code`.
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không có câu hệ thống mang mã này ở phân hệ Tài chính.", "")
	case errors.Is(err, domain.ErrMessageCodeTaken):
		httpx.WriteError(w, http.StatusConflict, "message_code_taken", "Mã này đã được dùng cho một câu khác.", "")
	case errors.Is(err, domain.ErrShippedMessageNotDeletable):
		httpx.WriteError(w, http.StatusConflict, "system_message", "Câu đi kèm phần mềm chỉ sửa lời được, không xoá được.", "")
	case errors.Is(err, domain.ErrShippedMessageNotCustom):
		httpx.WriteError(w, http.StatusConflict, "system_message",
			"Câu đi kèm phần mềm chỉ sửa lời, khôi phục lời gốc hoặc tắt/bật lời đã sửa.", "")
	case errors.Is(err, domain.ErrCustomCatalogueFull):
		httpx.WriteError(w, http.StatusConflict, "catalogue_full",
			"Số câu xã tự thêm đã đạt tối đa. Hãy xoá bớt câu không dùng.", "")
	case domain.IsCustomMessageInputError(err):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
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
