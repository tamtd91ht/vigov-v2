package http

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-documents/internal/app"
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

// The routes behind the commune's document-type catalogue.
//
//	GET    /api/v1/document-types        AnyAuthenticated — the list every screen fills a box from
//	POST   /api/v1/document-types        admin.lookup     — the commune adds a row of its own
//	PATCH  /api/v1/document-types/{id}   admin.lookup     — relabel, reorder, disable, set default
//	DELETE /api/v1/document-types/{id}   admin.lookup     — SOFT delete, tier 1 only
//
// THE WRITE ROUTES USED TO BE ABSENT, AND THE COMMENT HERE SAID WHY: open question #21, "may a
// commune edit the CODE LIST or only the labels and the order". THAT READING WAS OUT OF DATE and
// is corrected rather than deleted, because the next reader will meet the same doubt.
//
// #21 is about the TASK STATUS catalogue, whose codes a fixed state machine walks — adding one
// makes a status with no way in and no way out, and disabling `hoan-thanh` stops every task in the
// commune from ever finishing. This catalogue has no state machine and its own answer is already in
// the schema, enforced by a TRIGGER rather than by a promise
// (migrations/0003_danh_muc_loai_van_ban.sql:72-77 and :157-185; ADR 0024):
//
//	tier 1  nguon = 'don-vi'                          soft delete YES  disable YES  relabel YES
//	tier 2  nguon = 'he-thong'                        soft delete NO   disable YES  relabel YES
//	tier 3  nguon = 'he-thong' AND ma_nguon_re_nhanh  soft delete NO   disable NO   relabel YES
//
// So a commune BOTH mints its own codes AND relabels and reorders every row. It is not one or the
// other, and the `ma` of a row that exists is still immutable in every tier.
//
// `source` AND `tier` ARE OUTPUT-ONLY, AND A REQUEST NAMING EITHER IS REFUSED WITH 400 rather than
// having the field quietly dropped. Provenance decides the tier, so a client that could set it
// could put its own row in tier 2 and walk around every guard — the migration says exactly that
// where the trigger refuses an edit of the column. The 400 exists because a silently ignored field
// is a client that believes it set something; the store writing the value as a LITERAL is what
// actually makes it impossible.
//
// THE CATALOGUE STILL SHIPS EMPTY, AND AN EMPTY LIST IS STILL THE CORRECT ANSWER. Nothing here sows
// a commune's `he-thong` rows: that is commune onboarding, it does not exist in this repository,
// and it is the customer's decision (migrations/0003_danh_muc_loai_van_ban.sql:47-52). What these
// routes add is the commune's own half of the list.

// loaiVanBanRa is one catalogue row as it leaves the API.
//
// NOTHING HERE IS PERSONAL DATA (rule 3): a document type is how the authority classifies its
// paperwork, not anything about a person. That is what makes this route's AnyAuthenticated
// declaration a question about convenience rather than about privacy — see the reason on the route
// itself.
type loaiVanBanRa struct {
	ID    string `json:"id"`    // ULID — what a document record would reference
	Code  string `json:"code"`  // "cong-van" — the value a document record stores, never renumbered
	Label string `json:"label"` // "Công văn" — the wording the commune may change

	// Active is `dang_dung`. Rows out of use ARE returned, carrying false: the configuration
	// screen lists them with a "Đã tắt" chip, and a document registered under a type since retired
	// still has to render its own label. A picker filling a form offers only the rows with
	// `active: true` — the filtering is the client's, because a second, filtered route would give
	// two answers to "what types does this commune have" and the stale one would reach a screen.
	Active bool `json:"active"`

	// IsDefault is the row a form pre-selects. At most one live row per commune carries it — the
	// schema's generated `moc_mac_dinh` column admits no second — and a commune whose catalogue
	// has not been sown carries none at all, which every caller must handle.
	IsDefault bool `json:"is_default"`

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
	// OUTPUT ONLY. A request carrying it is refused with 400 — see the head of this file.
	Source string `json:"source"`

	// Tier is 1, 2 or 3 and is what the screen needs to decide which buttons to draw: tier 3 has no
	// `Tắt`, tiers 2 and 3 have no `🗑`.
	//
	// DERIVED, NEVER STORED (domain.TangCua). A stored tier would be a second copy of a fact that
	// already lives in two columns, and the copies drift — this system has a rule about that
	// (rule 10, invariant 3, for the same shape of mistake on `is_overdue`).
	//
	// WHY THE TIER AND NOT `can_delete` / `can_disable`: the row is named after what it IS, not
	// after what a screen does with it (ADR 0017). Capability flags would also have to be kept in
	// step with the trigger from a second place.
	Tier int `json:"tier"`
}

