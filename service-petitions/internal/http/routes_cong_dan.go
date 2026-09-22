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
	"github.com/vihat/vigov/core/idem"
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
	case d.GuiPhieu == nil:
		panic("petitions/http: thiếu use case tiếp nhận phản ánh của công dân — " +
			"POST /api/v1/my-citizen-reports sẽ panic khi có người dân bấm Gửi")
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

	// --- the citizen files a petition ---------------------------------------------------------
	//
	// THE ACT RULE 10 EXISTS FOR, and the first write a member of the public performs on this
	// system. The whole argument for what is and is not taken from the request is on
	// HandlerCongDan.GuiPhieu.
	//
	// THE COLLECTION, WITH NO TRAILING SLASH — and `cmd/server` has to agree, which is not automatic.
	// The outer mux splits the two edge chains on a PATH PREFIX, and a prefix pattern ending in `/`
	// does NOT match the collection itself. Without a second, exact pattern there, Go's ServeMux
	// answers a bare `/api/v1/my-citizen-reports` with a 307 to the slash form — which matches no
	// route and returns `404 page not found`. cmd/server therefore registers the collection on the
	// citizen chain EXPLICITLY, and main_test.go asserts a POST to it is neither redirected nor
	// served by the staff chain.
	//
	// `idem.Required(idem.MoKhiHong)` — AND WHICH LAYER IS ACTUALLY PROTECTING THIS, the question
	// skills/rest-api-design §4 says to answer at the route. THE HONEST ANSWER IS: ONLY THIS ONE.
	// The catalogue write routes can say `UNIQUE (tenant_id, ma)` sits underneath them; there is no
	// equivalent here, because two submissions of the same report are two DIFFERENT rows with two
	// different random lookup codes and nothing in the schema can tell them apart. Saying so is the
	// point — a reviewer must not assume a second layer that does not exist.
	//
	// MoKhiHong AND NOT DongKhiHong, WITH THE COST STATED RATHER THAN GLOSSED. With no second layer,
	// a Redis outage means a double-tapped Gửi really can produce two petitions, and rule 7 makes
	// that permanent: the duplicate can only be soft-deleted and the code it consumed is never
	// reissued. Against that: DongKhiHong would answer 503 to every citizen for the duration of a
	// CACHE outage, on the one channel a commune has for hearing from the public, while the service
	// and the database are both healthy. The skill's own table settles it — "Intake paths. Refusing a
	// citizen because a cache is down is worse than a rare duplicate" — and the legal-consequence
	// cases it reserves DongKhiHong for are money, issued document numbers and CLOSING a commitment,
	// none of which is opening one.
	//
	// THE KEY IS SCOPED TO THE CITIZEN, and that is what makes it safe here. idem.Required refuses a
	// request with no principal outright (500), because an anonymous key space is shared and the
	// second sender would be handed the first one's lookup code — which on THIS route would be one
	// citizen reading another's petition. authz.CitizenOnly runs outside idem, so the principal is
	// always there by the time the key is built.
	//
	// @summary  Công dân gửi một phiếu phản ánh — trả MÃ TRA CỨU ngay khi tiếp nhận
	// @screen   09-phan-anh-nguoi-dan §13
	// @request  guiPhanAnhVao
	// 201 carries the lookup code (rule 10, invariant 1) in the SAME shape the GET answers, with the
	// contact details masked — see HandlerCongDan.GuiPhieu.
	//
	// 400 is a body that is not JSON, a body over 64 KiB, an empty or over-long field, and a body
	// naming something the client does not decide (người gửi, lĩnh vực, kênh, mã, trạng thái, hạn).
	//
	// 401 is the answer to THREE situations, folded together exactly as on the read route (ADR 0022):
	// no bearer token · a token that is not usable · a session that has not chosen a commune yet.
	//
	// 409 is idem's answer to a second request arriving while the first is still running. A second
	// request after the first FINISHED replays the original 201 and its lookup code instead.
	//
	// 503 is the commune having no processing-deadline configuration — TODAY'S ANSWER FOR EVERY
	// COMMUNE. No row is written and NO LOOKUP CODE IS ISSUED: an issued code is never reissued
	// (rule 7, invariant 3), so handing one out for a petition that does not exist cannot be undone.
	//
	// THERE IS NO 403 AND THERE CANNOT BE: citizens hold no permissions (rule 5, invariant 6).
	//
	// @reply    201 phieuCuaToiRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/my-citizen-reports",
		authz.CitizenOnly()(
			httpx.XaTuPhien()(
				idem.Required(idem.MoKhiHong)(
					http.HandlerFunc(h.GuiPhieu)))))
}
