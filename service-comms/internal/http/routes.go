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

	// The Excel import (ADR 0059) — routes_map_asset_type_import.go. On THIS interface because it is
	// the same use case writing the same table under the same transaction discipline; the preview
	// opens a transaction too (to read the snapshot) and always rolls it back.
	PreviewMapAssetTypeImport(ctx context.Context, rows []domain.MapAssetTypeImportRow) (app.MapAssetTypeImportResult, error)
	ImportMapAssetTypes(ctx context.Context, rows []domain.MapAssetTypeImportRow, actor audit.Actor) (app.MapAssetTypeImportResult, error)
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
	Delete(ctx context.Context, id, reason string, actor audit.Actor) error
}

// DanhMucMiniAppDoc is the READ half of the commune's Mini App category tree. NO page.Request
// PARAMETER: this route returns the whole tree on purpose, and the bound that replaces the missing
// `limit` is store.TranDanhMucMiniApp.
type DanhMucMiniAppDoc interface {
	DanhSach(ctx context.Context) ([]domain.DanhMucMiniApp, error)
}

// GhiDanhMucMiniApp is the WRITE half of the category tree: add, edit (name, parent, order, hidden —
// re-parenting walks the ancestors under a tree lock) and soft delete with a reason (ADR 0067 §3).
type GhiDanhMucMiniApp interface {
	Them(ctx context.Context, yc domain.YeuCauThemDanhMuc, nguoi audit.Actor) (domain.DanhMucMiniApp, error)
	Update(ctx context.Context, id string, yc domain.ContentCategoryUpdate, actor audit.Actor) (domain.DanhMucMiniApp, error)
	Delete(ctx context.Context, id, reason string, actor audit.Actor) error
}

// --- the map field schema (docs/ui-ux/14-cau-hinh.md §6) -----------------------------------------
//
// Read and write split the same way as the catalogue above: the read is a store call, each write
// opens a TRANSACTION and puts its audit entry inside it (rule 6, invariant 3).

// MapFieldSchemaReader is the READ half. assetTypeCode "" means every group.
type MapFieldSchemaReader interface {
	List(ctx context.Context, assetTypeCode string) ([]domain.MapFieldSchema, error)
}

// MapFieldSchemaWriter is the WRITE half.
type MapFieldSchemaWriter interface {
	Create(ctx context.Context, req app.CreateMapFieldRequest, actor audit.Actor) (domain.MapFieldSchema, error)
	Update(ctx context.Context, id string, req app.UpdateMapFieldRequest, actor audit.Actor) (domain.MapFieldSchema, error)
	Delete(ctx context.Context, id, reason string, actor audit.Actor) error
}

// --- the commune's mail server (docs/ui-ux/14-cau-hinh.md §10) ------------------------------------
//
// Split the same way: the read opens no transaction; each write puts its audit entry inside the
// transaction it opens (rule 6, invariant 3). Both are satisfied by app.MailSettingsAdmin.

// MailSettingsReader is the READ half. It returns no password — the view has no field for one.
type MailSettingsReader interface {
	Get(ctx context.Context) (app.MailSettingsView, error)
}

