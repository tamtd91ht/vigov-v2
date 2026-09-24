package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0011 (the two terminal branches, user decisions 24-25/09/2026).
//
// WHAT THIS IS AND IS NOT — same standing as nhan_trang_thai_nhiem_vu_test.go. It reads the SQL this
// binary embeds and asserts the constraints and the trigger are WRITTEN. It does NOT prove
// PostgreSQL enforces them, applies the file, or evaluates the CASE; that needs VIGOV_TEST_DSN
// (tools/schema-smoke, and the next card's pg suite). It exists so that a widened or deleted CHECK
// goes red somewhere that always runs.

const (
	tep0004 = "0004_phieu_phan_anh.sql"
	tep0011 = "0011_ket_thuc_nhanh_phan_anh.sql"
)

// TestMigration0011HaiNhanhCoTrongMayTrangThai — the two codes 0011 binds must be codes 0004's
// closed status list actually has. A typo (`chuyen-cap-tren` -> `chuyen-len-tren`) would make its
// WHEN arm unreachable, and the ELSE arm would then FORBID a reason on the very status that needs
// one: every referral would fail at the database, and the list of nine would still look right.
func TestMigration0011HaiNhanhCoTrongMayTrangThai(t *testing.T) {
	chin := danhSachMaTrongCheck(t, maChay(t, tep0004), "phieu_phan_anh_trang_thai_hop_le", "trang_thai")
	if len(chin) != 9 {
		t.Fatalf("0004 khai %d mã trạng thái, mong 9 (ADR 0027) — bộ phân tích đã mù hoặc 0004 đã đổi: %v",
			len(chin), chin)
	}
	co := map[string]bool{}
	for _, m := range chin {
		co[m] = true
	}
	sql := maChay(t, tep0011)
	for _, m := range []string{"khong-tiep-nhan", "chuyen-cap-tren"} {
		if !co[m] {
			t.Errorf("0004 không có mã %q", m)
		}
		if !strings.Contains(sql, "when '"+m+"' then") {
			t.Errorf("0011 thiếu nhánh WHEN '%s' trong ràng buộc gắn trạng thái", m)
		}
	}
}

// TestMigration0011RangBuocVaTrigger — the binding in both directions, the caps, and the trigger.
//
// THE MUTATIONS THAT MUST TURN THIS RED: delete the ELSE arm (a reason could then sit on a petition
// still being processed); drop `co_quan_nhan is not null` from the referral arm; drop the `is not
// null` guard before `btrim` (a CHECK treats NULL as passed, so an absent reason would be accepted);
// remove the trigger.
func TestMigration0011RangBuocVaTrigger(t *testing.T) {
	sql := maChay(t, tep0011)

	for _, c := range []struct{ can, vi string }{
		{"add column if not exists ly_do_ket_thuc_nhanh text;", "cột lý do phải có và NULLABLE"},
		{"add column if not exists co_quan_nhan text;", "cột cơ quan nhận phải có và NULLABLE"},
		{"add column if not exists ket_thuc_nhanh_luc timestamptz;", "cột thời điểm phải có và NULLABLE"},
		{"when 'khong-tiep-nhan' then ly_do_ket_thuc_nhanh is not null and btrim(ly_do_ket_thuc_nhanh) <> '' " +
			"and co_quan_nhan is null and ket_thuc_nhanh_luc is not null",
			"không tiếp nhận: bắt buộc lý do không rỗng, không có cơ quan nhận, có thời điểm"},
		{"when 'chuyen-cap-tren' then ly_do_ket_thuc_nhanh is not null and btrim(ly_do_ket_thuc_nhanh) <> '' " +
			"and co_quan_nhan is not null and btrim(co_quan_nhan) <> '' and ket_thuc_nhanh_luc is not null",
			"chuyển cấp trên: bắt buộc lý do VÀ cơ quan nhận không rỗng, có thời điểm"},
		{"else ly_do_ket_thuc_nhanh is null and co_quan_nhan is null and ket_thuc_nhanh_luc is null end",
			"mọi trạng thái khác: cả ba NULL — thiếu nhánh ELSE thì lý do trôi sang phiếu đang xử lý"},
		{"check (char_length(ly_do_ket_thuc_nhanh) <= 2000)", "lý do phải có trần theo KÝ TỰ, bằng KetQuaToiDa"},
		{"check (char_length(co_quan_nhan) <= 200)", "cơ quan nhận phải có trần theo KÝ TỰ"},
		{"before update on phieu_phan_anh for each row execute function phieu_phan_anh_ket_thuc_nhanh_bat_bien()",
			"trigger giữ bất biến ba cột sau khi ghi đã mất"},
		{"if old.ly_do_ket_thuc_nhanh is not null and new.ly_do_ket_thuc_nhanh is distinct from old.ly_do_ket_thuc_nhanh",
			"trigger không còn chặn sửa lý do"},
		{"if old.co_quan_nhan is not null and new.co_quan_nhan is distinct from old.co_quan_nhan",
			"trigger không còn chặn sửa cơ quan nhận"},
		{"if old.ket_thuc_nhanh_luc is not null and new.ket_thuc_nhanh_luc is distinct from old.ket_thuc_nhanh_luc",
			"trigger không còn chặn sửa thời điểm"},
	} {
		if !strings.Contains(sql, c.can) {
			t.Errorf("0011 thiếu %q — %s", c.can, c.vi)
		}
	}
}

// TestMigration0011KhongPhaHuy — this file only ADDS. It must not drop or retype anything, must not
// replace 0004's archival guard (its reversal would then depend on restoring 0004's exact body), and
// must not add NOT NULL columns (which would fail on, or invent a value for, every existing row).
//
// The executable text is checked (comments stripped by maChay), so the REVERSAL prose that spells out
// the down statements does not trip it.
func TestMigration0011KhongPhaHuy(t *testing.T) {
	sql := maChay(t, tep0011)
	for _, c := range []struct{ cam, vi string }{
		{"drop column", "xoá cột trên hồ sơ lưu trữ"},
		{"drop constraint", "gỡ một ràng buộc — cửa sổ không ràng buộc nào giữ"},
		{"alter column", "đổi kiểu hoặc NOT NULL của cột có sẵn"},
		{"function ho_so_luu_tru_bat_bien", "thay hàm canh lưu trữ của 0004"},
		{"text not null", "cột mới NOT NULL — hỏng hoặc bịa giá trị cho mọi dòng có sẵn"},
		{"timestamptz not null", "cột mới NOT NULL — hỏng hoặc bịa giá trị cho mọi dòng có sẵn"},
		{"dong_luc", "ghi thời điểm rẽ nhánh vào dong_luc — phiếu bị từ chối thành phiếu đã giải quyết"},
	} {
		if strings.Contains(sql, c.cam) {
			t.Errorf("0011 chứa %q — %s", c.cam, c.vi)
		}
	}
}
