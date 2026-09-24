package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The routes behind the commune's organisational chart (14-cau-hinh.md §1).
//
//	GET   /api/v1/org-units        the whole chart, with each unit's staff count — AnyAuthenticated
//	POST  /api/v1/org-units        add a unit                                  — `admin.org`
//	PATCH /api/v1/org-units/{id}   rename, move (change parent), re-rank       — `admin.org`
//
// WHO MAY RESHAPE THE CHART WAS ANSWERED ON 2026-09-24: `admin.org`, the key migration 0001:282
// already seeds ("Quản lý sơ đồ tổ chức"). No key was invented (rule 5, invariant 3c).
//
// THERE IS STILL NO DELETE, and that half of the old question remains open: removing a unit must be
// refused while it holds staff OR is named by records in `documents`, `petitions` or `comms`, and
// this service cannot see the last three without a cross-service contract (rule 2, stop condition
// #2). The `🗑` button of §1 has no route. See app.SoDoToChuc.

// boPhanRa is one node as it leaves the API.
//
// NOTHING HERE IS PERSONAL DATA (rule 3): a unit's name and code describe the authority's
// organisation, not a person. That is what makes the route's AnyAuthenticated declaration a
// question about convenience rather than about privacy — see the reason on the route itself.
type boPhanRa struct {
	ID   string `json:"id"`   // ULID — what other records reference
	Code string `json:"code"` // slug
	Name string `json:"name"`

	// ParentID is "" at the root. A flat list with parent ids, never a nested tree — the argument
	// is on domain.BoPhan.ChaID.
	ParentID string `json:"parent_id"`

	// Order is `thu_tu`, the rank the commune arranged its units in. The list is already sorted by
	// it; it is returned so the edit form can show and change it.
	Order int `json:"order"`

	// StaffCount is how many staff sit in the unit — not soft-deleted, not locked. Who is counted and
	// why: domain.BoPhan.SoCanBo. 0 is printed, never omitted: an empty unit is a fact the screen
	// shows ("0 cán bộ").
	StaffCount int `json:"staff_count"`
}

// boPhanDaGhiRa is what the two write routes return: the node as written, WITHOUT a staff count.
//
// A SEPARATE TYPE rather than boPhanRa with a zero count, because a zero there would be a false
// figure — the write path does not count, and a client redrawing the card from the write response
// would print "0 cán bộ" against a unit of twelve. The screen reloads the list, which does count.
type boPhanDaGhiRa struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	ParentID string `json:"parent_id"`
	Order    int    `json:"order"`
}

// danhSachBoPhanRa wraps the list in an OBJECT rather than returning a bare JSON array.
//
// A bare array cannot grow: the day this needs to say anything about the list itself — that it was
// truncated, when it was last changed — every client has to change shape at once. An object with
// one field costs one line now and nothing later. It is also the shape page.Result already gives
// every other list route, so a client reads `items` on all of them.
//
// THERE IS NO next_cursor AND NO has_more, and their absence is the contract: this route returns
// the WHOLE list or it fails. A `has_more` here would invite exactly the paging behaviour the route
// was designed not to need — see idstore.BoPhanStore.DanhSach.
type danhSachBoPhanRa struct {
	Items []boPhanRa `json:"items"`
}

func boPhanRaNgoai(bp domain.BoPhan) boPhanRa {
	return boPhanRa{ID: bp.ID, Code: bp.Ma, Name: bp.Ten, ParentID: bp.ChaID, Order: bp.ThuTu, StaffCount: bp.SoCanBo}
}

