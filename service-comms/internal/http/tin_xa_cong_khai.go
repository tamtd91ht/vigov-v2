package http

// THE PUBLIC SURFACE OF MINI APP CONTENT — what a resident reads in the Zalo Mini App with no session
// (owner decision 2026-09-27; docs/ui-ux/11-noi-dung-mini-app.md:189-190).
//
//	GET /api/v1/commune-news?host=              Public — one page of the commune's PUBLISHED items
//	GET /api/v1/commune-news/{id}?host=         Public — one published item, body as PLAIN TEXT
//	GET /api/v1/commune-news/categories?host=   Public — the categories that hold published items
//
// THIS LIFTS TWO OF THE THREE BLOCKERS noi_dung_mini_app.go RECORDED for "no citizen read" — and says
// how, so the note there is read as history rather than as a contradiction:
//
//	(a) the path    `commune-news` is the owner's noun, under /api/v1/, English (not §9's sketch)
//	(b) "công khai" decided by the owner 2026-09-27: Public, no sign-in
//	(c) no domain   the Mini App names the commune's DOMAIN in `?host=`; the PLATFORM resolves it,
//	                server-side (xaTheoHost). No tenant_id is ever taken from the request (rule 1,
//	                forbidden #2), and no commune ULID is ever returned
//
// WHAT IS DELIBERATELY ABSENT (owner decision 2026-09-27): no view counting (`luot_xem` stays as it is —
// a GET that writes is not a GET, and a public counter is a number anybody can inflate).
//
// A CATEGORIES ENDPOINT WAS ABSENT TOO, BY THE SAME 2026-09-27 DECISION — AND ON 2026-09-30 THE USER
// REVERSED IT, explicitly, for the Mini App news tab's two-level chip row (vigov-require NewsPage.tsx:
// root categories, then the chosen root's children; choosing a parent includes its descendants' items;
// no chip for a category with nothing published). WHY IT COULD NOT STAY CLIENT-SIDE: chips derived from
// the items the client had already loaded only ever saw the FIRST PAGE, so a category whose items sat on
// a later page had no chip — which is why the client-side category chips were removed in 49cca1f9.
// Only the server sees every published item, so the row is now `GET …/categories` (PublicNewsCategories)
// plus an optional `category=` on the list, both computed from the whole commune, never from one page.
//
// ADDED 2026-09-29 for SRS M6.1.4 (the Mini App's news screen, P0), all OPTIONAL on the wire so the
// contract only grows (rule 2, forbidden #4): `type` on each item, an optional `type` filter on the list,
// and the provenance pair `source` / `source_url`. STILL ABSENT, each for a reason on tinXaRa: the image
// and the view count.
//
// ADDED 2026-09-30 (ADR 0047 §6, migration 0011), optional in the same way: `published_at` (G1),
// `event_starts_at` / `event_ends_at` / `event_place` on `su-kien`, `video_url` on `video`.

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// TraXaTheoHost is the one platform read the public surface makes. *platformclient.Directory
// satisfies it — the SAME client main() already dials for the Host edge.
//
// NOT tenant.Directory: its bool folds an outage into "unknown Host", which is right for the staff edge
// (404 either way) and wrong here, where an outage must be 503 and never an empty list that reads as
// "this commune published nothing". See platformclient.XaTheoHost.
type TraXaTheoHost interface {
	XaTheoHost(ctx context.Context, host string) (tenant.Tenant, bool, error)
}

// NoiDungCongKhaiDoc is the PUBLISHED-only reads. *commsstore.NoiDungMiniAppStore satisfies it. There
// is deliberately no way to reach the staff register's all-states reads from this surface.
type NoiDungCongKhaiDoc interface {
	DanhSachCongKhai(ctx context.Context, itemType domain.LoaiNoiDung, categoryID string, yc page.Request) (page.Result[domain.NoiDungMiniApp], error)
	CongKhaiTheoID(ctx context.Context, id string) (domain.NoiDungMiniApp, error)

	// PublishedCategoryIDs is the live categories holding ≥1 published item DIRECTLY (itemType "" = any).
	PublishedCategoryIDs(ctx context.Context, itemType domain.LoaiNoiDung) ([]string, error)
}

