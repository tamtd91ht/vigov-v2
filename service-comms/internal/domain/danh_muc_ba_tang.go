package domain

// The three-tier rules of a reference catalogue, as pure functions.
//
// WHERE THE RULES REALLY LIVE, said first so nothing here is mistaken for the enforcement:
// migrations/0003_danh_muc_loai_tai_nguyen_ban_do.sql, function `danh_muc_ba_tang`, a BEFORE UPDATE OR
// DELETE trigger. That is the floor, and it holds against every writer — this service, a psql
// prompt, a future import job. The rules restated here exist for ONE reason: the trigger answers
// with a PostgreSQL exception, and an exception that reaches a member of staff says nothing they
// can act on and carries a driver's wording into a log line. This layer refuses first, in
// Vietnamese, naming the operation and the tier.
//
// SO A DRIFT BETWEEN THIS FILE AND THE TRIGGER IS NOT A HOLE — it is a worse error message. The
// direction that would be a hole, this layer allowing what the trigger allows but the rules
// forbid, cannot happen: the trigger runs last and refuses, and the transaction rolls back with
// the audit entry inside it (rule 6, invariant 3).
//
// WHY THE SAME FILE EXISTS IN FIVE SERVICES. Each service owns its own catalogue table (ADR 0024)
// and the services are separate Go modules; the only shared home would be core/, and this package
// imports nothing but the standard library on purpose (doc.go). The copies are identical by
// intent — see the note above for why a drift costs a message rather than a guarantee.

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// The two values `nguon` may hold. The CHECK constraint on every catalogue table admits no third.
//
// THE COMMUNE NEVER SUPPLIES THIS. A write route writes NguonDonVi as a literal; `nguon` decides
// which tier a row is in, so a client that could name it could put its own row in tier 2 and then
// walk around every guard below. The migration says the same thing where the trigger refuses an
// edit of the column: "Were it editable, every guard below could be stepped around by setting
// nguon = 'don-vi' first."
const (
	NguonDonVi   = "don-vi"   // the commune added this row itself — tier 1
	NguonHeThong = "he-thong" // the row ships with the software — tier 2, or 3 with MaNguonReNhanh
)

// Tang is which of the three tiers a row is in. DERIVED from `nguon` and `ma_nguon_re_nhanh`,
// never stored: two sources for one fact drift, and the stale one is what a screen would read.
type Tang int

const (
	// TangDonVi — the commune's own row. Soft delete YES, disable YES, relabel YES.
	TangDonVi Tang = 1
	// TangHeThong — ships with the software. Soft delete NO, disable YES, relabel YES.
	TangHeThong Tang = 2
	// TangReNhanh — ships with the software AND the source code branches on its `ma`.
	// Soft delete NO, disable NO, relabel YES. Relabelling is the only operation left, and it is
	// deliberately still allowed: the wording on a screen is the commune's, the code is not.
	TangReNhanh Tang = 3
)

// TangCua answers which tier a row is in.
//
// The pairing is not symmetric and that is the schema's own constraint, not a simplification here:
// `loai_tai_nguyen_ban_do_re_nhanh_thi_he_thong` refuses ma_nguon_re_nhanh on a `don-vi` row, because the
// software cannot branch on a code it has never seen. A row that somehow held both would be
// reported as tier 3 by this function — the stricter reading, which is the correct direction to
// be wrong in.
func TangCua(nguon string, maNguonReNhanh bool) Tang {
	switch {
	case maNguonReNhanh:
		return TangReNhanh
	case nguon == NguonHeThong:
		return TangHeThong
	default:
		return TangDonVi
	}
}

// Tang of one catalogue row.
func (l LoaiTaiNguyenBanDo) Tang() Tang { return TangCua(l.Nguon, l.MaNguonReNhanh) }

// The refusals. Separate values rather than one error with a message, because the HTTP layer maps
// them to different statuses and a caller telling them apart by string comparison is a caller that
// breaks when somebody fixes a typo.
var (
	// ErrKhongXoaDuocMucHeThong — tiers 2 and 3. A system row is taken out of use, never deleted
	// (ADR 0024, consequence #4; docs/ui-ux/14-cau-hinh.md:182).
	ErrKhongXoaDuocMucHeThong = errors.New("danh_muc: mục do hệ thống cấp không xoá được, chỉ tắt được")

	// ErrKhongTatDuocMucReNhanh — tier 3. The specification allows the one operation the system
	// cannot survive: disabling a code the source code branches on leaves that branch with no
	// reachable row, and the screen offering the button reports nothing wrong.
	ErrKhongTatDuocMucReNhanh = errors.New("danh_muc: mã nguồn có nhánh rẽ theo mục này nên không tắt được")

	// ErrMaBatBien — an issued code is never renumbered (rule 7, invariant 3). Business records
	// hold this code AS A VALUE and nothing rewrites them.
	ErrMaBatBien = errors.New("danh_muc: `ma` đã cấp thì không đổi được — sửa `nhan`, hoặc thêm dòng mới")

	// ErrNguonDoTuClient — the request named `nguon` or `ma_nguon_re_nhanh`. Refused BEFORE
	// anything is written, with a message that says why, rather than being silently dropped: a
	// field silently ignored is a client that believes it set something.
	ErrNguonDoTuClient = errors.New("danh_muc: `source` và tầng của mục do hệ thống quyết định, không nhận từ yêu cầu")
)

// Cho phép hoặc từ chối từng thao tác. One function per operation rather than one `Cho(op)`: the
// caller names the operation at the call site, so a new operation cannot silently fall into a
// default branch that allows it.

