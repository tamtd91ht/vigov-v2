package migrations

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0011 (explicit operand columns of a budget % column, user decision
// 30/09/2026 "máy chủ tự tính").
//
// WHAT THIS IS AND IS NOT — the same caveat as dot_thu_chi_test.go. It reads the SQL this binary
// embeds and asserts the constraints and the backfill guards are WRITTEN. It does NOT prove
// PostgreSQL accepts the file, that the CHECKs recurse into the 32 partitions, or that the backfill
// resolves what it claims to — that is internal/store/percent_operands_pg_test.go, which needs
// VIGOV_TEST_DSN. This half always runs, so a loosened CHECK or a removed guard goes red somewhere.

const file0011 = "0011_budget_percent_operands.sql"

// TestMigration0011ColumnsAndChecks — the two columns, the four CHECKs, each guarded by a lookup
// pinned to the PARENT table.
//
// THE MUTATIONS THAT MUST TURN THIS RED: drop any CHECK; turn both-or-neither into "at least one";
// let a `so` column carry operands; drop the self-reference half of the distinctness CHECK; look the
// constraint up by name alone (33 matches, one per partition plus the parent).
func TestMigration0011ColumnsAndChecks(t *testing.T) {
	sql := maChay(t, file0011)

	for _, c := range []struct{ want, why string }{
		{"alter table cot_ngan_sach add column if not exists numerator_column_id text;", "cột tử số, TEXT như id"},
		{"alter table cot_ngan_sach add column if not exists denominator_column_id text;", "cột mẫu số, TEXT như id"},
		{"add constraint cot_ngan_sach_operands_only_on_percent check (kieu = 'phan_tram' or " +
			"(numerator_column_id is null and denominator_column_id is null))",
			"cột `so` không mang toán hạng"},
		{"add constraint cot_ngan_sach_operands_both_or_neither check " +
			"((numerator_column_id is null) = (denominator_column_id is null))",
			"cột % có đủ hai hoặc không có cái nào"},
		{"add constraint cot_ngan_sach_operands_distinct check (numerator_column_id <> denominator_column_id " +
			"and numerator_column_id <> id and denominator_column_id <> id)",
			"tử khác mẫu, và không trỏ vào chính nó"},
		{"add constraint cot_ngan_sach_operands_not_blank check " +
			"(btrim(numerator_column_id) <> '' and btrim(denominator_column_id) <> '')",
			"chuỗi rỗng là tham chiếu hỏng"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0011 thiếu %q — %s", c.want, c.why)
		}
	}

	for _, name := range []string{
		"cot_ngan_sach_operands_only_on_percent", "cot_ngan_sach_operands_both_or_neither",
		"cot_ngan_sach_operands_distinct", "cot_ngan_sach_operands_not_blank",
	} {
		guard := "where conrelid = 'cot_ngan_sach'::regclass and conname = '" + name + "'"
		if !strings.Contains(sql, guard) {
			t.Errorf("0011: ràng buộc %s không có canh ghim vào bảng CHA", name)
		}
	}
}

// TestMigration0011BackfillGuards — the backfill fills only EMPTY pairs, only unambiguous positions,
// never A = B, and writes its audit entry in the same statement.
func TestMigration0011BackfillGuards(t *testing.T) {
	sql := maChay(t, file0011)

	for _, c := range []struct{ want, why string }{
		{"where ties = 1 and ties_all = 1 and position = position_all",
			"chỉ vị trí xác định duy nhất theo CẢ HAI cách đọc col_N"},
		{"and p.m[1] <> p.m[2]", "col_A / col_A bị bỏ qua — vi phạm CHECK phân biệt"},
		{"where c.kieu = 'phan_tram' and c.deleted_at is null and c.numerator_column_id is null " +
			"and c.denominator_column_id is null",
			"chỉ chọn cột % còn sống và chưa có toán hạng"},
		{"and c.kieu = 'phan_tram' and c.numerator_column_id is null and c.denominator_column_id is null " +
			"returning", "UPDATE tự canh lại: không ghi đè cặp đã có, chạy lại không đổi gì"},
		{"join bang_ngan_sach b on b.tenant_id = p.tenant_id and b.id = p.bang_id",
			"chủ thể nhật ký là mã bảng — cột không có bảng thì không điền"},
		{"insert into audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta) " +
			"select tenant_id, 'system', 'system', '', 'dien_toan_hang_cot_phan_tram', sheet_code, now(),",
			"nhật ký hệ thống, cùng câu lệnh với thay đổi (luật 6 bất biến 3, 6, 8)"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0011 thiếu %q — %s", c.want, c.why)
		}
	}
	if n := strings.Count(sql, "update cot_ngan_sach"); n != 1 {
		t.Errorf("0011 có %d câu UPDATE cot_ngan_sach, mong đúng 1 (câu điền)", n)
	}
}

// TestMigration0011DestroysNothing — only ADDS. `cong_thuc` is never written; nothing is dropped or
// retyped; no constraint of 0006 is replaced.
func TestMigration0011DestroysNothing(t *testing.T) {
	sql := maChay(t, file0011)
	for _, c := range []struct{ banned, why string }{
		{"drop column", "xoá cột trên hồ sơ lưu trữ"},
		{"drop table", "xoá bảng"},
		{"drop constraint", "thay ràng buộc có sẵn của 0006"},
		{"alter column", "đổi kiểu hoặc NOT NULL của cột có sẵn"},
		{"cong_thuc =", "công thức gốc của xã phải giữ nguyên (luật 7)"},
		{"delete ", "xoá cứng"},
		{"truncate", "xoá cứng"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0011 chứa %q — %s", c.banned, c.why)
		}
	}
}

// formulaPattern extracts the regexp_match literal from 0011's RAW text (not maChay: that lower-cases,
// and the pattern is case-sensitive on purpose).
func formulaPattern(t *testing.T) *regexp.Regexp {
	t.Helper()
	b, err := fs.ReadFile(FS, file0011)
	if err != nil {
		t.Fatalf("đọc %s: %v", file0011, err)
	}
	m := regexp.MustCompile(`regexp_match\(c\.cong_thuc,\s*'([^']*)'\)`).FindStringSubmatch(string(b))
	if m == nil {
		t.Fatal("không tìm thấy mẫu regexp_match trong 0011 — bộ phân tích đã mù")
	}
	// THE SAME LITERAL, compiled by Go. PostgreSQL's ARE and Go's RE2 read every construct this
	// pattern uses (^ $ \s \* [1-9] [0-9]{0,2} groups) identically; \s differs only on \v.
	return regexp.MustCompile(m[1])
}

// TestMigration0011FormulaShape — the pattern the database runs accepts the starter-set formulas and
// nothing looser. Each rejected shape is one a guess would have filled wrongly or not at all.
func TestMigration0011FormulaShape(t *testing.T) {
	re := formulaPattern(t)
	for _, c := range []struct {
		formula string
		a, b    string // "" = must not match
	}{
		// web-admin/src/features/thu-chi/nhan-thu-chi.ts:811 and :819, and the spec's §7 sample.
		{"col_2 / col_1 * 100", "2", "1"},
		{"col_3 / col_1 * 100", "3", "1"},
		{"col_4 / col_2 * 100", "4", "2"},
		{"col_4/col_2*100", "4", "2"},
		{"  col_12 /\tcol_3 *  100  ", "12", "3"},
		{"col_2 / col_1 * 100.0", "", ""},
		{"COL_2 / col_1 * 100", "", ""},
		{"col_02 / col_1 * 100", "", ""},
		{"col_0 / col_1 * 100", "", ""},
		{"col_1000 / col_1 * 100", "", ""},
		{"(col_2) / col_1 * 100", "", ""},
		{"col_2 / col_1", "", ""},
		{"100 * col_2 / col_1", "", ""},
		{"(col_1 + col_2) / col_3 * 100", "", ""},
	} {
		m := re.FindStringSubmatch(c.formula)
		switch {
		case c.a == "" && m != nil:
			t.Errorf("%q KHỚP mẫu, mong bị bỏ qua (để NULL)", c.formula)
		case c.a != "" && m == nil:
			t.Errorf("%q KHÔNG khớp mẫu, mong khớp", c.formula)
		case c.a != "" && (m[1] != c.a || m[2] != c.b):
			t.Errorf("%q đọc ra col_%s / col_%s, mong col_%s / col_%s", c.formula, m[1], m[2], c.a, c.b)
		}
	}
}
