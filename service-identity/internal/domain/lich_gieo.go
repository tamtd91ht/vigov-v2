package domain

import "fmt"

// The SEED sets for a commune's working calendar — the rows POST /api/v1/working-hours/defaults and
// POST /api/v1/public-holidays/defaults write for a commune that has none (migration 0006).
//
// =================================================================================================
// READ THIS BEFORE CONCLUDING THIS FILE BREAKS THE FAIL-CLOSED RULE.
//
// Migration 0006:56 refuses, at length, a DEFAULT on the READ path: a commune with no session must
// make the deadline function REFUSE, never fall back to "Mon–Fri 08:00–17:00", because a fallback
// is a commitment invented by software and then told to a citizen. This file is not that, and the
// difference is the same one domain.BoGieoSLA draws:
//
//	FORBIDDEN      hours in source READ AT THE MOMENT A DEADLINE IS COMPUTED, standing in for an
//	               empty table. The commune never owns those hours, cannot see them on any screen
//	               and cannot change them.
//	NOT FORBIDDEN  hours written ONCE into the commune's OWN ROWS by a deliberate administrative
//	               act, which then appear on the configuration screen and which the commune edits
//	               from the first day. After the seed runs the source is no longer the source: the
//	               row is.
//
// THE CONSEQUENCE THAT KEEPS THE TWO APART, AND IT MUST NOT BE SOFTENED: nothing on the
// deadline-computing path may ever read this file. An empty `lich_lam_viec` still makes
// grpc.AdvanceWorkingHours answer FAILED_PRECONDITION and still makes `service-documents` answer
// 409. This is a way for the tables to STOP BEING EMPTY; it is not a way to carry on while they are.
//
// WHY IT IS NOT A MIGRATION: migration 0006:48 already answered that. A seed row carries
// `tenant_id`, the migration path has no commune in it and deliberately accepts none (ADR 0013), and
// reading `platform`'s tenant registry from another service's migration is rule 2, forbidden #2.
// Seeding is therefore a PER-COMMUNE administrative act, which is what makes it a route.

// GieoCaLamViec is one row of the weekly seed set. A CaLamViec without an ID, because the ID is
// minted per commune at the moment the row is written and a fixed one here would be the same ULID
// in every commune's table.
type GieoCaLamViec struct {
	Thu     int
	BatDau  GioTrongNgay
	KetThuc GioTrongNgay
	GhiChu  string
}

// GioMoSang, GioDongSang, GioMoChieu and GioDongChieu are THE INITIAL VALUE, NOT A CONSTANT OF THE
// SOFTWARE — and the difference decides whether this file is legal at all.
//
// =================================================================================================
// ⚠ THESE FOUR NUMBERS ARE NOT IN ANY SPECIFICATION. SAID PLAINLY BECAUSE THE SIBLING SEED SET CAN
// SAY THE OPPOSITE.
//
// domain.BoGieoSLA can point at docs/ui-ux/14-cau-hinh.md §8 lines 297-312 and say "verbatim". This
// file cannot. 14-cau-hinh.md names the calendar tables at :318 and specifies NO SCREEN and NO
// HOURS; ADR 0007 §"Lỗ hổng đặc tả" lists *"Giờ hành chính của xã là mấy giờ tới mấy giờ?"* as a
// question NOBODY HAS ANSWERED, and answers it only in the sense that it classifies it: *"Bốn câu
// còn lại đều là DỮ LIỆU, không phải quy tắc: mỗi câu trả lời là một số dòng trong lich_lam_viec
// hoặc ngay_nghi_le, nên không có mã nào phải tự quyết chúng."*
//
// This file therefore writes DATA that a commune owns and edits, which is the shape ADR 0007 asked
// for. It does NOT decide a rule. What it cannot avoid is that SOME figure has to be written, and
// these are the figures:
//
//	07:30–11:30 morning, 13:30–17:00 afternoon, Monday to Friday
//
// They are migration 0006's OWN WORKED EXAMPLE, at :90 and :161 — "Mon–Fri, 07:30–11:30 and
// 13:30–17:00 ten rows" — chosen here rather than invented, so the number in the seed and the
// number the schema's author wrote down are one number. They are also the ordinary office hours of
// a commune People's Committee.
//
// WHAT A COMMUNE THAT WORKS DIFFERENT HOURS DOES: edits them, on the first day, on the same screen.
// Nothing downstream reads these constants; every deadline is computed from the ROWS.
//
// SATURDAY AND SUNDAY GET NO ROW AT ALL, and that is the schema's way of saying "not a working day"
// (migration 0006:88) — not a flag, not a zero-length session. A commune with a Saturday duty shift
// adds one row for `thu = 6`, which is exactly what the write route above this file is for.
// =================================================================================================
const (
	GioMoSang     GioTrongNgay = 7*3600 + 30*60
	GioDongSang   GioTrongNgay = 11*3600 + 30*60
	GioMoChieu    GioTrongNgay = 13*3600 + 30*60
	GioDongChieu  GioTrongNgay = 17 * 3600
	GhiChuCaSang               = "Buổi sáng"
	GhiChuCaChieu              = "Buổi chiều"
)

