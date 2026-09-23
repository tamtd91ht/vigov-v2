package http

// THE STAFF SURFACE OF MINI APP CONTENT (`docs/ui-ux/11-noi-dung-mini-app.md`).
//
//	GET   /api/v1/content-items        content.read
//	GET   /api/v1/content-items/{id}   content.read
//	POST  /api/v1/content-items        content.update
//	PATCH /api/v1/content-items/{id}   content.update
//	GET   /api/v1/content-categories   content.read
//	POST  /api/v1/content-categories   content.update
//
// # ⚠ THE URL NOUN IS A DECISION THAT NEEDS THE OWNER'S RATIFICATION — READ THIS BEFORE COPYING IT
//
// `kb/00-foundation/ubiquitous-language.md` §Tên tài nguyên trên URL is the ONE place that maps a
// concept to a path segment, and IT HAS NO ROW FOR ANY CONCEPT IN CHAPTER 11. That file's own
// instruction for that case is explicit (:219-221): "Gặp khái niệm chưa có dòng ở đây: dừng lại và
// hỏi, đừng tự dịch rồi viết route", and `kb/INDEX.yaml` `not_here` repeats it.
//
// SO THE NOUN WAS NOT TRANSLATED. It was taken the way `meetings` was taken — from the RUNNING
// sibling implementation, which is the evidence ADR 0011 asks for instead of a translation:
//
//	../vigov-require/apps/api/app/modules/content/router.py:54   APIRouter(prefix="/content")
//	                                                     :66    "/items"      → content-items
//	                                                     :186   "/categories" → content-categories
//	                                                     :68,83 require_permission("content.read"/"content.update")
//
// The two segments are joined with a hyphen rather than nested, because `tools/ingress` groups every
// REST route by the FIRST segment after `/api/v1/` (`dinhtuyen.go` §`taiNguyen`) and `content` alone
// is a module, not a resource — nesting would make one ingress resource with two owners, which is
// that generator's own stop condition. It is the same shape as `map-asset-types` next door.
//
// WHAT IS STILL OWED, and it is in the final report rather than fixed from here: a ROW in that
// mapping table, written by whoever owns the file. Writing it from this service would be a second
// copy of the mapping (rule 9, forbidden #2). Changing the noun is still free — no commune runs yet
// (ubiquitous-language.md:169) — and it stops being free the day one does.
//
// §9's OWN SKETCH CANNOT SHIP, TWICE OVER: `/api/mini-app/noi-dung` is a Vietnamese path segment,
// which ADR 0011 refuses and `hooks/rest_api_guard.py` blocks; and `/api/cong/…` is outside the
// `/api/v1/` prefix that `tools/ingress` requires, so it would be a route that 404s at release
// (ubiquitous-language.md:182-185, which records exactly that for §13 of chapter 09).
//
// # THE PERMISSIONS ARE THE SPECIFICATION'S OWN AND BOTH EXIST
//
// §10.5 names them: `content.read` ("xem nội dung và danh bạ Mini App") and `content.update`
// ("sửa"). BOTH ARE SEEDED — service-identity/migrations/0001_init.sql:292-293, group
// `NỘI DUNG MINI APP` — so a commune administrator can actually tick them, and no key was invented
// (rule 5, invariant 3c).
//
// THERE IS NO `content.create` AND NONE WAS INVENTED. The `quyen` table holds exactly two keys in
// this group, and §10.5 names exactly two, so composing and editing are both `content.update`. That
// is the specification's own division of the surface rather than a gap being papered over: had a
// third key been needed, the correct move is a finding for open question #27, never an INSERT.
//
// # WHAT IS ABSENT FROM THIS FILE, so the absence is not read as unfinished work
//
//	no citizen read     §9's `GET /api/cong/mini-app/noi-dung`, "endpoint công khai cho Mini App
//	                    đọc". THREE separate blockers, each on its own: (a) the path is unshippable,
//	                    above; (b) "công khai" is rule 5's stop condition #2 and rule 4's #2 — an
//	                    unauthenticated route is the customer's call, not a handler's; (c) the Mini
//	                    App HAS NO DOMAIN (kb/00-foundation/multi-tenant-model.md), so a route with
//	                    no session cannot resolve WHICH COMMUNE'S content to return, and rule 1,
//	                    invariant 3 says a commune that cannot be resolved is a 404, never a default.
//	                    ADR 0022's citizen edge answers the commune from the SESSION
//	                    (`httpx.XaTuPhien`), which is the opposite of public. Building it would mean
//	                    deciding (b) and (c) by writing a route.
//	no danh bạ route    §9's `GET /api/cong/mini-app/danh-ba`, and §4's card. §10.7 is explicit that
//	                    the flag lives on the STAFF DIRECTORY (`hien_tren_mini_app`, chapter 12),
//	                    which service-identity owns — reading it here would be a service reaching
//	                    into another's data (rule 2). §10.5 gives `content.read` the right to SEE it,
//	                    not the right to own it. It is an identity route or an identity RPC.
//	no sync routes      §9's four `dong-bo` rows and §3's whole card. Migration 0006 lists the three
//	                    blockers in full; the short form is that nothing here can hold the commune's
//	                    portal API key (ADR 0009, decision 7 — `core/crypto` does not exist), nothing
//	                    can schedule §3's `Mỗi 6 giờ`, and nothing can make the outbound call.
//	no DELETE           §9 proposes one; §6's action column offers `✎` AND NOTHING ELSE. Rule 7 makes
//	                    a delete here a soft delete with a MANDATORY `delete_reason`, and no screen
//	                    in chapter 11 collects one. Taking an item off the Mini App is the edit route
//	                    with the checkbox cleared, which is what §7's own wording describes.
//	no image upload     §7's `Chọn tệp từ máy · JPG, PNG hoặc WebP — tối đa 50MB`. There is no
//	                    `core/storage` in this repository, so nothing can accept bytes;
//	                    `image_url` carries a link instead, and §10.6's size and format rules belong
//	                    to the uploader that does not exist.

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// The two keys of §10.5, as constants the reasoning above can refer to. The ROUTE declarations in
// routes.go spell them out as LITERALS, because tools/apidoc refuses anything there that is not one
// — a route whose key it cannot read is a route absent from kb/20-contracts/openapi.json.
const (
	QuyenDocNoiDung authz.Perm = "content.read"
	QuyenSuaNoiDung authz.Perm = "content.update"
)

