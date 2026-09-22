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

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/service-finance/internal/app"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// THE PERMISSION ON THE THREE WRITE ROUTES BELOW IS `admin.lookup`, WRITTEN OUT AT EVERY CALL SITE.
//
// A CONSTANT WOULD READ BETTER AND IS DELIBERATELY NOT USED: tools/apidoc resolves the key from the
// authz.RequirePermission call and refuses anything that is not a string literal there — "khóa
// quyền không phải hằng chuỗi — không ghi vào hợp đồng được". A route whose key it cannot read is a
// route absent from kb/20-contracts/openapi.json, which is the contract the admin web builds
// against (ADR 0014). Three literals that a whole-repository scan checks beat one constant the
// generator cannot see.
//
// `admin.lookup` — "Quản lý danh mục" — IS THE KEY THE SPECIFICATION ALREADY NAMES FOR THIS EXACT
// SCREEN, checked rather than assumed: docs/ui-ux/14-cau-hinh.md:109 lists it in the permission
// matrix, and §5 of that same file (:148-182) is the `Danh mục` tab these routes serve — the one
// with `+ Thêm mục`, the `✎` / `Tắt` / `🗑` actions and the `Nguồn` column. The key is seeded at
// service-identity/migrations/0001_init.sql:274, so a commune administrator can actually tick it.
//
// NO NEW KEY WAS INVENTED, and that is rule 5, invariant 3c: a key no migration seeds is a right
// nobody can grant, so the route would answer 403 to every account forever while the tests stayed
// green. Had `admin.lookup` not existed, the correct move is a finding for open question #27 — not
// an INSERT. tools/check_quyen.py scans the whole repository against the `quyen` table on every
// `make check`.
//
// WHY THE WRITE ROUTES ARE GUARDED WHILE THE READ ROUTE IS AnyAuthenticated: they answer different
// questions. Reading the list is what fills a box on nearly every screen, so a configuration
// permission there would empty those screens for everybody who is not an administrator (the
// argument accepted for GET /api/v1/org-units and for all eight catalogue reads). Changing the list
// is administration of the commune's own configuration, which is precisely what this key is for.

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
// GET /api/v1/investment-projects and .../{id}.
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
// GhiHangMuc is the WRITE half of the capital plan category catalogue, and it is a second
// interface rather than three more methods on the read one — on purpose.
//
// The read is a store call; each of these three opens a TRANSACTION and writes an audit entry
// inside it (rule 6, invariant 3). Behind one interface a future caller would reach for whichever
// method was nearest and could end up writing the row outside a transaction, which is the exact
// defect core/audit was shaped to make impossible. Two interfaces, two obligations, visible at the
// point of use.
type GhiHangMuc interface {
	Them(ctx context.Context, yc app.YeuCauThemHangMuc, nguoi audit.Actor) (domain.HangMucKeHoachVon, error)
	Sua(ctx context.Context, id string, yc app.YeuCauSuaHangMuc, nguoi audit.Actor) (domain.HangMucKeHoachVon, error)
	Xoa(ctx context.Context, id, lyDo string, nguoi audit.Actor) error
}

