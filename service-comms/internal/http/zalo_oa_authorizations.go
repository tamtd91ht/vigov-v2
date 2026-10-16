package http

// THE ZALO OA AUTHORIZATION CALLBACK — GET /api/v1/zalo-oa-authorizations (ADR 0098 #3, #4, #7). Zalo
// sends the OA administrator's BROWSER here after they press "Cho phép" (or "Từ chối") on Zalo's
// permission page. No session, no commune Host: Zalo allows ONE callback URL per app, so this route lives
// on the platform's callback host (ZALO_OA_CALLBACK_HOST) and cmd/server mounts it on a chain WITHOUT
// TenantMiddleware. The commune is found FROM `state` only, inside the use case (crosstenant.ZaloOA) —
// never from Host, never from a header (StripTenantHeaders runs first), never from the query.
//
// THE ORDER, AND IT IS THE SECURITY OF THE ROUTE:
//
//	1. rate limit (ratelimit.ZaloOACallback, 60/min per network)   BEFORE the query is read: it bounds how
//	                                                               fast states can be probed (rule 13 #7)
//	2. state, code, oa_id, error from the query                    NEVER logged — not the values, not the
//	                                                               query, not the URL (rule 3; ADR 0098 #13)
//	3. CompleteAuthorization, ctx with NO commune                  detached from the browser hanging up, so
//	                                                               an exchange Zalo already honoured is not
//	                                                               rolled back half-way
//	4. the answer:
//	     state unknown / expired / used          404, ONE sentence naming no commune (ADR 0098 #4)
//	     request spent (success or failure)      302 to https://<commune host>/cau-hinh?tab=kenh-zalo&oa=…
//	                                             with the outcome web-admin reads (zalo-oa-form.ts)
//	     spent, but the registry gave no host    a plain page, one sentence — never a guessed host
//
// Every answer carries Cache-Control: no-store and Referrer-Policy: no-referrer: the request URL holds
// the authorization code and the state, and neither may land in a cache or in the commune site's
// Referer.

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// ZaloOAAuthorizationsPath is the callback's path. Exported because cmd/server routes the callback HOST's
// copy of this path to the callback chain; the route below spells it as a LITERAL for tools/apidoc, and
// zalo_oa_authorizations_test.go asserts the two agree with app.ZaloOACallbackPath (the URL comms puts in
// the permission link, and the one declared with Zalo).
const ZaloOAAuthorizationsPath = "/api/v1/zalo-oa-authorizations"

// maxZaloOACallbackQuery caps the query read. A real callback is a state (43 characters), a code and an
// oa_id — a few hundred bytes. Longer is not Zalo.
const maxZaloOACallbackQuery = 4096

// zaloOACallbackBudget bounds the work done for one callback (two Zalo calls, one transaction).
const zaloOACallbackBudget = 20 * time.Second

// The page sentences — fixed text, nothing interpolated (no value from the request ever reaches a page).
const (
	zaloOACallbackNotFound = "Liên kết uỷ quyền không còn hiệu lực (đã dùng, đã hết hạn hoặc không đúng). " +
		"Hãy quay lại trang quản trị của xã và bấm “Kết nối OA” để lấy liên kết mới."
	zaloOACallbackConnectedNoHost = "Đã nối OA. Hãy quay lại trang quản trị của xã."
	zaloOACallbackFailedNoHost    = "Chưa nối được OA. Hãy quay lại trang quản trị của xã để xem kết quả và thử lại."
	zaloOACallbackUnavailable     = "Hệ thống tạm thời chưa xử lý được uỷ quyền OA. Hãy quay lại trang quản trị của xã và thử lại sau ít phút."
	zaloOACallbackFailed          = "Đã xảy ra lỗi khi nối OA. Hãy quay lại trang quản trị của xã và thử lại."
)

// ZaloOACallback is the use case. *app.CommuneZaloOAs satisfies it.
type ZaloOACallback interface {
	CompleteAuthorization(ctx context.Context, in app.CompleteAuthorizationInput) (app.CompleteAuthorizationResult, error)
}

// ZaloOAAuthorizationDeps is everything the callback touches.
type ZaloOAAuthorizationDeps struct {
	Authorizations ZaloOACallback
	// Limiter is ratelimit.ZaloOACallback — REQUIRED (rule 13, invariant 7).
	Limiter *ratelimit.Limiter
	Log     *slog.Logger
}

type zaloOAAuthorizationHandler struct{ d ZaloOAAuthorizationDeps }

