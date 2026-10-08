package app

// The assignment act on a task — POST /api/v1/tasks/{ma}/assignment, `task.assign`.
//
// OWNER DECISIONS 28/09/2026, and this file implements them rather than restating them (the rules
// themselves are in domain/task_assignment.go):
//
//	"Chuyển tiếp" = the SAME row handed to another unit or person — one act with "đổi bộ phận /
//	người thực hiện" (PHAN_CHUA_DUNG #1).
//	Unit or assignee changed -> `moi-giao`. The deadline is never touched.
//
// ONE ROLE SINCE ADR 0065 NV5 (user decision 30/09/2026): the lead unit IS the unit and the monitoring
// officer IS the assignee. Changing "chuyên viên theo dõi" is changing the assignee, through this act;
// the retired columns are neither read nor written.
//
// # ONE TRANSACTION, THREE WRITES (rule 6, invariant 3)
//
// The UPDATE, the timeline row naming who handed it from whom to whom, and the audit entry. Written
// here and only here: the store's Reassign takes the transaction, and core/audit.Write takes it too,
// so there is no signature that could commit the hand-over without its trail.
//
// # WHAT IS CHECKED WITH IDENTITY, AND WHAT CANNOT BE
//
//	staff code (assignee)             identity.ResolveAssignableStaff, in the commune the context
//	                                  carries — BEFORE the transaction, for the reason
//	                                  kiemLanhDaoGiaoViec gives. A code of another commune, unknown,
//	                                  locked or account-less is ONE refusal (rule 1: no existence leak).
//	unit id (unit)                    identity.ResolveLiveOrgUnits (since 28/09/2026), also BEFORE the
//	                                  transaction — checkLiveOrgUnits (task_org_units.go). Unknown,
//	                                  removed or another commune's id is ONE refusal (400); identity
//	                                  down is 503 and nothing is written. Task creation checks the same
//	                                  column. (The PETITION assignment, domain.KiemPhanCong, still
//	                                  does not — outside this card.)
//
// # NOTIFYING THE NEW HOLDER — NOT DONE, ON PURPOSE
//
// The task register notifies nobody today: creating a task writes no outbox row (TaoTuNguon), and the
// only outbox in this service is the CITIZEN petition channel. Inventing a staff notification here
// would be a new mechanism nobody decided. Reported instead.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// ActionTaskAssignment is the verb in the trail. The VALUE is Vietnamese snake_case like every other
// action this service writes (an inspection reads it; ADR 0011); `phan_cong` is the ubiquitous
// language's code for "Phân công".
const ActionTaskAssignment = "phan_cong_nhiem_vu"

// ErrAssignmentStaffInvalid refuses a staff code identity did not answer as assignable in this commune.
//
// ONE ERROR FOR FIVE REASONS — unknown, deleted, no account, locked, another commune — for the reason
// ErrCanBoKhongNhanDuocViec is one. The code is not echoed.
var ErrAssignmentStaffInvalid = errors.New(
	"nhiem_vu: cán bộ được chọn không nhận được việc trong xã này")

// ErrAssignmentStaffUnchecked means identity could not be asked (or the check is not wired), so the
// task was NOT reassigned. Never "valid", never "invalid". Retryable — the handler answers 503.
var ErrAssignmentStaffUnchecked = errors.New("nhiem_vu: chưa kiểm được cán bộ nhận việc")

// TaskAssignmentRequest is the act as the handler hands it down. Change carries the two optional
// fields (nil = not mentioned); Note is the officer's optional line for the timeline.
type TaskAssignmentRequest struct {
	Change domain.TaskAssignmentChange
	Note   string
}

