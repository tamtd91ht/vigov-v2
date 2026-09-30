package migrations

import (
	"io/fs"
	"sort"
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0024 (`nhiem_vu_bat_bien` refuses every change to `ma` again, ADR
// 0065 NV3). Same standing as task_issued_code_test.go: it reads the SQL this binary embeds and asserts
// the refusal is WRITTEN. It does not prove PostgreSQL enforces it — that needs VIGOV_TEST_DSN.

const file0024 = "0024_task_code_immutable.sql"

// TestMigration0024CodeRefusalIsUnconditional — THE MUTATIONS THAT MUST TURN THIS RED: keeping 0015's
// "through the ledger" condition; dropping the hard-delete refusal or 0016's deadline rule while
// "only touching `ma`"; dropping or emptying `task_issued_code` (it still stops a renamed-away code
// being reissued).
func TestMigration0024CodeRefusalIsUnconditional(t *testing.T) {
	sql := maChay(t, file0024)
	for _, c := range []struct{ want, why string }{
		{"create or replace function nhiem_vu_bat_bien()", "hàm canh phải được thay tại chỗ, cùng tên"},
		{"if new.ma is distinct from old.ma then raise exception 'administrative record %: `ma` is immutable'",
			"đổi mã phải bị từ chối KHÔNG điều kiện (ADR 0065 NV3)"},
		{"if tg_op = 'delete' then raise exception 'administrative record %: hard delete refused'",
			"mất chặn xoá cứng nhiệm vụ"},
		{"if new.han_ban_dau is distinct from new.han_xu_ly or exists (select 1 from de_nghi_lui_han d " +
			"where d.tenant_id = new.tenant_id and d.nhiem_vu_id = new.id and d.trang_thai = 'da-duyet') then raise exception",
			"luật hạn ban đầu của 0016 phải giữ nguyên"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0024 thiếu %q — %s", c.want, c.why)
		}
	}
	for _, c := range []struct{ banned, why string }{
		{"c.code = new.ma", "điều kiện 'đổi mã qua sổ' của 0015 còn sót"},
		{"drop ", "tệp này không xoá gì — sổ task_issued_code phải còn"},
		{"delete from", "không xoá dòng nào"},
		{"truncate", "không làm rỗng bảng nào"},
		{"update ", "tệp này không ghi dữ liệu"},
		{"alter table", "tệp này không sửa bảng"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0024 chứa %q — %s", c.banned, c.why)
		}
	}
}

// TestLatestTaskGuardRefusesCodeChange — whichever migration defines `nhiem_vu_bat_bien` LAST is the
// one PostgreSQL runs. A later file that re-relaxes `ma` must turn this red, not pass because 0024's
// own text is still right.
func TestLatestTaskGuardRefusesCodeChange(t *testing.T) {
	names, err := fs.Glob(FS, "*.sql")
	if err != nil {
		t.Fatalf("liệt kê migration: %v", err)
	}
	sort.Strings(names)
	latest := ""
	for _, n := range names {
		if strings.Contains(maChay(t, n), "create or replace function nhiem_vu_bat_bien()") {
			latest = n
		}
	}
	if latest == "" {
		t.Fatal("không migration nào định nghĩa nhiem_vu_bat_bien — bộ đọc đã mù")
	}
	sql := maChay(t, latest)
	if !strings.Contains(sql, "if new.ma is distinct from old.ma then raise exception 'administrative record %: `ma` is immutable'") {
		t.Errorf("định nghĩa cuối của nhiem_vu_bat_bien (%s) không còn từ chối đổi mã vô điều kiện", latest)
	}
}

// TestNoLaterMigrationDropsIssuedCodeLedger — the ledger outlives the edit path it was built for.
func TestNoLaterMigrationDropsIssuedCodeLedger(t *testing.T) {
	names, err := fs.Glob(FS, "*.sql")
	if err != nil {
		t.Fatalf("liệt kê migration: %v", err)
	}
	for _, n := range names {
		if n <= tep0015 {
			continue
		}
		sql := maChay(t, n)
		for _, banned := range []string{
			"drop table if exists task_issued_code", "drop table task_issued_code",
			"drop trigger if exists task_register_issued_code on nhiem_vu;",
			"drop trigger task_register_issued_code",
		} {
			// 0015 itself re-creates its trigger with DROP … IF EXISTS; any later file doing so without
			// re-creating it would free every future code for reuse.
			if strings.Contains(sql, banned) &&
				!strings.Contains(sql, "after insert on nhiem_vu for each row execute function task_register_issued_code()") {
				t.Errorf("%s chứa %q — sổ mã đã cấp phải còn (luật 7 bất biến 3)", n, banned)
			}
		}
	}
}
