package domain

import (
	"errors"
	"testing"
)

// TestTaskWorkRightFor pins vigov-require a37ec96's table as mapped onto this service's columns.
func TestTaskWorkRightFor(t *testing.T) {
	n := NhiemVu{
		NguoiThucHienMa: "CB-00311", ChuyenVienTheoDoiMa: "CB-00412",
		LanhDaoGiaoViecMa: "CB-00007", NguoiTaoMa: "CB-00123",
	}
	for _, c := range []struct {
		name   string
		code   string
		update bool
		want   TaskWorkRight
	}{
		{"assignee", "CB-00311", false, TaskWorkFull},
		{"task.update holder", "CB-09999", true, TaskWorkFull},
		{"monitor", "CB-00412", false, TaskWorkLog},
		{"assigner", "CB-00007", false, TaskWorkLog},
		{"author", "CB-00123", false, TaskWorkLog},
		{"outsider", "CB-09999", false, TaskWorkNone},
		{"empty code", "", false, TaskWorkNone},
	} {
		if got := TaskWorkRightFor(n, c.code, c.update); got != c.want {
			t.Errorf("%s: %v, muốn %v", c.name, got, c.want)
		}
	}
	// An UNASSIGNED task (empty columns) makes nobody a participant — not even an empty code.
	if got := TaskWorkRightFor(NhiemVu{}, "", false); got != TaskWorkNone {
		t.Errorf("nhiệm vụ rỗng, mã rỗng: %v", got)
	}
}

func TestCheckMayWriteLogEntry(t *testing.T) {
	if err := CheckMayWriteLogEntry(TaskWorkNone); !errors.Is(err, ErrNotTaskParticipant) {
		t.Errorf("none: %v", err)
	}
	for _, r := range []TaskWorkRight{TaskWorkLog, TaskWorkFull} {
		if err := CheckMayWriteLogEntry(r); err != nil {
			t.Errorf("%v: %v", r, err)
		}
	}
}
