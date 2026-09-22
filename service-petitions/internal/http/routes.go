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

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/service-petitions/internal/app"
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

	// VetXemNguoiGui records ONE disclosure of a reporter's full name and phone number.
	//
	// IT IS A DEPENDENCY OF A READ ROUTE, which looks wrong and is not: rule 6, invariant 7 audits
	// reading FULL personal data, and ADR 0030 makes that the condition attached to the
	// `feedback.unmask` key rather than a follow-up task. The handler cannot write the entry
	// itself — audit.Write takes a *store.ScopedTx and there is no overload that writes outside a
	// transaction — so the use case owns it and the route depends on the use case.
	//
	// THE INTERFACE TAKES NO TRANSACTION AND RETURNS ONLY AN ERROR, and that shape is the
	// contract: an implementation either committed the entry or it did not, and the route treats
	// "did not" as "do not disclose". There is no third answer for a caller to interpret.
	VetXemNguoiGui interface {
		GhiVet(ctx context.Context, maTraCuu string, nguoi audit.Actor) error
	}

	// NhanLinhVucDanhMuc is the commune's OVERRIDES for the petition field labels — tier 2 of
	// ADR 0026. A code with no row here is valid and simply carries the platform's default
	// label; absence is never "unknown field".
	NhanLinhVucDanhMuc interface {
		DanhSach(ctx context.Context) ([]domain.NhanLinhVuc, error)
	}
)

// Deps are everything the routes need. Kept explicit so wiring stays in cmd/server.
// THE PERMISSION ON THE SIX CATALOGUE WRITE ROUTES BELOW IS `admin.lookup`, WRITTEN OUT AT EVERY CALL SITE.
//
// A CONSTANT WOULD READ BETTER AND IS DELIBERATELY NOT USED: tools/apidoc resolves the key from the
// authz.RequirePermission call and refuses anything that is not a string literal there — "khóa
// quyền không phải hằng chuỗi — không ghi vào hợp đồng được". A route whose key it cannot read is a
// route absent from kb/20-contracts/openapi.json, which is the contract the admin web builds
// against (ADR 0014). Six literals that a whole-repository scan checks beat one constant the
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

// The WRITE halves of the two reference catalogues this service owns. Two interfaces, two
// obligations, visible at the point of use — see the note on the Deps fields.
type GhiLoaiNhiemVu interface {
	Them(ctx context.Context, yc app.YeuCauThemLoaiNhiemVu, nguoi audit.Actor) (domain.LoaiNhiemVu, error)
	Sua(ctx context.Context, id string, yc app.YeuCauSuaLoaiNhiemVu, nguoi audit.Actor) (domain.LoaiNhiemVu, error)
	Xoa(ctx context.Context, id, lyDo string, nguoi audit.Actor) error
}

type GhiMucUuTien interface {
	Them(ctx context.Context, yc app.YeuCauThemMucUuTien, nguoi audit.Actor) (domain.MucUuTienNhiemVu, error)
	Sua(ctx context.Context, id string, yc app.YeuCauSuaMucUuTien, nguoi audit.Actor) (domain.MucUuTienNhiemVu, error)
	Xoa(ctx context.Context, id, lyDo string, nguoi audit.Actor) error
}

