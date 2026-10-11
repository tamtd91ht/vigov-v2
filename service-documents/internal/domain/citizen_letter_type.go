package domain

// The commune's LABEL TIER of "Loại đơn thư" (ADR 0039 §Hệ quả cho ADR 0024 §2: "tầng nhãn theo xã của
// danh mục Loại đơn thư thuộc documents"). Served on the Cấu hình › Danh mục screen beside the other
// catalogues (prototype LookupTable.tsx:37 `petition_type: "Loại đơn thư"`; bug sheet row 33).
//
// WHAT A ROW IS: the commune's label, order, colour and on/off switch FOR ONE OF C4's FOUR STATUTORY
// CODES (LetterTypes). It is NOT a new letter type. The register's `citizen_letter.type` CHECK admits
// those four codes and nothing else, each carries its own statutory procedure (ProtectsIdentity,
// RequiresIssuedResult, HasResolutionDeadline), and so does the prototype's own database
// (`ck_petitions_petition_type`, vigov-require models.py:183). A fifth code here would be a label no
// letter can ever be booked under, and a commune reading it on the Danh mục screen would believe the
// register now accepts a fifth kind of letter. So the code is REFUSED unless it is one of the four
// (ErrLetterTypeCodeNotStatutory) — the one place this catalogue is narrower than the prototype's.
//
// WHAT IT DOES NOT DO (yet): the register still prints its own fixed labels; reading the commune's label
// from here is a separate change.

import (
	"errors"
	"strings"
)

// CitizenLetterTypeEntry is one row of table `citizen_letter_type` (migration 0010).
type CitizenLetterTypeEntry struct {
	ID     string
	Code   LetterType // one of LetterTypes — never renumbered, never reissued (rule 7, invariant 3)
	Label  string
	Order  int
	Active bool
	Color  string // `#rrggbb` lower-case, "" = none chosen
}

// ErrLetterTypeCodeNotStatutory — the code is not one of C4's four. Refused, never stored.
var ErrLetterTypeCodeNotStatutory = errors.New("danh_muc: `code` của loại đơn thư phải là một trong " +
	"kien-nghi-phan-anh, khieu-nai, to-cao, de-nghi — bốn loại đơn có thủ tục theo luật; " +
	"sổ đơn thư không nhận loại thứ năm")

// NormalizeCitizenLetterTypeCode checks the shape (ChuanHoaMa) and then the closed set.
func NormalizeCitizenLetterTypeCode(code string) (LetterType, error) {
	c, err := ChuanHoaMa(code)
	if err != nil {
		return "", err
	}
	if !LetterType(c).Valid() {
		return "", ErrLetterTypeCodeNotStatutory
	}
	return LetterType(c), nil
}

// CitizenLetterTypeDeleteDefaultReason is `delete_reason` when the person removing a row types none —
// the owner's decision of 10/10/2026 for catalogue and directory deletes (identity's
// StaffDeleteDefaultReason, comms' MapFieldDeleteDefaultReason). Rule 7, invariant 1 still holds: the
// column is never empty, and the sentence says the reason was not given rather than inventing one.
const CitizenLetterTypeDeleteDefaultReason = "Xoá khỏi danh mục Loại đơn thư (người xoá không nhập lý do)"

// NormalizeCitizenLetterTypeDeleteReason trims the reason; blank becomes the fixed sentence.
func NormalizeCitizenLetterTypeDeleteReason(reason string) (string, error) {
	if strings.TrimSpace(reason) == "" {
		return CitizenLetterTypeDeleteDefaultReason, nil
	}
	return ChuanHoaLyDoXoa(reason)
}
