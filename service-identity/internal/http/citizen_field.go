package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The routes of "Cấu hình → Thuộc tính danh bạ" (ADR 0101 #1–#5; migration 0036) — a commune's dynamic
// attributes of the citizen directory:
//
//	GET    /api/v1/citizen-fields                live fields in order (citizen.read — the contact form needs
//	                                             them); ?include_retired=true adds retired ones and needs
//	                                             admin.citizen_field (the configuration screen)
//	POST   /api/v1/citizen-fields                add a field                       201 + the field
//	PATCH  /api/v1/citizen-fields/{code}         edit label/placeholder/options/…  200 + the field
//	DELETE /api/v1/citizen-fields/{code}         retire (soft), reason mandatory   204
//	PUT    /api/v1/citizen-field-order           the whole drag-and-drop order     204
//
// KEYS (rule 5, invariant 3c): `admin.citizen_field` on every write, seeded by migration 0036 for this
// purpose (owner, 17/10/2026, ADR 0101 #5); `citizen.read` (0028) on the plain list. No key invented.
//
// NOTHING HERE IS PERSONAL DATA: a definition is configuration. The handlers still log only the commune
// and the error.

// CitizenFieldUseCases is what the five routes need — *app.CitizenFields in production. Every method reads
// its commune from the context.
type CitizenFieldUseCases interface {
	List(ctx context.Context, includeRetired bool) ([]domain.CitizenField, error)
	Create(ctx context.Context, actor app.NguoiThucHien, in domain.CitizenFieldInput) (domain.CitizenField, error)
	Update(ctx context.Context, actor app.NguoiThucHien, code string, p domain.CitizenFieldPatch) (domain.CitizenField, error)
	Retire(ctx context.Context, actor app.NguoiThucHien, code, reason string) error
	Reorder(ctx context.Context, actor app.NguoiThucHien, codes []string) error
}

// citizenFieldAdminKey guards every write and the retired view.
const citizenFieldAdminKey = authz.Perm("admin.citizen_field")

type citizenFieldOptionWire struct {
	Value   string `json:"value"`
	Label   string `json:"label"`
	Retired bool   `json:"retired"`
}

// citizenFieldOut is one field. `options` is [] for a type without options; `number_kind` / `date_pattern`
// are "" where they do not apply; `retired_at` is null for a live field.
type citizenFieldOut struct {
	Code        string                   `json:"code"`
	Label       string                   `json:"label"`
	FieldType   string                   `json:"field_type"`
	Placeholder string                   `json:"placeholder"`
	Options     []citizenFieldOptionWire `json:"options"`
	NumberKind  string                   `json:"number_kind"`
	DatePattern string                   `json:"date_pattern"`
	Required    bool                     `json:"required"`
	ShowInList  bool                     `json:"show_in_list"`
	SortOrder   int                      `json:"sort_order"`
	CreatedAt   time.Time                `json:"created_at"`
	UpdatedAt   time.Time                `json:"updated_at"`
	RetiredAt   *time.Time               `json:"retired_at"`
}

type citizenFieldsOut struct {
	Items []citizenFieldOut `json:"items"`
}

// citizenFieldIn is the body of POST: the whole definition.
type citizenFieldIn struct {
	Code        string                   `json:"code"`
	Label       string                   `json:"label"`
	FieldType   string                   `json:"field_type"`
	Placeholder string                   `json:"placeholder"`
	Options     []citizenFieldOptionWire `json:"options"`
	NumberKind  string                   `json:"number_kind"`
	DatePattern string                   `json:"date_pattern"`
	Required    bool                     `json:"required"`
	ShowInList  bool                     `json:"show_in_list"`
}

// citizenFieldPatchIn is the body of PATCH: every member optional. `options` is the WHOLE list — every
// value already defined must still be there (relabel, retire, add; never remove). `code`, `field_type`
// and `number_kind` are immutable: sending a different value is 400.
type citizenFieldPatchIn struct {
	Code        *string                   `json:"code"`
	Label       *string                   `json:"label"`
	FieldType   *string                   `json:"field_type"`
	Placeholder *string                   `json:"placeholder"`
	Options     *[]citizenFieldOptionWire `json:"options"`
	NumberKind  *string                   `json:"number_kind"`
	DatePattern *string                   `json:"date_pattern"`
	Required    *bool                     `json:"required"`
	ShowInList  *bool                     `json:"show_in_list"`
}

type citizenFieldRetireIn struct {
	Reason string `json:"reason"`
}

type citizenFieldOrderIn struct {
	Codes []string `json:"codes"`
}

func optionsFromWire(in []citizenFieldOptionWire) []domain.CitizenFieldOption {
	out := make([]domain.CitizenFieldOption, 0, len(in))
	for _, o := range in {
		out = append(out, domain.CitizenFieldOption{Value: o.Value, Label: o.Label, Retired: o.Retired})
	}
	return out
}

func citizenFieldToOut(f domain.CitizenField) citizenFieldOut {
	opts := make([]citizenFieldOptionWire, 0, len(f.Options))
	for _, o := range f.Options {
		opts = append(opts, citizenFieldOptionWire{Value: o.Value, Label: o.Label, Retired: o.Retired})
	}
	out := citizenFieldOut{
		Code: f.Code, Label: f.Label, FieldType: string(f.Type), Placeholder: f.Placeholder, Options: opts,
		NumberKind: f.NumberKind, DatePattern: f.DatePattern, Required: f.Required, ShowInList: f.ShowInList,
		SortOrder: f.SortOrder, CreatedAt: f.CreatedAt.UTC(), UpdatedAt: f.UpdatedAt.UTC(),
	}
	if f.RetiredAt != nil {
		t := f.RetiredAt.UTC()
		out.RetiredAt = &t
	}
	return out
}

// ListCitizenFields — GET /api/v1/citizen-fields[?include_retired=true].
func (h *Handler) ListCitizenFields(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	includeRetired, ok := queryBool(w, r.URL.Query(), "include_retired")
	if !ok {
		return
	}
	retired := includeRetired != nil && *includeRetired
	if retired {
		// The retired view is the configuration screen's: the route admits citizen.read OR the admin key,
		// and this view needs the admin key itself (same commune check as the guard — the guard ran).
		p, _ := authz.From(ctx)
		if !h.d.Checker.Allows(ctx, p, citizenFieldAdminKey) {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", "Tài khoản của bạn không có quyền thực hiện thao tác này.", "")
			return
		}
	}
	// tenant_id: the context's, from Host; the store reads through store.DB.For(ctx).
	fields, err := h.d.CitizenFields.List(ctx, retired)
	if err != nil {
		h.d.Log.Error("thuộc tính danh bạ: đọc danh sách lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	out := citizenFieldsOut{Items: make([]citizenFieldOut, 0, len(fields))}
	for _, f := range fields {
		out.Items = append(out.Items, citizenFieldToOut(f))
	}
	vietJSON(w, http.StatusOK, out)
}

// CreateCitizenField — POST /api/v1/citizen-fields. 201 with the field; 409 `citizen_field_code_taken`.
func (h *Handler) CreateCitizenField(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var in citizenFieldIn
	if !readCitizenDirectoryBody(w, r, &in) {
		return
	}
	// tenant_id: the context's, from Host; the use case opens store.DB.For(ctx).Tx.
	f, err := h.d.CitizenFields.Create(r.Context(), actor, domain.CitizenFieldInput{
		Code: in.Code, Label: in.Label, Type: domain.CitizenFieldType(in.FieldType), Placeholder: in.Placeholder,
		Options: optionsFromWire(in.Options), NumberKind: in.NumberKind, DatePattern: in.DatePattern,
		Required: in.Required, ShowInList: in.ShowInList,
	})
	if err != nil {
		h.writeCitizenFieldError(w, r, "thêm", err)
		return
	}
	idem.RecordCode(r.Context(), f.Code)
	vietJSON(w, http.StatusCreated, citizenFieldToOut(f))
}

// UpdateCitizenField — PATCH /api/v1/citizen-fields/{code}. 200 with the field.
func (h *Handler) UpdateCitizenField(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var in citizenFieldPatchIn
	if !readCitizenDirectoryBody(w, r, &in) {
		return
	}
	code := r.PathValue("code")
	if in.Code != nil && *in.Code != code {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", domain.ErrCitizenFieldImmutable.Error(), "")
		return
	}
	p := domain.CitizenFieldPatch{Label: in.Label, Placeholder: in.Placeholder, Required: in.Required,
		ShowInList: in.ShowInList, DatePattern: in.DatePattern, NumberKind: in.NumberKind}
	if in.FieldType != nil {
		t := domain.CitizenFieldType(*in.FieldType)
		p.Type = &t
	}
	if in.Options != nil {
		opts := optionsFromWire(*in.Options)
		p.Options = &opts
	}
	// tenant_id: the context's, from Host; another commune's code is simply not found (404).
	f, err := h.d.CitizenFields.Update(r.Context(), actor, code, p)
	if err != nil {
		h.writeCitizenFieldError(w, r, "sửa", err)
		return
	}
	vietJSON(w, http.StatusOK, citizenFieldToOut(f))
}

// RetireCitizenField — DELETE /api/v1/citizen-fields/{code} with {"reason"}: soft retire, values kept
// (ADR 0101 #4). 204.
func (h *Handler) RetireCitizenField(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var in citizenFieldRetireIn
	if !readCitizenDirectoryBody(w, r, &in) {
		return
	}
	// tenant_id: the context's, from Host; the use case opens store.DB.For(ctx).Tx.
	if err := h.d.CitizenFields.Retire(r.Context(), actor, r.PathValue("code"), in.Reason); err != nil {
		h.writeCitizenFieldError(w, r, "ngừng dùng", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SetCitizenFieldOrder — PUT /api/v1/citizen-field-order with {"codes": [...]}, every live code once. 204.
func (h *Handler) SetCitizenFieldOrder(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var in citizenFieldOrderIn
	if !readCitizenDirectoryBody(w, r, &in) {
		return
	}
	// tenant_id: the context's, from Host; the use case opens store.DB.For(ctx).Tx.
	if err := h.d.CitizenFields.Reorder(r.Context(), actor, in.Codes); err != nil {
		h.writeCitizenFieldError(w, r, "sắp xếp", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) writeCitizenFieldError(w http.ResponseWriter, r *http.Request, what string, err error) {
	switch {
	case errors.Is(err, idstore.ErrCitizenFieldNotFound):
		httpx.WriteError(w, http.StatusNotFound, "citizen_field_not_found", "Không tìm thấy thuộc tính này (hoặc đã ngừng dùng).", "")
	case errors.Is(err, idstore.ErrCitizenFieldCodeTaken):
		httpx.WriteError(w, http.StatusConflict, "citizen_field_code_taken", idstore.ErrCitizenFieldCodeTaken.Error(), "")
	case errors.Is(err, app.ErrCitizenFieldLimit):
		httpx.WriteError(w, http.StatusConflict, "citizen_field_limit", app.ErrCitizenFieldLimit.Error(), "")
	case app.IsCitizenFieldInputError(err):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
	default:
		h.d.Log.Error("thuộc tính danh bạ: "+what+" lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}
