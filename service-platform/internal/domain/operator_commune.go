package domain

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The rules an OPERATOR's input to the commune registry must pass (ADR 0048 §"Chốt của chủ dự án —
// 01/10/2026" #4–#5). They reproduce the checks of the one-off Jenkins stages `tao-xa-thang-binh` and
// `gan-mini-app-thang-binh` (deploy/Jenkinsfile), so the console cannot write a row those stages
// would have refused.

var (
	// ErrCommuneHostInvalid — not a bare, lower-case-able DNS host name.
	ErrCommuneHostInvalid = errors.New("tenant: tên miền không hợp lệ")
	// ErrCommuneHostReserved — a platform address (domain.LaTenMienDanhRieng) or the operator area's
	// own host. Never a commune's.
	ErrCommuneHostReserved = errors.New("tenant: tên miền dành riêng cho nền tảng")
	// ErrNameInvalid — empty, too long, not UTF-8, or carrying control characters.
	ErrNameInvalid = errors.New("tenant: tên xã không hợp lệ")
	// ErrReasonInvalid — a mandatory reason that is blank, too long, or not text.
	ErrReasonInvalid = errors.New("tenant: lý do không hợp lệ")
	// ErrMiniAppIDInvalid — not a Zalo App ID shape.
	ErrMiniAppIDInvalid = errors.New("mini_app: app_id không hợp lệ")
)

// Bounds. Not security thresholds — sizes of a display string and of a free-text justification,
// chosen so a pasted document cannot land in an archival record's `delta`.
const (
	MaxCommuneNameRunes = 200
	MaxReasonRunes      = 500
	MaxMiniAppIDLength  = 32
)

// ParseCommuneHost turns an operator's input into the host stored in tenant_domain.host.
//
// NORMALISED WITH NormaliseHost (the same function the edge applies to every Host header), so the
// stored value is the one ByHost will look up. But NOT REPAIRED beyond case and surrounding space: a
// port, a scheme, a path or a trailing dot means the operator meant something a host column cannot
// express, and storing the stripped remainder would register an address nobody wrote down.
//
// operatorHost is OPERATOR_HOST ("" when off). It is refused as a commune host: the operator area
// and a commune sharing one host is ADR 0048 stop condition #6.
func ParseCommuneHost(raw, operatorHost string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	h, err := NormaliseHost(trimmed)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrCommuneHostInvalid, err)
	}
	if h != trimmed {
		return "", fmt.Errorf("%w: chỉ là tên miền trần, không kèm cổng", ErrCommuneHostInvalid)
	}
	if strings.HasSuffix(h, ".") || len(h) > 253 || net.ParseIP(strings.Trim(h, "[]")) != nil {
		return "", ErrCommuneHostInvalid
	}
	labels := strings.Split(h, ".")
	if len(labels) < 2 {
		return "", ErrCommuneHostInvalid
	}
	for _, l := range labels {
		if !validLabel(l) {
			return "", ErrCommuneHostInvalid
		}
	}
	if LaTenMienDanhRieng(h) || (operatorHost != "" && h == strings.ToLower(operatorHost)) {
		return "", ErrCommuneHostReserved
	}
	return h, nil
}

// validLabel accepts LDH labels only (letters a-z, digits, '-'). An internationalised name arrives
// as punycode (`xn--…`) and passes; raw Unicode does not, because two spellings of one host that do
// not compare equal are a commune offline for a reason no log names.
func validLabel(l string) bool {
	if l == "" || len(l) > 63 || l[0] == '-' || l[len(l)-1] == '-' {
		return false
	}
	for i := 0; i < len(l); i++ {
		c := l[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}

// ValidateCommuneName trims and checks a commune's display name. The caller has already applied
// Unicode NFC (the HTTP layer does — this package imports only the standard library).
func ValidateCommuneName(raw string) (string, error) {
	return validText(raw, MaxCommuneNameRunes, false, ErrNameInvalid)
}

// ValidateReason trims and checks a mandatory reason (name correction, activation change). Line
// breaks are allowed — a reason is a sentence or two, sometimes with a ticket number on its own line.
func ValidateReason(raw string) (string, error) {
	return validText(raw, MaxReasonRunes, true, ErrReasonInvalid)
}

func validText(raw string, maxRunes int, allowNewline bool, errInvalid error) (string, error) {
	if !utf8.ValidString(raw) {
		return "", errInvalid
	}
	s := strings.TrimSpace(raw)
	if s == "" || utf8.RuneCountInString(s) > maxRunes {
		return "", errInvalid
	}
	for _, r := range s {
		if r == '\n' && allowNewline {
			continue
		}
		// NUL and every other control character: they render as nothing on a screen and as
		// something in a log or a CSV export of the trail.
		if unicode.IsControl(r) {
			return "", errInvalid
		}
	}
	return s, nil
}

// NameKey is the comparison form of a commune name for the duplicate check: lower-cased, inner
// whitespace collapsed. EXACT EQUALITY of keys is a duplicate.
//
// CASE-INSENSITIVE, DIACRITIC-SENSITIVE — deliberately the same sensitivity as the Jenkins stage it
// replaces (`lower(ten) LIKE '%thăng bình%'`, deploy/Jenkinsfile `tao-xa-thang-binh`). Folding
// diacritics would make "Xã Tân Phú" and "Xã Tấn Phú" one name; they are two words in Vietnamese and
// can be two communes, and a refusal here has no override path. Unlike the stage it compares the
// WHOLE name, not a substring ("Xã Bình" must not block "Xã Thăng Bình").
func NameKey(name string) string {
	return strings.Join(strings.Fields(strings.ToLower(name)), " ")
}

// ProvinceKey is the comparison form of a province name: NameKey with a leading "tỉnh " or
// "thành phố " removed.
//
// WHY THE PREFIX IS DROPPED: tenant.tinh_thanh is a free display string written before the
// tinh_thanh catalogue existed — the Thăng Bình row reads "Thành phố Đà Nẵng" while the catalogue row
// reads "Đà Nẵng" (migration 0005). Without this, a duplicate of an existing commune created under
// the catalogue spelling would not be seen as being in the same province. Dropping it on both sides
// can only make the check STRICTER (more names compared), never looser.
func ProvinceKey(name string) string {
	k := NameKey(name)
	for _, p := range []string{"thành phố ", "tỉnh "} {
		if rest, ok := strings.CutPrefix(k, p); ok {
			return rest
		}
	}
	return k
}

// ValidateMiniAppID checks an App ID an operator types in: ASCII digits only, 1–32 of them.
//
// STRICTER THAN migration 0006, which only refuses blank and padded values (KiemAppID). Zalo issues
// numeric App IDs (the one the Jenkins stage registered is 19 digits); refusing anything else at the
// console stops a pasted URL or name from becoming a primary key that can never be reused (the key
// keeps soft-deleted rows). The 32 is a sanity bound, not a Zalo rule.
func ValidateMiniAppID(appID string) error {
	if err := KiemAppID(appID); err != nil {
		return fmt.Errorf("%w: %w", ErrMiniAppIDInvalid, err)
	}
	if len(appID) > MaxMiniAppIDLength {
		return ErrMiniAppIDInvalid
	}
	for i := 0; i < len(appID); i++ {
		if appID[i] < '0' || appID[i] > '9' {
			return ErrMiniAppIDInvalid
		}
	}
	return nil
}
