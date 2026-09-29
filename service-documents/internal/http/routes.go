package http

// Routes for the documents service.
//
// EVERY route declares its permission explicitly. The global guard only rejects users who are
// not signed in; it does not check permissions. A route with no declaration is callable by
// every staff role, and nothing reports it — see rule 5.
//
// Four declarations are available, and there is no fifth:
//
//	authz.RequirePermission(checker, "document.read")   // the normal case
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
//
// THE @request / @reply TYPE NAMES (`vanBanDenRa`, `themLoaiVanBanVao`, …) ARE KEPT VIETNAMESE ON
// PURPOSE in the English rename campaign's layer A. tools/apidoc publishes each one as a component
// name of kb/20-contracts/openapi.json, and layer A must regenerate that file byte for byte (ADR
// 0061 §Ba lớp). Renaming them is a contract change for web-admin's generated client, not a rename.

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/service-documents/internal/app"
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
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

// DocumentTypeLister is the commune's document-type catalogue, for GET /api/v1/document-types.
//
// AN INTERFACE DECLARED AT THE POINT OF USE, not the concrete *docstore.DocumentTypeStore. The route
// carries the isolation rules this service exists to enforce — the commune check before any read,
// the refusal to truncate — and those have to be testable without a PostgreSQL, or they get tested
// once and then never again. *docstore.DocumentTypeStore satisfies this as it is; nothing was
// changed to accommodate it.
//
// NO page.Request PARAMETER: this route returns the whole list on purpose. The reason is on
// docstore.DocumentTypeStore.List, and the bound that replaces the missing `limit` is
// docstore.MaxDocumentTypes.
type DocumentTypeLister interface {
	List(ctx context.Context) ([]domain.DocumentType, error)
}

// DocumentTypeWriter is the WRITE half, and it is a second interface rather than three more methods
// on the one above — on purpose.
//
// The read is a store call; each of these three opens a TRANSACTION and writes an audit entry
// inside it (rule 6, invariant 3). Behind one interface a future caller would reach for whichever
// method was nearest and could end up writing the row outside a transaction, which is the exact
// defect core/audit was shaped to make impossible. Two interfaces, two obligations, visible at the
// point of use.
type DocumentTypeWriter interface {
	Create(ctx context.Context, req app.CreateDocumentTypeRequest, actor audit.Actor) (domain.DocumentType, error)
	Update(ctx context.Context, id string, req app.UpdateDocumentTypeRequest, actor audit.Actor) (domain.DocumentType, error)
	SoftDelete(ctx context.Context, id, reason string, actor audit.Actor) error

	// The Excel import (ADR 0059 §3) — routes_document_type_import.go. On THIS interface because it is
	// the same use case writing the same table under the same transaction discipline; the preview opens
	// a transaction too (to read the snapshot) and always rolls it back.
	PreviewDocumentTypeImport(ctx context.Context, rows []domain.DocumentTypeImportRow) (app.DocumentTypeImportResult, error)
	ImportDocumentTypes(ctx context.Context, rows []domain.DocumentTypeImportRow, actor audit.Actor) (app.DocumentTypeImportResult, error)
}

// IncomingDocumentLister is the READ half of the incoming register, declared at the point of use.
//
// IT TAKES page.Request AND A FILTER STRUCT, NOT A URL — the handler parses and validates; the store
// binds. Nothing between them assembles SQL, which is what keeps a government register free of an
// injection point.
type IncomingDocumentLister interface {
	List(ctx context.Context, filter docstore.IncomingDocumentFilter, req page.Request) (page.Result[domain.IncomingDocument], error)
}

