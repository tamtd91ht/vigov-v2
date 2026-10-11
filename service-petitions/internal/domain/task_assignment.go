package domain

// The rules of handing a task to another holder — "Chuyển tiếp" and "đổi bộ phận / người thực hiện",
// which the owner decided on 28/09/2026 are ONE act on the SAME row (PHAN_CHUA_DUNG #1).
//
// ONE ROLE, NOT TWO (ADR 0065 NV5, user decision 30/09/2026, migration 0025): "Cơ quan chủ trì tham
// mưu" IS the unit and "Chuyên viên theo dõi" IS the assignee. The retired columns
// (`co_quan_chu_tri_id`, `chuyen_vien_theo_doi_ma`) are never read and never written; changing the
// monitoring officer IS changing the assignee, through this act.
//
// THIS FILE IMPORTS THE STANDARD LIBRARY AND NOTHING ELSE (rule 4 of the service pattern). What needs
// identity (is this staff code assignable in this commune) and what needs the row lock (the current
// holder) lives in internal/app; what lives here is the verdict over values already in hand.
//
// # THE DECISIONS THIS FILE ENCODES — do not re-decide them here
//
//	holder changed WITH a reason        FORWARDING ("Chuyển việc"): the task lands in `chuyen-tiep`, the
//	(the `note` of the act)             branch status, and stays there until the new holder takes it up
//	                                    (HandoverStatus). OWNER RULE 11/10/2026, bug sheet row 17
//	holder changed WITHOUT a reason     ASSIGNMENT ("Giao việc"): the status is UNCHANGED (HandoverStatus)
//	the deadline                        UNTOUCHED. A deadline moves only through an approved extension
//	                                    (ADR 0038); nothing here reads or writes `han_*`
//	who                                 `task.assign`, declared on the route
//
// WHY "A REASON" IS THE LINE — it is the prototype's own line, not a choice made here. vigov-require's
// drawer block "Giao việc, chuyển việc" (TaskDetailDrawer.tsx:207-235) calls POST /transfer when the
// "Lý do chuyển" box is filled and POST /assign when it is empty; transfer_task sets TRANSFERRED
// (service.py:486-539), assign_task touches no status (service.py:438-483). Our one route carries the
// same box as `note` (web-admin task-assignment-block.tsx labels it "Lý do chuyển"), so a non-empty note
// IS the transfer.
//
// ĐỔI CHIỀU CÓ CHỦ Ý 11/10/2026: until then a holder change ALWAYS sent the task back to `moi-giao`
// (owner decision 28/09/2026, ResetStatus). The owner's rule of 11/10/2026 ("chuẩn là prototype — tính
// năng trên prototype phải được đảm bảo") replaces it. ADR 0076 #4c and the 28/09 note still say
// `moi-giao` until amended.
//
// # A HANDOVER CARRIED BY A STATUS MOVE (user decision 07/10/2026)
//
// POST /api/v1/tasks/{ma}/status may carry an optional `handover` (the prototype's "Cập nhật và giao
// việc", vigov-require service.py report_progress_with_files). On THAT path, and only there:
//
//	the status       is the one the officer CHOSE — `chuyen-tiep` included since 11/10/2026;
//	                 HandoverStatus is not called
//	who              a holder of `task.assign` OR the current assignee (CheckMayHandOver)
//
// # TWO ASSUMPTIONS STATED BY THE MAIN SESSION (28/09/2026), implemented and named as such
//
//	not a FINISHED task       `hoan-thanh` is signed-off work; handing it to somebody new would reopen it
//	                          through a side door (CheckAssignable). Reopening is its own act, gated by
//	                          `task.approve`. CHANGED 28/09/2026: a legacy `chuyen-tiep` row is no longer
//	                          terminal (require 52ec9b5) and MAY be handed over — forwarding is exactly
//	                          this act now
//
// # ONE ASSUMPTION OF THIS PASS, stated because nobody decided it
//
//	`unit` cannot be CLEARED  a unit sent must name a unit. Handing work to nobody is not a
//	                          hand-over, and the petition path's assignment refuses it the same way
//	                          (ErrThieuBoPhan). The assignee MAY be cleared: "— Để bộ phận phân công —"
//	                          is a real choice

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

