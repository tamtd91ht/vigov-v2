// Package authz declares who may call what.
//
// WHY EXPLICIT DECLARATIONS: the global guard rejects users who are not logged in. It does
// NOT check permissions. A route with no declaration is therefore callable by EVERY staff
// role, including roles with nothing to do with that subsystem — and nothing reports it, no
// test turns red. Rule 5 exists because that failure is silent.
//
// Permissions are always evaluated WITHIN one commune. A check that forgets the commune is
// cross-commune privilege escalation, not a lesser bug.
package authz

import (
	"context"
	"net/http"
	"strings"

	"github.com/vihat/vigov/core/tenant"
)

// Perm is a permission key: "<nhóm>.<việc>", for example "task.extend".
//
// WHY A FLAT STRING AND NOT (subsystem, action): the administrative permissions this system
// grants are not a Cartesian product. "task.approve" (duyệt hoàn thành) and "task.extend"
// (duyệt gia hạn) are both approvals and are deliberately separate rights — one closes a
// commitment to a citizen, the other moves its deadline. A fixed action set collapses them,
// and a role holding one would silently hold the other.
//
// The keys are the ones the Phân quyền screen shows, so a permission in code, a row in
// `quyen`, and a tick box a commune administrator sees are all the same string. A translation
// layer in between is a place for them to drift.
type Perm string

func (p Perm) String() string { return string(p) }

// Nhom returns the part before the dot: "task.extend" -> "task". Used for grouping on the
// permission matrix, never for deciding access — access is decided by the whole key.
func Nhom(p Perm) string {
	s := string(p)
	if i := strings.IndexByte(s, '.'); i > 0 {
		return s[:i]
	}
	return s
}

// Principal is whoever is making the request.
type Principal struct {
	ID       string
	Kind     string // "staff" | "citizen"
	TenantID tenant.ID
	Roles    []string
}

type ctxKey struct{}

func Into(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

func From(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}

// Checker answers whether a principal holds a permission.
//
// The commune is not a parameter: it rides in the context, and an implementation that ignores
// it grants one commune's roles inside another commune (rule 1, invariant 3).
type Checker interface {
	Allows(ctx context.Context, p Principal, perm Perm) bool
}

// xacNhanXa reports whether the principal was issued for the commune this request arrived at.
//
// ONE FUNCTION, NOT A LINE REPEATED PER GUARD: the same invariant asserted in two places drifts,
// and the copy that gets forgotten is the one nobody notices — a missing commune check is
// cross-commune privilege escalation that no test of a single commune can produce (rule 5,
// invariant 3).
//
// A browser does not send a cookie across hosts, so a mismatch is never an ordinary user error:
// it is a deliberate probe or a stolen token.
//
// tenant.MustFrom panics when there is no commune, which is deliberate: a guard that ran without
// one would compare against nothing and let every commune through. It is a precondition of being
// mounted inside httpx.TenantMiddleware, not a case to handle.
func xacNhanXa(ctx context.Context, p Principal) bool {
	return p.TenantID == tenant.MustFrom(ctx)
}

// RequirePermission guards a route. This is the normal case.
func RequirePermission(c Checker, perm Perm) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			p, ok := From(ctx)
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			// The commune in the token must match the commune resolved from Host — see
			// xacNhanXa.
			if !xacNhanXa(ctx, p) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if !c.Allows(ctx, p, perm) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CitizenOnly guards a citizen route. Citizens have no roles; they are isolated by identity
// (rule 4), not by RBAC.
//
// TODO(stop-condition): THIS GUARD DOES NOT COMPARE THE COMMUNE, and that is not an oversight
// left for the next person to close.
//
// The citizen channel is the Zalo Mini App, which has NO DOMAIN (see CLAUDE.md, STACK). There is
// no Host to derive a commune from, so tenant.MustFrom would PANIC the moment this guard is
// mounted on that channel — turning a missing decision into a 500 on the citizen path. Adding
// the comparison and adding a fallback are both wrong: a fallback on the isolation path is
// rule 1, forbidden #1.
//
// What has to be decided first, by the customer, not here:
//   - how the Mini App resolves the commune (rule 1, stop condition #4 — skills/zalo-miniapp-
//     multi-tenant)
//   - what happens to a citizen acting with more than one commune (rule 4, stop condition #3)
//
// Until then this guard checks IDENTITY ONLY, and no citizen route may be mounted on a chain
// without a commune resolved ahead of it.
func CitizenOnly() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := From(r.Context())
			if !ok || p.Kind != "citizen" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// AnyAuthenticated opens a route to every signed-in account. The reason is mandatory and is
// kept in the binary so it can be audited: six months on, nobody dares remove an unexplained
// exemption.
//
// "Every signed-in account" means every account OF THIS COMMUNE. Dropping the permission check
// does not drop the commune check: waiving what a person may do never waives where they may do
// it (rule 5, invariant 3).
func AnyAuthenticated(reason string) func(http.Handler) http.Handler {
	if reason == "" {
		panic("authz: AnyAuthenticated requires a reason")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			p, ok := From(ctx)
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if !xacNhanXa(ctx, p) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Public opens a route with no authentication. The reason is mandatory for the same reason.
func Public(reason string) func(http.Handler) http.Handler {
	if reason == "" {
		panic("authz: Public requires a specific reason")
	}
	return func(next http.Handler) http.Handler { return next }
}