type Deps struct {
	// Checker decides permissions. IT IS NOW LOAD-BEARING: GET /api/v1/citizen-reports/{maTraCuu}
	// declares authz.RequirePermission, and the handler consults the Checker TWICE more — once
	// for the restricted field, once for `feedback.unmask`. A nil one is therefore in the panic
	// switch in Register — see the note there, which predicted exactly this route.
	Checker authz.Checker

	LoaiNhiemVu LoaiNhiemVuDanhMuc
	MucUuTien   MucUuTienDanhMuc

	// The WRITE half of each catalogue, and a second interface per catalogue rather than three
	// more methods on the read one — on purpose. The read is a store call; each of these three
	// opens a TRANSACTION and writes an audit entry inside it (rule 6, invariant 3). Behind one
	// interface a future caller would reach for whichever method was nearest and could end up
	// writing the row outside a transaction, which is the exact defect core/audit was shaped to
	// make impossible.
	GhiLoaiNhiemVu GhiLoaiNhiemVu
	GhiMucUuTien   GhiMucUuTien

	Phieu       PhieuPhanAnhDoc
	NhanLinhVuc NhanLinhVucDanhMuc

	// Vet writes the trail for a full-view read. Required, not optional — see the panic switch.
	Vet VetXemNguoiGui

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
	case d.GhiLoaiNhiemVu == nil:
		panic("petitions/http: thiếu use case ghi danh mục loại nhiệm vụ — POST/PATCH/DELETE /api/v1/task-types sẽ panic khi có người gọi")
	case d.GhiMucUuTien == nil:
		panic("petitions/http: thiếu use case ghi danh mục mức ưu tiên — POST/PATCH/DELETE /api/v1/task-priorities sẽ panic khi có người gọi")
	case d.Checker == nil:
		panic("petitions/http: thiếu Checker — GET /api/v1/citizen-reports/{maTraCuu} khai quyền feedback.read, " +
			"và một Checker rỗng sẽ panic lúc có cán bộ gọi chứ không phải lúc khởi động")
	case d.Phieu == nil:
		panic("petitions/http: thiếu kho phiếu phản ánh — GET /api/v1/citizen-reports/{maTraCuu} sẽ panic khi có người gọi")
	case d.NhanLinhVuc == nil:
		panic("petitions/http: thiếu kho nhãn lĩnh vực — GET /api/v1/citizen-reports/{maTraCuu} sẽ panic khi có người gọi")
	case d.Vet == nil:
		// THE MOST DANGEROUS OF THE FIVE TO LEAVE OUT, because a nil here does not crash a screen:
		// it crashes the ONE path that discloses a citizen's name and number, and only when
		// somebody with `feedback.unmask` reads a petition. Refusing at startup is what keeps
		// "every full read leaves a trail" (rule 6, invariant 7) from depending on a wiring line
		// nobody re-reads.
		panic("petitions/http: thiếu đường ghi vết xem đầy đủ — quyền feedback.unmask mở họ tên và số " +
			"điện thoại người gửi, và luật 6 bất biến 7 không cho phép đọc đầy đủ mà không ghi vết")
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
	// TWO OF THE BLOCKERS LISTED BELOW HAVE BEEN CLEARED SINCE THIS LIST WAS WRITTEN, and they are
	// marked CLEARED rather than deleted: a reader who finds this comment through ADR 0029 §115 or
	// through the ledger needs to see that the reason moved, not that it vanished.
	//
	//   POST /api/v1/citizen-reports  (nhập hộ)
	//       Booking a petition FIXES BOTH DEADLINES at once, because the staff form carries the
	//       field (ADR 0028, decision E). Both numbers come from the `sla` table of
	//       docs/ui-ux/14-cau-hinh.md §8.
	//       CLEARED — "a table that exists in no service" was true until 2026-09-20. ADR 0029 put
	//       `sla` in identity, beside the three calendar tables, and
	//       service-identity/migrations/0008_sla.sql creates it.
	//       STILL BLOCKED, one step further along: there is NO RPC that reads the hours. identity's
	//       .proto has AdvanceWorkingHours, which answers "lúc nào" and not "bao lâu", and ADR 0029
	//       §118 says in its own words that the shape of the reading contract is undecided —
	//       whether it returns the HOURS for the caller to add, or the DEADLINE already computed.
	//       Those put the working-hours arithmetic in two different services, and ADR 0007 forbids
	//       two implementations of it. Writing the read path before that contract exists is ADR
	//       0029's stop condition #3.
	//
	//   POST …/{maTraCuu}/<classification>  (phân loại)
	//       Blocked three ways; one of the three is now clear.
	//       (1) The same `sla` question, for `gio_xu_ly_xong` of the settled field — see above.
	//       (2) ADR 0026 requires the field code to be CHECKED ON WRITE against the tier-1 set in
	//       service `platform`, and how petitions reads that set — live gRPC or an event-fed
	//       replica — still has NO ADR. kb/10-decisions/ ends at 0032 and none of 0029..0032
	//       answers it. Writing the read path without one is ADR 0026's stop condition #2.
	//       (3) CLEARED — `feedback.classify` was seeded on 2026-09-20 by
	//       service-identity/migrations/0007_quyen_phan_loai_va_xem_day_du.sql, decided by ADR
	//       0030. The key that names this act now exists and must not be replaced by a guess.
	//       The URL noun is STILL missing: `ubiquitous-language.md` maps tiếp nhận, thụ lý, nghiệm
	//       thu and đóng phiếu, and deliberately does not map phân loại — and that file says to
	//       stop and ask rather than translate on the spot, because a path a commune is already
	//       running cannot be taken back.
	//
	//   POST /api/cong/citizen-reports  (công dân gửi)
	//       PARTLY CLEARED. identity's .proto now has ResolveCitizenSession and
	//       core/identityclient returns an httpx.CitizenSessions this service can use without
	//       importing service-identity/internal/ (rule 2, forbidden #1). One thing still blocks it
	//       and it is not programming: that RPC cannot carry `x-tenant-id` — it is the call that
	//       RESOLVES the commune (ADR 0022) — so its name has to be in core/grpcx.methodsWithoutTenant,
	//       and adding a name to that list is ADR 0012 decision 1's stop condition. Until then the
	//       caller-side interceptor refuses every call with InvalidArgument, so mounting a citizen
	//       edge here would produce an edge that builds, runs, and answers 401 to every citizen.
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
	// A SECOND AND A THIRD PERMISSION ARE CONSULTED INSIDE THE HANDLER, and neither changes the
	// status code — which is why neither can be read off this block and both are stated here:
	//
	//	feedback.restricted  absent, on a `can-bo` petition -> 404, identical to an unknown code
	//	feedback.unmask      absent                         -> 200 with the reporter MASKED
	//	feedback.unmask      present                        -> 200 with the reporter in full,
	//	                                                        and one audit entry written first
	//
	// 500 therefore covers one more cause than it did: the audit entry for a full-view read
	// failing to commit. Rule 6 gives no other answer — no trail, no disclosure.
	//
	// @reply    200 phieuPhanAnhRa
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/citizen-reports/{maTraCuu}",
		authz.RequirePermission(d.Checker, "feedback.read")(
			http.HandlerFunc(h.DocPhieuPhanAnh)))

	// --- the commune adds a task type of its own -------------------------------------------
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
	// @summary  Thêm một loại nhiệm vụ của riêng xã vào danh mục
	// @screen   14-cau-hinh §5
	// @request  themLoaiNhiemVuVao
	// @reply    201 loaiNhiemVuRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/task-types",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.ThemLoaiNhiemVu))))

	// --- the commune edits one row --------------------------------------------------------------
	//
	// PATCH AND NOT PUT: three of the four editable fields have a meaningful zero, so a full
	// replacement cannot tell "not mentioned" from "set to zero" — see suaLoaiNhiemVuVao.
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
	// @summary  Sửa nhãn, thứ tự, trạng thái dùng hoặc đặt mặc định cho một loại nhiệm vụ
	// @screen   14-cau-hinh §5
	// @request  suaLoaiNhiemVuVao
	// @reply    200 loaiNhiemVuRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/task-types/{id}",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("sửa là ghi đè một trạng thái đã biết; app.Sua không ghi gì khi không có trường nào đổi, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.SuaLoaiNhiemVu))))

	// --- the commune retires one of its own rows ------------------------------------------------
	//
	// THIS IS A SOFT DELETE AND THE METHOD IS THE ONLY THING THAT SAYS OTHERWISE. The row stays,
	// carrying `deleted_at`, `deleted_by` and `delete_reason` (rule 7, invariant 1), and its `ma`
	// stays taken forever — an issued code is never reissued, because task records hold it as
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
	// @request  xoaLoaiNhiemVuVao
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/task-types/{id}",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("xoá một dòng đã xoá cho cùng một kết quả: câu UPDATE mang `AND deleted_at IS NULL` nên lần thứ hai không ghi đè được người xoá và lý do")(
				http.HandlerFunc(h.XoaLoaiNhiemVu))))
	// --- the commune adds a task priority of its own -------------------------------------------
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
	// @summary  Thêm một mức ưu tiên nhiệm vụ của riêng xã vào danh mục
	// @screen   14-cau-hinh §5
	// @request  themMucUuTienVao
	// @reply    201 mucUuTienRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/task-priorities",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.ThemMucUuTien))))

	// --- the commune edits one row --------------------------------------------------------------
	//
	// PATCH AND NOT PUT: three of the four editable fields have a meaningful zero, so a full
	// replacement cannot tell "not mentioned" from "set to zero" — see suaMucUuTienVao.
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
	// @summary  Sửa nhãn, thứ tự, trạng thái dùng hoặc đặt mặc định cho một mức ưu tiên nhiệm vụ
	// @screen   14-cau-hinh §5
	// @request  suaMucUuTienVao
	// @reply    200 mucUuTienRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/task-priorities/{id}",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("sửa là ghi đè một trạng thái đã biết; app.Sua không ghi gì khi không có trường nào đổi, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.SuaMucUuTien))))

	// --- the commune retires one of its own rows ------------------------------------------------
	//
	// THIS IS A SOFT DELETE AND THE METHOD IS THE ONLY THING THAT SAYS OTHERWISE. The row stays,
	// carrying `deleted_at`, `deleted_by` and `delete_reason` (rule 7, invariant 1), and its `ma`
	// stays taken forever — an issued code is never reissued, because task records hold it as
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
	// @request  xoaMucUuTienVao
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/task-priorities/{id}",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("xoá một dòng đã xoá cho cùng một kết quả: câu UPDATE mang `AND deleted_at IS NULL` nên lần thứ hai không ghi đè được người xoá và lý do")(
				http.HandlerFunc(h.XoaMucUuTien))))
}