var (
	// ErrAssignmentEmpty — the body named neither field. 400: there is nothing to do.
	ErrAssignmentEmpty = errors.New(
		"nhiệm vụ: phải gửi ít nhất một trong `unit`, `assignee`")

	// ErrAssignmentUnitEmpty — `unit` was sent as "" (see the assumption above).
	ErrAssignmentUnitEmpty = errors.New(
		"nhiệm vụ: `unit` không được để trống — giao lại phải nêu bộ phận nhận việc")

	// ErrAssignmentFieldTooLong bounds the two values. They are ids and business codes, not text; the
	// bound is for a value that is not one at all. The input is NOT echoed (rule 3, forbidden #3).
	ErrAssignmentFieldTooLong = fmt.Errorf(
		"nhiệm vụ: mã bộ phận hoặc mã cán bộ quá dài (tối đa %d ký tự)", BoPhanToiDa)

	// ErrAssignmentNoChange — every field sent equals what the task already holds. 409, not 400: the
	// body is well formed, and it is THIS record's state that makes it a no-op. It is also what the
	// route's `idem.KhongCan` rests on — a double click finds the first click already applied, writes
	// nothing and answers this.
	ErrAssignmentNoChange = errors.New(
		"nhiệm vụ: bộ phận và người thực hiện đã đúng như yêu cầu — không có gì để đổi")

	// ErrTaskClosedForAssignment — the task is `hoan-thanh` (first assumption above). 409.
	ErrTaskClosedForAssignment = errors.New(
		"nhiệm vụ: nhiệm vụ đã hoàn thành nên không giao lại được — mở lại nhiệm vụ trước nếu cần giao tiếp")

	// ErrHandoverNotAllowed — a status move carried a `handover`, and the caller neither holds
	// `task.assign` nor is the task's current assignee. 403: it is about the person, not the body.
	ErrHandoverNotAllowed = errors.New(
		"nhiệm vụ: chỉ cán bộ có quyền giao nhiệm vụ hoặc người đang thực hiện nhiệm vụ mới giao tiếp được việc")
)

// CheckMayHandOver is the rule for a handover carried by a status move (user decision 07/10/2026,
// prototype service.py:863-873): the commune-wide `task.assign`, or being the CURRENT assignee of THIS
// row. Decided on the row read under the lock, so a holder change committing concurrently cannot let
// the previous assignee hand the work on after it.
//
// WHY THE ASSIGNEE WITHOUT THE KEY: the specialist finishes their part and passes the work to the next
// person; refusing them means asking a leader to click on their behalf (the prototype's own reason).
func CheckMayHandOver(n NhiemVu, staffCode string, assignRight bool) error {
	if assignRight {
		return nil
	}
	if staffCode != "" && n.NguoiThucHienMa == staffCode {
		return nil
	}
	return ErrHandoverNotAllowed
}

// TaskAssignmentChange is the request after validation. A nil pointer is "not mentioned" and leaves
// the column alone; a pointer to "" clears it (never allowed for Unit — CheckTaskAssignment).
type TaskAssignmentChange struct {
	Unit     *string // bo_phan_id — identity's `bo_phan` id
	Assignee *string // nguoi_thuc_hien_ma — STAFF BUSINESS CODE
}

// CheckTaskAssignment trims and bounds what was sent. TRIMMED ONCE HERE, so the code identity is asked
// about is exactly the code the UPDATE writes.
func CheckTaskAssignment(c TaskAssignmentChange) (TaskAssignmentChange, error) {
	if c.Unit == nil && c.Assignee == nil {
		return c, ErrAssignmentEmpty
	}
	for _, field := range []**string{&c.Unit, &c.Assignee} {
		if *field == nil {
			continue
		}
		value := strings.TrimSpace(**field)
		if utf8.RuneCountInString(value) > BoPhanToiDa {
			return c, ErrAssignmentFieldTooLong
		}
		*field = &value
	}
	if c.Unit != nil && *c.Unit == "" {
		return c, ErrAssignmentUnitEmpty
	}
	return c, nil
}

