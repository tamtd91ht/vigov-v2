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
	case d.Rating == nil:
		panic("petitions/http: thiếu use case đánh giá phản ánh của công dân — " +
			"POST /api/v1/my-citizen-reports/{maTraCuu}/rating sẽ panic khi có người dân chấm sao")
	case d.CitizenFields == nil:
		panic("petitions/http: thiếu danh mục lĩnh vực cho công dân — " +
			"GET /api/v1/my-citizen-report-fields sẽ panic khi có người dân mở bước chọn lĩnh vực")
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
	// 403 IS NEVER A PERMISSION REFUSAL — citizens hold no permissions (rule 5, invariant 6). It has
	// exactly ONE cause: `chua_xac_thuc_so`, a usable session with no verified phone, answered by
	// httpx.XaTuPhien before this handler runs (ADR 0045). It says nothing about any record — the
	// session has no citizen identity to filter by yet — and the Mini App needs the distinct code to
	// ask for the phone; folded into 401 it would reopen a session and loop.
	//
	// 200 CARRIES MASKED CONTACT DETAILS EVEN THOUGH THE READER TYPED THEM — the two reasons are
	// on phieuCuaToiRa.ReporterPhone. No audit entry is written on any branch; the reasoning, and
	// the condition under which it would stop holding, is on HandlerCongDan.PhieuCuaToi.
	//
	// 503 `field_catalogue_unavailable`: the petition carries a field and its label (commune wording,
	// else platform default) cannot be read — refused rather than showing a raw code (ADR 0060 §3).
	//
	// @reply    200 phieuCuaToiRa
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/my-citizen-reports/{maTraCuu}",
		authz.CitizenOnly()(
			httpx.XaTuPhien()(
				http.HandlerFunc(h.PhieuCuaToi))))

	// --- the citizen's own petitions, newest first ("Phản ánh của tôi") ------------------------
	//
	// THE COLLECTION, SERVED ON THE CITIZEN CHAIN BY THE SAME EXACT PATTERN THE POST USES: cmd/server
	// registers `tapCongDan` with no method, so a GET to it reaches this mux and not the staff one.
	// The reasoning for everything read and not read from the request is on
	// HandlerCongDan.DanhSachPhieuCuaToi; the store method binds the commune to $1 and the session
	// citizen to $2, and neither can be switched off from a call site.
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Danh sách phiếu phản ánh CỦA CHÍNH NGƯỜI GỬI trong xã của phiên, mới nhất trước — phân trang theo con trỏ, lọc tuỳ chọn theo trạng thái
	// @screen   09-phan-anh-nguoi-dan §8
	// 200 WITH `items: []` IS THE ANSWER FOR "NOTHING TO SHOW", whatever the cause — no petition filed,
	// petitions only in another commune, petitions only soft-deleted. None of those is a 404 or a
	// 403: a list that answered differently for them would say something about records that are not
	// the caller's (rule 4, forbidden #2). Petitions booked by staff with no citizen account never
	// appear.
	//
	// 400 is a bad cursor, limit, sort or order (only `desc` is accepted), or a `status` outside the
	// nine. The body never echoes what was sent.
	//
	// 401 is the same three situations the other citizen routes fold together (ADR 0022).
	//
	// 403 `chua_xac_thuc_so` only — a session with no verified phone (httpx.XaTuPhien, ADR 0045),
	// the same single cause as on the read route. Never a permission refusal.
	//
	// 503 `field_catalogue_unavailable`: a petition on the page carries a field and the labels cannot
	// be read (ADR 0060 §3).
	//
	// @reply    200 page.Result[phieuCuaToiTomTatRa]
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/my-citizen-reports",
		authz.CitizenOnly()(
			httpx.XaTuPhien()(
				http.HandlerFunc(h.DanhSachPhieuCuaToi))))

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
	// `field` (ADR 0050 point 1) is the code the citizen picked from GET /api/v1/my-citizen-report-fields.
	// OPTIONAL FOR NOW — ADR 0050 point 9 makes it required, but the shared citizen screen does not send
	// it yet. With a field, the petition carries it and BOTH deadlines (`acknowledge_due`,
	// `resolve_due`) are fixed at intake by ONE identity call for that field; without one,
	// `resolve_due` stays null until staff classify, as before. `field_label` in the 201 is the
	// commune's wording, else the platform default.
	//
	// 400 is a body that is not JSON, a body over 64 KiB, an empty or over-long box, a scene location
	// that is not two numbers in range sent together (`lat`/`lng`, both optional), a body naming
	// something the client does not decide (người gửi, kênh, mã, trạng thái, hạn — or the field spelled
	// `linh_vuc`), and `field_not_offered`: ONE identical answer for any field the commune's form does
	// not offer — unknown, retired, switched off, `can-bo`, another commune's, or blank. Nothing is
	// written and identity is not asked.
	//
	// 401 is the answer to THREE situations, folded together exactly as on the read route (ADR 0022):
	// no bearer token · a token that is not usable · a session that has not chosen a commune yet.
	//
	// 409 is idem's answer to a second request arriving while the first is still running. A second
	// request after the first FINISHED replays the original 201 and its lookup code instead.
	//
	// 503 is one of two things, and neither writes a row or ISSUES A LOOKUP CODE (an issued code is
	// never reissued, rule 7 invariant 3, so handing one out for a petition that does not exist cannot
	// be undone):
	//
	//	intake_not_configured        the commune has no processing-deadline configuration (for the
	//	                             picked field, or the default row) — TODAY'S ANSWER FOR EVERY COMMUNE
	//	field_catalogue_unavailable  a field was sent and platform could not be read past the 60-second
	//	                             cache (ADR 0060 §3) — clears by itself
	//
	// 403 `chua_xac_thuc_so` only — a session with no verified phone (httpx.XaTuPhien, ADR 0045).
	// No row is written and no lookup code is issued. Never a permission refusal: citizens hold no
	// permissions (rule 5, invariant 6).
	//
	// @reply    201 phieuCuaToiRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/my-citizen-reports",
		authz.CitizenOnly()(
			httpx.XaTuPhien()(
				idem.Required(idem.MoKhiHong)(
					http.HandlerFunc(h.GuiPhieu)))))

	// --- the citizen rates their own petition (ADR 0050 point 2) --------------------------------
	//
	// A SUB-PATH OF `my-citizen-reports/{code}`, so it rides the citizen chain through the same
	// prefix rule cmd/server already has (`tienToCongDan`) — no new edge wiring, and tools/ingress
	// routes it with the resource it belongs to.
	//
	// `rating`, A NOUN FOR THE SUB-RESOURCE THE CITIZEN WRITES, the name the owner gave the route and
	// migration 0017 gave the column (`rating_comment`). kb/00-foundation/ubiquitous-language.md has no
	// row for "đánh giá" yet — a finding for that table, not a second English word invented here.
	//
	// `idem.Required(idem.MoKhiHong)` — THE SAME CHOICE AS THE INTAKE ABOVE, and the second layer is
	// stated rather than assumed: a double-tapped 1–2 star rating is caught by the lifecycle too — the
	// first one reopened the petition, the second finds `dang-xu-ly` and is a 409, so no second
	// reopening is possible even with Redis down. A double-tapped 3–5 star rating writes the same value
	// twice and two audit entries, which is the cost of MoKhiHong accepted over refusing every citizen
	// for the length of a cache outage.
	//
	// @summary  Công dân chấm 1–5 sao cho phiếu phản ánh CỦA CHÍNH MÌNH khi phiếu đã xử lý / chờ xác nhận — 1–2 sao tự mở lại phiếu (không tính lại hạn)
	// @screen   09-phan-anh-nguoi-dan §8
	// @request  ratingInput
	// 200 carries the petition in the SAME shape the GET answers: its status (`dang-xu-ly` after a
	// reopening, unchanged otherwise) and the rating just recorded.
	//
	// 400 is a body that is not JSON, stars outside 1..5 (or missing), or a comment over 1000 characters.
	//
	// 401 is the same three situations the other citizen routes fold together (ADR 0022).
	//
	// 404 is the SAME FOUR CAUSES AND THE SAME BODY as the GET: no such code · another citizen's code ·
	// another commune's code · soft deleted (rule 4, forbidden #2).
	//
	// 409 is the petition not being at `da-xu-ly` / `cho-dan-xac-nhan` (not finished yet, already
	// reopened, closed, or a terminal branch), or it moved while the request ran — and idem's answer to
	// a duplicate still in flight.
	//
	// 403 `chua_xac_thuc_so` only — a session with no verified phone (httpx.XaTuPhien, ADR 0045).
	// Never a permission refusal: citizens hold no permissions (rule 5, invariant 6).
	//
	// @reply    200 phieuCuaToiRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/my-citizen-reports/{maTraCuu}/rating",
		authz.CitizenOnly()(
			httpx.XaTuPhien()(
				idem.Required(idem.MoKhiHong)(
					http.HandlerFunc(h.RatePetition)))))

	// --- the fields the commune offers on the new-submission form (ADR 0050 point 1) --------------
	//
	// `my-citizen-report-fields`: the citizen-surface counterpart of the staff resource
	// `citizen-report-fields`, per kb/00-foundation/ubiquitous-language.md §Tiền tố `my-` — a resource
	// of its own, so tools/ingress routes it separately and the two surfaces never share a handler.
	// cmd/server mounts this exact path on the CITIZEN chain; the staff chain would answer 404.
	//
	// SESSION-ONLY (user decision 2026-09-29): no public `?host=` variant. The commune comes from the
	// session, never from a client-supplied value (rule 1, forbidden #2).
	//
	// XaTuPhienChiXem AND NOT XaTuPhien: the list is the commune's configuration, the same for every
	// resident, and reads nobody's records — so a session without a verified phone may see it, and
	// the Mini App can show step 1 before asking for the phone. The handler takes no identity from
	// the principal and filters by none (ADR 0045 stop condition #6 is about exactly that).
	//
	// Active on the platform, enabled by the commune, commune order; `can-bo` is never listed until its
	// leaders-only flow exists (ADR 0050 point 10, open question #27).
	//
	// 200 WITH `items: []` is a commune that switched every field off — the form has nothing to offer.
	//
	// 401 is the three situations the other citizen routes fold together (ADR 0022). No 403: there is
	// no permission, and the phone is not required here.
	//
	// 503 `field_catalogue_unavailable`: platform unreachable past the 60-second cache (ADR 0060 §3).
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Danh sách lĩnh vực xã đang mở cho người dân chọn khi gửi phản ánh, theo thứ tự của xã
	// @screen   09-phan-anh-nguoi-dan §13
	// @reply    200 citizenFieldListOut
	// @reply    401 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/my-citizen-report-fields",
		authz.CitizenOnly()(
			httpx.XaTuPhienChiXem("danh mục lĩnh vực là cấu hình của xã, như nhau với mọi người dân và không chứa hồ sơ của ai — dân chưa xác nhận số vẫn cần thấy bước chọn lĩnh vực")(
				http.HandlerFunc(h.ListCitizenReportFields))))
}

// CitizenFieldsPath is the citizen catalogue's path, for cmd/server to mount on the citizen chain.
//
// THE ROUTE ABOVE SPELLS IT AS A LITERAL, because tools/apidoc reads route patterns at build time and
// accepts only a literal or a same-file constant used whole. The two spellings are held together by
// cmd/server's TestCitizenFieldCatalogueRidesTheCitizenChain, which requests THIS constant through the
// real chain: if they ever differ, the request finds no route and the test turns red.
const CitizenFieldsPath = "/api/v1/my-citizen-report-fields"
