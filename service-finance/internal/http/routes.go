package http

// Routes for the finance service.
//
// EVERY route declares its permission explicitly. The global guard only rejects users who are
// not signed in; it does not check permissions. A route with no declaration is callable by
// every staff role, and nothing reports it — see rule 5.
//
// Four declarations are available, and there is no fifth:
//
//	authz.RequirePermission(checker, "budget.read")   // the normal case
//	authz.CitizenOnly()                                     // citizen paths, isolated by identity
//	authz.AnyAuthenticated("<why any account needs this>")  // reason mandatory
//	authz.Public("<why this is public>")                    // reason mandatory
//
// EVERY route also carries an @-annotation block IMMEDIATELY above the statement — no blank
// line between. `tools/apidoc` reads it and generates kb/20-contracts/openapi.json, which is
// the type contract the admin web builds against.
//
//	// @summary  <one line, Vietnamese — a person reads it>
//	// @screen   <file in docs/ui-ux/ §section>   design intent, the one part no tool can derive
//	// @request  <Go type>                        omit when the route takes no body
//	// @reply    <status> <Go type|->             one line per status the handler REALLY returns
//
// The permission and the idempotency mode are NOT annotated: apidoc reads them from the
// authz.* / idem.* calls below, so there is no second copy to drift (rule 9).

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// HangMucKeHoachVonDanhMuc is the commune's capital plan category catalogue, for
// GET /api/v1/capital-plan-categories.
//
// AN INTERFACE DECLARED AT THE POINT OF USE, not the concrete *fistore.HangMucKeHoachVonStore.
// WHY: the properties this route exists to hold — the permission declaration, the commune check
// ahead of any read, the refusal instead of a truncated list — have to be testable without a
// PostgreSQL, or they get tested once and then never again. There is no PostgreSQL reachable from
// this repository's build environment, so "needs a database" means "never runs". The concrete store
// satisfies this as it is; nothing was changed to accommodate it.
//
// NO page.Request PARAMETER: this route returns the whole list on purpose. The reason is on
// fistore.HangMucKeHoachVonStore.DanhSach, and the bound that replaces the missing `limit` is
// fistore.TranDanhMucHangMuc.
type HangMucKeHoachVonDanhMuc interface {
	DanhSach(ctx context.Context) ([]domain.HangMucKeHoachVon, error)
}

// DuAnTienDo is the commune's investment projects with their DERIVED disbursement figures, for
// GET /api/v1/disbursements/projects and .../{id}.
//
// AN INTERFACE DECLARED AT THE POINT OF USE, not the concrete *fistore.DuAnStore — the same reason
// HangMucKeHoachVonDanhMuc gives: the properties these routes exist to hold (the permission
// declaration, the commune check ahead of any read, a refusal rather than a truncated list that
// gets totalled) have to be testable without a PostgreSQL, or they get tested once and never
// again. There is no PostgreSQL reachable from this repository's build environment.
type DuAnTienDo interface {
	DanhSach(ctx context.Context, loc fistore.LocDuAn) ([]domain.TienDoDuAn, error)
	ChiTiet(ctx context.Context, id string) (domain.TienDoDuAn, error)
}

// Deps are everything the routes need. Kept explicit so wiring stays in cmd/server.
type Deps struct {
	Checker authz.Checker
	HangMuc HangMucKeHoachVonDanhMuc
	DuAn    DuAnTienDo

	// Nay is the clock the derived disbursement figures are computed against. NIL IN PRODUCTION,
	// where Handler.nay falls back to time.Now — see the reason there. It exists so the delay
	// arithmetic of §3 can be exercised at the two dates it is most fragile on.
	Nay func() time.Time

	Log *slog.Logger
}

