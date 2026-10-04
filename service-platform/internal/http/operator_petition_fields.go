package http

// The tier-1 petition field codes in the operator area (ADR 0073 #3, ADR 0060): read with any ops.*
// key, written under the seventh key `ops.petition_field.manage`. One code set for every commune;
// a commune's own wording, order and on/off switch are tier 2, in service-petitions, never here.

import (
	"context"
	"errors"
	"net/http"

	"golang.org/x/text/unicode/norm"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/service-platform/internal/domain"
	"github.com/vihat/vigov/service-platform/internal/store"
)

// PetitionFieldEditor is the tier-1 store (*store.PetitionFieldStore).
type PetitionFieldEditor interface {
	ListPetitionFields(ctx context.Context) ([]domain.PetitionField, error)
	CreatePetitionField(ctx context.Context, f domain.PetitionField, reason string, by domain.OperatorActor) (domain.PetitionField, error)
	EditPetitionField(ctx context.Context, code string, next domain.PetitionField, reason string, by domain.OperatorActor) (domain.PetitionField, bool, error)
	SetPetitionFieldActive(ctx context.Context, code string, active bool, reason string, by domain.OperatorActor) (domain.PetitionField, bool, error)
}

type petitionFieldView struct {
	Code         string `json:"code"`
	DefaultLabel string `json:"default_label"`
	SortOrder    int32  `json:"sort_order"`
	Icon         string `json:"icon"`
	Tone         string `json:"tone"`
	Active       bool   `json:"active"`
}

// petitionFieldListView — every code, retired ones included (a retired code still labels old
// petitions), plus the tones the console may offer.
type petitionFieldListView struct {
	Items []petitionFieldView `json:"items"`
	Tones []string            `json:"tones"`
}

// createPetitionFieldBody — every field explicit: an absent icon or tone is a 400, never a silent "".
type createPetitionFieldBody struct {
	Code         string  `json:"code"`
	DefaultLabel string  `json:"default_label"`
	SortOrder    *int32  `json:"sort_order"`
	Icon         *string `json:"icon"`
	Tone         *string `json:"tone"`
	Reason       string  `json:"reason"`
}

// editPetitionFieldBody has NO code field: the code is the path's and never changes (ADR 0060 §4). A
// body carrying "code" is refused as an unknown field (decodeBody), so a rename cannot even be asked.
type editPetitionFieldBody struct {
	DefaultLabel string  `json:"default_label"`
	SortOrder    *int32  `json:"sort_order"`
	Icon         *string `json:"icon"`
	Tone         *string `json:"tone"`
	Reason       string  `json:"reason"`
}

type petitionFieldActivationBody struct {
	// A pointer so an absent field is a 400, never a silent `false` that retires a code everywhere.
	Active *bool  `json:"active"`
	Reason string `json:"reason"`
}

func toPetitionFieldView(f domain.PetitionField) petitionFieldView {
	return petitionFieldView{Code: f.Code, DefaultLabel: f.DefaultLabel, SortOrder: f.SortOrder,
		Icon: f.Icon, Tone: f.Tone, Active: f.Active}
}

func (h *operatorHandlers) writeFieldError(w http.ResponseWriter, r *http.Request, err error) {
	type m struct {
		status     int
		code, text string
	}
	for target, v := range map[error]m{
		store.ErrPetitionFieldNotFound: {http.StatusNotFound, "petition_field_not_found", "Không có mã lĩnh vực này."},
		store.ErrPetitionFieldCodeTaken: {http.StatusConflict, "petition_field_code_taken",
			"Mã lĩnh vực đã có (kể cả mã đã ngừng dùng). Mã đã cấp không cấp lại; mã ngừng dùng thì bật lại."},
		domain.ErrPetitionFieldCodeInvalid: {http.StatusUnprocessableEntity, "invalid_code",
			"Mã không đúng dạng: chữ thường không dấu, số, gạch nối giữa các phần; tối đa 64 ký tự."},
		domain.ErrPetitionFieldLabelInvalid: {http.StatusUnprocessableEntity, "invalid_label",
			"Nhãn mặc định không hợp lệ (không để trống, tối đa 200 ký tự, một dòng)."},
		domain.ErrPetitionFieldSortOrderInvalid: {http.StatusUnprocessableEntity, "invalid_sort_order", "Thứ tự phải từ 1 đến 10000."},
		domain.ErrPetitionFieldIconInvalid: {http.StatusUnprocessableEntity, "invalid_icon",
			"Tên biểu tượng không hợp lệ (tên biểu tượng lucide, ví dụ Trash2), hoặc để trống."},
		domain.ErrPetitionFieldToneInvalid: {http.StatusUnprocessableEntity, "invalid_tone", "Tông màu không thuộc bộ cho phép, hoặc để trống."},
		domain.ErrReasonInvalid:            {http.StatusUnprocessableEntity, "invalid_reason", "Cần ghi lý do (tối đa 500 ký tự)."},
	} {
		if errors.Is(err, target) {
			httpx.WriteError(w, v.status, v.code, v.text, "")
			return
		}
	}
	h.d.Log.ErrorContext(r.Context(), "khu vận hành: đọc/ghi mã lĩnh vực cấp 1 thất bại", "err", err)
	httpx.WriteError(w, http.StatusInternalServerError, "internal", msgInternal, "")
}