// BoGieoCaLamViec returns the weekly seed set: ten sessions, Monday to Friday, morning and
// afternoon.
//
// A FUNCTION RETURNING A FRESH SLICE, NOT AN EXPORTED VAR: a package-level slice is mutable by any
// caller in the process, and one commune's seeding run silently editing the set the next commune
// gets is the kind of defect that only shows up under load.
//
// THE LUNCH BREAK IS THE GAP BETWEEN THE TWO ROWS, not a third field and not a flag. That is the
// whole reason a row is a SESSION and not a day (migration 0006:83): 11:30–13:30 is simply time
// with no session covering it, so the deadline function skips it without being told to. Over a
// 40-hour deadline a mishandled lunch break is a full working day of drift.
func BoGieoCaLamViec() []GieoCaLamViec {
	ra := make([]GieoCaLamViec, 0, 10)
	for thu := 1; thu <= 5; thu++ { // ISO: 1 = thứ Hai … 5 = thứ Sáu
		ra = append(ra,
			GieoCaLamViec{Thu: thu, BatDau: GioMoSang, KetThuc: GioDongSang, GhiChu: GhiChuCaSang},
			GieoCaLamViec{Thu: thu, BatDau: GioMoChieu, KetThuc: GioDongChieu, GhiChu: GhiChuCaChieu},
		)
	}
	return ra
}

// GieoNgayNghiLe is one row of the holiday seed set for one year.
type GieoNgayNghiLe struct {
	Ngay string // YYYY-MM-DD
	Ten  string
}

