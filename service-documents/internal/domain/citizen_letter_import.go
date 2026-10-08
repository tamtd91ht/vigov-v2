package domain

// The rules of `Nhập từ Excel` on the citizen-letter register (ADR 0084 #6; owner spec
// van-ban-don-thu-theo-prototype-v2 §4.1). THIS FILE ONLY READS AND CHECKS THE SHEET — standard library
// only. Booking each row (number, deadline, trail) is app.CitizenLetterImport, which sends every row
// through the SAME booking code POST /api/v1/citizen-letters runs.
//
// THE COLUMNS are the prototype's (vigov-require book.py:348-359, PETITION_HEADERS), MINUS THREE the
// register never takes from a client:
//
//	Số đến                        the number is the commune's counter's (ErrLetterNumberFromClient)
//	Hạn giải quyết (YYYY-MM-DD)   the deadline is identity's answer for the commune's rule (ADR 0085 B)
//	Lĩnh vực (mã)                 a citizen letter carries no field (migration 0006 has no column)
//
// AND ONE COLUMN READ DIFFERENTLY: the prototype matched `Bộ phận xử lý` by NAME; here it is the unit's
// CODE (`bo_phan.ma`). identityclient.LiveOrgUnitIDsByCode is the only lookup this repository offers for
// a typed unit, and its contract forbids matching by name — two units may share a name, and a guess
// writes a letter into the wrong unit's queue. The header says `(mã)` so the person filling it knows.
//
// TWO PROTOTYPE LENIENCIES ARE NOT COPIED, both because a default on these columns is a silent decision
// about an archival record:
//
//	blank `Loại đơn`   the prototype read it as "kiến nghị". The type decides whether the sender is
//	                   protected (Luật Tố cáo 2018 Đ.8 — ProtectsIdentity); a denunciation left blank
//	                   would be booked as a feedback letter and its sender shown on every list. Refused.
//	blank `Ngày đến`   the prototype let create_petition fill in today. A batch of last month's letters
//	                   would be dated today, and every deadline counted from the wrong day. Refused.
//
// THE SENDER (C7, owner spec §4.4): `Họ tên người gửi` is required, as on the booking form; a letter
// whose sender is genuinely unknown says so with `Không rõ`, which books an EMPTY name — C7's "Không rõ
// người gửi" is derived from empty columns (SenderUnknown), never stored as text.
//
// THE MESSAGES NEVER ECHO A CELL (rule 3, forbidden #3): a cell holds a citizen's name, phone number or
// complaint. They name the row and the column, which is all a person needs to find it.

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// The template's columns, in order — the prototype's labels where it has one.
const (
	LetterImportColReceived = "Ngày đến (YYYY-MM-DD)"
	LetterImportColSender   = "Họ tên người gửi"
	LetterImportColAddress  = "Địa chỉ"
	LetterImportColPhone    = "Số điện thoại"
	LetterImportColType     = "Loại đơn (khiếu nại / tố cáo / kiến nghị / đề nghị)"
	LetterImportColSummary  = "Nội dung đơn"
	LetterImportColUnit     = "Bộ phận xử lý (mã)"
)

// LetterImportColumns is the header row, in order.
func LetterImportColumns() []string {
	return []string{LetterImportColReceived, LetterImportColSender, LetterImportColAddress, LetterImportColPhone,
		LetterImportColType, LetterImportColSummary, LetterImportColUnit}
}

// MaxLetterImportRows bounds one file — the document-type import's 200, which is also the ceiling of one
// identity lookup (identityclient.MaxLookupKeysPerCall), so the unit codes of a whole file are ONE call.
const MaxLetterImportRows = 200

// MaxLetterImportUnitCode bounds the typed unit code before it is sent to identity.
const MaxLetterImportUnitCode = 64

// LetterSenderUnknownWord is what a row writes for a sender nobody knows.
const LetterSenderUnknownWord = "Không rõ"

// LetterImportRow is one data row, every shape rule already applied. Row is the spreadsheet's own
// number (header = 1). UnitCode is "" when the row names no unit.
type LetterImportRow struct {
	Row           int
	ReceivedDate  time.Time // a calendar day, UTC midnight — the shape the booking route parses
	Type          LetterType
	SenderName    string // "" = Không rõ
	SenderPhone   string
	SenderAddress string
	Summary       string
	UnitCode      string
}

