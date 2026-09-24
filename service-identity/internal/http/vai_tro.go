package http

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The read route behind the commune's role catalogue. GET /api/v1/roles
//
// GRANTING A ROLE PERMISSIONS IS PUT /api/v1/roles/{id}/permissions (internal/http/quyen.go), built
// once #13 and #14 were decided (2026-09-22). CREATING, RENAMING OR REMOVING a role has no route:
// what happens to the people holding a role that is removed has not been asked, and a half-written
// write path looks like a decision somebody made.

// vaiTroMucRa is one role as it leaves the catalogue.
//
// IT CARRIES `id` WHILE phienHienTaiRa.role CARRIES ONLY `code`, and the difference is the job each
// one does. The catalogue exists so a client can turn `staff.role_id` — the ULID the staff list
// returns — into a name, so it must hand back the very value that list references. The session
// surface names the caller's own role for display and for choosing a landing screen; a slug is the
// stable thing to key that on, and handing out an internal id there would be an id nobody needs.
//
// NOTHING HERE IS PERSONAL DATA (rule 3): a role's name describes the authority's structure, not a
// person. That is what makes the route's AnyAuthenticated declaration a question about convenience
// rather than about privacy — see the reason on the route itself.
type vaiTroMucRa struct {
	ID   string `json:"id"`   // ULID — what nguoi_dung.vai_tro_id references
	Code string `json:"code"` // slug
	Name string `json:"name"`

	// IsLeader CHOOSES THE DEFAULT SCREEN AND NOTHING ELSE. The whole argument is on
	// domain.VaiTro.LaLanhDao, and a catalogue is where that line is easiest to cross: a client
	// holding the entire list can branch on any field in it. Reading this to decide whether an
	// operation is allowed gives the project a second authorisation system that does not go through
	// `(tenant_id, role, permission)` (rule 5, forbidden #2 and #3).
	IsLeader bool `json:"is_leader"`
}

// danhSachVaiTroRa wraps the list in an OBJECT rather than a bare JSON array — same reasoning as
// danhSachBoPhanRa, and the same absence of `next_cursor`/`has_more`: this route returns the WHOLE
// list or it fails.
type danhSachVaiTroRa struct {
	Items []vaiTroMucRa `json:"items"`
}

func vaiTroMucRaNgoai(vt domain.VaiTro) vaiTroMucRa {
	return vaiTroMucRa{ID: vt.ID, Code: vt.Ma, Name: vt.Ten, IsLeader: vt.LaLanhDao}
}

// DanhSachVaiTro serves the commune's role catalogue. GET /api/v1/roles
//
// NO AUDIT ENTRY: rule 6, invariant 7 audits reading FULL personal data and reading ACROSS communes.
// This is neither — a reference list of the authority's own roles, read inside the commune the
// request arrived in. An entry per dropdown fill would bury the entries that carry legal weight.
//
// NO idem.* DECLARATION: a GET changes no state.
func (h *Handler) DanhSachVaiTro(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vt, err := h.d.VaiTroMuc.DanhSach(ctx)
	if err != nil {
		if errors.Is(err, idstore.ErrQuaNhieuVaiTro) {
			// REFUSED, NOT TRUNCATED. This list fills the role picker on the staff form, so a short
			// list means the next person created gets the wrong role — and a wrong role is a wrong
			// set of permissions, which nothing on the screen shows.
			h.d.Log.Error("danh mục vai trò vượt trần — TỪ CHỐI thay vì cắt bớt",
				"xa", string(tenant.MustFrom(ctx)), "tran", idstore.TranDanhMucVaiTro)
			httpx.WriteError(w, http.StatusInternalServerError, "internal",
				"Đã xảy ra lỗi. Vui lòng thử lại.", "")
			return
		}
		// The wrapped error carries the store failure and never reaches the client (rule 3,
		// forbidden #3).
		h.d.Log.Error("danh mục vai trò: lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// make(..., 0, ...) and not a nil slice: `items` must marshal as [] on a commune whose roles
	// have not been set up yet, never as null.
	//
	// THE ORDER IS THE STORE'S and is not touched here: `thu_tu` is the order the commune arranged
	// its own roles in, and a handler that re-sorted would silently overrule it.
	ra := danhSachVaiTroRa{Items: make([]vaiTroMucRa, 0, len(vt))}
	for _, mot := range vt {
		ra.Items = append(ra.Items, vaiTroMucRaNgoai(mot))
	}
	vietJSON(w, http.StatusOK, ra)
}
