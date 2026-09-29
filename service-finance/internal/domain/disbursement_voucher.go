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

// VoucherStatus is the lifecycle state of one voucher. §8.2: `Kế toán nhập` -> `Đã xác nhận` ->
// `Đã khoá`.
//
// A NAMED string AND NOT A BARE one, so that a state and a code cannot be passed to each other's
// parameter. The VALUES are Vietnamese without diacritics, which is ADR 0011 and is also what the
// CHECK constraint on the column admits — a third spelling here would be a row PostgreSQL refuses,
// discovered on somebody's first save.
type VoucherStatus string

const (
	// VoucherEntered — the accountant has entered it. IT ALREADY COUNTS toward "đã giải ngân":
	// §11 defines the total as the sum of vouchers "WHERE trạng thái ≥ kế toán nhập", and this is
	// the first state. Written out because a reader meeting these three for the first time will
	// assume the total waits for confirmation.
	VoucherEntered VoucherStatus = "ke-toan-nhap"

	// VoucherConfirmed — somebody holding `budget.confirm` has checked it.
	VoucherConfirmed VoucherStatus = "da-xac-nhan"

	// VoucherLocked — frozen. No figure and no fact of the voucher may change, and it cannot be
	// removed, until somebody ELSE holding `budget.confirm` unlocks it with a reason.
	VoucherLocked VoucherStatus = "da-khoa"
)

// DisbursementVoucher is one voucher as this service holds it (table `chung_tu_giai_ngan`).
//
// COLUMNS DELIBERATELY ABSENT:
//
//	tenant_id     never a field. It rides in context.Context and is bound by the scoped
//	              repository (rule 1, invariant 4). A field would be a value somebody can pass.
//	deleted_at    a soft-deleted voucher never leaves the store (rule 7, invariant 2).
//
// EVERY `…ID` FIELD HERE HOLDS A STAFF BUSINESS CODE (`CB-2026-7K3M9Q`), never an internal ULID —
// the same value `audit_log.actor_id` holds, for the same reason (rule 6, invariant 8): the column
// is read years later by somebody handling an inspection, and a ULID names nobody. The migration
// states the convention once, at 0005.
type DisbursementVoucher struct {
	ID                  string // ULID
	InvestmentProjectID string

	// PaymentDate is the date of the payment, not the date the row was typed. They differ routinely —
	// a commune enters last week's vouchers on Monday — and every cumulative chart of §4 is
	// ordered by THIS one.
	PaymentDate time.Time

	// Amount is the amount in ĐỒNG. Always > 0 — see ValidateAmount and open question #30.
	Amount Dong

	Description  string
	Counterparty string // "Công ty ABC" — a company, not a person. Nothing here is personal data.
	VoucherNo    string

	// FundingSourceID is which funding source this payment was drawn from (`nguon_von.id`, migration
	// 0007). OPTIONAL, AND ITS EMPTINESS IS A STATE THE SPECIFICATION DEFINES rather than a gap to
	// be closed: §13 rule 6 says a voucher with no source still counts toward "đã giải ngân" and is
	// reported separately as "đã chi nhưng chưa ghi rút từ nguồn nào" — a figure §6 prints on a real
	// commune's screen.
	//
	// "" HERE IS `NULL` IN THE DATABASE, never the empty string. The column carries
	// `CHECK (nguon_von_id IS NULL OR btrim(nguon_von_id) <> '')` (0007:274-277) precisely because ''
	// would drop a voucher out of that warning while attaching it to no source either — money missing
	// from both sides of the screen with every row looking filled in. The store spells the conversion
	// (emptyToNil) and this layer never sends a blank downward.
	FundingSourceID string

	Status VoucherStatus

	EnteredByID   string
	ConfirmedByID string

	LockedByID string
	LockedAt   time.Time

	// The unlock ledger (migration 0005). These describe the LAST unlock; every unlock is also an
	// entry in `audit_log`, which is append-only and is where the history lives.
	UnlockedByID string
	UnlockedAt   time.Time
	UnlockReason string
	UnlockCount  int
}

// IsLocked is the one question three different callers ask, answered in one place so that a fourth
// caller cannot compare against a fourth spelling of the state.
func (c DisbursementVoucher) IsLocked() bool { return c.Status == VoucherLocked }

// --- the refusals -----------------------------------------------------------------------------
//
// Separate error values rather than one error carrying a message, because the HTTP layer maps them
// to different statuses — and a caller telling them apart by comparing strings is a caller that
// breaks the day somebody fixes a typo.

