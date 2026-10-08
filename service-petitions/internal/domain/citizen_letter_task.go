package domain

// Creating a task FROM a citizen letter (POST /api/v1/citizen-letter-tasks, ADR 0085 A; ADR 0084 #6) —
// the two rules that need no outside answer: the deadline the task inherits, and who it goes to when
// the dialog named nobody.

import (
	"errors"
	"strings"
	"time"
)

// CitizenLetterIDMaxLen mirrors ResolveCitizenLetterForTaskRequest.letter_id's ceiling
// (proto/vigov/documents/v1/documents.proto). A longer id names no letter; refused as input rather than
// sent.
const CitizenLetterIDMaxLen = 64

// ErrCitizenLetterIDInvalid refuses a body whose `letter_id` is blank or longer than any letter id. 400.
var ErrCitizenLetterIDInvalid = errors.New(
	"nhiệm vụ: thiếu `letter_id` hoặc mã đơn thư không hợp lệ — chọn đơn trên sổ đơn thư")

// ErrCitizenLetterAssignmentRequired refuses a task with neither a unit nor an assignee, after the
// letter's own holder was tried (ADR 0085 §Trả lời #6, owner 08/10/2026; prototype `assignment_required`).
// NOT "Chưa xác định": a task nobody holds sits on no list anybody opens. The sentence on the wire is the
// owner's, CitizenLetterAssignmentRequiredSentence.
var ErrCitizenLetterAssignmentRequired = errors.New(
	"nhiệm vụ: hộp thoại không chọn bộ phận hay người thực hiện, và đơn thư cũng chưa có bộ phận đang giữ")

// CitizenLetterAssignmentRequiredSentence is the refusal as the owner worded it (08/10/2026).
const CitizenLetterAssignmentRequiredSentence = "Chọn bộ phận hoặc người thực hiện."

// CheckCitizenLetterID trims nothing and answers the id as given: the id checked is the id sent to
// documents and the id stored in `nguon_id`.
func CheckCitizenLetterID(id string) error {
	if strings.TrimSpace(id) == "" || len(id) > CitizenLetterIDMaxLen {
		return ErrCitizenLetterIDInvalid
	}
	return nil
}

// citizenLetterZone is Asia/Ho_Chi_Minh as a FIXED +07:00, for muiGioChoDan's reason
// (xu_ly_phan_anh.go): no daylight saving since 1975, and a fixed zone cannot fail to load.
var citizenLetterZone = time.FixedZone("ICT", 7*3600)

// citizenLetterTaskHour is C9's (24/09/2026) "hạn = ngày hạn của đơn lúc 17:00".
const citizenLetterTaskHour = 17

// CitizenLetterTaskDeadline is the task's deadline from the letter's current deadline (ADR 0085 A6):
// the Vietnam DATE of `letterDue`, at 17:00 Vietnam time, returned in UTC. ZERO IN, ZERO OUT — a letter
// with no deadline gives a task with none ("Không đặt hạn" is inherited, never defaulted).
//
// PINNING A TIME OF DAY ON A DATE, NOT COUNTING A DURATION: rule 10 forbidden #2 is about adding working
// time, which this does not do. A letter due at 23:30 on the 20th (Vietnam) keeps the 20th — the DATE in
// Vietnam, not in UTC, which would be the 20th at 16:30Z and the 21st at 17:00Z respectively.
//
// ⚠ A LETTER DUE AFTER 17:00 GIVES A TASK DUE EARLIER THE SAME DAY. That is C9's rule as written ("ngày
// hạn của đơn lúc 17:00") and the prototype's; it is stated, not corrected here.
func CitizenLetterTaskDeadline(letterDue time.Time) time.Time {
	if letterDue.IsZero() {
		return time.Time{}
	}
	local := letterDue.In(citizenLetterZone)
	return time.Date(local.Year(), local.Month(), local.Day(), citizenLetterTaskHour, 0, 0, 0,
		citizenLetterZone).UTC()
}

// CitizenLetterTaskHolder decides who the task goes to (ADR 0085 §Trả lời #6, prototype
// service.py:658-670): what the dialog named, if it named EITHER; otherwise the letter's holding unit and
// assignee together. Neither anywhere → ErrCitizenLetterAssignmentRequired.
//
// ALL OR NOTHING FROM EACH SIDE, as the prototype does: a dialog naming only a person does not borrow the
// letter's unit — the officer chose, and half of a choice mixed with half of the letter's would be a
// pairing nobody made.
func CitizenLetterTaskHolder(bodyUnit, bodyAssignee, letterUnit, letterAssignee string) (unit, assignee string, err error) {
	if bodyUnit != "" || bodyAssignee != "" {
		return bodyUnit, bodyAssignee, nil
	}
	if letterUnit == "" && letterAssignee == "" {
		return "", "", ErrCitizenLetterAssignmentRequired
	}
	return letterUnit, letterAssignee, nil
}
