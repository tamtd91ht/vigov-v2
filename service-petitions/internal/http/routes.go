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
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
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

	// TrangThaiNhiemVuDoc is the commune's OVERRIDES of the seven task-status labels (migration
	// 0010), for GET /api/v1/task-statuses. It returns only the rows that exist; the handler merges
	// them over domain.MacDinhTrangThaiNhiemVu, so a code with no row is the default, never missing.
	TrangThaiNhiemVuDoc interface {
		DanhSach(ctx context.Context) ([]domain.NhanTrangThaiNhiemVu, error)
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

	// PhieuPhanAnhDanhSach is the paginated read of the register, for GET /api/v1/citizen-reports.
	//
	// A THIRD INTERFACE OVER THE SAME STORE, and not a method added to PhieuPhanAnhDoc, because the
	// two answer different questions and one of them carries the restricted-field decision. A list
	// route that could reach TheoMaTraCuu would be a list route somebody could make read one
	// petition WITHOUT the `can-bo` filter — which is exactly the path that filter exists to close.
	PhieuPhanAnhDanhSach interface {
		DanhSach(ctx context.Context, loc petstore.LocPhieu, yc page.Request) (
			page.Result[domain.PhieuPhanAnh], error)
	}

	// NhiemVuDoc is the read side of ONE task, for GET /api/v1/tasks/{ma}.
	//
	// READ ONLY, and separate from the list below for the same reason PhieuPhanAnhDoc is separate
	// from PhieuPhanAnhDanhSach: a route depending on a wider surface than it uses is how the next
	// person justifies reaching through it. The write methods this store will grow in the next pass
	// have no business being reachable from a GET.
	NhiemVuDoc interface {
		TheoMa(ctx context.Context, ma string) (domain.NhiemVu, error)

		// §5.4's document block (migration 0009). A SECOND METHOD AND NOT A WIDER TheoMa, because
		// the two surfaces need different things: the register LIST renders no documents, and
		// folding the block into the single-task read would have been the shape that put it there
		// too. It is keyed by the task's INTERNAL id, which the caller has from the row it just
		// read.
		VanBanCuaNhiemVu(ctx context.Context, nhiemVuID string) ([]domain.NhiemVuVanBan, error)
	}

	// NhiemVuDanhSach is the paginated read of the task register, for GET /api/v1/tasks.
	NhiemVuDanhSach interface {
		DanhSach(ctx context.Context, loc petstore.LocNhiemVu, yc page.Request) (
			page.Result[domain.NhiemVu], error)
	}

	// GhiNhiemVuUseCase is the six STAFF acts on a task, each of which opens a transaction and
	// writes the business change, the timeline row and the audit entry inside it (rule 6,
	// invariant 3).
	//
	// ONE INTERFACE FOR THE SIX, unlike the two catalogue writers above, and the reason is the
	// opposite of theirs: those are DIFFERENT CATALOGUES, so one shared interface would need to be
	// told which table to write. These six are six acts on ONE record — they share the locking
	// read, the tree walks and the timeline — and splitting them would be six wiring lines that can
	// disagree about which use case instance, and therefore which transaction boundary, is in play.
	//
	// THE PERMISSIONS ARE NOT IN THE INTERFACE, deliberately: they are declared per route below, at
	// the one place rbac_guard and tools/apidoc both read.
	//
	// ⚠ ONLY DoiTrangThai TAKES app.QuyenDuyetHoanThanh, AND THE ASYMMETRY IS THE DEFENCE. It is
	// the caller's answer to "does this account hold `task.approve`", and it can only ever NARROW —
	// no value of it lets anybody do something they could not otherwise do. A second method growing
	// the same parameter would be a second act that could be signed off by the wrong key.
	GhiNhiemVuUseCase interface {
		Tao(ctx context.Context, yc app.YeuCauTaoNhiemVu, nguoi audit.Actor) (domain.NhiemVu, error)
		Sua(ctx context.Context, ma string, sua petstore.SuaNhiemVu, nguoi audit.Actor) (
			domain.NhiemVu, error)
		DoiTrangThai(ctx context.Context, ma string, yc app.YeuCauDoiTrangThai, nguoi audit.Actor,
			duyet app.QuyenDuyetHoanThanh) (domain.NhiemVu, error)
		Xoa(ctx context.Context, ma, lyDo string, nguoi audit.Actor) error
		DeNghiLuiHan(ctx context.Context, ma string, yc app.YeuCauDeNghiLuiHan, nguoi audit.Actor) (
			domain.DeNghiLuiHan, error)
		QuyetDinhLuiHan(ctx context.Context, ma, deNghiID string, yc app.YeuCauQuyetDinhLuiHan,
			nguoi audit.Actor) (domain.DeNghiLuiHan, error)
	}

	// BienBanDanhSach is the paginated read of the meeting-minutes register, for
	// GET /api/v1/meetings — each meeting already carrying its conclusions and their task counters.
	//
	// ONE METHOD, AND THE COUNTERS ARE INSIDE IT rather than a second method the handler would call
	// and add up. §7.5's badge is a SUM over the conclusions of one meeting, and a handler holding
	// two halves of that sum is a handler that can pair the wrong ones: the aggregation belongs to
	// the query that already visits both tables, and domain.BienBanHop.TienDoNhiemVu is the only
	// arithmetic left above it.
	BienBanDanhSach interface {
		DanhSach(ctx context.Context, yc page.Request) (page.Result[domain.BienBanHop], error)
	}

	// GhiBienBanUseCase is the three STAFF acts on the meeting register (§3, §4): record the
	// minutes, append a conclusion, split a conclusion into a task.
	//
	// ONE INTERFACE FOR THE THREE, for the reason GhiNhiemVuUseCase gives: they are three acts on
	// ONE record, they share the ordinal rule and the lock it is read under, and splitting them
	// would be three wiring lines that can disagree about which use case instance — and therefore
	// which transaction boundary — is in play.
	//
	// ⚠ THE THIRD METHOD RETURNS A TASK AND NOT A CONCLUSION, and that asymmetry IS the flow: the
	// split does not write into this register at all. It resolves the conclusion and hands the work
	// to the task register's own create use case, so everything a task is — the minted number, the
	// cycle check, the deadline fixed once, the timeline row, the audit entry in one transaction —
	// stays in one place (app.TachKetLuanThanhNhiemVu).
	//
	// ⚠ IT TAKES app.YeuCauTaoNhiemVu WITH NO `nguon_giao` / `nguon_id` SUPPLIED BY THE HANDLER.
	// The pair is set from the conclusion named in the PATH, which is what makes §3's locked "Nguồn
	// giao" field a property of the server rather than a field somebody has to remember to check.
	GhiBienBanUseCase interface {
		TaoBienBan(ctx context.Context, yc app.YeuCauTaoBienBan, nguoi audit.Actor) (
			domain.BienBanHop, error)
		ThemKetLuan(ctx context.Context, bienBanID string, yc app.YeuCauThemKetLuan,
			nguoi audit.Actor) (domain.KetLuanHop, error)
		TachKetLuanThanhNhiemVu(ctx context.Context, bienBanID string, thuTu int,
			yc app.YeuCauTaoNhiemVu, nguoi audit.Actor) (domain.NhiemVu, error)
	}

	// XuLyPhieuPhanAnh is the four STAFF acts, each of which opens a transaction and writes the
	// business change, the audit entry and the notification obligation inside it (rule 6, invariant
	// 3; rule 10, invariant 5).
	//
	// ONE INTERFACE FOR THE FOUR, unlike the two catalogue writers above, and the reason is the
	// opposite of theirs: those two are DIFFERENT CATALOGUES, so one shared interface would need to
	// be told which table to write. These four are four acts on ONE record, they share the locking
	// read and the outbox, and splitting them would be four wiring lines that can disagree about
	// which use case instance — and therefore which transaction boundary — is in play.
	//
	// THE PERMISSIONS ARE NOT IN THE INTERFACE, deliberately: they are declared per route below, at
	// the one place rbac_guard and tools/apidoc both read.
	//
	// ALL FOUR TAKE app.QuyenXemHanChe, AND THE UNIFORMITY IS THE DEFENCE. It is the caller's answer
	// to "does this account hold `feedback.restricted`", and the use case refuses the ACT on a
	// petition in `can-bo` when it is false (app.duocChamPhieuHanChe). A method here without that
	// parameter would be a write path that cannot refuse — which is exactly what all four were until
	// 2026-09-23, while the two READ paths had refused from the day they were written.
	XuLyPhieuPhanAnh interface {
		ChotLinhVuc(ctx context.Context, ma string, yc app.YeuCauChotLinhVuc, nguoi audit.Actor,
			hanChe app.QuyenXemHanChe) (domain.PhieuPhanAnh, error)
		PhanCong(ctx context.Context, ma string, yc app.YeuCauPhanCong, nguoi audit.Actor,
			hanChe app.QuyenXemHanChe) (domain.PhieuPhanAnh, error)
		// TienTrangThai TAKES THE COMMUNE-WIDE RIGHT AS AN ARGUMENT AND Dong DOES NOT — see
		// app.QuyenXuLyCaXa. The holding rule of 2026-09-23 widened the WORKING path only, and the
		// asymmetry in this interface is what makes the closing path impossible to widen by accident.
		// The restricted fact beside it is a DIFFERENT KIND of argument: it can only ever narrow.
		TienTrangThai(ctx context.Context, ma string, nguoi audit.Actor, quyen app.QuyenXuLyCaXa,
			hanChe app.QuyenXemHanChe) (domain.PhieuPhanAnh, error)
		Dong(ctx context.Context, ma, ketQua string, nguoi audit.Actor,
			hanChe app.QuyenXemHanChe) (domain.PhieuPhanAnh, error)
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

// GhiTrangThaiNhiemVu is the ONE write on task-status wording. There is no Them and no Xoa, and that
// absence is open question #21's answer (ADR 0035 §C): the code list is closed and has no `Tắt`.
type GhiTrangThaiNhiemVu interface {
	Sua(ctx context.Context, ma string, yc app.YeuCauSuaNhanTrangThai, nguoi audit.Actor) (
		domain.TrangThaiHienThi, error)
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

	// The task-status wording: a read of the overrides, and the one write that opens a transaction
	// and audits inside it. Two fields for the reason the catalogue pairs above give.
	TrangThaiNhiemVu    TrangThaiNhiemVuDoc
	GhiTrangThaiNhiemVu GhiTrangThaiNhiemVu

	Phieu       PhieuPhanAnhDoc
	NhanLinhVuc NhanLinhVucDanhMuc

	// The register list and the four staff acts. THE LIST IS SEPARATE FROM `Phieu` on purpose — see
	// PhieuPhanAnhDanhSach — and `XuLyPhieu` is given *store.DB rather than a transaction because
	// opening one is precisely what it is for (rule 6, invariant 3).
	DanhSachPhieu PhieuPhanAnhDanhSach
	XuLyPhieu     XuLyPhieuPhanAnh

	// The TASK register — two read paths over one store, see NhiemVuDoc.
	NhiemVu         NhiemVuDoc
	DanhSachNhiemVu NhiemVuDanhSach

	// The SIX WRITE acts on a task (migrations 0006 and 0008). A THIRD field rather than methods on
	// either interface above, and for the reason the catalogue pair states: a read is a store call,
	// while each of these opens a TRANSACTION and writes an audit entry inside it. Behind one
	// interface a future caller would reach for whichever method was nearest, and could end up
	// writing the row outside a transaction — the exact defect core/audit was shaped to prevent.
	GhiNhiemVu GhiNhiemVuUseCase

	// The MEETING MINUTES register — one read path in this pass (migration 0007). It is the ORIGIN
	// of the tasks above: a conclusion is split into a task and the task keeps a permanent back-link
	// to it through `nguon_giao`/`nguon_id`, which is why both registers live in this service.
	DanhSachBienBan BienBanDanhSach

	// The THREE WRITE acts on the meeting register. A SEPARATE field from the read one, and for the
	// reason every pair above states: a read is a store call, while recording minutes opens a
	// TRANSACTION and writes an audit entry inside it. Behind one interface a future caller would
	// reach for whichever method was nearest — and could end up writing the row outside a
	// transaction, the exact defect core/audit was shaped to prevent.
	GhiBienBan GhiBienBanUseCase

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
	case d.TrangThaiNhiemVu == nil:
		panic("petitions/http: thiếu kho nhãn trạng thái nhiệm vụ — GET /api/v1/task-statuses sẽ panic khi có người gọi")
	case d.GhiTrangThaiNhiemVu == nil:
		panic("petitions/http: thiếu use case ghi nhãn trạng thái nhiệm vụ — PATCH /api/v1/task-statuses/{code} sẽ panic khi có người gọi")
	case d.Checker == nil:
		panic("petitions/http: thiếu Checker — GET /api/v1/citizen-reports/{maTraCuu} khai quyền feedback.read, " +
			"và một Checker rỗng sẽ panic lúc có cán bộ gọi chứ không phải lúc khởi động")
	case d.Phieu == nil:
		panic("petitions/http: thiếu kho phiếu phản ánh — GET /api/v1/citizen-reports/{maTraCuu} sẽ panic khi có người gọi")
	case d.NhanLinhVuc == nil:
		panic("petitions/http: thiếu kho nhãn lĩnh vực — GET /api/v1/citizen-reports/{maTraCuu} sẽ panic khi có người gọi")
	case d.DanhSachPhieu == nil:
		panic("petitions/http: thiếu đường đọc danh sách phiếu — GET /api/v1/citizen-reports sẽ panic khi có người gọi")
	case d.XuLyPhieu == nil:
		// THE FOUR WRITE ROUTES AT ONCE. A nil here does not break a screen quietly: it breaks every
		// act a member of staff can perform on a petition, which means petitions come in and nothing
		// can be done with them — the exact state this service was in before these routes existed.
		panic("petitions/http: thiếu use case xử lý phiếu — bốn tuyến phân loại/phân công/chuyển trạng thái/đóng phiếu sẽ panic khi có người gọi")
	case d.NhiemVu == nil:
		panic("petitions/http: thiếu kho nhiệm vụ — GET /api/v1/tasks/{ma} sẽ panic khi có người gọi")
	case d.DanhSachNhiemVu == nil:
		panic("petitions/http: thiếu đường đọc danh sách nhiệm vụ — GET /api/v1/tasks sẽ panic khi có người gọi")
	case d.GhiNhiemVu == nil:
		// THE SIX WRITE ROUTES AT ONCE. A nil here does not break one screen: it breaks giao việc,
		// sửa, chuyển trạng thái, xoá and both halves of lùi hạn — which puts the task register back
		// in the state it was in before this pass, readable and unchangeable, while four other
		// subsystems stand on it.
		panic("petitions/http: thiếu use case ghi nhiệm vụ — sáu tuyến giao việc/sửa/chuyển trạng thái/xoá/lùi hạn sẽ panic khi có người gọi")
	case d.DanhSachBienBan == nil:
		panic("petitions/http: thiếu đường đọc danh sách biên bản họp — GET /api/v1/meetings sẽ panic khi có người gọi")
	case d.GhiBienBan == nil:
		// THE THREE WRITE ROUTES AT ONCE. A nil here leaves the meeting register readable and
		// unfillable — and with it §3, the ONE path by which a conclusion becomes a task and keeps a
		// back-link to where it came from.
		panic("petitions/http: thiếu use case ghi biên bản họp — ba tuyến nhập biên bản/thêm kết luận/tách nhiệm vụ sẽ panic khi có người gọi")
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
	// The task-STATUS wording is not a 0003 catalogue: #21 (ADR 0035 §C) closed its code list, so it
	// is an override table (migration 0010) with a read and a single PATCH — see task-statuses below.
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

	// --- the commune's wording and order for the seven task statuses (#21, ADR 0035 §C) ----------
	//
	// ALL SEVEN CODES, ALWAYS: the commune's override where it has one, the default
	// (domain.MacDinhTrangThaiNhiemVu) where it has not. Sorted by effective order, ties by default
	// order. AnyAuthenticated for the reason the two catalogue reads above give: these labels are the
	// Kanban column headers and the status filter of every task screen.
	//
	// 500 when the overrides cannot be read — REFUSED rather than answered with the defaults, which
	// would silently show a commune wording it replaced. 401, no 403: there is no permission to fail.
	//
	// @summary  Bảy trạng thái nhiệm vụ với nhãn và thứ tự của xã (mặc định nếu xã chưa sửa) — cột Kanban, bộ lọc, màn hình cấu hình
	// @screen   02-nhiem-vu §6
	// @reply    200 danhSachTrangThaiNhiemVuRa
	// @reply    401 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/task-statuses",
		authz.AnyAuthenticated("nhãn trạng thái là tiêu đề cột Kanban và giá trị bộ lọc trạng thái của mọi màn hình nhiệm vụ — đòi một quyền cấu hình sẽ làm mất tên cột cho mọi tài khoản không phải quản trị; đánh đổi đã chấp nhận: cách gọi trạng thái của xã lộ cho mọi tài khoản đã đăng nhập CỦA CHÍNH XÃ ĐÓ, không chéo xã vì Scoped buộc tenant_id")(
			http.HandlerFunc(h.DanhSachTrangThaiNhiemVu)))

	// Re-word or re-order ONE status. `admin.lookup` — the same key as the six catalogue writes (see
	// the block above Register): this is the same `Danh mục` configuration of the commune.
	//
	// 404 for a code outside the seven — the list is closed (#21). 400 for a blank label, a label
	// over 100 characters (counted in characters, as the CHECK counts), an order below 1, or a body
	// naming `code` or `active` (no rename, no `Tắt`).
	//
	// idem.KhongCan: the use case writes nothing and audits nothing when neither field moves, and the
	// upsert stores absolute values — so the same request sent twice leaves one row, one entry.
	//
	// @summary  Sửa nhãn và/hoặc thứ tự hiển thị của một trạng thái nhiệm vụ trong xã (không thêm, xoá hay tắt mã)
	// @screen   14-cau-hinh §5
	// @request  suaTrangThaiNhiemVuVao
	// @reply    200 trangThaiNhiemVuRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/task-statuses/{code}",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("upsert ghi giá trị tuyệt đối và use case không ghi, không để vết khi không trường nào đổi, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.SuaTrangThaiNhiemVu))))

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

	// --- THE STAFF PROCESSING PATH. FIVE ROUTES, and until today there were NONE ------------------
	//
	// A citizen could file a petition and look it up, and no member of staff could classify it,
	// assign it, move it or close it. Every petition that arrived stayed where it landed, with a
	// deadline running and a person watching. The reasoning for each route, its URL noun and the two
	// findings raised for open question #27 are on internal/http/xu_ly_phan_anh.go.
	//
	// ONE THING IS TRUE OF ALL FOUR WRITE ROUTES AND IS SAID HERE ONCE (rule 9, invariant 2): a
	// petition in the field `can-bo` — a report ABOUT a member of staff — is refused to a caller
	// without `feedback.restricted`, and the answer is the 404 each block already declares, identical
	// to an unknown code. THE ACT IS REFUSED, not merely the body: nothing is written, no audit entry
	// is filed and no event is recorded. The decision is app.duocChamPhieuHanChe's, inside the
	// transaction; the route permissions below are unchanged and no key was added.

	// @summary  Danh sách phiếu phản ánh của xã — phân trang theo con trỏ, lọc theo trạng thái · lĩnh vực · thôn · bộ phận · kênh · trễ hạn
	// @screen   09-phan-anh-nguoi-dan §2, §4
	// `feedback.read` and NOT AnyAuthenticated, unlike the two catalogue reads above: this is a
	// commune's register of what its citizens have reported — names, numbers, addresses and the free
	// text of a complaint (rule 3). The specification gives it its own key for that reason.
	//
	// TWO THINGS DECIDED INSIDE THE HANDLER THAT NO STATUS CODE SHOWS, so they are stated here:
	//
	//	feedback.restricted  absent -> petitions in the field `can-bo` are NOT in the page, NOT in
	//	                     the cursor, and therefore not in any count derived by paging
	//	feedback.unmask      irrelevant -> the reporter is masked on this route WHATEVER the caller
	//	                     holds, because rule 6 invariant 7 wants one audit entry per disclosure
	//	                     and a list cannot produce an honest one
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @reply    200 page.Result[phieuPhanAnhRa]
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/citizen-reports",
		authz.RequirePermission(d.Checker, "feedback.read")(
			http.HandlerFunc(h.DanhSachPhieu)))

	// PHÂN LOẠI — the act that ISSUES THE COMMUNE'S PROMISE, which is why it has a key of its own.
	//
	// `feedback.classify` AND NOT `feedback.assign` (ADR 0030, and rule 5 invariant 3b: these rights
	// are not a Cartesian product). Settling the field is what fixes `han_xu_ly_xong` — being handed
	// the work is not being allowed to promise on behalf of the authority.
	//
	// idem.KhongCan, AND IT IS THE HONEST DECLARATION RATHER THAN THE COMFORTABLE ONE: the act is
	// idempotent by the shape of the lifecycle, not by luck. The UPDATE carries
	// `trang_thai = 'da-tiep-nhan'`, so a second identical request matches no row and answers 409 —
	// one classification, one deadline, one audit entry. idem.Required would hide the second attempt
	// instead of telling the officer their screen is stale.
	//
	// 409 COVERS TWO VERY DIFFERENT THINGS and the code in the body tells them apart:
	// `petition_state` (somebody classified it first) and `sla_chua_cau_hinh` (the commune has not
	// configured the hours for this field — the ordinary answer in every commune today).
	//
	// @summary  Chốt lĩnh vực cho phiếu phản ánh — hành vi ẤN ĐỊNH hạn xử lý xong theo cấu hình của xã
	// @screen   09-phan-anh-nguoi-dan §8.2
	// @request  phanLoaiVao
	// @reply    200 phieuPhanAnhRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/citizen-reports/{maTraCuu}/classification",
		authz.RequirePermission(d.Checker, "feedback.classify")(
			idem.KhongCan("câu UPDATE mang `trang_thai = 'da-tiep-nhan'`, nên lần gửi thứ hai không khớp dòng nào và trả 409 — đúng một lần chốt lĩnh vực, đúng một hạn, đúng một vết")(
				http.HandlerFunc(h.PhanLoaiPhieu))))

	// PHÂN CÔNG — who inside the authority is answerable for this petition.
	//
	// idem.KhongCan, and here the reason is different from the route above: a second identical
	// assignment writes the same department and the same officer onto the same row, so the ROW ends
	// in one state. It does file a second audit entry, and that is CORRECT rather than a duplicate —
	// somebody performed the act twice, and an append-only ledger records acts (rule 6, invariant 4).
	//
	// @summary  Chuyển phiếu phản ánh cho một bộ phận xử lý, kèm cán bộ phụ trách nếu đã biết
	// @screen   09-phan-anh-nguoi-dan §8.5
	// @request  phanCongVao
	// @reply    200 phieuPhanAnhRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/citizen-reports/{maTraCuu}/assignment",
		authz.RequirePermission(d.Checker, "feedback.assign")(
			idem.KhongCan("phân công lại cùng một bộ phận để lại đúng một dòng ở đúng một trạng thái; vết thứ hai là một hành vi CÓ THẬT của con người, không phải bản sao")(
				http.HandlerFunc(h.PhanCongPhieu))))

	// CHUYỂN TRẠNG THÁI — one step along the main flow, and the target is deliberately not a
	// parameter. See the handler.
	//
	// ⚠ `feedback.read` GUARDS THIS ROUTE AND `feedback.resolve` STILL GUARDS THE CLOSING ROUTE BELOW.
	// That difference is the holding rule the owner decided on 2026-09-23 ("theo require"), and it is
	// not a relaxation of rule 5: the declaration here is an explicit, seeded key, so an account
	// without `feedback.read` is refused at this gate before anything is read. The condition that
	// actually decides — `feedback.resolve` OR being the officer this petition was assigned to — lives
	// in app.duocTienTrangThai, inside the transaction, on the row read under the lock. A hamlet leader
	// handed one petition can move it; giving them `feedback.resolve` instead would let them close the
	// neighbouring hamlet's petitions too.
	//
	// NO KEY WAS INVENTED (rule 5, invariant 3c): `feedback.read` is seeded at
	// service-identity/migrations/0001_init.sql. The older note here raised the missing "move the work
	// along" key as a finding for open question #27; that finding still stands — this change answers
	// WHO may advance a petition, not whether advancing deserves a key of its own.
	//
	// idem.KhongCan: the UPDATE carries the expected status, so a double click advances the petition
	// exactly one step and the second request answers 409.
	//
	// @summary  Chuyển phiếu phản ánh sang bước kế tiếp của luồng chính (máy trạng thái quyết định bước nào)
	// @screen   09-phan-anh-nguoi-dan §8.2
	// @reply    200 phieuPhanAnhRa
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/citizen-reports/{maTraCuu}/status",
		authz.RequirePermission(d.Checker, "feedback.read")(
			idem.KhongCan("câu UPDATE mang trạng thái đang chờ, nên bấm hai lần vẫn chỉ tiến đúng một bước và lần thứ hai trả 409")(
				http.HandlerFunc(h.TienTrangThaiPhieu))))

	// ĐÓNG PHIẾU — the act rule 10, invariant 6 governs: it records a RESULT THE CITIZEN CAN READ,
	// and the server refuses a closing without one. "Đã xử lý" alone is not a result.
	//
	// `closure` IS THE SETTLED URL NOUN for `dong_phieu` (kb/00-foundation/ubiquitous-language.md) —
	// not translated on the spot.
	//
	// idem.KhongCan for the same mechanical reason as the two above: the UPDATE carries
	// `trang_thai = 'cho-dan-xac-nhan'`, so one closing, one result, one entry.
	//
	// @summary  Đóng phiếu phản ánh kèm kết quả xử lý người dân đọc được
	// @screen   09-phan-anh-nguoi-dan §8.2
	// @request  dongPhieuVao
	// @reply    200 phieuPhanAnhRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/citizen-reports/{maTraCuu}/closure",
		authz.RequirePermission(d.Checker, "feedback.resolve")(
			idem.KhongCan("câu UPDATE mang `trang_thai = 'cho-dan-xac-nhan'`, nên lần gửi thứ hai không khớp dòng nào — đúng một lần đóng, đúng một kết quả, đúng một vết")(
				http.HandlerFunc(h.DongPhieu))))

	// --- THE TASK REGISTER. TWO READ ROUTES AND SIX WRITE ROUTES ---------------------------------
	//
	// `tasks` is the settled URL noun for `nhiem_vu` (kb/00-foundation/ubiquitous-language.md:144),
	// not translated on the spot (ADR 0011).
	//
	// `task.read` ON BOTH READS, AND NOT AnyAuthenticated, unlike the two catalogue reads above. The
	// catalogues are the WORDS a commune sorts its work with and fill a picker on nearly every
	// screen; this is the work itself — who was told to do what, by when, and who has not. The
	// specification gives it a key of its own for that reason (§6), and the key is seeded at
	// service-identity/migrations/0001_init.sql:304. NO KEY WAS INVENTED (rule 5, invariant 3c).
	//
	// THE THREE REASONS THIS BLOCK ONCE GAVE FOR HAVING NO WRITE ROUTE ARE ANSWERED, and they are
	// marked answered rather than deleted: a reader who arrives through ADR 0029 §118 or through the
	// progress ledger needs to see that the reason MOVED, not that it vanished.
	//
	//	the sub-task tree        ANSWERED — ADR 0037 (2026-09-23) decided all four questions, and
	//	                         migration 0008 added the column. The three rules that do not fit in
	//	                         a CHECK live in internal/app, plus a FIFTH the specification never
	//	                         raised: a cycle across two or more rows.
	//	who approves an extension  ANSWERED — ADR 0038 (2026-09-23): the leader named ON THE RECORD,
	//	                         compared by staff business code, as a SECOND layer under
	//	                         `task.extend`. The empty-column case fails CLOSED.
	//	the deadline              NOT THE SAME QUESTION AS THE PETITION INTAKE'S. §7.1 puts "Hạn hoàn
	//	                         thành" on the form as a date a leader types, so creating a task
	//	                         derives no deadline and calls identity not at all — see the header of
	//	                         internal/app/nhiem_vu.go. (For completeness: identity's
	//	                         ResolveDeadlines does now cover `WORK_KIND_NHIEM_VU`, so the paths
	//	                         that WOULD need a computed deadline — §8's Excel import, splitting a
	//	                         meeting conclusion — have a contract to call when they are built.)
	//
	// STILL ABSENT, AND DELIBERATELY: §10's `giao-viec`. Assignment is `task.assign`'s act and PATCH
	// cannot move `bo_phan_id` or `nguoi_thuc_hien_ma` — folding it in would hand assignment to every
	// holder of `task.update`. Reported as a finding.

	// @summary  Danh sách nhiệm vụ của xã — phân trang theo con trỏ, lọc theo trạng thái · loại · khối · ưu tiên · bộ phận · người thực hiện · nguồn giao · trễ hạn
	// @screen   02-nhiem-vu §3, §4
	// 400 covers a filter the server REFUSES rather than ignores, and two of those refusals are not
	// bad input at all — they are missing contracts, answered with a sentence naming what is
	// missing instead of a page that answers a different question:
	//
	//	scope=related  needs the caller's own department; the staff principal carries none
	//	soon=true      needs the commune's own `sla.gio_sap_den_han`; identity exposes no RPC for it
	//
	// 500 additionally covers `scope=mine` on a principal with no staff business code — a wiring
	// fault, refused rather than silently widened to the whole register.
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @reply    200 page.Result[nhiemVuRa]
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/tasks",
		authz.RequirePermission(d.Checker, "task.read")(
			http.HandlerFunc(h.DanhSachNhiemVu)))

	// @summary  Một nhiệm vụ, tra theo mã nhiệm vụ của xã (NV19)
	// @screen   02-nhiem-vu §5
	// 404 is the single answer to three causes — no such number, another commune's number, and a
	// soft-deleted task. Telling them apart tells a caller which numbers exist in a register they
	// are not reading.
	//
	// 401 is RequirePermission's answer to no session AND to a session issued by another commune:
	// authz.xacNhanXa refuses before the permission is consulted, so no query runs and the response
	// cannot differ by commune.
	//
	// THE `Theo văn bản` DOCUMENT BLOCK OF §5.4 IS NOT IN THE REPLY — `nhiem_vu_van_ban` does not
	// exist yet (migration 0006 says so). Nor are the progress log (§5.9) and the extension requests
	// (§5.8): both have tables now, and neither has a route in this pass.
	//
	// @reply    200 nhiemVuRa
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/tasks/{ma}",
		authz.RequirePermission(d.Checker, "task.read")(
			http.HandlerFunc(h.DocNhiemVu)))

	// GIAO VIỆC MỚI (§7) — `task.create`, seeded at 0001_init.sql:301 ("Tạo nhiệm vụ").
	//
	// idem.Required(MoKhiHong), AND WHICH LAYER IS ACTUALLY PROTECTING THIS — the question
	// skills/rest-api-design §4 says to answer at the route. The real guard is
	// `UNIQUE (tenant_id, ma)`, which counts soft-deleted rows: two tasks carrying one register
	// number CANNOT EXIST, whatever happens to Redis. The idempotency key is the second, independent
	// layer — it is what stops a double-submitted form from producing two tasks with two DIFFERENT
	// minted numbers, which the unique key cannot see anything wrong with.
	//
	// MoKhiHong AND NOT DongKhiHong: with the unique key underneath, a cache outage cannot produce a
	// duplicate NUMBER, and refusing a commune mid-morning would pay with an outage for a risk that
	// is largely covered. THE RESIDUAL RISK IS STATED RATHER THAN GLOSSED: while the cache is down, a
	// double submit can mint NV20 and NV21 for one piece of work, and the spare is soft-deleted by
	// hand — a visible, recoverable nuisance, unlike a refused register.
	//
	// 409 COVERS TWO DIFFERENT THINGS and the code in the body tells them apart: `code_taken` (a
	// hand-typed number already issued, soft-deleted rows included) and `task_tree` (the named parent
	// is gone, or the move would close a cycle).
	//
	// @summary  Giao việc mới — tạo một nhiệm vụ, tự sinh mã theo dãy NV của xã hoặc nhận mã tự nhập
	// @screen   02-nhiem-vu §7
	// @request  taoNhiemVuVao
	// @reply    201 nhiemVuRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/tasks",
		authz.RequirePermission(d.Checker, "task.create")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.TaoNhiemVu))))

	// SỬA THÔNG TIN (§5.4's ✎ Sửa) — `task.update`, seeded at 0001_init.sql:305 ("Cập nhật tiến độ").
	//
	// PATCH AND NOT PUT: four editable fields have a meaningful zero, so a full replacement cannot
	// tell "not mentioned" from "set to zero" — see suaNhiemVuVao.
	//
	// ⚠ IT CANNOT MOVE THE DEADLINE AND IT CANNOT REWRITE "Lãnh đạo giao việc". The first is the
	// extension flow's act, decided by the leader; the second IS the approver under ADR 0038, so a
	// holder of this key able to rewrite it could name themselves the approver of their own
	// extension requests. Both absences are enforced by petstore.SuaNhiemVu having no such field.
	//
	// idem.KhongCan, AND THE REASON IS A PROPERTY OF THE USE CASE RATHER THAN A HOPE: app.Sua
	// compares the row it read against the row it would write and, when nothing moved, writes
	// NOTHING — no UPDATE and no audit entry. Were that comparison removed, this declaration would
	// become a lie and the second request would file an entry saying nothing changed.
	//
	// @summary  Sửa thông tin mô tả của một nhiệm vụ — không đụng tới hạn, trạng thái hay phân công
	// @screen   02-nhiem-vu §5.4
	// @request  suaNhiemVuVao
	// @reply    200 nhiemVuRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/tasks/{ma}",
		authz.RequirePermission(d.Checker, "task.update")(
			idem.KhongCan("app.Sua so dòng đọc được với dòng sắp ghi và KHÔNG ghi gì khi không có trường nào đổi, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.SuaNhiemVu))))

	// CHUYỂN TRẠNG THÁI (§6) — `task.update` at the gate, `task.approve` for the last step.
	//
	// ⚠ TWO KEYS, ONE ROUTE, AND THAT IS RULE 5 INVARIANT 3b RATHER THAN A RELAXATION OF IT. §6 names
	// both: `task.update` is "Cập nhật tiến độ" and `task.approve` is "Duyệt hoàn thành" — an officer
	// reports progress on their own work and somebody else signs it off. A route declares ONE
	// permission, so the gate is the broader key (an account without it is refused here, before
	// anything is read) and the narrower one is consulted in the handler and decided in
	// app.duocHoanThanh, inside the transaction. NEITHER KEY IS INVENTED; both are seeded.
	//
	// THE TARGET IS ON THE WIRE, unlike the petition path's `…/status`. §6's lifecycle BRANCHES at
	// every state, so there is no single "next" for the server to choose — what the server owns is
	// the MAP, and a move the diagram does not draw is refused with a 409.
	//
	// idem.KhongCan: the UPDATE carries the expected status, so a double click moves the task exactly
	// one step and the second request answers 409.
	//
	// @summary  Chuyển trạng thái một nhiệm vụ theo vòng đời §6, kèm ghi nhật ký — hoàn thành cần quyền duyệt và mọi việc con đã xong
	// @screen   02-nhiem-vu §6
	// @request  doiTrangThaiVao
	// @reply    200 nhiemVuRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/tasks/{ma}/status",
		authz.RequirePermission(d.Checker, "task.update")(
			idem.KhongCan("câu UPDATE mang trạng thái đang chờ, nên bấm hai lần vẫn chỉ chuyển đúng một bước và lần thứ hai trả 409")(
				http.HandlerFunc(h.DoiTrangThaiNhiemVu))))

	// XOÁ (§11.5) — `task.delete`, seeded at 0001_init.sql:302 ("Xoá nhiệm vụ khỏi sổ").
	//
	// THIS IS A SOFT DELETE AND THE METHOD IS THE ONLY THING THAT SAYS OTHERWISE. The row stays with
	// `deleted_at`, `deleted_by` and `delete_reason` (rule 7, invariant 1), its number stays taken
	// for ever, and its timeline survives — §11.5 asks for exactly that ("nên xoá mềm để giữ nhật
	// ký"). DELETE is still the right method: the resource is gone from every read path.
	//
	// ⚠ IT ANSWERS 409 WHILE THE TASK STILL HAS LIVE CHILDREN. That is ADR 0037 decision 3, and the
	// body says how many — refusing without saying what is in the way sends an officer hunting.
	//
	// A BODY ON A DELETE, and the alternative was worse: the reason is mandatory, and the query
	// string would put free text about a government record into every access log and proxy cache.
	//
	// idem.KhongCan — deleting an already-deleted task is a 404 either way, and the second request
	// cannot overwrite who deleted it or why: the UPDATE carries `AND deleted_at IS NULL`.
	//
	// @summary  Xoá mềm một nhiệm vụ khỏi sổ, kèm lý do bắt buộc — từ chối khi còn việc con chưa xoá
	// @screen   02-nhiem-vu §11
	// @request  xoaNhiemVuVao
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/tasks/{ma}",
		authz.RequirePermission(d.Checker, "task.delete")(
			idem.KhongCan("xoá một nhiệm vụ đã xoá cho cùng một kết quả: câu UPDATE mang `AND deleted_at IS NULL` nên lần thứ hai không ghi đè được người xoá và lý do")(
				http.HandlerFunc(h.XoaNhiemVu))))

	// ĐỀ NGHỊ LÙI HẠN (§5.8) — `task.update`, NOT `task.extend`.
	//
	// ⚠ THAT CHOICE IS ADR 0038'S WHOLE POINT. `task.extend` is labelled "Duyệt gia hạn" in the
	// `quyen` table — it is the right to DECIDE. Guarding this route with it would mean only people
	// who can approve an extension may ask for one, which is the opposite of §5.8: the box sits on
	// the drawer of the officer doing the work.
	//
	// idem.KhongCan: at most one request may be pending per task —
	// `UNIQUE (tenant_id, nhiem_vu_id, moc_cho_duyet)` — so a double submit answers 409 rather than
	// filing a second request the leader could approve twice.
	//
	// @summary  Gửi đề nghị lùi hạn cho một nhiệm vụ — hạn mới phải muộn hơn hạn đang có, kèm lý do bắt buộc
	// @screen   02-nhiem-vu §5.8
	// @request  deNghiLuiHanVao
	// @reply    201 deNghiLuiHanRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/tasks/{ma}/extensions",
		authz.RequirePermission(d.Checker, "task.update")(
			idem.KhongCan("mỗi nhiệm vụ chỉ có một đề nghị đang chờ duyệt — khoá duy nhất `(tenant_id, nhiem_vu_id, moc_cho_duyet)` làm lần gửi thứ hai trả 409 chứ không sinh đề nghị thứ hai")(
				http.HandlerFunc(h.DeNghiLuiHanNhiemVu))))

	// QUYẾT ĐỊNH LÙI HẠN (§5.8, ADR 0038) — `task.extend`, seeded at 0001_init.sql:303.
	//
	// ⚠ THE KEY IS THE FIRST OF TWO LAYERS AND IS NOT THE WHOLE ANSWER. Rule 5 checks
	// `(tenant_id, role, permission)` and has no "which record" dimension, so holding `task.extend`
	// says only that this account may touch extensions AT ALL. Whether it may decide THIS one —
	// `Principal.Ma == nhiem_vu.lanh_dao_giao_viec_ma`, compared as two STAFF BUSINESS CODES — is
	// decided in domain.DuocDuyetLuiHan, inside the transaction, on the task row read under the lock.
	// Drop the gate and anybody may call the route; drop the second layer and every leader holding
	// the key decides every task in the commune.
	//
	// A TASK THAT NAMES NO LEADER IS REFUSED, not routed to `nguoi_tao_ma`. That is ADR 0038's open
	// question failing CLOSED, and the sentence names what is missing — the only thing the commune
	// can act on. 403 covers it, together with the independent rule that nobody decides their own
	// request.
	//
	// idem.KhongCan: the UPDATE carries `trang_thai = 'cho-duyet'`, so one request gets one decision
	// and the second attempt answers 409.
	//
	// @summary  Lãnh đạo giao việc duyệt hoặc từ chối một đề nghị lùi hạn — duyệt thì đổi hạn xử lý, hạn ban đầu giữ nguyên
	// @screen   02-nhiem-vu §5.8
	// @request  quyetDinhLuiHanVao
	// @reply    200 deNghiLuiHanRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/tasks/{ma}/extensions/{deNghiID}/decision",
		authz.RequirePermission(d.Checker, "task.extend")(
			idem.KhongCan("câu UPDATE mang `trang_thai = 'cho-duyet'`, nên lần gửi thứ hai không khớp dòng nào — đúng một đề nghị, đúng một quyết định, đúng một vết")(
				http.HandlerFunc(h.QuyetDinhLuiHanNhiemVu))))

	// --- MEETING MINUTES AND THEIR CONCLUSIONS. ONE READ ROUTE AND THREE WRITE ROUTES -----------
	//
	// `meetings` IS NOT FROM kb/00-foundation/ubiquitous-language.md — that table has no row for
	// `bien_ban_hop` — and it is not translated on the spot either (ADR 0011 forbids both). It is
	// the noun the RUNNING sibling implementation serves at
	// `apps/api/app/modules/tasks/router.py:45`, read under the project owner's instruction of
	// 2026-09-23. The missing row is REPORTED as a finding for the session that owns that file,
	// never written from a route. The full reasoning is on internal/http/bien_ban_hop.go.
	//
	// `task.read` ON THE READ AND `task.create` ON THE THREE WRITES, and no `meeting.*` key is
	// invented: the `quyen` table has none (service-identity/migrations/0001_init.sql:273-305) and a
	// key no migration seeds is a right no administrator can grant — 403 to every account, forever,
	// with the tests still green (rule 5, invariant 3c). Everything this screen shows is either the
	// origin of a task or a count of tasks. Whether a commune wants "may record minutes" separated
	// from "may create tasks" is a question for the CUSTOMER — open question #27, whose `ask_before`
	// list names exactly this case — and it is reported as a finding. The full reasoning, including
	// what the shared key costs and why `document.create` would be worse, is on
	// internal/http/bien_ban_hop_ghi.go.
	//
	// WHAT IS STILL ABSENT, so the absence is not read as unfinished work: there is NO EDIT and NO
	// DELETE route for minutes or conclusions. Whether a conclusion that tasks already point at may
	// be reworded is an open question migration 0007 deliberately refused to answer in a trigger,
	// and §7.1's removal is "không xoá cứng, cảnh báo và giữ liên kết" — a warning flow, not an
	// UPDATE. Answering either from a route would be deciding it.
	//
	// ⚠ THE DEADLINE ON A SPLIT TASK IS TYPED, NOT DERIVED. §3 suggests a date when the conclusion's
	// sentence contains one ("báo cáo trước ngày 20/8") — that suggestion is the SCREEN's, and what
	// reaches `han_xu_ly` is whatever the person confirmed. Nothing on this path performs
	// working-hours arithmetic; that has one implementation, in identity (rule 10, forbidden #2).

	// @summary  Danh sách biên bản họp của xã — mỗi biên bản kèm các kết luận và bộ đếm nhiệm vụ đã tách / đã xong
	// @screen   04-bien-ban-hop §2, §5
	// NO FILTER AND NO SEARCH PARAMETER, because §2's screen has neither: one vertical list of
	// cards, newest first. The only query parameters are the page cursor's.
	//
	// 400 is page.Parse refusing a cursor or a sort key; the body never echoes what was sent.
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @reply    200 page.Result[bienBanRa]
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/meetings",
		authz.RequirePermission(d.Checker, "task.read")(
			http.HandlerFunc(h.DanhSachBienBan)))

	// NHẬP BIÊN BẢN (§4) — `task.create`, seeded at 0001_init.sql:301 ("Tạo nhiệm vụ").
	//
	// idem.Required(MoKhiHong), AND WHICH LAYER IS ACTUALLY PROTECTING THIS — the question
	// skills/rest-api-design §4 says to answer at the route. THE HONEST ANSWER HERE IS: ONLY THE
	// IDEMPOTENCY KEY. Unlike POST /api/v1/tasks there is no unique key underneath — migration 0007
	// refuses one on `so_hieu`, which is typed by hand, optional, and restarts every year — so a
	// double-submitted form produces two meeting cards and nothing in the database objects.
	//
	// MoKhiHong AND NOT DongKhiHong, with the residual risk stated rather than glossed: recording
	// minutes is an INTAKE path, not one of the acts with legal consequence the skill reserves
	// DongKhiHong for (money, an issued document number, closing a commitment to a citizen). While
	// the cache is down a double submit leaves a duplicate card — visible, and one somebody must
	// live with until §6's delete route exists, because rule 7 forbids removing it with a command.
	// Refusing the whole register mid-morning would be the larger harm.
	//
	// @summary  Nhập một biên bản họp, kèm các kết luận đã gõ trên biểu mẫu — kết luận đánh số ① ② ③ theo thứ tự nhập
	// @screen   04-bien-ban-hop §4
	// @request  taoBienBanVao
	// @reply    201 bienBanRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/meetings",
		authz.RequirePermission(d.Checker, "task.create")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.TaoBienBan))))

	// THÊM MỘT KẾT LUẬN (§2's last row) — `task.create`.
	//
	// idem.Required(MoKhiHong) FOR THE SAME REASON AND WITH THE SAME GAP: the unique key
	// `(tenant_id, bien_ban_id, thu_tu)` does NOT protect this route, because the second request
	// mints the NEXT ordinal rather than colliding with the first. Two identical conclusions, ③ and
	// ④, are both perfectly valid to the database — which is exactly the shape an idempotency key
	// exists for.
	//
	// 404 COVERS THE MINUTES NOT EXISTING, another commune's minutes, and minutes that have been
	// removed. Telling them apart tells a caller which minutes exist in a register they are not
	// reading.
	//
	// @summary  Thêm một kết luận vào biên bản đã có — số thứ tự nối tiếp số ĐÃ CẤP, kể cả khi kết luận mang số đó đã bị xoá
	// @screen   04-bien-ban-hop §2, §7.2
	// @request  themKetLuanVao
	// @reply    201 ketLuanRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/meetings/{id}/conclusions",
		authz.RequirePermission(d.Checker, "task.create")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.ThemKetLuan))))

	// TÁCH KẾT LUẬN THÀNH NHIỆM VỤ (§3) — `task.create`, the same key POST /api/v1/tasks declares,
	// because this IS that act: it runs the same use case and mints from the same `NV…` series.
	//
	// ⚠ THE BODY IS THE FULL TASK FORM AND THE SERVER DERIVES NOTHING FROM THE CONCLUSION'S
	// SENTENCE. §3 pre-fills the "Giao việc mới" modal and waits for a person to confirm it; the
	// sibling implementation measured what guessing costs — right three times out of four, and the
	// fourth leaves a wrongly titled task in a register that cannot be deleted, only withdrawn.
	//
	// ⚠ `source` / `source_id` ARE NOT ON THE WIRE. §3 shows "Nguồn giao = Từ kết luận họp" as
	// locked, and here that is the pair being ABSENT from the request rather than validated on it:
	// the use case sets both from the conclusion named in the PATH, so there is no value a client
	// could send to point a task at a record of its choosing.
	//
	// idem.Required(MoKhiHong), the same declaration POST /api/v1/tasks carries and for the same
	// reason: `UNIQUE (tenant_id, ma)` cannot see anything wrong with two tasks minted under two
	// DIFFERENT numbers for one piece of work, and §3 permits a conclusion to be split many times —
	// so a duplicate is indistinguishable from an intended second split without the key.
	//
	// 409 COVERS TWO DIFFERENT THINGS, exactly as on POST /api/v1/tasks: `code_taken` and
	// `task_tree`. They are mapped by the task register's own function, so one act answers one way
	// whichever door it came through.
	//
	// @summary  Tách một kết luận họp thành một nhiệm vụ — nhiệm vụ giữ liên kết ngược về kết luận gốc qua cặp nguồn giao
	// @screen   04-bien-ban-hop §3
	// @request  tachKetLuanVao
	// @reply    201 nhiemVuRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/meetings/{id}/conclusions/{stt}/task",
		authz.RequirePermission(d.Checker, "task.create")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.TachKetLuanThanhNhiemVu))))

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
