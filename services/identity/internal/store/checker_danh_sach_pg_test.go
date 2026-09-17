package store

import (
	"strings"
	"testing"
)

// Integration tests for QuyenCua — the LIST behind GET /api/v1/sessions/current — against a real
// PostgreSQL. Skipped unless VIGOV_TEST_DSN is set; the harness lives in checker_pg_test.go.
//
// WHY THEY ARE HERE AND NOT SATISFIED BY THE HANDLER'S FAKE: what is being verified is that the
// list and the CHECK answer with the same predicates. That property lives entirely in the SQL —
// in the shared truyVanQuyenGoc, in each JOIN's ON clause, in co_tai_khoan and dang_hoat_dong —
// and a fake would simply agree with whatever the query says. A list looser than the check draws
// buttons the server then refuses; a list stricter than it hides work somebody is entitled to do,
// and nobody ever finds out, because a missing button asks no questions.

func quyenCuaList(t *testing.T, c *Checker, tenantID, nguoiID string) string {
	t.Helper()
	ra, err := c.QuyenCua(ctxXa(tenantID), canBo(nguoiID, tenantID))
	if err != nil {
		t.Fatalf("QuyenCua lỗi: %v", err)
	}
	ma := make([]string, 0, len(ra))
	for _, q := range ra {
		ma = append(ma, string(q))
	}
	return strings.Join(ma, ",")
}

func TestQuyenCuaTraDungNhungGiDuocCap(t *testing.T) {
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)
	dungXa(t, db, xaA, "vt-1", "nd-1", []string{"task.read", "admin.user"})

	// ORDER BY quyen_ma, so the answer is stable between requests: an unstable order turns every
	// client-side comparison into an accidental set comparison.
	if got := quyenCuaList(t, dungChecker(db), xaA, "nd-1"); got != "admin.user,task.read" {
		t.Errorf("QuyenCua = %q, muốn admin.user,task.read", got)
	}
}

func TestQuyenCuaKhongVuotSangXaKhac(t *testing.T) {
	// THE ONE THAT MATTERS MOST, and the reason this file exists at all: the same role id and the
	// same staff id in two communes — the shape produced by seeding communes from one template. A
	// JOIN matching on id alone would hand commune B's staff the list of commune A's grants, which
	// the web would then draw as a menu.
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)
	dungXa(t, db, xaA, "vt-chung", "nd-chung", []string{"task.approve", "admin.role"})
	dungXa(t, db, xaB, "vt-chung", "nd-chung", []string{"task.read"})

	c := dungChecker(db)
	if got := quyenCuaList(t, c, xaB, "nd-chung"); got != "task.read" {
		t.Fatalf("RÒ RỈ: ở xã B nhận %q, muốn đúng task.read", got)
	}
}

func TestQuyenCuaCungDieuKienVoiPhepKiem(t *testing.T) {
	// The list and the check must refuse the same people. Two columns, two cases, exactly as
	// TestCanBoNgungHoatDongBiTuChoi and TestNguoiChiCoTrongDanhBaKhongCoQuyenNao do for Allows:
	// they were one column carrying both meanings until migration 0003.
	db := moKetNoi(t)
	c := dungChecker(db)

	for ten, cot := range map[string]string{
		"tài khoản đã khoá":    "dang_hoat_dong",
		"chỉ có trong danh bạ": "co_tai_khoan",
	} {
		t.Run(ten, func(t *testing.T) {
			xa, _ := xaRieng(t)
			dungXa(t, db, xa, "vt-1", "nd-1", []string{"task.read"})
			if _, err := db.Exec(
				`UPDATE nguoi_dung SET `+cot+` = false WHERE tenant_id = $1 AND id = $2`,
				xa, "nd-1"); err != nil {
				t.Fatal(err)
			}

			if got := quyenCuaList(t, c, xa, "nd-1"); got != "" {
				t.Errorf("QuyenCua = %q, muốn rỗng — phép kiểm từ chối người này", got)
			}
			if c.Allows(ctxXa(xa), canBo("nd-1", xa), "task.read") {
				t.Error("phép kiểm lại cho qua — hai truy vấn đã lệch nhau")
			}
		})
	}
}
