package http

// The edge of "Loại đơn thư" on the Cấu hình › Danh mục screen (bug sheet row 33; prototype
// LookupTable.tsx:37). Routes and permissions: routes_citizen_letter_type.go. What a row is:
// domain/citizen_letter_type.go — the commune's label for one of C4's four statutory codes.
//
// THE ROW HAS THE DOCUMENT-TYPE SHAPE (loaiVanBanRa) so the screen's one table draws it with the same
// columns and the same buttons: `source` is always `don-vi` and `tier` always 1 (every row is the
// commune's own — soft delete, switch off and relabel all allowed), `is_default` always false (the
// prototype offers a default for `task_kind` only, LookupTable.tsx:32). All three are DERIVED constants,
// never stored, and refused on the way in exactly as on document-types.

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-documents/internal/app"
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

// citizenLetterTypeOut is one row as it leaves the API. Nothing here is personal data.
type citizenLetterTypeOut struct {
	ID        string  `json:"id"`
	Code      string  `json:"code"`  // kien-nghi-phan-anh · khieu-nai · to-cao · de-nghi
	Label     string  `json:"label"` // the commune's wording
	Active    bool    `json:"active"`
	IsDefault bool    `json:"is_default"` // always false — no default for this group
	Order     int     `json:"order"`
	Source    string  `json:"source"` // always `don-vi`
	Tier      int     `json:"tier"`   // always 1
	Color     *string `json:"color"`
}

type citizenLetterTypeListOut struct {
	Items []citizenLetterTypeOut `json:"items"`
}

func citizenLetterTypeOutOf(e domain.CitizenLetterTypeEntry) citizenLetterTypeOut {
	return citizenLetterTypeOut{
		ID: e.ID, Code: string(e.Code), Label: e.Label, Active: e.Active, Order: e.Order,
		Source: domain.NguonDonVi, Tier: int(domain.TangDonVi), Color: colorOut(e.Color),
	}
}

// addCitizenLetterTypeIn is the body of POST. `source` / `tier` / `is_default` are declared only to be
// refused (400): none of them is the client's to set here.
type addCitizenLetterTypeIn struct {
	Code  string  `json:"code"` // one of the four statutory codes; anything else is 400
	Label string  `json:"label"`
	Order int     `json:"order,omitempty"`
	Color *string `json:"color,omitempty"` // `#RRGGBB`; absent or null = none

	IsDefault *bool   `json:"is_default,omitempty"`
	Source    *string `json:"source,omitempty"`
	Tier      *int    `json:"tier,omitempty"`
}

// editCitizenLetterTypeIn is the body of PATCH. Pointers: absent = leave alone. `code` is refused — an
// issued code is never renumbered (rule 7, invariant 3).
type editCitizenLetterTypeIn struct {
	Label  *string         `json:"label,omitempty"`
	Order  *int            `json:"order,omitempty"`
	Active *bool           `json:"active,omitempty"`
	Color  optionalColorIn `json:"color,omitempty"` // absent = keep; null = clear; `#RRGGBB` = set

	Code      *string `json:"code,omitempty"`
	IsDefault *bool   `json:"is_default,omitempty"`
	Source    *string `json:"source,omitempty"`
	Tier      *int    `json:"tier,omitempty"`
}

// removeCitizenLetterTypeIn is the body of DELETE. `reason` is OPTIONAL (owner decision 10/10/2026):
// blank stores domain.CitizenLetterTypeDeleteDefaultReason. In the body, never the query string.
type removeCitizenLetterTypeIn struct {
	Reason string `json:"reason,omitempty"`
}

// errDefaultNotForLetterTypes — `is_default` named on a letter type.
var errDefaultNotForLetterTypes = errors.New("danh_muc: nhóm Loại đơn thư không có mục mặc định — bỏ `is_default` khỏi yêu cầu")

// ListCitizenLetterTypes — GET /api/v1/citizen-letter-types. No audit entry: a reference list, read
// inside the request's commune. `items` is [] (never null) on a commune that has added none.
func (h *Handler) ListCitizenLetterTypes(w http.ResponseWriter, r *http.Request) {
	rows, err := h.d.CitizenLetterTypeList.ListEntries(r.Context())
	if err != nil {
		h.citizenLetterTypeError(w, r, "đọc", err)
		return
	}
	out := citizenLetterTypeListOut{Items: make([]citizenLetterTypeOut, 0, len(rows))}
	for _, e := range rows {
		out.Items = append(out.Items, citizenLetterTypeOutOf(e))
	}
	vietJSON(w, http.StatusOK, out)
}

