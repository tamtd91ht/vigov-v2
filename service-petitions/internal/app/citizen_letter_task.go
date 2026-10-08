package app

// Creating a task FROM a citizen letter — POST /api/v1/citizen-letter-tasks (ADR 0085 A; ADR 0084 #6;
// kb/30-indexes/transaction-boundaries.json `tao_nhiem_vu_tu_don_thu`).
//
// # PETITIONS ASKS, DOCUMENTS ANSWERS, PETITIONS WRITES
//
// The letter belongs to service-documents; the task belongs here. Before any transaction opens, this
// asks documents (ResolveCitizenLetterForTask) whether the letter exists in this commune, whether it is a
// denunciation, and what a task inherits from it. Nothing is written on the letter — the link is the
// task's `nguon_giao = 'don-thu'` + `nguon_id = letter id`, one fact, one owner — so there is ONE write,
// ONE transaction and nothing to compensate.
//
// # WHAT THE SERVER DERIVES, AND THE CLIENT CANNOT SEND
//
//	source pair   `don-thu` + the id documents echoed (never the body's)
//	deadline      the Vietnam date of the letter's current deadline at 17:00, or none (A6)
//	holder        the body's unit/assignee if it named either; else the letter's (§Trả lời #6)
//
// # THE ORDER OF REFUSALS, AND WHY NOTHING IS WRITTEN BY ANY OF THEM
//
//	documents not asked / failed / broke contract   ErrCitizenLetterUnchecked   (503)
//	unknown, deleted, another commune's             ErrCitizenLetterNotFound    (404)
//	denunciation                                    ErrCitizenLetterDenunciation (422)
//	nobody to hold it                               domain.ErrCitizenLetterAssignmentRequired (422)
//	assignee not assignable / not checked           ErrAssignmentStaffInvalid / …Unchecked (400 / 503)
//	unit not live / not checked                     inside CreateFromSource, BEFORE its transaction
//
// Every one of them happens before CreateFromSource opens its transaction.

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/documentsclient"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// CitizenLetterResolver asks the owner of the letter. *documentsclient.Client satisfies it as it is.
type CitizenLetterResolver interface {
	ResolveCitizenLetterForTask(ctx context.Context, letterID string) (documentsclient.CitizenLetterForTask, error)
}

// CitizenLetterTaskBooker is what this door borrows from the task register: the ONE create path, and the
// assignee check Reassign uses. *GhiNhiemVu satisfies it.
type CitizenLetterTaskBooker interface {
	CreateFromSource(ctx context.Context, yc YeuCauTaoNhiemVu, nguoi audit.Actor, steps SourceSteps) (domain.NhiemVu, error)
	CheckAssignableStaff(ctx context.Context, codes []string) error
}

// ErrCitizenLetterNotFound — documents answered NOT_FOUND: unknown, soft-deleted or another commune's
// letter, indistinguishable (rule 1). 404.
var ErrCitizenLetterNotFound = errors.New("nhiem_vu: không tìm thấy đơn thư trong xã này")

// ErrCitizenLetterDenunciation — a `to-cao` letter (C9, ADR 0084 #6): never a task, to keep the denouncer
// confidential (Luật Tố cáo 2018 Đ.8). 422. documents returned nothing about it, and nothing is logged.
var ErrCitizenLetterDenunciation = errors.New("nhiem_vu: đơn tố cáo không chuyển thành nhiệm vụ")

// ErrCitizenLetterUnchecked — documents could not be asked (down, older than the RPC, not wired) or its
// answer broke the contract. NEVER "not found", never "eligible": nothing was written. 503, retryable.
var ErrCitizenLetterUnchecked = errors.New("nhiem_vu: chưa hỏi được sổ đơn thư")

// CitizenLetterTaskCreation owns the act.
type CitizenLetterTaskCreation struct {
	letters CitizenLetterResolver
	tasks   CitizenLetterTaskBooker
}

// NewCitizenLetterTaskCreation: a nil `letters` is allowed and FAILS CLOSED — every call answers
// ErrCitizenLetterUnchecked, nothing is written. That is how the service runs until documents' address is
// wired (cmd/server).
func NewCitizenLetterTaskCreation(letters CitizenLetterResolver, tasks CitizenLetterTaskBooker) *CitizenLetterTaskCreation {
	return &CitizenLetterTaskCreation{letters: letters, tasks: tasks}
}

// CreateTask books one task whose source is citizen letter `letterID`. Route permissions: `task.create`
// AND `petition.read` (ADR 0085 A8). `yc` is the dialog as typed; its source pair and deadline are
// IGNORED and set here — the handler never fills them, this is the second wall.
func (uc *CitizenLetterTaskCreation) CreateTask(ctx context.Context, letterID string, yc YeuCauTaoNhiemVu,
	actor audit.Actor) (domain.NhiemVu, error) {

	if err := coCanBoThucHien(actor); err != nil {
		return domain.NhiemVu{}, err
	}
	if err := domain.CheckCitizenLetterID(letterID); err != nil {
		return domain.NhiemVu{}, err
	}
	if uc.letters == nil {
		return domain.NhiemVu{}, fmt.Errorf("%w: chưa nối dây tới documents", ErrCitizenLetterUnchecked)
	}
	letter, err := uc.letters.ResolveCitizenLetterForTask(ctx, letterID)
	if err != nil {
		return domain.NhiemVu{}, fmt.Errorf("%w: %w", ErrCitizenLetterUnchecked, err)
	}
	switch letter.Eligibility {
	case documentsclient.CitizenLetterEligible:
	case documentsclient.CitizenLetterNotFound:
		return domain.NhiemVu{}, ErrCitizenLetterNotFound
	case documentsclient.CitizenLetterDenunciation:
		return domain.NhiemVu{}, ErrCitizenLetterDenunciation
	default:
		// The client refuses every value it does not know; this is the second wall for a fake or a future
		// client that does not.
		return domain.NhiemVu{}, fmt.Errorf("%w: câu trả lời không rõ", ErrCitizenLetterUnchecked)
	}

	unit, assignee, err := domain.CitizenLetterTaskHolder(yc.BoPhanID, yc.NguoiThucHienMa,
		letter.HoldingUnitID, letter.AssigneeCode)
	if err != nil {
		return domain.NhiemVu{}, err
	}
	yc.BoPhanID, yc.NguoiThucHienMa = unit, assignee
	// THE ASSIGNEE IS CHECKED HERE, typed or inherited: documents does not validate the letter's holder
	// (documents.proto, CitizenLetterTaskSource), and a code of a locked account or of nobody would park
	// the task where no one opens it. The unit is checked by CreateFromSource, before its transaction.
	if assignee != "" {
		if err := uc.tasks.CheckAssignableStaff(ctx, []string{assignee}); err != nil {
			return domain.NhiemVu{}, err
		}
	}

	// SET HERE, UNCONDITIONALLY — from documents' answer, never from the body.
	yc.NguonGiao = string(domain.SourceCitizenLetter)
	yc.NguonID = letter.LetterID
	yc.HanXuLy = domain.CitizenLetterTaskDeadline(letter.CurrentDueAt)

	// The letter's BUSINESS number on the task's own entry — never the summary (it may quote the citizen,
	// and audit_log is permanent: rule 6, forbidden #4).
	return uc.tasks.CreateFromSource(ctx, yc, actor, SourceSteps{AuditExtra: map[string]any{
		AuditExtraCitizenLetter: map[string]any{"so_vao_so": letter.Number, "nam": letter.Year},
	}})
}
