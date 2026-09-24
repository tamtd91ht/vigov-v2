package domain

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// The filters and the free-text search of the staff register — GET /api/v1/staff and
// POST /api/v1/staff/searches (user decision 2026-09-24).
//
// TWO SURFACES FOR ONE FILTER SET, AND THE SPLIT IS RULE 3, NOT TASTE. `unit` and `published` are
// a department id and a boolean: neither says anything about a person, so they ride on the URL.
// The search text is whatever the administrator typed into "Tìm theo tên, chức vụ, số điện
// thoại", which is a person's name or telephone number more often than not — and a URL is copied
// into access logs, proxy logs and browser history (rule 3, forbidden #4). So the text travels
// only in a request BODY, and this type is the one value both routes hand to the store.

// LocCanBo is the filter set of one register read. Every field's zero value means "no filter on
// this", and all present fields are AND-combined.
//
// THE COMMUNE IS NOT A FIELD AND MUST NEVER BECOME ONE: it is bound to $1 from the context by
// store.Scoped (rule 1, invariant 4). A filter struct carrying a tenant is the shortest path to a
// client choosing its own commune.
type LocCanBo struct {
	// BoPhanID — `bo_phan_id`. "" = every department, including people in none.
	BoPhanID string
	// CongKhai — `hien_tren_mini_app`. nil = both. A pointer because false is a real filter
	// ("who is NOT yet on the Mini App") and must not collapse into "no filter".
	CongKhai *bool
	// TuKhoa — the search text, ALREADY normalised by ChuanHoaTuKhoaTimCanBo. "" = no text search.
	// Only the POST route sets it; the GET route has no way to.
	TuKhoa string
}

var (
	ErrThieuTuKhoa  = errors.New("can_bo: thiếu từ khoá tìm kiếm")
	ErrTuKhoaQuaDai = errors.New("can_bo: từ khoá tìm kiếm quá dài")
)

// TuKhoaTimCanBoToiDa bounds the search text, in CHARACTERS.
//
// CHARACTERS AND NOT BYTES, AND THAT IS THE WHOLE POINT OF THIS CONSTANT. A Vietnamese letter with
// its diacritics is two or three bytes in UTF-8 ("ễ" is three), so a byte limit of 200 refuses a
// name of about seventy letters — which is exactly how a sibling service was found refusing
// ordinary Vietnamese input. The figure itself matches service-comms' TuKhoaTimToiDa: far past any
// name, position or telephone number, and short enough that an unindexable `ILIKE '%…%'` over one
// commune's register stays cheap.
const TuKhoaTimCanBoToiDa = 200

// ChuanHoaTuKhoaTimCanBo trims the search text, collapses its internal runs of whitespace, and
// bounds it.
//
// COLLAPSING MATCHES WHAT IS STORED: ChuanHoaHoTen collapses the name before it is written, so
// "Nguyễn  Văn" (two spaces, a copy-paste from Excel) must be searched as "Nguyễn Văn" or it finds
// nobody.
//
// REQUIRED. A search with no text is the list, and the list already has a route — GET
// /api/v1/staff — which is also the one that pages without a body. Accepting an empty `q` here
// would give the same read two contracts.
//
// NEITHER ERROR QUOTES THE INPUT: it is very likely a person's name or number, and an error
// message is the string that reaches logs, consoles and support screenshots (rule 3, forbidden #3).
func ChuanHoaTuKhoaTimCanBo(tho string) (string, error) {
	tu := strings.Join(strings.Fields(tho), " ")
	if tu == "" {
		return "", ErrThieuTuKhoa
	}
	if utf8.RuneCountInString(tu) > TuKhoaTimCanBoToiDa {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrTuKhoaQuaDai, TuKhoaTimCanBoToiDa)
	}
	return tu, nil
}

// ChuSoTimSoDienThoai returns the digits of a search text that LOOKS LIKE a telephone number, or
// "" when it does not.
//
// "Looks like" means: every character is one a telephone number may be written with
// (kyTuSoDienThoai — the same set ChuanHoaSoDienThoai accepts on the way in) and there is at least
// one digit. That lets "0900 000 001" and "0900.000.001" find a number stored as "0900000001", and
// the reverse, because the store compares digits against the column's own digits.
//
// A TEXT MIXING LETTERS AND DIGITS GETS NO DIGIT MATCH. "Thôn 3" is a position, not a number, and
// stripping it to "3" would match every telephone number containing a 3.
//
// WHAT THIS DOES NOT DO: a leading "+84" is kept as the digits "84", so "+84 900 000 001" does NOT
// find "0900000001". Rewriting a country prefix is a rule about numbering plans, and it is not
// asked for; the ordinary ILIKE still finds a number typed the way it was stored.
func ChuSoTimSoDienThoai(tu string) string {
	var b strings.Builder
	for _, r := range tu {
		if !strings.ContainsRune(kyTuSoDienThoai, r) {
			return ""
		}
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
