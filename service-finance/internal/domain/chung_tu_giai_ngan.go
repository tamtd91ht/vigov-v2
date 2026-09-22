package domain

// The disbursement voucher — one recorded payment against one investment project
// (docs/ui-ux/06-giai-ngan.md §8.2, §11, §13 rule 3).
//
// WHERE THE RULES REALLY LIVE, said first so nothing here is mistaken for the enforcement:
//
//	CHECK (so_tien > 0)              migrations/0004:305
//	the three states                 migrations/0004:298
//	a locked voucher is frozen       migrations/0004, trigger `chung_tu_da_khoa` (:141-168)
//	a locked voucher cannot be gone  the same trigger (:161-165)
//	hard DELETE refused outright     the same file, `ho_so_luu_tru_cam_xoa_cung`
//	an unlock carries its reason     migrations/0005, `chung_tu_giai_ngan_mo_khoa_du_vet`
//
// Those are the floor, and they hold against every writer — this service, a psql prompt, an import
// job written next year. The rules restated in this file exist for ONE reason: the database answers
// with a PostgreSQL exception, and an exception that reaches an accountant in a commune says
// nothing they can act on while carrying a driver's wording into a log line. This layer refuses
// first, in Vietnamese, naming the operation.
//
// SO A DRIFT BETWEEN THIS FILE AND THE DATABASE IS A WORSE ERROR MESSAGE, NOT A HOLE. The direction
// that WOULD be a hole — this layer allowing what the database forbids — cannot happen: the
// constraint runs last and refuses, and the transaction rolls back with the audit entry inside it
// (rule 6, invariant 3).
//
// THIS PACKAGE IMPORTS NOTHING BUT THE STANDARD LIBRARY (rule 4, and doc.go).

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// TrangThaiChungTu is the lifecycle state of one voucher. §8.2: `Kế toán nhập` -> `Đã xác nhận` ->
// `Đã khoá`.
//
// A NAMED string AND NOT A BARE one, so that a state and a code cannot be passed to each other's
// parameter. The VALUES are Vietnamese without diacritics, which is ADR 0011 and is also what the
// CHECK constraint on the column admits — a third spelling here would be a row PostgreSQL refuses,
// discovered on somebody's first save.
type TrangThaiChungTu string

const (
	// ChungTuKeToanNhap — the accountant has entered it. IT ALREADY COUNTS toward "đã giải ngân":
	// §11 defines the total as the sum of vouchers "WHERE trạng thái ≥ kế toán nhập", and this is
	// the first state. Written out because a reader meeting these three for the first time will
	// assume the total waits for confirmation.
	ChungTuKeToanNhap TrangThaiChungTu = "ke-toan-nhap"

	// ChungTuDaXacNhan — somebody holding `budget.confirm` has checked it.
	ChungTuDaXacNhan TrangThaiChungTu = "da-xac-nhan"

	// ChungTuDaKhoa — frozen. No figure and no fact of the voucher may change, and it cannot be
	// removed, until somebody ELSE holding `budget.confirm` unlocks it with a reason.
	ChungTuDaKhoa TrangThaiChungTu = "da-khoa"
)

// ChungTuGiaiNgan is one voucher as this service holds it (table `chung_tu_giai_ngan`).
//
// COLUMNS DELIBERATELY ABSENT:
//
//	tenant_id     never a field. It rides in context.Context and is bound by the scoped
//	              repository (rule 1, invariant 4). A field would be a value somebody can pass.
//	deleted_at    a soft-deleted voucher never leaves the store (rule 7, invariant 2).
//	nguon_von_id  the funding-source table does not exist yet (0004's header says why), and §13
//	              rule 6 already says a voucher with no source still counts toward the total.
//
// EVERY `…ID` FIELD HERE HOLDS A STAFF BUSINESS CODE (`CB-2026-7K3M9Q`), never an internal ULID —
// the same value `audit_log.actor_id` holds, for the same reason (rule 6, invariant 8): the column
// is read years later by somebody handling an inspection, and a ULID names nobody. The migration
// states the convention once, at 0005.
type ChungTuGiaiNgan struct {
	ID     string // ULID
	DuAnID string

	// NgayChi is the date of the payment, not the date the row was typed. They differ routinely —
	// a commune enters last week's vouchers on Monday — and every cumulative chart of §4 is
	// ordered by THIS one.
	NgayChi time.Time

	// SoTien is the amount in ĐỒNG. Always > 0 — see KiemTraSoTien and open question #30.
	SoTien Dong

	NoiDung   string
	DoiTac    string // "Công ty ABC" — a company, not a person. Nothing here is personal data.
	SoChungTu string
	TrangThai TrangThaiChungTu

	NguoiNhapID    string
	NguoiXacNhanID string

	NguoiKhoaID  string
	ThoiDiemKhoa time.Time

	// The unlock ledger (migration 0005). These describe the LAST unlock; every unlock is also an
	// entry in `audit_log`, which is append-only and is where the history lives.
	NguoiMoKhoaID  string
	ThoiDiemMoKhoa time.Time
	LyDoMoKhoa     string
	SoLanMoKhoa    int
}

