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
	return loaiNhiemVuRa{
		ID: l.ID, Code: l.Ma, Label: l.Nhan, IsDefault: l.LaMacDinh, Active: l.DangDung,
		Order: l.ThuTu, Source: l.Nguon, Tier: int(l.Tang()),
	}
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

// --- the write routes ------------------------------------------------------------------------------

// `omitempty` ON EVERY OPTIONAL FIELD, AND IT IS NOT COSMETIC. tools/apidoc marks a field REQUIRED
// in kb/20-contracts/openapi.json unless it carries `omitempty` (schema.go:658-673), so without it
// this body would tell every generated client that `source` and `tier` MUST be sent — on a route
// that answers 400 to exactly that. A contract that demands the one thing the server refuses is
// worse than no contract: the client is wrong before it makes a single request.
//
// These structs are only ever DECODED, never marshalled, so `omitempty` costs nothing at runtime.
// themLoaiNhiemVuVao is the body of POST /api/v1/task-types.
//
// `Source` AND `Tier` ARE HERE ONLY SO THEY CAN BE REFUSED. They are not written anywhere and never
// reach the store — the store writes `nguon` and `ma_nguon_re_nhanh` as literals. Declaring them
// and answering 400 is the difference between a client learning that it may not decide provenance
// and a client believing it just did.
//
// Unknown fields are IGNORED rather than refused, which is deliberate: a screen that reads a row
// and posts it back carries `id`, and rejecting that would make the obvious client wrong for no
// benefit. The fields that MUST NOT be silently dropped are the ones named above.
type themLoaiNhiemVuVao struct {
	Code      string `json:"code"`
	Label     string `json:"label"`
	Order     int    `json:"order,omitempty"`
	IsDefault bool   `json:"is_default,omitempty"`

	Source *string `json:"source,omitempty"`
	Tier   *int    `json:"tier,omitempty"`
}

// suaLoaiNhiemVuVao is the body of PATCH /api/v1/task-types/{id}.
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
type suaLoaiNhiemVuVao struct {
	Label     *string `json:"label,omitempty"`
	Order     *int    `json:"order,omitempty"`
	Active    *bool   `json:"active,omitempty"`
	IsDefault *bool   `json:"is_default,omitempty"`

	Code   *string `json:"code,omitempty"`
	Source *string `json:"source,omitempty"`
	Tier   *int    `json:"tier,omitempty"`
}

// xoaLoaiNhiemVuVao is the body of DELETE /api/v1/task-types/{id}.
//
// A DELETE WITH A BODY, and the alternative was worse. Rule 7, invariant 1 names three columns —
// `deleted_at`, `deleted_by`, `delete_reason` — so the reason is not optional, and the only other
// place to put it is the query string, where it would land in every access log and proxy cache of
// a free-text sentence somebody typed about a government record.
type xoaLoaiNhiemVuVao struct {
	Reason string `json:"reason"`
}

// ThemLoaiNhiemVu adds one document type the commune owns. POST /api/v1/task-types
func (h *Handler) ThemLoaiNhiemVu(w http.ResponseWriter, r *http.Request) {
	var vao themLoaiNhiemVuVao
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

	moi, err := h.d.GhiLoaiNhiemVu.Them(r.Context(), app.YeuCauThemLoaiNhiemVu{
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
	vietJSON(w, http.StatusCreated, loaiNhiemVuRaNgoai(moi))
}

// SuaLoaiNhiemVu edits one row. PATCH /api/v1/task-types/{id}
func (h *Handler) SuaLoaiNhiemVu(w http.ResponseWriter, r *http.Request) {
	var vao suaLoaiNhiemVuVao
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

	sau, err := h.d.GhiLoaiNhiemVu.Sua(r.Context(), r.PathValue("id"), app.YeuCauSuaLoaiNhiemVu{
		Nhan:      vao.Label,
		ThuTu:     vao.Order,
		DangDung:  vao.Active,
		LaMacDinh: vao.IsDefault,
	}, nguoi)
	if err != nil {
		h.traLoiLoiGhi(w, r, "sửa", err)
		return
	}
	vietJSON(w, http.StatusOK, loaiNhiemVuRaNgoai(sau))
}

// XoaLoaiNhiemVu soft deletes one row. DELETE /api/v1/task-types/{id}
//
// 204 AND NO BODY. The row is still there — it carries `deleted_at`, `deleted_by` and
// `delete_reason` and its code stays taken forever — but there is nothing the caller can do with it
// and returning it would invite a client to display a row it has just removed from the screen.
func (h *Handler) XoaLoaiNhiemVu(w http.ResponseWriter, r *http.Request) {
	var vao xoaLoaiNhiemVuVao
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

	if err := h.d.GhiLoaiNhiemVu.Xoa(r.Context(), r.PathValue("id"), vao.Reason, nguoi); err != nil {
		h.traLoiLoiGhi(w, r, "xoá", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
