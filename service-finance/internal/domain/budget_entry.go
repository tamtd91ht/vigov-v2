package domain

// The batches of revenue/expenditure recorded against one leaf budget line — the `⇄ Các đợt thu, chi`
// dialog (docs/ui-ux/07-thu-chi-ngan-sach.md §5) and the `entries` calculation mode (§4.2) that sums
// them. Storage: migration 0008 (`dot_thu_chi`, `gia_tri_dot`). User decision 25/09/2026, ledger
// service-finance/thu-chi-ngan-sach-82.
//
// WHAT THE USER DECIDED AND THIS FILE ENCODES:
//
//   - A LEAF line chooses `manual` or `entries` (ValidateChosenMethod). `children` is never chosen: it
//     follows from the tree (MethodFromTree).
//   - Writing a batch does NOT switch the mode. The line's DISPLAYED figure comes from the batches
//     only while its mode is `entries` (FullSheet.Value).
//   - A line in `entries` mode that still has live batches may not gain a child
//     (ErrEntriesLineHasEntries) — decided by the main session under the user's "follow the
//     recommendations", 25/09/2026. Without live batches it becomes `children` as before.
//   - A batch is never edited. A wrong batch is removed (soft delete, reason) and recorded again.

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// BudgetEntry is one batch (table `dot_thu_chi`) with its amounts (table `gia_tri_dot`).
//
// tenant_id IS NOT A FIELD, for the reason BudgetSheet gives. `deleted_*` are not fields either: a
// removed batch never leaves the store.
type BudgetEntry struct {
	ID      string
	LineID  string
	Date    time.Time // a DATE — the day the money moved, as the commune states it
	Content string

	// Counterparty is "Đơn vị, cá nhân" — "" when not stated. PERSONAL DATA WHEN IT NAMES A PERSON
	// (migration 0008's header): never in a log line, an error message or an audit delta. The audit
	// entry records THAT it was stated and its length, never the text (rule 3; rule 6 forbidden #4).
	Counterparty string

	VoucherNo string    // "" when not stated
	EnteredBy string    // staff business code (`CB-00123`), never the internal id — rule 6 inv. 8
	CreatedAt time.Time // zero on a batch just built in memory, before the database stamped it

	// Amounts is columnID -> amount in đồng, and ONLY STATED AMOUNTS APPEAR. An absent key is an empty
	// amount (§9 rule 4), exactly as in FullSheet.Values.
	Amounts map[string]Dong
}

// LineEntries is what the `⇄` dialog reads: the line, the sheet's live columns (so the client can
// draw one amount per NUMBER column), and the line's live batches newest first.
type LineEntries struct {
	Line    BudgetLine
	Columns []BudgetColumn
	Entries []BudgetEntry
}

