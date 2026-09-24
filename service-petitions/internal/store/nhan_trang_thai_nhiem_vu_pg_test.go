package store

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// Integration tests for migration 0010 — a commune's label and order for the seven task statuses
// (open question #21, ADR 0035 §C).
//
// ⚠ READ THIS BEFORE BELIEVING A GREEN RUN: every test here SKIPS unless VIGOV_TEST_DSN is set, and
// the package still prints `ok`. The schema harness (TestMain, moKetNoi, xaRieng) lives in
// danh_muc_nhiem_vu_pg_test.go. The always-running half is migrations/nhan_trang_thai_nhiem_vu_test.go.
//
// WHY EACH REFUSAL IS ASSERTED BY SQLSTATE AND NOT BY `err != nil`: a misspelled column also makes
// an INSERT fail, and a test that only checks "it failed" would stay green with the CHECK gone.
// 23514 is check_violation, 23505 unique_violation, P0001 a RAISE from the trigger.

// themNhanTrangThai inserts one override and returns the error UNTOUCHED.
func themNhanTrangThai(db *sql.DB, xa, ma, nhan string, thuTu int, boi string) error {
	_, err := db.Exec(
		`INSERT INTO nhan_trang_thai_nhiem_vu (tenant_id, ma, nhan, thu_tu, cap_nhat_boi)
		 VALUES ($1,$2,$3,$4,$5)`, xa, ma, nhan, thuTu, boi)
	return err
}