// DepsCongKhai is everything the public routes may touch. Nothing else is reachable from them.
type DepsCongKhai struct {
	Xa      TraXaTheoHost
	NoiDung NoiDungCongKhaiDoc

	// DanhMuc resolves `category_name`. The commune's category tree is the name under which it files its
	// own public articles — public by construction, and bounded (commsstore.TranDanhMucMiniApp).
	DanhMuc DanhMucMiniAppDoc

	// CoverImages resolves `image_url`: the public-bucket URL of a cover's PUBLISHED derivative.
	// *app.ContentCovers satisfies it.
	CoverImages PublicCoverImages

	Log *slog.Logger
}

// PublicCoverImages maps cover file ids to the anonymous URL of their published derivative; a file
// with no recorded public copy is absent (ADR 0052 §2, §11).
type PublicCoverImages interface {
	PublicImageURLs(ctx context.Context, fileIDs []string) (map[string]string, error)
}

// HandlerCongKhai serves the public routes. A SEPARATE TYPE from Handler, with its own Deps, so a
// public route cannot reach a staff store even by typing it.
type HandlerCongKhai struct {
	d DepsCongKhai
}

func newHandlerCongKhai(d DepsCongKhai) *HandlerCongKhai {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &HandlerCongKhai{d: d}
}

// tinXaRa is one published item on the public wire.
//
// EVERY TEXT FIELD IS PLAIN TEXT (domain.VanBanThuanChoDan) — title and summary too, not only the body:
// the staff screen accepts markup in any of them, and "no HTML reaches the citizen" is a property of the
// response, not of one field.
//
// WHAT IS ABSENT IS THE CONTRACT: no `status` (it is always `dang-hien`), no author code, no portal id
// (`source_ref`), no category id, no commune id or host, and:
//
//   - NOT `anh_dai_dien_url`. It is a link a member of staff typed to a file SOME OTHER SYSTEM serves
//     (migrations/0006_noi_dung_mini_app.sql:277-280) — not an approved derivative in the public bucket
//     (ADR 0052 §2). Handing it to every resident would make the Mini App fetch whatever host was typed.
//     The image residents get is `image_url` (2026-10-01): the UPLOADED cover's published derivative.
//   - NO VIEW COUNT. Nothing increments `luot_xem` (0006:93-97), so it is 0 on every row; a public 0 reads
//     as "nobody read this", which is false.
type tinXaRa struct {
	// ID is the item's own ULID — what the detail route takes. Random, so it enumerates nothing (rule 4,
	// invariant 4). NEVER the commune's id.
	ID string `json:"id"`

	// Type is one of the six closed codes (domain.LoaiNoiDungHopLe) — the same values the staff register
	// sends and the list's `type` filter takes. Optional on the wire only so the contract grows additively;
	// every row carries one (NOT NULL + CHECK, 0006:264,367-368).
	Type string `json:"type,omitempty"`

	Title   string `json:"title"`
	Summary string `json:"summary"`

	// PublishedOn is `ngay_dang`, a DATE — the same field and format the staff register sends.
	PublishedOn string `json:"published_on"`

	// CategoryName is "" when the item is filed nowhere, or under a category since soft-deleted.
	CategoryName string `json:"category_name"`

	// Source is `thu-cong` (composed in ViGov) or `dong-bo-cong` (taken from the commune's own portal).
	Source string `json:"source,omitempty"`

	// SourceURL is the original article on the commune's portal. ABSENT unless it is an http(s) link:
	// the only writer is the portal sync, which is not built, so nothing has validated this column at
	// write time — a `javascript:` value must never reach a link in the Mini App.
	SourceURL string `json:"source_url,omitempty"`

	// Body is PLAIN TEXT, paragraphs separated by one blank line ("\n\n"), line breaks by "\n". Absent
	// from the list (the page does not read it — store.cotNoiDungMiniApp), present on the detail.
	Body *string `json:"body,omitempty"`

	// ADDED 2026-09-30 (ADR 0047 §6 (2), (3), G1; migration 0011). All OPTIONAL — the contract only
	// grows (rule 2, forbidden #4) — and on the list AND the detail.

	// PublishedAt is the instant of the FIRST publish (RFC 3339), never changed afterwards. ABSENT
	// when not recorded (published before 0011 — no backfill): the client falls back to PublishedOn.
	PublishedAt *time.Time `json:"published_at,omitempty"`

	// EventStartsAt, EventEndsAt (RFC 3339) and EventPlace (PLAIN TEXT, like every text field here) —
	// only on `su-kien`, only when set.
	EventStartsAt *time.Time `json:"event_starts_at,omitempty"`
	EventEndsAt   *time.Time `json:"event_ends_at,omitempty"`
	EventPlace    string     `json:"event_place,omitempty"`

	// VideoURL — only on `video`, only when it is an http(s) link: re-checked on the way out like
	// SourceURL. The write path already refuses anything else (and so does 0011's CHECK); the output
	// check is the second wall on a surface anybody can read.
	VideoURL string `json:"video_url,omitempty"`

	// ImageURL is the cover (ADR 0047 §6 (1), 2026-10-01): the anonymous URL of the 1280 px JPEG
	// DERIVATIVE in the public bucket, never the original (ADR 0052 §2). ABSENT when the item has no
	// uploaded cover or no public copy is recorded for it. On the list and the detail. The URL is
	// immutable and cached a year (core/storage.PublicCacheControl).
	ImageURL string `json:"image_url,omitempty"`
}

