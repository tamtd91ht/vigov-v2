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

	"github.com/vihat/vigov/core/authz"
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
type Deps struct {
	// Checker is unused by the single route below (it is AnyAuthenticated) and is kept because the
	// next route will almost certainly need it. It is deliberately NOT in the refusal switch: this
	// service cannot build a Checker yet — cmd/server step 4 is still a TODO — and panicking on a
	// dependency no mounted route consults would stop a process that otherwise serves correctly.
	Checker authz.Checker

	LoaiTaiNguyen LoaiTaiNguyenDanhMuc

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
}
