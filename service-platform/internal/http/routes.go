package http

// Routes for the platform service.
//
// EVERY route declares its permission explicitly. The global guard only rejects users who are
// not signed in; it does not check permissions. A route with no declaration is callable by
// every staff role, and nothing reports it — see rule 5.
//
// Four declarations are available, and there is no fifth:
//
//	authz.RequirePermission(checker, "<not agreed yet>")   // the normal case
//	authz.CitizenOnly()                                     // citizen paths, isolated by identity
//	authz.AnyAuthenticated("<why any account needs this>")  // reason mandatory
//	authz.Public("<why this is public>")                    // reason mandatory

import (
	"net/http"

	"github.com/vihat/vigov/core/authz"
)

// Deps are everything the routes need. Kept explicit so wiring stays in cmd/server.
type Deps struct {
	Checker authz.Checker
}

// Register mounts the platform routes.
//
// No business routes yet — this is the skeleton. Add them with their permission declaration
// in the SAME statement, never on a nearby line: rbac_guard anchors to the statement, and so
// should a reader.
func Register(mux *http.ServeMux, d Deps) {
	_ = d // no routes yet

	// Example of the shape every real route must take:
	//
	// The platform console is the VENDOR's, not a commune's. Its permissions do not live
	// in a commune's `quyen` table, and ADR 0003 limits this service to metadata — there
	// is deliberately no path to a commune's business data. Do not reach for a commune
	// permission key here; the vendor-side model has to be agreed first.
	//
	//	mux.Handle("GET /api/v1/communes", authz.RequirePermission(d.Checker, "<not agreed yet>")(
	//		http.HandlerFunc(h.list)))
	//
	// Paths are English, plural, versioned. A state-changing route declares duplicate
	// protection in the SAME statement, and a non-CRUD action is a nominalised
	// sub-resource (.../closure), never a verb.
	// -> .claude/skills/rest-api-design/SKILL.md
}
