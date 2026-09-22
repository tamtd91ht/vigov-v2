package app

// The use cases behind the WRITE surface of SỔ VĂN BẢN ĐẾN
// (docs/ui-ux/05-van-ban-don-thu.md §3, §5, §7).
//
// WHY THIS LAYER EXISTS: rule 6, invariant 3 requires the audit entry to share a transaction with
// the business write, and core/audit.Write takes a *store.ScopedTx with no overload that writes
// outside one. Opening that transaction is this layer's job. The handler translates HTTP and nothing
// else; the store knows SQL and nothing else.
//
// THE SECOND REASON IS THE NUMBER. Booking a document is: allocate the next number under a row lock,
// insert the row, record the trail — and all three have to be ONE transaction, or the register ends
// up with a number nobody used, or a document nobody can account for. That needs the lock AND the
// rules AND the transaction at once, which is exactly one place: here.
//
// ---------------------------------------------------------------------------
// THE DATABASE IS THE FLOOR AND THIS LAYER IS THE SENTENCE. Said once, here:
//
//	UNIQUE (tenant_id, nam, so_vao_so)   migration 0004 — counts soft-deleted rows
//	the six states                       0004, `van_ban_den_trang_thai_hop_le`
//	the issued number is immutable       0004, trigger `so_van_ban_bat_bien`
//	the counter only moves forward       0004, trigger `day_so_van_ban_khong_lui`
//	hard removal refused outright        0004, `ho_so_luu_tru_cam_xoa_cung`
//	the timeline is append-only          0004, trigger `lich_su_chuyen_chi_them`
//
// Those hold against every writer — this service, a psql prompt, an import job written next year.
// What they cannot do is explain themselves: PostgreSQL answers with an exception whose text is
// English and names a constraint, which tells a clerk in a commune nothing they can act on. This
// layer refuses FIRST, in Vietnamese. A drift between the two is therefore a worse error message,
// never a hole — the constraint still runs last, and the whole transaction rolls back with the audit
// entry inside it.

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
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

// KhoVanBanDen is the register's write half, declared at the point of use.
//
// EVERY METHOD TAKES THE TRANSACTION. That is what makes it impossible to write the document in one
// transaction and the audit entry in another: there is no signature here that would let you. It is
// also what makes the properties worth proving — that a refusal writes nothing, that the entry
// shares the transaction, that the number is allocated under a lock — provable without a PostgreSQL.
// There is none reachable from this repository's build environment (VIGOV_TEST_DSN is unset), so a
// test that needed one would be a test that never runs.
type KhoVanBanDen interface {
	TheoIDDeSua(ctx context.Context, tx *store.ScopedTx, id string) (domain.VanBanDen, error)
	LoaiVanBanConDung(ctx context.Context, tx *store.ScopedTx, ma string) error
	Chen(ctx context.Context, tx *store.ScopedTx, v domain.VanBanDen) error
	CapNhat(ctx context.Context, tx *store.ScopedTx, v domain.VanBanDen) error
	ChuyenBoPhan(ctx context.Context, tx *store.ScopedTx, id, denBoPhan, canBoMa string,
		trangThai domain.TrangThaiVanBanDen) error
	ChenLichSuChuyen(ctx context.Context, tx *store.ScopedTx, c domain.ChuyenVanBan) error
	XoaMem(ctx context.Context, tx *store.ScopedTx, id, boi, lyDo string) error
}

// KhoDaySo allocates register numbers. A SEPARATE INTERFACE from the one above, and not three more
// methods on it, because the two have different obligations: everything in KhoVanBanDen writes the
// document, while this one writes the COUNTER and must be called exactly once per booking, inside
// the same transaction, under its row lock. Two interfaces, two obligations, visible at the point
// of use.
type KhoDaySo interface {
	CapSo(ctx context.Context, tx *store.ScopedTx, so docstore.SoSach, nam int) (int, error)
}

// HanXuLyDoc asks identity for the instant this commune's commitment on a document falls due.
//
// THE FULL identityclient.Client SIGNATURE, DELIBERATELY, rather than a narrow `Han(ctx, t)` that
// would read better here. Three of the four arguments are decisions ADR 0028 makes about THIS act —
// which work kind's hours, which field code, which clock is being fixed — and a wrapper would move
// them out of this file into a piece of wiring nobody tests. Keeping them here is what lets the test
// assert that the document register asks for `van-ban-den`, for the DEFAULT row (`linh_vuc` ""), and
// for the processing clock ALONE.
//
// *identityclient.Client satisfies this as it is.
type HanXuLyDoc interface {
	HanXuLy(ctx context.Context, loaiViec identityv1.WorkKind, linhVuc string, tuLuc time.Time,
		can []identityv1.DeadlineKind) (map[identityv1.DeadlineKind]time.Time, error)
}

