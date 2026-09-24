package domain

// The shape rules of one staff record, as it arrives on a write route.
//
// THIS FILE IMPORTS NOTHING BUT THE STANDARD LIBRARY (rule 4 of the service pattern) and knows
// nothing about SQL, HTTP or the commune. What it owns is the question "is this value a sensible
// thing to store", which is the ONE question that must be answered BEFORE a transaction opens: a
// request that fails its shape has to be told why, not handed a rollback.
//
// WHY THE LIMITS EXIST AT ALL, when every column here is an unbounded `TEXT`. A field with no
// ceiling is a field a client chooses the size of, on a process serving 200+ communes, and the
// row it writes is an archival record nobody may delete afterwards. The numbers are deliberately
// far above any real value — a Vietnamese full name and a commune position both fit inside 150
// characters with room to spare — so they never refuse honest input; they refuse a payload.
//
// NO ERROR MESSAGE HERE EVER QUOTES THE VALUE IT REFUSED (rule 3, forbidden #3). Every string
// this file inspects is personal data or close to it: a name, a work e-mail, two telephone
// numbers. The sentences name the FIELD and the RULE, which is what the person filling the form
// needs, and they are safe to log, to return and to read out.

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The refusals. Sentinels, so the HTTP layer maps them to 400 by identity rather than by
// matching on text — see http/can_bo_ghi.go laLoiDauVaoCanBo.
var (
	ErrThieuHoTen        = errors.New("can_bo: thiếu họ và tên")
	ErrHoTenQuaDai       = errors.New("can_bo: họ và tên quá dài")
	ErrChucVuQuaDai      = errors.New("can_bo: chức vụ quá dài")
	ErrThieuEmail        = errors.New("can_bo: thiếu thư điện tử")
	ErrEmailSaiDinhDang  = errors.New("can_bo: thư điện tử không đúng định dạng")
	ErrEmailQuaDai       = errors.New("can_bo: thư điện tử quá dài")
	ErrSoDienThoaiSai    = errors.New("can_bo: số điện thoại chứa ký tự không dùng được")
	ErrSoDienThoaiQuaDai = errors.New("can_bo: số điện thoại quá dài")
	ErrIDThamChieuQuaDai = errors.New("can_bo: mã tham chiếu quá dài")
	ErrThuTuDanhBaAm     = errors.New("can_bo: thứ tự hiển thị trong danh bạ không được âm")
	ErrThuTuDanhBaQuaLon = errors.New("can_bo: thứ tự hiển thị trong danh bạ quá lớn")
)

const (
	tranHoTen       = 150
	tranChucVu      = 150
	tranEmail       = 254 // RFC 5321 §4.5.3.1.3 — the longest address a server must accept
	tranSoDienThoai = 32
	tranIDThamChieu = 64 // a ULID is 26; a slug the commune types is shorter still
)

// ChuanHoaHoTen trims and collapses the runs of whitespace a copy-paste from Excel leaves behind.
//
// COLLAPSING IS A REAL NORMALISATION AND NOT COSMETIC: `nguoi_dung.ho_ten` is what the Danh bạ
// screen searches and sorts on, and "Nguyễn  Văn A" with two spaces sorts and matches as a
// different person from "Nguyễn Văn A". The commune's directory arrives by Excel import
// (12-danh-ba-can-bo.md §6), which is exactly where doubled spaces come from.
//
// IT IS REQUIRED. A directory row with no name is a row no screen can render and no audit trail
// can explain, and `ho_ten` is NOT NULL in the schema — refusing here produces a sentence instead
// of a constraint violation.
func ChuanHoaHoTen(tho string) (string, error) {
	ten := gomKhoangTrang(tho)
	if ten == "" {
		return "", ErrThieuHoTen
	}
	if utf8.RuneCountInString(ten) > tranHoTen {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrHoTenQuaDai, tranHoTen)
	}
	return ten, nil
}

// ChuanHoaChucVu is the same treatment, but OPTIONAL: `chuc_vu` is NOT NULL DEFAULT ” and a
// person may legitimately hold no stated position.
func ChuanHoaChucVu(tho string) (string, error) {
	cv := gomKhoangTrang(tho)
	if utf8.RuneCountInString(cv) > tranChucVu {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrChucVuQuaDai, tranChucVu)
	}
	return cv, nil
}