// coverIDs collects the cover file ids of the PUBLISHED items of a page — the only ones the public
// surface may resolve an image for.
func coverIDs(ds []domain.NoiDungMiniApp) []string {
	ids := make([]string, 0, len(ds))
	seen := make(map[string]bool, len(ds))
	for _, n := range ds {
		if n.HienChoDan() && n.CoverImageFileID != "" && !seen[n.CoverImageFileID] {
			seen[n.CoverImageFileID] = true
			ids = append(ids, n.CoverImageFileID)
		}
	}
	return ids
}

func tinXaRaNgoai(n domain.NoiDungMiniApp, tenDanhMuc map[string]string, images map[string]string, coThan bool) tinXaRa {
	ra := tinXaRa{
		ID:           n.ID,
		Type:         string(n.Loai),
		Title:        domain.VanBanThuanChoDan(n.TieuDe),
		Summary:      domain.VanBanThuanChoDan(n.TomTat),
		PublishedOn:  n.NgayDang.Format("2006-01-02"),
		CategoryName: tenDanhMuc[n.DanhMucID],
		Source:       string(n.Nguon),
	}
	if u, err := domain.ChuanHoaURL(n.NguonURL); err == nil {
		// An invalid stored link is dropped, not an error: the article is still worth showing, and the
		// refusal is the point. err is deliberately not returned — there is nothing a resident can do.
		ra.SourceURL = u
	}
	ra.PublishedAt = instantOut(n.PublishedAt)
	// THE TYPE GATES EACH FIELD HERE TOO, not only the CHECK underneath: an event window printed on a
	// news article is a date a resident acts on.
	if n.Loai == domain.LoaiSuKien {
		ra.EventStartsAt = instantOut(n.EventStartsAt)
		ra.EventEndsAt = instantOut(n.EventEndsAt)
		ra.EventPlace = domain.VanBanThuanChoDan(n.EventPlace)
	}
	if n.Loai == domain.LoaiVideo {
		if u, err := domain.ChuanHoaURL(n.VideoURL); err == nil {
			// Dropped, not an error — the same reasoning as SourceURL above.
			ra.VideoURL = u
		}
	}
	if n.HienChoDan() && n.CoverImageFileID != "" {
		// ONLY A PUBLISHED ITEM'S, a second wall under the store's predicate and coverIDs.
		ra.ImageURL = images[n.CoverImageFileID]
	}
	if coThan {
		than := domain.VanBanThuanChoDan(n.NoiDung)
		ra.Body = &than
	}
	return ra
}