// DaKhoa is the one question three different callers ask, answered in one place so that a fourth
// caller cannot compare against a fourth spelling of the state.
func (c ChungTuGiaiNgan) DaKhoa() bool { return c.TrangThai == ChungTuDaKhoa }

// --- the refusals -----------------------------------------------------------------------------
//
// Separate error values rather than one error carrying a message, because the HTTP layer maps them
// to different statuses — and a caller telling them apart by comparing strings is a caller that
// breaks the day somebody fixes a typo.

var (
	// ErrChungTuDaKhoa — the voucher is frozen. The trigger refuses the same thing underneath; this
	// is the sentence an accountant can act on, and it NAMES THE WAY OUT because otherwise the
	// screen looks broken: the row is right there and the Save button did nothing.
	ErrChungTuDaKhoa = errors.New("chung_tu: chứng từ đã khoá thì không sửa, không gỡ — phải mở khoá trước (quyền `budget.confirm`)")

	// ErrChungTuChuaKhoa — asked to unlock something that is not locked. A 409, not a 400: the
	// request was well formed, the voucher is simply not in that state (somebody else got there
	// first, or the screen is stale).
	ErrChungTuChuaKhoa = errors.New("chung_tu: chứng từ này chưa khoá nên không có gì để mở")

	// ErrTuMoKhoaChungTuMinhVuaKhoa — open question #29, decided by this project on 2026-09-22 in
	// the direction that can be loosened later with one line and cannot be tightened later at all.
	//
	// WHY IT IS ITS OWN ERROR AND NOT A 403: the caller HOLDS `budget.confirm` and is allowed to
	// unlock vouchers. What is refused is this person against THIS row. Answering 403 would send
	// them to the Phân quyền screen to be granted a permission they already have.
	ErrTuMoKhoaChungTuMinhVuaKhoa = errors.New("chung_tu: người vừa khoá chứng từ không tự mở lại được — cần một cán bộ khác có quyền `budget.confirm`")

	// ErrChungTuDaXacNhan — confirming twice. The second confirmation would overwrite who confirmed
	// it and when, which is editing a historical fact (rule 7, forbidden #5).
	ErrChungTuDaXacNhan = errors.New("chung_tu: chứng từ này đã được xác nhận rồi")

	// ErrChuaXacNhanThiChuaKhoaDuoc — the lifecycle of §8.2 is a CHAIN, and locking straight from
	// `Kế toán nhập` is refused.
	//
	// WHY, because the screen draws both buttons on a `Kế toán nhập` row and this will look like a
	// bug: unlocking has to put the voucher back in the state it was in BEFORE the lock, and the
	// row does not store what that state was. Requiring the chain makes "before the lock" always
	// `Đã xác nhận`, so an unlock restores exactly what was there and invents nothing. The
	// alternative — one more column remembering the pre-lock state — is a column that exists only
	// to record a shortcut the specification does not describe.
	ErrChuaXacNhanThiChuaKhoaDuoc = errors.New("chung_tu: phải xác nhận chứng từ trước khi khoá — vòng đời là `Kế toán nhập` → `Đã xác nhận` → `Đã khoá`")

	// ErrChungTuDaKhoaRoi — locking a locked voucher. Would overwrite who locked it and when.
	ErrChungTuDaKhoaRoi = errors.New("chung_tu: chứng từ này đã khoá rồi")
)

// --- what a client may actually supply ----------------------------------------------------------

