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

// THE THREE PATTERNS A SESSION UNDER THE FORCED PASSWORD CHANGE MAY STILL REACH, and they are
// CONSTANTS RATHER THAN LITERALS FOR EXACTLY ONE REASON: `XacThuc` in middleware.go refuses every
// other route while `phai_doi_mat_khau` is true (open question #9), and it decides that by
// comparing the request against these same three values. Written twice, the allow-list and the mux
// would drift — and the drift has no symptom in either direction that a test would catch by
// accident. Too loose and a person carrying an administrator's password walks into the rest of the
// system; too tight and they cannot reach the screen that would free them, which is a member of
// staff locked out of a government system with no way forward at all.
//
// THEY CARRY THE METHOD AS WELL AS THE PATH, because that is what `mux.Handle` takes and what the
// comparison needs. `tools/apidoc` resolves a route pattern given as a same-file constant
// (tools/apidoc/route.go, mauRoute), so the REST contract is generated from these exactly as it is
// from a literal.
//
// THE FOURTH THING THAT IS ALLOWED IS NOT HERE, because it needs no permission at all: a request
// carrying NO principal. A Public route — the sign-in screen, the commune's name — never reaches
// the check. See duocPhepKhiPhaiDoiMatKhau.
const (
	// MauDoiMatKhauChinhMinh is the one route that clears the flag. Exported because the forced
	// change is a behaviour of the whole edge, and a caller assembling this service's chain — or a
	// test asserting the refusal — needs to name it without copying the string.
	MauDoiMatKhauChinhMinh = "PUT /api/v1/staff/current/password"

	// mauXemPhienHienTai — "who am I, and why am I being sent to this screen". Without it the
	// forced-change page cannot render the person's own name, and the client cannot tell a session
	// that must change its password from one that was simply signed out.
	mauXemPhienHienTai = "GET /api/v1/sessions/current"

	// mauDangXuat — ending one's own session must never be the thing that is blocked. Somebody who
	// does not want to change their password on a shared counter machine has to be able to walk
	// away, and leaving them signed in is worse for everybody than letting them leave.
	mauDangXuat = "DELETE /api/v1/sessions/{sid}"
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
	//
	// ONE DanhSach FOR THE LIST AND THE SEARCH: GET /api/v1/staff passes the URL filters, POST
	// /api/v1/staff/searches passes the same filters plus the text. One store method means one
	// predicate, one soft-delete clause and one keyset walk for both — two would be two places for
	// `deleted_at IS NULL` to be forgotten.
	CanBoDanhBa interface {
		DanhSach(ctx context.Context, loc domain.LocCanBo, yc page.Request) (page.Result[domain.CanBoTomTat], error)
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
	// TaiKhoanCanBoUC is the CREDENTIAL surface — issuing an account, resetting a password, and a
	// person changing their own (open questions #9, #17).
	//
	// SEPARATE FROM CanBoGhiDanhBa, although both are use cases over the same table and both are
	// reached from the Cấu hình → Người dùng screen. An interface is the list of things a handler
	// CAN do, and these three are the only operations in this service that mint a working
	// credential: keeping them on their own type means the five register handlers — which edit
	// names, telephone numbers and roles — hold no value that could produce one.
	//
	// THE SELF ROUTE IS ON THE SAME INTERFACE AS THE TWO ADMINISTRATOR ROUTES even though it is
	// guarded differently. They are one use case type because they are one invariant: exactly one
	// of the three clears `phai_doi_mat_khau`, and that is only checkable if all three are written
	// and read together (app/tai_khoan_can_bo.go).
	TaiKhoanCanBoUC interface {
		Cap(ctx context.Context, id string, nguoi app.NguoiThucHien) (app.KetQuaCapMatKhau, error)
		DatLai(ctx context.Context, id string, nguoi app.NguoiThucHien) (app.KetQuaCapMatKhau, error)
		DoiCuaChinhMinh(ctx context.Context, yc app.YeuCauDoiMatKhau, nguoi app.NguoiThucHien) error
	}

	CanBoGhiDanhBa interface {
		Them(ctx context.Context, yc app.YeuCauThemCanBo, nguoi app.NguoiThucHien) (domain.CanBoTomTat, error)
		Sua(ctx context.Context, id string, yc app.YeuCauSuaCanBo, nguoi app.NguoiThucHien) (domain.CanBoTomTat, error)
		DatKhoa(ctx context.Context, id string, khoa bool, nguoi app.NguoiThucHien) (domain.CanBoTomTat, error)
		DoiVaiTro(ctx context.Context, id, vaiTroID string, nguoi app.NguoiThucHien) (domain.CanBoTomTat, error)
		// DatCongKhai — the Mini App publication of ONE person (#12). On this interface because it
		// is a use case over the same row with the same transaction-plus-audit shape; guarded by a
		// DIFFERENT permission (`content.update`) at the route.
		DatCongKhai(ctx context.Context, id string, yc app.YeuCauCongKhai, nguoi app.NguoiThucHien) (domain.CanBoTomTat, error)
	}

	// SLADoc reads the commune's processing-deadline table, for GET /api/v1/sla.
	//
	// SEPARATE FROM SLAGhi although one *idstore.SLAStore sits behind both in production, and the
	// split is the same one CanBoDanhBa / CanBoGhiDanhBa makes: the read is a STORE, the write is a
	// USE CASE, because every write opens the transaction its audit entry shares (rule 6,
	// invariant 3). One interface would let a future handler call a write with no transaction in
	// sight.
	SLADoc interface {
		DanhSach(ctx context.Context) ([]domain.DongSLA, error)
	}

	// SLAGhi is the WRITE surface of the deadline table: PATCH /api/v1/sla/{id} and
	// POST /api/v1/sla/defaults. A use case, not a store.
	//
	// NEITHER METHOD TAKES A FIELD CODE, and that is what keeps these routes clear of ADR 0026
	// stop condition #2 — see internal/http/sla.go and internal/store/sla_ghi.go.
	SLAGhi interface {
		Sua(ctx context.Context, id string, yc app.YeuCauSuaSLA, nguoi app.NguoiThucHien) (domain.DongSLA, error)
		GieoMacDinh(ctx context.Context, nguoi app.NguoiThucHien) (app.KetQuaGieo, error)
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
	// THE THREE READS HAVE THREE WRITE SIBLINGS SINCE 2026-09-23 — LichLamViecGhi, NgayNghiLeGhi
	// and NgayLamBuGhi below. The sentence that used to stand here said the write path was missing
	// because *who may edit a commune's calendar had not been asked*. It has an answer that invents
	// nothing: `admin.sla`, the key migration 0001:277 already seeds for "Cấu hình thời hạn xử lý",
	// which is the same screen (14-cau-hinh.md §8 names these three tables at :318 as what "giờ làm
	// việc" means). No new permission key was created — rule 5, invariant 3c.

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

	// The WRITE surface of the working calendar. THREE INTERFACES OVER ONE *app.Lich, and the split
	// is not decoration: an interface is the list of things a handler CAN do, so the holiday
	// handler must not be able to move a working session. One type sits behind them because the
	// rules that matter here SPAN the three tables — a date that is both a closure and a swap day,
	// a swap day on a weekday that already works (ADR 0007 decision 9) — and three types would be
	// three copies of the same three store handles (app/lich_lam_viec.go states it in full).
	//
	// EVERY METHOD IS A USE CASE, NEVER A STORE: each one opens the transaction its audit entry
	// shares (rule 6, invariant 3). There is no signature here that would let a calendar change and
	// its trail land in two transactions.

	// LichLamViecGhi is the ordinary week: add a session, move one, remove one, sow the default week.
	LichLamViecGhi interface {
		ThemCa(ctx context.Context, yc app.YeuCauThemCa, nguoi app.NguoiThucHien) (domain.CaLamViec, error)
		SuaCa(ctx context.Context, id string, yc app.YeuCauSuaCa, nguoi app.NguoiThucHien) (domain.CaLamViec, error)
		XoaCa(ctx context.Context, id, lyDo string, nguoi app.NguoiThucHien) error
		GieoTuanMacDinh(ctx context.Context, nguoi app.NguoiThucHien) (app.KetQuaGieoLich, error)
	}

	// NgayNghiLeGhi is the closure dates. THE SEED TAKES A YEAR and has no default for it, for the
	// same reason the read route's `year` is mandatory: "this year" silently changes at midnight on
	// 31/12.
	NgayNghiLeGhi interface {
		ThemNgayNghi(ctx context.Context, yc app.YeuCauThemNgayNghi, nguoi app.NguoiThucHien) (domain.NgayNghiLe, error)
		SuaNgayNghi(ctx context.Context, id string, yc app.YeuCauSuaNgayNghi, nguoi app.NguoiThucHien) (domain.NgayNghiLe, error)
		XoaNgayNghi(ctx context.Context, id, lyDo string, nguoi app.NguoiThucHien) error
		GieoNgayNghiLeMacDinh(ctx context.Context, nam int, nguoi app.NguoiThucHien) (app.KetQuaGieoLich, error)
	}

	// NgayLamBuGhi is the swap working days. THREE METHODS AND NOT FOUR — there is deliberately no
	// seed: a swap day exists only because the Prime Minister announced one for a particular year,
	// so there is no fixed set to sow (domain.KhongCoNgayLamBuMacDinh).
	NgayLamBuGhi interface {
		ThemLamBu(ctx context.Context, yc app.YeuCauThemLamBu, nguoi app.NguoiThucHien) (domain.CaLamBu, error)
		SuaLamBu(ctx context.Context, id string, yc app.YeuCauSuaLamBu, nguoi app.NguoiThucHien) (domain.CaLamBu, error)
		XoaLamBu(ctx context.Context, id, lyDo string, nguoi app.NguoiThucHien) error
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
	// /api/v1/public-holidays, /api/v1/swap-working-days.
	LichLamViec LichLamViecDoc
	NgayNghiLe  NgayNghiLeDoc
	NgayLamBu   NgayLamBuDoc
	// The WRITE surface of the same three tables. SIX FIELDS FOR THREE TABLES, read and write, for
	// the reason given at LichLamViecGhi: the read is a STORE, every write is a USE CASE, because a
	// write opens the transaction its audit entry shares. In production one *app.Lich satisfies all
	// three write fields.
	//
	// AN EMPTY CALENDAR IS WHAT MAKES grpc.AdvanceWorkingHours ANSWER FAILED_PRECONDITION, and
	// therefore what makes `service-documents` refuse every entry into the document register and
	// `service-petitions` refuse every petition. These three fields are the only path by which a
	// commune can fill it.
	GhiLichLamViec LichLamViecGhi
	GhiNgayNghiLe  NgayNghiLeGhi
	GhiNgayLamBu   NgayLamBuGhi
	// The commune's processing deadlines in working hours (migration 0008, ADR 0029) — the other
	// half of the three calendar tables above: those answer "lúc nào", this one answers "bao lâu".
	//
	// TWO FIELDS FOR ONE TABLE, read and write, for the reason given at SLADoc. Unlike the calendar,
	// this one DOES have a write surface: `admin.sla` already names who may configure deadlines
	// (migration 0001:277), and an empty table here is what refuses every entry into the document
	// register and every petition (ADR 0029 §"Xã chưa cấu hình").
	SLA       SLADoc
	GhiSLA    SLAGhi
	Signer    *token.Signer
	Phien     PhienDoc
	CanBo     CanBoDoc
	DanhBa    CanBoDanhBa
	GhiDanhBa CanBoGhiDanhBa
	// TaiKhoan is the credential surface: POST /api/v1/staff/{id}/account,
	// PUT /api/v1/staff/{id}/password and PUT /api/v1/staff/current/password. A use case, not a
	// store — every method opens the transaction the write and its audit entry share.
	TaiKhoan TaiKhoanCanBoUC
	DangNhap DangNhapUC
	DangXuat DangXuatUC

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
	case d.TaiKhoan == nil:
		// Refused at construction, like every other dependency here — and this one has a second
		// consequence worth naming: without the three credential routes, an account whose
		// `phai_doi_mat_khau` is true has NO route by which to clear it, so the forced-change gate
		// in XacThuc would refuse that person everything, permanently.
		panic("identity/http: thiếu use case tài khoản cán bộ — không cấp, không đặt lại và KHÔNG ĐỔI ĐƯỢC mật khẩu, nên tài khoản bị bắt đổi sẽ kẹt vĩnh viễn")
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
	case d.GhiLichLamViec == nil || d.GhiNgayNghiLe == nil || d.GhiNgayLamBu == nil:
		// Refused at construction like every other dependency, and this one has the same second
		// consequence as GhiSLA below: without these write routes a commune has NO way to fill an
		// empty working calendar, and an empty calendar makes every deadline uncomputable — so
		// every entry into the document register and every petition is refused, with the service
		// running perfectly and nothing saying why.
		panic("identity/http: thiếu use case ghi lịch làm việc — xã sẽ không có cách nào khai giờ làm việc, ngày nghỉ lễ hay ngày làm bù, và mọi tuyến tính hạn vẫn bị từ chối")
	case d.SLA == nil:
		panic("identity/http: thiếu kho thời hạn xử lý — GET /api/v1/sla sẽ panic khi có người gọi")
	case d.GhiSLA == nil:
		// Refused at construction like every other dependency, and this one has a second consequence
		// worth naming: without the two write routes a commune has NO way to fill an empty `sla`
		// table, and an empty table is what makes every entry into the document register and every
		// petition answer 409 `sla_chua_cau_hinh`. The service would start and the commune would be
		// unable to do its work, with nothing saying why.
		panic("identity/http: thiếu use case ghi thời hạn xử lý — xã sẽ không có cách nào cấu hình SLA, và mọi tuyến vào sổ vẫn bị từ chối")
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
	// THE PATTERN IS A CONSTANT, NOT A LITERAL, and the reason is at the declaration: XacThuc
	// compares against the same value to decide what a forced password change still allows.
	mux.Handle(mauDangXuat,
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
	mux.Handle(mauXemPhienHienTai,
		authz.AnyAuthenticated("mọi tài khoản đã đăng nhập đều được hỏi 'tôi là ai' — kể cả tài khoản vừa bị gỡ hết vai trò, vốn không còn quyền nào để đòi, và kể cả tài khoản đang bị bắt đổi mật khẩu, vốn cần đúng màn hình này để biết vì sao")(
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

	// Searching the register by free text. POST /api/v1/staff/searches
	//
	// A POST FOR A READ, because the text is usually a name or a telephone number and a URL is
	// written into access logs, proxies and browser history (rule 3, forbidden #4; user decision
	// 2026-09-24). The full argument is on TimCanBo in can_bo_tim.go.
	//
	// `admin.user`, THE SAME KEY AS THE LIST IT MIRRORS: it returns the same rows in the same shape,
	// so a different key would let somebody find, by searching, the people they may not list — or
	// the reverse. Already seeded (migration 0001:278); no key invented (rule 5, invariant 3c).
	//
	// idem.KhongCan AND NOT Required: nothing is written, so a repeated request cannot leave a second
	// row, a second code or a second audit entry — it simply reads again.
	//
	// @summary  Tìm cán bộ trong danh bạ của xã theo họ tên, chức vụ hoặc số điện thoại — từ khoá đi trong THÂN, không lên URL
	// @screen   12-danh-ba-can-bo §3
	// @request  timCanBoVao
	// @reply    200 page.Result[canBoTomTat]
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/staff/searches",
		authz.RequirePermission(d.Checker, "admin.user")(
			idem.KhongCan("tìm kiếm chỉ đọc, không đổi trạng thái nào — gửi lại chỉ là đọc lại, không sinh dòng, mã hay vết thứ hai")(
				http.HandlerFunc(h.TimCanBo))))

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
	// shows — the publication flag and the ordering, i.e. open question #12. None of the five
	// routes here touches that surface; PUT /api/v1/staff/{id}/publication below does, and it is
	// the one staff route that declares `content.update`.
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

	// Publishing somebody to the Zalo Mini App directory. PUT /api/v1/staff/{id}/publication
	//
	// OPEN QUESTION #12 (decided 2026-09-22; lean consent form and this route's shape decided by the
	// user on 2026-09-24): putting a personal mobile on a public channel is publication of personal
	// data under Decree 13/2023/NĐ-CP. PER PERSON, never bulk — one {id} per request, and there is no
	// list form. Publishing REQUIRES `consent_confirmed: true` in the body; the server records WHEN
	// and WHO (the recorder's staff code, never an internal id). Unpublishing clears both marks in
	// the same UPDATE, so publishing again later asks again. Every change is one audit entry in the
	// same transaction.
	//
	// `content.update` AND NOT `admin.user` — user decision 2026-09-24, and it is the key the Danh bạ
	// screen specifies for the Mini App directory (12-danh-ba-can-bo.md §9.4). Seeded by migration
	// 0001:293 ("Sửa nội dung và danh bạ Mini App"). Managing accounts and deciding what the public
	// sees are different authorities; a commune may give them to different people.
	//
	// `publication` IS THE NOUN rest_api_guard.py itself proposes for the verb `publish`. It has no
	// row in kb/00-foundation/ubiquitous-language.md yet — the same standing `lockout` has (ADR
	// 0011): written so the work is testable, reported as an open naming decision.
	//
	// `display_order` IS ON THIS ROUTE and not on the PATCH: the position in the citizen-facing
	// directory belongs to the publication surface (user decision 2026-09-24).
	//
	// 404 FOR ANOTHER COMMUNE'S ID AND FOR A SOFT-DELETED PERSON — TheoIDDeGhi is scoped and
	// excludes deleted rows, so neither can be told from an invented id (rule 4, forbidden #2).
	//
	// idem.KhongCan — PUT carries an absolute state, and the use case writes NOTHING when nothing
	// moves: publishing somebody already published keeps the original consent marks, and
	// unpublishing somebody unpublished is a no-op. The same request twice leaves one row state and
	// one entry.
	//
	// @summary  Công khai / thôi công khai một cán bộ lên danh bạ Zalo Mini App — bắt buộc xác nhận đã được người đó đồng ý (#12)
	// @screen   12-danh-ba-can-bo §9.4
	// @request  datCongKhaiVao
	// @reply    200 canBoTomTat
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PUT /api/v1/staff/{id}/publication",
		authz.RequirePermission(d.Checker, "content.update")(
			idem.KhongCan("PUT mang trạng thái tuyệt đối: công khai người đã công khai (giữ nguyên dấu đồng ý gốc) hay thôi công khai người chưa công khai thì use case không ghi gì, nên lần gửi thứ hai để lại đúng một trạng thái và đúng một vết")(
				http.HandlerFunc(h.DatCongKhaiCanBo))))

	// --- the staff register: the CREDENTIAL routes (#9, #17, #18) --------------------------------
	//
	// TWO SUB-RESOURCE NOUNS ARE NEW HERE, AND NEITHER HAS A ROW IN
	// kb/00-foundation/ubiquitous-language.md: `account` and `password`. ADR 0011 says a concept
	// with no row is a concept to ASK about rather than translate on the spot, so both are written
	// to make the work testable and BOTH ARE REPORTED AS OPEN NAMING DECISIONS — the same standing
	// `lockout` has had since it was written one turn earlier. Changing either is still free: no
	// commune is live.
	//
	// WHY THEY ARE TWO NOUNS AND NOT ONE ROUTE WITH TWO MEANINGS. `POST .../account` and
	// `PUT .../password` do the same mechanical thing — mint a value, hash it, set
	// `phai_doi_mat_khau` — and they are two acts with two consequences. One creates the ability to
	// sign in to a government system at all; the other replaces a credential on an ability that
	// already exists. Folded into one route they would share one audit verb, and a ledger where
	// "this person was given access" and "this person's password was reset" are the same entry
	// cannot answer an inspection without somebody interpreting every delta (rule 6, invariant 2).
	//
	// `account` IS A STATE SUB-RESOURCE, the same shape as `lockout`: POST creates it. A future
	// `GET .../account` describing when it was issued fits without a new path, and a `DELETE` —
	// withdrawing somebody's access while keeping their directory entry — is DELIBERATELY ABSENT.
	// It is a third act with its own permission, no key in the 35-key `quyen` table means it, and
	// rule 5, invariant 3c forbids inventing one. Same finding for #27 as the soft delete of #10.
	//
	// ALL THREE DECLARE A PERMISSION EXPLICITLY, and the two administrator routes declare
	// `admin.user` — CHECKED AGAINST THE TABLE, not assumed: it is the key the Cấu hình → Người
	// dùng tab is specified with (14-cau-hinh.md §12.8), seeded by migration 0001:278, and already
	// carried by the seven staff routes above. Managing accounts IS what "Quản lý người dùng"
	// means, so a commune that granted somebody that screen granted them this.

	// Giving somebody an account. POST /api/v1/staff/{id}/account
	//
	// 201 CARRIES THE ONE-TIME PASSWORD, and it is the only response in this service that carries a
	// working credential. It is never stored, never audited and never returned again — see the
	// header of tai_khoan_can_bo.go.
	//
	// idem.KhongCan, AND THE PROTECTION IS REAL RATHER THAN CLAIMED: the account itself is the
	// natural unique key. `capTaiKhoanCanBo` carries `AND NOT co_tai_khoan` in its WHERE clause and
	// the use case refuses ErrDaCoTaiKhoan before it, both inside the transaction that holds the row
	// — so a double-submitted form produces exactly ONE account and ONE audit entry, and the second
	// request answers 409. That is the opposite of POST /api/v1/staff above, where there is no key
	// underneath and a duplicate row is permanent.
	//
	// WHAT A LOST FIRST RESPONSE COSTS, STATED: the account exists and nobody knows its password.
	// The administrator resets it, which mints a different value and files a second entry saying so.
	// Recoverable, visible, and better than the alternative — an idempotency replay cannot return
	// the password either, because core/idem deliberately never stores a response body.
	//
	// @summary  Cấp tài khoản đăng nhập cho một cán bộ đang có trong danh bạ — hệ thống sinh mật khẩu tạm, trả về ĐÚNG MỘT LẦN
	// @screen   14-cau-hinh §3
	// @reply    201 capTaiKhoanRa
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/staff/{id}/account",
		authz.RequirePermission(d.Checker, "admin.user")(
			idem.KhongCan("tài khoản LÀ khoá tự nhiên: câu UPDATE mang `AND NOT co_tai_khoan` và use case từ chối trước đó, cả hai trong giao dịch giữ dòng — nên lần gửi thứ hai trả 409 chứ không cấp thêm tài khoản nào")(
				http.HandlerFunc(h.CapTaiKhoanCanBo))))

	// An administrator resetting somebody's password. PUT /api/v1/staff/{id}/password
	//
	// THIS IS OPEN QUESTION #17's WHOLE ANSWER. There is no `/quen-mat-khau` and no
	// `/dat-lai-mat-khau/:token`: the self-service flow inherits #9's dead loop — a commune's mail
	// server is per-commune configuration and may not be filled in — so the one moment a person
	// needs it most, being unable to sign in, is the moment it is least likely to work.
	//
	// #14 REFUSES A SELF-TARGET HERE AND IT STOPS SOMETHING REAL, unlike on the account route: this
	// route does not ask for the current password, because its purpose is to help somebody who
	// cannot supply one. On #18's SHARED counter machine, an unattended signed-in browser would
	// otherwise be a complete account takeover with no credential and no distinguishing trail. An
	// administrator changing their own password uses the route below, which demands the current one.
	//
	// idem.Required(idem.MoKhiHong) — THE ONE ROUTE IN THIS SERVICE WITH NO NATURAL KEY AND NO
	// STATE TO LEAN ON. Each call mints a DIFFERENT password, so a double-click leaves the
	// administrator reading out a value that the second request invalidated 200ms earlier, and the
	// person is locked out with nothing on any screen saying why.
	//
	// MoKhiHong AND NOT DongKhiHong, which is the opposite choice from POST /api/v1/staff and needs
	// its reason stated: this route IS the recovery path. Refusing it while Redis is down would
	// leave the person it exists to rescue locked out for the length of a cache outage, and the harm
	// it guards against — a superseded password read aloud — is visible immediately and fixed by
	// resetting again. Nothing here is permanent: no record is created, no code is issued (rule 7).
	//
	// @summary  Quản trị viên xã đặt lại mật khẩu hộ một cán bộ — sinh mật khẩu tạm mới, trả về ĐÚNG MỘT LẦN
	// @screen   14-cau-hinh §3
	// @reply    200 capTaiKhoanRa
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PUT /api/v1/staff/{id}/password",
		authz.RequirePermission(d.Checker, "admin.user")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.DatLaiMatKhauCanBo))))

	// The person changing their own password. PUT /api/v1/staff/current/password
	//
	// `current` IS THE SAME SELECTOR `GET /api/v1/sessions/current` AND `GET /api/v1/communes/
	// current` ALREADY USE: "the one this request is", derived from the request itself and never
	// from a parameter. It cannot be a request body field and it cannot be `{id}` — a person naming
	// whose password they are setting is rule 4, invariant 2 applied to staff.
	//
	// IT DOES NOT COLLIDE WITH `PUT /api/v1/staff/{id}/password` ABOVE, and that is a rule of
	// net/http rather than luck: a literal segment beats a wildcard because this pattern matches a
	// strict subset of that one, so ServeMux registers both and routes `current` here. It is pinned
	// by a test rather than left to a reader's memory — if it ever inverted, a person changing their
	// own password would be answered by the administrator route, which requires `admin.user` and
	// would refuse most of the commune. A staff id is a ULID and is never the literal `current`.
	//
	// AnyAuthenticated AND NOT A PERMISSION, for the same reason as sign-out: every account must be
	// able to do this, including one whose roles were all withdrawn, and INCLUDING one sitting under
	// the forced change of #9 — which is most of the accounts that will ever call it. Requiring a
	// permission would mean the people who must change their password are the people who cannot.
	//
	// THE CURRENT PASSWORD IS MANDATORY, INCLUDING ON THE FORCED-CHANGE SCREEN. See doiMatKhauVao.
	//
	// idem.KhongCan — the state protects it, and visibly: after the first request the old password
	// no longer verifies, so a second identical request answers 400, and every session including
	// this one has been revoked, so it more likely answers 401. Either way exactly one change and
	// one audit entry.
	//
	// @summary  Cán bộ tự đổi mật khẩu của chính mình — bắt buộc ở lần đăng nhập đầu, và là đường DUY NHẤT gỡ cờ bắt đổi
	// @screen   15-phu-luc-giao-dien-chung §1
	// @request  doiMatKhauVao
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle(MauDoiMatKhauChinhMinh,
		authz.AnyAuthenticated("mọi tài khoản phải tự đổi được mật khẩu của mình — kể cả tài khoản không còn quyền nào, và NHẤT LÀ tài khoản đang bị bắt đổi mật khẩu lần đầu (#9), vốn bị chặn mọi tuyến khác")(
			idem.KhongCan("đổi xong thì mật khẩu cũ không còn đúng và mọi phiên đã bị thu hồi, nên lần gửi thứ hai bị chính trạng thái từ chối — đúng một lần đổi, đúng một vết")(
				http.HandlerFunc(h.DoiMatKhauChinhMinh))))

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
	// ✔ KHOẢNG TRỐNG NÀY ĐÃ ĐÓNG 23/09/2026 — `?year=` NAY CÓ TRONG HỢP ĐỒNG. Khối này giữ lại
	// thay vì xoá sạch, vì bản thân nó là thứ đáng đọc hơn thứ nó từng tả.
	//
	// Khối này TỪNG viết: "`?year=` DOES NOT REACH THE CONTRACT … apidoc's vocabulary has no
	// annotation for a query parameter". Câu ấy đúng lúc viết và hết đúng khi `tools/apidoc` học
	// đọc tham số truy vấn bằng cách ĐI AST từ câu lệnh route xuống thân handler, thay vì chờ một
	// chú thích `@query` ai đó phải nhớ gõ — `tools/apidoc/truyvan.go`. Đo lại: openapi.json khai
	// `year` với `required: true` cho CẢ HAI tuyến `/api/v1/public-holidays` và `/swap-working-days`.
	//
	// VÌ SAO GHI RA THAY VÌ XOÁ IM LẶNG: một chú thích nói "chỗ này còn thiếu" khi nó không còn
	// thiếu là chú thích đẩy người sau đi vá LẦN HAI — và bản vá thứ hai ấy sẽ đúng là cái `@query`
	// viết tay mà bộ sinh vừa thôi cần. Bài học rộng hơn: một câu "chưa làm được" viết trong mã
	// KHÔNG tự hết hạn. Nó sống đúng bằng trí nhớ của người viết ra nó, và ở đây trí nhớ ấy là hai ngày.

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

	// --- the commune's working calendar, WRITE. ELEVEN ROUTES, ALL `admin.sla` -------------------
	//
	// THESE ROUTES ARE THE OTHER HALF OF WHAT UNBLOCKS TWO SERVICES. `ResolveDeadlines` needs TWO
	// things in order: the number of working hours (`sla`, filled by the three routes below) and
	// then the calendar to count those hours through (these three tables). With the calendar empty,
	// domain.TienGioLamViec refuses with `empty_calendar`, grpc.AdvanceWorkingHours turns that into
	// FAILED_PRECONDITION, and `service-documents` answers 409 on every entry into the register —
	// exactly as it does for an empty `sla`. Until now there was no route by which a commune could
	// fix that, which made it a closed loop.
	//
	// THEY BREAK THE LOOP BY FILLING THE TABLES. Not one of them softens a refusal: an empty
	// calendar is still FAILED_PRECONDITION, a date that is both a closure and a swap day is still
	// refused with no winner picked, and a swap day on a weekday that already works is still
	// ADR 0007 decision 9. Nothing here may ever become a fallback on the deadline path (rule 10,
	// forbidden #3; the argument in full at domain.BoGieoCaLamViec).
	//
	// `admin.sla` ON ALL ELEVEN, AND NO KEY IS INVENTED. The key exists in the `quyen` table
	// (migration 0001:277, "Cấu hình thời hạn xử lý") and the calendar is the same screen's other
	// half — 14-cau-hinh.md §8:318 names these three tables as what "giờ làm việc" means. Rule 5,
	// invariant 3c: a key no migration seeds is a right no administrator can grant, so the route
	// would answer 403 to every account forever while its tests stayed green. Rule 5, invariant 3b
	// is the other half of the argument: these rights are not a Cartesian product to be generated,
	// and a second key would need a migration on the permission catalogue — a finding for open
	// question #27, not a decision a route may take.
	//
	// THE READS ABOVE STAY AnyAuthenticated AND THAT ASYMMETRY IS DELIBERATE: office hours sit under
	// every deadline printed on a screen, so a configuration permission on the READ would break
	// those screens for every non-administrator. Writing is the act that moves a commune's
	// commitments, and exactly one job does that.
	//
	// EVERY ONE OF THE ELEVEN DECLARES idem.KhongCan, AND EACH CLAIM IS EARNED BY A NATURAL KEY OR
	// BY THE USE CASE WRITING NOTHING — never by hope. The reason is written on each statement.
	//
	// ⚠ 409 IS THE STATUS OF EVERY CALENDAR REFUSAL ON THIS SURFACE, and it is 409 rather than 403
	// on purpose: the caller HOLDS `admin.sla`. What is refused is the operation against the state
	// of the commune's own calendar, and 403 would send an administrator to the Phân quyền screen
	// to be granted a right they already have.

	// @summary  Thêm một ca làm việc vào tuần của xã — nghỉ trưa là khoảng hở giữa hai ca, không phải một cờ
	// @screen   14-cau-hinh §8
	// 400 is a weekday outside 1–7 (ISO: 1 = thứ Hai … 7 = Chủ nhật, KHÔNG phải getDay()), a time
	// that is not HH:MM/HH:MM:SS, or a session ending before it starts.
	// 409 is either of the two states the schema cannot refuse: a session overlapping one the commune
	// already has (the EXCLUDE constraint needs `btree_gist` — migration 0006:109), or a start minute
	// already taken on that weekday, INCLUDING by a soft-deleted row.
	//
	// @request  themCaLamViecVao
	// @reply    201 caLamViecRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/working-hours",
		authz.RequirePermission(d.Checker, "admin.sla")(
			idem.KhongCan("khoá duy nhất (tenant_id, thu, bat_dau) chỉ cho một ca mở đúng phút đó trong một thứ, nên lần gửi thứ hai bị chính khoá ấy từ chối chứ không tạo ca thứ hai — không có dòng trùng nào để chặn")(
				http.HandlerFunc(h.ThemCaLamViec))))

	// @summary  Sửa một ca làm việc — KHÔNG hồi tố lên hạn đã phát ra cho hồ sơ cũ
	// @screen   14-cau-hinh §8
	// 404 is an id matching no live row OF THIS COMMUNE — the same answer for an invented id, a
	// soft-deleted row and another authority's row, so none can be told apart by trying.
	//
	// @request  suaCaLamViecVao
	// @reply    200 caLamViecRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/working-hours/{id}",
		authz.RequirePermission(d.Checker, "admin.sla")(
			idem.KhongCan("sửa là ghi đè một trạng thái đã biết trên một dòng đã có; use case không ghi gì khi kết quả bằng đúng dòng vừa đọc, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.SuaCaLamViec))))

	// @summary  Xoá mềm một ca làm việc, kèm lý do bắt buộc — dòng ở lại, giờ mở ca không cấp lại được
	// @screen   14-cau-hinh §8
	// 400 is a missing or over-long `reason`: rule 7, invariant 1 names `delete_reason`, and a
	// removal nobody can be asked about is not a removal this surface performs.
	//
	// @request  xoaLichVao
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/working-hours/{id}",
		authz.RequirePermission(d.Checker, "admin.sla")(
			idem.KhongCan("xoá một dòng đã xoá trả 404 ở cả hai lần vì câu lệnh mang `deleted_at IS NULL`; lần gửi thứ hai không ghi đè lý do của lần xoá thật, nên không có trạng thái nào để bảo vệ")(
				http.HandlerFunc(h.XoaCaLamViec))))

	// @summary  Gieo tuần làm việc mặc định cho xã chưa cấu hình — KHÔNG ghi đè giờ xã đã sửa
	// @screen   14-cau-hinh §8
	// ⚠ GIỜ MẶC ĐỊNH KHÔNG CÓ TRONG ĐẶC TẢ NÀO. domain.BoGieoCaLamViec giữ lập luận đầy đủ: ADR 0007
	// liệt kê "Giờ hành chính của xã là mấy giờ tới mấy giờ?" là câu CHƯA AI TRẢ LỜI, và phân loại nó
	// là DỮ LIỆU chứ không phải quy tắc. Bốn con số ở đó là ví dụ của chính migration 0006:90, và là
	// GIÁ TRỊ KHỞI TẠO xã sửa được ngay ngày đầu — không phải hằng số của phần mềm.
	// 200 on every run, first or tenth: the request brings the week to a known state rather than
	// creating one addressable resource, so there is no Location a 201 would owe. `skipped` counts
	// seed rows NOT written because they would have overlapped a session the commune already had —
	// the seed button must never be the thing that makes deadlines uncomputable.
	//
	// @reply    200 gieoLichRa
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/working-hours/defaults",
		authz.RequirePermission(d.Checker, "admin.sla")(
			idem.KhongCan("bộ gieo chỉ CHÈN những ca xã chưa có, quyết định bên trong đúng giao dịch ghi, tính cả dòng đã xoá mềm là 'đã có', và khoá duy nhất (tenant_id, thu, bat_dau) chặn dòng thứ hai — nên lần bấm thứ hai không ghi gì, không ghi đè giờ xã đã sửa, và trả seeded: 0")(
				http.HandlerFunc(h.GieoCaLamViecMacDinh))))

	// @summary  Thêm một ngày nghỉ lễ của xã — ngày xã KHÔNG làm việc, gồm cả lễ quốc gia lẫn lễ địa phương
	// @screen   14-cau-hinh §8
	// 409 is a date the commune has ALREADY declared a ngày làm bù — closed and working on one day.
	// The system refuses rather than picking a winner: a silent precedence rule would make one of two
	// VISIBLE configuration rows do nothing, and nobody would ever see which (migration 0006:254).
	//
	// @request  themNgayNghiLeVao
	// @reply    201 ngayNghiLeRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/public-holidays",
		authz.RequirePermission(d.Checker, "admin.sla")(
			idem.KhongCan("khoá duy nhất (tenant_id, ngay) chỉ cho một dòng mỗi ngày, nên lần gửi thứ hai bị chính khoá ấy từ chối chứ không tạo dòng thứ hai")(
				http.HandlerFunc(h.ThemNgayNghiLe))))

	// @summary  Sửa một ngày nghỉ lễ — KHÔNG hồi tố lên hạn đã phát ra cho hồ sơ cũ
	// @screen   14-cau-hinh §8
	//
	// @request  suaNgayNghiLeVao
	// @reply    200 ngayNghiLeRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/public-holidays/{id}",
		authz.RequirePermission(d.Checker, "admin.sla")(
			idem.KhongCan("sửa là ghi đè một trạng thái đã biết; use case không ghi gì khi kết quả bằng đúng dòng vừa đọc, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.SuaNgayNghiLe))))

	// @summary  Xoá mềm một ngày nghỉ lễ, kèm lý do bắt buộc — dòng ở lại, ngày đó không khai lại được
	// @screen   14-cau-hinh §8
	//
	// @request  xoaLichVao
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/public-holidays/{id}",
		authz.RequirePermission(d.Checker, "admin.sla")(
			idem.KhongCan("xoá một dòng đã xoá trả 404 ở cả hai lần vì câu lệnh mang `deleted_at IS NULL`; lần gửi thứ hai không ghi đè lý do của lần xoá thật")(
				http.HandlerFunc(h.XoaNgayNghiLe))))

	// @summary  Gieo các ngày nghỉ lễ CỐ ĐỊNH THEO DƯƠNG LỊCH của một năm — BỐN ngày, không phải mười một
	// @screen   14-cau-hinh §8
	// ⚠ TẾT NGUYÊN ĐÁN VÀ GIỖ TỔ HÙNG VƯƠNG CỐ Ý KHÔNG ĐƯỢC GIEO: cả hai theo ÂM LỊCH, và một phép
	// quy đổi âm lịch tự viết sai một ngày là một hạn đếm xuyên qua ngày cơ quan đóng cửa — sai đúng
	// chiều báo cáo một cơ quan là trễ trong khi nó không trễ. NGÀY LIỀN KỀ 02/9 cũng không gieo: Bộ
	// luật Lao động 2019 điều 112 khoản 3 giao cho Thủ tướng chọn giữa 01/9 và 03/9 TỪNG NĂM. Xã tự
	// nhập ba nhóm ngày ấy qua POST /api/v1/public-holidays. Lập luận đầy đủ: domain.BoGieoNgayNghiLe.
	// 400 is a missing `year` — "năm nay" is not a default this code gets to pick, for the same reason
	// the read route's `year` is mandatory: it would silently change at midnight on 31/12.
	//
	// @request  gieoNgayNghiLeVao
	// @reply    200 gieoLichRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/public-holidays/defaults",
		authz.RequirePermission(d.Checker, "admin.sla")(
			idem.KhongCan("bộ gieo chỉ CHÈN những ngày xã chưa có trong năm ấy, quyết định bên trong đúng giao dịch ghi, tính cả dòng đã xoá mềm là 'đã có', và khoá duy nhất (tenant_id, ngay) chặn dòng thứ hai — nên lần bấm thứ hai trả seeded: 0 và không ghi đè tên xã đã sửa")(
				http.HandlerFunc(h.GieoNgayNghiLeMacDinh))))

	// @summary  Thêm một ca làm bù — ngày xã CÓ làm việc dù lịch tuần nói không, kèm giờ làm của chính ngày đó
	// @screen   14-cau-hinh §8
	// 409 covers THREE refusals, each one a state the deadline function would otherwise meet later:
	// the date is also a ngày nghỉ lễ; the weekday ALREADY has sessions in the ordinary week
	// (ADR 0007 decision 9 — REPLACE and ADD are both defensible and nothing decides between them);
	// or the session overlaps another swap-day session on that date.
	//
	// @request  themNgayLamBuVao
	// @reply    201 caLamBuRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/swap-working-days",
		authz.RequirePermission(d.Checker, "admin.sla")(
			idem.KhongCan("khoá duy nhất (tenant_id, ngay, bat_dau) chỉ cho một ca mở đúng phút đó trong một ngày, nên lần gửi thứ hai bị chính khoá ấy từ chối chứ không tạo ca thứ hai")(
				http.HandlerFunc(h.ThemNgayLamBu))))

	// @summary  Sửa một ca làm bù — KHÔNG hồi tố lên hạn đã phát ra cho hồ sơ cũ
	// @screen   14-cau-hinh §8
	//
	// @request  suaNgayLamBuVao
	// @reply    200 caLamBuRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/swap-working-days/{id}",
		authz.RequirePermission(d.Checker, "admin.sla")(
			idem.KhongCan("sửa là ghi đè một trạng thái đã biết; use case không ghi gì khi kết quả bằng đúng dòng vừa đọc, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.SuaNgayLamBu))))

	// @summary  Xoá mềm một ca làm bù, kèm lý do bắt buộc — dòng ở lại vì hạn đã phát ra đếm qua nó
	// @screen   14-cau-hinh §8
	//
	// @request  xoaLichVao
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/swap-working-days/{id}",
		authz.RequirePermission(d.Checker, "admin.sla")(
			idem.KhongCan("xoá một dòng đã xoá trả 404 ở cả hai lần vì câu lệnh mang `deleted_at IS NULL`; lần gửi thứ hai không ghi đè lý do của lần xoá thật")(
				http.HandlerFunc(h.XoaNgayLamBu))))

	// ---- Cấu hình → Thời hạn xử lý (14-cau-hinh.md §8, migration 0008, ADR 0029) -----------------
	//
	// ALL THREE DECLARE `admin.sla`, WHICH ALREADY EXISTS IN THE `quyen` TABLE (migration 0001:277,
	// "Cấu hình thời hạn xử lý"). No key is invented — rule 5, invariant 3c.
	//
	// THESE ARE THE ROUTES THAT UNBLOCK TWO OTHER SERVICES. `sla` is empty in every commune, so
	// grpc.ResolveDeadlines answers FAILED_PRECONDITION and `service-documents` answers 409
	// `sla_chua_cau_hinh` on every entry into the register; `service-petitions` is refused the same
	// way. Until now there was no route by which a commune could fix that, which made it a closed
	// loop. These routes break the loop BY FILLING THE TABLE — they do not soften the refusal, and
	// nothing here may ever become a fallback on the deadline path (rule 10, forbidden #3; the
	// argument in full at domain.BoGieoSLA).
	//
	// WHY THE READ IS `admin.sla` AND NOT AnyAuthenticated LIKE THE THREE CALENDAR READS ABOVE:
	// internal/http/sla.go states it — the calendar is read to RENDER every screen showing a date,
	// this table is read to CONFIGURE, and the service that needs the figures reads them over gRPC
	// rather than here. Restricting it breaks no screen, and 14-cau-hinh.md:394 hides the tab from
	// accounts without the key anyway.
	//
	// ⚠ THE PATH SEGMENT `sla` IS NOT A NOUN THE USER HAS APPROVED —
	// kb/00-foundation/ubiquitous-language.md:296 says of this exact table "đừng điền sẵn một cái
	// tên", and ADR 0011 makes it the user's call. It is used because the commissioning task named
	// it. `processing-deadlines` would match the migration's `-- @entity: ProcessingDeadline` and the
	// plural-word convention of every sibling resource. Renaming is cheap here and only here — no
	// APPLIED migration mark references it. STATED GAP, needs the user's decision.

	// @summary  Bảng thời hạn xử lý của xã — số GIỜ LÀM VIỆC cho từng loại việc và lĩnh vực
	// @screen   14-cau-hinh §8
	// 200 carries `problems` beside `items`: an empty table and a kind of work missing its default
	// row are both states the database cannot refuse, and both are DERIVED on every read, never
	// stored (rule 10, invariant 3). An empty table is today the answer for EVERY commune.
	// 500 covers an ordinary store failure and the table exceeding idstore.TranSLA — REFUSED rather
	// than truncated, because a dropped row makes DongTheoLinhVuc fall back to the default and
	// quietly answer with a different promise than the commune made.
	//
	// @reply    200 danhSachSLARa
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/sla",
		authz.RequirePermission(d.Checker, "admin.sla")(
			http.HandlerFunc(h.DanhSachSLA)))

	// idem.KhongCan, AND THE PROTECTION IS THE STATE RATHER THAN A HOPE: app.SLA.Sua applies the
	// figures to the row it read and writes NOTHING when the result equals what was already there —
	// no UPDATE, no audit entry. So a second identical request leaves exactly one row and exactly
	// one entry, which is what the declaration claims.
	//
	// @summary  Sửa năm con số của một dòng thời hạn — KHÔNG hồi tố lên hồ sơ đã tiếp nhận
	// @screen   14-cau-hinh §8
	// 400 is a body that is not JSON, a body mentioning no figure at all, or a figure outside
	// 0 < giờ <= domain.GioToiDa.
	// 404 is an id matching no live row OF THIS COMMUNE — the same answer for an invented id, a
	// soft-deleted row and another authority's row, so none can be told apart by trying.
	//
	// @request  suaSLAVao
	// @reply    200 dongSLARa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/sla/{id}",
		authz.RequirePermission(d.Checker, "admin.sla")(
			idem.KhongCan("sửa là ghi đè một trạng thái đã biết trên một dòng đã có; use case không ghi gì khi năm con số không đổi, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.SuaSLA))))

	// idem.KhongCan, AND THIS IS THE ONE WHERE THE CLAIM HAS TO BE EARNED RATHER THAN ASSERTED: the
	// natural key IS the protection. `UNIQUE (tenant_id, loai_viec, linh_vuc_khoa)` admits one live
	// row per pair, and the use case only ever INSERTS rows whose pair it did not find inside the
	// same transaction. A second request therefore finds all fifteen present, writes nothing, audits
	// nothing, and answers `seeded: 0, kept: 15`. It also OVERWRITES NOTHING a commune has edited —
	// there is no path from the use case to an UPDATE, and the store offers no UPSERT to reach for.
	//
	// @summary  Gieo bộ thời hạn mặc định cho xã chưa cấu hình — KHÔNG ghi đè con số xã đã sửa
	// @screen   14-cau-hinh §8
	// 200 on every run, first or fifteenth: the request brings the table to a known state rather
	// than creating one addressable resource, so there is no Location a 201 would owe.
	// 409 is two administrators pressing the button at the same instant — this transaction lost the
	// race, nothing was half-written, and retrying answers `seeded: 0`.
	//
	// @reply    200 gieoSLARa
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/sla/defaults",
		authz.RequirePermission(d.Checker, "admin.sla")(
			idem.KhongCan("bộ gieo chỉ CHÈN những dòng xã chưa có, quyết định bên trong đúng giao dịch ghi, và khoá duy nhất (tenant_id, loai_viec, linh_vuc_khoa) chặn dòng thứ hai — nên lần bấm thứ hai không ghi gì, không ghi đè con số xã đã sửa, và trả seeded: 0")(
				http.HandlerFunc(h.GieoSLAMacDinh))))
}
