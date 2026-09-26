package httpx

import "net/http"

// NguonCORS answers whether a browser Origin may read citizen-edge responses.
//
// AN INTERFACE AND NOT config.NguonCORS, so httpx keeps importing nothing but tenant: the parser
// and the matching rule are ONE thing and live together in core/config (cors.go there); this
// package only asks the question.
type NguonCORS interface {
	// ChoPhep reports whether the raw Origin header value matches a configured entry.
	ChoPhep(origin string) bool
	// Rong reports whether nothing is configured — CORS off.
	Rong() bool
}

// The fixed CORS answer of the citizen edge. Constants, not configuration: they describe what the
// citizen routes ACCEPT, which is a property of the code, not of a deployment.
const (
	// GET and POST are the only citizen methods (service-petitions/internal/http/routes_cong_dan.go).
	corsPhuongThuc = "GET, POST, OPTIONS"
	// Exactly what citizen-app sends (citizen-app/src/cong-dan/api/goi-vigov.ts): the bearer token,
	// the JSON body type and the duplicate-request key of the intake.
	corsTieuDe = "Authorization, Content-Type, Idempotency-Key"
	// Ten minutes of cached preflight: short enough that removing an origin takes effect the same
	// morning, long enough that the Mini App does not pay a preflight per tap.
	corsMaxAge = "600"
)

// CORSCongDan lets the Zalo Mini App webview call the CITIZEN edge from its own origin.
//
// MOUNT IT ON THE CITIZEN CHAIN ONLY, OUTERMOST. Staff routes are same-origin through web-admin
// (ADR 0043) and carry a host-only cookie; a CORS grant there is a way for another page to drive a
// staff session, so this middleware must never wrap the staff chain.
//
// WHAT IT DOES, AND WHAT IT DOES NOT:
//
//	nothing configured      returns next unchanged — no header of any kind (fail closed: the
//	                        browser blocks the Mini App)
//	preflight               OPTIONS + Origin + Access-Control-Request-Method: answered 204 HERE,
//	                        before the session and commune layers — a preflight never carries
//	                        Authorization, so letting it through would only earn a 401 the browser
//	                        reads as "CORS failed". Unknown origin: 204 with NO CORS header, so
//	                        the browser refuses, and nothing tells a probe which origins exist
//	any other request       Access-Control-Allow-Origin echoing the matched origin, then the normal
//	                        chain UNCHANGED. CORS decides who may READ an answer, never who may
//	                        ASK: authentication still decides, and a request with no Origin
//	                        (server-to-server) is served exactly as before
//
// NEVER `*`, AND NEVER Access-Control-Allow-Credentials. The citizen edge authenticates with a
// bearer token and reads no cookie (CitizenEdge), so credentials would add nothing but a CSRF
// surface. The echoed origin is the request's own value, byte for byte: a browser compares it
// exactly. `Vary: Origin` is on every response of the chain, matched or not, so a shared cache never
// hands one origin's answer to another.
//
// Nothing is exposed via Access-Control-Expose-Headers: citizen-app reads no response header.
func CORSCongDan(nguon NguonCORS) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if nguon == nil || nguon.Rong() {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			h := w.Header()
			h.Add("Vary", "Origin")
			duoc := origin != "" && nguon.ChoPhep(origin)

			if r.Method == http.MethodOptions && origin != "" &&
				r.Header.Get("Access-Control-Request-Method") != "" {
				h.Add("Vary", "Access-Control-Request-Method")
				h.Add("Vary", "Access-Control-Request-Headers")
				if duoc {
					h.Set("Access-Control-Allow-Origin", origin)
					h.Set("Access-Control-Allow-Methods", corsPhuongThuc)
					h.Set("Access-Control-Allow-Headers", corsTieuDe)
					h.Set("Access-Control-Max-Age", corsMaxAge)
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			if duoc {
				h.Set("Access-Control-Allow-Origin", origin)
			}
			next.ServeHTTP(w, r)
		})
	}
}
