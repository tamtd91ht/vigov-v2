package domain

import "strconv"

// CaLamViec is ONE WORKING SESSION of the commune's ordinary week (@entity WorkingHours,
// migration 0006).
//
// A ROW IS A SESSION, NOT A DAY, and that is the decision the whole table turns on. A single
// (start, end) pair per weekday cannot express a lunch break, and a lunch break is 1–1.5 hours a
// day — over a 40-hour deadline that is a full working day of drift (migration 0006:83). So:
//
//	Mon–Fri, 07:30–11:30 and 13:30–17:00   ten rows
//	Saturday morning duty                   one more row, Thu = 6
//	A day the commune does not work         NO ROW for that weekday
//
// There is no "works on Saturday" flag and no half-day type, because each of those would be a
// second way to say what the rows already say (rule 9).
type CaLamViec struct {
	// ID is the ULID of the row.
	ID string

	// Thu is the ISO 8601 weekday: 1 = Monday … 7 = Sunday.
	//
	// NOT THE VIETNAMESE ORDINAL, where "thứ Hai" is Monday and Sunday is "Chủ nhật" with no
	// number. PostgreSQL's EXTRACT(ISODOW FROM d) returns exactly this range, so the stored value
	// and the computed one compare directly (migration 0006:98). A weekday conversion is where an
	// off-by-one lives for a year before anybody notices the deadline is a day out.
	Thu int

	// BatDau and KetThuc are wall-clock times of day — see GioTrongNgay for why they are not
	// time.Time. KetThuc > BatDau is enforced by the schema (lich_lam_viec_co_do_dai).
	BatDau  GioTrongNgay
	KetThuc GioTrongNgay

	// GhiChu is free text the commune writes for itself — "Buổi sáng", "Ca trực thứ Bảy". Never
	// parsed and never matched on: it is a label for a person reading the configuration screen.
	GhiChu string
}

// LoaiVanDeLich names a way a commune's weekly calendar is unusable for computing anything.
//
// THE VALUES ARE PART OF THE CONTRACT — the read route publishes them — so they are stable English
// identifiers, in the same spelling a client would branch on.
type LoaiVanDeLich string

const (
	// VanDeLichTrong: the commune has no working session at all.
	//
	// AN EMPTY CALENDAR IS NOT A DEFAULT, AND THIS CONSTANT EXISTS SO NOBODY CAN TREAT IT AS ONE.
	// A commune with no `lich_lam_viec` row has NO working hours, so a deadline counted in working
	// hours never arrives. Anything computing from this calendar must REFUSE rather than fall back
	// to "Mon–Fri 08:00–17:00" or "24 hours": a silent default here is a commitment INVENTED BY
	// SOFTWARE and then told to a citizen (rule 10; migration 0006:56; the fail-closed principle
	// CLAUDE.md opens with).
	//
	// It is reported rather than turned into an error by the read path on purpose — the
	// configuration screen has to be able to show "chưa cấu hình", and today that is EVERY commune,
	// because migration 0006 seeds nothing and the onboarding step does not exist yet. A read that
	// refused would leave the only screen that can fix the problem unable to load.
	VanDeLichTrong LoaiVanDeLich = "empty_calendar"

	// VanDeCaChongNhau: two sessions on one weekday overlap, so those hours are counted twice.
	// The database cannot refuse this — see TimChongNhau for why the EXCLUDE constraint is absent.
	VanDeCaChongNhau LoaiVanDeLich = "overlapping_sessions"
)

// VanDeLich is one problem found in a commune's weekly calendar.
//
// DERIVED ON EVERY READ, NEVER STORED. Same discipline as rule 10, invariant 3 applies to the
// overdue state: a column recording "this calendar is broken" would be written by something, and
// whatever wrote it would be stale the moment a row changed. Two sources for one fact drift, and
// the stale one is the one that reaches the screen.
type VanDeLich struct {
	Loai LoaiVanDeLich

	// Thu is the weekday the problem sits on, 0 when the problem is not about one weekday —
	// VanDeLichTrong is about the whole calendar. The read route maps 0 to `null` rather than
	// publishing a weekday nobody configured.
	Thu int

	// CaID names the sessions involved, empty for VanDeLichTrong. IDs and not times: the times are
	// on the rows the same response carries, and copying them here would be a second copy to drift.
	CaID []string
}

// VanDeCuaLich reports everything that makes this weekly calendar unusable for computing working
// hours. An empty result means the calendar can be computed from.
//
// TWO PROBLEMS IN ONE LIST, AND THEY REALLY ARE ONE KIND OF THING: both mean "you cannot count
// working hours against this". A caller that must refuse checks `len(...) > 0` and needs to know
// nothing else; a caller that merely displays walks the list.
func VanDeCuaLich(cas []CaLamViec) []VanDeLich {
	if len(cas) == 0 {
		return []VanDeLich{{Loai: VanDeLichTrong, CaID: []string{}}}
	}

	ks := make([]Khoang, 0, len(cas))
	for _, c := range cas {
		// The group is the weekday: two sessions on different days cannot overlap.
		ks = append(ks, Khoang{
			ID: c.ID, Nhom: strconv.Itoa(c.Thu), BatDau: c.BatDau, KetThuc: c.KetThuc,
		})
	}

	var ra []VanDeLich
	for _, cap := range TimChongNhau(ks) {
		// The group string was built from Thu just above, so it parses back. An error here would
		// mean this function's own two halves disagree, and 0 makes that visible on the screen
		// rather than hiding it — the route publishes `null` for it.
		thu, _ := strconv.Atoi(cap.Nhom)
		ra = append(ra, VanDeLich{
			Loai: VanDeCaChongNhau, Thu: thu, CaID: []string{cap.A, cap.B},
		})
	}
	return ra
}
