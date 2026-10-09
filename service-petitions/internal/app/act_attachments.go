package app

// FILES ON A STATUS ACT (owner decisions of 09/10/2026, batch A): classification, assignment, the
// status step, closure, rejection and referral may carry `attachments` — log attachments the SAME
// officer uploaded for the SAME petition and completed beforehand (petition_log_attachment.go), exactly
// what the manual note already takes (GhiChuNoiBo). OPTIONAL and ADDITIVE: no id, no file read, the act
// as before.
//
// THE FILES RIDE THE ACT'S OWN TIMELINE ROW, IN THE ACT'S TRANSACTION. Migration 0027's
// `petition_log_attachment_check` refuses a link to an entry another transaction wrote, to another
// petition's file, to another author's file, to anything that is not a `petition-log-attachment`, and to
// a file not yet stored. The checks below are that trigger's sentence before its 500
// (domain.CheckPetitionLogAttachable); the trigger stays the floor.
//
// THE CLOSE GATE IS UNTOUCHED: it counts `petition-verification-photo` rows, and a log attachment is a
// different purpose — refused here if offered, never counted there. Dong runs the gate BEFORE the files
// are read, so a closing refused for a missing photo reads no file and writes nothing.

import (
	"context"
	"errors"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// errActFilesNotWired: files were offered and the file store is not wired. A wiring fault (500), never
// a reason to drop the files and record the act alone.
var errActFilesNotWired = errors.New("xu_ly_phan_anh: chưa nối kho tệp đính kèm")

// checkActAttachmentList refuses an empty id, a duplicate or a missing wiring BEFORE any transaction.
func (uc *XuLyPhanAnh) checkActAttachmentList(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if err := domain.CheckAttachmentList(ids); err != nil {
		return err
	}
	if uc.staffFiles == nil {
		return errActFilesNotWired
	}
	return nil
}

// readActAttachments reads the offered files FOR UPDATE inside the act's transaction and checks each,
// BEFORE the act writes anything — so a wrong id refuses the whole act and writes nothing. `author` is
// the acting officer's business code, the one the timeline row will carry.
func (uc *XuLyPhanAnh) readActAttachments(ctx context.Context, tx *store.ScopedTx, petitionID, author string,
	ids []string) error {

	if len(ids) == 0 {
		return nil
	}
	cands, err := uc.staffFiles.PetitionAttachCandidates(ctx, tx, ids)
	if err != nil {
		return err
	}
	for _, id := range ids {
		c, ok := cands[id]
		if !ok {
			return domain.ErrPetitionAttachmentNotUsable
		}
		if err := domain.CheckPetitionLogAttachable(c, petitionID, author); err != nil {
			return err
		}
	}
	return nil
}

// linkActAttachments links the files to the timeline row the act has just written.
func (uc *XuLyPhanAnh) linkActAttachments(ctx context.Context, tx *store.ScopedTx, logID string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return uc.staffFiles.LinkToPetitionLogEntry(ctx, tx, logID, ids)
}

// withAttachmentIDs adds the file IDS to an act's audit delta — never their names (a name can describe
// a case, rule 3). Absent when the act carried none, so the entries of every act before this batch and
// every act without files keep their shape.
func withAttachmentIDs(delta map[string]any, ids []string) map[string]any {
	if len(ids) > 0 {
		delta["tep_dinh_kem"] = ids
	}
	return delta
}
