package http

// Routes for the documents service.
//
// EVERY route declares its permission explicitly. The global guard only rejects users who are
// not signed in; it does not check permissions. A route with no declaration is callable by
// every staff role, and nothing reports it — see rule 5.
//
// Four declarations are available, and there is no fifth:
//
//	authz.RequirePermission(checker, "document.read")   // the normal case
//	authz.CitizenOnly()                                     // citizen paths, isolated by identity
//	authz.AnyAuthenticated("<why any account needs this>")  // reason mandatory
//	authz.Public("<why this is public>")                    // reason mandatory
//
// EVERY route also carries an @-annotation block IMMEDIATELY above the statement — no blank line
// between. `tools/apidoc` reads it and generates kb/20-contracts/openapi.json, which is the type
// contract the admin web builds against.
//
//	// @summary  <one line, Vietnamese — a person reads it>
//	// @screen   <file in docs/ui-ux/ §section>   design intent, the one part no tool can derive
//	// @request  <Go type>                        omit when the route takes no body
//	// @reply    <status> <Go type|->             one line per status the handler REALLY returns
//
// The permission and the idempotency mode are NOT annotated: apidoc reads them from the authz.* /
// idem.* calls below, so there is no second copy to drift (rule 9).

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

// LoaiVanBanDanhMuc is the commune's document-type catalogue, for GET /api/v1/document-types.
//
// AN INTERFACE DECLARED AT THE POINT OF USE, not the concrete *docstore.LoaiVanBanStore. The route
// carries the isolation rules this service exists to enforce — the commune check before any read,
// the refusal to truncate — and those have to be testable without a PostgreSQL, or they get tested
// once and then never again. *docstore.LoaiVanBanStore satisfies this as it is; nothing was
// changed to accommodate it.
//
// NO page.Request PARAMETER: this route returns the whole list on purpose. The reason is on
// docstore.LoaiVanBanStore.DanhSach, and the bound that replaces the missing `limit` is
// docstore.TranDanhMucLoaiVanBan.
type LoaiVanBanDanhMuc interface {
	DanhSach(ctx context.Context) ([]domain.LoaiVanBan, error)
}

// Deps are everything the routes need. Kept explicit so wiring stays in cmd/server.
type Deps struct {
	// Checker guards the routes declared with authz.RequirePermission. No route here uses one
	// yet — the single route below is AnyAuthenticated — so Register does NOT refuse a nil
	// Checker: a panic on a dependency nothing reads would stop a service for a reason that is
	// not true. The first RequirePermission route added here adds that case to the switch.
	Checker    authz.Checker
	LoaiVanBan LoaiVanBanDanhMuc

	Log *slog.Logger
}

// Register mounts the documents routes.
//
// Add every new route with its permission declaration in the SAME statement, never on a nearby
// line: rbac_guard anchors to the statement, and so should a reader.
func Register(mux *http.ServeMux, d Deps) {
	// Refusing incomplete wiring HERE, at construction, not at request time: a route mounted
	// without the store behind it would accept requests it cannot honour, and the first person to
	// find out would be a member of staff registering a document in a government system.
	switch {
	case d.LoaiVanBan == nil:
		panic("documents/http: thiếu kho loại văn bản — GET /api/v1/document-types sẽ panic khi có người gọi")
	}

	h := NewHandler(d)

	// --- the commune's document-type catalogue -------------------------------------------------
	//
	// `document-types` — the entity is `DocumentType`, written on the `-- @entity` mark above the
	// table (migrations/0003_danh_muc_loai_van_ban.sql), and the path is its plural kebab-case
	// form (skills/rest-api-design REQUIRED #1). Nothing is translated on the spot here: the
	// English name was settled when the table was born, and a path cannot be taken back once a
	// commune is live. STATED GAP: the URL-resource cell for this concept in
	// kb/00-foundation/ubiquitous-language.md:158 still reads *(chưa chốt)*; filling it belongs to
	// whoever owns that table (rule 9), and this route has no external caller yet.
	//
	// AnyAuthenticated, AND THE REASON IS THE SHAPE OF THE DATA'S USE — the same call made for
	// GET /api/v1/org-units in the identity service. Type names fill the registration form, the
	// filter on every document list and the label on every document already registered, so
	// requiring a configuration permission would not protect anything: it would break those
	// screens for everybody who is not an administrator. The alternative that actually protects
	// something does not exist here — there is nothing sensitive in the list of names an authority
	// files its paperwork under.
	//
	// THE TRADE-OFF, STATED RATHER THAN GLOSSED: a commune's catalogue is readable by every
	// signed-in account OF THAT COMMUNE. It is not readable across communes, and two independent
	// things stop it: authz.AnyAuthenticated compares the commune in the principal against the
	// commune resolved from Host and answers 401 before the handler runs, and the query itself
	// could not reach another commune's rows in any case — Scoped.Query binds `tenant_id` from the
	// context (rule 1, invariant 5). What is accepted is that a member of staff with no
	// configuration rights can see how their own authority classifies its documents, which is
	// information printed on the documents themselves.
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Danh mục loại văn bản của xã — dùng cho ô chọn loại khi vào sổ, bộ lọc và nhãn trên mọi văn bản
	// @screen   14-cau-hinh §5
	// 500 covers two different causes and says so honestly: an ordinary store failure, and the
	// commune's catalogue exceeding docstore.TranDanhMucLoaiVanBan — which this route REFUSES
	// rather than truncating, because a silently short list files a document under the wrong type
	// and numbering follows the type.
	//
	// 403 IS ABSENT ON PURPOSE: authz.AnyAuthenticated answers 401 for a missing principal AND for
	// a token issued by another commune, and never 403. Declaring one would name a status this
	// handler cannot produce.
	//
	// @reply    200 danhSachLoaiVanBanRa
	// @reply    401 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/document-types",
		authz.AnyAuthenticated("tên loại văn bản xuất hiện ở ô chọn loại khi vào sổ, bộ lọc của mọi danh sách văn bản và nhãn trên từng văn bản đã vào sổ — đòi một quyền cấu hình sẽ làm hỏng những màn hình đó cho mọi tài khoản không phải quản trị; đánh đổi đã chấp nhận: danh mục lộ cho mọi tài khoản đã đăng nhập CỦA CHÍNH XÃ ĐÓ, không chéo xã vì Scoped buộc tenant_id")(
			http.HandlerFunc(h.DanhSachLoaiVanBan)))
}
