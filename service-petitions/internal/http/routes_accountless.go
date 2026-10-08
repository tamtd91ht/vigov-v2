package http

// Routes of the ACCOUNTLESS petition surface — ADR 0083, TEMPORARY. Handlers and the reasoning are in
// accountless.go. REMOVE this file with it (ADR 0083 §Gỡ bỏ).
//
// A THIRD Register INTO A THIRD MUX, behind its own chain (cmd/server: CORS for the Mini App origin,
// header stripping, recovery, idem's store) — no TenantMiddleware (the Mini App calls the reserved API
// host, which maps to no commune), no CitizenEdge (there is no session). The commune is resolved per
// request from the QR's domain, by the platform, in the handler (ADR 0083 row 2).
//
// `public-` PREFIX: the existing `my-` resources are the session ones (ubiquitous-language §Tiền tố
// `my-`); these are reachable by anybody holding a commune's domain, and the name must say so.

import (
	"net/http"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
)

// AccountlessReportsPath and AccountlessFieldsPath are the paths cmd/server must route to the public
// chain — the collection, its `{code}` subtree, and the catalogue. The routes below spell them as
// LITERALS because tools/apidoc reads patterns from the source; cmd/server's
// TestAccountlessRoutesRideThePublicChain requests THESE constants through the real chain, so a drift
// between the two spellings turns that test red.
const (
	AccountlessReportsPath = "/api/v1/public-citizen-reports"
	AccountlessFieldsPath  = "/api/v1/public-citizen-report-fields"
)

