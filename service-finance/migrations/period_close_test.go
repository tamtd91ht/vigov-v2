package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0012 (budget period close, user decision 30/09/2026).
//
// WHAT THIS IS AND IS NOT — the same caveat as dot_thu_chi_test.go. It reads the SQL this binary
// embeds and asserts the keys, CHECKs and triggers are WRITTEN. It does NOT prove PostgreSQL accepts
// the file — in particular the partial NULLS NOT DISTINCT unique index on a partitioned table, the
// first of its kind in this service. That is internal/store/budget_period_close_pg_test.go, which
// needs VIGOV_TEST_DSN. This half always runs, so a loosened key or a removed trigger goes red.

const file0012 = "0012_budget_period_close.sql"

// TestMigration0012OneActiveClose — the uniqueness that makes "one active close per period" a
// database fact, not a domain hope.
//
// THE MUTATIONS THAT MUST TURN THIS RED: drop NULLS NOT DISTINCT (two active year closes both pass,
// because month NULL is "distinct" from month NULL); drop tenant_id (the second commune cannot close
// its September); make the code key partial (an issued code is reissued after a reopen); drop the
// partial predicate (a period can never be closed again after a reopen).
func TestMigration0012OneActiveClose(t *testing.T) {
	sql := maChay(t, file0012)

	for _, c := range []struct{ want, why string }{
		{"create unique index if not exists budget_period_closes_one_active on budget_period_closes " +
			"(tenant_id, year, month) nulls not distinct where reopened_at is null;",
			"một lần chốt còn hiệu lực mỗi kỳ, kể cả kỳ cả năm (month NULL)"},
		{"create unique index if not exists budget_period_closes_revision_once on budget_period_closes " +
			"(tenant_id, year, month, revision) nulls not distinct;",
			"số lần chốt không cấp lại, tính cả lần đã mở lại"},
		{"primary key (tenant_id, id),", "khoá chính hợp thành với tenant_id"},
		{"unique (tenant_id, code),", "mã nghiệp vụ không cấp lại — KHÔNG từng phần"},
		{"current_setting('server_version_num')::int < 150000",
			"NULLS NOT DISTINCT cần PostgreSQL 15"},
		{") partition by hash (tenant_id);", "phân vùng theo xã"},
		{"for values with (modulus 32, remainder %s)", "32 phân vùng"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0012 thiếu %q — %s", c.want, c.why)
		}
	}
	if strings.Contains(sql, "unique (tenant_id, code) where") {
		t.Error("0012: khoá mã nghiệp vụ không được từng phần (luật 7 bất biến 3)")
	}
}

// TestMigration0012Checks — each CHECK that keeps a close or a reopen a whole fact.
func TestMigration0012Checks(t *testing.T) {
	sql := maChay(t, file0012)

	for _, c := range []struct{ want, why string }{
		{"constraint budget_period_closes_month_valid check (month between 1 and 12)",
			"tháng 1..12, NULL = cả năm"},
		{"constraint budget_period_closes_year_valid check (year between 2000 and 2100)",
			"cùng khung năm với bang_ngan_sach.nam"},
		{"constraint budget_period_closes_revision_positive check (revision >= 1)", "lần chốt từ 1"},
		{"constraint budget_period_closes_code_not_blank check (btrim(code) <> '')", "mã không rỗng"},
		{"constraint budget_period_closes_closed_by_not_blank check (btrim(closed_by) <> '')",
			"người chốt phải nêu được (luật 6 bất biến 8)"},
		{"constraint budget_period_closes_reopen_all_or_none check ( " +
			"(reopened_at is null and reopened_by is null and reopen_reason is null) " +
			"or (reopened_at is not null and reopened_by is not null and btrim(reopened_by) <> '' " +
			"and reopen_reason is not null and btrim(reopen_reason) <> ''))",
			"mở lại đủ ba hoặc không có gì, lý do không rỗng"},
		{"constraint budget_period_closes_reopen_reason_max check (char_length(reopen_reason) <= 500)",
			"lý do có giới hạn"},
		{"constraint budget_period_closes_reopen_after_close check (reopened_at >= closed_at)",
			"mở lại không trước lúc chốt"},
		{"alter table dot_thu_chi add column if not exists adjustment_reason text;",
			"đợt điều chỉnh mang lý do, cột mới cho phép NULL"},
		{"check (adjustment_reason is null or btrim(adjustment_reason) <> '')",
			"lý do điều chỉnh: NULL hoặc không rỗng"},
		{"check (char_length(adjustment_reason) <= 500)", "lý do điều chỉnh có giới hạn"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0012 thiếu %q — %s", c.want, c.why)
		}
	}

	for _, name := range []string{
		"dot_thu_chi_adjustment_reason_not_blank", "dot_thu_chi_adjustment_reason_max",
	} {
		guard := "where conrelid = 'dot_thu_chi'::regclass and conname = '" + name + "'"
		if !strings.Contains(sql, guard) {
			t.Errorf("0012: ràng buộc %s không có canh ghim vào bảng CHA", name)
		}
	}
}

// TestMigration0012History — a close is never hard deleted, and only the reopen trio may be filled,
// once. THE MUTATIONS: drop either trigger; subtract a fourth column from the row comparison (it
// becomes editable); drop the "stays reopened" branch (an un-reopen becomes possible).
func TestMigration0012History(t *testing.T) {
	sql := maChay(t, file0012)

	for _, c := range []struct{ want, why string }{
		{"create trigger budget_period_closes_no_hard_delete before delete on budget_period_closes " +
			"for each row execute function ho_so_luu_tru_cam_xoa_cung();", "không xoá cứng"},
		{"create trigger budget_period_closes_immutable before update on budget_period_closes " +
			"for each row execute function budget_period_close_immutable();", "không sửa lịch sử"},
		{"if (to_jsonb(new) - 'reopened_at' - 'reopened_by' - 'reopen_reason') is distinct from " +
			"(to_jsonb(old) - 'reopened_at' - 'reopened_by' - 'reopen_reason') then",
			"cả dòng trừ đúng ba cột mở lại phải giữ nguyên"},
		{"if old.reopened_at is not null and (new.reopened_at is distinct from old.reopened_at " +
			"or new.reopened_by is distinct from old.reopened_by " +
			"or new.reopen_reason is distinct from old.reopen_reason) then",
			"đã mở lại thì không đổi được nữa"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0012 thiếu %q — %s", c.want, c.why)
		}
	}
}

// TestMigration0012DestroysNothing — only ADDS.
func TestMigration0012DestroysNothing(t *testing.T) {
	sql := maChay(t, file0012)
	for _, c := range []struct{ banned, why string }{
		{"drop column", "xoá cột trên hồ sơ lưu trữ"},
		{"drop table", "xoá bảng"},
		{"drop constraint", "thay ràng buộc có sẵn"},
		{"alter column", "đổi kiểu hoặc NOT NULL của cột có sẵn"},
		{"delete from", "xoá cứng"},
		{"truncate", "xoá cứng"},
		{"update dot_thu_chi", "migration này không ghi dòng nào"},
		{"update budget_period_closes", "migration này không ghi dòng nào"},
		{"insert into", "migration này không ghi dòng nào"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0012 chứa %q — %s", c.banned, c.why)
		}
	}
}
