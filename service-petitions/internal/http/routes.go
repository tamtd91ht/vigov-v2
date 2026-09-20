package http

// Routes for the petitions service.
//
// EVERY route declares its permission explicitly. The global guard only rejects users who are
// not signed in; it does not check permissions. A route with no declaration is callable by
// every staff role, and nothing reports it — see rule 5.
//
// Four declarations are available, and there is no fifth:
//
//	authz.RequirePermission(checker, "feedback.read")   // the normal case
//	authz.CitizenOnly()                                     // citizen paths, isolated by identity
//	authz.AnyAuthenticated("<why any account needs this>")  // reason mandatory
//	authz.Public("<why this is public>")                    // reason mandatory
//
// EVERY route also carries an @-annotation block IMMEDIATELY above the statement — no blank line
// between. `tools/apidoc` reads it and generates kb/20-contracts/openapi.json, the type contract
// the admin web builds against. Without it the web types every response by hand.
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
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The catalogue readers are INTERFACES, not the concrete stores.
//
// WHY: these routes carry the isolation rule this service exists to keep — one commune's catalogue
// never reaching another commune's caller — and that has to be testable without a PostgreSQL, or it
// gets tested once and then never again. There is no PostgreSQL in this build environment at all
// (VIGOV_TEST_DSN unset), so a test that needs one is a test that does not run.
//
// *petstore.LoaiNhiemVuStore and *petstore.MucUuTienNhiemVuStore satisfy these as they are; nothing
// was changed to accommodate them.
type (
	// LoaiNhiemVuDanhMuc is the commune's task-type catalogue, for GET /api/v1/task-types.
	//
	// NO page.Request PARAMETER: this route returns the whole list on purpose. The reason is on
	// petstore.LoaiNhiemVuStore.DanhSach, and the bound that replaces the missing `limit` is
	// petstore.TranDanhMucLoaiNhiemVu.
	LoaiNhiemVuDanhMuc interface {
		DanhSach(ctx context.Context) ([]domain.LoaiNhiemVu, error)
	}

	// MucUuTienDanhMuc is the commune's task-priority scale, for GET /api/v1/task-priorities.
	//
	// A SECOND, NARROW INTERFACE RATHER THAN ONE SHARED "catalogue reader", although the two have
	// the same method shape and the same arity. A shared reader would have to be told WHICH
	// catalogue to read, which means a table name or a key travelling as an argument — and the
	// handler that passes the wrong one produces a perfectly valid response listing the wrong
	// catalogue. Two interfaces make that call impossible to express, and each route depends on
	// exactly the one list it renders: a dependency wider than the work requires is how the next
	// person justifies using it for something else.
	MucUuTienDanhMuc interface {
		DanhSach(ctx context.Context) ([]domain.MucUuTienNhiemVu, error)
	}

	// PhieuPhanAnhDoc is the read side of the petition register, for
	// GET /api/v1/citizen-reports/{maTraCuu}.
	//
	// READ ONLY, and the two write methods on *petstore.PhieuPhanAnhStore are deliberately NOT
	// in this interface. A route depending on a wider surface than it uses is how the next
	// person justifies writing through it — and the reasons the two write ROUTES are missing
	// (below) are not reasons this interface should carry the methods anyway.
	PhieuPhanAnhDoc interface {
		TheoMaTraCuu(ctx context.Context, ma string) (domain.PhieuPhanAnh, error)
	}

	// NhanLinhVucDanhMuc is the commune's OVERRIDES for the petition field labels — tier 2 of
	// ADR 0026. A code with no row here is valid and simply carries the platform's default
	// label; absence is never "unknown field".
	NhanLinhVucDanhMuc interface {
		DanhSach(ctx context.Context) ([]domain.NhanLinhVuc, error)
	}
)

// Deps are everything the routes need. Kept explicit so wiring stays in cmd/server.
type Deps struct {
	// Checker decides permissions. IT IS NOW LOAD-BEARING: GET /api/v1/citizen-reports/{maTraCuu}
	// declares authz.RequirePermission, and the handler consults the Checker a second time for
	// the restricted field. A nil one is therefore in the panic switch in Register — see the note
	// there, which predicted exactly this route.
	Checker authz.Checker

	LoaiNhiemVu LoaiNhiemVuDanhMuc
	MucUuTien   MucUuTienDanhMuc

	Phieu       PhieuPhanAnhDoc
	NhanLinhVuc NhanLinhVucDanhMuc

	Log *slog.Logger
}

