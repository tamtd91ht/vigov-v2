package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0025 (the monitoring officer / lead unit pair merged into the
// assignee / assigned unit, ADR 0065 NV5). It reads the SQL this binary embeds; PostgreSQL behaviour
// needs VIGOV_TEST_DSN and is not proved here.

const file0025 = "0025_task_single_assignee_role.sql"

// TestMigration0025FillsOnlyEmptySurvivorsWithAudit — THE MUTATIONS THAT MUST TURN THIS RED: filling
// the RETIRED column instead of the survivor; overwriting a survivor that already holds a value
// (that is a reassignment, not a merge); an UPDATE whose audit entry is a separate statement (rule 6,
// invariant 3); an audit subject other than the business code; a `deleted_at` filter.
func TestMigration0025FillsOnlyEmptySurvivorsWithAudit(t *testing.T) {
	sql := maChay(t, file0025)
	for _, c := range []struct{ want, why string }{
		{"where nullif(btrim(nguoi_thuc_hien_ma), '') is null and nullif(btrim(chuyen_vien_theo_doi_ma), '') is not null",
			"chỉ điền người thực hiện đang trống, từ chuyên viên theo dõi đang có"},
		{"set nguoi_thuc_hien_ma = t.new_value, cap_nhat_luc = now()", "điền vào cột giữ lại, không phải cột nghỉ"},
		{"and nullif(btrim(n.nguoi_thuc_hien_ma), '') is null returning", "UPDATE phải kiểm lại cột trống dưới khoá dòng"},
		{"where nullif(btrim(bo_phan_id), '') is null and nullif(btrim(co_quan_chu_tri_id), '') is not null",
			"chỉ điền bộ phận đang trống, từ cơ quan chủ trì đang có"},
		{"set bo_phan_id = t.new_value, cap_nhat_luc = now()", "điền vào cột giữ lại, không phải cột nghỉ"},
		{"and nullif(btrim(n.bo_phan_id), '') is null returning", "UPDATE phải kiểm lại cột trống dưới khoá dòng"},
		{") insert into audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta) " +
			"select tenant_id, 'system', 'system', '', 'gop_vai_tro_nhiem_vu', ma, now(),",
			"vết ghi cùng câu lệnh với thay đổi, chủ thể hệ thống, subject là mã nghiệp vụ"},
		{"'truoc', jsonb_build_object('nguoi_thuc_hien_ma', old_value)", "vết phải giữ giá trị trước để đảo được"},
		{"'truoc', jsonb_build_object('bo_phan_id', old_value)", "vết phải giữ giá trị trước để đảo được"},
		{"'gop_vai_tro_nhiem_vu_lech'", "dòng hai người khác nhau phải được ghi vết"},
		{"and n.nguoi_thuc_hien_ma is distinct from n.chuyen_vien_theo_doi_ma and not exists",
			"vết lệch chỉ ghi một lần, chạy lại không nhân đôi"},
		{"comment on column nhiem_vu.chuyen_vien_theo_doi_ma is 'deprecated", "cột nghỉ phải được đánh dấu"},
		{"comment on column nhiem_vu.co_quan_chu_tri_id is 'deprecated", "cột nghỉ phải được đánh dấu"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0025 thiếu %q — %s", c.want, c.why)
		}
	}
	for _, c := range []struct{ banned, why string }{
		{"drop ", "không xoá cột, bảng hay trigger nào (luật 7 cấm #3)"},
		{"alter table", "không đổi hình dạng bảng"},
		{"delete from", "không xoá dòng nào"},
		{"set chuyen_vien_theo_doi_ma", "cột nghỉ không bị ghi"},
		{"set co_quan_chu_tri_id", "cột nghỉ không bị ghi"},
		{"trang_thai =", "gộp vai trò không phải giao việc — không đặt lại trạng thái"},
		{"deleted_at is null", "nhiệm vụ đã xoá mềm vẫn là hồ sơ, phải được gộp như nhau"},
		{"insert into nhat_ky_nhiem_vu", "gộp là quyết định lược đồ, không phải việc của cán bộ trên dòng thời gian"},
		{"set ma", "mã nhiệm vụ không đổi"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0025 chứa %q — %s", c.banned, c.why)
		}
	}
}

// TestMigration0025RunsAfterCodeFreeze — the audit subject is `ma`, and the reversal finds the row by
// it. That is sound only once `ma` can no longer change, i.e. after 0024.
func TestMigration0025RunsAfterCodeFreeze(t *testing.T) {
	if !(file0024 < file0025) {
		t.Fatalf("%s phải chạy sau %s: vết gộp tìm dòng theo mã, chỉ đúng khi mã đã khoá", file0025, file0024)
	}
}
