package http

// Routes for the PUBLIC surface of the comms service — the first non-staff surface this service has
// (owner decision 2026-09-27). Handlers and the reasoning are in commune_news.go.
//
// A SECOND Register INTO A SECOND MUX, with its own Deps, behind its own chain (cmd/server
// buildPublicChain): CORS for the Mini App origin, header stripping, recovery — no TenantMiddleware (the
// Mini App calls the reserved API host, ADR 0046, which maps to no commune) and no staffauth (there is
// no session). The commune is resolved per request from `?host=` by the platform, in the handler.

import (
	"net/http"

	"github.com/vihat/vigov/core/authz"
)

// CommuneNewsPath is the collection the public chain serves. Exported because cmd/server must route the
// SAME path — and its subtree, for `{id}` — to the public chain on the outer mux; two spellings of one
// path is a route that silently falls to the staff chain and 404s on the reserved API host. The routes
// below spell it as a LITERAL because tools/apidoc reads the pattern from the source;
// commune_news_test.go asserts the two agree.
const CommuneNewsPath = "/api/v1/commune-news"

// RegisterPublic mounts the public routes onto their OWN mux.
func RegisterPublic(mux *http.ServeMux, d PublicDeps) {
	switch {
	case d.Tenants == nil:
		panic("comms/http: thiếu kho tra xã theo tên miền — tuyến tin của xã sẽ panic khi có người gọi")
	case d.ContentItems == nil:
		panic("comms/http: thiếu kho nội dung công khai — tuyến tin của xã sẽ panic khi có người gọi")
	case d.Categories == nil:
		panic("comms/http: thiếu kho danh mục nội dung — tuyến tin của xã sẽ panic khi có người gọi")
	}
	h := newPublicHandler(d)

	// `commune-news` — the owner's noun (2026-09-27). DISTINCT from `content-items`, the staff register
	// of every state: a different audience, a different predicate, a different response type.
	//
	// PUBLIC (owner decision 2026-09-27, following docs/ui-ux/11-noi-dung-mini-app.md:189-190 "công
	// khai cho Mini App đọc"). The commune published these items TO its residents; a resident opening
	// the Mini App has no account and needs none to read the commune's own notice board.
	//
	// NO idem.* DECLARATION: a GET changes no state (and nothing here counts views).
	//
	// @summary  Tin đã đăng của xã trên Zalo Mini App, theo tên miền của xã — mới nhất trước, văn bản thuần, phân trang con trỏ
	// @screen   11-noi-dung-mini-app §9
	// @consumer citizen-app
	//
	// 200 is one page. An EMPTY page for a commune that published nothing AND for a domain no active
	// commune holds — identical bytes.
	//
	// 400 is `host` missing, repeated or malformed (the platform is not asked), or a bad `limit`/`cursor`.
	//
	// 500 is a store failure. 503 is the platform registry unreachable.
	//
	// @reply    200 page.Result[tinXaRa]
	// @reply    400 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/commune-news",
		authz.Public("bảng tin xã công bố cho người dân trên Zalo Mini App (docs/ui-ux/11-noi-dung-mini-app.md:189-190): người dân đọc không cần tài khoản; chỉ trả mục đã đăng (dang-hien) của đúng xã mà nền tảng phân giải từ tên miền, dạng văn bản thuần")(
			http.HandlerFunc(h.ListCommuneNews)))

	// ONE published item, body included, as plain text.
	//
	// @summary  Một tin đã đăng của xã, toàn văn dạng văn bản thuần — tin chưa đăng hay của xã khác trả cùng một 404
	// @screen   11-noi-dung-mini-app §9
	// @consumer citizen-app
	//
	// 404 is ONE answer for: no such id, another commune's id, not published, soft-deleted, or a domain
	// no active commune holds.
	//
	// @reply    200 tinXaRa
	// @reply    400 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/commune-news/{id}",
		authz.Public("toàn văn một tin xã đã công bố cho người dân trên Zalo Mini App (docs/ui-ux/11-noi-dung-mini-app.md:189-190): người dân đọc không cần tài khoản; tin chưa đăng hoặc của xã khác trả cùng một 404")(
			http.HandlerFunc(h.GetCommuneNews)))
}
