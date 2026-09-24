package app

// The four STAFF acts on a petition — classify, assign, advance, close.
//
// Until today a citizen could file a petition and look it up, and NO MEMBER OF STAFF COULD DO
// ANYTHING WITH IT. A petition that arrives and never moves is the failure rule 10 is about: the
// citizen cannot tell "being processed" from "ignored", and a channel nobody trusts stops receiving
// the reports the commune actually needs.
//
// # WHY THIS LAYER EXISTS
//
// Rule 6, invariant 3 requires the audit entry to share a transaction with the business write, and
// core/audit.Write takes a *store.ScopedTx with no overload that writes outside one. Opening that
// transaction is this layer's job. The handler translates HTTP and nothing else; the store knows SQL
// and nothing else.
//
// THE SECOND REASON IS THE THIRD WRITE. Every act below produces THREE writes that must stand or
// fall together:
//
//	the petition moves           `phieu_phan_anh`
//	the trail records who        `audit_log`        rule 6, invariant 3
//	the citizen is owed a word   `su_kien_di`       rule 10, invariant 5
//
// The third is the one that is easy to get wrong and impossible to notice: publishing the
// notification after the commit, with nothing recorded, loses it permanently if the process dies in
// the window — and nothing anywhere then says a citizen was never told. Recording the obligation
// inside the transaction is the only shape in which "every status change notifies the citizen"
// survives a crash. See KhoSuKien.
//
// ---------------------------------------------------------------------------
// THE DATABASE IS THE FLOOR AND THIS LAYER IS THE SENTENCE. Said once, here:
//
//	the nine statuses           migration 0004, `phieu_phan_anh_trang_thai_hop_le`
//	the lookup code is immutable        0004, trigger `ho_so_luu_tru_bat_bien`
//	`goc_dem_han` is immutable          0004, same trigger — which is why the deadline this layer
//	                                    computes outside the transaction cannot go stale inside it
//	hard removal refused outright       0004, same trigger
//	a deadline is never before its origin  0004 + 0005, the two `han_… >= goc_dem_han` CHECKs
//
// Those hold against every writer — this service, a psql prompt, an import job written next year.
// What they cannot do is explain themselves, and they cannot express the lifecycle at all. This
// layer refuses FIRST, in Vietnamese. A drift between the two is therefore a worse error message,
// never a hole.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/vihat/vigov/core/audit"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	petitionsv1 "github.com/vihat/vigov/core/gen/vigov/petitions/v1"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// KhoPhieuXuLy is the write half of the register as the staff path needs it, declared at the point
// of use.
//
// EVERY METHOD TAKES THE TRANSACTION. That is what makes it impossible to move the petition in one
// transaction and record the trail in another: there is no signature here that would let you. It is
// also what makes the properties worth proving — that a refusal writes nothing, that the entry and
// the outbox row share the transaction, that the row is read under a lock — provable without a
// PostgreSQL, of which there is none reachable from this build environment (VIGOV_TEST_DSN unset).
//
// *petstore.PhieuPhanAnhStore satisfies it as it is; nothing was changed to accommodate this.
type KhoPhieuXuLy interface {
	TheoMaTraCuu(ctx context.Context, ma string) (domain.PhieuPhanAnh, error)
	TheoMaTraCuuDeSua(ctx context.Context, tx *store.ScopedTx, ma string) (domain.PhieuPhanAnh, error)
	ChotLinhVuc(ctx context.Context, tx *store.ScopedTx, id, linhVuc string,
		tuTrangThai, sangTrangThai domain.TrangThai, phanLoaiLuc, hanXuLyXong time.Time) error
	PhanCong(ctx context.Context, tx *store.ScopedTx, id, boPhanID, canBoID string,
		tuTrangThai, sangTrangThai domain.TrangThai) error
	DoiTrangThai(ctx context.Context, tx *store.ScopedTx, id string,
		tuTrangThai, sangTrangThai domain.TrangThai, xuLyXongLuc time.Time) error
	Dong(ctx context.Context, tx *store.ScopedTx, id string, tuTrangThai domain.TrangThai,
		ketQua string, dongLuc time.Time) error
	KhongTiepNhan(ctx context.Context, tx *store.ScopedTx, id string, tuTrangThai domain.TrangThai,
		lyDo string, luc time.Time) error
	ChuyenCapTren(ctx context.Context, tx *store.ScopedTx, id string, tuTrangThai domain.TrangThai,
		lyDo, coQuanNhan string, luc time.Time) error
}

// KhoSuKien records the obligation to tell the citizen, in the SAME transaction as the change.
//
// A SEPARATE INTERFACE FROM KhoPhieuXuLy AND NOT ONE MORE METHOD ON IT, because the two have
// different obligations and different lifetimes: everything above writes the archival record, this
// one writes infrastructure state that a relay will drain. Behind one interface a later caller would
// reach for whichever method was nearest, and the one it would reach for is the one that lets a
// status change commit with no notification recorded.
//
// ⚠ THE RELAY THAT DRAINS IT DOES NOT EXIST. There is no broker client anywhere in this repository —
// `core/events.Publisher` is a bare interface with no implementation, which service-comms' consumer
// states in its own header. So rows accumulate and no citizen is messaged yet. What this buys today
// is that every notification owed since the first petition is RECORDED, transactionally, in order,
// and is still there the day the relay runs. It is reported as a gap, not hidden as one.
type KhoSuKien interface {
	Chen(ctx context.Context, tx *store.ScopedTx, e petstore.SuKienDi) error
}

// DocHanXuLyXong asks identity for the instant this commune's RESOLVE commitment falls due.
//
// THE FULL identityclient.Client SIGNATURE, DELIBERATELY, rather than a narrow `Han(ctx, linhVuc, t)`
// that would read better here. Three of the four arguments are decisions ADR 0028 makes about THIS
// act — which work kind's hours, which field's row, which of the two clocks is being fixed — and a
// wrapper would move them out of this file into a piece of wiring nobody tests. Keeping them here is
// what lets the test assert that classification asks for `phan-anh`, for the FIELD JUST SETTLED, and
// for the resolve clock ALONE.
//
// IT IS A SECOND INTERFACE WITH THE SAME SHAPE AS HanTiepNhanDoc, on purpose. The intake use case
// asks for the DEFAULT row and the acknowledge clock; this one asks for a named field and the resolve
// clock. One shared interface would say the two are the same question, and the day somebody widens it
// for one caller the other silently inherits the change — which for these two would rebuild the
// 56-hour ceiling ADR 0028 removed.
//
// *identityclient.Client satisfies this as it is.
type DocHanXuLyXong interface {
	HanXuLy(ctx context.Context, loaiViec identityv1.WorkKind, linhVuc string, tuLuc time.Time,
		can []identityv1.DeadlineKind) (map[identityv1.DeadlineKind]time.Time, error)
}

// KiemCanBoGiaoViec asks identity which staff codes may be handed NEW work in the commune the context
// carries (identity.ResolveAssignableStaff: not deleted, has an account, not locked — the predicate
// of the assignee picker).
//
// WHY THE ASSIGNMENT ACT ASKS AT ALL (user decision 2026-09-25): the assignee code arrives in the
// request BODY, so it is client-supplied. Written unchecked, a crafted body hands a citizen's
// petition to a code of another commune, a locked former employee or a directory-only person — and
// every later holder check (duocTienTrangThai) compares the stored code with a signed-in
// Principal.Ma that will never exist, so the work sits with nobody, forever.
//
// *identityclient.Client satisfies this as it is.
type KiemCanBoGiaoViec interface {
	CanBoGiaoViecDuoc(ctx context.Context, ma []string) (map[string]struct{}, error)
}

// ErrCanBoKhongNhanDuocViec refuses an assignment whose officer code identity did not answer as
// assignable.
//
// ONE ERROR FOR FIVE REASONS — unknown, deleted, no account, locked, another commune — because the
// contract collapses them into one answer ("absent") and the user sees one sentence. Telling "another
// commune" from "unknown" would leak that a code exists elsewhere (rule 1); telling "locked" apart
// would publish an employment fact about a person.
var ErrCanBoKhongNhanDuocViec = errors.New("xu_ly_phan_anh: cán bộ được chọn không nhận được việc")

