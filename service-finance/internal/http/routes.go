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

// GhiDuAn is the WRITE half of the investment project register, and it is its own interface rather
// than three more methods on DuAnTienDo — the same reason GhiChungTu is separate from DuAnTienDo and
// GhiHangMuc from HangMucKeHoachVonDanhMuc.
//
// The reads are store calls; each of these three opens a TRANSACTION and writes an audit entry
// inside it (rule 6, invariant 3). Behind one interface a future caller would reach for whichever
// method was nearest and could end up writing the project outside a transaction — the exact defect
// core/audit was shaped to make impossible. Two interfaces, two obligations, visible at the point of
// use.
//
// IT MATTERS MORE HERE THAN ON THE CATALOGUE, because creating a project is TWO writes: the project
// and its funding allocation lines (§9). A project committed with half its allocation is a §6 card
// that is wrong with nothing on any screen saying so.
type GhiDuAn interface {
	Them(ctx context.Context, yc app.YeuCauThemDuAn, nguoi audit.Actor) (app.KetQuaThemDuAn, error)
	Sua(ctx context.Context, id string, yc app.YeuCauSuaDuAn, nguoi audit.Actor) (domain.DuAn, error)
	Xoa(ctx context.Context, id, lyDo string, nguoi audit.Actor) error
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

// GhiChungTu is the WRITE half of the disbursement voucher register, and it is its own interface
// rather than more methods on DuAnTienDo for the same reason GhiHangMuc is separate from
// HangMucKeHoachVonDanhMuc.
//
// The reads are store calls; each of these six opens a TRANSACTION and writes an audit entry inside
// it (rule 6, invariant 3). Behind one interface a future caller would reach for whichever method
// was nearest and could end up writing the voucher outside a transaction — the exact defect
// core/audit was shaped to make impossible. Two interfaces, two obligations, visible at the point
// of use.
//
// SIX METHODS AND NOT ONE `DoiTrangThai(op)`: the caller names the operation at the call site, so a
// seventh lifecycle move cannot silently fall into a default branch that allows it. It is also what
// lets each route declare the permission the specification assigns to THAT act — `budget.update`
// for entry and correction, `budget.confirm` for confirmation and the lock.
type GhiChungTu interface {
	Them(ctx context.Context, yc app.YeuCauThemChungTu, nguoi audit.Actor) (domain.ChungTuGiaiNgan, error)
	Sua(ctx context.Context, id string, yc app.YeuCauSuaChungTu, nguoi audit.Actor) (domain.ChungTuGiaiNgan, error)
	Go(ctx context.Context, id, lyDo string, nguoi audit.Actor) error
	XacNhan(ctx context.Context, id string, nguoi audit.Actor) (domain.ChungTuGiaiNgan, error)
	Khoa(ctx context.Context, id string, nguoi audit.Actor) (domain.ChungTuGiaiNgan, error)
	MoKhoa(ctx context.Context, id, lyDo string, nguoi audit.Actor) (domain.ChungTuGiaiNgan, error)
}

// NguongCham is the commune's own slow-project warning threshold (§13 rule 5, migration 0005).
//
// AN INTERFACE AT THE POINT OF USE, like the two reads above, and for one extra reason worth
// stating: this value decides whether a commune's KPI card reads "29 dự án chậm" or "0", so the two
// cases a test has to be able to reach — the commune has set a figure, and the commune has set
// nothing — must be reachable without a PostgreSQL. There is none in this repository's build
// environment, so "needs a database" means "never runs", and this is precisely the read that must
// not silently fall back to a default.
type NguongCham interface {
	NguongCanhBaoCham(ctx context.Context, nam int) (domain.NguongCanhBaoCham, error)
}

// NganSachDoc is the commune's budget board, for GET /api/v1/budget-sheets and
// GET /api/v1/budget-indicators.
//
// AN INTERFACE DECLARED AT THE POINT OF USE, like the reads above and for the same reason: the
// properties these routes exist to hold — the permission declaration, the commune check ahead of
// any read, and above all a MISSING FIGURE ARRIVING AS A SENTENCE RATHER THAN AS 0 — have to be
// testable without a PostgreSQL, or they get tested once and then never again. There is none
// reachable from this repository's build environment.
//
// ONE METHOD, AND THE INDICATOR ROUTE CALLS IT TWICE. `Cân đối thu - chi` is the revenue sheet's
// `Thu xã hưởng` minus the expenditure sheet's `Chi ngân sách` (ADR 0035 #32), so it needs both
// sheets of one year. A dedicated store method would put that subtraction in SQL, where the ADR's
// reasoning could not be read and the two candidate revenue columns are one edit apart.
type NganSachDoc interface {
	BangDayDu(ctx context.Context, nam int, loai domain.LoaiBang) (domain.BangDayDu, error)
}

// GhiNganSach is the WRITE half of the budget board, and it is its own interface rather than more
// methods on NganSachDoc — the same reason GhiChungTu is separate from DuAnTienDo.
//
// The reads are store calls; each of these six opens a TRANSACTION and writes an audit entry inside
// it (rule 6, invariant 3). Behind one interface a future caller would reach for whichever method
// was nearest and could end up writing the row outside a transaction.
//
// SEVEN METHODS AND NOT ONE `Sua(op)`: the caller names the act at the call site, so an eighth cannot
// silently fall into a default branch that allows it. It is also what lets each route declare the
// permission that act deserves — and DatDongTong in particular is NOT an edit: it decides which row
// the commune's reported total is read from (ADR 0035 §A).
type GhiNganSach interface {
	TaoBang(ctx context.Context, yc app.YeuCauTaoBang, nguoi audit.Actor) (domain.BangNganSach, error)
	GoBang(ctx context.Context, id, lyDo string, nguoi audit.Actor) error
	SuaBang(ctx context.Context, id string, yc app.YeuCauSuaBang, nguoi audit.Actor) (domain.BangNganSach, error)
	ThemKhoanMuc(ctx context.Context, yc app.YeuCauThemKhoanMuc, nguoi audit.Actor) (domain.KhoanMucNganSach, error)
	SuaKhoanMuc(ctx context.Context, id string, yc app.YeuCauSuaKhoanMuc, nguoi audit.Actor) (domain.KhoanMucNganSach, error)
	GoKhoanMuc(ctx context.Context, id, lyDo string, nguoi audit.Actor) error
	DatDongTong(ctx context.Context, id string, nguoi audit.Actor) (domain.KhoanMucNganSach, error)
}

type Deps struct {
	Checker     authz.Checker
	HangMuc     HangMucKeHoachVonDanhMuc
	GhiHangMuc  GhiHangMuc
	DuAn        DuAnTienDo
	GhiDuAn     GhiDuAn
	GhiChungTu  GhiChungTu
	Nguong      NguongCham
	NganSach    NganSachDoc
	GhiNganSach GhiNganSach

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
	if d.GhiDuAn == nil {
		panic("finance/http: thiếu use case ghi dự án đầu tư — POST/PATCH/DELETE /api/v1/investment-projects sẽ panic khi có người gọi")
	}
	if d.GhiChungTu == nil {
		panic("finance/http: thiếu use case ghi chứng từ giải ngân — các tuyến /api/v1/disbursements sẽ panic khi có người gọi")
	}
	if d.Nguong == nil {
		// REFUSED AT CONSTRUCTION RATHER THAN DEFAULTED AT REQUEST TIME, and that is the whole point
		// of migration 0005. A nil store here would have to fall back to the software's 10 points on
		// every read — which is exactly the state the migration was written to end: every commune
		// silently on the vendor's number, with nothing on the screen saying so. A process that
		// refuses to start is a deployment that fails visibly.
		panic("finance/http: thiếu kho cấu hình giải ngân — ngưỡng cảnh báo chậm sẽ im lặng về mặc định của phần mềm")
	}
	if d.NganSach == nil {
		panic("finance/http: thiếu kho bảng thu-chi ngân sách — các tuyến /api/v1/budget-sheets sẽ panic khi có người gọi")
	}
	if d.GhiNganSach == nil {
		panic("finance/http: thiếu use case ghi thu-chi ngân sách — các tuyến ghi ngân sách sẽ panic khi có người gọi")
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

	// --- the commune enters, corrects and withdraws its own investment projects ------------------
	//
	// THE TWO PERMISSION KEYS BELOW ARE THE SPECIFICATION'S OWN, checked rather than assumed:
	// docs/ui-ux/06-giai-ngan.md:202 — "`budget.update` (nhập/sửa), `budget.confirm` (xác nhận,
	// khoá), `budget.read`". Both are seeded at service-identity/migrations/0001_init.sql:282-284
	// (`budget.confirm` :282, `budget.update` :284), so a commune administrator can actually tick
	// them on the Phân quyền screen. NO NEW KEY WAS INVENTED (rule 5, invariant 3c): a key no
	// migration seeds is a right nobody can grant, so the route would answer 403 to every account
	// forever while every test stayed green — this repository carried three such keys for several
	// sessions. Needing a `budget.project.*` the `quyen` table does not have would be a finding for
	// open question #27, never an INSERT.
	//
	// Written out as string literals at every call site because tools/apidoc refuses anything that
	// is not one there — a route whose key it cannot read is a route absent from
	// kb/20-contracts/openapi.json, which is the contract the admin web builds against (ADR 0014).

	// `budget.update` — "Cập nhật giải ngân". §9's modal is opened from the disbursement screen's
	// own header (`+ Thêm dự án`) and entering a project's capital plan is data entry by the
	// accountant, which is precisely what that key is for. It is the same key §8.2 assigns to
	// entering a voucher, and a project is the thing a voucher is entered against.
	//
	// idem.Required(MoKhiHong), AND THE QUESTION skills/rest-api-design §4 SAYS TO ANSWER AT THE
	// ROUTE — which layer is actually protecting this. The real guard is `UNIQUE (tenant_id, ma)`
	// (0004:216-221), which counts soft-deleted rows: a second project with the same code CANNOT
	// EXIST, whatever happens to Redis. The idempotency key is the second, independent layer — it is
	// what stops a double-submitted form from producing one project and one confusing 409 instead of
	// one project and a replayed 201.
	//
	// MoKhiHong AND NOT DongKhiHong, AND THE CONTRAST WITH POST /api/v1/disbursements IS THE WHOLE
	// ARGUMENT. That route takes DongKhiHong because two genuine payments to one company on one day
	// for one amount are a real thing, so NO uniqueness constraint could tell them from a
	// double-submitted form — with nothing underneath, a cache outage plus a double click is money
	// counted twice in "đã giải ngân". Here the code is mandatory and unique for ever, so a duplicate
	// project is impossible by construction; refusing a commune mid-entry during a Redis outage would
	// be paying with an outage for a risk that is already covered.
	//
	// ⚠ `code` IS REQUIRED AND §9's `☑ Tự sinh mã` IS NOT IMPLEMENTED. The specification gives two
	// incompatible formats for that column (`DA01, DA02…` in §9, `DA-2026-be-tong-hoa-duong-ngo-xo-2`
	// in §7.2 · §8 · §11) and no scope for the sequence — `UNIQUE (tenant_id, ma)` has no `nam` in
	// it, so a per-year sequence collides across years while a per-commune one exhausts `DA01..DA99`
	// inside two budget years (§14's commune carries 63 projects in ONE year). A project code is an
	// ISSUED CODE: rule 7, invariant 3 forbids reissuing one and forbidden #4 forbids renumbering
	// one. Refusing writes nothing and can be loosened with one function; generating cannot be taken
	// back. This is a finding for the user — domain.ErrThieuMaDuAn carries the full argument.
	//
	// 409 AND NOT 403 for a code already issued: the caller holds `budget.update` and is allowed to
	// enter projects. What is refused is this value against the state of the data.
	//
	// @summary  Thêm một dự án đầu tư cho năm ngân sách, kèm phân bổ nguồn vốn nếu xã khai
	// @screen   06-giai-ngan §9
	// @request  themDuAnVao
	// @reply    201 duAnGhiRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/investment-projects",
		authz.RequirePermission(d.Checker, "budget.update")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.ThemDuAn))))

	// PATCH AND NOT PUT: several fields have a meaningful zero — a description cleared to "", an
	// officer unassigned back to "Chưa phân công", a plan revised down to 0 — so a full replacement
	// cannot tell "not mentioned" from "cleared", and a dialog editing only the name would silently
	// unassign the officer in charge.
	//
	// `code` AND `year` ANSWER 400, REFUSED RATHER THAN IGNORED, for two different reasons: a code
	// that has been issued is never renumbered (rule 7, forbidden #4), and each budget year is its
	// own set of projects (§13 rule 8), so moving one takes its whole plan and every voucher filed
	// against it out of one year's totals and into another's. Ignoring either would leave the client
	// believing the change landed while every screen still showed the old value.
	//
	// ⚠ A PROJECT WITH CONFIRMED OR LOCKED VOUCHERS IS EDITED NORMALLY, AND THAT IS DELIBERATE.
	// ADR 0036 decided that a CONFIRMED VOUCHER returns to `Kế toán nhập` when ITS OWN figures move;
	// that decision is about the voucher's own confirmation and does not carry over. A project has no
	// `trang_thai` column and no confirmation on it, so there is nothing here for that rule to act on
	// — and inventing an equivalent would be giving this record a lifecycle the specification never
	// gave it. Revising a plan DOES move the denominator of §3's delay score and §7.2's ratio, which
	// is exactly why the audit entry carries the before/after pair.
	//
	// idem.KhongCan, AND THE REASON IS A PROPERTY OF THE USE CASE RATHER THAN A HOPE: app.Sua
	// compares the project it read against the project it would write and, when nothing moved, writes
	// NOTHING — no UPDATE and no audit entry. So the same request sent twice leaves one row in one
	// state and one entry in the ledger. Were that comparison removed, this declaration would become
	// a lie and the second request would file an entry saying nothing changed.
	//
	// @summary  Sửa hạng mục, tên, mô tả, kế hoạch vốn, đơn vị, cán bộ phụ trách hoặc các mốc thời gian của một dự án đầu tư
	// @screen   06-giai-ngan §8
	// @request  suaDuAnVao
	// @reply    200 duAnGhiRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/investment-projects/{id}",
		authz.RequirePermission(d.Checker, "budget.update")(
			idem.KhongCan("sửa là ghi đè một trạng thái đã biết; app.Sua không ghi gì khi không có trường nào đổi, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.SuaDuAn))))

	// `budget.confirm` ON A REMOVAL, AND THE SPECIFICATION ASSIGNS NONE — the same decision the
	// voucher's `🗑 Gỡ` route had to make, made the same way and for a heavier reason.
	//
	// 06-giai-ngan.md:202 covers `budget.update` for entry/edit and `budget.confirm` for
	// confirm/lock; withdrawing a PROJECT is listed with no key at all. The choice is between the two
	// that exist:
	//
	//	budget.update    "the person who entered it can take it back". True for a typo caught in the
	//	                 same minute — and it is also the key every accountant holds, so it would make
	//	                 withdrawing a whole year's capital plan line the same authority as typing one.
	//	budget.confirm   CHOSEN. A project's `ke_hoach_von_nam` is inside "KẾ HOẠCH VỐN NĂM" on §3 and
	//	                 inside its category's row on §5 from the moment it is entered, so removing one
	//	                 CHANGES A FIGURE THAT HAS ALREADY BEEN READ off a screen and possibly reported
	//	                 upward — and it takes the project's funding allocation lines with it, moving
	//	                 two more figures on §6.
	//
	// CHOSEN IN THE DIRECTION THAT CAN BE LOOSENED LATER WITH ONE LINE and cannot be tightened later
	// at all: widening it to `budget.update` the day the customer says so costs one edit, while
	// narrowing it afterwards means every removal already made was made under the wrong authority.
	// Needing a third key — a `budget.delete` the `quyen` table does not have — would be a finding for
	// open question #27, never an INSERT (rule 5, invariant 3c).
	//
	// A BODY ON A DELETE, and the alternative was worse: the reason is mandatory (rule 7, invariant 1
	// names `delete_reason`), and the query string would put free text about a public authority's
	// spending into every access log and proxy cache.
	//
	// ⚠ 409 WHEN THE PROJECT STILL HAS LIVE VOUCHERS, AND THAT IS AN OPEN QUESTION ANSWERED IN THE
	// ONLY DIRECTION THAT WRITES NOTHING. The specification says nothing about removing a project
	// that has vouchers filed against it, and ADR 0037 settled the same SHAPE for the task tree — a
	// different record, whose answer does not carry over. Leaving the vouchers live would strand
	// their money: every read path drops the project, so the figures disappear from §3 and §5 while
	// the rows sit in `chung_tu_giai_ngan`, and the commune's own totals stop agreeing with the sum
	// of its own vouchers. Cascading would soft delete somebody else's payment records — including
	// LOCKED ones, which `chung_tu_da_khoa` refuses outright — as a side effect. store
	// .ErrDuAnConChungTu carries the full argument. This is a finding for the user.
	//
	// idem.KhongCan — removing an already-removed project is a 404 either way, and the second request
	// cannot overwrite who removed it or why: the UPDATE carries `AND deleted_at IS NULL`.
	//
	// @summary  Xoá mềm một dự án đầu tư kèm lý do bắt buộc — từ chối khi dự án còn chứng từ giải ngân
	// @screen   06-giai-ngan §7
	// @request  xoaDuAnVao
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/investment-projects/{id}",
		authz.RequirePermission(d.Checker, "budget.confirm")(
			idem.KhongCan("xoá một dự án đã xoá cho cùng một kết quả: câu UPDATE mang `AND deleted_at IS NULL` nên lần thứ hai không ghi đè được người xoá và lý do")(
				http.HandlerFunc(h.XoaDuAn))))

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

	// --- the disbursement voucher register (§8.2) -----------------------------------------------
	//
	// THE URL NOUN IS `disbursements` — kb/00-foundation/ubiquitous-language.md:145 settles
	// `giai_ngan` -> `disbursements`, and a voucher IS one recorded disbursement.
	//
	// TOP LEVEL, NOT NESTED UNDER `investment-projects/{id}`, and the reason is the SAME row that
	// rejected the inverse nesting (:146). A path declares ownership, and a voucher does belong to a
	// project — but the four lifecycle routes below act on ONE voucher by its own id, and nesting
	// would put a project id in every one of them that nothing reads and nothing checks. A segment
	// the server ignores is a segment a client will eventually get wrong, and nobody will notice.
	// The project is a FIELD of the create body, where it is validated against this commune's live
	// projects (fistore.ErrKhongThayDuAnCuaChungTu).
	//
	// THE TWO PERMISSION KEYS BELOW ARE THE SPECIFICATION'S OWN, checked rather than assumed:
	// docs/ui-ux/06-giai-ngan.md:202 — "`budget.update` (nhập/sửa), `budget.confirm` (xác nhận,
	// khoá), `budget.read`". Both are seeded at service-identity/migrations/0001_init.sql:282-284,
	// so a commune administrator can actually tick them. NO NEW KEY WAS INVENTED (rule 5, invariant
	// 3c): a key no migration seeds is a right nobody can grant, so the route would answer 403 to
	// every account forever while every test stayed green — this repository carried three such keys
	// for several sessions. Written out as string literals at every call site because tools/apidoc
	// refuses anything that is not one there.

	// `budget.update` — "Cập nhật giải ngân". Entering a payment is data entry by the accountant,
	// which is precisely what §8.2 assigns to this key.
	//
	// idem.Required(DongKhiHong), AND THIS IS THE FIRST ROUTE IN THIS SERVICE THAT EARNS IT. The
	// catalogue's POST takes MoKhiHong because `UNIQUE (tenant_id, ma)` makes a duplicate row
	// impossible whatever happens to Redis. THERE IS NO SUCH KEY HERE and there must not be one: two
	// genuine payments to the same company, on the same day, for the same amount are a real thing
	// (two instalments, two lines of one treasury document), so no uniqueness constraint could tell
	// them from a double-submitted form. With nothing underneath, a cache outage plus a double click
	// is a voucher counted twice in "đã giải ngân" — money on a figure a decision quotes. That is
	// exactly the legal-consequence case skills/rest-api-design reserves DongKhiHong for: answer 503
	// and let the commune retry, rather than take a payment record on trust.
	//
	// @summary  Ghi nhận một chứng từ giải ngân cho dự án đầu tư
	// @screen   06-giai-ngan §8.2
	// @request  themChungTuVao
	// @reply    201 chungTuRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/disbursements",
		authz.RequirePermission(d.Checker, "budget.update")(
			idem.Required(idem.DongKhiHong)(
				http.HandlerFunc(h.ThemChungTu))))

	// PATCH AND NOT PUT: `counterparty` and `voucher_no` are optional and their empty string is a
	// meaningful value, so a full replacement cannot tell "not mentioned" from "cleared".
	//
	// A LOCKED VOUCHER ANSWERS 409 WITH §13 RULE 3'S OWN SENTENCE, and that is this route's real
	// job. The `chung_tu_da_khoa` trigger refuses the same UPDATE underneath (0004:141-168) with an
	// English exception naming a constraint; what an accountant is owed is "chứng từ đã khoá thì
	// không sửa, không gỡ — phải mở khoá trước (quyền `budget.confirm`)". The trigger is the floor,
	// this is the sentence, and the sentence arrives first.
	//
	// ⚠ EDITING A CONFIRMED VOUCHER SENDS IT BACK TO `Kế toán nhập`, AND THE 200 SAYS SO — the
	// response carries `status: "ke-toan-nhap"` and an empty `confirmed_by`. The customer's sentence
	// is the whole argument: *"lãnh đạo xác nhận những con số kia, không phải những con số này"*
	// (`../vigov-require` commit `c3f4d6a`). It fills a gap open questions #29 and #30 left — they
	// settled unlocking and refunds and say nothing about a confirmation whose figures moved — and
	// takes nothing back from either. A LOCKED voucher is still refused outright (409); the only way
	// to a frozen figure is still the unlock route, with a reason, by somebody else.
	//
	// idem.KhongCan, AND THE REASON IS A PROPERTY OF THE USE CASE RATHER THAN A HOPE: app.Sua
	// compares the voucher it read against the voucher it would write and, when nothing moved,
	// writes NOTHING — no UPDATE, no state change and no audit entry. So the same request sent twice
	// leaves one row in one state and one entry in the ledger. Were that comparison removed, this
	// declaration would become a lie AND a repeated PATCH would strip a leader's confirmation off a
	// voucher nobody actually edited.
	//
	// @summary  Sửa ngày chi, số tiền, nội dung, đối tác hoặc số chứng từ của một chứng từ chưa khoá — chứng từ đã xác nhận sẽ về nháp
	// @screen   06-giai-ngan §8.2
	// @request  suaChungTuVao
	// @reply    200 chungTuRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/disbursements/{id}",
		authz.RequirePermission(d.Checker, "budget.update")(
			idem.KhongCan("sửa là ghi đè một trạng thái đã biết; app.Sua không ghi gì khi không có trường nào đổi, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.SuaChungTu))))

	// `budget.confirm` ON A REMOVAL, AND THE SPECIFICATION ASSIGNS NONE — this is the decision
	// migration 0005:39-42 deferred to the route, so here it is.
	//
	// 06-giai-ngan.md:202 covers `budget.update` for entry/edit and `budget.confirm` for
	// confirm/lock; the `🗑 Gỡ` button on the same screen is listed with no key at all. The choice is
	// between the two that exist:
	//
	//	budget.update    "the person who entered it can take it back". True for a typo caught in the
	//	                 same minute — and it is also the key every accountant holds, so it makes
	//	                 removing a payment record the same authority as typing one.
	//	budget.confirm   CHOSEN. A voucher's money is already inside "đã giải ngân" from the moment
	//	                 it is entered (§11 counts `ke-toan-nhap`), so removing one CHANGES A FIGURE
	//	                 THAT HAS ALREADY BEEN READ off a screen and possibly reported upward. That
	//	                 is the same class of act as freezing one, which is what this key is for.
	//
	// CHOSEN IN THE DIRECTION THAT CAN BE LOOSENED LATER WITH ONE LINE and cannot be tightened later
	// at all: widening it to `budget.update` the day the customer says so costs one edit, while
	// narrowing it afterwards means every removal already made was made under the wrong authority.
	// Needing a third key — a `budget.delete` the `quyen` table does not have — would be a finding
	// for open question #27, never an INSERT (rule 5, invariant 3c).
	//
	// A BODY ON A DELETE, and the alternative was worse: the reason is mandatory (rule 7, invariant
	// 1 names `delete_reason`), and the query string would put free text about a public authority's
	// spending into every access log and proxy cache.
	//
	// idem.KhongCan — removing an already-removed voucher is a 404 either way, and the second
	// request cannot overwrite who removed it or why: the UPDATE carries `AND deleted_at IS NULL`.
	//
	// @summary  Gỡ mềm một chứng từ giải ngân, kèm lý do bắt buộc
	// @screen   06-giai-ngan §8.2
	// @request  goChungTuVao
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/disbursements/{id}",
		authz.RequirePermission(d.Checker, "budget.confirm")(
			idem.KhongCan("gỡ một chứng từ đã gỡ cho cùng một kết quả: câu UPDATE mang `AND deleted_at IS NULL` nên lần thứ hai không ghi đè được người gỡ và lý do")(
				http.HandlerFunc(h.GoChungTu))))

	// `confirmation` IS A NOMINALISED SUB-RESOURCE, NOT THE VERB `confirm`: a verb in a path is what
	// skills/rest-api-design forbids and `rest_api_guard` reports, and the nominalisation it names
	// for this verb is exactly `confirmation`. The act becomes a record you can point at.
	//
	// POST AND NO DELETE, DELIBERATELY. A confirmation cannot be taken back — domain.ChoXacNhan
	// refuses a second one, because overwriting `nguoi_xac_nhan_id` would be editing a historical
	// fact (rule 7, forbidden #5). The way back from `Đã xác nhận` does not exist and is not being
	// invented here; the way back from `Đã khoá` does, and it is the route below.
	//
	// idem.KhongCan — the second identical request finds the voucher already `Đã xác nhận` and
	// answers 409 without writing anything. One confirmation, one entry, whatever the network did.
	//
	// @summary  Xác nhận một chứng từ giải ngân (`Kế toán nhập` → `Đã xác nhận`)
	// @screen   06-giai-ngan §8.2
	// @reply    200 chungTuRa
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/disbursements/{id}/confirmation",
		authz.RequirePermission(d.Checker, "budget.confirm")(
			idem.KhongCan("xác nhận lần thứ hai gặp chứng từ đã ở `da-xac-nhan` và trả 409 mà không ghi gì — một lần xác nhận, một dòng vết")(
				http.HandlerFunc(h.XacNhanChungTu))))

	// `lockout` IS THE NOUN THIS SYSTEM ALREADY SETTLED FOR THIS EXACT SHAPE — a STATE that POST
	// creates and DELETE removes, rather than a verb in a path. See
	// kb/00-foundation/ubiquitous-language.md:160, which chose it over `disable` / `deactivate` /
	// `suspend` for `POST`/`DELETE /api/v1/staff/{id}/lockout`.
	//
	// ⚠ THAT ROW IS ABOUT A STAFF ACCOUNT AND NOT ABOUT A VOUCHER. The mapping table has no row for
	// "khoá chứng từ", and ADR 0011 says to ask rather than translate on the spot; the noun is being
	// REUSED here by this session on the strength of the shape being identical. It is written into
	// the hand-over as a finding, not buried in a route — and it is still free to change, because no
	// commune is live on this path.
	//
	// LOCKING FROM `Kế toán nhập` IS REFUSED (409) even though §8.2's screen draws both buttons on
	// such a row, and domain.ErrChuaXacNhanThiChuaKhoaDuoc carries the reason: unlocking has to put
	// the voucher back in the state it was in BEFORE the lock, and the row does not store what that
	// was. Requiring the chain makes "before the lock" always `Đã xác nhận`, so an unlock restores
	// exactly what was there and invents nothing. The alternative is one more column that exists
	// only to remember a shortcut the specification does not describe.
	//
	// @summary  Khoá một chứng từ giải ngân (`Đã xác nhận` → `Đã khoá`)
	// @screen   06-giai-ngan §8.2
	// @reply    200 chungTuRa
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/disbursements/{id}/lockout",
		authz.RequirePermission(d.Checker, "budget.confirm")(
			idem.KhongCan("khoá lần thứ hai gặp chứng từ đã ở `da-khoa` và trả 409 mà không ghi gì — người khoá và thời điểm khoá không bị ghi đè")(
				http.HandlerFunc(h.KhoaChungTu))))

	// --- the unlock: the most expensive route in this file ---------------------------------------
	//
	// TWO RULES RIDE ON IT, both settled by THIS PROJECT on 2026-09-22 rather than by the customer
	// (open question #29, migration 0005's header, ADR 0035 §B), and both chosen in the direction
	// that can be loosened later with one line and cannot be tightened later at all:
	//
	//	the reason is MANDATORY       every unlock that has ALREADY HAPPENED is unexplainable
	//	                              otherwise, and no source anywhere can rebuild it. That is
	//	                              exactly the figure an inspection asks about, because it is a
	//	                              figure somebody signed and somebody then changed.
	//	the person who LOCKED it      nobody acts alone on the act that gives themselves room — the
	//	  may NOT unlock it           same shape the customer already settled for the staff register
	//	                              in #13 and #14. It needs a SECOND holder of `budget.confirm`.
	//
	// THE SELF-UNLOCK REFUSAL IS 409 AND NOT 403, and the distinction is not cosmetic: the caller
	// HOLDS `budget.confirm` and is allowed to unlock vouchers. What is refused is this person
	// against THIS row. A 403 would send them to the Phân quyền screen to be granted a permission
	// they already have, where nothing they could do would help.
	//
	// THERE IS NO CEILING ON THE NUMBER OF UNLOCKS, and every one is counted (`so_lan_mo_khoa`,
	// incremented in SQL). A ceiling is a number that belongs to the customer; a count is what lets
	// them choose one later from real figures instead of somebody's guess.
	//
	// A BODY ON A DELETE, for the same reason as the removal above: the reason is mandatory and the
	// query string is a place free text about a reopened financial record must not go.
	//
	// idem.KhongCan — after the first unlock the voucher is `Đã xác nhận`, so a repeat finds nothing
	// locked and answers 409 without writing. The count cannot be incremented twice by one retry.
	//
	// @summary  Mở khoá một chứng từ giải ngân, kèm lý do bắt buộc; người vừa khoá không tự mở lại được
	// @screen   06-giai-ngan §8.2
	// @request  moKhoaVao
	// @reply    200 chungTuRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/disbursements/{id}/lockout",
		authz.RequirePermission(d.Checker, "budget.confirm")(
			idem.KhongCan("mở khoá lần thứ hai gặp chứng từ đã ở `da-xac-nhan` và trả 409 mà không ghi gì — `so_lan_mo_khoa` không tăng hai lần vì một lần thử lại")(
				http.HandlerFunc(h.MoKhoaChungTu))))

	// --- the commune's revenue/expenditure budget board (07-thu-chi-ngan-sach) --------------------
	//
	// THE URL NOUNS ARE THIS SESSION'S AND ARE A FINDING, NOT A DECISION. ADR 0011 says to ASK rather
	// than translate on the spot, and kb/00-foundation/ubiquitous-language.md has no row for
	// `bang_ngan_sach` or `khoan_muc_ngan_sach`. `budget-sheets` / `budget-lines` /
	// `budget-indicators` are used here because the permission group is already `budget.*` and
	// because the screen is a sheet of lines; they are still free to change, since no commune is live
	// on this path. The alternative — waiting — would have left eight routes unbuildable.
	//
	// `budget-lines` IS TOP LEVEL AND NOT NESTED UNDER `budget-sheets/{id}`, and the reason is the one
	// that already rejected `disbursements/projects`: the three routes below act on ONE line by its
	// own id, and nesting would put a sheet id in every one of them that nothing reads and nothing
	// checks. A segment the server ignores is a segment a client will eventually get wrong and nobody
	// will notice. The sheet is a FIELD of the create body, validated against this commune's live
	// sheets.
	//
	// THE THREE PERMISSION KEYS ARE THE SPECIFICATION'S OWN, checked rather than assumed:
	// docs/ui-ux/07-thu-chi-ngan-sach.md:236 — "Quyền: xem/sửa theo nhóm GIẢI NGÂN (`budget.read`,
	// `budget.update`, `budget.confirm`)". All three are seeded at
	// service-identity/migrations/0001_init.sql:282-284, so a commune administrator can actually tick
	// them. NO NEW KEY WAS INVENTED (rule 5, invariant 3c): a key no migration seeds is a right
	// nobody can grant, so the route would answer 403 to every account forever while every test
	// stayed green. Written out as string literals at every call site because tools/apidoc refuses
	// anything that is not one there.
	//
	// ⚠ THE SPECIFICATION ASSIGNS THE GROUP AND NOT THE ACTS. Which of the three guards WHICH button
	// is this session's reading, and the two that are not obvious are written out at their routes:
	// removing a line and moving the star both take `budget.confirm`.

	// `budget.read` — reading the commune's own budget figures. NOT AnyAuthenticated, unlike the
	// catalogue route above, and the difference is the data: this is how much the commune plans to
	// take in and spend, line by line, before it is published anywhere.
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Bảng thu hoặc chi của một năm ngân sách: cột, cây khoản mục, số liệu và ô tóm tắt
	// @screen   07-thu-chi-ngan-sach §2 §4
	// @reply    200 bangDayDuRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/budget-sheets",
		authz.RequirePermission(d.Checker, "budget.read")(
			http.HandlerFunc(h.DocBangNganSach)))

	// ITS OWN ROUTE BECAUSE `Cân đối thu - chi` NEEDS BOTH SHEETS (ADR 0035 #32: revenue's
	// `Thu xã hưởng` minus expenditure's `Chi ngân sách`), so it cannot be a field on either sheet's
	// response without that route quietly reading the other one. This is the block §9 rule 6 feeds to
	// `/tong-quan` and `/bao-cao`.
	//
	// IT ANSWERS 200 WITH REASONS WHEN THE DATA IS INCOMPLETE, never 404 and never 0. A commune that
	// has not entered its expenditure sheet has no balance, and that is a fact about the data rather
	// than a failure of the request — ADR 0035 §A: "để trống kèm lý do, không đặt mặc định".
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Ba chỉ số ngân sách của một năm: thu đạt dự toán, chi đạt dự toán, cân đối thu - chi
	// @screen   07-thu-chi-ngan-sach §9 quy tắc 6
	// @reply    200 chiSoNamRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/budget-indicators",
		authz.RequirePermission(d.Checker, "budget.read")(
			http.HandlerFunc(h.DocChiSoNganSach)))

	// `budget.update` — creating the year's sheet with its columns is data entry by the accountant,
	// which is what §9 rule 7 assigns this key to.
	//
	// idem.Required(DongKhiHong), AND IT EARNS IT FOR THE REASON THE VOUCHER POST DOES. There is no
	// unique key that could tell a double-submitted form from a deliberate second sheet — `lan` makes
	// the second one legitimate by construction, because a commune really does reload a year after a
	// `🗑 Gỡ`. With nothing underneath, a cache outage plus a double click is TWO live sheets for one
	// year, which is the state `CoBangConSong` refuses and which a retry would otherwise create by
	// racing it. A 503 costs a commune one retry on an act performed twice a year.
	//
	// @summary  Tạo bảng thu hoặc chi cho một năm ngân sách, kèm bộ cột của biểu
	// @screen   07-thu-chi-ngan-sach §3 §7
	// @request  taoBangVao
	// @reply    201 bangRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/budget-sheets",
		authz.RequirePermission(d.Checker, "budget.update")(
			idem.Required(idem.DongKhiHong)(
				http.HandlerFunc(h.TaoBangNganSach))))

	// `budget.confirm` ON §6's `🗑 Gỡ`, AND THE SPECIFICATION ASSIGNS NONE — the screen's own dialog
	// calls it irreversible. It takes a whole year of figures off every report in one act, including
	// figures already quoted upward, which is the same class of act as freezing a voucher. Chosen in
	// the direction that can be LOOSENED later with one line and cannot be tightened later at all:
	// widening it to `budget.update` the day the customer says so costs one edit, while narrowing it
	// afterwards means every removal already made was made under the wrong authority.
	//
	// A BODY ON A DELETE, and the alternative was worse: the reason is mandatory (rule 7, invariant 1
	// names `delete_reason`), and the query string would put free text about a public authority's
	// budget into every access log and proxy cache.
	//
	// idem.KhongCan — removing an already-removed sheet is a 404 either way, and the second request
	// cannot overwrite who removed it or why: the UPDATE carries `AND deleted_at IS NULL`.
	//
	// @summary  Gỡ mềm cả bảng ngân sách của một năm, kèm lý do bắt buộc
	// @screen   07-thu-chi-ngan-sach §6
	// @request  goVao
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/budget-sheets/{id}",
		authz.RequirePermission(d.Checker, "budget.confirm")(
			idem.KhongCan("gỡ một bảng đã gỡ cho cùng một kết quả: câu UPDATE mang `AND deleted_at IS NULL` nên lần thứ hai không ghi đè được người gỡ và lý do")(
				http.HandlerFunc(h.GoBangNganSach))))

	// `budget.update` — correcting a sheet's title, its cut-off date (`Luỹ kế đến`) and its DISPLAY
	// unit is data entry by the accountant, the same weight as editing a line (§9 rule 7).
	//
	// THE UNIT IS A CLOSED LIST (the customer's decision of 25/09/2026): `dong` | `nghin-dong` |
	// `trieu-dong`, validated in internal/domain. Changing it changes only how the figures are
	// DISPLAYED — every stored figure is đồng and none is converted.
	//
	// idem.KhongCan, FOR THE REASON THE LINE PATCH HAS IT: app.SuaBang compares what it read against
	// what it would write and, when nothing moved, writes nothing — no UPDATE, no audit entry. The
	// same request sent twice leaves one row in one state and one entry in the ledger.
	//
	// A REMOVED SHEET AND ANOTHER COMMUNE'S SHEET ANSWER THE SAME 404 BODY: the locked read excludes
	// soft-deleted rows and is scoped by tenant_id, so neither is reachable at all.
	//
	// @summary  Sửa tiêu đề, đơn vị tính hiển thị hoặc mốc luỹ kế của một bảng ngân sách
	// @screen   07-thu-chi-ngan-sach §1 §2
	// @request  suaBangVao
	// @reply    200 bangRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/budget-sheets/{id}",
		authz.RequirePermission(d.Checker, "budget.update")(
			idem.KhongCan("sửa là ghi đè một trạng thái đã biết; app.SuaBang không ghi gì khi không trường nào đổi, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.SuaBangNganSach))))

	// `budget.update` — `＋ Thêm khoản mục con` and `⊞ Thêm khoản mục cấp cao nhất` (§4.3).
	//
	// idem.Required(DongKhiHong), AND THE COST IS STATED: with no Redis, adding a line answers 503 and
	// the accountant cannot enter data. It is chosen anyway because there is no unique key that could
	// tell a double-submitted form from two genuinely similar lines — two rows both named
	// "Chi quốc phòng" under one parent are a real thing in these forms — so a cache outage plus a
	// double click is a duplicated line inside a parent's total, which is a figure on a report.
	// `MoKhiHong` would be the choice if a uniqueness constraint existed underneath; there is none,
	// and rest-api-design reserves DongKhiHong for exactly this.
	//
	// @summary  Thêm một khoản mục vào cây của bảng ngân sách
	// @screen   07-thu-chi-ngan-sach §4.1 §4.3
	// @request  themDongVao
	// @reply    201 dongRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/budget-lines",
		authz.RequirePermission(d.Checker, "budget.update")(
			idem.Required(idem.DongKhiHong)(
				http.HandlerFunc(h.ThemKhoanMucNganSach))))

	// PATCH AND NOT PUT: `no` is optional and its empty string is a meaningful value, so a full
	// replacement cannot tell "not mentioned" from "cleared" — and the same holds cell by cell, which
	// is why `values` is a map whose `null` CLEARS (§9 rule 4).
	//
	// A FIGURE TYPED INTO A LINE THAT HAS CHILDREN ANSWERS 409 WITH THE CUSTOMER'S OWN RULE. That is
	// this route's real job: anh Hà settled on 06/09/2026 that a parent always sums its children, and
	// blocking it on the screen only leaves it open to an import script written next year. The price
	// the customer accepted comes with it and is written into migration 0006 — on the rows where a
	// real form has a parent that is NOT the sum of its children (khoản mục ngoài cân đối, the
	// `Trong đó:` lines), the number on the screen will differ from the paper the commune signed.
	//
	// idem.KhongCan, AND THE REASON IS A PROPERTY OF THE USE CASE RATHER THAN A HOPE: app.SuaKhoanMuc
	// compares what it read against what it would write and, when nothing moved, writes NOTHING — no
	// UPDATE, no cell, no audit entry. So the same request sent twice leaves one row in one state and
	// one entry in the ledger.
	//
	// @summary  Sửa số thứ tự, tên hoặc các ô số của một khoản mục chưa có dòng con
	// @screen   07-thu-chi-ngan-sach §4.1
	// @request  suaDongVao
	// @reply    200 dongRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/budget-lines/{id}",
		authz.RequirePermission(d.Checker, "budget.update")(
			idem.KhongCan("sửa là ghi đè một trạng thái đã biết; app.SuaKhoanMuc không ghi gì khi không có trường và không có ô nào đổi, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.SuaKhoanMucNganSach))))

	// `budget.confirm` ON `🗑 Gỡ khoản mục`, for the same reason the sheet's removal takes it and
	// chosen the same way — tight now, loosened later with one line if the customer says so. A line's
	// figure is inside the summary card and inside both indicators from the moment it is entered, so
	// removing one CHANGES A FIGURE THAT HAS ALREADY BEEN READ off a screen.
	//
	// ⚠ THE COST IS REAL AND IS STATED RATHER THAN DISCOVERED: a commune with one accountant cannot
	// take back a line they have just added without somebody holding `budget.confirm`. They can edit
	// it, which covers the ordinary typo. If the customer finds that too tight, the correction is one
	// literal on this line.
	//
	// A LINE WITH CHILDREN ANSWERS 409, not a cascade. A cascade takes a whole branch off every total
	// in one click, and these rows are archival — "undo" is not a button, it is re-entering them.
	//
	// idem.KhongCan — removing an already-removed line is a 404 either way, and the second request
	// cannot overwrite who removed it or why.
	//
	// @summary  Gỡ mềm một khoản mục chưa có dòng con, kèm lý do bắt buộc
	// @screen   07-thu-chi-ngan-sach §4.1
	// @request  goVao
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/budget-lines/{id}",
		authz.RequirePermission(d.Checker, "budget.confirm")(
			idem.KhongCan("gỡ một khoản mục đã gỡ cho cùng một kết quả: câu UPDATE mang `AND deleted_at IS NULL` nên lần thứ hai không ghi đè được người gỡ và lý do")(
				http.HandlerFunc(h.GoKhoanMucNganSach))))

	// --- the star: the most consequential route on this screen ------------------------------------
	//
	// `headline` IS A NOMINALISED SUB-RESOURCE, NOT A VERB IN A PATH — the shape `lockout` already
	// uses. POST creates the state on one row; the store clears it from every other row of the sheet
	// in the same transaction, which is what makes the mark a RADIO.
	//
	// `budget.confirm` AND NOT `budget.update`, AND THIS IS THE DECISION TO READ TWICE. Marking a row
	// does not change one figure: it changes WHICH ROW EVERY SUMMARY CELL AND BOTH INDICATORS ARE
	// READ FROM, and those are the numbers that go into a document sent to a higher authority. ADR
	// 0035 §A exists because getting it wrong is not visible — the thu sheet has two nested top-level
	// rows and the chi sheet has `Tổng số` beside A…E, so a wrong star produces a complete, plausible,
	// double-counted report. That is the weight of a confirmation, not of data entry.
	//
	// NO DELETE ROUTE, DELIBERATELY. §4.1 draws the star as something that MOVES ("bấm ngôi sao ở đầu
	// một dòng khác để đổi"); a sheet that HAD a total and then deliberately had none is a state
	// nothing on that screen asks for. A sheet loses its total only by the marked row being removed,
	// which releases the flag — and the summary card then says so in a sentence rather than showing 0.
	//
	// idem.KhongCan — marking the same row twice leaves the same one row marked and writes the same
	// state; the second entry in the ledger records an act that really was performed twice.
	//
	// @summary  Đánh dấu một khoản mục là dòng tổng của bảng — số tóm tắt và chỉ số đọc từ dòng này
	// @screen   07-thu-chi-ngan-sach §4.1 §5 quy tắc 5
	// @reply    200 dongRa
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/budget-lines/{id}/headline",
		authz.RequirePermission(d.Checker, "budget.confirm")(
			idem.KhongCan("đánh dấu lại đúng dòng đang là dòng tổng để lại đúng một dòng được đánh dấu và đúng trạng thái ấy")(
				http.HandlerFunc(h.DatDongTongNganSach))))
}
