package http

// Routes for the platform service — the COMMUNE-HOST surface. The operator area has its own handler
// and its own declarations (operator_routes.go, OPERATOR_HOST only).
//
// EVERY route declares its permission explicitly. The global guard only rejects users who are
// not signed in; it does not check permissions. A route with no declaration is callable by
// every staff role, and nothing reports it — see rule 5.
//
// Four declarations are available, and there is no fifth:
//
//	authz.RequirePermission(checker, "admin.org")           // the normal case
//	authz.CitizenOnly()                                     // citizen paths, isolated by identity
//	authz.AnyAuthenticated("<why any account needs this>")  // reason mandatory
//	authz.Public("<why this is public>")                    // reason mandatory
//
// EVERY route also carries an @-annotation block IMMEDIATELY above the statement — no blank line
// between. `tools/apidoc` reads it and generates kb/20-contracts/openapi.json (ADR 0014). The
// permission and the idempotency mode are NOT annotated: apidoc reads them from the authz.* / idem.*
// calls, so there is no second copy to drift (rule 9).

import (
	"log/slog"
	"net/http"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
)

// Deps are everything the routes need. Kept explicit so wiring stays in cmd/server.
type Deps struct {
	Checker authz.Checker
	// Branding is the commune identity-image use case (ADR 0069). Required.
	Branding BrandingActs
	// URLs turns a public key into its anonymous URL (app.Branding.PublicURL). Required.
	URLs interface{ PublicURL(key string) string }
	Log  *slog.Logger
}

