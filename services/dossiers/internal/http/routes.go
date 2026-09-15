package http

// Routes for the dossiers service.
//
// EVERY route declares its permission explicitly. The global guard only rejects users who are
// not signed in; it does not check permissions. A route with no declaration is callable by
// every staff role, and nothing reports it — see rule 5.
//
// Four declarations are available, and there is no fifth:
//
//	authz.RequirePermission(checker, "dossiers", authz.View)   // the normal case
//	authz.CitizenOnly()                                     // citizen paths, isolated by identity
//	authz.AnyAuthenticated("<why any account needs this>")  // reason mandatory
//	authz.Public("<why this is public>")                    // reason mandatory

import (
	"net/http"

	"github.com/vihat/vigov/pkg/authz"
)

// Deps are everything the routes need. Kept explicit so wiring stays in cmd/server.
type Deps struct {
	Checker authz.Checker
}

// Register mounts the dossiers routes.
//
// No business routes yet — this is the skeleton. Add them with their permission declaration
// in the SAME statement, never on a nearby line: rbac_guard anchors to the statement, and so
// should a reader.
func Register(mux *http.ServeMux, d Deps) {
	_ = d // no routes yet

	// Example of the shape every real route must take:
	//
	//	mux.Handle("GET /dossiers", authz.RequirePermission(d.Checker, "dossiers", authz.View)(
	//		http.HandlerFunc(h.list)))
}
