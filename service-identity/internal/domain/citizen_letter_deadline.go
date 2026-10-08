package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// A commune's deadline rules for CITIZEN LETTERS, per letter type (migration 0027; ADR 0084 #3,
// ADR 0085 B, ADR 0064), and the day arithmetic that turns one into an instant.
//
// TWO HALVES IN ONE FILE BECAUSE THEY ARE ONE RULE. The unit lock decides WHICH count a row may ask
// for; the arithmetic performs that count. A row the lock refuses never reaches the arithmetic, so
// the arithmetic never has to decide what a working-hours letter deadline means.
//
// THE ARITHMETIC LIVES HERE AND NOWHERE ELSE (ADR 0064 #4; rule 10, forbidden #2). It walks the SAME
// calendar the working-hours count walks — newWorkingWalk validates the week, caCuaNgay reconciles
// holidays and swap days for one date — so a date this file calls "working" is exactly a date on which
// TienGioLamViec would credit hours. A second reading of the calendar here would be the second
// implementation ADR 0007 forbids.

// CitizenLetterType is `citizen_letter_deadline_rule.letter_type` — the four letter types of the
// citizen-letter register (C4, 24/09/2026), the SAME closed set as service-documents' 0006
// `citizen_letter_type_valid`.
//
// THE CONSTANT NAMES MIRROR THE STORED VALUES, as the contract's enum does: ubiquitous-language.md
// gives these four no English name, and inventing one here would be a second vocabulary for a
// statutory category (rule 12, forbidden #2).
type CitizenLetterType string

const (
	CitizenLetterKienNghiPhanAnh CitizenLetterType = "kien-nghi-phan-anh" // vi-name-ok: mirrors the stored letter_type value (ADR 0011), no English name in ubiquitous-language.md
	CitizenLetterKhieuNai        CitizenLetterType = "khieu-nai"          // vi-name-ok: mirrors the stored letter_type value (ADR 0011)
	CitizenLetterToCao           CitizenLetterType = "to-cao"             // vi-name-ok: mirrors the stored letter_type value (ADR 0011)
	CitizenLetterDeNghi          CitizenLetterType = "de-nghi"            // vi-name-ok: mirrors the stored letter_type value (ADR 0011)
)

// Valid reports whether the value is one of the four.
func (t CitizenLetterType) Valid() bool {
	switch t {
	case CitizenLetterKienNghiPhanAnh, CitizenLetterKhieuNai, CitizenLetterToCao, CitizenLetterDeNghi:
		return true
	}
	return false
}

// CitizenLetterDeadlineKind is WHICH of a letter's two deadlines a rule fixes — the contract's
// CitizenLetterDeadlineKind PROCESSING / RESOLUTION, stored in Vietnamese (ADR 0011).
type CitizenLetterDeadlineKind string

const (
	// CitizenLetterProcessing — "hạn xử lý đơn", fixed when the letter is booked.
	CitizenLetterProcessing CitizenLetterDeadlineKind = "xu-ly-don"
	// CitizenLetterResolution — "hạn giải quyết", fixed at `thu-ly`. Only khieu-nai and to-cao have one.
	CitizenLetterResolution CitizenLetterDeadlineKind = "giai-quyet"
)

// Valid reports whether the value is one of the two.
func (k CitizenLetterDeadlineKind) Valid() bool {
	return k == CitizenLetterProcessing || k == CitizenLetterResolution
}

// CitizenLetterDeadlineUnit is what `amount` counts.
type CitizenLetterDeadlineUnit string

const (
	// UnitWorkingHours is admitted by the schema (ADR 0084 §3 once named it) and REFUSED by the lock
	// below: the owner chose working DAYS on 08/10/2026 (ADR 0085 câu 3).
	UnitWorkingHours CitizenLetterDeadlineUnit = "gio-lam-viec"
	// UnitWorkingDays counts dates on which the commune has at least one working session.
	UnitWorkingDays CitizenLetterDeadlineUnit = "ngay-lam-viec"
	// UnitCalendarDays counts every date, weekends and holidays included (ADR 0064 #1).
	UnitCalendarDays CitizenLetterDeadlineUnit = "ngay-lich"
)

// Valid reports whether the value is one of the three the schema admits.
func (u CitizenLetterDeadlineUnit) Valid() bool {
	switch u {
	case UnitWorkingHours, UnitWorkingDays, UnitCalendarDays:
		return true
	}
	return false
}

// CitizenLetterDeadlineRule is one live row: (letter type, which deadline, amount, unit).
type CitizenLetterDeadlineRule struct {
	ID         string
	LetterType CitizenLetterType
	Kind       CitizenLetterDeadlineKind
	Amount     int
	Unit       CitizenLetterDeadlineUnit
}

