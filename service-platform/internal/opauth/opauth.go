// Package opauth authenticates and authorises the ViHAT OPERATOR realm on service-platform's operator
// edge (ADR 0048 §"Chốt của chủ dự án — 28/09/2026" #2–#4, §"Chốt … 01/10/2026" #2).
//
// IT IS NOT core/authz, AND MUST NOT BECOME IT. authz answers "what may this STAFF member do IN THIS
// COMMUNE": its guards compare the principal's tenant with the commune resolved from Host, and its
// keys are rows of the per-commune `quyen` table (rule 5 invariants 3, 3c). An operator belongs to no
// commune and holds `ops.*` keys of its own realm, never rows of `quyen` (ADR 0048 §28/09 #3). The two
// realms therefore have two principal TYPES in two context keys: an operator principal can never
// satisfy an authz guard, and a staff principal can never satisfy one here — ADR 0048 stop condition
// #6 held by the type system, not by a string comparison somebody can forget.
//
// THE DECLARATIONS — every operator route states exactly one, in the same statement as the route,
// and tools/apidoc reads them (rule 5 invariant 1, the operator-realm equivalent):
//
//	opauth.RequireKey(d.Auth, opauth.KeyTenantManage, …)  the normal case — every key listed is required
//	opauth.SignedIn(d.Auth, "<why any operator needs this>")  any live operator session
//	opauth.Public("<why this has no session>")              sign-in steps only, always rate limited
//
// HYBRID, NOT CACHED (owner, 01/10 #2): the `op1.` signature is checked HERE first, so garbage, a
// forgery or a staff `v1.` token is refused without a round trip; a well-signed token is then
// resolved by identity on EVERY request — the only place revocation and the 5-minute idle limit are
// decided. Identity unreachable is 503, never 401: answering "signed out" for an outage would sign
// every operator out through the service that is down.
package opauth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/operatorclient"
	"github.com/vihat/vigov/core/operatortoken"
	"github.com/vihat/vigov/core/secret"
)

// CookieName is the operator session cookie.
//
// THE `__Host-` PREFIX IS THE POINT, not decoration: a browser accepts a `__Host-` cookie only when
// it is Secure, has Path=/ and carries NO Domain attribute — so the cookie is host-only on
// OPERATOR_HOST by the browser's own rule, and no later edit adding `Domain: ".vigov.vn"` can widen
// it without the browser dropping the cookie outright (rule 1 forbidden #3, rule 13 invariant 4).
// A different name from the staff cookie (core/staffauth.CookieName): neither edge ever reads the
// other realm's cookie.
const CookieName = "__Host-vigov_operator_session"

// Key is one `ops.<group>.<action>` permission key of the operator realm.
//
// NOT authz.Perm and NOT a row of `quyen`: see the package comment. The set below is CLOSED — the
// six keys the owner decided (ADR 0048 §28/09 #3). A key outside it is a programming error, refused
// at wiring time by RequireKey, because a route guarded by a key nobody can be granted answers 403
// to every operator forever with every test green (the same failure rule 5 invariant 3c names).
type Key string

const (
	KeyTenantManage       Key = "ops.tenant.manage"
	KeyDomainManage       Key = "ops.domain.manage"
	KeyProfileManage      Key = "ops.profile.manage"
	KeyMiniAppManage      Key = "ops.mini_app.manage"
	KeyUploadPolicyManage Key = "ops.upload_policy.manage"
	KeyQRIssue            Key = "ops.qr.issue"
)

var decidedKeys = map[Key]bool{
	KeyTenantManage: true, KeyDomainManage: true, KeyProfileManage: true,
	KeyMiniAppManage: true, KeyUploadPolicyManage: true, KeyQRIssue: true,
}

// Principal is the operator behind THIS request. Never cached, never written anywhere.
type Principal struct {
	// OperatorID authorises nothing here and is never written into a trail (rule 6 invariant 8).
	OperatorID string
	// OperatorCode, `VH-00001`, is "who" in every audit entry. Never empty on a principal in context.
	OperatorCode string
	Keys         []string
}

