package http

import (
	"log/slog"
	"mime"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
)

// operatorRequestGuard refuses every state-changing request that the operator console's own pages
// did not send. It wraps the WHOLE operator mux (OperatorHandler), so a route added tomorrow is
// covered without anybody remembering to opt in — the shape where an opt-in list quietly misses one.
//
// WHY SameSite=Strict IS NOT ENOUGH HERE. SameSite is about SITES, and a site is the registrable
// domain: OPERATOR_HOST (admin.vigov.vn) and every commune host (<xa>.vigov.vn) are the SAME site.
// A script on any commune page — or an XSS in one — is "same-site" to the operator area, so the
// browser attaches the operator cookie to its requests. And a `text/plain` (or form) POST is a CORS
// "simple request": it is SENT without a preflight, so the absence of CORS headers does not stop it
// from reaching the handler — it only hides the answer. decodeBody parses JSON whatever the
// Content-Type says, so before this guard such a POST was a working CSRF on every operator route,
// including the sign-in ones (a login-CSRF signs the victim into the attacker's session).
//
// THE TWO CHECKS, both required, on every method but GET and HEAD:
//
//  1. ORIGIN. `Origin` must equal "https://"+OPERATOR_HOST exactly. Browsers send it on every
//     cross-origin request and on every same-origin non-GET fetch, so a request from a commune page
//     carries that commune's origin and is refused. When Origin is ABSENT, `Sec-Fetch-Site` must be
//     `same-origin` (a browser that sends neither is not one this console supports). Absent both ⇒
//     refused: fail closed, never "no header, so probably a script, so allow".
//     platform-admin's gateway forwards both headers unchanged (platform-admin/src/lib/server/
//     gateway.ts: outgoingHeaders drops only connection-scoped headers, host, forwarded,
//     x-forwarded-host, expect and x-tenant*), so the browser's own values reach this check.
//
//  2. CONTENT TYPE. The media type must be exactly `application/json` (parameters such as
//     charset are allowed; decodeBody refuses non-UTF-8 itself). That makes every accepted write a
//     non-simple request, which a cross-origin page cannot send without a preflight this server
//     never approves. ONE NARROW EXCEPTION: a request with NO BODY and NO Content-Type — the
//     console's `DELETE /operator-sessions/current` (platform-admin/src/lib/api.ts sends no
//     Content-Type when there is no body). Nothing in it is a media type to check, a cross-origin
//     page cannot send a bodyless DELETE without a preflight either (DELETE is not a CORS-safelisted
//     method), and check 1 still applies to it. A bodyless POST/PUT reaches its handler, which
//     answers 400 for the missing JSON.
//
// STATUS CODES: 403 `origin_refused` for check 1 — the request is understood and refused for who
// sent it; 415 `unsupported_media_type` for check 2 — RFC 9110 §15.5.16, the body's format is one
// this route does not accept. Origin is checked FIRST, so a cross-origin probe learns nothing about
// which content types would be accepted.
//
// Every refusal is a SECURITY EVENT (skills/security-logging): event `operator.request_refused`,
// the reason, the method, the path (operator paths carry ULIDs and fixed nouns, no personal data),
// and the client address. Never a header value beyond the reason, never the body.
func operatorRequestGuard(operatorHost string, log *slog.Logger) func(http.Handler) http.Handler {
	if operatorHost == "" {
		// A wiring fault: with no host there is no origin to compare against, and "allow" is the
		// only thing a blank comparison could mean. Refuse at mount time.
		panic("operatorRequestGuard: empty OPERATOR_HOST")
	}
	wantOrigin := "https://" + operatorHost
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead {
				next.ServeHTTP(w, r)
				return
			}
			if !sameOrigin(r, wantOrigin) {
				refuseRequest(w, r, log, "origin", http.StatusForbidden, "origin_refused",
					"Yêu cầu không xuất phát từ trang quản trị vận hành.")
				return
			}
			if !jsonOrEmpty(r) {
				refuseRequest(w, r, log, "content_type", http.StatusUnsupportedMediaType, "unsupported_media_type",
					"Dữ liệu gửi lên phải ở dạng JSON.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// sameOrigin is check 1. More than one Origin header is refused: a browser sends one, and two leave
// "which one counts" to whichever reader comes next.
func sameOrigin(r *http.Request, want string) bool {
	if origins := r.Header.Values("Origin"); len(origins) > 0 {
		return len(origins) == 1 && origins[0] == want
	}
	sites := r.Header.Values("Sec-Fetch-Site")
	return len(sites) == 1 && sites[0] == "same-origin"
}

// jsonOrEmpty is check 2, with its one exception (see operatorRequestGuard).
func jsonOrEmpty(r *http.Request) bool {
	types := r.Header.Values("Content-Type")
	if len(types) == 0 {
		// ContentLength 0 is "known to be empty"; -1 (unknown, chunked) is a body.
		return r.ContentLength == 0
	}
	if len(types) != 1 {
		return false
	}
	mt, _, err := mime.ParseMediaType(types[0])
	return err == nil && mt == "application/json"
}

func refuseRequest(w http.ResponseWriter, r *http.Request, log *slog.Logger, reason string, status int, code, msg string) {
	log.WarnContext(r.Context(), "CẢNH BÁO BẢO MẬT: yêu cầu ghi tới khu vận hành không từ trang vận hành — từ chối",
		"event", "operator.request_refused", "outcome", "refused", "reason", reason,
		"method", r.Method, "path", r.URL.Path, "ip", httpx.ClientIP(r))
	httpx.WriteError(w, status, code, msg, "")
}
