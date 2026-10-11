package migrations

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/textfold"
)

// Schema-TEXT checks for migration 0041 (accent-insensitive register search, bug sheet 10/10 row 8).
// It reads the SQL this binary embeds. It does NOT prove PostgreSQL computes the columns — that is
// internal/store/search_folded_pg_test.go, which SKIPS without VIGOV_TEST_DSN.

const file0041 = "0041_search_folded_columns.sql"

// rawSQL is the file as embedded, comments included, case kept: the fold expression carries capital
// letters (`Đ`, `U&'\031B'`) that maChay's lower-casing would erase from both sides of the comparison.
func rawSQL(t *testing.T, name string) string {
	t.Helper()
	b, err := fs.ReadFile(FS, name)
	if err != nil {
		t.Fatalf("đọc %s: %v", name, err)
	}
	// A checkout with CRLF line endings must compare like one with LF.
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

// TestMigration0041ColumnsAreTextfoldSQL — THE STORED FOLD AND THE TERM'S FOLD ARE ONE FUNCTION. The
// store folds the search term with textfold.SQL at query time; the column was folded with the text in
// this file at migration time. If core/textfold's table changes, this turns red: the stored columns
// were computed with the OLD table, and a new migration must recompute them (0041 is applied and
// cannot be edited).
func TestMigration0041ColumnsAreTextfoldSQL(t *testing.T) {
	sql := rawSQL(t, file0041)
	for _, c := range []struct{ table, column, source string }{
		{"nhiem_vu", "title_folded", "tieu_de"},
		{"phieu_phan_anh", "content_folded", "noi_dung"},
		{"phieu_phan_anh", "address_folded", "dia_chi"},
	} {
		want := "ALTER TABLE " + c.table + " ADD COLUMN IF NOT EXISTS " + c.column +
			" TEXT GENERATED ALWAYS AS (\n    " + textfold.SQL(c.source) + "\n) STORED;"
		if !strings.Contains(sql, want) {
			t.Errorf("0041 không còn khai %s.%s đúng bằng textfold.SQL(%q) — cột lưu và từ khoá tìm sẽ gập khác nhau",
				c.table, c.column, c.source)
		}
	}
}

// TestMigration0041WritesNothing — the file adds derived columns and touches no business value.
func TestMigration0041WritesNothing(t *testing.T) {
	sql := maChay(t, file0041)
	for _, c := range []struct{ banned, why string }{
		{"drop ", "tệp này không xoá gì — lệnh đảo ngược chỉ nằm trong chú thích REVERSAL"},
		{"alter column", "không đổi kiểu hay NOT NULL của cột có sẵn"},
		{"delete from", "không xoá dòng nào"},
		{"truncate", "không làm rỗng bảng nào"},
		{"update ", "không ghi dữ liệu — cột sinh do PostgreSQL tính"},
		{"insert into", "không ghi dữ liệu"},
		{"create extension", "không cài extension (ADR 0010)"},
		{"unaccent", "không dùng unaccent (ADR 0010)"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0041 chứa %q — %s", c.banned, c.why)
		}
	}
}
