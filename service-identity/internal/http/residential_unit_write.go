package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The WRITE routes of the commune's residential units (user decision 2026-09-29, ADR 0059 §2), both
// `admin.org` — the key the org chart's write routes declare; no key invented (rule 5, invariant 3c):
//
//	POST  /api/v1/residential-units        add a unit
//	PATCH /api/v1/residential-units/{id}   edit it — including `active: false`, TAKING IT OUT OF USE
//
// NO DELETE ROUTE, AND THAT IS THE DECISION, NOT A GAP. "Ngưng dùng" is `active: false` on the PATCH —
// the same field, and the same reading, as the seven catalogues' PATCH (danh_muc_ghi.go): the unit
// stays on the list with `active: false`, pickers filter it out, and every record pointing at it keeps
// printing its name. A DELETE that left the row on the list would be a verb that does not do what it
// says. There is no merge and no split (rule 1, stop condition #3).

// maxResidentialUnitBody bounds a write body: eight short fields.
const maxResidentialUnitBody = 8 << 10

// createResidentialUnitIn is the create body.
//
//	name              required. Case kept as typed.
//	code              absent or "" = derived from the name ("Thôn Bình An" → "thon-binh-an"), with
//	                  `-2`, `-3`… when taken. PRESENT = used exactly, or 409 when taken — a code taken
//	                  by a unit out of use or soft-deleted is taken (rule 7, invariant 3).
//	type_code         absent or "" = not classified; else a type in use in THIS commune, else 400.
//	head_staff_code   absent or "" = no head; else the STAFF CODE (GET /api/v1/staff-directory
//	                  `code`) of a live, unlocked member of staff of THIS commune, else 400.
//	household_count   absent or null = NOT ENTERED — never read as 0.
//	population_count  absent or null = NOT ENTERED — never read as 0.
//	order             absent = 0.
//	active            REFUSED (400): a new unit is in use; retiring it is a PATCH, with its own trail.
type createResidentialUnitIn struct {
	Name            string `json:"name"`
	Code            string `json:"code,omitempty"`
	TypeCode        string `json:"type_code,omitempty"`
	HeadStaffCode   string `json:"head_staff_code,omitempty"`
	HouseholdCount  *int   `json:"household_count,omitempty"`
	PopulationCount *int   `json:"population_count,omitempty"`
	Order           *int   `json:"order,omitempty"`
	Active          *bool  `json:"active,omitempty"`
}

// updateResidentialUnitIn is a PARTIAL edit: absent = leave alone.
//
//	type_code / head_staff_code          "" = CLEAR it
//	household_count / population_count   null = CLEAR it ("not entered"), a number = set it
//	active                               false = TAKE OUT OF USE, true = back into use
//	code                                 NOT EDITABLE — declared only so a body naming it is REFUSED
//	                                     with 400 rather than silently ignored.
type updateResidentialUnitIn struct {
	Name            *string         `json:"name,omitempty"`
	Code            *string         `json:"code,omitempty"`
	TypeCode        *string         `json:"type_code,omitempty"`
	HeadStaffCode   *string         `json:"head_staff_code,omitempty"`
	HouseholdCount  optionalCountIn `json:"household_count"`
	PopulationCount optionalCountIn `json:"population_count"`
	Order           *int            `json:"order,omitempty"`
	Active          *bool           `json:"active,omitempty"`
}

// optionalCountIn records whether a count was present at all, and its value when it was a number —
// optionalHoursIn's shape. encoding/json calls UnmarshalJSON for a present field INCLUDING an explicit
// null, and never for an absent one.
type optionalCountIn struct {
	present bool
	value   *int
}

func (o *optionalCountIn) UnmarshalJSON(b []byte) error {
	o.present = true
	if string(b) == "null" {
		o.value = nil
		return nil
	}
	var n int
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	o.value = &n
	return nil
}

func (o optionalCountIn) change() app.OptionalCount {
	return app.OptionalCount{Set: o.present, Value: o.value}
}

func readResidentialUnitBody(w http.ResponseWriter, r *http.Request, in any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxResidentialUnitBody)
	if err := json.NewDecoder(r.Body).Decode(in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Nội dung gửi lên không phải JSON hợp lệ hoặc quá lớn.", "")
		return false
	}
	return true
}