var (
	// ErrVoucherLocked — the voucher is frozen. The trigger refuses the same thing underneath; this
	// is the sentence an accountant can act on, and it NAMES THE WAY OUT because otherwise the
	// screen looks broken: the row is right there and the Save button did nothing.
	ErrVoucherLocked = errors.New("chung_tu: chứng từ đã khoá thì không sửa, không gỡ — phải mở khoá trước (quyền `budget.confirm`)")

	// ErrVoucherNotLocked — asked to unlock something that is not locked. A 409, not a 400: the
	// request was well formed, the voucher is simply not in that state (somebody else got there
	// first, or the screen is stale).
	ErrVoucherNotLocked = errors.New("chung_tu: chứng từ này chưa khoá nên không có gì để mở")

	// ErrSelfUnlockAfterOwnLock — open question #29, decided by this project on 2026-09-22 in
	// the direction that can be loosened later with one line and cannot be tightened later at all.
	//
	// WHY IT IS ITS OWN ERROR AND NOT A 403: the caller HOLDS `budget.confirm` and is allowed to
	// unlock vouchers. What is refused is this person against THIS row. Answering 403 would send
	// them to the Phân quyền screen to be granted a permission they already have.
	ErrSelfUnlockAfterOwnLock = errors.New("chung_tu: người vừa khoá chứng từ không tự mở lại được — cần một cán bộ khác có quyền `budget.confirm`")

	// ErrVoucherAlreadyConfirmed — confirming twice. The second confirmation would overwrite who confirmed
	// it and when, which is editing a historical fact (rule 7, forbidden #5).
	ErrVoucherAlreadyConfirmed = errors.New("chung_tu: chứng từ này đã được xác nhận rồi")

	// ErrLockRequiresConfirmation — the lifecycle of §8.2 is a CHAIN, and locking straight from
	// `Kế toán nhập` is refused.
	//
	// WHY, because the screen draws both buttons on a `Kế toán nhập` row and this will look like a
	// bug: unlocking has to put the voucher back in the state it was in BEFORE the lock, and the
	// row does not store what that state was. Requiring the chain makes "before the lock" always
	// `Đã xác nhận`, so an unlock restores exactly what was there and invents nothing. The
	// alternative — one more column remembering the pre-lock state — is a column that exists only
	// to record a shortcut the specification does not describe.
	ErrLockRequiresConfirmation = errors.New("chung_tu: phải xác nhận chứng từ trước khi khoá — vòng đời là `Kế toán nhập` → `Đã xác nhận` → `Đã khoá`")

	// ErrVoucherAlreadyLocked — locking a locked voucher. Would overwrite who locked it and when.
	ErrVoucherAlreadyLocked = errors.New("chung_tu: chứng từ này đã khoá rồi")
)

// --- what a client may actually supply ----------------------------------------------------------

var (
	ErrInvestmentProjectMissing = errors.New("chung_tu: thiếu dự án cho chứng từ")

	// ErrStatusFromClient — a request tried to set `trang_thai` itself.
	//
	// REFUSED RATHER THAN IGNORED, and the difference is what the client learns. The state is not
	// reachable from any layer above the store — the INSERT writes `'ke-toan-nhap'` as a literal and
	// no UPDATE outside the three lifecycle statements names the column — so ignoring it would be
	// safe and would leave the client believing it had just created a voucher already `Đã khoá`: a
	// figure nobody confirmed, frozen against editing, counting toward the commune's total. The
	// state moves through the four lifecycle routes, each with its own permission and its own entry
	// in the trail.
	ErrStatusFromClient = errors.New("chung_tu: `status` không do client đặt — vòng đời đi qua các tuyến xác nhận / khoá / mở khoá, mỗi bước một vết kiểm toán")

	// ErrInvestmentProjectImmutable — a request tried to move a voucher to another project.
	//
	// WHY IT IS REFUSED AT ALL, when "the accountant filed it under the wrong project" is a real and
	// ordinary mistake: moving the row moves money between two totals that have already been read
	// off a screen, and it does so leaving ONE entry that reads as an edit. Removing the voucher
	// with a reason and entering it again leaves two, both naming the project they belong to, which
	// is what an inspection can actually follow.
	ErrInvestmentProjectImmutable = errors.New("chung_tu: không chuyển chứng từ sang dự án khác — hãy gỡ chứng từ kèm lý do rồi nhập lại ở dự án đúng")

	ErrAmountNotPositive         = errors.New("chung_tu: `amount` phải lớn hơn 0 đồng")
	ErrAmountTooLarge            = errors.New("chung_tu: `amount` vượt mức một chứng từ giải ngân cấp xã có thể có")
	ErrPaymentDateMissing        = errors.New("chung_tu: thiếu `payment_date`")
	ErrPaymentDateOutOfRange     = errors.New("chung_tu: `payment_date` ngoài khoảng năm hợp lệ (2000..2100)")
	ErrVoucherDescriptionMissing = errors.New("chung_tu: thiếu `description`")
	ErrVoucherDescriptionTooLong = errors.New("chung_tu: `description` quá dài")
	ErrCounterpartyTooLong       = errors.New("chung_tu: `counterparty` quá dài")
	ErrVoucherNoTooLong          = errors.New("chung_tu: `voucher_no` quá dài")
	ErrInvalidFundingSourceID    = errors.New("chung_tu: `funding_source_id` không phải một mã nguồn vốn hợp lệ")
	ErrUnlockReasonMissing       = errors.New("chung_tu: thiếu lý do mở khoá — mở khoá một con số đã có người ký thì phải giải thích được")
	ErrUnlockReasonTooLong       = errors.New("chung_tu: lý do mở khoá quá dài")
	ErrRemoveReasonMissing       = errors.New("chung_tu: thiếu lý do gỡ chứng từ")
	ErrRemoveReasonTooLong       = errors.New("chung_tu: lý do gỡ quá dài")
)

