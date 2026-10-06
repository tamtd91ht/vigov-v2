package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0015 (§8.1 issues, §8.4 discussion — user decision 06/10/2026).
// Same caveat as period_close_test.go: this proves the SQL is WRITTEN, not that PostgreSQL accepts
// it — internal/store/project_discussion_pg_test.go does that when VIGOV_TEST_DSN is set.

const file0015 = "0015_project_issues_and_comments.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: a key without tenant_id, dropping the partitions, dropping
// the hard-delete triggers, dropping the immutability guards, losing the soft-delete columns.
func TestMigration0015Shape(t *testing.T) {
	sql := maChay(t, file0015)
	for _, c := range []struct{ want, why string }{
		{"create table if not exists project_issues (", "bảng vướng mắc"},
		{"create table if not exists project_comments (", "bảng trao đổi"},
		{"primary key (tenant_id, id),", "khoá hợp thành với tenant_id"},
		{") partition by hash (tenant_id);", "phân vùng theo xã"},
		{"for values with (modulus 32, remainder %s)", "32 phân vùng"},
		{"deleted_at timestamptz, deleted_by text, delete_reason text,", "cột xoá mềm (luật 7)"},
		{"tracking_task_id text,", "chỗ cho nhiệm vụ theo dõi về sau"},
		{"check ((resolved_at is null) = (resolved_by is null))", "gỡ thì phải có người gỡ"},
		{"create trigger project_issues_no_hard_delete before delete on project_issues " +
			"for each row execute function ho_so_luu_tru_cam_xoa_cung();", "không xoá cứng vướng mắc"},
		{"create trigger project_comments_no_hard_delete before delete on project_comments " +
			"for each row execute function ho_so_luu_tru_cam_xoa_cung();", "không xoá cứng trao đổi"},
		{"create trigger project_issues_guard before update on project_issues " +
			"for each row execute function project_issue_guard();", "không sửa vướng mắc đã ghi"},
		{"create trigger project_comments_guard before update on project_comments " +
			"for each row execute function project_comment_guard();", "không sửa trao đổi đã gửi"},
		{"old.resolved_at is not null", "đã gỡ thì không mở lại"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0015 thiếu %q — %s", c.want, c.why)
		}
	}
	if n := strings.Count(sql, "deleted_at timestamptz, deleted_by text, delete_reason text,"); n != 2 {
		t.Errorf("0015: %d bảng có đủ cột xoá mềm, muốn 2", n)
	}
}

// Only ADDS: no existing row, column or constraint is touched.
func TestMigration0015DestroysNothing(t *testing.T) {
	sql := maChay(t, file0015)
	for _, banned := range []string{"drop column", "drop table", "drop constraint", "alter column",
		"alter table", "delete from", "truncate", "insert into", "update du_an"} {
		if strings.Contains(sql, banned) {
			t.Errorf("0015 chứa %q", banned)
		}
	}
}
