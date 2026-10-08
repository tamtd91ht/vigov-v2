package app

// The use case behind the CITIZEN intake route — POST /api/v1/my-citizen-reports.
//
// THIS IS THE ACT THE WHOLE SYSTEM EXISTS FOR (rule 10): a petition is the one object a CITIZEN
// creates and then watches, and the surface the commune is judged on. Three things happen here and
// they are ordered rather than grouped, because the order is what makes each promise keepable:
//
//	0. CHECK the picked field, if any, against the commune's offered catalogue (ADR 0050, 0060)
//	1. ASK identity for the deadline(s) — acknowledge, plus resolve when a field was picked; may
//	   refuse, and a refusal ends the intake
//	2. MINT the lookup code                          — only once step 1 has succeeded
//	3. ONE TRANSACTION: the row, its audit entry     — rule 6, invariant 3
//	   and the `petitions.status_changed.v1` outbox row  — rule 10, invariant 5; rule 2, invariant 6
//
// # WHY STEP 1 COMES BEFORE STEP 2, AND WHY THAT IS NOT A DETAIL
//
// Rule 7, invariant 3: an issued lookup code is never reissued, and `UNIQUE (tenant_id,
// ma_tra_cuu)` counts soft-deleted rows so the database enforces it. Rule 10, invariant 1: the code
// is the commitment — the moment a citizen holds one, the authority has said "we have your report".
//
// Put the mint first and a failed intake burns a code AND, worse, makes it possible for somebody to
// "helpfully" answer the citizen with it. Put it second and the failing path has produced nothing
// at all: no row, no code, no commitment. That is what "fail the intake" has to mean.
//
// # WHY THERE IS NO FALLBACK WHEN THE COMMUNE HAS NO SLA
//
// Today `sla` is EMPTY FOR EVERY COMMUNE — migration 0008 seeds nothing and the onboarding step
// does not exist — so this use case refuses every submission of every commune. THAT IS THE
// CONTRACT WORKING, not a bug to route around (core/identityclient/han_xu_ly.go says the same at
// length). A commitment invented by software is still told to a citizen as though the authority
// made it (rule 10, forbidden #3), and a row stored with no deadline is a commitment nobody counts
// while every on-time report counts it anyway.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// KhoPhieuGhi is the write half of the petition register, declared at the point of use.
//
// IT TAKES THE TRANSACTION, and that is what makes it impossible to write the row in one
// transaction and the audit entry in another: there is no signature here that would let you.
// *petstore.PhieuPhanAnhStore satisfies it as it is; nothing was changed to accommodate this.
type KhoPhieuGhi interface {
	Tao(ctx context.Context, tx *store.ScopedTx, p domain.PhieuPhanAnh) error
}

// CitizenIntakePetitions is KhoPhieuGhi plus the UNVERIFIED-petition ceiling (ADR 0080 decision 7):
// serialise one Zalo account's intakes in this commune, then count what it already sent today — both
// inside the intake transaction. A separate interface so the staff-booked intake, which has no Zalo
// owner, does not depend on it. *petstore.PhieuPhanAnhStore satisfies it.
type CitizenIntakePetitions interface {
	KhoPhieuGhi
	LockZaloAccountIntake(ctx context.Context, tx *store.ScopedTx, zaloAccountID string) error
	CountZaloAccountPetitionsSince(ctx context.Context, tx *store.ScopedTx, zaloAccountID string,
		since time.Time) (int, error)
}

// IntakeSender is who files a petition through the citizen channel — and therefore who OWNS it — plus
// the address the request arrived from, for the trail (rule 6, invariant 2).
//
// AN EXPLICIT OWNER WITH A KIND, never a bare id (domain.PetitionOwner says why). The handler builds it
// from the SESSION's principal (authz.CitizenChannelOwner) and from nothing else (rule 4, invariant 2);
// YeuCauGuiPhanAnh has no owner field, so a request body cannot name one.
type IntakeSender struct {
	Owner domain.PetitionOwner
	IP    string
}

