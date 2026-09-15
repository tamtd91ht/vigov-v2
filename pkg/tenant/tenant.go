// Package tenant carries the commune identity for one request.
//
// WHY A PACKAGE AND NOT A PARAMETER: tenant_id is a DATA DIMENSION, not an argument. If a
// business function takes it as a parameter it can be passed wrong, and eventually it will
// be. Here it rides in context.Context from the edge down to the store, and no business
// function ever names it.
//
// The whole point of MustFrom panicking: failing loudly in development beats failing
// silently in production, where silence means serving every commune's data at once.
package tenant

import (
	"context"
	"errors"
	"fmt"
)

// ID is the opaque, immutable identifier of a commune (ULID).
//
// It is deliberately NOT the administrative code, the domain name, or the commune name.
// Vietnam reorganises commune-level units periodically; an identifier carrying meaning would
// force rewriting foreign keys across archival records at the first merger, which the law
// does not permit. See kb/10-decisions/0004-shard-by-tenant.md.
type ID string

func (t ID) String() string { return string(t) }
func (t ID) Valid() bool    { return len(t) == 26 } // ULID length

type ctxKey struct{}

var ErrNoTenant = errors.New("tenant: no commune in context")

// Into returns a context carrying the commune. Called exactly once, at the edge.
func Into(ctx context.Context, id ID) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// From reports the commune, if any. Prefer MustFrom in service code.
func From(ctx context.Context) (ID, bool) {
	id, ok := ctx.Value(ctxKey{}).(ID)
	return id, ok && id.Valid()
}

// MustFrom returns the commune or panics.
//
// Panicking is the point. A query without a commune returns rows from EVERY commune and
// raises no error; that failure is invisible until somebody complains. A panic in a handler
// is recovered into a 500 and shows up immediately in development.
func MustFrom(ctx context.Context) ID {
	id, ok := From(ctx)
	if !ok {
		panic(fmt.Sprintf("tenant: %v — a query would have crossed commune boundaries", ErrNoTenant))
	}
	return id
}

// Directory resolves an incoming Host to a commune.
//
// Implementations must be cached with a short TTL: at 200+ communes this is on the path of
// every single request, and the mapping changes only when a commune is renamed or its domain
// is reassigned. Invalidate on those events, never poll.
type Directory interface {
	// ByHost returns the commune for a Host header.
	// A Host matching nothing must return ok=false — never a fallback commune.
	ByHost(ctx context.Context, host string) (Tenant, bool)
}

// Tenant is the platform's view of a commune. Business services never store this; they hold
// only the ID and read the rest through the platform service when they need to display it.
type Tenant struct {
	ID     ID
	Host   string
	Name   string // display name at this moment in time, not an identifier
	Active bool
}
