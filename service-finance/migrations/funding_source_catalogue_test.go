package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0013 (funding source catalogue + per-year granted amount, user
// decision 06/10/2026).
//
// WHAT THIS IS AND IS NOT — the same caveat as period_close_test.go. It reads the SQL this binary
// embeds and asserts the refusal, the keys and the trigger are WRITTEN. It does NOT prove PostgreSQL
// accepts the file; internal/store/nguon_von_pg_test.go does that once VIGOV_TEST_DSN is set. This
// half always runs, so a loosened key or a removed refusal goes red on every machine.

const file0013 = "0013_funding_source_catalogue.sql"

// TestMigration0013RefusesPopulatedSources — the line that keeps 0013 from dropping a granted amount.
//
// THE MUTATION THAT MUST TURN THIS RED: deleting the count-and-raise block, or adding a deleted_at
// predicate to the count (a soft-deleted source still holds a year and an amount).
func TestMigration0013RefusesPopulatedSources(t *testing.T) {
	sql := maChay(t, file0013)

	for _, c := range []struct{ want, why string }{
		{"select count(*), count(distinct tenant_id) into n_rows, n_communes from nguon_von;",
			"đếm CẢ dòng đã xoá mềm trước khi bỏ cột"},
		{"if n_rows > 0 then raise exception", "có dòng thì từ chối, không bỏ cột âm thầm"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0013 thiếu %q — %s", c.want, c.why)
		}
	}
	// The refusal must come BEFORE the first column is dropped, or it guards nothing.
	if i, j := strings.Index(sql, "if n_rows > 0"), strings.Index(sql, "drop column"); i < 0 || j < 0 || i > j {
		t.Error("0013: khối từ chối phải đứng TRƯỚC câu bỏ cột đầu tiên")
	}
	if strings.Contains(sql, "from nguon_von where deleted_at") {
		t.Error("0013: phép đếm không được bỏ qua dòng đã xoá mềm")
	}
}

// TestMigration0013UniqueKeys — the three keys the user decided, each composite with tenant_id and
// none partial (tools/check_khoa_duy_nhat.py enforces the same, repository-wide).
func TestMigration0013UniqueKeys(t *testing.T) {
	sql := maChay(t, file0013)

	for _, c := range []struct{ want, why string }{
		{"create unique index if not exists nguon_von_name_unique on nguon_von (tenant_id, ten);",
			"quyết định 3(a): tên nguồn duy nhất trong một xã"},
		{"create unique index if not exists phan_bo_nguon_von_one_line_per_source on phan_bo_nguon_von " +
			"(tenant_id, du_an_id, nguon_von_id);",
			"quyết định 3(b): một dự án một dòng cho mỗi nguồn"},
		{"unique (tenant_id, funding_source_id, year),", "quyết định 2: một số vốn mỗi nguồn mỗi năm"},
		{"primary key (tenant_id, id),", "khoá chính hợp thành với tenant_id"},
		{"add constraint nguon_von_name_trimmed check (ten = btrim(ten));",
			"khoảng trắng thừa không tạo được bản sao vô hình của một tên"},
		{"constraint funding_source_annual_amounts_amount_not_negative check (granted_amount >= 0)",
			"vốn được giao không âm"},
		{"constraint funding_source_annual_amounts_year_valid check (year between 2000 and 2100)",
			"cùng khung năm với du_an.nam"},
		{") partition by hash (tenant_id);", "phân vùng theo xã"},
		{"before delete on funding_source_annual_amounts for each row execute function " +
			"ho_so_luu_tru_cam_xoa_cung();", "không xoá cứng số vốn được giao"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0013 thiếu %q — %s", c.want, c.why)
		}
	}
	for _, partial := range []string{
		"on nguon_von (tenant_id, ten) where",
		"(tenant_id, du_an_id, nguon_von_id) where",
	} {
		if strings.Contains(sql, partial) {
			t.Errorf("0013: khoá duy nhất không được từng phần (%q)", partial)
		}
	}
}