// CreateResidentialUnit adds one unit. POST /api/v1/residential-units
func (h *Handler) CreateResidentialUnit(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var in createResidentialUnitIn
	if !readResidentialUnitBody(w, r, &in) {
		return
	}
	if in.Active != nil {
		httpx.WriteError(w, http.StatusBadRequest, "active_not_settable",
			"Thôn / tổ dân phố mới luôn ở trạng thái đang dùng. Ngưng dùng bằng thao tác sửa sau khi tạo.", "")
		return
	}
	order := 0
	if in.Order != nil {
		order = *in.Order
	}
	// The commune is NOT passed: the use case opens store.DB.For(ctx), which binds tenant_id from the
	// Host-derived context (rule 1, invariant 4).
	u, err := h.d.ResidentialUnits.Create(r.Context(), app.CreateResidentialUnit{
		Name: in.Name, Code: in.Code, TypeCode: in.TypeCode, HeadCode: in.HeadStaffCode,
		Households: in.HouseholdCount, Population: in.PopulationCount, Order: order,
	}, actor)
	if err != nil {
		h.writeResidentialUnitError(w, r, "thêm", err)
		return
	}
	vietJSON(w, http.StatusCreated, thonToDanPhoRaNgoai(u))
}

// UpdateResidentialUnit edits one unit. PATCH /api/v1/residential-units/{id}
func (h *Handler) UpdateResidentialUnit(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var in updateResidentialUnitIn
	if !readResidentialUnitBody(w, r, &in) {
		return
	}
	if in.Code != nil {
		httpx.WriteError(w, http.StatusBadRequest, "code_not_editable",
			"Mã thôn / tổ dân phố đã cấp thì không đổi được.", "")
		return
	}
	// EVERY FIELD ABSENT IS REFUSED, not a silent 200 — see SuaSLA.
	if in.Name == nil && in.TypeCode == nil && in.HeadStaffCode == nil && !in.HouseholdCount.present &&
		!in.PopulationCount.present && in.Order == nil && in.Active == nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Không có trường nào được gửi lên để sửa.", "")
		return
	}
	// The commune is NOT passed: the use case opens store.DB.For(ctx), which binds tenant_id from the
	// Host-derived context, so another commune's id is simply not found (404).
	u, err := h.d.ResidentialUnits.Update(r.Context(), r.PathValue("id"), app.UpdateResidentialUnit{
		Name: in.Name, TypeCode: in.TypeCode, HeadCode: in.HeadStaffCode,
		Households: in.HouseholdCount.change(), Population: in.PopulationCount.change(),
		Order: in.Order, Active: in.Active,
	}, actor)
	if err != nil {
		h.writeResidentialUnitError(w, r, "sửa", err)
		return
	}
	vietJSON(w, http.StatusOK, thonToDanPhoRaNgoai(u))
}

// writeResidentialUnitError maps one use-case failure onto a status and a sentence. One function for
// both routes, so the two cannot drift apart.
//
// 409 AND NOT 403 FOR THE STATE REFUSALS: the caller holds `admin.org`; what is refused is the
// operation against the current state of the commune's list.
func (h *Handler) writeResidentialUnitError(w http.ResponseWriter, r *http.Request, op string, err error) {
	switch {
	case errors.Is(err, idstore.ErrResidentialUnitNotFound):
		// 404 for another commune's id as for an invented one (rule 4, forbidden #2).
		httpx.WriteError(w, http.StatusNotFound, "residential_unit_not_found", "Không tìm thấy thôn / tổ dân phố.", "")
	case errors.Is(err, idstore.ErrResidentialUnitCodeTaken):
		httpx.WriteError(w, http.StatusConflict, "residential_unit_code_taken",
			"Mã này đã được cấp trong xã (kể cả cho đơn vị đã ngưng dùng — mã đã cấp không cấp lại). Hãy chọn mã khác.", "")
	case errors.Is(err, idstore.ErrResidentialUnitNameTaken):
		httpx.WriteError(w, http.StatusConflict, "residential_unit_name_taken",
			"Xã đã có một thôn / tổ dân phố cùng tên (kể cả đơn vị đã ngưng dùng). Nếu đó là đơn vị cũ, hãy dùng lại đơn vị ấy thay vì tạo mới.", "")
	case errors.Is(err, app.ErrResidentialUnitListFull):
		httpx.WriteError(w, http.StatusConflict, "residential_unit_list_full",
			"Danh sách thôn / tổ dân phố của xã đã tới trần, không thêm được nữa.", "")
	case errors.Is(err, idstore.ErrResidentialUnitTypeNotFound):
		httpx.WriteError(w, http.StatusBadRequest, "residential_unit_type_not_found",
			"Loại đơn vị dân cư được chọn không có hoặc đã ngưng dùng trong xã. Hãy tải lại danh mục.", "")
	case errors.Is(err, idstore.ErrHeadStaffNotFound):
		// ONE ANSWER for an invented code, a locked or removed person and another commune's code.
		httpx.WriteError(w, http.StatusBadRequest, "head_staff_not_found",
			"Không tìm thấy cán bộ đang làm việc của xã có mã được chọn làm trưởng thôn / tổ trưởng.", "")
	case app.IsResidentialUnitInputError(err):
		// The domain's own sentence: it names the rule and holds no personal data.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
	default:
		h.d.Log.Error("thôn/tổ dân phố: "+op+" lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}