// Register mounts the petitions routes.
//
// Add every new route with its permission declaration in the SAME statement, never on a nearby
// line: rbac_guard anchors to the statement, and so should a reader.
func Register(mux *http.ServeMux, d Deps) {
	// Refusing incomplete wiring HERE, at construction, not at request time: a route mounted
	// without its store would accept requests it cannot answer, and the first person to find out
	// would be a member of staff in front of a government screen. The process failing to start is
	// loud, happens before any commune is served, and names the missing piece.
	//
	// d.Checker IS NOW IN THE SWITCH, and that note used to say the opposite for a reason that
	// has expired: it said a nil Checker guarded nothing because no route declared a permission,
	// and it said THE FIRST RequirePermission ROUTE MUST ADD THE CASE. The petition read route
	// below is that route. authz.RequirePermission(nil, …) panics when somebody CALLS it — a
	// panic on a member of staff's screen — whereas this one happens before any commune is served
	// and names the missing piece.
	switch {
	case d.LoaiNhiemVu == nil:
		panic("petitions/http: thiếu kho loại nhiệm vụ — GET /api/v1/task-types sẽ panic khi có người gọi")
	case d.MucUuTien == nil:
		panic("petitions/http: thiếu kho mức ưu tiên — GET /api/v1/task-priorities sẽ panic khi có người gọi")
	case d.Checker == nil:
		panic("petitions/http: thiếu Checker — GET /api/v1/citizen-reports/{maTraCuu} khai quyền feedback.read, " +
			"và một Checker rỗng sẽ panic lúc có cán bộ gọi chứ không phải lúc khởi động")
	case d.Phieu == nil:
		panic("petitions/http: thiếu kho phiếu phản ánh — GET /api/v1/citizen-reports/{maTraCuu} sẽ panic khi có người gọi")
	case d.NhanLinhVuc == nil:
		panic("petitions/http: thiếu kho nhãn lĩnh vực — GET /api/v1/citizen-reports/{maTraCuu} sẽ panic khi có người gọi")
	}

	h := NewHandler(d)

	// --- the commune's task catalogues. TWO READ ROUTES, AND DELIBERATELY NO WRITE ROUTE --------
	//
	// `task-types` and `task-priorities` — English, plural, kebab-case, under /api/v1. The nouns
	// are the entities migration 0003 declares (`-- @entity: TaskType`, `-- @entity: TaskPriority`),
	// so the table, the Go type and the URL all say the same thing about the same data. Nothing is
	// translated on the spot here.
	//
	// A THIRD CATALOGUE IN THAT MIGRATION WOULD HAVE BEEN MOUNTED HERE AND IS NOT THERE: there is
	// no task-STATUS table in 0003, and none is served. Open question #21 governs the task
	// lifecycle, and a status list is the lifecycle's alphabet.
	//
	// AnyAuthenticated ON BOTH, AND THE REASON IS THE SHAPE OF THE DATA'S USE — the same reading
	// service-identity states on GET /api/v1/org-units and GET /api/v1/roles. These names fill the
	// picker on the task form and the filter bar of nearly every task screen, so requiring a
	// configuration permission would not protect anything: it would empty those pickers for
	// everybody who is not an administrator. There is nothing sensitive in the list of words an
	// authority uses to sort its own work.
	//
	// THE TRADE-OFF, STATED RATHER THAN GLOSSED: a commune's own catalogues are readable by every
	// signed-in account OF THAT COMMUNE. They are NOT readable across communes and cannot be —
	// store.Scoped.Query binds `tenant_id` from the context (rule 1, invariant 5), and the same
	// request carrying another commune's token is refused at the token layer before any query runs
	// (authz compares the token's commune with the one resolved from Host).
	//
	// NO idem.* DECLARATION on either: a GET changes no state, and declaring a duplicate-request
	// protection here would claim a protection with nothing to protect.

	// @summary  Danh mục loại nhiệm vụ của xã — dùng cho ô chọn loại trên biểu mẫu nhiệm vụ và bộ lọc
	// @screen   14-cau-hinh §5
	// 500 covers two causes and says so honestly: an ordinary store failure, and the commune's
	// catalogue exceeding petstore.TranDanhMucLoaiNhiemVu — which this route REFUSES rather than
	// truncating, because a silently short list is a missing option in a form.
	//
	// 401 is authz.AnyAuthenticated's answer to "no session", and there is no 403 on this route
	// because there is no permission to fail.
	//
	// IT IS ALSO THE ANSWER TO A SESSION FROM ANOTHER COMMUNE, BUT NOT FOR THE REASON IT LOOKS.
	// AnyAuthenticated's own commune check cannot fail here: core/staffauth stamps the Host
	// commune onto the principal it builds (staffauth.go:157 and :241), so xacNhanXa compares a
	// value with itself. The comparison that decides happens inside identity, at
	// service-identity/internal/grpc/server.go:257, where the commune from `x-tenant-id` meets
	// the commune INSIDE the credential — the only place both values exist. A mismatch there
	// yields no principal, so this route answers 401 for ABSENCE, not for disagreement.
	//
	// The wall that is local and does hold alone: Scoped.Query binds `tenant_id` from the context
	// (rule 1, invariant 5), so neither query could read another commune's rows in any case.
	//
	// @reply    200 danhSachLoaiNhiemVuRa
	// @reply    401 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/task-types",
		authz.AnyAuthenticated("tên loại nhiệm vụ xuất hiện ở ô chọn trên biểu mẫu nhiệm vụ và ở bộ lọc của hầu hết màn hình nhiệm vụ — đòi một quyền cấu hình sẽ làm rỗng những ô đó cho mọi tài khoản không phải quản trị; đánh đổi đã chấp nhận: danh mục của xã lộ cho mọi tài khoản đã đăng nhập CỦA CHÍNH XÃ ĐÓ, không chéo xã vì Scoped buộc tenant_id")(
			http.HandlerFunc(h.DanhSachLoaiNhiemVu)))

	// THE ORDER OF `items` IS THE CONTRACT HERE, not a detail: priority is a SCALE, and the store
	// returns it ranked (ORDER BY thu_tu). That is why the reply carries no `order` field — one
	// fact, one representation (rule 9).
	//
	// @summary  Danh mục mức ưu tiên nhiệm vụ của xã, theo đúng thứ tự thang — dùng cho ô chọn và bộ lọc
	// @screen   14-cau-hinh §5
	// 500 covers an ordinary store failure and the commune's scale exceeding
	// petstore.TranDanhMucMucUuTien — REFUSED rather than truncated, because a truncated scale
	// loses the levels at one end of it and every task filed from it is ranked wrong.
	//
	// 401, and no 403, for the same reason as the route above.
	//
	// @reply    200 danhSachMucUuTienRa
	// @reply    401 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/task-priorities",
		authz.AnyAuthenticated("tên mức ưu tiên xuất hiện ở ô chọn trên biểu mẫu nhiệm vụ và ở bộ lọc của hầu hết màn hình nhiệm vụ — đòi một quyền cấu hình sẽ làm rỗng những ô đó cho mọi tài khoản không phải quản trị; đánh đổi đã chấp nhận: thang ưu tiên của xã lộ cho mọi tài khoản đã đăng nhập CỦA CHÍNH XÃ ĐÓ, không chéo xã vì Scoped buộc tenant_id")(
			http.HandlerFunc(h.DanhSachMucUuTien)))

	// --- the petition register. ONE READ ROUTE, AND THREE WRITE ROUTES THAT ARE NOT HERE ------
	//
	// `citizen-reports` is the settled URL noun for `phan_anh`
	// (kb/00-foundation/ubiquitous-language.md §Tên tài nguyên trên URL). It is NOT `feedback`:
	// feedback means product suggestions, while a phản ánh is a class of administrative
	// submission with a processing deadline and somebody answerable for it. The permission keys
	// say `feedback.*` because they were fixed earlier, and that mismatch is a known, recorded
	// cost rather than a licence to spell the resource the same way.
	//
	// WHY THE THREE WRITE ROUTES OF docs/ui-ux/09 §13 ARE ABSENT. Each is blocked on something
	// nobody has decided, and each would look entirely reasonable if written anyway — which is
	// what makes writing them expensive rather than merely premature:
	//
	//   POST /api/v1/citizen-reports  (nhập hộ)
	//       Booking a petition FIXES BOTH DEADLINES at once, because the staff form carries the
	//       field (ADR 0028, decision E). Both numbers come from the `sla` table of
	//       docs/ui-ux/14-cau-hinh.md §8 — A TABLE THAT EXISTS IN NO SERVICE. Its rows cover
	//       `van-ban-den` (documents), `phan-anh` and `nhiem-vu` (petitions), so which service
	//       owns it is rule 2's stop condition #1: the same question the user answered for
	//       `lich_lam_viec` by putting the three calendar tables in identity. Nobody has answered
	//       it for `sla`, and a route that guessed would either split one configuration screen
	//       across two services or hold another service's numbers.
	//
	//   POST …/{maTraCuu}/<classification>  (phân loại)
	//       Blocked three ways. (1) The same `sla` question, for `gio_xu_ly_xong` of the settled
	//       field. (2) ADR 0026 requires the field code to be CHECKED ON WRITE against the
	//       tier-1 set in service `platform`, and how petitions reads that set — live gRPC or an
	//       event-fed replica — has no ADR; writing the read path without one is ADR 0026's stop
	//       condition #2. (3) NO PERMISSION KEY NAMES THIS ACT. `quyen` holds exactly five
	//       `feedback.*` keys — read, create, assign, resolve, restricted — and classification is
	//       none of them: it is not assignment, and rule 5, invariant 3b is explicit that these
	//       rights are not a Cartesian product to be derived. Guarding the act that fixes a
	//       citizen's resolve deadline with a neighbouring key is a guess about authority inside
	//       a public body.
	//       The URL noun is missing too: `ubiquitous-language.md` maps tiếp nhận, thụ lý, nghiệm
	//       thu and đóng phiếu, and deliberately does not map phân loại — and that file says to
	//       stop and ask rather than translate on the spot, because a path a commune is already
	//       running cannot be taken back.
	//
	//   POST /api/cong/citizen-reports  (công dân gửi)
	//       This service has no citizen edge and cannot build one today: httpx.CitizenEdge needs
	//       an httpx.CitizenSessions, the only implementation is inside
	//       service-identity/internal/ (unreachable, rule 2 forbidden #1), and identity's .proto
	//       has no RPC that resolves a citizen session. Adding one is the contract owner's work,
	//       not this service's.
	//
	// The store already carries the mechanical half of the two staff writes — Tao and
	// ChotLinhVuc, each taking a *store.ScopedTx so the audit entry cannot be written anywhere
	// but inside the same transaction (rule 6, invariant 3). What is missing above is the
	// DECISIONS, not the plumbing.

	// @summary  Một phiếu phản ánh, tra theo mã tra cứu đã trả cho người dân
	// @screen   09-phan-anh-nguoi-dan §8
	// 404 is the single answer to four different causes — no such code, another commune's code, a
	// soft-deleted petition, and a petition in the restricted field `can-bo` read by somebody
	// without `feedback.restricted`. The reasoning for folding the last one in is on
	// Handler.khongTimThay: a 403 there would confirm to a colleague that a report about them
	// exists.
	//
	// 403 is RequirePermission's answer to an account without `feedback.read`. 401 is its answer
	// to no session AND to a session issued by another commune — those are one answer because
	// authz.xacNhanXa refuses before the permission is ever consulted (rule 5, invariant 3), so
	// no query runs and the response cannot differ by commune.
	//
	// @reply    200 phieuPhanAnhRa
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/citizen-reports/{maTraCuu}",
		authz.RequirePermission(d.Checker, "feedback.read")(
			http.HandlerFunc(h.DocPhieuPhanAnh)))
}