// Actor is the audit "who" of the sender's act: the opaque owner id with its Kind. A citizen has no
// staff business code, and rule 6 invariant 8 binds staff only (core/authz Principal.Ma); for a Zalo
// account the "who" is `tai_khoan_zalo.id` — ADR 0080 decision 9. The ONE place in this service that
// turns a citizen-channel owner into an audit actor; the scene-photo handlers use it too.
func (s IntakeSender) Actor() (audit.Actor, error) {
	if !s.Owner.Valid() {
		return audit.Actor{}, fmt.Errorf("gui_phan_anh: chủ phiếu không hợp lệ (kind=%q) — tuyến thiếu "+
			"authz.CitizenOnly hoặc lớp xã từ phiên", s.Owner.Kind)
	}
	switch s.Owner.Kind {
	case domain.OwnerCitizen:
		return audit.Actor{ID: s.Owner.ID, Kind: "citizen", IP: s.IP}, nil
	case domain.OwnerZaloAccount:
		// @actor-ok: the "who" of an unverified petition is `tai_khoan_zalo.id` with Kind zalo-account —
		// ADR 0080 decision 9; rule 6 invariant 8 (business code) binds staff only (core/authz/authz.go
		// Principal.Ma). Not a staff actor, so there is no business code to use instead.
		return audit.Actor{ID: s.Owner.ID, Kind: audit.KindZaloAccount, IP: s.IP}, nil
	}
	return audit.Actor{}, fmt.Errorf("gui_phan_anh: loại chủ phiếu không được khai (kind=%q)", s.Owner.Kind)
}

// ErrUnverifiedDailyLimit — the Zalo account already sent domain.UnverifiedDailyCeiling unverified
// petitions in this commune today (ADR 0080 decision 7). Nothing was written and no code was issued.
// 429 to the sender.
var ErrUnverifiedDailyLimit = errors.New("gui_phan_anh: tài khoản Zalo đã gửi đủ số phiếu chưa xác thực trong ngày")

// HanTiepNhanDoc asks identity for the instants this commune's commitments fall due.
//
// THE FULL identityclient.Client SIGNATURE, DELIBERATELY, rather than a narrow
// `HanTiepNhan(ctx, t) (time.Time, error)` that would read better here. Three of the four arguments
// are decisions ADR 0028 makes about THIS act — which business domain's hours, which field code,
// which of the two clocks is being fixed — and a wrapper would move them out of this file into a
// piece of wiring nobody tests. Keeping them here is what lets gui_phan_anh_test.go assert that the
// citizen channel asks for `phan-anh`, for the DEFAULT row (`linh_vuc` ""), and for the acknowledge
// clock ALONE.
//
// # THE SECOND METHOD IS THE CLASSIFICATION CEILING, AND IT IS A DIFFERENT QUESTION
//
// ADR 0035 §C (which closed open question #26 on 2026-09-22) puts a ceiling on how long a petition
// may sit unclassified: ONE WORKING DAY from intake, counted in WORKING HOURS through
// identity.AdvanceWorkingHours. That number does NOT come from the `sla` table — that table has two
// columns, `gio_tiep_nhan` and `gio_xu_ly_xong`, and no third — so ResolveDeadlines cannot answer it
// and TienGioLamViec is the correct call: this is precisely the case core/identityclient describes as
// "a caller that genuinely holds its own number of hours and is not reading `sla`".
//
// BOTH METHODS ON ONE INTERFACE because one act — creating the row — fixes both clocks from the same
// origin, and a use case that could be built with one and not the other is a use case that can store
// a petition with an acknowledge deadline and no ceiling. Those petitions are invisible to the
// indicator ADR 0035 §C created, which is the exact hole it was written to close.
//
// *identityclient.Client satisfies this as it is.
type HanTiepNhanDoc interface {
	HanXuLy(ctx context.Context, loaiViec identityv1.WorkKind, linhVuc string, tuLuc time.Time,
		can []identityv1.DeadlineKind) (map[identityv1.DeadlineKind]time.Time, error)

	TienGioLamViec(ctx context.Context, tuLuc time.Time, gio []uint32) (map[uint32]time.Time, error)
}

// CitizenIntakeFields checks the field a citizen picked against what the commune offers.
// *PetitionFieldCatalogue satisfies it. Answers ErrFieldNotOffered (one sentinel for every reason the
// field is not on the form) or ErrFieldCatalogueUnavailable (platform unreachable — 503).
type CitizenIntakeFields interface {
	CheckCitizenIntakeField(ctx context.Context, code string) (string, error)
}

