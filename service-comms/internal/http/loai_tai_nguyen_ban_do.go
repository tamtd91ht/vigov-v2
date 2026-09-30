package http

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
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
		Order:     lt.ThuTu,
		Source:    lt.Nguon,
		Tier:      int(lt.Tang()),
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

// --- the write routes ------------------------------------------------------------------------------

// `omitempty` ON EVERY OPTIONAL FIELD, AND IT IS NOT COSMETIC. tools/apidoc marks a field REQUIRED
// in kb/20-contracts/openapi.json unless it carries `omitempty` (schema.go:658-673), so without it
// this body would tell every generated client that `source` and `tier` MUST be sent — on a route
// that answers 400 to exactly that. A contract that demands the one thing the server refuses is
// worse than no contract: the client is wrong before it makes a single request.
//
// These structs are only ever DECODED, never marshalled, so `omitempty` costs nothing at runtime.
// themLoaiTaiNguyenVao is the body of POST /api/v1/map-asset-types.
//
// `Source` AND `Tier` ARE HERE ONLY SO THEY CAN BE REFUSED. They are not written anywhere and never
// reach the store — the store writes `nguon` and `ma_nguon_re_nhanh` as literals. Declaring them
// and answering 400 is the difference between a client learning that it may not decide provenance
// and a client believing it just did.
//
// Unknown fields are IGNORED rather than refused, which is deliberate: a screen that reads a row
// and posts it back carries `id`, and rejecting that would make the obvious client wrong for no
// benefit. The fields that MUST NOT be silently dropped are the ones named above.
type themLoaiTaiNguyenVao struct {
	Code      string `json:"code"`
	Label     string `json:"label"`
	Order     int    `json:"order,omitempty"`
	IsDefault bool   `json:"is_default,omitempty"`

	Source *string `json:"source,omitempty"`
	Tier   *int    `json:"tier,omitempty"`
}

// suaLoaiTaiNguyenVao is the body of PATCH /api/v1/map-asset-types/{id}.
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
type suaLoaiTaiNguyenVao struct {
	Label     *string `json:"label,omitempty"`
	Order     *int    `json:"order,omitempty"`
	Active    *bool   `json:"active,omitempty"`
	IsDefault *bool   `json:"is_default,omitempty"`

	Code   *string `json:"code,omitempty"`
	Source *string `json:"source,omitempty"`
	Tier   *int    `json:"tier,omitempty"`
}

// xoaLoaiTaiNguyenVao is the body of DELETE /api/v1/map-asset-types/{id}.
//
// A DELETE WITH A BODY, and the alternative was worse. Rule 7, invariant 1 names three columns —
// `deleted_at`, `deleted_by`, `delete_reason` — so the reason is not optional, and the only other
// place to put it is the query string, where it would land in every access log and proxy cache of
// a free-text sentence somebody typed about a government record.
type xoaLoaiTaiNguyenVao struct {
	Reason string `json:"reason"`
}

// ThemLoaiTaiNguyen adds one document type the commune owns. POST /api/v1/map-asset-types
func (h *Handler) ThemLoaiTaiNguyen(w http.ResponseWriter, r *http.Request) {
	var vao themLoaiTaiNguyenVao
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

	moi, err := h.d.GhiLoaiTaiNguyen.Them(r.Context(), app.YeuCauThemLoaiTaiNguyen{
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
	vietJSON(w, http.StatusCreated, loaiTaiNguyenRaNgoai(moi))
}

// SuaLoaiTaiNguyen edits one row. PATCH /api/v1/map-asset-types/{id}
func (h *Handler) SuaLoaiTaiNguyen(w http.ResponseWriter, r *http.Request) {
	var vao suaLoaiTaiNguyenVao
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

	sau, err := h.d.GhiLoaiTaiNguyen.Sua(r.Context(), r.PathValue("id"), app.YeuCauSuaLoaiTaiNguyen{
		Nhan:      vao.Label,
		ThuTu:     vao.Order,
		DangDung:  vao.Active,
		LaMacDinh: vao.IsDefault,
	}, nguoi)
	if err != nil {
		h.traLoiLoiGhi(w, r, "sửa", err)
		return
	}
	vietJSON(w, http.StatusOK, loaiTaiNguyenRaNgoai(sau))
}

// XoaLoaiTaiNguyen soft deletes one row. DELETE /api/v1/map-asset-types/{id}
//
// 204 AND NO BODY. The row is still there — it carries `deleted_at`, `deleted_by` and
// `delete_reason` and its code stays taken forever — but there is nothing the caller can do with it
// and returning it would invite a client to display a row it has just removed from the screen.
func (h *Handler) XoaLoaiTaiNguyen(w http.ResponseWriter, r *http.Request) {
	var vao xoaLoaiTaiNguyenVao
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

	if err := h.d.GhiLoaiTaiNguyen.Xoa(r.Context(), r.PathValue("id"), vao.Reason, nguoi); err != nil {
		h.traLoiLoiGhi(w, r, "xoá", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
