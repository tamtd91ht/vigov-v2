package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0016 (`nhiem_vu_bat_bien` replaced so `han_ban_dau` may follow a
// deadline correction before any approved extension). Same standing as task_issued_code_test.go: the
// PostgreSQL behaviour is proved by internal/store/task_deadline_correction_pg_test.go, which SKIPS
// without VIGOV_TEST_DSN; this runs always.

const tep0016 = "0016_task_deadline_correction.sql"

// TestMigration0016GuardKeepsEveryOtherRefusal — THE MUTATIONS THAT MUST TURN THIS RED: dropping 0015's
// `ma` condition while editing the deadline branch; dropping the hard-delete refusal; letting
// `han_ban_dau` take a value of its own; filtering the approved count on `deleted_at` (a soft-deleted
// approval would re-open the denominator); counting any status other than `da-duyet`.
func TestMigration0016GuardKeepsEveryOtherRefusal(t *testing.T) {
	sql := maChay(t, tep0016)
	for _, c := range []struct{ can, vi string }{
		{"create or replace function nhiem_vu_bat_bien()", "hàm canh phải được thay tại chỗ, cùng tên"},
		{"if tg_op = 'delete' then raise exception 'administrative record %: hard delete refused'",
			"mất chặn xoá cứng nhiệm vụ"},
		{"where c.tenant_id = new.tenant_id and c.code = new.ma and c.task_id = new.id",
			"mất điều kiện đổi mã của 0015"},
		{"where c.tenant_id = old.tenant_id and c.code = old.ma", "mất điều kiện giữ mã cũ của 0015"},
		{"if new.han_ban_dau is distinct from new.han_xu_ly or exists (select 1 from de_nghi_lui_han d " +
			"where d.tenant_id = new.tenant_id and d.nhiem_vu_id = new.id and d.trang_thai = 'da-duyet') then raise exception",
			"hạn ban đầu chỉ được đi theo hạn xử lý, và chỉ khi chưa có gia hạn nào được duyệt"},
	} {
		if !strings.Contains(sql, c.can) {
			t.Errorf("0016 thiếu %q — %s", c.can, c.vi)
		}
	}
	for _, c := range []struct{ cam, vi string }{
		{"d.deleted_at", "đếm gia hạn đã duyệt không được lọc xoá mềm"},
		{"create table", "tệp này không thêm bảng"},
		{"alter table", "tệp này không sửa bảng"},
		{"drop ", "tệp này không xoá gì"},
	} {
		if strings.Contains(sql, c.cam) {
			t.Errorf("0016 chứa %q — %s", c.cam, c.vi)
		}
	}
}