// The bounds. They are not business rules and are not pretending to be: they are the point past
// which a value stops being a voucher field and starts being a mistake or an attack. An unbounded
// client-supplied string in a government database is a liability, not a feature.
const (
	VoucherDescriptionMax = 1000
	CounterpartyMax       = 300
	VoucherNoMax          = 100

	// FundingSourceIDMax bounds the funding-source id a client may send. A ULID is 26 characters, so
	// this is twice over and is not a format check: what it refuses is an unbounded client string
	// being carried into a query parameter. The value is looked up against this commune's live
	// sources anyway (store.CheckLiveFundingSource), and that lookup — not this bound — is the real check.
	FundingSourceIDMax     = 64
	UnlockReasonMax        = 500
	VoucherRemoveReasonMax = 500
	VoucherYearMin         = 2000
	VoucherYearMax         = 2100

	// AmountMax is one hundred thousand billion đồng (10^17), and it is a TYPO GUARD, not a
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
	AmountMax Dong = 100_000_000_000_000_000
)

// ValidateAmount refuses anything that is not a positive amount.
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
func ValidateAmount(amount Dong) error {
	switch {
	case amount <= 0:
		return ErrAmountNotPositive
	case amount > AmountMax:
		return fmt.Errorf("%w (tối đa %d đồng)", ErrAmountTooLarge, int64(AmountMax))
	}
	return nil
}

// ValidatePaymentDate bounds the payment date.
//
// THE CHECK IS ON THE YEAR AND NOT ON "not in the future", and the difference matters: a commune
// legitimately records a payment dated a few days ahead when the decision is signed before the
// transfer clears, and refusing that would send an accountant to type a date they know is wrong. A
// year of 1026 or 20226 is a different thing entirely — a typo that makes the voucher fall out of
// every year-filtered screen while sitting in the table looking healthy, so its money is missing
// from a total with no row appearing wrong.
func ValidatePaymentDate(date time.Time) error {
	if date.IsZero() {
		return ErrPaymentDateMissing
	}
	if n := date.Year(); n < VoucherYearMin || n > VoucherYearMax {
		return ErrPaymentDateOutOfRange
	}
	return nil
}

// NormalizeVoucherDescription trims and validates the description — "Thanh toán đợt 3".
//
// It carries Vietnamese WITH diacritics: it is a sentence a person reads, so nothing here restricts
// the character set beyond refusing control characters, which corrupt a screen and a log line
// alike.
func NormalizeVoucherDescription(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", ErrVoucherDescriptionMissing
	case len([]rune(s)) > VoucherDescriptionMax:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrVoucherDescriptionTooLong, VoucherDescriptionMax)
	case hasControlChar(s):
		return "", ErrVoucherDescriptionMissing
	}
	return s, nil
}