// RegisterZaloOAAuthorizations mounts the callback onto ITS OWN mux (cmd/server: the callback chain).
func RegisterZaloOAAuthorizations(mux *http.ServeMux, d ZaloOAAuthorizationDeps) {
	switch {
	case d.Authorizations == nil:
		panic("comms/http: thiếu use case OA Zalo của xã — GET /api/v1/zalo-oa-authorizations sẽ panic khi Zalo chuyển hướng về")
	case d.Limiter == nil:
		// Rule 13 invariant 7: an unauthenticated route without its rate limit is refused at startup.
		panic("comms/http: thiếu bộ giới hạn tần suất callback OA Zalo (ratelimit.ZaloOACallback)")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	h := &zaloOAAuthorizationHandler{d: d}

	// PUBLIC — the caller is the OA administrator's browser, redirected by Zalo; it holds no ViGov session
	// and the host is the platform's, not a commune's. Authenticated instead by the one-time `state` (32
	// CSPRNG bytes, stored hashed, 10 minutes) the commune's own staff put in the permission link.
	//
	// RATE LIMIT — ratelimit.ZaloOACallback, 60 per minute per client network (owner, ADR 0098 #7), counted
	// before the query is read. Over it: 429 + Retry-After. Redis unavailable: 503 (closed).
	//
	// idem.KhongCan: a browser never sends an Idempotency-Key, and the state is single-use — a replay meets
	// a spent request and is the same 404 as an unknown state.
	//
	//	302   the request was spent (success or failure): back to the commune's Cấu hình › Kênh Zalo
	//	200   spent, but the commune's host is unknown: a plain page saying to go back
	//	404   the state is unknown, expired or used — one sentence, no commune named
	//	429   the rate limit
	//	503   the platform's Zalo App or secret encryption is not configured, or the counter store is down
	//
	// @summary  Callback uỷ quyền OA Zalo — Zalo chuyển trình duyệt của quản trị viên OA về đây sau khi bấm Cho phép/Từ chối; xã tìm từ state một lần (không từ Host), đổi code lấy token (PKCE), niêm token bằng khoá của xã, rồi chuyển về Cấu hình › Kênh Zalo của xã kèm kết quả; state lạ/hết hạn/đã dùng trả 404 một câu
	// @screen   ADR 0098 #3 #4 #7 — Cấu hình → tab Kênh Zalo, mục "OA Zalo của xã"
	// @reply    302 -
	// @reply    200 -
	// @reply    404 -
	// @reply    429 httpx.Error rate_limited
	// @reply    503 -
	// @reply    500 -
	mux.Handle("GET /api/v1/zalo-oa-authorizations",
		authz.Public("Zalo chuyển hướng trình duyệt của quản trị viên OA về đây sau khi bấm Cho phép, không có phiên và không có Host của xã: xã tìm từ state một lần đã niêm (ADR 0098 #4); state lạ/hết hạn/đã dùng trả 404 một câu; giới hạn 60 lượt/phút mỗi mạng máy gọi trước mọi việc")(
			idem.KhongCan("trình duyệt không gửi Idempotency-Key; state chỉ dùng một lần nên lần gọi lại gặp yêu cầu đã đóng và nhận cùng câu 404 như state lạ")(
				http.HandlerFunc(h.CompleteZaloOAAuthorization))))
}

// CompleteZaloOAAuthorization — GET /api/v1/zalo-oa-authorizations
func (h *zaloOAAuthorizationHandler) CompleteZaloOAAuthorization(w http.ResponseWriter, r *http.Request) {
	// The URL carries the code and the state: no cache, no Referer onward, whatever the answer.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")

	ip := httpx.ClientIP(r)
	if !ratelimit.Gate(w, r, h.d.Limiter, ratelimit.WebhookIPKey(ip), h.d.Log) {
		return
	}
	// Only TenantMiddleware puts a commune in the context, and the callback chain has none. A commune
	// here means the route was mounted behind a commune Host — refuse rather than let Host and state
	// disagree about which commune this is (rule 1).
	if _, ok := tenant.From(r.Context()); ok {
		h.d.Log.Error("callback OA Zalo chạy trong chuỗi có xã — sai dây, từ chối", "event", "zalo_oa_callback.miswired")
		zaloOAPage(w, http.StatusNotFound, zaloOACallbackNotFound)
		return
	}
	if len(r.URL.RawQuery) > maxZaloOACallbackQuery {
		h.notFound(w, ip)
		return
	}
	q := r.URL.Query()
	in := app.CompleteAuthorizationInput{
		State:    q.Get("state"),
		Code:     q.Get("code"),
		OAID:     q.Get("oa_id"),
		ClientIP: ip,
		// Zalo comes back with `error` when the OA admin refuses. Its value is never read into a log.
		Denied: q.Get("error") != "",
	}
	if in.State == "" {
		h.notFound(w, ip)
		return
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), zaloOACallbackBudget)
	res, err := h.d.Authorizations.CompleteAuthorization(ctx, in)
	cancel()

	switch {
	case errors.Is(err, app.ErrAuthorizationNotFound):
		h.notFound(w, ip)
		return
	case errors.Is(err, app.ErrZaloOANotConfigured):
		h.d.Log.Warn("CẢNH BÁO CẤU HÌNH: callback OA Zalo tới khi nền tảng chưa khai ứng dụng Zalo dùng chung (ZALO_OA_*)",
			"event", "zalo_oa_callback.not_configured")
		zaloOAPage(w, http.StatusServiceUnavailable, zaloOACallbackUnavailable)
		return
	case res.Outcome == "" && res.Tenant == "":
		// A fault before any commune was found (the resolver's query): nothing spent.
		h.d.Log.Error("callback OA Zalo: không tra được yêu cầu uỷ quyền", "event", "zalo_oa_callback.lookup_failed", "err", err)
		zaloOAPage(w, http.StatusServiceUnavailable, zaloOACallbackUnavailable)
		return
	case res.Outcome == "" && errors.Is(err, crypto.ErrNotConfigured):
		h.d.Log.Warn("CẢNH BÁO CẤU HÌNH: callback OA Zalo — SECRET_ENCRYPTION_KEYS trống, không niêm được token",
			"event", "zalo_oa_callback.encryption_not_configured", "xa", string(res.Tenant))
		zaloOAPage(w, http.StatusServiceUnavailable, zaloOACallbackUnavailable)
		return
	case res.Outcome == "":
		// The commune was found but the request could not be spent (the database). Nothing about this
		// reaches the page; the operator reads it here. err never carries the state or the code.
		h.d.Log.Error("callback OA Zalo: lỗi hệ thống", "event", "zalo_oa_callback.failed", "xa", string(res.Tenant), "err", err)
		zaloOAPage(w, http.StatusInternalServerError, zaloOACallbackFailed)
		return
	}

	xa := string(res.Tenant)
	target, ok := zaloOAReturnURL(res.Host, res.Outcome)
	if !ok {
		h.d.Log.Warn("callback OA Zalo: không có tên miền của xã để chuyển về — trả trang tĩnh",
			"event", "zalo_oa_callback.no_host", "xa", xa, "outcome", res.Outcome)
		if res.Outcome == domain.AuthorizationSucceeded {
			zaloOAPage(w, http.StatusOK, zaloOACallbackConnectedNoHost)
			return
		}
		zaloOAPage(w, http.StatusOK, zaloOACallbackFailedNoHost)
		return
	}
	http.Redirect(w, r, target, http.StatusFound)
}