// Has reports whether the principal holds k.
func (p Principal) Has(k Key) bool {
	for _, x := range p.Keys {
		if x == string(k) {
			return true
		}
	}
	return false
}

type ctxKey struct{}

// From returns the operator principal a guard put in the context.
func From(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}

// Into is exported for tests of handlers that run behind a guard.
func Into(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// TokenVerifier checks an `op1.` signature locally. *operatortoken.Signer satisfies it.
type TokenVerifier interface {
	Verify(tok string, now time.Time) (operatortoken.Claims, error)
}

// SessionResolver asks identity whether the session is live. *operatorclient.Client satisfies it.
type SessionResolver interface {
	Resolve(ctx context.Context, token secret.Secret) (operatorclient.Principal, bool, error)
}

// Auth is what the guards need. A nil verifier means OPERATOR_SESSION_SIGNING_KEYS is unset: every
// guarded route answers 503 (dev only — staging/prod refuse to start without the keys).
type Auth struct {
	verifier TokenVerifier
	resolver SessionResolver
	now      func() time.Time
	log      *slog.Logger
}

// NewAuth builds the guards' dependencies. resolver must not be nil: a guard that cannot ask identity
// is a guard that has to choose between 401 for everybody and trusting the signature — and the second
// is the decision the owner rejected.
func NewAuth(v TokenVerifier, r SessionResolver, log *slog.Logger) *Auth {
	if r == nil {
		panic("opauth.NewAuth: nil session resolver")
	}
	if log == nil {
		log = slog.Default()
	}
	return &Auth{verifier: v, resolver: r, now: time.Now, log: log}
}

// WithClock is for tests that sign tokens at a fixed time.
func (a *Auth) WithClock(now func() time.Time) *Auth { a.now = now; return a }

const (
	msgUnauthorized = "Phiên làm việc không hợp lệ hoặc đã kết thúc. Vui lòng đăng nhập lại."
	msgForbidden    = "Tài khoản vận hành của bạn không có quyền thực hiện thao tác này."
	msgUnavailable  = "Hệ thống tạm thời không xác thực được phiên vận hành. Vui lòng thử lại sau."
)

// TokenFrom reads the operator cookie. ok is false when it is absent or empty.
func TokenFrom(r *http.Request) (secret.Secret, bool) {
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return nil, false
	}
	return secret.Secret(c.Value), true
}

// authenticate runs the hybrid check and writes the refusal itself. ok=false means the response is
// already written.
func (a *Auth) authenticate(w http.ResponseWriter, r *http.Request) (Principal, bool) {
	if a == nil || a.verifier == nil {
		a.logger().WarnContext(r.Context(), "CẢNH BÁO CẤU HÌNH: khu vận hành bật mà không có khoá kiểm phiên — từ chối")
		httpx.WriteError(w, http.StatusServiceUnavailable, "operator_auth_unavailable", msgUnavailable, "")
		return Principal{}, false
	}
	tok, ok := TokenFrom(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", msgUnauthorized, "")
		return Principal{}, false
	}
	// LOCAL SIGNATURE FIRST, and its refusal makes NO call to identity: a flood of forged cookies
	// must cost this process a MAC, not identity a database read each.
	if _, err := a.verifier.Verify(string(tok.Lo()), a.now()); err != nil {
		if !errors.Is(err, operatortoken.ErrExpired) {
			// A forged or foreign-realm token on the operator host is not an ordinary event. Logged
			// with the client address and nothing of the token (rule 8).
			a.log.WarnContext(r.Context(), "CẢNH BÁO BẢO MẬT: token vận hành sai chữ ký hoặc sai miền",
				"event", "operator.token_refused", "ip", httpx.ClientIP(r))
		}
		ClearCookie(w)
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", msgUnauthorized, "")
		return Principal{}, false
	}
	p, live, err := a.resolver.Resolve(r.Context(), tok)
	if err != nil {
		// operatorclient already logged the gRPC code. 503, never 401 — see the package comment.
		httpx.WriteError(w, http.StatusServiceUnavailable, "operator_auth_unavailable", msgUnavailable, "")
		return Principal{}, false
	}
	if !live {
		ClearCookie(w)
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", msgUnauthorized, "")
		return Principal{}, false
	}
	return Principal{OperatorID: p.OperatorID, OperatorCode: p.OperatorCode, Keys: p.PermissionKeys}, true
}

