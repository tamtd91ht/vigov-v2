package http

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The read route behind the commune's residential-unit-type catalogue.
// GET /api/v1/residential-unit-types
//
// THERE IS NO WRITE ROUTE, and no scaffolding for one is left here. Whether a commune may edit the
// CODE LIST at all — or only the labels and the order — is open question #21 and nobody has
// answered it. The migration already refuses the dangerous half in the database (a tier-3 row
// cannot be disabled or deleted); what it does not do is decide who may add a row, and a
// half-written write path looks like a decision somebody made.

// loaiDonViDanCuRa is one catalogue entry as it leaves the API.
//
// NOTHING HERE IS PERSONAL DATA (rule 3): a classification's code and label describe how the
// commune organises its territory, not a person. That is what makes the route's AnyAuthenticated
// declaration a question about convenience rather than about privacy — see the reason on the route.
//
// WHAT IS DELIBERATELY ABSENT, AND THE GAP IS STATED RATHER THAN GLOSSED:
//
//	nguon, ma_nguon_re_nhanh   they answer "what may be DONE to this row" (migration 0005:76), and
//	                           nothing may be done to it through this API — there is no write
//	                           route. Publishing them would describe a write surface that does not
//	                           exist, and a client greying out a button from them would be
//	                           enforcing in the browser a rule the server is the one enforcing
//	                           (rule 5, forbidden #1).
//	thu_tu                     the order is already carried, by the order of `items`. The NUMBER is
//	                           only of use to a screen that edits it.
//
// THE Danh mục TAB (docs/ui-ux/14-cau-hinh.md §5) SHOWS BOTH OF THOSE AS COLUMNS, so the day the
// write route exists it will want them. Adding a field to this object then is additive and breaks
// no client; publishing it now, for a screen that cannot act on it, is a field that has to be kept
// correct forever for nobody.
type loaiDonViDanCuRa struct {
	ID   string `json:"id"`   // ULID — what a later reference would point at
	Code string `json:"code"` // slug: "thon" — the value thon_to_dan_pho.loai holds

	// Label, AND NOT `name` AS ON /org-units AND /roles. The distinction is the schema's own and it
	// is worth keeping: a bộ phận has a `ten` — it is a thing with a name — while a catalogue row
	// has a `nhan`, a label put on a code. The code is the thing; the label is what is written on
	// it, and it may be re-worded (the trigger permits exactly that, and only that) while the code
	// may not. `name` here would invite a client to key on it.
	Label string `json:"label"` // "Thôn" — what a person reads

	// IsDefault is the row a form pre-selects. At most one per commune, guaranteed by the schema —
	// the reason it is carried out at all is on domain.LoaiDonViDanCu.LaMacDinh.
	IsDefault bool `json:"is_default"`

	// Active is false for a row the commune has taken out of use. IT IS RETURNED RATHER THAN
	// FILTERED SERVER-SIDE so one route can serve both consumers: the catalogue screen, which must
	// show a disabled row with its "Đã tắt" chip, and a picker, which must not offer it. Dropping
	// the flag would put a disabled option in a form with nothing on the screen to show it.
	//
	// `active` AND NOT `is_active`, BESIDE AN `is_default` — THE ASYMMETRY IS DELIBERATE AND SHARED,
	// SO DO NOT "FIX" EITHER HALF OF IT. Five sibling catalogue routes answer the same pair
	// (service-documents/internal/http/loai_van_ban.go:43,48 and four more), and this service already
	// shipped `active` on a route web-admin consumes (can_bo.go:51). One concept spelled two ways
	// across one contract forces a client writing a single catalogue reader to branch on which
	// service answered — the drift that cost four services a rename today.
	Active bool `json:"active"`
}

// danhSachLoaiDonViDanCuRa wraps the list in an OBJECT rather than returning a bare JSON array —
// same reasoning as danhSachBoPhanRa, and the same absence of `next_cursor` / `has_more`: this
// route returns the WHOLE list or it fails.
type danhSachLoaiDonViDanCuRa struct {
	Items []loaiDonViDanCuRa `json:"items"`
}

func loaiDonViDanCuRaNgoai(l domain.LoaiDonViDanCu) loaiDonViDanCuRa {
	return loaiDonViDanCuRa{
		ID: l.ID, Code: l.Ma, Label: l.Nhan, IsDefault: l.LaMacDinh, Active: l.DangDung,
	}
}

// DanhSachLoaiDonViDanCu serves the commune's residential-unit-type catalogue.
// GET /api/v1/residential-unit-types
//
// NO AUDIT ENTRY, and that is a decision rather than an omission. Rule 6, invariant 7 audits
// reading FULL personal data and reading ACROSS communes; this is neither — a reference list of the
// commune's own classifications, read inside the commune the request arrived in. An entry for every
// dropdown fill would bury the entries that carry legal weight under thousands that carry none.
//
// NO idem.* DECLARATION: a GET changes no state.
func (h *Handler) DanhSachLoaiDonViDanCu(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ds, err := h.d.LoaiDonViDanCu.DanhSach(ctx)
	if err != nil {
		if errors.Is(err, idstore.ErrQuaNhieuLoaiDonViDanCu) {
			// REFUSED, NOT TRUNCATED — the argument is on idstore.ErrQuaNhieuLoaiDonViDanCu. The log
			// line names the commune because that is the only thing an operator can act on.
			h.d.Log.Error("danh mục loại đơn vị dân cư vượt trần — TỪ CHỐI thay vì cắt bớt",
				"xa", string(tenant.MustFrom(ctx)), "tran", idstore.TranDanhMucLoaiDonViDanCu)
			httpx.WriteError(w, http.StatusInternalServerError, "internal",
				"Đã xảy ra lỗi. Vui lòng thử lại.", "")
			return
		}
		// The wrapped error carries the store failure and never reaches the client (rule 3,
		// forbidden #3).
		h.d.Log.Error("danh mục loại đơn vị dân cư: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// make(..., 0, ...) and not a nil slice: `items` must marshal as [] and never as null.
	//
	// AN EMPTY LIST IS THE CORRECT ANSWER HERE, NOT A DEGRADED ONE, and today it is the ONLY
	// answer: migration 0005 creates this table and seeds nothing, deliberately, because a seeded
	// row would belong to one named commune and the migration runner has no commune in it. The step
	// that sows a commune's first rows is onboarding, which does not exist in this repository yet
	// (migration 0005:38). A visibly empty list is the failure people report; a half-seeded one
	// looks complete.
	//
	// THE ORDER IS THE STORE'S and is not touched here: `thu_tu` is the order the commune arranged
	// its own catalogue in, and a handler that re-sorted would silently overrule it.
	ra := danhSachLoaiDonViDanCuRa{Items: make([]loaiDonViDanCuRa, 0, len(ds))}
	for _, mot := range ds {
		ra.Items = append(ra.Items, loaiDonViDanCuRaNgoai(mot))
	}
	vietJSON(w, http.StatusOK, ra)
}