// ErrChuaKiemDuocCanBo means identity could not be asked, so the assignment was NOT written.
//
// IT IS NEVER "not assignable" AND NEVER "assignable" (identity.proto, ResolveAssignableStaff status
// table): the check did not happen. Falling back to writing the code unchecked would re-open exactly
// the hole the check closes, on the day identity is down. Retryable — the handler answers 503.
var ErrChuaKiemDuocCanBo = errors.New("xu_ly_phan_anh: chưa kiểm được cán bộ nhận việc")

// The business verbs written into the trail. Vietnamese snake_case, like every other action this
// system already writes (`dang_nhap`, `cong_dan_gui_phan_anh`, `vao_so_van_ban_den`) — an inspection
// reads these strings, and a function name would tell them nothing.
//
// FOUR VERBS AND NOT ONE `sua_phan_anh`, because they are four different administrative acts with
// four different permissions. `phan_loai_phan_anh` in particular is the one an inspection searches
// for by name: it is the act that ISSUED THE COMMUNE'S PROMISE about when the matter would be
// settled, and the entry carries the deadline it fixed.
const (
	HanhViPhanLoaiPhanAnh  = "phan_loai_phan_anh"
	HanhViPhanCongPhanAnh  = "phan_cong_phan_anh"
	HanhViChuyenTrangPhieu = "chuyen_trang_thai_phan_anh"
	HanhViDongPhanAnh      = "dong_phan_anh"
	// The two terminal branches. Two verbs and not one `ket_thuc_nhanh`: an inspection asks "which
	// petitions did this commune REFUSE" and "which did it pass on" as two different questions.
	HanhViKhongTiepNhanPhanAnh = "khong_tiep_nhan_phan_anh"
	HanhViChuyenCapTrenPhanAnh = "chuyen_cap_tren_phan_anh"
	tenSuKienDoiTrangThai      = "petitions.status_changed.v1"
	chuThePhaiLaCanBo          = "staff"
	loiThieuChuThe             = "xu_ly_phan_anh: thiếu mã cán bộ thực hiện"
	loiChuTheKhongPhaiCanBo    = "xu_ly_phan_anh: chủ thể không phải cán bộ"
)

// --- the holding rule: who may move a petition along ----------------------------------------------

// QuyenXuLyCaXa is the answer to ONE question, asked at the edge and carried down as a FACT rather
// than as a decision: does this principal hold `feedback.resolve`, the commune-wide right to work on
// ANY petition of this commune?
//
// IT IS A NAMED TYPE AND NOT A BARE bool, because a bare bool at a call site reads as nothing at all
// and the wrong literal there opens every petition in the commune to every account holding
// `feedback.read`.
//
// THE DECISION IS NOT MADE WHERE THIS VALUE IS PRODUCED. The handler may only answer "does this
// person hold the key"; whether the act is allowed is duocTienTrangThai's, below, in this layer —
// which is what keeps the rule provable without an HTTP request and impossible to bypass by adding a
// second caller.
type QuyenXuLyCaXa bool

// ErrKhongPhaiNguoiDuocGiao refuses an advance by somebody who neither holds the commune-wide right
// nor is the officer this petition was handed to.
//
// # WHY THIS BRANCH EXISTS AT ALL — the owner decided it on 2026-09-23 ("theo require")
//
// `feedback.resolve` is the right to work on EVERY petition of the commune. A hamlet leader handed
// one petition about their own hamlet needs to move it along, and granting them the commune-wide key
// just for that would let them close the neighbouring hamlet's petitions too. So the ROUTE declares
// `feedback.read` (rule 5, invariant 1 — an explicit key, never AnyAuthenticated) and the real
// condition is this: the commune-wide right OR being the named assignee.
//
// ⚠ IT WIDENS THE WORKING PATH AND NOT THE CLOSING PATH. `Dong` deliberately has no such branch:
// closing records a RESULT THE CITIZEN READS (rule 10, invariant 6) and is the act open question #7
// settled on 2026-09-16 — "`feedback.resolve` quyết định ai đóng được". The five routes the other
// repository lowered are all WORKING routes; none of them is the closing. Widening `Dong` with this
// rule would silently overturn a decision the customer made.
var ErrKhongPhaiNguoiDuocGiao = errors.New(
	"xu_ly_phan_anh: phiếu này không được giao cho người thực hiện, và người thực hiện không có " +
		"quyền xử lý phiếu của cả xã")

// duocTienTrangThai decides the holding rule. Called INSIDE the transaction, on the row read under
// the lock, so the assignee it compares against is the one the database holds now — not one read
// before another officer reassigned the petition.
//
// # THE COMPARISON IS BETWEEN TWO STAFF BUSINESS CODES, AND THAT IS THE WHOLE CORRECTNESS OF IT
//
// `phieu_phan_anh.can_bo_xu_ly_id` holds a staff BUSINESS CODE (`CB-00123` — domain.CanBoToiDa says
// so, and it is the same value the assignment act writes), and audit.Actor.ID holds the same kind of
// value for a staff actor (rule 6, invariant 8; the handler builds it from authz.Principal.Ma).
// Comparing an internal id (`authz.Principal.ID`, a ULID) against this column compares two DIFFERENT
// KINDS of identifier: it never matches, every officer falls into the "not the assignee" branch, and
// the feature simply does not work — with nothing red anywhere, because both values are non-empty
// strings that look plausible.
//
// # AN UNASSIGNED PETITION MATCHES NOBODY, AND THAT IS CHECKED EXPLICITLY
//
// `can_bo_xu_ly_id` is NULL — empty here — whenever a department was told to assign internally
// ("— Để bộ phận phân công —" is a real answer on the screen). An empty actor code can also reach
// this layer if identity is older than the `ma` field. Without the two guards below, `"" == ""` would
// be true and EVERY account holding `feedback.read` could advance EVERY unassigned petition in the
// commune — the widest possible failure, produced by the narrowest possible omission.
func duocTienTrangThai(p domain.PhieuPhanAnh, nguoi audit.Actor, quyen QuyenXuLyCaXa) error {
	if quyen {
		return nil
	}
	if nguoi.ID == "" || p.CanBoXuLyID == "" {
		return ErrKhongPhaiNguoiDuocGiao
	}
	if nguoi.ID != p.CanBoXuLyID {
		return ErrKhongPhaiNguoiDuocGiao
	}
	return nil
}

// --- the restricted field: a report ABOUT a member of staff ---------------------------------------

// QuyenXemHanChe is the answer to ONE question, asked at the edge and carried down as a FACT rather
// than as a decision: does this principal hold `feedback.restricted`, the key that opens the field
// `can-bo` — "Thái độ / tác phong cán bộ", which is a report ABOUT a member of staff?
//
// IT IS A NAMED TYPE AND NOT A BARE bool, for the same reason QuyenXuLyCaXa is one: a bare bool at a
// call site reads as nothing at all, and the wrong literal there opens precisely what this
// restriction protects — the ordinary readers of this register are the COLLEAGUES of the person
// being reported on.
//
// THE DECISION IS NOT MADE WHERE THIS VALUE IS PRODUCED. The handler may only answer "does this
// account hold the key"; whether the act is allowed is duocChamPhieuHanChe's, below.
type QuyenXemHanChe bool