// Reassign hands the task to another unit and/or assignee. Permission:
// `task.assign`, declared on the route.
func (uc *GhiNhiemVu) Reassign(ctx context.Context, ma string, req TaskAssignmentRequest,
	nguoi audit.Actor) (domain.NhiemVu, error) {

	change, err := domain.CheckTaskAssignment(req.Change)
	if err != nil {
		return domain.NhiemVu{}, err
	}
	note, err := domain.KiemVanBanTuyChon(req.Note, domain.NoiDungNhatKyToiDa,
		domain.ErrNoiDungNhatKyQuaDai)
	if err != nil {
		return domain.NhiemVu{}, err
	}
	if err := coCanBoThucHien(nguoi); err != nil {
		return domain.NhiemVu{}, err
	}
	// EVERY non-empty staff code SENT is checked, not only the ones that differ from the row: which ones
	// differ is known only under the lock, and a gRPC call inside the transaction would hold a
	// government register's row lock for a network round trip. A client sends only what it changes.
	if err := uc.checkAssignableStaff(ctx, change.StaffCodes()); err != nil {
		return domain.NhiemVu{}, err
	}
	// The unit id SENT, for the same reason and at the same moment (task_org_units.go).
	var units []string
	if change.Unit != nil {
		units = append(units, *change.Unit)
	}
	if err := uc.checkLiveOrgUnits(ctx, units...); err != nil {
		return domain.NhiemVu{}, err
	}

	now := uc.nayHoac()
	var after domain.NhiemVu

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.kho.TheoMaDeSua(ctx, tx, ma)
		if err != nil {
			return err
		}
		if err := domain.CheckAssignable(before); err != nil {
			return err
		}
		after = change.Apply(before)
		if !domain.AssignmentChanged(before, after) {
			// NOTHING MOVED — no UPDATE, no timeline row, no audit entry. What the route's
			// idem.KhongCan declaration rests on.
			return domain.ErrAssignmentNoChange
		}
		after.TrangThai = domain.ResetStatus(before, after)

		if err := uc.kho.Reassign(ctx, tx, before.ID, before.TrangThai, after); err != nil {
			return err
		}
		if err := uc.attachReplyTreeFacts(ctx, tx, &after); err != nil {
			return err
		}
		if err := uc.refreshUpdatedAt(ctx, tx, &after); err != nil {
			return err
		}

		// THE TIMELINE ROW carries the NEW holder in its own columns (ghiNhatKy copies them from the
		// task) and the from→to sentence in its text — §5.9's "thông tin bộ phận/phụ trách khi có thay
		// đổi phân công". The officer's note, if any, follows on its own line; it lives here and NOT in
		// the audit entry (free text may name a citizen's case, and audit_log is permanent).
		line := domain.AssignmentLogText(before, after)
		if note != "" {
			line += "\n" + note
		}
		if err := uc.ghiNhatKy(ctx, tx, after, now, nguoi.ID, line); err != nil {
			return err
		}

		// BEFORE AND AFTER OF BOTH HOLDER COLUMNS AND THE STATUS. Unit ids and STAFF business codes only —
		// no citizen personal data, nothing to mask (rule 3 is about the people a commune serves; staff
		// data inside one commune is not masked, open question #11). The deadline is recorded as
		// UNCHANGED evidence, not as a field that moved: both values are the same by construction.
		delta, err := json.Marshal(map[string]any{
			"truoc":              assignmentAuditFields(before),
			"sau":                assignmentAuditFields(after),
			"dat_lai_trang_thai": after.TrangThai != before.TrangThai,
			"han_xu_ly":          lucRaVet(after.HanXuLy),
			"do_dai_ghi_chu":     len([]rune(note)),
		})
		if err != nil {
			return fmt.Errorf("nhiem_vu: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  ActionTaskAssignment,
			Subject: before.Ma,
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.NhiemVu{}, bocNhiemVu(ctx, "giao lại nhiệm vụ", err)
	}
	return after, nil
}

// assignmentAuditFields is one side of the delta. ONE function for both sides, so "before" and "after"
// cannot name different keys.
func assignmentAuditFields(n domain.NhiemVu) map[string]any {
	return map[string]any{
		"bo_phan_id":         n.BoPhanID,
		"nguoi_thuc_hien_ma": n.NguoiThucHienMa,
		"trang_thai":         string(n.TrangThai),
	}
}

// checkAssignableStaff asks identity about the staff codes the act would write. An empty list asks
// nothing (clearing the assignee, or a change of units only).
//
// FAIL CLOSED: no checker wired refuses, it never writes a code unchecked.
func (uc *GhiNhiemVu) checkAssignableStaff(ctx context.Context, codes []string) error {
	if len(codes) == 0 {
		return nil
	}
	if uc.giaoViec == nil {
		return fmt.Errorf("%w: chưa nối dây kiểm cán bộ", ErrAssignmentStaffUnchecked)
	}
	accepted, err := uc.giaoViec.CanBoGiaoViecDuoc(ctx, codes)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrAssignmentStaffUnchecked, err)
	}
	for _, code := range codes {
		if _, ok := accepted[code]; !ok {
			return ErrAssignmentStaffInvalid
		}
	}
	return nil
}

// CheckAssignableStaff is checkAssignableStaff for another door into the create path — the citizen-letter
// door (citizen_letter_task.go), whose assignee may be the letter's own and so was typed by nobody in
// this act. The SAME checker and the SAME refusals as Reassign, so one officer code answers one way.
func (uc *GhiNhiemVu) CheckAssignableStaff(ctx context.Context, codes []string) error {
	return uc.checkAssignableStaff(ctx, codes)
}