// IncomingDocumentWriter is the WRITE half, and it is a second interface rather than four more
// methods on the one above — on purpose, and for the reason DocumentTypeWriter states: each of these
// opens a TRANSACTION and writes an audit entry inside it (rule 6, invariant 3), and one of them
// allocates an issued number under a row lock. Behind one interface a future caller would reach for
// whichever method was nearest and could end up writing the row outside a transaction — which for
// this register means a number issued with no document behind it.
type IncomingDocumentWriter interface {
	Register(ctx context.Context, req app.RegisterIncomingDocumentRequest, actor audit.Actor) (domain.IncomingDocument, error)
	Update(ctx context.Context, id string, req app.UpdateIncomingDocumentRequest, actor audit.Actor) (domain.IncomingDocument, error)
	Remove(ctx context.Context, id, reason string, actor audit.Actor) error
	Route(ctx context.Context, id string, req app.RouteDocumentRequest, actor audit.Actor) (domain.IncomingDocument, error)
}

// IncomingDocumentReader is the detail drawer's two reads (§3.5). A THIRD interface rather than two
// more methods on IncomingDocumentWriter, because that one's contract is "each method writes an
// audit entry in its transaction" and these write nothing. *app.IncomingDocuments satisfies it.
type IncomingDocumentReader interface {
	Detail(ctx context.Context, id string) (domain.IncomingDocument, error)
	RoutingHistory(ctx context.Context, id string) ([]domain.DocumentRouting, error)
}

// OutgoingDocumentLister and OutgoingDocumentWriter are the same split for the outgoing register.
type OutgoingDocumentLister interface {
	List(ctx context.Context, filter docstore.OutgoingDocumentFilter, req page.Request) (page.Result[domain.OutgoingDocument], error)
}

type OutgoingDocumentWriter interface {
	IssueNumber(ctx context.Context, req app.IssueOutgoingDocumentRequest, actor audit.Actor) (domain.OutgoingDocument, error)
	Update(ctx context.Context, id string, req app.UpdateOutgoingDocumentRequest, actor audit.Actor) (domain.OutgoingDocument, error)
	Remove(ctx context.Context, id, reason string, actor audit.Actor) error
}

// IncomingSummaryReader counts the dashboard figures of the incoming register.
// *docstore.IncomingDocumentStore satisfies it.
type IncomingSummaryReader interface {
	CountIncomingSummary(ctx context.Context, window domain.ArrivalWindow, now time.Time) (domain.IncomingSummary, error)
}

// OverdueQueueReader is the "CẦN XỬ LÝ NGAY" queue. *app.IncomingDashboard satisfies it.
type OverdueQueueReader interface {
	OverdueQueue(ctx context.Context, now time.Time, limit int) ([]app.OverdueQueueItem, error)
}

// Deps are everything the routes need. Kept explicit so wiring stays in cmd/server.
type Deps struct {
	// Checker guards the routes declared with authz.RequirePermission. THREE ROUTES NOW USE ONE,
	// so a nil Checker is refused at construction — the comment here used to say the opposite and
	// pointed at exactly this moment. Without the refusal, a nil interface would panic on the
	// first write a member of staff attempted, long after the deployment that caused it.
	Checker            authz.Checker
	DocumentTypes      DocumentTypeLister
	DocumentTypeWriter DocumentTypeWriter

	// The two registers this service exists for. Each is refused at construction when missing, for
	// the reason the switch in Register states.
	IncomingDocuments      IncomingDocumentLister
	IncomingDocumentWriter IncomingDocumentWriter
	IncomingDocumentReader IncomingDocumentReader
	OutgoingDocuments      OutgoingDocumentLister
	OutgoingDocumentWriter OutgoingDocumentWriter

	// The leadership dashboard's incoming-register block (/tong-quan §4.2, §5). Both refused at
	// construction when missing.
	IncomingSummary IncomingSummaryReader
	OverdueQueue    OverdueQueueReader

	// AuditLog reads this service's own `audit_log` for the "Xem nhật ký hệ thống" screen (ADR 0054).
	// *audit.Log in production. Refused at construction when missing.
	AuditLog AuditLogReader

	// Clock is the instant `overdue` is judged at, for the summary, the queue and the drill-down.
	// A TEST SEAM: nil in production means the real clock, UTC.
	Clock func() time.Time

	Log *slog.Logger
}

