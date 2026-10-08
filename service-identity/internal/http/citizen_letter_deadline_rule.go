package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The routes behind Cấu hình → Thời hạn xử lý, citizen-letter part (migration 0027; ADR 0084 #3,
// ADR 0085 B and câu 2–4): a commune's deadline rule per letter type and deadline kind.
//
//	GET    /api/v1/citizen-letter-deadline-rules        the commune's live rules
//	POST   /api/v1/citizen-letter-deadline-rules        create one rule
//	PATCH  /api/v1/citizen-letter-deadline-rules/{id}   change its amount and/or unit
//	DELETE /api/v1/citizen-letter-deadline-rules/{id}   soft delete it, with a reason
//
// ALL FOUR DECLARE `admin.sla`, the key the `sla` routes use, seeded in `quyen` at
// migrations/0001_init.sql:284 ("Cấu hình thời hạn xử lý"). No key is invented (rule 5, invariant 3c).
//
// NOTHING IS SEEDED AND NOTHING IS PREFILLED (ADR 0085 câu 4). A type with no rule is "Không đặt hạn":
// the empty list is the honest state of every commune today, not a fault.
//
// THE COMMUNE IS NEVER PASSED. Every read and write below goes through store.DB.For(ctx), which binds
// tenant_id from the Host-derived context (rule 1, invariant 4); another commune's id is not found.
//
// ⚠ THE NOUN `citizen-letter-deadline-rules` IS NOT ONE THE USER HAS APPROVED. It follows the
// `citizen-letters` resource (ubiquitous-language.md) and the table name; renaming is a path string here
// plus the generated contract, while no client depends on it.

// citizenLetterRuleBodyMax bounds a request body — four short fields.
const citizenLetterRuleBodyMax = 4 << 10

// CitizenLetterDeadlineRuleReader reads the commune's live rules. *idstore.CitizenLetterDeadlineRuleStore.
type CitizenLetterDeadlineRuleReader interface {
	List(ctx context.Context) ([]domain.CitizenLetterDeadlineRule, error)
}

// CitizenLetterDeadlineRuleWriting is the write surface — a USE CASE, because every write opens the
// transaction its audit entry shares. *app.CitizenLetterDeadlineRules.
type CitizenLetterDeadlineRuleWriting interface {
	Create(ctx context.Context, req app.CreateCitizenLetterDeadlineRuleRequest, actor app.NguoiThucHien) (domain.CitizenLetterDeadlineRule, error)
	Update(ctx context.Context, id string, req app.UpdateCitizenLetterDeadlineRuleRequest, actor app.NguoiThucHien) (domain.CitizenLetterDeadlineRule, error)
	Remove(ctx context.Context, id, reason string, actor app.NguoiThucHien) error
}

// citizenLetterDeadlineRuleOut is one rule as it leaves the API. No personal data (rule 3).
type citizenLetterDeadlineRuleOut struct {
	ID string `json:"id"` // ULID — what PATCH / DELETE reference

	// LetterType is `kien-nghi-phan-anh` · `khieu-nai` · `to-cao` · `de-nghi`.
	LetterType string `json:"letter_type"`

	// DeadlineKind is `xu-ly-don` (hạn xử lý đơn, fixed at booking) or `giai-quyet` (hạn giải quyết,
	// fixed at `thu-ly`; khiếu nại and tố cáo only).
	DeadlineKind string `json:"deadline_kind"`

	// Amount is a count IN Unit. Never read one without the other.
	Amount int `json:"amount"`

	// Unit is `ngay-lam-viec` or `ngay-lich` (`gio-lam-viec` is stored-admissible and always refused).
	Unit string `json:"unit"`

	// RequiredUnit is the ONE unit the lock allows for this letter type and deadline kind (owner,
	// 08/10/2026) — so the screen can show it rather than offer a choice. "" if the pair itself is invalid.
	RequiredUnit string `json:"required_unit"`

	// Problem is null when the rule is usable, otherwise the sentence saying why it is NOT USED: booking
	// a letter of this type is refused until it is fixed (ResolveCitizenLetterDeadline answers
	// FAILED_PRECONDITION). Derived on every read, never stored.
	Problem *string `json:"problem"`
}

type citizenLetterDeadlineRulesOut struct {
	Items []citizenLetterDeadlineRuleOut `json:"items"`
}

func citizenLetterDeadlineRuleToOut(r domain.CitizenLetterDeadlineRule) citizenLetterDeadlineRuleOut {
	out := citizenLetterDeadlineRuleOut{
		ID: r.ID, LetterType: string(r.LetterType), DeadlineKind: string(r.Kind),
		Amount: r.Amount, Unit: string(r.Unit),
	}
	if u, err := domain.RequiredCitizenLetterUnit(r.LetterType, r.Kind); err == nil {
		out.RequiredUnit = string(u)
	}
	if err := domain.CheckCitizenLetterDeadlineRule(r); err != nil {
		msg := err.Error()
		out.Problem = &msg
	}
	return out
}

// createCitizenLetterDeadlineRuleIn is the body of POST. Every field is required.
type createCitizenLetterDeadlineRuleIn struct {
	LetterType   string `json:"letter_type"`
	DeadlineKind string `json:"deadline_kind"`
	Amount       int    `json:"amount"`
	Unit         string `json:"unit"`
}

// updateCitizenLetterDeadlineRuleIn is the body of PATCH: absent leaves the value alone. There is no
// letter type and no kind — what a rule governs is fixed at creation.
type updateCitizenLetterDeadlineRuleIn struct {
	Amount *int    `json:"amount,omitempty"`
	Unit   *string `json:"unit,omitempty"`
}

// removeCitizenLetterDeadlineRuleIn is the body of DELETE — the reason is mandatory (rule 7) and kept
// out of the query string, where free text would land in every access log.
type removeCitizenLetterDeadlineRuleIn struct {
	Reason string `json:"reason"`
}

// ListCitizenLetterDeadlineRules — GET /api/v1/citizen-letter-deadline-rules. No audit entry: a
// commune's own published policy, read inside the commune (rule 6, invariant 7 does not apply).
func (h *Handler) ListCitizenLetterDeadlineRules(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// tenant_id: the context's, from Host; the store reads through store.DB.For(ctx).Query.
	rules, err := h.d.CitizenLetterDeadlineRules.List(ctx)
	if err != nil {
		// Including ErrTooManyCitizenLetterDeadlineRules: refused, never truncated. The cause stays in
		// this service's log (rule 3, forbidden #3).
		h.d.Log.Error("quy tắc hạn đơn thư: lỗi đọc", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	out := citizenLetterDeadlineRulesOut{Items: make([]citizenLetterDeadlineRuleOut, 0, len(rules))}
	for _, rule := range rules {
		out.Items = append(out.Items, citizenLetterDeadlineRuleToOut(rule))
	}
	vietJSON(w, http.StatusOK, out)
}

// CreateCitizenLetterDeadlineRule — POST /api/v1/citizen-letter-deadline-rules. 201 with the rule. It
// changes no deadline already stored on a letter (rule 10, invariant 2).
func (h *Handler) CreateCitizenLetterDeadlineRule(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var in createCitizenLetterDeadlineRuleIn
	r.Body = http.MaxBytesReader(w, r.Body, citizenLetterRuleBodyMax)
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		// The decoder's message quotes the input and is not returned (rule 3, forbidden #3).
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Nội dung gửi lên không phải JSON hợp lệ hoặc quá lớn.", "")
		return
	}
	// tenant_id: the context's, from Host; the use case opens store.DB.For(ctx).Tx.
	rule, err := h.d.WriteCitizenLetterDeadlineRules.Create(r.Context(), app.CreateCitizenLetterDeadlineRuleRequest{
		LetterType: domain.CitizenLetterType(in.LetterType),
		Kind:       domain.CitizenLetterDeadlineKind(in.DeadlineKind),
		Amount:     in.Amount,
		Unit:       domain.CitizenLetterDeadlineUnit(in.Unit),
	}, actor)
	if err != nil {
		h.writeCitizenLetterRuleError(w, r, "thêm quy tắc", err)
		return
	}
	// What a retry with the same Idempotency-Key is told: the rule id, never the body.
	idem.RecordCode(r.Context(), rule.ID)
	vietJSON(w, http.StatusCreated, citizenLetterDeadlineRuleToOut(rule))
}

// UpdateCitizenLetterDeadlineRule — PATCH /api/v1/citizen-letter-deadline-rules/{id}.
func (h *Handler) UpdateCitizenLetterDeadlineRule(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var in updateCitizenLetterDeadlineRuleIn
	r.Body = http.MaxBytesReader(w, r.Body, citizenLetterRuleBodyMax)
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Nội dung gửi lên không phải JSON hợp lệ hoặc quá lớn.", "")
		return
	}
	// `{}` is a form the client failed to read; answering 200 would tell the person it was saved.
	if in.Amount == nil && in.Unit == nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Không có số ngày hay đơn vị nào được gửi lên để sửa.", "")
		return
	}
	req := app.UpdateCitizenLetterDeadlineRuleRequest{Amount: in.Amount}
	if in.Unit != nil {
		u := domain.CitizenLetterDeadlineUnit(*in.Unit)
		req.Unit = &u
	}
	// tenant_id: the context's, from Host; the use case opens store.DB.For(ctx).Tx, so another
	// commune's id is simply not found (404).
	rule, err := h.d.WriteCitizenLetterDeadlineRules.Update(r.Context(), r.PathValue("id"), req, actor)
	if err != nil {
		h.writeCitizenLetterRuleError(w, r, "sửa quy tắc", err)
		return
	}
	vietJSON(w, http.StatusOK, citizenLetterDeadlineRuleToOut(rule))
}

