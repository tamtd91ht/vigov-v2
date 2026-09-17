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

	"github.com/vihat/vigov/pkg/authz"
	"github.com/vihat/vigov/pkg/idem"
	"github.com/vihat/vigov/pkg/token"
	"github.com/vihat/vigov/services/identity/internal/app"
	"github.com/vihat/vigov/services/identity/internal/domain"
	idstore "github.com/vihat/vigov/services/identity/internal/store"
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
	Signer   *token.Signer
	Phien    PhienDoc
	CanBo    CanBoDoc
	DangNhap DangNhapUC
	DangXuat DangXuatUC
	Log      *slog.Logger
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
	case d.DangNhap == nil || d.DangXuat == nil:
		panic("identity/http: thiếu use case đăng nhập/đăng xuất")
	case d.Checker == nil:
		panic("identity/http: thiếu authz.Checker — mọi route có quyền sẽ không kiểm được")
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
}
