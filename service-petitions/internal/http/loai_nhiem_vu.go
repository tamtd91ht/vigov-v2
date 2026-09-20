package http

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The read route behind the commune's task-type catalogue. GET /api/v1/task-types
//
// THERE IS NO WRITE ROUTE, and no scaffolding for one is left here. Open question #21 — whether a
// commune may edit the CODE LIST of a task catalogue or only the labels and the order — is still
// OPEN, and it is not a stylistic question: a code the source branches on, taken out of use from a
// screen that offers the button, leaves a flow with nowhere to go and nothing turns red. A
// half-written write path looks like a decision somebody made.

// loaiNhiemVuRa is one catalogue row as it leaves the API.
//
// NOTHING HERE IS PERSONAL DATA (rule 3): a type's code and label describe how the authority
// classifies its own work, not a person. That is what makes this route's AnyAuthenticated
// declaration a question about convenience rather than about privacy — see the reason on the route.
//
// THE FIELDS THAT ARE ABSENT ARE THE DESIGN. No `nguon`, no `ma_nguon_re_nhanh`, no `thu_tu`:
// the first two are write-side facts with no write route (domain.LoaiNhiemVu says why), and the
// third is the ORDER, which this response carries as the order of `items` and must not also carry
// as a number — see danhSachLoaiNhiemVuRa.
type loaiNhiemVuRa struct {
	ID   string `json:"id"`   // ULID — what a task record references
	Code string `json:"code"` // slug: "theo-van-ban"

	// Label is `nhan` — "Theo văn bản".
	//
	// IT WAS `name` FIRST, AND THE QUESTION IS RECORDED RATHER THAN THE ANSWER ALONE, because it is
	// a question somebody will ask again. The first reading was that `name` is the ordinary English
	// word for the string a person reads, and service-identity's `org-units` and `roles` both answer
	// `name`.
	//
	// WHAT MOVED IT: the line is drawn at the SCHEMA, not at the screen slot. `bo_phan` and `vai_tro`
	// carry `ten` — a name something HAS — so `name` is true of them. Every ADR 0024 catalogue carries
	// `nhan`, a LABEL, and re-wording it is precisely the one change the three-tier trigger PERMITS a
	// commune to make while `ma` stays immutable (migration 0003, `catalogue %: `ma` is immutable`).
	// So `name` would assert the row IS what the string says; `label` says the string is what the row
	// is CALLED — the weaker and true claim. ADR 0017 decides it: a contract field is named after what
	// the data IS, not after the screen slot it fills.
	Label string `json:"label"`

	// IsDefault marks the row a form pre-selects. AT MOST ONE ROW IN THE LIST CARRIES IT — the
	// database holds that, not this handler (migration 0003, UNIQUE (tenant_id, moc_mac_dinh)) — so
	// a client may take the first it finds without wondering whether there is a second.
	IsDefault bool `json:"is_default"`

	// Active is false for a row the commune has taken out of use. SUCH ROWS ARE STILL IN THE LIST:
	// an older task may hold the code, and dropping the row would leave that task showing a raw code
	// with no label. A picker offering NEW choices filters on this field; a screen LABELLING an
	// existing record must not.
	//
	// `active` AND NOT `is_active`, which is what this shipped as first: the four other ADR 0024
	// catalogues in this system (documents, finance, comms, identity) all answer `active`, and one
	// route spelling a shared shape differently is a client writing two readers for one concept. The
	// asymmetry with `is_default` beside it is theirs too, and matching it beats being locally tidy.
	Active bool `json:"active"`
}

// danhSachLoaiNhiemVuRa wraps the list in an OBJECT rather than returning a bare JSON array.
//
// A bare array cannot grow: the day this needs to say anything about the list itself, every client
// changes shape at once. An object with one field costs one line now and nothing later, and it is
// the shape page.Result gives every other list route, so a client reads `items` on all of them.
//
// THERE IS NO next_cursor AND NO has_more, and their absence is the contract: this route returns
// the WHOLE list or it fails — see petstore.LoaiNhiemVuStore.DanhSach.
//
// THE ORDER OF `items` IS PART OF THE CONTRACT: it is the order the commune arranged its own
// catalogue in (`thu_tu`). There is deliberately no `order` field beside it — one fact, one
// representation (rule 9); two copies drift the moment a client re-sorts the array and keeps the
// numbers.
type danhSachLoaiNhiemVuRa struct {
	Items []loaiNhiemVuRa `json:"items"`
}

func loaiNhiemVuRaNgoai(l domain.LoaiNhiemVu) loaiNhiemVuRa {
	return loaiNhiemVuRa{ID: l.ID, Code: l.Ma, Label: l.Nhan, IsDefault: l.LaMacDinh, Active: l.DangDung}
}

// DanhSachLoaiNhiemVu serves the commune's task-type catalogue. GET /api/v1/task-types
//
// NO AUDIT ENTRY, and that is a decision rather than an omission. Rule 6, invariant 7 audits
// reading FULL personal data and reading ACROSS communes; this is neither — a reference list of the
// authority's own classifications, read inside the commune the request arrived in. An entry per
// dropdown fill would bury the entries that carry legal weight under thousands that carry none.
//
// NO idem.* DECLARATION: a GET changes no state.
func (h *Handler) DanhSachLoaiNhiemVu(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ds, err := h.d.LoaiNhiemVu.DanhSach(ctx)
	if err != nil {
		if errors.Is(err, petstore.ErrQuaNhieuLoaiNhiemVu) {
			// REFUSED, NOT TRUNCATED. This list fills the type picker on the task form and the type
			// filter on every task list, so a silently short list files work under the wrong type
			// or hides tasks that exist — with nothing on the screen to show it. The log line names
			// the commune because that is the only thing an operator can act on: the ceiling is
			// about fifty times the real figure, so reaching it means the data is wrong.
			h.d.Log.Error("danh mục loại nhiệm vụ vượt trần — TỪ CHỐI thay vì cắt bớt",
				"xa", string(tenant.MustFrom(ctx)), "tran", petstore.TranDanhMucLoaiNhiemVu)
			httpx.WriteError(w, http.StatusInternalServerError, "internal",
				"Đã xảy ra lỗi. Vui lòng thử lại.", "")
			return
		}
		// The wrapped error carries the store failure and never reaches the client (rule 3,
		// forbidden #3).
		h.d.Log.Error("danh mục loại nhiệm vụ: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// make(..., 0, ...) and not a nil slice: `items` must marshal as [] and never as null.
	//
	// AN EMPTY LIST IS THE CORRECT ANSWER HERE, NOT AN ERROR, AND TODAY IT IS THE ONLY ANSWER.
	// Migration 0003 creates both catalogues EMPTY on purpose: a catalogue row carries tenant_id,
	// so seeding one would mean seeding it for one named commune, and the step that sows a
	// commune's first rows — onboarding — does not exist in this repository yet. A visibly empty
	// list is the failure people report; a half-seeded one looks complete.
	//
	// THE ORDER IS THE STORE'S and is not touched here: `thu_tu` is the order the commune arranged
	// its own catalogue in, and a handler that re-sorted would silently overrule it.
	ra := danhSachLoaiNhiemVuRa{Items: make([]loaiNhiemVuRa, 0, len(ds))}
	for _, mot := range ds {
		ra.Items = append(ra.Items, loaiNhiemVuRaNgoai(mot))
	}
	vietJSON(w, http.StatusOK, ra)
}
