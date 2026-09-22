package http

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
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
	// Order is `thu_tu`, the arrangement the commune chose. It is returned because the
	// configuration screen shows a `Thứ tự` column and offers to change it
	// (docs/ui-ux/14-cau-hinh.md:158) — a screen that cannot read the value cannot edit it.
	//
	// THE LIST IS ALREADY IN THIS ORDER. A client re-sorting on this field is a second answer to
	// the same question, and the two disagree the moment two rows share a rank.
	Order int `json:"order"`

	// Source is `nguon` — `don-vi` or `he-thong`, the `Nguồn` column of the screen. A Vietnamese
	// value without diacritics, which is ADR 0011: only the surrounding contract is English.
	//
	// OUTPUT ONLY. A request carrying it is refused with 400 — provenance decides the tier, and a
	// client that could set it could put its own row in tier 2 and step around every guard.
	Source string `json:"source"`

	// Tier is 1, 2 or 3 and is what the screen needs to decide which buttons to draw: tier 3 has no
	// `Tắt`, tiers 2 and 3 have no `🗑`.
	//
	// DERIVED, NEVER STORED (domain.TangCua). A stored tier would be a second copy of a fact that
	// already lives in two columns, and the copies drift — this system has a rule about exactly
	// that shape of mistake (rule 10, invariant 3, on `is_overdue`).
	//
	// WHY THE TIER AND NOT `can_delete` / `can_disable`: the row is named after what it IS, not
	// after what a screen does with it (ADR 0017). Capability flags would also have to be kept in
	// step with the trigger from a second place.
	Tier int `json:"tier"`
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
	return mucUuTienRa{
		ID: m.ID, Code: m.Ma, Label: m.Nhan, IsDefault: m.LaMacDinh, Active: m.DangDung,
		Order: m.ThuTu, Source: m.Nguon, Tier: int(m.Tang()),
	}
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

// --- the write routes ------------------------------------------------------------------------------

// `omitempty` ON EVERY OPTIONAL FIELD, AND IT IS NOT COSMETIC. tools/apidoc marks a field REQUIRED
// in kb/20-contracts/openapi.json unless it carries `omitempty` (schema.go:658-673), so without it
// this body would tell every generated client that `source` and `tier` MUST be sent — on a route
// that answers 400 to exactly that. A contract that demands the one thing the server refuses is
// worse than no contract: the client is wrong before it makes a single request.
//
// These structs are only ever DECODED, never marshalled, so `omitempty` costs nothing at runtime.
// themMucUuTienVao is the body of POST /api/v1/task-priorities.
//
// `Source` AND `Tier` ARE HERE ONLY SO THEY CAN BE REFUSED. They are not written anywhere and never
// reach the store — the store writes `nguon` and `ma_nguon_re_nhanh` as literals. Declaring them
// and answering 400 is the difference between a client learning that it may not decide provenance
// and a client believing it just did.
//
// Unknown fields are IGNORED rather than refused, which is deliberate: a screen that reads a row
// and posts it back carries `id`, and rejecting that would make the obvious client wrong for no
// benefit. The fields that MUST NOT be silently dropped are the ones named above.
type themMucUuTienVao struct {
	Code      string `json:"code"`
	Label     string `json:"label"`
	Order     int    `json:"order,omitempty"`
	IsDefault bool   `json:"is_default,omitempty"`

	Source *string `json:"source,omitempty"`
	Tier   *int    `json:"tier,omitempty"`
}

// suaMucUuTienVao is the body of PATCH /api/v1/task-priorities/{id}.
//
// EVERY EDITABLE FIELD IS A POINTER, and that is the whole reason this is a PATCH and not a PUT:
// three of the four have a meaningful zero — order 0 is the first position, `active` false is "out
// of use", `is_default` false is "no longer the default". A body of plain values cannot tell "not
// mentioned" from "set to zero", so a dialog that edits only the label would move the row to the
// top of the list and clear the commune's default on the way.
//
// `Code` IS REFUSED, NOT IGNORED. An issued code is never renumbered (rule 7, invariant 3) and
// document records hold it as a value; a client that sends it back unchanged with the rest of the
// row is told so rather than left to assume it could have changed it.
type suaMucUuTienVao struct {
	Label     *string `json:"label,omitempty"`
	Order     *int    `json:"order,omitempty"`
	Active    *bool   `json:"active,omitempty"`
	IsDefault *bool   `json:"is_default,omitempty"`

	Code   *string `json:"code,omitempty"`
	Source *string `json:"source,omitempty"`
	Tier   *int    `json:"tier,omitempty"`
}

