package domain

// The shape rules of one org-chart node, as it arrives on a write route (14-cau-hinh.md §1).
//
// STANDARD LIBRARY ONLY, like every file in this package. There is deliberately no
// golang.org/x/text here: the slug table below covers exactly the Vietnamese alphabet, and a
// general Unicode normaliser would be a dependency whose only job is to be right about letters this
// system never sees in a unit name.
//
// NOTHING HERE IS PERSONAL DATA (rule 3): a unit's name and code describe the authority's
// organisation, not a person — which is why, unlike danh_ba_ghi.go, these refusals may be logged.

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The refusals. Sentinels, so the HTTP layer maps them to 400 by identity (app.LaLoiDauVaoBoPhan).
var (
	ErrThieuTenBoPhan      = errors.New("bo_phan: thiếu tên bộ phận")
	ErrTenBoPhanQuaDai     = errors.New("bo_phan: tên bộ phận quá dài")
	ErrTenBoPhanKyTuLa     = errors.New("bo_phan: tên bộ phận chứa ký tự điều khiển")
	ErrMaBoPhanSaiDinhDang = errors.New("bo_phan: mã bộ phận chỉ gồm chữ thường không dấu, chữ số và dấu gạch ngang đơn, không đứng đầu hay cuối")
	ErrMaBoPhanQuaDai      = errors.New("bo_phan: mã bộ phận quá dài")

	// ErrTenKhongSinhDuocMa — a name with no Latin letter or digit at all ("—", "!!!") produces an
	// empty slug. REFUSED rather than replaced by a random code: the code is permanent (rule 7,
	// invariant 3) and a meaningless one is a permanent meaningless identifier. The person can type
	// one.
	ErrTenKhongSinhDuocMa = errors.New("bo_phan: không sinh được mã từ tên này — hãy nhập mã")

	ErrThuTuBoPhanAm     = errors.New("bo_phan: thứ tự không được âm")
	ErrThuTuBoPhanQuaLon = errors.New("bo_phan: thứ tự quá lớn")
	ErrIDChaQuaDai       = errors.New("bo_phan: mã tham chiếu bộ phận cha quá dài")
)

const (
	// TranTenBoPhan — "THƯỜNG TRỰC ỦY BAN MẶT TRẬN TỔ QUỐC VIỆT NAM XÃ …" fits in a third of it.
	TranTenBoPhan = 200

	// TranMaBoPhan bounds a slug, typed or generated. A generated slug longer than this is CUT at a
	// word boundary rather than refused: the name is valid, only its derived key is long.
	TranMaBoPhan = 80

	// TranThuTuBoPhan — a commune runs about ten units; 9999 refuses a payload, never a real rank.
	TranThuTuBoPhan = 9999
)

// ChuanHoaTenBoPhan trims a unit name and bounds it.
//
// THE CASE IS KEPT AS TYPED. The specification says names are written in upper case (§1, "viết
// HOA"), and that is the COMMUNE'S convention, not a rule this service enforces: upper-casing here
// would rewrite what the person typed into an archival record, and strings.ToUpper is not even
// reliably right for every Vietnamese letter a keyboard can produce. The screen can upper-case the
// input box.
//
// Counted in RUNES: Vietnamese is up to three bytes per accented letter.
func ChuanHoaTenBoPhan(tho string) (string, error) {
	ten := strings.TrimSpace(tho)
	switch {
	case ten == "":
		return "", ErrThieuTenBoPhan
	case utf8.RuneCountInString(ten) > TranTenBoPhan:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrTenBoPhanQuaDai, TranTenBoPhan)
	}
	for _, r := range ten {
		if unicode.IsControl(r) {
			// A newline inside a unit name breaks every dropdown and every printed routing slip.
			return "", ErrTenBoPhanKyTuLa
		}
	}
	return ten, nil
}

// ChuanHoaMaBoPhan validates a code the CLIENT typed.
//
// NO CASE FOLDING AND NO REPAIR: `Van-Phong` is refused, not lower-cased. The code is permanent and
// never reissued (rule 7, invariant 3), so a code stored differently from the one the person saw
// is a mistake nobody may correct afterwards. Same discipline as service-comms'
// ChuanHoaSlugDanhMuc, written again here because a service never imports another service's
// internal packages (rule 2, forbidden #1).
func ChuanHoaMaBoPhan(tho string) (string, error) {
	ma := strings.TrimSpace(tho)
	if len(ma) > TranMaBoPhan {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrMaBoPhanQuaDai, TranMaBoPhan)
	}
	if !laMaHopLe(ma) {
		return "", ErrMaBoPhanSaiDinhDang
	}
	return ma, nil
}

