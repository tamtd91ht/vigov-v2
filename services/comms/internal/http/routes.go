package http

// Routes for the comms service.
//
// EVERY route declares its permission explicitly. The global guard only rejects users who are
// not signed in; it does not check permissions. A route with no declaration is callable by
// every staff role, and nothing reports it — see rule 5.
//
// Four declarations are available, and there is no fifth:
//
//	authz.RequirePermission(checker, "content.read")   // the normal case
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

// Register mounts the comms routes.
//
// No business routes yet — this is the skeleton. Add them with their permission declaration
// in the SAME statement, never on a nearby line: rbac_guard anchors to the statement, and so
// should a reader.
func Register(mux *http.ServeMux, d Deps) {
	_ = d // no routes yet

	// Example of the shape every real route must take:
	//
	//	mux.Handle("GET /api/v1/articles", authz.RequirePermission(d.Checker, "content.read")(
	//		http.HandlerFunc(h.list)))
	//
	// Paths are English, plural, versioned. A state-changing route declares duplicate
	// protection in the SAME statement, and a non-CRUD action is a nominalised
	// sub-resource (.../closure), never a verb.
	// -> .claude/skills/rest-api-design/SKILL.md
}
