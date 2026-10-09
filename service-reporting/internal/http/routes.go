package http

// Routes for the reporting service.
//
// EVERY route declares its permission explicitly. The global guard only rejects users who are
// not signed in; it does not check permissions. A route with no declaration is callable by
// every staff role, and nothing reports it — see rule 5.
//
// Four declarations are available, and there is no fifth:
//
//	authz.RequirePermission(checker, "report.read")   // the normal case
//	authz.CitizenOnly()                                     // citizen paths, isolated by identity
//	authz.AnyAuthenticated("<why any account needs this>")  // reason mandatory
//	authz.Public("<why this is public>")                    // reason mandatory

import (
	"log/slog"
	"net/http"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
)

// Deps are everything the routes need. Kept explicit so wiring stays in cmd/server.
type Deps struct {
	Checker authz.Checker

	// SystemMessages is "Lời hệ thống" for the `report.*` sentences this service prints
	// (migration 0003). *app.SystemMessages in production. Refused at construction when missing.
	SystemMessages SystemMessageService

	Log *slog.Logger
}

// Register mounts the reporting routes.
//
// Add every new route with its permission declaration in the SAME statement, never on a nearby
// line: rbac_guard anchors to the statement, and so should a reader. Paths are English, plural,
// versioned; a non-CRUD action is a nominalised sub-resource, never a verb
// (-> .claude/skills/rest-api-design/SKILL.md).
func Register(mux *http.ServeMux, d Deps) {
	// Refusing incomplete wiring HERE, at construction, not at request time: a route mounted
	// without the use case behind it would accept requests it cannot honour, and the first person
	// to find out would be a member of staff in front of a government screen. A nil Checker would
	// make authz.RequirePermission panic on the first request.
	if d.Checker == nil {
		panic("reporting/http: thiếu authz.Checker — các tuyến lời hệ thống khai admin.lookup và sẽ panic khi có người gọi")
	}
	if d.SystemMessages == nil {
		panic("reporting/http: thiếu use case lời hệ thống — các tuyến /api/v1/reporting-system-messages sẽ panic khi có người gọi")
	}
	h := NewHandler(d)

	// --- "Lời hệ thống", the reporting half (14-cau-hinh §7, ADR 0024 §Phụ, Bổ sung 29/09/2026) ---
	//
	// THE SAME THREE ROUTES finance and petitions serve, under this service's own first segment:
	// tools/ingress refuses two services on one first segment (ADR 0054 §3). The 38 `report.*` keys
	// are listed here.
	//
	// `{code}` AND NOT `{key}`, and the response field is `code`: tools/apidoc's credential guard
	// refuses the word `key` in a response field, and the path parameter names the same thing.
	//
	// `override` IS A SUB-RESOURCE: the commune's own wording is a state PUT sets and DELETE removes,
	// and the message itself survives both — a shipped key cannot be deleted (14-cau-hinh §7).
	//
	// `admin.lookup` — "Quản lý danh mục" — ON ALL THREE, the key the requirement repository guards
	// this screen with (../vigov-require/docs/spec/04-api.md:41-44), seeded at
	// service-identity/migrations/0001_init.sql:281, and the key finance's and petitions' halves use.
	// NO KEY INVENTED (rule 5, invariant 3c). NOT `report.read`: reading a report is not administering
	// the commune's configuration.

	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Lời hệ thống của phân hệ Báo cáo: câu mặc định, câu xã đang dùng và ai sửa lần cuối
	// @screen   14-cau-hinh §7
	// @reply    200 systemMessageListOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/reporting-system-messages",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			http.HandlerFunc(h.ListSystemMessages)))

	// PUT, a full replacement of the one field this resource has. Empty `text` is 400 and names the
	// DELETE below — an empty sentence is not a state this screen offers.
	//
	// idem.KhongCan: app.Reword writes nothing and files no entry when the text in force already
	// equals the text sent — so a retry leaves one row, one entry.
	//
	// @summary  Xã sửa lời một câu hệ thống của phân hệ Báo cáo
	// @screen   14-cau-hinh §7
	// @request  rewordSystemMessageIn
	// @reply    200 systemMessageOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PUT /api/v1/reporting-system-messages/{code}/override",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("đặt lại đúng câu đang dùng không ghi gì và không để vết, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.RewordSystemMessage))))

	// "Khôi phục câu mặc định". A soft delete of the commune's live wording (rule 7, invariant 1);
	// the history stays on disk and in audit_log. 204 also when the commune is already on the
	// default — the state asked for holds, and nothing is written.
	//
	// idem.KhongCan: the second request finds no live wording and writes nothing.
	//
	// @summary  Khôi phục câu mặc định của phần mềm cho một câu hệ thống của phân hệ Báo cáo
	// @screen   14-cau-hinh §7
	// @reply    204 -
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/reporting-system-messages/{code}/override",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("khôi phục khi xã đã dùng câu mặc định thì không còn dòng nào để gỡ và không ghi gì")(
				http.HandlerFunc(h.RestoreSystemMessage))))

	// --- ADR 0079 Q2 ("Làm đúng prototype", migration 0004) — the switch ---------------------
	//
	// SAME KEY, `admin.lookup`: the same screen and the same act of administering the commune's
	// configuration as the three routes above (rule 5, invariant 3c — no key invented).

	// "Tắt / Bật" of a SHIPPED key, reworded or not (user decision 09/10/2026, which retired the 409
	// `no_commune_wording`): a PATCH of the override sub-resource's one other field. A switched-off
	// sentence is hidden where it is used, or the shipped default where the consumer must say something.
	// Nothing prints a `report.*` sentence yet; the export, when built, decides per
	// sentence hidden (Active false) or default (CurrentText).
	//
	// idem.KhongCan: app.SetActive writes nothing and files no entry when the state already holds.
	//
	// @summary  Xã tắt hoặc bật một câu hệ thống (tắt thì ẩn nơi dùng, nơi bắt buộc có lời thì dùng lời gốc)
	// @screen   14-cau-hinh §7
	// @request  switchSystemMessageIn
	// @reply    200 systemMessageOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/reporting-system-messages/{code}/override",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("đặt lại đúng trạng thái đang có không ghi gì và không để vết, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.SwitchSystemMessage))))
}
