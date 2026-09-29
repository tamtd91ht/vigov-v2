package http

// THE PUBLIC SURFACE OF MINI APP CONTENT — what a resident reads in the Zalo Mini App with no session
// (owner decision 2026-09-27; docs/ui-ux/11-noi-dung-mini-app.md:189-190).
//
//	GET /api/v1/commune-news?host=        Public — one page of the commune's PUBLISHED items
//	GET /api/v1/commune-news/{id}?host=   Public — one published item, body as PLAIN TEXT
//
// THIS LIFTS TWO OF THE THREE BLOCKERS content_item.go RECORDED for "no citizen read" — and says
// how, so the note there is read as history rather than as a contradiction:
//
//	(a) the path    `commune-news` is the owner's noun, under /api/v1/, English (not §9's sketch)
//	(b) "công khai" decided by the owner 2026-09-27: Public, no sign-in
//	(c) no domain   the Mini App names the commune's DOMAIN in `?host=`; the PLATFORM resolves it,
//	                server-side (resolveTenant). No tenant_id is ever taken from the request (rule 1,
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

// TenantByHost is the one platform read the public surface makes. *platformclient.Directory
// satisfies it — the SAME client main() already dials for the Host edge.
//
// NOT tenant.Directory: its bool folds an outage into "unknown Host", which is right for the staff edge
// (404 either way) and wrong here, where an outage must be 503 and never an empty list that reads as
// "this commune published nothing". See platformclient.XaTheoHost.
type TenantByHost interface {
	XaTheoHost(ctx context.Context, host string) (tenant.Tenant, bool, error)
}

// PublicContentReader is the two PUBLISHED-only reads. *commsstore.ContentItemStore satisfies it. There
// is deliberately no way to reach the staff register's all-states reads from this surface.
type PublicContentReader interface {
	ListPublic(ctx context.Context, itemType domain.ContentType, req page.Request) (page.Result[domain.ContentItem], error)
	PublicByID(ctx context.Context, id string) (domain.ContentItem, error)
}

// PublicDeps is everything the public routes may touch. Nothing else is reachable from them.
type PublicDeps struct {
	Tenants      TenantByHost
	ContentItems PublicContentReader

	// Categories resolves `category_name`. The commune's category tree is the name under which it files
	// its own public articles — public by construction, and bounded (commsstore.ContentCategoryCeiling).
	Categories ContentCategoryReader

	Log *slog.Logger
}

// PublicHandler serves the public routes. A SEPARATE TYPE from Handler, with its own Deps, so a
// public route cannot reach a staff store even by typing it.
type PublicHandler struct {
	d PublicDeps
}

func newPublicHandler(d PublicDeps) *PublicHandler {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &PublicHandler{d: d}
}

// tinXaRa is one published item on the public wire.
//
// vi-name-ok: the type name is the OpenAPI component name `comms.tinXaRa` in
// kb/20-contracts/openapi.json; renaming it is a contract change, not layer A of ADR 0061.
//
// EVERY TEXT FIELD IS PLAIN TEXT (domain.PlainTextForCitizen) — title and summary too, not only the body:
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

	// Type is one of the six closed codes (domain.IsValidContentType) — the same values the staff register
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
	// from the list (the page does not read it — store.contentItemColumns), present on the detail.
	Body *string `json:"body,omitempty"`
}

func toCommuneNewsOut(n domain.ContentItem, categoryNames map[string]string, withBody bool) tinXaRa {
	out := tinXaRa{
		ID:           n.ID,
		Type:         string(n.Type),
		Title:        domain.PlainTextForCitizen(n.Title),
		Summary:      domain.PlainTextForCitizen(n.Summary),
		PublishedOn:  n.PublishedOn.Format("2006-01-02"),
		CategoryName: categoryNames[n.CategoryID],
		Source:       string(n.Source),
	}
	if u, err := domain.NormalizeURL(n.SourceURL); err == nil {
		// An invalid stored link is dropped, not an error: the article is still worth showing, and the
		// refusal is the point. err is deliberately not returned — there is nothing a resident can do.
		out.SourceURL = u
	}
	if withBody {
		body := domain.PlainTextForCitizen(n.Body)
		out.Body = &body
	}
	return out
}