// ErrPhieuHanChe refuses a staff act on a petition in `can-bo` by somebody without
// `feedback.restricted`.
//
// # WHY THE ACT IS REFUSED AND NOT MERELY THE RESPONSE BODY
//
// Withholding the reply would leave the thing that actually matters untouched: a colleague of the
// person being reported on would still be CLASSIFYING, ASSIGNING, ADVANCING and CLOSING the
// complaint about them. The READ path has refused such a caller since it was written
// (internal/http/phieu_phan_anh.go, and the list excludes them inside the WHERE clause); the four
// WRITE paths did not, so one successful POST returned the whole record to an account holding
// `feedback.resolve` and nothing else.
//
// # IT BECOMES A 404 AND NOT A 403 — the same sentence the read path already says
//
// A 403 here would confirm that a report about a member of staff exists under this code, TO A
// COLLEAGUE OF THAT PERSON, and the existence of such a report is exactly what the restriction
// protects. Fail closed, and answer the same thing an unknown code answers. The mapping lives at
// internal/http.traLoiLoiXuLy, beside the other refusals.
//
// ⚠ THE CONSEQUENCE, STATED RATHER THAN BURIED: in a commune where NOBODY holds
// `feedback.restricted`, a `can-bo` petition can be processed by nobody at all. That is fail closed
// and it is the right direction — it has been true of the READ path from the first day (no such
// account can list one or open one), and the alternative is a petition somebody may act on but
// nobody may read. It is a configuration finding for the commune, not a defect to patch out here.
var ErrPhieuHanChe = errors.New(
	"xu_ly_phan_anh: phiếu thuộc lĩnh vực hạn chế và người thực hiện không có quyền xem lĩnh vực đó")

// duocChamPhieuHanChe decides it, on the row that was just read.
//
// CALLED INSIDE THE TRANSACTION, ON THE LOCKED ROW, for the reason duocTienTrangThai gives and one
// more of its own: the field is a property of the row, so it is knowable only after the read, and a
// classification racing this act can make a petition restricted in exactly the window between an
// unlocked read and the UPDATE. Refusing here writes NOTHING — no UPDATE, no audit entry, no outbox
// row — and the transaction rolls back empty.
//
// IT COMPARES THE FIELD THE ROW HOLDS, which is the same value the read path compares
// (internal/http/phieu_phan_anh.go) — one rule, one source, and the write path following the read
// path rather than the other way round.
//
// ⚠ WHAT IT DELIBERATELY DOES NOT LOOK AT: the field an officer is SETTING in this request.
// Classifying a petition INTO `can-bo` discloses nothing — the row was readable to that officer a
// moment earlier, unrestricted — and refusing it would mean nobody but a holder of the key could
// ever mark a report as being about a colleague. Whether that act should itself need the key is the
// owner's call and is raised as a finding, not decided here.
func duocChamPhieuHanChe(p domain.PhieuPhanAnh, quyen QuyenXemHanChe) error {
	if p.LinhVuc == domain.LinhVucHanChe && !quyen {
		return ErrPhieuHanChe
	}
	return nil
}

// ErrChuaAnDinhDuocHanXuLy means the commune's resolve commitment could not be established, so the
// petition was NOT classified.
//
// # THIS IS THE MOST CONSEQUENTIAL REFUSAL ON THE STAFF PATH AND IT IS DELIBERATE
//
// `sla` is EMPTY FOR EVERY COMMUNE until somebody fills it in: service-identity's migration 0008
// seeds nothing and the onboarding step does not exist in this repository. So until then, classifying
// answers 409 and settles nothing. That is the contract working, not a bug to route around
// (core/identityclient.HanXuLy states it in full):
//
//	DO NOT fall back to a number. Not 56 hours, not the specification's own table. A commitment
//	invented by software is still told to a citizen as though the authority made it (rule 10,
//	forbidden #3) — and unlike a document, a petition's deadline is shown to the person waiting.
//
//	DO NOT settle the field without the deadline. `linh_vuc` and `han_xu_ly_xong` are set by ONE
//	statement precisely so that cannot happen: a petition classified with no resolve commitment would
//	sit with a NULL nothing will ever come back to fill, because the step that would have is the one
//	that just ran.
//
//	DO NOT add hours here. hooks/citizen_commitment_guard blocks the shape outside service-identity,
//	and rule 10, forbidden #2 is why: adding a duration walks straight through nights, weekends,
//	`ngay_nghi_le` and `ngay_lam_bu` while looking like it respected the unit.
var ErrChuaAnDinhDuocHanXuLy = errors.New(
	"xu_ly_phan_anh: xã chưa cấu hình thời hạn xử lý cho lĩnh vực này nên chưa phân loại được")

// YeuCauChotLinhVuc is one classification as it arrives from the handler.
//
// ONE FIELD, AND THE ABSENCES ARE THE DESIGN. There is no `Status`: the act moves the petition to
// `dang-phan-loai` and nowhere else. There is no `DueAt`: the deadline is this commune's SLA and this
// commune's calendar, computed by identity — a client choosing its own deadline is a client choosing
// how long the authority may take. There is no `ClockFrom`: the origin is `goc_dem_han`, which the
// trigger refuses to change.
type YeuCauChotLinhVuc struct {
	LinhVuc string
}

// YeuCauPhanCong is the "Chuyển xử lý" block of docs/ui-ux/09 §8.5.
//
// `CanBo` IS OPTIONAL because the screen says so: "— Để bộ phận phân công —" is a real choice, and it
// means the department decides internally who takes it. `BoPhan` is NOT optional — see
// domain.ErrThieuBoPhan.
type YeuCauPhanCong struct {
	BoPhan string
	CanBo  string
}

// XuLyPhanAnh owns the four staff acts.
type XuLyPhanAnh struct {
	db     *store.DB
	kho    KhoPhieuXuLy
	suKien KhoSuKien
	han    DocHanXuLyXong

	// giaoViec verifies a client-supplied assignee code before PhanCong writes it. nil means the
	// check cannot run, and PhanCong then REFUSES any assignment naming an officer (fail closed).
	giaoViec KiemCanBoGiaoViec

	// sinhID is injected so a test can pin the outbox row's id. In production it is ulid.Moi.
	sinhID func() (string, error)

	// nay is the clock every recorded instant comes from — `phan_loai_luc`, `xu_ly_xong_luc`,
	// `dong_luc` and the event's `occurred_at`.
	//
	// A SEAM AND NOT time.Now() AT THE CALL SITE, for a reason about evidence rather than
	// convenience: `phan_loai_luc` is the instant the ACKNOWLEDGE clock and the CLASSIFICATION
	// CEILING are both measured against, so a test that cannot pin it cannot assert whether a
	// petition was acknowledged in time. In production this is nil and nayHoac returns the real clock.
	nay func() time.Time
}

func NewXuLyPhanAnh(db *store.DB, kho KhoPhieuXuLy, suKien KhoSuKien, han DocHanXuLyXong,
	giaoViec KiemCanBoGiaoViec) *XuLyPhanAnh {
	return &XuLyPhanAnh{db: db, kho: kho, suKien: suKien, han: han, giaoViec: giaoViec, sinhID: ulid.Moi}
}

// nayHoac is the clock, UTC. `TIMESTAMPTZ` stores an instant rather than a wall reading, so the
// location changes nothing that is stored — it is fixed so a value read back in a test compares equal
// without a location dance.
func (uc *XuLyPhanAnh) nayHoac() time.Time {
	if uc.nay == nil {
		return time.Now().UTC()
	}
	return uc.nay().UTC()
}

// --- 1. classification — the act that fixes the commune's promise ---------------------------------