// ErrChuaAnDinhDuocHan means the commune's commitment could not be established, so the document was
// NOT booked.
//
// # THIS IS THE MOST CONSEQUENTIAL LINE IN THE SERVICE AND IT IS DELIBERATE
//
// `sla` is EMPTY FOR EVERY COMMUNE TODAY: migration 0008 of service-identity seeds nothing and the
// onboarding step that sows a commune's first rows does not exist in this repository. So until
// somebody fills in a commune's SLA, POST /api/v1/incoming-documents answers 409 and books nothing.
// That is the contract working, not a bug to route around (core/identityclient.HanXuLy states it in
// full):
//
//	DO NOT fall back to a number. Not 40 hours, not the specification's own table. §7 rule 3's
//	figures came from ONE commune's prototype; a commitment invented by software is still reported
//	upward as though the authority made it (rule 10, forbidden #3).
//
//	DO NOT book the document without a deadline. `han_xu_ly_xong` is NOT NULL precisely so that
//	this decision cannot be made quietly by an INSERT; a row with no deadline is a document nobody
//	counts while every overdue report counts it anyway.
//
//	DO NOT add hours here. hooks/citizen_commitment_guard.py blocks the shape outside
//	service-identity, and rule 10, forbidden #2 is why: adding a duration walks straight through
//	nights, weekends, `ngay_nghi_le` and `ngay_lam_bu` while looking like it respected the unit.
//
// ONE SENTINEL FOR EVERY CAUSE — no `sla` row, no working calendar, identity unreachable, wrong
// caller key. The caller's answer is the same for all of them: refuse and say which screen fixes it.
// The cause is not dropped: it rides in the wrapped chain, and core/identityclient has already
// logged the gRPC code, which is the value that tells an operator whether to open the configuration
// screen or the incident channel.
var ErrChuaAnDinhDuocHan = errors.New("van_ban_den: xã chưa cấu hình thời hạn xử lý nên chưa vào sổ được")

// The business verbs written into the trail. Vietnamese snake_case, like every other action this
// system already writes (`dang_nhap`, `them_chung_tu_giai_ngan`): an inspection reads these strings,
// and a function name would tell them nothing.
//
// FOUR VERBS AND NOT ONE `sua_van_ban_den`, because they are four different administrative acts.
// `chuyen_van_ban_den` in particular is the one an inspection searches for by name — it is the
// record of who was made responsible for a document, and when.
const (
	HanhViVaoSoVanBanDen  = "vao_so_van_ban_den"
	HanhViSuaVanBanDen    = "sua_van_ban_den"
	HanhViGoVanBanDen     = "go_van_ban_den"
	HanhViChuyenVanBanDen = "chuyen_van_ban_den"
)

// YeuCauVaoSoVanBanDen is one document as it arrives from the handler.
//
// THERE IS NO `SoVaoSo` FIELD AND THERE MUST NEVER BE ONE. The number is the register's to give: it
// comes from the counter, under a row lock, inside the transaction. A field here is a field a
// handler can fill from a request body, and what that buys is a client reissuing a number that is
// already on a document somebody has acted on.
//
// THERE IS NO `Nam` FIELD EITHER. The year is the year of the ACT — see Them.
//
// THERE IS NO `TrangThai` FIELD: the store writes `moi-vao-so` as a literal, and the state moves by
// routing and by nothing else.
//
// THERE IS NO `HanXuLyXong` FIELD: the commitment is this commune's SLA and this commune's calendar,
// computed by identity. A client choosing its own deadline is a client choosing how long the
// authority may take.
type YeuCauVaoSoVanBanDen struct {
	NgayDen       time.Time
	SoKyHieu      string
	NgayVanBan    time.Time
	CoQuanBanHanh string
	LoaiVanBan    string
	TrichYeu      string
	DoKhan        domain.DoKhan
}