// NormalizeCounterparty trims and validates the counterparty. OPTIONAL: a commune may not know it yet, and
// §8.2's own sample table shows a voucher without one.
//
// NOTHING HERE IS PERSONAL DATA AND NOTHING THAT IS SHOULD EVER ARRIVE. 0004:283-284 says it: this
// is a company ("Công ty ABC"), not a person. A commune that starts typing an individual's name and
// national ID into this box turns a disbursement register into a store of personal data under
// Decree 13 — that is a decision for the customer, not something to accommodate here.
func NormalizeCounterparty(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case len([]rune(s)) > CounterpartyMax:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrCounterpartyTooLong, CounterpartyMax)
	case hasControlChar(s):
		return "", ErrCounterpartyTooLong
	}
	return s, nil
}

// NormalizeVoucherNo trims and validates the voucher number the treasury put on the paper. OPTIONAL
// for the same reason as the counterparty: §8.2's sample rows show `—`.
//
// IT IS NOT UNIQUE AND MUST NOT BECOME SO. This is a number issued by ANOTHER authority, on paper
// this system only records; a uniqueness constraint here would refuse a legitimate entry whenever
// the treasury reuses a number across years or an accountant records two lines of one document.
func NormalizeVoucherNo(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case len([]rune(s)) > VoucherNoMax:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrVoucherNoTooLong, VoucherNoMax)
	case hasControlChar(s):
		return "", ErrVoucherNoTooLong
	}
	return s, nil
}

// NormalizeFundingSourceID trims and bounds the funding-source id a client sent.
//
// THE EMPTY RESULT IS A MEANINGFUL ANSWER AND NOT AN ERROR: "this payment is not recorded against
// any source" is the state §13 rule 6 defines and §6 reports, so a blank is returned as a blank and
// the store turns it into `NULL`. That is why this function does not refuse "" the way
// NormalizeRemoveReason does.
//
// WHITESPACE IS TRIMMED TO "" ON PURPOSE. A value of "   " would otherwise reach the column and be
// refused by `chung_tu_giai_ngan_nguon_von_khong_rong` (0007:274-277) as a PostgreSQL exception, when
// what the client meant — and what the screen sent — is "no source". Trimming here makes the two
// spellings of nothing into one, which is the whole point of that CHECK.
//
// IT DOES NOT VALIDATE THE FORMAT, and must not start to: `nguon_von.id` is a ULID today, and a
// format assertion here would be a second, silent copy of that decision that nothing regenerates.
// Whether the id names a LIVE source OF THIS COMMUNE is the question that actually matters, and it
// is answered inside the transaction (rule 1 — there is no foreign key, 0007:102-113 says why).
func NormalizeFundingSourceID(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case len([]rune(s)) > FundingSourceIDMax:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrInvalidFundingSourceID, FundingSourceIDMax)
	case hasControlChar(s):
		return "", ErrInvalidFundingSourceID
	}
	return s, nil
}

// NormalizeUnlockReason validates the reason recorded beside an unlock.
//
// MANDATORY. Open question #29, decided by this project on 2026-09-22 — not by the customer. The
// direction was chosen because it is the only one that is not one-way: adding the column later is a
// cheap migration, but the span before it exists is a span in which every unlock that HAS HAPPENED
// is unexplainable, and no source anywhere can rebuild it. That is exactly the figure an inspection
// asks about, because it is a figure somebody signed and somebody then changed.
//
// The database says the same thing underneath — `chung_tu_giai_ngan_mo_khoa_du_vet` (0005) refuses
// a row claiming an unlock with a blank reason.
func NormalizeUnlockReason(s string) (string, error) {
	return normalizeReason(s, UnlockReasonMax, ErrUnlockReasonMissing, ErrUnlockReasonTooLong)
}

// NormalizeRemoveReason validates the reason recorded beside a soft delete.
//
// MANDATORY, AND THAT IS RULE 7, INVARIANT 1: `deleted_at`, `deleted_by` AND `delete_reason`. A
// voucher that vanished from a project's total with no reason attached is money nobody can account
// for — and the row is still there, so the question WILL be asked.
func NormalizeRemoveReason(s string) (string, error) {
	return normalizeReason(s, VoucherRemoveReasonMax, ErrRemoveReasonMissing, ErrRemoveReasonTooLong)
}

func normalizeReason(s string, limit int, missing, tooLong error) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", missing
	case len([]rune(s)) > limit:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", tooLong, limit)
	case hasControlChar(s):
		return "", missing
	}
	return s, nil
}