// CitizenLetterDeadlineMaxAmount bounds `amount`. 365 because the walk refuses any answer more than
// ChanTroiNgay (366) days after its start — holidays are only ever entered about a year ahead — so a
// larger figure could never produce a deadline, only a refusal at the first letter booked. The
// statutory figures in ADR 0064 are 7 to 45. NOT a number anybody chose for a commune: a ceiling on
// what the arithmetic can answer.
const CitizenLetterDeadlineMaxAmount = 365

// CitizenLetterDeleteReasonMaxLen is the CHECK on `delete_reason` (migration 0027).
const CitizenLetterDeleteReasonMaxLen = 500

// The refusals. Each sentence is shown to the administrator as is, so it names the rule.
var (
	ErrCitizenLetterTypeUnknown          = errors.New("hạn đơn thư: loại đơn không hợp lệ")
	ErrCitizenLetterDeadlineKindUnknown  = errors.New("hạn đơn thư: loại hạn không hợp lệ (xu-ly-don hoặc giai-quyet)")
	ErrCitizenLetterNoResolutionDeadline = errors.New("hạn đơn thư: chỉ đơn khiếu nại và đơn tố cáo có hạn giải quyết")
	ErrCitizenLetterDeadlineUnitUnknown  = errors.New("hạn đơn thư: đơn vị không hợp lệ")
	ErrCitizenLetterDeadlineUnitLocked   = errors.New("hạn đơn thư: đơn vị này không được dùng cho loại đơn và loại hạn đã chọn")
	ErrCitizenLetterAmountNotPositive    = errors.New("hạn đơn thư: số ngày phải lớn hơn 0")
	ErrCitizenLetterAmountTooLarge       = errors.New("hạn đơn thư: số ngày vượt trần")
	ErrCitizenLetterDeleteReasonMissing  = errors.New("hạn đơn thư: phải nêu lý do xoá")
	ErrCitizenLetterDeleteReasonTooLong  = errors.New("hạn đơn thư: lý do xoá quá dài")
)

// RequiredCitizenLetterUnit is THE UNIT LOCK (owner, 08/10/2026; ADR 0085 câu 3, ADR 0064's table):
//
//	letter_type          xu-ly-don         giai-quyet
//	kien-nghi-phan-anh   ngay-lam-viec     —
//	de-nghi              ngay-lam-viec     —
//	khieu-nai            ngay-lich         ngay-lich
//	to-cao               ngay-lam-viec     ngay-lich
//
// EVERY CELL HAS EXACTLY ONE UNIT. A commune switching a complaint to working days silently stretches
// a statutory deadline by about half (ADR 0064 §Hệ quả, "khoá đơn vị") — the on-time figure sent
// upward then reads "on time" for an authority already past the legal limit.
//
// IN CODE AND NOT A CHECK because the legal table behind it is UNVERIFIED (ADR 0064, "cần pháp chế đối
// chiếu"): a cell moved by the legal review is one release here, and a CHECK would fail every commune's
// existing row for that cell at the next migration (migration 0027 header).
func RequiredCitizenLetterUnit(t CitizenLetterType, k CitizenLetterDeadlineKind) (CitizenLetterDeadlineUnit, error) {
	if !t.Valid() {
		return "", ErrCitizenLetterTypeUnknown
	}
	if !k.Valid() {
		return "", ErrCitizenLetterDeadlineKindUnknown
	}
	switch t {
	case CitizenLetterKienNghiPhanAnh, CitizenLetterDeNghi:
		if k == CitizenLetterResolution {
			return "", ErrCitizenLetterNoResolutionDeadline
		}
		return UnitWorkingDays, nil
	case CitizenLetterKhieuNai:
		return UnitCalendarDays, nil
	case CitizenLetterToCao:
		if k == CitizenLetterProcessing {
			return UnitWorkingDays, nil
		}
		return UnitCalendarDays, nil
	}
	return "", ErrCitizenLetterTypeUnknown
}

// CheckCitizenLetterDeadlineRule validates a whole rule: type, kind, their pairing, the unit against
// the lock, and the amount.
//
// THE SAME FUNCTION GUARDS THE WRITE AND THE READ. The configuration routes refuse a row it refuses;
// ResolveCitizenLetterDeadline refuses to USE a stored row it refuses (FAILED_PRECONDITION) — a row
// restored from elsewhere, or written before a legal review moved a cell, is never silently counted.
func CheckCitizenLetterDeadlineRule(r CitizenLetterDeadlineRule) error {
	want, err := RequiredCitizenLetterUnit(r.LetterType, r.Kind)
	if err != nil {
		return err
	}
	if !r.Unit.Valid() {
		return ErrCitizenLetterDeadlineUnitUnknown
	}
	if r.Unit != want {
		return fmt.Errorf("%w — %s / %s phải đếm bằng %s", ErrCitizenLetterDeadlineUnitLocked,
			r.LetterType, r.Kind, want)
	}
	switch {
	case r.Amount <= 0:
		return ErrCitizenLetterAmountNotPositive
	case r.Amount > CitizenLetterDeadlineMaxAmount:
		return fmt.Errorf("%w (tối đa %d)", ErrCitizenLetterAmountTooLarge, CitizenLetterDeadlineMaxAmount)
	}
	return nil
}