// YeuCauSuaVanBanDen is a PARTIAL edit: a nil pointer means "leave this alone".
//
// WHY POINTERS AND NOT A FULL REPLACEMENT. `SoKyHieu`, `NgayVanBan` and `DoKhan` are optional and
// their meaningful value includes the empty one — "this document carries no number of its own" is a
// statement, not an absence of one. A struct of plain values cannot tell "the client did not mention
// this" from "the client cleared it", so a dialog editing only the summary would silently wipe the
// issuing body's number off an archival record.
type YeuCauSuaVanBanDen struct {
	NgayDen       *time.Time
	SoKyHieu      *string
	NgayVanBan    *time.Time
	CoQuanBanHanh *string
	LoaiVanBan    *string
	TrichYeu      *string
	DoKhan        *domain.DoKhan
}

// YeuCauChuyenVanBan is the "Chuyển cho bộ phận khác" block of §3.5.
type YeuCauChuyenVanBan struct {
	DenBoPhan   string
	CanBoXuLyMa string // optional — "— Để bộ phận tự phân công —"
	LyDo        string
}

// VanBanDen owns booking, correcting, routing and removing one commune's incoming documents.
type VanBanDen struct {
	db    *store.DB
	kho   KhoVanBanDen
	daySo KhoDaySo
	han   HanXuLyDoc

	// sinhID is injected so a test can pin the id. In production it is ulid.Moi.
	sinhID func() (string, error)

	// nay is the clock the booking instant, the routing instant and the deadline's origin are all
	// taken from.
	//
	// A SEAM AND NOT time.Now() AT THE CALL SITE, for a reason about evidence rather than
	// convenience: the instant this returns becomes the ORIGIN the commitment is counted from, and a
	// test that cannot pin it cannot assert that the deadline asked for and the deadline stored are
	// about the same moment. In production this is nil and nayHoac returns the real clock.
	nay func() time.Time
}

func NewVanBanDen(db *store.DB, kho KhoVanBanDen, daySo KhoDaySo, han HanXuLyDoc) *VanBanDen {
	return &VanBanDen{db: db, kho: kho, daySo: daySo, han: han, sinhID: ulid.Moi}
}

// nayHoac is the clock, UTC. `TIMESTAMPTZ` stores an instant rather than a wall reading, so the
// location here changes nothing that is stored — it is fixed so that a value read back in a test
// compares equal without a location dance.
func (uc *VanBanDen) nayHoac() time.Time {
	if uc.nay == nil {
		return time.Now().UTC()
	}
	return uc.nay().UTC()
}

// Them books one incoming document: allocates its number, stores it, records the trail.
//
// THE SHAPE IS VALIDATED BEFORE THE TRANSACTION OPENS. A request that fails its shape must never
// hold the counter's row lock while doing so — that lock serialises every booking in the commune —
// and the caller needs the reason rather than a rollback.
//
// THE DEADLINE IS FETCHED BEFORE THE TRANSACTION OPENS TOO, AND THAT IS NOT A STYLE CHOICE. It is a
// gRPC call to another service; made inside the transaction it would hold the counter row for the
// length of a network round trip, so every booking in the commune would queue behind identity's
// latency, and an identity outage would become a register that hangs rather than one that refuses.
//
// `nam` IS THE YEAR OF THE ACT, not the year on the document. A register is opened per year and a
// number is given when the document is BOOKED: a letter dated 31/12/2026 that reaches the office on
// 02/01/2027 takes số 1/2027, with `ngay_den` and `ngay_van_ban` saying the rest. Taking the year
// from `ngay_den` instead would insert a number into a series that was closed — an issued number
// appearing in a year's register after that year ended (rule 7). ⚠ THIS IS A READING OF
// ADMINISTRATIVE PRACTICE, not a line of the specification, and it is reported as such.
func (uc *VanBanDen) Them(ctx context.Context, yc YeuCauVaoSoVanBanDen,
	nguoi audit.Actor) (domain.VanBanDen, error) {

	bayGio := uc.nayHoac()

	moi, err := chuanHoaVaoSo(yc, bayGio)
	if err != nil {
		return domain.VanBanDen{}, err
	}
	if err := coNguoiThucHienVanBan(nguoi); err != nil {
		return domain.VanBanDen{}, err
	}

	id, err := uc.sinhID()
	if err != nil {
		return domain.VanBanDen{}, fmt.Errorf("van_ban_den: sinh mã: %w", err)
	}
	moi.ID = id
	moi.Nam = bayGio.Year()
	moi.TrangThai = domain.VanBanMoiVaoSo
	// THE STAFF BUSINESS CODE, never the internal id — audit.Actor.ID already carries exactly that
	// value, because the handler reads `Principal.Ma` (rule 6, invariant 8).
	moi.NguoiTaoMa = nguoi.ID

	han, err := uc.hanXuLyXong(ctx, bayGio)
	if err != nil {
		return domain.VanBanDen{}, err
	}
	moi.HanXuLyXong = han

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// THE TYPE IS CHECKED FIRST, BEFORE THE NUMBER IS TAKEN. A booking that is going to be
		// refused must not consume a number: the counter only moves forward, so a number taken by a
		// request that then fails would leave a permanent hole in the commune's register. The order
		// of these two statements is the whole of that property.
		if err := uc.kho.LoaiVanBanConDung(ctx, tx, moi.LoaiVanBan); err != nil {
			return err
		}
		so, err := uc.daySo.CapSo(ctx, tx, docstore.SoSachDen, moi.Nam)
		if err != nil {
			return err
		}
		moi.SoVaoSo = so

		if err := uc.kho.Chen(ctx, tx, moi); err != nil {
			return err
		}

		delta, err := json.Marshal(map[string]any{"sau": tomTatVanBanDen(moi)})
		if err != nil {
			return fmt.Errorf("van_ban_den: mã hoá delta: %w", err)
		}
		// SAME TRANSACTION AS THE INSERT AND AS THE NUMBER (rule 6, invariant 3). TenantID is left
		// unset on purpose: audit.Write fills it from the transaction, which took it from the
		// context (rule 1, invariant 4). Passing it here would be a second source for the one fact
		// that decides which commune the entry belongs to.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViVaoSoVanBanDen,
			Subject: domain.MaVanBanDen(moi.Nam, moi.SoVaoSo),
			Delta:   delta,
		})
	})
	if err != nil {
		// Nothing was committed: no document, no number, no trail. The three states agree.
		return domain.VanBanDen{}, bocVanBan(ctx, "vào sổ văn bản đến", err)
	}
	return moi, nil
}

