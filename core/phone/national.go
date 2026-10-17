package phone

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// The citizen directory's NATIONAL fold (ADR 0097 D5), shared because ADR 0102 #3 makes it the JOIN KEY
// between two services: identity stores `citizen_contact.phone` in this form, and service-petitions
// derives "is this petition's sender in the directory" / "which petitions are this person's" by
// equality against it. Two implementations of one fold are two answers to "is this the same person" —
// which is why petitions must call THIS function and never write its own.
//
// NOT NormalizeVNMobile. That one answers "can SMS / ZNS reach this number" (mobile plan only, a
// dropped leading 0 restored); this one answers "what is the canonical spelling of a number a commune
// keeps on a record" (any national number, nothing guessed). Merging them would either refuse every
// landline in the directory or invent a 0 in front of a number staff typed without one.
//
// IDENTITY'S COPY: service-identity/internal/domain.NormalizeCitizenPhone performs this same fold with
// the same order and bounds. It cannot call this package — every service's internal/domain imports the
// standard library only — so the two are PINNED EQUAL by a test outside that package
// (service-identity/internal/app/citizen_contact_match_fold_test.go). Change one, that test turns red.

// The bounds, and why each is the number it is.
const (
	// MaxTypedLen bounds the TYPED text, in characters, before anything is folded — the staff
	// directory's own limit, so "what may a telephone number contain" has one answer in the system.
	MaxTypedLen = 32
	// MinNationalDigits / MaxNationalDigits are the `phone ~ '^[0-9]{9,15}$'` CHECK of identity
	// migration 0033: the stored shape. The schema refuses what the application missed.
	MinNationalDigits = 9
	MaxNationalDigits = 15
)

// typedCharset is everything a typed telephone number may contain: digits and the separators people
// actually type. A character outside it cannot be part of any telephone number.
const typedCharset = "0123456789+-()., "

// The refusals. Sentinels so a caller maps them by identity, never by text. NONE OF THEM QUOTES THE
// INPUT: a fragment of a phone number is still personal data (rule 3, forbidden #3).
var (
	// ErrMissing — nothing but whitespace.
	ErrMissing = errors.New("số điện thoại: trống")
	// ErrTooLong — more than MaxTypedLen characters as typed.
	ErrTooLong = errors.New("số điện thoại: quá dài")
	// ErrInvalid — a character no telephone number contains, or a "+" after the first digit.
	ErrInvalid = errors.New("số điện thoại: chứa ký tự không dùng được")
	// ErrNotNational — well-formed characters, but not 0 + 8..14 digits once folded.
	ErrNotNational = errors.New("số điện thoại: không phải số trong nước (bắt đầu bằng 0, từ 9 đến 15 chữ số)")
)

var nationalSeparators = strings.NewReplacer(" ", "", "(", "", ")", "", "-", "", ".", "")

// NormalizeNational brings one typed number to the NATIONAL form: digits only, leading 0 —
// "0900000000".
//
// THE FOLD, in order: surrounding whitespace trimmed; separators removed; a leading "+" dropped; a
// leading "00" (the international dialling prefix, "0084…") dropped; then a number that does NOT start
// with 0 and starts with the country code 84 gets its 84 replaced by 0. A number that already starts
// with 0 is never touched — "084…" is a real subscriber prefix, and eating its 84 would turn one person
// into another.
//
// A NUMBER WITHOUT A LEADING 0 (after the fold) IS REFUSED rather than given one: it may be a number whose
// 0 was dropped or a foreign number, and guessing writes — or matches — the wrong person.
func NormalizeNational(raw string) (string, error) {
	typed := strings.TrimSpace(raw)
	switch {
	case typed == "":
		return "", ErrMissing
	case utf8.RuneCountInString(typed) > MaxTypedLen:
		return "", ErrTooLong
	}
	for _, r := range typed {
		if !strings.ContainsRune(typedCharset, r) {
			return "", ErrInvalid
		}
	}
	// Separators go first, so "(+84) 900-000-000" is judged as "+84900000000". A "+" means "international
	// prefix follows" and only makes sense first; anywhere else it is a typo, and dropping it silently
	// would join two numbers into one.
	compact := nationalSeparators.Replace(typed)
	if strings.LastIndex(compact, "+") > 0 {
		return "", ErrInvalid
	}
	d := strings.TrimPrefix(strings.TrimPrefix(compact, "+"), "00")
	if !strings.HasPrefix(d, "0") && strings.HasPrefix(d, "84") {
		d = "0" + d[2:]
	}
	if !IsNational(d) {
		return "", ErrNotNational
	}
	return d, nil
}

// IsNational reports whether d is ALREADY in the stored shape: ASCII digits only, leading 0, within
// MinNationalDigits..MaxNationalDigits. It folds nothing — "0900 000 000" is false.
func IsNational(d string) bool {
	if len(d) < MinNationalDigits || len(d) > MaxNationalDigits || d[0] != '0' {
		return false
	}
	for i := 0; i < len(d); i++ {
		if d[i] < '0' || d[i] > '9' {
			return false
		}
	}
	return true
}
