package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// The petition processing logbook `nhat_ky_phan_anh` (migration 0013) — the timeline column of the
// petition drawer (docs/ui-ux/09 §8.7).
//
// IT IS NOT THE AUDIT LOG AND DOES NOT REPLACE IT. `audit_log` answers "who changed what" for the
// whole service and is invisible to a commune; this is a BUSINESS record an officer reads. Both are
// written, in the same transaction, for the same act (rule 6, invariant 3).
//
// IT IS STAFF-INTERNAL. Nothing in this file may reach the citizen surface (rule 4, forbidden #5;
// rule 10, invariant 7).

// HanhViNhatKy is the kind of act one timeline row records. The closed list is migration 0013's
// CHECK `nhat_ky_phan_anh_hanh_vi_hop_le`; a new act is a migration, never a new constant alone.
type HanhViNhatKy string

const (
	NhatKyPhanLoai        HanhViNhatKy = "phan-loai"
	NhatKyPhanCong        HanhViNhatKy = "phan-cong"
	NhatKyChuyenTrangThai HanhViNhatKy = "chuyen-trang-thai"
	NhatKyDongPhieu       HanhViNhatKy = "dong-phieu"
	NhatKyKhongTiepNhan   HanhViNhatKy = "khong-tiep-nhan"
	NhatKyChuyenCapTren   HanhViNhatKy = "chuyen-cap-tren"
	NhatKyGhiChu          HanhViNhatKy = "ghi-chu"

	// The citizen's rating (ADR 0050 point 2) — the two rows the requirement repository writes as
	// timeline events (`service.py:826-838`). TWO CODES AND NOT ONE, so the drawer can tell "rated,
	// nothing moved" from "rated and the petition came back" without parsing the sentence.
	//
	// ⚠ BOTH NEED MIGRATION 0013's CHECK WIDENED (a new migration, `nhat_ky_phan_anh_hanh_vi_hop_le`).
	// Until it lands every rating rolls back on the CHECK; internal/app's
	// TestLogActionsAreAllowedByTheSchema is red for exactly that reason.
	LogActionCitizenRating  HanhViNhatKy = "danh-gia"
	LogActionReopenByRating HanhViNhatKy = "mo-lai-theo-danh-gia"
)

// CitizenLogActor is what `nguoi_ma` holds on a row the CITIZEN caused.
//
// A FIXED MARKER AND NOT THE CITIZEN'S ID, and this is a deliberate departure from the requirement
// repository (which stores the citizen's user id as `actor_id`). The column is read on a STAFF screen,
// and this repository never puts `cong_dan_id` there — phieuPhanAnhRa carries `has_citizen`, a boolean,
// for that reason — and on an ANONYMOUS petition an id on the timeline would let staff link one
// person's anonymous reports together (ADR 0008). The trail keeps the real actor: audit_log.actor_id is
// the citizen id with Kind "citizen". `cong-dan` cannot collide with a staff business code (`CB-…`) and
// satisfies the column's non-blank CHECK.
const CitizenLogActor = "cong-dan"

// RatingLogText is the timeline sentence for a rating — the requirement repository's wording
// (`service.py:830-834`). It carries the stars and nothing the citizen typed: the comment lives in
// `phieu_phan_anh.rating_comment` only.
func RatingLogText(stars int, reopened bool) string {
	if reopened {
		return fmt.Sprintf("Người dân đánh giá %d sao — phiếu được mở lại", stars)
	}
	return fmt.Sprintf("Người dân đánh giá %d sao", stars)
}

// NhatKyPhanAnh is one timeline row.
//
// BoPhanID AND CanBoXuLyMa ARE SET ON `phan-cong` AND ONLY THERE — migration 0013's
// `nhat_ky_phan_anh_phan_cong_du_truong` refuses them on any other act, because a stray assignee on
// a note row reads as a hand-over that never happened. CanBoXuLyMa is a STAFF BUSINESS CODE.
//
// ⚠ NoiDung IS PERSONAL DATA (rule 3). Staff free text that will eventually quote the reporter, their
// number or their address. Never logged, never in an error message, never on an event, never in an
// audit delta (its LENGTH may be).
type NhatKyPhanAnh struct {
	ID             string
	PhieuPhanAnhID string
	ThoiDiem       time.Time
	NguoiMa        string
	HanhVi         HanhViNhatKy
	TrangThai      TrangThai // the status the petition stood in at this moment — AFTER the act
	BoPhanID       string
	CanBoXuLyMa    string
	NoiDung        string
}

// GhiChuToiDa bounds a note, IN CHARACTERS: migration 0013's `nhat_ky_phan_anh_noi_dung_toi_da`
// counts with char_length, and validating the same number in runes here is what turns an oversized
// note into a 400 instead of a 500. Same number as KetQuaToiDa, by the migration's own definition.
const GhiChuToiDa = KetQuaToiDa

var (
	// ErrThieuGhiChu — the manual note route with nothing in it. Migration 0013 refuses a blank
	// `ghi-chu` row, and this table is never edited afterwards.
	ErrThieuGhiChu = errors.New("phan_anh: `note` trống — ghi chú phải có nội dung")

	// ErrGhiChuQuaDai — a note over GhiChuToiDa characters, on any of the seven routes that take one.
	ErrGhiChuQuaDai = errors.New("phan_anh: `note` quá dài")
)

// KiemGhiChuTuyChon trims and bounds the OPTIONAL internal note an act may carry.
//
// BLANK AFTER TRIM IS ABSENT, returned as "" and stored as NULL, never as an empty string. Migration
// 0013's CHECK refuses a blank remark on a non-note row (an empty string is not "no remark"), so a
// textarea of newlines must become NULL here rather than a 500 there.
//
// THE INPUT IS NOT ECHOED in the error: it may hold citizen personal data (rule 3, forbidden #3).
func KiemGhiChuTuyChon(s string) (string, error) {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > GhiChuToiDa {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrGhiChuQuaDai, GhiChuToiDa)
	}
	return s, nil
}

// KiemGhiChu is the MANDATORY form, for the manual `ghi-chu` row: the same bound, and blank refused.
func KiemGhiChu(s string) (string, error) {
	s, err := KiemGhiChuTuyChon(s)
	if err != nil {
		return "", err
	}
	if s == "" {
		return "", ErrThieuGhiChu
	}
	return s, nil
}