// RegisterAccountless mounts the accountless routes onto their OWN mux.
func RegisterAccountless(mux *http.ServeMux, d DepsAccountless) {
	switch {
	case d.Communes == nil:
		panic("petitions/http: thiếu đường tra xã theo tên miền — ba tuyến không tài khoản sẽ panic")
	case d.Intake == nil:
		panic("petitions/http: thiếu use case tiếp nhận — POST /api/v1/public-citizen-reports sẽ panic")
	case d.Petitions == nil:
		panic("petitions/http: thiếu kho đọc phiếu không tài khoản — tra cứu công khai sẽ panic")
	case d.Fields == nil:
		panic("petitions/http: thiếu danh mục lĩnh vực — GET /api/v1/public-citizen-report-fields sẽ panic")
	case d.SendLimiter == nil || d.LookupLimiter == nil || d.FieldsLimiter == nil:
		// Rule 13, invariant 7: an unauthenticated route without its rate limit is refused at startup.
		panic("petitions/http: thiếu bộ giới hạn tần suất của tuyến không tài khoản (ratelimit.Accountless*)")
	}
	h := newHandlerAccountless(d)

	// --- the fields the commune's form offers, by the QR's domain (ADR 0083 row 8) ----------------
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Lĩnh vực xã đang mở cho người dân chọn khi gửi phản ánh KHÔNG TÀI KHOẢN, theo tên miền xã trên QR — đường tạm ADR 0083
	// @screen   09-phan-anh-nguoi-dan §13
	// @consumer citizen-app
	// 200 is the SAME body as GET /api/v1/my-citizen-report-fields: the commune's offered fields, in its
	// order; `items: []` when it switched every field off.
	//
	// 400 `invalid_host`: `host` missing, repeated or malformed — the platform is not asked.
	//
	// 404 `commune_not_found`: no ACTIVE commune holds the domain (unknown, reserved or inactive).
	//
	// 429 `rate_limited` (+ Retry-After): ratelimit.AccountlessFieldRead per (host, network) —
	// PROVISIONAL figure, not the owner's. 503 `rate_limit_unavailable` when Redis cannot be asked (fails
	// closed), `platform_unavailable` when the registry cannot, `field_catalogue_unavailable` (ADR 0060 §3).
	//
	// @reply    200 citizenFieldListOut
	// @reply    400 httpx.Error invalid_host
	// @reply    404 httpx.Error commune_not_found
	// @reply    429 httpx.Error rate_limited
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/public-citizen-report-fields",
		authz.Public("ADR 0083 #8 (đường tạm khi App ViHAT chưa được Zalo duyệt): danh mục lĩnh vực là cấu hình xã công bố cho mọi người dân, không chứa hồ sơ của ai; xã do máy chủ tra từ tên miền trên QR, chỉ xã đang hoạt động; giới hạn tần suất theo tên miền và mạng của máy gọi, Redis hỏng thì từ chối")(
			http.HandlerFunc(h.ListAccountlessFields)))

	// --- send one petition with no account (ADR 0083 rows 1-3, 5, 7, 8, 10-12) ---------------------
	//
	// THE ONE PUBLIC WRITE IN THIS SYSTEM, and temporary. Body: the session body (POST
	// /api/v1/my-citizen-reports) with the same names and the same refuse-list, plus `host`, the domain
	// on the commune's QR. No photos. Stored on the Mini App channel with NO owner — neither a citizen
	// nor a Zalo account; trail actor Kind `anonymous`, IP from this socket.
	//
	// idem.RequiredAccountless(idem.DongKhiHong) — THE ONE ROUTE WITHOUT A PRINCIPAL THAT HAS DUPLICATE
	// PROTECTION: the key space is (commune, Idempotency-Key), the key a 128-bit random value per compose
	// (core/idem says why that is safe). DongKhiHong because the rate limit fails closed on the same Redis
	// anyway (ADR 0083 row 12), and there is no second layer: two sends are two rows with two codes.
	//
	// @summary  Gửi một phiếu phản ánh KHÔNG TÀI KHOẢN (không phiên, không tài khoản Zalo) tới xã theo tên miền trên QR — trả MÃ TRA CỨU; đường tạm ADR 0083
	// @screen   09-phan-anh-nguoi-dan §13
	// @consumer citizen-app
	// @request  accountlessReportIn
	// 201 is `{code, status, acknowledge_due, resolve_due}` — the names and formats of phieuCuaToiRa. A
	// retry with the SAME key after the first finished answers the same 201 shape again (header
	// `Idempotent-Replay: true`), read back from the database, and creates nothing.
	//
	// 400: not JSON or over 64 KiB · `invalid_host` · a field the client does not decide (người gửi,
	// kênh, mã, trạng thái, hạn, `linh_vuc`) · `lat`/`lng` (no scene location on this path) · an empty
	// or over-long box ·
	// `field_not_offered` · `missing_idempotency_key` / `invalid_idempotency_key` (needs 32 hex digits
	// or 22 base64url characters). Nothing written.
	//
	// 404 `commune_not_found`: no ACTIVE commune holds the domain. Nothing written.
	//
	// 409 `request_in_progress`: the same key is still being processed.
	//
	// 429 `rate_limited` (+ Retry-After): 5 sends per hour per (domain, network) — ADR 0083 row 3.
	// 429 `commune_daily_limit`: the commune already received 200 accountless petitions today (from
	// 00:00 Viet Nam time, soft-deleted included). Both sentences point to the commune's reception desk.
	//
	// 503: `rate_limit_unavailable` / `idempotency_unavailable` (Redis), `platform_unavailable`
	// (registry), `intake_not_configured` (the commune has no deadline configuration),
	// `field_catalogue_unavailable`. Nothing written, no code issued.
	//
	// @reply    201 accountlessReceiptOut
	// @reply    400 httpx.Error
	// @reply    404 httpx.Error commune_not_found
	// @reply    409 httpx.Error request_in_progress
	// @reply    429 httpx.Error rate_limited commune_daily_limit
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error platform_unavailable intake_not_configured
	mux.Handle("POST /api/v1/public-citizen-reports",
		authz.Public("ADR 0083 #1-#3 (đường tạm khi App ViHAT chưa được Zalo duyệt, getAccessToken trả -1401): người dân gửi phản ánh không cần phiên; xã do máy chủ tra từ tên miền trên QR, chỉ xã đang hoạt động; 5 phiếu/giờ theo tên miền và mạng của máy gọi, 200 phiếu không tài khoản/ngày/xã; Redis hỏng thì từ chối")(
			h.withBodyCommune(
				idem.RequiredAccountless(idem.DongKhiHong, h.replayReceipt)(
					http.HandlerFunc(h.SendAccountless)))))

	// --- the public lookup by code (ADR 0083 rows 4, 9, 12) --------------------------------------
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Tra cứu công khai tình trạng một phiếu phản ánh KHÔNG TÀI KHOẢN theo mã tra cứu và tên miền xã — chỉ tình trạng, hạn, kết quả, lý do; đường tạm ADR 0083
	// @screen   09-phan-anh-nguoi-dan §8
	// @consumer citizen-app
	// 200 is `{code, status, acknowledge_due, resolve_due, result, reason?}` — nothing else: no name,
	// phone, content, address or location. `result` is "" until closed; `reason` only on
	// `khong-tiep-nhan` / `chuyen-cap-tren`.
	//
	// 404 `not_found`, BYTE-IDENTICAL for: a malformed, unknown or inactive domain · no such code · a code
	// of another commune · a citizen's or a Zalo account's petition (only accountless petitions are
	// readable here) · a soft-deleted petition (rule 4, forbidden #2).
	//
	// 429 `rate_limited` (+ Retry-After): 30 lookups per hour per network, counted before anything else
	// (ADR 0083 row 9). 503 `rate_limit_unavailable` (fails closed) or `platform_unavailable`.
	//
	// @reply    200 accountlessLookupOut
	// @reply    404 httpx.Error not_found
	// @reply    429 httpx.Error rate_limited
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/public-citizen-reports/{maTraCuu}",
		authz.Public("ADR 0083 #4 và #9 (đường tạm khi App ViHAT chưa được Zalo duyệt): người cầm mã tra cứu xem tình trạng phiếu không tài khoản của chính mình; chỉ phiếu không tài khoản của xã theo tên miền, chỉ tình trạng/hạn/kết quả/lý do, không dữ liệu cá nhân; mã sai và mã xã khác cùng một 404; 30 lần/giờ theo mạng của máy gọi, Redis hỏng thì từ chối")(
			http.HandlerFunc(h.LookupAccountless)))
}
