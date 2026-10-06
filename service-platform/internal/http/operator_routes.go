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
	// Policies is upload_policy (ADR 0073 #5); OperatorLog the operator log (ADR 0073 #2).
	Policies    UploadPolicyEditor
	OperatorLog OperatorLogReader
	// SharedApp is the shared Mini App + QR link source (owner 04/10/2026); Fields the tier-1 petition
	// field codes (ADR 0073 #3).
	SharedApp SharedMiniAppEditor
	Fields    PetitionFieldEditor
	// MapFrames is a commune's default map frame (ADR 0072 amendment 2, K1).
	MapFrames MapFrameDefaultEditor
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

	// @summary  Mọi liên kết mở Mini App của xã để in QR, người vận hành chọn mã nào in (chủ đầu tư 06/10/2026): app dùng chung với tên miền chính (source=chung) trước, app riêng đang sống (source=rieng, không d=) sau; liên kết không tạo được nằm ở unavailable kèm mã lỗi; chỉ từ chối khi không có liên kết nào; không ghi vết (ADR 0048 §30/09 #9)
	// @reply    200 launchLinksView
	// @reply    401 httpx.Error unauthorized
	// @reply    403 httpx.Error forbidden
	// @reply    404 httpx.Error commune_not_found
	// @reply    409 httpx.Error commune_inactive commune_no_primary_domain shared_mini_app_not_declared shared_mini_app_ambiguous own_mini_app_ambiguous
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
}
