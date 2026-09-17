package http

// Routes for the identity service.
//
// EVERY route declares its permission explicitly. The global guard only rejects users who are
// not signed in; it does not check permissions. A route with no declaration is callable by
// every staff role, and nothing reports it — see rule 5.
//
// Four declarations are available, and there is no fifth:
//
//	authz.RequirePermission(checker, "admin.user")   // the normal case
//	authz.CitizenOnly()                                     // citizen paths, isolated by identity
//	authz.AnyAuthenticated("<why any account needs this>")  // reason mandatory
//	authz.Public("<why this is public>")                    // reason mandatory
//
// EVERY route also carries an @-annotation block IMMEDIATELY above the statement — no blank
// line between. `tools/apidoc` reads it and generates kb/20-contracts/openapi.json, which is
// the type contract the admin web builds against. Without it the web types every response by
// hand, which is how v1 ended up with three hand-copied versions of one shape.
//
//	// @summary  <one line, Vietnamese — a person reads it>
//	// @screen   <file in docs/ui-ux/ §section>   design intent, the one part no tool can derive
//	// @request  <Go type>                        omit when the route takes no body
//	// @reply    <status> <Go type|->             one line per status the handler REALLY returns
//
// The permission and the idempotency mode are NOT annotated: apidoc reads them from the
// authz.* / idem.* calls below, so there is no second copy to drift (rule 9).

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/token"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The four collaborators below are interfaces, not the concrete store and use-case types.
//
// WHY: the routes and the middleware carry the isolation rules this service exists to enforce —
// the commune check before any read, the sid that may only be its owner's, the cookie with no
// Domain. Those have to be testable without a PostgreSQL, or they get tested once and then
// never again. The concrete *idstore.PhienStore, *idstore.CanBoStore, *app.DangNhap and
// *app.DangXuat satisfy these as they are; nothing was changed to accommodate them.
type (
	// PhienDoc is the session registry, read on every request.
	PhienDoc interface {
		KiemTra(ctx context.Context, sid string) (idstore.Phien, error)
		GhiNhanDung(ctx context.Context, sid string)
	}

	// CanBoDoc rebuilds the staff account behind a session.
	CanBoDoc interface {
		TheoID(ctx context.Context, id string) (domain.CanBo, error)
	}

	// CanBoDanhBa reads the commune's staff register for the two routes below.
	//
	// SEPARATE FROM CanBoDoc ON PURPOSE, although *idstore.CanBoStore satisfies both and the same
	// value is wired into each. CanBoDoc runs on EVERY request that carries a session, and its
	// query filters three conditions so a locked or withdrawn account stops working at once;
	// this one runs on one screen and filters only `deleted_at IS NULL`, because the register
	// must show the people who have no account at all. Widening CanBoDoc to carry both would put
	// the register's looser predicate one careless edit away from the authentication path.
	CanBoDanhBa interface {
		DanhSach(ctx context.Context, yc page.Request) (page.Result[domain.CanBoTomTat], error)
		ChiTiet(ctx context.Context, id string) (domain.CanBoTomTat, error)
	}

	// QuyenDoc lists the caller's OWN permission keys, for GET /api/v1/sessions/current.
	//
	// SEPARATE FROM authz.Checker ON PURPOSE, although *idstore.Checker satisfies both and the
	// same value is wired into each. Checker DECIDES access, one key at a time, and every route in
	// all eight services depends on it; this one only DESCRIBES what the caller already holds, so
	// the web can avoid drawing what the server would refuse. Widening authz.Checker with a list
	// method would put a shape whose only job is drawing menus onto the interface that guards
	// every route — and the first caller to decide access from a list is the accident that costs.
	QuyenDoc interface {
		QuyenCua(ctx context.Context, p authz.Principal) ([]authz.Perm, error)
	}

	// VaiTroDoc reads the CALLER'S OWN role, for GET /api/v1/sessions/current.
	//
	// READ IN THE HANDLER, NOT AT THE EDGE, AND THIS IS THE DECISION MOST LIKELY TO BE UNDONE BY
	// SOMEBODY BEING HELPFUL. XacThuc already reads the account at step 5 and copies HoTen and
	// ChucVu onto PhienHienTai, with a comment saying it does so "so the route costs no second
	// query on a call that happens on every page load". Adding the role there looks like the same
	// move and is not: those two fields are already on the row the middleware HAS to read, while
	// the role is a second table and therefore a JOIN. The middleware runs on EVERY request of
	// EVERY route in this service; the role is wanted by ONE route. Paying a join on every request
	// of every screen to save one query on one route is the wrong side of that trade by three
	// orders of magnitude.
	//
	// The bool is the second outcome and is not optional to handle — see VaiTroCuaCanBo.
	VaiTroDoc interface {
		VaiTroCuaCanBo(ctx context.Context, canBoID string) (domain.VaiTro, bool, error)
	}

	// BoPhanDanhMuc is the commune's org chart, for GET /api/v1/org-units.
	//
	// NO page.Request PARAMETER, unlike CanBoDanhBa: this route returns the whole list on purpose.
	// The reason is on idstore.BoPhanStore.DanhSach, and the bound that replaces the missing `limit`
	// is idstore.TranDanhMucBoPhan.
	BoPhanDanhMuc interface {
		DanhSach(ctx context.Context) ([]domain.BoPhan, error)
	}

	// VaiTroDanhMuc is the commune's role catalogue, for GET /api/v1/roles.
	//
	// SEPARATE FROM VaiTroDoc ON PURPOSE, even though one store implements both. That one answers
	// "what is the CALLER's role" for GET /api/v1/sessions/current; this one answers "what roles
	// does this commune have". Merging them would hand a handler the whole catalogue where it only
	// ever needed the caller's own row — and a dependency wider than the work requires is how the
	// next person justifies using it.
	VaiTroDanhMuc interface {
		DanhSach(ctx context.Context) ([]domain.VaiTro, error)
	}

	// DangNhapUC and DangXuatUC are the use cases. The handlers only translate HTTP; the
	// business write and its audit entry share one transaction inside these.
	DangNhapUC interface {
		Chay(ctx context.Context, yc app.YeuCauDangNhap) (app.KetQuaDangNhap, error)
	}

	DangXuatUC interface {
		Chay(ctx context.Context, sid, maCanBo, ip string) error
	}
)

