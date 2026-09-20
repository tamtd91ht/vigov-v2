package domain

// NgayNghiLe is one date this commune does NOT work, whatever the weekly calendar says
// (@entity PublicHoliday, migration 0006).
//
// PER COMMUNE, INCLUDING NATIONAL HOLIDAYS — ADR 0007 decision 4, restated by migration 0006:176
// and not re-argued here (rule 9). The cost is that Tết is stored 200+ times, once per commune;
// what makes it survivable is that the onboarding step sows the national list, so no human types
// it 200 times.
//
// WHAT THIS IS NOT: not a calendar of events, not one person's days off, not a schedule. One row
// means "on this date this authority is closed".
type NgayNghiLe struct {
	// ID is the ULID of the row.
	ID string

	// Ngay is the calendar date as `YYYY-MM-DD`, NOT a time.Time — and this is the same decision
	// as GioTrongNgay's, for the other half of the same reason.
	//
	// The column is `DATE`: a day in the commune's own calendar, with no instant and no zone. A
	// time.Time would arrive as midnight UTC, and the first place it were formatted in local time
	// it would move to the PREVIOUS DAY — a holiday that silently shifts by one day is a deadline
	// computed through a day the office was closed. As text it compares, sorts and serialises as
	// exactly the day the commune entered, and there is nothing to convert.
	Ngay string

	// Ten is what a person reads: "Quốc khánh", "Giỗ Tổ Hùng Vương", "Lễ hội đình làng". Written by
	// the commune, shown on the configuration screen, never matched on.
	Ten string
}