func (a *Auth) logger() *slog.Logger {
	if a == nil || a.log == nil {
		return slog.Default()
	}
	return a.log
}

// RequireKey guards a route with one or more `ops.*` keys; the operator must hold EVERY key listed.
//
// Panics at wiring time on an empty list, an unknown key, or a nil Auth: each is a route that would
// silently deny everybody or — worse — be mounted with no guard at all.
func RequireKey(a *Auth, keys ...Key) func(http.Handler) http.Handler {
	if a == nil {
		panic("opauth.RequireKey: nil Auth")
	}
	if len(keys) == 0 {
		panic("opauth.RequireKey: no key — use SignedIn(reason) for a route any operator may call")
	}
	for _, k := range keys {
		if !decidedKeys[k] {
			panic("opauth.RequireKey: " + string(k) + " is not a decided operator key (ADR 0048 §28/09 #3)")
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := a.authenticate(w, r)
			if !ok {
				return
			}
			for _, k := range keys {
				if !p.Has(k) {
					httpx.WriteError(w, http.StatusForbidden, "forbidden", msgForbidden, "")
					return
				}
			}
			next.ServeHTTP(w, r.WithContext(Into(r.Context(), p)))
		})
	}
}

// SignedIn opens a route to every live operator session. The reason is mandatory and kept in the
// binary so it can be audited (rule 5 forbidden #4, the operator-realm equivalent).
func SignedIn(a *Auth, reason string) func(http.Handler) http.Handler {
	if a == nil {
		panic("opauth.SignedIn: nil Auth")
	}
	if reason == "" {
		panic("opauth.SignedIn requires a reason")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := a.authenticate(w, r)
			if !ok {
				return
			}
			next.ServeHTTP(w, r.WithContext(Into(r.Context(), p)))
		})
	}
}

// Public opens an operator-host route with no session — the sign-in steps, which are what CREATE a
// session. The reason is mandatory. Every Public operator route is also behind the per-IP rate
// limit (ratelimit.OperatorSignIn, rule 13 invariant 7) — operator_routes.go mounts both together.
func Public(reason string) func(http.Handler) http.Handler {
	if reason == "" {
		panic("opauth.Public requires a specific reason")
	}
	return func(next http.Handler) http.Handler { return next }
}

// SetCookie writes the operator session cookie for the session identity just opened.
//
// Max-Age is the ABSOLUTE expiry identity returned (8 hours), never longer. The idle limit is
// identity's to enforce; a cookie outliving its session is harmless because every request is
// resolved, while a cookie dying early would sign a person out for no reason.
func SetCookie(w http.ResponseWriter, tok secret.Secret, expiresAt, now time.Time) {
	maxAge := int(expiresAt.Sub(now).Seconds())
	if maxAge < 1 {
		// Already expired by our clock: setting it would hand the browser a cookie that is refused
		// on the next request. Clearing is the honest answer.
		ClearCookie(w)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    string(tok.Lo()),
		Path:     "/",
		Expires:  expiresAt.UTC(),
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   true,
		// Strict, unlike the staff cookie's Lax: the console is reached by typing its address,
		// never by following a link from another site, and every operator write is cross-commune.
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearCookie removes the operator cookie with the attributes it was set with — a Set-Cookie that
// differs in name or path deletes nothing.
func ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}