// NormalizeCitizenLetterDeleteReason trims and bounds the reason stored beside a soft delete.
// MANDATORY (rule 7, invariant 1; the 0027 CHECK refuses a blank one).
func NormalizeCitizenLetterDeleteReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	switch {
	case reason == "":
		return "", ErrCitizenLetterDeleteReasonMissing
	case len([]rune(reason)) > CitizenLetterDeleteReasonMaxLen:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrCitizenLetterDeleteReasonTooLong,
			CitizenLetterDeleteReasonMaxLen)
	}
	return reason, nil
}

// CitizenLetterDueAt counts `amount` days of `unit` from countFrom through this commune's calendar
// and answers the instant the deadline falls due.
//
// # The count — Bộ luật Dân sự (owner, 08/10/2026; ADR 0085 câu 5, answering ADR 0064 open #3, #4)
//
//  1. countFrom becomes ITS DATE in Asia/Ho_Chi_Minh. The hour is irrelevant: a letter received at
//     16:59 and one received at 07:00 on the same date share a deadline.
//  2. THAT DATE IS NOT COUNTED. Counting starts the next day.
//  3. ngay-lich: + amount dates. ngay-lam-viec: + amount dates on which the commune has at least one
//     working session (holidays removed, swap days added — caCuaNgay decides, as for working hours).
//  4. A last date that is not a working date MOVES to the next working date — for BOTH units. For
//     working days the count already lands on one; the step is written once for both.
//  5. The deadline is the END OF THE LAST WORKING SESSION of that date: the commune's own hours,
//     never a fixed 17:00 — a commune that closes at 16:30 is due at 16:30.
//
// The legal review flag of ADR 0064 still stands on steps 2, 4 and 5.
//
// # What it refuses — every refusal is the configuration's, never an answer
//
// An empty or overlapping week (newWorkingWalk — the same refusal the working-hours count gives; even
// a calendar-day count needs the week, for step 4 and step 5), a holiday that is also a swap day, a
// swap day on a weekday that already works, and a last date past the ChanTroiNgay horizon (holidays
// are entered about a year ahead; a date past them would be "working" only because nobody entered
// its holidays yet).
//
// A unit other than the two day units, or a non-positive amount, is a plain error: the caller has
// already run CheckCitizenLetterDeadlineRule, so reaching here means this service's halves disagree.
func CitizenLetterDueAt(countFrom time.Time, amount int, unit CitizenLetterDeadlineUnit,
	week []CaLamViec, readYear DocLichNam) (time.Time, error) {

	if amount <= 0 {
		return time.Time{}, fmt.Errorf("hạn đơn thư: số ngày %d không hợp lệ", amount)
	}
	if unit != UnitWorkingDays && unit != UnitCalendarDays {
		return time.Time{}, fmt.Errorf("hạn đơn thư: phép đếm ngày không nhận đơn vị %q", unit)
	}

	w, err := newWorkingWalk(countFrom, week, readYear)
	if err != nil {
		return time.Time{}, err
	}

	beyond := func() error {
		return &LoiKhongTinhDuocHan{
			Loai: LoiVuotChanTroi,
			ChiTiet: fmt.Sprintf(
				"lịch làm việc: hạn %d %s rơi quá %d ngày sau ngày bắt đầu — lịch nghỉ lễ và ngày làm bù "+
					"chỉ được khai trước khoảng một năm", amount, unit, ChanTroiNgay),
		}
	}

	// Step 1 and 2: the received date itself is day zero.
	day := nuaDem(countFrom, w.zone)

	switch unit {
	case UnitCalendarDays:
		day = day.AddDate(0, 0, amount)
	case UnitWorkingDays:
		for counted := 0; counted < amount; {
			day = day.AddDate(0, 0, 1)
			if day.After(w.lastDay) {
				return time.Time{}, beyond()
			}
			sessions, err := caCuaNgay(day, w.byWeekday, w.years)
			if err != nil {
				return time.Time{}, err
			}
			if len(sessions) > 0 {
				counted++
			}
		}
	}

	// Step 4 and 5: the first working date at or after `day`, due at the end of its last session.
	for ; ; day = day.AddDate(0, 0, 1) {
		if day.After(w.lastDay) {
			return time.Time{}, beyond()
		}
		sessions, err := caCuaNgay(day, w.byWeekday, w.years)
		if err != nil {
			return time.Time{}, err
		}
		if len(sessions) == 0 {
			continue
		}
		end := sessions[0].KetThuc
		for _, s := range sessions[1:] {
			if s.KetThuc > end {
				end = s.KetThuc
			}
		}
		return day.Add(time.Duration(end) * time.Second), nil
	}
}
