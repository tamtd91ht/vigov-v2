package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	docstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The petition field catalogue — tier 1 (platform, ADR 0060) merged with the commune's tier 2
// (`nhan_linh_vuc`, ADR 0026) — on both surfaces:
//
//	GET   /api/v1/citizen-report-fields          staff, `admin.lookup` — every code, disabled and retired too
//	PATCH /api/v1/citizen-report-fields/{code}   staff, `admin.lookup` — label, order, on/off for one code
//	GET   /api/v1/my-citizen-report-fields       citizen, session-only — what the new-submission form offers
//	GET   /api/v1/citizen-report-intake-fields   staff, `feedback.create` — what the staff intake modal offers
//
// NO POST, NO DELETE: a commune cannot add or remove a code (ADR 0026). "Back to the default" is a
// PATCH carrying the default label / order, which the staff response hands over as default_label /
// default_order.

// PetitionFieldCatalogue is the staff side. Catalogue reads; Edit opens a transaction and audits
// inside it (app.PetitionFieldCatalogue).
type PetitionFieldCatalogue interface {
	Catalogue(ctx context.Context) ([]domain.PetitionFieldView, error)
	Edit(ctx context.Context, code string, e domain.PetitionFieldEdit, actor audit.Actor) (domain.PetitionFieldView, error)
	// StaffIntakeCatalogue is the staff intake modal's list — the same predicate the intake write checks
	// (app.PetitionFieldCatalogue.CheckStaffIntakeField), so the two cannot disagree.
	StaffIntakeCatalogue(ctx context.Context) ([]domain.PetitionFieldView, error)
}

// CitizenFieldCatalogue is the citizen side — the filtered read and nothing else, so the citizen
// surface cannot reach the edit even by typing it (rule 4, invariant 5).
type CitizenFieldCatalogue interface {
	CitizenCatalogue(ctx context.Context) ([]domain.PetitionFieldView, error)
}

// petitionFieldOut is one code as the staff configuration screen reads it.
type petitionFieldOut struct {
	// Code is the tier-1 code — what a petition stores. Immutable.
	Code string `json:"code"`
	// Label is effective: the commune's wording, else the platform default.
	Label string `json:"label"`
	// Order is the effective display position; `items` is already sorted by it.
	Order        int    `json:"order"`
	DefaultLabel string `json:"default_label"`
	DefaultOrder int    `json:"default_order"`
	// Icon / Tone are platform presentation; "" = not declared, render neutral.
	Icon string `json:"icon"`
	Tone string `json:"tone"`
	// Active false: retired platform-wide — refused for new petitions, still labelled everywhere.
	Active bool `json:"active"`
	// Enabled false: the commune hides it from the citizen's new-submission form. Old petitions keep it.
	Enabled bool `json:"enabled"`
	// Customised is DERIVED: some setting differs from the platform default.
	Customised bool `json:"customised"`
}

type petitionFieldListOut struct {
	Items []petitionFieldOut `json:"items"`
}

func petitionFieldOutOf(v domain.PetitionFieldView) petitionFieldOut {
	return petitionFieldOut{Code: v.Code, Label: v.Label, Order: v.Order, DefaultLabel: v.DefaultLabel,
		DefaultOrder: v.DefaultOrder, Icon: v.Icon, Tone: v.Tone, Active: v.Active, Enabled: v.Enabled,
		Customised: v.Customised}
}

// ListPetitionFields serves the commune's whole catalogue. GET /api/v1/citizen-report-fields
//
// NO AUDIT ENTRY: configuration, not personal data, not a cross-commune read (rule 6, invariant 7).
func (h *Handler) ListPetitionFields(w http.ResponseWriter, r *http.Request) {
	all, err := h.d.PetitionFields.Catalogue(r.Context())
	if err != nil {
		writeFieldCatalogueError(w, r, h.d.Log.Error, "đọc danh mục lĩnh vực", err)
		return
	}
	out := petitionFieldListOut{Items: make([]petitionFieldOut, 0, len(all))}
	for _, v := range all {
		out.Items = append(out.Items, petitionFieldOutOf(v))
	}
	vietJSON(w, http.StatusOK, out)
}

// updatePetitionFieldIn is the body of PATCH /api/v1/citizen-report-fields/{code}. Pointers: absent
// means unchanged. `code` is here only to be refused — a code is never renamed (rule 7, invariant 3).
type updatePetitionFieldIn struct {
	Label   *string `json:"label,omitempty"`
	Order   *int    `json:"order,omitempty"`
	Enabled *bool   `json:"enabled,omitempty"`

	Code *string `json:"code,omitempty"`
}

