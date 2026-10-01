package http

// Routes of the OPERATOR edge — mounted ONLY on the handler chain service-platform serves for
// Host == OPERATOR_HOST (cmd/server: buildOperatorEdge), always behind operatorRequestGuard
// (operator_request_guard.go: Origin + JSON on every non-GET, the CSRF defence). Every other Host reaches the commune chain,
// where none of these exist; with OPERATOR_HOST unset this function is never called (ADR 0048
// §28/09 #2 + #4).
//
// EVERY ROUTE DECLARES ITS REALM GUARD IN THE SAME STATEMENT, and there are three (internal/opauth):
//
//	opauth.RequireKey(d.Auth, opauth.Key…)  every listed `ops.*` key required
//	opauth.SignedIn(d.Auth, "<reason>")    any live operator session
//	opauth.Public("<reason>")              the sign-in steps; always behind signInLimit
//
// NOT authz.*: those are the commune realm's guards, and an operator principal cannot satisfy them
// (and the reverse). tools/apidoc reads `opauth.*` and keeps these routes OUT of
// kb/20-contracts/openapi.json — the commune-facing contract from which the Ingress and web-admin's
// gateway table are generated (ADR 0048 §01/10 #6c). A route here that reached that file would be
// routable on a commune's host.
//
// THE PATH NOUNS BELOW were confirmed by the user on 2026-10-01 and are recorded in
// kb/00-foundation/ubiquitous-language.md §"Miền vận hành ViHAT". Renaming one now is a contract
// change, not a tidy-up: the console and anything an integrator wrote call these exact paths.

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/service-platform/internal/opauth"
)

// OperatorDeps is everything the operator routes need. Kept explicit so wiring stays in cmd/server.
type OperatorDeps struct {
	Auth     *opauth.Auth
	Identity OperatorIdentity
	// Limiter is the per-IP operator sign-in limit (ratelimit.OperatorSignIn, ADR 0048 §01/10 #3).
	// It must not be nil: a missing store is wired as a counter that always fails, which the
	// middleware turns into 503 — fail closed, never an unbounded sign-in route.
	Limiter  *ratelimit.Limiter
	Registry CommuneReader
	Writer   CommuneWriter
	// OperatorHost is refused as a commune host (ADR 0048 stop condition #6).
	OperatorHost string
	NewID        func() (string, error)
	// Forget drops a host from this process's cached directory after a registry write. Other
	// services keep their cached answer for up to one TENANT_CACHE_TTL.
	Forget func(host string)
	Now    func() time.Time
	Log    *slog.Logger
}

// signInKey is the limiter key: the address the edge observed (httpx.ClientIP, which trusts only
// TRUSTED_PROXY_CIDRS), never a header the client wrote.
func signInKey(r *http.Request) ratelimit.Key { return ratelimit.OperatorIPKey(httpx.ClientIP(r)) }

// OperatorHandler is the operator routes behind operatorRequestGuard — the ONLY way to obtain
// them. The routes are mounted by an unexported function so that no caller can take the mux
// without the guard: a mux exported bare is a mux somebody will mount bare.
func OperatorHandler(d OperatorDeps) http.Handler {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	mux := http.NewServeMux()
	registerOperator(mux, d)
	return operatorRequestGuard(d.OperatorHost, d.Log)(mux)
}

