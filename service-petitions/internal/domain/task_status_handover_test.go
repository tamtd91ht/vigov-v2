package domain

import (
	"errors"
	"testing"
)

// CheckMayHandOver — user decision 07/10/2026: `task.assign`, or the CURRENT assignee of the row.
func TestCheckMayHandOver(t *testing.T) {
	n := NhiemVu{NguoiThucHienMa: "CB-00311"}
	for _, c := range []struct {
		name   string
		task   NhiemVu
		code   string
		assign bool
		ok     bool
	}{
		{"có task.assign", n, "CB-09999", true, true},
		{"người đang thực hiện", n, "CB-00311", false, true},
		{"người khác", n, "CB-09999", false, false},
		// An empty session code never matches an empty assignee — fail closed.
		{"mã rỗng, việc chưa có người", NhiemVu{}, "", false, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := CheckMayHandOver(c.task, c.code, c.assign)
			if c.ok != (err == nil) {
				t.Fatalf("lỗi = %v, muốn được phép = %v", err, c.ok)
			}
			if !c.ok && !errors.Is(err, ErrHandoverNotAllowed) {
				t.Errorf("lỗi = %v, muốn ErrHandoverNotAllowed", err)
			}
		})
	}
}