// viet400Host is the ONE refusal of a malformed `host`. The body never echoes what was sent.
func viet400Host(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusBadRequest, "invalid_host", "Tên miền của xã không hợp lệ.", "")
}

// viet404Tin is the ONE "not here" of the detail route — nonexistent, another commune's, not published,
// soft-deleted, or a domain no active commune holds. Byte-identical for all of them (rule 4, forbidden #2).
func viet404Tin(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy tin này.", "")
}

// xaTheoHost resolves an ALREADY-VALIDATED host to the ACTIVE commune the platform says holds it.
//
//	ok=false           503 already written: the platform could not be asked. Never "no commune".
//	ok=true, co=false  unknown, reserved or inactive — one answer; the caller writes its "nothing here".
//	ok=true, co=true   the handler puts xa.ID into the context (tenant.Into) and reads a scoped store.
//
// The host is logged on an outage because it is a domain, not a person, and it has passed
// domain.HopLeTenMienXa, so it cannot carry a log-injection payload.
func (h *HandlerCongKhai) xaTheoHost(w http.ResponseWriter, r *http.Request, host, viec string) (tenant.Tenant, bool, bool) {
	xa, co, err := h.d.Xa.XaTheoHost(r.Context(), host)
	if err != nil {
		h.d.Log.WarnContext(r.Context(), viec+": không hỏi được dịch vụ nền tảng", "host", host, "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "platform_unavailable",
			"Hệ thống đang bận. Vui lòng thử lại sau ít phút.", "")
		return tenant.Tenant{}, false, false
	}
	if !co || !xa.Active || !xa.ID.Valid() {
		return tenant.Tenant{}, false, true
	}
	return xa, true, true
}

// tenDanhMuc reads the commune's category names, keyed by id — only when some item is filed under one.
func (h *HandlerCongKhai) tenDanhMuc(ctx context.Context, ds []domain.NoiDungMiniApp) (map[string]string, error) {
	can := false
	for _, n := range ds {
		if n.DanhMucID != "" {
			can = true
			break
		}
	}
	if !can {
		return nil, nil
	}
	dm, err := h.d.DanhMuc.DanhSach(ctx)
	if err != nil {
		return nil, err
	}
	ten := make(map[string]string, len(dm))
	for _, d := range dm {
		ten[d.ID] = d.Ten
	}
	return ten, nil
}

