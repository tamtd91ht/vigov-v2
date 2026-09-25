package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0011 (ADR 0045 §Phiên chưa có số). The weaker half, as the note
// on danh_ba_mini_app_test.go says: it reads the embedded SQL, it does not prove PostgreSQL enforces
// it. The proof is internal/store/cau_phien_pg_test.go, which SKIPS without VIGOV_TEST_DSN — so a
// deleted constraint has to turn red somewhere that always runs, and this is that somewhere.

const tep0011 = "0011_tai_khoan_zalo_va_phien_chua_co_so.sql"

func TestMigration0011RangBuocTaiKhoanZalo(t *testing.T) {
	sql := maChay(t, tep0011)

	for _, can := range []struct{ ten, bieuThuc, vi string }{
		{"tai_khoan_zalo_khoa_duy_nhat", "unique (app_id, bam_zalo_user_id)",
			"khoá theo (app, mã Zalo) — UNKNOWN #2 chưa đo mã Zalo có theo app hay không"},
		{"tai_khoan_zalo_bam_la_sha256", "check (bam_zalo_user_id ~ '^[0-9a-f]{64}$')",
			"mã Zalo thô lọt vào cột là dữ liệu cá nhân lưu nguyên văn (luật 3)"},
		{"tai_khoan_zalo_lien_ket_co_thoi_diem", "check ((cong_dan_id is null) = (lien_ket_luc is null))",
			"liên kết danh tính không có thời điểm là một lần liên kết không ai giải thích được"},
	} {
		if !strings.Contains(sql, "constraint "+can.ten+" "+can.bieuThuc) {
			t.Errorf("0011 KHÔNG CÒN ràng buộc %s %q — %s", can.ten, can.bieuThuc, can.vi)
		}
	}
}

func TestMigration0011PhienVanPhaiCoChuThe(t *testing.T) {
	sql := maChay(t, tep0011)
	if !strings.Contains(sql, "alter column cong_dan_id drop not null") {
		t.Error("0011 không còn nới NOT NULL của phien_cong_dan.cong_dan_id — phiên chưa có số không ghi được")
	}
	// Relaxing NOT NULL without this would allow a session belonging to NOBODY — revocable by
	// neither path, attributable to no one.
	if !strings.Contains(sql,
		"add constraint phien_cong_dan_co_chu_the check (cong_dan_id is not null or tai_khoan_zalo_id is not null)") {
		t.Error("0011 KHÔNG CÒN ràng buộc phien_cong_dan_co_chu_the — một phiên không thuộc ai được ghi")
	}
}

func TestMigration0011KhongCoLenhPhaHuy(t *testing.T) {
	// Rule 7: this file adds and relaxes; nothing in its EXECUTABLE part removes data or structure.
	// The REVERSAL prose mentions removal and is stripped by maChay, which is why this reads the
	// executable text only.
	sql := maChay(t, tep0011)
	for _, cam := range []string{"drop table", "drop column", "delete from", "truncate"} {
		if strings.Contains(sql, cam) {
			t.Errorf("0011 chứa %q ở phần chạy được", cam)
		}
	}
}
