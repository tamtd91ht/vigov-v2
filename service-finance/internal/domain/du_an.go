package domain

import "time"

// Dong is an amount of money in ĐỒNG — the smallest unit the currency has.
//
// A NAMED int64 AND NEVER A float64, AND THE REASON IS THE SAME ONE THE MIGRATION GIVES: binary
// floating point cannot hold a decimal amount exactly, and a sum over it depends on the order the
// values arrive in. A disbursement figure is quoted in a decision with legal effect and totalled
// into a report that goes upward; a difference of one đồng between two printouts of the same
// figure is something a person has to answer for, and it appears without any line of code looking
// wrong.
//
// It is a distinct type rather than a bare int64 so that a count of projects and an amount of
// money cannot be added together by accident — the compiler refuses, and that is the cheapest
// place this class of mistake can ever be caught.
type Dong int64

// PhanVan is a proportion in parts per TEN THOUSAND. 1033 reads as 10,33%.
//
// WHY NOT A float64 PERCENTAGE, which is what every screen shows: the numbers compared here decide
// whether a project is flagged as behind (§3, "điểm chậm"), and a comparison of two floats near a
// threshold is a comparison whose answer depends on rounding nobody can see. In parts per ten
// thousand every value is an exact integer, the comparison is exact, and the screen divides by 100
// once, at the very edge.
//
// TEN THOUSAND AND NOT A HUNDRED: the specification prints two decimal places of a percentage
// ("10,33%", "chậm 31,36 điểm"), so a hundredth of a percent is the smallest thing anyone is shown.
// Holding exactly that unit means the stored value and the printed value are the same number.
type PhanVan int64

// MotPhanTram is one percent expressed in PhanVan. Use it to turn a threshold a person states in
// points ("10 điểm") into the unit this package compares in.
const MotPhanTram PhanVan = 100

// NguongCanhBaoChamMacDinh is the default slow-project threshold: 10 points (§3, §13 rule 5).
//
// A CONSTANT, AND THAT IS A KNOWN TEMPORARY SHAPE. §11 puts `nguong_canh_bao_cham` on
// `nam_ngan_sach`, per commune and per budget year, which is where it belongs — a threshold
// hardcoded in source is a commune's business rule living in the vendor's repository (rule 1,
// forbidden). No table and no screen writes it yet, so every caller passes this value explicitly
// rather than the functions below reaching for it; when the table arrives, the call sites change
// and nothing here has to be believed.
const NguongCanhBaoChamMacDinh = 10 * MotPhanTram

// DuAn is one investment project of one commune in one budget year (table `du_an`).
//
// THERE IS NO DaGiaiNgan FIELD ON THIS STRUCT, and that is the whole point of the design. The
// disbursed total is derived from the project's live vouchers on every read — see TienDoDuAn. A
// field here would be a second home for a number that already has one, and the stale copy is
// always the one that reaches the report (the reasoning rule 10 gives for refusing an `is_overdue`
// column, applied to money).
//
// COLUMNS DELIBERATELY ABSENT:
//
//	tenant_id    never a field. It rides in context.Context and is bound by the scoped repository
//	             (rule 1, invariant 4). A field would be a value somebody can pass, and pass wrong.
//	deleted_at   a soft-deleted row never leaves the store (rule 7, invariant 2), so no reader
//	             needs to ask.
type DuAn struct {
	ID  string // ULID
	Ma  string // "DA-2026-be-tong-hoa-duong-ngo-xo-2" — issued once, never reissued
	Nam int    // budget year. A project of 2025 and its successor in 2026 are two rows (§13 rule 8)

	HangMucID string // the capital plan category this project is classified under
	Ten       string
	MoTa      string

	// KeHoachVonNam is what the commune allocated to this project FOR THIS YEAR. It is the
	// denominator of every ratio on every disbursement screen.
	KeHoachVonNam Dong

	// TongMucDuocDuyet is the approved total for the WHOLE project, across years. Zero means the
	// commune left it blank, in which case §9 says to read it as equal to this year's plan —
	// TongMucHieuLuc does that, so no caller has to remember the rule.
	TongMucDuocDuyet Dong

	DonViThucHienID string
	CanBoPhuTrachID string

	NgayKhoiCong  time.Time // zero when not set
	NgayHoanThanh time.Time // zero when not set

	// ThoiHanGiaiNgan is the date this year's money must be disbursed by — NOT the date the works
	// finish. §9 spells out the difference: a project completed in March may still have to be
	// disbursed before 31/12.
	ThoiHanGiaiNgan time.Time
}

