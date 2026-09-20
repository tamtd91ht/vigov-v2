package http

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// The read route behind the commune's capital plan category catalogue.
// GET /api/v1/capital-plan-categories
//
// THERE IS NO WRITE ROUTE, and no scaffolding for one is left here. Open question #21 — whether a
// commune may edit the CODE LIST of a reference catalogue or only its labels and order — is
// unanswered, and a write path would settle it silently in whichever direction the first person to
// write it happened to assume. The three tiers the schema enforces (ADR 0024 §6, the
// `danh_muc_ba_tang` trigger) describe what the database will refuse; they do not say who is
// allowed to try.
//
// THE CATALOGUE SHIPS EMPTY, ON PURPOSE. The migration sows no row, because a row here carries a
// tenant_id and the migration runner has no commune in it (0003_danh_muc_hang_muc_ke_hoach_von.sql,
// §WHERE THE `nguon = 'he-thong'` ROWS COME FROM). Until commune onboarding exists, the correct
// answer from this route is an empty list — visibly empty, which is the failure people report,
// unlike a half-seeded catalogue that looks complete. Nothing in this file compensates for that.

// hangMucRa is one category as it leaves the API.
//
// NOTHING HERE IS PERSONAL DATA (rule 3): a category code and its label describe budget
// classification, not a person. That is what makes this route's AnyAuthenticated declaration a
// question about convenience rather than about privacy — see the reason on the route itself.
type hangMucRa struct {
	ID   string `json:"id"`   // ULID — what a capital plan line will reference
	Code string `json:"code"` // `ma`: Vietnamese without diacritics, the value a plan line stores

	// `label` AND NOT `name`, AND THE QUESTION WAS ASKED TWICE — the first answer is kept here
	// because the second only makes sense against it.
	//
	//	first answer, `name`   the field fills the visible slot of a picker, and `org-units` and
	//	                       `roles` in the identity service already answer `name` there, so one
	//	                       word across every list looked like the consistent choice.
	//	what it missed         those two columns are `ten`. This one is `nhan`, and ADR 0017 names
	//	                       a contract field after what the data IS, not after the screen slot it
	//	                       fills. A `nhan` is a LABEL — re-wording it is precisely the one
	//	                       change the three-tier trigger permits a commune to make on a row
	//	                       whose `ma` is immutable (ADR 0024 §6). `name` asserts the row IS what
	//	                       the string says; `label` says the string is what the row is CALLED,
	//	                       which is the weaker and true claim.
	//
	// So the line now falls at the SCHEMA, not at the screen: every ADR 0024 catalogue answers
	// `label` (identity's `/residential-unit-types` and `/task-blocs`, documents' `/document-types`,
	// comms' map asset types), and named entities keep `name` (`org-units`, `roles`,
	// `/residential-units` — all `ten`). Two shapes for two kinds of thing, so the web carries one
	// branch, not one per catalogue.
	//
	// CHANGED WHILE IT WAS STILL FREE: `make kb` has not regenerated kb/20-contracts/openapi.json,
	// so no client has ever seen `name` on this route. A field name is a contract, and the cost of
	// moving one only ever goes up.
	Label string `json:"label"` // `nhan`: what the category is called, and what a commune may re-word

	// IsDefault marks the row a form pre-selects. At most one live row per commune carries it, and
	// a commune that has set none is an ordinary state, not an error: the client selects nothing.
	IsDefault bool `json:"is_default"`

	// Active is false for a row the commune has taken out of use. It is RETURNED rather than
	// filtered away — the catalogue screen lists it with a "Đã tắt" chip, and a plan line from an
	// earlier budget year still holds its code, so a reader that never saw the row would render
	// an existing figure with no category name. A picker for NEW work must not offer it.
	Active bool `json:"active"`
}

// danhSachHangMucRa wraps the list in an OBJECT rather than returning a bare JSON array.
//
// A bare array cannot grow: the day this needs to say anything about the list itself — that it was
// truncated, when it last changed — every client has to change shape at once. An object with one
// field costs one line now and nothing later. It is also the shape every other list route in this
// system gives, so a client reads `items` on all of them.
//
// THERE IS NO next_cursor AND NO has_more, and their absence is the contract: this route returns
// the WHOLE list or it fails. A `has_more` here would invite exactly the paging behaviour the route
// was designed not to need — see fistore.HangMucKeHoachVonStore.DanhSach.
type danhSachHangMucRa struct {
	Items []hangMucRa `json:"items"`
}

func hangMucRaNgoai(hm domain.HangMucKeHoachVon) hangMucRa {
	return hangMucRa{
		ID:        hm.ID,
		Code:      hm.Ma,
		Label:     hm.Nhan,
		IsDefault: hm.LaMacDinh,
		Active:    hm.DangDung,
	}
}

// DanhSachHangMucKeHoachVon serves the commune's capital plan category catalogue.
// GET /api/v1/capital-plan-categories
//
// NO AUDIT ENTRY, and that is a decision rather than an omission. Rule 6, invariant 7 audits
// reading FULL personal data and reading ACROSS communes; this is neither — it is a reference list
// of budget classifications, read inside the commune the request arrived in. An entry for every
// dropdown fill would bury the entries that carry legal weight under thousands that carry none.
//
// NO idem.* DECLARATION: a GET changes no state.
func (h *Handler) DanhSachHangMucKeHoachVon(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	hm, err := h.d.HangMuc.DanhSach(ctx)
	if err != nil {
		if errors.Is(err, fistore.ErrQuaNhieuHangMuc) {
			// REFUSED, NOT TRUNCATED. This list fills the classifier on a capital plan line, and
			// those figures are totalled and reported upward, so a missing category is a line filed
			// under the wrong heading with nothing on the screen to show it. The log names the
			// commune because that is the only thing an operator can act on — the ceiling is about
			// twenty-five times a real catalogue, so reaching it means the data is wrong, not that
			// the commune is large.
			h.d.Log.Error("danh mục hạng mục kế hoạch vốn vượt trần — TỪ CHỐI thay vì cắt bớt",
				"xa", string(tenant.MustFrom(ctx)), "tran", fistore.TranDanhMucHangMuc)
			httpx.WriteError(w, http.StatusInternalServerError, "internal",
				"Đã xảy ra lỗi. Vui lòng thử lại.", "")
			return
		}
		// The wrapped error carries the store failure and never reaches the client (rule 3,
		// forbidden #3).
		h.d.Log.Error("danh mục hạng mục kế hoạch vốn: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// make(..., 0, ...) and not a nil slice: `items` must marshal as [] and never as null. THIS IS
	// THE ORDINARY CASE HERE, not an edge one — the table ships empty for every commune until
	// onboarding sows its rows, so the empty list is what this route returns today for everybody.
	// A client that has to handle both [] and null handles one of them wrong.
	//
	// THE ORDER IS THE STORE'S and is not touched here: `thu_tu` is the order the commune arranged
	// its own catalogue in, and a handler that re-sorted would silently overrule it.
	ra := danhSachHangMucRa{Items: make([]hangMucRa, 0, len(hm))}
	for _, mot := range hm {
		ra.Items = append(ra.Items, hangMucRaNgoai(mot))
	}
	vietJSON(w, http.StatusOK, ra)
}
