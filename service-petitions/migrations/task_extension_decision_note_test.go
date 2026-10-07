package migrations

import (
	"io/fs"
	"os"
	"sort"
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0031 (`de_nghi_lui_han.decision_note`, user decision 07/10/2026).
// Same standing as task_code_immutable_test.go: it reads the SQL this binary embeds and asserts the
// column, the CHECKs and the freeze are WRITTEN. It does NOT prove PostgreSQL enforces them — that is
// internal/store/task_extension_decision_note_pg_test.go, which SKIPS without VIGOV_TEST_DSN.

const file0031 = "0031_task_extension_decision_note.sql"

// TestMigration0031ColumnAndConstraints — THE MUTATIONS THAT MUST TURN THIS RED: the column made NOT
// NULL or given a default (every pending row would carry a note, or the ADD fails on a populated
// table); a cap other than 5000 or counted in bytes; a blank string admitted as a note; a pending request allowed
// to hold one.
func TestMigration0031ColumnAndConstraints(t *testing.T) {
	sql := maChay(t, file0031)
	for _, c := range []struct{ want, why string }{
		{"alter table de_nghi_lui_han add column if not exists decision_note text;",
			"cột ghi chú quyết định phải có, NULLABLE, không mặc định"},
		{"check (decision_note is null or (btrim(decision_note) <> '' and char_length(decision_note) <= 5000))",
			"ghi chú: không rỗng, trần 5000 KÝ TỰ như đường ghi"},
		{"check (trang_thai <> 'cho-duyet' or decision_note is null)",
			"đề nghị còn chờ duyệt không được mang ghi chú quyết định"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0031 thiếu %q — %s", c.want, c.why)
		}
	}
}

// TestMigration0031CapMatchesWritePath — the CHECK's 5000 is the number QuyetDinhLuiHan validates the
// note against. If the Go constant moves alone, a note the route accepts becomes a 500.
func TestMigration0031CapMatchesWritePath(t *testing.T) {
	b, err := os.ReadFile("../internal/domain/nhiem_vu_ghi.go")
	if err != nil {
		t.Fatalf("đọc domain: %v", err)
	}
	if !strings.Contains(strings.Join(strings.Fields(string(b)), " "), "NoiDungNhatKyToiDa = 5000") {
		t.Error("domain.NoiDungNhatKyToiDa không còn là 5000 — trần CHECK decision_note của 0031 lệch đường ghi")
	}
	a, err := os.ReadFile("../internal/app/nhiem_vu.go")
	if err != nil {
		t.Fatalf("đọc app: %v", err)
	}
	if !strings.Contains(string(a), "KiemVanBanTuyChon(yc.GhiChu, domain.NoiDungNhatKyToiDa,") {
		t.Error("QuyetDinhLuiHan không còn kiểm ghi chú theo NoiDungNhatKyToiDa — xem lại trần của 0031")
	}
}

// TestMigration0031FreezeKeepsEveryRefusalOf0006 — the replaced guard still refuses the hard delete
// and every edit of the request as filed, and adds the note arm bound to the cho-duyet → decided move.
func TestMigration0031FreezeKeepsEveryRefusalOf0006(t *testing.T) {
	sql := maChay(t, file0031)
	for _, c := range []struct{ want, why string }{
		{"create or replace function de_nghi_lui_han_bat_bien()", "hàm canh phải được thay tại chỗ, cùng tên"},
		{"if tg_op = 'delete' then raise exception 'administrative record %: hard delete refused'",
			"mất chặn xoá cứng đề nghị"},
		{"if new.nhiem_vu_id is distinct from old.nhiem_vu_id or new.nguoi_de_nghi_ma is distinct from old.nguoi_de_nghi_ma " +
			"or new.han_moi is distinct from old.han_moi or new.ly_do is distinct from old.ly_do " +
			"or new.thoi_diem is distinct from old.thoi_diem then raise exception",
			"năm trường của đề nghị như đã gửi phải còn bất biến (0006)"},
		{"if new.decision_note is distinct from old.decision_note and not (old.trang_thai = 'cho-duyet' " +
			"and new.trang_thai in ('da-duyet', 'tu-choi')) then raise exception",
			"ghi chú chỉ được ghi trong đúng lượt chuyển chờ duyệt → đã quyết, rồi đóng băng"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0031 thiếu %q — %s", c.want, c.why)
		}
	}
	for _, c := range []struct{ banned, why string }{
		{"drop ", "tệp này không xoá gì — lệnh đảo ngược chỉ nằm trong chú thích REVERSAL"},
		{"alter column", "đổi kiểu hoặc NOT NULL của cột có sẵn"},
		{"delete from", "không xoá dòng nào"},
		{"truncate", "không làm rỗng bảng nào"},
		{"update de_nghi_lui_han", "tệp này không ghi dữ liệu — không backfill"},
		{"insert into", "tệp này không ghi dữ liệu"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0031 chứa %q — %s", c.banned, c.why)
		}
	}
}

// TestLatestExtensionGuardFreezesNote — whichever migration defines `de_nghi_lui_han_bat_bien` LAST is
// the one PostgreSQL runs. A later file that rewrites the guard and forgets the note arm, or the
// request-as-filed arm, must turn this red.
func TestLatestExtensionGuardFreezesNote(t *testing.T) {
	names, err := fs.Glob(FS, "*.sql")
	if err != nil {
		t.Fatalf("liệt kê migration: %v", err)
	}
	sort.Strings(names)
	latest := ""
	for _, n := range names {
		if strings.Contains(maChay(t, n), "create or replace function de_nghi_lui_han_bat_bien()") {
			latest = n
		}
	}
	if latest < file0031 {
		t.Fatalf("định nghĩa cuối của de_nghi_lui_han_bat_bien là %q, phải từ 0031 trở đi", latest)
	}
	sql := maChay(t, latest)
	for _, want := range []string{
		"if new.decision_note is distinct from old.decision_note and not (old.trang_thai = 'cho-duyet' " +
			"and new.trang_thai in ('da-duyet', 'tu-choi')) then raise exception",
		"or new.han_moi is distinct from old.han_moi or new.ly_do is distinct from old.ly_do",
		"if tg_op = 'delete' then raise exception",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("định nghĩa cuối của de_nghi_lui_han_bat_bien (%s) thiếu %q", latest, want)
		}
	}
}