// ChoXoaMem reports whether this row may be soft deleted. Tier 1 only.
func (t Tang) ChoXoaMem() error {
	if t != TangDonVi {
		return fmt.Errorf("%w (tầng %d)", ErrKhongXoaDuocMucHeThong, int(t))
	}
	return nil
}

// ChoTat reports whether this row may be taken out of use. Tiers 1 and 2.
//
// RE-ENABLING IS NOT THE SAME QUESTION and is always allowed — the trigger only refuses the
// transition true -> false. A tier-3 row that somehow ended up disabled must be able to come back.
func (t Tang) ChoTat() error {
	if t == TangReNhanh {
		return ErrKhongTatDuocMucReNhanh
	}
	return nil
}

// --- validation of what a client may actually supply ---------------------------------------------

var (
	ErrMaTrong          = errors.New("danh_muc: thiếu `code`")
	ErrMaSaiDinhDang    = errors.New("danh_muc: `code` chỉ gồm chữ thường a-z, số và dấu gạch ngang, ví dụ `cong-van`")
	ErrMaQuaDai         = errors.New("danh_muc: `code` quá dài")
	ErrNhanTrong        = errors.New("danh_muc: thiếu `label`")
	ErrNhanQuaDai       = errors.New("danh_muc: `label` quá dài")
	ErrThuTuNgoaiKhoang = errors.New("danh_muc: `order` ngoài khoảng cho phép")
	ErrThieuLyDoXoa     = errors.New("danh_muc: thiếu lý do xoá")
	ErrLyDoXoaQuaDai    = errors.New("danh_muc: lý do xoá quá dài")
)

// The bounds. They are not business rules and are not pretending to be: they are the point past
// which a value stops being a catalogue entry and starts being a mistake or an attack. An
// unbounded client-supplied string in a government database is a liability, not a feature — the
// same argument PhienStore.Tao makes for the user-agent column.
const (
	MaToiDa      = 64
	NhanToiDa    = 200
	ThuTuToiDa   = 9999
	LyDoXoaToiDa = 500
)

// ChuanHoaMa trims and validates a catalogue code.
//
// THE FORM IS `tiếng Việt không dấu`, kebab-case — `cong-van`, `quyet-dinh` — and that is ADR 0011,
// not a preference: catalogue VALUES stay Vietnamese without diacritics while only the surrounding
// contract is English. Accepting anything else here would let a commune mint `Công Văn` or
// `cong_van`, and the code goes straight into business records as a value nothing ever rewrites.
//
// NO CASE FOLDING. `Cong-Van` is REFUSED rather than lower-cased: silently changing a code the
// person typed means the code stored is not the code they saw, on the one column rule 7 never lets
// us correct afterwards.
func ChuanHoaMa(ma string) (string, error) {
	ma = strings.TrimSpace(ma)
	switch {
	case ma == "":
		return "", ErrMaTrong
	case len(ma) > MaToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrMaQuaDai, MaToiDa)
	}
	// Hand-rolled rather than a regexp: the rule is four lines, and a regexp here would be one more
	// thing to read carefully in five copies of this file.
	if ma[0] == '-' || ma[len(ma)-1] == '-' {
		return "", ErrMaSaiDinhDang
	}
	truocLaGach := false
	for i := 0; i < len(ma); i++ {
		c := ma[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			truocLaGach = false
		case c == '-':
			if truocLaGach {
				// `cong--van` reads as one code and sorts as another. Refuse it while it is still
				// a typo rather than after it is a value in a document record.
				return "", ErrMaSaiDinhDang
			}
			truocLaGach = true
		default:
			return "", ErrMaSaiDinhDang
		}
	}
	return ma, nil
}

// ChuanHoaNhan trims and validates a display label.
//
// THE LABEL IS THE ONE FIELD EVERY TIER MAY CHANGE, including tier 3 where it is the only operation
// left. It carries Vietnamese WITH diacritics — it is a sentence a person reads — so nothing here
// restricts the character set beyond refusing control characters, which would corrupt a screen and
// a log line alike.
func ChuanHoaNhan(nhan string) (string, error) {
	nhan = strings.TrimSpace(nhan)
	switch {
	case nhan == "":
		return "", ErrNhanTrong
	case len([]rune(nhan)) > NhanToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrNhanQuaDai, NhanToiDa)
	}
	for _, r := range nhan {
		if unicode.IsControl(r) {
			return "", ErrNhanTrong
		}
	}
	return nhan, nil
}

// KiemTraThuTu bounds the display order.
//
// NEGATIVE IS REFUSED, not clamped. A client sending -1 to mean "first" would work until a second
// client sent -2, and the order a commune arranged its own catalogue in would then depend on who
// edited last.
func KiemTraThuTu(thuTu int) error {
	if thuTu < 0 || thuTu > ThuTuToiDa {
		return fmt.Errorf("%w (0..%d)", ErrThuTuNgoaiKhoang, ThuTuToiDa)
	}
	return nil
}

// ChuanHoaLyDoXoa validates the reason recorded beside a soft delete.
//
// MANDATORY, AND THAT IS RULE 7, INVARIANT 1: `deleted_at`, `deleted_by` AND `delete_reason`. A
// row that disappeared from every screen with no reason attached is a row nobody can explain when
// somebody asks why a document type vanished — and the row is still there, so the question WILL be
// asked.
func ChuanHoaLyDoXoa(lyDo string) (string, error) {
	lyDo = strings.TrimSpace(lyDo)
	switch {
	case lyDo == "":
		return "", ErrThieuLyDoXoa
	case len([]rune(lyDo)) > LyDoXoaToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrLyDoXoaQuaDai, LyDoXoaToiDa)
	}
	return lyDo, nil
}
