package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// POST /api/v1/staff/searches — the free-text search of the staff register (user decision
// 2026-09-24).
//
// WHY A POST FOR A READ. The search box takes a name, a position or a telephone number, and two of
// those three are personal data. A GET would carry the text in the URL — into the access log of
// every proxy on the way, the browser history of a shared counter machine, and any monitoring that
// records URLs (rule 3, forbidden #4). A request BODY is logged by none of those. `searches` is the
// nominalised noun: each POST describes one search, and it creates nothing.
//
// THE SAME RESULT SHAPE AS GET /api/v1/staff — page.Result[canBoTomTat] through raNgoai, via the
// shared traTrangCanBo — so the screen renders one row type whichever box the administrator used,
// and every rule about what leaves this service (#11's unmasked numbers inside the commune, no
// credential field) holds on both by construction.

// timCanBoVao is the whole body.
//
//	q          REQUIRED. Trimmed, internal whitespace collapsed, 1..200 CHARACTERS
//	           (domain.TuKhoaTimCanBoToiDa — runes, not bytes). Matched against name, position, both
//	           telephone numbers, email and the live department name (store.menhDeLocCanBo).
//	unit       optional department id → `bo_phan_id`. Same validation as GET's `?unit=`.
//	published  optional → `hien_tren_mini_app`. null/absent = both.
//	limit      optional, as GET's `?limit=`: absent = page.DefaultLimit, above page.MaxLimit is
//	           capped, below 1 is 400.
//	cursor     optional, the `next_cursor` of the previous page of THIS search, under THIS sort/order
//	           (a cursor from another sort or direction is 400 invalid_cursor).
//	sort       optional, SINCE 08/10/2026 (owner decision) — the SAME allowlist as GET's `?sort=`
//	           (store.SapXepCanBo): code · created_at · full_name · position · department · phone ·
//	           last_login_at · status. Absent/"" = `code`. Anything else is 400 invalid_sort.
//	order      optional — `asc` | `desc`. Absent/"" = `asc`. Anything else is 400 invalid_sort.
//
// `sort` AND `order` CARRY `omitempty` so the generated contract marks them optional: a client written
// before them sends neither and keeps today's order. They are in the BODY, not the URL, only because
// the rest of this request is; neither is personal data.
type timCanBoVao struct {
	Q         string `json:"q"`
	Unit      string `json:"unit"`
	Published *bool  `json:"published"`
	Limit     *int   `json:"limit"`
	Cursor    string `json:"cursor"`
	Sort      string `json:"sort,omitempty"`
	Order     string `json:"order,omitempty"`
}

// TimCanBo serves one page of a search. POST /api/v1/staff/searches
//
// NO AUDIT ENTRY, for exactly the reason and with exactly the caveat DanhSachCanBo states: it
// returns the same rows the list returns to the same holders of `admin.user`, and whether reading
// the unmasked register must be audited is rule 6 stop condition #1 — KEPT AS-IS, not decided here.
//
// `q` IS NEVER LOGGED, NOT ON SUCCESS AND NOT ON FAILURE. It is passed to the store inside
// domain.LocCanBo and to no logger; the one log line on this path (traTrangCanBo) carries the
// commune and the store error only. Rule 3, invariant 1.
func (h *Handler) TimCanBo(w http.ResponseWriter, r *http.Request) {
	var than timCanBoVao
	if !docThanCanBo(w, r, &than) {
		return
	}

	loc, ok := staffSearchFilter(w, than.Q, than.Unit, than.Published)
	if !ok {
		return
	}

	// page.New and not page.Parse: the paging fields arrive in the body, not the query string. The
	// same allowlist and the same limit/cursor/sort/order rules apply, so a cursor from this search
	// decodes exactly as one from the list would. Validated BEFORE the store is touched.
	limit := ""
	if than.Limit != nil {
		limit = strconv.Itoa(*than.Limit)
	}
	yc, err := page.New(idstore.SapXepCanBo, than.Sort, than.Order, limit, than.Cursor)
	if err != nil {
		status, ma, thongBao := page.HTTPError(err)
		httpx.WriteError(w, status, ma, thongBao, "")
		return
	}

	h.traTrangCanBo(w, r, "tìm cán bộ", loc, yc)
}

// staffSearchFilter validates the three filter fields of a register search and builds the store's
// filter, answering 400 itself. SHARED by POST /api/v1/staff/searches and POST
// /api/v1/staff-count-queries, so a total and the list it heads are refused, normalised and filtered
// by one piece of code — a second copy is where "the count says 12, the list shows 11" comes from.
func staffSearchFilter(w http.ResponseWriter, q, unit string, published *bool) (domain.LocCanBo, bool) {
	text, err := domain.ChuanHoaTuKhoaTimCanBo(q)
	if err != nil {
		// Each sentence names the rule, never the input (the input is likely a name or a number).
		message := "Hãy nhập từ khoá tìm kiếm."
		if errors.Is(err, domain.ErrTuKhoaQuaDai) {
			message = "Từ khoá tìm kiếm quá dài (tối đa " +
				strconv.Itoa(domain.TuKhoaTimCanBoToiDa) + " ký tự)."
		}
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", message, "")
		return domain.LocCanBo{}, false
	}

	loc := domain.LocCanBo{
		BoPhanID: strings.TrimSpace(unit),
		CongKhai: published,
		TuKhoa:   text,
	}
	if !kiemBoPhanLoc(w, loc.BoPhanID) {
		return domain.LocCanBo{}, false
	}
	return loc, true
}