// AddCitizenLetterType — POST /api/v1/citizen-letter-types
func (h *Handler) AddCitizenLetterType(w http.ResponseWriter, r *http.Request) {
	var in addCitizenLetterTypeIn
	if !docThan(w, r, &in) {
		return
	}
	switch {
	case in.Source != nil || in.Tier != nil:
		h.citizenLetterTypeError(w, r, "thêm", domain.ErrNguonDoTuClient)
		return
	case in.IsDefault != nil && *in.IsDefault:
		h.citizenLetterTypeError(w, r, "thêm", errDefaultNotForLetterTypes)
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	e, err := h.d.CitizenLetterTypes.AddType(r.Context(), app.AddLetterTypeRequest{
		Code: in.Code, Label: in.Label, Order: in.Order, Color: in.Color,
	}, actor)
	if err != nil {
		h.citizenLetterTypeError(w, r, "thêm", err)
		return
	}
	idem.RecordCode(r.Context(), string(e.Code))
	vietJSON(w, http.StatusCreated, citizenLetterTypeOutOf(e))
}

// EditCitizenLetterType — PATCH /api/v1/citizen-letter-types/{id}
func (h *Handler) EditCitizenLetterType(w http.ResponseWriter, r *http.Request) {
	var in editCitizenLetterTypeIn
	if !docThan(w, r, &in) {
		return
	}
	switch {
	case in.Source != nil || in.Tier != nil:
		h.citizenLetterTypeError(w, r, "sửa", domain.ErrNguonDoTuClient)
		return
	case in.Code != nil:
		h.citizenLetterTypeError(w, r, "sửa", domain.ErrMaBatBien)
		return
	case in.IsDefault != nil && *in.IsDefault:
		h.citizenLetterTypeError(w, r, "sửa", errDefaultNotForLetterTypes)
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	e, err := h.d.CitizenLetterTypes.EditType(r.Context(), r.PathValue("id"), app.EditLetterTypeRequest{
		Label: in.Label, Order: in.Order, Active: in.Active, Color: in.Color.change(),
	}, actor)
	if err != nil {
		h.citizenLetterTypeError(w, r, "sửa", err)
		return
	}
	vietJSON(w, http.StatusOK, citizenLetterTypeOutOf(e))
}

// RemoveCitizenLetterType — DELETE /api/v1/citizen-letter-types/{id}. 204, no body. A request with no
// body at all is the ordinary "no reason given" case, not a 400.
func (h *Handler) RemoveCitizenLetterType(w http.ResponseWriter, r *http.Request) {
	var in removeCitizenLetterTypeIn
	if r.ContentLength != 0 {
		if !docThan(w, r, &in) {
			return
		}
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	if err := h.d.CitizenLetterTypes.RemoveType(r.Context(), r.PathValue("id"), in.Reason, actor); err != nil {
		h.citizenLetterTypeError(w, r, "xoá", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// citizenLetterTypeError maps one failure onto ONE status and ONE sentence.
func (h *Handler) citizenLetterTypeError(w http.ResponseWriter, r *http.Request, op string, err error) {
	switch {
	case errors.Is(err, docstore.ErrDanhMucKhongTonTai):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy mục danh mục này.", "")
	case errors.Is(err, docstore.ErrMaDaTonTai):
		httpx.WriteError(w, http.StatusConflict, "code_taken",
			"Loại đơn này đã có một dòng trong danh mục của xã — kể cả khi dòng ấy đã bị xoá. "+
				"Mã đã cấp thì không cấp lại; hãy sửa nhãn hoặc bật lại dòng đang có.", "")
	case laLoiDauVao(err) || errors.Is(err, domain.ErrLetterTypeCodeNotStatutory) ||
		errors.Is(err, errDefaultNotForLetterTypes):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
	default:
		h.d.Log.Error("danh mục loại đơn thư: "+op+" lỗi hệ thống",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}
