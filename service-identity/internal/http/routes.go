package http

// Routes for the identity service.
//
// EVERY route declares its permission explicitly. The global guard only rejects users who are
// not signed in; it does not check permissions. A route with no declaration is callable by
// every staff role, and nothing reports it — see rule 5.
//
// Four declarations are available, and there is no fifth:
//
//	authz.RequirePermission(checker, "admin.user")   // the normal case
//	authz.CitizenOnly()                                     // citizen paths, isolated by identity
//	authz.AnyAuthenticated("<why any account needs this>")  // reason mandatory
//	authz.Public("<why this is public>")                    // reason mandatory
//
// EVERY route also carries an @-annotation block IMMEDIATELY above the statement — no blank
// line between. `tools/apidoc` reads it and generates kb/20-contracts/openapi.json, which is
// the type contract the admin web builds against. Without it the web types every response by
// hand, which is how v1 ended up with three hand-copied versions of one shape.
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

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/token"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The four collaborators below are interfaces, not the concrete store and use-case types.
//
// WHY: the routes and the middleware carry the isolation rules this service exists to enforce —
// the commune check before any read, the sid that may only be its owner's, the cookie with no
// Domain. Those have to be testable without a PostgreSQL, or they get tested once and then
// never again. The concrete *idstore.PhienStore, *idstore.CanBoStore, *app.DangNhap and
// *app.DangXuat satisfy these as they are; nothing was changed to accommodate them.
type (
	// PhienDoc is the session registry, read on every request.
	PhienDoc interface {
		KiemTra(ctx context.Context, sid string) (idstore.Phien, error)
		GhiNhanDung(ctx context.Context, sid string)
	}

	// CanBoDoc rebuilds the staff account behind a session.
	CanBoDoc interface {
		TheoID(ctx context.Context, id string) (domain.CanBo, error)
	}

	// CanBoDanhBa reads the commune's staff register for the two routes below.
	//
	// SEPARATE FROM CanBoDoc ON PURPOSE, although *idstore.CanBoStore satisfies both and the same
	// value is wired into each. CanBoDoc runs on EVERY request that carries a session, and its
	// query filters three conditions so a locked or withdrawn account stops working at once;
	// this one runs on one screen and filters only `deleted_at IS NULL`, because the register
	// must show the people who have no account at all. Widening CanBoDoc to carry both would put
	// the register's looser predicate one careless edit away from the authentication path.
	CanBoDanhBa interface {
		DanhSach(ctx context.Context, yc page.Request) (page.Result[domain.CanBoTomTat], error)
		ChiTiet(ctx context.Context, id string) (domain.CanBoTomTat, error)
	}

	// CanBoGhiDanhBa is the WRITE surface of the register — five use cases, one interface.
	//
	// SEPARATE FROM CanBoDanhBa ON PURPOSE, although both describe the same table and the same
	// screen. They are not the same kind of thing: that one is a store, this one is
	// *app.DanhBaCanBo, and every method here opens a transaction in which the business write and
	// its audit entry land together (rule 6, invariant 3). Merging them would put a read the two
	// GET routes depend on behind a type whose other methods can rewrite the commune's org chart —
	// and an interface is the list of things a handler CAN do.
	//
	// THE FOUR VERBS ARE FOUR METHODS AND NOT ONE `Ghi(...)` WITH A MODE. Each carries a different
	// set of the refusals the customer decided on 2026-09-22 (#10, #13, #14); a single entry point
	// would make those refusals conditionals inside one function, where the one that is missing
	// looks exactly like the one that is there.
	//
	// NO `Xoa` METHOD. #10's soft delete carries its own permission and the `quyen` table holds no
	// key that means it — see the header of can_bo_ghi.go. An unused method here would be the
	// scaffolding that makes the absence look like an oversight.
	CanBoGhiDanhBa interface {
		Them(ctx context.Context, yc app.YeuCauThemCanBo, nguoi app.NguoiThucHien) (domain.CanBoTomTat, error)
		Sua(ctx context.Context, id string, yc app.YeuCauSuaCanBo, nguoi app.NguoiThucHien) (domain.CanBoTomTat, error)
		DatKhoa(ctx context.Context, id string, khoa bool, nguoi app.NguoiThucHien) (domain.CanBoTomTat, error)
		DoiVaiTro(ctx context.Context, id, vaiTroID string, nguoi app.NguoiThucHien) (domain.CanBoTomTat, error)
	}

	// QuyenDoc lists the caller's OWN permission keys, for GET /api/v1/sessions/current.
	//
	// SEPARATE FROM authz.Checker ON PURPOSE, although *idstore.Checker satisfies both and the
	// same value is wired into each. Checker DECIDES access, one key at a time, and every route in
	// all eight services depends on it; this one only DESCRIBES what the caller already holds, so
	// the web can avoid drawing what the server would refuse. Widening authz.Checker with a list
	// method would put a shape whose only job is drawing menus onto the interface that guards
	// every route — and the first caller to decide access from a list is the accident that costs.
	QuyenDoc interface {
		QuyenCua(ctx context.Context, p authz.Principal) ([]authz.Perm, error)
	}

	// VaiTroDoc reads the CALLER'S OWN role, for GET /api/v1/sessions/current.
	//
	// READ IN THE HANDLER, NOT AT THE EDGE, AND THIS IS THE DECISION MOST LIKELY TO BE UNDONE BY
	// SOMEBODY BEING HELPFUL. XacThuc already reads the account at step 5 and copies HoTen and
	// ChucVu onto PhienHienTai, with a comment saying it does so "so the route costs no second
	// query on a call that happens on every page load". Adding the role there looks like the same
	// move and is not: those two fields are already on the row the middleware HAS to read, while
	// the role is a second table and therefore a JOIN. The middleware runs on EVERY request of
	// EVERY route in this service; the role is wanted by ONE route. Paying a join on every request
	// of every screen to save one query on one route is the wrong side of that trade by three
	// orders of magnitude.
	//
	// The bool is the second outcome and is not optional to handle — see VaiTroCuaCanBo.
	VaiTroDoc interface {
		VaiTroCuaCanBo(ctx context.Context, canBoID string) (domain.VaiTro, bool, error)
	}

	// BoPhanDanhMuc is the commune's org chart, for GET /api/v1/org-units.
	//
	// NO page.Request PARAMETER, unlike CanBoDanhBa: this route returns the whole list on purpose.
	// The reason is on idstore.BoPhanStore.DanhSach, and the bound that replaces the missing `limit`
	// is idstore.TranDanhMucBoPhan.
	BoPhanDanhMuc interface {
		DanhSach(ctx context.Context) ([]domain.BoPhan, error)
	}

	// VaiTroDanhMuc is the commune's role catalogue, for GET /api/v1/roles.
	//
	// SEPARATE FROM VaiTroDoc ON PURPOSE, even though one store implements both. That one answers
	// "what is the CALLER's role" for GET /api/v1/sessions/current; this one answers "what roles
	// does this commune have". Merging them would hand a handler the whole catalogue where it only
	// ever needed the caller's own row — and a dependency wider than the work requires is how the
	// next person justifies using it.
	VaiTroDanhMuc interface {
		DanhSach(ctx context.Context) ([]domain.VaiTro, error)
	}

	// MaTranQuyenDoc is the Phân quyền matrix, for GET /api/v1/role-permissions.
	//
	// SEPARATE FROM authz.Checker AND FROM QuyenDoc, although all three read grants and two of them
	// are satisfied by the same *idstore.Checker. The three answer three different questions, for
	// three different callers, and merging any pair of them moves a decision onto a description:
	//
	//	authz.Checker   "may THIS caller do THIS one thing"  — decides access, on every request
	//	QuyenDoc        "what may THIS caller do"            — draws the caller's own menu
	//	MaTranQuyenDoc  "what may EVERY ROLE do"             — one administration screen
	//
	// This one returns the grants of OTHER roles, which is precisely why it is the only one of the
	// three behind a permission (`admin.role`). Widening authz.Checker to carry it would put the
	// whole commune's grant table behind the interface that guards every route in all eight
	// services — and the first caller to decide access from it is the accident that costs.
	//
	// ONE METHOD RETURNING ALL THREE LISTS, not three methods: the matrix is only correct if rows,
	// columns and cells were read from the same instant. See maTranQuyenRa.
	MaTranQuyenDoc interface {
		MaTran(ctx context.Context) (domain.MaTranQuyen, error)
	}

	// The three reference reads of migration 0005 (ADR 0024). THREE INTERFACES AND NOT ONE WIDE
	// READER, although all three have the identical method signature and two of them are satisfied
	// by stores built the same way.
	//
	// The discipline is the one BoPhanDanhMuc and VaiTroDanhMuc already follow one screen up, and
	// the reason is the same: an interface is the list of things a handler CAN do. A single
	// `DanhMucDoc` carrying three methods would hand the residential-unit handler the task-bloc
	// catalogue it has no business reading, and the day somebody reaches for it — to "enrich" a
	// response, to fill a picker on the wrong screen — nothing turns red and the two catalogues are
	// coupled through a handler. Three names also mean the panic in Register can say WHICH route
	// would have failed, which is the whole value of refusing incomplete wiring at construction.

	// ThonToDanPhoDanhSach is the commune's residential units, for GET /api/v1/residential-units.
	//
	// NO page.Request PARAMETER, like the two catalogues below and unlike CanBoDanhBa: this route
	// returns the whole list on purpose. The reason is on idstore.ThonToDanPhoStore.DanhSach, and
	// the bound that replaces the missing `limit` is idstore.TranDanhSachThonToDanPho.
	ThonToDanPhoDanhSach interface {
		DanhSach(ctx context.Context) ([]domain.ThonToDanPho, error)
	}

	// LoaiDonViDanCuDanhMuc is the residential-unit-type catalogue, for
	// GET /api/v1/residential-unit-types.
	//
	// SEPARATE FROM ThonToDanPhoDanhSach even though the list route above already carries each
	// unit's type LABEL. Those answer two different questions: that one says what the units the
	// commune HAS are classified as, this one says what classifications EXIST — including the ones
	// no unit uses yet, which is exactly what a picker must offer and what a list can never reveal.
	LoaiDonViDanCuDanhMuc interface {
		DanhSach(ctx context.Context) ([]domain.LoaiDonViDanCu, error)
	}

	// KhoiNhiemVuDanhMuc is the task-bloc catalogue, for GET /api/v1/task-blocs.
	//
	// IT IS READ BY THIS SERVICE AND CONSUMED BY `petitions`, which holds the chosen code as a
	// VALUE and never joins into this schema (rule 2, invariant 3; ADR 0024). The table lives here
	// because the list changes with the org chart, not with tasks — domain.KhoiNhiemVu.
	KhoiNhiemVuDanhMuc interface {
		DanhSach(ctx context.Context) ([]domain.KhoiNhiemVu, error)
	}

	// The commune's working calendar (migration 0006). THREE INTERFACES, THREE TABLES, and the
	// same discipline as the three reference reads above: an interface is the list of things a
	// handler CAN do, so the holiday handler must not be able to reach the weekly calendar.
	//
	// The three nouns — `working-hours`, `public-holidays`, `swap-working-days` — were ASKED and
	// answered by the user on 2026-09-20 rather than translated on the spot (ADR 0011). What
	// decided them is the `@entity` marks already in migration 0006; the full argument, including
	// the `closure-days` objection that was raised and rejected, is on the routes at the bottom of
	// Register. Do not reopen it here.
	//
	// NO WRITE PATH ON ANY OF THE THREE: who may edit a commune's working calendar has not been
	// asked — it is the sibling of open question #21 — and a half-written write path looks like a
	// decision somebody made.

	// LichLamViecDoc is the commune's ordinary working week.
	//
	// NO page.Request PARAMETER: a calendar is only correct WHOLE, so the bound is a ceiling
	// (idstore.TranLichLamViec) rather than a `limit`.
	LichLamViecDoc interface {
		DanhSach(ctx context.Context) ([]domain.CaLamViec, error)
	}

	// NgayNghiLeDoc is the dates the commune does not work, READ ONE YEAR AT A TIME.
	//
	// THE YEAR IS A PARAMETER WHERE THE CATALOGUES HAVE NONE, and it is the one difference that
	// matters: this table grows with TIME rather than with how the commune is organised, so "all
	// of it" is a list with no upper bound. The window is the unit the screen shows and the unit
	// the annual announcement arrives in — see idstore.TranNgayNghiLeMotNam.
	NgayNghiLeDoc interface {
		TheoNam(ctx context.Context, nam int) ([]domain.NgayNghiLe, error)
	}

	// NgayLamBuDoc is the dates the commune DOES work although the week says otherwise.
	//
	// SEPARATE FROM NgayNghiLeDoc although both are read by year and both answer "what is special
	// about this date". They are two tables with two shapes — a holiday is a date, a swap day is a
	// date plus the hours worked — and a date appearing in both is a configuration error both
	// stores refuse rather than resolve (domain.LoiNgayVuaNghiVuaLamBu).
	NgayLamBuDoc interface {
		TheoNam(ctx context.Context, nam int) ([]domain.CaLamBu, error)
	}

	// DangNhapUC and DangXuatUC are the use cases. The handlers only translate HTTP; the
	// business write and its audit entry share one transaction inside these.
	DangNhapUC interface {
		Chay(ctx context.Context, yc app.YeuCauDangNhap) (app.KetQuaDangNhap, error)
	}

	DangXuatUC interface {
		Chay(ctx context.Context, sid, maCanBo, ip string) error
	}
)