// UpdatePetitionField edits one code's label, order or switch. PATCH /api/v1/citizen-report-fields/{code}
func (h *Handler) UpdatePetitionField(w http.ResponseWriter, r *http.Request) {
	var in updatePetitionFieldIn
	if !docThan(w, r, &in) {
		return
	}
	if in.Code != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", domain.ErrMaBatBien.Error(), "")
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.d.Log.Error("tuyến sửa danh mục lĩnh vực chạy mà không có chủ thể hoặc mã cán bộ — SAI CẤU HÌNH",
			"xa", string(tenant.MustFrom(r.Context())), "duong", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	after, err := h.d.PetitionFields.Edit(r.Context(), r.PathValue("code"), domain.PetitionFieldEdit{
		Label: in.Label, Order: in.Order, Enabled: in.Enabled,
	}, actor)
	if err != nil {
		writeFieldCatalogueError(w, r, h.d.Log.Error, "sửa danh mục lĩnh vực", err)
		return
	}
	vietJSON(w, http.StatusOK, petitionFieldOutOf(after))
}

// citizenFieldOut is one field on the citizen's form — the shape of the requirement's Mini App
// (`../vigov-require@0053854` apps/miniapp/src/services/feedback-adapter.ts, ApiFieldOption) where
// this service has the data. `icon` / `tone` are null when the platform declares none, as there:
// the Mini App substitutes its own neutral icon and tone for null. No `placeholder`, `color` or
// `resolve_hours`: tier 1 carries no placeholder (ADR 0060 §4), `color` duplicates `tone`, and a
// deadline is only ever counted by identity at intake (rule 10, forbidden #2).
type citizenFieldOut struct {
	Code  string  `json:"code"`
	Label string  `json:"label"`
	Icon  *string `json:"icon"`
	Tone  *string `json:"tone"`
}

type citizenFieldListOut struct {
	Items []citizenFieldOut `json:"items"`
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ListCitizenReportFields serves the fields the commune offers on the new-submission form, in the
// commune's order. GET /api/v1/my-citizen-report-fields
//
// NO IDENTITY FILTER, AND NONE IS NEEDED: this reads the commune's configuration, not anybody's
// records, which is why the route may accept a session without a verified phone (XaTuPhienChiXem).
func (h *HandlerCongDan) ListCitizenReportFields(w http.ResponseWriter, r *http.Request) {
	offered, err := h.d.CitizenFields.CitizenCatalogue(r.Context())
	if err != nil {
		writeFieldCatalogueError(w, r, h.d.Log.Error, "đọc danh mục lĩnh vực cho công dân", err)
		return
	}
	out := citizenFieldListOut{Items: make([]citizenFieldOut, 0, len(offered))}
	for _, v := range offered {
		out.Items = append(out.Items, citizenFieldOut{Code: v.Code, Label: v.Label,
			Icon: nullIfEmpty(v.Icon), Tone: nullIfEmpty(v.Tone)})
	}
	vietJSON(w, http.StatusOK, out)
}

// ListStaffIntakeFields serves the fields the commune offers to STAFF INTAKE ("Nhập hộ phản ánh",
// docs/ui-ux/09 §11), in the commune's order. GET /api/v1/citizen-report-intake-fields
//
// THE CITIZEN LIST'S SHAPE ({items:[{code,label,icon,tone}]}, icon/tone null when undeclared) and the
// citizen list's type, on purpose: the modal renders the same picker, and a second type for the same
// four fields is a second place to drift. What differs is the PREDICATE (`can-bo` is offered here,
// domain.PetitionFieldView.OfferedToStaffIntake says why) and the surface (staff chain, `feedback.create`).
//
// NO AUDIT ENTRY: configuration, not personal data, not a cross-commune read (rule 6, invariant 7).
func (h *Handler) ListStaffIntakeFields(w http.ResponseWriter, r *http.Request) {
	offered, err := h.d.PetitionFields.StaffIntakeCatalogue(r.Context())
	if err != nil {
		writeFieldCatalogueError(w, r, h.d.Log.Error, "đọc danh mục lĩnh vực cho nhập hộ", err)
		return
	}
	out := citizenFieldListOut{Items: make([]citizenFieldOut, 0, len(offered))}
	for _, v := range offered {
		out.Items = append(out.Items, citizenFieldOut{Code: v.Code, Label: v.Label,
			Icon: nullIfEmpty(v.Icon), Tone: nullIfEmpty(v.Tone)})
	}
	vietJSON(w, http.StatusOK, out)
}

// writeFieldCatalogueError maps one failure onto a status and a sentence. Listed explicitly: an
// unrecognised error is a 500, never a 400 (the reasoning on laLoiDauVao).
func writeFieldCatalogueError(w http.ResponseWriter, r *http.Request,
	logError func(string, ...any), what string, err error) {

	switch {
	case errors.Is(err, app.ErrFieldCatalogueUnavailable):
		// FAIL CLOSED (ADR 0060 §3): no fallback list, no raw codes. Logged, because platform being
		// unreachable for longer than the reader's TTL is an operator's problem.
		logError("danh mục lĩnh vực: không đọc được bộ mã từ platform — TỪ CHỐI",
			"xa", string(tenant.MustFrom(r.Context())), "viec", what, "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "field_catalogue_unavailable",
			"Chưa đọc được danh mục lĩnh vực. Vui lòng thử lại sau ít phút.", "")
	case errors.Is(err, docstore.ErrDanhMucKhongTonTai):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không có lĩnh vực mang mã này.", "")
	case errors.Is(err, domain.ErrFieldEditEmpty), errors.Is(err, domain.ErrNhanTrong),
		errors.Is(err, domain.ErrNhanQuaDai), errors.Is(err, domain.ErrThuTuNgoaiKhoang):
		// The domain's own sentence: it names the field and the bound, and holds no personal data.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
	default:
		logError("danh mục lĩnh vực: "+what+" lỗi hệ thống",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}
