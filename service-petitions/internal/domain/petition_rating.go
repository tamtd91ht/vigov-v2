package domain

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// The citizen's star rating of a petition — ADR 0050 point 2, decided by the owner in rounds 2 and 3
// of 28/09/2026. THIS FILE IMPORTS THE STANDARD LIBRARY AND NOTHING ELSE.
//
//	stars 1..5, optional comment ≤ 1000 characters
//	allowed at `da-xu-ly` and `cho-dan-xac-nhan`, and nowhere else
//	stars ≥ 3   the rating is recorded, the STATUS IS UNCHANGED (the petition waits for staff to close)
//	stars 1–2   the rating is recorded AND the petition reopens to `dang-xu-ly`: no cap on how many
//	            times, NO DEADLINE RECOMPUTED OR MOVED (rule 10, invariant 2)
//	rating again after a reopen and a second resolve REPLACES the earlier rating
//
// NOTHING HERE IS PER-COMMUNE CONFIGURATION, and that is the decision rather than a shortcut: ADR 0050
// replaced ADR 0008's `nguong_sao_mo_lai`, `so_lan_mo_lai_toi_da`, `tinh_lai_han_khi_mo_lai` and
// `cho_phep_mo_lai` with one fixed rule for every commune. Reading any of them would re-introduce a
// switch the owner removed (migration 0017's header says the same to anyone about to read them).

const (
	// RatingMinStars and RatingMaxStars bound a rating — migration 0004's
	// `phieu_phan_anh_diem_hai_long_hop_le`, repeated here so an out-of-range value is a 400 and not
	// a 500 from the CHECK.
	RatingMinStars = 1
	RatingMaxStars = 5

	// RatingReopenThreshold is the HIGHEST rating that reopens the petition: 1 or 2 stars.
	//
	// A FIXED NUMBER AND NOT CONFIGURATION (ADR 0050 point 2: "Ngưỡng là con số cố định '1 hoặc 2 sao',
	// không phải cấu hình"). Changing it is a decision of the owner and a change to this line, never a
	// row a commune edits.
	RatingReopenThreshold = 2

	// RatingCommentMaxLen bounds the comment IN CHARACTERS — migration 0017's
	// `phieu_phan_anh_rating_comment_valid` counts with char_length, so validating the same number in
	// runes here is what turns an oversized comment into a 400 instead of a 500.
	RatingCommentMaxLen = 1000
)

var (
	// ErrRatingStarsOutOfRange — stars missing, zero, or outside 1..5.
	ErrRatingStarsOutOfRange = errors.New("phan_anh: `stars` phải là số nguyên từ 1 đến 5")

	// ErrRatingCommentTooLong — a comment over RatingCommentMaxLen characters.
	ErrRatingCommentTooLong = errors.New("phan_anh: `comment` quá dài")

	// ErrRatingNotOpen — the petition is not at a point where the citizen's verdict means anything:
	// the work is not finished yet, it has been closed, or it went down a terminal branch. A refusal
	// about the STATE OF THE RECORD, answered 409, never 400.
	ErrRatingNotOpen = errors.New(
		"phan_anh: chỉ đánh giá được phiếu đã xử lý xong hoặc đang chờ người dân xác nhận")
)

// CheckRating validates the stars and trims the optional comment.
//
// A BLANK COMMENT AFTER TRIM IS "NO COMMENT", returned as "" and stored as NULL: migration 0017's
// CHECK refuses a blank string, and a textarea of spaces must not become a 500.
//
// THE INPUT IS NOT ECHOED in any error — the comment is citizen free text (rule 3, forbidden #3).
func CheckRating(stars int, comment string) (int, string, error) {
	if stars < RatingMinStars || stars > RatingMaxStars {
		return 0, "", ErrRatingStarsOutOfRange
	}
	comment = strings.TrimSpace(comment)
	if utf8.RuneCountInString(comment) > RatingCommentMaxLen {
		return 0, "", fmt.Errorf("%w (tối đa %d ký tự)", ErrRatingCommentTooLong, RatingCommentMaxLen)
	}
	return stars, comment, nil
}

// RatingOpen reports whether a petition in status `t` may be rated now.
//
// `da-xu-ly` AND `cho-dan-xac-nhan`, AND NOTHING ELSE (ADR 0050 point 2). FAIL CLOSED: every other
// status, and any string that is not one of the nine, is refused.
//
// `da-dong` IS REFUSED ON PURPOSE even though the map has `da-dong -> dang-xu-ly`: a closed petition
// carries a result the citizen has already been told is final, and reopening it from a rating is a
// lifecycle nobody decided (rule 10, stop condition #2).
func RatingOpen(t TrangThai) bool {
	return t == DaXuLy || t == ChoDanXacNhan
}

// RatingReopens reports whether a rating of `stars` reopens the petition — 1 or 2 stars.
func RatingReopens(stars int) bool {
	return stars >= RatingMinStars && stars <= RatingReopenThreshold
}

// ReopenByRatingAllowed is the one gate of the reopen edge: the status is one a rating is allowed at
// AND the lifecycle map has the edge to `dang-xu-ly`. Both are checked so the two declarations can
// never disagree in the permissive direction — the same discipline as DongDuoc.
func ReopenByRatingAllowed(t TrangThai) bool {
	return RatingOpen(t) && t.ChuyenSangDuoc(DangXuLy)
}

// IsRatingInputError reports whether the error is about what the citizen SENT (400), as opposed to
// the state of the record (409) or a failure of the system (500). Listed explicitly, never a default —
// the same discipline as LaLoiXuLyPhanAnh.
func IsRatingInputError(err error) bool {
	return errors.Is(err, ErrRatingStarsOutOfRange) || errors.Is(err, ErrRatingCommentTooLong)
}
