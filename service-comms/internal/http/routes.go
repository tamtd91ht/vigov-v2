package http

// Routes for the comms service.
//
// EVERY route declares its permission explicitly. The global guard only rejects users who are
// not signed in; it does not check permissions. A route with no declaration is callable by
// every staff role, and nothing reports it — see rule 5.
//
// Four declarations are available, and there is no fifth:
//
//	authz.RequirePermission(checker, "content.read")   // the normal case
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

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// LoaiTaiNguyenDanhMuc is the commune's map-asset-type catalogue, for GET /api/v1/map-asset-types.
//
// AN INTERFACE DECLARED AT THE POINT OF USE, not the concrete *store.LoaiTaiNguyenBanDoStore. The
// route carries the isolation this service exists to enforce — the commune check before any read —
// and that has to be testable without a PostgreSQL, or it gets tested once and then never again.
// The concrete store satisfies this as written; nothing was changed to accommodate it.
//
// NO page.Request PARAMETER: this route returns the whole list on purpose. The reason is on
// store.LoaiTaiNguyenBanDoStore.DanhSach, and the bound that replaces the missing `limit` is
// store.TranDanhMucLoaiTaiNguyen.
type LoaiTaiNguyenDanhMuc interface {
	DanhSach(ctx context.Context) ([]domain.LoaiTaiNguyenBanDo, error)
}

// Deps are everything the routes need. Kept explicit so wiring stays in cmd/server.
// GhiLoaiTaiNguyen is the WRITE half of the map-asset-type catalogue, and it is a second interface
// rather than three more methods on the read one — on purpose.
//
// The read is a store call; each of these three opens a TRANSACTION and writes an audit entry
// inside it (rule 6, invariant 3). Behind one interface a future caller would reach for whichever
// method was nearest and could end up writing the row outside a transaction, which is the exact
// defect core/audit was shaped to make impossible. Two interfaces, two obligations, visible at the
// point of use.
type GhiLoaiTaiNguyen interface {
	Them(ctx context.Context, yc app.YeuCauThemLoaiTaiNguyen, nguoi audit.Actor) (domain.LoaiTaiNguyenBanDo, error)
	Sua(ctx context.Context, id string, yc app.YeuCauSuaLoaiTaiNguyen, nguoi audit.Actor) (domain.LoaiTaiNguyenBanDo, error)
	Xoa(ctx context.Context, id, lyDo string, nguoi audit.Actor) error
}

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

// ThongBaoDanhSach is the READ half of the internal announcement book, for
// GET /api/v1/announcements. AN INTERFACE DECLARED AT THE POINT OF USE, for the same reason
// LoaiTaiNguyenDanhMuc is: the route carries the isolation this service exists to enforce, and
// that has to be testable without a PostgreSQL or it gets tested once and then never again.
type ThongBaoDanhSach interface {
	DanhSach(ctx context.Context, yc page.Request) (page.Result[domain.ThongBaoNoiBo], error)
}

// GhiThongBao is the WRITE half, and it is a second interface rather than another method on the
// read one — same argument as GhiLoaiTaiNguyen: the read is a store call, while this opens a
// TRANSACTION and writes an audit entry inside it (rule 6, invariant 3). Two interfaces, two
// obligations, visible at the point of use.
type GhiThongBao interface {
	PhatHanh(ctx context.Context, yc domain.YeuCauSoanThongBao, nguoi audit.Actor) (domain.ThongBaoNoiBo, error)
}

// --- Mini App content (docs/ui-ux/11-noi-dung-mini-app.md) ----------------------------------------
//
// FOUR INTERFACES FOR TWO TABLES, split the same way as the two above: reads on one side, writes on
// the other. The reads are store calls; each write opens a TRANSACTION and puts an audit entry
// inside it (rule 6, invariant 3). Behind one interface a future caller would reach for whichever
// method was nearest and could end up writing the row outside a transaction, which is the exact
// defect core/audit was shaped to make impossible. Declared at the POINT OF USE so the routes stay
// testable without a PostgreSQL — a test that needs infrastructure is a test that stops being run.