// xoaMucUuTienVao is the body of DELETE /api/v1/task-priorities/{id}.
//
// A DELETE WITH A BODY, and the alternative was worse. Rule 7, invariant 1 names three columns —
// `deleted_at`, `deleted_by`, `delete_reason` — so the reason is not optional, and the only other
// place to put it is the query string, where it would land in every access log and proxy cache of
// a free-text sentence somebody typed about a government record.
type xoaMucUuTienVao struct {
	Reason string `json:"reason"`
}

// ThemMucUuTien adds one document type the commune owns. POST /api/v1/task-priorities
func (h *Handler) ThemMucUuTien(w http.ResponseWriter, r *http.Request) {
	var vao themMucUuTienVao
	if !docThan(w, r, &vao) {
		return
	}
	// BEFORE ANYTHING ELSE. A body naming `source` or `tier` is refused outright, so the caller
	// learns that provenance is not theirs to set rather than watching the field disappear.
	if vao.Source != nil || vao.Tier != nil {
		h.traLoiLoiGhi(w, r, "thêm", domain.ErrNguonDoTuClient)
		return
	}

	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.d.Log.Error("tuyến ghi danh mục chạy mà không có chủ thể — SAI CẤU HÌNH ROUTE",
			"xa", string(tenant.MustFrom(r.Context())), "duong", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	moi, err := h.d.GhiMucUuTien.Them(r.Context(), app.YeuCauThemMucUuTien{
		Ma:        vao.Code,
		Nhan:      vao.Label,
		ThuTu:     vao.Order,
		LaMacDinh: vao.IsDefault,
	}, nguoi)
	if err != nil {
		h.traLoiLoiGhi(w, r, "thêm", err)
		return
	}

	// What a retry carrying the same Idempotency-Key is told about. THE CODE AND NOT THE BODY: the
	// body would go into Redis, which is a cache and not a record store (skills/rest-api-design §4).
	idem.RecordCode(r.Context(), moi.Ma)
	vietJSON(w, http.StatusCreated, mucUuTienRaNgoai(moi))
}

// SuaMucUuTien edits one row. PATCH /api/v1/task-priorities/{id}
func (h *Handler) SuaMucUuTien(w http.ResponseWriter, r *http.Request) {
	var vao suaMucUuTienVao
	if !docThan(w, r, &vao) {
		return
	}
	if vao.Source != nil || vao.Tier != nil {
		h.traLoiLoiGhi(w, r, "sửa", domain.ErrNguonDoTuClient)
		return
	}
	if vao.Code != nil {
		h.traLoiLoiGhi(w, r, "sửa", domain.ErrMaBatBien)
		return
	}

	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.d.Log.Error("tuyến ghi danh mục chạy mà không có chủ thể — SAI CẤU HÌNH ROUTE",
			"xa", string(tenant.MustFrom(r.Context())), "duong", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	sau, err := h.d.GhiMucUuTien.Sua(r.Context(), r.PathValue("id"), app.YeuCauSuaMucUuTien{
		Nhan:      vao.Label,
		ThuTu:     vao.Order,
		DangDung:  vao.Active,
		LaMacDinh: vao.IsDefault,
	}, nguoi)
	if err != nil {
		h.traLoiLoiGhi(w, r, "sửa", err)
		return
	}
	vietJSON(w, http.StatusOK, mucUuTienRaNgoai(sau))
}

// XoaMucUuTien soft deletes one row. DELETE /api/v1/task-priorities/{id}
//
// 204 AND NO BODY. The row is still there — it carries `deleted_at`, `deleted_by` and
// `delete_reason` and its code stays taken forever — but there is nothing the caller can do with it
// and returning it would invite a client to display a row it has just removed from the screen.
func (h *Handler) XoaMucUuTien(w http.ResponseWriter, r *http.Request) {
	var vao xoaMucUuTienVao
	if !docThan(w, r, &vao) {
		return
	}

	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.d.Log.Error("tuyến ghi danh mục chạy mà không có chủ thể — SAI CẤU HÌNH ROUTE",
			"xa", string(tenant.MustFrom(r.Context())), "duong", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	if err := h.d.GhiMucUuTien.Xoa(r.Context(), r.PathValue("id"), vao.Reason, nguoi); err != nil {
		h.traLoiLoiGhi(w, r, "xoá", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