// Register mounts the finance routes.
//
// Add every new route with its permission declaration in the SAME statement, never on a nearby
// line: rbac_guard anchors to the statement, and so should a reader.
func Register(mux *http.ServeMux, d Deps) {
	// Refusing incomplete wiring HERE, at construction, not at request time: a route mounted
	// without the store behind it would accept requests it cannot honour, and the first person to
	// find out would be a member of staff in front of a government screen. Same discipline as
	// authz.Public("") and idem.KhongCan("").
	//
	// Deps.Checker IS NOW CHECKED, and the comment that used to stand here said why it was not:
	// no route declared RequirePermission, so demanding a checker would have refused to start over
	// a dependency nothing read. The disbursement routes below declare `budget.read`, so that is no
	// longer true — a nil Checker would make authz.RequirePermission panic on the first request
	// from a member of staff, which is the worst possible moment to find out.
	if d.Checker == nil {
		panic("finance/http: thiếu authz.Checker — các tuyến giải ngân khai budget.read và sẽ panic khi có người gọi")
	}
	if d.HangMuc == nil {
		panic("finance/http: thiếu kho danh mục hạng mục kế hoạch vốn — GET /api/v1/capital-plan-categories sẽ panic khi có người gọi")
	}
	if d.DuAn == nil {
		panic("finance/http: thiếu kho dự án — các tuyến /api/v1/disbursements/projects sẽ panic khi có người gọi")
	}

	h := NewHandler(d)

	// --- the commune's capital plan category catalogue ----------------------------------------
	//
	// `capital-plan-categories` — English, plural, kebab-case, derived from the entity already
	// settled as `CapitalPlanCategory` (kb/00-foundation/ubiquitous-language.md:157). Nothing is
	// translated on the spot here: the row exists, and it states why the concept is a `…Category`
	// and not an `…Item` — a hạng mục CLASSIFIES the lines of a capital plan, it is not a line of
	// money. Renaming this path toward `items` would invite the next person to hang an amount on
	// the catalogue itself.
	//
	// STATED GAP: that row's "Tài nguyên URL" column still reads *(chưa chốt)*. The path was
	// decided by the user on 20/09/2026 and the row belongs to whoever owns that table, not to this
	// file — copying the decision into a second place is how two sources drift (rule 9).
	//
	// AnyAuthenticated, AND THE REASON IS THE SHAPE OF THE DATA'S USE — the same call the user
	// accepted for GET /api/v1/org-units in the identity service. Category names fill the pickers
	// and filters of nearly every screen that touches a capital plan, so requiring a configuration
	// permission would not protect anything; it would break those screens for everybody who is not
	// an administrator. The alternative that actually protects something does not exist here: there
	// is nothing sensitive in a list of budget classifications, which come from budget regulation
	// rather than from anything the commune keeps to itself.
	//
	// THE TRADE-OFF, STATED RATHER THAN GLOSSED: the commune's own catalogue is readable by every
	// signed-in account OF THAT COMMUNE. It is not readable across communes — and where that is
	// enforced is NOT where it looks:
	//
	//	LOCAL AND REAL      Scoped.Query binds `tenant_id` from the context (rule 1, invariant 5),
	//	                    so this query cannot reach another commune's rows even if a principal
	//	                    lied about which commune it belongs to.
	//	NOT LOCAL           authz.AnyAuthenticated's commune check cannot fail HERE: core/staffauth
	//	                    stamps the Host commune onto the principal it builds (staffauth.go:157
	//	                    and :241), so xacNhanXa compares a value with itself. The comparison
	//	                    that decides is inside identity — service-identity/internal/grpc/
	//	                    server.go:257 — where the commune from `x-tenant-id` meets the commune
	//	                    INSIDE the credential. A mismatch there yields no principal, and this
	//	                    route then answers 401 for absence, not for disagreement.
	//
	// Written out because three edits would remove it with no test in THIS service turning red:
	// exempting ResolveStaffPrincipal from carrying a commune, moving that comparison below the
	// session-registry read, or taking the commune from the RPC response instead of from Host.
	//
	// What is accepted is that a member of staff with no configuration rights can
	// see how their own authority classifies its capital plan. What is NOT exposed here is any
	// amount, any plan, and the tier a row sits in — see internal/domain.
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Danh mục hạng mục kế hoạch vốn của xã — dùng cho ô phân loại dòng kế hoạch và bộ lọc
	// @screen   14-cau-hinh §5
	// 500 covers two different causes and says so honestly: an ordinary store failure, and the
	// commune's catalogue exceeding fistore.TranDanhMucHangMuc — which this route REFUSES rather
	// than truncating, because a silently short list is a category missing from a classifier.
	//
	// 401 comes from authz.AnyAuthenticated: no session, or a token issued for another commune.
	// There is no 403 on this route and none is claimed — there is no permission to be refused.
	//
	// @reply    200 danhSachHangMucRa
	// @reply    401 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/capital-plan-categories",
		authz.AnyAuthenticated("tên hạng mục xuất hiện ở ô phân loại dòng kế hoạch vốn và mọi bộ lọc của các màn hình tài chính — đòi một quyền cấu hình sẽ làm hỏng những màn hình đó cho mọi tài khoản không phải quản trị; đánh đổi đã chấp nhận: danh mục của xã lộ cho mọi tài khoản đã đăng nhập CỦA CHÍNH XÃ ĐÓ, không chéo xã vì Scoped buộc tenant_id")(
			http.HandlerFunc(h.DanhSachHangMucKeHoachVon)))

	// --- disbursement tracking: the commune's investment projects -----------------------------
	//
	// `budget.read` IS A KEY THAT ALREADY EXISTS — it is one of the three `budget.*` rows loaded by
	// service-identity/migrations/0001_init.sql (`budget.read`, `budget.update`, `budget.confirm`).
	// Nothing here invents a new one: a permission string absent from `quyen` is a permission no
	// administrator can grant on the Phân quyền screen, so a route guarded by one is a route
	// nobody can ever reach (rule 5, invariant 3b).
	//
	// NOT AnyAuthenticated, unlike the catalogue route above, and the difference is the data. A
	// list of budget classifications says nothing; this says how much money the commune has and
	// how much of it has moved, project by project, with the names of the units responsible. That
	// is the commune's financial position before it is published anywhere — readable by the
	// accountant and by leadership, not by every account that can sign in.
	//
	// THE URL NOUN `projects` IS NOT SETTLED, AND THIS IS THE ONE THING TO CONFIRM BEFORE THE
	// CONTRACT IS PUBLISHED. `disbursements` is settled — kb/00-foundation/ubiquitous-language.md
	// maps `giai_ngan` to it. `du_an` HAS NO ROW IN THAT TABLE, and ADR 0011 says a concept with no
	// row is asked about, not translated on the spot: `org-units` is the worked example of an
	// obvious English word being the wrong one. The path is written here so the slice runs; it has
	// reached no client, because kb/20-contracts/openapi.json has not been regenerated. Moving it
	// is one string today and a contract change after that.
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Danh sách dự án đầu tư của xã theo năm ngân sách, kèm số đã giải ngân suy ra từ chứng từ
	// @screen   06-giai-ngan §7
	// @reply    200 danhSachDuAnRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/disbursements/projects",
		authz.RequirePermission(d.Checker, "budget.read")(
			http.HandlerFunc(h.DanhSachDuAn)))

	// 404 covers both "no such project" and "a project of another commune", deliberately — see the
	// handler. There is no path here that could answer differently for the two, because the store
	// cannot reach another commune's row at all.
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Chi tiết một dự án đầu tư: kế hoạch vốn, đã giải ngân, tỷ lệ và điểm chậm
	// @screen   06-giai-ngan §8
	// @reply    200 duAnRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/disbursements/projects/{id}",
		authz.RequirePermission(d.Checker, "budget.read")(
			http.HandlerFunc(h.ChiTietDuAn)))
}
