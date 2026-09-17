// Package httpx holds the outermost edge of every service.
//
// This is where the commune is resolved, and it is the only place it may be resolved. Doing
// it anywhere else means some path exists that skipped it.
package httpx

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/vihat/vigov/core/tenant"
)

// TenantMiddleware resolves the commune from Host and puts it in the context.
//
// A Host matching no commune returns 404 — not 400, not a fallback commune. 404 also reveals
// nothing about which communes exist on the platform.
func TenantMiddleware(dir tenant.Directory) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := strings.ToLower(hostOnly(r.Host))
			t, ok := dir.ByHost(r.Context(), host)
			if !ok || !t.Active {
				http.NotFound(w, r)
				return
			}
			next.ServeHTTP(w, r.WithContext(tenant.Into(r.Context(), t.ID)))
		})
	}
}

// StripTenantHeaders removes any commune header supplied by the client.
//
// The client naming its own commune is the client granting itself access. The reverse proxy
// should already strip these; this is the second line, because "should already" is how
// isolation is lost.
func StripTenantHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for h := range r.Header {
			if strings.HasPrefix(strings.ToLower(h), "x-tenant") {
				r.Header.Del(h)
			}
		}
		next.ServeHTTP(w, r)
	})
}

func hostOnly(h string) string {
	if i := strings.IndexByte(h, ':'); i >= 0 {
		return h[:i]
	}
	return h
}

// Error is the single response shape for every failure.
//
// WHY ONE SHAPE: the previous system classified 159 business errors correctly but had no
// global filter, so unexpected failures fell through to a different shape entirely — and
// there was no single place to strip personal data out of an error message, or to attach a
// trace id the caller could quote when reporting a problem.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"` // safe for a citizen to read: never personal data, never internals
	TraceID string `json:"trace_id"`
}

func WriteError(w http.ResponseWriter, status int, code, msg, traceID string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Error{Code: code, Message: msg, TraceID: traceID})
}

// Recover turns a panic into a 500 with a trace id, and never leaks the panic value.
//
// tenant.MustFrom panics by design when a commune is missing; this is what turns that into a
// loud, traceable failure instead of a crashed process.
func Recover(traceIDFrom func(context.Context) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					WriteError(w, http.StatusInternalServerError, "internal",
						"Đã xảy ra lỗi. Vui lòng thử lại.", traceIDFrom(r.Context()))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