// danhSachLoaiVanBanRa wraps the list in an OBJECT rather than returning a bare JSON array.
//
// A bare array cannot grow: the day this needs to say anything about the list itself — that it was
// truncated, when it was last changed — every client has to change shape at once. An object with
// one field costs one line now and nothing later. It is also the shape page.Result already gives
// every other list route, so a client reads `items` on all of them.
//
// THERE IS NO next_cursor AND NO has_more, and their absence is the contract: this route returns
// the WHOLE list or it fails. A `has_more` here would invite exactly the paging behaviour the
// route was designed not to need — see docstore.LoaiVanBanStore.DanhSach.
type danhSachLoaiVanBanRa struct {
	Items []loaiVanBanRa `json:"items"`
}

func loaiVanBanRaNgoai(l domain.LoaiVanBan) loaiVanBanRa {
	return loaiVanBanRa{
		ID:        l.ID,
		Code:      l.Ma,
		Label:     l.Nhan,
		Active:    l.DangDung,
		IsDefault: l.LaMacDinh,
		Order:     l.ThuTu,
		Source:    l.Nguon,
		Tier:      int(l.Tang()),
	}
}

// DanhSachLoaiVanBan serves the commune's document-type catalogue. GET /api/v1/document-types
//
// NO AUDIT ENTRY, and that is a decision rather than an omission. Rule 6, invariant 7 audits
// reading FULL personal data and reading ACROSS communes; this is neither — it is a reference list
// of the authority's own classifications, read inside the commune the request arrived in. An entry
// for every dropdown fill would bury the entries that carry legal weight under thousands that
// carry none.
//
// NO idem.* DECLARATION: a GET changes no state.
func (h *Handler) DanhSachLoaiVanBan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	lvb, err := h.d.LoaiVanBan.DanhSach(ctx)
	if err != nil {
		if errors.Is(err, docstore.ErrQuaNhieuLoaiVanBan) {
			// REFUSED, NOT TRUNCATED. This list is what a document is registered under, and
			// numbering follows the type (ADR 0024), so a list missing a type files the document
			// under the wrong one with nothing on the screen to show it. The log line names the
			// commune because that is the only thing an operator can act on — the ceiling is about
			// thirty times the shipped code list, so reaching it means the data is wrong, not that
			// the commune is large.
			h.d.Log.Error("danh mục loại văn bản vượt trần — TỪ CHỐI thay vì cắt bớt",
				"xa", string(tenant.MustFrom(ctx)), "tran", docstore.TranDanhMucLoaiVanBan)
			httpx.WriteError(w, http.StatusInternalServerError, "internal",
				"Đã xảy ra lỗi. Vui lòng thử lại.", "")
			return
		}
		// The wrapped error carries the store failure and never reaches the client (rule 3,
		// forbidden #3).
		h.d.Log.Error("danh mục loại văn bản: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// make(..., 0, ...) and not a nil slice: `items` must marshal as [] on a commune whose
	// catalogue has not been sown yet, never as null. TODAY THAT IS EVERY COMMUNE — the table ships
	// empty on purpose — so this is the ordinary path, not an edge case, and a client that has to
	// handle both shapes handles one of them wrong.
	//
	// THE ORDER IS THE STORE'S and is not touched here: `thu_tu` is the order the commune arranged
	// its own catalogue in, and a handler that re-sorted would silently overrule it.
	ra := danhSachLoaiVanBanRa{Items: make([]loaiVanBanRa, 0, len(lvb))}
	for _, mot := range lvb {
		ra.Items = append(ra.Items, loaiVanBanRaNgoai(mot))
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
// themLoaiVanBanVao is the body of POST /api/v1/document-types.
//
// `Source` AND `Tier` ARE HERE ONLY SO THEY CAN BE REFUSED. They are not written anywhere and never
// reach the store — the store writes `nguon` and `ma_nguon_re_nhanh` as literals. Declaring them
// and answering 400 is the difference between a client learning that it may not decide provenance
// and a client believing it just did.
//
// Unknown fields are IGNORED rather than refused, which is deliberate: a screen that reads a row
// and posts it back carries `id`, and rejecting that would make the obvious client wrong for no
// benefit. The fields that MUST NOT be silently dropped are the ones named above.
type themLoaiVanBanVao struct {
	Code      string `json:"code"`
	Label     string `json:"label"`
	Order     int    `json:"order,omitempty"`
	IsDefault bool   `json:"is_default,omitempty"`

	Source *string `json:"source,omitempty"`
	Tier   *int    `json:"tier,omitempty"`
}

// suaLoaiVanBanVao is the body of PATCH /api/v1/document-types/{id}.
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
type suaLoaiVanBanVao struct {
	Label     *string `json:"label,omitempty"`
	Order     *int    `json:"order,omitempty"`
	Active    *bool   `json:"active,omitempty"`
	IsDefault *bool   `json:"is_default,omitempty"`

	Code   *string `json:"code,omitempty"`
	Source *string `json:"source,omitempty"`
	Tier   *int    `json:"tier,omitempty"`
}

// xoaLoaiVanBanVao is the body of DELETE /api/v1/document-types/{id}.
//
// A DELETE WITH A BODY, and the alternative was worse. Rule 7, invariant 1 names three columns —
// `deleted_at`, `deleted_by`, `delete_reason` — so the reason is not optional, and the only other
// place to put it is the query string, where it would land in every access log and proxy cache of
// a free-text sentence somebody typed about a government record.
type xoaLoaiVanBanVao struct {
	Reason string `json:"reason"`
}

// ThemLoaiVanBan adds one document type the commune owns. POST /api/v1/document-types
func (h *Handler) ThemLoaiVanBan(w http.ResponseWriter, r *http.Request) {
	var vao themLoaiVanBanVao
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

	moi, err := h.d.GhiLoaiVanBan.Them(r.Context(), app.YeuCauThemLoaiVanBan{
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
	vietJSON(w, http.StatusCreated, loaiVanBanRaNgoai(moi))
}

// SuaLoaiVanBan edits one row. PATCH /api/v1/document-types/{id}
func (h *Handler) SuaLoaiVanBan(w http.ResponseWriter, r *http.Request) {
	var vao suaLoaiVanBanVao
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

	sau, err := h.d.GhiLoaiVanBan.Sua(r.Context(), r.PathValue("id"), app.YeuCauSuaLoaiVanBan{
		Nhan:      vao.Label,
		ThuTu:     vao.Order,
		DangDung:  vao.Active,
		LaMacDinh: vao.IsDefault,
	}, nguoi)
	if err != nil {
		h.traLoiLoiGhi(w, r, "sửa", err)
		return
	}
	vietJSON(w, http.StatusOK, loaiVanBanRaNgoai(sau))
}

// XoaLoaiVanBan soft deletes one row. DELETE /api/v1/document-types/{id}
//
// 204 AND NO BODY. The row is still there — it carries `deleted_at`, `deleted_by` and
// `delete_reason` and its code stays taken forever — but there is nothing the caller can do with it
// and returning it would invite a client to display a row it has just removed from the screen.
func (h *Handler) XoaLoaiVanBan(w http.ResponseWriter, r *http.Request) {
	var vao xoaLoaiVanBanVao
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

	if err := h.d.GhiLoaiVanBan.Xoa(r.Context(), r.PathValue("id"), vao.Reason, nguoi); err != nil {
		h.traLoiLoiGhi(w, r, "xoá", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