// ChotLinhVuc settles which field the petition belongs to, stops the acknowledge clock, and FIXES
// `han_xu_ly_xong` (ADR 0028, decision E). Permission: `feedback.classify`.
//
// # THE DEADLINE IS FETCHED BEFORE THE TRANSACTION OPENS, AND THAT IS NOT A STYLE CHOICE
//
// It is a gRPC call to another service. Made inside the transaction it would hold this petition's row
// lock for a network round trip, and an identity outage would become a register that hangs rather
// than one that refuses.
//
// WHAT MAKES IT SAFE TO COMPUTE OUTSIDE THE LOCK, which is the part a reviewer should check rather
// than assume: the deadline is a function of `goc_dem_han` and the field the officer just chose.
// `goc_dem_han` is IMMUTABLE — migration 0004's `ho_so_luu_tru_bat_bien` refuses to change it — so
// the value cannot go stale between the two reads. What CAN change in that window is the STATUS, and
// that is caught twice: re-read under the lock, and the UPDATE itself carries `trang_thai = $tu`, so
// the loser of a race writes nothing and is told so.
//
// # ONLY THE RESOLVE CLOCK IS ASKED FOR
//
// `han_tiep_nhan` was fixed when the row was created and is never recomputed (rule 10, invariant 2),
// and `han_phan_loai` likewise. Asking for the acknowledge clock here would produce a second answer
// to a question already answered — and stored.
func (uc *XuLyPhanAnh) ChotLinhVuc(ctx context.Context, ma string, yc YeuCauChotLinhVuc,
	nguoi audit.Actor, hanChe QuyenXemHanChe) (domain.PhieuPhanAnh, error) {

	linhVuc, err := domain.KiemLinhVuc(yc.LinhVuc)
	if err != nil {
		return domain.PhieuPhanAnh{}, err
	}
	if err := coCanBoThucHien(nguoi); err != nil {
		return domain.PhieuPhanAnh{}, err
	}

	// READ ONCE WITHOUT THE LOCK, only to learn the origin and to refuse early. A petition that is
	// not waiting to be classified must not become load on identity.
	truoc, err := uc.kho.TheoMaTraCuu(ctx, ma)
	if err != nil {
		return domain.PhieuPhanAnh{}, bocPhieu(ctx, "phân loại", err)
	}

	// THE RESTRICTED FIELD, BEFORE THE LIFECYCLE CHECK AND BEFORE IDENTITY IS ASKED — and the ORDER
	// is the correctness here, not the check. The refusal below this line is a 409 saying "this
	// petition has already moved", which is a statement ABOUT THE RECORD: answering it to somebody
	// who may not touch the record at all confirms that a report about a member of staff exists under
	// this code. The same argument TienTrangThai makes for putting the holding rule before its
	// lifecycle check.
	//
	// It also keeps a refused act off identity's RPC, which is the lesser reason and is why it is
	// second in this comment.
	//
	// THIS IS NOT THE CHECK THAT DECIDES. The row was read WITHOUT the lock, so it may already be
	// stale; the one that decides is inside the transaction, below, on the locked row.
	if err := duocChamPhieuHanChe(truoc, hanChe); err != nil {
		return domain.PhieuPhanAnh{}, bocPhieu(ctx, "phân loại", err)
	}

	if !truoc.TrangThai.ChuyenSangDuoc(domain.DangPhanLoai) {
		// The petition has already been read by somebody, or it has gone down a branch. Re-settling
		// the field AFTER the first time is a different act — ADR 0027 decision C makes the deadline
		// only ever SHORTEN from then on — and it is not built. See the report.
		return domain.PhieuPhanAnh{}, bocPhieu(ctx, "phân loại", petstore.ErrPhieuDaChuyenTrang)
	}

	han, err := uc.hanXuLyXong(ctx, linhVuc, truoc.GocDemHan)
	if err != nil {
		return domain.PhieuPhanAnh{}, err
	}

	bayGio := uc.nayHoac()
	var sau domain.PhieuPhanAnh

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		p, err := uc.kho.TheoMaTraCuuDeSua(ctx, tx, ma)
		if err != nil {
			return err
		}
		// THE CHECK THAT DECIDES, on the locked row. The unlocked one above refuses early and asks
		// identity nothing; this one is the one that cannot be raced, because between the two reads
		// another officer can classify the petition INTO `can-bo`.
		if err := duocChamPhieuHanChe(p, hanChe); err != nil {
			return err
		}
		if !p.TrangThai.ChuyenSangDuoc(domain.DangPhanLoai) {
			return petstore.ErrPhieuDaChuyenTrang
		}

		// THE EARLIER OF THE TWO, ALWAYS (ADR 0027 decision C). On a FIRST settling `p.HanXuLyXong`
		// is zero and HanSomHon returns the new one — which is the ADR 0028 decision E case, and the
		// reason HanSomHon has that branch at all. The call is here rather than "when re-classification
		// is built" so the rule is wired and tested from the first day, not retrofitted onto a column
		// that already carries promises.
		hanChot := domain.HanSomHon(p.HanXuLyXong, han)

		if err := uc.kho.ChotLinhVuc(ctx, tx, p.ID, linhVuc,
			p.TrangThai, domain.DangPhanLoai, bayGio, hanChot); err != nil {
			return err
		}

		sau = p
		sau.LinhVuc = linhVuc
		sau.TrangThai = domain.DangPhanLoai
		sau.PhanLoaiLuc = bayGio
		sau.HanXuLyXong = hanChot

		// BEFORE AND AFTER, INCLUDING THE DEADLINE THIS ACT FIXED (rule 6, invariant 5). The deadline
		// is the whole reason this entry matters: it is the moment the authority committed to a date,
		// and an inspection asking "when was this promised and by whom" has no other place to look.
		delta, err := json.Marshal(map[string]any{
			"truoc": map[string]any{
				"trang_thai":     string(p.TrangThai),
				"linh_vuc":       p.LinhVuc,
				"han_xu_ly_xong": lucRaVet(p.HanXuLyXong),
			},
			"sau": map[string]any{
				"trang_thai":     string(domain.DangPhanLoai),
				"linh_vuc":       linhVuc,
				"han_xu_ly_xong": lucRaVet(hanChot),
			},
			// The ceiling this act was measured against, and whether it was met. DERIVED at the
			// instant of the act and RECORDED — which is not the stored flag rule 10, invariant 3
			// forbids: that forbids a COLUMN that is written and then read as truth later. An audit
			// entry states what was true at one moment and is never read as the current state.
			"tre_tran_phan_loai": sau.QuaHanPhanLoai(bayGio),
		})
		if err != nil {
			return fmt.Errorf("xu_ly_phan_anh: mã hoá delta: %w", err)
		}
		if err := audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViPhanLoaiPhanAnh,
			Subject: p.MaTraCuu,
			Delta:   delta,
		}); err != nil {
			return err
		}

		// CLASSIFICATION OWES THE CITIZEN NOTHING, and the event still goes out. It is an INTERNAL
		// step (owner's decision of 2026-09-24; rule 4, forbidden #5 keeps routing history away from the
		// citizen), so domain.ViecTiepTheo returns "" and the message carries no `citizen_message`.
		// The FACT is published anyway because the event is named after the fact — a consumer counting
		// time-in-status must not discover that three of the nine transitions were never emitted.
		return uc.ghiSuKien(ctx, tx, sau, domain.DangPhanLoai, bayGio)
	})
	if err != nil {
		return domain.PhieuPhanAnh{}, bocPhieu(ctx, "phân loại", err)
	}
	return sau, nil
}

// hanXuLyXong asks identity for the one clock this act fixes.
func (uc *XuLyPhanAnh) hanXuLyXong(ctx context.Context, linhVuc string, gocDemHan time.Time) (
	time.Time, error) {

	han, err := uc.han.HanXuLy(ctx, identityv1.WorkKind_WORK_KIND_PHAN_ANH, linhVuc, gocDemHan,
		[]identityv1.DeadlineKind{identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG})
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %w", ErrChuaAnDinhDuocHanXuLy, err)
	}
	t, co := han[identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG]
	if !co || t.IsZero() {
		// identityclient already refuses a partial answer; this is the second wall, and it is here
		// because a zero time.Time reaching the column is a deadline in the year 1 — a petition
		// overdue the moment it was classified, in front of the citizen who is watching it.
		return time.Time{}, fmt.Errorf("%w: hạn xử lý xong rỗng", ErrChuaAnDinhDuocHanXuLy)
	}
	return t, nil
}

// --- 2. assignment ---------------------------------------------------------------------------------