// Register mounts the platform routes of the commune host.
//
// It PANICS on a missing dependency, at startup: a nil Checker would make RequirePermission meet a nil
// interface on the first request, and a nil use case a nil-pointer 500 nobody can explain.
func Register(mux *http.ServeMux, d Deps) {
	if d.Checker == nil || d.Branding == nil || d.URLs == nil || d.Log == nil {
		panic("platform/http: Register thiếu phụ thuộc — Checker, Branding, URLs và Log đều bắt buộc")
	}
	h := &brandingHandlers{acts: d.Branding, urls: d.URLs, log: d.Log}

	// --- the commune's identity images: logo and web-admin banner (ADR 0069) -----------------------------
	//
	// `admin.org` ON EVERY ROUTE, read and write: ADR 0069 #2 names it ("Quyền admin.org (khoá đã có
	// trong quyen)"), and it is seeded (service-identity/migrations/0001_init.sql:282). NO KEY WAS
	// INVENTED (rule 5, invariant 3c). The read is guarded too: it is the settings tab, and every other
	// surface reads the same two URLs without a session through the public commune routes (identity,
	// ADR 0069 #8) — so guarding this one costs nobody anything.
	//
	// `commune-branding` IS ITS OWN FIRST SEGMENT because tools/ingress routes by the first segment after
	// /api/v1/, and `communes` / `commune-profiles` already belong to identity. The noun is the session's
	// choice, NOT YET CONFIRMED BY THE USER — kb/00-foundation/ubiquitous-language.md has no row for it.
	//
	// THE COMMUNE COMES FROM Host (rule 1, invariant 3) and the file's subject IS that commune
	// (migration 0017): there is no id on the wire that names a profile.
	//
	// @summary  Xem logo và banner web-admin hiện tại của xã (tab Cấu hình › Nhận diện xã)
	// @screen   *(chưa có đặc tả — ADR 0069, Cấu hình › Nhận diện xã)*
	// @reply    200 brandingSettingsOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/commune-branding",
		authz.RequirePermission(d.Checker, "admin.org")(
			http.HandlerFunc(h.settings)))

	// XIN TẢI LOGO — ADR 0052 §1a: a pending row and a presigned POST into the temp bucket (15 minutes).
	// The limits are platform's own `tenant-logo` policy (2 MB, PNG/WebP/JPEG, migration 0016). A
	// soft-deleted display profile refuses here (409), before anybody uploads.
	//
	// idem.Required(idem.MoKhiHong): a double submit issues a second pending row — one unused upload slot
	// for 15 minutes, never a second published logo. A cache outage must not stop an administrator.
	//
	// @summary  Xin tải logo xã — trả biểu mẫu tải thẳng lên kho lưu tệp (15 phút)
	// @screen   *(chưa có đặc tả — ADR 0069, Cấu hình › Nhận diện xã)*
	// @request  brandingUploadIn
	// @reply    201 brandingUploadOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/commune-branding/logo-uploads",
		authz.RequirePermission(d.Checker, "admin.org")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.requestLogoUpload))))

	// HOÀN TẤT TẢI LOGO — stat · sniff · the CURRENT policy · ClamAV · sha256 · promote to the private
	// bucket · decode, orient, normalise to a 512 px square PNG KEEPING TRANSPARENCY · publish · point the
	// profile at it · audit, in ONE transaction (saving is publishing, ADR 0069 #3). The previous logo's
	// public copy is withdrawn after commit. Only the officer the upload was issued to; anybody else's id
	// answers 404. Infected, wrong type, too large, undecodable, too many pixels → 422. Scanner, limits or
	// object store down → 503, nothing written, retryable, NEVER stored unscanned (ADR 0052 §9).
	//
	// idem.KhongCan: a second completion of a ready file answers that file and writes nothing; two in
	// flight serialise on the row lock and the loser lands on the winner's row.
	//
	// @summary  Hoàn tất tải logo xã — quét mã độc, chuẩn hoá PNG vuông 512px giữ nền trong, đăng và đặt làm logo hiện tại
	// @screen   *(chưa có đặc tả — ADR 0069, Cấu hình › Nhận diện xã)*
	// @reply    200 brandingFileOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    422 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/commune-branding/logo-uploads/{id}/completion",
		authz.RequirePermission(d.Checker, "admin.org")(
			idem.KhongCan("hoàn tất lần hai trên ảnh đã sẵn sàng trả lại đúng ảnh ấy và không ghi gì; hai lượt cùng lúc tuần tự hoá trên khoá dòng")(
				http.HandlerFunc(h.completeLogoUpload))))

	// GỠ LOGO — the profile column set NULL + audit in one transaction; then the public copy is withdrawn
	// and the file row SOFT-deleted (rule 7). The sidebar falls back to the building icon (ADR 0069 #7).
	// DELETE because the resource — "the commune's current logo" — is gone from every read path.
	//
	// idem.KhongCan: removing an image that is not set writes nothing and answers 204 again.
	//
	// @summary  Gỡ logo xã — thanh bên và màn đăng nhập quay về biểu tượng toà nhà
	// @screen   *(chưa có đặc tả — ADR 0069, Cấu hình › Nhận diện xã)*
	// @reply    204 -
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/commune-branding/logo",
		authz.RequirePermission(d.Checker, "admin.org")(
			idem.KhongCan("gỡ ảnh chưa đặt không ghi gì và trả 204 như lần trước; dòng hồ sơ được khoá nên hai lượt gỡ cùng lúc chỉ ghi một vết")(
				http.HandlerFunc(h.removeLogo))))

	// XIN TẢI BANNER WEB-ADMIN — the same three steps as the logo, purpose `tenant-banner` (2 MB,
	// PNG/WebP/JPEG). NOT the Mini App banner (comms, ADR 0069 #6).
	//
	// @summary  Xin tải banner web-admin của xã — trả biểu mẫu tải thẳng lên kho lưu tệp (15 phút)
	// @screen   *(chưa có đặc tả — ADR 0069, Cấu hình › Nhận diện xã)*
	// @request  brandingUploadIn
	// @reply    201 brandingUploadOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/commune-branding/banner-uploads",
		authz.RequirePermission(d.Checker, "admin.org")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.requestBannerUpload))))

	// HOÀN TẤT TẢI BANNER — as the logo's completion; the derivative is a JPEG exactly 1600 px wide
	// (aspect kept, flattened onto white; never taller than 1600 px).
	//
	// @summary  Hoàn tất tải banner web-admin — quét mã độc, chuẩn hoá rộng 1600px, đăng và đặt làm banner hiện tại
	// @screen   *(chưa có đặc tả — ADR 0069, Cấu hình › Nhận diện xã)*
	// @reply    200 brandingFileOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    422 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/commune-branding/banner-uploads/{id}/completion",
		authz.RequirePermission(d.Checker, "admin.org")(
			idem.KhongCan("hoàn tất lần hai trên ảnh đã sẵn sàng trả lại đúng ảnh ấy và không ghi gì; hai lượt cùng lúc tuần tự hoá trên khoá dòng")(
				http.HandlerFunc(h.completeBannerUpload))))

	// GỠ BANNER — as removing the logo; web-admin then draws no strip (ADR 0069 #7).
	//
	// @summary  Gỡ banner web-admin của xã — không còn dải banner dưới thanh trên cùng
	// @screen   *(chưa có đặc tả — ADR 0069, Cấu hình › Nhận diện xã)*
	// @reply    204 -
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/commune-branding/banner",
		authz.RequirePermission(d.Checker, "admin.org")(
			idem.KhongCan("gỡ ảnh chưa đặt không ghi gì và trả 204 như lần trước; dòng hồ sơ được khoá nên hai lượt gỡ cùng lúc chỉ ghi một vết")(
				http.HandlerFunc(h.removeBanner))))
}