// MailSettingsWriter is the WRITE half: saving, and sending the fixed test message.
type MailSettingsWriter interface {
	Save(ctx context.Context, req app.SaveMailSettingsRequest, actor audit.Actor) (app.MailSettingsView, error)
	SendTestMessage(ctx context.Context, recipient string, actor audit.Actor) error
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

	// The cover upload of an article (ADR 0047 §6 (1), ADR 0052) — internal/http/content_cover.go.
	// Built even when object storage, the scanner or platform's limits are absent: the two routes then
	// answer 503 and every other route keeps serving.
	ContentCovers ContentCoverActs

	// The broadcast audio of a `truyen-thanh` item (ADR 0067 §4) — internal/http/content_audio.go. Built
	// even when object storage, the scanner or platform's limits are absent: its two routes then answer
	// 503 and every other route keeps serving.
	ContentAudio ContentAudioActs

	// The map field schema (migration 0007) — see internal/http/map_field_schema.go.
	MapFieldSchemas      MapFieldSchemaReader
	WriteMapFieldSchemas MapFieldSchemaWriter

	// The commune's mail server (migration 0008) — see internal/http/mail_settings.go.
	MailSettings      MailSettingsReader
	WriteMailSettings MailSettingsWriter

	// AuditLog reads this service's own `audit_log` for the "Xem nhật ký hệ thống" screen (ADR 0054).
	// *audit.Log in production. Refused at construction when missing.
	AuditLog AuditLogReader

	// The header bell — the signed-in staff member's own inbox (migration 0010) — see
	// internal/http/routes_staff_notification.go.
	StaffInbox      StaffInboxReader
	WriteStaffInbox StaffInboxWriter

	// The portal sync (migration 0013, ADR 0067 §2) — internal/http/routes_portal_sync.go. Built even
	// without SECRET_ENCRYPTION_KEYS: its writes then answer 503 and every other route keeps serving.
	PortalSync      PortalSyncReader
	WritePortalSync PortalSyncWriter

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
	if d.ContentCovers == nil {
		panic("comms/http: thiếu use case ảnh bìa — hai tuyến /api/v1/content-items/cover-images và chi tiết nội dung sẽ panic khi có người gọi")
	}
	if d.ContentAudio == nil {
		panic("comms/http: thiếu use case âm thanh truyền thanh — hai tuyến /api/v1/content-items/audio-files và chi tiết nội dung sẽ panic khi có người gọi")
	}
	if d.MapFieldSchemas == nil {
		panic("comms/http: thiếu kho trường bản đồ — GET /api/v1/map-field-schemas sẽ panic khi có người gọi")
	}
	if d.WriteMapFieldSchemas == nil {
		panic("comms/http: thiếu use case ghi trường bản đồ — POST/PATCH/DELETE /api/v1/map-field-schemas sẽ panic khi có người gọi")
	}
	if d.MailSettings == nil {
		panic("comms/http: thiếu kho máy chủ thư — GET /api/v1/mail-settings sẽ panic khi có người gọi")
	}
	if d.WriteMailSettings == nil {
		panic("comms/http: thiếu use case ghi máy chủ thư — PUT /api/v1/mail-settings và POST …/test-messages sẽ panic khi có người gọi")
	}
	if d.AuditLog == nil {
		panic("comms/http: thiếu bộ đọc nhật ký hệ thống — GET /api/v1/comms-audit-entries sẽ panic khi có người gọi")
	}
	if d.StaffInbox == nil || d.WriteStaffInbox == nil {
		panic("comms/http: thiếu kho hoặc use case hộp chuông — GET/PATCH /api/v1/notifications sẽ panic khi có người gọi")
	}
	if d.PortalSync == nil || d.WritePortalSync == nil {
		panic("comms/http: thiếu use case đồng bộ Cổng — sáu tuyến /api/v1/portal-sync/… sẽ panic khi có người gọi")
	}
	if d.Checker == nil {
		panic("comms/http: thiếu authz.Checker — mọi tuyến có khai quyền sẽ không kiểm được quyền")
	}

	h := NewHandler(d)

	// The header bell — four routes, the caller's own inbox only (routes_staff_notification.go).
	registerStaffNotificationRoutes(mux, h)

	// The map-asset-type Excel import — three routes, all `admin.lookup` (routes_map_asset_type_import.go).
	registerMapAssetTypeImportRoutes(mux, d, h)

	// The portal sync — six routes, `content.read` / `content.update` (routes_portal_sync.go).
	registerPortalSyncRoutes(mux, h)

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
	// 400 is `invalid_request` with the domain's own sentence naming the field — including migration
	// 0011's per-type refusals: an event field on a type other than `su-kien`, `video_url` on a type
	// other than `video`, an end with no start or before the start, a place past 500 characters or
	// with a control character, an instant that is not RFC 3339 with an offset, a non-http(s) link.
	// 409 is `category_missing`. `published_at` is never a request field: `publish: true` fixes it (G1).
	//
	// `cover_image_file_id` (optional): a completed cover upload issued without `content_item_id`; the
	// item takes the id that upload reserved. 409 `cover_not_usable` for any other file. Published with a
	// cover, the derivative is copied to the public bucket in the same transaction; the copy failing is
	// 503 `cover_publish_unavailable` and nothing is written (app/content_cover.go, coverPublisher).
	//
	// `body` IS SANITISED BEFORE IT IS STORED (ADR 0067 §1, internal/richtext): p, br, strong, em, ul,
	// ol, li, h2, h3 and https links survive, nothing else. 422 is a banner field refused (migration
	// 0012): `invalid_link_to`, `invalid_display_order`, `banner_fields_only_for_banner`,
	// `banner_cover_required`.
	//
	// @summary  Soạn một mục nội dung cho Mini App — chưa bật `publish` thì bà con chưa thấy
	// @screen   11-noi-dung-mini-app §7
	// @request  themNoiDungVao
	// @reply    201 noiDungRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    422 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
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
	// 400 is `invalid_request` for the same refusals as the create, judged on the row AFTER the merge
	// (a PATCH of `event_ends_at` alone is checked against the stored start); a `type` change away from
	// `su-kien` / `video` clears that type's fields in the same UPDATE instead of refusing. 404 is
	// `not_found`, 409 `category_missing`. The edit that first moves the item into `dang-hien` fixes
	// `published_at` (G1); no edit changes it afterwards.
	//
	// `cover_image_file_id`: "" detaches, an id attaches a completed upload issued for THIS item (409
	// `cover_not_usable` otherwise). The cover's public copy follows the item: published with a cover →
	// copied in the transaction (503 `cover_publish_unavailable` if that fails, nothing written);
	// unpublished, detached or replaced → the old copy is withdrawn after commit.
	//
	// A `body` sent is sanitised like the create's; a body not sent is left exactly as stored. 422 as on
	// the create, plus `banner_cover_required` when the edit removes a banner's cover or turns an item
	// into a banner without one. A type change away from `banner` clears `link_to` / `display_order`.
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
	// @reply    422 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("PATCH /api/v1/content-items/{id}",
		authz.RequirePermission(d.Checker, "content.update")(
			idem.KhongCan("sửa là ghi đè một trạng thái đã biết; app.Sua không ghi gì khi không có trường nào đổi, nên lần gửi thứ hai để lại đúng một dòng, đúng một vết, và không đặt cờ da_sua_tay")(
				http.HandlerFunc(h.SuaNoiDung))))

	// --- the commune retires one item (user decision 2026-10-02) -------------------------------------
	//
	// The per-row delete of the requirement prototype (../vigov-require/apps/api/app/modules/content/
	// router.py:120-126, guarded by `content.update`), with ONE DIFFERENCE: the reason is MANDATORY (rule
	// 7, invariant 1; migration 0006's CHECK `noi_dung_mini_app_xoa_mem_day_du`). The shape is
	// DELETE /api/v1/staff/{id}'s in service-identity: a body `{ "reason": "…" }`, 204, no body back.
	//
	// `content.update`, THE KEY THE PROTOTYPE USES and the one every write of this screen carries; seeded
	// at service-identity/migrations/0001_init.sql:293. No key invented (rule 5, invariant 3c).
	//
	// THE ITEM LEAVES THE MINI APP IN THE SAME UPDATE (`trang_thai = 'an'` as a literal), and its cover's
	// public copy is withdrawn after commit, the way an unpublish withdraws it (app.SoanNoiDungMiniApp.Delete).
	// `nguon_id_ngoai` is kept, so the portal sync counts the article as skipped_deleted and never imports
	// it again.
	//
	//	400 invalid_request   no / blank reason, a reason past 500 characters, a body that is not JSON.
	//	404 not_found         already deleted, another commune's id, or invented — one answer (rule 4,
	//	                      forbidden #2 on the commune axis).
	//
	// A BODY ON A DELETE: a query string would put free text about a public record into every access log
	// and proxy cache.
	//
	// idem.KhongCan — deleting an already-deleted item is a 404 either way, and the second request cannot
	// overwrite who deleted it or why: both the locked read and the UPDATE carry `AND deleted_at IS NULL`.
	//
	// @summary  Xoá mềm một mục nội dung Mini App, kèm lý do bắt buộc — gỡ khỏi Mini App ngay trong cùng lần ghi
	// @screen   11-noi-dung-mini-app §6
	// @request  deleteContentItemIn
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/content-items/{id}",
		authz.RequirePermission(d.Checker, "content.update")(
			idem.KhongCan("xoá một mục đã xoá cho cùng một kết quả: lượt đọc khoá dòng và câu UPDATE đều mang `AND deleted_at IS NULL`, nên lần thứ hai trả 404 và không ghi đè được người xoá và lý do")(
				http.HandlerFunc(h.DeleteContentItem))))

	// --- the cover image of an article: ADR 0052's three-step upload -----------------------------------
	//
	// internal/app/content_cover.go has the whole flow; internal/http/content_cover.go says why the noun
	// is `cover-images` and why it sits at collection level. `content.update` on both: uploading a cover
	// is composing the article (§10.5 divides the screen into read and update, nothing else). The key is
	// seeded (service-identity/migrations/0001_init.sql:293); NO KEY WAS INVENTED (rule 5, invariant 3c).
	//
	// THE LIMITS ARE PLATFORM'S (`content-image` policy, ADR 0052 §10): size and types on every request.
	// Not configured → 503 `storage_not_configured`, as is a missing object store or scanner.
	//
	// idem.Required(idem.MoKhiHong): a double submit issues a second pending row — one unused upload
	// slot for 15 minutes, never a second stored file, never a second article. A cache outage must not
	// stop a member of staff mid-composition (petitions' attachment route makes the same call).
	//
	// 404 is a named `content_item_id` that is no live article of this commune — the same answer as
	// another commune's (rule 4, forbidden #2 on the commune axis).
	//
	// @summary  Xin tải ảnh bìa cho mục nội dung Mini App — trả biểu mẫu tải thẳng lên kho lưu tệp (15 phút); bỏ trống content_item_id khi bài chưa lưu
	// @screen   11-noi-dung-mini-app §7
	// @request  coverUploadIn
	// @reply    201 coverUploadOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/content-items/cover-images",
		authz.RequirePermission(d.Checker, "content.update")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.RequestCoverUpload))))

	// HOÀN TẤT TẢI ẢNH BÌA — `completion`, a nominalised sub-resource (skills/rest-api-design §3). Only
	// the officer the upload was issued to; anybody else's id answers 404.
	//
	// Stat · sniff (the client's type is never trusted) · the CURRENT policy · ClamAV · sha256 · copy to
	// the private bucket · decode, orient, fit to 1280 px, re-encode JPEG without EXIF · `ready` and the
	// trail in ONE transaction. Infected, wrong type, too large, undecodable, too many pixels → 422.
	// Scanner or platform down → 503, nothing written, retryable, NEVER stored unscanned (ADR 0052 §9).
	//
	// idem.KhongCan: a second completion of a ready file answers that file and writes nothing; two in
	// flight at once serialise on the row lock and the loser lands on the winner's row.
	//
	// @summary  Hoàn tất tải ảnh bìa — dò kiểu, quét mã độc, lưu bản gốc riêng tư, tạo bản 1280px không EXIF
	// @screen   11-noi-dung-mini-app §7
	// @reply    200 coverFileOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    422 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/content-items/cover-images/{id}/completion",
		authz.RequirePermission(d.Checker, "content.update")(
			idem.KhongCan("hoàn tất lần hai trên ảnh đã sẵn sàng trả lại đúng ảnh ấy và không ghi gì; hai lượt cùng lúc tuần tự hoá trên khoá dòng")(
				http.HandlerFunc(h.CompleteCoverUpload))))

	// --- the images INSIDE the body: the same three-step upload (ADR 0067 §Sửa đổi 03/10/2026) -----------
	//
	// internal/http/content_body_image.go has the contract; internal/app/content_cover.go the flow. The
	// cover's pipeline under purpose `content-body-image`: sniff, ClamAV, `thumb-1280` derivative without
	// EXIF, published and withdrawn with the article. `content.update` on both — K8 names it, the key the
	// cover routes use, seeded (service-identity/migrations/0001_init.sql:293); NO KEY WAS INVENTED (rule 5,
	// invariant 3c). No rate limit of its own (K8): authenticated, audited, and bounded by the per-article
	// count of platform's policy (H7, 20 on 03/10/2026 — read per request, never a constant here).
	//
	// 404 is a `content_item_id` that is neither a live article of this commune nor an id an earlier upload
	// of THIS officer reserved — one answer for all (rule 4, forbidden #2 on the commune axis). 409
	// `body_image_limit` when the article already holds the policy's count of live body images.
	//
	// idem.Required(idem.MoKhiHong): as on the cover route — a double submit issues one more pending row
	// (one slot for 15 minutes), never a stored file.
	//
	// @summary  Xin tải ảnh chèn trong thân bài nội dung Mini App — trả biểu mẫu tải thẳng lên kho lưu tệp (15 phút) và mã mục nội dung; bỏ trống content_item_id cho ảnh đầu tiên của bài chưa lưu
	// @screen   11-noi-dung-mini-app §7
	// @request  bodyImageUploadIn
	// @reply    201 bodyImageUploadOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/content-items/body-images",
		authz.RequirePermission(d.Checker, "content.update")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.RequestBodyImageUpload))))

	// HOÀN TẤT TẢI ẢNH THÂN BÀI — the cover completion's acts on a body-image upload. Only the officer it
	// was issued to; anybody else's id, and a cover's id, answer 404. 422 `body_image_rejected` for an
	// infected, mistyped, oversized, undecodable or over-count file; 503 when the scanner or platform is
	// down — nothing written, retryable, NEVER stored unscanned (ADR 0052 §9).
	//
	// @summary  Hoàn tất tải ảnh thân bài — dò kiểu, quét mã độc, lưu bản gốc riêng tư, tạo bản 1280px không EXIF, trả liên kết xem trước khi đã sẵn sàng
	// @screen   11-noi-dung-mini-app §7
	// @reply    200 bodyImageFileOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    422 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/content-items/body-images/{id}/completion",
		authz.RequirePermission(d.Checker, "content.update")(
			idem.KhongCan("hoàn tất lần hai trên ảnh đã sẵn sàng trả lại đúng ảnh ấy và không ghi gì; hai lượt cùng lúc tuần tự hoá trên khoá dòng")(
				http.HandlerFunc(h.CompleteBodyImageUpload))))

	// ẢNH THÂN BÀI TỪ LIÊN KẾT — the server downloads a pasted https link (ADR 0067 §Sửa đổi 03/10/2026,
	// H5, K6) and stores it as a READY body image: the residents' phones then load it from ViGov's public
	// store, never from the pasted host. `from-url` is a creation of the same collection by another source
	// (skills/rest-api-design §3), so it sits under `body-images`.
	//
	// `content.update` — K8 names this route too; the cover routes' key, seeded
	// (service-identity/migrations/0001_init.sql:293); NO KEY WAS INVENTED (rule 5, invariant 3c). No rate
	// limit of its own (K8): authenticated, audited, bounded by the per-article count of platform's policy.
	//
	// THE OUTBOUND CALL IS AN SSRF SURFACE (internal/imagefetch): the URL shape is checked before any
	// network (400 `invalid_image_url`), every resolved address at dial time with the portal's predicate,
	// ≤ 3 re-checked redirects, verified TLS, 30 s. Every network failure is ONE answer, 502
	// `image_fetch_failed` — distinct answers would let a caller map internal addresses. Over the size cap,
	// not an image, infected, undecodable → 422 `body_image_rejected`; the 21st image → 409
	// `body_image_limit`; scanner / storage / platform limits down → 503, nothing stored.
	//
	// idem.Required(idem.MoKhiHong): as on the upload request — a double submit replays the first answer;
	// the worst a missed replay does is one more fetched image in the article's count.
	//
	// @summary  Tải ảnh thân bài từ liên kết https cán bộ dán — máy chủ tải về, dò kiểu, quét mã độc, tạo bản 1280px không EXIF, trả ảnh đã sẵn sàng kèm liên kết xem trước; bỏ trống content_item_id cho ảnh đầu tiên của bài chưa lưu
	// @screen   11-noi-dung-mini-app §7
	// @request  bodyImageFromURLIn
	// @reply    201 bodyImageFileOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    422 httpx.Error
	// @reply    500 httpx.Error
	// @reply    502 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/content-items/body-images/from-url",
		authz.RequirePermission(d.Checker, "content.update")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.FetchBodyImageFromURL))))

	// --- the broadcast audio of a `truyen-thanh` item: ADR 0052's three-step upload (ADR 0067 §4) -------
	//
	// internal/app/content_audio.go has the whole flow and says why it differs from the cover: the item
	// must already be a saved `truyen-thanh`, the file is ATTACHED BY THE COMPLETION (with the duration
	// the officer typed), and nothing of it is ever public — residents get a short-lived presigned GET.
	//
	// `content.update` ON BOTH: ADR 0067 §4.3 names it ("người tải cần content.update"); it is the key
	// the cover routes use and is seeded (service-identity/migrations/0001_init.sql:293). NO KEY WAS
	// INVENTED (rule 5, invariant 3c).
	//
	// THE LIMITS ARE PLATFORM'S (`content-audio`, platform 0013: 30 MiB, MP3/M4A, ONE file per item).
	// Not configured → 503 `storage_not_configured`, as is a missing object store or scanner.
	//
	// idem.Required(idem.MoKhiHong): a double submit issues at most one pending row — the second meets the
	// one-file count (409 `audio_limit`) — never a second stored file. A cache outage must not stop an
	// officer mid-composition (the cover route makes the same call).
	//
	// 404 is a `content_item_id` that is no live item of this commune — the same answer as another
	// commune's (rule 4, forbidden #2). 422 `audio_only_for_truyen_thanh` for an item of another type.
	// 409 `audio_limit` when the item already has its file (remove it first: PATCH audio_file_id "").
	//
	// @summary  Xin tải tệp âm thanh (MP3/M4A) cho mục truyền thanh đã lưu — trả biểu mẫu tải thẳng lên kho lưu tệp (15 phút)
	// @screen   11-noi-dung-mini-app §7
	// @request  audioUploadIn
	// @reply    201 audioUploadOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    422 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/content-items/audio-files",
		authz.RequirePermission(d.Checker, "content.update")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.RequestAudioUpload))))

	// HOÀN TẤT TẢI ÂM THANH — `completion`, as for the cover. Only the officer the upload was issued to;
	// anybody else's id answers 404. Body: `audio_duration_seconds`, 1 .. 21600 (422 otherwise).
	//
	// Stat · sniff (MP3: ID3 or MPEG frame sync; M4A: `ftyp` brand) · the CURRENT policy · ClamAV · the
	// WHOLE-STREAM audio check (an M4A with a video track, or a renamed video, is refused) · sha256 · copy
	// to the private bucket · in ONE transaction: `ready`, the item's `audio_file_id` and
	// `audio_duration_seconds`, one trail entry. Infected, wrong type, too large, not audio → 422. Scanner
	// or platform down → 503, nothing written, retryable, NEVER stored unscanned (ADR 0052 §9).
	//
	// idem.KhongCan: a second completion of a ready file answers that file and writes nothing (the
	// duration it carries is NOT applied — PATCH corrects it); two in flight serialise on the row lock.
	//
	// @summary  Hoàn tất tải âm thanh truyền thanh — dò kiểu, kiểm tra đúng là âm thanh, quét mã độc, lưu bản gốc riêng tư và gắn vào mục cùng thời lượng cán bộ nhập
	// @screen   11-noi-dung-mini-app §7
	// @request  audioCompletionIn
	// @reply    200 audioFileOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    422 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/content-items/audio-files/{id}/completion",
		authz.RequirePermission(d.Checker, "content.update")(
			idem.KhongCan("hoàn tất lần hai trên tệp đã sẵn sàng trả lại đúng tệp ấy và không ghi gì; hai lượt cùng lúc tuần tự hoá trên khoá dòng")(
				http.HandlerFunc(h.CompleteAudioUpload))))

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

	// --- the commune edits one category (ADR 0067 §3) ----------------------------------------------
	//
	// `content.update`, THE SAME KEY AS THE CREATE above: managing the tree is composing the commune's
	// Mini App (§10.5 divides the screen into read and update, nothing else). Seeded at
	// service-identity/migrations/0001_init.sql:293; no key invented (rule 5, invariant 3c).
	//
	// PATCH: name, parent (`""` = root), order, hidden. `slug` is REFUSED when sent (400) — an issued
	// code never changes. A new parent that is the category itself is 400; one of its descendants is 409
	// `category_cycle` (the use case walks the ancestors under the commune's tree lock); a parent that is
	// not a live category of this commune is 409 `parent_missing`.
	//
	// idem.KhongCan: app.DanhMucNoiDungMiniApp.Update writes and audits nothing when nothing moved, so the
	// same request twice leaves one state and one entry.
	//
	// @summary  Sửa tên, danh mục cha, thứ tự hoặc cờ ẩn của một danh mục tin Mini App — slug không đổi được
	// @screen   11-noi-dung-mini-app §6
	// @request  updateCategoryIn
	// @reply    200 danhMucRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/content-categories/{id}",
		authz.RequirePermission(d.Checker, "content.update")(
			idem.KhongCan("sửa là ghi đè một trạng thái đã biết; use case không ghi gì khi không có trường nào đổi, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.UpdateContentCategory))))

	// --- the commune retires one category (ADR 0067 §3) --------------------------------------------
	//
	// A SOFT DELETE with a mandatory reason, IN THE BODY for the reason DELETE /map-asset-types/{id}
	// gives: free text about a government record does not belong in a query string. REFUSED with 409
	// `category_not_empty` while any live item (any state) or live child category is filed under it —
	// the sentence suggests hiding instead. The slug stays taken forever.
	//
	// idem.KhongCan: a second delete is a 404 — the UPDATE carries `AND deleted_at IS NULL`, so it cannot
	// overwrite who deleted it or why.
	//
	// @summary  Xoá mềm một danh mục tin Mini App kèm lý do bắt buộc — từ chối khi còn nội dung hoặc danh mục con
	// @screen   11-noi-dung-mini-app §6
	// @request  deleteCategoryIn
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/content-categories/{id}",
		authz.RequirePermission(d.Checker, "content.update")(
			idem.KhongCan("xoá một danh mục đã xoá trả 404: câu UPDATE mang `AND deleted_at IS NULL` nên lần thứ hai không ghi đè được người xoá và lý do")(
				http.HandlerFunc(h.DeleteContentCategory))))

	// --- the map field schema (Cấu hình → Trường bản đồ) ----------------------------------------------
	//
	// `map-field-schemas` IS VENDOR-CHOSEN: ubiquitous-language.md §Tên tài nguyên trên URL has no row
	// for it. It follows the entity `MapFieldSchema` (migration 0007) and the sibling
	// `map-asset-types`.
	//
	// THE KEYS ARE THE ONES ../vigov-require GIVES THESE FOUR ROUTES (docs/spec/04-api.md:58-61):
	// `asset.read` to read, `admin.lookup` to change. Both are seeded at
	// service-identity/migrations/0001_init.sql (:281, :287); no key was invented (rule 5, 3c).
	//
	// `asset.read` AND NOT AnyAuthenticated, unlike GET /map-asset-types: a type's LABEL is shown on
	// every map screen, while a form's field definitions are only needed by whoever reads or edits
	// the map's records — exactly the holders of `asset.read`.
	//
	// NO idem.* DECLARATION ON THE GET: it changes no state.
	//
	// @summary  Các trường tuỳ biến của biểu mẫu tài nguyên bản đồ — của một nhóm hoặc mọi nhóm, kể cả trường đang tắt
	// @screen   14-cau-hinh §6
	// @reply    200 mapFieldSchemaListOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/map-field-schemas",
		authz.RequirePermission(d.Checker, "asset.read")(
			http.HandlerFunc(h.ListMapFieldSchemas)))

	// idem.Required(MoKhiHong) — same reasoning as POST /map-asset-types: the real guard is
	// UNIQUE (tenant_id, asset_type_code, field_code), which counts soft-deleted rows, so a cache
	// outage cannot produce a duplicate field. The key only turns a double submit into a replayed 201.
	//
	// 409 `asset_type_missing` when the code is not a live row of THIS commune's type catalogue —
	// which, while every catalogue ships empty (migration 0003), is every request.
	//
	// @summary  Thêm một trường tuỳ biến vào biểu mẫu của một nhóm tài nguyên bản đồ
	// @screen   14-cau-hinh §6
	// @request  createMapFieldSchemaIn
	// @reply    201 mapFieldSchemaOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/map-field-schemas",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.CreateMapFieldSchema))))

	// PATCH edits label · options (relabel/append only) · is_required · sort_order · is_active
	// (`Tắt` / re-enable). The field's type, key and group are immutable and a body naming any of them
	// is refused with 400.
	//
	// idem.KhongCan: `options` is the whole list, not a delta, and app.MapFieldSchemas.Update writes
	// and audits nothing when nothing moved — so a second identical request leaves one state and one
	// entry.
	//
	// @summary  Sửa nhãn, lựa chọn (chỉ đổi nhãn hoặc thêm), bắt buộc, thứ tự hoặc bật/tắt một trường bản đồ
	// @screen   14-cau-hinh §6
	// @request  updateMapFieldSchemaIn
	// @reply    200 mapFieldSchemaOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/map-field-schemas/{id}",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("options là cả danh sách chứ không phải phần thêm, và app.MapFieldSchemas.Update không ghi gì khi không có trường nào đổi, nên lần gửi thứ hai để lại đúng một trạng thái và đúng một vết")(
				http.HandlerFunc(h.UpdateMapFieldSchema))))

	// `Xoá` — a SOFT DELETE with a mandatory reason. The row leaves every read path, its key stays
	// taken forever, and values already stored under it stay in the records (docs/ui-ux/14-cau-hinh.md
	// :202). Different from `Tắt`, which is PATCH is_active=false and stays listed.
	//
	// @summary  Xoá mềm một trường bản đồ, kèm lý do bắt buộc — mã trường không được cấp lại
	// @screen   14-cau-hinh §6
	// @request  deleteMapFieldSchemaIn
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/map-field-schemas/{id}",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("xoá một trường đã xoá trả 404: câu UPDATE mang `AND deleted_at IS NULL` nên lần thứ hai không ghi đè được người xoá và lý do")(
				http.HandlerFunc(h.DeleteMapFieldSchema))))

	// --- the commune's mail server (Cấu hình → Máy chủ thư) -------------------------------------------
	//
	// `mail-settings` IS VENDOR-CHOSEN: ubiquitous-language.md §Tên tài nguyên trên URL has no row for
	// it. See internal/http/mail_settings.go.
	//
	// `admin.lookup` ON ALL THREE, READ INCLUDED, AND IT IS THE REQUIREMENT REPOSITORY'S KEY FOR THESE
	// ROUTES (../vigov-require/docs/spec/04-api.md:33-35), seeded at
	// service-identity/migrations/0001_init.sql:281. No key was invented (rule 5, 3c). The read is NOT
	// AnyAuthenticated, unlike the catalogue reads: this is one administrative screen, and the host
	// and account of the commune's mail server feed no other.
	//
	// NO idem.* DECLARATION ON THE GET: it changes no state.
	//
	// @summary  Cấu hình máy chủ thư của xã — không bao giờ trả mật khẩu, chỉ báo đã đặt hay chưa
	// @screen   14-cau-hinh §10
	// @reply    200 mailSettingsOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/mail-settings",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			http.HandlerFunc(h.GetMailSettings)))

	// PUT AND NOT PATCH: §10 is one form saved whole (`💾 Lưu cấu hình`), and every field is sent. The
	// one exception is the password, which is write-only: blank means keep — unless host, port or
	// account changed, which is a 400.
	//
	// 503 `encryption_not_configured` when the platform has no SECRET_ENCRYPTION_KEYS — refused before
	// anything is written, whether or not the body carries a password.
	//
	// idem.KhongCan: the result is the state the body names. A save that changes nothing and carries
	// no password writes and audits nothing; one that carries a password re-seals it (fresh nonce) and
	// files a second entry saying the password was set again — which is true.
	//
	// @summary  Lưu cấu hình máy chủ thư của xã — mật khẩu chỉ ghi, để trống là giữ nguyên, đổi máy chủ/cổng/tài khoản thì phải nhập lại
	// @screen   14-cau-hinh §10
	// @request  mailSettingsIn
	// @reply    200 mailSettingsOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    503 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PUT /api/v1/mail-settings",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("lưu là ghi đè cả cấu hình bằng trạng thái trong thân; không đổi gì và không kèm mật khẩu thì không ghi và không có vết, kèm mật khẩu thì lần thứ hai chỉ niêm lại cùng mật khẩu và ghi đúng một vết nói điều đó")(
				http.HandlerFunc(h.PutMailSettings))))

	// `✉ Gửi thử` — THE FIRST OUTBOUND MAIL OF THIS SERVICE. A fixed test message, no citizen data, to
	// the one address the administrator typed, through the commune's stored settings over TLS with
	// the certificate verified (internal/mail). The attempt is audited before the send.
	//
	// 502 when the commune's server refused or could not be reached, with a code per category and a
	// sentence that never quotes the server. 409 when nothing has been saved yet.
	//
	// idem.Required(MoKhiHong): a double click must not send two messages; a cache outage costs at
	// worst a second test message to the administrator's own address, which is not worth refusing
	// the button over.
	//
	// ⚠ IN THE CLUSTER TODAY THIS ANSWERS 502 `mail_connect_failed` FOR EVERY COMMUNE:
	// deploy/base/mang/netpol.yaml opens egress on 5432 / 6379 / 9092 only. Opening 465/587 for comms
	// is a network decision for the owner, not a change made from here.
	//
	// @summary  Gửi một thư thử cố định tới địa chỉ quản trị viên nhập, qua máy chủ thư đã lưu của xã
	// @screen   14-cau-hinh §10
	// @request  mailTestIn
	// @reply    200 mailTestOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    502 httpx.Error
	// @reply    503 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/mail-settings/test-messages",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.SendTestMail))))

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
	// @summary  Nhật ký hệ thống của phân hệ Thông tin – truyền thông — vết thao tác của xã, mới nhất trước, lọc theo thời gian · người · động từ · đối tượng
	// @screen   14-cau-hinh §12.1
	// @reply    200 page.Result[audit.EntryView]
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/comms-audit-entries",
		authz.RequirePermission(d.Checker, "admin.audit")(
			http.HandlerFunc(h.ListAuditEntries)))
}