// DanhSachBoPhan serves the commune's org chart. GET /api/v1/org-units
//
// NO AUDIT ENTRY, and that is a decision rather than an omission. Rule 6, invariant 7 audits
// reading FULL personal data and reading ACROSS communes; this is neither — it is a reference list
// of the authority's own units, read inside the commune the request arrived in. An entry for every
// dropdown fill would bury the entries that carry legal weight under thousands that carry none.
//
// NO idem.* DECLARATION: a GET changes no state.
func (h *Handler) DanhSachBoPhan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	bp, err := h.d.BoPhan.DanhSach(ctx)
	if err != nil {
		if errors.Is(err, idstore.ErrQuaNhieuBoPhan) {
			// REFUSED, NOT TRUNCATED. This list fills the boxes work is assigned in, so a list
			// missing a unit sends work to the wrong one with nothing on the screen to show it. The
			// log line names the commune because that is the only thing an operator can act on —
			// the ceiling is about fifty times a real org chart, so reaching it means the data is
			// wrong, not that the commune is large.
			h.d.Log.Error("danh mục bộ phận vượt trần — TỪ CHỐI thay vì cắt bớt",
				"xa", string(tenant.MustFrom(ctx)), "tran", idstore.TranDanhMucBoPhan)
			httpx.WriteError(w, http.StatusInternalServerError, "internal",
				"Đã xảy ra lỗi. Vui lòng thử lại.", "")
			return
		}
		// The wrapped error carries the store failure and never reaches the client (rule 3,
		// forbidden #3).
		h.d.Log.Error("danh mục bộ phận: lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// make(..., 0, ...) and not a nil slice: `items` must marshal as [] on a commune that has not
	// set its org chart up yet, never as null. A newly onboarded commune is exactly that, and a
	// client that has to handle both shapes handles one of them wrong.
	//
	// THE ORDER IS THE STORE'S and is not touched here: `thu_tu` is the order the commune arranged
	// its own units in, and a handler that re-sorted would silently overrule it.
	ra := danhSachBoPhanRa{Items: make([]boPhanRa, 0, len(bp))}
	for _, mot := range bp {
		ra.Items = append(ra.Items, boPhanRaNgoai(mot))
	}
	vietJSON(w, http.StatusOK, ra)
}

// thanBoPhanToiDa bounds a write body: four short fields.
const thanBoPhanToiDa = 8 << 10

// themBoPhanVao is the create body.
//
//	name       required. Case kept as typed — upper case is the commune's convention (§1), not a
//	           rule this service rewrites into an archival record.
//	parent_id  absent or "" = the root; otherwise a live unit of THIS commune, else 400.
//	order      absent = 0.
//	code       absent = derived from the name ("VĂN PHÒNG ĐẢNG ỦY" → "van-phong-dang-uy"), with
//	           `-2`, `-3`… when taken. PRESENT = used exactly, or 409 when taken — never suffixed
//	           behind the person's back. Either way a code taken by a SOFT-DELETED unit is taken
//	           (rule 7, invariant 3).
type themBoPhanVao struct {
	Name     string `json:"name"`
	ParentID string `json:"parent_id,omitempty"`
	Order    *int   `json:"order,omitempty"`
	Code     string `json:"code,omitempty"`
}

// suaBoPhanVao is a PARTIAL edit: absent or null = leave alone.
//
//	parent_id  "" = MOVE TO THE ROOT (the value GET prints for a root); an id = move under it.
//	code       NOT EDITABLE. Declared only so a body that names it is REFUSED with 400 rather than
//	           silently ignored — a client that believes it renamed a code must be told it did not.
type suaBoPhanVao struct {
	Name     *string `json:"name,omitempty"`
	ParentID *string `json:"parent_id,omitempty"`
	Order    *int    `json:"order,omitempty"`
	Code     *string `json:"code,omitempty"`
}

func docThanBoPhan(w http.ResponseWriter, r *http.Request, vao any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, thanBoPhanToiDa)
	if err := json.NewDecoder(r.Body).Decode(vao); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Nội dung gửi lên không phải JSON hợp lệ hoặc quá lớn.", "")
		return false
	}
	return true
}

func boPhanDaGhiRaNgoai(bp domain.BoPhan) boPhanDaGhiRa {
	return boPhanDaGhiRa{ID: bp.ID, Code: bp.Ma, Name: bp.Ten, ParentID: bp.ChaID, Order: bp.ThuTu}
}