func (h *operatorHandlers) listPetitionFields(w http.ResponseWriter, r *http.Request) {
	fs, err := h.d.Fields.ListPetitionFields(r.Context())
	if err != nil {
		h.writeFieldError(w, r, err)
		return
	}
	out := petitionFieldListView{Items: []petitionFieldView{}, Tones: domain.PetitionFieldTones}
	for _, f := range fs {
		out.Items = append(out.Items, toPetitionFieldView(f))
	}
	writeJSON(w, http.StatusOK, out)
}

// presentation validates the editable values shared by create and edit.
func presentation(label string, order *int32, icon, tone *string) (domain.PetitionField, error) {
	return domain.ValidatePetitionFieldPresentation(domain.PetitionField{
		DefaultLabel: norm.NFC.String(label), SortOrder: *order, Icon: *icon, Tone: *tone})
}

// createPetitionField issues a new tier-1 code, active (ADR 0060 "Cái giá": every service sees it
// within one 60-second TTL).
func (h *operatorHandlers) createPetitionField(w http.ResponseWriter, r *http.Request) {
	var b createPetitionFieldBody
	if !decodeBody(w, r, &b) {
		return
	}
	if b.SortOrder == nil || b.Icon == nil || b.Tone == nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", msgInvalidBody, "")
		return
	}
	if err := domain.ValidatePetitionFieldCode(b.Code); err != nil {
		h.writeFieldError(w, r, err)
		return
	}
	f, err := presentation(b.DefaultLabel, b.SortOrder, b.Icon, b.Tone)
	if err != nil {
		h.writeFieldError(w, r, err)
		return
	}
	f.Code = b.Code
	reason, err := domain.ValidateReason(norm.NFC.String(b.Reason))
	if err != nil {
		h.writeFieldError(w, r, err)
		return
	}
	out, err := h.d.Fields.CreatePetitionField(r.Context(), f, reason, actorOf(r))
	if err != nil {
		h.writeFieldError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toPetitionFieldView(out))
}

// pathFieldCode is the code of the path. A malformed one is answered exactly like an unknown one.
func (h *operatorHandlers) pathFieldCode(w http.ResponseWriter, r *http.Request) (string, bool) {
	code := r.PathValue("code")
	if domain.ValidatePetitionFieldCode(code) != nil {
		h.writeFieldError(w, r, store.ErrPetitionFieldNotFound)
		return "", false
	}
	return code, true
}

// editPetitionField sets the default label, order, icon and tone of one code. Never the code.
func (h *operatorHandlers) editPetitionField(w http.ResponseWriter, r *http.Request) {
	code, ok := h.pathFieldCode(w, r)
	if !ok {
		return
	}
	var b editPetitionFieldBody
	if !decodeBody(w, r, &b) {
		return
	}
	if b.SortOrder == nil || b.Icon == nil || b.Tone == nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", msgInvalidBody, "")
		return
	}
	next, err := presentation(b.DefaultLabel, b.SortOrder, b.Icon, b.Tone)
	if err != nil {
		h.writeFieldError(w, r, err)
		return
	}
	reason, err := domain.ValidateReason(norm.NFC.String(b.Reason))
	if err != nil {
		h.writeFieldError(w, r, err)
		return
	}
	out, _, err := h.d.Fields.EditPetitionField(r.Context(), code, next, reason, actorOf(r))
	if err != nil {
		h.writeFieldError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toPetitionFieldView(out))
}

// setPetitionFieldActivation retires or brings back one code in every commune at once.
func (h *operatorHandlers) setPetitionFieldActivation(w http.ResponseWriter, r *http.Request) {
	code, ok := h.pathFieldCode(w, r)
	if !ok {
		return
	}
	var b petitionFieldActivationBody
	if !decodeBody(w, r, &b) {
		return
	}
	if b.Active == nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", msgInvalidBody, "")
		return
	}
	reason, err := domain.ValidateReason(norm.NFC.String(b.Reason))
	if err != nil {
		h.writeFieldError(w, r, err)
		return
	}
	out, _, err := h.d.Fields.SetPetitionFieldActive(r.Context(), code, *b.Active, reason, actorOf(r))
	if err != nil {
		h.writeFieldError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toPetitionFieldView(out))
}
