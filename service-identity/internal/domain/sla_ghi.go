package domain

import "errors"

// The rules a change to one deadline row must satisfy, in the one place that knows them.
//
// THEY LIVE IN domain/ AND NOT IN THE HANDLER because they are properties of a commitment, not of
// HTTP: the seeding path and the edit path both have to hold them, and a copy in each is a copy
// that drifts. domain/ imports nothing but the standard library (rule 4 of the service pattern).
//
// THE DATABASE ALSO CHECKS THE LOWER BOUND (sla_gio_phai_duong, migration 0008) AND THAT IS NOT
// DUPLICATION WORTH REMOVING. The CHECK is the floor that holds against every path including a
// restore or a hand-written UPDATE; this one is the sentence a person reads on the configuration
// screen. Deleting either leaves one of those two jobs undone.

var (
	// ErrGioPhaiDuong — zero working hours is a deadline breached at the instant it is made, and a
	// negative one is a typo. Migration 0008 argues at length why the bound is strict from the start:
	// a 0 that has been stored acquires a meaning somebody invented ("no alert", "immediately") that
	// cannot afterwards be told apart from a commune that mistyped it.
	ErrGioPhaiDuong = errors.New("sla: số giờ phải lớn hơn 0")

	// ErrGioQuaLon — an UPPER bound that the database deliberately does not have.
	//
	// WHY IT EXISTS HERE AND NOT AS A CHECK: migration 0008 refuses an upper bound in SQL because a
	// constraint that rejects real configuration is discovered on the day a commune is being set up,
	// and nobody could say what the real ceiling is. This is a different thing — a typed-input guard
	// on the ONE path a person types into. 8760 working hours is a full calendar YEAR of wall-clock
	// time, so no genuine commune deadline reaches it; what does reach it is a slipped digit, and a
	// slipped digit here is a commitment to a citizen that is ten times too long with nothing on the
	// screen to say so.
	ErrGioQuaLon = errors.New("sla: số giờ vượt mức hợp lý")

	// ErrChairmanBeforeUnitHead — the chairman threshold is below the unit head's (user decision
	// 2026-09-29: Y >= X). With Y < X the commune's leadership would hear of late work BEFORE the head
	// of the unit holding it, which inverts the two levels of the escalation.
	ErrChairmanBeforeUnitHead = errors.New("sla: số giờ báo Chủ tịch không được nhỏ hơn số giờ báo lãnh đạo trực tiếp")
)

// GioToiDa is the typed-input ceiling. See ErrGioQuaLon for why it is here and not in SQL.
const GioToiDa = 8760

// KiemTraGio refuses a single hour figure.
//
// IT TAKES NO FIELD NAME AND RETURNS A SENTINEL, so the caller decides the sentence. Naming the
// field in the error would put five near-identical sentences in this file, and five sentences that
// must stay in step with five JSON names is how a message ends up pointing at the wrong box.
func KiemTraGio(gio int) error {
	switch {
	case gio <= 0:
		return ErrGioPhaiDuong
	case gio > GioToiDa:
		return ErrGioQuaLon
	}
	return nil
}

// KiemTraDongSLA refuses a whole row: all five figures and the optional sixth, together.
//
// ALL ARE CHECKED EVEN WHEN ONLY ONE WAS EDITED, and that is deliberate. The edit path applies the
// change to the row it read and validates the RESULT, so a row that was already bad — restored from
// elsewhere, written before this check existed — cannot be committed again untouched by a screen
// that only meant to change one number.
func KiemTraDongSLA(d DongSLA) error {
	for _, gio := range []int{
		d.GioTiepNhan, d.GioXuLyXong, d.GioSapDenHan, d.GioBaoLanhDao, d.GioBaoChuTich,
	} {
		if err := KiemTraGio(gio); err != nil {
			return err
		}
	}
	// ZERO IS "NOT SET" (NULL, do not report) for this one column only — see DongSLA. Any value that
	// IS set obeys the same bounds as the other five.
	if d.UnassignedHoldHours != 0 {
		if err := KiemTraGio(d.UnassignedHoldHours); err != nil {
			return err
		}
	}
	// ONE ORDERING RULE, AND ONLY ONE: the chairman threshold is never below the unit head's (user
	// decision 2026-09-29). Every seeded row satisfies it (14-cau-hinh.md §8 has Y = 2X throughout).
	if d.GioBaoChuTich < d.GioBaoLanhDao {
		return ErrChairmanBeforeUnitHead
	}
	// THERE IS NO ORDERING RULE AMONG THE OTHER FIGURES, and that absence is measured rather than lazy:
	// the specification's own `nhiem-vu` row (14-cau-hinh.md:312) has gio_sap_den_han = 72 against
	// gio_xu_ly_xong = 40, so the intuitive "the warning window fits inside the deadline" would
	// refuse configuration the customer already uses. Same conclusion migration 0008 reached for the
	// CHECK constraint, reached again here rather than assumed.
	return nil
}
