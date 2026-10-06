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

	// AllocationsOfYear feeds the list's funding chip — ONE read for the whole page, keyed by project
	// id, under the same filter as DanhSach (no read per project).
	AllocationsOfYear(ctx context.Context, loc fistore.LocDuAn) (map[string][]domain.ProjectAllocation, error)
	// AllocationsOfProject feeds the detail screen's "Giải ngân theo nguồn vốn" block.
	AllocationsOfProject(ctx context.Context, projectID string) ([]domain.ProjectAllocation, error)
	// VouchersOfProject feeds §8.2's voucher tab: the project's live vouchers, newest payment first.
	// fistore.ErrKhongThayDuAn for a project not in this commune.
	VouchersOfProject(ctx context.Context, projectID string) ([]domain.ProjectVoucher, error)
	// DisbursedByMonth feeds the cumulative curves (§4 for the whole year when projectID is "", §8.3
	// for one project): ONE read of at most 13 buckets, never a read per project.
	DisbursedByMonth(ctx context.Context, year int, projectID string) (domain.DisbursedByMonth, error)
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
	Sua(ctx context.Context, id string, yc app.YeuCauSuaDuAn, nguoi audit.Actor) (app.ProjectEditResult, error)
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

	// DotCuaKhoanMuc is the `⇄` dialog's list (GET /api/v1/budget-lines/{id}/entries).
	DotCuaKhoanMuc(ctx context.Context, khoanMucID string) (domain.DotCuaKhoanMuc, error)

	// BudgetPeriodClosesOfYear is the close history of one year (GET /api/v1/budget-period-closes).
	BudgetPeriodClosesOfYear(ctx context.Context, year int) ([]domain.BudgetPeriodClose, error)
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
	GhiDot(ctx context.Context, yc app.YeuCauGhiDot, nguoi audit.Actor) (domain.DotThuChi, error)
	GoDot(ctx context.Context, id, lyDo string, nguoi audit.Actor) error

	// Budget period close (migration 0012) — app/budget_period_close.go.
	CloseBudgetPeriod(ctx context.Context, req app.BudgetPeriodCloseRequest, nguoi audit.Actor) (domain.BudgetPeriodClose, error)
	ReopenBudgetPeriodClose(ctx context.Context, code, reason string, nguoi audit.Actor) (domain.BudgetPeriodClose, error)
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

	// CapitalPlanCategoryImports is the Excel import of the HangMuc catalogue (ADR 0059 §3), three
	// routes in routes_catalogue_import.go. See CatalogueImporting.
	CapitalPlanCategoryImports CatalogueImporting

	// DisbursementImports is the Excel import of disbursement vouchers (§10), three routes in
	// routes_disbursement_import.go. *app.DisbursementImporter in production. Refused when missing.
	DisbursementImports DisbursementImporting

	// AuditLog reads this service's own `audit_log` for the "Xem nhật ký hệ thống" screen (ADR 0054).
	// *audit.Log in production. Refused at construction when missing.
	AuditLog AuditLogReader

	// SystemMessages is "Lời hệ thống" for the sentences this service raises (migration 0010).
	// *app.SystemMessages in production. Refused at construction when missing.
	SystemMessages SystemMessageService

	// FundingSources / FundingSourceWrites are §6's funding source block and its "Quản lý nguồn vốn"
	// dialog (funding_sources.go): *fistore.NguonVonStore and *app.FundingSources in production.
	// Refused at construction when missing.
	FundingSources      FundingSourceReading
	FundingSourceWrites FundingSourceWriting

	// ProjectDiscussion / ProjectDiscussionWrites are §8.1's issues and §8.4's discussion
	// (project_discussion.go): *fistore.ProjectDiscussionStore and *app.ProjectDiscussion in
	// production. The read half also feeds §7.2's latest-issue column and §3's open-issue count.
	// Refused at construction when missing.
	ProjectDiscussion       ProjectDiscussionReading
	ProjectDiscussionWrites ProjectDiscussionWriting

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
	if d.CapitalPlanCategoryImports == nil {
		panic("finance/http: thiếu use case nhập Excel hạng mục kế hoạch vốn — ba tuyến /api/v1/capital-plan-categories/import* sẽ panic khi có người gọi")
	}
	if d.DisbursementImports == nil {
		panic("finance/http: thiếu use case nhập Excel giải ngân — ba tuyến /api/v1/disbursements/import* sẽ panic khi có người gọi")
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
	if d.AuditLog == nil {
		panic("finance/http: thiếu bộ đọc nhật ký hệ thống — GET /api/v1/finance-audit-entries sẽ panic khi có người gọi")
	}
	if d.SystemMessages == nil {
		panic("finance/http: thiếu use case lời hệ thống — các tuyến /api/v1/finance-system-messages sẽ panic khi có người gọi")
	}
	if d.FundingSources == nil || d.FundingSourceWrites == nil {
		panic("finance/http: thiếu kho đọc hoặc use case ghi nguồn vốn — các tuyến /api/v1/funding-sources sẽ panic khi có người gọi")
	}
	if d.ProjectDiscussion == nil || d.ProjectDiscussionWrites == nil {
		// The list and the summary read it too, so a missing store would break §3 and §7 as well as
		// the two tabs — or, worse, tempt a nil check that prints "0 vướng mắc" for every commune.
		panic("finance/http: thiếu kho đọc hoặc use case ghi vướng mắc/trao đổi — các tuyến issues/comments và danh sách dự án sẽ panic khi có người gọi")
	}

	h := NewHandler(d)

	// The catalogue's Excel import — three routes, all `admin.lookup` (routes_catalogue_import.go).
	registerCatalogueImportRoutes(mux, d, h)

	// The voucher register's Excel import — `budget.read` template, `budget.update` preview and import
	// (routes_disbursement_import.go).
	registerDisbursementImportRoutes(mux, d, h)

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

	// §8.2's voucher tab: one project's live vouchers, newest `payment_date` first (ties: newest id).
	//
	// NESTED UNDER THE PROJECT, unlike the voucher write routes, and the reason those give does not
	// apply here: they act on ONE voucher by its own id, where a project segment would be read by
	// nothing. Here the project IS the filter — the segment is the one thing the route reads — and a
	// voucher belongs to a project, so the path states the relationship the right way round.
	//
	// `budget.read`, the key that guards the project's own detail (seeded at
	// service-identity/migrations/0001_init.sql:282-284; no key invented, rule 5 invariant 3c). Every
	// state is returned, `ke-toan-nhap` included — the same set `da_giai_ngan` totals (§11).
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Danh sách chứng từ giải ngân còn hiệu lực của một dự án, ngày chi mới nhất trước, kèm tên nguồn vốn
	// @screen   06-giai-ngan §8.2
	// @reply    200 projectVouchersOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/investment-projects/{id}/disbursements",
		authz.RequirePermission(d.Checker, "budget.read")(
			http.HandlerFunc(h.ListProjectVouchers)))

	// §3's four KPI cards, §4's cumulative curve and §5's category table for one budget year — every
	// figure computed here, none in the browser (disbursement_summary.go says why).
	//
	// `investment-project-summary`, TOP LEVEL AND SINGULAR, following the repository's own precedent for
	// an aggregate read (`incoming-document-summary`, `task-summary`, `map-asset-summary`). NOT
	// `investment-projects/overview`: under `{id}` that segment would be one ULID-shaped value away from
	// meaning a project, and a reader of the path could not tell the two apart.
	//
	// `budget.read`, the key the project list it summarises already requires (seeded at
	// service-identity/migrations/0001_init.sql:282-284; no key invented, rule 5 invariant 3c).
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Tổng hợp giải ngân của xã theo năm ngân sách: bốn thẻ KPI, luỹ kế theo tháng, bảng theo hạng mục
	// @screen   06-giai-ngan §3
	// @reply    200 projectSummaryOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/investment-project-summary",
		authz.RequirePermission(d.Checker, "budget.read")(
			http.HandlerFunc(h.ProjectSummary)))

	// §8.3's chart tab: one project's cumulative disbursement by month against its plan line, which
	// follows the project's OWN calendar (prototype 03c1787 — domain.ProjectCurve).
	//
	// `disbursement-curve`, A NOUN FOR THE DATA, not `chart`: a chart is one way of drawing it, and the
	// path outlives the drawing. Nested under the project because the project IS the filter.
	//
	// `budget.read`, as the project's detail. 404 for a project of another commune, as the detail.
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Luỹ kế giải ngân theo tháng của một dự án so với kế hoạch theo lịch của chính dự án
	// @screen   06-giai-ngan §8.3
	// @reply    200 projectCurveOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/investment-projects/{id}/disbursement-curve",
		authz.RequirePermission(d.Checker, "budget.read")(
			http.HandlerFunc(h.ProjectDisbursementCurve)))

	// --- §8.1 "Vướng mắc" and §8.4 "Trao đổi" (migration 0015, user decision 06/10/2026) ----------
	//
	// THE KEYS FOLLOW THE PROTOTYPE'S ROUTER, checked rather than assumed (vigov-require
	// apps/api/app/modules/budget/router.py:266-342): `budget.read` to read both tabs, `budget.update`
	// to record and to resolve an issue, and `budget.read` — NOT `budget.update` — to post a message.
	// All three keys are seeded at service-identity/migrations/0001_init.sql:282-284; no key invented
	// (rule 5, invariant 3c).
	//
	// NESTED UNDER THE PROJECT for the reads and the two creates — the project IS the parent and the
	// filter. The resolution is addressed by the issue's own id (`project-issues/{id}`), like the
	// voucher lifecycle routes: a project segment there would be read by nothing.
	//
	// NOTHING CROSSES A SERVICE BOUNDARY HERE: the tracking task (§13 rule 4) and the mention
	// notification arrive later as events (app/project_discussion.go says how).

	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Dòng thời gian vướng mắc của một dự án, mới nhất trước — cả vướng mắc đã gỡ
	// @screen   06-giai-ngan §8.1
	// @reply    200 projectIssuesOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/investment-projects/{id}/issues",
		authz.RequirePermission(d.Checker, "budget.read")(
			http.HandlerFunc(h.ListProjectIssues)))

	// `budget.update`, the prototype's key for recording an obstacle (router.py:281-291) and the key
	// §8.2 gives to data entry on this screen.
	//
	// idem.Required(MoKhiHong): nothing underneath stops a double-submitted form from recording the same
	// obstacle twice, so the key is the layer that does. MoKhiHong and not DongKhiHong: a duplicated note
	// is visible on the timeline and costs nothing legal — no money moves, no number is issued — so
	// refusing a clerk while the idempotency store is down would trade an outage for a cosmetic risk.
	//
	// @summary  Ghi nhận một vướng mắc của dự án — dòng đầu là tiêu đề, phần sau là diễn giải
	// @screen   06-giai-ngan §8.1
	// @request  projectIssueIn
	// @reply    201 projectIssueOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/investment-projects/{id}/issues",
		authz.RequirePermission(d.Checker, "budget.update")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.RecordProjectIssue))))

	// `budget.update`, the prototype's key for moving an issue along (router.py:294-306).
	//
	// A SUB-RESOURCE NOUN, `resolution`, not a verb — the shape `.../signature` and `.../rating` use.
	// ONE-WAY: there is no DELETE of it and no reopen (the prototype has one "Đã gỡ xong" button and
	// none to undo it); 0015's trigger refuses a reopen underneath.
	//
	// idem.KhongCan: a second resolution is refused with 409 and cannot overwrite who resolved it or
	// when (the UPDATE carries `resolved_at IS NULL`), so a retry leaves one row state and one entry.
	//
	// @summary  Ghi vướng mắc là đã gỡ — một lần, không mở lại
	// @screen   06-giai-ngan §8.1
	// @reply    200 projectIssueOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error issue_already_resolved
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/project-issues/{id}/resolution",
		authz.RequirePermission(d.Checker, "budget.update")(
			idem.KhongCan("gỡ lần hai bị từ chối 409 và câu UPDATE mang `resolved_at IS NULL`, nên lần gửi lại không ghi đè người gỡ và thời điểm gỡ")(
				http.HandlerFunc(h.ResolveProjectIssue))))

	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Các ý kiến trao đổi về một dự án, cũ nhất trước
	// @screen   06-giai-ngan §8.4
	// @reply    200 projectCommentsOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/investment-projects/{id}/comments",
		authz.RequirePermission(d.Checker, "budget.read")(
			http.HandlerFunc(h.ListProjectComments)))

	// `budget.read` ON A WRITE, AND THAT IS THE PROTOTYPE'S STATED DECISION (router.py:321-342):
	// "discussing a figure is not changing it, and shutting people who can see the project out of the
	// conversation would push it back to Zalo". A message changes no figure on any screen. Every message
	// is still audited with who posted it, when and from where.
	//
	// idem.Required(MoKhiHong), for the reason the issue POST gives.
	//
	// @summary  Gửi một ý kiến trao đổi về dự án, có thể nhắc tên cán bộ (chưa gửi thông báo)
	// @screen   06-giai-ngan §8.4
	// @request  projectCommentIn
	// @reply    201 projectCommentOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/investment-projects/{id}/comments",
		authz.RequirePermission(d.Checker, "budget.read")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.AddProjectComment))))

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
	// idem.Required(DongKhiHong) — CHANGED FROM MoKhiHong ON 06/10/2026, when `code` became optional.
	// skills/rest-api-design §4 says to answer at the route which layer is actually protecting this.
	// With a TYPED code the answer was `UNIQUE (tenant_id, ma)` (0004:216-221): a second project with
	// that code cannot exist whatever happens to Redis, so MoKhiHong was enough. With an AUTO-ISSUED
	// code (§9 `☑ Tự sinh mã`, user decision 06/10/2026) that key protects nothing against a double
	// submit: the second request is simply issued the NEXT number, and the commune's plan counts one
	// project twice under DA07 and DA08 — the same shape as POST /api/v1/disbursements, which takes
	// DongKhiHong for that reason. The skill reserves DongKhiHong for issued numbers; this route now
	// issues one. THE COST, STATED: while the idempotency store is down, entering a project is refused
	// (typed codes included) rather than risking a duplicate.
	//
	// `code` OPTIONAL (user decision 06/10/2026): omitted or blank → the next code of the commune's
	// ONE series, DA01, DA02 … DA99, DA100 (the prototype's format, vigov-require budget/service.py
	// `_next_serial_code`). Issued under a per-commune counter row lock (migration 0014), stepping over
	// every code already used — removed projects included — so an issued code is never issued again
	// (rule 7, invariant 3). A typed code is still accepted and must never have been used (§9).
	//
	// 409 AND NOT 403 for a code already issued: the caller holds `budget.update` and is allowed to
	// enter projects. What is refused is this value against the state of the data. `code_series_blocked`
	// is the auto path refusing after domain.ProjectSerialSkipLimit hand-typed DA codes in a row.
	//
	// 409 ALSO COVERS THE ALLOCATION REFUSALS (decision 06/10/2026): a total above `planned_amount`
	// (`allocation_exceeds_plan`, the sentence names the overrun) and one source twice
	// (`duplicate_source`).
	//
	// @summary  Thêm một dự án đầu tư cho năm ngân sách, kèm phân bổ nguồn vốn nếu xã khai; để trống mã thì hệ thống cấp mã tiếp theo trong dãy DA01, DA02…
	// @screen   06-giai-ngan §9
	// @request  themDuAnVao
	// @reply    201 duAnGhiRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error code_taken code_series_blocked allocation_exceeds_plan duplicate_source
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/investment-projects",
		authz.RequirePermission(d.Checker, "budget.update")(
			idem.Required(idem.DongKhiHong)(
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
	// `funding_allocations` IS EDITABLE HERE SINCE 06/10/2026 (user decision, following the prototype):
	// a full replacement set — kept sources updated in place, re-added ones revived, dropped ones soft
	// deleted. 409 when the total would exceed the plan (also when `planned_amount` is revised below
	// what is allocated), when one source is named twice, or when a dropped source already has
	// vouchers of this project. idem.KhongCan still holds: an identical replacement set is a no-op.
	//
	// @summary  Sửa hạng mục, tên, mô tả, kế hoạch vốn, đơn vị, cán bộ phụ trách, các mốc thời gian hoặc phân bổ nguồn vốn của một dự án đầu tư
	// @screen   06-giai-ngan §8
	// @request  suaDuAnVao
	// @reply    200 duAnGhiRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error allocation_exceeds_plan duplicate_source source_has_disbursements
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/investment-projects/{id}",
		authz.RequirePermission(d.Checker, "budget.update")(
			idem.KhongCan("sửa là ghi đè một trạng thái đã biết; app.Sua không ghi gì khi không có trường nào đổi, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.SuaDuAn))))

	// `budget.update` ON A REMOVAL — USER DECISION 06/10/2026, following the prototype
	// (vigov-require apps/api/app/modules/budget/router.py:171-175, where removing an item takes the
	// same right as entering one). The specification (06-giai-ngan.md:202) assigns the removal no key;
	// this route carried `budget.confirm` until that decision, chosen then as the direction that could
	// be loosened with one line. This is that line. What the decision accepts, stated: withdrawing a
	// project — which moves "KẾ HOẠCH VỐN NĂM" on §3, its category row on §5 and two figures on §6 — is
	// now the same authority as entering one. Every removal is still a soft delete with an audit entry
	// naming who and when. The key is seeded (service-identity/migrations/0001_init.sql:284); no key
	// invented (rule 5, invariant 3c).
	//
	// THE BODY IS OPTIONAL (same decision): `reason` omitted, blank, or no body → `delete_reason` is the
	// fixed "Rút khỏi danh sách dự án", so rule 7 invariant 1's column is never empty. A reason that is
	// given travels in the body, never the query string, which would put free text about a public
	// authority's spending into every access log and proxy cache.
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
	// @summary  Rút một dự án đầu tư khỏi danh sách (xoá mềm), lý do không bắt buộc — từ chối khi dự án còn chứng từ giải ngân
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
		authz.RequirePermission(d.Checker, "budget.update")(
			idem.KhongCan("xoá một dự án đã xoá cho cùng một kết quả: câu UPDATE mang `AND deleted_at IS NULL` nên lần thứ hai không ghi đè được người xoá và lý do")(
				http.HandlerFunc(h.XoaDuAn))))

	// --- §6 "Tiến độ theo nguồn vốn" and "Quản lý nguồn vốn" (migration 0013) --------------------
	//
	// THE URL NOUN IS `funding-sources`, plural, kebab-case — the English the contract already uses for
	// this concept (`funding_source_id` on vouchers and allocation lines, `funding_allocations`, the
	// FundingSourceAnnualAmount entity of 0013). A second English word for it would be rule 12's
	// forbidden #2. TOP LEVEL: a source belongs to the commune, not to a project or a year.
	//
	// THE KEYS ARE THE USER'S (decision 06/10/2026): `budget.read` to read, `budget.update` to manage —
	// both seeded at service-identity/migrations/0001_init.sql:282-284 (rule 5, invariant 3c; no key
	// invented). `budget.update` is the key §8.2 gives to data entry on this screen, and entering a
	// source or the amount granted to it is data entry. There is NO remove and NO rename route — the
	// user decided neither exists, so no key had to be chosen for them.

	// NO idem.* DECLARATION: a GET changes no state. `year` is required and never defaults.
	//
	// @summary  Tiến độ theo nguồn vốn của một năm ngân sách: vốn được giao, đã phân bổ, đã giải ngân, ba tỷ lệ, và số đã chi chưa ghi rút từ nguồn nào
	// @screen   06-giai-ngan §6
	// @reply    200 fundingSourcesOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/funding-sources",
		authz.RequirePermission(d.Checker, "budget.read")(
			http.HandlerFunc(h.ListFundingSources)))

	// idem.Required(MoKhiHong), AND WHICH LAYER PROTECTS THIS (skills/rest-api-design §4): the real
	// guard is `nguon_von_name_unique` (0013), which counts soft-deleted rows — a second source with the
	// same name in one commune CANNOT EXIST, whatever happens to Redis. The key is the second layer:
	// a double-submitted form gets one source and a replayed 201 instead of a confusing 409. MoKhiHong
	// and not DongKhiHong for the reason POST /api/v1/investment-projects gives: with the unique key
	// underneath, a cache outage cannot produce a duplicate, so refusing a clerk during one would be
	// paying with an outage for a risk already covered.
	//
	// 409 AND NOT 403 for a taken name or a full catalogue: the caller may manage sources; what is
	// refused is this value against the state of the data.
	//
	// @summary  Thêm một nguồn vốn vào danh mục nguồn vốn của xã, kèm vốn được giao của năm nếu xã khai
	// @screen   06-giai-ngan §6
	// @request  fundingSourceCreateIn
	// @reply    201 fundingSourceOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error funding_source_name_taken funding_source_catalogue_full
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/funding-sources",
		authz.RequirePermission(d.Checker, "budget.update")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.CreateFundingSource))))

	// PUT, AN UPSERT OF THE ONE FIGURE THIS SUB-RESOURCE HAS — the shape the system-message override
	// uses: `annual-amounts/{year}` is addressed by its natural key (one figure per source per year,
	// 0013's unique key), so the client names it and PUT sets it, whether or not a figure existed.
	//
	// ⚠ A PAST YEAR'S FIGURE MAY BE CORRECTED — an assumption, stated in app.SetGrantedAmount, because
	// no lock rule was decided. Every correction is audited with before and after.
	//
	// idem.KhongCan: app.SetGrantedAmount writes nothing and files no entry when the figure already
	// recorded equals the figure sent — so a retry leaves one row and one entry.
	//
	// @summary  Ghi hoặc sửa vốn được giao của một nguồn vốn cho một năm ngân sách
	// @screen   06-giai-ngan §6
	// @request  grantedAmountIn
	// @reply    200 grantedAmountOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PUT /api/v1/funding-sources/{id}/annual-amounts/{year}",
		authz.RequirePermission(d.Checker, "budget.update")(
			idem.KhongCan("đặt lại đúng số vốn đang ghi không ghi gì và không để vết, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.SetFundingSourceGrantedAmount))))

	// The projects behind one card (prototype SourceItemsDialog): this source's share of each project
	// of the year, and what was paid from it. 404 covers "no such source" and "a source of another
	// commune" as one answer. NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Các dự án của một năm lấy vốn từ một nguồn: phần phân bổ từ nguồn ấy, đã chi từ nguồn ấy và tỷ lệ
	// @screen   06-giai-ngan §6
	// @reply    200 fundingSourceProjectsOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/funding-sources/{id}/projects",
		authz.RequirePermission(d.Checker, "budget.read")(
			http.HandlerFunc(h.ListFundingSourceProjects)))

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
	// 409 FOR THE FUNDING-SOURCE RULE (decision 06/10/2026): `source_required` when the project has
	// allocation lines and none was named, `source_not_allocated` when the named source is not one of
	// them (or the project has none). Decided under the project row's share lock, inside the write's
	// transaction — store.ProjectForVoucherWrite.
	//
	// @summary  Ghi nhận một chứng từ giải ngân cho dự án đầu tư
	// @screen   06-giai-ngan §8.2
	// @request  themChungTuVao
	// @reply    201 chungTuRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error source_required source_not_allocated
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
	// 409 `source_required` / `source_not_allocated` WHEN THE BODY NAMES `funding_source_id` and the
	// project's allocation lines refuse it (decision 06/10/2026); a body that does not name it is not
	// checked. A locked voucher answers `voucher_state` first, whatever the body says.
	//
	// @summary  Sửa ngày chi, số tiền, nội dung, đối tác hoặc số chứng từ của một chứng từ chưa khoá — chứng từ đã xác nhận sẽ về nháp
	// @screen   06-giai-ngan §8.2
	// @request  suaChungTuVao
	// @reply    200 chungTuRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error voucher_state source_required source_not_allocated
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
	// `method` IS ACCEPTED HERE, AND ONLY `manual` OR `entries`, FOR A LEAF (user decision 25/09/2026,
	// §4.2). `children` is refused with 400 — it follows the tree — and a line with children answers
	// 409. entries -> manual copies the batch sums into the cells in the same transaction (§9.1);
	// manual -> entries leaves the typed cells stored and undisplayed. A figure typed into a line whose
	// mode is `entries` after the request answers 409.
	//
	// @summary  Sửa số thứ tự, tên, cách tính (manual/entries) hoặc các ô số của một khoản mục chưa có dòng con
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

	// --- the batches: the `⇄ Các đợt thu, chi` dialog (§5, migration 0008) ----------------------------
	//
	// `entries` UNDER A LINE for the list and the create — both act on ONE line, whose id is the
	// segment and is checked (live line, live sheet, this commune). `budget-entries/{id}` AT TOP LEVEL
	// for the removal, for the reason `budget-lines` is not nested under `budget-sheets`: the act
	// names one batch by its own id, and a line segment would be one nothing reads or checks. URL
	// noun `budget-entries` chosen by the user, 25/09/2026.
	//
	// THE SAME THREE KEYS, SPLIT THE WAY THE LINE ROUTES SPLIT THEM: reading is `budget.read`,
	// recording a batch is data entry (`budget.update`), removing one takes a figure that may already
	// have been read off a screen out of a total (`budget.confirm`, as removing a line does).

	// `budget.read` — the batches of one line of the commune's own budget. The list carries
	// `counterparty` ("Đơn vị, cá nhân"), which may name a person, so it is MASKED
	// (privacy.MaskName) for every caller — no full-view key exists (rule 3 stop condition #1,
	// open question #27). See internal/http/dot_thu_chi.go for the stated costs. Never logged.
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Các đợt thu, chi đã ghi của một khoản mục, mới nhất trước, kèm số tiền theo từng cột số
	// @screen   07-thu-chi-ngan-sach §5
	// @reply    200 danhSachDotRa
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/budget-lines/{id}/entries",
		authz.RequirePermission(d.Checker, "budget.read")(
			http.HandlerFunc(h.DocDotThuChi)))

	// `budget.update` — `+ Ghi đợt` is data entry by the accountant.
	//
	// idem.Required(DongKhiHong), FOLLOWING POST /api/v1/budget-lines AND FOR THE SAME REASON: 0008
	// deliberately has no unique key beyond the primary key (two instalments of one fee on one day
	// are a real case), so a cache outage plus a double click is a batch counted twice in a figure
	// that goes upward. A 503 costs the accountant one retry.
	//
	// A LINE WITH CHILDREN ANSWERS 409. Writing a batch does NOT switch the line to `entries`; the
	// user chooses the mode through PATCH /api/v1/budget-lines/{id} (§4.2).
	//
	// @summary  Ghi một đợt thu, chi vào một khoản mục lá
	// @screen   07-thu-chi-ngan-sach §5
	// @request  ghiDotVao
	// @reply    201 dotRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/budget-lines/{id}/entries",
		authz.RequirePermission(d.Checker, "budget.update")(
			idem.Required(idem.DongKhiHong)(
				http.HandlerFunc(h.GhiDotThuChi))))

	// `budget.confirm` — removing a batch changes the figure of a line in `entries` mode, which may
	// already have been read off a screen; the same weight, and the same key, as removing a line.
	// Loosening it to `budget.update` is one literal if the customer asks.
	//
	// A BODY ON A DELETE for the reason goVao gives: the reason is mandatory (rule 7, invariant 1).
	//
	// idem.KhongCan — removing an already-removed batch is a 404 either way, and the second request
	// cannot overwrite who removed it or why (`AND deleted_at IS NULL`, plus 0008's trigger).
	//
	// @summary  Gỡ mềm một đợt thu, chi, kèm lý do bắt buộc
	// @screen   07-thu-chi-ngan-sach §5
	// @request  goVao
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/budget-entries/{id}",
		authz.RequirePermission(d.Checker, "budget.confirm")(
			idem.KhongCan("gỡ một đợt đã gỡ cho cùng một kết quả: câu UPDATE mang `AND deleted_at IS NULL` nên lần thứ hai không ghi đè được người gỡ và lý do")(
				http.HandlerFunc(h.GoDotThuChi))))

	// --- budget period close (chốt kỳ), migration 0012 ------------------------------------------------
	//
	// `budget-period-closes` — the noun the user settled on 30/09/2026 (kb/00-foundation/
	// ubiquitous-language.md): PLURAL because the resource is the CLOSE ACTS, one row each, not "the
	// period". A re-close after a reopen is a new item with the next revision and a new `code`.
	//
	// `budget.confirm` FOR CLOSE AND REOPEN — the user's decision, and the key §9 rule 7's group already
	// gives the weight of a confirmation. `budget.read` to read the history. Both seeded at
	// service-identity/migrations/0001_init.sql:282-284; no key invented (rule 5, invariant 3c).
	//
	// WHAT A CLOSE LOCKS is enforced on the write routes above, inside their transactions (409
	// `budget_period_closed`): adding/removing an entry dated in a closed month or year, or on a sheet
	// of a closed year; and — for a YEAR close only — every sheet, line, headline and value write of
	// that year's sheets.

	// `budget.read` — the close history of one year: active AND reopened closes, who and when. NO
	// idem.* DECLARATION: a GET changes no state. `year` is required and never defaults, for the reason
	// namVaLoai gives.
	//
	// @summary  Lịch sử chốt kỳ ngân sách của một năm: các lần chốt tháng, chốt cả năm, đang hiệu lực hay đã mở chốt
	// @screen   07-thu-chi-ngan-sach §6
	// @reply    200 budgetPeriodClosesOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/budget-period-closes",
		authz.RequirePermission(d.Checker, "budget.read")(
			http.HandlerFunc(h.ListBudgetPeriodCloses)))

	// `budget.confirm` — closing a month (`month` 1..12) or a whole year (`month` absent). 409 when
	// the period already has an active close, naming it.
	//
	// idem.Required(DongKhiHong): a close is a declaration that a commune's figures are final — the
	// class of act core/idem reserves DongKhiHong for. The unique index `budget_period_closes_one_active`
	// is the floor under a duplicate, but a retry must be told the code the first attempt issued, not a
	// 409 about its own close; that is what RecordCode below the handler gives it.
	//
	// @summary  Chốt kỳ ngân sách theo tháng hoặc cả năm — sau khi chốt không thêm, gỡ đợt thu chi của kỳ
	// @screen   07-thu-chi-ngan-sach §6
	// @request  budgetPeriodCloseIn
	// @reply    201 budgetPeriodCloseOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/budget-period-closes",
		authz.RequirePermission(d.Checker, "budget.confirm")(
			idem.Required(idem.DongKhiHong)(
				http.HandlerFunc(h.CreateBudgetPeriodClose))))

	// `reopening` IS A NOMINALISED SUB-RESOURCE, NOT THE VERB `reopen` — the noun rest_api_guard itself
	// proposes for it, and the shape `headline` uses. POST creates the reopening of ONE close, once:
	// 409 when it is already reopened. The reason is mandatory (≤500), because a period whose figures
	// were declared final and then unlocked with no reason is the question an inspection asks first.
	//
	// {code} IS THE CLOSE'S BUSINESS CODE (`CK-2026-09-01`), the identifier the list shows and the
	// audit trail is filed under. Another commune's code answers the same 404 as a code never issued.
	//
	// idem.Required(DongKhiHong), for the reason the close gives: unlocking final figures is an act
	// with legal consequence, and the retry is told the code instead of a 409 about its own reopen.
	//
	// @summary  Mở chốt một lần chốt kỳ ngân sách, kèm lý do bắt buộc — ghi một lần, không xoá lần chốt
	// @screen   07-thu-chi-ngan-sach §6
	// @request  budgetPeriodReopeningIn
	// @reply    200 budgetPeriodCloseOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/budget-period-closes/{code}/reopening",
		authz.RequirePermission(d.Checker, "budget.confirm")(
			idem.Required(idem.DongKhiHong)(
				http.HandlerFunc(h.CreateBudgetPeriodReopening))))

	// --- NHẬT KÝ HỆ THỐNG — this service's own audit log (ADR 0054) ------------------------------
	//
	// ONE OF FIVE ROUTES, one per service owning an `audit_log`; web-admin merges them (ADR 0054 §1,
	// §6). The noun carries the service name because tools/ingress routes by the first path segment
	// and five owners for one `audit-entries` segment is a STOP there (ADR 0054 §3).
	//
	// `admin.audit` — "Xem nhật ký hệ thống" — seeded at service-identity/migrations/0001_init.sql:280;
	// no key invented (rule 5, invariant 3c). Staff of the request's commune only; there is no operator
	// variant (ADR 0054 §2, ADR 0003:32).
	//
	// A GET THAT WRITES: every call leaves one `xem_nhat_ky_he_thong` entry, in the same transaction
	// as the read (ADR 0054 §5) — so a page whose entry could not be written is a 500, never data.
	// NO idem.* DECLARATION: repeating the read is a second read, and it is recorded as one.
	//
	// @summary  Nhật ký hệ thống của phân hệ Tài chính — vết thao tác của xã, mới nhất trước, lọc theo thời gian · người · động từ · đối tượng
	// @screen   14-cau-hinh §12.1
	// @reply    200 page.Result[audit.EntryView]
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/finance-audit-entries",
		authz.RequirePermission(d.Checker, "admin.audit")(
			http.HandlerFunc(h.ListAuditEntries)))

	// --- LỜI HỆ THỐNG — the sentences this service raises, reworded per commune (migration 0010) ----
	//
	// THE NOUN IS VENDOR-CHOSEN (2026-09-29) and is a finding for ubiquitous-language, which has no
	// row for `loi_he_thong`. `finance-` for the reason `finance-audit-entries` carries it: ADR 0024
	// splits the 39 keys by the service that RAISES them, petitions will mount its own list, and
	// tools/ingress refuses two services on one first segment (ADR 0054 §3). `system-messages`
	// because the tab is "Lời hệ thống" and each item is one message the system says.
	//
	// `override` IS A SUB-RESOURCE, THE SHAPE `no-task-marker` ALREADY USES: the commune's own
	// wording is a state PUT sets and DELETE removes, and the message itself survives both — a shipped
	// key cannot be deleted (14-cau-hinh §7). `DELETE …/{code}` would read as removing the message;
	// `POST …/{code}/revert` is a verb in a path, which rest_api_guard refuses.
	//
	// `admin.lookup` — "Quản lý danh mục" — ON ALL THREE, the key the requirement repository guards
	// this screen with (../vigov-require/docs/spec/04-api.md:41-44), seeded at
	// service-identity/migrations/0001_init.sql:274. NO KEY INVENTED (rule 5, invariant 3c). READ IS
	// NOT AnyAuthenticated: this list is the configuration screen, and the one reader who needs the
	// sentence without the key — the budget screen — gets it with the figures, not from here.

	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Lời hệ thống của phân hệ Tài chính: câu mặc định, câu xã đang dùng và ai sửa lần cuối
	// @screen   14-cau-hinh §7
	// @reply    200 systemMessageListOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/finance-system-messages",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			http.HandlerFunc(h.ListSystemMessages)))

	// PUT, a full replacement of the one field this resource has. Empty `text` is 400 and names the
	// DELETE below — an empty sentence is not a state this screen offers.
	//
	// idem.KhongCan: app.Reword writes nothing and files no entry when the text in force already
	// equals the text sent — so a retry leaves one row, one entry.
	//
	// @summary  Xã sửa lời một câu hệ thống của phân hệ Tài chính
	// @screen   14-cau-hinh §7
	// @request  rewordSystemMessageIn
	// @reply    200 systemMessageOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PUT /api/v1/finance-system-messages/{code}/override",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("đặt lại đúng câu đang dùng không ghi gì và không để vết, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.RewordSystemMessage))))

	// "Khôi phục câu mặc định". A soft delete of the commune's live wording (rule 7, invariant 1);
	// the history stays on disk and in audit_log. 204 also when the commune is already on the
	// default — the state asked for holds, and nothing is written.
	//
	// idem.KhongCan: the second request finds no live wording and writes nothing.
	//
	// @summary  Khôi phục câu mặc định của phần mềm cho một câu hệ thống của phân hệ Tài chính
	// @screen   14-cau-hinh §7
	// @reply    204 -
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/finance-system-messages/{code}/override",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("khôi phục khi xã đã dùng câu mặc định thì không còn dòng nào để gỡ và không ghi gì")(
				http.HandlerFunc(h.RestoreSystemMessage))))
}
