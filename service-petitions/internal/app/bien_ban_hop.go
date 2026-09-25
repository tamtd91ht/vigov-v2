package app

// The THREE STAFF acts on the meeting-minutes register — record the minutes (§4), append a
// conclusion (§2's last row), and SPLIT A CONCLUSION INTO A TASK (§3).
//
// Until this file a commune could READ its minutes and record none: the register had one GET and no
// way to put anything in it, and §1's whole purpose — "Nhập một biên bản, tách thành nhiều nhiệm vụ.
// Mỗi nhiệm vụ giữ liên kết ngược về kết luận gốc để truy vết được về sau" — had no server behind it.
//
// # WHY THIS LAYER EXISTS
//
// Rule 6, invariant 3 requires the audit entry to share a transaction with the business write, and
// core/audit.Write takes a *store.ScopedTx with no overload that writes outside one. Opening that
// transaction is this layer's job. The handler translates HTTP and nothing else; the store knows SQL
// and nothing else.
//
// ---------------------------------------------------------------------------
// ⚠ THE SPLIT DOES NOT WRITE A TASK OF ITS OWN. IT CALLS THE TASK USE CASE.
//
// `TachKetLuanThanhNhiemVu` resolves the conclusion and then hands the whole thing to
// GhiNhiemVu.Tao — the same use case POST /api/v1/tasks runs — with `nguon_giao` and `nguon_id`
// filled in. It does NOT insert a task itself, and the difference is not tidiness:
//
//	mã nhiệm vụ         minted from the commune's `NV…` series inside the transaction, against
//	                    `UNIQUE (tenant_id, ma)`
//	cây việc con        the cycle check and the parent lock of ADR 0037
//	hạn                 written ONCE into `han_xu_ly` and `han_ban_dau` together (ADR 0028)
//	nhật ký + vết       the first timeline row and the audit entry, in that same transaction
//
// A second create path would be a second place all five live, and the second place is the one
// nobody remembers to change when a rule moves. What this file adds is the ORIGIN of the task, and
// nothing else.
//
// ---------------------------------------------------------------------------
// ⚠ SPLITTING TAKES THE FULL TASK BODY. IT DOES NOT GUESS A TITLE OUT OF THE CONCLUSION.
//
// §3 opens the "Giao việc mới" modal PRE-FILLED from the conclusion and waits for a person to
// confirm it; the running sibling implementation records what the alternative costs in real cases —
// three times out of four the guess is right, and the fourth leaves a task with the wrong title in a
// register that cannot be deleted, only withdrawn. So the confirmation is part of the act, and at
// this layer that means the request carries the same fields POST /api/v1/tasks carries. Nothing here
// derives a title, and nothing here derives a DEADLINE out of "báo cáo trước ngày 20/8" — a deadline
// is working-hours arithmetic owned by identity (rule 10, forbidden #2; ADR 0007), and §3's hint is
// a suggestion the SCREEN makes into a date field the person can see.
//
// ---------------------------------------------------------------------------
// WHAT THIS FILE DELIBERATELY DOES NOT DO:
//
//   - NO EDIT AND NO DELETE ROUTE for minutes or conclusions. Whether a conclusion tasks have
//     already been split from may still be reworded is a STOP CONDITION (migration 0007 declared no
//     immutability trigger precisely so the answer stays open), and §7.1 says removing one is "không
//     xoá cứng, cảnh báo và giữ liên kết" — which needs the warning flow, not just an UPDATE.
//   - NO LIMIT ON HOW MANY TASKS ONE CONCLUSION PRODUCES. §3 says MANY and counts them `x/y`.
//   - NO SECOND AUDIT ENTRY ON THE SPLIT. One act leaves one entry — `tao_nhiem_vu`, carrying the
//     source pair — and an extra entry written outside GhiNhiemVu.Tao's transaction would be exactly
//     the "write the record now, write the trail afterwards if it works" shape rule 6, forbidden #2
//     refuses.

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// KhoBienBanGhi is the write half of the meeting register as this layer needs it, declared at the
// point of use.
//
// FOUR OF THE FIVE METHODS TAKE THE TRANSACTION. That is what makes it impossible to record minutes
// in one transaction and the trail in another: there is no signature here that would let you.
//
// KetLuanTheoThuTu IS THE ONE THAT DOES NOT, and its asymmetry is the design: the split act's write
// happens inside GhiNhiemVu.Tao's transaction, so this read is a precondition rather than part of
// the write. See TachKetLuanThanhNhiemVu for what that window costs and why it is empty today.
//
// *petstore.BienBanHopStore satisfies it as it is.
type KhoBienBanGhi interface {
	TaoBienBan(ctx context.Context, tx *store.ScopedTx, b domain.BienBanHop) error
	BienBanTheoIDDeSua(ctx context.Context, tx *store.ScopedTx, id string) (domain.BienBanHop, error)
	ThuTuLonNhat(ctx context.Context, tx *store.ScopedTx, bienBanID string) (int, error)
	TaoKetLuan(ctx context.Context, tx *store.ScopedTx, k domain.KetLuanHop) error

	KetLuanTheoThuTu(ctx context.Context, bienBanID string, thuTu int) (domain.KetLuanHop, error)
}