// hanXuLyXong asks identity for the one clock this register fixes.
//
// `DEADLINE_KIND_XU_LY_XONG` ALONE, AND `DEADLINE_KIND_TIEP_NHAN` DELIBERATELY NOT ASKED FOR. The
// commune's `sla` row for `van-ban-den` carries both figures, and the acknowledge clock measures how
// long a record waits before a human reads it — but an incoming document is BOOKED BY a member of
// staff, so the act that creates the row IS the reading (ADR 0028 decision E, and the same reasoning
// kb/00-foundation/ubiquitous-language.md:75 states for staff-booked petitions). Asking for it would
// store a commitment that was met at the instant it was created.
//
// ⚠ IF THE CUSTOMER MEANS "8 working hours from the date on the document until somebody books it",
// THAT IS A DIFFERENT ORIGIN and a different act, and it is a stop condition (rule 10) rather than a
// second argument here: counting from a date a clerk types would let a document be booked already
// overdue.
//
// `linhVuc` IS "" — THE DEFAULT ROW. A document register has no `lĩnh vực` concept anywhere in the
// specification, so there is no per-field row to read. It is passed straight through, never
// substituted for some "general" code.
func (uc *VanBanDen) hanXuLyXong(ctx context.Context, tuLuc time.Time) (time.Time, error) {
	han, err := uc.han.HanXuLy(ctx, identityv1.WorkKind_WORK_KIND_VAN_BAN_DEN, "", tuLuc,
		[]identityv1.DeadlineKind{identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG})
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %w", ErrChuaAnDinhDuocHan, err)
	}
	t, co := han[identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG]
	if !co || t.IsZero() {
		// identityclient already refuses a partial answer; this is the second wall, and it is here
		// because a zero time.Time reaching the column is a deadline in the year 1 — a document
		// overdue the moment it is booked.
		return time.Time{}, fmt.Errorf("%w: hạn xử lý xong rỗng", ErrChuaAnDinhDuocHan)
	}
	return t, nil
}