func (h *HandlerCongKhai) loi500(ctx context.Context, w http.ResponseWriter, viec string, err error) {
	// The wrapped error carries the store failure, never a title or a body — a commune's news can name
	// residents (rule 3) — and never reaches the client.
	h.d.Log.ErrorContext(ctx, viec+": lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
	httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
}

// DanhSachTinXa serves one page of the commune's published items, newest first.
// GET /api/v1/commune-news?host=
//
// CURSOR-PAGINATED (skills/rest-api-design §5): `limit` (default page.DefaultLimit, capped at
// page.MaxLimit) and `cursor`, the same parameters and the same `next_cursor` / `has_more` every list
// in this system uses. The order is `tao_luc` DESC — SapXepNoiDungMiniApp's only column.
//
// UNKNOWN, RESERVED OR INACTIVE DOMAIN → 200 with an EMPTY PAGE, byte-identical to an active commune
// that has published nothing. The page parameters are still validated first, so a bad cursor is a 400
// whatever the domain — otherwise the 400/200 split would tell which domains are communes.
//
// OPTIONAL `type` FILTER: one of the six codes, the same values and the same 400 as the staff register's
// filter. Validated BEFORE the platform is asked, like the cursor, so the 400/200 split says nothing about
// which domains are communes. Absent or empty = every type.
//
// OPTIONAL `category` FILTER (user decision 2026-09-30, the chip row): an id from GET …/categories. Only
// items filed under that category OR ANY LIVE DESCENDANT, ANDed with `type`. Its SHAPE is validated before
// the platform is asked (domain.ValidPublicCategoryID); a well-formed id that is unknown, soft-deleted or
// another commune's is an EMPTY PAGE — the same bytes as a category with nothing published, so the answer
// never says which ids exist anywhere. Absent or empty = every category, items filed nowhere included.
//
// NO AUDIT ENTRY: nothing is written; this is what the commune chose to publish, read inside the one
// commune the host resolved to (rule 6, invariant 7 asks for neither case).
func (h *HandlerCongKhai) DanhSachTinXa(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	gia := q["host"]
	if len(gia) != 1 || !domain.HopLeTenMienXa(gia[0]) {
		viet400Host(w)
		return
	}
	itemType, typeOK := publicTypeFilter(q)
	if !typeOK {
		viet400Type(w)
		return
	}
	categoryID, categoryOK := publicCategoryFilter(q)
	if !categoryOK {
		// Shape only — a well-formed id naming nothing is an empty page below, never a 400.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Tham số `category` không hợp lệ.", "")
		return
	}
	yc, err := page.Parse(q, commsstore.SapXepNoiDungMiniApp)
	if err != nil {
		status, ma, thongBao := page.HTTPError(err)
		httpx.WriteError(w, status, ma, thongBao, "")
		return
	}

	xa, co, ok := h.xaTheoHost(w, r, gia[0], "tin của xã")
	if !ok {
		return
	}
	ra := page.Result[tinXaRa]{Items: []tinXaRa{}} // `[]`, never `null`
	if !co {
		vietJSON(w, http.StatusOK, ra)
		return
	}

	ctx := tenant.Into(r.Context(), xa.ID)
	kq, err := h.d.NoiDung.DanhSachCongKhai(ctx, itemType, categoryID, yc)
	if err != nil {
		h.loi500(ctx, w, "tin của xã", err)
		return
	}
	ten, err := h.tenDanhMuc(ctx, kq.Items)
	if err != nil {
		h.loi500(ctx, w, "tin của xã: tên danh mục", err)
		return
	}
	images, err := h.d.CoverImages.PublicImageURLs(ctx, coverIDs(kq.Items))
	if err != nil {
		h.loi500(ctx, w, "tin của xã: ảnh bìa", err)
		return
	}

	ra.NextCursor, ra.HasMore = kq.NextCursor, kq.HasMore
	for _, n := range kq.Items {
		if !n.HienChoDan() {
			// THE SECOND WALL. The store already binds `dang-hien`; a row in any other state here means
			// that predicate broke. Dropped, never shown — and loud, because it should be impossible.
			h.d.Log.ErrorContext(ctx, "tin của xã: kho trả một mục CHƯA ĐĂNG trên tuyến công khai — đã bỏ",
				"xa", string(xa.ID), "trang_thai", string(n.TrangThai))
			continue
		}
		ra.Items = append(ra.Items, tinXaRaNgoai(n, ten, images, false))
	}
	vietJSON(w, http.StatusOK, ra)
}

// publicTypeFilter reads the list's OPTIONAL `type`: ("", true) when absent or empty, (code, true) for
// one of the six, ("", false) for anything else — the caller answers 400.
//
// A HELPER AND NOT AN INLINE `if`, FOR THE GENERATED CONTRACT: tools/apidoc marks a query parameter
// `required: true` whenever a variable read from it appears in the condition of a 400 branch
// (truyvan.go:158-174, docThanHam). Inline, `type` would be published as REQUIRED — a breaking change
// on paper to a route the Mini App already calls, and every generated client would have to send it.
// Here the handler's condition reads only the bool, and the read inside this function has no 400.
func publicTypeFilter(q url.Values) (domain.LoaiNoiDung, bool) {
	v := thamSoLoc(q, "type")
	if v == "" {
		return "", true
	}
	if !domain.LoaiNoiDungHopLe(v) {
		return "", false
	}
	return domain.LoaiNoiDung(v), true
}

// viet400Type is the ONE refusal of a `type` outside the six. It names the closed list and never echoes
// what was sent.
func viet400Type(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
		"Tham số `type` phải là một trong sáu loại nội dung: tin-tuc, su-kien, thong-bao, "+
			"truyen-thanh, video, banner.", "")
}

