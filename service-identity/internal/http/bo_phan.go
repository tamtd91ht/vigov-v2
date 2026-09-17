package http

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The read route behind the commune's organisational chart. GET /api/v1/org-units
//
// THERE IS NO WRITE ROUTE, and no scaffolding for one is left here. Who may reshape a commune's org
// chart, and what happens to the people and the documents attached to a unit that is removed, are
// questions nobody has answered — and a half-written write path looks like a decision somebody made.

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
	return boPhanRa{ID: bp.ID, Code: bp.Ma, Name: bp.Ten, ParentID: bp.ChaID}
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