// Sua corrects one booked document.
//
// A NO-OP WRITES NOTHING AND AUDITS NOTHING. Sending a document the values it already has is not an
// event; recording it would fill a public authority's ledger with entries saying nothing changed,
// and those are the entries that bury the ones carrying legal weight. It is also what makes this
// route genuinely idempotent, which is what its `idem.KhongCan` declaration claims.
//
// THE NUMBER, THE YEAR, THE STATE AND THE DEADLINE ARE NOT REACHABLE FROM HERE — see the store's
// capNhatVanBanDen for which rule each absence serves.
func (uc *VanBanDen) Sua(ctx context.Context, id string, yc YeuCauSuaVanBanDen,
	nguoi audit.Actor) (domain.VanBanDen, error) {

	if id == "" {
		return domain.VanBanDen{}, docstore.ErrKhongThayVanBanDen
	}
	if err := coNguoiThucHienVanBan(nguoi); err != nil {
		return domain.VanBanDen{}, err
	}
	bayGio := uc.nayHoac()

	var sau domain.VanBanDen
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		truoc, err := uc.kho.TheoIDDeSua(ctx, tx, id)
		if err != nil {
			return err
		}

		sau = truoc
		if err := apSuaVanBanDen(&sau, yc, bayGio); err != nil {
			return err
		}
		if khongDoiVanBanDen(truoc, sau) {
			return nil
		}
		if sau.LoaiVanBan != truoc.LoaiVanBan {
			if err := uc.kho.LoaiVanBanConDung(ctx, tx, sau.LoaiVanBan); err != nil {
				return err
			}
		}
		if err := uc.kho.CapNhat(ctx, tx, sau); err != nil {
			return err
		}

		// BEFORE AND AFTER, AND ONLY THE FIELDS THAT MOVED (rule 6, invariant 5). A delta carrying
		// every column on every edit makes the one field somebody actually changed impossible to find
		// in a ledger that is never deleted.
		delta, err := json.Marshal(map[string]any{
			"van_ban_id": sau.ID,
			"truoc":      tomTatDoiVanBanDen(truoc, sau, true),
			"sau":        tomTatDoiVanBanDen(truoc, sau, false),
		})
		if err != nil {
			return fmt.Errorf("van_ban_den: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViSuaVanBanDen,
			Subject: domain.MaVanBanDen(truoc.Nam, truoc.SoVaoSo),
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.VanBanDen{}, bocVanBan(ctx, "sửa văn bản đến", err)
	}
	return sau, nil
}

// Go soft deletes one entry.
//
// THIS IS NOT A DELETE AND THE NAME IS THE ONLY PLACE THAT COULD SUGGEST OTHERWISE. The row stays,
// carrying `deleted_at`, `deleted_by` and `delete_reason` (rule 7, invariant 1); it leaves every
// screen because every read path excludes soft-deleted rows, not because anything was destroyed.
//
// THE NUMBER DOES NOT COME BACK. Nothing here touches the counter, and the unique key still counts
// this row — so the next document takes the NEXT number and the removed one leaves a visible gap.
// That gap IS the record: an archival register that silently closed its own holes would be a
// register nobody could audit.
//
// THE REASON IS MANDATORY. An entry that vanished from the register with no reason attached is a
// document nobody can account for — and the row is still there, so the question WILL be asked.
func (uc *VanBanDen) Go(ctx context.Context, id, lyDoTho string, nguoi audit.Actor) error {
	if id == "" {
		return docstore.ErrKhongThayVanBanDen
	}
	lyDo, err := domain.ChuanHoaChuoi(lyDoTho, domain.TranLyDoGo, domain.ErrThieuLyDoGo)
	if err != nil {
		return err
	}
	if err := coNguoiThucHienVanBan(nguoi); err != nil {
		return err
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		truoc, err := uc.kho.TheoIDDeSua(ctx, tx, id)
		if err != nil {
			return err
		}
		// `deleted_by` HOLDS THE STAFF BUSINESS CODE, the same value the entry's actor holds. Two
		// kinds of identifier in one column is a column nobody can query (rule 6, invariant 8).
		if err := uc.kho.XoaMem(ctx, tx, truoc.ID, nguoi.ID, lyDo); err != nil {
			return err
		}

		// THE REASON IS IN THE ENTRY AS WELL AS IN THE COLUMN, and that is not the duplication rule 9
		// forbids: the column is the current state of the row and can only ever hold the FIRST
		// removal, while the entry is the append-only record of the act. They answer two different
		// questions and neither can be derived from the other.
		delta, err := json.Marshal(map[string]any{
			"van_ban_id": truoc.ID,
			"truoc":      tomTatVanBanDen(truoc),
			"ly_do":      lyDo,
			"xoa_mem":    true,
		})
		if err != nil {
			return fmt.Errorf("van_ban_den: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViGoVanBanDen,
			Subject: domain.MaVanBanDen(truoc.Nam, truoc.SoVaoSo),
			Delta:   delta,
		})
	})
	if err != nil {
		return bocVanBan(ctx, "gỡ văn bản đến", err)
	}
	return nil
}