var (
	ErrThieuDuAn = errors.New("chung_tu: thiếu dự án cho chứng từ")

	// ErrTrangThaiDoTuClient — a request tried to set `trang_thai` itself.
	//
	// REFUSED RATHER THAN IGNORED, and the difference is what the client learns. The state is not
	// reachable from any layer above the store — the INSERT writes `'ke-toan-nhap'` as a literal and
	// no UPDATE outside the three lifecycle statements names the column — so ignoring it would be
	// safe and would leave the client believing it had just created a voucher already `Đã khoá`: a
	// figure nobody confirmed, frozen against editing, counting toward the commune's total. The
	// state moves through the four lifecycle routes, each with its own permission and its own entry
	// in the trail.
	ErrTrangThaiDoTuClient = errors.New("chung_tu: `status` không do client đặt — vòng đời đi qua các tuyến xác nhận / khoá / mở khoá, mỗi bước một vết kiểm toán")

	// ErrDuAnBatBien — a request tried to move a voucher to another project.
	//
	// WHY IT IS REFUSED AT ALL, when "the accountant filed it under the wrong project" is a real and
	// ordinary mistake: moving the row moves money between two totals that have already been read
	// off a screen, and it does so leaving ONE entry that reads as an edit. Removing the voucher
	// with a reason and entering it again leaves two, both naming the project they belong to, which
	// is what an inspection can actually follow.
	ErrDuAnBatBien = errors.New("chung_tu: không chuyển chứng từ sang dự án khác — hãy gỡ chứng từ kèm lý do rồi nhập lại ở dự án đúng")

	ErrSoTienKhongDuong = errors.New("chung_tu: `amount` phải lớn hơn 0 đồng")
	ErrSoTienQuaLon     = errors.New("chung_tu: `amount` vượt mức một chứng từ giải ngân cấp xã có thể có")
	ErrThieuNgayChi     = errors.New("chung_tu: thiếu `payment_date`")
	ErrNgayChiNgoaiLich = errors.New("chung_tu: `payment_date` ngoài khoảng năm hợp lệ (2000..2100)")
	ErrThieuNoiDung     = errors.New("chung_tu: thiếu `description`")
	ErrNoiDungQuaDai    = errors.New("chung_tu: `description` quá dài")
	ErrDoiTacQuaDai     = errors.New("chung_tu: `counterparty` quá dài")
	ErrSoChungTuQuaDai  = errors.New("chung_tu: `voucher_no` quá dài")
	ErrThieuLyDoMoKhoa  = errors.New("chung_tu: thiếu lý do mở khoá — mở khoá một con số đã có người ký thì phải giải thích được")
	ErrLyDoMoKhoaQuaDai = errors.New("chung_tu: lý do mở khoá quá dài")
	ErrThieuLyDoGo      = errors.New("chung_tu: thiếu lý do gỡ chứng từ")
	ErrLyDoGoQuaDai     = errors.New("chung_tu: lý do gỡ quá dài")
)

// The bounds. They are not business rules and are not pretending to be: they are the point past
// which a value stops being a voucher field and starts being a mistake or an attack. An unbounded
// client-supplied string in a government database is a liability, not a feature.
const (
	NoiDungChungTuToiDa = 1000
	DoiTacToiDa         = 300
	SoChungTuToiDa      = 100
	LyDoMoKhoaToiDa     = 500
	LyDoGoChungTuToiDa  = 500
	NamChungTuSom       = 2000
	NamChungTuMuon      = 2100

	// SoTienToiDa is one hundred thousand billion đồng (10^17), and it is a TYPO GUARD, not a
	// business ceiling.
	//
	// The specification's whole commune plans 3,3 × 10^10 đồng for a year (§14), so this is seven
	// orders of magnitude above anything real. What it catches is the class of mistake that has no
	// other symptom: an amount pasted with the thousands separators stripped, or a figure meant in
	// nghìn đồng typed as đồng. Such a voucher would push one project past 100% and drag the
	// commune's whole "đã giải ngân" total to a number leadership reads and acts on.
	//
	// IT IS DELIBERATELY NOT THE int64 CEILING. BIGINT tops out at 9,22 × 10^18, and a constraint
	// that only catches overflow catches nothing a person would ever type.
	SoTienToiDa Dong = 100_000_000_000_000_000
)