// Deps are everything the routes need. Kept explicit so wiring stays in cmd/server.
type Deps struct {
	Checker   authz.Checker
	Quyen     QuyenDoc
	VaiTro    VaiTroDoc
	BoPhan    BoPhanDanhMuc
	VaiTroMuc VaiTroDanhMuc
	Signer    *token.Signer
	Phien     PhienDoc
	CanBo     CanBoDoc
	DanhBa    CanBoDanhBa
	DangNhap  DangNhapUC
	DangXuat  DangXuatUC

	Log *slog.Logger
}

// Register mounts the identity routes.
//
// Add every new route with its permission declaration in the SAME statement, never on a nearby
// line: rbac_guard anchors to the statement, and so should a reader.
func Register(mux *http.ServeMux, d Deps) {
	// Refusing incomplete wiring HERE, at construction, not at request time: a sign-in route
	// mounted without a signing key or without the session registry would accept requests it
	// cannot honour, and the first person to find out would be a member of staff locked out of
	// a government system. Same discipline as authz.Public("") and idem.KhongCan("").
	switch {
	case d.Signer == nil:
		panic("identity/http: thiếu token.Signer — không ký được phiên")
	case d.Phien == nil || d.CanBo == nil:
		panic("identity/http: thiếu kho phiên hoặc kho cán bộ — không dựng được Principal")
	case d.DanhBa == nil:
		panic("identity/http: thiếu kho danh bạ cán bộ — hai tuyến đọc cán bộ sẽ panic khi có người gọi")
	case d.DangNhap == nil || d.DangXuat == nil:
		panic("identity/http: thiếu use case đăng nhập/đăng xuất")
	case d.Checker == nil:
		panic("identity/http: thiếu authz.Checker — mọi route có quyền sẽ không kiểm được")
	case d.Quyen == nil:
		panic("identity/http: thiếu kho quyền — GET /api/v1/sessions/current sẽ panic khi có người gọi")
	case d.VaiTro == nil:
		panic("identity/http: thiếu kho vai trò — GET /api/v1/sessions/current sẽ panic khi có người gọi")
	case d.BoPhan == nil:
		panic("identity/http: thiếu kho bộ phận — GET /api/v1/org-units sẽ panic khi có người gọi")
	case d.VaiTroMuc == nil:
		panic("identity/http: thiếu kho danh mục vai trò — GET /api/v1/roles sẽ panic khi có người gọi")
	}

	h := NewHandler(d)

	// Opening a session. Public because there is no session yet to check a permission against —
	// this IS the screen that creates one.
	//
	// KhongCan and not Required: signing in twice opens a second session, which is revocable,
	// leaves no archival record and consumes no business code. There is nothing here that rule 7
	// would make permanent. Password guessing is a different problem with a different answer
	// (rate limiting), and pretending an idempotency key addresses it would be the more
	// dangerous mistake.
	//
	// @summary  Đăng nhập bằng email và mật khẩu, mở một phiên làm việc
	// @screen   15-phu-luc-giao-dien-chung §1
	// @request  thanDangNhap
	// @reply    201 phanHoiDangNhap
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/sessions",
		authz.Public("màn hình đăng nhập — chưa có phiên nên chưa có gì để kiểm quyền")(
			idem.KhongCan("đăng nhập lần hai mở một phiên thứ hai, thu hồi được, không sinh hồ sơ lưu trữ; chống dò mật khẩu là việc của rate limit")(
				http.HandlerFunc(h.DangNhap))))

	// Ending a session. AnyAuthenticated and not a permission: every account must be able to
	// sign itself out, and requiring a permission for it would leave an account whose role was
	// stripped unable to close its own session. The handler enforces the real limit — the sid
	// must be this request's own.
	//
	// 404 and not 403 on somebody else's sid is deliberate and is part of the contract: the
	// existence of another person's session is itself information (rule 4, forbidden #2).
	//
	// @summary  Kết thúc phiên làm việc của chính mình
	// @screen   15-phu-luc-giao-dien-chung §3.3
	// @reply    204 -
	// @reply    401 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/sessions/{sid}",
		authz.AnyAuthenticated("mọi tài khoản đã đăng nhập đều được kết thúc phiên của chính mình")(
			idem.KhongCan("thu hồi một phiên đã thu hồi cho cùng một kết quả")(
				http.HandlerFunc(h.DangXuat))))

	// Reading one's own session. AnyAuthenticated and not a permission, for a reason close to the
	// one above: this is the route that answers "who am I and what may I do", and an account that
	// has just had every role withdrawn holds no permission with which to ask. Requiring one would
	// mean the person whose rights changed is exactly the person who can no longer load the
	// screen that would tell them.
	//
	// The literal `current` cannot shadow the wildcard route above: that one is DELETE, this one
	// is GET, and net/http matches method first. It could only collide if a session id were
	// literally "current", and a sid is a random value the server generates, never a string a
	// client chooses (idstore.PhienStore.Tao).
	//
	// NO idem.* DECLARATION: a GET changes no state, and declaring a duplicate-request protection
	// here would claim a protection with nothing to protect.
	//
	// @summary  Phiên làm việc hiện tại của chính người gọi, kèm vai trò và danh sách quyền để ẩn/hiện menu
	// @screen   15-phu-luc-giao-dien-chung §1
	// @reply    200 phienHienTaiRa
	// @reply    401 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/sessions/current",
		authz.AnyAuthenticated("mọi tài khoản đã đăng nhập đều được hỏi 'tôi là ai' — kể cả tài khoản vừa bị gỡ hết vai trò, vốn không còn quyền nào để đòi")(
			http.HandlerFunc(h.XemPhienHienTai)))

	// --- the commune behind this Host ---------------------------------------------------------
	//
	// WHICH SERVICE THIS ROUTE BELONGS TO IS NOT SETTLED, AND THIS COMMENT IS NOT A DECISION.
	// It is built here because that is what was asked for this turn; the question is rule 2, stop
	// conditions #1 and #2 and is going to the customer. The two readings, stated so the next
	// reader does not have to reconstruct them:
	//
	//	platform owns it   the commune registry — domains, display name, lifecycle — is the
	//	                   platform service's data (kb/30-indexes/services.json). Identity reads it
	//	                   over gRPC and owns none of it. platform/internal/http/
	//	                   routes.go:42 already sketches a GET /api/v1/communes there — the
	//	                   vendor's list of all communes, not this route, but it is the same data
	//	                   and it is held up on an undecided vendor-side permission model. And a
	//	                   commune's name is needed by EVERY screen, not only the sign-in one.
	//	identity owns it   identity is the service the browser already talks to before a session
	//	                   exists, it resolves the commune at its own edge on every request anyway,
	//	                   and one more public surface on the platform service is one more thing to
	//	                   expose to the internet.
	//
	// PUBLIC, AND THE REASON IS THE SCREEN ITSELF: the sign-in form prints the name of the
	// authority a person is about to sign in to, and at that moment there is no session, therefore
	// no principal, therefore nothing to check a permission against. Same reading as POST
	// /api/v1/sessions above. Nothing in the reply is personal data, nothing counts staff, nothing
	// describes the commune's internal operation — see thongTinXa, which is where the absent
	// fields are argued.
	//
	// `/communes/current` — PLURAL SEGMENT, SINGULAR SUB-RESOURCE, and this replaces an earlier
	// `/api/v1/commune` that argued itself an exception to the plural rule.
	//
	// The old argument was that a caller can never see more than one, so a plural segment would
	// promise a collection this route must never have. That reasoning ignored what the plural
	// segment actually names: the RESOURCE TYPE, not the size of one answer. `skills/
	// rest-api-design` REQUIRED #1 has no exception, and an endpoint granting itself one is the
	// shape rule 9 calls drift.
	//
	// WHAT MADE IT WORTH CHANGING rather than leaving as a style point: the citizen Mini App is
	// getting `GET /api/v1/communes` — the list of communes a citizen picks from when they belong
	// to none yet. Two paths one letter `s` apart, answering two different questions, serving two
	// classes of user with very different trust levels (rule 4). Neither leaks — `thongTinXa`
	// carries no id and the platform summary carries no host — so this is a confusion risk, not a
	// leak. It is also the kind nobody untangles six months later.
	//
	// `/current` is not invented here: `GET /api/v1/sessions/current` above is the same shape,
	// answering the same kind of question — "which one is THIS request's", derived from the
	// request itself and never from a parameter.
	//
	// NO idem.* DECLARATION: GET, changes no state.
	//
	// @summary  Thông tin xã ứng với tên miền đang gọi, cho màn hình đăng nhập
	// @screen   15-phu-luc-giao-dien-chung §1
	// MỘT mã trả lời duy nhất, và đó là hệ quả trực tiếp của việc biên mang cả tenant.Tenant:
	// handler không còn phân giải gì nữa nên không còn thất bại nào để khai. Tên miền không
	// thuộc xã nào thì bị biên từ chối bằng 404 TRƯỚC khi tới đây, nên 404 không phải kết quả
	// của tuyến này mà của chuỗi trước nó.
	//
	// @reply    200 thongTinXa
	mux.Handle("GET /api/v1/communes/current",
		authz.Public("màn hình đăng nhập phải hiện tên xã TRƯỚC khi có phiên nào để kiểm quyền")(
			http.HandlerFunc(h.ThongTinXa)))

	// --- the staff register. TWO READ ROUTES, AND DELIBERATELY NO WRITE ROUTE -----------------
	//
	// `staff` is the resource noun, and it is the SAME word as `message Staff` in the proto. One
	// concept was already carrying four names — nguoi_dung (table), can_bo (Go), CanBo (type),
	// Staff (contract); picking `staff` adds no fifth. See kb/00-foundation/ubiquitous-language.md.
	//
	// admin.user — "Quản lý người dùng" — is the permission the Cấu hình → Người dùng tab is
	// declared with (docs/ui-ux/14-cau-hinh.md §12.8). No route here is AnyAuthenticated: a
	// commune's staff register is not something every role has business reading.
	//
	// NO idem.* DECLARATION: these are GET routes and change no state. rest_api_guard only asks
	// for one on POST/PUT/PATCH/DELETE, and declaring one here would claim a protection that has
	// nothing to protect.
	//
	// 401 AND 403 REALLY ARE httpx.Error. They come from authz.RequirePermission, which used to
	// answer with http.Error — a plain-text body while the contract declared JSON. That gap was
	// closed in core/authz; this note stays so nobody "fixes" it a second time.

	// @summary  Danh sách cán bộ của xã — gồm cả người có tài khoản đăng nhập và người chỉ có trong danh bạ
	// @screen   14-cau-hinh §3
	// @page     idstore.SapXepCanBo
	// @reply    200 page.Result[canBoTomTat]
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/staff",
		authz.RequirePermission(d.Checker, "admin.user")(
			http.HandlerFunc(h.DanhSachCanBo)))

	// 404 and not 403 for an id belonging to another commune: the existence of another
	// authority's record is itself information (rule 4, forbidden #2). Same reading as the sid on
	// DELETE /api/v1/sessions/{sid} above.
	//
	// @summary  Chi tiết một cán bộ trong xã
	// @screen   14-cau-hinh §3
	// @reply    200 canBoTomTat
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/staff/{id}",
		authz.RequirePermission(d.Checker, "admin.user")(
			http.HandlerFunc(h.ChiTietCanBo)))

	// --- the commune's organisational chart ----------------------------------------------------
	//
	// `org-units` AND NOT `departments`, AND THE NAME WAS LOOKED UP RATHER THAN TRANSLATED.
	// kb/00-foundation/ubiquitous-language.md owns the URL-resource mapping (ADR 0011) and settles
	// this row: the tree holds Đảng uỷ, HĐND and UBMTTQ as well as the UBND's own units, so
	// `departments` would assert that every node is a department of the People's Committee. Most of
	// them are not. A path cannot be taken back once a commune is live, which is why the table
	// exists and why nothing here translates on the spot.
	//
	// AnyAuthenticated, AND THE REASON IS THE SHAPE OF THE DATA'S USE. Unit names appear on nearly
	// every screen — the assignment box on a task, document routing, the staff directory, the
	// filter dropdowns — so requiring a configuration permission would not protect anything, it
	// would break every one of those screens for everybody who is not an administrator. The
	// alternative that actually protects something does not exist here: there is nothing sensitive
	// in a list of the authority's own units.
	//
	// THE TRADE-OFF, STATED RATHER THAN GLOSSED: a commune's org chart is readable by every signed-in
	// account OF THAT COMMUNE. It is not readable across communes and cannot be — Scoped.Query binds
	// `tenant_id` from the context (rule 1, invariant 5), so the same request against another
	// commune's domain is refused at the token layer before any query runs. What is accepted is that
	// a member of staff with no configuration rights can see how their own authority is organised,
	// which is information they can also read off the noticeboard in the lobby.
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Danh mục bộ phận của xã — cây tổ chức, dùng cho ô phân công, luồng văn bản và bộ lọc
	// @screen   14-cau-hinh §3
	// 500 covers two different causes and says so honestly: an ordinary store failure, and the
	// commune's org chart exceeding idstore.TranDanhMucBoPhan — which this route REFUSES rather
	// than truncating, because a silently short list is a unit missing from an assignment box.
	//
	// @reply    200 danhSachBoPhanRa
	// @reply    401 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/org-units",
		authz.AnyAuthenticated("tên bộ phận xuất hiện ở ô phân công nhiệm vụ, luồng văn bản, danh bạ và mọi bộ lọc — đòi một quyền cấu hình sẽ làm hỏng những màn hình đó cho mọi tài khoản không phải quản trị; đánh đổi đã chấp nhận: sơ đồ tổ chức lộ cho mọi tài khoản đã đăng nhập CỦA CHÍNH XÃ ĐÓ, không chéo xã vì Scoped buộc tenant_id")(
			http.HandlerFunc(h.DanhSachBoPhan)))

	// The commune's role catalogue. GET /api/v1/roles
	//
	// `roles` — the URL-resource table settles this row (kb/00-foundation/ubiquitous-language.md).
	// It was ASKED rather than translated on the spot: `org-units` one route up is what happens when
	// the obvious English word (`departments`) asserts something false, so the obvious word gets
	// looked up here too even when it turns out to be right. `role` is already the term rule 5
	// invariant 3 uses — `(tenant_id, role, permission)` — so the contract surface and the
	// authorisation model now say the same word for the same thing.
	//
	// AnyAuthenticated, SAME REASON AS org-units: role names fill the picker on the staff form, the
	// column on the staff directory and the header of the Phân quyền matrix. Gating them behind a
	// configuration permission would break the screen for whoever holds `admin.role` but not
	// `admin.user`, and vice versa — three permissions to render one screen.
	//
	// THE TRADE-OFF, STATED: every signed-in account OF THIS COMMUNE can read what the commune's
	// roles are called. Not across communes, and it cannot be — Scoped.Query binds `tenant_id` from
	// the context (rule 1, invariant 5). What is NOT readable here is what each role may DO: the
	// permission grants live in `vai_tro_quyen` and no route exposes them.
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Danh mục vai trò của xã — dùng cho ô chọn vai trò, cột danh bạ và ma trận phân quyền
	// @screen   14-cau-hinh §4
	// @reply    200 danhSachVaiTroRa
	// @reply    401 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/roles",
		authz.AnyAuthenticated("tên vai trò xuất hiện ở ô chọn vai trò trên form cán bộ, cột Vai trò của danh bạ và tiêu đề cột của ma trận phân quyền — đòi một quyền cấu hình sẽ cần ba quyền để dựng một màn hình; đánh đổi đã chấp nhận: tên các vai trò lộ cho mọi tài khoản đã đăng nhập CỦA CHÍNH XÃ ĐÓ, còn quyền của từng vai trò thì không tuyến nào phơi ra")(
			http.HandlerFunc(h.DanhSachVaiTro)))
}