// PhanCong names the department — and optionally the officer — answerable for the petition.
// Permission: `feedback.assign`.
//
// IT IS A DIFFERENT RIGHT FROM CLASSIFICATION AND THE KEYS SAY SO (rule 5, invariant 3b, and
// ADR 0030 in its own words): being handed the work is not being allowed to PROMISE on behalf of the
// authority. `feedback.classify` issues the commitment; this one distributes the work.
//
// WHETHER IT MOVES THE STATUS depends on where the petition is — see domain.SauKhiPhanCong, which
// carries the reasoning and the ⚠ about reconciling §8.5 with ADR 0027's map.
func (uc *XuLyPhanAnh) PhanCong(ctx context.Context, ma string, yc YeuCauPhanCong,
	nguoi audit.Actor, hanChe QuyenXemHanChe) (domain.PhieuPhanAnh, error) {

	boPhan, canBo, err := domain.KiemPhanCong(yc.BoPhan, yc.CanBo)
	if err != nil {
		return domain.PhieuPhanAnh{}, err
	}
	if err := coCanBoThucHien(nguoi); err != nil {
		return domain.PhieuPhanAnh{}, err
	}
	if err := uc.kiemCanBoNhanViec(ctx, canBo); err != nil {
		return domain.PhieuPhanAnh{}, err
	}

	bayGio := uc.nayHoac()
	var sau domain.PhieuPhanAnh

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		p, err := uc.kho.TheoMaTraCuuDeSua(ctx, tx, ma)
		if err != nil {
			return err
		}
		// BEFORE THE LIFECYCLE CHECK. Handing a report about a member of staff to a department is the
		// act that decides who reads it next, and a caller who may not see the petition may not decide
		// that. Refusing first also keeps the answer identical to an unknown code — a 409 about the
		// state would say the record exists.
		if err := duocChamPhieuHanChe(p, hanChe); err != nil {
			return err
		}
		sangTrangThai, err := domain.SauKhiPhanCong(p.TrangThai)
		if err != nil {
			return err
		}
		if err := uc.kho.PhanCong(ctx, tx, p.ID, boPhan, canBo, p.TrangThai, sangTrangThai); err != nil {
			return err
		}

		sau = p
		sau.BoPhanID = boPhan
		sau.CanBoXuLyID = canBo
		sau.TrangThai = sangTrangThai

		// THE DELTA NAMES A DEPARTMENT AND A STAFF BUSINESS CODE, WHICH ARE NOT CITIZEN PERSONAL DATA
		// (rule 3 is about the people a commune serves, and rule 6, invariant 2 requires the entry to
		// say who was made responsible). Nothing about the reporter is in it.
		delta, err := json.Marshal(map[string]any{
			"truoc": map[string]any{
				"trang_thai":      string(p.TrangThai),
				"bo_phan_id":      p.BoPhanID,
				"can_bo_xu_ly_id": p.CanBoXuLyID,
			},
			"sau": map[string]any{
				"trang_thai":      string(sangTrangThai),
				"bo_phan_id":      boPhan,
				"can_bo_xu_ly_id": canBo,
			},
		})
		if err != nil {
			return fmt.Errorf("xu_ly_phan_anh: mã hoá delta: %w", err)
		}
		if err := audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViPhanCongPhanAnh,
			Subject: p.MaTraCuu,
			Delta:   delta,
		}); err != nil {
			return err
		}

		if sangTrangThai == p.TrangThai {
			// A RE-ASSIGNMENT MOVED NOTHING THE CITIZEN CAN SEE, so there is no status-change fact to
			// publish. The event is `status_changed`; emitting one where no status changed would put a
			// lie on the wire, and a consumer counting time-in-status would count a transition that
			// did not happen.
			return nil
		}
		return uc.ghiSuKien(ctx, tx, sau, sangTrangThai, bayGio)
	})
	if err != nil {
		return domain.PhieuPhanAnh{}, bocPhieu(ctx, "phân công", err)
	}
	return sau, nil
}

// kiemCanBoNhanViec verifies the officer code an assignment is about to write.
//
// BEFORE THE TRANSACTION, for the reason ChotLinhVuc asks identity outside it: a gRPC round trip
// inside would hold the petition's row lock for a network call, and an identity outage would become
// a register that hangs rather than one that refuses. The window it leaves — an account locked
// between this answer and the commit — is the one the contract accepts: the lock itself stops the
// person acting, and the holding rule runs again against a live principal on every later act.
//
// A UNIT-ONLY ASSIGNMENT ASKS NOTHING. "— Để bộ phận tự phân công —" names no officer, so there is
// no client-supplied code to verify, and making the department-only path depend on identity's
// health would refuse a valid act for no reason.
//
// THE COMMUNE TRAVELS IN THE CONTEXT (rule 1, invariant 4) — core/identityclient lifts it into
// "x-tenant-id", which is what makes another commune's code absent.
func (uc *XuLyPhanAnh) kiemCanBoNhanViec(ctx context.Context, canBo string) error {
	if canBo == "" {
		return nil
	}
	if uc.giaoViec == nil {
		// FAIL CLOSED: wiring without the check must refuse, never write the code unchecked.
		return fmt.Errorf("%w: chưa nối dây kiểm cán bộ giao việc", ErrChuaKiemDuocCanBo)
	}
	duoc, err := uc.giaoViec.CanBoGiaoViecDuoc(ctx, []string{canBo})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrChuaKiemDuocCanBo, err)
	}
	if _, co := duoc[canBo]; !co {
		return ErrCanBoKhongNhanDuocViec
	}
	return nil
}

// --- 3. moving along the main flow -------------------------------------------------------------------