// Register mounts the documents routes.
//
// Add every new route with its permission declaration in the SAME statement, never on a nearby
// line: rbac_guard anchors to the statement, and so should a reader.
func Register(mux *http.ServeMux, d Deps) {
	// Refusing incomplete wiring HERE, at construction, not at request time: a route mounted
	// without the store behind it would accept requests it cannot honour, and the first person to
	// find out would be a member of staff registering a document in a government system.
	switch {
	case d.DocumentTypes == nil:
		panic("documents/http: thiếu kho loại văn bản — GET /api/v1/document-types sẽ panic khi có người gọi")
	case d.DocumentTypeWriter == nil:
		panic("documents/http: thiếu use case ghi danh mục loại văn bản — POST/PATCH/DELETE /api/v1/document-types sẽ panic khi có người gọi")
	case d.IncomingDocuments == nil:
		panic("documents/http: thiếu kho sổ văn bản đến — GET /api/v1/incoming-documents sẽ panic khi có người gọi")
	case d.IncomingDocumentWriter == nil:
		panic("documents/http: thiếu use case ghi sổ văn bản đến — các tuyến vào sổ / sửa / gỡ / chuyển sẽ panic")
	case d.IncomingDocumentReader == nil:
		panic("documents/http: thiếu use case đọc chi tiết văn bản đến — GET /api/v1/incoming-documents/{id} và /routings sẽ panic")
	case d.OutgoingDocuments == nil:
		panic("documents/http: thiếu kho sổ văn bản đi — GET /api/v1/outgoing-documents sẽ panic khi có người gọi")
	case d.OutgoingDocumentWriter == nil:
		panic("documents/http: thiếu use case ghi sổ văn bản đi — các tuyến cấp số / sửa / gỡ sẽ panic")
	case d.IncomingSummary == nil:
		panic("documents/http: thiếu kho đếm tổng quan văn bản đến — GET /api/v1/incoming-document-summary sẽ panic khi có người gọi")
	case d.OverdueQueue == nil:
		panic("documents/http: thiếu use case hàng đợi quá hạn — GET /api/v1/incoming-document-overdue-queue sẽ panic khi có người gọi")
	case d.AuditLog == nil:
		panic("documents/http: thiếu bộ đọc nhật ký hệ thống — GET /api/v1/documents-audit-entries sẽ panic khi có người gọi")
	case d.Checker == nil:
		panic("documents/http: thiếu authz.Checker — mọi tuyến ghi sẽ không kiểm được quyền")
	}

	h := NewHandler(d)

	// The document-type Excel import — three routes, all `admin.lookup` (routes_document_type_import.go).
	registerDocumentTypeImportRoutes(mux, d, h)

	// --- the commune's document-type catalogue -------------------------------------------------
	//
	// `document-types` — the entity is `DocumentType`, written on the `-- @entity` mark above the
	// table (migrations/0003_danh_muc_loai_van_ban.sql), and the path is its plural kebab-case
	// form (skills/rest-api-design REQUIRED #1). Nothing is translated on the spot here: the
	// English name was settled when the table was born, and a path cannot be taken back once a
	// commune is live. STATED GAP: the URL-resource cell for this concept in
	// kb/00-foundation/ubiquitous-language.md:158 still reads *(chưa chốt)*; filling it belongs to
	// whoever owns that table (rule 9), and this route has no external caller yet.
	//
	// AnyAuthenticated, AND THE REASON IS THE SHAPE OF THE DATA'S USE — the same call made for
	// GET /api/v1/org-units in the identity service. Type names fill the registration form, the
	// filter on every document list and the label on every document already registered, so
	// requiring a configuration permission would not protect anything: it would break those
	// screens for everybody who is not an administrator. The alternative that actually protects
	// something does not exist here — there is nothing sensitive in the list of names an authority
	// files its paperwork under.
	//
	// THE TRADE-OFF, STATED RATHER THAN GLOSSED: a commune's catalogue is readable by every
	// signed-in account OF THAT COMMUNE. It is not readable across communes — but READ THE NEXT
	// PARAGRAPH BEFORE RELYING ON WHERE THAT IS ENFORCED, because the obvious answer is wrong.
	//
	// authz.AnyAuthenticated DOES compare the principal's commune against the commune from Host,
	// and in THIS service that comparison CANNOT FAIL: core/staffauth stamps the Host commune
	// onto the principal it builds (staffauth.go:157 and :241), so authz.xacNhanXa compares a
	// value with itself. The comparison that actually decides anything happens inside identity,
	// at service-identity/internal/grpc/server.go:257, where the commune sent in `x-tenant-id`
	// metadata is compared against the commune INSIDE the credential — the only place both
	// values exist. A mismatch there yields no principal at all, and this route then answers 401
	// because there is nobody, not because the communes differed.
	//
	// WHY IT IS WRITTEN OUT: three edits would remove the protection without turning one test in
	// this service red — adding ResolveStaffPrincipal to grpcx.methodsWithoutTenant, moving that
	// comparison below the session-registry read, or "simplifying" staffauth to take the commune
	// from the response instead of from Host.
	//
	// The second wall is local and does hold on its own: Scoped.Query binds `tenant_id` from the
	// context (rule 1, invariant 5), so this query could not reach another commune's rows even
	// with a principal that lied.
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Danh mục loại văn bản của xã — dùng cho ô chọn loại khi vào sổ, bộ lọc và nhãn trên mọi văn bản
	// @screen   14-cau-hinh §5
	// 500 covers two different causes and says so honestly: an ordinary store failure, and the
	// commune's catalogue exceeding docstore.MaxDocumentTypes — which this route REFUSES
	// rather than truncating, because a silently short list files a document under the wrong type
	// and numbering follows the type.
	//
	// 403 IS ABSENT ON PURPOSE: authz.AnyAuthenticated answers 401 for a missing principal AND for
	// a token issued by another commune, and never 403. Declaring one would name a status this
	// handler cannot produce.
	//
	// @reply    200 danhSachLoaiVanBanRa
	// @reply    401 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/document-types",
		authz.AnyAuthenticated("tên loại văn bản xuất hiện ở ô chọn loại khi vào sổ, bộ lọc của mọi danh sách văn bản và nhãn trên từng văn bản đã vào sổ — đòi một quyền cấu hình sẽ làm hỏng những màn hình đó cho mọi tài khoản không phải quản trị; đánh đổi đã chấp nhận: danh mục lộ cho mọi tài khoản đã đăng nhập CỦA CHÍNH XÃ ĐÓ, không chéo xã vì Scoped buộc tenant_id")(
			http.HandlerFunc(h.ListDocumentTypes)))

	// --- the commune adds a document type of its own -------------------------------------------
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
	// @summary  Thêm một loại văn bản của riêng xã vào danh mục
	// @screen   14-cau-hinh §5
	// @request  themLoaiVanBanVao
	// @reply    201 loaiVanBanRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/document-types",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.CreateDocumentType))))

	// --- the commune edits one row --------------------------------------------------------------
	//
	// PATCH AND NOT PUT: three of the four editable fields have a meaningful zero, so a full
	// replacement cannot tell "not mentioned" from "set to zero" — see suaLoaiVanBanVao.
	//
	// WHAT EACH TIER ALLOWS IS DECIDED PER FIELD AND PER TRANSITION, not per route: relabel and
	// reorder at every tier, disable at tiers 1 and 2, and `ma` nowhere. The refusal is a 409
	// naming the tier, and the trigger refuses the same thing underneath (ADR 0024).
	//
	// idem.KhongCan, AND THE REASON IS A PROPERTY OF THE USE CASE RATHER THAN A HOPE: app.Update
	// compares the row it read against the row it would write and, when nothing moved, writes
	// NOTHING — no UPDATE and no audit entry. So the same request sent twice leaves one row in one
	// state and one entry in the ledger. Were that comparison removed, this declaration would
	// become a lie and the second request would file a second entry saying nothing changed.
	//
	// @summary  Sửa nhãn, thứ tự, trạng thái dùng hoặc đặt mặc định cho một loại văn bản
	// @screen   14-cau-hinh §5
	// @request  suaLoaiVanBanVao
	// @reply    200 loaiVanBanRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/document-types/{id}",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("sửa là ghi đè một trạng thái đã biết; app.Sua không ghi gì khi không có trường nào đổi, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.UpdateDocumentType))))

	// --- the commune retires one of its own rows ------------------------------------------------
	//
	// THIS IS A SOFT DELETE AND THE METHOD IS THE ONLY THING THAT SAYS OTHERWISE. The row stays,
	// carrying `deleted_at`, `deleted_by` and `delete_reason` (rule 7, invariant 1), and its `ma`
	// stays taken forever — an issued code is never reissued, because document records hold it as
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
	// @request  xoaLoaiVanBanVao
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/document-types/{id}",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("xoá một dòng đã xoá cho cùng một kết quả: câu UPDATE mang `AND deleted_at IS NULL` nên lần thứ hai không ghi đè được người xoá và lý do")(
				http.HandlerFunc(h.DeleteDocumentType))))

	// --- SỔ VĂN BẢN ĐẾN -------------------------------------------------------------------------
	//
	// THE THREE KEYS BELOW ARE THE SPECIFICATION'S OWN (docs/ui-ux/05-van-ban-don-thu.md §7 rule 4)
	// and all three are seeded at service-identity/migrations/0001_init.sql:287-289. NO KEY WAS
	// INVENTED — rule 5, invariant 3c, and `tools/check_quyen.py` scans the whole repository against
	// the `quyen` table on every `make check`.
	//
	// ⚠ `document.create` GUARDS THREE ROUTES, NOT ONE, AND THAT IS A FINDING RATHER THAN A CHOICE.
	// The table has no `document.update` and no `document.delete`, so correcting an entry and
	// removing one are guarded by the key for booking. A commune may well want those separated —
	// see the block at the top of incoming_document.go. Inventing a key here would produce routes
	// that answer 403 to EVERY account forever while every test stayed green.

	// VÀO SỔ — the act that ISSUES A NUMBER, which is why this is the one route in the service with
	// idem.Required(DongKhiHong).
	//
	// DongKhiHong AND NOT MoKhiHong, and the difference is what happens during a Redis outage:
	// MoKhiHong would let a double-submitted form through, and the second submission would take THE
	// NEXT NUMBER. That is not a duplicate row that can be removed — it is a second entry in an
	// archival register, and the number it consumed can never be given back (rule 7, invariant 3).
	// skills/rest-api-design reserves DongKhiHong for exactly this: acts with legal consequences,
	// naming "issued document numbers" outright. Refusing to book while the cache is down is the
	// cheaper failure, and it is visible.
	//
	// @summary  Vào sổ một văn bản đến; hệ thống cấp số đến và ấn định hạn xử lý theo cấu hình của xã
	// @screen   05-van-ban-don-thu §3.4
	// @request  themVanBanDenVao
	// @reply    201 vanBanDenRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/incoming-documents",
		authz.RequirePermission(d.Checker, "document.create")(
			idem.Required(idem.DongKhiHong)(
				http.HandlerFunc(h.RegisterIncomingDocument))))

	// PATCH AND NOT PUT: three of the seven editable fields have a meaningful empty value, so a full
	// replacement cannot tell "not mentioned" from "cleared" — see suaVanBanDenVao.
	//
	// idem.KhongCan, AND THE REASON IS A PROPERTY OF THE USE CASE RATHER THAN A HOPE: app.Update
	// compares the row it read against the row it would write and, when nothing moved, writes
	// NOTHING — no UPDATE and no audit entry. So the same request sent twice leaves one row in one
	// state and one entry in the ledger.
	//
	// @summary  Sửa thông tin một văn bản đến đã vào sổ (số đến, trạng thái và hạn xử lý không sửa được)
	// @screen   05-van-ban-don-thu §3.4
	// @request  suaVanBanDenVao
	// @reply    200 vanBanDenRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/incoming-documents/{id}",
		authz.RequirePermission(d.Checker, "document.create")(
			idem.KhongCan("sửa là ghi đè một trạng thái đã biết; app.Sua không ghi gì khi không có trường nào đổi, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.UpdateIncomingDocument))))

	// THIS IS A SOFT DELETE AND THE METHOD IS THE ONLY THING THAT SAYS OTHERWISE. The row stays,
	// carrying `deleted_at`, `deleted_by` and `delete_reason` (rule 7, invariant 1), and ITS NUMBER
	// STAYS TAKEN — the series never goes back, so the removed entry leaves a visible gap. That gap
	// is the record. DELETE is still the right method: the resource is gone from every read path.
	//
	// A BODY ON A DELETE, and the alternative was worse: the reason is mandatory, and the query
	// string would put free text about a government record into every access log and proxy cache.
	//
	// @summary  Gỡ một văn bản đến khỏi sổ (xoá mềm, kèm lý do bắt buộc; số đến không được cấp lại)
	// @screen   05-van-ban-don-thu §3.1
	// @request  goVanBanVao
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/incoming-documents/{id}",
		authz.RequirePermission(d.Checker, "document.create")(
			idem.KhongCan("gỡ một văn bản đã gỡ cho cùng một kết quả: câu UPDATE mang `AND deleted_at IS NULL` nên lần thứ hai không ghi đè được người gỡ và lý do")(
				http.HandlerFunc(h.RemoveIncomingDocument))))

	// CHUYỂN XỬ LÝ — the chairman's instruction and the office's handover, in one transaction with
	// the timeline entry and the audit entry.
	//
	// `document.route` IS ITS OWN KEY AND THE SPECIFICATION SAYS SO (§7 rule 4). It is the one act
	// on this register that decides WHO IS RESPONSIBLE, which is not the same right as being able to
	// type a document into the book.
	//
	// idem.KhongCan, AND IT IS THE HONEST DECLARATION RATHER THAN THE COMFORTABLE ONE: a second
	// identical routing DOES write a second timeline entry, because the timeline is append-only and
	// records acts, not states. That is correct — two routings happened — so this route is not
	// idempotent and does not claim to be. `idem.Required` would not help either: it would hide the
	// second act instead of recording it.
	//
	// @summary  Chuyển văn bản đến cho một bộ phận xử lý, kèm ý kiến chỉ đạo — ghi vào dòng thời gian không sửa được
	// @screen   05-van-ban-don-thu §3.5
	// @request  chuyenVanBanVao
	// @reply    200 vanBanDenRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/incoming-documents/{id}/routings",
		authz.RequirePermission(d.Checker, "document.route")(
			idem.KhongCan("mỗi lần chuyển là một hành vi có thật và để lại một dòng lịch sử riêng — bảng lịch sử chỉ thêm, không sửa (luật 7 cấm #5), nên gửi lại là một lần chuyển nữa chứ không phải một bản sao")(
				http.HandlerFunc(h.RouteIncomingDocument))))

	// THE REGISTER ITSELF. `document.read` and not AnyAuthenticated: unlike the type catalogue, this
	// is the commune's correspondence — issuing bodies, summaries, and free text that may name a
	// citizen (rule 3). The specification gives it its own key for that reason.
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Danh sách sổ văn bản đến, phân trang theo con trỏ, lọc theo năm · trạng thái · loại · bộ phận đang giữ
	// @screen   05-van-ban-don-thu §3.1
	// @reply    200 page.Result[vanBanDenRa]
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/incoming-documents",
		authz.RequirePermission(d.Checker, "document.read")(
			http.HandlerFunc(h.ListIncomingDocuments)))

	// ONE DOCUMENT, for the detail drawer. `document.read` for the reason the list gives. The item
	// is the SAME shape as one list item (vanBanDenRa) — a second shape would be a second contract
	// for one row.
	//
	// 404 IS ONE ANSWER FOR THREE CASES: an id that never existed, one removed from the register
	// (rule 7, invariant 2) and one belonging to another commune (rule 1). The query binds the
	// commune from the context and excludes removed rows, so it cannot tell them apart — and a
	// different body for any of them would tell a caller what another authority holds.
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Một văn bản đến của xã, cùng dạng với một dòng của danh sách — dùng cho ngăn chi tiết
	// @screen   05-van-ban-don-thu §3.5
	// @reply    200 vanBanDenRa
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/incoming-documents/{id}",
		authz.RequirePermission(d.Checker, "document.read")(
			http.HandlerFunc(h.GetIncomingDocument)))

	// THE ROUTING TIMELINE of one document, oldest first — "Dòng thời gian chuyển tiếp". Read-only;
	// the table is append-only (rule 7, forbidden #5). `document.read` and not `document.route`:
	// seeing who holds a document is reading the register, not routing it.
	//
	// NOT PAGINATED, BOUNDED: the whole timeline or a refusal (500) past docstore.MaxRoutingHistory —
	// a history missing its first page reads as complete.
	//
	// 404 identical to the route above when the document is not visible: the use case reads the
	// document first, in the same transaction.
	//
	// @summary  Dòng thời gian chuyển xử lý của một văn bản đến, cũ nhất trước — chỉ đọc
	// @screen   05-van-ban-don-thu §3.5
	// @reply    200 danhSachLichSuChuyenRa
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/incoming-documents/{id}/routings",
		authz.RequirePermission(d.Checker, "document.read")(
			http.HandlerFunc(h.ListIncomingDocumentRoutings)))

	// --- TỔNG QUAN ĐIỀU HÀNH — the incoming register's block ------------------------------------------
	//
	// TWO KEYS, BOTH REQUIRED, NESTED: `report.read` because this is the dashboard (the key the
	// reporting service is built around), AND `document.read` because the figures and the queue are
	// read out of the commune's correspondence. Holding one is 403. Both keys are seeded at
	// service-identity/migrations/0001_init.sql — no key was invented (rule 5, invariant 3c).
	//
	// ⚠ tools/apidoc RECORDS ONE KEY PER ROUTE and keeps the LAST RequirePermission it visits, so
	// kb/20-contracts/openapi.json will list `document.read` alone for these two routes. The runtime
	// check is both keys; the contract under-states it. Reported rather than worked around.
	//
	// SINGULAR NOUNS, ON PURPOSE: each is ONE resource per commune — the summary and the queue — not a
	// collection with ids. The drill-down lists behind the figures are the ordinary collection,
	// GET /api/v1/incoming-documents?metric=…, so the rows are the register's own shape.
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Ba số của khối Văn bản trên Tổng quan: đến trong kỳ, chưa xử lý xong, quá hạn xử lý
	// @screen   01-tong-quan-dieu-hanh §4.2
	// @reply    200 incomingSummaryOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/incoming-document-summary",
		authz.RequirePermission(d.Checker, "report.read")(
			authz.RequirePermission(d.Checker, "document.read")(
				http.HandlerFunc(h.IncomingDocumentSummary))))

	// THE "CẦN XỬ LÝ NGAY" ROWS of the incoming register: at most 10 overdue documents, the longest
	// missed deadline first, each marked `critical` once 48 WORKING hours have passed since it was missed.
	// Keys as above.
	//
	// 503 when identity cannot count the working hours: every `critical` flag depends on it, and a
	// queue answered without it would say nothing is urgent.
	//
	// @summary  Văn bản đến quá hạn cần xử lý ngay (tối đa 10), trễ lâu nhất trước, kèm cờ nghiêm trọng
	// @screen   01-tong-quan-dieu-hanh §5
	// @reply    200 overdueQueueOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/incoming-document-overdue-queue",
		authz.RequirePermission(d.Checker, "report.read")(
			authz.RequirePermission(d.Checker, "document.read")(
				http.HandlerFunc(h.IncomingDocumentOverdueQueue))))

	// --- SỔ VĂN BẢN ĐI --------------------------------------------------------------------------
	//
	// ⚠ NO SPECIFICATION EXISTS FOR THIS REGISTER. The routes mirror the incoming ones minus routing
	// and minus the deadline; the permission keys are the same two, because the `quyen` table has no
	// others for this subsystem. See outgoing_document.go.

	// CẤP SỐ VĂN BẢN ĐI — the heaviest write in the service. The number it issues goes onto paper,
	// under a seal, to a district office or a citizen.
	//
	// idem.Required(DongKhiHong) for the same reason as the incoming register, one step stronger: a
	// double submission during a cache outage would issue a SECOND OUTGOING NUMBER, and the document
	// carrying the first one is already outside this commune.
	//
	// @summary  Cấp số văn bản đi và ghi vào sổ; số đã cấp không bao giờ cấp lại
	// @screen   *(chưa có đặc tả — xem migration 0004)*
	// @request  capSoVanBanDiVao
	// @reply    201 vanBanDiRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/outgoing-documents",
		authz.RequirePermission(d.Checker, "document.create")(
			idem.Required(idem.DongKhiHong)(
				http.HandlerFunc(h.IssueOutgoingDocument))))

	// @summary  Sửa thông tin một văn bản đi đã cấp số (số đi và năm không sửa được)
	// @screen   *(chưa có đặc tả — xem migration 0004)*
	// @request  suaVanBanDiVao
	// @reply    200 vanBanDiRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/outgoing-documents/{id}",
		authz.RequirePermission(d.Checker, "document.create")(
			idem.KhongCan("sửa là ghi đè một trạng thái đã biết; app.Sua không ghi gì khi không có trường nào đổi, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.UpdateOutgoingDocument))))

	// @summary  Gỡ một văn bản đi khỏi sổ (xoá mềm, kèm lý do bắt buộc; số đi không được cấp lại)
	// @screen   *(chưa có đặc tả — xem migration 0004)*
	// @request  goVanBanVao
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/outgoing-documents/{id}",
		authz.RequirePermission(d.Checker, "document.create")(
			idem.KhongCan("gỡ một văn bản đã gỡ cho cùng một kết quả: câu UPDATE mang `AND deleted_at IS NULL` nên lần thứ hai không ghi đè được người gỡ và lý do")(
				http.HandlerFunc(h.RemoveOutgoingDocument))))

	// @summary  Danh sách sổ văn bản đi, phân trang theo con trỏ, lọc theo năm · loại văn bản
	// @screen   *(chưa có đặc tả — xem migration 0004)*
	// @reply    200 page.Result[vanBanDiRa]
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/outgoing-documents",
		authz.RequirePermission(d.Checker, "document.read")(
			http.HandlerFunc(h.ListOutgoingDocuments)))

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
	// @summary  Nhật ký hệ thống của phân hệ Văn bản — vết thao tác của xã, mới nhất trước, lọc theo thời gian · người · động từ · đối tượng
	// @screen   14-cau-hinh §12.1
	// @reply    200 page.Result[audit.EntryView]
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/documents-audit-entries",
		authz.RequirePermission(d.Checker, "admin.audit")(
			http.HandlerFunc(h.ListAuditEntries)))
}