// publicCategoryFilter reads the list's OPTIONAL `category`: ("", true) when absent or empty, (id, true)
// for a well-formed id, ("", false) otherwise — the caller answers 400. A HELPER for the same reason as
// publicTypeFilter: inline, tools/apidoc would publish `category` as REQUIRED.
func publicCategoryFilter(q url.Values) (string, bool) {
	v := thamSoLoc(q, "category")
	if v == "" {
		return "", true
	}
	if !domain.ValidPublicCategoryID(v) {
		return "", false
	}
	return v, true
}

// publicCategoryOut is one chip on the public wire.
//
// WHAT IS ABSENT IS THE CONTRACT: no slug (the client selects by id and prints the name; a slug is a
// staff business code with no reader here), no commune id, no creation time, no item count (a count is
// a second figure that has to agree with the list, and the chip row does not print one).
type publicCategoryOut struct {
	// ID is the category's ULID — what `category=` on the list takes. Random; enumerates nothing.
	ID string `json:"id"`

	// Name is PLAIN TEXT (domain.VanBanThuanChoDan), like every text field on this surface.
	Name string `json:"name"`

	// ParentID is ABSENT on a root — row 1 of the chip bar. Always the id of another item in the same
	// response: a category whose parent is not shown is not shown either.
	ParentID string `json:"parent_id,omitempty"`

	// Order is `thu_tu`, the order the commune arranged its own tree in. Items already arrive sorted by
	// it (then by slug); it is sent so a client grouping children under parents need not rely on that.
	Order int `json:"order"`
}

// publicCategoriesOut is the whole answer: never paginated, bounded by commsstore.TranDanhMucMiniApp.
type publicCategoriesOut struct {
	Items []publicCategoryOut `json:"items"`
}

// PublicNewsCategories serves the chip row of the Mini App news tab.
// GET /api/v1/commune-news/categories?host=[&type=]
//
// A CATEGORY IS LISTED IF IT, OR ANY LIVE DESCENDANT, HOLDS AT LEAST ONE PUBLISHED, NOT-DELETED ITEM (of
// `type`, when given). Soft-deleted categories never appear, nor does a live category cut off from the
// roots by a deleted parent (domain.CategoriesWithPublishedItems). So every chip, when chosen, lists at
// least one item on GET /api/v1/commune-news?category=<id>&type=<same>.
//
// THE SAME NEGATIVES AS THE LIST: `host` and `type` are validated before the platform is asked; an
// unknown, reserved or inactive domain is `{"items":[]}` with 200, byte-identical to an active commune
// that has published nothing; an unreachable platform is 503.
//
// OVER THE CAP (commsstore.TranDanhMucMiniApp) IS A 500, NOT A SHORT LIST — the refusal the staff tree
// read makes: a silently missing chip is a category a resident can never reach.
//
// NO AUDIT ENTRY: nothing is written, and this is the commune's own published filing, read inside the
// one commune the host resolved to.
func (h *HandlerCongKhai) PublicNewsCategories(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	gia := q["host"]
	if len(gia) != 1 || !domain.HopLeTenMienXa(gia[0]) {
		viet400Host(w)
		return
	}
	itemType, typeOK := publicTypeFilter(q)
	if !typeOK {
		viet400Type(w)
		return
	}

	xa, co, ok := h.xaTheoHost(w, r, gia[0], "danh mục tin của xã")
	if !ok {
		return
	}
	ra := publicCategoriesOut{Items: []publicCategoryOut{}} // `[]`, never `null`
	if !co {
		vietJSON(w, http.StatusOK, ra)
		return
	}

	ctx := tenant.Into(r.Context(), xa.ID)
	withItems, err := h.d.NoiDung.PublishedCategoryIDs(ctx, itemType)
	if err != nil {
		h.categoryError(ctx, w, err)
		return
	}
	if len(withItems) == 0 {
		// Nothing published is filed anywhere: the tree is not read at all.
		vietJSON(w, http.StatusOK, ra)
		return
	}
	live, err := h.d.DanhMuc.DanhSach(ctx)
	if err != nil {
		h.categoryError(ctx, w, err)
		return
	}
	for _, c := range domain.CategoriesWithPublishedItems(live, withItems) {
		ra.Items = append(ra.Items, publicCategoryOut{
			ID:       c.ID,
			Name:     domain.VanBanThuanChoDan(c.Ten),
			ParentID: c.ChaID,
			Order:    c.ThuTu,
		})
	}
	vietJSON(w, http.StatusOK, ra)
}

