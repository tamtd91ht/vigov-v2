package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0015 (`task_issued_code` and the relaxed `nhiem_vu_bat_bien`).
//
// Same standing as nhat_ky_phan_anh_test.go: it reads the SQL this binary embeds and asserts the key,
// the backfill, the triggers and the relaxed guard are WRITTEN. It does not prove PostgreSQL enforces
// them — internal/store/task_issued_code_pg_test.go does, and SKIPS without VIGOV_TEST_DSN. It exists
// so that a removed trigger or a narrowed backfill goes red somewhere that always runs.

const tep0015 = "0015_task_issued_code.sql"

// TestMigration0015LedgerKeyTriggersAndBackfill — THE MUTATIONS THAT MUST TURN THIS RED: a key without
// tenant_id; a `deleted_at` filter on the backfill (a soft-deleted task's code would become free);
// removing the append-only trigger, the per-leaf TRUNCATE guard, or the insert registration.
func TestMigration0015LedgerKeyTriggersAndBackfill(t *testing.T) {
	sql := maChay(t, tep0015)

	for _, c := range []struct{ can, vi string }{
		{"primary key (tenant_id, code)", "khoá sổ mã đã cấp phải ghép với tenant_id (luật 1 bất biến 6)"},
		{") partition by hash (tenant_id);", "bảng phải phân mảnh hash theo tenant_id (ADR 0010)"},
		{"partition of task_issued_code ' 'for values with (modulus 32, remainder %s)",
			"thiếu vòng tạo 32 mảnh — bảng phân mảnh không mảnh từ chối mọi INSERT"},
		{"insert into task_issued_code (tenant_id, code, task_id, issued_at) select tenant_id, ma, id, tao_luc from nhiem_vu on conflict (tenant_id, code) do nothing;",
			"backfill phải lấy MỌI dòng nhiem_vu, kể cả đã xoá mềm, và chạy lại được"},
		{"before update or delete on task_issued_code for each row execute function task_issued_code_append_only()",
			"sổ mã đã cấp phải chỉ-thêm"},
		{"before truncate on %s ' 'for each statement execute function task_issued_code_append_only()",
			"thiếu chặn TRUNCATE từng mảnh"},
		{"after insert on nhiem_vu for each row execute function task_register_issued_code()",
			"nhiệm vụ mới phải tự vào sổ mã đã cấp — mọi đường ghi, không chỉ service"},
		{"on task_issued_code (tenant_id, task_id, issued_at)", "chỉ mục tra ngược theo nhiệm vụ, bắt đầu bằng tenant_id"},
	} {
		if !strings.Contains(sql, c.can) {
			t.Errorf("0015 thiếu %q — %s", c.can, c.vi)
		}
	}
}

// TestMigration0015GuardStillRefusesWhatItRefused — the replaced `nhiem_vu_bat_bien` keeps 0006's two
// unconditional refusals and makes only the `ma` one conditional on the ledger. THE MUTATIONS THAT
// MUST TURN THIS RED: dropping the `han_ban_dau` refusal while "only touching `ma`"; dropping the hard
// delete refusal; relaxing `ma` without binding the new code to THIS task.
func TestMigration0015GuardStillRefusesWhatItRefused(t *testing.T) {
	sql := maChay(t, tep0015)
	for _, c := range []struct{ can, vi string }{
		{"create or replace function nhiem_vu_bat_bien()", "hàm canh của 0006 phải được thay tại chỗ, cùng tên"},
		{"if tg_op = 'delete' then raise exception 'administrative record %: hard delete refused'",
			"mất chặn xoá cứng nhiệm vụ"},
		{"if new.han_ban_dau is distinct from old.han_ban_dau then raise exception",
			"mất chặn đổi hạn ban đầu (§11.3)"},
		{"where c.tenant_id = new.tenant_id and c.code = new.ma and c.task_id = new.id",
			"mã mới phải được cấp cho CHÍNH nhiệm vụ này trước khi đổi"},
		{"where c.tenant_id = old.tenant_id and c.code = old.ma",
			"mã cũ phải nằm trong sổ, để còn giữ chỗ sau khi đổi"},
	} {
		if !strings.Contains(sql, c.can) {
			t.Errorf("0015 thiếu %q — %s", c.can, c.vi)
		}
	}
	for _, c := range []struct{ cam, vi string }{
		{"deleted_at timestamptz", "sổ mã đã cấp không có trạng thái 'đã ẩn'"},
		{"deleted_at is null", "backfill không được lọc xoá mềm — mã của việc đã xoá vẫn là mã đã cấp"},
		{"drop table", "xoá bảng"},
		{"drop column", "xoá cột"},
		{"alter table", "sửa một bảng có sẵn — tệp này chỉ thêm bảng, trigger và thay hàm"},
	} {
		if strings.Contains(sql, c.cam) {
			t.Errorf("0015 chứa %q — %s", c.cam, c.vi)
		}
	}
}