// TongMucHieuLuc is the approved total to use, applying §9's rule for a blank field in ONE place.
//
// The rule is applied here rather than copied into the store, the handler and the screen: three
// copies of a default is three places for it to drift, and the one that drifts is invisible
// because a plausible number still appears.
func (d DuAn) TongMucHieuLuc() Dong {
	if d.TongMucDuocDuyet <= 0 {
		return d.KeHoachVonNam
	}
	return d.TongMucDuocDuyet
}

// TienDoDuAn is one project together with the figures DERIVED from its vouchers.
//
// It is a separate type from DuAn on purpose: DuAn is what the table holds, TienDoDuAn is what a
// screen is shown. Merging them would put a computed total on the entity, which is the first step
// toward somebody storing it.
type TienDoDuAn struct {
	DuAn DuAn

	// DaGiaiNgan is SUM(so_tien) over the project's LIVE vouchers, in every state — `ke-toan-nhap`
	// counts (§11). Computed by the store on each read; nothing persists it.
	DaGiaiNgan Dong
}

// ConPhaiGiaiNgan is what is left of this year's plan.
//
// IT CAN BE NEGATIVE, and the caller is not protected from that: §13 rule 2 says the disbursement
// ratio may exceed 100% ("176,5%") and must not be blocked, only displayed. Clamping this to zero
// would hide an over-disbursement — which is exactly the figure somebody needs to see.
func (t TienDoDuAn) ConPhaiGiaiNgan() Dong {
	return t.DuAn.KeHoachVonNam - t.DaGiaiNgan
}

// TyLeGiaiNgan is disbursed / planned, in parts per ten thousand.
//
// ok IS false WHEN THE PLAN IS ZERO, and it is a second return value rather than a zero result on
// purpose. A project with no allocation has NO ratio — 0% and "not applicable" are different
// statements, and a screen that prints "0%" for a project nobody has allocated money to reports it
// as the worst performer in the commune. The caller has to decide what to show; it cannot receive
// a plausible number by accident.
//
// ROUNDING IS TOWARD ZERO, which is Go's integer division. The unit is a hundredth of a percent,
// so the discarded part is smaller than anything any screen prints.
func (t TienDoDuAn) TyLeGiaiNgan() (PhanVan, bool) {
	if t.DuAn.KeHoachVonNam <= 0 {
		return 0, false
	}
	return PhanVan(int64(t.DaGiaiNgan) * 10000 / int64(t.DuAn.KeHoachVonNam)), true
}

// PhanTramThoiGianDaQua is how much of the budget year has elapsed at `nay`, in parts per ten
// thousand (§3: `(hôm nay − 01/01/năm) / (31/12/năm − 01/01/năm) × 100`).
//
// THE DENOMINATOR IS THE WHOLE YEAR, 01/01 TO 01/01 OF THE NEXT YEAR, and not "31/12 minus 01/01"
// read literally. Taken literally that denominator is one day short, which would make the last day
// of the year read as more than 100% elapsed and every project on it look worse than it is. A leap
// year is handled by the same subtraction with no special case.
//
// WALL-CLOCK TIME IS CORRECT HERE, AND THIS IS THE ONE PLACE IN THE SYSTEM WHERE IT IS. Rule 10
// forbids counting a citizen's processing deadline in wall-clock time because that deadline is a
// promise measured in WORKING days. This is a different quantity: "how much of the calendar year
// has gone" is a statement about the calendar, and a commune's budget year does not pause at
// weekends. Do not "fix" this by routing it through the working-hours calendar.
//
// THE LOCATION IS THE CALLER'S. `nay` and the year boundaries are compared in `nay`'s own
// location, so a caller passing a UTC clock while the commune lives in UTC+7 shifts the boundary
// by seven hours — at the very start and very end of a year that changes the figure. The edge is
// expected to pass a clock already in the commune's zone.
func PhanTramThoiGianDaQua(nay time.Time, nam int) PhanVan {
	dau := time.Date(nam, time.January, 1, 0, 0, 0, 0, nay.Location())
	cuoi := time.Date(nam+1, time.January, 1, 0, 0, 0, 0, nay.Location())

	if !nay.After(dau) {
		return 0
	}
	if !nay.Before(cuoi) {
		return 10000
	}
	// SECONDS, NOT time.Duration, AND THE FIRST VERSION OF THIS LINE WAS WRONG BECAUSE OF IT. A
	// Duration counts NANOSECONDS in an int64: a year is 3,15 × 10^16 ns, so multiplying it by
	// 10000 before dividing overflows int64 and wraps to a small positive number. The failure is
	// the dangerous kind — no panic, no error, just a plausible single-digit percentage that made
	// every project in the commune look on schedule. Dividing to seconds first leaves the product
	// at 3,15 × 10^11, eight orders of magnitude inside the type.
	daQua := int64(nay.Sub(dau) / time.Second)
	caNam := int64(cuoi.Sub(dau) / time.Second)
	return PhanVan(daQua * 10000 / caNam)
}

