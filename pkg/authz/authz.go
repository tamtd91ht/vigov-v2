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

	"github.com/vihat/vigov/pkg/tenant"
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
			// The commune in the token must match the commune resolved from Host. A mismatch
			// is not an ordinary user error: browsers do not send cookies across hosts, so
			// seeing this means a deliberate probe or a stolen token.
			if p.TenantID != tenant.MustFrom(ctx) {
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
func AnyAuthenticated(reason string) func(http.Handler) http.Handler {
	if reason == "" {
		panic("authz: AnyAuthenticated requires a reason")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := From(r.Context()); !ok {
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
