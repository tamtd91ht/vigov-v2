package ratelimit

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/vihat/vigov/core/httpx"
)

// Middleware limits every request through it by keyOf(r).
//
//	within the limit   next runs.
//	over the limit     429, Retry-After in whole seconds (rounded UP, at least 1), next never runs.
//	store unavailable  503, next never runs — FAIL CLOSED (package doc). Not 429: the client did
//	                   nothing wrong, and a 429 would tell an operator to wait out a window that
//	                   does not exist.
//
// MOUNT IT BEFORE THE HANDLER THAT CALLS IDENTITY, so a throttled attempt never reaches the account
// lockout counter (proto/vigov/identity/v1/operator.proto, RESOURCE_EXHAUSTED paragraph).
//
// keyOf must derive the key from what the EDGE observed — for the operator area
// OperatorIPKey(httpx.ClientIP(r)) — never from a header, body or query value the client controls.
func Middleware(l *Limiter, keyOf func(*http.Request) Key, log *slog.Logger) func(http.Handler) http.Handler {
	if l == nil || keyOf == nil {
		// A wiring fault. Panicking at mount time names it at startup; a nil limiter discovered on
		// the first sign-in would be a route that is either unbounded or always down.
		panic("ratelimit.Middleware: nil limiter or key function")
	}
	if log == nil {
		log = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			allowed, retryAfter, err := l.Allow(r.Context(), keyOf(r))
			if err != nil {
				// The key is NOT logged: it holds a client address, and the policy name is what an
				// operator needs to find the failing dependency.
				log.WarnContext(r.Context(), "CẢNH BÁO HẠ TẦNG: không đếm được giới hạn tần suất — từ chối (đóng kín)",
					"chinh_sach", l.p.name, "err", err)
				httpx.WriteError(w, http.StatusServiceUnavailable, "rate_limit_unavailable",
					"Hệ thống tạm thời không xử lý được yêu cầu. Vui lòng thử lại sau.", "")
				return
			}
			if !allowed {
				w.Header().Set("Retry-After", retryAfterSeconds(retryAfter))
				httpx.WriteError(w, http.StatusTooManyRequests, "rate_limited",
					"Bạn đã thử quá nhiều lần. Vui lòng thử lại sau.", "")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// retryAfterSeconds renders a delay as RFC 9110 delay-seconds. Rounded UP: rounding down tells the
// client to come back while the window is still closed, and 0 invites an immediate retry loop.
func retryAfterSeconds(d time.Duration) string {
	s := int64((d + time.Second - 1) / time.Second)
	if s < 1 {
		s = 1
	}
	return strconv.FormatInt(s, 10)
}