// writeInvalidHost is the ONE refusal of a malformed `host`. The body never echoes what was sent.
func writeInvalidHost(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusBadRequest, "invalid_host", "Tên miền của xã không hợp lệ.", "")
}

// writeNewsNotFound is the ONE "not here" of the detail route — nonexistent, another commune's, not
// published, soft-deleted, or a domain no active commune holds. Byte-identical for all of them (rule 4,
// forbidden #2).
func writeNewsNotFound(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy tin này.", "")
}

// resolveTenant resolves an ALREADY-VALIDATED host to the ACTIVE commune the platform says holds it.
//
//	ok=false              503 already written: the platform could not be asked. Never "no commune".
//	ok=true, found=false  unknown, reserved or inactive — one answer; the caller writes its "nothing here".
//	ok=true, found=true   the handler puts t.ID into the context (tenant.Into) and reads a scoped store.
//
// The host is logged on an outage because it is a domain, not a person, and it has passed
// domain.IsValidTenantDomain, so it cannot carry a log-injection payload.
func (h *PublicHandler) resolveTenant(w http.ResponseWriter, r *http.Request, host, op string) (tenant.Tenant, bool, bool) {
	t, found, err := h.d.Tenants.XaTheoHost(r.Context(), host)
	if err != nil {
		h.d.Log.WarnContext(r.Context(), op+": không hỏi được dịch vụ nền tảng", "host", host, "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "platform_unavailable",
			"Hệ thống đang bận. Vui lòng thử lại sau ít phút.", "")
		return tenant.Tenant{}, false, false
	}
	if !found || !t.Active || !t.ID.Valid() {
		return tenant.Tenant{}, false, true
	}
	return t, true, true
}

// categoryNames reads the commune's category names, keyed by id — only when some item is filed under one.
func (h *PublicHandler) categoryNames(ctx context.Context, items []domain.ContentItem) (map[string]string, error) {
	needed := false
	for _, n := range items {
		if n.CategoryID != "" {
			needed = true
			break
		}
	}
	if !needed {
		return nil, nil
	}
	// Scoped in the store: ContentCategoryStore.List reads through db.For(ctx).Query.
	categories, err := h.d.Categories.List(ctx)
	if err != nil {
		return nil, err
	}
	names := make(map[string]string, len(categories))
	for _, c := range categories {
		names[c.ID] = c.Name
	}
	return names, nil
}