// TaoNhiemVuTuNguon is the ONE act this file borrows from the task register: booking a task.
//
// A ONE-METHOD INTERFACE AND NOT *GhiNhiemVu ITSELF. The split may create a task and may do nothing
// else to one — it must not be able to move a status, grant an extension or remove a row — and a
// dependency wider than the work requires is how the next person justifies using it for something
// else. *GhiNhiemVu satisfies this as it is; nothing was changed to accommodate it.
type TaoNhiemVuTuNguon interface {
	Tao(ctx context.Context, yc YeuCauTaoNhiemVu, nguoi audit.Actor) (domain.NhiemVu, error)
}

// The business verbs written into the trail. Vietnamese snake_case, like every other action this
// system already writes (`dang_nhap`, `vao_so_van_ban_den`, `tao_nhiem_vu`) — an inspection reads
// these strings, and a function name would tell them nothing.
//
// THERE IS NO `tach_ket_luan_thanh_nhiem_vu` VERB, and its absence is deliberate: the split IS the
// creation of a task, it leaves `tao_nhiem_vu` naming the register number, and that entry's delta
// carries the source pair. Two verbs for one act would make an inspection count it twice.
const (
	HanhViTaoBienBanHop = "tao_bien_ban_hop"
	HanhViThemKetLuan   = "them_ket_luan_hop"
)

// GhiBienBanHop owns the three staff acts.
type GhiBienBanHop struct {
	db  *store.DB
	kho KhoBienBanGhi

	// nhiemVu is the task register's own create use case — see the header. It is a DEPENDENCY rather
	// than an inherited method set, so this file cannot grow a second way to write a task.
	nhiemVu TaoNhiemVuTuNguon

	// sinhID is injected so a test can pin every generated id. In production it is ulid.Moi.
	sinhID func() (string, error)

	// nay is the clock the RETURNED record carries in `tao_luc`, so the card the clerk sees straight
	// after saving shows an instant rather than the year 1.
	//
	// ⚠ IT IS NOT WHAT THE COLUMN STORES. `tao_luc` takes the schema's `now()` default (the store
	// writes no value for it), which keeps one clock for the whole register — so these two can differ
	// by the round trip. The same trade the task register makes, and it is stated rather than hidden:
	// nothing in this system COUNTS from `tao_luc`, it only orders the register's own page.
	nay func() time.Time
}

func NewGhiBienBanHop(db *store.DB, kho KhoBienBanGhi, nhiemVu TaoNhiemVuTuNguon) *GhiBienBanHop {
	return &GhiBienBanHop{db: db, kho: kho, nhiemVu: nhiemVu, sinhID: ulid.Moi}
}

// nayHoac is the clock, UTC. `TIMESTAMPTZ` stores an instant rather than a wall reading, so
// the location changes nothing that is stored — it is fixed so a value read back in a test compares
// equal without a location dance.
func (uc *GhiBienBanHop) nayHoac() time.Time {
	if uc.nay == nil {
		return time.Now().UTC()
	}
	return uc.nay().UTC()
}

// --- 1. nhập biên bản (§4) ------------------------------------------------------------------------

