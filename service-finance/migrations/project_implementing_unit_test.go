package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0016 (prototype columns for giai-ngan — owner instruction
// 07/10/2026). Same caveat as 0015's test: this proves the SQL is WRITTEN, not that PostgreSQL
// accepts it.

const file0016 = "0016_project_implementing_unit_and_issue_owner.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: a NOT NULL / defaulted column (rewrites archival rows),
// losing the blank/length CHECK, dropping any of 0015's guard branches while replacing the function,
// leaving the new columns editable after insert.
func TestMigration0016Shape(t *testing.T) {
	sql := maChay(t, file0016)
	for _, c := range []struct{ want, why string }{
		{"alter table du_an add column if not exists implementing_unit text;", "cột tự do, để trống được"},
		{"check (implementing_unit is null or (btrim(implementing_unit) <> '' and char_length(implementing_unit) <= 255))",
			"không chuỗi trắng, tối đa 255 như prototype"},
		{"alter table project_issues add column if not exists owner_code text;", "mã cán bộ theo dõi"},
		{"alter table project_issues add column if not exists due_on date;", "hạn xử lý vướng mắc"},
		{"check (owner_code is null or btrim(owner_code) <> '')", "mã cán bộ không trắng"},
		{"where conrelid = 'du_an'::regclass", "ràng buộc dò theo bảng cha"},
		{"where conrelid = 'project_issues'::regclass", "ràng buộc dò theo bảng cha"},
		{"create or replace function project_issue_guard()", "thay hàm canh cùng tên"},
		{"or new.owner_code is distinct from old.owner_code", "người theo dõi chỉ đặt lúc ghi"},
		{"or new.due_on is distinct from old.due_on", "hạn chỉ đặt lúc ghi"},
		// 0015's protections, kept verbatim.
		{"or new.recorded_at is distinct from old.recorded_at", "không sửa thời điểm ghi"},
		{"old.resolved_at is not null", "đã gỡ thì không mở lại"},
		{"old.tracking_task_id is not null", "liên kết nhiệm vụ đặt một lần"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0016 thiếu %q — %s", c.want, c.why)
		}
	}
}

// Every 0015 condition in the guard must survive the replacement: compare the IF block of both files.
func TestMigration0016GuardKeeps0015(t *testing.T) {
	old := maChay(t, file0015)
	neu := maChay(t, file0016)
	for _, line := range []string{
		"new.tenant_id is distinct from old.tenant_id",
		"or new.project_id is distinct from old.project_id",
		"or new.title is distinct from old.title",
		"or new.description is distinct from old.description",
		"or new.recorded_by is distinct from old.recorded_by",
		"or new.recorded_at is distinct from old.recorded_at",
		"and (new.resolved_at is distinct from old.resolved_at or new.resolved_by is distinct from old.resolved_by)",
		"and new.tracking_task_id is distinct from old.tracking_task_id",
	} {
		if !strings.Contains(old, line) {
			t.Fatalf("0015 không còn %q — kiểm tra này đã mù", line)
		}
		if !strings.Contains(neu, line) {
			t.Errorf("0016 bỏ mất điều kiện canh của 0015: %q", line)
		}
	}
}

// Only ADDS: no existing row, column, constraint or key is touched, no status/reopen sneaks in.
func TestMigration0016DestroysNothing(t *testing.T) {
	sql := maChay(t, file0016)
	for _, banned := range []string{"drop column", "drop table", "drop constraint", "alter column",
		"delete from", "truncate", "insert into", "update du_an", "update project_issues",
		"default", "text not null", "date not null", "primary key", "partition", "status", "drop trigger"} {
		if strings.Contains(sql, banned) {
			t.Errorf("0016 chứa %q", banned)
		}
	}
}