// Deps are everything the routes need. Kept explicit so wiring stays in cmd/server.
type Deps struct {
	Checker   authz.Checker
	Quyen     QuyenDoc
	VaiTro    VaiTroDoc
	BoPhan    BoPhanDanhMuc
	VaiTroMuc VaiTroDanhMuc
	MaTran    MaTranQuyenDoc
	// The three reference reads of migration 0005. Three fields, three interfaces — see the note
	// above them.
	ThonToDanPho   ThonToDanPhoDanhSach
	LoaiDonViDanCu LoaiDonViDanCuDanhMuc
	KhoiNhiemVu    KhoiNhiemVuDanhMuc
	// The commune's working calendar (migration 0006) — GET /api/v1/working-hours,
	// /api/v1/public-holidays, /api/v1/swap-working-days. Read only; no write route exists.
	LichLamViec LichLamViecDoc
	NgayNghiLe  NgayNghiLeDoc
	NgayLamBu   NgayLamBuDoc
	Signer      *token.Signer
	Phien       PhienDoc
	CanBo       CanBoDoc
	DanhBa      CanBoDanhBa
	GhiDanhBa   CanBoGhiDanhBa
	DangNhap    DangNhapUC
	DangXuat    DangXuatUC

	Log *slog.Logger
}

// Register mounts the identity routes.
//
// Add every new route with its permission declaration in the SAME statement, never on a nearby
// line: rbac_guard anchors to the statement, and so should a reader.
func Register(mux *http.ServeMux, d Deps) {
	// Refusing incomplete wiring HERE, at construction, not at request time: a sign-in route
	// mounted without a signing key or without the session registry would accept requests it
	// cannot honour, and the first person to find out would be a member of staff locked out of
	// a government system. Same discipline as authz.Public("") and idem.KhongCan("").
	switch {
	case d.Signer == nil:
		panic("identity/http: thiếu token.Signer — không ký được phiên")
	case d.Phien == nil || d.CanBo == nil:
		panic("identity/http: thiếu kho phiên hoặc kho cán bộ — không dựng được Principal")
	case d.DanhBa == nil:
		panic("identity/http: thiếu kho danh bạ cán bộ — hai tuyến đọc cán bộ sẽ panic khi có người gọi")
	case d.GhiDanhBa == nil:
		panic("identity/http: thiếu use case ghi danh bạ cán bộ — năm tuyến ghi cán bộ sẽ panic khi có người gọi")
	case d.DangNhap == nil || d.DangXuat == nil:
		panic("identity/http: thiếu use case đăng nhập/đăng xuất")
	case d.Checker == nil:
		panic("identity/http: thiếu authz.Checker — mọi route có quyền sẽ không kiểm được")
	case d.Quyen == nil:
		panic("identity/http: thiếu kho quyền — GET /api/v1/sessions/current sẽ panic khi có người gọi")
	case d.VaiTro == nil:
		panic("identity/http: thiếu kho vai trò — GET /api/v1/sessions/current sẽ panic khi có người gọi")
	case d.BoPhan == nil:
		panic("identity/http: thiếu kho bộ phận — GET /api/v1/org-units sẽ panic khi có người gọi")
	case d.VaiTroMuc == nil:
		panic("identity/http: thiếu kho danh mục vai trò — GET /api/v1/roles sẽ panic khi có người gọi")
	case d.MaTran == nil:
		panic("identity/http: thiếu kho ma trận phân quyền — GET /api/v1/role-permissions sẽ panic khi có người gọi")
	case d.ThonToDanPho == nil:
		panic("identity/http: thiếu kho thôn/tổ dân phố — GET /api/v1/residential-units sẽ panic khi có người gọi")
	case d.LoaiDonViDanCu == nil:
		panic("identity/http: thiếu kho loại đơn vị dân cư — GET /api/v1/residential-unit-types sẽ panic khi có người gọi")
	case d.KhoiNhiemVu == nil:
		panic("identity/http: thiếu kho khối nhiệm vụ — GET /api/v1/task-blocs sẽ panic khi có người gọi")
	case d.LichLamViec == nil:
		panic("identity/http: thiếu kho lịch làm việc — GET /api/v1/working-hours sẽ panic khi có người gọi")
	case d.NgayNghiLe == nil:
		panic("identity/http: thiếu kho ngày nghỉ lễ — GET /api/v1/public-holidays sẽ panic khi có người gọi")
	case d.NgayLamBu == nil:
		panic("identity/http: thiếu kho ngày làm bù — GET /api/v1/swap-working-days sẽ panic khi có người gọi")
	}

	h := NewHandler(d)

	// Opening a session. Public because there is no session yet to check a permission against —
	// this IS the screen that creates one.
	//
	// KhongCan and not Required: signing in twice opens a second session, which is revocable,
	// leaves no archival record and consumes no business code. There is nothing here that rule 7
	// would make permanent. Password guessing is a different problem with a different answer
	// (rate limiting), and pretending an idempotency key addresses it would be the more
	// dangerous mistake.
	//
	// @summary  Đăng nhập bằng email và mật khẩu, mở một phiên làm việc
	// @screen   15-phu-luc-giao-dien-chung §1
	// @request  thanDangNhap
	// @reply    201 phanHoiDangNhap
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/sessions",
		authz.Public("màn hình đăng nhập — chưa có phiên nên chưa có gì để kiểm quyền")(
			idem.KhongCan("đăng nhập lần hai mở một phiên thứ hai, thu hồi được, không sinh hồ sơ lưu trữ; chống dò mật khẩu là việc của rate limit")(
				http.HandlerFunc(h.DangNhap))))

	// Ending a session. AnyAuthenticated and not a permission: every account must be able to
	// sign itself out, and requiring a permission for it would leave an account whose role was
	// stripped unable to close its own session. The handler enforces the real limit — the sid
	// must be this request's own.
	//
	// 404 and not 403 on somebody else's sid is deliberate and is part of the contract: the
	// existence of another person's session is itself information (rule 4, forbidden #2).
	//
	// @summary  Kết thúc phiên làm việc của chính mình
	// @screen   15-phu-luc-giao-dien-chung §3.3
	// @reply    204 -
	// @reply    401 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/sessions/{sid}",
		authz.AnyAuthenticated("mọi tài khoản đã đăng nhập đều được kết thúc phiên của chính mình")(
			idem.KhongCan("thu hồi một phiên đã thu hồi cho cùng một kết quả")(
				http.HandlerFunc(h.DangXuat))))

	// Reading one's own session. AnyAuthenticated and not a permission, for a reason close to the
	// one above: this is the route that answers "who am I and what may I do", and an account that
	// has just had every role withdrawn holds no permission with which to ask. Requiring one would
	// mean the person whose rights changed is exactly the person who can no longer load the
	// screen that would tell them.
	//
	// The literal `current` cannot shadow the wildcard route above: that one is DELETE, this one
	// is GET, and net/http matches method first. It could only collide if a session id were
	// literally "current", and a sid is a random value the server generates, never a string a
	// client chooses (idstore.PhienStore.Tao).
	//
	// NO idem.* DECLARATION: a GET changes no state, and declaring a duplicate-request protection
	// here would claim a protection with nothing to protect.
	//
	// @summary  Phiên làm việc hiện tại của chính người gọi, kèm vai trò và danh sách quyền để ẩn/hiện menu
	// @screen   15-phu-luc-giao-dien-chung §1
	// @reply    200 phienHienTaiRa
	// @reply    401 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/sessions/current",
		authz.AnyAuthenticated("mọi tài khoản đã đăng nhập đều được hỏi 'tôi là ai' — kể cả tài khoản vừa bị gỡ hết vai trò, vốn không còn quyền nào để đòi")(
			http.HandlerFunc(h.XemPhienHienTai)))

	// --- the commune behind this Host ---------------------------------------------------------
	//
	// WHICH SERVICE THIS ROUTE BELONGS TO IS NOT SETTLED, AND THIS COMMENT IS NOT A DECISION.
	// It is built here because that is what was asked for this turn; the question is rule 2, stop
	// conditions #1 and #2 and is going to the customer. The two readings, stated so the next
	// reader does not have to reconstruct them:
	//
	//	platform owns it   the commune registry — domains, display name, lifecycle — is the
	//	                   platform service's data (kb/30-indexes/services.json). Identity reads it
	//	                   over gRPC and owns none of it. platform/internal/http/
	//	                   routes.go:42 already sketches a GET /api/v1/communes there — the
	//	                   vendor's list of all communes, not this route, but it is the same data
	//	                   and it is held up on an undecided vendor-side permission model. And a
	//	                   commune's name is needed by EVERY screen, not only the sign-in one.
	//	identity owns it   identity is the service the browser already talks to before a session
	//	                   exists, it resolves the commune at its own edge on every request anyway,
	//	                   and one more public surface on the platform service is one more thing to
	//	                   expose to the internet.
	//
	// PUBLIC, AND THE REASON IS THE SCREEN ITSELF: the sign-in form prints the name of the
	// authority a person is about to sign in to, and at that moment there is no session, therefore
	// no principal, therefore nothing to check a permission against. Same reading as POST
	// /api/v1/sessions above. Nothing in the reply is personal data, nothing counts staff, nothing
	// describes the commune's internal operation — see thongTinXa, which is where the absent
	// fields are argued.
	//
	// `/communes/current` — PLURAL SEGMENT, SINGULAR SUB-RESOURCE, and this replaces an earlier
	// `/api/v1/commune` that argued itself an exception to the plural rule.
	//
	// The old argument was that a caller can never see more than one, so a plural segment would
	// promise a collection this route must never have. That reasoning ignored what the plural
	// segment actually names: the RESOURCE TYPE, not the size of one answer. `skills/
	// rest-api-design` REQUIRED #1 has no exception, and an endpoint granting itself one is the
	// shape rule 9 calls drift.
	//
	// WHAT MADE IT WORTH CHANGING rather than leaving as a style point: the citizen Mini App is
	// getting `GET /api/v1/communes` — the list of communes a citizen picks from when they belong
	// to none yet. Two paths one letter `s` apart, answering two different questions, serving two
	// classes of user with very different trust levels (rule 4). Neither leaks — `thongTinXa`
	// carries no id and the platform summary carries no host — so this is a confusion risk, not a
	// leak. It is also the kind nobody untangles six months later.
	//
	// `/current` is not invented here: `GET /api/v1/sessions/current` above is the same shape,
	// answering the same kind of question — "which one is THIS request's", derived from the
	// request itself and never from a parameter.
	//
	// NO idem.* DECLARATION: GET, changes no state.
	//
	// @summary  Thông tin xã ứng với tên miền đang gọi, cho màn hình đăng nhập
	// @screen   15-phu-luc-giao-dien-chung §1
	// MỘT mã trả lời duy nhất, và đó là hệ quả trực tiếp của việc biên mang cả tenant.Tenant:
	// handler không còn phân giải gì nữa nên không còn thất bại nào để khai. Tên miền không
	// thuộc xã nào thì bị biên từ chối bằng 404 TRƯỚC khi tới đây, nên 404 không phải kết quả
	// của tuyến này mà của chuỗi trước nó.
	//
	// @reply    200 thongTinXa
	mux.Handle("GET /api/v1/communes/current",
		authz.Public("màn hình đăng nhập phải hiện tên xã TRƯỚC khi có phiên nào để kiểm quyền")(
			http.HandlerFunc(h.ThongTinXa)))

	// --- the staff register. TWO READ ROUTES, AND DELIBERATELY NO WRITE ROUTE -----------------
	//
	// `staff` is the resource noun, and it is the SAME word as `message Staff` in the proto. One
	// concept was already carrying four names — nguoi_dung (table), can_bo (Go), CanBo (type),
	// Staff (contract); picking `staff` adds no fifth. See kb/00-foundation/ubiquitous-language.md.
	//
	// admin.user — "Quản lý người dùng" — is the permission the Cấu hình → Người dùng tab is
	// declared with (docs/ui-ux/14-cau-hinh.md §12.8). No route here is AnyAuthenticated: a
	// commune's staff register is not something every role has business reading.
	//
	// NO idem.* DECLARATION: these are GET routes and change no state. rest_api_guard only asks
	// for one on POST/PUT/PATCH/DELETE, and declaring one here would claim a protection that has
	// nothing to protect.
	//
	// 401 AND 403 REALLY ARE httpx.Error. They come from authz.RequirePermission, which used to
	// answer with http.Error — a plain-text body while the contract declared JSON. That gap was
	// closed in core/authz; this note stays so nobody "fixes" it a second time.

	// @summary  Danh sách cán bộ của xã — gồm cả người có tài khoản đăng nhập và người chỉ có trong danh bạ
	// @screen   14-cau-hinh §3
	// @page     idstore.SapXepCanBo
	// @reply    200 page.Result[canBoTomTat]
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/staff",
		authz.RequirePermission(d.Checker, "admin.user")(
			http.HandlerFunc(h.DanhSachCanBo)))

	// 404 and not 403 for an id belonging to another commune: the existence of another
	// authority's record is itself information (rule 4, forbidden #2). Same reading as the sid on
	// DELETE /api/v1/sessions/{sid} above.
	//
	// @summary  Chi tiết một cán bộ trong xã
	// @screen   14-cau-hinh §3
	// @reply    200 canBoTomTat
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/staff/{id}",
		authz.RequirePermission(d.Checker, "admin.user")(
			http.HandlerFunc(h.ChiTietCanBo)))

	// --- the staff register: the WRITE routes ---------------------------------------------------
	//
	// ALL FIVE DECLARE `admin.user`, AND THAT WAS CHECKED AGAINST THE `quyen` TABLE RATHER THAN
	// ASSUMED. It is the key the Cấu hình → Người dùng tab is specified with (14-cau-hinh.md
	// §12.8), it is already seeded (migration 0001:278, "Quản lý người dùng"), and it is what the
	// two read routes above declare — so a commune that granted somebody the staff screen granted
	// them the screen, not half of it.
	//
	// WHY NOT `content.update`, which the OTHER screen specifies (12-danh-ba-can-bo.md §9.4): that
	// key is "Sửa nội dung và danh bạ Mini App" and it governs what the CITIZEN-FACING directory
	// shows — the publication flag and the ordering, i.e. open question #12, which needs a consent
	// column that does not exist. None of the five routes here touches that surface.
	//
	// THE ONE ROUTE THAT IS NOT HERE is the soft delete of #10. It carries a permission of its own
	// by the customer's decision, no key in the table means it, and rule 5 invariant 3c forbids
	// inventing one — the full argument is in the header of can_bo_ghi.go. Declaring it with
	// `admin.user` would collapse two operations the customer deliberately separated.

	// Adding somebody to the register. POST /api/v1/staff
	//
	// idem.Required(idem.DongKhiHong), AND WHICH LAYER IS ACTUALLY PROTECTING THIS — the question
	// skills/rest-api-design §4 says to answer at the route. THE ANSWER IS: ONLY THIS ONE. There is
	// no natural unique key underneath, and that is not an oversight of the schema: the staff code
	// is minted from crypto/rand precisely so that it owes nothing to the table (#15), and a
	// person's name, position and department are not a key — two people in one commune may share
	// all three. So a double-submitted form produces TWO rows with TWO permanent codes.
	//
	// DongKhiHong AND NOT MoKhiHong, which is the stricter of the two and needs its reason stated:
	// rule 7 forbids hard delete, so the duplicate row is permanent, and the operation that would
	// retire it — #10's soft delete — CANNOT BE BUILT YET for want of a permission key. A duplicate
	// created during a Redis outage would therefore sit in the commune's directory with no route
	// able to remove it. Refusing to add a member of staff for the minutes a cache is down is the
	// cheaper failure by a wide margin.
	//
	// @summary  Thêm một cán bộ vào danh bạ của xã — mã cán bộ do hệ thống sinh, không có ô nhập
	// @screen   12-danh-ba-can-bo §5
	// @request  themCanBoVao
	// @reply    201 canBoTomTat
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/staff",
		authz.RequirePermission(d.Checker, "admin.user")(
			idem.Required(idem.DongKhiHong)(
				http.HandlerFunc(h.ThemCanBo))))

	// Correcting a profile. PATCH /api/v1/staff/{id}
	//
	// PATCH AND NOT PUT: four of the six editable fields have a meaningful empty value — clearing a
	// position, a department or either telephone number is a legitimate edit — so a full
	// replacement cannot tell "not mentioned" from "cleared". See suaCanBoVao.
	//
	// THIS ROUTE CHANGES NO AUTHORITY, which is why it carries none of the three refusals #13 and
	// #14 produce. The UPDATE statement names six columns and `vai_tro_id`, `dang_hoat_dong`,
	// `co_tai_khoan` and `ma` are not among them, so there is no request body that could reach
	// them.
	//
	// idem.KhongCan, AND THE REASON IS A PROPERTY OF THE USE CASE RATHER THAN A HOPE: app.Sua
	// compares the row it read against the row it would write and, when nothing moved, writes
	// NOTHING — no UPDATE and no audit entry. The same request sent twice leaves one row in one
	// state and one entry in the ledger. Were that comparison removed, this declaration would
	// become a lie and the second request would file an entry saying nothing changed.
	//
	// @summary  Sửa hồ sơ một cán bộ — họ tên, chức vụ, thư điện tử, bộ phận, hai số điện thoại
	// @screen   12-danh-ba-can-bo §5
	// @request  suaCanBoVao
	// @reply    200 canBoTomTat
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/staff/{id}",
		authz.RequirePermission(d.Checker, "admin.user")(
			idem.KhongCan("sửa là ghi đè một trạng thái đã biết; app.Sua không ghi gì khi không có trường nào đổi, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.SuaCanBo))))

	// Retiring somebody. POST /api/v1/staff/{id}/lockout
	//
	// ⚠ `lockout` IS THE ONE NAME ON THESE FIVE ROUTES THAT THE USER HAS NOT CONFIRMED, and ADR
	// 0011 says a concept with no row in kb/00-foundation/ubiquitous-language.md is a concept to
	// ASK about rather than translate. It is written here so the work is testable; it is reported
	// as an open naming decision, and it is still free to change because no commune is live.
	//
	// WHY NOT `lock`, WHICH WAS WRITTEN FIRST: `.claude/hooks/rest_api_guard.py` lists `lock` among
	// the VERB segments, and the rule it enforces is REQUIRED #8 — an action with legal consequence
	// must be a record you can GET back, not a command. The skill's own prose mentions "lock on a
	// disbursement" as a transition, which is what made `lock` look settled; the hook is the
	// machine-decidable half and it disagrees. Renaming rather than arguing with the guard.
	//
	// WHY NOT `suspension`, WHICH IS THE OBVIOUS NOMINALISATION: in Vietnamese administrative
	// language `đình chỉ` is a DISCIPLINARY measure with legal meaning. #10's lock is for somebody
	// who retired or transferred. An English word asserting a distinction the data does not have is
	// the exact failure `org-units` exists to illustrate.
	//
	// `lockout` IS A NOUN — the state the account is in — so POST creates it and DELETE lifts it,
	// and a future `GET .../lockout` returning who shut the account and when fits without a new
	// path.
	//
	// THIS IS OPEN QUESTION #10's ANSWER AND IT IS NOT A DELETE. A member of staff who retires or
	// transfers is LOCKED and STAYS IN THE DIRECTORY, because their name is what makes years of
	// administrative records readable — BatchGetStaff resolving a name is the whole reason #10
	// attached a mandatory requirement to its own decision. Deleting them is the other situation
	// (a duplicated row), and that route is absent for want of its own permission key.
	//
	// #13 IS ENFORCED HERE, INSIDE THE TRANSACTION, NOT ON THE SCREEN: locking the last account
	// that can administer this commune answers 409 and writes nothing. A warning would not do —
	// the customer's word is "CHẶN CỨNG" — and ADR 0003 leaves the vendor no way back in.
	//
	// idem.KhongCan — locking an account that is already locked writes nothing and audits nothing,
	// so the second request leaves exactly one row and one entry.
	//
	// @summary  Khoá tài khoản một cán bộ đã nghỉ hưu hoặc chuyển công tác — người này vẫn còn trong danh bạ
	// @screen   14-cau-hinh §3
	// @reply    200 canBoTomTat
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/staff/{id}/lockout",
		authz.RequirePermission(d.Checker, "admin.user")(
			idem.KhongCan("khoá một tài khoản đã khoá thì use case không ghi gì — không UPDATE, không vết — nên lần gửi thứ hai cho cùng một trạng thái")(
				http.HandlerFunc(h.KhoaCanBo))))

	// They are back. DELETE /api/v1/staff/{id}/lockout
	//
	// DELETE ON THE SUB-RESOURCE AND NOT A SECOND VERB: removing the lockout is the exact inverse
	// of creating it, and two paths spelled `.../lock` and `.../unlock` would be two verbs for one
	// thing. It does NOT delete the person — rule 7 — and nothing on this path can: the handler
	// writes one boolean.
	//
	// NO #13 CHECK ON THIS DIRECTION, deliberately: unlocking can only ever make the set of
	// administrators BIGGER, and a rule that fires on an operation which cannot cause the harm is a
	// rule people learn to route around.
	//
	// @summary  Mở khoá tài khoản một cán bộ
	// @screen   14-cau-hinh §3
	// @reply    200 canBoTomTat
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/staff/{id}/lockout",
		authz.RequirePermission(d.Checker, "admin.user")(
			idem.KhongCan("mở khoá một tài khoản đang mở thì use case không ghi gì, nên lần gửi thứ hai cho cùng một trạng thái")(
				http.HandlerFunc(h.MoKhoaCanBo))))

	// Moving somebody to a role. PUT /api/v1/staff/{id}/role
	//
	// PUT ON A SINGULAR SUB-RESOURCE, the same shape rest-api-design §3 gives
	// `PUT /api/v1/tasks/{id}/assignment`: a person holds at most one role, so the body carries the
	// WHOLE state of that relationship and `""` means "no role" — a legitimate destination, since
	// `nguoi_dung.vai_tro_id` is nullable and somebody can sit in the org chart holding nothing.
	// `role` singular and not `roles`: this is the one relationship, not a collection.
	//
	// THIS IS WHERE #13 AND #14 BOTH LAND, and it is why the role is not a field on the PATCH
	// above:
	//
	//	#14 first    the caller may not move THEMSELVES. Without it, `admin.user` silently contains
	//	             every other permission — its holder can put themselves in the strongest role.
	//	#14 second   the caller may not grant a key they do not hold. Without it the first
	//	             constraint is a formality: promote a colleague, then ask them to promote you.
	//	             THE CONSEQUENCE, STATED BECAUSE A COMMUNE WILL MEET IT: an administrator who
	//	             holds only `admin.user` cannot assign a role carrying `budget.confirm`. That is
	//	             what the customer chose — the alternative is a permission table that describes
	//	             a flat model while one key stands above all of them (rule 5, forbidden #2).
	//	#13          moving the last administrator to a role without `admin.user` answers 409.
	//
	// idem.KhongCan — assigning the role somebody already holds writes nothing and audits nothing.
	// PUT carries an absolute state rather than a step, so the second request cannot compound the
	// first.
	//
	// @summary  Đổi vai trò của một cán bộ — chuỗi rỗng nghĩa là gỡ vai trò
	// @screen   14-cau-hinh §3
	// @request  datVaiTroVao
	// @reply    200 canBoTomTat
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PUT /api/v1/staff/{id}/role",
		authz.RequirePermission(d.Checker, "admin.user")(
			idem.KhongCan("PUT mang trạng thái tuyệt đối: gán đúng vai trò đang có thì use case không ghi gì, nên lần gửi thứ hai cho cùng một kết quả")(
				http.HandlerFunc(h.DoiVaiTroCanBo))))

	// --- the commune's organisational chart ----------------------------------------------------
	//
	// `org-units` AND NOT `departments`, AND THE NAME WAS LOOKED UP RATHER THAN TRANSLATED.
	// kb/00-foundation/ubiquitous-language.md owns the URL-resource mapping (ADR 0011) and settles
	// this row: the tree holds Đảng uỷ, HĐND and UBMTTQ as well as the UBND's own units, so
	// `departments` would assert that every node is a department of the People's Committee. Most of
	// them are not. A path cannot be taken back once a commune is live, which is why the table
	// exists and why nothing here translates on the spot.
	//
	// AnyAuthenticated, AND THE REASON IS THE SHAPE OF THE DATA'S USE. Unit names appear on nearly
	// every screen — the assignment box on a task, document routing, the staff directory, the
	// filter dropdowns — so requiring a configuration permission would not protect anything, it
	// would break every one of those screens for everybody who is not an administrator. The
	// alternative that actually protects something does not exist here: there is nothing sensitive
	// in a list of the authority's own units.
	//
	// THE TRADE-OFF, STATED RATHER THAN GLOSSED: a commune's org chart is readable by every signed-in
	// account OF THAT COMMUNE. It is not readable across communes and cannot be — Scoped.Query binds
	// `tenant_id` from the context (rule 1, invariant 5), so the same request against another
	// commune's domain is refused at the token layer before any query runs. What is accepted is that
	// a member of staff with no configuration rights can see how their own authority is organised,
	// which is information they can also read off the noticeboard in the lobby.
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Danh mục bộ phận của xã — cây tổ chức, dùng cho ô phân công, luồng văn bản và bộ lọc
	// @screen   14-cau-hinh §3
	// 500 covers two different causes and says so honestly: an ordinary store failure, and the
	// commune's org chart exceeding idstore.TranDanhMucBoPhan — which this route REFUSES rather
	// than truncating, because a silently short list is a unit missing from an assignment box.
	//
	// @reply    200 danhSachBoPhanRa
	// @reply    401 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/org-units",
		authz.AnyAuthenticated("tên bộ phận xuất hiện ở ô phân công nhiệm vụ, luồng văn bản, danh bạ và mọi bộ lọc — đòi một quyền cấu hình sẽ làm hỏng những màn hình đó cho mọi tài khoản không phải quản trị; đánh đổi đã chấp nhận: sơ đồ tổ chức lộ cho mọi tài khoản đã đăng nhập CỦA CHÍNH XÃ ĐÓ, không chéo xã vì Scoped buộc tenant_id")(
			http.HandlerFunc(h.DanhSachBoPhan)))

	// The commune's role catalogue. GET /api/v1/roles
	//
	// `roles` — the URL-resource table settles this row (kb/00-foundation/ubiquitous-language.md).
	// It was ASKED rather than translated on the spot: `org-units` one route up is what happens when
	// the obvious English word (`departments`) asserts something false, so the obvious word gets
	// looked up here too even when it turns out to be right. `role` is already the term rule 5
	// invariant 3 uses — `(tenant_id, role, permission)` — so the contract surface and the
	// authorisation model now say the same word for the same thing.
	//
	// AnyAuthenticated, SAME REASON AS org-units: role names fill the picker on the staff form, the
	// column on the staff directory and the header of the Phân quyền matrix. Gating them behind a
	// configuration permission would break the screen for whoever holds `admin.role` but not
	// `admin.user`, and vice versa — three permissions to render one screen.
	//
	// THE TRADE-OFF, STATED: every signed-in account OF THIS COMMUNE can read what the commune's
	// roles are called. Not across communes, and it cannot be — Scoped.Query binds `tenant_id` from
	// the context (rule 1, invariant 5). What is NOT readable here is what each role may DO: the
	// permission grants live in `vai_tro_quyen` and no route exposes them.
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Danh mục vai trò của xã — dùng cho ô chọn vai trò, cột danh bạ và ma trận phân quyền
	// @screen   14-cau-hinh §4
	// @reply    200 danhSachVaiTroRa
	// @reply    401 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/roles",
		authz.AnyAuthenticated("tên vai trò xuất hiện ở ô chọn vai trò trên form cán bộ, cột Vai trò của danh bạ và tiêu đề cột của ma trận phân quyền — đòi một quyền cấu hình sẽ cần ba quyền để dựng một màn hình; đánh đổi đã chấp nhận: tên các vai trò lộ cho mọi tài khoản đã đăng nhập CỦA CHÍNH XÃ ĐÓ, còn quyền của từng vai trò thì không tuyến nào phơi ra")(
			http.HandlerFunc(h.DanhSachVaiTro)))

	// --- the Phân quyền matrix. ONE READ ROUTE, AND DELIBERATELY NO WRITE ROUTE ----------------
	//
	// `role-permissions` — THE RESOURCE IS THE RELATION, and the name says what the data IS rather
	// than what one part of it is called (ADR 0017, the same rule that settled `province`).
	//
	// It was `permissions` first, and that was rejected for two reasons worth keeping, because the
	// obvious word is the one somebody will propose again:
	//
	//	it under-describes    the response is `groups` + `roles` + `grants`. Two thirds of it is
	//	                      about ROLES, and the third that names keys exists only so the matrix
	//	                      has rows. A caller reading `/permissions` expects a list of keys;
	//	                      what arrives is a commune's authorisation model.
	//	it squats             a plain catalogue of permission keys — no commune, no roles — is a
	//	                      route this system may well want, and `permissions` is its name. Taking
	//	                      it here would leave that one with no honest path, and a path cannot be
	//	                      taken back once a commune is live.
	//
	// The specification's own suggestion (§11, `GET /api/cau-hinh/quyen`) is a Vietnamese segment
	// under a screen-shaped prefix, which rest-api-design forbids on both counts (FORBIDDEN #1,
	// REQUIRED #1) — `cau-hinh` names the screen, and screens get rearranged.
	//
	// STATED GAP: kb/00-foundation/ubiquitous-language.md has no row for this concept yet; one is
	// being added by whoever owns that table. This path has no external caller.
	//
	// RequirePermission("admin.role") — "Phân quyền" — AND NOT AnyAuthenticated, which is the
	// opposite of the call made one route up for `roles`, deliberately. `roles` exposes what the
	// commune's roles are CALLED, which fills pickers on half the screens in the system. This route
	// exposes what every role may DO — the commune's authorisation model in one response. That is
	// the map of where authority sits in an authority, and reading it is the reconnaissance step
	// before an escalation: it names which role to get into and which cell to have ticked. Spec
	// §12.8 declares the tab with this key; the route declares the same key, so the tab and the data
	// behind it cannot drift apart.
	//
	// THE MATRIX IS READ, NEVER WRITTEN, BY THIS SERVICE'S HTTP SURFACE. The write side — the
	// spec's `Lưu` button per column (§12.5) — is missing on purpose; open questions #13 and #14 sit
	// underneath it, and the full argument is at the top of internal/http/quyen.go, which is where
	// the next person will be standing when they add it.
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Ma trận phân quyền của xã — nhóm quyền, vai trò kèm số cán bộ, và các ô đã cấp
	// @screen   14-cau-hinh §4
	// 500 covers an ordinary store failure and any of the three ceilings in idstore being reached —
	// which this route REFUSES rather than truncating, because a matrix missing a row reads exactly
	// like a role that does not hold the right.
	//
	// @reply    200 maTranQuyenRa
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/role-permissions",
		authz.RequirePermission(d.Checker, "admin.role")(
			http.HandlerFunc(h.MaTranQuyen)))

	// --- the three reference reads of migration 0005. THREE READ ROUTES, DELIBERATELY NO WRITE ---
	//
	// The resource nouns come from kb/00-foundation/ubiquitous-language.md:159-161, which already
	// fixed the ENTITY names (ADR 0024) — `ResidentialUnit`, `ResidentialUnitType`, `TaskBloc` —
	// and skills/rest-api-design REQUIRED #1, which makes a path segment English, plural and
	// kebab-case. Nothing here was translated on the spot: `org-units` above is what happens when
	// the obvious English word asserts something false, so the mapping gets looked up even when the
	// obvious word turns out to be right.
	//
	// NO WRITE ROUTE ON ANY OF THE THREE, and the reason is one unanswered question rather than an
	// oversight: open question #21 — whether a commune may edit the CODE LIST itself or only the
	// labels and the order — is still open. A half-written write path looks like a decision
	// somebody made. The database already refuses the dangerous half (migration 0005's trigger
	// refuses six operations); what nobody has settled is who may add a row.
	//
	// ALL THREE ARE AnyAuthenticated, SAME CALL AND SAME REASON AS /org-units AND /roles ABOVE.
	// These lists fill pickers and filters on nearly every screen in the system — the `Loại` column
	// on the residential-unit list, the address picker on a petition, the bloc field on a task form
	// in another service. Requiring a configuration permission would not protect anything; it would
	// break those screens for every account that is not an administrator.
	//
	// THE TRADE-OFF, STATED RATHER THAN LEFT IMPLICIT: a commune's hamlets, its residential-unit
	// types and its task blocs are readable by every signed-in account OF THAT COMMUNE. They are
	// NOT readable across communes and cannot be — Scoped binds `tenant_id` from the context
	// (rule 1, invariant 5), so the same request against another commune's domain is refused at the
	// token layer before any query runs. What is accepted is that a member of staff with no
	// configuration rights can read how their own authority divides its territory and its work,
	// which is information printed on the noticeboard in the lobby.
	//
	// NO idem.* DECLARATION ON ANY OF THE THREE: a GET changes no state.
	//
	// NO 403 IN ANY OF THE @reply BLOCKS, AND THAT IS NOT AN OMISSION: an AnyAuthenticated route has
	// no permission to fail, so 403 is a status these handlers never return. A declared status the
	// handler cannot produce is a contract the admin web writes dead code for.

	// @summary  Danh sách thôn / tổ dân phố của xã — kèm nhãn loại đơn vị, dùng cho ô chọn địa bàn và bộ lọc
	// @screen   14-cau-hinh §2
	// 500 covers two different causes and says so honestly: an ordinary store failure, and the
	// commune's list exceeding idstore.TranDanhSachThonToDanPho — which this route REFUSES rather
	// than truncating, because a silently short list is a thôn missing from the address picker.
	//
	// @reply    200 danhSachThonToDanPhoRa
	// @reply    401 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/residential-units",
		authz.AnyAuthenticated("tên thôn/tổ dân phố xuất hiện ở ô chọn địa bàn của phản ánh, hồ sơ hộ và mọi bộ lọc theo địa bàn — đòi một quyền cấu hình sẽ làm hỏng những màn hình đó cho mọi tài khoản không phải quản trị; đánh đổi đã chấp nhận: danh sách địa bàn lộ cho mọi tài khoản đã đăng nhập CỦA CHÍNH XÃ ĐÓ, không chéo xã vì Scoped buộc tenant_id")(
			http.HandlerFunc(h.DanhSachThonToDanPho)))

	// @summary  Danh mục loại đơn vị dân cư của xã — thôn / tổ dân phố, dùng cho ô chọn Loại và bộ lọc
	// @screen   14-cau-hinh §5
	// 500 covers an ordinary store failure and the catalogue exceeding
	// idstore.TranDanhMucLoaiDonViDanCu — REFUSED rather than truncated, because a missing
	// classification is a unit filed under the wrong one.
	//
	// @reply    200 danhSachLoaiDonViDanCuRa
	// @reply    401 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/residential-unit-types",
		authz.AnyAuthenticated("nhãn loại đơn vị dân cư xuất hiện ở cột Loại của danh sách thôn/tổ dân phố, ở ô chọn khi thêm địa bàn và ở bộ lọc — đòi một quyền cấu hình sẽ làm hỏng những màn hình đó cho mọi tài khoản không phải quản trị; đánh đổi đã chấp nhận: danh mục lộ cho mọi tài khoản đã đăng nhập CỦA CHÍNH XÃ ĐÓ, không chéo xã vì Scoped buộc tenant_id")(
			http.HandlerFunc(h.DanhSachLoaiDonViDanCu)))

	// @summary  Danh mục khối nhiệm vụ của xã — Khối Uỷ ban / Khối Đảng / Khác, dùng cho ô chọn khối và bộ lọc nhiệm vụ
	// @screen   14-cau-hinh §5
	// 500 covers an ordinary store failure and the catalogue exceeding idstore.TranDanhMucKhoiNhiemVu
	// — REFUSED rather than truncated, because a missing bloc files a task under the wrong arm of the
	// apparatus and the count reported upward is then false.
	//
	// @reply    200 danhSachKhoiNhiemVuRa
	// @reply    401 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/task-blocs",
		authz.AnyAuthenticated("nhãn khối nhiệm vụ xuất hiện ở ô chọn khối trên biểu mẫu nhiệm vụ, ở nhãn dòng và ở bộ lọc danh sách nhiệm vụ — đòi một quyền cấu hình sẽ làm hỏng những màn hình đó cho mọi tài khoản không phải quản trị; đánh đổi đã chấp nhận: danh mục lộ cho mọi tài khoản đã đăng nhập CỦA CHÍNH XÃ ĐÓ, không chéo xã vì Scoped buộc tenant_id")(
			http.HandlerFunc(h.DanhSachKhoiNhiemVu)))

	// --- the commune's working calendar. THREE READ ROUTES, DELIBERATELY NO WRITE ROUTE --------
	//
	// THE THREE NOUNS ARE SETTLED (user, 2026-09-20) AND ARE NOT REOPENED HERE: `working-hours`,
	// `public-holidays`, `swap-working-days`. That includes the objection raised against
	// `public-holidays` — it was put to the user and answered.
	//
	// WHY EACH NAME, AND WHAT WAS REJECTED → kb/00-foundation/ubiquitous-language.md
	// §"Lịch làm việc của xã". That file owns the URL-resource mapping (ADR 0011) and the argument
	// behind every row of it. NOTHING OF IT IS SUMMARISED HERE ON PURPOSE: a summary is a third
	// copy of one fact, copies drift, and once two disagree a reader cannot tell which is current —
	// so both lose their authority, including the one that is right (rule 9, invariant 2).
	//
	// THE LOCAL CONSEQUENCE, which is the only part that belongs in this file: these three path
	// strings are compared against that table and against the generated contract. A one-character
	// divergence is fixed HERE, never in the table — the generated tier is the truth for this
	// surface (ADR 0014).
	//
	// AN OVERLAPPING OR EMPTY CALENDAR IS SURFACED ON THE READ, NOT REFUSED — user's decision,
	// 2026-09-20, and it is the one that looks backwards until the reason is stated: THE
	// CONFIGURATION SCREEN THAT FIXES AN OVERLAP READS THROUGH THIS SAME ROUTE, so refusing the
	// read locks away the very thing needed to repair it. Both routes therefore answer 200 with a
	// derived `problems` list beside the whole `items` list. The REFUSAL belongs where a DEADLINE
	// is computed, not where a calendar is listed — see domain.VanDeCuaLich, whose non-empty
	// result is what a computing caller must refuse on (rule 10; migration 0006:56 and :109).
	//
	// THE ONE THING THAT IS STILL REFUSED IS A CALENDAR THAT CONTRADICTS ITSELF: a date recorded
	// both as a holiday and as a swap working day. That is not a defect in one list a screen can
	// show and fix — it is two lists asserting opposite things, and picking a winner would make
	// one of two VISIBLE configuration rows do nothing with nothing on screen to say which
	// (migration 0006:254). Both date routes answer 409 and name the dates.
	//
	// ALL THREE ARE AnyAuthenticated, SAME CALL AND SAME REASON AS /org-units, /roles AND THE
	// THREE READS ABOVE, which the user settled for catalogue reads on 2026-09-20: office hours
	// and public holidays are shown on nearly every screen that states a deadline — the due date
	// on a task, the "còn mấy ngày" chip on a petition, any form that offers a date — so requiring
	// a configuration permission would not protect anything, it would break those screens for
	// everybody who is not an administrator.
	//
	// THE TRADE-OFF, STATED RATHER THAN GLOSSED: a commune's working hours, its holidays and its
	// swap days are readable by every signed-in account OF THAT COMMUNE. They are NOT readable
	// across communes and cannot be — Scoped binds `tenant_id` from the context (rule 1,
	// invariant 5), so the same request against another commune's domain is refused at the token
	// layer before any query runs. What is accepted is that a member of staff with no
	// configuration rights can read when their own authority is open, which is information printed
	// on the door.
	//
	// NO WRITE ROUTE ON ANY OF THE THREE. `14-cau-hinh.md §8` NAMES these tables as what "giờ làm
	// việc" is counted from (line 318) but specifies NO SCREEN that edits them, and ADR 0007 says
	// the same in its gap list. Who may edit a commune's calendar has not been asked — it is the
	// sibling of open question #21 — and a half-written write path looks like a decision somebody
	// made. The consequence is heavier here than on a catalogue: a working calendar is the BASIS
	// OF AN ISSUED COMMITMENT (migration 0006:41).
	//
	// NO idem.* DECLARATION ON ANY OF THE THREE: a GET changes no state, and declaring a
	// duplicate-request protection would claim a protection with nothing to protect.
	//
	// NO 403 IN ANY @reply BLOCK, AND THAT IS NOT AN OMISSION: an AnyAuthenticated route has no
	// permission to fail, so 403 is a status these handlers never return. A declared status the
	// handler cannot produce is a contract the admin web writes dead code for.
	//
	// ⚠ STATED GAP — `?year=` DOES NOT REACH THE CONTRACT. The two date routes take a MANDATORY
	// `year` query parameter, and apidoc's vocabulary has no annotation for a query parameter: it
	// derives path parameters from the template, adds Idempotency-Key from idem.*, and adds
	// paging from @page (tools/apidoc/openapi.go:168-213). So kb/20-contracts/openapi.json will
	// describe these two routes WITHOUT the one parameter a caller must send, and a client
	// generated from it gets 400 until somebody reads this file. The honest fix is a `@query`
	// annotation in apidoc, which is a change to the contract tooling (ADR 0014) and not
	// something these routes may decide on their own. Said out loud rather than papered over.

	// @summary  Lịch làm việc thông thường của xã — mỗi dòng là một CA, nghỉ trưa là khoảng hở giữa hai ca
	// @screen   14-cau-hinh §8
	// 200 carries `problems` beside `items`: an empty calendar and two overlapping sessions are
	// both states the database cannot refuse, and both are DERIVED on every read, never stored.
	// 500 covers an ordinary store failure and the week exceeding idstore.TranLichLamViec — which
	// this route REFUSES rather than truncating, because a session missing from the calendar makes
	// every deadline computed afterwards longer than the commitment the commune actually made.
	//
	// @reply    200 danhSachCaLamViecRa
	// @reply    401 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/working-hours",
		authz.AnyAuthenticated("giờ làm việc của xã nằm dưới mọi hạn xử lý hiện trên màn hình — ngày đến hạn của nhiệm vụ, chip 'còn mấy ngày' của phản ánh, mọi ô chọn ngày — nên đòi một quyền cấu hình sẽ làm hỏng những màn hình đó cho mọi tài khoản không phải quản trị; đánh đổi đã chấp nhận: giờ làm việc lộ cho mọi tài khoản đã đăng nhập CỦA CHÍNH XÃ ĐÓ, không chéo xã vì Scoped buộc tenant_id")(
			http.HandlerFunc(h.DanhSachCaLamViec)))

	// @summary  Ngày nghỉ lễ của xã trong một năm — ngày xã KHÔNG làm việc, gồm cả lễ quốc gia lẫn lễ địa phương
	// @screen   14-cau-hinh §8
	// 400 is the mandatory `year`: missing, unreadable, outside 2000–2100, or given TWICE — a
	// repeated parameter is refused rather than resolved to the first value, which is the same
	// discipline as the 409 below.
	// 409 is a date recorded BOTH here and in ngay_lam_bu. Not a failure and not a bad request:
	// the commune's own configuration contradicts itself, and this route refuses rather than
	// picking a winner (domain.LoiNgayVuaNghiVuaLamBu).
	// 500 covers a store failure and the year exceeding idstore.TranNgayNghiLeMotNam.
	//
	// @reply    200 danhSachNgayNghiLeRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/public-holidays",
		authz.AnyAuthenticated("ngày nghỉ lễ quyết định hạn xử lý hiện trên màn hình nhiệm vụ, phản ánh và văn bản, và mọi ô chọn ngày phải biết ngày nào xã đóng cửa — đòi một quyền cấu hình sẽ làm hỏng những màn hình đó cho mọi tài khoản không phải quản trị; đánh đổi đã chấp nhận: lịch nghỉ lộ cho mọi tài khoản đã đăng nhập CỦA CHÍNH XÃ ĐÓ, không chéo xã vì Scoped buộc tenant_id")(
			http.HandlerFunc(h.DanhSachNgayNghiLe)))

	// @summary  Ngày làm bù của xã trong một năm — ngày xã CÓ làm việc dù lịch tuần nói không, kèm giờ làm của chính ngày đó
	// @screen   14-cau-hinh §8
	// 400 and 409 read exactly as on the route above — the two date routes share one year parser
	// and one refusal, because a conflict answered on only one side would leave the other screen
	// looking healthy and the commune would fix nothing.
	// 200 carries `problems` for two swap-day sessions that overlap on one date: the same double
	// count as the weekly calendar, on a table whose UNIQUE key also only stops two sessions
	// STARTING at the same minute.
	// 500 covers a store failure and the year exceeding idstore.TranNgayLamBuMotNam.
	//
	// @reply    200 danhSachCaLamBuRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/swap-working-days",
		authz.AnyAuthenticated("ngày làm bù theo thông báo hằng năm của Thủ tướng quyết định hạn xử lý đúng vào những ngày tồn đọng nhiều nhất trong năm, và mọi ô chọn ngày phải biết ngày nào xã vẫn làm việc — đòi một quyền cấu hình sẽ làm hỏng những màn hình đó cho mọi tài khoản không phải quản trị; đánh đổi đã chấp nhận: lịch làm bù lộ cho mọi tài khoản đã đăng nhập CỦA CHÍNH XÃ ĐÓ, không chéo xã vì Scoped buộc tenant_id")(
			http.HandlerFunc(h.DanhSachCaLamBu)))
}