type Deps struct {
	Checker    authz.Checker
	HangMuc    HangMucKeHoachVonDanhMuc
	GhiHangMuc GhiHangMuc
	DuAn       DuAnTienDo

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
	if d.GhiHangMuc == nil {
		panic("finance/http: thiếu use case ghi danh mục hạng mục kế hoạch vốn — POST/PATCH/DELETE /api/v1/capital-plan-categories sẽ panic khi có người gọi")
	}
	if d.DuAn == nil {
		panic("finance/http: thiếu kho dự án — các tuyến /api/v1/investment-projects sẽ panic khi có người gọi")
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
	// THE URL NOUN IS SETTLED — the user decided `investment-projects` on 20/09/2026, and both
	// halves of that name were chosen against a specific failure.
	//
	// TOP LEVEL, NOT NESTED UNDER `disbursements`, because the provisional
	// `/api/v1/disbursements/projects` stated the relationship BACKWARDS: a project exists on its
	// own, and a disbursement voucher is the thing that belongs to a project. A path that inverts
	// a business relationship teaches the inversion to whoever builds the screen against it, and
	// they build the data model to match.
	//
	// `investment-projects`, NOT `projects`, because ADR 0011's worked example is `org-units` —
	// an obvious English word that turned out to be the wrong one. `projects` is that kind of
	// word: the day a commune has a "dự án" in another sense — a livelihood project, a
	// digital-transformation project — the noun is already taken, and a URL is not reclaimable
	// once a commune is live on it.
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
	mux.Handle("GET /api/v1/investment-projects",
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
	mux.Handle("GET /api/v1/investment-projects/{id}",
		authz.RequirePermission(d.Checker, "budget.read")(
			http.HandlerFunc(h.ChiTietDuAn)))

	// --- the commune adds a capital plan category of its own -------------------------------------------
	//
	// TIER 1 ONLY, AND THE STORE IS WHAT MAKES THAT TRUE: `nguon` and `ma_nguon_re_nhanh` are
	// LITERALS in the INSERT, not parameters, so there is no value any layer above could pass. The
	// handler additionally answers 400 to a body naming `source` or `tier` — not as the defence,
	// but so a client learns it may not decide provenance instead of watching the field vanish.
	//
	// idem.Required(MoKhiHong), AND WHICH LAYER IS ACTUALLY PROTECTING THIS — the question
	// skills/rest-api-design §4 says to answer at the route. The real guard is
	// `UNIQUE (tenant_id, ma)`, which counts soft-deleted rows: a second row with the same code
	// CANNOT EXIST, whatever happens to Redis. The idempotency key is the second, independent
	// layer — it is what stops a double-submitted form from producing one row and one confusing
	// 409 instead of one row and a replayed 201.
	//
	// MoKhiHong and not DongKhiHong for exactly that reason: with the unique key underneath, a
	// cache outage cannot produce a duplicate catalogue row, so refusing a commune administrator
	// mid-configuration would be paying with an outage for a risk that is already covered. The
	// legal-consequence cases the skill reserves DongKhiHong for — money, issued document numbers,
	// closing a commitment to a citizen — are not this.
	//
	// 409 AND NOT 403 for a full catalogue or a taken code: the caller holds `admin.lookup` and is
	// allowed to manage the list. What is refused is this value against the state of the data.
	//
	// @summary  Thêm một hạng mục kế hoạch vốn của riêng xã vào danh mục
	// @screen   14-cau-hinh §5
	// @request  themHangMucVao
	// @reply    201 hangMucRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/capital-plan-categories",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.ThemHangMuc))))

	// --- the commune edits one row --------------------------------------------------------------
	//
	// PATCH AND NOT PUT: three of the four editable fields have a meaningful zero, so a full
	// replacement cannot tell "not mentioned" from "set to zero" — see suaHangMucVao.
	//
	// WHAT EACH TIER ALLOWS IS DECIDED PER FIELD AND PER TRANSITION, not per route: relabel and
	// reorder at every tier, disable at tiers 1 and 2, and `ma` nowhere. The refusal is a 409
	// naming the tier, and the trigger refuses the same thing underneath (ADR 0024).
	//
	// idem.KhongCan, AND THE REASON IS A PROPERTY OF THE USE CASE RATHER THAN A HOPE: app.Sua
	// compares the row it read against the row it would write and, when nothing moved, writes
	// NOTHING — no UPDATE and no audit entry. So the same request sent twice leaves one row in one
	// state and one entry in the ledger. Were that comparison removed, this declaration would
	// become a lie and the second request would file a second entry saying nothing changed.
	//
	// @summary  Sửa nhãn, thứ tự, trạng thái dùng hoặc đặt mặc định cho một hạng mục kế hoạch vốn
	// @screen   14-cau-hinh §5
	// @request  suaHangMucVao
	// @reply    200 hangMucRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/capital-plan-categories/{id}",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("sửa là ghi đè một trạng thái đã biết; app.Sua không ghi gì khi không có trường nào đổi, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.SuaHangMuc))))

	// --- the commune retires one of its own rows ------------------------------------------------
	//
	// THIS IS A SOFT DELETE AND THE METHOD IS THE ONLY THING THAT SAYS OTHERWISE. The row stays,
	// carrying `deleted_at`, `deleted_by` and `delete_reason` (rule 7, invariant 1), and its `ma`
	// stays taken forever — an issued code is never reissued, because plan lines hold it as
	// a value and nothing rewrites them. DELETE is still the right method: the resource is gone
	// from every read path, which is what the caller is asking for.
	//
	// TIER 1 ONLY. A `he-thong` row answers 409 and is told to use `Tắt` instead — the same
	// refusal the trigger makes, arriving first and in a sentence somebody can act on.
	//
	// A BODY ON A DELETE, and the alternative was worse: the reason is mandatory, and the query
	// string would put free text about a government record into every access log and proxy cache.
	//
	// idem.KhongCan — deleting an already-deleted row is a 404 either way, and the second request
	// cannot overwrite who deleted it or why: the UPDATE carries `AND deleted_at IS NULL`.
	//
	// @summary  Xoá mềm một mục danh mục do xã tự thêm, kèm lý do bắt buộc
	// @screen   14-cau-hinh §5
	// @request  xoaHangMucVao
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/capital-plan-categories/{id}",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("xoá một dòng đã xoá cho cùng một kết quả: câu UPDATE mang `AND deleted_at IS NULL` nên lần thứ hai không ghi đè được người xoá và lý do")(
				http.HandlerFunc(h.XoaHangMuc))))
}
