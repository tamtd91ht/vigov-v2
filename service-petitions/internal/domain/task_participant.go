package domain

// Who may work on ONE task — vigov-require a37ec96 ("the officer holding a task can work it"), read at
// anchor 0053854:apps/api/app/modules/tasks/service.py:542-599 (`work_rights`, `_assert_may_work`).
// User decision 28/09/2026: the open questions of the Nhiệm vụ menu are resolved from that repository.
//
// # A RULE FOR TASKS ONLY
//
// ADR 0038 forbids carrying the petition / document holder rule across registers, and this is NOT
// that rule: it is a37ec96's own table for tasks, with its own sets. The petition path keeps
// duocTienTrangThai; nothing here is shared with it.
//
// # THE TABLE, MAPPED ONTO THIS SERVICE'S COLUMNS
//
//	a37ec96                        here                                   right
//	has `task.update`              the caller holds `task.update`         full
//	task.assignee_id == user       nguoi_thuc_hien_ma == Principal.Ma     full
//	monitor_id                     chuyen_vien_theo_doi_ma                log
//	assigner_id                    lanh_dao_giao_viec_ma                  log
//	created_by                     nguoi_tao_ma                           log
//	collaborators                  — NO SUCH CONCEPT HERE (no table)      —
//	staff of org_unit / lead unit  — NOT IMPLEMENTED, see below           —
//	anybody else                                                          none
//
// "full" may change the status and write the log; "log" may only write the log; "none" touches
// nothing. BOTH ROUTES APPLY IT (user decision 28/09/2026): POST …/log-entries (CheckMayWriteLogEntry)
// and POST …/status (CheckMayChangeStatus), each gated by `task.read` exactly as a37ec96's router is. Require's reason, quoted from the docstring: related people "nói được 'tôi đã gửi công văn
// sang huyện', còn tuyên bố việc xong thì thuộc về người chịu trách nhiệm".
//
// ⚠ THE UNIT HALF IS MISSING, AND IT FAILS CLOSED. a37ec96 also lets any officer of the task's unit or
// lead unit write the log. authz.Principal carries no unit. identity now has ResolveStaffOrgUnits, but
// its contract says in so many words that its answer "narrows a LIST; it grants nothing, and the
// caller must never use it in a guard" (proto/vigov/identity/v1/identity.proto, on that RPC) — and
// "may write on this task" IS a guard. So an officer related ONLY through their unit is still refused
// here (403) where require would let them write, until the contract owner decides otherwise.
//
// THE CODES ARE STAFF BUSINESS CODES (`CB-…`), compared with Principal.Ma — the value every `…_ma`
// column holds (rule 6, invariant 8). AN EMPTY CODE MATCHES NOTHING: a task with nobody assigned must
// not make every caller with an empty code its holder.

import "errors"

// TaskWorkRight is what one caller may do on one task.
type TaskWorkRight int

const (
	TaskWorkNone TaskWorkRight = iota
	TaskWorkLog
	TaskWorkFull
)

// ErrNotTaskParticipant refuses a log entry from somebody who holds `task.read` but is neither the
// assignee, nor related to the task, nor a holder of the commune-wide `task.update`. 403.
var ErrNotTaskParticipant = errors.New(
	"nhiệm vụ: chỉ người thực hiện, chuyên viên theo dõi, lãnh đạo giao việc, người tạo nhiệm vụ " +
		"hoặc cán bộ có quyền cập nhật nhiệm vụ mới ghi được nhật ký của nhiệm vụ này")

// TaskWorkRightFor is a37ec96's `work_rights` over values already in hand. `communeWideUpdate` is the
// answer to "does this caller hold `task.update`", asked at the edge.
func TaskWorkRightFor(n NhiemVu, staffCode string, communeWideUpdate bool) TaskWorkRight {
	if communeWideUpdate {
		return TaskWorkFull
	}
	if staffCode == "" {
		return TaskWorkNone
	}
	if n.NguoiThucHienMa == staffCode {
		return TaskWorkFull
	}
	for _, related := range []string{n.ChuyenVienTheoDoiMa, n.LanhDaoGiaoViecMa, n.NguoiTaoMa} {
		if related == staffCode {
			return TaskWorkLog
		}
	}
	return TaskWorkNone
}

// ErrStatusNeedsHolder refuses a status move to anybody below TaskWorkFull — a related person, or an
// officer who only reads the register. a37ec96's `status_needs_holder`: "chỉ người thực hiện mới đổi
// được trạng thái nhiệm vụ". 403.
var ErrStatusNeedsHolder = errors.New(
	"nhiệm vụ: chỉ người thực hiện hoặc cán bộ có quyền cập nhật nhiệm vụ mới đổi được trạng thái")

// CheckMayChangeStatus is a37ec96's `_assert_may_work(changing_status=True)`: only TaskWorkFull moves
// the status. It decides WHO may move at all; which moves additionally need `task.approve` is
// NeedsApproval's, checked after it.
func CheckMayChangeStatus(r TaskWorkRight) error {
	if r != TaskWorkFull {
		return ErrStatusNeedsHolder
	}
	return nil
}

// CheckMayWriteLogEntry refuses a manual timeline entry to TaskWorkNone. Both other rights may write:
// the log is exactly what a37ec96 opens to related people.
func CheckMayWriteLogEntry(r TaskWorkRight) error {
	if r == TaskWorkNone {
		return ErrNotTaskParticipant
	}
	return nil
}