// LetterImportError is one refusal. Row 0 and Column "" mean the FILE.
type LetterImportError struct {
	Row     int
	Column  string
	Message string
}

// ReadLetterImportSheet turns core/xlsx.ReadSheet's rows into checked data rows, or returns EVERY error.
// rows is nil whenever errs is non-empty, so a caller cannot book half a file by accident. now is the
// instant of the request — a received date after today is refused, as the booking route refuses it.
func ReadLetterImportSheet(sheet [][]string, now time.Time) ([]LetterImportRow, []LetterImportError) {
	if len(sheet) == 0 {
		return nil, []LetterImportError{{Message: "Tệp không có dòng tiêu đề."}}
	}
	want := LetterImportColumns()
	head := sheet[0]
	ok := len(head) >= len(want)
	for i := 0; ok && i < len(want); i++ {
		ok = strings.TrimSpace(head[i]) == want[i]
	}
	for i := len(want); ok && i < len(head); i++ {
		ok = strings.TrimSpace(head[i]) == ""
	}
	if !ok {
		return nil, []LetterImportError{{Message: "Dòng tiêu đề không khớp tệp mẫu (cần đúng bảy cột: " +
			strings.Join(want, " · ") + "). Hãy tải tệp mẫu và nhập vào đó."}}
	}

	var (
		rows []LetterImportRow
		errs []LetterImportError
	)
	data := 0
	for i, cells := range sheet[1:] {
		if blankImportCells(cells) {
			continue
		}
		data++
		if data > MaxLetterImportRows {
			return nil, []LetterImportError{{Message: fmt.Sprintf(
				"Tệp có hơn %d dòng đơn thư; tối đa %d dòng cho một lần nhập. Hãy chia thành nhiều tệp.",
				MaxLetterImportRows, MaxLetterImportRows)}}
		}
		r, rowErrs := readLetterImportRow(i+2, cells, len(want), now)
		errs = append(errs, rowErrs...)
		if len(rowErrs) == 0 {
			rows = append(rows, r)
		}
	}
	if data == 0 {
		return nil, []LetterImportError{{Message: "Tệp không có dòng đơn thư nào dưới dòng tiêu đề."}}
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return rows, nil
}

func readLetterImportRow(rowNo int, raw []string, width int, now time.Time) (LetterImportRow, []LetterImportError) {
	r := LetterImportRow{Row: rowNo}
	var errs []LetterImportError
	fail := func(col, msg string) { errs = append(errs, LetterImportError{Row: rowNo, Column: col, Message: msg}) }

	for j := width; j < len(raw); j++ {
		if strings.TrimSpace(raw[j]) != "" {
			fail("", "Dòng có dữ liệu ngoài bảy cột của tệp mẫu.")
			break
		}
	}
	cell := func(j int) string {
		if j < len(raw) {
			return strings.TrimSpace(raw[j])
		}
		return ""
	}

	if day, msg := parseLetterImportDate(cell(0)); msg != "" {
		fail(LetterImportColReceived, msg)
	} else if err := CheckReceivedDate(day, now); err != nil {
		if errors.Is(err, ErrLetterReceivedDateFuture) {
			fail(LetterImportColReceived, "Ngày đến ở tương lai.")
		} else {
			fail(LetterImportColReceived, "Ngày đến quá xa trong quá khứ.")
		}
	} else {
		r.ReceivedDate = day
	}

	switch name := cell(1); {
	case name == "":
		fail(LetterImportColSender, "Thiếu họ tên người gửi. Nếu không biết người gửi, ghi \""+LetterSenderUnknownWord+"\".")
	case foldLetterImportWord(name) == foldLetterImportWord(LetterSenderUnknownWord):
		r.SenderName = "" // C7: derived "Không rõ người gửi"
	default:
		if s, err := TrimOptional(name, MaxSenderName); err != nil {
			fail(LetterImportColSender, fmt.Sprintf("Họ tên người gửi dài quá %d ký tự.", MaxSenderName))
		} else {
			r.SenderName = s
		}
	}

	if s, err := TrimOptional(cell(2), MaxSenderAddress); err != nil {
		fail(LetterImportColAddress, fmt.Sprintf("Địa chỉ dài quá %d ký tự.", MaxSenderAddress))
	} else {
		r.SenderAddress = s
	}

	if s, err := TrimPhone(cell(3)); err != nil {
		if errors.Is(err, ErrLetterPhoneInvalid) {
			fail(LetterImportColPhone, "Số điện thoại chỉ gồm chữ số, dấu cách, dấu +, -, . và ngoặc.")
		} else {
			fail(LetterImportColPhone, fmt.Sprintf("Số điện thoại dài quá %d ký tự.", MaxSenderPhone))
		}
	} else {
		r.SenderPhone = s
	}

	if t, msg := parseLetterImportType(cell(4)); msg != "" {
		fail(LetterImportColType, msg)
	} else {
		r.Type = t
	}

	if s, err := TrimRequired(cell(5), MaxLetterSummary, ErrLetterSummaryMissing); err != nil {
		if errors.Is(err, ErrLetterSummaryMissing) {
			fail(LetterImportColSummary, "Thiếu nội dung đơn.")
		} else {
			fail(LetterImportColSummary, fmt.Sprintf("Nội dung đơn dài quá %d ký tự.", MaxLetterSummary))
		}
	} else {
		r.Summary = s
	}

	if code := cell(6); len([]rune(code)) > MaxLetterImportUnitCode {
		fail(LetterImportColUnit, fmt.Sprintf("Mã bộ phận dài quá %d ký tự.", MaxLetterImportUnitCode))
	} else {
		r.UnitCode = code
	}
	return r, errs
}

// excelEpoch is day zero of Excel's 1900 date system as every modern workbook stores it (the 1900 leap
// year bug folded in: serial 61 is 1 March 1900).
var excelEpoch = time.Date(1899, time.December, 30, 0, 0, 0, 0, time.UTC)

// parseLetterImportDate reads `Ngày đến`. Accepted: the template's YYYY-MM-DD, the Vietnamese
// d/m/yyyy, and a spreadsheet DATE cell — which core/xlsx hands over raw, as its serial day number.
// The time of day of a serial is dropped: the column is a calendar day. Never a default: blank refuses.
func parseLetterImportDate(s string) (time.Time, string) {
	const unreadable = "Ngày đến không đọc được — ghi theo dạng YYYY-MM-DD (ví dụ 2026-10-07) hoặc ngày/tháng/năm."
	if s == "" {
		return time.Time{}, "Thiếu ngày đến."
	}
	for _, layout := range []string{time.DateOnly, "2/1/2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, ""
		}
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil && !math.IsNaN(f) && f >= 1 && f < 2958466 {
		return excelEpoch.AddDate(0, 0, int(math.Floor(f))), ""
	}
	return time.Time{}, unreadable
}

// parseLetterImportType reads `Loại đơn`: the prototype's words, the form's label "Kiến nghị, phản ánh",
// or the code itself — case and diacritics ignored. Blank refuses (file header).
func parseLetterImportType(s string) (LetterType, string) {
	if s == "" {
		return "", "Thiếu loại đơn — ghi một trong: khiếu nại, tố cáo, kiến nghị (phản ánh), đề nghị."
	}
	switch foldLetterImportWord(s) {
	case "khieu nai":
		return LetterTypeComplaint, ""
	case "to cao":
		return LetterTypeDenunciation, ""
	case "kien nghi", "phan anh", "kien nghi phan anh":
		return LetterTypeFeedback, ""
	case "de nghi":
		return LetterTypeRequest, ""
	}
	return "", "Loại đơn phải là một trong: khiếu nại, tố cáo, kiến nghị (phản ánh), đề nghị."
}

// foldLetterImportWord lower-cases, strips Vietnamese diacritics (vietnameseBase) and turns every run of
// anything but a letter or digit into one space: "Kiến nghị, phản ánh" and "kien-nghi-phan-anh" are
// both "kien nghi phan anh".
func foldLetterImportWord(s string) string {
	var b strings.Builder
	sep := false
	for _, r := range s {
		r = unicode.ToLower(r)
		if base, ok := vietnameseBase[r]; ok {
			r = base
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if sep && b.Len() > 0 {
				b.WriteByte(' ')
			}
			sep = false
			b.WriteRune(r)
			continue
		}
		sep = true
	}
	return b.String()
}