// RemoveCitizenLetterDeadlineRule — DELETE /api/v1/citizen-letter-deadline-rules/{id}. 204; the row
// stays with its three soft-delete columns (rule 7).
func (h *Handler) RemoveCitizenLetterDeadlineRule(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var in removeCitizenLetterDeadlineRuleIn
	r.Body = http.MaxBytesReader(w, r.Body, citizenLetterRuleBodyMax)
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Nội dung gửi lên không phải JSON hợp lệ hoặc quá lớn.", "")
		return
	}
	// tenant_id: the context's, from Host; the use case opens store.DB.For(ctx).Tx.
	if err := h.d.WriteCitizenLetterDeadlineRules.Remove(r.Context(), r.PathValue("id"), in.Reason, actor); err != nil {
		h.writeCitizenLetterRuleError(w, r, "xoá quy tắc", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeCitizenLetterRuleError maps one use-case failure onto a status — one function for the three
// write routes, so the mapping cannot drift between them.
func (h *Handler) writeCitizenLetterRuleError(w http.ResponseWriter, r *http.Request, what string, err error) {
	switch {
	case errors.Is(err, idstore.ErrCitizenLetterDeadlineRuleNotFound):
		// One answer for an invented id, a removed rule and another commune's rule (rule 4, forbidden #2).
		httpx.WriteError(w, http.StatusNotFound, "citizen_letter_deadline_rule_not_found",
			"Không tìm thấy quy tắc hạn đơn thư này.", "")
	case errors.Is(err, app.ErrCitizenLetterDeadlineRuleExists):
		httpx.WriteError(w, http.StatusConflict, "citizen_letter_deadline_rule_exists",
			"Loại đơn này đã có quy tắc cho loại hạn này — hãy sửa quy tắc sẵn có.", "")
	case app.IsCitizenLetterDeadlineRuleInputError(err):
		// The domain's own sentence: it names the rule (and the required unit), holds no personal data.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
	default:
		h.d.Log.Error("quy tắc hạn đơn thư: "+what+" lỗi hệ thống",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}
