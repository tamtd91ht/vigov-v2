package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0014 (auto-issued project codes, user decision 06/10/2026). Same
// caveat as period_close_test.go: this proves the SQL is WRITTEN, not that PostgreSQL accepts it —
// internal/store/project_code_counter_pg_test.go does that when VIGOV_TEST_DSN is set.

const file0014 = "0014_project_code_counter.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: a key without tenant_id (one series shared by every
// commune — commune B's DA07 would skip because commune A issued it), partitioning it (ADR 0096),
// dropping the hard-delete trigger.
func TestMigration0014CounterShape(t *testing.T) {
	sql := maChay(t, file0014)
	requirePlainTable(t, file0014, sql, "project_code_counters")
	for _, c := range []struct{ want, why string }{
		{"create table if not exists project_code_counters (", "bảng bộ đếm"},
		{"primary key (tenant_id),", "một dãy mỗi xã — khoá theo tenant_id"},
		{"constraint project_code_counters_next_number_positive check (next_number >= 1)", "số bắt đầu từ 1"},
		{"create trigger project_code_counters_no_hard_delete before delete on project_code_counters " +
			"for each row execute function ho_so_luu_tru_cam_xoa_cung();", "không xoá cứng bộ đếm"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0014 thiếu %q — %s", c.want, c.why)
		}
	}
}

// Only ADDS: no existing row, column or constraint is touched.
func TestMigration0014DestroysNothing(t *testing.T) {
	sql := maChay(t, file0014)
	for _, banned := range []string{"drop column", "drop table", "drop constraint", "alter column",
		"delete from", "truncate", "insert into", "update du_an"} {
		if strings.Contains(sql, banned) {
			t.Errorf("0014 chứa %q", banned)
		}
	}
}
