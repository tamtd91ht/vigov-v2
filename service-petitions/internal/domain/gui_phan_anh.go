package domain

// What a CITIZEN may put in a petition they file themselves, and the bounds each value is held to.
//
// THIS FILE IMPORTS THE STANDARD LIBRARY AND NOTHING ELSE (rule 4 of the service pattern). Every
// rule here is a pure function over a value, so it can be proved without a database, without a
// network and without a commune's configuration.
//
// # WHY THE BOUNDS LIVE HERE AND NOT IN THE DATABASE
//
// MEASURED, NOT ASSUMED: migration 0004 puts NO length constraint on any column of
// `phieu_phan_anh` — `noi_dung` is a bare `TEXT`, which PostgreSQL will happily accept a gigabyte
// into. The only ceiling that exists today is the 64 KiB body cap in internal/http (`thanToiDa`),
// and that one answers "Nội dung gửi lên không phải JSON hợp lệ hoặc quá lớn" — a sentence that
// tells a citizen nothing about which box on their screen is too long.
//
// So the refusal has to be here, where it can name the field and the limit. The database is NOT
// the second layer for this one, and saying so out loud matters: a reader who assumes a `CHECK`
// underneath would delete these functions as duplication.
//
// # THE FOUR NUMBERS ARE AN ASSUMPTION, AND THEY ARE STATED AS ONE
//
// docs/ui-ux/09-phan-anh-nguoi-dan.md specifies NO maximum for any of these fields, and neither
// does any ADR. They are bounds on an UNBOUNDED INPUT FROM AN UNAUTHENTICATED-IN-PRACTICE PARTY
// (a citizen identity is weak, rule 4), not business rules — the smallest number that cannot
// refuse a genuine report. If the customer sets real limits these change; nothing else does.

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// The bounds, in RUNES and not bytes.
//
// COUNTED IN RUNES BECAUSE THE TEXT IS VIETNAMESE. Every diacritic is 2–3 bytes in UTF-8, so a
// byte limit refuses a Vietnamese report at roughly a third of the length it refuses an English
// one — and the person who hits it is the citizen this channel exists for.
const (
	// NoiDungToiDa — the report itself. Long enough for several paragraphs of a detailed
	// complaint; short enough that no single request carries a document.
	NoiDungToiDa = 4000

	// DiaChiToiDa — "Đầu ngõ thôn Hà Lam" and the long-winded versions of it.
	DiaChiToiDa = 500

	// HoTenToiDa — the same bound as the catalogue label, for the same reason: a name longer
	// than this is not a name.
	HoTenToiDa = 200

	// DienThoaiToiDa — a Vietnamese number is 10 digits; the room is for country codes,
	// separators and the way people actually type a number.
	//
	// THE FORMAT IS DELIBERATELY NOT VALIDATED, only the length. Which shapes a commune accepts
	// — landline, +84, an extension, a relative's number — is a business rule nobody has stated,
	// and a regex invented here would refuse a real citizen's real number at the one moment they
	// are trying to report something.
	DienThoaiToiDa = 32
)

var (
	// ErrNoiDungTrong — a petition with no content is not a petition. It is the ONE mandatory
	// field on this channel: the field code is not (nobody knows it yet, ADR 0028 decision E) and
	// neither is the reporter (ADR 0008 allows an anonymous filing).
	ErrNoiDungTrong    = errors.New("phan_anh: `content` trống — chưa có nội dung phản ánh")
	ErrNoiDungQuaDai   = errors.New("phan_anh: `content` quá dài")
	ErrDiaChiQuaDai    = errors.New("phan_anh: `address` quá dài")
	ErrHoTenQuaDai     = errors.New("phan_anh: `reporter_name` quá dài")
	ErrDienThoaiQuaDai = errors.New("phan_anh: `reporter_phone` quá dài")
)

// ChuanHoaNoiDung trims and bounds the report text.
//
// TRIMMED BEFORE IT IS MEASURED, so a form that submits a textarea full of newlines is refused as
// empty rather than stored as a blank petition holding a lookup code and a commitment.
//
// THE MESSAGE NEVER ECHOES THE INPUT (rule 3, forbidden #3). The text of a petition is citizen
// personal data, and an error travels into centralised logging across every commune at once.
func ChuanHoaNoiDung(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", ErrNoiDungTrong
	case utf8.RuneCountInString(s) > NoiDungToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrNoiDungQuaDai, NoiDungToiDa)
	}
	return s, nil
}

// ChuanHoaDiaChi trims and bounds the place the incident is at. EMPTY IS VALID — the specification
// renders a missing address as "Chưa rõ vị trí", and a citizen reporting from the spot often has
// nothing to type.
func ChuanHoaDiaChi(s string) (string, error) {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > DiaChiToiDa {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrDiaChiQuaDai, DiaChiToiDa)
	}
	return s, nil
}

// ChuanHoaHoTen trims and bounds the reporter's name. EMPTY IS VALID (ADR 0008: a petition may be
// filed anonymously, and even a named one does not require the box to be filled).
func ChuanHoaHoTen(s string) (string, error) {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > HoTenToiDa {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrHoTenQuaDai, HoTenToiDa)
	}
	return s, nil
}

// ChuanHoaDienThoai trims and bounds the callback number. EMPTY IS VALID, and the format is not
// checked — see DienThoaiToiDa.
func ChuanHoaDienThoai(s string) (string, error) {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > DienThoaiToiDa {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrDienThoaiQuaDai, DienThoaiToiDa)
	}
	return s, nil
}

// LaLoiGuiPhanAnh reports whether this is a refusal of what the citizen sent, as opposed to a
// failure of the system.
//
// LISTED EXPLICITLY RATHER THAN DEFAULTING TO 400 — the same discipline as
// internal/http.laLoiDauVao and for the same measured reason: a default of "anything I do not
// recognise is the sender's fault" turns an identity outage into a 400, and the Mini App then
// tells a citizen to fix their report forever while nobody is told the server is broken.
func LaLoiGuiPhanAnh(err error) bool {
	for _, mot := range []error{
		ErrNoiDungTrong, ErrNoiDungQuaDai,
		ErrDiaChiQuaDai, ErrHoTenQuaDai, ErrDienThoaiQuaDai,
	} {
		if errors.Is(err, mot) {
			return true
		}
	}
	return false
}
