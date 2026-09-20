// Package staffauth rebuilds the staff principal for a service that does NOT own the session
// registry.
//
// # What it is the twin of
//
// service-identity/internal/http.XacThuc does this job for identity itself: it reads the session
// cookie, verifies the token, compares the commune, reads the session registry and the account,
// and puts an authz.Principal in the context. Same job here, DIFFERENT SOURCE OF TRUTH — the
// answer comes from identity over gRPC (vigov.identity.v1.ResolveStaffPrincipal) instead of from
// a local database, because the session registry lives inside identity's `internal/` and rule 2,
// forbidden #1 forbids the other services from importing it.
//
// ONE IMPLEMENTATION FOR FOUR SERVICES, and that is the point of it being in core. Four copies of
// this middleware is four places to forget one of the four behaviours below, and the service that
// forgets is the one that serves one commune's staff inside another commune's data.
//
// # The four behaviours, and none of them is negotiable
//
//	no cookie            DO NOT CALL. Every anonymous request and every authz.Public route would
//	                     otherwise pay a LAN round trip forever, and nothing would report it.
//	OK, no principal     no principal in the context, and the request CONTINUES. The route's own
//	                     guard answers — Public serves, AnyAuthenticated and RequirePermission
//	                     answer 401. Turning it into 401 here would stop a Public route serving.
//	the call failed      503, NEVER "no principal". A caller that reads an outage as anonymity
//	                     tells every member of staff to sign in again — through the service that
//	                     is down.
//	a principal          authz.Into, plus the permission set of THIS ONE REQUEST (see boQuyen).
//
// # The commune comparison is deliberately NOT here
//
// Rule 1, invariant 8 — the commune in the credential must equal the commune resolved from Host —
// is enforced INSIDE the RPC, once for all four services. The "x-tenant-id" metadata the client
// interceptor attaches already IS the Host-derived commune, so the server has both values and
// makes the comparison itself, and raises the security alert skills/session-and-token §2 requires.
//
// Re-implementing it here would need this package to obtain the commune from the credential, which
// it cannot read and must not learn to read; sending a commune in the request body would be the
// caller declaring its own commune, which rule 1, forbidden #2 forbids outright. Four services each
// remembering to make the comparison is four places for one of them to forget.
package staffauth

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
)

// CookieName is the name of the staff session cookie, as the browser holds it.
//
// IT IS THE ONE DEFINITION, NOT A COPY. `service-identity/internal/http/cookie.go:22` — the file
// that SETS the cookie — declares `const CookiePhien = staffauth.CookieName`, so the compiler
// holds the two equal and there is nothing here that can drift.
//
// It was a copy for part of one day, and the cost of that state is worth keeping written down
// because it is why the alias exists: identity may not be imported (`internal/`, rule 2 forbidden
// #1), so two literals could only ever have been kept equal by hand. Renaming one alone would
// have meant every service EXCEPT identity stopped seeing a cookie the browser was still sending
// — and the symptom reads as "everybody is signed out everywhere, except the sign-in screen",
// which sends an operator to inspect the session registry, the one thing that is fine.
const CookieName = "vigov_session"

// StaffPrincipal is what one resolution answered: who is acting, and what they may do, in the
// commune this request already resolved from Host.
//
// It mirrors vigov.identity.v1.StaffPrincipal field for field, and the list of fields that are
// NOT here — name, position, business code, sid, expiry, tenant_id, kind, roles — is argued in
// that message's comment. Read it before widening this struct.
type StaffPrincipal struct {
	// StaffID is the INTERNAL id of the staff record, never the business code `ma`. Identity's own
	// permission query matches on `nd.id`, so the business code matches no row: every permission
	// check would answer false and every guarded route 403, with nothing in the response, the logs
	// or a test to point at the cause.
	StaffID string

	// PermissionKeys is every key this person holds in this commune, AS OF THIS REQUEST.
	//
	// THE ONE OBLIGATION THE CONTRACT PUTS ON US, and the place somebody will try to optimise it:
	// THIS SET IS SCOPED TO THE SINGLE REQUEST IT AUTHENTICATED. It is never written into a token,
	// never attached to a session, never cached across requests, never written to disk. A withdrawn
	// role must stop working on the very next request — the moment this set outlives one request it
	// IS a role embedded in a token, which skills/session-and-token names FORBIDDEN and which rule
	// 5, invariant 4 exists to prevent. There is no TTL short enough to make it safe: identity runs
	// several replicas, there is no shared cache (ADR 0010), and a revocation served by one replica
	// has no channel on which to reach a copy held by another.
	//
	// AN EMPTY SET IS A REAL ANSWER, not a failure: an account whose role was withdrawn holds no
	// key. It is why "present with no keys" and "no principal" must never be merged — see Middleware.
	PermissionKeys []authz.Perm
}