// --- responses ------------------------------------------------------------------------------

// noiDungRa is one item of content as it leaves the API to a member of staff.
//
// THE FIELD NAMES SAY WHAT THE DATA IS, NOT WHAT THE SCREEN CALLS IT (ADR 0017).
//
// NOTHING HERE IS MASKED AND NOTHING HERE IS SUPPOSED TO BE (rule 3): every person named on this
// record is a member of STAFF, by business code. `title`, `summary` and `body`, however, are free
// text that a commune's news routinely spends on residents ("Trao quà cho gia đình ông Nguyễn Văn
// A…") — they must not be logged, put in a file name, a URL or a cache key on the way to a screen,
// and they are kept out of the audit ledger entirely.
type noiDungRa struct {
	// ID is the internal id. It is on the wire because every act §6 and §7 perform afterwards —
	// open, edit — addresses the item by it.
	ID string `json:"id"`

	// Type is one of §5's six codes, a Vietnamese value without diacritics (ADR 0011: only the
	// surrounding contract is English).
	Type string `json:"type"`

	// CategoryID is "" for §7's `— Chưa xếp danh mục —`. An ordinary state, not a missing value.
	CategoryID string `json:"category_id"`

	Title   string `json:"title"`
	Summary string `json:"summary"`

	// Body IS ABSENT FROM THE LIST AND PRESENT ON THE DETAIL, and the pointer is what says which.
	//
	// It is not a plain string, because "" would then mean two different things — "this article has
	// no body" and "you asked for a page, which does not carry bodies" — and a client that rendered
	// the second as the first would show an empty article with nothing saying so. The list omits the
	// key; the detail sends it, possibly as "". See store.cotNoiDungMiniApp for why a page cannot
	// carry it.
	Body *string `json:"body,omitempty"`

	// ImageURL is §7's `Ảnh đại diện`, as a LINK: there is no file storage in this repository.
	ImageURL string `json:"image_url"`

	// HasImage is §6's `Tệp đính kèm` column, `🔗 Có ảnh` / `—`. DERIVED from ImageURL and stored
	// nowhere — a column beside the URL would be two representations of one fact (rule 9).
	HasImage bool `json:"has_image"`

	// PublishedOn is §6's `Ngày đăng`, a DATE. It is sent as a date-only string rather than a
	// timestamp because that is what it is: an article carried over from the portal was published on
	// a day, not at an instant, and a timestamp would invent a time zone for it.
	PublishedOn string `json:"published_on"`

	// ViewCount is §6's `👁 {n}`. IT IS ZERO ON EVERY ROW TODAY: the only thing that legitimately
	// increments it is a resident opening the article, and the citizen-facing read is not built.
	ViewCount int `json:"view_count"`

	// Status is §6's chip: `dang-hien` · `cho-duyet` · `an`.
	Status string `json:"status"`

	// Source, SourceURL, SourceRef and HandEdited are the provenance of §8 and §10.4. They are on
	// the wire although nothing writes anything but `thu-cong` today, because §6's table is the
	// screen that has to show a commune WHICH of its articles came from its portal — and the day the
	// sync lands, the screen must not need a new field to say so.
	Source     string `json:"source"`
	SourceURL  string `json:"source_url"`
	SourceRef  string `json:"source_ref"`
	HandEdited bool   `json:"hand_edited"`

	// AuthorCode is a STAFF BUSINESS CODE (`CB-2026-7K3M9Q`), never an internal id (rule 6,
	// invariant 8). No name is joined in: this service does not own the staff directory (rule 2).
	AuthorCode string `json:"author_code"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func noiDungRaNgoai(n domain.NoiDungMiniApp, coThan bool) noiDungRa {
	ra := noiDungRa{
		ID:          n.ID,
		Type:        string(n.Loai),
		CategoryID:  n.DanhMucID,
		Title:       n.TieuDe,
		Summary:     n.TomTat,
		ImageURL:    n.AnhDaiDienURL,
		HasImage:    n.CoAnh(),
		PublishedOn: n.NgayDang.Format("2006-01-02"),
		ViewCount:   n.LuotXem,
		Status:      string(n.TrangThai),
		Source:      string(n.Nguon),
		SourceURL:   n.NguonURL,
		SourceRef:   n.NguonIDNgoai,
		HandEdited:  n.DaSuaTay,
		AuthorCode:  n.NguoiTaoMa,
		CreatedAt:   n.TaoLuc,
		UpdatedAt:   n.CapNhatLuc,
	}
	if coThan {
		than := n.NoiDung
		ra.Body = &than
	}
	return ra
}

// danhMucRa is one category of the commune's filing tree.
//
// THE FIELD IS `name` AND NOT `label`, AND THAT LINE IS DRAWN AT THE SCHEMA rather than at the route
// (kb/00-foundation/ubiquitous-language.md, the note under §Danh mục tham chiếu). The column is
// `ten` — a NAME the commune gave a category of its own — which is the `bo_phan.ten` /
// `thon_to_dan_pho.ten` side of that line, not the `nhan` side that the eight reference catalogues
// sit on.
type danhMucRa struct {
	ID string `json:"id"`

	Name string `json:"name"`

	// Slug is the business code: the handle an audit entry and an export have on this row.
	Slug string `json:"slug"`

	// ParentID is "" at the root. THE TREE IS RETURNED FLAT: §7 draws a select and §3 draws an
	// indented list, and the two want different shapes of the same rows.
	ParentID string `json:"parent_id"`

	Order     int       `json:"order"`
	CreatedAt time.Time `json:"created_at"`
}

// danhSachDanhMucRa wraps the whole tree. AN OBJECT AND NOT A BARE ARRAY, for the reason
// skills/rest-api-design gives: a top-level array cannot grow a field, so the day this route has to
// say anything about the list itself there is nowhere to put it.
type danhSachDanhMucRa struct {
	Items []danhMucRa `json:"items"`
}

func danhMucRaNgoai(dm domain.DanhMucMiniApp) danhMucRa {
	return danhMucRa{
		ID:        dm.ID,
		Name:      dm.Ten,
		Slug:      dm.Slug,
		ParentID:  dm.ChaID,
		Order:     dm.ThuTu,
		CreatedAt: dm.TaoLuc,
	}
}

// --- the read routes --------------------------------------------------------------------------

// thamSoLoc reads one filter parameter off the query string, trimmed. It is `url.Values`' own
// single-value accessor plus a TrimSpace, and it indexes the map rather than calling that method.
//
// A HELPER FOR THREE ONE-LINERS, AND THE REASON IS WORTH RECORDING rather than looking like
// over-abstraction. Two guards read this file and both key on that method's NAME:
//
//	rbac_guard         (:29-30) reads it as a MOUNTED ROUTE when the argument is a string literal, so
//	                   three of them in a handler read as three routes with no permission declared.
//	tenant_scope_guard (:37-40) reads it as a DATABASE CALL. It exempts the variable bound from
//	                   `r.URL.Query()` — but only when it can SEE that binding, and on an Edit it
//	                   sees the replacement text alone.
//
// Both are false alarms: the permission for this handler is declared on the mux statement in
// routes.go, and the commune is bound by the store. tenant_scope_guard's own comment (:62-63) says
// what a false alarm costs better than this one could — a guard that cries at a harmless line
// teaches the reader to skim past it, and then past the time it is right. Indexing the map avoids
// both reports without weakening either guard.
func thamSoLoc(q url.Values, ten string) string {
	v, co := q[ten]
	if !co || len(v) == 0 {
		return ""
	}
	return strings.TrimSpace(v[0])
}

// DanhSachNoiDung serves one page of §6's table.
// GET /api/v1/content-items
//
// # THE THREE FILTERS ARE §6'S OWN AND AN UNKNOWN `type` IS A 400
//
// §6 offers the six tabs, `Tất cả danh mục ▾` and `🔍 Tìm theo tiêu đề…`; §9 names them `loai`,
// `danh_muc` and `q`. The parameters here are the English spellings the contract surface uses
// (ADR 0011) — `type`, `category`, `q` — matching `citizen-letters`' own `?type=`.
//
// AN UNKNOWN `type` IS REFUSED RATHER THAN PASSED THROUGH. Passed through it would simply match
// nothing, and the screen would show an empty tab with no way to tell "this commune has no banners"
// from "the client sent a code that does not exist".
//
// # NO AUDIT ENTRY
//
// Rule 6, invariant 7 audits reading FULL personal data and reading ACROSS communes. This is
// neither: the query cannot leave the commune the request arrived in, and everything on the row is
// either the commune's own publishing or a staff business code. An audit ledger that grew a row per
// screen opened would bury the disclosures it exists to make findable.
//
// NO idem.* DECLARATION: a GET changes no state.
func (h *Handler) DanhSachNoiDung(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	loc := commsstore.LocNoiDung{
		Loai:      thamSoLoc(q, "type"),
		DanhMucID: thamSoLoc(q, "category"),
		Tu:        thamSoLoc(q, "q"),
	}
	if loc.Loai != "" && !domain.LoaiNoiDungHopLe(loc.Loai) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Tham số `type` phải là một trong sáu loại nội dung: tin-tuc, su-kien, thong-bao, "+
				"truyen-thanh, video, banner.", "")
		return
	}
	if len([]rune(loc.Tu)) > commsstore.TuKhoaTimToiDa {
		// THE REFUSAL DOES NOT ECHO WHAT WAS SENT. The search box is free text, and a message
		// quoting it would put it in whatever log the client's error handler writes to.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Từ khoá tìm kiếm quá dài.", "")
		return
	}

	// THE COMMUNE IS FIXED BY httpx.TenantMiddleware FROM Host, and the store binds `tenant_id` to
	// $1 from the context on every statement (rule 1, invariant 5). NOTHING HERE READS `tenant_id`
	// FROM THE QUERY STRING — a client naming its own commune is a client granting itself access
	// (rule 1, forbidden #2).
	yc, err := page.Parse(q, commsstore.SapXepNoiDungMiniApp)
	if err != nil {
		// page.HTTPError owns the mapping so every service answers a bad cursor the same way. It
		// never echoes what the client sent: a cursor is opaque, and a rejected sort key is often a
		// probe.
		status, ma, thongBao := page.HTTPError(err)
		httpx.WriteError(w, status, ma, thongBao, "")
		return
	}

	kq, err := h.d.NoiDung.DanhSach(ctx, loc, yc)
	if err != nil {
		h.d.Log.Error("sổ nội dung Mini App: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// page.Result[T] DIRECTLY — tools/apidoc understands it, so there is no second three-field struct
	// copying it and no way for the two to drift.
	//
	// make(..., 0, ...) and not a nil slice: `items` must marshal as [] on a commune that has
	// published nothing, never as null. EVERY commune is in that case today, and a client that has to
	// handle both shapes handles one of them wrong.
	ra := page.Result[noiDungRa]{
		Items:      make([]noiDungRa, 0, len(kq.Items)),
		NextCursor: kq.NextCursor,
		HasMore:    kq.HasMore,
	}
	for _, n := range kq.Items {
		ra.Items = append(ra.Items, noiDungRaNgoai(n, false))
	}
	vietJSON(w, http.StatusOK, ra)
}

// MotNoiDung serves one item with its body — §6's `✎` reopening §7's modal.
// GET /api/v1/content-items/{id}
//
// IT EXISTS BECAUSE THE LIST DOES NOT CARRY THE BODY, not as a second way of asking one question.
// store.cotNoiDungMiniApp gives the figures: a hundred articles at the body's cap is twenty million
// runes in one response, and §6's table needs a title and one line of summary.
//
// 404 AND NOT 403 FOR ANOTHER COMMUNE'S ITEM, and the two causes are deliberately indistinguishable
// from outside: the store's predicate binds the commune to $1, so an id belonging to another
// authority is simply not there. Telling the two apart would confirm what another authority holds.
func (h *Handler) MotNoiDung(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy nội dung này.", "")
		return
	}

	n, err := h.d.NoiDung.TheoID(r.Context(), id)
	if err != nil {
		h.traLoiLoiNoiDung(w, r, "đọc", err)
		return
	}
	vietJSON(w, http.StatusOK, noiDungRaNgoai(n, true))
}

// DanhSachDanhMucNoiDung serves the commune's whole Mini App category tree.
// GET /api/v1/content-categories
//
// NOT PAGINATED ON PURPOSE — store.DanhMucMiniAppStore.DanhSach gives the three reasons, and the
// bound that replaces the missing `limit` is store.TranDanhMucMiniApp.
//
// `content.read` AND NOT AnyAuthenticated, WHICH IS THE OPPOSITE CALL FROM `map-asset-types` NEXT
// DOOR — and the difference is which screens the list feeds. The eight reference catalogues fill a
// selector on nearly every screen in the system, so a permission there would empty those screens for
// everybody who is not an administrator. This tree appears on exactly one screen, the one §10.5
// already gates with `content.read`. Using the key the specification names costs nothing here and
// keeps the two routes of this module answering the same question the same way.
func (h *Handler) DanhSachDanhMucNoiDung(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ds, err := h.d.DanhMucNoiDung.DanhSach(ctx)
	if err != nil {
		if errors.Is(err, commsstore.ErrQuaNhieuDanhMucMiniApp) {
			// REFUSED RATHER THAN TRUNCATED, and 500 is the honest status: nothing the caller sent is
			// wrong. A silently short tree is a category that has quietly disappeared from §7's
			// select, so articles get filed under the wrong one and the screen looks entirely normal.
			h.d.Log.Error("danh mục Mini App: vượt trần",
				"xa", string(tenant.MustFrom(ctx)), "tran", commsstore.TranDanhMucMiniApp)
		} else {
			h.d.Log.Error("danh mục Mini App: lỗi hệ thống",
				"xa", string(tenant.MustFrom(ctx)), "err", err)
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	ra := danhSachDanhMucRa{Items: make([]danhMucRa, 0, len(ds))}
	for _, dm := range ds {
		ra.Items = append(ra.Items, danhMucRaNgoai(dm))
	}
	vietJSON(w, http.StatusOK, ra)
}

// --- the write routes -------------------------------------------------------------------------

// themNoiDungVao is the body of POST /api/v1/content-items — §7's modal, field for field.
//
// `omitempty` ON EVERY OPTIONAL FIELD, AND IT IS NOT COSMETIC. tools/apidoc marks a field REQUIRED in
// kb/20-contracts/openapi.json unless it carries `omitempty`, so without it this body would tell
// every generated client that the summary and the image MUST be sent.
//
// THERE IS NO `status` FIELD AND THERE MUST NEVER BE ONE. §7 collects a CHECKBOX; the state is
// decided from it by the use case. A client that could name the state could publish past whatever
// approval a commune later introduces, or claim `cho-duyet`, which §10.2 reserves for the sync.
//
// THERE IS NO `source`, `source_ref` OR `author_code` EITHER. Provenance decides what a later sync
// may do to the row (§10.4) and the author is the session's principal (rule 6, invariant 8). A field
// here for any of them is a client deciding something about a public record that is not its to
// decide.
//
// Unknown fields are IGNORED rather than refused, which is deliberate: a screen that reads a row and
// posts it back carries `id`, `created_at` and `view_count`, and rejecting that would make the
// obvious client wrong for no benefit.
type themNoiDungVao struct {
	// Type is one of §5's six codes. REQUIRED: §7 defaults the select to `Tin tức`, and a body with
	// no type would make this route pick a tab for the commune.
	Type string `json:"type"`

	Title string `json:"title"`

	CategoryID string `json:"category_id,omitempty"`
	Summary    string `json:"summary,omitempty"`
	Body       string `json:"body,omitempty"`
	ImageURL   string `json:"image_url,omitempty"`

	// Publish is §7's `☐ Đăng lên Mini App`. Absent means false, which is the checkbox's own default
	// and the safe direction: an item nobody chose to publish stays invisible.
	Publish bool `json:"publish,omitempty"`
}

// ThemNoiDung composes one item. POST /api/v1/content-items
//
// 201 AND THE WHOLE ROW BACK, body included. §6's list re-reads itself after the modal closes, but
// the response carries what was actually stored — in particular `status` and `published_on`, neither
// of which the author can check from the form they just submitted.
func (h *Handler) ThemNoiDung(w http.ResponseWriter, r *http.Request) {
	var vao themNoiDungVao
	if !docThan(w, r, &vao) {
		return
	}

	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.d.Log.Error("tuyến thêm nội dung Mini App chạy mà không có chủ thể — SAI CẤU HÌNH ROUTE",
			"xa", string(tenant.MustFrom(r.Context())), "duong", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	moi, err := h.d.GhiNoiDung.Them(r.Context(), domain.YeuCauThemNoiDung{
		Loai:           vao.Type,
		DanhMucID:      vao.CategoryID,
		TieuDe:         vao.Title,
		TomTat:         vao.Summary,
		NoiDung:        vao.Body,
		AnhDaiDienURL:  vao.ImageURL,
		DangLenMiniApp: vao.Publish,
	}, nguoi)
	if err != nil {
		h.traLoiLoiNoiDung(w, r, "thêm", err)
		return
	}

	// What a retry carrying the same Idempotency-Key is told about. THE ID AND NOT THE BODY: the body
	// would go into Redis, which is a cache and not a record store — and an article body is free text
	// about the commune's own business. An item has no business code, so the id is what there is.
	idem.RecordCode(r.Context(), moi.ID)
	vietJSON(w, http.StatusCreated, noiDungRaNgoai(moi, true))
}

// suaNoiDungVao is the body of PATCH /api/v1/content-items/{id}.
//
// EVERY FIELD IS A POINTER, AND THAT IS THE WHOLE REASON THIS IS A PATCH AND NOT A PUT. Each one has
// a meaningful zero: `""` for summary is "no summary", `""` for category is `— Chưa xếp danh mục —`,
// and `publish: false` is "take it off the Mini App". A full replacement cannot tell "not mentioned"
// from "cleared", so a screen editing only the title would silently unpublish the article.
type suaNoiDungVao struct {
	Type       *string `json:"type,omitempty"`
	CategoryID *string `json:"category_id,omitempty"`
	Title      *string `json:"title,omitempty"`
	Summary    *string `json:"summary,omitempty"`
	Body       *string `json:"body,omitempty"`
	ImageURL   *string `json:"image_url,omitempty"`
	Publish    *bool   `json:"publish,omitempty"`
}

// SuaNoiDung applies a partial edit. PATCH /api/v1/content-items/{id}
//
// THE RESPONSE IS THE ROW AFTER THE EDIT, WITH `hand_edited` ON IT. That field is the visible half of
// §10.4: a member of staff who corrected a portal article needs to see that the correction is now
// protected from the next sync, and this is the only place the screen learns it.
func (h *Handler) SuaNoiDung(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var vao suaNoiDungVao
	if !docThan(w, r, &vao) {
		return
	}

	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.d.Log.Error("tuyến sửa nội dung Mini App chạy mà không có chủ thể — SAI CẤU HÌNH ROUTE",
			"xa", string(tenant.MustFrom(r.Context())), "duong", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	sau, err := h.d.GhiNoiDung.Sua(r.Context(), id, domain.YeuCauSuaNoiDung{
		Loai:           vao.Type,
		DanhMucID:      vao.CategoryID,
		TieuDe:         vao.Title,
		TomTat:         vao.Summary,
		NoiDung:        vao.Body,
		AnhDaiDienURL:  vao.ImageURL,
		DangLenMiniApp: vao.Publish,
	}, nguoi)
	if err != nil {
		h.traLoiLoiNoiDung(w, r, "sửa", err)
		return
	}
	vietJSON(w, http.StatusOK, noiDungRaNgoai(sau, true))
}

// themDanhMucVao is the body of POST /api/v1/content-categories — §6's `⊞ Danh mục tin`.
type themDanhMucVao struct {
	Name string `json:"name"`
	Slug string `json:"slug"`

	// ParentID is "" or absent for a root category.
	ParentID string `json:"parent_id,omitempty"`
	Order    int    `json:"order,omitempty"`
}

// ThemDanhMucNoiDung adds one category. POST /api/v1/content-categories
func (h *Handler) ThemDanhMucNoiDung(w http.ResponseWriter, r *http.Request) {
	var vao themDanhMucVao
	if !docThan(w, r, &vao) {
		return
	}

	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.d.Log.Error("tuyến thêm danh mục Mini App chạy mà không có chủ thể — SAI CẤU HÌNH ROUTE",
			"xa", string(tenant.MustFrom(r.Context())), "duong", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	moi, err := h.d.GhiDanhMucNoiDung.Them(r.Context(), domain.YeuCauThemDanhMuc{
		Ten:   vao.Name,
		Slug:  vao.Slug,
		ChaID: vao.ParentID,
		ThuTu: vao.Order,
	}, nguoi)
	if err != nil {
		h.traLoiLoiNoiDung(w, r, "thêm danh mục", err)
		return
	}

	idem.RecordCode(r.Context(), moi.Slug)
	vietJSON(w, http.StatusCreated, danhMucRaNgoai(moi))
}

// --- the error mapping ------------------------------------------------------------------------

// traLoiLoiNoiDung maps one failure onto a status and a sentence.
//
// ONE FUNCTION FOR ALL SIX ROUTES, because copies of this mapping would drift and the copy that
// drifts is the one that answers 500 where it meant 409 — which reads to an operator as a broken
// server rather than as a rule doing its job.
//
// WHY 409 AND NOT 403 FOR A TAKEN SLUG OR A FULL TREE: the caller holds `content.update` and is
// allowed to manage the list. What is refused is this VALUE against the state of the data, and 403
// would send an administrator to the Phân quyền screen to grant a permission that would change
// nothing.
//
// THE FIFTH ARGUMENT OF httpx.WriteError IS `traceID`, NOT A FIELD NAME. httpx.Error carries `code`,
// `message` and `trace_id` and has no `field`; where a field is at fault, its name goes in the
// SENTENCE, which is where a person reads it.
func (h *Handler) traLoiLoiNoiDung(w http.ResponseWriter, r *http.Request, viec string, err error) {
	switch {
	case errors.Is(err, commsstore.ErrNoiDungKhongTonTai):
		httpx.WriteError(w, http.StatusNotFound, "not_found",
			"Không tìm thấy nội dung này.", "")
	case errors.Is(err, commsstore.ErrDanhMucKhongTonTaiMiniApp):
		httpx.WriteError(w, http.StatusConflict, "category_missing",
			"Danh mục đã chọn không còn trong danh mục tin của xã. Hãy tải lại trang và chọn lại.", "")
	case errors.Is(err, app.ErrDanhMucChaKhongTonTai):
		httpx.WriteError(w, http.StatusConflict, "parent_missing",
			"Danh mục cha đã chọn không còn trong danh mục tin của xã. Hãy tải lại trang và chọn lại.", "")
	case errors.Is(err, commsstore.ErrSlugDanhMucDaTonTai):
		// The message says WHY a slug that is nowhere on the screen is nonetheless taken: a
		// soft-deleted row keeps its code forever. Without that sentence this reads as a bug.
		httpx.WriteError(w, http.StatusConflict, "code_taken",
			"Slug này đã được dùng trong xã — kể cả khi danh mục mang slug đó đã bị xoá. "+
				"Mã đã cấp thì không cấp lại. Hãy chọn một slug khác.", "")
	case errors.Is(err, commsstore.ErrQuaNhieuDanhMucMiniApp):
		httpx.WriteError(w, http.StatusConflict, "catalogue_full",
			"Danh mục tin của xã đã đạt số mục tối đa.", "")
	case laLoiDauVaoNoiDung(err):
		// The domain's own sentence is returned: it names the field and the rule, holds no personal
		// data and no internal detail, and a second sentence written here would drift from it.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
	case errors.Is(err, app.ErrThieuNguoiTaoNoiDung):
		// The principal reached the handler but carries no staff code — this service is talking to an
		// identity older than the `ma` field. 500 is the honest answer: rule 6 does not permit a
		// business write whose trail cannot name who made it, and there is NO FALLBACK to an id.
		h.d.Log.Error("nội dung Mini App: chủ thể không có mã cán bộ — vết kiểm toán sẽ vô danh",
			"xa", string(tenant.MustFrom(r.Context())))
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
	default:
		// The wrapped error carries the store failure and never reaches the client (rule 3,
		// forbidden #3). The commune is logged because it is the only thing an operator can act on —
		// NOT the title and NOT the body.
		h.d.Log.Error("nội dung Mini App: "+viec+" lỗi hệ thống",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}

// laLoiDauVaoNoiDung reports whether this is a refusal of what the client sent, as opposed to a
// failure.
//
// LISTED EXPLICITLY RATHER THAN DEFAULTING TO 400, for the reason laLoiDauVao gives next door: a
// default of "anything I do not recognise is the client's fault" turns a database outage into a 400,
// and a client that believes its input is wrong retries with different input forever while nobody is
// told the server is broken.
func laLoiDauVaoNoiDung(err error) bool {
	for _, mot := range []error{
		domain.ErrLoaiNoiDungKhongHopLe,
		domain.ErrTieuDeNoiDungTrong, domain.ErrTieuDeNoiDungQuaDai,
		domain.ErrTomTatQuaDai, domain.ErrThanNoiDungQuaDai,
		domain.ErrURLKhongHopLe, domain.ErrURLQuaDai,
		domain.ErrTenDanhMucTrong, domain.ErrTenDanhMucQuaDai,
		domain.ErrSlugDanhMucTrong, domain.ErrSlugDanhMucSaiDinhDang, domain.ErrSlugDanhMucQuaDai,
		domain.ErrThuTuDanhMucNgoaiKhoang,
		domain.ErrDanhMucTuLamCha,
		domain.ErrMaCanBoQuaDai,
	} {
		if errors.Is(err, mot) {
			return true
		}
	}
	return false
}