// notFound is the ONE answer for an unknown, expired, used or malformed state (ADR 0098 #4). The security
// log gets the network, never the state.
func (h *zaloOAAuthorizationHandler) notFound(w http.ResponseWriter, ip string) {
	h.d.Log.Warn("CẢNH BÁO BẢO MẬT: callback OA Zalo với state không hợp lệ — 404",
		"event", "zalo_oa_callback.state_refused", "outcome", "refused", "ip", ip)
	zaloOAPage(w, http.StatusNotFound, zaloOACallbackNotFound)
}

// zaloOAOutcomeParam maps the stored outcome onto the `oa=` value web-admin reads
// (web-admin/src/features/cau-hinh/zalo-oa-form.ts CALLBACK_OUTCOME_TEXT). Only success is renamed — the
// screen says "đã nối"; every failure keeps its stored word (ADR 0011).
func zaloOAOutcomeParam(outcome string) string {
	if outcome == domain.AuthorizationSucceeded {
		return "da-noi"
	}
	return outcome
}

// zaloOAReturnURL is https://<host>/cau-hinh?tab=kenh-zalo&oa=<outcome>. ok=false when the host is not a
// bare host name — the registry's value is trusted, but a redirect target is checked anyway: an open
// redirect on a public route is how a phishing link borrows the platform's name.
func zaloOAReturnURL(host, outcome string) (string, bool) {
	if !bareHost(host) {
		return "", false
	}
	u := url.URL{Scheme: "https", Host: host, Path: "/cau-hinh",
		RawQuery: url.Values{"tab": {"kenh-zalo"}, "oa": {zaloOAOutcomeParam(outcome)}}.Encode()}
	return u.String(), true
}

// bareHost: letters, digits, dots and hyphens, an optional :port — nothing that could carry a path, a
// user-info or a second host.
func bareHost(h string) bool {
	if h == "" || len(h) > 253 {
		return false
	}
	for i := 0; i < len(h); i++ {
		c := h[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '.', c == '-', c == ':':
		default:
			return false
		}
	}
	return h[0] != '.' && h[0] != '-' && h[0] != ':'
}

// zaloOAPage writes one fixed sentence as a minimal HTML page. The text is a constant of this file —
// nothing from the request is ever interpolated — and the page may load nothing (CSP default-src 'none').
func zaloOAPage(w http.ResponseWriter, status int, sentence string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`<!doctype html><html lang="vi"><head><meta charset="utf-8"><title>OA Zalo của xã</title></head><body><p>` +
		sentence + `</p></body></html>`))
}
