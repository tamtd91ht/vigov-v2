package http

// The portal sync routes — §3 of docs/ui-ux/11-noi-dung-mini-app.md (`🔄 Đồng bộ từ Cổng TTĐT`,
// `Cấu hình`, `⟳ Đồng bộ ngay`), ADR 0067 §2, migration 0013. Called from Register; in their own file
// so the six declarations sit together with their reasons. Handlers: portal_sync.go.
//
// `portal-sync` — ONE PATH ELEMENT for the module, so tools/ingress routes one prefix to comms, and the
// sub-resources under it are nouns (`settings`, `categories`, `runs`). VENDOR-CHOSEN:
// kb/00-foundation/ubiquitous-language.md §Tên tài nguyên trên URL has no row for the portal sync; the
// entities are 0013's `PortalSyncSettings`, `PortalCategory`, `PortalSyncRun`.
//
// `content.read` / `content.update` — §10.5's own keys for this screen, seeded at
// service-identity/migrations/0001_init.sql:292-293. NO KEY WAS INVENTED (rule 5, invariant 3c). Reads
// are `content.read`; every write — the configuration, the selection, starting a run — is
// `content.update`, the key that already publishes to every resident: the sync publishes too.
//
// THE KEYS ARE LITERALS AT EVERY CALL SITE: tools/apidoc reads them from the authz.RequirePermission
// call and refuses anything else (routes.go says why).