// TienTrangThai advances the petition ONE step along the main flow. Route permission:
// `feedback.read`; the real condition is duocTienTrangThai — `feedback.resolve` OR being the named
// assignee.
//
// # ONE STEP, AND THE TARGET IS NOT A PARAMETER
//
// The caller says "advance", not "put it in `da-dong`". A target status on the wire is a client able
// to skip steps — to jump a petition from `da-chuyen-xu-ly` straight to `da-xu-ly` without anybody
// ever working on it — and every intermediate state would then be optional in practice while looking
// mandatory in the map.
//
// # WHY THE ROUTE'S KEY IS NOT THE WHOLE ANSWER HERE
//
// This route used to declare `feedback.resolve` and stop there, and the note in its place recorded
// that as a FINDING for open question #27: the `quyen` table has no key meaning "move the work
// along". The owner settled the working half on 2026-09-23 ("theo require") without adding a key:
// the route declares `feedback.read` — still an explicit declaration, so an account without it is
// refused at the gate — and this layer holds the condition that actually decides. No key was
// invented (rule 5, invariant 3c); both strings are seeded in `quyen`.
//
// ⚠ THE CLOSING ROUTE WAS NOT WIDENED WITH IT. See ErrKhongPhaiNguoiDuocGiao and Dong.
//
// # THE CHECK IS INSIDE THE TRANSACTION, ON THE LOCKED ROW
//
// Not on a read taken beforehand: between an unlocked read and the UPDATE, another officer can
// reassign the petition, and an authorisation decision made against a row that has since moved is an
// authorisation decision made against nothing. Refusing here writes no business row, no audit entry
// and no outbox row — the transaction rolls back with nothing in it.
func (uc *XuLyPhanAnh) TienTrangThai(ctx context.Context, ma string, nguoi audit.Actor,
	quyen QuyenXuLyCaXa, hanChe QuyenXemHanChe) (domain.PhieuPhanAnh, error) {

	if err := coCanBoThucHien(nguoi); err != nil {
		return domain.PhieuPhanAnh{}, err
	}

	bayGio := uc.nayHoac()
	var sau domain.PhieuPhanAnh

	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		p, err := uc.kho.TheoMaTraCuuDeSua(ctx, tx, ma)
		if err != nil {
			return err
		}

		// THE RESTRICTED FIELD BEFORE THE HOLDING RULE, AND BOTH BEFORE THE LIFECYCLE CHECK.
		//
		// This order is not arbitrary: the two refusals answer differently (404 for the field, 403 for
		// the holding rule), and the weaker disclosure has to win. Somebody who is the named assignee
		// of a `can-bo` petition but holds no `feedback.restricted` must be told the same thing an
		// unknown code is told — a 403 would say "there IS a report here about a colleague, you simply
		// may not move it".
		if err := duocChamPhieuHanChe(p, hanChe); err != nil {
			return err
		}

		// BEFORE THE LIFECYCLE CHECK, NOT AFTER. Somebody who may not act on this petition must not
		// learn from the answer which state it is in: a 409 saying "this petition has already moved"
		// is a statement about the record, and it is not owed to a caller who is about to be refused
		// anyway.
		if err := duocTienTrangThai(p, nguoi, quyen); err != nil {
			return err
		}

		sangTrangThai, co := domain.TienTrinhChinh(p.TrangThai)
		if !co {
			// `da-tiep-nhan` (classify first), `dang-phan-loai` (assign first), `cho-dan-xac-nhan`
			// (close, which is its own act and its own body), and the three terminal statuses.
			return domain.ErrKhongConCamKet
		}
		if !p.TrangThai.ChuyenSangDuoc(sangTrangThai) {
			// UNREACHABLE while tienTrinhChinh is a subset of the lifecycle map, and checked anyway:
			// the two are separate declarations, and the one that would drift is the narrow one.
			return domain.ErrKhongConCamKet
		}

		// `xu_ly_xong_luc` IS RECORDED ONLY ON THE STEP THAT FINISHES THE WORK. It is what
		// domain.QuaHan compares against once the petition is settled — the reason work finished late
		// STAYS late — so stamping it on any other step would freeze that comparison at the wrong
		// moment and change a figure that has already been reported upward.
		var xongLuc time.Time
		if sangTrangThai == domain.DaXuLy {
			xongLuc = bayGio
		}

		if err := uc.kho.DoiTrangThai(ctx, tx, p.ID, p.TrangThai, sangTrangThai, xongLuc); err != nil {
			return err
		}

		sau = p
		sau.TrangThai = sangTrangThai
		if !xongLuc.IsZero() {
			sau.XuLyXongLuc = xongLuc
		}

		delta, err := json.Marshal(map[string]any{
			"truoc": map[string]any{"trang_thai": string(p.TrangThai)},
			"sau":   map[string]any{"trang_thai": string(sangTrangThai)},
			// DERIVED AT THE INSTANT OF THE ACT AND RECORDED, never stored as a column (rule 10,
			// invariant 3). An inspection asking "was this finished on time" reads it from the entry
			// that recorded the finishing, not from a flag somebody could have refreshed since.
			"tre_han": sau.QuaHan(bayGio),
		})
		if err != nil {
			return fmt.Errorf("xu_ly_phan_anh: mã hoá delta: %w", err)
		}
		if err := audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViChuyenTrangPhieu,
			Subject: p.MaTraCuu,
			Delta:   delta,
		}); err != nil {
			return err
		}
		return uc.ghiSuKien(ctx, tx, sau, sangTrangThai, bayGio)
	})
	if err != nil {
		return domain.PhieuPhanAnh{}, bocPhieu(ctx, "chuyển trạng thái", err)
	}
	return sau, nil
}

// --- 4. closing ---------------------------------------------------------------------------------------

// Dong closes the petition, recording a result the CITIZEN can read. Permission: `feedback.resolve`.
//
// FROM `cho-dan-xac-nhan`, OR FROM `da-xu-ly` WHEN NOBODY CAN CONFIRM (no citizen account — owner's
// decision of 2026-09-24, domain.DongDuoc). The second case writes no outbox row, by the existing "no
// recipient, no row" rule of ghiSuKienDoiTrangThai, and its audit entry says the confirmation was
// skipped (`dong_khong_qua_xac_nhan`).
//
// ⚠ IT TAKES NO QuyenXuLyCaXa AND MUST NOT GROW ONE — and the QuyenXemHanChe it DOES take is not
// that parameter wearing another name. The two facts pull in opposite directions: QuyenXuLyCaXa can
// only ever WIDEN who may act, while QuyenXemHanChe can only ever NARROW it, and no value of it lets
// anybody close anything they could not close before. The holding rule of 2026-09-23 widened the
// WORKING path (TienTrangThai) and deliberately left this one alone: closing is what open question #7
// settled on 2026-09-16 — "`feedback.resolve` quyết định ai đóng được" — and the other repository's
// five lowered routes are all working routes, none of them the closing. The absence of the parameter
// is the enforcement: there is no value a caller could pass that would let an assignee without
// `feedback.resolve` close a petition, so the negative cannot be lost to a one-line edit that looks
// like consistency.
//
// THE RESULT IS MANDATORY AND THAT IS RULE 10, INVARIANT 6, NOT A FORM PREFERENCE: "closing a
// petition records a result the citizen can read. Never close silently." domain.KiemKetQua refuses an
// empty one and refuses the ones that are technically non-empty and say nothing — "ok", "xong",
// "đã xử lý" — because a citizen who is told only that their report was closed cannot tell being
// helped from being dismissed.
//
// ⚠ WHAT THIS CANNOT ENFORCE, SAID PLAINLY: docs/ui-ux/09 §14 rule 2 forbids closing without an
// "after" photograph, and ADR 0008 makes that a PER-COMMUNE flag `bat_buoc_anh_nghiem_thu` (default
// TRUE). Neither the `anh_phan_anh` table nor a store for the flags exists in this repository, so
// this act cannot check it. Writing a hardcoded TRUE would block every closing in every commune on a
// table that does not exist; writing FALSE would silently drop a rule the customer stated. Neither is
// chosen — it is reported as the gap it is.
func (uc *XuLyPhanAnh) Dong(ctx context.Context, ma, ketQuaTho string, nguoi audit.Actor,
	hanChe QuyenXemHanChe) (domain.PhieuPhanAnh, error) {

	ketQua, err := domain.KiemKetQua(ketQuaTho)
	if err != nil {
		return domain.PhieuPhanAnh{}, err
	}
	if err := coCanBoThucHien(nguoi); err != nil {
		return domain.PhieuPhanAnh{}, err
	}

	bayGio := uc.nayHoac()
	var sau domain.PhieuPhanAnh

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		p, err := uc.kho.TheoMaTraCuuDeSua(ctx, tx, ma)
		if err != nil {
			return err
		}
		// THE HEAVIEST OF THE FOUR TO GET WRONG, which is why the check is here as well and not only
		// on the three acts above: closing is what records the RESULT A CITIZEN READS (rule 10,
		// invariant 6) and ends the matter. A colleague of the person being reported on writing the
		// final word on a complaint about them is the whole reason `feedback.restricted` exists.
		if err := duocChamPhieuHanChe(p, hanChe); err != nil {
			return err
		}
		// ON THE LOCKED ROW, because the answer depends on the petition and not only its status: from
		// `da-xu-ly` a closing is allowed only when no citizen account stands behind the petition
		// (owner's decision of 2026-09-24; domain.DongDuoc carries the reasoning).
		//
		// `xu_ly_xong_luc` IS ALREADY SET on a petition in `da-xu-ly` — TienTrangThai stamps it on the
		// step that enters that status — so closing from there leaves the resolve clock where the work
		// actually finished, exactly as a closing from `cho-dan-xac-nhan` does.
		boQuaXacNhan, err := domain.DongDuoc(p)
		if err != nil {
			return err
		}
		if err := uc.kho.Dong(ctx, tx, p.ID, p.TrangThai, ketQua, bayGio); err != nil {
			return err
		}

		sau = p
		sau.TrangThai = domain.DaDong
		sau.KetQuaXuLy = ketQua
		sau.DongLuc = bayGio

		// THE RESULT TEXT IS NOT IN THE DELTA, AND ITS LENGTH IS.
		//
		// It is free text a member of staff typed about ONE citizen's case, and it will eventually
		// name the reporter, quote their complaint or give their address. `audit_log` is append-only
		// and never deleted (rule 6, invariant 4), so a copy there is a second permanent store of
		// citizen personal data — which is exactly rule 6, forbidden #4. The result itself lives in
		// `ket_qua_xu_ly`, on a row the archival trigger already protects from silent editing, so
		// nothing is lost: the entry says a result WAS recorded and when, and the record holds it.
		delta, err := json.Marshal(map[string]any{
			"truoc":           map[string]any{"trang_thai": string(p.TrangThai)},
			"sau":             map[string]any{"trang_thai": string(domain.DaDong)},
			"do_dai_ket_qua":  len([]rune(ketQua)),
			"tre_han":         sau.QuaHan(bayGio),
			"co_ket_qua_doc":  true,
			"dong_luc_da_ghi": lucRaVet(bayGio),
			// WHETHER THIS CLOSING SKIPPED THE CITIZEN'S CONFIRMATION. Always present, true or false,
			// so an inspection asking "which petitions were closed without anybody confirming" can
			// query one key rather than infer it from the `truoc` status.
			"dong_khong_qua_xac_nhan": boQuaXacNhan,
		})
		if err != nil {
			return fmt.Errorf("xu_ly_phan_anh: mã hoá delta: %w", err)
		}
		if err := audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViDongPhanAnh,
			Subject: p.MaTraCuu,
			Delta:   delta,
		}); err != nil {
			return err
		}
		return uc.ghiSuKien(ctx, tx, sau, domain.DaDong, bayGio)
	})
	if err != nil {
		return domain.PhieuPhanAnh{}, bocPhieu(ctx, "đóng phiếu", err)
	}
	return sau, nil
}

