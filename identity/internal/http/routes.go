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
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/token"
	"github.com/vihat/vigov/identity/internal/app"
	"github.com/vihat/vigov/identity/internal/domain"
	idstore "github.com/vihat/vigov/identity/internal/store"
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
	Checker  authz.Checker
	Quyen    QuyenDoc
	Signer   *token.Signer
	Phien    PhienDoc
	CanBo    CanBoDoc
	DanhBa   CanBoDanhBa
	DangNhap DangNhapUC
	DangXuat DangXuatUC

	// Xa is the SAME tenant.Directory the edge resolves Host with — see the note at the
	// GET /api/v1/commune route. tenant.Directory rather than a local interface because it is
	// already an interface, already owned by pkg/tenant, and a second declaration of it here
	// would be a second place for "which commune is this Host" to be answered.
	Xa tenant.Directory

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
	case d.Xa == nil:
		panic("identity/http: thiếu thư mục xã — màn hình đăng nhập sẽ không có tên xã để hiển thị")
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
	// @summary  Phiên làm việc hiện tại của chính người gọi, kèm danh sách quyền để ẩn/hiện menu
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
	// `commune` HAS NO ROW YET in kb/00-foundation/ubiquitous-language.md's URL-resource table;
	// the path was named in the request for this work, not translated here (ADR 0011 forbids
	// translating on the spot). SINGULAR, against the general plural rule, and deliberately: a
	// caller can never see more than one, because which one it gets is decided by the Host it
	// arrived on. `/communes` would promise a collection this route must never have.
	//
	// NO idem.* DECLARATION: GET, changes no state.
	//
	// @summary  Thông tin xã ứng với tên miền đang gọi, cho màn hình đăng nhập
	// @screen   15-phu-luc-giao-dien-chung §1
	// @reply    200 thongTinXa
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/commune",
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
	// ONE HONEST NOTE ON THE 401/403 LINES BELOW. Those two statuses are produced by
	// authz.RequirePermission, which today answers with http.Error — a plain-text body, not the
	// httpx.Error JSON the contract declares. The declaration states the shape the whole system
	// has agreed on (pkg/httpx/edge.go) and the one every other route here returns; closing the
	// gap is a change inside pkg/authz, outside this service.

	// @summary  Danh sách cán bộ của xã — gồm cả người có tài khoản đăng nhập và người chỉ có trong danh bạ
	// @screen   14-cau-hinh §3
	// @reply    200 trangCanBo
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
}
