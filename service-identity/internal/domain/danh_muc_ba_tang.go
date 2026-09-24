package domain

// The three-tier rules of a reference catalogue, as pure functions — for this service's two
// catalogues `loai_don_vi_dan_cu` (ResidentialUnitType) and `khoi_nhiem_vu` (TaskBloc).
//
// WHERE THE RULES REALLY LIVE, said first so nothing here is mistaken for the enforcement:
// migrations/0005_don_vi_dan_cu_va_danh_muc.sql, function `danh_muc_ba_tang` (:157-206), a BEFORE
// UPDATE OR DELETE trigger on both tables. That is the floor, and it holds against every writer. The
// rules restated here exist for ONE reason: the trigger answers with a PostgreSQL exception, and an
// exception that reaches a member of staff says nothing they can act on. This layer refuses first,
// in Vietnamese, naming the operation and the tier.
//
// SO A DRIFT BETWEEN THIS FILE AND THE TRIGGER IS NOT A HOLE — it is a worse error message. The
// direction that would be a hole, this layer allowing what the rules forbid, cannot happen: the
// trigger runs last and refuses, and the transaction rolls back with the audit entry inside it
// (rule 6, invariant 3).
//
// THE SAME FILE EXISTS IN FIVE SERVICES (documents, finance, comms, petitions, identity). Each owns
// its own catalogue tables (ADR 0024) and the services are separate Go modules; the only shared home
// would be core/, and this package imports nothing but the standard library (doc.go).
//
// THE NAMES CARRY A `DanhMuc` SUFFIX HERE, WHERE THE OTHER FOUR COPIES DO NOT, and that is forced
// rather than chosen: this package already owns ChuanHoaLyDoXoa / ErrThieuLyDoXoa for the staff
// directory's soft delete (danh_ba_ghi.go), whose sentence names the directory. Reusing them would
// tell an administrator deleting a task bloc that "danh bạ là hồ sơ lưu trữ". The RULES and the
// wire messages are identical to the siblings'; only the Go identifiers differ.

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// The two values `nguon` may hold. The CHECK constraint on both tables admits no third
// (`..._nguon_hop_le`, migration 0005:267, :337).
//
// THE COMMUNE NEVER SUPPLIES THIS. The store writes NguonDonVi as a LITERAL; `nguon` decides which
// tier a row is in, so a client that could name it could put its own row in tier 2 and then walk
// around every guard below (migration 0005:176-181 says the same about the trigger).
const (
	NguonDonVi   = "don-vi"   // the commune added this row itself — tier 1
	NguonHeThong = "he-thong" // the row ships with the software — tier 2, or 3 with MaNguonReNhanh
)

// Tang is which of the three tiers a row is in (ADR 0024 §6; migration 0005:76-81). DERIVED from
// `nguon` and `ma_nguon_re_nhanh`, never stored: two sources for one fact drift.
type Tang int

const (
	// TangDonVi — the commune's own row. Soft delete YES, disable YES, relabel YES.
	TangDonVi Tang = 1
	// TangHeThong — ships with the software. Soft delete NO, disable YES, relabel YES.
	TangHeThong Tang = 2
	// TangReNhanh — ships with the software AND the source code branches on its `ma`.
	// Soft delete NO, disable NO, relabel YES.
	TangReNhanh Tang = 3
)

// TangCua answers which tier a row is in.
//
// A row holding ma_nguon_re_nhanh on a `don-vi` row cannot come out of the database
// (`..._re_nhanh_thi_he_thong`, migration 0005:271); if it ever did, it is reported as tier 3 — the
// stricter reading, the correct direction to be wrong in.
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

// Tang of one catalogue row. ONE METHOD PER CATALOGUE — a shared helper taking two loose columns
// would be a helper every call site could hand the wrong pair to.
func (l LoaiDonViDanCu) Tang() Tang { return TangCua(l.Nguon, l.MaNguonReNhanh) }

func (k KhoiNhiemVu) Tang() Tang { return TangCua(k.Nguon, k.MaNguonReNhanh) }

// The refusals. Separate values, because the HTTP layer maps them to different statuses.
var (
	// ErrKhongXoaDuocMucHeThong — tiers 2 and 3. A system row is taken out of use, never deleted
	// (ADR 0024, consequence #4; migration 0005:190-194).
	ErrKhongXoaDuocMucHeThong = errors.New("danh_muc: mục do hệ thống cấp không xoá được, chỉ tắt được")

	// ErrKhongTatDuocMucReNhanh — tier 3. Disabling a code the source code branches on leaves that
	// branch with no reachable row (migration 0005:196-203).
	ErrKhongTatDuocMucReNhanh = errors.New("danh_muc: mã nguồn có nhánh rẽ theo mục này nên không tắt được")

	// ErrMaBatBien — an issued code is never renumbered (rule 7, invariant 3). `petitions` holds the
	// task-bloc code AS A VALUE and `thon_to_dan_pho.loai` holds the residential-unit-type code; nothing
	// rewrites either.
	ErrMaBatBien = errors.New("danh_muc: `ma` đã cấp thì không đổi được — sửa `nhan`, hoặc thêm dòng mới")

	// ErrNguonDoTuClient — the request named `source` or `tier`. Refused BEFORE anything is written,
	// rather than silently dropped: a field silently ignored is a client that believes it set something.
	ErrNguonDoTuClient = errors.New("danh_muc: `source` và tầng của mục do hệ thống quyết định, không nhận từ yêu cầu")
)

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
// transition true -> false.
func (t Tang) ChoTat() error {
	if t == TangReNhanh {
		return ErrKhongTatDuocMucReNhanh
	}
	return nil
}

