package domain

// EXTERNAL CONTACTS — the commune's list of bodies OUTSIDE its own apparatus that residents call (the
// health station, the commune police, the power company), shown on the Mini App directory. The table,
// its CHECKs and every vendor choice are argued in migrations/0014_external_contacts.sql; the checks
// below are the SAME bounds, applied first so a refusal is a sentence rather than a constraint name.
//
// NO PUBLISH FLAG: every live row is public (0014, VENDOR CHOICES). `category` is FREE TEXT — the group
// heading the Mini App draws — not a catalogue.

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode"
)

// ExternalContact is one row of `external_contacts` as the service reads it. Address "" is NULL (no
// address); DisplayOrder nil is NULL (sorted last).
type ExternalContact struct {
	ID           string
	Name         string
	Category     string
	Phone        string
	Address      string
	DisplayOrder *int
}

// Bounds — the CHECKs of migration 0014, never retyped elsewhere.
const (
	ExternalContactNameMaxLen     = 255
	ExternalContactCategoryMaxLen = 100
	ExternalContactAddressMaxLen  = 500
	ExternalContactPhoneMaxLen    = 32
	ExternalContactPhoneMinDigits = 3
	ExternalContactPhoneMaxDigits = 15
)

var (
	ErrExternalContactNameEmpty       = errors.New("external_contact: thiếu `name`")
	ErrExternalContactNameTooLong     = errors.New("external_contact: `name` quá dài")
	ErrExternalContactCategoryEmpty   = errors.New("external_contact: thiếu `category`")
	ErrExternalContactCategoryTooLong = errors.New("external_contact: `category` quá dài")
	ErrExternalContactPhoneEmpty      = errors.New("external_contact: thiếu `phone`")
	ErrExternalContactPhoneShape      = errors.New("external_contact: `phone` không đúng dạng")
	ErrExternalContactAddressInvalid  = errors.New("external_contact: `address` chứa ký tự không hợp lệ")
	ErrExternalContactAddressTooLong  = errors.New("external_contact: `address` quá dài")
	ErrExternalContactOrderInvalid    = errors.New("external_contact: `display_order` phải là số nguyên không âm")
)

// NormalizeExternalContactName trims and bounds the institution's name (1–255, no control character).
func NormalizeExternalContactName(s string) (string, error) {
	return normalizeText(s, ExternalContactNameMaxLen, ErrExternalContactNameEmpty, ErrExternalContactNameTooLong)
}

// NormalizeExternalContactCategory trims and bounds the group heading (1–100, no control character).
// Rows sharing the EXACT text are one group — nothing here folds case or spacing, because a heading
// the commune typed is what residents read.
func NormalizeExternalContactCategory(s string) (string, error) {
	return normalizeText(s, ExternalContactCategoryMaxLen, ErrExternalContactCategoryEmpty, ErrExternalContactCategoryTooLong)
}

// NormalizeExternalContactPhone trims and checks a dial string: digits, `+`, space, `.`, `-`,
// parentheses only; 3 to 15 digits; at most 32 characters (0014's `external_contacts_phone_shape`).
//
// WHY SO STRICT: the Mini App turns the value into a `tel:` link, and a letter in it ("máy lẻ 12")
// breaks the link silently. An extension belongs in `address` or a second row.
func NormalizeExternalContactPhone(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ErrExternalContactPhoneEmpty
	}
	if len(s) > ExternalContactPhoneMaxLen {
		return "", ErrExternalContactPhoneShape
	}
	digits := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			digits++
		case c == '+', c == ' ', c == '.', c == '-', c == '(', c == ')':
		default:
			return "", ErrExternalContactPhoneShape
		}
	}
	if digits < ExternalContactPhoneMinDigits || digits > ExternalContactPhoneMaxDigits {
		return "", ErrExternalContactPhoneShape
	}
	return s, nil
}

// NormalizeExternalContactAddress trims an OPTIONAL address: blank is "" (stored NULL — 0014 refuses an
// all-blank value as a second spelling of NULL), otherwise at most 500 characters, no control character.
func NormalizeExternalContactAddress(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	if len([]rune(s)) > ExternalContactAddressMaxLen {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrExternalContactAddressTooLong, ExternalContactAddressMaxLen)
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return "", ErrExternalContactAddressInvalid
		}
	}
	return s, nil
}

// CheckExternalContactDisplayOrder is 0014's `display_order >= 0` within the INT column's range. nil
// (NULL, "no position given") is valid.
func CheckExternalContactDisplayOrder(n *int) error {
	if n != nil && (*n < 0 || *n > math.MaxInt32) {
		return ErrExternalContactOrderInvalid
	}
	return nil
}
