package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0034 (fifth task source `don-thu`, ADR 0085 A5). It reads the SQL this
// binary embeds; PostgreSQL behaviour needs VIGOV_TEST_DSN and is not proved here.

const file0034 = "0034_task_source_citizen_letter.sql"

// TestMigration0034OnlyWidensTheSourceCheck — THE MUTATIONS THAT MUST TURN THIS RED: dropping one of 0006's
// four codes (rows holding it would fail validation, and the reversal note would lie); forgetting
// `don-thu`; renaming the constraint (0006's would then survive beside it and keep refusing `don-thu`);
// anything that touches rows.
func TestMigration0034OnlyWidensTheSourceCheck(t *testing.T) {
	old := danhSachMaTrongCheck(t, maChay(t, tep0006), "nhiem_vu_nguon_giao_hop_le", "nguon_giao")
	sql := maChay(t, file0034)
	now := danhSachMaTrongCheck(t, sql, "nhiem_vu_nguon_giao_hop_le", "nguon_giao")

	have := map[string]bool{}
	for _, c := range now {
		have[c] = true
	}
	for _, c := range old {
		if !have[c] {
			t.Errorf("0034 bỏ mất nguồn %q của 0006 — dòng đang giữ mã ấy sẽ làm ADD CONSTRAINT đổ", c)
		}
	}
	if !have["don-thu"] {
		t.Error("0034 không thêm `don-thu`")
	}
	if len(now) != len(old)+1 {
		t.Errorf("0034 có %d mã, muốn đúng %d (bốn của 0006 cộng một)", len(now), len(old)+1)
	}
	if !strings.Contains(sql, "alter table nhiem_vu drop constraint if exists nhiem_vu_nguon_giao_hop_le;") {
		t.Error("0034 phải gỡ CHECK cũ cùng tên trên bảng cha (bảng phân mảnh: lan xuống mọi phân mảnh)")
	}
	for _, banned := range []string{"delete from", "update ", "drop table", "drop column", "insert into"} {
		if strings.Contains(sql, banned) {
			t.Errorf("0034 chứa %q — nới CHECK không được chạm dòng hay cột nào (luật 7)", banned)
		}
	}
}