// ChuanHoaEmail trims, lower-cases and checks the shape of a work address.
//
// LOWER-CASING IS LOAD-BEARING, NOT TIDINESS. `UNIQUE (tenant_id, email)` is case SENSITIVE, so
// `A.B@xa.gov.vn` and `a.b@xa.gov.vn` are two different rows to PostgreSQL and one single mailbox
// to the person. Two rows means two staff members with one address — and the sign-in path looks
// the address up with an exact match (store.CanBoStore.TheoEmail), so which of the two can sign in
// depends on how somebody typed it. Normalising on the WRITE path is the only place this can be
// fixed once; doing it on the read path would leave the duplicate rows in the table.
//
// IT IS REQUIRED, AND THAT IS A DEVIATION FROM THE SPECIFICATION WORTH READING BEFORE CHANGING.
// docs/ui-ux/12-danh-ba-can-bo.md §5 does not mark Email required. The schema does, in effect:
// `email TEXT NOT NULL` carries `UNIQUE (tenant_id, email)`, so the empty string is a VALUE and a
// commune may hold it exactly ONCE. The second person with no address would be refused by the
// server with a message about a unique key — a failure that reads as a bug, arrives at whoever is
// typing the twenty-sixth row, and has no fix from the screen. Refusing the FIRST one with a
// sentence is the honest version of the same limit.
//
// → It is also a FINDING, not a decision this file is entitled to make permanent: the schema fix
// is a partial unique index (`WHERE email <> ”`), which is a migration on a table whose key is
// already deployed. Reported to the user rather than written here.
//
// THE CHECK IS DELIBERATELY WEAK: one `@`, something either side, no whitespace, a dot in the
// domain. Every stricter rule anybody writes refuses somebody's real address, and the address is
// verified by the only test that means anything — an e-mail arriving at it.
func ChuanHoaEmail(tho string) (string, error) {
	em := strings.ToLower(strings.TrimSpace(tho))
	if em == "" {
		return "", ErrThieuEmail
	}
	if utf8.RuneCountInString(em) > tranEmail {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrEmailQuaDai, tranEmail)
	}
	if strings.ContainsFunc(em, unicode.IsSpace) {
		return "", ErrEmailSaiDinhDang
	}
	truoc, sau, co := strings.Cut(em, "@")
	if !co || truoc == "" || sau == "" || strings.Contains(sau, "@") || !strings.Contains(sau, ".") {
		return "", ErrEmailSaiDinhDang
	}
	return em, nil
}

// kyTuSoDienThoai is what a telephone number may contain once the surrounding spaces are gone.
//
// NO FORMAT IS IMPOSED, and migration 0009 §2 says why in full: a CHECK on shape would refuse an
// area code in brackets, an extension, or an international prefix — all of which a commune's real
// directory contains. What IS refused is a character that cannot be part of any telephone number,
// because the field is otherwise a free-text box on a government record that later feeds a `tel:`
// link on the Mini App.
const kyTuSoDienThoai = "0123456789+-()., "

// ChuanHoaSoDienThoai trims and checks one telephone number. OPTIONAL — both columns are
// NOT NULL DEFAULT ”, and "" is the one spelling of "no number" (migration 0009 §2).
//
// THE SAME FUNCTION SERVES BOTH COLUMNS AND THAT IS CORRECT, even though the two are different
// kinds of data in law (#16): they differ in who may SEE them, never in what a telephone number
// may contain. The distinction lives at the edge, where masking and export decide — never here.
func ChuanHoaSoDienThoai(tho string) (string, error) {
	so := strings.TrimSpace(tho)
	if so == "" {
		return "", nil
	}
	if utf8.RuneCountInString(so) > tranSoDienThoai {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrSoDienThoaiQuaDai, tranSoDienThoai)
	}
	for _, r := range so {
		if !strings.ContainsRune(kyTuSoDienThoai, r) {
			// The refused character is NOT quoted back: one character of a telephone number is
			// still personal data, and an error message is the one string that reaches a log
			// aggregator, a browser console and a screenshot in a support ticket.
			return "", ErrSoDienThoaiSai
		}
	}
	return so, nil
}

// KiemTraIDThamChieu bounds an id the client supplies for a department or a role.
//
// IT DOES NOT CHECK THAT THE ROW EXISTS, and must not: that is the foreign key's job, inside the
// transaction, where the answer cannot go stale between the check and the write. What this stops
// is a megabyte of text arriving in a column comparison.
//
// An EMPTY id is legitimate and means "none": `bo_phan_id` and `vai_tro_id` are both nullable —
// a person can sit in the directory belonging to no unit and holding no role.
func KiemTraIDThamChieu(id string) error {
	if utf8.RuneCountInString(id) > tranIDThamChieu {
		return fmt.Errorf("%w (tối đa %d ký tự)", ErrIDThamChieuQuaDai, tranIDThamChieu)
	}
	return nil
}

// thuTuDanhBaToiDa is the ceiling of `thu_tu_danh_ba`, which is a PostgreSQL INTEGER. It is the
// column's own range, not a business limit: past it the server refuses the write with an overflow
// error that would reach the person as a 500.
const thuTuDanhBaToiDa = 1<<31 - 1

// KiemTraThuTuDanhBa checks an explicit directory position. NIL IS LEGITIMATE and means "no
// explicit order" (migration 0010 §1). Negative values have no meaning on a form asking for a
// display position, and the database refuses them too (`nguoi_dung_thu_tu_danh_ba_khong_am`);
// refusing here produces a sentence instead of a constraint violation.
func KiemTraThuTuDanhBa(thuTu *int) error {
	if thuTu == nil {
		return nil
	}
	if *thuTu < 0 {
		return ErrThuTuDanhBaAm
	}
	if *thuTu > thuTuDanhBaToiDa {
		return fmt.Errorf("%w (tối đa %d)", ErrThuTuDanhBaQuaLon, thuTuDanhBaToiDa)
	}
	return nil
}

// gomKhoangTrang collapses every run of whitespace into one ordinary space and trims the ends.
// strings.Fields splits on all Unicode whitespace, which is what arrives from a spreadsheet —
// non-breaking spaces and tabs included.
func gomKhoangTrang(tho string) string {
	return strings.Join(strings.Fields(tho), " ")
}