// laMaHopLe: non-empty, [a-z0-9] runs joined by single '-'.
func laMaHopLe(ma string) bool {
	if ma == "" || ma[0] == '-' || ma[len(ma)-1] == '-' {
		return false
	}
	truocLaGach := false
	for i := 0; i < len(ma); i++ {
		c := ma[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			truocLaGach = false
		case c == '-':
			if truocLaGach {
				return false
			}
			truocLaGach = true
		default:
			return false
		}
	}
	return true
}

// boDauTiengViet maps every lower-case Vietnamese letter carrying a diacritic onto its base letter.
//
// WRITTEN OUT RATHER THAN DERIVED, because the standard library has no Unicode decomposition and
// `đ` would survive a decomposition anyway — it is a letter of its own, not `d` plus a mark. The
// table is lower case only: SinhMaBoPhan lowers first, and unicode.ToLower is correct for every
// upper-case letter here (Ă → ă, Ư → ư, Đ → đ).
var boDauTiengViet = func() map[rune]rune {
	nhom := map[rune]string{
		'a': "àáảãạăằắẳẵặâầấẩẫậ",
		'e': "èéẻẽẹêềếểễệ",
		'i': "ìíỉĩị",
		'o': "òóỏõọôồốổỗộơờớởỡợ",
		'u': "ùúủũụưừứửữự",
		'y': "ỳýỷỹỵ",
		'd': "đ",
	}
	ra := make(map[rune]rune, 80)
	for goc, cac := range nhom {
		for _, r := range cac {
			ra[r] = goc
		}
	}
	return ra
}()

// SinhMaBoPhan derives the slug from a unit name: "VĂN PHÒNG ĐẢNG ỦY" → "van-phong-dang-uy".
//
// Lower-case, strip Vietnamese diacritics (đ → d), every run of anything else becomes ONE '-', no
// leading or trailing '-'. A letter outside the Vietnamese alphabet with a mark on it (é is in the
// table; ñ is not) counts as a separator — this is a key, not a transliteration.
//
// CUT AT TranMaBoPhan, AT THE LAST '-' INSIDE THE BOUND when there is one, so a long name yields a
// shorter slug of whole words rather than a word cut in half.
//
// The result is NOT guaranteed free in the commune. Uniqueness is the store's question (the unique
// key counts soft-deleted rows too); see app.SoDoToChuc.Them for the suffix.
func SinhMaBoPhan(ten string) (string, error) {
	var b strings.Builder
	canGach := false
	for _, r := range ten {
		r = unicode.ToLower(r)
		if goc, ok := boDauTiengViet[r]; ok {
			r = goc
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if canGach && b.Len() > 0 {
				b.WriteByte('-')
			}
			canGach = false
			b.WriteRune(r)
			continue
		}
		canGach = true
	}
	ma := b.String()
	if len(ma) > TranMaBoPhan {
		ma = ma[:TranMaBoPhan]
		if i := strings.LastIndexByte(ma, '-'); i > 0 {
			ma = ma[:i]
		}
		ma = strings.TrimRight(ma, "-")
	}
	if ma == "" {
		return "", ErrTenKhongSinhDuocMa
	}
	return ma, nil
}

// MaBoPhanThuN is the n-th candidate for a generated slug: n = 1 is the base itself, n ≥ 2 appends
// "-n". The base is shortened first when the suffix would push it past TranMaBoPhan, so every
// candidate is still a valid code.
func MaBoPhanThuN(goc string, n int) string {
	if n <= 1 {
		return goc
	}
	duoi := "-" + strconv.Itoa(n)
	if len(goc)+len(duoi) > TranMaBoPhan {
		goc = strings.TrimRight(goc[:TranMaBoPhan-len(duoi)], "-")
	}
	return goc + duoi
}

// KiemTraThuTuBoPhan bounds the rank a commune arranges its units in.
func KiemTraThuTuBoPhan(n int) error {
	switch {
	case n < 0:
		return ErrThuTuBoPhanAm
	case n > TranThuTuBoPhan:
		return fmt.Errorf("%w (tối đa %d)", ErrThuTuBoPhanQuaLon, TranThuTuBoPhan)
	}
	return nil
}

// KiemTraIDCha bounds a parent reference before it reaches SQL. "" is legitimate: the root.
func KiemTraIDCha(id string) error {
	if len(id) > 64 {
		return ErrIDChaQuaDai
	}
	return nil
}