import (
	"context"
	"net/http"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// PortalSyncReader is the READ half. *app.PortalSyncAdmin satisfies it. CategoryTree calls the portal.
type PortalSyncReader interface {
	Settings(ctx context.Context) (app.PortalSyncSettingsView, error)
	CategoryTree(ctx context.Context) (app.PortalCategoryTree, error)
	Runs(ctx context.Context, req page.Request) (page.Result[domain.PortalSyncRun], error)
}

// PortalSyncWriter is the WRITE half: each opens a transaction and files its audit entry inside it
// (rule 6, invariant 3). *app.PortalSyncAdmin satisfies it.
type PortalSyncWriter interface {
	SaveSettings(ctx context.Context, req app.SavePortalSyncSettingsRequest, actor audit.Actor) (app.PortalSyncSettingsView, error)
	SaveCategories(ctx context.Context, sel []domain.PortalCategorySelection, actor audit.Actor) ([]domain.PortalCategory, error)
	StartRun(ctx context.Context, actor audit.Actor) (domain.PortalSyncRun, error)
}

// registerPortalSyncRoutes mounts the six routes. Register has already refused nil PortalSync /
// WritePortalSync.
func registerPortalSyncRoutes(mux *http.ServeMux, h *Handler) {
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Cấu hình đồng bộ tin từ Cổng TTĐT của xã — không bao giờ trả mã bảo mật, chỉ báo đã đặt hay chưa
	// @screen   11-noi-dung-mini-app §3
	// @reply    200 portalSyncSettingsOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/portal-sync/settings",
		authz.RequirePermission(h.d.Checker, "content.read")(
			http.HandlerFunc(h.GetPortalSyncSettings)))

	// PUT AND NOT PATCH: the modal is one form saved whole. The key is write-only: blank keeps it —
	// except on the first save and when `api_url` changed, both 422. 400 for a value outside its rule
	// (including an api_url the outbound guard would refuse: not https, not `.gov.vn`, an IP, a port
	// other than 443, userinfo, a query). 503 without SECRET_ENCRYPTION_KEYS, nothing written.
	//
	// idem.KhongCan: the result is the state the body names. A save that changes nothing and carries no
	// key writes and audits nothing; one carrying a key re-seals it and files one entry saying so.
	//
	// @summary  Lưu cấu hình đồng bộ Cổng TTĐT — mã bảo mật chỉ ghi, để trống là giữ, đổi địa chỉ API thì phải nhập lại
	// @screen   11-noi-dung-mini-app §3
	// @request  portalSyncSettingsIn
	// @reply    200 portalSyncSettingsOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    422 httpx.Error
	// @reply    503 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PUT /api/v1/portal-sync/settings",
		authz.RequirePermission(h.d.Checker, "content.update")(
			idem.KhongCan("lưu là ghi đè cả cấu hình bằng trạng thái trong thân; không đổi gì và không kèm mã thì không ghi và không có vết, kèm mã thì lần thứ hai chỉ niêm lại cùng mã và ghi đúng một vết nói điều đó")(
				http.HandlerFunc(h.PutPortalSyncSettings))))

	// THE PORTAL'S CATEGORY TREE, ASKED NOW (ADR 0067 §2 "Chế độ đăng" #4 — never copied), with the
	// commune's stored choice merged in. Uses the SAVED settings. 409 when nothing is saved; 502 when
	// the portal fails, with a code per error class and a sentence that never quotes the portal.
	//
	// @summary  Cây chuyên mục của Cổng TTĐT, hỏi trực tiếp Cổng lúc mở cấu hình, kèm lựa chọn và ánh xạ loại đã lưu của xã
	// @screen   11-noi-dung-mini-app §3
	// @reply    200 portalCategoryTreeOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    502 httpx.Error
	// @reply    503 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/portal-sync/categories",
		authz.RequirePermission(h.d.Checker, "content.read")(
			http.HandlerFunc(h.GetPortalCategories)))

	// The selection and mapping: [{external_id, name, target_kind, is_selected}]. Upserted — a row is
	// created only for a category being selected; unticking is `is_selected = false` on the SAME row
	// (0013: never a delete). Categories not in the body are left as they are.
	//
	// idem.KhongCan: an entry that changes nothing writes nothing, so the same body twice leaves one
	// state and one entry.
	//
	// @summary  Lưu lựa chọn chuyên mục Cổng và ánh xạ loại nội dung (tin tức, sự kiện, thông báo)
	// @screen   11-noi-dung-mini-app §3
	// @request  portalCategoriesIn
	// @reply    200 portalCategoriesOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PUT /api/v1/portal-sync/categories",
		authz.RequirePermission(h.d.Checker, "content.update")(
			idem.KhongCan("lưu lựa chọn là ghi đè trạng thái các chuyên mục trong thân; mục không đổi thì không ghi, nên lần gửi thứ hai để lại đúng một trạng thái và đúng một vết")(
				http.HandlerFunc(h.PutPortalCategories))))

	// The run history, newest first. Counts, outcome and the error summary (a category and an error
	// class per entry) — no article text, no personal data.
	//
	// @summary  Lịch sử các lượt đồng bộ Cổng của xã — mới nhất trước, kèm số tin đọc, nhập, bỏ qua, lỗi
	// @screen   11-noi-dung-mini-app §3
	// @reply    200 page.Result[portalRunOut]
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/portal-sync/runs",
		authz.RequirePermission(h.d.Checker, "content.read")(
			http.HandlerFunc(h.ListPortalSyncRuns)))

	// `⟳ Đồng bộ ngay` — `runs` is the collection a POST adds a `chay-tay` run to, signed with the
	// caller's business code. 202: the row exists; the work continues in the background. 409
	// `portal_sync_in_progress` while the commune's run lock is held (by any replica), 409
	// `portal_sync_not_configured` before the first save; 503 without SECRET_ENCRYPTION_KEYS.
	//
	// idem.Required(MoKhiHong): the commune's run lock is the real guard — a second run cannot start
	// while one runs, whatever happens to Redis. The key turns a double click into a replayed 202
	// instead of a 409.
	//
	// @summary  Đồng bộ ngay — mở một lượt đồng bộ Cổng chạy tay cho xã, chạy nền, kết quả xem ở lịch sử
	// @screen   11-noi-dung-mini-app §3
	// @reply    202 portalRunOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    503 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/portal-sync/runs",
		authz.RequirePermission(h.d.Checker, "content.update")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.StartPortalSyncRun))))
}
