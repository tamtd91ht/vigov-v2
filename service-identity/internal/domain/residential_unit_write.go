package domain

// The shape rules of one residential unit (thôn / tổ dân phố) as it arrives on a write route or a
// row of an import file (user decision 2026-09-29, ADR 0059 §2; 14-cau-hinh.md §2).
//
// THE FIELDS FOLLOW THE REFERENCE SYSTEM'S FORM (../vigov-require/apps/api/app/modules/org/
// schemas.py:161-186): name required, code optional and derived from the name when blank, type,
// head, household and population counts, rank. What differs, deliberately:
//
//   - A TYPED CODE IS USED EXACTLY OR REFUSED, never repaired — the rule ChuanHoaMaBoPhan states for
//     org units, for the same reason: the code is permanent (rule 7, invariant 3).
//   - THE COUNTS ARE BOUNDED ABOVE as well as at zero. The reference system bounds them at zero only;
//     a 12-digit household count is a typo that travels into a report as a number.
//
// STANDARD LIBRARY ONLY, like every file in this package. NOTHING HERE IS PERSONAL DATA (rule 3): a
// unit's name, code and counts describe a territory, which is why these refusals may be logged.

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The refusals. Sentinels, so the HTTP layer maps them to 400 by identity
// (app.IsResidentialUnitInputError). The sentences are Vietnamese, for a person.
var (
	ErrResidentialUnitNameMissing   = errors.New("thon_to_dan_pho: thiếu tên thôn / tổ dân phố")
	ErrResidentialUnitNameTooLong   = errors.New("thon_to_dan_pho: tên thôn / tổ dân phố quá dài")
	ErrResidentialUnitNameControl   = errors.New("thon_to_dan_pho: tên thôn / tổ dân phố chứa ký tự điều khiển")
	ErrResidentialUnitCodeInvalid   = errors.New("thon_to_dan_pho: mã chỉ gồm chữ thường không dấu, chữ số và dấu gạch ngang đơn, không đứng đầu hay cuối")
	ErrResidentialUnitCodeTooLong   = errors.New("thon_to_dan_pho: mã quá dài")
	ErrResidentialUnitCodeUnderived = errors.New("thon_to_dan_pho: không sinh được mã từ tên này — hãy nhập mã")
	ErrResidentialUnitOrderRange    = errors.New("thon_to_dan_pho: thứ tự phải là số nguyên từ 0 đến 9999")
	ErrResidentialUnitCountRange    = errors.New("thon_to_dan_pho: số hộ và nhân khẩu phải là số nguyên từ 0 đến 10.000.000")
	ErrResidentialUnitRefTooLong    = errors.New("thon_to_dan_pho: mã tham chiếu quá dài")
)

const (
	// MaxResidentialUnitName — "Tổ dân phố số 12 khu phố Bình An Đông" fits in a quarter of it. The
	// same bound as a unit name on the org chart (TranTenBoPhan).
	MaxResidentialUnitName = TranTenBoPhan

	// MaxResidentialUnitOrder — the same rank bound as the org chart, and the CHECK of migration 0018.
	MaxResidentialUnitOrder = TranThuTuBoPhan

	// MaxResidentialUnitCount bounds both counts. The largest commune-level unit in the country
	// holds some tens of thousands of people; ten million refuses a typo, never a real figure.
	MaxResidentialUnitCount = 10_000_000

	// maxResidentialUnitRef bounds a type code or a staff code before it reaches SQL.
	maxResidentialUnitRef = 80
)

// NormalizeResidentialUnitName trims a name and bounds it. THE CASE IS KEPT AS TYPED ("Thôn Bình An"
// is the specification's own example) — same discipline as ChuanHoaTenBoPhan. Counted in runes.
func NormalizeResidentialUnitName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	switch {
	case name == "":
		return "", ErrResidentialUnitNameMissing
	case utf8.RuneCountInString(name) > MaxResidentialUnitName:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrResidentialUnitNameTooLong, MaxResidentialUnitName)
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			// A newline inside a hamlet name breaks every address picker and every printed form.
			return "", ErrResidentialUnitNameControl
		}
	}
	return name, nil
}

// NormalizeResidentialUnitCode validates a code the CLIENT typed: [a-z0-9] runs joined by single
// '-', at most TranMaBoPhan bytes. NO CASE FOLDING AND NO REPAIR — see the file comment.
func NormalizeResidentialUnitCode(raw string) (string, error) {
	code := strings.TrimSpace(raw)
	if len(code) > TranMaBoPhan {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrResidentialUnitCodeTooLong, TranMaBoPhan)
	}
	if !laMaHopLe(code) {
		return "", ErrResidentialUnitCodeInvalid
	}
	return code, nil
}

// DeriveResidentialUnitCode derives the slug from a name: "Thôn Bình An" → "thon-binh-an", the
// specification's own example (14-cau-hinh.md §2). The same derivation as an org unit's code
// (SinhMaBoPhan), so one commune's slugs read alike across its two lists.
//
// A name with no Latin letter or digit is REFUSED rather than given a random code: the code is
// permanent, and a meaningless permanent identifier is worse than asking for one.
func DeriveResidentialUnitCode(name string) (string, error) {
	code, err := SinhMaBoPhan(name)
	if errors.Is(err, ErrTenKhongSinhDuocMa) {
		return "", ErrResidentialUnitCodeUnderived
	}
	if err != nil {
		return "", fmt.Errorf("thon_to_dan_pho: sinh mã: %w", err)
	}
	return code, nil
}

// ResidentialUnitCodeCandidate is the n-th candidate for a DERIVED code on the create form: n = 1 is
// the base, n ≥ 2 appends "-n" (MaBoPhanThuN). The import never uses it — see
// PlanResidentialUnitImport.
func ResidentialUnitCodeCandidate(base string, n int) string { return MaBoPhanThuN(base, n) }

// CheckResidentialUnitOrder bounds the rank.
func CheckResidentialUnitOrder(n int) error {
	if n < 0 || n > MaxResidentialUnitOrder {
		return ErrResidentialUnitOrderRange
	}
	return nil
}

// CheckResidentialUnitCount bounds a household or population count. nil — "not entered" — is valid
// and is NOT zero (domain.ThonToDanPho.SoHo explains why the two must stay apart).
func CheckResidentialUnitCount(n *int) error {
	if n != nil && (*n < 0 || *n > MaxResidentialUnitCount) {
		return ErrResidentialUnitCountRange
	}
	return nil
}

// CheckResidentialUnitRef bounds a type code or staff code before it reaches SQL. "" is legitimate:
// none.
func CheckResidentialUnitRef(ref string) error {
	if len(ref) > maxResidentialUnitRef {
		return ErrResidentialUnitRefTooLong
	}
	return nil
}

// FoldResidentialUnitName is the comparison key for a unit name: trimmed, case-folded. Two live units
// whose names fold to the same key are refused (ADR 0059 §2 import rules, applied to the form too so
// the form cannot create what the import refuses). NOT Unicode-normalised — see foldName.
func FoldResidentialUnitName(s string) string { return foldName(s) }
