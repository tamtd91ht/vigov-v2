package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/phone"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// ONE FOLD, TWO COPIES, PINNED EQUAL. `citizen_contact.phone` was written by domain.NormalizeCitizenPhone;
// the petition ↔ directory link (ADR 0102 #3) matches it against core/phone.NormalizeNational, the copy
// service-petitions also uses. If the two ever disagree on one input, a person's petitions silently stop
// matching their directory row — no error, just a missing link. This test is what turns that red.
//
// WHY TWO COPIES AND NOT ONE: internal/domain imports the standard library only (every service keeps that
// rule), so it cannot call core/phone; and its callers (CheckCitizenContactInput, the Excel import, the
// notice audience) live inside domain. The pin lives here, the nearest package that may import both.

// foldInputs covers every branch of the fold, built on the agreed fake number (rule 3, invariant 5).
var foldInputs = []string{
	"", "   ", dienThoaiGia, " 0900000000 ", "0900 000 000", "0900.000.000", "090-000-0000",
	"(090) 000 0000", "+84900000000", "+84 900 000 000", "(+84) 900-000-000", "84900000000",
	"84 900 000 000", "0084900000000", "084" + "0000000", "84" + "840000000", "900000000",
	"0900", "090000000", "0" + strings.Repeat("9", 14), "0" + strings.Repeat("9", 15),
	strings.Repeat("0 ", 16), strings.Repeat("0 ", 17), "000000000", "0240000000",
	"abc", "0900000000a", "0900+000000", "+()-.", "+", "00", "84", "+84", "0900\t000000",
	"０９００００００００", "0900 000 000 ext 1",
}

func TestDirectoryFoldEqualsSharedFold(t *testing.T) {
	for _, raw := range foldInputs {
		want, wantErr := domain.NormalizeCitizenPhone(raw)
		got, gotErr := phone.NormalizeNational(raw)
		if got != want || (wantErr == nil) != (gotErr == nil) {
			t.Errorf("%q: identity fold = %q, %v; shared fold = %q, %v", raw, want, wantErr, got, gotErr)
			continue
		}
		// The refusal KIND agrees too, so a caller mapping the shared sentinels shows the same sentence.
		for _, pair := range []struct{ id, core error }{
			{domain.ErrCitizenPhoneMissing, phone.ErrMissing},
			{domain.ErrCitizenPhoneTooLong, phone.ErrTooLong},
			{domain.ErrCitizenPhoneInvalid, phone.ErrInvalid},
			{domain.ErrCitizenPhoneNotNational, phone.ErrNotNational},
		} {
			if errors.Is(wantErr, pair.id) != errors.Is(gotErr, pair.core) {
				t.Errorf("%q: identity refusal %v, shared refusal %v — different kinds", raw, wantErr, gotErr)
			}
		}
	}
}

// The identity form (`dinh_danh_cong_dan`) is derived from the national form; the shared IsNational must
// accept exactly what that derivation accepts, or the Zalo join and the petition link disagree on a row.
func TestIdentityFormAcceptsExactlyTheSharedNationalShape(t *testing.T) {
	for _, raw := range foldInputs {
		if (domain.CitizenPhoneIdentityForm(raw) != "") != phone.IsNational(raw) {
			t.Errorf("%q: identity form %q, IsNational %v", raw, domain.CitizenPhoneIdentityForm(raw), phone.IsNational(raw))
		}
	}
}
