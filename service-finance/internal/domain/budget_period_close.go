package domain

// Budget period close (chốt kỳ) — migration 0012. User decision 30/09/2026, ledger service-finance
// items `thu-chi-ngan-sach-82` (g) and `thu-chi-chot-ky-va-cong-khai`; names "budget period close",
// "reopen" and "adjustment entry" approved by the user (kb/00-foundation/ubiquitous-language.md).
//
// WHAT A CLOSE LOCKS, and the one place that decides it (EntryLockingClose / SheetLockingClose):
//
//	entry add / remove      the entry's DATE (`dot_thu_chi.ngay`) is in an active MONTH close of its
//	                        month, or in an active YEAR close of its year; OR the entry's SHEET year
//	                        (`bang_ngan_sach.nam`) has an active YEAR close.
//	sheet / line / values   the sheet's year has an active YEAR close. A MONTH close does not lock them:
//	                        hand-entered values carry no date (main-session decision, stated to the user).
//
// THE DATABASE DOES NOT REFUSE THESE WRITES (0012's header says why). The app layer asks these
// functions inside the write transaction, after the (tenant, year) advisory lock — without the lock a
// write and a close committing at the same instant could both succeed.

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// BudgetPeriodClose is one close act (table `budget_period_closes`). tenant_id is not a field, for the
// reason BangNganSach gives.
type BudgetPeriodClose struct {
	ID string
	// Code is the business code — `CK-2026-09-01`, `CK-2026-CN-01` — the audit subject of the close
	// and of its reopen. Issued once, never reissued (rule 7, invariant 3).
	Code string
	Year int
	// Month is 1..12, or 0 for the WHOLE YEAR (NULL in the table).
	Month    int
	Revision int
	ClosedAt time.Time // zero on a close just built in memory, before the database stamped it
	ClosedBy string    // staff business code, never the internal id (rule 6, invariant 8)

	// The reopen trio. ReopenedAt zero = the close is ACTIVE.
	ReopenedAt   time.Time
	ReopenedBy   string
	ReopenReason string
}

// Active reports whether this close still locks its period.
func (c BudgetPeriodClose) Active() bool { return c.ReopenedAt.IsZero() && c.ReopenedBy == "" }

// IsYearClose reports whether this close covers the whole year.
func (c BudgetPeriodClose) IsYearClose() bool { return c.Month == 0 }

// PeriodLabel is how a sentence names the period: "tháng 09/2026" or "năm 2026".
func (c BudgetPeriodClose) PeriodLabel() string { return PeriodLabel(c.Year, c.Month) }

// PeriodLabel names a period the way every sentence of this module does.
func PeriodLabel(year, month int) string {
	if month == 0 {
		return fmt.Sprintf("năm %d", year)
	}
	return fmt.Sprintf("tháng %02d/%d", month, year)
}

// ReopenReasonMax — the bound every reason on this sheet meets (0012: `budget_period_closes_reopen
// _reason_max`, `dot_thu_chi_adjustment_reason_max`, both = LyDoXoaNganSachToiDa).
const ReopenReasonMax = LyDoXoaNganSachToiDa

// AdjustmentReasonMax — same bound, same reason.
const AdjustmentReasonMax = LyDoXoaNganSachToiDa