// KiemTraSoTien refuses anything that is not a positive amount.
//
// ZERO AND NEGATIVE ARE BOTH REFUSED, AND THE SECOND ONE IS OPEN QUESTION #30 — DO NOT "FIX" IT.
// A voucher of zero đồng records nothing and is only ever an import artefact. A NEGATIVE one is a
// REFUND (thu hồi tạm ứng, điều chỉnh giảm), which is a different business event with a different
// name; letting it in here would silently reduce a disbursement total that a decision has already
// quoted. `CHECK (so_tien > 0)` says the same thing in the database (0004:305), and 0004:300-304
// states outright that refunds are a question for the customer rather than a sign change.
//
// THE DETOUR TO WATCH FOR is not this function: it is editing an old voucher down to a smaller
// figure so that a refund never appears as an event. That is what the unlock ledger of migration
// 0005 makes visible.
func KiemTraSoTien(so Dong) error {
	switch {
	case so <= 0:
		return ErrSoTienKhongDuong
	case so > SoTienToiDa:
		return fmt.Errorf("%w (tối đa %d đồng)", ErrSoTienQuaLon, int64(SoTienToiDa))
	}
	return nil
}

// KiemTraNgayChi bounds the payment date.
//
// THE CHECK IS ON THE YEAR AND NOT ON "not in the future", and the difference matters: a commune
// legitimately records a payment dated a few days ahead when the decision is signed before the
// transfer clears, and refusing that would send an accountant to type a date they know is wrong. A
// year of 1026 or 20226 is a different thing entirely — a typo that makes the voucher fall out of
// every year-filtered screen while sitting in the table looking healthy, so its money is missing
// from a total with no row appearing wrong.
func KiemTraNgayChi(ngay time.Time) error {
	if ngay.IsZero() {
		return ErrThieuNgayChi
	}
	if n := ngay.Year(); n < NamChungTuSom || n > NamChungTuMuon {
		return ErrNgayChiNgoaiLich
	}
	return nil
}

// ChuanHoaNoiDung trims and validates the description — "Thanh toán đợt 3".
//
// It carries Vietnamese WITH diacritics: it is a sentence a person reads, so nothing here restricts
// the character set beyond refusing control characters, which corrupt a screen and a log line
// alike.
func ChuanHoaNoiDung(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", ErrThieuNoiDung
	case len([]rune(s)) > NoiDungChungTuToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrNoiDungQuaDai, NoiDungChungTuToiDa)
	case coKyTuDieuKhien(s):
		return "", ErrThieuNoiDung
	}
	return s, nil
}

// ChuanHoaDoiTac trims and validates the counterparty. OPTIONAL: a commune may not know it yet, and
// §8.2's own sample table shows a voucher without one.
//
// NOTHING HERE IS PERSONAL DATA AND NOTHING THAT IS SHOULD EVER ARRIVE. 0004:283-284 says it: this
// is a company ("Công ty ABC"), not a person. A commune that starts typing an individual's name and
// national ID into this box turns a disbursement register into a store of personal data under
// Decree 13 — that is a decision for the customer, not something to accommodate here.
func ChuanHoaDoiTac(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case len([]rune(s)) > DoiTacToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrDoiTacQuaDai, DoiTacToiDa)
	case coKyTuDieuKhien(s):
		return "", ErrDoiTacQuaDai
	}
	return s, nil
}

// ChuanHoaSoChungTu trims and validates the voucher number the treasury put on the paper. OPTIONAL
// for the same reason as the counterparty: §8.2's sample rows show `—`.
//
// IT IS NOT UNIQUE AND MUST NOT BECOME SO. This is a number issued by ANOTHER authority, on paper
// this system only records; a uniqueness constraint here would refuse a legitimate entry whenever
// the treasury reuses a number across years or an accountant records two lines of one document.
func ChuanHoaSoChungTu(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case len([]rune(s)) > SoChungTuToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrSoChungTuQuaDai, SoChungTuToiDa)
	case coKyTuDieuKhien(s):
		return "", ErrSoChungTuQuaDai
	}
	return s, nil
}

// ChuanHoaLyDoMoKhoa validates the reason recorded beside an unlock.
//
// MANDATORY. Open question #29, decided by this project on 2026-09-22 — not by the customer. The
// direction was chosen because it is the only one that is not one-way: adding the column later is a
// cheap migration, but the span before it exists is a span in which every unlock that HAS HAPPENED
// is unexplainable, and no source anywhere can rebuild it. That is exactly the figure an inspection
// asks about, because it is a figure somebody signed and somebody then changed.
//
// The database says the same thing underneath — `chung_tu_giai_ngan_mo_khoa_du_vet` (0005) refuses
// a row claiming an unlock with a blank reason.
func ChuanHoaLyDoMoKhoa(s string) (string, error) {
	return chuanHoaLyDo(s, LyDoMoKhoaToiDa, ErrThieuLyDoMoKhoa, ErrLyDoMoKhoaQuaDai)
}

