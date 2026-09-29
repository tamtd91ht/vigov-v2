package domain

// The three-tier rules of a reference catalogue, as pure functions.
//
// WHERE THE RULES REALLY LIVE, said first so nothing here is mistaken for the enforcement:
// migrations/0003_danh_muc_hang_muc_ke_hoach_von.sql, function `danh_muc_ba_tang`, a BEFORE UPDATE OR
// DELETE trigger. That is the floor, and it holds against every writer — this service, a psql
// prompt, a future import job. The rules restated here exist for ONE reason: the trigger answers
// with a PostgreSQL exception, and an exception that reaches a member of staff says nothing they
// can act on and carries a driver's wording into a log line. This layer refuses first, in
// Vietnamese, naming the operation and the tier.
//
// SO A DRIFT BETWEEN THIS FILE AND THE TRIGGER IS NOT A HOLE — it is a worse error message. The
// direction that would be a hole, this layer allowing what the trigger allows but the rules
// forbid, cannot happen: the trigger runs last and refuses, and the transaction rolls back with
// the audit entry inside it (rule 6, invariant 3).
//
// WHY THE SAME FILE EXISTS IN FIVE SERVICES. Each service owns its own catalogue table (ADR 0024)
// and the services are separate Go modules; the only shared home would be core/, and this package
// imports nothing but the standard library on purpose (doc.go). The copies are identical by
// intent — see the note above for why a drift costs a message rather than a guarantee.

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// The two values `nguon` may hold. The CHECK constraint on every catalogue table admits no third.
//
// THE COMMUNE NEVER SUPPLIES THIS. A write route writes SourceCommune as a literal; `nguon` decides
// which tier a row is in, so a client that could name it could put its own row in tier 2 and then
// walk around every guard below. The migration says the same thing where the trigger refuses an
// edit of the column: "Were it editable, every guard below could be stepped around by setting
// nguon = 'don-vi' first."
const (
	SourceCommune = "don-vi"   // the commune added this row itself — tier 1
	SourceSystem  = "he-thong" // the row ships with the software — tier 2, or 3 with BranchedInSource
)

// Tier is which of the three tiers a row is in. DERIVED from `nguon` and `ma_nguon_re_nhanh`,
// never stored: two sources for one fact drift, and the stale one is what a screen would read.
type Tier int

const (
	// TierCommune — the commune's own row. Soft delete YES, disable YES, relabel YES.
	TierCommune Tier = 1
	// TierSystem — ships with the software. Soft delete NO, disable YES, relabel YES.
	TierSystem Tier = 2
	// TierBranched — ships with the software AND the source code branches on its `ma`.
	// Soft delete NO, disable NO, relabel YES. Relabelling is the only operation left, and it is
	// deliberately still allowed: the wording on a screen is the commune's, the code is not.
	TierBranched Tier = 3
)

// TierOf answers which tier a row is in.
//
// The pairing is not symmetric and that is the schema's own constraint, not a simplification here:
// `hang_muc_ke_hoach_von_re_nhanh_thi_he_thong` refuses ma_nguon_re_nhanh on a `don-vi` row, because the
// software cannot branch on a code it has never seen. A row that somehow held both would be
// reported as tier 3 by this function — the stricter reading, which is the correct direction to
// be wrong in.
func TierOf(source string, branchedInSource bool) Tier {
	switch {
	case branchedInSource:
		return TierBranched
	case source == SourceSystem:
		return TierSystem
	default:
		return TierCommune
	}
}

// Tier of one catalogue row.
func (l CapitalPlanCategory) Tier() Tier { return TierOf(l.Source, l.BranchedInSource) }

// The refusals. Separate values rather than one error with a message, because the HTTP layer maps
// them to different statuses and a caller telling them apart by string comparison is a caller that
// breaks when somebody fixes a typo.
var (
	// ErrSystemRowNotDeletable — tiers 2 and 3. A system row is taken out of use, never deleted
	// (ADR 0024, consequence #4; docs/ui-ux/14-cau-hinh.md:182).
	ErrSystemRowNotDeletable = errors.New("danh_muc: mục do hệ thống cấp không xoá được, chỉ tắt được")

	// ErrBranchedRowNotDisablable — tier 3. The specification allows the one operation the system
	// cannot survive: disabling a code the source code branches on leaves that branch with no
	// reachable row, and the screen offering the button reports nothing wrong.
	ErrBranchedRowNotDisablable = errors.New("danh_muc: mã nguồn có nhánh rẽ theo mục này nên không tắt được")

	// ErrCodeImmutable — an issued code is never renumbered (rule 7, invariant 3). Business records
	// hold this code AS A VALUE and nothing rewrites them.
	ErrCodeImmutable = errors.New("danh_muc: `ma` đã cấp thì không đổi được — sửa `nhan`, hoặc thêm dòng mới")

	// ErrSourceFromClient — the request named `nguon` or `ma_nguon_re_nhanh`. Refused BEFORE
	// anything is written, with a message that says why, rather than being silently dropped: a
	// field silently ignored is a client that believes it set something.
	ErrSourceFromClient = errors.New("danh_muc: `source` và tầng của mục do hệ thống quyết định, không nhận từ yêu cầu")
)

// Cho phép hoặc từ chối từng thao tác. One function per operation rather than one `Cho(op)`: the
// caller names the operation at the call site, so a new operation cannot silently fall into a
// default branch that allows it.

