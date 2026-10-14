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
//	opauth.RequireKey(d.Auth, opauth.AnyKey) any one `ops.*` key — the console's pure reads (ADR 0073 #1)
//	opauth.SignedIn(d.Auth, "<reason>")    any live operator session
//	opauth.Public("<reason>")              the sign-in steps; always behind signInLimit
//
// plus one narrowing inside SignedIn for the owner-only acts: opauth.RequireOwner() (internal/opauth),
// and one widening, directly inside SignedIn, for the ONLY three routes a session that must still
// change its temporary password may call: opauth.AllowWhilePasswordChangeRequired() (ADR 0048 §13/10
// #6). Every other route refuses that session with 403 password_change_required; adding the widening
// to a fourth route fails TestPasswordChangeRequiredRouteTable.
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

	commsv1 "github.com/vihat/vigov/core/gen/vigov/comms/v1"
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
	// Policies is upload_policy (ADR 0073 #5); OperatorLog the operator log (ADR 0073 #2).
	Policies    UploadPolicyEditor
	OperatorLog OperatorLogReader
	// SharedApp is the shared Mini App + QR link source (owner 04/10/2026); Fields the tier-1 petition
	// field codes (ADR 0073 #3).
	SharedApp SharedMiniAppEditor
	Fields    PetitionFieldEditor
	// MapFrames is a commune's default map frame (ADR 0072 amendment 2, K1).
	MapFrames MapFrameDefaultEditor
	// ZaloBot is comms' ZaloBotOperatorService (ADR 0074; *commsclient.ZaloBotOperatorClient). nil only
	// in dev without COMMS_GRPC_ADDR: every zalo-bots route then answers 503. ZaloBotLog writes the
	// display row of each Zalo Bot write (operator_zalo_bot.go).
	ZaloBot    commsv1.ZaloBotOperatorServiceClient
	ZaloBotLog ZaloBotActRecorder
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

	// THE THREE ROUTES BELOW are the only ones a session with must_change_password may call (ADR 0048
	// §13/10 #6): the console must learn it is restricted (who-am-I), the person must be able to leave
	// (sign-out) and to do the one thing asked of them (change the password). Every other operator
	// route answers that session 403 password_change_required.

	// @summary  Người vận hành đang đăng nhập: mã VH-, các khoá ops.*, is_owner và must_change_password (chỉ để hiển thị; mọi tuyến chỉ-owner tự kiểm lại; phiên phải đổi mật khẩu vẫn gọi được tuyến này)
	// @reply    200 operatorWhoAmIView
	// @reply    401 httpx.Error unauthorized
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("GET /api/v1/operator-sessions/current",
		opauth.SignedIn(h.d.Auth, "the console shows who is signed in and hides what the keys do not allow")(
			opauth.AllowWhilePasswordChangeRequired()(
				http.HandlerFunc(h.whoAmI))))

	// @summary  Đăng xuất khu vận hành (phiên phải đổi mật khẩu vẫn gọi được)
	// @reply    204 -
	// @reply    401 httpx.Error unauthorized
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("DELETE /api/v1/operator-sessions/current",
		opauth.SignedIn(h.d.Auth, "every operator may end their own session")(
			opauth.AllowWhilePasswordChangeRequired()(
				idem.KhongCan("revoking an already revoked session is the same outcome")(
					http.HandlerFunc(h.signOut)))))

	// @summary  Đổi mật khẩu người vận hành (cần mật khẩu hiện tại + TOTP); thu hồi mọi phiên; phiên phải đổi mật khẩu (mật khẩu tạm do owner đặt lại) gọi được tuyến này
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
			opauth.AllowWhilePasswordChangeRequired()(
				idem.KhongCan("a repeat carries the old password, which identity then refuses")(
					http.HandlerFunc(h.changePassword))))))

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
	//
	// THE PURE READS take opauth.AnyKey (ADR 0073 #1): an operator holding ANY ops.* key opens a
	// commune to do the work of that key — an account granted only ops.mini_app.manage could not
	// reach the commune it was granted to work on. Every WRITE keeps its own key.

	// @summary  Danh sách mọi xã (chỉ siêu dữ liệu: tên, tỉnh, trạng thái, tên miền)
	// @reply    200 communePageView
	// @reply    400 httpx.Error invalid_cursor invalid_sort invalid_limit
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("GET /api/v1/communes",
		opauth.RequireKey(h.d.Auth, opauth.AnyKey)(
			http.HandlerFunc(h.listCommunes)))

	// @summary  Một xã: siêu dữ liệu, các Mini App riêng đang gắn kèm trạng thái khoá bí mật (đã đặt lúc … bởi … / chưa đặt / không rõ — không bao giờ trả khoá), và khoá còn sống dưới App ID đã gỡ
	// @reply    200 communeDetailView
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error commune_not_found
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("GET /api/v1/communes/{id}",
		opauth.RequireKey(h.d.Auth, opauth.AnyKey)(
			http.HandlerFunc(h.getCommune)))

	// @summary  Danh mục tỉnh, thành phố (cho biểu mẫu tạo xã)
	// @reply    200 provinceListView
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("GET /api/v1/provinces",
		opauth.RequireKey(h.d.Auth, opauth.AnyKey)(
			http.HandlerFunc(h.listProvinces)))

	// @summary  Tạo xã: tên, tỉnh, tên miền chính
	// @request  createCommuneBody
	// @reply    201 communeDetailView
	// @reply    400 httpx.Error invalid_body
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    409 httpx.Error domain_taken domain_recently_removed duplicate_name
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
	// @reply    409 httpx.Error domain_taken domain_recently_removed commune_inactive
	// @reply    422 httpx.Error invalid_domain reserved_domain
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("POST /api/v1/communes/{id}/domains",
		opauth.RequireKey(h.d.Auth, opauth.KeyDomainManage)(
			idem.KhongCan("a repeat is refused by tenant_domain.host, the primary key")(
				http.HandlerFunc(h.addDomain))))

	// @summary  Gỡ MỀM một tên miền KHÔNG chính của xã: gõ lại đúng tên miền, bắt buộc lý do; ghi removed_at/removed_by/remove_reason và vết go_ten_mien cùng giao dịch; tên miền thôi phân giải ngay, xã khác dùng lại được sau 5 phút (chủ dự án 14/10/2026)
	// @request  domainRemovalBody
	// @reply    201 communeDetailView
	// @reply    400 httpx.Error invalid_body
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error commune_not_found
	// @reply    409 httpx.Error domain_is_primary commune_inactive
	// @reply    422 httpx.Error domain_not_in_commune confirm_domain_mismatch invalid_domain reserved_domain invalid_reason
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("POST /api/v1/communes/{id}/domain-removals",
		opauth.RequireKey(h.d.Auth, opauth.KeyDomainManage)(
			idem.KhongCan("a repeat finds the domain no longer live in the commune and is answered domain_not_in_commune; nothing is written twice")(
				http.HandlerFunc(h.removeDomain))))

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

	// OWNER ONLY (ADR 0048 §Sửa đổi 13/10/2026 #2, #7): SignedIn authenticates, RequireOwner inside it
	// refuses every operator that is not THE owner, whatever ops.* keys they hold (opauth.RequireOwner
	// says why this is not a key and not a fourth declaration).
	//
	// @summary  Xoá MỀM một xã — CHỈ OWNER: xã phải ngừng hoạt động liên tục đủ 7 ngày và người xoá gõ lại đúng một tên miền của xã (bắt buộc lý do); ghi deleted_at/deleted_by/delete_reason, vết xoa_mem_xa ở audit_log của xã cùng giao dịch; xã biến khỏi danh sách, tên miền thôi phân giải nhưng giữ chỗ vĩnh viễn; không có đường khôi phục từ màn hình
	// @request  communeDeletionBody
	// @reply    201 communeDeletionView
	// @reply    400 httpx.Error invalid_body
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error commune_not_found
	// @reply    409 communeDeletionRefusalView commune_active commune_inactive_too_recent
	// @reply    422 httpx.Error confirm_domain_mismatch invalid_reason
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("POST /api/v1/communes/{id}/deletion",
		opauth.SignedIn(h.d.Auth, "OWNER ONLY — opauth.RequireOwner inside refuses every operator that is not the operator owner (ADR 0048 amendment 13/10/2026 #2, #7)")(
			opauth.RequireOwner()(
				idem.KhongCan("a repeat finds the commune already deleted and is answered commune_not_found; nothing is written twice")(
					http.HandlerFunc(h.deleteCommune)))))

	// --- OWNER ONLY: accounts (ADR 0048 §Sửa đổi 13/10/2026 #2–#6; operator_owner_accounts.go) --------
	//
	// Same declaration as the deletion above: SignedIn authenticates, RequireOwner refuses every
	// operator that is not THE owner whatever ops.* keys they hold, and identity re-checks the owner on
	// the session it resolves. Platform is a relay: identity writes each act and its trail entry in one
	// transaction. Every 422 on these routes uses passwordRejectionView's shape for new_password_rejected;
	// invalid_reason is the plain httpx.Error (code, message, trace_id) — apidoc takes one body type per
	// status. Temporary passwords travel in request bodies only and are never echoed.

	// @summary  CHỈ OWNER — danh sách mọi tài khoản vận hành (kể cả tài khoản bị vô hiệu), theo mã VH-; không bao giờ trả mật khẩu, TOTP, mã khôi phục
	// @reply    200 operatorAccountListView
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    503 httpx.Error operator_auth_unavailable account_management_unavailable account_management_not_ready
	mux.Handle("GET /api/v1/operator-accounts",
		opauth.SignedIn(h.d.Auth, "OWNER ONLY — opauth.RequireOwner inside refuses every operator that is not the operator owner (ADR 0048 amendment 13/10/2026 #2)")(
			opauth.RequireOwner()(
				http.HandlerFunc(h.listOperatorAccounts))))

	// @summary  CHỈ OWNER — tạo tài khoản vận hành với mật khẩu TẠM và các khoá ops.* (có thể rỗng); lần đăng nhập đầu buộc đổi mật khẩu và đăng ký TOTP; identity ghi vết operator_audit_log cùng giao dịch
	// @request  operatorAccountBody
	// @reply    201 operatorAccountView
	// @reply    400 httpx.Error invalid_body invalid_account_input
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    409 httpx.Error account_email_taken
	// @reply    422 passwordRejectionView new_password_rejected
	// @reply    503 httpx.Error operator_auth_unavailable account_management_unavailable account_management_not_ready
	mux.Handle("POST /api/v1/operator-accounts",
		opauth.SignedIn(h.d.Auth, "OWNER ONLY — opauth.RequireOwner inside refuses every operator that is not the operator owner (ADR 0048 amendment 13/10/2026 #2)")(
			opauth.RequireOwner()(
				idem.KhongCan("a repeat is refused by identity with ALREADY_EXISTS on the email (operator_account index), nothing written twice (operator.proto); and no store may hold this body")(
					http.HandlerFunc(h.createOperatorAccount)))))

	// @summary  CHỈ OWNER — đặt TOÀN BỘ tập khoá ops.* của một tài khoản vận hành (rỗng = thu hồi hết; bắt buộc lý do); bớt khoá thì mọi phiên của tài khoản ấy bị thu hồi; identity ghi vết từng khoá cấp/thu hồi
	// @request  operatorPermissionsBody
	// @reply    200 operatorAccountView
	// @reply    400 httpx.Error invalid_body invalid_account_input
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error operator_account_not_found
	// @reply    422 httpx.Error invalid_reason
	// @reply    503 httpx.Error operator_auth_unavailable account_management_unavailable account_management_not_ready
	mux.Handle("PUT /api/v1/operator-accounts/{code}/permissions",
		opauth.SignedIn(h.d.Auth, "OWNER ONLY — opauth.RequireOwner inside refuses every operator that is not the operator owner (ADR 0048 amendment 13/10/2026 #2)")(
			opauth.RequireOwner()(
				idem.KhongCan("the body is the COMPLETE key set: the same set twice is no change and writes no entry (operator.proto)")(
					http.HandlerFunc(h.setOperatorPermissions)))))

	// @summary  CHỈ OWNER — đặt lại mật khẩu TẠM cho một tài khoản vận hành (bắt buộc lý do); giữ nguyên TOTP; buộc đổi ở lần đăng nhập sau; mọi phiên của tài khoản ấy bị thu hồi
	// @request  temporaryPasswordBody
	// @reply    200 operatorAccountView
	// @reply    400 httpx.Error invalid_body invalid_account_input
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error operator_account_not_found
	// @reply    422 passwordRejectionView new_password_rejected invalid_reason
	// @reply    503 httpx.Error operator_auth_unavailable account_management_unavailable account_management_not_ready
	mux.Handle("PUT /api/v1/operator-accounts/{code}/password",
		opauth.SignedIn(h.d.Auth, "OWNER ONLY — opauth.RequireOwner inside refuses every operator that is not the operator owner (ADR 0048 amendment 13/10/2026 #2)")(
			opauth.RequireOwner()(
				idem.KhongCan("a retry sets the same temporary password again and writes a second entry — two true entries, never a wrong state (operator.proto); and no store may hold this body")(
					http.HandlerFunc(h.resetOperatorPassword)))))

	// @summary  CHỈ OWNER — tài khoản đăng nhập của một xã (email đăng nhập, họ tên, vai trò, khoá, phải đổi mật khẩu); không số điện thoại, phòng ban, chức vụ; lượt đọc được identity ghi vết vào audit_log của xã
	// @reply    200 staffAccountListView
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error commune_not_found
	// @reply    503 httpx.Error operator_auth_unavailable account_management_unavailable account_management_not_ready
	mux.Handle("GET /api/v1/communes/{id}/staff-accounts",
		opauth.SignedIn(h.d.Auth, "OWNER ONLY — opauth.RequireOwner inside refuses every operator that is not the operator owner (ADR 0048 amendment 13/10/2026 #2)")(
			opauth.RequireOwner()(
				http.HandlerFunc(h.listCommuneStaffAccounts))))

	// @summary  CHỈ OWNER — tạo tài khoản QUẢN TRỊ của một xã đang hoạt động: tên đăng nhập (email), họ tên, mật khẩu TẠM; buộc đổi ở lần đăng nhập đầu; identity ghi vết vào audit_log của xã, người làm là mã VH-
	// @request  communeAdminAccountBody
	// @reply    201 staffAccountView
	// @reply    400 httpx.Error invalid_body invalid_account_input
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error commune_not_found
	// @reply    409 httpx.Error account_email_taken commune_inactive
	// @reply    422 passwordRejectionView new_password_rejected
	// @reply    503 httpx.Error operator_auth_unavailable account_management_unavailable account_management_not_ready
	mux.Handle("POST /api/v1/communes/{id}/admin-accounts",
		opauth.SignedIn(h.d.Auth, "OWNER ONLY — opauth.RequireOwner inside refuses every operator that is not the operator owner (ADR 0048 amendment 13/10/2026 #2)")(
			opauth.RequireOwner()(
				idem.KhongCan("a repeat is refused by identity with ALREADY_EXISTS on the commune's sign-in email, nothing written twice (operator.proto); and no store may hold this body")(
					http.HandlerFunc(h.createCommuneAdminAccount)))))

	// @summary  CHỈ OWNER — đặt lại mật khẩu TẠM cho một tài khoản cán bộ của xã (bắt buộc lý do); buộc đổi ở lần đăng nhập sau; mọi phiên của tài khoản ấy bị thu hồi; tài khoản xã đang khoá thì từ chối
	// @request  temporaryPasswordBody
	// @reply    200 staffAccountView
	// @reply    400 httpx.Error invalid_body invalid_account_input
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error commune_not_found staff_account_not_found
	// @reply    409 httpx.Error staff_account_locked
	// @reply    422 passwordRejectionView new_password_rejected invalid_reason
	// @reply    503 httpx.Error operator_auth_unavailable account_management_unavailable account_management_not_ready
	mux.Handle("PUT /api/v1/communes/{id}/staff-accounts/{user_id}/password",
		opauth.SignedIn(h.d.Auth, "OWNER ONLY — opauth.RequireOwner inside refuses every operator that is not the operator owner (ADR 0048 amendment 13/10/2026 #2)")(
			opauth.RequireOwner()(
				idem.KhongCan("a retry sets the same temporary password again and writes a second entry — two true entries, never a wrong state (operator.proto); and no store may hold this body")(
					http.HandlerFunc(h.resetStaffPassword)))))

	// @summary  Gắn Mini App riêng của xã (chế độ rieng)
	// @request  attachMiniAppBody
	// @reply    201 miniAppView
	// @reply    400 httpx.Error invalid_body
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error commune_not_found
	// @reply    409 httpx.Error mini_app_taken mini_app_removed commune_inactive mini_app_already_running
	// @reply    422 httpx.Error invalid_app_id invalid_note
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("POST /api/v1/communes/{id}/mini-apps",
		opauth.RequireKey(h.d.Auth, opauth.KeyMiniAppManage)(
			idem.KhongCan("a repeat is refused by mini_app.app_id, the primary key, which keeps soft-deleted rows")(
				http.HandlerFunc(h.attachMiniApp))))

	// @summary  Đổi App ID Mini App riêng của xã: gắn App ID mới và xoá mềm App ID ở đường dẫn, một giao dịch (bắt buộc lý do); App ID cũ không dùng lại được; sau đó tự thu hồi khoá bí mật của App ID cũ
	// @request  miniAppReplacementBody
	// @reply    201 miniAppChangeView
	// @reply    400 httpx.Error invalid_body
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error commune_not_found mini_app_not_found
	// @reply    409 httpx.Error mini_app_taken mini_app_removed mini_app_inactive commune_inactive
	// @reply    422 httpx.Error invalid_app_id invalid_reason
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("POST /api/v1/communes/{id}/mini-apps/{app_id}/replacement",
		opauth.RequireKey(h.d.Auth, opauth.KeyMiniAppManage)(
			idem.KhongCan("a repeat is refused: the old App ID is now soft-deleted (mini_app_not_found) and the new one has a row (mini_app_taken)")(
				http.HandlerFunc(h.replaceMiniApp))))

	// @summary  Gỡ một Mini App riêng khỏi xã — xoá mềm, bắt buộc lý do, chỉ nhận active=false (bật lại đã bỏ 05/10/2026); App ID đã gỡ không dùng lại được; sau đó tự thu hồi khoá bí mật của App ID
	// @request  miniAppActivationBody
	// @reply    200 miniAppChangeView
	// @reply    400 httpx.Error invalid_body
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error commune_not_found mini_app_not_found
	// @reply    409 httpx.Error commune_inactive
	// @reply    422 httpx.Error invalid_reason mini_app_reactivation_removed
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("PUT /api/v1/communes/{id}/mini-apps/{app_id}/activation",
		opauth.RequireKey(h.d.Auth, opauth.KeyMiniAppManage)(
			idem.KhongCan("removing an App ID already removed from this commune changes nothing and writes no entry; the repeat only retries the secret retirement")(
				http.HandlerFunc(h.removeMiniApp))))

	// @summary  Đánh dấu / bỏ đánh dấu Mini App riêng của xã ĐÃ PHÁT HÀNH trên Zalo (bắt buộc lý do; ADR 0070 bổ sung 13/10/2026): QR của xã chỉ dùng app riêng khi đã đánh dấu, ngược lại dùng app chung x=; vết danh_dau_phat_hanh_mini_app / bo_danh_dau_phat_hanh_mini_app ở audit_log của xã, cùng giao dịch; đặt đúng trạng thái đang có thì không ghi gì
	// @request  miniAppPublicationBody
	// @reply    200 communeDetailView
	// @reply    400 httpx.Error invalid_body
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error commune_not_found mini_app_not_found
	// @reply    409 httpx.Error commune_inactive mini_app_inactive
	// @reply    422 httpx.Error invalid_reason
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("PUT /api/v1/communes/{id}/mini-apps/{app_id}/publication",
		opauth.RequireKey(h.d.Auth, opauth.KeyMiniAppManage)(
			idem.KhongCan("setting the mark it already has changes nothing and writes no entry")(
				http.HandlerFunc(h.setMiniAppPublication))))

	// THE TWO SECRET ROUTES below forward a commune's Zalo app secret to identity and keep nothing
	// (operator_mini_app_secrets.go lists every place the value goes). KhongCan is also what keeps
	// the body out of any idempotency store — and core/idem never stores a body in any mode.

	// @summary  Đặt khoá bí mật Zalo cho một App ID Mini App riêng của xã (bắt buộc lý do); chuyển tiếp sang identity, không lưu ở đây
	// @request  miniAppSecretBody
	// @reply    200 miniAppSecretView
	// @reply    400 httpx.Error invalid_body
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error commune_not_found mini_app_not_found
	// @reply    409 httpx.Error mini_app_not_bound
	// @reply    422 httpx.Error invalid_secret invalid_reason
	// @reply    503 httpx.Error operator_auth_unavailable mini_app_secret_unavailable
	mux.Handle("PUT /api/v1/communes/{id}/mini-apps/{app_id}/secret",
		opauth.RequireKey(h.d.Auth, opauth.KeyMiniAppManage)(
			idem.KhongCan("a retry after a lost response writes one more version holding the same secret — two true entries, never a wrong state (operator.proto); and no store may hold this body")(
				http.HandlerFunc(h.setMiniAppSecret))))

	// @summary  Thu hồi khoá bí mật Zalo của một App ID Mini App riêng của xã (bắt buộc lý do); không còn khoá thì trả 200 retired=false
	// @request  miniAppSecretRetireBody
	// @reply    200 miniAppSecretRetirementView
	// @reply    400 httpx.Error invalid_body
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error commune_not_found mini_app_not_found
	// @reply    422 httpx.Error invalid_reason
	// @reply    503 httpx.Error operator_auth_unavailable mini_app_secret_unavailable
	mux.Handle("DELETE /api/v1/communes/{id}/mini-apps/{app_id}/secret",
		opauth.RequireKey(h.d.Auth, opauth.KeyMiniAppManage)(
			idem.KhongCan("retiring what is already retired is NOT_FOUND in identity, answered as the same end state with nothing written")(
				http.HandlerFunc(h.retireMiniAppSecret))))

	// --- the commune's default map frame (ADR 0072 amendment 2, K1–K2) -----------------------------

	// @summary  Khung bản đồ mặc định của một xã (tâm, bán kính, khung giới hạn [kinh độ nhỏ, vĩ độ nhỏ, kinh độ lớn, vĩ độ lớn]); chưa đặt thì configured=false; kèm bán kính khuyến nghị, khoảng thường dùng, trần
	// @reply    200 mapFrameDefaultView
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error commune_not_found
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("GET /api/v1/communes/{id}/map-frame-default",
		opauth.RequireKey(h.d.Auth, opauth.AnyKey)(
			http.HandlerFunc(h.getMapFrameDefault)))

	// @summary  Đặt / đổi khung bản đồ mặc định của một xã (bắt buộc lý do); bán kính ngoài 3–20 km phải kèm acknowledged_unusual=true; vết dat_khung_ban_do_mac_dinh ở audit_log của xã, cùng giao dịch
	// @request  mapFrameDefaultBody
	// @reply    200 mapFrameDefaultView
	// @reply    400 httpx.Error invalid_body
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error commune_not_found
	// @reply    409 httpx.Error commune_inactive
	// @reply    422 httpx.Error center_outside_mainland radius_out_of_range radius_unusual_unconfirmed invalid_reason
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("PUT /api/v1/communes/{id}/map-frame-default",
		opauth.RequireKey(h.d.Auth, opauth.KeyTenantManage)(
			idem.KhongCan("setting the values it already has changes nothing and writes no entry")(
				http.HandlerFunc(h.setMapFrameDefault))))

	// --- upload limits (ADR 0073 #5, ADR 0052 §10) -------------------------------------------------

	// @summary  Giới hạn tải lên của mọi mục đích (áp chung mọi xã), kèm kiểu tệp và mức trần được phép đặt
	// @reply    200 uploadPolicyListView
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("GET /api/v1/upload-policies",
		opauth.RequireKey(h.d.Auth, opauth.AnyKey)(
			http.HandlerFunc(h.listUploadPolicies)))

	// @summary  Sửa giới hạn tải lên của một mục đích (bắt buộc lý do); vết platform_audit_log cùng giao dịch; nơi đọc thấy sau tối đa 60 giây
	// @request  uploadPolicyBody
	// @reply    200 uploadPolicyView
	// @reply    400 httpx.Error invalid_body
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error upload_policy_not_found
	// @reply    422 httpx.Error invalid_max_bytes invalid_mime_types invalid_max_files invalid_reason
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("PUT /api/v1/upload-policies/{purpose}",
		opauth.RequireKey(h.d.Auth, opauth.KeyUploadPolicyManage)(
			idem.KhongCan("setting the values it already has changes nothing and writes no entry")(
				http.HandlerFunc(h.changeUploadPolicy))))

	// --- the operator log (ADR 0073 #2) -----------------------------------------------------------

	// @summary  Nhật ký vận hành: thao tác của người vận hành trên mọi xã + thay đổi cấu hình cấp nền tảng, mới nhất trước; lượt đọc được ghi vết
	// @reply    200 operatorAuditPageView
	// @reply    400 httpx.Error invalid_range invalid_cursor invalid_limit
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("GET /api/v1/operator-audit-entries",
		opauth.RequireKey(h.d.Auth, opauth.AnyKey)(
			http.HandlerFunc(h.listOperatorLog)))

	// @summary  Nhật ký vận hành của một xã, mới nhất trước; lượt đọc được ghi vết
	// @reply    200 operatorAuditPageView
	// @reply    400 httpx.Error invalid_range invalid_cursor invalid_limit
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error commune_not_found
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("GET /api/v1/communes/{id}/operator-audit-entries",
		opauth.RequireKey(h.d.Auth, opauth.AnyKey)(
			http.HandlerFunc(h.listCommuneOperatorLog)))

	// --- the shared Mini App and the commune QR link (owner 04/10/2026, ADR 0073 #5) -----------------

	// @summary  Mini App dùng chung của nền tảng (dòng mini_app chế độ chinh đang chạy)
	// @reply    200 sharedMiniAppView
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error shared_mini_app_not_declared
	// @reply    409 httpx.Error shared_mini_app_ambiguous
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("GET /api/v1/shared-mini-app",
		opauth.RequireKey(h.d.Auth, opauth.AnyKey)(
			http.HandlerFunc(h.getSharedMiniApp)))

	// @summary  Khai báo / đổi App ID của Mini App dùng chung (bắt buộc lý do): gắn dòng chinh mới và tắt dòng cũ, một giao dịch; vết shared_mini_app.changed ở platform_audit_log
	// @request  sharedMiniAppBody
	// @reply    200 sharedMiniAppView
	// @reply    400 httpx.Error invalid_body
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    409 httpx.Error mini_app_taken shared_mini_app_ambiguous
	// @reply    422 httpx.Error invalid_app_id invalid_reason
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("PUT /api/v1/shared-mini-app",
		opauth.RequireKey(h.d.Auth, opauth.KeyMiniAppManage)(
			idem.KhongCan("declaring the App ID that is already the running shared app changes nothing and writes no entry")(
				http.HandlerFunc(h.declareSharedMiniApp))))

	// @summary  Liên kết mở Mini App DUY NHẤT của xã để in QR (chủ dự án 13/10/2026), cùng quy tắc với GET /api/v1/commune-launch-link của cán bộ xã: app riêng đang chạy nếu xã có đúng một (source=rieng, không tham số xã), ngược lại app dùng chung với mã mở app x= (source=chung); bỏ dạng d=<tên miền>; links có tối đa một phần tử, unavailable nêu app riêng mơ hồ khi đã dùng app chung thay; chỉ từ chối khi không tạo được liên kết nào; không ghi vết (ADR 0048 §30/09 #9)
	// @reply    200 launchLinksView
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error commune_not_found
	// @reply    409 httpx.Error commune_inactive commune_launch_code_invalid shared_mini_app_not_declared shared_mini_app_ambiguous own_mini_app_ambiguous
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("GET /api/v1/communes/{id}/mini-app-launch-link",
		opauth.RequireKey(h.d.Auth, opauth.KeyQRIssue)(
			http.HandlerFunc(h.getLaunchLink)))

	// --- tier-1 petition field codes (ADR 0073 #3, ADR 0060) -----------------------------------------

	// @summary  Bộ mã lĩnh vực phản ánh cấp 1 (mọi mã, kể cả mã đã ngừng dùng) và các tông màu được phép
	// @reply    200 petitionFieldListView
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("GET /api/v1/petition-fields",
		opauth.RequireKey(h.d.Auth, opauth.AnyKey)(
			http.HandlerFunc(h.listPetitionFields)))

	// @summary  Cấp một mã lĩnh vực cấp 1 mới (bắt buộc lý do); mã không bao giờ đổi, không cấp lại; vết petition_field.created
	// @request  createPetitionFieldBody
	// @reply    201 petitionFieldView
	// @reply    400 httpx.Error invalid_body
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    409 httpx.Error petition_field_code_taken
	// @reply    422 httpx.Error invalid_code invalid_label invalid_sort_order invalid_icon invalid_tone invalid_reason
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("POST /api/v1/petition-fields",
		opauth.RequireKey(h.d.Auth, opauth.KeyPetitionFieldManage)(
			idem.KhongCan("a repeat is refused by petition_field.code, the primary key, which keeps every row")(
				http.HandlerFunc(h.createPetitionField))))

	// @summary  Sửa nhãn mặc định, thứ tự, biểu tượng, tông màu của một mã cấp 1 (bắt buộc lý do); không đổi mã; vết petition_field.changed
	// @request  editPetitionFieldBody
	// @reply    200 petitionFieldView
	// @reply    400 httpx.Error invalid_body
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error petition_field_not_found
	// @reply    422 httpx.Error invalid_label invalid_sort_order invalid_icon invalid_tone invalid_reason
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("PUT /api/v1/petition-fields/{code}",
		opauth.RequireKey(h.d.Auth, opauth.KeyPetitionFieldManage)(
			idem.KhongCan("setting the values it already has changes nothing and writes no entry")(
				http.HandlerFunc(h.editPetitionField))))

	// @summary  Ngừng dùng / dùng lại một mã cấp 1 ở mọi xã (bắt buộc lý do); phiếu cũ giữ nhãn; vết petition_field.deactivated / reactivated
	// @request  petitionFieldActivationBody
	// @reply    200 petitionFieldView
	// @reply    400 httpx.Error invalid_body
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error petition_field_not_found
	// @reply    422 httpx.Error invalid_reason
	// @reply    503 httpx.Error operator_auth_unavailable
	mux.Handle("PUT /api/v1/petition-fields/{code}/activation",
		opauth.RequireKey(h.d.Auth, opauth.KeyPetitionFieldManage)(
			idem.KhongCan("setting the state it already has changes nothing and writes no entry")(
				http.HandlerFunc(h.setPetitionFieldActivation))))

	// --- the shared platform Zalo Bot (ADR 0074 §"Tài nguyên URL") ----------------------------------
	//
	// EVERY route, reads included, takes ops.zalo_bot.manage (owner 05/10/2026): the screen shows
	// whether a token is set and by whom, and every commune's uptake. A relay to comms'
	// ZaloBotOperatorService, which trusts THIS check (ADR 0074 #3) and writes the trail of each act;
	// operator_zalo_bot.go lists every place the bot token goes. No reason on any write (owner
	// 11/10/2026). KhongCan on the writes also keeps the token's body out of any idempotency store.

	// @summary  Bot Zalo dùng chung: đã đặt token chưa, đặt lúc … bởi …, tên bot, liên kết trò chuyện, lần kiểm gần nhất — không bao giờ trả token
	// @reply    200 sharedZaloBotView
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    503 httpx.Error operator_auth_unavailable zalo_bot_unavailable zalo_bot_key_not_configured
	mux.Handle("GET /api/v1/zalo-bots/shared",
		opauth.RequireKey(h.d.Auth, opauth.KeyZaloBotManage)(
			http.HandlerFunc(h.getSharedZaloBot)))

	// The two refusal codes the screen keys on are kebab-case BY THE SCREEN CONTRACT (platform-admin
	// api.ts errorFrom: `can-xac-nhan-ghep-lai`, `token-khong-dung-duoc`), which tools/apidoc's
	// snake_case rule for @reply codes cannot express — they are named in the summary, not as codes.
	// The 422 token refusal is zaloBotTokenRefusedView (httpx.Error + `outcome`); apidoc takes one body
	// type per status, so the line below declares the field refusals' httpx.Error. These routes stay
	// out of openapi.json anyway (opauth).
	//
	// @summary  Đặt token bot Zalo dùng chung kèm tên bot và liên kết trò chuyện; comms hỏi Zalo getMe trước khi lưu; đổi sang tài khoản bot khác thì 409 can-xac-nhan-ghep-lai kèm live_link_count; Zalo không nhận token thì 422 token-khong-dung-duoc kèm outcome
	// @request  sharedZaloBotBody
	// @reply    200 sharedZaloBotView
	// @reply    400 httpx.Error invalid_body
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    409 zaloBotRelinkView
	// @reply    422 httpx.Error invalid_token invalid_bot_name invalid_chat_url invalid_expected_relink_count invalid_request
	// @reply    503 httpx.Error operator_auth_unavailable zalo_bot_unavailable zalo_bot_key_not_configured
	mux.Handle("PUT /api/v1/zalo-bots/shared",
		opauth.RequireKey(h.d.Auth, opauth.KeyZaloBotManage)(
			idem.KhongCan("a retry after a lost response finds the same bot account stored, keeps the links and writes the token again — two true trail entries (zalo_bot_operator.proto); and no store may hold this body")(
				http.HandlerFunc(h.setSharedZaloBot))))

	// @summary  Kiểm token bot Zalo dùng chung: comms gọi Zalo getMe một lần, ghi lại làm lần kiểm gần nhất (không có thân yêu cầu)
	// @reply    200 zaloBotCheckResultView
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    503 httpx.Error operator_auth_unavailable zalo_bot_unavailable zalo_bot_key_not_configured
	mux.Handle("POST /api/v1/zalo-bots/shared/check",
		opauth.RequireKey(h.d.Auth, opauth.KeyZaloBotManage)(
			idem.KhongCan("getMe changes nothing at Zalo; a repeat records one more true check (zalo_bot_operator.proto)")(
				http.HandlerFunc(h.checkSharedZaloBot))))

	// @summary  Webhook của bot Zalo dùng chung theo Zalo (getWebhookInfo), so với địa chỉ comms sẽ đặt; khoá webhook đặt lúc … bởi … — không bao giờ trả khoá
	// @reply    200 zaloBotWebhookView
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    503 httpx.Error operator_auth_unavailable zalo_bot_unavailable zalo_bot_key_not_configured
	mux.Handle("GET /api/v1/zalo-bots/shared/webhook",
		opauth.RequireKey(h.d.Auth, opauth.KeyZaloBotManage)(
			http.HandlerFunc(h.getSharedZaloBotWebhook)))

	// @summary  Trỏ webhook bot Zalo dùng chung về hệ thống: comms tự dựng địa chỉ và khoá mới (không có thân yêu cầu)
	// @reply    200 zaloBotWebhookSetView
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    503 httpx.Error operator_auth_unavailable zalo_bot_unavailable zalo_bot_key_not_configured
	mux.Handle("PUT /api/v1/zalo-bots/shared/webhook",
		opauth.RequireKey(h.d.Auth, opauth.KeyZaloBotManage)(
			idem.KhongCan("each call rotates the webhook secret once more; safe to retry by contract (zalo_bot_operator.proto)")(
				http.HandlerFunc(h.setSharedZaloBotWebhook))))

	// @summary  Mọi xã: bật kênh Zalo chưa, dùng bot chung hay bot riêng (tên bot riêng), số cán bộ đã ghép với bot đang dùng / tổng cán bộ đang hoạt động; chỉ số đếm; lượt đọc được comms và identity ghi vết
	// @reply    200 zaloBotCommuneListView
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    500 httpx.Error internal
	// @reply    503 httpx.Error operator_auth_unavailable staff_counts_unavailable zalo_bot_unavailable zalo_bot_key_not_configured
	mux.Handle("GET /api/v1/zalo-bots/shared/communes",
		opauth.RequireKey(h.d.Auth, opauth.KeyZaloBotManage)(
			http.HandlerFunc(h.listZaloBotCommunes)))
}