// NoiDungMiniAppDoc is the READ half of the content register.
//
// TWO METHODS AND NOT ONE, because the list does NOT carry the article body and the detail does —
// see store.cotNoiDungMiniApp for the figures. They answer two different questions.
type NoiDungMiniAppDoc interface {
	DanhSach(ctx context.Context, loc commsstore.LocNoiDung, yc page.Request) (
		page.Result[domain.NoiDungMiniApp], error)
	TheoID(ctx context.Context, id string) (domain.NoiDungMiniApp, error)
}

// GhiNoiDungMiniApp is the WRITE half of the content register.
type GhiNoiDungMiniApp interface {
	Them(ctx context.Context, yc domain.YeuCauThemNoiDung, nguoi audit.Actor) (domain.NoiDungMiniApp, error)
	Sua(ctx context.Context, id string, yc domain.YeuCauSuaNoiDung, nguoi audit.Actor) (domain.NoiDungMiniApp, error)
}

// DanhMucMiniAppDoc is the READ half of the commune's Mini App category tree. NO page.Request
// PARAMETER: this route returns the whole tree on purpose, and the bound that replaces the missing
// `limit` is store.TranDanhMucMiniApp.
type DanhMucMiniAppDoc interface {
	DanhSach(ctx context.Context) ([]domain.DanhMucMiniApp, error)
}

// GhiDanhMucMiniApp is the WRITE half of the category tree. ONE METHOD: §9 lists GET and POST, and
// editing or removing a category is not shipped — internal/store says why, and the reason for the
// edit route in particular is that RE-PARENTING can create a cycle no CHECK constraint can refuse.
type GhiDanhMucMiniApp interface {
	Them(ctx context.Context, yc domain.YeuCauThemDanhMuc, nguoi audit.Actor) (domain.DanhMucMiniApp, error)
}

type Deps struct {
	// Checker WAS unused and deliberately outside the refusal switch, and the comment here said so:
	// no mounted route declared authz.RequirePermission. The three catalogue WRITE routes below do,
	// so a nil Checker would now make authz.RequirePermission meet a nil interface on the first
	// request from an administrator — which is the worst possible moment to find out.
	Checker authz.Checker

	LoaiTaiNguyen    LoaiTaiNguyenDanhMuc
	GhiLoaiTaiNguyen GhiLoaiTaiNguyen

	// The internal announcement book (docs/ui-ux/08-thong-bao.md) — see internal/http/
	// thong_bao_noi_bo.go for what is shipped, what is not, and which question blocks the rest.
	ThongBao    ThongBaoDanhSach
	GhiThongBao GhiThongBao

	// Mini App content (docs/ui-ux/11-noi-dung-mini-app.md) — see internal/http/noi_dung_mini_app.go
	// for what is shipped, what is not, and which question blocks the rest.
	NoiDung           NoiDungMiniAppDoc
	GhiNoiDung        GhiNoiDungMiniApp
	DanhMucNoiDung    DanhMucMiniAppDoc
	GhiDanhMucNoiDung GhiDanhMucMiniApp

	Log *slog.Logger
}

