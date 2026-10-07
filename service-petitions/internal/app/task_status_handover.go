package app

// The optional parts of a status move — POST /api/v1/tasks/{ma}/status with `handover` and/or
// `attachments` — the prototype's "Cập nhật và giao việc" (vigov-require TaskStatusPipeline.tsx,
// service.py report_progress_with_files). USER DECISION 07/10/2026, for THIS combined path only:
//
//	the status   lands in the status the officer CHOSE, never `moi-giao` (domain.ResetStatus is NOT
//	             called here; the standalone assignment act still calls it)
//	who          a holder of `task.assign` OR the current assignee (domain.CheckMayHandOver)
//
// # ONE TRANSACTION (rule 6, invariant 3; rule 2, invariant 6)
//
// Lock the row → every check → the handover (UPDATE of the holder columns, its timeline row, its audit
// entry `phan_cong_nhiem_vu`) → the status UPDATE → the status timeline row → the file links → the
// status audit entry `chuyen_trang_thai_nhiem_vu`. Any failure rolls back all of it.
//
// # TWO AUDIT ENTRIES, NOT ONE COMBINED ACTION — and why
//
// Each act keeps the verb an inspection already searches for: "every handover of this task" is a
// query on `phan_cong_nhiem_vu`, "every status move" one on `chuyen_trang_thai_nhiem_vu`. A third verb
// for the combination would be missed by both queries. The two entries point at each other through
// `kem_chuyen_trang_thai` (the status the move landed in) and `kem_giao_lai`.
//
// # WHICH HOLDER THE STATUS RULE IS CHECKED AGAINST — the one BEFORE the handover
//
// The prototype reports progress AFTER assigning, but there the status is optional; here it is
// mandatory, and checking the holder after the handover would refuse exactly the caller this path was
// decided for — the current assignee passing the work on, who stops being the holder the moment the
// handover applies. So both rules read the row AS LOCKED: the status move needs the holder or
// `task.update` (unchanged), the handover needs `task.assign` or being that same holder. Consequence,
// stated: a `task.assign` holder WITHOUT `task.update` who is not the assignee cannot use the combined
// path (403 `ErrStatusNeedsHolder`); the standalone assignment act stays open to them.
//
// # WHAT IS NOT DONE, ON PURPOSE
//
//	pending extension request   NOTHING, exactly as Reassign: a request filed by the previous holder
//	                            stays `cho-duyet`, decidable as before (ledger giao-lai-nhiem-vu item 6
//	                            is undecided)
//	notifying the new holder    nothing, for the reason task_assignment.go gives
//	evidence before cho-duyet   the prototype's "≥1 file before pending approval" is NOT adopted — not
//	                            decided here
//	a handover changing nothing NOT refused (Reassign answers 409 `no_change`): the drawer sends the
//	                            handover block pre-filled, and the prototype records a handover only when
//	                            a holder moved. Nothing of the handover is written; the status move goes on

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// TaskAssignRight is ONE fact asked at the edge: does this account hold `task.assign`? A named type for
// the reason TaskUpdateRight is one.
type TaskAssignRight bool

// statusExtras is the validated optional part of one status move. The zero value is "none".
type statusExtras struct {
	handover    *domain.TaskAssignmentChange
	note        string
	assign      TaskAssignRight
	attachments []string
}

// prepareStatusExtras validates what can be validated without the row, and asks identity — BEFORE the
// transaction, for the reason Reassign gives (no gRPC call while a register row is locked).
//
// ⚠ THE AUTHORITY CHECK IS NOT HERE: who may hand over depends on the locked row. So a caller who will
// be refused 403 may first be answered 400/503 by the identity check — about a STAFF code or unit id of
// their own commune, which the assignable-staff list already shows them.
func (uc *GhiNhiemVu) prepareStatusExtras(ctx context.Context, yc YeuCauDoiTrangThai) (statusExtras, error) {
	var x statusExtras
	if yc.Handover != nil {
		change, err := domain.CheckTaskAssignment(yc.Handover.Change)
		if err != nil {
			return x, err
		}
		note, err := domain.KiemVanBanTuyChon(yc.Handover.Note, domain.NoiDungNhatKyToiDa,
			domain.ErrNoiDungNhatKyQuaDai)
		if err != nil {
			return x, err
		}
		x.handover, x.note, x.assign = &change, note, yc.AssignRight
	}
	if len(yc.Attachments) > 0 {
		if err := domain.CheckAttachmentList(yc.Attachments); err != nil {
			return x, err
		}
		if uc.files == nil {
			return x, errAttachmentsNotWired
		}
		x.attachments = yc.Attachments
	}
	if x.handover != nil {
		if err := uc.checkAssignableStaff(ctx, x.handover.StaffCodes()); err != nil {
			return x, err
		}
		var units []string
		if x.handover.Unit != nil {
			units = append(units, *x.handover.Unit)
		}
		if err := uc.checkLiveOrgUnits(ctx, units...); err != nil {
			return x, err
		}
	}
	return x, nil
}

