package domain

// The rating a citizen gave BY PHONE OR AT THE COUNTER, recorded by a member of staff — the prototype's
// block "Ghi nhận đánh giá của người dân" (vigov-require FeedbackDetailDrawer.tsx:516-540), brought back
// by the owner rule of 11/10/2026 (bug sheet row 60). THIS FILE IMPORTS THE STANDARD LIBRARY AND NOTHING
// ELSE.
//
// # IT REVERSES ADR 0062 DECISIONS 1 AND 3, AND KEEPS DECISION 2
//
// ADR 0062 (30/09/2026) said staff never record a rating and the block does not exist. The owner's rule of
// 11/10/2026 ("chuẩn là prototype — tính năng trên prototype phải được đảm bảo") overrides decisions 1 and
// 3. Decision 2 — every satisfaction figure counts ONLY the citizen's own verdict — is kept BY
// CONSTRUCTION: this record lives in its own table (migration 0042), never in `diem_hai_long`, which every
// figure reads. ADR 0062 itself is to be amended by the main session.
//
// # THE RULES
//
//	stars          1..5, the citizen's range (RatingMinStars..RatingMaxStars)
//	channel        `dien-thoai` | `tai-quay` — the two the prototype's hint names, a closed list
//	note           OPTIONAL, ≤ RatingCommentMaxLen: what the citizen said, and how. Staff free text that
//	               may name people (rule 3) — never in the audit delta, never logged
//	source         ALWAYS `staff_recorded` (StaffRecordedSource); the schema refuses any other value
//	which states   EVERY status — the prototype shows the block whenever no rating exists, whatever the
//	               status (FeedbackDetailDrawer.tsx:516). A soft-deleted petition is simply not found
//	refused        once the CITIZEN has rated (ErrStaffRatingCitizenRated, 409) — theirs is authoritative;
//	               and a second staff record (ErrStaffRatingAlreadyRecorded, 409)
//	later citizen  their rating goes to `diem_hai_long` exactly as before; this record stays as history
//	no reopen      a staff record moves no status and owes the citizen nothing. NOT the prototype's "1–2
//	               sao sẽ tự mở lại phiếu": a reopen is a lifecycle transition with a citizen message (rule
//	               10 stop conditions #2/#3) — left for the owner, not decided here

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// StaffRecordedSource is the value of `source` on every row — the label that keeps a staff-recorded
// rating from ever passing for the citizen's own. The string the main session fixed (11/10/2026).
const StaffRecordedSource = "staff_recorded"

// StaffRatingChannel is how the citizen gave the rating. Enum VALUES per ADR 0011.
type StaffRatingChannel string

const (
	StaffRatingByPhone   StaffRatingChannel = "dien-thoai"
	StaffRatingAtCounter StaffRatingChannel = "tai-quay"
)

// Valid reports whether the channel is one of the two. FAIL CLOSED: anything else is refused.
func (c StaffRatingChannel) Valid() bool {
	return c == StaffRatingByPhone || c == StaffRatingAtCounter
}

// StaffRecordedRating is one row of `petition_staff_recorded_rating`.
type StaffRecordedRating struct {
	ID         string
	PetitionID string // phieu_phan_anh.id, internal
	Stars      int
	Channel    StaffRatingChannel
	Note       string // "" = none (NULL)
	Source     string // always StaffRecordedSource

	// RecordedBy is the STAFF BUSINESS CODE (rule 6, invariant 8).
	RecordedBy string
	RecordedAt time.Time
}

var (
	// ErrStaffRatingChannelInvalid — `channel` missing or not one of the two. 400.
	ErrStaffRatingChannelInvalid = errors.New(
		"phan_anh: `channel` phải là `dien-thoai` hoặc `tai-quay`")

	// ErrStaffRatingNoteTooLong — a note over RatingCommentMaxLen characters. 400. The note is not echoed.
	ErrStaffRatingNoteTooLong = errors.New("phan_anh: `note` quá dài")

	// ErrStaffRatingCitizenRated — the citizen has already rated this petition. 409: their verdict is
	// authoritative and a staff record beside it would be a second, weaker answer to the same question.
	ErrStaffRatingCitizenRated = errors.New(
		"phan_anh: người dân đã tự đánh giá phiếu này — không ghi nhận đánh giá thay được nữa")

	// ErrStaffRatingAlreadyRecorded — a staff record already exists for this petition. 409.
	ErrStaffRatingAlreadyRecorded = errors.New(
		"phan_anh: phiếu này đã có một đánh giá do cán bộ ghi nhận")
)

// CheckStaffRecordedRating validates the three inputs and trims the note. The stars reuse the citizen
// rating's sentinel, so one range answers one way on both routes.
func CheckStaffRecordedRating(stars int, channel, note string) (int, StaffRatingChannel, string, error) {
	if stars < RatingMinStars || stars > RatingMaxStars {
		return 0, "", "", ErrRatingStarsOutOfRange
	}
	ch := StaffRatingChannel(strings.TrimSpace(channel))
	if !ch.Valid() {
		return 0, "", "", ErrStaffRatingChannelInvalid
	}
	note = strings.TrimSpace(note)
	if utf8.RuneCountInString(note) > RatingCommentMaxLen {
		return 0, "", "", fmt.Errorf("%w (tối đa %d ký tự)", ErrStaffRatingNoteTooLong, RatingCommentMaxLen)
	}
	return stars, ch, note, nil
}

// CheckStaffRatingAllowed is the verdict over the petition read under its lock: refused once the citizen
// has rated (`Rating != 0` — `diem_hai_long` is the citizen's column, written by nobody else).
func CheckStaffRatingAllowed(p PhieuPhanAnh) error {
	if p.Rating != 0 {
		return ErrStaffRatingCitizenRated
	}
	return nil
}

// IsStaffRatingInputError reports whether the error is about what was SENT (400). Listed explicitly.
func IsStaffRatingInputError(err error) bool {
	return errors.Is(err, ErrRatingStarsOutOfRange) || errors.Is(err, ErrStaffRatingChannelInvalid) ||
		errors.Is(err, ErrStaffRatingNoteTooLong)
}
