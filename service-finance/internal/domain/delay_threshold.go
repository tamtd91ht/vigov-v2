package domain

// The slow-project warning threshold, as a value that knows WHERE IT CAME FROM.
//
// WHY THAT SECOND HALF IS THE WHOLE POINT. Until 2026-09-22 the threshold was
// DefaultDelayThreshold, a constant in this package, read directly by the two read routes. That
// is rule 1, invariant 10 read backwards: a business rule belonging to a commune, living in the
// vendor's source, deciding whether that commune's KPI card says "29 dự án chậm" or "0" — with the
// commune unable to see the number, let alone change it (open question #31).
//
// Migration 0005 gives the value a home per commune and per budget year. But a table nobody writes
// yet is the failure 0004's header named exactly: it "reads as 'already configurable' while every
// commune silently gets the default". The answer is not to hide the table — it is to make the read
// path unable to lie. This type carries the threshold AND its provenance, every caller has to
// handle both, and the API sends both out, so a commune sitting on the default can be told so.
//
// STANDARD LIBRARY ONLY (rule 4). Nothing here reads a database; the store fills it in.

import "errors"

// ThresholdSource says where a threshold came from. A string rather than a bool, because "did the
// commune set this" acquires a third answer the moment a district-level default exists, and a bool
// would have to be replaced everywhere at once.
type ThresholdSource string

const (
	// ThresholdFromDefault — no row for this commune and budget year. The software's 10 points (§13
	// rule 5) is in force, and NOBODY IN THE COMMUNE HAS CHOSEN IT.
	ThresholdFromDefault ThresholdSource = "mac-dinh"

	// ThresholdFromCommune — the commune set this figure for this budget year.
	ThresholdFromCommune ThresholdSource = "xa"
)

// DelayThreshold is the threshold in force for one commune in one budget year.
//
// THE UNIT IS BasisPoints — parts per ten thousand — and not percentage points, because that is the
// unit DelayScore produces and the comparison `diem_cham > nguong` has to be exact. "10 điểm" is
// 1000. Two units either side of one comparison is how a project ends up flagged on one screen and
// not on another; see BasisPoints for why the comparison is integer in the first place.
type DelayThreshold struct {
	Value  BasisPoints
	Source ThresholdSource
}

// SoftwareDefaultThreshold is the value in force when the commune has set nothing: §13 rule 5's 10 points.
//
// IT IS BUILT FROM DefaultDelayThreshold RATHER THAN REPEATING THE NUMBER. One literal `10` in
// this package, and it is the one the constant already carries.
func SoftwareDefaultThreshold() DelayThreshold {
	return DelayThreshold{Value: DefaultDelayThreshold, Source: ThresholdFromDefault}
}

// CommuneThreshold is a threshold the commune itself chose.
func CommuneThreshold(value BasisPoints) DelayThreshold {
	return DelayThreshold{Value: value, Source: ThresholdFromCommune}
}

// IsFromCommune reports whether the commune chose this figure. A method rather than a comparison at each
// call site, so a fourth provenance cannot be added without every reader being looked at.
func (n DelayThreshold) IsFromCommune() bool { return n.Source == ThresholdFromCommune }

// ErrThresholdOutOfRange is a stored threshold outside the range the score can actually take.
//
// THE RANGE IS THE MATHEMATICAL RANGE, NOT A BUSINESS CHOICE: DelayScore is at most 10000 (a whole
// budget year elapsed with nothing disbursed) and 0 means "flag anything at all behind". A value of
// 100 meant as "10 points" would flag every project 1 point behind — which reads to a commune as
// the system being broken rather than as a setting being wrong, so it is refused where it is read
// rather than displayed.
var ErrThresholdOutOfRange = errors.New("nguong_canh_bao_cham: ngưỡng ngoài khoảng 0..10000 phần vạn")

// ValidateThreshold bounds a threshold read from storage.
//
// CHECKED ON THE WAY OUT OF THE STORE AND NOT ONLY ON THE WAY IN. The CHECK constraint in migration
// 0005 is the floor, but this service will not be the only writer forever — an import, a support
// script, a future onboarding step — and a threshold that is wrong by a factor of a hundred changes
// a reported figure without anything looking broken.
func ValidateThreshold(value BasisPoints) error {
	if value < 0 || value > 10000 {
		return ErrThresholdOutOfRange
	}
	return nil
}
