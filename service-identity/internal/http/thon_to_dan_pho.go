package http

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The read route behind the commune's residential units. GET /api/v1/residential-units
//
// THERE IS NO WRITE ROUTE, and no scaffolding for one is left here. Creating a hamlet, merging two,
// or taking one out of use are administrative acts against a record the commune's business data
// references; who may perform them, and what happens to the petitions and households already
// pointing at a unit that disappears, is nobody's answered question yet.

// thonToDanPhoRa is one residential unit as it leaves the API.
//
// ---------------------------------------------------------------------------------------------
// THE RESPONSE SHAPE DECISION, ARGUED RATHER THAN ASSUMED: the unit's type appears here as TWO FLAT
// FIELDS — the code and the label — and not as an embedded type object, and not as the code alone.
//
//	code alone          every screen that lists hamlets shows a `Loại` column with "Thôn" in it
//	                    (docs/ui-ux/14-cau-hinh.md §5). With only the code, that screen must call
//	                    GET /api/v1/residential-unit-types as well and join the two in the browser.
//	                    Two routes to render one list is a cost paid on every load, and the join is
//	                    written once per client — the admin web, the Mini App, whatever comes next —
//	                    each with its own idea of what to show when the code matches nothing.
//	embedded object     `"type": {id, code, label, is_default, is_active}` makes this response
//	                    change shape every time the catalogue's own shape changes, and hands a
//	                    caller fields it has no use for. Worse, it invites the client to believe it
//	                    now holds the catalogue — and a catalogue assembled from whichever types
//	                    happen to be in use is missing exactly the rows nobody has used yet.
//	two flat fields     the label is display-only and the code stays the key. A client renders the
//	                    label and keys on the code; a client that needs the full catalogue (a
//	                    picker, a filter with all options) calls the catalogue route, which is what
//	                    it is for.
//
// THE LABEL COSTS ONE LEFT JOIN IN ONE STATEMENT, not one lookup per row — the N+1 that
// skills/load-data-once exists to prevent. The reasoning for each clause of that join is on
// idstore.truyVanThonToDanPho.
// ---------------------------------------------------------------------------------------------
//
// NOTHING HERE IS PERSONAL DATA (rule 3). The two counts are counts of a territory and name nobody;
// they are the aggregate figures the commune already publishes about itself.
type thonToDanPhoRa struct {
	ID   string `json:"id"`   // ULID — what other records reference
	Code string `json:"code"` // slug: "thon-binh-an" — immutable once issued
	Name string `json:"name"` // "Thôn Bình An" — a unit has a NAME, unlike a catalogue row's label

	// TypeCode is the code from the residential-unit-type catalogue, "" when the classification has
	// not been entered — a legitimate state, shown as "—" on the commune's own screen.
	TypeCode string `json:"type_code"`

	// TypeLabel is that type's label, read in the same query.
	//
	// IT CAN BE "" WHILE TypeCode IS NOT, and a client must render that pair rather than treat it
	// as an error: it means the type row was soft-deleted while units still carry its code. The
	// unit survives on purpose — dropping it would turn untidy catalogue data into a hamlet missing
	// from the commune's own list. Fall back to showing the code.
	TypeLabel string `json:"type_label"`

	// HouseholdCount and PopulationCount ARE POINTERS SO THAT `null` SURVIVES THE WHOLE WAY OUT.
	//
	// 0 is a statement about a unit ("no households"); null is the absence of one ("not entered").
	// The schema keeps them apart (migration 0005:385), the domain type keeps them apart, and
	// flattening them here would undo both at the last step — a commune that imported a spreadsheet
	// without those columns would be publishing zeros it never asserted, and a zero travels onward
	// into a report as a number.
	HouseholdCount  *int `json:"household_count"`
	PopulationCount *int `json:"population_count"`

	// IsActive is false for a unit taken out of use. Returned rather than filtered server-side, so
	// the list screen can show it while a picker filters it out — same reading as the catalogues.
	IsActive bool `json:"is_active"`
}

// danhSachThonToDanPhoRa wraps the list in an OBJECT rather than a bare JSON array — same reasoning
// as danhSachBoPhanRa, and the same absence of `next_cursor` / `has_more`: this route returns the
// WHOLE list or it fails.
type danhSachThonToDanPhoRa struct {
	Items []thonToDanPhoRa `json:"items"`
}

func thonToDanPhoRaNgoai(t domain.ThonToDanPho) thonToDanPhoRa {
	return thonToDanPhoRa{
		ID:              t.ID,
		Code:            t.Ma,
		Name:            t.Ten,
		TypeCode:        t.LoaiMa,
		TypeLabel:       t.LoaiNhan,
		HouseholdCount:  t.SoHo,
		PopulationCount: t.NhanKhau,
		IsActive:        t.DangDung,
	}
}

// DanhSachThonToDanPho serves the commune's residential units. GET /api/v1/residential-units
//
// NO AUDIT ENTRY: rule 6, invariant 7 audits reading FULL personal data and reading ACROSS
// communes. This is neither — the commune's own list of its own hamlets, read inside the commune
// the request arrived in.
//
// NO idem.* DECLARATION: a GET changes no state.
func (h *Handler) DanhSachThonToDanPho(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ds, err := h.d.ThonToDanPho.DanhSach(ctx)
	if err != nil {
		if errors.Is(err, idstore.ErrQuaNhieuThonToDanPho) {
			// REFUSED, NOT TRUNCATED. This list fills the address picker on a petition and every
			// filter that groups the commune's work by area; a silently short list files work
			// against the wrong place, and the screen looks entirely normal. The log line names the
			// commune because that is the only thing an operator can act on.
			h.d.Log.Error("danh sách thôn/tổ dân phố vượt trần — TỪ CHỐI thay vì cắt bớt",
				"xa", string(tenant.MustFrom(ctx)), "tran", idstore.TranDanhSachThonToDanPho)
			httpx.WriteError(w, http.StatusInternalServerError, "internal",
				"Đã xảy ra lỗi. Vui lòng thử lại.", "")
			return
		}
		// The wrapped error carries the store failure and never reaches the client (rule 3,
		// forbidden #3).
		h.d.Log.Error("danh sách thôn/tổ dân phố: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// make(..., 0, ...) and not a nil slice: `items` must marshal as [] on a commune that has not
	// entered its residential units yet, never as null. That is every commune today — migration
	// 0005 creates the table and seeds nothing, deliberately (0005:38).
	//
	// THE ORDER IS THE STORE'S and is not touched here: by name, which is what the commune's own
	// list screen shows, with the code breaking ties so the order is total.
	ra := danhSachThonToDanPhoRa{Items: make([]thonToDanPhoRa, 0, len(ds))}
	for _, mot := range ds {
		ra.Items = append(ra.Items, thonToDanPhoRaNgoai(mot))
	}
	vietJSON(w, http.StatusOK, ra)
}