// --- 5. the two terminal branches ----------------------------------------------------------------------

// YeuCauChuyenCapTren is one referral as it arrives from the handler. Both fields are mandatory — see
// domain.KiemLyDoKetThucNhanh and domain.KiemCoQuanNhan.
type YeuCauChuyenCapTren struct {
	LyDo       string
	CoQuanNhan string
}

// KhongTiepNhan refuses the petition: the commune does not take it, and says why. Permission:
// `feedback.classify` (user decision 25/09/2026).
//
// # WHY THE CLASSIFY KEY AND NOT A NEW ONE
//
// Both branches leave from `dang-phan-loai` and from nowhere else (domain.KetThucNhanhDuoc). They are
// the two other ANSWERS to the question classification asks — "does the commune take this, and under
// which field" (docs/ui-ux/09 §8.2) — so they are outcomes of the act ADR 0030 gave its own key. No
// key was invented (rule 5, invariant 3c).
//
// # WHAT IT SHARES WITH Dong, AND WHY
//
// A mandatory text the citizen reads (rule 10, invariant 6 in spirit: never end silently), the text
// NEVER in the audit delta and NEVER on the event, the restricted-field refusal inside the
// transaction, and one transaction for the three writes.
func (uc *XuLyPhanAnh) KhongTiepNhan(ctx context.Context, ma, lyDoTho string, nguoi audit.Actor,
	hanChe QuyenXemHanChe) (domain.PhieuPhanAnh, error) {

	lyDo, err := domain.KiemLyDoKetThucNhanh(lyDoTho)
	if err != nil {
		return domain.PhieuPhanAnh{}, err
	}
	return uc.ketThucNhanh(ctx, ma, domain.KhongTiepNhan, lyDo, "", nguoi, hanChe)
}

// ChuyenCapTren refers the petition to another body: the commune stops owning it, names who received
// it, and says why. Permission: `feedback.classify` — see KhongTiepNhan.
//
// ⚠ ONLY FROM `dang-phan-loai`. A referral of work already begun (`dang-xu-ly` -> `chuyen-cap-tren`)
// is not in the lifecycle map and is undecided with the owner (rule 10, stop condition #2); it is not
// built here and must not be added by widening domain.KetThucNhanhDuoc.
func (uc *XuLyPhanAnh) ChuyenCapTren(ctx context.Context, ma string, yc YeuCauChuyenCapTren,
	nguoi audit.Actor, hanChe QuyenXemHanChe) (domain.PhieuPhanAnh, error) {

	lyDo, err := domain.KiemLyDoKetThucNhanh(yc.LyDo)
	if err != nil {
		return domain.PhieuPhanAnh{}, err
	}
	coQuanNhan, err := domain.KiemCoQuanNhan(yc.CoQuanNhan)
	if err != nil {
		return domain.PhieuPhanAnh{}, err
	}
	return uc.ketThucNhanh(ctx, ma, domain.ChuyenCapTren, lyDo, coQuanNhan, nguoi, hanChe)
}

// ketThucNhanh is the shared body of the two branch acts. `lyDo` and `coQuanNhan` are ALREADY
// validated; `coQuanNhan` is "" for a refusal.
//
// ONE BODY FOR TWO ACTS, unlike the four acts above, because these two differ in ONE column and one
// verb and are otherwise the same act with the same obligations. Two copies would be two places where
// "the reason is not in the delta" could be kept in one and lost in the other.
func (uc *XuLyPhanAnh) ketThucNhanh(ctx context.Context, ma string, nhanh domain.TrangThai,
	lyDo, coQuanNhan string, nguoi audit.Actor, hanChe QuyenXemHanChe) (domain.PhieuPhanAnh, error) {

	viec, hanhVi := "không tiếp nhận", HanhViKhongTiepNhanPhanAnh
	if nhanh == domain.ChuyenCapTren {
		viec, hanhVi = "chuyển cấp trên", HanhViChuyenCapTrenPhanAnh
	}
	if err := coCanBoThucHien(nguoi); err != nil {
		return domain.PhieuPhanAnh{}, err
	}

	// THE SAME CLOCK EVERY OTHER ACT USES. It becomes `ket_thuc_nhanh_luc`, which migration 0011
	// requires to be at or after `phan_loai_luc` — both come from this seam, so they cannot disagree.
	bayGio := uc.nayHoac()
	var sau domain.PhieuPhanAnh

	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		p, err := uc.kho.TheoMaTraCuuDeSua(ctx, tx, ma)
		if err != nil {
			return err
		}
		// BEFORE THE LIFECYCLE CHECK, for the reason Dong gives: refusing a report about a member of
		// staff — or passing it to another body — is writing the final word on it, and a 409 about
		// the state would tell a colleague of that person that the record exists.
		if err := duocChamPhieuHanChe(p, hanChe); err != nil {
			return err
		}
		if err := domain.KetThucNhanhDuoc(p.TrangThai, nhanh); err != nil {
			return err
		}

		switch nhanh {
		case domain.KhongTiepNhan:
			err = uc.kho.KhongTiepNhan(ctx, tx, p.ID, p.TrangThai, lyDo, bayGio)
		default:
			err = uc.kho.ChuyenCapTren(ctx, tx, p.ID, p.TrangThai, lyDo, coQuanNhan, bayGio)
		}
		if err != nil {
			return err
		}

		sau = p
		sau.TrangThai = nhanh
		sau.LyDoKetThucNhanh = lyDo
		sau.CoQuanNhan = coQuanNhan
		sau.KetThucNhanhLuc = bayGio

		// THE REASON AND THE RECEIVING BODY ARE NOT IN THE DELTA; THEIR LENGTHS ARE.
		//
		// Same argument as Dong makes for `ket_qua_xu_ly`: free text a member of staff typed about ONE
		// case will eventually name the reporter or a third person, and `audit_log` is append-only and
		// never deleted — a copy there is a second permanent store of personal data (rule 6,
		// forbidden #4). Nothing is lost: migration 0011's trigger freezes both columns once written,
		// so the record itself is the tamper-proof copy, and the entry says a reason WAS recorded,
		// by whom and when.
		//
		// ⚠ THE RECEIVING BODY IS NORMALLY AN AUTHORITY'S NAME, NOT PERSONAL DATA, and excluding it is
		// the conservative reading of this session — reported, not buried. If an inspection needs it
		// in the trail itself, that is one line here and the owner's call.
		delta := map[string]any{
			"truoc":                  map[string]any{"trang_thai": string(p.TrangThai)},
			"sau":                    map[string]any{"trang_thai": string(nhanh)},
			"do_dai_ly_do":           len([]rune(lyDo)),
			"co_ly_do_doc":           true,
			"ket_thuc_nhanh_luc_ghi": lucRaVet(bayGio),
		}
		if nhanh == domain.ChuyenCapTren {
			delta["do_dai_co_quan_nhan"] = len([]rune(coQuanNhan))
		}
		than, err := json.Marshal(delta)
		if err != nil {
			return fmt.Errorf("xu_ly_phan_anh: mã hoá delta: %w", err)
		}
		if err := audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  hanhVi,
			Subject: p.MaTraCuu,
			Delta:   than,
		}); err != nil {
			return err
		}

		// THE EVENT CARRIES THE STATUS AND THE FIXED SENTENCE OF domain.loiNhanChoDan — NEVER THE
		// REASON OR THE BODY (events.proto forbids staff-written text). A staff-booked petition has no
		// recipient and gets no row, as on every other act (ghiSuKienDoiTrangThai).
		return uc.ghiSuKien(ctx, tx, sau, nhanh, bayGio)
	})
	if err != nil {
		return domain.PhieuPhanAnh{}, bocPhieu(ctx, viec, err)
	}
	return sau, nil
}