// maLoiPg returns the SQLSTATE of err, or "" when it is not a server error.
func maLoiPg(err error) string {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

func canMaLoi(t *testing.T, err error, muon, vi string) {
	t.Helper()
	if err == nil {
		t.Fatalf("được chấp nhận — %s", vi)
	}
	if got := maLoiPg(err); got != muon {
		t.Fatalf("bị từ chối với SQLSTATE %q (%v), muốn %s — %s", got, err, muon, vi)
	}
}

// TestPgNhanTrangThaiGhiVaGhiDe — the one operation the table exists for: a commune sets a label and
// an order, then overwrites them in place (upsert on the primary key).
func TestPgNhanTrangThaiGhiVaGhiDe(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	if err := themNhanTrangThai(db, xa, "moi-giao", "Chưa thực hiện", 1, "CB-00123"); err != nil {
		t.Fatalf("ghi nhãn hợp lệ bị từ chối: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO nhan_trang_thai_nhiem_vu (tenant_id, ma, nhan, thu_tu, cap_nhat_boi)
		 VALUES ($1,'moi-giao','Mới giao',2,'CB-00124')
		 ON CONFLICT (tenant_id, ma) DO UPDATE
		   SET nhan = EXCLUDED.nhan, thu_tu = EXCLUDED.thu_tu,
		       cap_nhat_boi = EXCLUDED.cap_nhat_boi, cap_nhat_luc = now()`, xa); err != nil {
		t.Fatalf("ghi đè bị từ chối: %v", err)
	}
	var nhan string
	var thuTu, n int
	if err := db.QueryRow(
		`SELECT nhan, thu_tu, count(*) OVER () FROM nhan_trang_thai_nhiem_vu WHERE tenant_id = $1`,
		xa).Scan(&nhan, &thuTu, &n); err != nil {
		t.Fatal(err)
	}
	if n != 1 || nhan != "Mới giao" || thuTu != 2 {
		t.Errorf("sau ghi đè: %d dòng, nhãn %q, thứ tự %d — muốn 1 dòng, \"Mới giao\", 2", n, nhan, thuTu)
	}
}

// TestPgNhanTrangThaiMaNgoaiBayMaBiTuChoi IS #21 AT THE SCHEMA LEVEL: a label for a code the state
// machine does not have is a state with no way in and no way out.
func TestPgNhanTrangThaiMaNgoaiBayMaBiTuChoi(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	canMaLoi(t, themNhanTrangThai(db, xa, "da-huy", "Đã huỷ", 8, "CB-00123"), "23514",
		"xã tự thêm được mã trạng thái thứ tám — #21 chốt danh sách mã là ĐÓNG")
}

// TestPgNhanTrangThaiMotDongMoiMaMoiXa — composite key: one override per status per commune, and the
// same status is independently configurable in another commune (rule 1, invariant 6).
func TestPgNhanTrangThaiMotDongMoiMaMoiXa(t *testing.T) {
	db := moKetNoi(t)
	xa, xaKhac := xaRieng(t)

	if err := themNhanTrangThai(db, xa, "cho-duyet", "Chờ duyệt", 4, "CB-00123"); err != nil {
		t.Fatalf("ghi nhãn: %v", err)
	}
	canMaLoi(t, themNhanTrangThai(db, xa, "cho-duyet", "Chờ lãnh đạo duyệt", 4, "CB-00123"), "23505",
		"hai nhãn cho một mã trong một xã — nhãn hiện ra sẽ tuỳ thứ tự đọc")
	if err := themNhanTrangThai(db, xaKhac, "cho-duyet", "Chờ lãnh đạo duyệt", 4, "CB-00999"); err != nil {
		t.Fatalf("xã thứ hai không đặt được nhãn cho cùng mã — khoá phải hợp thành với tenant_id: %v", err)
	}
}

// TestPgNhanTrangThaiRangBuocGiaTri — blank / oversize label, position below 1, no author.
func TestPgNhanTrangThaiRangBuocGiaTri(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	// 101 characters of a 2-byte letter: over the bound in CHARACTERS (202 bytes). A byte-counting
	// CHECK would refuse 51 characters already; this case pins the character reading from above.
	dai := ""
	for i := 0; i < 101; i++ {
		dai += "ơ"
	}
	vua := dai[:len("ơ")*100]

	canMaLoi(t, themNhanTrangThai(db, xa, "tam-dung", " \t ", 6, "CB-00123"), "23514", "nhãn chỉ có khoảng trắng")
	canMaLoi(t, themNhanTrangThai(db, xa, "tam-dung", dai, 6, "CB-00123"), "23514", "nhãn 101 ký tự")
	canMaLoi(t, themNhanTrangThai(db, xa, "tam-dung", "Tạm dừng", 0, "CB-00123"), "23514", "thu_tu = 0")
	canMaLoi(t, themNhanTrangThai(db, xa, "tam-dung", "Tạm dừng", 6, "  "), "23514",
		"không ghi ai sửa (luật 6 bất biến 8)")

	if err := themNhanTrangThai(db, xa, "tam-dung", vua, 6, "CB-00123"); err != nil {
		t.Fatalf("nhãn đúng 100 ký tự (200 byte) bị từ chối — trần đang đếm byte chứ không đếm ký tự: %v", err)
	}
}

// TestPgNhanTrangThaiTrigger — hard DELETE refused; the row cannot be moved to another status code
// or another commune. Enforced in the database, not promised by the application (ADR 0013).
func TestPgNhanTrangThaiTrigger(t *testing.T) {
	db := moKetNoi(t)
	xa, xaKhac := xaRieng(t)

	if err := themNhanTrangThai(db, xa, "hoan-thanh", "Hoàn thành", 5, "CB-00123"); err != nil {
		t.Fatalf("ghi nhãn: %v", err)
	}

	_, err := db.Exec(`DELETE FROM nhan_trang_thai_nhiem_vu WHERE tenant_id = $1 AND ma = 'hoan-thanh'`, xa)
	canMaLoi(t, err, "P0001", "xoá cứng được — 'về mặc định' phải là một lần GHI có vết")

	_, err = db.Exec(`UPDATE nhan_trang_thai_nhiem_vu SET ma = 'cho-duyet'
	                   WHERE tenant_id = $1 AND ma = 'hoan-thanh'`, xa)
	canMaLoi(t, err, "P0001", "chuyển được nhãn của một trạng thái sang trạng thái khác")

	_, err = db.Exec(`UPDATE nhan_trang_thai_nhiem_vu SET tenant_id = $2
	                   WHERE tenant_id = $1 AND ma = 'hoan-thanh'`, xa, xaKhac)
	canMaLoi(t, err, "P0001", "chuyển được cấu hình của xã này sang xã khác (luật 1)")

	// And the one edit that IS allowed still passes the trigger.
	if _, err := db.Exec(`UPDATE nhan_trang_thai_nhiem_vu SET nhan = 'Đã xong', cap_nhat_boi = 'CB-00124'
	                       WHERE tenant_id = $1 AND ma = 'hoan-thanh'`, xa); err != nil {
		t.Fatalf("sửa nhãn bị trigger chặn: %v", err)
	}
}