// ChuanHoaLyDoGo validates the reason recorded beside a soft delete.
//
// MANDATORY, AND THAT IS RULE 7, INVARIANT 1: `deleted_at`, `deleted_by` AND `delete_reason`. A
// voucher that vanished from a project's total with no reason attached is money nobody can account
// for — and the row is still there, so the question WILL be asked.
func ChuanHoaLyDoGo(s string) (string, error) {
	return chuanHoaLyDo(s, LyDoGoChungTuToiDa, ErrThieuLyDoGo, ErrLyDoGoQuaDai)
}

func chuanHoaLyDo(s string, toiDa int, thieu, quaDai error) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", thieu
	case len([]rune(s)) > toiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", quaDai, toiDa)
	case coKyTuDieuKhien(s):
		return "", thieu
	}
	return s, nil
}

// coKyTuDieuKhien reports whether the string carries a control character. Tab, newline and the rest
// corrupt a screen, a CSV export and a log line alike, and a government record is read on all three.
func coKyTuDieuKhien(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

// --- the lifecycle ------------------------------------------------------------------------------
//
// ONE FUNCTION PER OPERATION rather than one `Cho(op)`: the caller names the operation at the call
// site, so a new operation cannot silently fall into a default branch that allows it.

// ChoSua reports whether the figures and facts of this voucher may be edited.
//
// The trigger refuses the same thing underneath and lists the columns one by one (0004:148-159).
// The trigger is the floor; this is the sentence, and it arrives first.
func (c ChungTuGiaiNgan) ChoSua() error {
	if c.DaKhoa() {
		return ErrChungTuDaKhoa
	}
	return nil
}

// ChoGo reports whether this voucher may be soft deleted. The trigger refuses a soft delete of a
// locked row (0004:161-165), and a HARD delete is refused on every row by `ho_so_luu_tru_cam_xoa_cung`.
func (c ChungTuGiaiNgan) ChoGo() error {
	if c.DaKhoa() {
		return ErrChungTuDaKhoa
	}
	return nil
}

// ChoXacNhan reports whether this voucher may be confirmed. From `Kế toán nhập` only.
func (c ChungTuGiaiNgan) ChoXacNhan() error {
	switch c.TrangThai {
	case ChungTuDaXacNhan:
		return ErrChungTuDaXacNhan
	case ChungTuDaKhoa:
		return ErrChungTuDaKhoa
	}
	return nil
}

// ChoKhoa reports whether this voucher may be locked. From `Đã xác nhận` only — see
// ErrChuaXacNhanThiChuaKhoaDuoc for why the chain is required even though the screen draws both
// buttons at once.
func (c ChungTuGiaiNgan) ChoKhoa() error {
	switch c.TrangThai {
	case ChungTuDaKhoa:
		return ErrChungTuDaKhoaRoi
	case ChungTuDaXacNhan:
		return nil
	default:
		return ErrChuaXacNhanThiChuaKhoaDuoc
	}
}

// ChoMoKhoa reports whether `maCanBo` may unlock this voucher.
//
// `maCanBo` IS THE STAFF BUSINESS CODE, the same value `nguoi_khoa_id` holds and the same value the
// audit trail records (rule 6, invariant 8). Comparing anything else here — an internal id, a
// display name — compares two things that are not the same kind of identifier, and the comparison
// would simply never be true, which reads as "the rule is off" rather than as a bug.
//
// AN EMPTY `maCanBo` IS REFUSED, and it is refused as a MISSING ACTOR rather than by falling
// through to "not the same person". A caller that cannot name who is acting must not perform an act
// whose entire point is that two different people performed it.
func (c ChungTuGiaiNgan) ChoMoKhoa(maCanBo string) error {
	if !c.DaKhoa() {
		return ErrChungTuChuaKhoa
	}
	if maCanBo == "" || maCanBo == c.NguoiKhoaID {
		return ErrTuMoKhoaChungTuMinhVuaKhoa
	}
	return nil
}

// TrangThaiSauKhiMoKhoa is where an unlocked voucher lands: back in `Đã xác nhận`.
//
// EXACT RATHER THAN CHOSEN. ChoKhoa only admits a lock from `Đã xác nhận`, so that IS the state the
// voucher was in immediately before, and restoring it invents nothing. Returning it to
// `Kế toán nhập` instead would erase a confirmation that really happened — including who made it,
// since `nguoi_xac_nhan_id` would then describe a state the row is no longer in.
func TrangThaiSauKhiMoKhoa() TrangThaiChungTu { return ChungTuDaXacNhan }