var (
	// ErrPeriodClosed — the write falls in a closed period. Always wrapped in a *PeriodCloseConflict
	// that names the period and the close code; errors.Is matches on this sentinel.
	ErrPeriodClosed = errors.New("ngan_sach: kỳ ngân sách đã chốt")

	// ErrPeriodAlreadyClosed — a second active close of the same period. The unique index
	// `budget_period_closes_one_active` is the floor; the app checks first so the sentence can name
	// the existing close.
	ErrPeriodAlreadyClosed = errors.New(
		"ngan_sach: kỳ này đã chốt — tải lại danh sách chốt kỳ; muốn chốt lại thì mở chốt trước, kèm lý do")

	// ErrCloseAlreadyReopened — a reopen of a close that is already reopened. A reopen is recorded
	// once (0012's trigger); the way forward is a new close.
	ErrCloseAlreadyReopened = errors.New(
		"ngan_sach: lần chốt này đã được mở chốt — mỗi lần chốt chỉ mở chốt một lần; chốt lại là một lần chốt mới")

	// ErrBudgetPeriodCloseNotFound — "no such close IN THIS COMMUNE". Another commune's close and a
	// code that was never issued are one answer.
	ErrBudgetPeriodCloseNotFound = errors.New("ngan_sach: không có lần chốt kỳ này trong xã")

	ErrCloseMonthInvalid = errors.New(
		"ngan_sach: `month` phải từ 1 đến 12, hoặc bỏ trống để chốt cả năm")
	ErrReopenReasonMissing   = errors.New("ngan_sach: thiếu lý do mở chốt")
	ErrReopenReasonTooLong   = errors.New("ngan_sach: lý do mở chốt quá dài")
	ErrAdjustmentReasonBlank = errors.New(
		"ngan_sach: `adjustment_reason` không được chỉ có khoảng trắng — bỏ trường này nếu đây là đợt thường")
	ErrAdjustmentReasonTooLong = errors.New("ngan_sach: `adjustment_reason` quá dài")
)

// PeriodCloseConflict is a refusal that NAMES the close it ran into — the period and the code — so
// the commune can find it on the close list. It carries no figure and no batch text (rule 3).
type PeriodCloseConflict struct {
	Close BudgetPeriodClose
	kind  error
	msg   string
}

func (e *PeriodCloseConflict) Error() string { return e.msg }
func (e *PeriodCloseConflict) Unwrap() error { return e.kind }

// EntryPeriodClosedError — adding or removing an entry dated in (or on a sheet of) a closed period.
func EntryPeriodClosedError(c BudgetPeriodClose) error {
	return &PeriodCloseConflict{Close: c, kind: ErrPeriodClosed, msg: fmt.Sprintf(
		"ngan_sach: kỳ %s đã chốt (mã %s) — không thêm hay gỡ đợt thu, chi thuộc kỳ này. "+
			"Sai sót của kỳ đã chốt sửa bằng một đợt điều chỉnh ghi ở kỳ còn mở, kèm lý do; "+
			"hoặc người có quyền xác nhận mở chốt kỳ, kèm lý do",
		c.PeriodLabel(), c.Code)}
}

// SheetYearClosedError — editing a sheet, its lines or its values in a year that is closed.
func SheetYearClosedError(c BudgetPeriodClose) error {
	return &PeriodCloseConflict{Close: c, kind: ErrPeriodClosed, msg: fmt.Sprintf(
		"ngan_sach: năm ngân sách %d đã chốt (mã %s) — không tạo, sửa hay gỡ bảng, khoản mục hay "+
			"số liệu của năm này. Muốn sửa, người có quyền xác nhận mở chốt năm, kèm lý do",
		c.Year, c.Code)}
}

// AlreadyClosedError — closing a period that already has an active close.
func AlreadyClosedError(c BudgetPeriodClose) error {
	return &PeriodCloseConflict{Close: c, kind: ErrPeriodAlreadyClosed, msg: fmt.Sprintf(
		"ngan_sach: kỳ %s đã chốt (mã %s) — muốn chốt lại thì mở chốt lần này trước, kèm lý do",
		c.PeriodLabel(), c.Code)}
}

// AlreadyReopenedError — reopening a close that is already reopened.
func AlreadyReopenedError(c BudgetPeriodClose) error {
	return &PeriodCloseConflict{Close: c, kind: ErrCloseAlreadyReopened, msg: fmt.Sprintf(
		"ngan_sach: lần chốt %s (kỳ %s) đã được mở chốt — chốt lại là một lần chốt mới",
		c.Code, c.PeriodLabel())}
}