func (h *PublicHandler) writeInternalError(ctx context.Context, w http.ResponseWriter, op string, err error) {
	// The wrapped error carries the store failure, never a title or a body — a commune's news can name
	// residents (rule 3) — and never reaches the client.
	h.d.Log.ErrorContext(ctx, op+": lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
	httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
}

// ListCommuneNews serves one page of the commune's published items, newest first.
// GET /api/v1/commune-news?host=
//
// CURSOR-PAGINATED (skills/rest-api-design §5): `limit` (default page.DefaultLimit, capped at
// page.MaxLimit) and `cursor`, the same parameters and the same `next_cursor` / `has_more` every list
// in this system uses. The order is `tao_luc` DESC — ContentItemSorts' only column.
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
func (h *PublicHandler) ListCommuneNews(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	hosts := q["host"]
	if len(hosts) != 1 || !domain.IsValidTenantDomain(hosts[0]) {
		writeInvalidHost(w)
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
	req, err := page.Parse(q, commsstore.ContentItemSorts)
	if err != nil {
		status, code, message := page.HTTPError(err)
		httpx.WriteError(w, status, code, message, "")
		return
	}

	t, found, ok := h.resolveTenant(w, r, hosts[0], "tin của xã")
	if !ok {
		return
	}
	out := page.Result[tinXaRa]{Items: []tinXaRa{}} // `[]`, never `null`
	if !found {
		writeJSON(w, http.StatusOK, out)
		return
	}

	ctx := tenant.Into(r.Context(), t.ID)
	res, err := h.d.ContentItems.ListPublic(ctx, itemType, req)
	if err != nil {
		h.writeInternalError(ctx, w, "tin của xã", err)
		return
	}
	names, err := h.categoryNames(ctx, res.Items)
	if err != nil {
		h.writeInternalError(ctx, w, "tin của xã: tên danh mục", err)
		return
	}

	out.NextCursor, out.HasMore = res.NextCursor, res.HasMore
	for _, n := range res.Items {
		if !n.IsVisibleToCitizens() {
			// THE SECOND WALL. The store already binds `dang-hien`; a row in any other state here means
			// that predicate broke. Dropped, never shown — and loud, because it should be impossible.
			h.d.Log.ErrorContext(ctx, "tin của xã: kho trả một mục CHƯA ĐĂNG trên tuyến công khai — đã bỏ",
				"xa", string(t.ID), "trang_thai", string(n.Status))
			continue
		}
		out.Items = append(out.Items, toCommuneNewsOut(n, names, false))
	}
	writeJSON(w, http.StatusOK, out)
}

// publicTypeFilter reads the list's OPTIONAL `type`: ("", true) when absent or empty, (code, true) for
// one of the six, ("", false) for anything else — the caller answers 400.
//
// A HELPER AND NOT AN INLINE `if`, FOR THE GENERATED CONTRACT: tools/apidoc marks a query parameter
// `required: true` whenever a variable read from it appears in the condition of a 400 branch
// (truyvan.go:158-174, docThanHam). Inline, `type` would be published as REQUIRED — a breaking change
// on paper to a route the Mini App already calls, and every generated client would have to send it.
// Here the handler's condition reads only the bool, and the read inside this function has no 400.
func publicTypeFilter(q url.Values) (domain.ContentType, bool) {
	v := queryParam(q, "type")
	if v == "" {
		return "", true
	}
	if !domain.IsValidContentType(v) {
		return "", false
	}
	return domain.ContentType(v), true
}

// newsIDMaxLen bounds `{id}` before it reaches the store. Item ids are 26-character ULIDs; anything
// longer cannot name one, and answers the same 404 without a query.
const newsIDMaxLen = 64

// GetCommuneNews serves one published item, body as plain text. GET /api/v1/commune-news/{id}?host=
//
// ONE 404 for: no such id, another commune's id, an item not published (`an`, `cho-duyet`), a
// soft-deleted item, and a domain no active commune holds. Byte-identical — telling any two apart would
// say what a commune holds or is preparing (rule 4, forbidden #2).
func (h *PublicHandler) GetCommuneNews(w http.ResponseWriter, r *http.Request) {
	hosts := r.URL.Query()["host"]
	if len(hosts) != 1 || !domain.IsValidTenantDomain(hosts[0]) {
		writeInvalidHost(w)
		return
	}
	id := r.PathValue("id")
	if id == "" || len(id) > newsIDMaxLen {
		writeNewsNotFound(w)
		return
	}

	t, found, ok := h.resolveTenant(w, r, hosts[0], "chi tiết tin của xã")
	if !ok {
		return
	}
	if !found {
		writeNewsNotFound(w)
		return
	}

	ctx := tenant.Into(r.Context(), t.ID)
	n, err := h.d.ContentItems.PublicByID(ctx, id)
	if errors.Is(err, commsstore.ErrContentItemNotFound) {
		writeNewsNotFound(w)
		return
	}
	if err != nil {
		h.writeInternalError(ctx, w, "chi tiết tin của xã", err)
		return
	}
	if !n.IsVisibleToCitizens() {
		h.d.Log.ErrorContext(ctx, "chi tiết tin của xã: kho trả một mục CHƯA ĐĂNG trên tuyến công khai — trả 404",
			"xa", string(t.ID), "trang_thai", string(n.Status))
		writeNewsNotFound(w)
		return
	}
	names, err := h.categoryNames(ctx, []domain.ContentItem{n})
	if err != nil {
		h.writeInternalError(ctx, w, "chi tiết tin của xã: tên danh mục", err)
		return
	}
	writeJSON(w, http.StatusOK, toCommuneNewsOut(n, names, true))
}