// checkHandover is the handover's own two refusals, on the locked row: the authority (403), then a
// finished task (409, as Reassign — a handover would reopen it through a side door).
func (x statusExtras) checkHandover(before domain.NhiemVu, actor audit.Actor) error {
	if x.handover == nil {
		return nil
	}
	if err := domain.CheckMayHandOver(before, actor.ID, bool(x.assign)); err != nil {
		return err
	}
	return domain.CheckAssignable(before)
}

// checkStatusAttachments reads the candidate files under lock and refuses any that is not a completed
// upload of THIS officer for THIS task, not yet on an entry — domain.CheckAttachable, the same rule a
// manual log entry applies.
func (uc *GhiNhiemVu) checkStatusAttachments(ctx context.Context, tx *store.ScopedTx, x statusExtras,
	task domain.NhiemVu, actor audit.Actor) error {

	if len(x.attachments) == 0 {
		return nil
	}
	cands, err := uc.files.AttachCandidates(ctx, tx, x.attachments)
	if err != nil {
		return err
	}
	for _, fid := range x.attachments {
		c, ok := cands[fid]
		if !ok {
			return domain.ErrAttachmentNotUsable
		}
		if err := domain.CheckAttachable(c, task.ID, actor.ID); err != nil {
			return err
		}
	}
	return nil
}

// writeStatusHandover applies the handover: the holder UPDATE (status kept), the timeline row naming
// from→to, the `phan_cong_nhiem_vu` audit entry. Returns the row with the new holder, and whether a
// holder actually moved. No handover, or one that changes nothing, writes nothing and returns `before`.
func (uc *GhiNhiemVu) writeStatusHandover(ctx context.Context, tx *store.ScopedTx, x statusExtras,
	before domain.NhiemVu, target domain.TrangThaiNhiemVu, now time.Time,
	actor audit.Actor) (domain.NhiemVu, bool, error) {

	if x.handover == nil {
		return before, false, nil
	}
	after := x.handover.Apply(before)
	if !domain.AssignmentChanged(before, after) {
		return before, false, nil
	}
	// THE STATUS STAYS — user decision 07/10/2026. The status UPDATE that follows moves it to the target.
	after.TrangThai = before.TrangThai
	if err := uc.kho.Reassign(ctx, tx, before.ID, before.TrangThai, after); err != nil {
		return before, false, err
	}

	// The same row Reassign writes: the NEW holder in its columns, the from→to sentence, the officer's
	// handover note on its own line (the note lives here, never in the audit entry).
	line := domain.AssignmentLogText(before, after)
	if x.note != "" {
		line += "\n" + x.note
	}
	if err := uc.ghiNhatKy(ctx, tx, after, now, actor.ID, line); err != nil {
		return before, false, err
	}

	delta, err := json.Marshal(map[string]any{
		"truoc":              assignmentAuditFields(before),
		"sau":                assignmentAuditFields(after),
		"dat_lai_trang_thai": false,
		"han_xu_ly":          lucRaVet(after.HanXuLy),
		"do_dai_ghi_chu":     len([]rune(x.note)),
		// THE COMBINED PATH, named: the status this handover travelled with, and which door the caller
		// came through — an inspection asks "was this the assigner, or the holder passing it on".
		"kem_chuyen_trang_thai": string(target),
		"quyen_giao":            handoverRightCode(x.assign),
	})
	if err != nil {
		return before, false, fmt.Errorf("nhiem_vu: mã hoá delta: %w", err)
	}
	if err := audit.Write(ctx, tx, audit.Entry{
		Actor:   actor,
		Action:  ActionTaskAssignment,
		Subject: before.Ma,
		Delta:   delta,
	}); err != nil {
		return before, false, err
	}
	return after, true, nil
}

// handoverRightCode names the door in the trail: `giao-nhiem-vu` (holds `task.assign`) or
// `nguoi-thuc-hien` (the current assignee without it). A code, not a name.
func handoverRightCode(assign TaskAssignRight) string {
	if assign {
		return "giao-nhiem-vu"
	}
	return "nguoi-thuc-hien"
}

// linkStatusAttachments links the checked files to the status move's timeline row — in the
// transaction that wrote it, which is the only one migration 0021's trigger accepts.
func (uc *GhiNhiemVu) linkStatusAttachments(ctx context.Context, tx *store.ScopedTx, x statusExtras,
	logEntryID string) error {

	if len(x.attachments) == 0 {
		return nil
	}
	return uc.files.LinkToLogEntry(ctx, tx, logEntryID, x.attachments)
}

// statusExtrasAudit adds the optional parts to the status entry's delta — and NOTHING when neither was
// used, so the entry of a plain move is unchanged. File IDS only, never their names (rule 3).
func statusExtrasAudit(vet map[string]any, x statusExtras, handedOver bool, logEntryID string) {
	if handedOver {
		vet["kem_giao_lai"] = true
	}
	if len(x.attachments) > 0 {
		vet["nhat_ky_id"] = logEntryID
		vet["tep_dinh_kem"] = x.attachments
	}
}