func (h *HandlerCongKhai) categoryError(ctx context.Context, w http.ResponseWriter, err error) {
	if errors.Is(err, commsstore.ErrQuaNhieuDanhMucMiniApp) {
		h.d.Log.ErrorContext(ctx, "danh mục tin của xã: vượt trần",
			"xa", string(tenant.MustFrom(ctx)), "tran", commsstore.TranDanhMucMiniApp)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	h.loi500(ctx, w, "danh mục tin của xã", err)
}

// maTinToiDa bounds `{id}` before it reaches the store. Item ids are 26-character ULIDs; anything
// longer cannot name one, and answers the same 404 without a query.
const maTinToiDa = 64

// MotTinXa serves one published item, body as plain text. GET /api/v1/commune-news/{id}?host=
//
// ONE 404 for: no such id, another commune's id, an item not published (`an`, `cho-duyet`), a
// soft-deleted item, and a domain no active commune holds. Byte-identical — telling any two apart would
// say what a commune holds or is preparing (rule 4, forbidden #2).
func (h *HandlerCongKhai) MotTinXa(w http.ResponseWriter, r *http.Request) {
	gia := r.URL.Query()["host"]
	if len(gia) != 1 || !domain.HopLeTenMienXa(gia[0]) {
		viet400Host(w)
		return
	}
	id := r.PathValue("id")
	if id == "" || len(id) > maTinToiDa {
		viet404Tin(w)
		return
	}

	xa, co, ok := h.xaTheoHost(w, r, gia[0], "chi tiết tin của xã")
	if !ok {
		return
	}
	if !co {
		viet404Tin(w)
		return
	}

	ctx := tenant.Into(r.Context(), xa.ID)
	n, err := h.d.NoiDung.CongKhaiTheoID(ctx, id)
	if errors.Is(err, commsstore.ErrNoiDungKhongTonTai) {
		viet404Tin(w)
		return
	}
	if err != nil {
		h.loi500(ctx, w, "chi tiết tin của xã", err)
		return
	}
	if !n.HienChoDan() {
		h.d.Log.ErrorContext(ctx, "chi tiết tin của xã: kho trả một mục CHƯA ĐĂNG trên tuyến công khai — trả 404",
			"xa", string(xa.ID), "trang_thai", string(n.TrangThai))
		viet404Tin(w)
		return
	}
	ten, err := h.tenDanhMuc(ctx, []domain.NoiDungMiniApp{n})
	if err != nil {
		h.loi500(ctx, w, "chi tiết tin của xã: tên danh mục", err)
		return
	}
	images, err := h.d.CoverImages.PublicImageURLs(ctx, coverIDs([]domain.NoiDungMiniApp{n}))
	if err != nil {
		h.loi500(ctx, w, "chi tiết tin của xã: ảnh bìa", err)
		return
	}
	vietJSON(w, http.StatusOK, tinXaRaNgoai(n, ten, images, true))
}