var (
	ErrEntryDateMissing         = errors.New("ngan_sach: thiếu `date` của đợt (YYYY-MM-DD)")
	ErrEntryDateOutOfRange      = errors.New("ngan_sach: `date` của đợt ngoài khoảng hợp lệ (2000-01-01..2100-12-31)")
	ErrEntryContentMissing      = errors.New("ngan_sach: thiếu `content` của đợt")
	ErrEntryContentTooLong      = errors.New("ngan_sach: `content` của đợt quá dài")
	ErrEntryCounterpartyTooLong = errors.New("ngan_sach: `counterparty` quá dài")
	ErrEntryVoucherNoTooLong    = errors.New("ngan_sach: `document_no` quá dài")
	ErrEntryHasNoAmount         = errors.New(
		"ngan_sach: đợt phải có ít nhất một số tiền ở một cột số — một đợt không có số không cộng vào đâu cả")

	// ErrEntryLeafOnly — a batch on a line that has children. The line's figure is the sum of its
	// children (customer decision 06/09/2026); a batch there would sit in the table counting nowhere.
	ErrEntryLeafOnly = errors.New(
		"ngan_sach: khoản mục có dòng con thì không ghi đợt — số của nó luôn cộng từ các dòng con")

	// ErrEntriesLineNoDirectValue — a figure typed into a line in `entries` mode. §4.2: the cells
	// become read-only. A typed figure there would be stored and never displayed, and would silently
	// reappear only if somebody later switched modes — which overwrites it anyway (§9.1).
	ErrEntriesLineNoDirectValue = errors.New(
		"ngan_sach: khoản mục đang cộng theo đợt — số lấy từ tổng các đợt; đổi sang `manual` trước khi gõ số")

	// ErrEntriesLineHasEntries — adding a child under a line in `entries` mode that still has live
	// batches. Accepted, the line would become `children` and every batch would silently stop counting
	// while still listed in the dialog.
	ErrEntriesLineHasEntries = errors.New(
		"ngan_sach: khoản mục đang cộng theo đợt và còn đợt chưa gỡ — gỡ các đợt hoặc đổi sang `manual` trước khi thêm dòng con")

	// ErrParentLineMethodFixed — `method` sent for a line that has children. Its mode is
	// `children` and follows the tree; neither `manual` nor `entries` can be chosen for it.
	ErrParentLineMethodFixed = errors.New(
		"ngan_sach: khoản mục có dòng con thì luôn cộng từ dòng con — không đổi được cách tính")

	// ErrEntryNotFound — "no such live batch IN THIS COMMUNE". A batch of another commune, a removed
	// batch, and a batch whose line or sheet was removed are one answer: the query reaches none of them.
	ErrEntryNotFound = errors.New("ngan_sach: không có đợt thu chi này trong xã")

	// ErrEntryTotalOverflow — the sum of a line's batches in one column is past ValueMax, or past
	// int64 altogether. REACHABLE: two batches at the ceiling already exceed it, and 2000 of them
	// exceed int64. On READ it makes THAT line's figure unavailable with this sentence (FullSheet
	// .Value) — never the whole sheet, because RemoveEntry, the remedy, reads the sheet too. On the
	// entries -> manual switch it is a refusal (409): the sum cannot become a typed figure.
	ErrEntryTotalOverflow = errors.New(
		"ngan_sach: tổng các đợt của khoản mục vượt mức một con số ngân sách có thể có — gỡ đợt ghi nhầm để tính lại")

	// ErrLineEntriesFull — recording one more batch would pass MaxEntriesPerLine. REFUSED AT THE
	// WRITE, because the `⇄` list refuses to read past the same ceiling: a batch accepted beyond it is
	// counted in the figure yet can never be listed, reconciled or removed.
	ErrLineEntriesFull = errors.New(
		"ngan_sach: khoản mục đã đủ số đợt thu chi tối đa — không ghi thêm được; gỡ bớt đợt ghi nhầm trước")
)

// MaxEntriesPerLine is the hard upper bound on one line's live batches, enforced on the write
// (ErrLineEntriesFull) and on the list read (store.ErrTooManyEntries) alike. Migration 0008 expects "a
// few dozen per leaf line per year"; 2000 is far past that and short of anything that is still a list
// a person reads. Approaching it means paging, not a bigger constant.
const MaxEntriesPerLine = 2000

// ValidateChosenMethod accepts the two modes a CLIENT may choose for a leaf: `manual` and `entries`.
// `children` is refused here with the same sentence as before — it follows the tree.
func ValidateChosenMethod(c LineMethod) error {
	if c != MethodManual && c != MethodEntries {
		return ErrMethodFromClient
	}
	return nil
}

// ValidateEntryDate — required, and inside the window migration 0008's `dot_thu_chi_ngay_hop_le`
// admits. Checked here so a typo year is a 400 with a sentence and not a 500 from the CHECK.
func ValidateEntryDate(t time.Time) error {
	if t.IsZero() {
		return ErrEntryDateMissing
	}
	if n := t.Year(); n < VoucherYearMin || n > VoucherYearMax {
		return ErrEntryDateOutOfRange
	}
	return nil
}

// NormalizeEntryContent — required. The cap is the voucher's (VoucherDescriptionMax) because migration
// 0008 reuses it: one limit for one kind of text across the module. COUNTED IN RUNES, as
// `char_length` counts: Vietnamese diacritics are 2-3 bytes each.
func NormalizeEntryContent(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", ErrEntryContentMissing
	case utf8.RuneCountInString(s) > VoucherDescriptionMax:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrEntryContentTooLong, VoucherDescriptionMax)
	case hasControlChar(s):
		return "", ErrEntryContentMissing
	}
	return s, nil
}

// NormalizeEntryCounterparty — OPTIONAL; "" means not stated and is stored as NULL. The error names the
// field and the limit, NEVER the value: this text may be a citizen's name (rule 3, forbidden #3).
func NormalizeEntryCounterparty(s string) (string, error) {
	return normalizeOptionalEntryText(s, CounterpartyMax, ErrEntryCounterpartyTooLong)
}

// NormalizeEntryVoucherNo — OPTIONAL; "" is stored as NULL.
func NormalizeEntryVoucherNo(s string) (string, error) {
	return normalizeOptionalEntryText(s, VoucherNoMax, ErrEntryVoucherNoTooLong)
}

func normalizeOptionalEntryText(s string, limit int, tooLong error) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case utf8.RuneCountInString(s) > limit:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", tooLong, limit)
	case hasControlChar(s):
		return "", tooLong
	}
	return s, nil
}