// --- validation of what a client may actually supply ---------------------------------------------

var (
	ErrMaDanhMucTrong          = errors.New("danh_muc: thiếu `code`")
	ErrMaDanhMucSaiDinhDang    = errors.New("danh_muc: `code` chỉ gồm chữ thường a-z, số và dấu gạch ngang, ví dụ `cong-van`")
	ErrMaDanhMucQuaDai         = errors.New("danh_muc: `code` quá dài")
	ErrNhanDanhMucTrong        = errors.New("danh_muc: thiếu `label`")
	ErrNhanDanhMucQuaDai       = errors.New("danh_muc: `label` quá dài")
	ErrThuTuDanhMucNgoaiKhoang = errors.New("danh_muc: `order` ngoài khoảng cho phép")
	ErrThieuLyDoXoaDanhMuc     = errors.New("danh_muc: thiếu lý do xoá")
	ErrLyDoXoaDanhMucQuaDai    = errors.New("danh_muc: lý do xoá quá dài")
)

// The bounds — the same four numbers as the four sibling services. They are the point past which a
// value stops being a catalogue entry and starts being a mistake.
const (
	MaDanhMucToiDa      = 64
	NhanDanhMucToiDa    = 200
	ThuTuDanhMucToiDa   = 9999
	LyDoXoaDanhMucToiDa = 500
)

// ChuanHoaMaDanhMuc trims and validates a catalogue code.
//
// THE FORM IS `tiếng Việt không dấu`, kebab-case (ADR 0011): catalogue VALUES stay Vietnamese
// without diacritics while only the surrounding contract is English.
//
// NO CASE FOLDING. `Khoi-Dang` is REFUSED rather than lower-cased: the code stored must be the code
// the person saw, on the one column rule 7 never lets us correct afterwards.
func ChuanHoaMaDanhMuc(ma string) (string, error) {
	ma = strings.TrimSpace(ma)
	switch {
	case ma == "":
		return "", ErrMaDanhMucTrong
	case len(ma) > MaDanhMucToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrMaDanhMucQuaDai, MaDanhMucToiDa)
	}
	if ma[0] == '-' || ma[len(ma)-1] == '-' {
		return "", ErrMaDanhMucSaiDinhDang
	}
	truocLaGach := false
	for i := 0; i < len(ma); i++ {
		c := ma[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			truocLaGach = false
		case c == '-':
			if truocLaGach {
				// `khoi--dang` reads as one code and sorts as another.
				return "", ErrMaDanhMucSaiDinhDang
			}
			truocLaGach = true
		default:
			return "", ErrMaDanhMucSaiDinhDang
		}
	}
	return ma, nil
}

// ChuanHoaNhanDanhMuc trims and validates a display label — the one field every tier may change.
// Vietnamese WITH diacritics; only control characters are refused. Counted in runes.
func ChuanHoaNhanDanhMuc(nhan string) (string, error) {
	nhan = strings.TrimSpace(nhan)
	switch {
	case nhan == "":
		return "", ErrNhanDanhMucTrong
	case len([]rune(nhan)) > NhanDanhMucToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrNhanDanhMucQuaDai, NhanDanhMucToiDa)
	}
	for _, r := range nhan {
		if unicode.IsControl(r) {
			return "", ErrNhanDanhMucTrong
		}
	}
	return nhan, nil
}

// KiemTraThuTuDanhMuc bounds the display order. NEGATIVE IS REFUSED, not clamped.
func KiemTraThuTuDanhMuc(thuTu int) error {
	if thuTu < 0 || thuTu > ThuTuDanhMucToiDa {
		return fmt.Errorf("%w (0..%d)", ErrThuTuDanhMucNgoaiKhoang, ThuTuDanhMucToiDa)
	}
	return nil
}

// ChuanHoaLyDoXoaDanhMuc validates the reason recorded beside a soft delete. MANDATORY — rule 7,
// invariant 1 names `deleted_at`, `deleted_by` AND `delete_reason`.
func ChuanHoaLyDoXoaDanhMuc(lyDo string) (string, error) {
	lyDo = strings.TrimSpace(lyDo)
	switch {
	case lyDo == "":
		return "", ErrThieuLyDoXoaDanhMuc
	case len([]rune(lyDo)) > LyDoXoaDanhMucToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrLyDoXoaDanhMucQuaDai, LyDoXoaDanhMucToiDa)
	}
	return lyDo, nil
}
