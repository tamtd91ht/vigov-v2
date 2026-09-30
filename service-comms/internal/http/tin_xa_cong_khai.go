package http

// THE PUBLIC SURFACE OF MINI APP CONTENT — what a resident reads in the Zalo Mini App with no session
// (owner decision 2026-09-27; docs/ui-ux/11-noi-dung-mini-app.md:189-190).
//
//	GET /api/v1/commune-news?host=        Public — one page of the commune's PUBLISHED items
//	GET /api/v1/commune-news/{id}?host=   Public — one published item, body as PLAIN TEXT
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
// a GET that writes is not a GET, and a public counter is a number anybody can inflate), no categories
// endpoint.
//
// ADDED 2026-09-29 for SRS M6.1.4 (the Mini App's news screen, P0), all OPTIONAL on the wire so the
// contract only grows (rule 2, forbidden #4): `type` on each item, an optional `type` filter on the list,
// and the provenance pair `source` / `source_url`. STILL ABSENT, each for a reason on tinXaRa: the image
// and the view count.

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"

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

// NoiDungCongKhaiDoc is the two PUBLISHED-only reads. *commsstore.NoiDungMiniAppStore satisfies it. There
// is deliberately no way to reach the staff register's all-states reads from this surface.
type NoiDungCongKhaiDoc interface {
	DanhSachCongKhai(ctx context.Context, itemType domain.LoaiNoiDung, yc page.Request) (page.Result[domain.NoiDungMiniApp], error)
	CongKhaiTheoID(ctx context.Context, id string) (domain.NoiDungMiniApp, error)
}

// DepsCongKhai is everything the public routes may touch. Nothing else is reachable from them.
type DepsCongKhai struct {
	Xa      TraXaTheoHost
	NoiDung NoiDungCongKhaiDoc

	// DanhMuc resolves `category_name`. The commune's category tree is the name under which it files its
	// own public articles — public by construction, and bounded (commsstore.TranDanhMucMiniApp).
	DanhMuc DanhMucMiniAppDoc

	Log *slog.Logger
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
//   - NO IMAGE. `anh_dai_dien_url` is a link a member of staff typed to a file SOME OTHER SYSTEM serves
//     (migrations/0006_noi_dung_mini_app.sql:277-280) — not an approved derivative in the public bucket
//     (ADR 0052 §2). Handing it to every resident would make the Mini App fetch whatever host was typed.
//     It arrives when the image is stored as a public-bucket derivative.
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
}

func tinXaRaNgoai(n domain.NoiDungMiniApp, tenDanhMuc map[string]string, coThan bool) tinXaRa {
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
		// The refusal names the closed list and never echoes what was sent.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Tham số `type` phải là một trong sáu loại nội dung: tin-tuc, su-kien, thong-bao, "+
				"truyen-thanh, video, banner.", "")
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
	kq, err := h.d.NoiDung.DanhSachCongKhai(ctx, itemType, yc)
	if err != nil {
		h.loi500(ctx, w, "tin của xã", err)
		return
	}
	ten, err := h.tenDanhMuc(ctx, kq.Items)
	if err != nil {
		h.loi500(ctx, w, "tin của xã: tên danh mục", err)
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
		ra.Items = append(ra.Items, tinXaRaNgoai(n, ten, false))
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
	vietJSON(w, http.StatusOK, tinXaRaNgoai(n, ten, true))
}
