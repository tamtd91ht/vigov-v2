// Package phone normalises a Vietnamese MOBILE number typed or imported by staff.
//
// WHY IN core: a notice campaign's typed / imported recipients (service-comms, ADR 0099 §Sửa đổi S1) need
// the national form and a yes/no on "is this a mobile number SMS / ZNS can reach". The nearest existing
// fold, NormalizeCitizenPhone, lives in service-identity/internal/domain — another service's internal
// package, which rule 2 forbids importing. This package is the shared copy; it repeats that fold's ORDER
// on purpose (separators, a leading "+", a leading "00", then "84" → "0" only when the number does not
// already start with 0), so one number typed two ways folds to the same national form in both services.
//
// It is STRICTER than identity's fold: identity accepts any national number (landlines included) for a
// directory row; a campaign sends SMS / ZNS, which only a mobile subscription receives, so the valid set
// here is the ten-digit mobile plan: 0 followed by 3, 5, 7, 8 or 9 and eight more digits.
//
// ONE DEPARTURE FROM identity's fold, on purpose (owner's intent, 17/10/2026, relayed for TASK D-7): a
// number of NINE digits starting 3, 5, 7, 8 or 9 is read as a mobile number that lost its leading 0. That
// is what a spreadsheet does to a phone column stored as a number — and a commune's file exported from Excel
// must not turn every row invalid. Nine digits starting with one of those prefixes cannot be anything else
// in the national plan; any other nine-digit value is still refused.
//
// Neither the argument nor the result is ever put in an error or a log line by this package (rule 3).
package phone

import "strings"

// separators — what people put inside a number when they type or paste it.
var separators = strings.NewReplacer(" ", "", ".", "", "-", "", "(", "", ")", "", "\t", "", " ", "")

// NormalizeVNMobile folds raw into the national form ("0" + nine digits) and reports whether it is a
// Vietnamese mobile number. ok=false returns "" — never a half-folded guess.
//
//	"0900 000 000", "090.000.0000", "+84 900000000", "84900000000" → "0900000000", true
//	"900000000" (a spreadsheet dropped the 0)                      → "0900000000", true
//	a number that already starts with "084" is never re-folded     → eleven digits, refused
//	a landline ("024…"), "12345", "abc"                            → "", false
func NormalizeVNMobile(raw string) (string, bool) {
	s := separators.Replace(strings.TrimSpace(raw))
	// A "+" means "international prefix follows" and only makes sense first; elsewhere it is a typo, and
	// dropping it silently would join two numbers into one.
	if strings.LastIndex(s, "+") > 0 {
		return "", false
	}
	d := strings.TrimPrefix(strings.TrimPrefix(s, "+"), "00")
	if !strings.HasPrefix(d, "0") && strings.HasPrefix(d, "84") && len(d) == 11 {
		d = "0" + d[2:]
	} else if len(d) == 9 && strings.ContainsRune("35789", rune(d[0])) {
		d = "0" + d // a spreadsheet's numeric cell dropped the leading 0
	}
	if !isMobile(d) {
		return "", false
	}
	return d, true
}

// isMobile — ten ASCII digits, 0 then one of 3 5 7 8 9 (the mobile prefixes of the 2018 renumbering).
func isMobile(d string) bool {
	if len(d) != 10 || d[0] != '0' {
		return false
	}
	switch d[1] {
	case '3', '5', '7', '8', '9':
	default:
		return false
	}
	for i := 2; i < len(d); i++ {
		if d[i] < '0' || d[i] > '9' {
			return false
		}
	}
	return true
}