// registerOperator mounts the operator routes on mux. Reached only through OperatorHandler.
func registerOperator(mux *http.ServeMux, d OperatorDeps) {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	if d.Forget == nil {
		d.Forget = func(string) {}
	}
	h := &operatorHandlers{d: d}
	limit := ratelimit.Middleware(d.Limiter, signInKey, d.Log)

	// --- sign-in -------------------------------------------------------------------------------

	// @summary  Đăng nhập khu vận hành: mật khẩu + mã TOTP hoặc mã khôi phục
	// @request  operatorSignInBody
	// @reply    201 operatorSessionView
	// @reply    400 httpx.Error invalid_body second_factor_ambiguous
	// @reply    401 httpx.Error sign_in_refused
	// @reply    403 httpx.Error enrollment_required
	// @reply    429 httpx.Error rate_limited
	// @reply    503 httpx.Error operator_auth_unavailable rate_limit_unavailable
	mux.Handle("POST /api/v1/operator-sessions", limit(
		opauth.Public("signing in is what creates an operator session; bounded 20/15 min per IP")(
			idem.KhongCan("every accepted sign-in opens a new session by contract; a retry is a second sign-in, not a duplicate record")(
				http.HandlerFunc(h.createSession)))))

	// @summary  Bắt đầu đăng ký ứng dụng xác thực (lần đăng nhập đầu, bằng mật khẩu tạm)
	// @request  operatorEnrollmentBody
	// @reply    201 operatorEnrollmentView
	// @reply    400 httpx.Error invalid_body
	// @reply    401 httpx.Error sign_in_refused
	// @reply    429 httpx.Error rate_limited
	// @reply    503 httpx.Error operator_auth_unavailable rate_limit_unavailable
	mux.Handle("POST /api/v1/operator-enrollments", limit(
		opauth.Public("first sign-in with the temporary password, before any session exists; bounded 20/15 min per IP")(
			idem.KhongCan("calling again replaces the pending secret by contract (operator.proto)")(
				http.HandlerFunc(h.beginEnrollment)))))

	// @summary  Hoàn tất đăng ký: mật khẩu mới + mã TOTP từ bí mật đang chờ
	// @request  operatorEnrollmentCompletionBody
	// @reply    201 operatorEnrollmentCompletedView
	// @reply    400 httpx.Error invalid_body
	// @reply    401 httpx.Error sign_in_refused
	// @reply    422 passwordRejectionView new_password_rejected
	// @reply    429 httpx.Error rate_limited
	// @reply    503 httpx.Error operator_auth_unavailable rate_limit_unavailable
	mux.Handle("POST /api/v1/operator-enrollments/completion", limit(
		opauth.Public("completes a first sign-in, before any session exists; bounded 20/15 min per IP")(
			idem.KhongCan("a second completion is refused by identity once the factor is active")(
				http.HandlerFunc(h.completeEnrollment)))))

	// --- the signed-in operator ----------------------------------------------------------------

	// @summary  Người vận hành đang đăng nhập: mã VH- và các khoá ops.*
	// @reply    200 operatorWhoAmIView
	// @reply    401 httpx.Error unauthorized
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("GET /api/v1/operator-sessions/current",
		opauth.SignedIn(h.d.Auth, "the console shows who is signed in and hides what the keys do not allow")(
			http.HandlerFunc(h.whoAmI)))

	// @summary  Đăng xuất khu vận hành
	// @reply    204 -
	// @reply    401 httpx.Error unauthorized
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("DELETE /api/v1/operator-sessions/current",
		opauth.SignedIn(h.d.Auth, "every operator may end their own session")(
			idem.KhongCan("revoking an already revoked session is the same outcome")(
				http.HandlerFunc(h.signOut))))

	// @summary  Đổi mật khẩu người vận hành (cần mật khẩu hiện tại + TOTP); thu hồi mọi phiên
	// @request  operatorPasswordBody
	// @reply    204 -
	// @reply    400 httpx.Error invalid_body
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error credentials_refused
	// @reply    422 passwordRejectionView new_password_rejected
	// @reply    429 httpx.Error rate_limited
	// @reply    503 httpx.Error operator_auth_unavailable rate_limit_unavailable
	mux.Handle("PUT /api/v1/operators/current/password", limit(
		opauth.SignedIn(h.d.Auth, "every operator changes only their own password, re-proving it and a TOTP code")(
			idem.KhongCan("a repeat carries the old password, which identity then refuses")(
				http.HandlerFunc(h.changePassword)))))

	// @summary  Tạo lại bộ mã khôi phục (cần TOTP); bộ cũ bị huỷ
	// @request  operatorRecoveryCodesBody
	// @reply    201 operatorRecoveryCodesView
	// @reply    400 httpx.Error invalid_body
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error credentials_refused
	// @reply    429 httpx.Error rate_limited
	// @reply    503 httpx.Error operator_auth_unavailable rate_limit_unavailable
	mux.Handle("POST /api/v1/operators/current/recovery-codes", limit(
		opauth.SignedIn(h.d.Auth, "every operator regenerates only their own recovery codes, re-proving a TOTP code")(
			idem.KhongCan("each call voids the previous batch by contract; the latest batch is the only valid one")(
				http.HandlerFunc(h.regenerateRecoveryCodes)))))

	// --- the commune registry ------------------------------------------------------------------

	// @summary  Danh sách mọi xã (chỉ siêu dữ liệu: tên, tỉnh, trạng thái, tên miền)
	// @reply    200 communePageView
	// @reply    400 httpx.Error invalid_cursor invalid_sort invalid_limit
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("GET /api/v1/communes",
		opauth.RequireKey(h.d.Auth, opauth.KeyTenantManage)(
			http.HandlerFunc(h.listCommunes)))

	// @summary  Một xã: siêu dữ liệu và các Mini App riêng đã gắn
	// @reply    200 communeDetailView
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error commune_not_found
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("GET /api/v1/communes/{id}",
		opauth.RequireKey(h.d.Auth, opauth.KeyTenantManage)(
			http.HandlerFunc(h.getCommune)))

	// @summary  Danh mục tỉnh, thành phố (cho biểu mẫu tạo xã)
	// @reply    200 provinceListView
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("GET /api/v1/provinces",
		opauth.RequireKey(h.d.Auth, opauth.KeyTenantManage)(
			http.HandlerFunc(h.listProvinces)))

	// @summary  Tạo xã: tên, tỉnh, tên miền chính
	// @request  createCommuneBody
	// @reply    201 communeDetailView
	// @reply    400 httpx.Error invalid_body
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    409 httpx.Error domain_taken duplicate_name
	// @reply    422 httpx.Error invalid_name invalid_domain reserved_domain unknown_province
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("POST /api/v1/communes",
		opauth.RequireKey(h.d.Auth, opauth.KeyTenantManage, opauth.KeyDomainManage)(
			idem.KhongCan("a repeat is refused by the primary host it carries: tenant_domain.host is the primary key")(
				http.HandlerFunc(h.createCommune))))

	// @summary  Thêm tên miền (không chính) cho một xã
	// @request  addDomainBody
	// @reply    201 communeDetailView
	// @reply    400 httpx.Error invalid_body
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error commune_not_found
	// @reply    409 httpx.Error domain_taken commune_inactive
	// @reply    422 httpx.Error invalid_domain reserved_domain
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("POST /api/v1/communes/{id}/domains",
		opauth.RequireKey(h.d.Auth, opauth.KeyDomainManage)(
			idem.KhongCan("a repeat is refused by tenant_domain.host, the primary key")(
				http.HandlerFunc(h.addDomain))))

	// @summary  Đặt tên miền chính của một xã (trong các tên miền xã đang giữ)
	// @request  primaryDomainBody
	// @reply    200 communeDetailView
	// @reply    400 httpx.Error invalid_body
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error commune_not_found
	// @reply    409 httpx.Error commune_inactive
	// @reply    422 httpx.Error domain_not_in_commune invalid_domain reserved_domain
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("PUT /api/v1/communes/{id}/primary-domain",
		opauth.RequireKey(h.d.Auth, opauth.KeyDomainManage)(
			idem.KhongCan("setting the host that is already primary changes nothing and writes no entry")(
				http.HandlerFunc(h.setPrimaryDomain))))

	// @summary  Sửa lỗi gõ trong tên xã (bắt buộc lý do; không phải đổi tên đơn vị hành chính)
	// @request  correctNameBody
	// @reply    200 communeDetailView
	// @reply    400 httpx.Error invalid_body
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error commune_not_found
	// @reply    409 httpx.Error duplicate_name commune_inactive
	// @reply    422 httpx.Error invalid_name invalid_reason
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("PUT /api/v1/communes/{id}/name",
		opauth.RequireKey(h.d.Auth, opauth.KeyTenantManage)(
			idem.KhongCan("setting the name it already has changes nothing and writes no entry")(
				http.HandlerFunc(h.correctName))))

	// @summary  Bật / tắt hoạt động của một xã (bắt buộc lý do); tắt thì tên miền của xã ngừng phân giải
	// @request  activationBody
	// @reply    200 communeDetailView
	// @reply    400 httpx.Error invalid_body
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error commune_not_found
	// @reply    409 httpx.Error commune_succeeded
	// @reply    422 httpx.Error invalid_reason
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("PUT /api/v1/communes/{id}/activation",
		opauth.RequireKey(h.d.Auth, opauth.KeyTenantManage)(
			idem.KhongCan("setting the state it already has changes nothing and writes no entry")(
				http.HandlerFunc(h.setActivation))))

	// @summary  Gắn Mini App riêng của xã (chế độ rieng)
	// @request  attachMiniAppBody
	// @reply    201 miniAppView
	// @reply    400 httpx.Error invalid_body
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error commune_not_found
	// @reply    409 httpx.Error mini_app_taken commune_inactive
	// @reply    422 httpx.Error invalid_app_id invalid_note
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("POST /api/v1/communes/{id}/mini-apps",
		opauth.RequireKey(h.d.Auth, opauth.KeyMiniAppManage)(
			idem.KhongCan("a repeat is refused by mini_app.app_id, the primary key, which keeps soft-deleted rows")(
				http.HandlerFunc(h.attachMiniApp))))
}