// kenhCongDan is the channel EVERY petition filed through this use case carries.
//
// A CONSTANT, NEVER A REQUEST FIELD. `kenh_tiep_nhan` decides which deadline rules apply (ADR 0028
// decision E) and it is IMMUTABLE once written — the `ho_so_luu_tru_bat_bien` trigger refuses to
// change it. A client naming its own channel is a client choosing its own SLA, and on a surface
// reachable by anybody holding a phone number that is not a field, it is a lever.
//
// `zalo-mini-app` AND NOT ONE OF THE OTHER THREE, checked rather than assumed: the citizen edge
// this route sits behind exists for the Mini App, which has no domain and resolves its commune from
// the session (ADR 0022). A SECOND CITIZEN CHANNEL THAT IS NOT THE MINI APP IS ADR 0022's STOP
// CONDITION #4 — whoever adds one answers where its channel value comes from at the same time.
const kenhCongDan = domain.KenhZaloMiniApp

// HanhViGuiPhanAnh is the business verb in the trail. Vietnamese snake_case, like every other
// action this system already writes (`dang_nhap`, `xem_day_du_nguoi_gui`, `them_loai_nhiem_vu`) —
// an inspection reads these strings, and a function name would tell them nothing.
//
// IT NAMES THE CITIZEN'S ACT, not the staff one. The staff-booked intake has a verb of its own
// (ActionStaffIntake, staff_intake.go): "who filed this" is the first question asked of a disputed petition, and one
// verb across both channels would make the ledger unable to answer it without reading a row that
// may since have been edited.
const HanhViGuiPhanAnh = "cong_dan_gui_phan_anh"

// ErrChuaAnDinhDuocHan means the commune's commitment could not be established, so the intake did
// not happen.
//
// ONE SENTINEL FOR EVERY CAUSE — the commune has no `sla` table, it has one with no default row,
// identity is unreachable, the caller key is wrong. The CALLER's answer is the same for all of
// them: refuse, hand out no code, and tell the citizen the channel is not open. The cause is not
// dropped: it rides in the wrapped chain and core/identityclient has already logged the gRPC code,
// which is the value that tells an operator whether to open the configuration screen or the
// incident channel.
var ErrChuaAnDinhDuocHan = errors.New("gui_phan_anh: chưa ấn định được hạn tiếp nhận của xã")

// YeuCauGuiPhanAnh is one petition as a CITIZEN sends it.
//
// # THERE IS NO CongDanID FIELD AND THERE MUST NEVER BE ONE
//
// Rule 4, invariant 2: the citizen's identity comes FROM THE SESSION. A field here is a field a
// handler can fill from a request body, and the value looks identical in a diff whichever end it
// came from — which is why rule 4, forbidden #1 is the top entry of that rule's list. The owner of
// the record is taken from the `sender` argument of Gui (IntakeSender), which the handler builds from
// authz.Principal and from nothing else.
//
// # Field IS THE CITIZEN'S PICK, AND IT IS THE PETITION'S FIELD (ADR 0050 point 1, 28/09/2026)
//
// This reverses ADR 0028's closing of open question #23 for the Mini App channel, with its cost stated
// there: a citizen can pick the most urgent field to get the shortest commitment, and since a deadline
// may only be shortened, staff cannot lengthen it. The owner accepted that; the tracking figure is the
// share of petitions whose field staff change at classification.
//
// OPTIONAL FOR NOW, and "" keeps the pre-ADR-0050 shape (han_xu_ly_xong NULL until classification).
// ADR 0050 point 9 makes it required, but the shared citizen screen does not send it yet; requiring it
// today would refuse every petition from that screen.
//
// # THERE IS NO GocDemHan FIELD
//
// The origin of both clocks is when the citizen pressed send, and on this channel that instant is
// NOW — the request is the press. `goc_dem_han` as a parameter belongs to the staff-booked form,
// where an officer records when the citizen ACTUALLY reported it and domain.KiemGocDemHan bounds
// how far back they may put it (ADR 0028 decision F). Accepting one here would hand a client the
// ability to manufacture an already-overdue petition.
type YeuCauGuiPhanAnh struct {
	NoiDung   string
	DiaChi    string
	HoTen     string
	DienThoai string

	// AnDanh hides the reporter from staff screens and from the public page. It does NOT mean the
	// identity is not recorded: CongDanID, the name and the number are all stored either way
	// (ADR 0008), because without them there is no anti-spam, the citizen cannot find their own
	// petition, and a defamatory report becomes untraceable.
	AnDanh bool

	// Field is the field code the citizen picked from GET /api/v1/my-citizen-report-fields. "" = none.
	Field string

	// Lat and Lng are the OPTIONAL scene location (ADR 0050). The citizen app obtains them from the
	// vihat-miniapp backend, which holds the Zalo app secret that exchanges the getLocation token
	// (ADR 0032), and forwards them with the petition. They are CLIENT-SUPPLIED DESCRIPTION of where
	// the problem is and grant nothing: no isolation decision reads them.
	//
	// STILL OPTIONAL. ADR 0050 point 9 defers the mandatory photos-and-location check until both
	// actually work end to end; requiring it before then would refuse every petition.
	Lat *float64
	Lng *float64
}

