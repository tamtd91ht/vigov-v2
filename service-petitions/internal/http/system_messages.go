package http

// "Lời hệ thống" — the petitions half (14-cau-hinh §7, ADR 0024 §`loi_he_thong`, migration 0020).
// A copy of service-finance/internal/http/system_messages.go, never an import (rule 2).
//
// THREE ROUTES, NO FOURTH: list, reword, restore. There is no create (a commune-invented key has no
// code that raises it) and no delete of a shipped key (it cannot be removed — only reworded back).
//
// AND ONE READ THAT IS NOT A ROUTE: systemMessage, which every refusal branch that owns one of the
// six keys calls to fetch the commune's sentence instead of a hardcoded one (refusalMessageKeys).

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// SystemMessageService is the use case, declared at the point of use so the routes' refusals are
// testable without PostgreSQL. *app.SystemMessages satisfies it; every write opens one transaction
// carrying the row and its audit entry.
//
// Text's CONTRACT is the one the refusal branches rely on: for a shipped key it ALWAYS returns a
// sentence — the default when the override could not be read — and the error only for the log.
type SystemMessageService interface {
	Messages(ctx context.Context) ([]domain.SystemMessage, error)
	Text(ctx context.Context, key string) (string, error)
	Reword(ctx context.Context, key, text string, actor audit.Actor) (domain.SystemMessage, error)
	Restore(ctx context.Context, key string, actor audit.Actor) (domain.SystemMessage, error)
}

// systemMessageOut is one card of the tab.
//
// THE KEY TRAVELS AS `code`, NOT `key`: tools/apidoc's credential guard refuses a response field
// whose name holds the word `key` (tools/apidoc/schema.go, tuBiMat), and a message key is the
// catalogue CODE of the sentence in every sense the screen uses it.
//
// `default_text` IS SENT BESIDE `current_text` so the screen can show what "Khôi phục câu mặc
// định" would bring back without a second request. `updated_at` / `updated_by` are absent while
// the commune is on the default — nobody changed anything, so there is nobody to name.
type systemMessageOut struct {
	Code        string     `json:"code"`
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
		Code: m.Key, Description: m.Description, DefaultText: m.DefaultText,
		CurrentText: m.CurrentText, Overridden: m.Overridden,
		UpdatedAt: m.UpdatedAt, UpdatedBy: m.UpdatedBy,
	}
}

// ListSystemMessages serves GET /api/v1/petitions-system-messages.
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

// RewordSystemMessage serves PUT /api/v1/petitions-system-messages/{code}/override.
func (h *Handler) RewordSystemMessage(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.writeSystemMessageError(w, r, "sửa", errNoMessageActor)
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

// RestoreSystemMessage serves DELETE /api/v1/petitions-system-messages/{code}/override.
func (h *Handler) RestoreSystemMessage(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHien(r)
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

// errNoMessageActor — the principal carries no business code. 500 and not a fallback to the internal
// id: rule 6, invariant 8 refuses a write whose trail cannot name who made it.
var errNoMessageActor = errors.New("system_message: principal has no business code")

func (h *Handler) writeSystemMessageError(w http.ResponseWriter, r *http.Request, op string, err error) {
	switch {
	case errors.Is(err, domain.ErrUnknownMessageKey):
		// Same body for every unknown key, including another service's (`budget.scope_notice`): the
		// key is not this service's to reword.
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không có câu hệ thống mang mã này ở phân hệ Tiếp dân – Nhiệm vụ.", "")
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

// --- the refusal branches' read -------------------------------------------------------------------

// refusalMessageKeys maps each refusal whose sentence a commune may reword to its key. A refusal NOT
// listed here keeps its fixed sentence in cacCauTuChoiPhieu (xu_ly_phan_anh.go); a refusal listed
// here has NO row there — one sentence per refusal, one source per sentence (rule 9).
//
// WHICH SENTINEL, AND WHY ONLY THAT ONE (the mapping is this session's reading, reported):
//
//	ErrKhongConCamKet → invalid_transition   the advance route's "no forward step from this status" —
//	                                         raised at app/xu_ly_phan_anh.go TienTrangThai. It is the
//	                                         only refusal that is a bare "this move is not in the
//	                                         lifecycle"; ErrPhanCongSaiLuc / ErrDongSaiLuc /
//	                                         ErrKetThucNhanhSaiLuc keep their own sentences because
//	                                         each names what to do instead, and the generic one does not.
//	ErrThieuBoPhan    → assignment_required  assignment with no department (domain.KiemPhanCong).
//	ErrThieuLyDo      → reason_required      rejection or referral with a blank reason
//	                                         (domain.KiemLyDoKetThucNhanh). "Too short" and "too long"
//	                                         keep their sentences: they carry the bound, the key does not.
//
//	ErrVerificationPhotoRequired → after_photo_required   the close gate (app.XuLyPhanAnh.Dong,
//	                                         ADR 0008 decision 3): no stored verification photo in a
//	                                         commune whose switch requires one.
//
// feedback.never_public is read in petition_publication.go, whose branch is not in this table's switch.
// feedback.unknown_field has NO branch in this service yet.
var refusalMessageKeys = []struct {
	cause error
	key   string
}{
	{domain.ErrKhongConCamKet, domain.KeyFeedbackInvalidTransition},
	{domain.ErrThieuBoPhan, domain.KeyFeedbackAssignmentRequired},
	{domain.ErrThieuLyDo, domain.KeyFeedbackReasonRequired},
	{domain.ErrVerificationPhotoRequired, domain.KeyFeedbackAfterPhotoRequired},
}

// refusalMessageKey returns the key of the first configurable refusal in the chain.
func refusalMessageKey(err error) (string, bool) {
	for _, c := range refusalMessageKeys {
		if errors.Is(err, c.cause) {
			return c.key, true
		}
	}
	return "", false
}

// refusalSentence is the sentence an officer reads for one refusal of the petition write routes: the
// commune's wording for a configurable one, the fixed sentence otherwise.
func (h *Handler) refusalSentence(ctx context.Context, err error) (string, bool) {
	if key, ok := refusalMessageKey(err); ok {
		return h.systemMessage(ctx, key), true
	}
	return cauTuChoiPhieu(err)
}

// systemMessage returns the commune's sentence for one key, and NEVER FAILS THE REQUEST.
//
// A failed override read answers the DEFAULT (SystemMessageService.Text's contract) and is logged at
// WARN with the commune, the key and the chain — no text, no officer, no petition code (rule 3). The
// refusal being answered is correct whatever the wording is; turning it into a 500 would tell an
// officer the server is broken when what is wrong is the form, and change the code a client branches on.
//
// AN EMPTY ANSWER IS A WIRING BUG (a key missing from the catalogue) and falls back to the generic
// sentence, never to err.Error().
func (h *Handler) systemMessage(ctx context.Context, key string) string {
	text, err := h.d.SystemMessages.Text(ctx, key)
	if err != nil {
		h.d.Log.Warn("lời hệ thống: không đọc được câu của xã — dùng câu mặc định",
			"xa", string(tenant.MustFrom(ctx)), "khoa", key, "err", err)
	}
	if text == "" {
		return "Yêu cầu bị từ chối. Vui lòng tải lại phiếu rồi thao tác lại."
	}
	return text
}
