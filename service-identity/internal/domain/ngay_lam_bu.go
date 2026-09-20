package domain

// CaLamBu is ONE WORKING SESSION on a date the commune works although the week says otherwise
// (@entity SwapWorkingDay, migration 0006).
//
// WHY THE TABLE EXISTS AT ALL, and it is the Vietnamese working year rather than a design
// preference: every year the Prime Minister announces the Tết and National Day arrangements, and
// they routinely include làm bù — a Saturday that becomes a working day to repay a long holiday
// run. Without these rows every deadline crossing those dates is computed as if the commune were
// closed, and the error lands on the days of the year when the backlog is largest
// (migration 0006:238).
//
// IT CANNOT BE FOLDED INTO NgayNghiLe WITH A `loai` FIELD: a holiday is a date, a swap day is a
// date PLUS the hours worked — the weekday it falls on usually has no session at all. One type
// would then carry fields that are meaningless half the time.
type CaLamBu struct {
	// ID is the ULID of the row.
	ID string

	// Ngay is the calendar date as `YYYY-MM-DD` — see NgayNghiLe.Ngay for why it is not a
	// time.Time.
	Ngay string

	// BatDau and KetThuc are the hours worked ON THAT DATE. They are on the row, not taken from
	// the weekly calendar, precisely because the weekday usually has no session rows.
	BatDau  GioTrongNgay
	KetThuc GioTrongNgay

	// Ten is the announcement this row implements — "Làm bù nghỉ Tết theo Thông báo số …". Prose
	// for a person; an inspection asking why a deadline ran through a Saturday reads this.
	Ten string
}

// VanDeLamBu is one problem found in a year's swap days: two sessions on one date that overlap.
//
// ONE KIND ONLY, unlike VanDeLich, AND THE MISSING KIND IS THE POINT: a year with no swap day at
// all is perfectly ordinary — most years for most communes — so there is no "empty" problem to
// report here. An empty weekly calendar is the opposite: it means the commune has no working hours
// at all.
type VanDeLamBu struct {
	// Ngay is the date the two sessions sit on, `YYYY-MM-DD`.
	Ngay string

	// CaID names the two sessions. IDs and not times — the times are on the rows the same response
	// carries.
	CaID []string
}

// CaLamBuChongNhau reports every pair of swap-day sessions that overlap on one date.
//
// THE SAME DOUBLE-COUNT AS THE WEEKLY CALENDAR, AND THE SAME ABSENT CONSTRAINT: `ngay_lam_bu` has
// UNIQUE (tenant_id, ngay, bat_dau), which stops two sessions STARTING at the same minute and
// nothing more (migration 0006:283). Two sessions of 07:30–11:30 and 09:00–12:00 on one swap day
// count those hours twice, exactly as they would on a Monday.
func CaLamBuChongNhau(cas []CaLamBu) []VanDeLamBu {
	ks := make([]Khoang, 0, len(cas))
	for _, c := range cas {
		// The group is the DATE here, where the weekly calendar groups by weekday. Two sessions on
		// two different swap days cannot overlap.
		ks = append(ks, Khoang{ID: c.ID, Nhom: c.Ngay, BatDau: c.BatDau, KetThuc: c.KetThuc})
	}

	var ra []VanDeLamBu
	for _, cap := range TimChongNhau(ks) {
		ra = append(ra, VanDeLamBu{Ngay: cap.Nhom, CaID: []string{cap.A, cap.B}})
	}
	return ra
}
