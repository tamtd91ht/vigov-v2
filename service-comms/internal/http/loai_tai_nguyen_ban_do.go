package http

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// The read route behind the commune's map-asset-type catalogue. GET /api/v1/map-asset-types
//
// THERE IS NO WRITE ROUTE, AND NO SCAFFOLDING FOR ONE IS LEFT HERE. Open question #21 — whether a
// commune may edit the CODE LIST or only the labels and the order — is unanswered, and a write
// route decides it silently: the first commune to add a code makes "the codes are the platform's"
// false, and rule 7 does not allow taking that back. A half-written write path also reads like a
// decision somebody made.

// loaiTaiNguyenRa is one catalogue row as it leaves the API.
//
// NOTHING HERE IS PERSONAL DATA (rule 3): a group's code and label describe how the commune files
// what is on its map, not a person. That is what makes the route's AnyAuthenticated declaration a
// question about convenience rather than about privacy — see the reason on the route itself.
//
// `label` AND NOT `name`, AND THE QUESTION WAS ASKED TWICE — the first answer is kept here because
// it was wrong for a reason worth seeing, not because it was careless.
//
//	FIRST ANSWER, `name`   `org-units` and `roles` both answer "what is this row called" in a field
//	                       named `name`, and the admin web renders catalogues through one picker. A
//	                       second word for the same SLOT would put a per-catalogue branch in it.
//	WHY IT MOVED           right about the cost, wrong about which rows are the same kind of thing.
//	                       The line is drawn at the SCHEMA: `bo_phan` and `vai_tro` carry `ten` — a
//	                       NAME, something a unit or a role HAS — while every ADR 0024 catalogue
//	                       carries `nhan`, a LABEL, and a label is precisely the one thing the tier
//	                       trigger permits a commune to re-word (migration 0003; `ma` is immutable).
//	                       ADR 0017 decides it: a contract field is named after what the data IS,
//	                       not after the screen slot it happens to fill.
//
// THE PICKER ARGUMENT SURVIVES INTACT, which is why the move costs nothing: catalogues answer
// `label` and named entities answer `name`, so the web carries TWO shapes for two kinds of thing
// rather than one branch per catalogue. `service-identity` drew the same line in the same session
// (`khoi_nhiem_vu`, `loai_don_vi_dan_cu` → `label`; `thon_to_dan_pho` → `name`), and
// `service-documents` reached it independently (`loai_van_ban` → `label`).
//
// It was changed while it was still free: no `make kb` has regenerated kb/20-contracts/openapi.json
// since this route was written, so no client has ever seen `name` here.
type loaiTaiNguyenRa struct {
	ID    string `json:"id"`    // ULID — what a map asset record references
	Code  string `json:"code"`  // `ma`: slug "doanh-nghiep", the value asset records store
	Label string `json:"label"` // `nhan`: the wording a commune may change without touching `ma`

	// IsDefault marks the group the map's selector opens on. At most one row in the list carries
	// it; a list where none does is normal — a commune need not nominate one.
	IsDefault bool `json:"is_default"`

	// Active is false for a group taken out of use. Such a row IS in the list: the catalogue screen
	// shows it with a "Đã tắt" chip, and an asset already filed under it still has to render its
	// group's name. A consumer filling a picker filters on this; a consumer rendering a stored value
	// does not.
	Active bool `json:"active"`
}

// danhSachLoaiTaiNguyenRa wraps the list in an OBJECT rather than returning a bare JSON array.
//
// A bare array cannot grow: the day this needs to say anything about the list itself, every client
// has to change shape at once. It is also the shape page.Result gives every other list route, so a
// client reads `items` on all of them.
//
// THERE IS NO next_cursor AND NO has_more, and their absence is the contract: this route returns
// the WHOLE list or it fails. A `has_more` here would invite exactly the paging behaviour the
// route was designed not to need — see commsstore.LoaiTaiNguyenBanDoStore.DanhSach.
type danhSachLoaiTaiNguyenRa struct {
	Items []loaiTaiNguyenRa `json:"items"`
}

func loaiTaiNguyenRaNgoai(lt domain.LoaiTaiNguyenBanDo) loaiTaiNguyenRa {
	return loaiTaiNguyenRa{
		ID:        lt.ID,
		Code:      lt.Ma,
		Label:     lt.Nhan,
		IsDefault: lt.LaMacDinh,
		Active:    lt.DangDung,
	}
}

// DanhSachLoaiTaiNguyen serves the commune's map-asset-type catalogue.
// GET /api/v1/map-asset-types
//
// NO AUDIT ENTRY, and that is a decision rather than an omission. Rule 6, invariant 7 audits
// reading FULL personal data and reading ACROSS communes; this is neither — it is a reference list
// of how one commune files its own map, read inside the commune the request arrived in. An entry
// for every dropdown fill would bury the entries that carry legal weight under thousands that
// carry none.
//
// AN EMPTY LIST IS A CORRECT ANSWER, NOT A FAULT. The table ships empty for every commune on
// purpose (migration 0003), so `{"items":[]}` is what every commune gets today. Nothing here
// substitutes a default row, and nothing here treats emptiness as an error: a visibly empty
// catalogue is the failure people report, unlike a half-seeded one that looks complete.
//
// NO idem.* DECLARATION: a GET changes no state.
func (h *Handler) DanhSachLoaiTaiNguyen(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	dm, err := h.d.LoaiTaiNguyen.DanhSach(ctx)
	if err != nil {
		if errors.Is(err, commsstore.ErrQuaNhieuLoaiTaiNguyen) {
			// REFUSED, NOT TRUNCATED. This list fills the group selector on the economic map, so a
			// list missing a group files an asset under the wrong one with nothing on the screen to
			// show it. The log line names the commune because that is the only thing an operator can
			// act on — the ceiling is about forty times a real catalogue, so reaching it means the
			// data is wrong, not that the commune is large.
			h.d.Log.Error("danh mục loại tài nguyên bản đồ vượt trần — TỪ CHỐI thay vì cắt bớt",
				"xa", string(tenant.MustFrom(ctx)), "tran", commsstore.TranDanhMucLoaiTaiNguyen)
			httpx.WriteError(w, http.StatusInternalServerError, "internal",
				"Đã xảy ra lỗi. Vui lòng thử lại.", "")
			return
		}
		// The wrapped error carries the store failure and never reaches the client (rule 3,
		// forbidden #3).
		h.d.Log.Error("danh mục loại tài nguyên bản đồ: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// make(..., 0, ...) and not a nil slice: `items` must marshal as [] and never as null. Today
	// EVERY commune is in that case — the table ships empty — so a nil slice here would make `null`
	// the answer the whole system gets, and a client that has to handle both shapes handles one of
	// them wrong.
	//
	// THE ORDER IS THE STORE'S and is not touched here: `thu_tu` is the order the commune arranged
	// its own groups in, and a handler that re-sorted would silently overrule it.
	ra := danhSachLoaiTaiNguyenRa{Items: make([]loaiTaiNguyenRa, 0, len(dm))}
	for _, mot := range dm {
		ra.Items = append(ra.Items, loaiTaiNguyenRaNgoai(mot))
	}
	vietJSON(w, http.StatusOK, ra)
}