// ThemBoPhan adds one unit. POST /api/v1/org-units
func (h *Handler) ThemBoPhan(w http.ResponseWriter, r *http.Request) {
	nguoi, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var than themBoPhanVao
	if !docThanBoPhan(w, r, &than) {
		return
	}
	bp, err := h.d.GhiBoPhan.Them(r.Context(), app.YeuCauThemBoPhan{
		Ten: than.Name, ChaID: than.ParentID, ThuTu: than.Order, Ma: than.Code,
	}, nguoi)
	if err != nil {
		h.traLoiLoiGhiBoPhan(w, r, "thêm bộ phận", err)
		return
	}
	vietJSON(w, http.StatusCreated, boPhanDaGhiRaNgoai(bp))
}

// SuaBoPhan renames, moves or re-ranks one unit. PATCH /api/v1/org-units/{id}
func (h *Handler) SuaBoPhan(w http.ResponseWriter, r *http.Request) {
	nguoi, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var than suaBoPhanVao
	if !docThanBoPhan(w, r, &than) {
		return
	}
	if than.Code != nil {
		httpx.WriteError(w, http.StatusBadRequest, "code_not_editable",
			"Mã bộ phận đã cấp thì không đổi được. Chỉ sửa được tên, bộ phận cha và thứ tự.", "")
		return
	}
	// EVERY FIELD ABSENT IS REFUSED, not a silent 200 — see SuaSLA.
	if than.Name == nil && than.ParentID == nil && than.Order == nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Không có trường nào được gửi lên để sửa.", "")
		return
	}
	bp, err := h.d.GhiBoPhan.Sua(r.Context(), r.PathValue("id"), app.YeuCauSuaBoPhan{
		Ten: than.Name, ChaID: than.ParentID, ThuTu: than.Order,
	}, nguoi)
	if err != nil {
		h.traLoiLoiGhiBoPhan(w, r, "sửa bộ phận", err)
		return
	}
	vietJSON(w, http.StatusOK, boPhanDaGhiRaNgoai(bp))
}

// traLoiLoiGhiBoPhan maps one use-case failure onto a status and a sentence. One function for both
// routes, so the two cannot drift apart.
//
// 409 AND NOT 403 FOR THE TWO STATE REFUSALS: the caller holds `admin.org`; what is refused is the
// operation against the current state of the chart, and a 403 would send an administrator to the
// Phân quyền screen for a right they already have.
func (h *Handler) traLoiLoiGhiBoPhan(w http.ResponseWriter, r *http.Request, viec string, err error) {
	switch {
	case errors.Is(err, idstore.ErrKhongTimThayBoPhan):
		// 404 for another commune's id as for an invented or soft-deleted one (rule 4, forbidden #2).
		httpx.WriteError(w, http.StatusNotFound, "org_unit_not_found", "Không tìm thấy bộ phận.", "")
	case errors.Is(err, idstore.ErrBoPhanChaKhongTonTai):
		httpx.WriteError(w, http.StatusBadRequest, "parent_not_found",
			"Bộ phận cha được chọn không còn trong xã. Hãy tải lại sơ đồ tổ chức.", "")
	case errors.Is(err, idstore.ErrMaBoPhanDaDung):
		httpx.WriteError(w, http.StatusConflict, "org_unit_code_taken",
			"Mã bộ phận này đã được dùng trong xã (kể cả bởi bộ phận đã xoá — mã đã cấp không cấp lại). Hãy chọn mã khác.", "")
	case errors.Is(err, app.ErrCayBoPhanVongLap):
		httpx.WriteError(w, http.StatusConflict, "org_unit_cycle",
			"Không thể dời một bộ phận vào dưới chính nó hay dưới một bộ phận con của nó.", "")
	case app.LaLoiDauVaoBoPhan(err):
		// The domain's own sentence: it names the rule and holds no personal data (unit names are the
		// authority's organisation, not a person).
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
	default:
		h.d.Log.Error("sơ đồ tổ chức: "+viec+" lỗi hệ thống",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}