// hasControlChar reports whether the string carries a control character. Tab, newline and the rest
// corrupt a screen, a CSV export and a log line alike, and a government record is read on all three.
func hasControlChar(s string) bool {
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

// CanUpdate reports whether the figures and facts of this voucher may be edited.
//
// The trigger refuses the same thing underneath and lists the columns one by one (0004:148-159).
// The trigger is the floor; this is the sentence, and it arrives first.
func (c DisbursementVoucher) CanUpdate() error {
	if c.IsLocked() {
		return ErrVoucherLocked
	}
	return nil
}

// CanRemove reports whether this voucher may be soft deleted. The trigger refuses a soft delete of a
// locked row (0004:161-165), and a HARD delete is refused on every row by `ho_so_luu_tru_cam_xoa_cung`.
func (c DisbursementVoucher) CanRemove() error {
	if c.IsLocked() {
		return ErrVoucherLocked
	}
	return nil
}

// CanConfirm reports whether this voucher may be confirmed. From `Kế toán nhập` only.
func (c DisbursementVoucher) CanConfirm() error {
	switch c.Status {
	case VoucherConfirmed:
		return ErrVoucherAlreadyConfirmed
	case VoucherLocked:
		return ErrVoucherLocked
	}
	return nil
}

// CanLock reports whether this voucher may be locked. From `Đã xác nhận` only — see
// ErrLockRequiresConfirmation for why the chain is required even though the screen draws both
// buttons at once.
func (c DisbursementVoucher) CanLock() error {
	switch c.Status {
	case VoucherLocked:
		return ErrVoucherAlreadyLocked
	case VoucherConfirmed:
		return nil
	default:
		return ErrLockRequiresConfirmation
	}
}

// CanUnlock reports whether `staffCode` may unlock this voucher.
//
// `staffCode` IS THE STAFF BUSINESS CODE, the same value `nguoi_khoa_id` holds and the same value the
// audit trail records (rule 6, invariant 8). Comparing anything else here — an internal id, a
// display name — compares two things that are not the same kind of identifier, and the comparison
// would simply never be true, which reads as "the rule is off" rather than as a bug.
//
// AN EMPTY `staffCode` IS REFUSED, and it is refused as a MISSING ACTOR rather than by falling
// through to "not the same person". A caller that cannot name who is acting must not perform an act
// whose entire point is that two different people performed it.
func (c DisbursementVoucher) CanUnlock(staffCode string) error {
	if !c.IsLocked() {
		return ErrVoucherNotLocked
	}
	if staffCode == "" || staffCode == c.LockedByID {
		return ErrSelfUnlockAfterOwnLock
	}
	return nil
}

// StatusAfterUpdate is where a voucher lands after its figures are corrected.
//
// EDITING A CONFIRMED VOUCHER SENDS IT BACK TO `Kế toán nhập`, and the sentence that decides it is
// the customer's own: *"lãnh đạo xác nhận những con số kia, không phải những con số này"*. Measured
// in `../vigov-require` commit `c3f4d6a` and recorded at
// kb/50-doi-chieu/2026-09-23-feat-m8-multitenant-foundation.md §M3.
//
// WHY IT IS NOT A CONTRADICTION OF OPEN QUESTIONS #29 AND #30. Those two settled UNLOCKING (a reason
// is mandatory, the person who locked it may not reopen it) and REFUNDS (a separate voucher, never a
// sign change). Neither says anything about what happens to a CONFIRMATION when the figures beneath
// it move — this fills exactly that gap and takes nothing back from either.
//
// A LOCKED VOUCHER NEVER REACHES HERE. CanUpdate refuses it first and the `chung_tu_da_khoa` trigger
// refuses the same UPDATE underneath, so the only way to correct a frozen figure is still to unlock
// it — with a reason, by somebody else — and that path is unchanged.
//
// THE OTHER TWO STATES ARE UNTOUCHED, and the switch is written out rather than defaulted so that a
// fourth state cannot silently inherit this behaviour: `Kế toán nhập` is already where this would
// send it, and `Đã khoá` cannot get here at all.
func StatusAfterUpdate(before VoucherStatus) VoucherStatus {
	if before == VoucherConfirmed {
		return VoucherEntered
	}
	return before
}

// StatusAfterUnlock is where an unlocked voucher lands: back in `Đã xác nhận`.
//
// EXACT RATHER THAN CHOSEN. CanLock only admits a lock from `Đã xác nhận`, so that IS the state the
// voucher was in immediately before, and restoring it invents nothing. Returning it to
// `Kế toán nhập` instead would erase a confirmation that really happened — including who made it,
// since `nguoi_xac_nhan_id` would then describe a state the row is no longer in.
func StatusAfterUnlock() VoucherStatus { return VoucherConfirmed }