// CanSoftDelete reports whether this row may be soft deleted. Tier 1 only.
func (t Tier) CanSoftDelete() error {
	if t != TierCommune {
		return fmt.Errorf("%w (tầng %d)", ErrSystemRowNotDeletable, int(t))
	}
	return nil
}

// CanDisable reports whether this row may be taken out of use. Tiers 1 and 2.
//
// RE-ENABLING IS NOT THE SAME QUESTION and is always allowed — the trigger only refuses the
// transition true -> false. A tier-3 row that somehow ended up disabled must be able to come back.
func (t Tier) CanDisable() error {
	if t == TierBranched {
		return ErrBranchedRowNotDisablable
	}
	return nil
}

// --- validation of what a client may actually supply ---------------------------------------------

var (
	ErrCodeEmpty           = errors.New("danh_muc: thiếu `code`")
	ErrCodeInvalidFormat   = errors.New("danh_muc: `code` chỉ gồm chữ thường a-z, số và dấu gạch ngang, ví dụ `cong-van`")
	ErrCodeTooLong         = errors.New("danh_muc: `code` quá dài")
	ErrLabelEmpty          = errors.New("danh_muc: thiếu `label`")
	ErrLabelTooLong        = errors.New("danh_muc: `label` quá dài")
	ErrSortOrderOutOfRange = errors.New("danh_muc: `order` ngoài khoảng cho phép")
	ErrDeleteReasonMissing = errors.New("danh_muc: thiếu lý do xoá")
	ErrDeleteReasonTooLong = errors.New("danh_muc: lý do xoá quá dài")
)

// The bounds. They are not business rules and are not pretending to be: they are the point past
// which a value stops being a catalogue entry and starts being a mistake or an attack. An
// unbounded client-supplied string in a government database is a liability, not a feature — the
// same argument PhienStore.Tao makes for the user-agent column.
const (
	CodeMax         = 64
	LabelMax        = 200
	SortOrderMax    = 9999
	DeleteReasonMax = 500
)

// NormalizeCode trims and validates a catalogue code.
//
// THE FORM IS `tiếng Việt không dấu`, kebab-case — `cong-van`, `quyet-dinh` — and that is ADR 0011,
// not a preference: catalogue VALUES stay Vietnamese without diacritics while only the surrounding
// contract is English. Accepting anything else here would let a commune mint `Công Văn` or
// `cong_van`, and the code goes straight into business records as a value nothing ever rewrites.
//
// NO CASE FOLDING. `Cong-Van` is REFUSED rather than lower-cased: silently changing a code the
// person typed means the code stored is not the code they saw, on the one column rule 7 never lets
// us correct afterwards.
func NormalizeCode(code string) (string, error) {
	code = strings.TrimSpace(code)
	switch {
	case code == "":
		return "", ErrCodeEmpty
	case len(code) > CodeMax:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrCodeTooLong, CodeMax)
	}
	// Hand-rolled rather than a regexp: the rule is four lines, and a regexp here would be one more
	// thing to read carefully in five copies of this file.
	if code[0] == '-' || code[len(code)-1] == '-' {
		return "", ErrCodeInvalidFormat
	}
	prevWasHyphen := false
	for i := 0; i < len(code); i++ {
		c := code[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			prevWasHyphen = false
		case c == '-':
			if prevWasHyphen {
				// `cong--van` reads as one code and sorts as another. Refuse it while it is still
				// a typo rather than after it is a value in a document record.
				return "", ErrCodeInvalidFormat
			}
			prevWasHyphen = true
		default:
			return "", ErrCodeInvalidFormat
		}
	}
	return code, nil
}

// NormalizeLabel trims and validates a display label.
//
// THE LABEL IS THE ONE FIELD EVERY TIER MAY CHANGE, including tier 3 where it is the only operation
// left. It carries Vietnamese WITH diacritics — it is a sentence a person reads — so nothing here
// restricts the character set beyond refusing control characters, which would corrupt a screen and
// a log line alike.
func NormalizeLabel(label string) (string, error) {
	label = strings.TrimSpace(label)
	switch {
	case label == "":
		return "", ErrLabelEmpty
	case len([]rune(label)) > LabelMax:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrLabelTooLong, LabelMax)
	}
	for _, r := range label {
		if unicode.IsControl(r) {
			return "", ErrLabelEmpty
		}
	}
	return label, nil
}

// ValidateSortOrder bounds the display order.
//
// NEGATIVE IS REFUSED, not clamped. A client sending -1 to mean "first" would work until a second
// client sent -2, and the order a commune arranged its own catalogue in would then depend on who
// edited last.
func ValidateSortOrder(sortOrder int) error {
	if sortOrder < 0 || sortOrder > SortOrderMax {
		return fmt.Errorf("%w (0..%d)", ErrSortOrderOutOfRange, SortOrderMax)
	}
	return nil
}

// NormalizeDeleteReason validates the reason recorded beside a soft delete.
//
// MANDATORY, AND THAT IS RULE 7, INVARIANT 1: `deleted_at`, `deleted_by` AND `delete_reason`. A
// row that disappeared from every screen with no reason attached is a row nobody can explain when
// somebody asks why a document type vanished — and the row is still there, so the question WILL be
// asked.
func NormalizeDeleteReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	switch {
	case reason == "":
		return "", ErrDeleteReasonMissing
	case len([]rune(reason)) > DeleteReasonMax:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrDeleteReasonTooLong, DeleteReasonMax)
	}
	return reason, nil
}