// YeuCauTaoBienBan is "Nhập biên bản" as it arrives from the handler.
//
// # THE ABSENCES ARE THE DESIGN
//
// There is no `NguoiTaoMa`: that is the acting principal, and a request that could name its own
// author is a request that can forge the trail. There is no `DinhKem`: there is no file store in
// this repository, so an attachment list on the wire would be a promise nothing keeps. And there is
// no id: the register mints it.
type YeuCauTaoBienBan struct {
	TenCuocHop string

	// NgayHop is a CALENDAR DAY (§4: "Ngày họp | date"), required. Nothing counts from it — see
	// domain.ChuanHoaNgayHop for why the time of day is cut off before the column sees it.
	NgayHop time.Time

	SoHieu   string
	DiaDiem  string
	ChuTriMa string
	NoiDung  string

	// ThanhPhan is §4's attendee list: staff codes and/or free text.
	//
	// ⚠ IT IS NOT A PLACE FOR CITIZEN DETAILS (rule 3) — migration 0007 says so on the column and
	// domain.KiemThanhPhan repeats it, because no check in this system can tell a staff name from a
	// citizen's once it is free text.
	ThanhPhan []string

	// KetLuan is §4's dynamic list, IN THE ORDER THE CLERK TYPED THEM — which becomes ① ② ③. Empty
	// is a real, supported state: §7.3 saves minutes with no conclusions ("nhập nháp trước, bổ sung
	// sau"), and the card then shows `0 kết luận`.
	KetLuan []string
}