// DiemCham is the slow-project score of §3: elapsed share of the year MINUS disbursed share of the
// plan, both in parts per ten thousand. A project that has disbursed nothing in a year that is
// 70,96% gone scores exactly 7096 — "chậm 70,96 điểm", which is the worked example in the
// specification.
//
// ok IS false WHEN THE PROJECT HAS NO PLAN, carried straight through from TyLeGiaiNgan: without a
// denominator there is no disbursed share, so there is no score. Returning the elapsed share alone
// would flag every unallocated project as maximally behind halfway through the year.
//
// IT CAN BE NEGATIVE — a project ahead of the calendar. Kept, not clamped: "ahead by 20 points" is
// a real and useful statement, and LaCham only ever compares it against a positive threshold.
func DiemCham(t TienDoDuAn, nay time.Time) (PhanVan, bool) {
	tyLe, ok := t.TyLeGiaiNgan()
	if !ok {
		return 0, false
	}
	return PhanTramThoiGianDaQua(nay, t.DuAn.Nam) - tyLe, true
}

// LaCham reports whether a project counts as behind: `diem_cham > nguong` (§3).
//
// STRICTLY GREATER, matching the specification's own wording ("dự án CHẬM khi diem_cham >
// nguong_canh_bao_cham"). A project exactly ON the threshold is not flagged, and the difference
// shows up on real data — a commune that sets the threshold to 0 would otherwise see every project
// that has disbursed exactly its share of the year reported as behind.
//
// A PROJECT WITH NO PLAN IS NOT BEHIND, because it has no score. That is a deliberate choice with
// a visible consequence: such projects fall out of the "N dự án chậm" count entirely. The
// alternative — counting them as behind — would flag a project the commune has not yet allocated
// money to, which is an accounting state, not a delay. If the customer wants them counted, that is
// their call, not this function's.
func LaCham(t TienDoDuAn, nay time.Time, nguong PhanVan) bool {
	diem, ok := DiemCham(t, nay)
	if !ok {
		return false
	}
	return diem > nguong
}

// LuyKe is the running total of a sequence of amounts: element i is the sum of amounts 0..i.
//
// DERIVED, NEVER STORED — the reason is written out on the migration and it is the same one rule
// 10 gives for refusing an `is_overdue` column. The cumulative curve of §4 ("Luỹ kế giải ngân so
// với kế hoạch") is the figure leadership reads; a stored total that a nightly job maintains is
// wrong the moment a voucher is corrected, and nothing on the chart says so.
//
// THE INPUT IS A SEQUENCE ALREADY IN ORDER and this function does not sort it: the order is a
// decision about WHICH clock (payment date, entry date) and belongs to the caller that read the
// rows, not to a helper that would quietly pick one.
func LuyKe(so []Dong) []Dong {
	ra := make([]Dong, len(so))
	var tong Dong
	for i, mot := range so {
		tong += mot
		ra[i] = tong
	}
	return ra
}

// KeHoachTuyenTinh is the straight planning line of §4: the plan spread evenly across `moc` points
// of the year, so element i is the share of the plan that should have been disbursed by point i.
//
// The last element is EXACTLY the plan, not the plan minus a rounding remainder: each element is
// computed from the plan directly rather than by adding a per-point increment, so the error of one
// division never accumulates. A chart whose planning line stops a few đồng short of the plan
// invites the question of which number is wrong, every single time somebody looks at it.
func KeHoachTuyenTinh(keHoach Dong, moc int) []Dong {
	if moc <= 0 {
		return nil
	}
	ra := make([]Dong, moc)
	for i := range ra {
		ra[i] = Dong(int64(keHoach) * int64(i+1) / int64(moc))
	}
	return ra
}
