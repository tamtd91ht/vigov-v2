package domain

// The slow-project warning threshold, as a value that knows WHERE IT CAME FROM.
//
// WHY THAT SECOND HALF IS THE WHOLE POINT. Until 2026-09-22 the threshold was
// NguongCanhBaoChamMacDinh, a constant in this package, read directly by the two read routes. That
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

// NguonNguong says where a threshold came from. A string rather than a bool, because "did the
// commune set this" acquires a third answer the moment a district-level default exists, and a bool
// would have to be replaced everywhere at once.
type NguonNguong string

const (
	// NguongTuMacDinh — no row for this commune and budget year. The software's 10 points (§13
	// rule 5) is in force, and NOBODY IN THE COMMUNE HAS CHOSEN IT.
	NguongTuMacDinh NguonNguong = "mac-dinh"

	// NguongTuXa — the commune set this figure for this budget year.
	NguongTuXa NguonNguong = "xa"
)

// NguongCanhBaoCham is the threshold in force for one commune in one budget year.
//
// THE UNIT IS PhanVan — parts per ten thousand — and not percentage points, because that is the
// unit DiemCham produces and the comparison `diem_cham > nguong` has to be exact. "10 điểm" is
// 1000. Two units either side of one comparison is how a project ends up flagged on one screen and
// not on another; see PhanVan for why the comparison is integer in the first place.
type NguongCanhBaoCham struct {
	Gia   PhanVan
	Nguon NguonNguong
}

// MacDinhCuaPhanMem is the value in force when the commune has set nothing: §13 rule 5's 10 points.
//
// IT IS BUILT FROM NguongCanhBaoChamMacDinh RATHER THAN REPEATING THE NUMBER. One literal `10` in
// this package, and it is the one the constant already carries.
func MacDinhCuaPhanMem() NguongCanhBaoCham {
	return NguongCanhBaoCham{Gia: NguongCanhBaoChamMacDinh, Nguon: NguongTuMacDinh}
}

// CuaXa is a threshold the commune itself chose.
func CuaXa(gia PhanVan) NguongCanhBaoCham {
	return NguongCanhBaoCham{Gia: gia, Nguon: NguongTuXa}
}

// TuXa reports whether the commune chose this figure. A method rather than a comparison at each
// call site, so a fourth provenance cannot be added without every reader being looked at.
func (n NguongCanhBaoCham) TuXa() bool { return n.Nguon == NguongTuXa }

// ErrNguongNgoaiKhoang is a stored threshold outside the range the score can actually take.
//
// THE RANGE IS THE MATHEMATICAL RANGE, NOT A BUSINESS CHOICE: DiemCham is at most 10000 (a whole
// budget year elapsed with nothing disbursed) and 0 means "flag anything at all behind". A value of
// 100 meant as "10 points" would flag every project 1 point behind — which reads to a commune as
// the system being broken rather than as a setting being wrong, so it is refused where it is read
// rather than displayed.
var ErrNguongNgoaiKhoang = errors.New("nguong_canh_bao_cham: ngưỡng ngoài khoảng 0..10000 phần vạn")

// KiemTraNguong bounds a threshold read from storage.
//
// CHECKED ON THE WAY OUT OF THE STORE AND NOT ONLY ON THE WAY IN. The CHECK constraint in migration
// 0005 is the floor, but this service will not be the only writer forever — an import, a support
// script, a future onboarding step — and a threshold that is wrong by a factor of a hundred changes
// a reported figure without anything looking broken.
func KiemTraNguong(gia PhanVan) error {
	if gia < 0 || gia > 10000 {
		return ErrNguongNgoaiKhoang
	}
	return nil
}
