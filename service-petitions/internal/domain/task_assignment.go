package domain

// The rules of handing a task to another holder — "Chuyển tiếp" and "đổi bộ phận / người thực hiện",
// which the owner decided on 28/09/2026 are ONE act on the SAME row (PHAN_CHUA_DUNG #1), plus the two
// `theo-van-ban` fields of §5.4, "Cơ quan chủ trì tham mưu" and "Chuyên viên theo dõi" (#7).
//
// THIS FILE IMPORTS THE STANDARD LIBRARY AND NOTHING ELSE (rule 4 of the service pattern). What needs
// identity (is this staff code assignable in this commune) and what needs the row lock (the current
// holder) lives in internal/app; what lives here is the verdict over values already in hand.
//
// # THE FOUR DECISIONS THIS FILE ENCODES — do not re-decide them here
//
//	holder changed (unit or assignee)   status goes back to `moi-giao`: the new holder acknowledges
//	                                    the work again (HolderChanged, ResetStatus)
//	only lead unit / monitor changed    status unchanged — they follow the work, they do not hold it
//	the deadline                        UNTOUCHED. A deadline moves only through an approved extension
//	                                    (ADR 0038); nothing here reads or writes `han_*`
//	who                                 `task.assign`, declared on the route
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
//	                          is a real choice, and so may lead unit and monitor (both NULL on a
//	                          `co-ban` task)

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

var (
	// ErrAssignmentEmpty — the body named none of the four fields. 400: there is nothing to do.
	ErrAssignmentEmpty = errors.New(
		"nhiệm vụ: phải gửi ít nhất một trong `unit`, `assignee`, `lead_unit`, `monitor`")

	// ErrAssignmentUnitEmpty — `unit` was sent as "" (see the assumption above).
	ErrAssignmentUnitEmpty = errors.New(
		"nhiệm vụ: `unit` không được để trống — giao lại phải nêu bộ phận nhận việc")

	// ErrAssignmentFieldTooLong bounds the four values. They are ids and business codes, not text; the
	// bound is for a value that is not one at all. The input is NOT echoed (rule 3, forbidden #3).
	ErrAssignmentFieldTooLong = fmt.Errorf(
		"nhiệm vụ: mã bộ phận hoặc mã cán bộ quá dài (tối đa %d ký tự)", BoPhanToiDa)

	// ErrAssignmentNoChange — every field sent equals what the task already holds. 409, not 400: the
	// body is well formed, and it is THIS record's state that makes it a no-op. It is also what the
	// route's `idem.KhongCan` rests on — a double click finds the first click already applied, writes
	// nothing and answers this.
	ErrAssignmentNoChange = errors.New(
		"nhiệm vụ: bộ phận, người thực hiện, cơ quan chủ trì và chuyên viên theo dõi đã đúng như yêu cầu — không có gì để đổi")

	// ErrTaskClosedForAssignment — the task is `hoan-thanh` (first assumption above). 409.
	ErrTaskClosedForAssignment = errors.New(
		"nhiệm vụ: nhiệm vụ đã hoàn thành nên không giao lại được — mở lại nhiệm vụ trước nếu cần giao tiếp")
)

// TaskAssignmentChange is the request after validation. A nil pointer is "not mentioned" and leaves
// the column alone; a pointer to "" clears it (never allowed for Unit — CheckTaskAssignment).
type TaskAssignmentChange struct {
	Unit     *string // bo_phan_id — identity's `bo_phan` id
	Assignee *string // nguoi_thuc_hien_ma — STAFF BUSINESS CODE
	LeadUnit *string // co_quan_chu_tri_id — identity's `bo_phan` id
	Monitor  *string // chuyen_vien_theo_doi_ma — STAFF BUSINESS CODE
}

// CheckTaskAssignment trims and bounds what was sent. TRIMMED ONCE HERE, so the code identity is asked
// about is exactly the code the UPDATE writes.
func CheckTaskAssignment(c TaskAssignmentChange) (TaskAssignmentChange, error) {
	if c.Unit == nil && c.Assignee == nil && c.LeadUnit == nil && c.Monitor == nil {
		return c, ErrAssignmentEmpty
	}
	for _, field := range []**string{&c.Unit, &c.Assignee, &c.LeadUnit, &c.Monitor} {
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
// before the transaction opens. Deduplicated: one person as assignee and monitor is one question.
func (c TaskAssignmentChange) StaffCodes() []string {
	var codes []string
	for _, field := range []*string{c.Assignee, c.Monitor} {
		if field == nil || *field == "" {
			continue
		}
		if len(codes) == 1 && codes[0] == *field {
			continue
		}
		codes = append(codes, *field)
	}
	return codes
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

// Apply folds the change onto a task read under the lock. It touches the four holder columns and
// NOTHING ELSE — in particular neither deadline, so the reply and the audit delta cannot claim a
// deadline moved.
func (c TaskAssignmentChange) Apply(n NhiemVu) NhiemVu {
	if c.Unit != nil {
		n.BoPhanID = *c.Unit
	}
	if c.Assignee != nil {
		n.NguoiThucHienMa = *c.Assignee
	}
	if c.LeadUnit != nil {
		n.CoQuanChuTriID = *c.LeadUnit
	}
	if c.Monitor != nil {
		n.ChuyenVienTheoDoiMa = *c.Monitor
	}
	return n
}

// HolderChanged reports whether the unit or the assignee differ between the two states — the change
// that sends the task back to `moi-giao`. Lead unit and monitor are deliberately NOT here (owner #7).
func HolderChanged(before, after NhiemVu) bool {
	return before.BoPhanID != after.BoPhanID || before.NguoiThucHienMa != after.NguoiThucHienMa
}

// AssignmentChanged reports whether any of the four columns differ.
func AssignmentChanged(before, after NhiemVu) bool {
	return HolderChanged(before, after) ||
		before.CoQuanChuTriID != after.CoQuanChuTriID ||
		before.ChuyenVienTheoDoiMa != after.ChuyenVienTheoDoiMa
}

// ResetStatus is the status the task lands in after the act: `moi-giao` when the holder changed, the
// current status otherwise (owner decision 28/09/2026). A paused task handed to a new holder also
// lands in `moi-giao`: the decision names no exception, and the new holder has acknowledged nothing.
func ResetStatus(before, after NhiemVu) TrangThaiNhiemVu {
	if HolderChanged(before, after) {
		return MoiGiao
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
		{"cơ quan chủ trì", before.CoQuanChuTriID, after.CoQuanChuTriID},
		{"chuyên viên theo dõi", before.ChuyenVienTheoDoiMa, after.ChuyenVienTheoDoiMa},
	} {
		if c.from == c.to {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %s → %s", c.label, dashIfEmpty(c.from), dashIfEmpty(c.to)))
	}
	line := "Giao lại: " + strings.Join(parts, "; ")
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