// BoGieoNgayNghiLe returns the FIXED-DATE public holidays of one year, per Bộ luật Lao động 2019
// điều 112.
//
// =================================================================================================
// ⚠⚠ THE LUNAR HOLIDAYS ARE DELIBERATELY ABSENT, AND THIS IS THE MOST CONSEQUENTIAL LINE IN THE
// FILE. NOBODY MAY "COMPLETE" THIS LIST.
//
// Điều 112 lists eleven days. Four of them sit on FIXED SOLAR DATES and are below. The rest do not:
//
//	Tết Nguyên đán, 5 days   LUNAR. Falls between 21 January and 20 February depending on the year.
//	Giỗ Tổ Hùng Vương        LUNAR — mùng 10 tháng 3 âm lịch.
//	the day beside 02/9      SOLAR but NOT FIXED: điều 112 khoản 1 điểm đ gives "02/9 và 01 ngày
//	                         liền kề trước hoặc sau", and khoản 3 hands the choice between 01/9 and
//	                         03/9 to the Prime Minister, EACH YEAR.
//
// A LUNAR-TO-SOLAR CONVERSION WRITTEN HERE WOULD BE WRONG, AND WRONG BY A DAY IS THE WORST KIND.
// The Vietnamese lunar calendar is computed against UTC+7 with astronomical new moons and leap
// months; every hand-rolled implementation of it in existence differs from the official calendar in
// some year. A holiday that is one day out does not fail: it produces a deadline that counts
// through a day the office was shut, in the direction that reports the authority late when it was
// not — or lets a file be marked on time when it was not. That figure then goes into a report sent
// upward, and the citizen who was told "within 2 hours" is the one who finds out.
//
// GUESSING THE DAY BESIDE 02/9 IS THE SAME MISTAKE IN A SHORTER FORM. It is 01/9 in some years and
// 03/9 in others; seeding either is a coin toss written into a commune's configuration.
//
// SO THE COMMUNE ENTERS THOSE DAYS, from the Prime Minister's annual announcement, through
// POST /api/v1/public-holidays. That is not a gap in this seed — it is the only honest answer, and
// it is the same answer ADR 0007 gives for every other calendar question: the data belongs to the
// commune. A seed that is four rows and correct beats a seed that is eleven rows and wrong in one.
//
// WHAT THE SCREEN OWES THE PERSON PRESSING THE BUTTON, and it is a finding for whoever builds it:
// the response says `seeded: 4`, and the commune must be told IN WORDS that Tết, Giỗ Tổ and the day
// beside Quốc khánh are still missing. A count alone reads as "done".
// =================================================================================================
//
// THE NAMES ARE THE LAW'S OWN WORDING, so a person reading the configuration screen sees what the
// announcement they are holding says. `nam` is not validated here — the caller bounds it (see
// internal/http, NamNhoNhat/NamLonNhat), because a year is a window the client chooses and a domain
// rule about it would be a second, quieter bound.
func BoGieoNgayNghiLe(nam int) []GieoNgayNghiLe {
	return []GieoNgayNghiLe{
		{Ngay: fmt.Sprintf("%04d-01-01", nam), Ten: "Tết Dương lịch"},
		{Ngay: fmt.Sprintf("%04d-04-30", nam), Ten: "Ngày Chiến thắng"},
		{Ngay: fmt.Sprintf("%04d-05-01", nam), Ten: "Ngày Quốc tế lao động"},
		{Ngay: fmt.Sprintf("%04d-09-02", nam), Ten: "Quốc khánh"},
		//
		// Tết Nguyên đán (5 ngày), Giỗ Tổ Hùng Vương (10/3 âm lịch) và NGÀY LIỀN KỀ 02/9 CỐ Ý KHÔNG
		// CÓ Ở ĐÂY. Xem khối chú thích phía trên: hai cái đầu theo ÂM LỊCH và một phép quy đổi tự
		// viết sai một ngày là sai một cam kết với dân; cái thứ ba do Thủ tướng chọn từng năm giữa
		// 01/9 và 03/9. Xã tự nhập theo thông báo hằng năm.
	}
}

// KhongCoNgayLamBuMacDinh stands where a swap-day seed set would have been.
//
// THERE IS NO SEED FOR `ngay_lam_bu`, AND THAT IS AN ANSWER RATHER THAN A GAP. A swap day exists
// only because the Prime Minister announced one for a particular year — "làm bù nghỉ Tết theo Thông
// báo số …" — so there is no fixed set to seed at all. Most years, most communes have none
// (domain.VanDeLamBu says the same from the read side: an empty year is perfectly ordinary here,
// unlike an empty weekly calendar).
//
// Seeding a guess would be worse than seeding nothing: a swap day the commune did not work makes
// every deadline crossing it come out EARLIER than the commune's real hours.
const KhongCoNgayLamBuMacDinh = "ngày làm bù theo thông báo hằng năm của Thủ tướng — không có bộ gieo"
