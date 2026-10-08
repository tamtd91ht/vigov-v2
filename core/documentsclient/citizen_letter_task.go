package documentsclient

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc/status"

	documentsv1 "github.com/vihat/vigov/core/gen/vigov/documents/v1"
)

// MaxCitizenLetterIDLen mirrors ResolveCitizenLetterForTaskRequest.letter_id's ceiling
// (proto/vigov/documents/v1/documents.proto, the source of truth — rule 2, invariant 7). Refused here
// rather than sent: the server would answer INVALID_ARGUMENT, which reads as a fault, not as a refusal.
const MaxCitizenLetterIDLen = 64

// CitizenLetterEligibility is the answer to "may a task be created from this letter". The zero value
// is NOT an answer: an error always travels beside it.
type CitizenLetterEligibility int

const (
	// CitizenLetterEligible: a live letter of this commune, not a denunciation. The facts are set.
	CitizenLetterEligible CitizenLetterEligibility = iota + 1
	// CitizenLetterNotFound: unknown, soft-deleted or another commune's id — one answer (rule 1).
	CitizenLetterNotFound
	// CitizenLetterDenunciation: a live `to-cao` letter. NOTHING about it is returned (C9, ADR 0084 #6).
	CitizenLetterDenunciation
)

// CitizenLetterForTask is the letter as a task inherits it. Codes, ids and one instant — no personal
// data, not even the summary (see the RPC). Every field but Eligibility is empty unless ELIGIBLE.
type CitizenLetterForTask struct {
	Eligibility CitizenLetterEligibility

	LetterID string
	// Number/Year are the letter's register number, for the task's audit entry ("đơn số 12/2026").
	Number uint32
	Year   uint32

	// HoldingUnitID / AssigneeCode: the fallback when the dialog names neither. "" = none. NOT
	// validated by documents — the caller re-checks both with identity before writing.
	HoldingUnitID string
	AssigneeCode  string

	// CurrentDueAt is the deadline the letter shows now, UTC. ZERO = the letter has no deadline, and the
	// task is created with none — never replaced by a default.
	CurrentDueAt time.Time
}

// ErrCitizenLetterAnswerInvalid marks an OK answer that breaks the contract (UNSPECIFIED or unknown
// eligibility, facts on a non-eligible answer, facts missing on an eligible one, another letter's id).
// FAIL CLOSED: never read as any of the three answers — the caller refuses as for an outage.
var ErrCitizenLetterAnswerInvalid = errors.New(
	"documentsclient: ResolveCitizenLetterForTask trả lời sai hợp đồng — từ chối, không đoán")

// ResolveCitizenLetterForTask asks documents whether a task may be created from letter `letterID` of
// the commune in ctx, and the facts the task inherits. Not cached: the answer is point-in-time and a
// cached ELIGIBLE could outlive a soft delete.
//
// # READING THE ANSWER — fail closed
//
//	nil error       Eligibility is one of the three values; facts set iff Eligible.
//	error           the call did not happen, or its answer broke the contract. NEVER "not found",
//	                never "eligible". errors.Is(err, ErrDocumentsUnavailable) when retrying may work.
//
// A blank or over-long id is refused before the wire: the caller validates the body first, so arriving
// here with one is a caller bug, not a letter that does not exist.
func (c *Client) ResolveCitizenLetterForTask(ctx context.Context, letterID string) (CitizenLetterForTask, error) {
	if strings.TrimSpace(letterID) == "" || len(letterID) > MaxCitizenLetterIDLen {
		return CitizenLetterForTask{}, fmt.Errorf(
			"documentsclient: ResolveCitizenLetterForTask với mã đơn rỗng hoặc dài quá %d — bên gọi phải kiểm trước",
			MaxCitizenLetterIDLen)
	}

	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()

	resp, err := c.cl.ResolveCitizenLetterForTask(ctx,
		&documentsv1.ResolveCitizenLetterForTaskRequest{LetterId: letterID})
	if err != nil {
		// No letter id in the line: an id is not personal data, but the log needs only the code.
		c.log.WarnContext(ctx, "CẢNH BÁO: không hỏi được documents về đơn thư để chuyển thành nhiệm vụ",
			"ma_loi", status.Code(err).String(), "err", err)
		return CitizenLetterForTask{}, wrapCallError("ResolveCitizenLetterForTask", err)
	}
	return readCitizenLetterAnswer(letterID, resp)
}

// readCitizenLetterAnswer turns the wire answer into the typed one, refusing every shape the contract
// says cannot occur. A separate function so the contract checks are tested without a server.
func readCitizenLetterAnswer(asked string, resp *documentsv1.ResolveCitizenLetterForTaskResponse) (
	CitizenLetterForTask, error) {

	letter := resp.GetLetter()
	switch resp.GetEligibility() {
	case documentsv1.CitizenLetterTaskEligibility_CITIZEN_LETTER_TASK_ELIGIBILITY_NOT_FOUND:
		if letter != nil {
			return CitizenLetterForTask{}, fmt.Errorf("%w: NOT_FOUND kèm thông tin đơn", ErrCitizenLetterAnswerInvalid)
		}
		return CitizenLetterForTask{Eligibility: CitizenLetterNotFound}, nil

	case documentsv1.CitizenLetterTaskEligibility_CITIZEN_LETTER_TASK_ELIGIBILITY_DENUNCIATION:
		// A denunciation carrying facts is the one leak C9 forbids. Refused, and the facts are dropped
		// with the error — nothing of them is returned or logged.
		if letter != nil {
			return CitizenLetterForTask{}, fmt.Errorf("%w: DENUNCIATION kèm thông tin đơn", ErrCitizenLetterAnswerInvalid)
		}
		return CitizenLetterForTask{Eligibility: CitizenLetterDenunciation}, nil

	case documentsv1.CitizenLetterTaskEligibility_CITIZEN_LETTER_TASK_ELIGIBILITY_ELIGIBLE:
		if letter == nil {
			return CitizenLetterForTask{}, fmt.Errorf("%w: ELIGIBLE thiếu thông tin đơn", ErrCitizenLetterAnswerInvalid)
		}
		if letter.GetLetterId() != asked {
			// The caller writes this id into nguon_id: an id it did not ask about is refused, not stored.
			return CitizenLetterForTask{}, fmt.Errorf("%w: ELIGIBLE cho một mã đơn KHÔNG ĐƯỢC HỎI", ErrCitizenLetterAnswerInvalid)
		}
		out := CitizenLetterForTask{
			Eligibility:   CitizenLetterEligible,
			LetterID:      letter.GetLetterId(),
			Number:        letter.GetNumber(),
			Year:          letter.GetYear(),
			HoldingUnitID: letter.GetHoldingUnitId(),
			AssigneeCode:  letter.GetAssigneeCode(),
		}
		if ts := letter.GetCurrentDueAt(); ts != nil {
			if err := ts.CheckValid(); err != nil {
				return CitizenLetterForTask{}, fmt.Errorf("%w: hạn của đơn không hợp lệ: %w", ErrCitizenLetterAnswerInvalid, err)
			}
			out.CurrentDueAt = ts.AsTime().UTC()
		}
		return out, nil

	default:
		// UNSPECIFIED, or a value newer than this client (a status rule added later): REFUSE — never
		// treated as ELIGIBLE (the enum's own comment).
		return CitizenLetterForTask{}, fmt.Errorf("%w: eligibility %v", ErrCitizenLetterAnswerInvalid, resp.GetEligibility())
	}
}
