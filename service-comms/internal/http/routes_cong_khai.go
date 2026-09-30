package http

// Routes for the PUBLIC surface of the comms service — the first non-staff surface this service has
// (owner decision 2026-09-27). Handlers and the reasoning are in tin_xa_cong_khai.go.
//
// A SECOND Register INTO A SECOND MUX, with its own Deps, behind its own chain (cmd/server
// dungBienCongKhai): CORS for the Mini App origin, header stripping, recovery — no TenantMiddleware (the
// Mini App calls the reserved API host, ADR 0046, which maps to no commune) and no staffauth (there is
// no session). The commune is resolved per request from `?host=` by the platform, in the handler.

import (
	"net/http"

	"github.com/vihat/vigov/core/authz"
)

// MauTinXa is the collection the public chain serves. Exported because cmd/server must route the SAME
// path — and its subtree, for `{id}` and `categories` — to the public chain on the outer mux; two
// spellings of one path is a route that silently falls to the staff chain and 404s on the reserved API
// host. The routes below spell it as a LITERAL because tools/apidoc reads the pattern from the source;
// tin_xa_cong_khai_test.go asserts the two agree.
const MauTinXa = "/api/v1/commune-news"

// RegisterCongKhai mounts the public routes onto their OWN mux.
func RegisterCongKhai(mux *http.ServeMux, d DepsCongKhai) {
	switch {
	case d.Xa == nil:
		panic("comms/http: thiếu kho tra xã theo tên miền — tuyến tin của xã sẽ panic khi có người gọi")
	case d.NoiDung == nil:
		panic("comms/http: thiếu kho nội dung công khai — tuyến tin của xã sẽ panic khi có người gọi")
	case d.DanhMuc == nil:
		panic("comms/http: thiếu kho danh mục nội dung — tuyến tin của xã sẽ panic khi có người gọi")
	}
	h := newHandlerCongKhai(d)

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
	// commune holds — identical bytes. Also empty for a well-formed `category` naming nothing here.
	//
	// 400 is `host` missing, repeated or malformed (the platform is not asked), a `type` outside the six,
	// a malformed `category`, or a bad `limit`/`cursor`.
	//
	// 500 is a store failure. 503 is the platform registry unreachable.
	//
	// @reply    200 page.Result[tinXaRa]
	// @reply    400 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/commune-news",
		authz.Public("bảng tin xã công bố cho người dân trên Zalo Mini App (docs/ui-ux/11-noi-dung-mini-app.md:189-190): người dân đọc không cần tài khoản; chỉ trả mục đã đăng (dang-hien) của đúng xã mà nền tảng phân giải từ tên miền, dạng văn bản thuần")(
			http.HandlerFunc(h.DanhSachTinXa)))

	// The chip row of the Mini App news tab (user decision 2026-09-30, reversing the 2026-09-27 "no
	// categories endpoint" — tin_xa_cong_khai.go says why). The commune's own categories that hold a
	// published item, themselves or through a descendant; flat, `parent_id` absent on a root.
	//
	// INSIDE THE /api/v1/commune-news SUBTREE ON PURPOSE: cmd/server routes MauTinXa+"/" to the public
	// chain, so this path needs no second outer-mux entry. It does NOT collide with `{id}` below: Go
	// 1.22's ServeMux prefers the more specific pattern, and a literal segment is more specific than a
	// wildcard, so `categories` is never read as an item id (TestPublicNewsCategoriesNotSwallowedByID).
	// No item id is `categories` either — ids are 26-character ULIDs.
	//
	// PUBLIC for the reason the list is: the commune published these items to its residents, and the
	// names it files them under are part of that publication.
	//
	// @summary  Danh mục tin của xã có ít nhất một tin đã đăng (tự nó hoặc danh mục con), theo tên miền của xã — cho hàng chip lọc hai tầng trên Zalo Mini App
	// @screen   11-noi-dung-mini-app §9
	// @consumer citizen-app
	//
	// 200 is the whole list, never paginated. `{"items":[]}` for a commune with nothing filed AND for a
	// domain no active commune holds — identical bytes.
	//
	// 400 is `host` missing, repeated or malformed, or a `type` outside the six (the platform is not asked).
	//
	// 500 is a store failure or the category cap exceeded. 503 is the platform registry unreachable.
	//
	// @reply    200 publicCategoriesOut
	// @reply    400 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/commune-news/categories",
		authz.Public("hàng chip danh mục của bảng tin xã trên Zalo Mini App (người dùng quyết định 30/09/2026): người dân đọc không cần tài khoản; chỉ trả tên danh mục của đúng xã mà nền tảng phân giải từ tên miền, và chỉ danh mục có tin đã đăng (dang-hien), dạng văn bản thuần")(
			http.HandlerFunc(h.PublicNewsCategories)))

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
			http.HandlerFunc(h.MotTinXa)))
}