// Chuyen routes a document to a department — the chairman's instruction and the office's handover in
// one act (§3.5, permission `document.route`).
//
// THREE WRITES, ONE TRANSACTION, AND THAT IS THE WHOLE DESIGN: the document moves, the timeline
// records the move, and the audit trail records who did it. Split across transactions, any window
// between them is a register whose "Đang giữ" column and whose timeline disagree — and the timeline
// is append-only, so there is no repair afterwards.
//
// THE TIMELINE ENTRY IS A BUSINESS RECORD, NOT A SECOND AUDIT LOG. A clerk reads it on the screen;
// `audit_log` is invisible to the commune and answers a different question. Both are written.
func (uc *VanBanDen) Chuyen(ctx context.Context, id string, yc YeuCauChuyenVanBan,
	nguoi audit.Actor) (domain.VanBanDen, error) {

	if id == "" {
		return domain.VanBanDen{}, docstore.ErrKhongThayVanBanDen
	}
	denBoPhan, err := domain.ChuanHoaChuoi(yc.DenBoPhan, domain.TranBoPhanID, domain.ErrThieuBoPhanNhan)
	if err != nil {
		return domain.VanBanDen{}, err
	}
	// THE REASON IS MANDATORY, and that is a decision this session makes rather than one the
	// specification states: §3.5 draws the field with a placeholder and does not mark it required.
	// It is required here because the entry can never be edited afterwards (rule 7, forbidden #5) —
	// a routing with no reason is an instruction nobody can account for, and the person who gave it
	// will have moved on by the time anybody asks.
	lyDo, err := domain.ChuanHoaChuoi(yc.LyDo, domain.TranLyDoChuyen, domain.ErrThieuLyDoChuyen)
	if err != nil {
		return domain.VanBanDen{}, err
	}
	canBoMa, err := domain.ChuanHoaTuyChon(yc.CanBoXuLyMa, domain.TranMaCanBo)
	if err != nil {
		return domain.VanBanDen{}, err
	}
	if err := coNguoiThucHienVanBan(nguoi); err != nil {
		return domain.VanBanDen{}, err
	}

	luc := uc.nayHoac()
	idLichSu, err := uc.sinhID()
	if err != nil {
		return domain.VanBanDen{}, fmt.Errorf("van_ban_den: sinh mã lịch sử: %w", err)
	}

	var sau domain.VanBanDen
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		truoc, err := uc.kho.TheoIDDeSua(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := truoc.ChoChuyen(); err != nil {
			return err
		}

		trangThai := domain.TrangThaiSauKhiChuyen(truoc.TrangThai)
		if err := uc.kho.ChuyenBoPhan(ctx, tx, truoc.ID, denBoPhan, canBoMa, trangThai); err != nil {
			return err
		}
		if err := uc.kho.ChenLichSuChuyen(ctx, tx, domain.ChuyenVanBan{
			ID:                   idLichSu,
			VanBanDenID:          truoc.ID,
			ThoiDiem:             luc,
			NguoiMa:              nguoi.ID,
			TrangThaiTaiThoiDiem: trangThai,
			TuBoPhan:             truoc.BoPhanDangGiu,
			DenBoPhan:            denBoPhan,
			CanBoXuLyMa:          canBoMa,
			NoiDung:              lyDo,
		}); err != nil {
			return err
		}

		sau = truoc
		sau.BoPhanDangGiu = denBoPhan
		sau.CanBoXuLyMa = canBoMa
		sau.TrangThai = trangThai

		delta, err := json.Marshal(map[string]any{
			"van_ban_id": truoc.ID,
			"truoc": map[string]any{
				"trang_thai":   string(truoc.TrangThai),
				"bo_phan_giu":  truoc.BoPhanDangGiu,
				"can_bo_xu_ly": truoc.CanBoXuLyMa,
			},
			"sau": map[string]any{
				"trang_thai":   string(sau.TrangThai),
				"bo_phan_giu":  sau.BoPhanDangGiu,
				"can_bo_xu_ly": sau.CanBoXuLyMa,
			},
			"ly_do": lyDo,
		})
		if err != nil {
			return fmt.Errorf("van_ban_den: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViChuyenVanBanDen,
			Subject: domain.MaVanBanDen(truoc.Nam, truoc.SoVaoSo),
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.VanBanDen{}, bocVanBan(ctx, "chuyển văn bản đến", err)
	}
	return sau, nil
}

// --- shape ---------------------------------------------------------------------------------------

// chuanHoaVaoSo validates and trims one booking request. OUTSIDE THE TRANSACTION — see Them.
func chuanHoaVaoSo(yc YeuCauVaoSoVanBanDen, bayGio time.Time) (domain.VanBanDen, error) {
	var v domain.VanBanDen
	var err error

	if err = domain.KiemNgayDen(yc.NgayDen, bayGio); err != nil {
		return domain.VanBanDen{}, err
	}
	if err = domain.KiemNgayVanBanDen(yc.NgayVanBan, yc.NgayDen); err != nil {
		return domain.VanBanDen{}, err
	}
	if err = domain.KiemDoKhan(yc.DoKhan); err != nil {
		return domain.VanBanDen{}, err
	}
	if v.CoQuanBanHanh, err = domain.ChuanHoaChuoi(yc.CoQuanBanHanh, domain.TranCoQuan,
		domain.ErrThieuCoQuanBanHanh); err != nil {
		return domain.VanBanDen{}, err
	}
	if v.LoaiVanBan, err = domain.ChuanHoaChuoi(yc.LoaiVanBan, domain.TranLoaiVanBan,
		domain.ErrThieuLoaiVanBan); err != nil {
		return domain.VanBanDen{}, err
	}
	if v.TrichYeu, err = domain.ChuanHoaChuoi(yc.TrichYeu, domain.TranTrichYeu,
		domain.ErrThieuTrichYeu); err != nil {
		return domain.VanBanDen{}, err
	}
	if v.SoKyHieu, err = domain.ChuanHoaTuyChon(yc.SoKyHieu, domain.TranSoKyHieu); err != nil {
		return domain.VanBanDen{}, err
	}
	v.NgayDen = yc.NgayDen
	v.NgayVanBan = yc.NgayVanBan
	v.DoKhan = yc.DoKhan
	return v, nil
}

// apSuaVanBanDen applies a partial edit onto the row that was read under the lock.
func apSuaVanBanDen(v *domain.VanBanDen, yc YeuCauSuaVanBanDen, bayGio time.Time) error {
	if yc.NgayDen != nil {
		if err := domain.KiemNgayDen(*yc.NgayDen, bayGio); err != nil {
			return err
		}
		v.NgayDen = *yc.NgayDen
	}
	if yc.NgayVanBan != nil {
		if err := domain.KiemNgayVanBanDen(*yc.NgayVanBan, v.NgayDen); err != nil {
			return err
		}
		v.NgayVanBan = *yc.NgayVanBan
	}
	if yc.DoKhan != nil {
		if err := domain.KiemDoKhan(*yc.DoKhan); err != nil {
			return err
		}
		v.DoKhan = *yc.DoKhan
	}
	if yc.CoQuanBanHanh != nil {
		s, err := domain.ChuanHoaChuoi(*yc.CoQuanBanHanh, domain.TranCoQuan, domain.ErrThieuCoQuanBanHanh)
		if err != nil {
			return err
		}
		v.CoQuanBanHanh = s
	}
	if yc.LoaiVanBan != nil {
		s, err := domain.ChuanHoaChuoi(*yc.LoaiVanBan, domain.TranLoaiVanBan, domain.ErrThieuLoaiVanBan)
		if err != nil {
			return err
		}
		v.LoaiVanBan = s
	}
	if yc.TrichYeu != nil {
		s, err := domain.ChuanHoaChuoi(*yc.TrichYeu, domain.TranTrichYeu, domain.ErrThieuTrichYeu)
		if err != nil {
			return err
		}
		v.TrichYeu = s
	}
	if yc.SoKyHieu != nil {
		s, err := domain.ChuanHoaTuyChon(*yc.SoKyHieu, domain.TranSoKyHieu)
		if err != nil {
			return err
		}
		v.SoKyHieu = s
	}
	return nil
}

// khongDoiVanBanDen reports whether the edit would change nothing. The number, the year, the state,
// the deadline and the holding department are not compared because no path through Sua can move them.
func khongDoiVanBanDen(truoc, sau domain.VanBanDen) bool {
	return truoc.NgayDen.Equal(sau.NgayDen) &&
		truoc.NgayVanBan.Equal(sau.NgayVanBan) &&
		truoc.SoKyHieu == sau.SoKyHieu &&
		truoc.CoQuanBanHanh == sau.CoQuanBanHanh &&
		truoc.LoaiVanBan == sau.LoaiVanBan &&
		truoc.TrichYeu == sau.TrichYeu &&
		truoc.DoKhan == sau.DoKhan
}

// tomTatVanBanDen is the audit delta's view of one entry.
//
// ⚠ `trich_yeu` IS FREE TEXT AND MAY NAME A CITIZEN (rule 3). An incoming document is ordinarily
// official correspondence between bodies, but a commune that types "Đơn của ông Nguyễn Văn A, số
// điện thoại …" into the summary has put personal data into an append-only ledger that is never
// deleted. THE SUMMARY IS STILL RECORDED, because an audit entry that cannot say WHICH document was
// removed is an entry nobody can use — and it is the field an edit is most likely to touch. What
// this layer can guarantee is narrower and is stated rather than implied: nothing here logs it, and
// no error message carries it out to a client.
func tomTatVanBanDen(v domain.VanBanDen) map[string]any {
	return map[string]any{
		"van_ban_id":       v.ID,
		"so_vao_so":        v.SoVaoSo,
		"nam":              v.Nam,
		"ngay_den":         v.NgayDen.Format("2006-01-02"),
		"so_ky_hieu":       v.SoKyHieu,
		"co_quan_ban_hanh": v.CoQuanBanHanh,
		"loai_van_ban":     v.LoaiVanBan,
		"trich_yeu":        v.TrichYeu,
		"do_khan":          string(v.DoKhan),
		"trang_thai":       string(v.TrangThai),
		"han_xu_ly_xong":   v.HanXuLyXong.UTC().Format(time.RFC3339),
	}
}

// tomTatDoiVanBanDen returns only the fields that actually moved, from whichever side is asked for.
func tomTatDoiVanBanDen(truoc, sau domain.VanBanDen, ben bool) map[string]any {
	ra := map[string]any{}
	if !truoc.NgayDen.Equal(sau.NgayDen) {
		ra["ngay_den"] = chonNgay(ben, truoc.NgayDen, sau.NgayDen)
	}
	if !truoc.NgayVanBan.Equal(sau.NgayVanBan) {
		ra["ngay_van_ban"] = chonNgay(ben, truoc.NgayVanBan, sau.NgayVanBan)
	}
	if truoc.SoKyHieu != sau.SoKyHieu {
		ra["so_ky_hieu"] = chonChuoi(ben, truoc.SoKyHieu, sau.SoKyHieu)
	}
	if truoc.CoQuanBanHanh != sau.CoQuanBanHanh {
		ra["co_quan_ban_hanh"] = chonChuoi(ben, truoc.CoQuanBanHanh, sau.CoQuanBanHanh)
	}
	if truoc.LoaiVanBan != sau.LoaiVanBan {
		ra["loai_van_ban"] = chonChuoi(ben, truoc.LoaiVanBan, sau.LoaiVanBan)
	}
	if truoc.TrichYeu != sau.TrichYeu {
		ra["trich_yeu"] = chonChuoi(ben, truoc.TrichYeu, sau.TrichYeu)
	}
	if truoc.DoKhan != sau.DoKhan {
		ra["do_khan"] = chonChuoi(ben, string(truoc.DoKhan), string(sau.DoKhan))
	}
	return ra
}

func chonChuoi(truocBen bool, a, b string) string {
	if truocBen {
		return a
	}
	return b
}

// chonNgay formats the chosen side, and renders the zero date as "" rather than as the year 1 — an
// audit entry saying a date was cleared must not say it was set to 0001-01-01.
func chonNgay(truocBen bool, a, b time.Time) string {
	t := b
	if truocBen {
		t = a
	}
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

// coNguoiThucHienVanBan refuses a write whose trail cannot name who made it.
//
// core/audit refuses an entry with no actor for the same reason; refusing HERE, before the
// transaction opens, keeps `nguoi_tao_ma` / `deleted_by` / `nguoi_ma` and the entry telling the same
// story — and avoids a rollback whose cause is a missing principal rather than anything about the
// document. Rule 6 does not permit a business write whose trail cannot name its author.
func coNguoiThucHienVanBan(nguoi audit.Actor) error {
	if nguoi.ID == "" {
		return fmt.Errorf("van_ban: thiếu người thực hiện")
	}
	return nil
}

// bocVanBan wraps a failure with the commune and the operation, and NOTHING ELSE.
//
// No summary, no issuing body, no document number: an error travels into centralised logging across
// every commune at once, and the summary of an incoming document is free text about a government
// matter that may name a citizen (rule 3). The commune is not personal data and is the one thing an
// operator can act on.
//
// The chain is kept with %w so the handler can still tell a refusal from a failure with errors.Is. A
// %v here would collapse "this document is already settled" and "the database is down" into one 500.
func bocVanBan(ctx context.Context, viec string, err error) error {
	return fmt.Errorf("van_ban: %s cho xã %s: %w", viec, tenant.MustFrom(ctx), err)
}
