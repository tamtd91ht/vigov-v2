package domain

import (
	"strings"
	"testing"
)

// LaKhoaQuyenPhang guards the `permission` filter of GET /api/v1/staff-directory. Every seeded key
// must pass (a validator refusing a real key makes that filter answer 400 for it forever), and every
// shape that is not ONE flat "<nhóm>.<việc>" must not.
func TestLaKhoaQuyenPhang(t *testing.T) {
	// Real keys from migrations 0001 and 0007, the longest included.
	for _, k := range []string{"task.extend", "task.approve", "admin.user", "feedback.restricted", "feedback.unmask"} {
		if !LaKhoaQuyenPhang(k) {
			t.Errorf("%q là khoá thật mà bị từ chối", k)
		}
	}
	for _, k := range []string{
		"", "task", ".extend", "task.", "task.extend.x", "a..b",
		"TASK.EXTEND", "Task.extend", "task.extend ", " task.extend", "task,extend",
		"*", "task.*", "1task.extend", "_task.extend", "task;extend", "nhiệm.vụ",
		strings.Repeat("a", TranKhoaQuyen) + ".b",
	} {
		if LaKhoaQuyenPhang(k) {
			t.Errorf("%q không phải khoá phẳng mà được nhận", k)
		}
	}
}