// Resolver turns one session credential into a principal.
//
// THREE OUTCOMES, AND COLLAPSING ANY TWO OF THEM IS THE EXPENSIVE MISTAKE:
//
//	(p, true, nil)    a usable credential. p.PermissionKeys may be EMPTY and that is ordinary.
//	(_, false, nil)   the credential is not usable — unknown, malformed, expired, revoked, the
//	                  account locked or deleted, or issued for another commune. The contract does
//	                  not distinguish these, on purpose: an answer that varies with the reason
//	                  tells whoever is probing how close they are.
//	(_, _, err)       THE CALL DID NOT HAPPEN. Not "no principal": see the package doc.
//
// core/identityclient implements it over gRPC. It is an interface here so the middleware's four
// behaviours can be tested without a running identity service — a test that needs infrastructure
// is a test that stops being run.
type Resolver interface {
	ResolveStaff(ctx context.Context, sessionToken, clientIP string) (StaffPrincipal, bool, error)
}

// boQuyen is the permission set of ONE request, and the two fields beside it are what bind it to
// that request rather than to a person.
//
// IT LIVES IN context.Context AND NOWHERE ELSE — no package-level map, no struct field on the
// middleware, no store. A context dies with the request that carried it, which is the only storage
// that makes "never outlives one request" a property of the code instead of a promise in a comment.
// If you are here to add a cache in front of this, read StaffPrincipal.PermissionKeys first: the
// thing you would be building is a role embedded in a token.
type boQuyen struct {
	canBo string
	xa    tenant.ID
	khoa  map[authz.Perm]struct{}
}

type khoaBoQuyen struct{}

func vaoBoQuyen(ctx context.Context, b boQuyen) context.Context {
	return context.WithValue(ctx, khoaBoQuyen{}, b)
}

func boQuyenTu(ctx context.Context) (boQuyen, bool) {
	b, ok := ctx.Value(khoaBoQuyen{}).(boQuyen)
	return b, ok
}

