package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// Tests for the assignment rules (owner decision 28/09/2026).
//
//	PROVED HERE   unit or assignee changed -> `moi-giao`; lead unit / monitor alone -> status kept ·
//	              Apply touches the four holder columns and NEITHER deadline · terminal tasks refused ·
//	              empty body, empty unit, over-long value refused · staff codes deduplicated and
//	              trimmed · the timeline sentence names only what moved.
//
//	NOT PROVED    the transaction and the identity check — internal/app/task_assignment_test.go.

func ptr(s string) *string { return &s }

func assignedTask() NhiemVu {
	due := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	return NhiemVu{
		Ma: "NV19", TrangThai: DangThucHien,
		BoPhanID: "bp-a", NguoiThucHienMa: "CB-00311",
		CoQuanChuTriID: "bp-lead", ChuyenVienTheoDoiMa: "CB-00412",
		HanXuLy: due, HanBanDau: due,
	}
}

func TestAssignmentHolderChangeResetsStatus(t *testing.T) {
	for name, change := range map[string]TaskAssignmentChange{
		"unit":           {Unit: ptr("bp-b")},
		"assignee":       {Assignee: ptr("CB-00999")},
		"clear assignee": {Assignee: ptr("")},
	} {
		t.Run(name, func(t *testing.T) {
			before := assignedTask()
			after := change.Apply(before)
			if got := ResetStatus(before, after); got != MoiGiao {
				t.Errorf("trạng thái sau = %q, muốn moi-giao — người nhận mới phải tiếp nhận lại", got)
			}
		})
	}
}

func TestAssignmentLeadAndMonitorKeepStatus(t *testing.T) {
	before := assignedTask()
	after := TaskAssignmentChange{LeadUnit: ptr("bp-other"), Monitor: ptr("CB-00500")}.Apply(before)
	if !AssignmentChanged(before, after) {
		t.Fatal("đổi cơ quan chủ trì + chuyên viên mà không nhận là có thay đổi")
	}
	if HolderChanged(before, after) {
		t.Error("đổi cơ quan chủ trì / chuyên viên bị coi là đổi người giữ việc")
	}
	if got := ResetStatus(before, after); got != DangThucHien {
		t.Errorf("trạng thái sau = %q, muốn giữ dang-thuc-hien (chủ quyết #7)", got)
	}
}

func TestAssignmentApplyNeverTouchesDeadline(t *testing.T) {
	before := assignedTask()
	after := TaskAssignmentChange{Unit: ptr("bp-b"), Assignee: ptr("CB-00999"),
		LeadUnit: ptr(""), Monitor: ptr("")}.Apply(before)
	if !after.HanXuLy.Equal(before.HanXuLy) || !after.HanBanDau.Equal(before.HanBanDau) {
		t.Errorf("hạn đã đổi: %v/%v -> %v/%v", before.HanXuLy, before.HanBanDau, after.HanXuLy, after.HanBanDau)
	}
	if after.BoPhanID != "bp-b" || after.NguoiThucHienMa != "CB-00999" ||
		after.CoQuanChuTriID != "" || after.ChuyenVienTheoDoiMa != "" {
		t.Errorf("Apply không ghi đúng bốn cột: %+v", after)
	}
}

func TestAssignmentSameValuesIsNoChange(t *testing.T) {
	before := assignedTask()
	after := TaskAssignmentChange{Unit: ptr("bp-a"), Monitor: ptr("CB-00412")}.Apply(before)
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
	if _, err := CheckTaskAssignment(TaskAssignmentChange{Monitor: &long}); !errors.Is(err, ErrAssignmentFieldTooLong) {
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

func TestCheckTaskAssignmentTrimsAndDedupesStaff(t *testing.T) {
	c, err := CheckTaskAssignment(TaskAssignmentChange{Assignee: ptr(" CB-00999 "), Monitor: ptr("CB-00999")})
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

func TestAssignmentLogTextNamesOnlyWhatMoved(t *testing.T) {
	before := assignedTask()
	after := TaskAssignmentChange{Assignee: ptr("CB-00999")}.Apply(before)
	after.TrangThai = ResetStatus(before, after)
	line := AssignmentLogText(before, after)
	for _, want := range []string{"người thực hiện CB-00311 → CB-00999", "dang-thuc-hien → moi-giao"} {
		if !strings.Contains(line, want) {
			t.Errorf("dòng nhật ký %q thiếu %q", line, want)
		}
	}
	if strings.Contains(line, "bộ phận bp-a") || strings.Contains(line, "chuyên viên") {
		t.Errorf("dòng nhật ký nêu cả trường không đổi: %q", line)
	}
}