// GuiPhanAnh owns the citizen intake.
type GuiPhanAnh struct {
	db  *store.DB
	kho CitizenIntakePetitions
	han HanTiepNhanDoc

	// suKien records the intake's `petitions.status_changed.v1` (into `da-tiep-nhan`) in the SAME
	// transaction as the row and its audit entry. The same KhoSuKien the staff acts write through —
	// see ghiSuKienDoiTrangThai.
	suKien KhoSuKien

	// fields checks the citizen's picked field against the commune's offered catalogue (ADR 0050 pt 1,
	// ADR 0060). Consulted only when a field is sent.
	fields CitizenIntakeFields

	// sinhID and sinhMa are injected so a test can pin both values. In production they are
	// ulid.Moi and domain.SinhMaTraCuu. sinhID mints BOTH the petition id and the outbox row id.
	sinhID func() (string, error)
	sinhMa func() (string, error)

	// luc is the clock. Injected for the same reason, and it matters more than usual here: this
	// instant becomes `goc_dem_han`, which is IMMUTABLE and which both deadlines are counted from.
	luc func() time.Time
}

func NewGuiPhanAnh(db *store.DB, kho CitizenIntakePetitions, suKien KhoSuKien, han HanTiepNhanDoc,
	fields CitizenIntakeFields) *GuiPhanAnh {
	return &GuiPhanAnh{
		db: db, kho: kho, suKien: suKien, han: han, fields: fields,
		sinhID: ulid.Moi,
		sinhMa: domain.SinhMaTraCuu,
		luc:    func() time.Time { return time.Now().UTC() },
	}
}