// Middleware rebuilds the principal from the session cookie, by asking identity.
//
// IT IS MOUNTED INSIDE httpx.TenantMiddleware, always. Two things depend on that and both fail
// badly without it: the outgoing gRPC call carries the commune from the context (core/grpcx
// refuses to send it otherwise), and the principal is stamped with the commune resolved from Host.
// Mounted outside, every request would answer 503 with a message about identity being unreachable
// while identity was perfectly healthy. The panic below is the loud version of that mistake.
func Middleware(pg Resolver, log *slog.Logger) func(http.Handler) http.Handler {
	if pg == nil {
		// At construction, where a human is watching a process fail to start. Wired nil, this
		// middleware would panic on the first request of the first member of staff instead —
		// recovered into a 500 that names nothing.
		panic("staffauth: thiếu Resolver — mọi yêu cầu có phiên sẽ chết ở giữa chuỗi biên")
	}
	if log == nil {
		log = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Precondition, not a step. See the doc comment above.
			xa := tenant.MustFrom(ctx)

			// 1. NO COOKIE MEANS DO NOT CALL. An anonymous request has nothing to resolve, and a
			// round trip that can only answer "no principal" is a round trip paid on every hit of
			// every authz.Public route, forever, with no line anywhere saying why the service is
			// slow. The contract says the same from its end: an empty session_token is
			// INVALID_ARGUMENT, a wiring fault, not an answer.
			c, err := r.Cookie(CookieName)
			if err != nil || c.Value == "" {
				next.ServeHTTP(w, r)
				return
			}

			// 2. One call, no cache, per request. The cost is stated out loud in the RPC's own
			// comment rather than optimised away here.
			//
			// THE TOKEN IS AN ARGUMENT, NEVER A LOG FIELD AND NEVER PART OF AN ERROR (rule 3,
			// rule 8). Nothing below prints c.Value, and nothing may: a credential in the log
			// pipeline cannot be recalled.
			p, co, err := pg.ResolveStaff(ctx, c.Value, httpx.ClientIP(r))

			// 3. THE CALL FAILED — 503, AND SPECIFICALLY NOT 401.
			//
			// 401 would tell every member of staff of every commune to sign in again, and the
			// sign-in they are being sent to runs on the service that is down. It would also be a
			// lie the client acts on: a browser that sees 401 clears its session, so an outage of
			// a few minutes ends with everybody genuinely signed out.
			//
			// EVERY failure lands here, not only UNAVAILABLE: a deadline, a refused connection, a
			// contract disagreement. Answering "no principal" for any of them is the same mistake
			// in a quieter form. The cost of the uniform answer, stated rather than hidden: a
			// misconfiguration that makes identity answer INVALID_ARGUMENT is reported to the
			// client as an outage. The gRPC code is in the log line below, which is where an
			// operator tells the two apart.
			if err != nil {
				log.WarnContext(ctx, "CẢNH BÁO HẠ TẦNG: không xác thực được phiên cán bộ vì không gọi được "+
					"dịch vụ định danh — MỌI yêu cầu có phiên sẽ trả 503 cho tới khi khôi phục",
					"xa", xa.String(), "path", r.URL.Path, "err", err)
				httpx.WriteError(w, http.StatusServiceUnavailable, "service_unavailable",
					"Hệ thống đang tạm thời không kiểm tra được phiên làm việc. Vui lòng thử lại sau ít phút.", "")
				return
			}

			// 4. A credential that is not usable is NOT an error, and must not become a 401 here.
			// The route's own declaration decides: authz.Public still serves, AnyAuthenticated and
			// RequirePermission answer 401 by themselves. Refusing here would mean a Public route
			// stopped working the moment somebody's cookie went stale.
			//
			// IT ALSO DOES NOT CLEAR THE COOKIE, where identity's XacThuc does. That is a
			// deliberate difference, argued on the RPC: this service does not know WHY the
			// credential is unusable — the contract refuses to say — so clearing it would discard a
			// session that is merely being presented at the wrong host, and clearing on behalf of a
			// service that does not issue the cookie is a second owner of it.
			//
			// NOTE WHAT THIS BRANCH IS NOT: it is `co == false`, "there is no principal". A
			// principal WITH AN EMPTY PERMISSION SET does not come here — it is a live session held
			// by somebody who currently holds no permission, and AnyAuthenticated must serve them
			// while RequirePermission refuses them. Merging the two signs that person out instead
			// of telling them they may not do this one thing.
			if !co {
				next.ServeHTTP(w, r)
				return
			}

			// 5. The principal.
			//
			// TenantID IS THE COMMUNE RESOLVED FROM Host, AND THAT IS NOT THE TAUTOLOGY IT LOOKS
			// LIKE. authz.xacNhanXa compares p.TenantID against tenant.MustFrom(ctx), so stamping
			// the Host commune here would make that comparison compare a value with itself — IF
			// nothing else had checked. Something else has: the RPC received this same commune in
			// "x-tenant-id", compared it with the commune inside the credential, and answered no
			// principal at all on a mismatch. So reaching this line already means the two agreed,
			// and the only commune this principal can carry is the one both sides agreed on.
			//
			// Read that argument before changing either end. The day the RPC stops making the
			// comparison, this line silently becomes the tautology it resembles, and nothing in
			// this service turns red.
			//
			// Kind is "staff" because this is the staff RPC; the contract carries no kind field for
			// exactly that reason. Roles is left empty, as XacThuc leaves it: a role carried on the
			// principal is a role change that takes effect only when the session ends.
			ctx = authz.Into(ctx, authz.Principal{
				ID:       p.StaffID,
				Kind:     "staff",
				TenantID: xa,
			})
			// The key set travels beside the principal, bound to this staff member and this
			// commune, for this request only. Checker is the only reader.
			khoa := make(map[authz.Perm]struct{}, len(p.PermissionKeys))
			for _, k := range p.PermissionKeys {
				khoa[k] = struct{}{}
			}
			ctx = vaoBoQuyen(ctx, boQuyen{canBo: p.StaffID, xa: xa, khoa: khoa})

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Checker answers authz.RequirePermission from the key set Middleware put in the context.
//
// IT HOLDS NO STATE AT ALL, and the empty struct is the design rather than an accident: a field on
// this type would be a place for one request's grants to survive into the next one. Everything it
// reads comes from the context of the request being served, which dies with that request.
//
// FAIL CLOSED, EVERY BRANCH. No key set, a set belonging to somebody else, a set from another
// commune, a citizen principal: all answer false. Rule 5, invariant 2 — a missing declaration
// means deny, not allow — and a widening failure here would be invisible: the request succeeds and
// nothing turns red.
type Checker struct{}

// Allows reports whether the principal holds perm, in the commune carried by ctx.
//
// The commune is not a parameter: it rides in the context, and an implementation that ignored it
// would grant one commune's roles inside another (rule 1, invariant 3).
func (Checker) Allows(ctx context.Context, p authz.Principal, perm authz.Perm) bool {
	// Citizens have no roles; they are isolated by identity (rule 4), never by RBAC. A citizen
	// reaching an RBAC-guarded route is a routing mistake, and false is the safe reading of it.
	if p.Kind != "staff" || p.ID == "" || perm == "" {
		return false
	}

	b, ok := boQuyenTu(ctx)
	if !ok {
		// No set means Middleware never ran for this request — mounted wrong, or a handler called
		// with a context it built itself. Answering false refuses; answering anything else would
		// grant a permission nobody looked up.
		return false
	}

	// THE SET IS BOUND TO ONE PERSON. A handler that constructs its own authz.Principal and asks
	// about it gets false unless it is the very principal this request authenticated. Without this
	// line the set would answer for any id somebody cared to pass.
	if b.canBo != p.ID {
		return false
	}

	// ...AND TO ONE COMMUNE. tenant.From rather than MustFrom: a Checker can be reached from a
	// background goroutine holding a derived context, and taking the process down there turns one
	// mis-wired call into an outage. No commune means no answer, which means false.
	xa, co := tenant.From(ctx)
	if !co || b.xa != xa || p.TenantID != xa {
		return false
	}

	_, giu := b.khoa[perm]
	return giu
}

var _ authz.Checker = Checker{}
