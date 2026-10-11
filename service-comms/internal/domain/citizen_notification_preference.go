package domain

// CITIZEN NOTIFICATION PREFERENCE — whether ONE citizen of ONE commune receives Zalo messages about
// their own records (migration 0027; owner decision 11/10/2026, ADR 0091 §4). The table, its key and
// "absent row = enabled" are argued in the migration header.
//
// ABSENT IS ENABLED, AND THAT IS NOT A DEFAULT ON AN ISOLATION PATH. Rule 10 invariant 5 owes every
// citizen the messages of ADR 0041's table; the only thing that may stop them is the citizen's own act,
// which is what writes a row. Reading "no row" as "off" would silence every citizen nobody asked (ADR
// 0091 open question #2 records this as the recorder's inference, not yet the owner's words).

import (
	"errors"
	"strings"
	"unicode"
)

// CitizenNotificationPreference is the switch as the service reads it. Stored false means no row exists
// — ZaloEnabled is then true by the rule above, never by a zero value.
type CitizenNotificationPreference struct {
	ZaloEnabled bool
	Stored      bool
}

// DefaultCitizenNotificationPreference is the answer for a citizen with no row: enabled, not stored.
func DefaultCitizenNotificationPreference() CitizenNotificationPreference {
	return CitizenNotificationPreference{ZaloEnabled: true}
}

// CitizenIDMaxLen is 0027's `citizen_id_shape` bound.
const CitizenIDMaxLen = 64

// ErrCitizenIDInvalid — the opaque citizen id is blank, too long, or holds a space or control
// character (0027's CHECK). The value is never echoed: it is a handle on a person.
var ErrCitizenIDInvalid = errors.New("citizen_notification_preference: định danh công dân không hợp lệ")

// CheckCitizenID is 0027's `citizen_id_shape` on one value.
func CheckCitizenID(id string) error {
	if strings.TrimSpace(id) == "" || len(id) > CitizenIDMaxLen {
		return ErrCitizenIDInvalid
	}
	for _, r := range id {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return ErrCitizenIDInvalid
		}
	}
	return nil
}
