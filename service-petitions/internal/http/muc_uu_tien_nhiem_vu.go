package http

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The read route behind the commune's task-priority scale. GET /api/v1/task-priorities
//
// THERE IS NO WRITE ROUTE, for the reason written on the task-type route: open question #21 is
// still OPEN.

// mucUuTienRa is one level of the scale as it leaves the API.
//
// NOTHING HERE IS PERSONAL DATA (rule 3). The absent fields are the same as on loaiNhiemVuRa and
// absent for the same reasons — with one that matters more here: there is NO `order` / `rank`
// number. The rank is the position of the item in `items`, and one fact gets one representation
// (rule 9). A number beside the array is a second copy, and the copy a client keeps after
// re-sorting the array is the one that lies about which level outranks which.
type mucUuTienRa struct {
	ID   string `json:"id"`   // ULID — what a task record references
	Code string `json:"code"` // slug: "khan"

	// Label is `nhan` — "Khẩn". It was `name` first; the whole argument, including the reading that
	// was rejected, is on loaiNhiemVuRa.Label. Short version: `nhan` is a LABEL, and re-wording it is
	// the one change the tier trigger permits a commune to make while `ma` is immutable, so `label`
	// is the weaker and true claim (ADR 0017).
	Label string `json:"label"`

	// IsDefault marks the level a new task starts at. At most one row in the list carries it, held
	// by the database rather than by this handler (migration 0003, UNIQUE (tenant_id,
	// moc_mac_dinh)).
	IsDefault bool `json:"is_default"`

	// Active is false for a level taken out of use. Such levels are still in the list, so an older
	// task holding the code still has a label; a picker offering NEW choices filters on this.
	//
	// `active` AND NOT `is_active` — same reason as on loaiNhiemVuRa.Active: four other ADR 0024
	// catalogues already answer `active`.
	Active bool `json:"active"`
}

// danhSachMucUuTienRa wraps the list in an object — same reasoning as danhSachLoaiNhiemVuRa, and
// the same absence of `next_cursor` / `has_more`: the whole list or a failure.
//
// THE ORDER OF `items` IS THE SCALE. For the type catalogue the order is display preference; here
// it is the meaning of the data — "khẩn" is urgent only relative to what sits around it. A client
// that re-sorts this array alphabetically has not restyled a list, it has changed which task the
// commune treats as most urgent, and every screen still looks entirely normal.
type danhSachMucUuTienRa struct {
	Items []mucUuTienRa `json:"items"`
}

func mucUuTienRaNgoai(m domain.MucUuTienNhiemVu) mucUuTienRa {
	return mucUuTienRa{ID: m.ID, Code: m.Ma, Label: m.Nhan, IsDefault: m.LaMacDinh, Active: m.DangDung}
}

// DanhSachMucUuTien serves the commune's priority scale. GET /api/v1/task-priorities
//
// NO AUDIT ENTRY and NO idem.* DECLARATION — same reading as DanhSachLoaiNhiemVu, where both are
// argued.
func (h *Handler) DanhSachMucUuTien(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ds, err := h.d.MucUuTien.DanhSach(ctx)
	if err != nil {
		if errors.Is(err, petstore.ErrQuaNhieuMucUuTien) {
			// REFUSED, NOT TRUNCATED. The levels arrive in rank order, so a truncated scale loses
			// the levels at one END of it — the picker then offers a scale that stops short, and
			// every task filed from it is ranked wrong with nothing on the screen to show it.
			h.d.Log.Error("danh mục mức ưu tiên vượt trần — TỪ CHỐI thay vì cắt bớt",
				"xa", string(tenant.MustFrom(ctx)), "tran", petstore.TranDanhMucMucUuTien)
			httpx.WriteError(w, http.StatusInternalServerError, "internal",
				"Đã xảy ra lỗi. Vui lòng thử lại.", "")
			return
		}
		// The wrapped error carries the store failure and never reaches the client (rule 3,
		// forbidden #3).
		h.d.Log.Error("danh mục mức ưu tiên: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// [] and never null; an empty scale is today's correct answer for every commune, because
	// migration 0003 ships both catalogues empty and onboarding does not exist yet — the argument
	// is written out on DanhSachLoaiNhiemVu.
	//
	// THE ORDER IS THE STORE'S AND IS NOT TOUCHED HERE. This loop is append-only over the store's
	// slice on purpose: any sort in this handler would overrule the commune's own ranking.
	ra := danhSachMucUuTienRa{Items: make([]mucUuTienRa, 0, len(ds))}
	for _, mot := range ds {
		ra.Items = append(ra.Items, mucUuTienRaNgoai(mot))
	}
	vietJSON(w, http.StatusOK, ra)
}
