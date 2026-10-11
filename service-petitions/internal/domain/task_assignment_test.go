package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// Tests for the assignment rules (owner decision 28/09/2026).
//
//	PROVED HERE   unit or assignee changed with a reason -> `chuyen-tiep`, without -> unchanged
//	              (owner rule 11/10/2026) · Apply touches the two holder columns and
//	              NEITHER deadline · terminal tasks refused · empty body, empty unit, over-long value
//	              refused · staff codes trimmed · the timeline sentence names only what moved.
//	              Since ADR 0065 NV5 there is no lead unit / monitor on the change at all — the type
//	              cannot carry them, so no test is needed to prove they are not written.
//
//	NOT PROVED    the transaction and the identity check — internal/app/task_assignment_test.go.

func ptr(s string) *string { return &s }

func assignedTask() NhiemVu {
	due := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	return NhiemVu{
		Ma: "NV19", TrangThai: DangThucHien,
		BoPhanID: "bp-a", NguoiThucHienMa: "CB-00311",
		HanXuLy: due, HanBanDau: due,
	}
}

// ĐỔI CHIỀU CÓ CHỦ Ý 11/10/2026: was TestAssignmentHolderChangeResetsStatus, which pinned every holder
// change to `moi-giao` (28/09 decision). Owner rule "chuẩn là prototype", bug sheet row 17: a holder
// change WITH a reason is the prototype's transfer and lands in `chuyen-tiep`; WITHOUT one it is the
// prototype's assign and keeps the status.
func TestAssignmentHolderChangeStatus(t *testing.T) {
	for name, change := range map[string]TaskAssignmentChange{
		"unit":           {Unit: ptr("bp-b")},
		"assignee":       {Assignee: ptr("CB-00999")},
		"clear assignee": {Assignee: ptr("")},
	} {
		t.Run(name, func(t *testing.T) {
			before := assignedTask()
			after := change.Apply(before)
			if got := HandoverStatus(before, after, true); got != ChuyenTiep {
				t.Errorf("chuyển việc có lý do: trạng thái sau = %q, muốn chuyen-tiep", got)
			}
			if got := HandoverStatus(before, after, false); got != before.TrangThai {
				t.Errorf("giao việc không lý do: trạng thái sau = %q, muốn giữ %q", got, before.TrangThai)
			}
		})
	}
	// Nothing moved: a reason alone forwards nothing.
	before := assignedTask()
	if got := HandoverStatus(before, before, true); got != before.TrangThai {
		t.Errorf("không đổi người giữ mà vẫn sang %q", got)
	}
	// From a state the lifecycle map has no TRANSFERRED edge from — the prototype's transfer_task sets it
	// regardless (service.py:508).
	for _, s := range []TrangThaiNhiemVu{TamDung, ChoDuyet, ChuyenTiep} {
		b := assignedTask()
		b.TrangThai = s
		a := TaskAssignmentChange{Unit: ptr("bp-b")}.Apply(b)
		if got := HandoverStatus(b, a, true); got != ChuyenTiep {
			t.Errorf("%s chuyển việc: %q, muốn chuyen-tiep", s, got)
		}
	}
}

func TestAssignmentApplyNeverTouchesDeadline(t *testing.T) {
	before := assignedTask()
	after := TaskAssignmentChange{Unit: ptr("bp-b"), Assignee: ptr("CB-00999")}.Apply(before)
	if !after.HanXuLy.Equal(before.HanXuLy) || !after.HanBanDau.Equal(before.HanBanDau) {
		t.Errorf("hạn đã đổi: %v/%v -> %v/%v", before.HanXuLy, before.HanBanDau, after.HanXuLy, after.HanBanDau)
	}
	if after.BoPhanID != "bp-b" || after.NguoiThucHienMa != "CB-00999" {
		t.Errorf("Apply không ghi đúng hai cột: %+v", after)
	}
}

func TestAssignmentSameValuesIsNoChange(t *testing.T) {
	before := assignedTask()
	after := TaskAssignmentChange{Unit: ptr("bp-a"), Assignee: ptr("CB-00311")}.Apply(before)
	if AssignmentChanged(before, after) {
		t.Error("gửi đúng giá trị đang có mà bị coi là thay đổi — lần bấm thứ hai sẽ ghi thêm một vết")
	}
}

// ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026: a legacy `chuyen-tiep` row was refused here as terminal. It is no
// longer terminal (require 52ec9b5, user decision) and forwarding IS this act, so it may be handed
// over; only `hoan-thanh` is refused — reopening is its own act, gated by `task.approve`.
func TestAssignmentTerminalRefused(t *testing.T) {
	n := assignedTask()
	n.TrangThai = HoanThanh
	if err := CheckAssignable(n); !errors.Is(err, ErrTaskClosedForAssignment) {
		t.Errorf("hoan-thanh: lỗi = %v, muốn ErrTaskClosedForAssignment", err)
	}
	for _, s := range []TrangThaiNhiemVu{MoiGiao, DaTiepNhanNV, DangThucHien, ChoDuyet, TamDung, ChuyenTiep} {
		n := assignedTask()
		n.TrangThai = s
		if err := CheckAssignable(n); err != nil {
			t.Errorf("%s bị từ chối giao lại: %v", s, err)
		}
	}
}

func TestCheckTaskAssignmentRefusals(t *testing.T) {
	if _, err := CheckTaskAssignment(TaskAssignmentChange{}); !errors.Is(err, ErrAssignmentEmpty) {
		t.Errorf("thân rỗng: lỗi = %v", err)
	}
	if _, err := CheckTaskAssignment(TaskAssignmentChange{Unit: ptr("   ")}); !errors.Is(err, ErrAssignmentUnitEmpty) {
		t.Errorf("unit trống: lỗi = %v", err)
	}
	long := strings.Repeat("a", BoPhanToiDa+1)
	if _, err := CheckTaskAssignment(TaskAssignmentChange{Assignee: &long}); !errors.Is(err, ErrAssignmentFieldTooLong) {
		t.Errorf("quá dài: lỗi = %v", err)
	}
	for _, e := range []error{ErrAssignmentEmpty, ErrAssignmentUnitEmpty, ErrAssignmentFieldTooLong} {
		if !LaLoiDauVaoNhiemVu(e) {
			t.Errorf("%v phải là lỗi đầu vào (400)", e)
		}
	}
	for _, e := range []error{ErrAssignmentNoChange, ErrTaskClosedForAssignment} {
		if LaLoiDauVaoNhiemVu(e) {
			t.Errorf("%v là lỗi về trạng thái bản ghi (409), không phải lỗi đầu vào", e)
		}
	}
}

func TestCheckTaskAssignmentTrimsStaff(t *testing.T) {
	c, err := CheckTaskAssignment(TaskAssignmentChange{Assignee: ptr(" CB-00999 ")})
	if err != nil {
		t.Fatal(err)
	}
	if *c.Assignee != "CB-00999" {
		t.Errorf("assignee = %q, muốn đã cắt khoảng trắng", *c.Assignee)
	}
	if got := c.StaffCodes(); len(got) != 1 || got[0] != "CB-00999" {
		t.Errorf("StaffCodes = %v, muốn [CB-00999]", got)
	}
	if got := (TaskAssignmentChange{Assignee: ptr(""), Unit: ptr("bp-b")}).StaffCodes(); len(got) != 0 {
		t.Errorf("xoá người thực hiện mà vẫn hỏi identity: %v", got)
	}
}

// ĐỔI CHIỀU CÓ CHỦ Ý 11/10/2026: the status half of the sentence used to read `dang-thuc-hien →
// moi-giao`; a forwarding now reads `→ chuyen-tiep` and opens with "Chuyển việc", a plain assignment
// opens with "Giao lại" and names no status.
func TestAssignmentLogTextNamesOnlyWhatMoved(t *testing.T) {
	before := assignedTask()
	after := TaskAssignmentChange{Assignee: ptr("CB-00999")}.Apply(before)
	after.TrangThai = HandoverStatus(before, after, true)
	line := AssignmentLogText(before, after)
	for _, want := range []string{"Chuyển việc: ", "người thực hiện CB-00311 → CB-00999", "dang-thuc-hien → chuyen-tiep"} {
		if !strings.Contains(line, want) {
			t.Errorf("dòng nhật ký %q thiếu %q", line, want)
		}
	}
	if strings.Contains(line, "bộ phận bp-a") || strings.Contains(line, "chuyên viên") {
		t.Errorf("dòng nhật ký nêu cả trường không đổi: %q", line)
	}

	plain := TaskAssignmentChange{Assignee: ptr("CB-00999")}.Apply(before)
	plain.TrangThai = HandoverStatus(before, plain, false)
	line = AssignmentLogText(before, plain)
	if !strings.HasPrefix(line, "Giao lại: ") || strings.Contains(line, "Chuyển trạng thái") {
		t.Errorf("giao việc không lý do: %q, muốn \"Giao lại: …\" không nêu trạng thái", line)
	}
}