// Register mounts the comms routes.
//
// Add every new route with its permission declaration in the SAME statement, never on a nearby
// line: rbac_guard anchors to the statement, and so should a reader.
func Register(mux *http.ServeMux, d Deps) {
	// Refusing incomplete wiring HERE, at construction, not at request time: a route mounted
	// without its store would answer every caller with a panic recovered into a 500, and the first
	// person to find out would be a member of staff in front of a broken map. Same discipline as
	// authz.Public("") and idem.KhongCan("").
	if d.LoaiTaiNguyen == nil {
		panic("comms/http: thiếu kho danh mục loại tài nguyên bản đồ — GET /api/v1/map-asset-types sẽ panic khi có người gọi")
	}
	if d.GhiLoaiTaiNguyen == nil {
		panic("comms/http: thiếu use case ghi danh mục loại tài nguyên bản đồ — POST/PATCH/DELETE /api/v1/map-asset-types sẽ panic khi có người gọi")
	}
	if d.ThongBao == nil {
		panic("comms/http: thiếu kho sổ thông báo nội bộ — GET /api/v1/announcements sẽ panic khi có người gọi")
	}
	if d.GhiThongBao == nil {
		panic("comms/http: thiếu use case phát hành thông báo — POST /api/v1/announcements sẽ panic khi có người gọi")
	}
	if d.NoiDung == nil {
		panic("comms/http: thiếu kho nội dung Mini App — GET /api/v1/content-items sẽ panic khi có người gọi")
	}
	if d.GhiNoiDung == nil {
		panic("comms/http: thiếu use case ghi nội dung Mini App — POST/PATCH /api/v1/content-items sẽ panic khi có người gọi")
	}
	if d.DanhMucNoiDung == nil {
		panic("comms/http: thiếu kho danh mục Mini App — GET /api/v1/content-categories sẽ panic khi có người gọi")
	}
	if d.GhiDanhMucNoiDung == nil {
		panic("comms/http: thiếu use case ghi danh mục Mini App — POST /api/v1/content-categories sẽ panic khi có người gọi")
	}
	if d.Checker == nil {
		panic("comms/http: thiếu authz.Checker — mười một tuyến có khai quyền sẽ không kiểm được quyền")
	}

	h := NewHandler(d)

	// --- the commune's map-asset-type catalogue -----------------------------------------------
	//
	// `map-asset-types` — THE NOUN WAS LOOKED UP, NOT TRANSLATED. The entity is `MapAssetType`
	// (migration 0003, `-- @entity`), and `asset` is already the word this system uses for these
	// records: `asset.read` / `asset.update` are settled at docs/ui-ux/10-ban-do-kinh-te-so.md:255.
	// Choosing `resource` or `poi` here would give one concept two English words on two surfaces,
	// which is exactly what `feedback.*` versus `citizen-reports` already costs
	// (kb/00-foundation/ubiquitous-language.md §Khoá quyền feedback.*).
	//
	// STATED GAP: the URL column of row :156 in that table still reads *(chưa chốt)*, and :165 says
	// the column is empty because no catalogue had a route yet. That stops being true with this
	// statement. Filling the row belongs to whoever owns that file — writing it from here would be
	// a second copy of the mapping (rule 9, forbidden #2).
	//
	// AnyAuthenticated, AND THE REASON IS THE SHAPE OF THE DATA'S USE — the same call the user
	// accepted for GET /api/v1/org-units. Group names fill the selector on the economic map, the
	// filter beside it and the label of every asset already filed, so requiring a configuration
	// permission would not protect anything: it would break those screens for everybody who is not
	// an administrator. There is nothing sensitive in the list of buckets a commune sorts its own
	// map into.
	//
	// THE TRADE-OFF, STATED RATHER THAN GLOSSED: a commune's catalogue is readable by every
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
	// What is accepted
	// is that a member of staff with no configuration rights can see how their own authority files
	// what is on its map.
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// WHAT THIS ROUTE ANSWERS TODAY IS AN EMPTY LIST, for every commune, and that is correct rather
	// than unfinished: migration 0003 creates the table and seeds nothing, because the
	// specification contradicts itself about the code list (11 groups at
	// docs/ui-ux/10-ban-do-kinh-te-so.md:37 against 8 at :53, in two different spellings) and
	// because commune onboarding — the step that would sow a commune's system rows — does not
	// exist. Neither question is settled by mounting a read route.
	//
	// @summary  Danh mục loại tài nguyên bản đồ của xã — dùng cho ô chọn nhóm trên bản đồ kinh tế số, bộ lọc và nhãn của tài nguyên đã lưu
	// @screen   10-ban-do-kinh-te-so §2
	// 500 covers two different causes and says so honestly: an ordinary store failure, and the
	// commune's catalogue exceeding store.TranDanhMucLoaiTaiNguyen — which this route REFUSES rather
	// than truncating, because a silently short list is a group missing from the selector.
	//
	// 401 covers two causes as well, and both really are answered by authz.AnyAuthenticated: no
	// session at all, and a session issued by another commune presented at this one's domain. There
	// is deliberately no 403 line — this route checks no permission, so it has none to refuse.
	//
	// @reply    200 danhSachLoaiTaiNguyenRa
	// @reply    401 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/map-asset-types",
		authz.AnyAuthenticated("tên nhóm tài nguyên xuất hiện ở ô chọn nhóm trên bản đồ kinh tế số, bộ lọc bên cạnh và nhãn của mọi tài nguyên đã lưu — đòi một quyền cấu hình sẽ làm hỏng những màn hình đó cho mọi tài khoản không phải quản trị; đánh đổi đã chấp nhận: danh mục lộ cho mọi tài khoản đã đăng nhập CỦA CHÍNH XÃ ĐÓ, không chéo xã vì Scoped buộc tenant_id")(
			http.HandlerFunc(h.DanhSachLoaiTaiNguyen)))

	// --- the commune adds a map asset type of its own -------------------------------------------
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
	// @summary  Thêm một loại tài nguyên bản đồ của riêng xã vào danh mục
	// @screen   14-cau-hinh §5
	// @request  themLoaiTaiNguyenVao
	// @reply    201 loaiTaiNguyenRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/map-asset-types",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.ThemLoaiTaiNguyen))))

	// --- the commune edits one row --------------------------------------------------------------
	//
	// PATCH AND NOT PUT: three of the four editable fields have a meaningful zero, so a full
	// replacement cannot tell "not mentioned" from "set to zero" — see suaLoaiTaiNguyenVao.
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
	// @summary  Sửa nhãn, thứ tự, trạng thái dùng hoặc đặt mặc định cho một loại tài nguyên bản đồ
	// @screen   14-cau-hinh §5
	// @request  suaLoaiTaiNguyenVao
	// @reply    200 loaiTaiNguyenRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/map-asset-types/{id}",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("sửa là ghi đè một trạng thái đã biết; app.Sua không ghi gì khi không có trường nào đổi, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.SuaLoaiTaiNguyen))))

	// --- the commune retires one of its own rows ------------------------------------------------
	//
	// THIS IS A SOFT DELETE AND THE METHOD IS THE ONLY THING THAT SAYS OTHERWISE. The row stays,
	// carrying `deleted_at`, `deleted_by` and `delete_reason` (rule 7, invariant 1), and its `ma`
	// stays taken forever — an issued code is never reissued, because map asset records hold it as
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
	// @request  xoaLoaiTaiNguyenVao
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/map-asset-types/{id}",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("xoá một dòng đã xoá cho cùng một kết quả: câu UPDATE mang `AND deleted_at IS NULL` nên lần thứ hai không ghi đè được người xoá và lý do")(
				http.HandlerFunc(h.XoaLoaiTaiNguyen))))

	// --- the commune's internal announcement book -----------------------------------------------
	//
	// `announcements` — THE NOUN WAS LOOKED UP, NOT TRANSLATED: it is the settled mapping for this
	// meaning of `thông báo` in kb/00-foundation/ubiquitous-language.md, which splits the one
	// Vietnamese word into three resources for three audiences. §7's `/api/thong-bao` sketch cannot
	// ship — ADR 0011 puts path segments in English, and `thong-bao` is a blocked segment.
	//
	// `announcement.create` ON THE READ ROUTE IS A KNOWN GAP AND IS ARGUED IN FULL at the top of
	// internal/http/thong_bao_noi_bo.go. In one line: the `quyen` table has no read key for this
	// group, §2's `Cả sổ thông báo` wants one, and rule 5 invariant 3c forbids inventing it — so
	// the book is readable by whoever may compose announcements, which UNDER-grants rather than
	// over-grants, and §2's `Gửi cho tôi` filter is not shipped at all.
	//
	// NO idem.* DECLARATION ON THE GET: it changes no state.
	//
	// @summary  Sổ thông báo nội bộ của xã — một trang thẻ, mới nhất ở trên, kèm bộ đếm xác nhận
	// @screen   08-thong-bao §2, §3
	// 400 is page.Parse refusing a cursor, a sort key or a limit; there is no other client input on
	// this route. 403 is a real answer here, unlike on the catalogue read: this route checks a key.
	// @reply    200 page.Result[thongBaoRa]
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/announcements",
		authz.RequirePermission(d.Checker, "announcement.create")(
			http.HandlerFunc(h.DanhSachThongBao)))

	// --- the commune issues an announcement to named staff ---------------------------------------
	//
	// `announcement.create` IS THE KEY THE SPECIFICATION ITSELF NAMES — §9.1, "Quyền soạn & phát
	// hành: announcement.create" — and it is seeded at service-identity/migrations/0001_init.sql:286,
	// so a commune administrator can actually tick it. NO NEW KEY WAS INVENTED (rule 5, invariant
	// 3c): a key no migration seeds is a right nobody can grant, so the route would answer 403 to
	// every account forever while the tests stayed green.
	//
	// idem.Required(DongKhiHong), AND THE CHOICE IS THE OPPOSITE OF THE CATALOGUE'S. There is NO
	// unique key underneath this write — an announcement has no code and two identical notices are
	// two legitimate rows — so a Redis outage with MoKhiHong would let a double-submitted form
	// issue the SAME announcement twice, to the same colleagues, with two acknowledgement counters.
	// The idempotency key is the ONLY layer here, so it cannot be allowed to fail open. Refusing
	// while the cache is down costs one button; the alternative is telling a commune's staff the
	// same thing twice and being unable to say which copy is the real one.
	//
	// 501 is on this list and is not a placeholder: an announcement addressed to DEPARTMENTS is
	// refused, because expanding one into its staff needs a service-identity RPC that does not
	// exist. See app.ErrGuiTheoBoPhanChuaCo — the alternative was delivering to nobody, silently.
	//
	// @summary  Phát hành một thông báo nội bộ tới các cán bộ được chọn đích danh
	// @screen   08-thong-bao §5
	// @request  phatHanhThongBaoVao
	// @reply    201 thongBaoRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    501 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/announcements",
		authz.RequirePermission(d.Checker, "announcement.create")(
			idem.Required(idem.DongKhiHong)(
				http.HandlerFunc(h.PhatHanhThongBao))))

	// --- the commune's Mini App content register ------------------------------------------------
	//
	// `content-items` AND `content-categories` — THE NOUNS WERE TAKEN FROM THE RUNNING SIBLING
	// IMPLEMENTATION, NOT TRANSLATED, and they are the ONE thing in this module still owed a
	// decision. kb/00-foundation/ubiquitous-language.md §Tên tài nguyên trên URL has NO ROW for any
	// concept in chapter 11, and its own instruction for that case is "dừng lại và hỏi" (:219-221).
	// The full argument, the evidence (`../vigov-require/apps/api/app/modules/content/router.py:54,
	// 66, 186`) and what is still owed are at the top of internal/http/noi_dung_mini_app.go. §9's own
	// `/api/mini-app/noi-dung` and `/api/cong/…` sketches cannot ship at all.
	//
	// `content.read` AND `content.update` ARE THE SPECIFICATION'S OWN KEYS (§10.5) AND BOTH EXIST —
	// service-identity/migrations/0001_init.sql:292-293, group `NỘI DUNG MINI APP`. NO NEW KEY WAS
	// INVENTED, and that is rule 5, invariant 3c: a key no migration seeds is a right nobody can
	// grant, so the route would answer 403 to every account forever while the tests stayed green.
	// There is no `content.create` in the table and none was added — composing and editing are both
	// `content.update`, which is how §10.5 itself divides the surface.
	//
	// THE KEYS ARE LITERALS AT EVERY CALL SITE AND NOT CONSTANTS, although QuyenDocNoiDung and
	// QuyenSuaNoiDung exist for the prose to refer to: tools/apidoc resolves the key from the
	// authz.RequirePermission call and refuses anything that is not a string literal there — "khóa
	// quyền không phải hằng chuỗi — không ghi vào hợp đồng được". A route whose key it cannot read is
	// a route absent from kb/20-contracts/openapi.json, which is the contract web-admin builds
	// against (ADR 0014).
	//
	// @summary  Sổ nội dung Mini App của xã — một trang của bảng §6, lọc theo loại, danh mục và tiêu đề
	// @screen   11-noi-dung-mini-app §6
	// 400 covers three refusals of what the client sent: a `type` outside §5's six codes, a search
	// term past the bound, and page.Parse refusing a cursor, a sort key or a limit. THE BODY OF EACH
	// ITEM IS NOT IN THIS RESPONSE — see noiDungRa.Body and the detail route below.
	// @reply    200 page.Result[noiDungRa]
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/content-items",
		authz.RequirePermission(d.Checker, "content.read")(
			http.HandlerFunc(h.DanhSachNoiDung)))

	// --- one item, with its body ------------------------------------------------------------------
	//
	// IT EXISTS BECAUSE THE LIST DOES NOT CARRY THE BODY, not as a second way of asking one question:
	// a hundred articles at domain.ThanNoiDungToiDa is twenty million runes in one response, and §6's
	// table shows a title and one line of summary.
	//
	// 404 COVERS "NO SUCH ITEM" AND "ANOTHER COMMUNE'S ITEM", indistinguishably and on purpose. The
	// store binds the commune to $1, so another authority's id is simply not there; telling the two
	// apart would confirm what that authority holds (rule 4, forbidden #2 on the commune axis).
	//
	// @summary  Một mục nội dung Mini App kèm toàn văn — dùng cho modal sửa ở §7
	// @screen   11-noi-dung-mini-app §7
	// @reply    200 noiDungRa
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/content-items/{id}",
		authz.RequirePermission(d.Checker, "content.read")(
			http.HandlerFunc(h.MotNoiDung)))

	// --- the commune composes an item --------------------------------------------------------------
	//
	// THE PROVENANCE IS A LITERAL IN THE INSERT, not a parameter: `nguon` is written as `'thu-cong'`
	// and `nguon_id_ngoai` / `nguon_url` / `da_sua_tay` are bound nowhere, so there is no value any
	// layer above could pass and no field a client could fill. That is what keeps §10.4's protection
	// — a hand-edited portal article survives the next sync — from being reachable by a request.
	//
	// idem.Required(DongKhiHong), AND THE CHOICE IS THE SAME AS THE ANNOUNCEMENT'S AND THE OPPOSITE OF
	// THE CATEGORY'S BELOW. There is NO unique key underneath this write — an item has no code, and
	// two articles with the same title are two legitimate rows — so a Redis outage with MoKhiHong
	// would let a double-submitted form publish the same article twice to every resident of the
	// commune, with no way to say which copy is the real one. The idempotency key is the ONLY layer
	// here, so it cannot be allowed to fail open.
	//
	// 409 AND NOT 400 for a category that is not there: the body is well-formed and the caller holds
	// the permission; what is refused is this value against the state of the data, usually a screen
	// somebody left open while a colleague retired the category.
	//
	// @summary  Soạn một mục nội dung cho Mini App — chưa bật `publish` thì bà con chưa thấy
	// @screen   11-noi-dung-mini-app §7
	// @request  themNoiDungVao
	// @reply    201 noiDungRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/content-items",
		authz.RequirePermission(d.Checker, "content.update")(
			idem.Required(idem.DongKhiHong)(
				http.HandlerFunc(h.ThemNoiDung))))

	// --- the commune edits one item ----------------------------------------------------------------
	//
	// PATCH AND NOT PUT: every field §7's modal collects has a meaningful zero, so a full replacement
	// cannot tell "not mentioned" from "cleared" — and a screen editing only the title would silently
	// unpublish the article and drop it out of its category. See suaNoiDungVao.
	//
	// THIS IS ALSO WHERE §10.4 IS RECORDED. Editing an item whose provenance is the portal sets
	// `da_sua_tay`, which migration 0006 then refuses to clear: the commune is promised that the
	// correction survives the next synchronisation.
	//
	// idem.KhongCan, AND THE REASON IS A PROPERTY OF THE USE CASE RATHER THAN A HOPE: app.Sua compares
	// the row it read against the row it would write and, when nothing moved, writes NOTHING — no
	// UPDATE, no audit entry, and no `da_sua_tay`. So the same request sent twice leaves one row in
	// one state and one entry in the ledger. Were that comparison removed, this declaration would
	// become a lie and the second request would file an entry saying nothing changed.
	//
	// @summary  Sửa một mục nội dung Mini App — sửa bài đồng bộ về sẽ khoá không cho lượt đồng bộ sau ghi đè
	// @screen   11-noi-dung-mini-app §6, §7
	// @request  suaNoiDungVao
	// @reply    200 noiDungRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/content-items/{id}",
		authz.RequirePermission(d.Checker, "content.update")(
			idem.KhongCan("sửa là ghi đè một trạng thái đã biết; app.Sua không ghi gì khi không có trường nào đổi, nên lần gửi thứ hai để lại đúng một dòng, đúng một vết, và không đặt cờ da_sua_tay")(
				http.HandlerFunc(h.SuaNoiDung))))

	// --- the commune's Mini App category tree --------------------------------------------------------
	//
	// `content.read` AND NOT AnyAuthenticated, WHICH IS THE OPPOSITE CALL FROM GET /map-asset-types
	// ABOVE — and the difference is which screens the list feeds. The eight reference catalogues fill
	// a selector on nearly every screen in the system, so a configuration permission there would empty
	// those screens for everybody who is not an administrator. This tree appears on exactly one
	// screen, the one §10.5 already gates with `content.read`. Using the key the specification names
	// costs nothing here and keeps the routes of this module answering the same question the same way.
	//
	// NOT PAGINATED ON PURPOSE — store.DanhMucMiniAppStore.DanhSach gives the three reasons, and the
	// bound that replaces the missing `limit` is store.TranDanhMucMiniApp. Past it the route REFUSES
	// with a 500 rather than truncating: a silently short tree is a category that has disappeared from
	// §7's select, so articles get filed under the wrong one and the screen looks entirely normal.
	//
	// @summary  Danh mục tin nội bộ của Mini App — cây phẳng, dùng cho ô chọn ở §7 và bộ lọc ở §6
	// @screen   11-noi-dung-mini-app §6
	// @reply    200 danhSachDanhMucRa
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/content-categories",
		authz.RequirePermission(d.Checker, "content.read")(
			http.HandlerFunc(h.DanhSachDanhMucNoiDung)))

	// --- the commune adds a category ------------------------------------------------------------------
	//
	// idem.Required(MoKhiHong), AND WHICH LAYER IS ACTUALLY PROTECTING THIS — the question
	// skills/rest-api-design §4 says to answer at the route. The real guard is
	// `UNIQUE (tenant_id, slug)`, which counts soft-deleted rows: a second category with the same slug
	// CANNOT EXIST, whatever happens to Redis. The idempotency key is the second, independent layer —
	// it is what stops a double-submitted form from producing one row and one confusing 409 instead of
	// one row and a replayed 201.
	//
	// MoKhiHong and not DongKhiHong for exactly that reason, and it is the opposite call from the two
	// content-item writes above: with the unique key underneath, a cache outage cannot produce a
	// duplicate category, so refusing a member of staff mid-configuration would be paying with an
	// outage for a risk that is already covered.
	//
	// @summary  Thêm một danh mục tin của riêng xã vào cây danh mục Mini App
	// @screen   11-noi-dung-mini-app §6
	// @request  themDanhMucVao
	// @reply    201 danhMucRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/content-categories",
		authz.RequirePermission(d.Checker, "content.update")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.ThemDanhMucNoiDung))))
}