// Gui receives one petition from the citizen who filed it, and returns the record — including the
// lookup code the caller must hand back (rule 10, invariant 1).
//
// `sender` IS BOTH THE ACTOR AND THE OWNER, and that is a property of this route rather than a
// shortcut: on a self-filed petition the person acting IS the person the record belongs to. Taking
// one value and using it for both is what makes them impossible to get out of step — two parameters
// would be two things a caller could fill from two places, and one of those places is the request
// body.
//
// TWO OWNER KINDS, ONE INTAKE (ADR 0080). A verified citizen's petition is stored with `cong_dan_id`
// exactly as before. A Zalo account's — a Mini App session with no verified phone — is stored with
// `zalo_account_id` and `cong_dan_id` NULL, the typed name and number kept as contact details only
// (decision 2), and is refused past domain.UnverifiedDailyCeiling a day (decision 7). Nothing else
// differs: same channel constant, same deadlines, same trail in the same transaction. Its outbox row is
// not written — writeStatusChangedEvent has no recipient without `cong_dan_id` (decision 4: no ZNS).
func (uc *GuiPhanAnh) Gui(ctx context.Context, yc YeuCauGuiPhanAnh, sender IntakeSender) (
	domain.PhieuPhanAnh, error) {

	// FAIL CLOSED, BEFORE ANYTHING ELSE. authz.CitizenOnly and the session class refuse every request
	// without an owner before the handler runs, so reaching here without one means the route was
	// mounted wrong. A petition stored with no owner cannot be found again by the person who filed it,
	// and core/audit refuses an entry with no actor for the same reason.
	actor, err := sender.Actor()
	if err != nil {
		return domain.PhieuPhanAnh{}, err
	}

	// THE CHANNEL'S OWN PREDICATES ARE CONSULTED, NOT ASSUMED. They are the vocabulary ADR 0028
	// decision E is written in, and they are what must fail the day this route is pointed at another
	// channel — rather than the wrong deadline shape reaching a citizen.
	if !kenhCongDan.HanTiepNhanApDung() || kenhCongDan.CoLinhVucLucVaoSo() {
		return domain.PhieuPhanAnh{}, fmt.Errorf(
			"gui_phan_anh: kênh %q không phải kênh công dân tự gửi — biểu mẫu của nó mang lĩnh vực "+
				"ngay lúc vào sổ, nên hình dạng hai đồng hồ khác hẳn (ADR 0028 quyết định E)", kenhCongDan)
	}

	// Validated BEFORE the deadline is asked for and before the transaction opens. A misshapen
	// request must not become load on identity and must not hold a row lock while being refused.
	noiDung, err := domain.ChuanHoaNoiDung(yc.NoiDung)
	if err != nil {
		return domain.PhieuPhanAnh{}, err
	}
	diaChi, err := domain.ChuanHoaDiaChi(yc.DiaChi)
	if err != nil {
		return domain.PhieuPhanAnh{}, err
	}
	hoTen, err := domain.ChuanHoaHoTen(yc.HoTen)
	if err != nil {
		return domain.PhieuPhanAnh{}, err
	}
	dienThoai, err := domain.ChuanHoaDienThoai(yc.DienThoai)
	if err != nil {
		return domain.PhieuPhanAnh{}, err
	}
	lat, lng, err := domain.NormaliseSceneLocation(yc.Lat, yc.Lng)
	if err != nil {
		return domain.PhieuPhanAnh{}, err
	}

	// THE FIELD, CHECKED BEFORE IDENTITY IS ASKED. identity answers a code it has no row for with the
	// DEFAULT row, silently (identity.proto, ResolveDeadlines) — so an unchecked code would fix a wrong
	// commitment onto an archival record. The check is the catalogue's own rule, so a field the form
	// did not offer is refused here exactly as the form would not have shown it.
	field := ""
	if yc.Field != "" {
		if uc.fields == nil {
			return domain.PhieuPhanAnh{}, errors.New("gui_phan_anh: thiếu bộ kiểm lĩnh vực — sai nối dây")
		}
		if field, err = uc.fields.CheckCitizenIntakeField(ctx, yc.Field); err != nil {
			return domain.PhieuPhanAnh{}, err
		}
	}

	// ONE INSTANT FOR BOTH COLUMNS, read once. `goc_dem_han` is when the citizen pressed send and
	// `vao_so_luc` is when the row was created; on this channel they are the same act, and the
	// schema's `goc_dem_han <= vao_so_luc` holds by equality. Two calls to the clock would put a
	// few microseconds between them in the wrong direction on a bad day, and the CHECK would refuse
	// the intake for a reason nobody could reproduce.
	bayGio := uc.luc()

	// STEP 1 — the commitment, from the commune's own table, counted in WORKING HOURS by the one
	// service that owns the calendar (ADR 0007). It is asked for ONCE, here, at the act that fixes
	// it, and stored (rule 10, invariant 2). Nothing recomputes it on read.
	//
	// TWO SHAPES, DECIDED BY WHETHER THE CITIZEN PICKED A FIELD:
	//
	//	field ""   `linh_vuc` "" = the DEFAULT row, and ONLY the acknowledge clock. Asking for both here
	//	           would fix `han_xu_ly_xong` from the default row — the 56-hour ceiling ADR 0028
	//	           removed, rebuilt by accident. It is fixed later, when staff settle the field.
	//	field set  that field's row, and BOTH clocks in ONE call (ADR 0050 point 1): the field is the
	//	           petition's field, so both commitments are fixed now, by the act that fixes them
	//	           (rule 10, invariant 2). One call is one calendar read — two could read two calendars
	//	           (identity.proto, "ONE CALL CAN STILL NAME BOTH").
	clocks := []identityv1.DeadlineKind{identityv1.DeadlineKind_DEADLINE_KIND_TIEP_NHAN}
	if field != "" {
		clocks = append(clocks, identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG)
	}
	han, err := uc.han.HanXuLy(ctx, identityv1.WorkKind_WORK_KIND_PHAN_ANH, field, bayGio, clocks)
	if err != nil {
		// THE INTAKE FAILS HERE, AND NOTHING HAS BEEN PRODUCED YET: no code, no row, no commitment.
		// Falling back to a number instead would be this software making a promise on behalf of a
		// public authority (rule 10, forbidden #3).
		return domain.PhieuPhanAnh{}, fmt.Errorf("%w cho xã %s: %w",
			ErrChuaAnDinhDuocHan, tenant.MustFrom(ctx), err)
	}
	hanTiepNhan, co := han[identityv1.DeadlineKind_DEADLINE_KIND_TIEP_NHAN]
	if !co || hanTiepNhan.IsZero() {
		// UNREACHABLE unless the contract stops holding — identityclient already refuses a partial
		// answer. Checked rather than assumed because the alternative is the zero time.Time, which
		// reaches `han_tiep_nhan` as SQL NULL, i.e. "KHÔNG ÁP DỤNG" — a citizen's petition silently
		// recorded as one nobody owes an acknowledgement for.
		return domain.PhieuPhanAnh{}, fmt.Errorf("%w cho xã %s: hợp đồng trả về hạn tiếp nhận rỗng",
			ErrChuaAnDinhDuocHan, tenant.MustFrom(ctx))
	}
	var resolveDue time.Time
	if field != "" {
		// Same reasoning as the acknowledge clock: a zero here would reach `han_xu_ly_xong` as NULL —
		// "CHƯA CÓ" on a petition whose field is settled, i.e. a commitment nobody counts.
		if resolveDue = han[identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG]; resolveDue.IsZero() {
			return domain.PhieuPhanAnh{}, fmt.Errorf("%w cho xã %s: hợp đồng trả về hạn xử lý xong rỗng",
				ErrChuaAnDinhDuocHan, tenant.MustFrom(ctx))
		}
	}

	// STEP 1b — THE CLASSIFICATION CEILING (ADR 0035 §C, open question #26 closed 2026-09-22).
	//
	// A SECOND CALL AND NOT A SECOND CLOCK ON THE FIRST ONE: ResolveDeadlines answers from the `sla`
	// table, which has two hour columns and no third, so the ceiling has no row there. This is the
	// caller that genuinely holds its own number of hours — domain.GioTranPhanLoai, which states in
	// full why 8 is an assumption of the implementer rather than a figure the customer wrote.
	//
	// COUNTED FROM THE SAME `bayGio` AS THE ACKNOWLEDGE CLOCK. ADR 0035 §C says "kể từ khi tiếp
	// nhận", and on this channel the instant the citizen pressed send IS the intake — it is
	// `goc_dem_han` and `vao_so_luc` both.
	//
	// A FAILURE HERE FAILS THE INTAKE, exactly like the first call, and the reason is the indicator
	// rather than the petition: a row stored with no ceiling is a petition that can never be counted
	// as late-to-classify, so a commune that lost this call for an afternoon would report a better
	// classification figure than one that did not. The ceiling is not decoration on the record; it is
	// the denominator.
	tran, err := uc.han.TienGioLamViec(ctx, bayGio, []uint32{domain.GioTranPhanLoai})
	if err != nil {
		return domain.PhieuPhanAnh{}, fmt.Errorf("%w cho xã %s: %w",
			ErrChuaAnDinhDuocHan, tenant.MustFrom(ctx), err)
	}
	hanPhanLoai, co := tran[domain.GioTranPhanLoai]
	if !co || hanPhanLoai.IsZero() {
		// UNREACHABLE unless the contract stops holding — identityclient already refuses a partial
		// answer. Checked rather than assumed because the alternative is the zero time.Time, which
		// reaches `han_phan_loai` as SQL NULL, i.e. "KHÔNG ÁP DỤNG" — a citizen's petition silently
		// recorded as one nobody has to classify in any particular time.
		return domain.PhieuPhanAnh{}, fmt.Errorf("%w cho xã %s: hợp đồng trả về trần phân loại rỗng",
			ErrChuaAnDinhDuocHan, tenant.MustFrom(ctx))
	}

	// STEP 2 — the code, minted only now. See the note at the top of this file.
	id, err := uc.sinhID()
	if err != nil {
		return domain.PhieuPhanAnh{}, fmt.Errorf("gui_phan_anh: sinh định danh: %w", err)
	}
	maTraCuu, err := uc.sinhMa()
	if err != nil {
		// domain.SinhMaTraCuu never papers over a randomness failure with a fallback, and neither
		// does this: a predictable code handed to a citizen cannot be withdrawn, because the code is
		// never reissued and the slip is already in their hand.
		return domain.PhieuPhanAnh{}, fmt.Errorf("gui_phan_anh: sinh mã tra cứu: %w", err)
	}

	moi := domain.PhieuPhanAnh{
		ID:       id,
		MaTraCuu: maTraCuu,
		Kenh:     kenhCongDan,
		NoiDung:  noiDung,
		DiaChi:   diaChi,
		Lat:      lat,
		Lng:      lng,

		NguoiGuiHoTen:     hoTen,
		NguoiGuiDienThoai: dienThoai,
		AnDanh:            yc.AnDanh,

		// The first of the nine (ADR 0027). `dang-phan-loai` is the step where a human first reads
		// the report, and claiming it here would stop the acknowledge clock before anybody had.
		TrangThai: domain.DaTiepNhan,

		GocDemHan: bayGio,
		VaoSoLuc:  bayGio,

		// STORED, ONCE, HERE. Rule 10, invariant 2.
		HanTiepNhan: hanTiepNhan,

		// THE CLASSIFICATION CEILING, fixed by the same act and from the same origin. Also stored
		// once and never recomputed: ADR 0035 §C fixes that changing the ceiling later applies only
		// to petitions received from that moment on.
		HanPhanLoai: hanPhanLoai,

		// THE CITIZEN'S FIELD AND ITS RESOLVE COMMITMENT, or neither. With no field, HanXuLyXong stays
		// zero -> SQL NULL -> "CHƯA CÓ" until staff settle it (ADR 0028 decision E); the opposite NULL —
		// `han_tiep_nhan` NULL, "KHÔNG ÁP DỤNG" — belongs to the staff-booked channel alone. Staff may
		// still change the field at classification; the deadline then only ever moves EARLIER (ADR 0027
		// decision C, XuLyPhanAnh.ChotLinhVuc).
		LinhVuc:     field,
		HanXuLyXong: resolveDue,
	}
	// THE PUBLIC PAGE (ADR 0050 point 8): nobody has decided yet (`cho-duyet`), or `an` for a petition
	// born in the staff-conduct field. Computed FROM THE FIELD THE ROW CARRIES rather than written as a
	// literal. `can-bo` cannot arrive here from a citizen today (CheckCitizenIntakeField refuses it),
	// but the rule stays tied to the field so the day it can, the petition is still born hidden.
	moi.PublicationStatus = domain.InitialPublicationStatus(moi.LinhVuc)

	// THE OWNER, IN EXACTLY ONE OF TWO COLUMNS — the other stays "" and the store writes NULL, never ''
	// (migration 0032: non-blank, single owner). sender.Actor() already refused any other kind.
	switch sender.Owner.Kind {
	case domain.OwnerCitizen:
		moi.CongDanID = sender.Owner.ID
	case domain.OwnerZaloAccount:
		moi.ZaloAccountID = sender.Owner.ID
	}

	// STEP 3 — the row and its trail, in ONE transaction (rule 6, invariant 3). There is no ordering
	// here in which the petition exists and the trail does not: either both commit or neither does.
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// THE UNVERIFIED CEILING, INSIDE THE TRANSACTION THAT WRITES THE ROW (ADR 0080 decision 7):
		// lock this account's intakes in this commune, count today's, refuse at the ceiling. Outside the
		// transaction two sends at one instant would both pass at 9. Counted from the commune's day
		// (domain.UnverifiedCountingDayStart) and INCLUDING soft-deleted rows — the store says why.
		if moi.ZaloAccountID != "" {
			if err := uc.kho.LockZaloAccountIntake(ctx, tx, moi.ZaloAccountID); err != nil {
				return err
			}
			n, err := uc.kho.CountZaloAccountPetitionsSince(ctx, tx, moi.ZaloAccountID,
				domain.UnverifiedCountingDayStart(bayGio))
			if err != nil {
				return err
			}
			if n >= domain.UnverifiedDailyCeiling {
				return ErrUnverifiedDailyLimit
			}
		}

		if err := uc.kho.Tao(ctx, tx, moi); err != nil {
			return err
		}

		// THE DELTA CARRIES NO PERSONAL DATA AT ALL — not raw, not masked (rule 6, forbidden #4).
		//
		// MASKED-BUT-PRESENT WAS CONSIDERED AND REJECTED. Rule 6, invariant 5 admits masked values,
		// and a masked number would be defensible; it would also be pointless. The entry's Subject is
		// the lookup code, which names the row, and the row itself is an archival record that is
		// never hard deleted and whose immutable columns the `ho_so_luu_tru_bat_bien` trigger
		// refuses to change. So the values are already recoverable from the place they belong, and
		// copying them into an append-only ledger that is never deleted only builds a second store
		// of citizen personal data.
		//
		// WHAT IS HERE INSTEAD ANSWERS WHAT AN INSPECTION ACTUALLY ASKS: which channel it came in
		// through, what the authority committed to and by when, and which optional boxes the citizen
		// filled — without one character of what they typed.
		//
		// THE SCENE LOCATION IS A BOOLEAN HERE, never the two numbers: raw coordinates in an
		// append-only ledger are rule 6, forbidden #4 in its plainest form. `has_scene_location` is
		// English because it is a new key (rule 12) — the `publication_status` precedent beside it.
		delta, err := json.Marshal(map[string]any{
			"kenh_tiep_nhan": string(kenhCongDan),
			"trang_thai":     string(moi.TrangThai),
			"an_danh":        moi.AnDanh,
			"goc_dem_han":    moi.GocDemHan,
			"han_tiep_nhan":  moi.HanTiepNhan,
			"han_phan_loai":  moi.HanPhanLoai,
			// The field and the resolve commitment it fixed — a field code is not personal data, and
			// "which field did the citizen pick, and what did that promise" is the first question of a
			// dispute about the deadline. Absent (not null) when no field was picked.
			"linh_vuc":           nilIfEmpty(moi.LinhVuc),
			"han_xu_ly_xong":     nilIfZero(moi.HanXuLyXong),
			"truong_da_dien":     truongDaDien(moi),
			"do_dai_noi_dung":    len([]rune(moi.NoiDung)),
			"publication_status": string(moi.PublicationStatus),
			"has_scene_location": moi.Lat != nil,
			// ADR 0080: whether the contact details are self-declared and unverified (a Zalo-account
			// owner). A boolean — the owner id is the entry's actor already, and nothing else is needed.
			"contact_unverified": moi.ContactUnverified(),
		})
		if err != nil {
			return fmt.Errorf("gui_phan_anh: mã hoá delta: %w", err)
		}

		// TenantID is left unset ON PURPOSE: audit.Write fills it from the transaction, which took it
		// from the context (rule 1, invariant 4). Passing it here would be a second source for the
		// one fact that decides which commune the entry belongs to.
		//
		// Subject is the BUSINESS CODE the citizen holds, never the internal ULID.
		if err := audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  HanhViGuiPhanAnh,
			Subject: moi.MaTraCuu,
			Delta:   delta,
		}); err != nil {
			return err
		}

		// THE THIRD WRITE — the citizen is owed a word (rule 10, invariant 5; owner's decision of
		// 2026-09-24: intake is the first transition that notifies). Inside the SAME transaction, so
		// there is no committed petition whose notification was never recorded: a failure here rolls
		// back the row and the trail too, and the citizen is told the report was not received (rule 2,
		// invariant 6). The row is drained after commit by the relay, never published from in here.
		//
		// A VERIFIED citizen's petition always has `moi.CongDanID`, so it gets its row. A ZALO-OWNED
		// one has none, and the helper's "no recipient, no row" branch writes nothing — ADR 0080
		// decision 4: no ZNS to a self-declared number. Occurrence is 1: `so_lan_mo_lai` is 0 on a
		// row that was just created.
		return ghiSuKienDoiTrangThai(ctx, tx, uc.suKien, uc.sinhID, moi, domain.DaTiepNhan, bayGio)
	})
	if err != nil {
		// Nothing was committed: no row, no trail, no outbox row. The three states agree, and the
		// citizen is told the report was not received — which is true.
		//
		// NEITHER THE CODE, THE CONTENT, THE REPORTER NOR THE CITIZEN IDENTIFIER IS IN THE MESSAGE.
		// The code is the one string that opens a citizen's petition and the identifier ties a pile
		// of lines together into one person's activity; an error travels into centralised logging
		// (rule 3, invariants 1 and 2). The commune is not personal data and is what an operator
		// needs.
		return domain.PhieuPhanAnh{}, fmt.Errorf("gui_phan_anh: ghi phiếu cho xã %s: %w",
			tenant.MustFrom(ctx), err)
	}
	return moi, nil
}

// nilIfEmpty and nilIfZero put JSON null in the trail for "not set", so a reader cannot mistake an
// empty string or 0001-01-01 for a real value.
func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nilIfZero(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

// truongDaDien names the optional COLUMNS the citizen filled, never their values.
//
// Same discipline as XemNguoiGui.truongDaMo, and the same reason: naming the fields answers "what
// did they send" for an inspection without keeping a second copy of it in a ledger that is never
// deleted.
func truongDaDien(p domain.PhieuPhanAnh) []string {
	ra := make([]string, 0, 3)
	if p.DiaChi != "" {
		ra = append(ra, "dia_chi")
	}
	if p.NguoiGuiHoTen != "" {
		ra = append(ra, "nguoi_gui_ho_ten")
	}
	if p.NguoiGuiDienThoai != "" {
		ra = append(ra, "nguoi_gui_dien_thoai")
	}
	return ra
}
