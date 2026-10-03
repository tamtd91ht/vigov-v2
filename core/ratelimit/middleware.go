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
//	                   does not exist. A policy that FailsOpen proceeds instead (Gate).
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
			if Gate(w, r, l, keyOf(r), log) {
				next.ServeHTTP(w, r)
			}
		})
	}
}

// Gate counts one request for key and answers it when it may not proceed. It returns true when the
// caller should go on serving, false when Gate has ALREADY WRITTEN the response.
//
// FOR A ROUTE WHOSE KEY IS KNOWN ONLY INSIDE THE HANDLER — the public news reads, whose commune is
// resolved from `?host=` by the platform after the request is validated, so no middleware in front of
// them can know it. Middleware is Gate with the key taken from the request up front.
//
//	within the limit                  true.
//	over the limit                    429 + Retry-After; false. A security event (below).
//	store unavailable, closed policy  503 `rate_limit_unavailable`; false.
//	store unavailable, FailsOpen      true — the request is served — and ONE security warning per
//	                                  window (`rate_limit.store_unavailable`, outcome `allowed`), so an
//	                                  outage is visible without one log line per resident request.
//
// attrs are extra fields of the security event — the public routes pass the commune — and must be
// identifiers only: never a body, a query value, a title (rule 3).
func Gate(w http.ResponseWriter, r *http.Request, l *Limiter, key Key, log *slog.Logger, attrs ...any) bool {
	return GateOutcome(w, r, l, key, log, attrs...) != Refused
}

// Outcome is what GateOutcome decided. Gate folds it into a bool; GateOutcome exists for the one caller
// that must tell "served WITHIN the limit" from "served because the limit could not be consulted".
type Outcome int

const (
	// Refused: the response (429 or 503) is ALREADY WRITTEN; the caller stops.
	Refused Outcome = iota
	// Enforced: the store counted this request and it is within the limit.
	Enforced
	// NotEnforced: the store could not be asked and the policy FailsOpen, so the request is served
	// UNBOUNDED. A caller whose side effect is only safe under the bound must skip it on this outcome —
	// the public news detail does not count a view (ADR 0047, row 02/10/2026): with the limiter down,
	// nothing stops one client inflating the figure.
	NotEnforced
)

// GateOutcome is Gate with the three outcomes kept apart. The responses it writes, the logs it emits
// and the fail-open behaviour are exactly Gate's — Gate is this function folded to a bool.
func GateOutcome(w http.ResponseWriter, r *http.Request, l *Limiter, key Key, log *slog.Logger, attrs ...any) Outcome {
	if log == nil {
		log = slog.Default()
	}
	allowed, retryAfter, err := l.Allow(r.Context(), key)
	if err != nil {
		if l.p.failOpen {
			if l.firstOutageInWindow() {
				// NO KEY, NO ADDRESS: the event says the guard is down, not who came through it. The
				// error names the store failure (go-redis), never a counter key.
				log.WarnContext(r.Context(), "CẢNH BÁO BẢO MẬT: không đếm được giới hạn tần suất — vẫn phục vụ (mở theo chính sách, chủ dự án 02/10/2026)",
					"event", "rate_limit.store_unavailable", "outcome", "allowed", "chinh_sach", l.p.name, "err", err)
			}
			return NotEnforced
		}
		// The key is NOT logged: it holds a client address, and the policy name is what an
		// operator needs to find the failing dependency.
		log.WarnContext(r.Context(), "CẢNH BÁO HẠ TẦNG: không đếm được giới hạn tần suất — từ chối (đóng kín)",
			"chinh_sach", l.p.name, "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "rate_limit_unavailable",
			"Hệ thống tạm thời không xử lý được yêu cầu. Vui lòng thử lại sau.", "")
		return Refused
	}
	if !allowed {
		// A SECURITY EVENT, not a debug line (skills/security-logging, "rate limit hit"): a burst of
		// these from one network is a guessing or scraping attempt. The fields are the policy and the
		// address the edge observed — NEVER the counter key (a derived form of the address that adds
		// nothing) and never anything from the body: a sign-in body holds an email and a password
		// (rule 3, rule 8).
		fields := append([]any{"event", l.p.event, "outcome", "refused", "chinh_sach", l.p.name,
			"ip", httpx.ClientIP(r)}, attrs...)
		log.WarnContext(r.Context(), "CẢNH BÁO BẢO MẬT: vượt giới hạn tần suất — từ chối", fields...)
		w.Header().Set("Retry-After", retryAfterSeconds(retryAfter))
		code, msg := "rate_limited", "Bạn đã thử quá nhiều lần. Vui lòng thử lại sau."
		if l.p.refusedCode != "" {
			code, msg = l.p.refusedCode, l.p.refusedMessage
		}
		httpx.WriteError(w, http.StatusTooManyRequests, code, msg, "")
		return Refused
	}
	return Enforced
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