// StaffCodes lists the non-empty staff codes the change would WRITE — the ones identity must accept
// before the transaction opens.
func (c TaskAssignmentChange) StaffCodes() []string {
	if c.Assignee == nil || *c.Assignee == "" {
		return nil
	}
	return []string{*c.Assignee}
}

// CheckAssignable refuses a finished task (the assumption in the header).
//
// IT NAMES `hoan-thanh` AND DOES NOT ASK "IS THERE A WAY OUT": since 28/09/2026 every status has one
// (hoan-thanh reopens), and the handover writes no `ngay_hoan_thanh`, so reassigning a finished task
// into `moi-giao` would also break the schema's biconditional.
func CheckAssignable(n NhiemVu) error {
	if n.TrangThai == HoanThanh {
		return ErrTaskClosedForAssignment
	}
	return nil
}

// Apply folds the change onto a task read under the lock. It touches the two holder columns and
// NOTHING ELSE — in particular neither deadline, so the reply and the audit delta cannot claim a
// deadline moved.
func (c TaskAssignmentChange) Apply(n NhiemVu) NhiemVu {
	if c.Unit != nil {
		n.BoPhanID = *c.Unit
	}
	if c.Assignee != nil {
		n.NguoiThucHienMa = *c.Assignee
	}
	return n
}

// HolderChanged reports whether the unit or the assignee differ between the two states.
func HolderChanged(before, after NhiemVu) bool {
	return before.BoPhanID != after.BoPhanID || before.NguoiThucHienMa != after.NguoiThucHienMa
}

// AssignmentChanged reports whether either holder column differs. Since ADR 0065 NV5 the holder IS
// the whole assignment, so this is HolderChanged; the name is kept for the act's "no change" check.
func AssignmentChanged(before, after NhiemVu) bool {
	return HolderChanged(before, after)
}

// HandoverStatus is the status the task lands in after the assignment act (owner rule 11/10/2026,
// header): `chuyen-tiep` when the holder changed AND the act carries a reason (a forwarding), the
// current status otherwise.
//
// FROM ANY STATUS THE ACT ACCEPTS — the prototype's transfer_task sets TRANSFERRED without consulting
// ALLOWED_TRANSITIONS, so a paused task or one at `cho-duyet` that is forwarded lands in `chuyen-tiep`
// too. `hoan-thanh` never reaches here (CheckAssignable refuses it first), which also keeps the schema's
// `ngay_hoan_thanh` biconditional intact. The way out of `chuyen-tiep` is the map's: the new holder
// takes it up into `da-tiep-nhan` or `dang-thuc-hien`.
func HandoverStatus(before, after NhiemVu, forwarding bool) TrangThaiNhiemVu {
	if forwarding && HolderChanged(before, after) {
		return ChuyenTiep
	}
	return before.TrangThai
}

// AssignmentLogText is the timeline sentence: who held it, who holds it now, by UNIT ID AND STAFF
// BUSINESS CODE only. Staff codes are not citizen personal data (rule 3 is about the people a commune
// serves), and names are not copied — the drawer resolves them, and a frozen copy would outlive a
// correction in the directory.
//
// ONLY THE COLUMNS THAT MOVED are named, so the line says what the act did. "—" stands for empty.
// Vietnamese, because an officer reads it on the timeline.
func AssignmentLogText(before, after NhiemVu) string {
	var parts []string
	for _, c := range []struct{ label, from, to string }{
		{"bộ phận", before.BoPhanID, after.BoPhanID},
		{"người thực hiện", before.NguoiThucHienMa, after.NguoiThucHienMa},
	} {
		if c.from == c.to {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %s → %s", c.label, dashIfEmpty(c.from), dashIfEmpty(c.to)))
	}
	// "Chuyển việc" when the act forwarded the task (it landed in `chuyen-tiep`), "Giao lại" otherwise —
	// the two words of the prototype's block, so the timeline says which act it was.
	verb := "Giao lại: "
	if after.TrangThai == ChuyenTiep && before.TrangThai != ChuyenTiep {
		verb = "Chuyển việc: "
	}
	line := verb + strings.Join(parts, "; ")
	if after.TrangThai != before.TrangThai {
		line += "; " + NoiDungChuyenTrangThai(before.TrangThai, after.TrangThai)
	}
	return line
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
