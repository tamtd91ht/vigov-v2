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

	"github.com/vihat/vigov/pkg/tenant"
)

type Action string

const (
	View    Action = "view"
	Edit    Action = "edit"
	Approve Action = "approve"
	Admin   Action = "admin"
)

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

// Checker answers whether a principal may perform an action in a subsystem.
type Checker interface {
	Allows(ctx context.Context, p Principal, subsystem string, a Action) bool
}

// RequirePermission guards a route. This is the normal case.
func RequirePermission(c Checker, subsystem string, a Action) func(http.Handler) http.Handler {
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
			if !c.Allows(ctx, p, subsystem, a) {
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