// TaoBienBan records one meeting's minutes, with its conclusions, in ONE transaction.
//
// # WHY THE CONCLUSIONS ARE IN THE SAME TRANSACTION AND NOT A FOLLOW-UP LOOP
//
// A meeting whose minutes landed and whose conclusions did not is a card showing `0 kết luận` for a
// meeting that reached three of them — and nobody would know which, because the request is gone.
// One transaction means the act either happened or did not.
//
// THE ORDINALS ARE 1..n IN THE ORDER SENT, minted here rather than read back, because these are the
// FIRST conclusions of a meeting that did not exist a statement ago: there is nothing to be
// continuous with. Appending later goes through ThemKetLuan, which reads the high-water mark.
func (uc *GhiBienBanHop) TaoBienBan(ctx context.Context, yc YeuCauTaoBienBan, nguoi audit.Actor) (
	domain.BienBanHop, error) {

	moi, noiDungKetLuan, err := chuanHoaTaoBienBan(yc)
	if err != nil {
		return domain.BienBanHop{}, err
	}
	if err := coCanBoThucHien(nguoi); err != nil {
		return domain.BienBanHop{}, err
	}
	moi.NguoiTaoMa = nguoi.ID

	if moi.ID, err = uc.sinhID(); err != nil {
		return domain.BienBanHop{}, fmt.Errorf("bien_ban_hop: sinh mã nội bộ: %w", err)
	}

	bayGio := uc.nayHoac()
	moi.TaoLuc = bayGio
	// What the column's DEFAULT writes (migration 0012) — stated on the returned record so the 201
	// reply says `du-thao` rather than an empty status. The INSERT does not send it: new minutes are
	// always drafts, and the schema is the one place that says so.
	moi.TrangThai = domain.TrangThaiBienBanDuThao

	moi.KetLuan = make([]domain.KetLuanHop, 0, len(noiDungKetLuan))
	for i, nd := range noiDungKetLuan {
		id, err := uc.sinhID()
		if err != nil {
			return domain.BienBanHop{}, fmt.Errorf("ket_luan_hop: sinh mã nội bộ: %w", err)
		}
		moi.KetLuan = append(moi.KetLuan, domain.KetLuanHop{
			ID:        id,
			BienBanID: moi.ID,
			ThuTu:     i + 1,
			NoiDung:   nd,
			TaoLuc:    bayGio,
		})
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if err := uc.kho.TaoBienBan(ctx, tx, moi); err != nil {
			return err
		}
		for _, k := range moi.KetLuan {
			if err := uc.kho.TaoKetLuan(ctx, tx, k); err != nil {
				return err
			}
		}

		// THE DELTA NAMES THE RECORD, NOT ITS CONTENTS. `noi_dung` — the minutes in full — and the
		// text of every conclusion are deliberately absent: the audit ledger is append-only and never
		// deleted (rule 6, invariant 4), so anything put in it is permanent, and a commune's minutes
		// quote cases (rule 3, forbidden #4). What an inspection needs from this entry is WHICH
		// minutes were filed and how much was in them, which is what the lengths and the count give.
		delta, err := json.Marshal(map[string]any{
			"sau": map[string]any{
				"ten_cuoc_hop":    moi.TenCuocHop,
				"ngay_hop":        moi.NgayHop.Format("2006-01-02"),
				"so_hieu":         moi.SoHieu,
				"dia_diem":        moi.DiaDiem,
				"chu_tri_ma":      moi.ChuTriMa,
				"so_ket_luan":     len(moi.KetLuan),
				"so_thanh_phan":   len(moi.ThanhPhan),
				"do_dai_noi_dung": len([]rune(moi.NoiDung)),
			},
		})
		if err != nil {
			return fmt.Errorf("bien_ban_hop: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViTaoBienBanHop,
			Subject: chuDeBienBan(moi),
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.BienBanHop{}, bocBienBan(ctx, "nhập biên bản họp", err)
	}
	return moi, nil
}

// chuanHoaTaoBienBan validates and trims the request. IT RUNS BEFORE THE TRANSACTION OPENS: a
// rejected form must hold no row lock on a government register.
func chuanHoaTaoBienBan(yc YeuCauTaoBienBan) (domain.BienBanHop, []string, error) {
	var moi domain.BienBanHop

	ten, err := domain.KiemTenCuocHop(yc.TenCuocHop)
	if err != nil {
		return moi, nil, err
	}
	ngay, err := domain.ChuanHoaNgayHop(yc.NgayHop)
	if err != nil {
		return moi, nil, err
	}
	soHieu, err := domain.KiemVanBanTuyChon(yc.SoHieu, domain.SoHieuBienBanToiDa,
		domain.ErrSoHieuBienBanQuaDai)
	if err != nil {
		return moi, nil, err
	}
	diaDiem, err := domain.KiemVanBanTuyChon(yc.DiaDiem, domain.DiaDiemBienBanToiDa,
		domain.ErrDiaDiemBienBanQuaDai)
	if err != nil {
		return moi, nil, err
	}
	chuTri, err := domain.KiemVanBanTuyChon(yc.ChuTriMa, domain.ChuTriMaToiDa, domain.ErrChuTriQuaDai)
	if err != nil {
		return moi, nil, err
	}
	noiDung, err := domain.KiemVanBanTuyChon(yc.NoiDung, domain.NoiDungBienBanToiDa,
		domain.ErrNoiDungBienBanQuaDai)
	if err != nil {
		return moi, nil, err
	}
	thanhPhan, err := domain.KiemThanhPhan(yc.ThanhPhan)
	if err != nil {
		return moi, nil, err
	}

	if len(yc.KetLuan) > domain.KetLuanMoiLanToiDa {
		return moi, nil, domain.ErrQuaNhieuKetLuan
	}
	// EVERY CONCLUSION IS CHECKED BEFORE ANY IS WRITTEN. Validating inside the loop that inserts
	// would file the first two and refuse the third — and the transaction would roll the first two
	// back, so the clerk would lose the whole form to a typo in the last box either way. Refusing
	// here means the message names the box before anything is attempted.
	ketLuan := make([]string, 0, len(yc.KetLuan))
	for _, nd := range yc.KetLuan {
		nd, err := domain.KiemNoiDungKetLuan(nd)
		if err != nil {
			return moi, nil, err
		}
		ketLuan = append(ketLuan, nd)
	}

	moi = domain.BienBanHop{
		TenCuocHop: ten,
		NgayHop:    ngay,
		SoHieu:     soHieu,
		DiaDiem:    diaDiem,
		ChuTriMa:   chuTri,
		NoiDung:    noiDung,
		ThanhPhan:  thanhPhan,
	}
	return moi, ketLuan, nil
}

// --- 2. thêm một kết luận (§2's last row) ----------------------------------------------------------

// YeuCauThemKetLuan is the single-textarea row at the bottom of every card.
type YeuCauThemKetLuan struct{ NoiDung string }

// ThemKetLuan appends one conclusion to minutes that already exist.
//
// # THE MINUTES ARE READ `FOR UPDATE` AND THAT IS WHAT MAKES §7.2 TRUE
//
// The ordinal is a read-decide-write: read the highest number ever issued in this meeting, mint the
// next one, insert. Two clerks typing into the same card at the same moment would otherwise both
// read ③ and both mint ④, and `UNIQUE (tenant_id, bien_ban_id, thu_tu)` would refuse the second with
// a constraint error instead of giving it ⑤. Locking the PARENT serialises them on the one row both
// acts have in common.
//
// THE HIGH-WATER MARK COUNTS SOFT-DELETED CONCLUSIONS. A removed ② keeps its number for ever — the
// printed minutes still say ② and the tasks split from it still point at it — so the next conclusion
// is ④ and the gap is correct (rule 7, invariant 3).
func (uc *GhiBienBanHop) ThemKetLuan(ctx context.Context, bienBanID string, yc YeuCauThemKetLuan,
	nguoi audit.Actor) (domain.KetLuanHop, error) {

	noiDung, err := domain.KiemNoiDungKetLuan(yc.NoiDung)
	if err != nil {
		return domain.KetLuanHop{}, err
	}
	if err := coCanBoThucHien(nguoi); err != nil {
		return domain.KetLuanHop{}, err
	}

	id, err := uc.sinhID()
	if err != nil {
		return domain.KetLuanHop{}, fmt.Errorf("ket_luan_hop: sinh mã nội bộ: %w", err)
	}

	moi := domain.KetLuanHop{ID: id, BienBanID: bienBanID, NoiDung: noiDung, TaoLuc: uc.nayHoac()}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		bb, err := uc.kho.BienBanTheoIDDeSua(ctx, tx, bienBanID)
		if err != nil {
			return err
		}
		lonNhat, err := uc.kho.ThuTuLonNhat(ctx, tx, bienBanID)
		if err != nil {
			return err
		}
		moi.ThuTu = domain.ThuTuKetLuanTiepTheo(lonNhat)

		if err := uc.kho.TaoKetLuan(ctx, tx, moi); err != nil {
			return err
		}

		// THE TEXT OF THE CONCLUSION IS NOT IN THE DELTA, only its length — see TaoBienBan.
		delta, err := json.Marshal(map[string]any{
			"sau": map[string]any{
				"bien_ban_id":     bienBanID,
				"thu_tu":          moi.ThuTu,
				"do_dai_noi_dung": len([]rune(moi.NoiDung)),
			},
		})
		if err != nil {
			return fmt.Errorf("ket_luan_hop: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViThemKetLuan,
			Subject: chuDeKetLuan(bb, moi.ThuTu),
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.KetLuanHop{}, bocBienBan(ctx, "thêm kết luận họp", err)
	}
	return moi, nil
}

// --- 3. tách kết luận thành nhiệm vụ (§3) ----------------------------------------------------------

// TachKetLuanThanhNhiemVu books a task whose origin is one conclusion of one meeting.
//
// # THE BACK-LINK IS THE BLURRED PAIR, AND NOTHING ELSE
//
// `nguon_giao = 'ket-luan-hop'` + `nguon_id = <conclusion id>` — the two columns migration 0006
// declared and 0007 deliberately did NOT duplicate with a `ket_luan_id` of its own. Both are set
// HERE, from the conclusion this method just read, and neither is taken from the request: a client
// that could name its own `nguon_id` could point a task at any record it liked, and §3 shows the
// field as locked for exactly that reason.
//
// # WHY THE CONCLUSION IS READ OUTSIDE THE TRANSACTION THAT WRITES THE TASK
//
// The write is GhiNhiemVu.Tao's, and it opens its own transaction — which is the point: the task,
// its first timeline row and its audit entry stay in ONE transaction, exactly as they are when the
// same act comes from POST /api/v1/tasks. This read is a PRECONDITION, so that a task cannot be
// created pointing at a conclusion that does not exist in this commune (migration 0007: such a task
// is counted SHORT by the badge, silently, for ever).
//
// THE WINDOW THAT LEAVES, STATED RATHER THAN DISCOVERED: between this read and that insert, the
// conclusion could in principle be soft-deleted. It cannot today — no route in this service can
// remove a conclusion at all — and when §7.1's removal flow is built, the honest closure is a source
// check INSIDE GhiNhiemVu.Tao's transaction rather than a lock held across two.
func (uc *GhiBienBanHop) TachKetLuanThanhNhiemVu(ctx context.Context, bienBanID string, thuTu int,
	yc YeuCauTaoNhiemVu, nguoi audit.Actor) (domain.NhiemVu, error) {

	k, err := uc.kho.KetLuanTheoThuTu(ctx, bienBanID, thuTu)
	if err != nil {
		return domain.NhiemVu{}, bocBienBan(ctx, "tách kết luận thành nhiệm vụ", err)
	}

	// SET HERE, UNCONDITIONALLY, AND NOT READ FROM THE REQUEST. The HTTP body of this route carries
	// no `source` field at all (see tachKetLuanVao), so there is nothing to refuse — the pair cannot
	// be influenced from outside, which is stronger than validating it.
	yc.NguonGiao = string(domain.NguonKetLuanHop)
	yc.NguonID = k.ID

	// EVERY OTHER RULE — the minted number, the cycle check, the deadline written once, the timeline
	// row and the audit entry in one transaction — is the task register's, unchanged. The error is
	// returned AS IT IS so the handler maps it exactly as it maps the same refusal from
	// POST /api/v1/tasks: one act, one set of answers, whichever door it came through.
	return uc.nhiemVu.Tao(ctx, yc, nguoi)
}

// --- shared ---------------------------------------------------------------------------------------

// chuDeBienBan and chuDeKetLuan are the audit subjects.
//
// NEITHER IS A ULID, for the reason rule 6, invariant 8 gives for `actor_id`: an entry is read years
// later by somebody handling a complaint or an inspection, and `bien-ban-hop/2026-08-05/31/BB-UBND`
// names a document that exists on paper, while a ULID names nothing and the row it points at may by
// then have been removed.
//
// THE MINUTES HAVE NO BUSINESS CODE OF THEIR OWN, and that is why these are COMPOSED rather than
// read from a column: `so_hieu` is optional, is typed by hand and restarts every year, so migration
// 0007 refuses a unique key on it — and minting a `BB…` series would be inventing a numbering scheme
// for archival records, which is the customer's to decide, not a write path's. The composition of
// the meeting DAY and the reference number is what a clerk uses to find the document in the folder.
// `khong-so` is the honest filler for minutes that carry no number (§4 marks it optional, and the
// prototype's draft has none) — an empty segment there would read as a truncated subject.
//
// THE REFERENCE NUMBER IS KEPT AS PRINTED, slash and all (`31/BB-UBND`): an inspection searches for
// the string on the document, not for a re-spelling of it.
//
// NONE OF IT IS PERSONAL DATA (rule 3): a date and a document number say which meeting was held.
func chuDeBienBan(b domain.BienBanHop) string {
	so := b.SoHieu
	if so == "" {
		so = "khong-so"
	}
	return "bien-ban-hop/" + b.NgayHop.Format("2006-01-02") + "/" + so
}

func chuDeKetLuan(b domain.BienBanHop, thuTu int) string {
	return chuDeBienBan(b) + "/ket-luan/" + itoa(thuTu)
}

// itoa keeps strconv out of the import list for one call. It is here rather than in domain because
// an audit subject is this layer's business.
func itoa(n int) string { return fmt.Sprintf("%d", n) }

// bocBienBan wraps a failure with the commune and the operation, and NOTHING ELSE.
//
// NOT THE TITLE, NOT THE MINUTES, NOT THE TEXT OF A CONCLUSION. An error travels into centralised
// logging across every commune at once (rule 3, invariants 1 and 2); the commune is not personal
// data and is the one thing an operator can act on.
//
// The chain is kept with %w so the handler can still tell a refusal from a failure with errors.Is. A
// %v here would collapse "these minutes do not exist" and "the database is down" into one 500.
func bocBienBan(ctx context.Context, viec string, err error) error {
	return fmt.Errorf("bien_ban_hop: %s cho xã %s: %w", viec, tenant.MustFrom(ctx), err)
}
