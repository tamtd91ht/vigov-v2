package http

// Routes for the CITIZEN surface of the petitions service — the first citizen edge in this
// repository, so the shape here is the template every other service will copy.
//
// # WHY A SECOND Register AND A SECOND MUX, RATHER THAN MORE LINES IN Register
//
// Rule 4, invariant 5: citizen routes and staff routes are separated AT ROUTING LEVEL. Two
// functions writing into two muxes is what makes that true mechanically —
//
//	1. the two surfaces run behind DIFFERENT middleware chains, and they must. The staff chain
//	   resolves the commune from Host (httpx.TenantMiddleware, 404 when unknown); the citizen
//	   chain resolves it from the session (httpx.CitizenEdge + httpx.XaTuPhien, ADR 0022) because
//	   the Mini App has NO DOMAIN. A citizen route registered on the staff mux answers 404 to
//	   every citizen, and nothing in this package would be red;
//	2. the two take DIFFERENT Deps, so a citizen route cannot reach the unfiltered staff read even
//	   by typing it — see the note at the top of phieu_cua_toi.go.
//
// # THE DECLARATIONS EVERY CITIZEN ROUTE CARRIES — two axes, both mandatory, same statement
//
//	authz.CitizenOnly()   WHO   — a citizen principal, or 401. No RBAC: rule 5, invariant 6
//	httpx.XaTuPhien()     WHICH COMMUNE — from the session, or 401 (ADR 0022)
//
// There is no third declaration and no default. `tools/apidoc` REFUSES a citizen route missing
// either one, and refuses `httpx.KhongThuocXa` on anything that touches business data — that class
// exists only for the commune-picker path, which reads no commune's data at all.
//
// The @-annotation block sits IMMEDIATELY above the statement, no blank line, exactly as on the
// staff routes: apidoc reads it into kb/20-contracts/openapi.json, and tools/ingress builds the
// routing table FROM that contract. A route added without regenerating both is a route that 404s
// in the cluster while working perfectly in a test.

import (
	"net/http"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
)

// RegisterCongDan mounts the citizen routes onto their OWN mux.
//
// The mux passed here must be the one behind the CITIZEN chain. Handing it the staff mux compiles
// and serves and is wrong in the one way no test in this package can see — which is why
// cmd/server/main_test.go drives the real chain and asserts that this path is NOT behind Host
// resolution.
func RegisterCongDan(mux *http.ServeMux, d DepsCongDan) {
	// Refusing incomplete wiring at construction, before any commune is served — the same
	// discipline as Register, and it matters more here: this surface faces citizens, so the first
	// person to find a nil would be a member of the public holding a lookup code.
	switch {
	case d.Phieu == nil:
		panic("petitions/http: thiếu kho đọc phiếu theo danh tính công dân — " +
			"GET /api/v1/my-citizen-reports/{maTraCuu} sẽ panic khi có người gọi")
	case d.NhanLinhVuc == nil:
		panic("petitions/http: thiếu kho nhãn lĩnh vực — " +
			"GET /api/v1/my-citizen-reports/{maTraCuu} sẽ panic khi có người gọi")
	}

	h := NewHandlerCongDan(d)

	// --- the petition the citizen filed, looked up by the code they were handed ---------------
	//
	// `my-citizen-reports` IS THE SETTLED NOUN, decided 2026-09-22 and owned by
	// kb/00-foundation/ubiquitous-language.md §Tiền tố `my-`. It is a RESOURCE OF ITS OWN and not
	// a sub-path of `citizen-reports`, so tools/ingress emits a rule of its own for it: the two
	// surfaces are then separable at the NETWORK layer, not only inside this process.
	//
	// `/api/cong/…` (docs/ui-ux/09 §13) WAS REJECTED FOR A MECHANICAL REASON, recorded so nobody
	// re-proposes it: tools/ingress routes only what lives under `/api/v1/` and stops on anything
	// else, so that path would be a 404 the day it deployed. It is also a Vietnamese path segment,
	// against ADR 0011.
	//
	// NO idem.* DECLARATION: a GET changes no state, and claiming duplicate-request protection
	// where there is nothing to protect is a claim a reviewer would have to check and disbelieve.
	//
	// @summary  Phiếu phản ánh CỦA CHÍNH NGƯỜI GỬI, tra theo mã tra cứu — dùng trong Zalo Mini App
	// @screen   09-phan-anh-nguoi-dan §8
	// 401 is the answer to THREE situations, and folding them together is the design (ADR 0022):
	// no bearer token · a token that is not usable (unknown, expired, revoked) · a session that
	// has not chosen a commune yet. Telling them apart tells somebody probing how far they got.
	//
	// 404 is the answer to FOUR situations, and folding THOSE together is rule 4, forbidden #2:
	// no such code · a code belonging to ANOTHER CITIZEN · a code of another commune · a
	// soft-deleted petition. The body is identical in all four.
	//
	// THERE IS NO 403 ON THIS ROUTE AND THERE CANNOT BE. Citizens hold no permissions (rule 5,
	// invariant 6), so there is no permission that could fail; every refusal is either "not a
	// usable session" (401) or "no such petition of yours" (404).
	//
	// 200 CARRIES MASKED CONTACT DETAILS EVEN THOUGH THE READER TYPED THEM — the two reasons are
	// on phieuCuaToiRa.ReporterPhone. No audit entry is written on any branch; the reasoning, and
	// the condition under which it would stop holding, is on HandlerCongDan.PhieuCuaToi.
	//
	// @reply    200 phieuCuaToiRa
	// @reply    401 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/my-citizen-reports/{maTraCuu}",
		authz.CitizenOnly()(
			httpx.XaTuPhien()(
				http.HandlerFunc(h.PhieuCuaToi))))
}
