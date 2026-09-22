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
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
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

type Deps struct {
	// Checker WAS unused and deliberately outside the refusal switch, and the comment here said so:
	// no mounted route declared authz.RequirePermission. The three catalogue WRITE routes below do,
	// so a nil Checker would now make authz.RequirePermission meet a nil interface on the first
	// request from an administrator — which is the worst possible moment to find out.
	Checker authz.Checker

	LoaiTaiNguyen    LoaiTaiNguyenDanhMuc
	GhiLoaiTaiNguyen GhiLoaiTaiNguyen

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
	if d.Checker == nil {
		panic("comms/http: thiếu authz.Checker — ba tuyến ghi danh mục sẽ không kiểm được quyền")
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
}