// ValidateClosePeriod — the year window of every budget year (KiemTraNamNganSach) and a month 1..12,
// or 0 for the whole year.
func ValidateClosePeriod(year, month int) error {
	if err := KiemTraNamNganSach(year); err != nil {
		return err
	}
	if month < 0 || month > 12 {
		return ErrCloseMonthInvalid
	}
	return nil
}

// NormaliseReopenReason — required, trimmed, ≤ ReopenReasonMax runes (as `char_length` counts).
func NormaliseReopenReason(s string) (string, error) {
	return chuanHoaBatBuoc(s, ReopenReasonMax, ErrReopenReasonMissing, ErrReopenReasonTooLong)
}

// NormaliseAdjustmentReason — OPTIONAL. nil = an ordinary entry. Present, it must say something:
// 0012's `dot_thu_chi_adjustment_reason_not_blank` refuses blank, and a blank "reason" would mark an
// entry as an adjustment that explains nothing — so it is refused rather than silently dropped.
func NormaliseAdjustmentReason(s *string) (string, error) {
	if s == nil {
		return "", nil
	}
	t := strings.TrimSpace(*s)
	switch {
	case t == "", coKyTuDieuKhien(t):
		return "", ErrAdjustmentReasonBlank
	case utf8.RuneCountInString(t) > AdjustmentReasonMax:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrAdjustmentReasonTooLong, AdjustmentReasonMax)
	}
	return t, nil
}

// BudgetPeriodCloseCode builds the close's business code: `CK-2026-09-01` for month 09 revision 1,
// `CK-2026-CN-01` for the whole year ("CN" = cả năm). BUILT, NEVER TYPED — the same reason maBang
// gives. The revision makes a re-close after a reopen a new code, so an issued code is never reissued.
func BudgetPeriodCloseCode(year, month, revision int) string {
	if month == 0 {
		return fmt.Sprintf("CK-%d-CN-%02d", year, revision)
	}
	return fmt.Sprintf("CK-%d-%02d-%02d", year, month, revision)
}

// FindActiveClose returns the active close of exactly (year, month) — month 0 = the year close.
func FindActiveClose(closes []BudgetPeriodClose, year, month int) (BudgetPeriodClose, bool) {
	for _, c := range closes {
		if c.Active() && c.Year == year && c.Month == month {
			return c, true
		}
	}
	return BudgetPeriodClose{}, false
}

// EntryLockingClose answers "may an entry dated `date` on a sheet of `sheetYear` be added or
// removed": the close that forbids it, or false.
//
// THREE WAYS TO BE LOCKED, checked in this order so the sentence names the narrowest close: the
// date's MONTH close, the date's YEAR close, the SHEET year's YEAR close. The last one matters when
// the two years differ (a 2026 sheet carrying an entry dated 2027-01-05): a closed 2026 means that
// sheet's figures are final, whatever date the entry carries.
func EntryLockingClose(closes []BudgetPeriodClose, date time.Time, sheetYear int) (BudgetPeriodClose, bool) {
	y, m := date.Year(), int(date.Month())
	if c, ok := FindActiveClose(closes, y, m); ok {
		return c, true
	}
	if c, ok := FindActiveClose(closes, y, 0); ok {
		return c, true
	}
	return FindActiveClose(closes, sheetYear, 0)
}

// SheetLockingClose answers "may a sheet of `sheetYear`, its lines or its hand-entered values be
// written": only a YEAR close locks them (see the file header).
func SheetLockingClose(closes []BudgetPeriodClose, sheetYear int) (BudgetPeriodClose, bool) {
	return FindActiveClose(closes, sheetYear, 0)
}

// LockYears returns the distinct years in ASCENDING order — the order every writer takes its
// (tenant, year) advisory locks in, so two writers needing the same two years cannot deadlock.
func LockYears(years ...int) []int {
	ra := make([]int, 0, len(years))
	seen := map[int]bool{}
	for _, y := range years {
		if !seen[y] {
			seen[y] = true
			ra = append(ra, y)
		}
	}
	sort.Ints(ra)
	return ra
}