// --- the notification obligation ------------------------------------------------------------------

// ghiSuKien records `petitions.status_changed.v1` in the SAME transaction as the change.
//
// # WHY IT IS A ROW AND NOT A PUBLISH CALL
//
// The contract says it in its own words: PUBLISHED AFTER THE BUSINESS TRANSACTION COMMITS, NEVER
// INSIDE IT. An event published inside an open transaction arrives at a consumer that then reads a
// petition the database has not yet made visible, and the failure is timing-dependent, so it survives
// every test. Publishing after the commit with nothing recorded loses the message whenever the
// process dies in the window — and rule 10, invariant 5 is not a promise that tolerates that. The
// outbox is the only shape in which both hold.
//
// # WHAT IS IN THE MESSAGE, AND WHAT MAY NEVER BE
//
// Four scalars and, on the transitions that owe the citizen a word, two composed sentences. The
// contract's header closes the list: no phone number, no name, no address, no national ID, no
// coordinates, NO TEXT OF THE PETITION, no photograph, at any nesting depth. A queue is persisted,
// replicated, backed up and read during debugging, and deleting a field later does not recall the
// copies.
//
// # NO RECIPIENT, NO ROW
//
// `citizen_id` is required by the contract and the consumer refuses a message without one. A
// staff-booked petition has no citizen account behind it, so there is nobody to tell and nothing that
// could be published. Writing a row with an empty recipient would put a message in the queue that is
// guaranteed to dead-letter; writing none says the truth.
//
// ⚠ THE CONSEQUENCE IS A REAL GAP AND IT IS REPORTED: on that channel, "every status change notifies
// the citizen" cannot hold, because the system does not know who the citizen is. The commune took the
// report by telephone or in person, and telling them back is outside this software until somebody
// decides how.
func (uc *XuLyPhanAnh) ghiSuKien(ctx context.Context, tx *store.ScopedTx,
	p domain.PhieuPhanAnh, moi domain.TrangThai, luc time.Time) error {
	return ghiSuKienDoiTrangThai(ctx, tx, uc.suKien, uc.sinhID, p, moi, luc)
}

// ghiSuKienDoiTrangThai is the ONE builder of the `petitions.status_changed.v1` outbox row, shared by
// the staff acts above and the citizen intake (gui_phan_anh.go).
//
// A PACKAGE FUNCTION AND NOT A METHOD, because two use cases publish the same fact and a second copy
// of this body in the intake would be a second place where the message shape, the occurrence rule and
// the "no recipient, no row" rule could drift apart. `p` is the petition AFTER the change.
func ghiSuKienDoiTrangThai(ctx context.Context, tx *store.ScopedTx, suKien KhoSuKien,
	sinhID func() (string, error), p domain.PhieuPhanAnh, moi domain.TrangThai, luc time.Time) error {

	if p.CongDanID == "" {
		return nil
	}

	tin := &petitionsv1.PetitionStatusChanged{
		LookupCode: p.MaTraCuu,
		Status:     string(moi),
		// WHICH OCCURRENCE of this status on this petition. `so_lan_mo_lai` is 0 until a reopening
		// happens, so the first time through is 1 — and ZERO IS NEVER SENT, which the consumer treats
		// as malformed on purpose: defaulting it to 1 would make the SECOND closing of a reopened
		// petition look like a duplicate of the first, and the citizen would never be told about it.
		Occurrence: uint32(p.SoLanMoLai) + 1,
		CitizenId:  p.CongDanID,
	}

	// PRESENT ONLY WHEN THIS TRANSITION OWES A MESSAGE. Absence is a fact, not an omission — see
	// domain.loiNhanChoDan for which six do and why the other three do not. The deadline, where one
	// is named, travels INSIDE the composed sentence: CitizenMessage has no structured deadline field,
	// and the contract says timing is phrased by the publisher, the only service that knows which of
	// the two clocks has been set.
	if viec := domain.ViecTiepTheo(p, moi); viec != "" {
		tin.CitizenMessage = &petitionsv1.CitizenMessage{
			StatusLabel: domain.NhanTrangThai(moi),
			NextStep:    viec,
		}
	}

	// UseProtoNames, because the contract says the payload carries proto field names: the person who
	// opens a dead-lettered message at 2am must see the field names the .proto declares, not a
	// lowerCamelCase spelling that matches nothing they can grep for.
	than, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(tin)
	if err != nil {
		return fmt.Errorf("xu_ly_phan_anh: mã hoá sự kiện: %w", err)
	}

	id, err := sinhID()
	if err != nil {
		return fmt.Errorf("xu_ly_phan_anh: sinh mã sự kiện: %w", err)
	}

	return suKien.Chen(ctx, tx, petstore.SuKienDi{
		ID:       id,
		Ten:      tenSuKienDoiTrangThai,
		DoiTuong: p.MaTraCuu,
		Than:     than,
		XayRaLuc: luc,
	})
}

// --- shared ----------------------------------------------------------------------------------------

// coCanBoThucHien refuses a write whose trail cannot name who made it.
//
// core/audit refuses an entry with no actor for the same reason; refusing HERE, before the transaction
// opens, avoids a rollback whose cause is a missing principal rather than anything about the petition.
// Rule 6 does not permit a business write whose trail cannot name its author.
//
// THE KIND IS CHECKED AS WELL AS THE ID. These are STAFF acts behind authz.RequirePermission, so a
// citizen principal reaching one means the route was mounted on the wrong mux — and rule 4,
// invariant 5 keeps the two surfaces apart at routing level precisely so that cannot happen quietly.
func coCanBoThucHien(nguoi audit.Actor) error {
	if nguoi.ID == "" {
		return errors.New(loiThieuChuThe)
	}
	if nguoi.Kind != chuThePhaiLaCanBo {
		return fmt.Errorf("%s (kind=%q)", loiChuTheKhongPhaiCanBo, nguoi.Kind)
	}
	return nil
}

// lucRaVet renders an instant for the audit delta, and renders the ZERO as "" rather than as the year
// 1 — an entry saying a deadline was "0001-01-01T00:00:00Z" before it was fixed reads as a petition
// that was already two thousand years overdue.
func lucRaVet(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// bocPhieu wraps a failure with the commune and the operation, and NOTHING ELSE.
//
// NOT THE LOOKUP CODE, NOT THE CONTENT, NOT THE REPORTER. The code is the one string that opens a
// citizen's petition, and an error travels into centralised logging across every commune at once
// (rule 3, invariants 1 and 2). The commune is not personal data and is the one thing an operator can
// act on.
//
// The chain is kept with %w so the handler can still tell a refusal from a failure with errors.Is. A
// %v here would collapse "this petition has already moved" and "the database is down" into one 500.
func bocPhieu(ctx context.Context, viec string, err error) error {
	return fmt.Errorf("xu_ly_phan_anh: %s cho xã %s: %w", viec, tenant.MustFrom(ctx), err)
}
