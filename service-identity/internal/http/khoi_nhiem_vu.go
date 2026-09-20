package http

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The read route behind the commune's task-bloc catalogue. GET /api/v1/task-blocs
//
// THE ROUTE IS SERVED BY identity ALTHOUGH THE ENTITY NAME CARRIES "Task" — the name follows the
// concept, ownership follows the rate of change, and the two are allowed to differ. The argument is
// on domain.KhoiNhiemVu; moving the table to `petitions` is forbidden in advance by ADR 0024:130.
// The consumer is the task form in `petitions`, which reads this over the service contract and
// stores the CODE as a value (rule 2, invariant 3).
//
// THERE IS NO WRITE ROUTE — open question #21, same as the other catalogue.

// khoiNhiemVuRa is one catalogue entry as it leaves the API.
//
// NOTHING HERE IS PERSONAL DATA (rule 3): a bloc names an arm of the apparatus, not a person.
//
// `nguon` AND `ma_nguon_re_nhanh` ARE DELIBERATELY ABSENT — they answer "what may be done to this
// row", and nothing may be done to it through this API. Same reading as loaiDonViDanCuRa.
type khoiNhiemVuRa struct {
	ID   string `json:"id"`   // ULID
	Code string `json:"code"` // slug: "khoi-uy-ban" — the value a task record in `petitions` holds

	// Label and not `name` — same reading as loaiDonViDanCuRa.Label: the code is the thing, the
	// label is what is written on it.
	Label string `json:"label"` // "Khối Uỷ ban"

	IsDefault bool `json:"is_default"` // the row a form pre-selects; at most one per commune

	// Active is false = taken out of use, returned so a picker can filter. `active` beside an
	// `is_default` is the pair five sibling catalogue routes already answer, and the pair this
	// service already ships on can_bo.go:51 — the asymmetry is shared on purpose, so do not tidy
	// one half of it. Two spellings of `dang_dung` across one contract make a client writing a
	// single catalogue reader branch on which service answered.
	Active bool `json:"active"`
}

// danhSachKhoiNhiemVuRa wraps the list in an OBJECT rather than a bare JSON array — same reasoning
// as danhSachBoPhanRa, and the same absence of `next_cursor` / `has_more`.
type danhSachKhoiNhiemVuRa struct {
	Items []khoiNhiemVuRa `json:"items"`
}

func khoiNhiemVuRaNgoai(k domain.KhoiNhiemVu) khoiNhiemVuRa {
	return khoiNhiemVuRa{
		ID: k.ID, Code: k.Ma, Label: k.Nhan, IsDefault: k.LaMacDinh, Active: k.DangDung,
	}
}

// DanhSachKhoiNhiemVu serves the commune's task-bloc catalogue. GET /api/v1/task-blocs
//
// NO AUDIT ENTRY: rule 6, invariant 7 audits reading FULL personal data and reading ACROSS
// communes. This is neither.
//
// NO idem.* DECLARATION: a GET changes no state.
func (h *Handler) DanhSachKhoiNhiemVu(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ds, err := h.d.KhoiNhiemVu.DanhSach(ctx)
	if err != nil {
		if errors.Is(err, idstore.ErrQuaNhieuKhoiNhiemVu) {
			// REFUSED, NOT TRUNCATED — and the cost of the alternative is paid in another service:
			// a bloc missing from the task form files the task under the wrong arm of the
			// apparatus, and the count reported upward is then simply false.
			h.d.Log.Error("danh mục khối nhiệm vụ vượt trần — TỪ CHỐI thay vì cắt bớt",
				"xa", string(tenant.MustFrom(ctx)), "tran", idstore.TranDanhMucKhoiNhiemVu)
			httpx.WriteError(w, http.StatusInternalServerError, "internal",
				"Đã xảy ra lỗi. Vui lòng thử lại.", "")
			return
		}
		// The wrapped error carries the store failure and never reaches the client (rule 3,
		// forbidden #3).
		h.d.Log.Error("danh mục khối nhiệm vụ: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// make(..., 0, ...) and not a nil slice: `items` must marshal as [] and never as null. An empty
	// list is the correct answer today for every commune — migration 0005 seeds nothing, on purpose
	// (0005:38), and the onboarding step that would sow the first rows does not exist yet.
	//
	// THE ORDER IS THE STORE'S and is not touched here.
	ra := danhSachKhoiNhiemVuRa{Items: make([]khoiNhiemVuRa, 0, len(ds))}
	for _, mot := range ds {
		ra.Items = append(ra.Items, khoiNhiemVuRaNgoai(mot))
	}
	vietJSON(w, http.StatusOK, ra)
}
