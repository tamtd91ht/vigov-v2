package store

import (
	"sort"
	"strings"
	"testing"
)

// GiaoViecDuoc — the read behind ResolveAssignableStaff — against a real PostgreSQL, sharing the
// harness in checker_pg_test.go and the row helpers in can_bo_ten_pg_test.go.
//
// The fake engine next door (can_bo_giao_viec_test.go) proves the predicate as the store WRITES it;
// this proves PostgreSQL agrees, including the text[] binding of `ANY($2)` by pgx.
//
// Skipped unless VIGOV_TEST_DSN is set (rule 8).
func TestPgGiaoViecChiMaDuocGiaoTrongXaMinh(t *testing.T) {
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)

	themNguoiCoMa(t, db, xaA, "nd-duoc-"+xaA, "CB-DUOC", "Nguyễn Văn A", true)
	themNguoiCoMa(t, db, xaA, "nd-khoa-"+xaA, "CB-KHOA", "Lê Văn C", false)
	themNguoiCoMa(t, db, xaA, "nd-xoa-"+xaA, "CB-XOA", "Trần Thị B", true)
	xoaMem(t, db, xaA, "nd-xoa-"+xaA)
	themNguoiCoMa(t, db, xaA, "nd-ktk-"+xaA, "CB-KHONG-TK", "Phạm Thị D", true)
	if _, err := db.Exec(
		`UPDATE nguoi_dung SET co_tai_khoan = false WHERE tenant_id = $1 AND id = $2`,
		xaA, "nd-ktk-"+xaA); err != nil {
		t.Fatalf("bỏ tài khoản: %v", err)
	}
	// Assignable — but in commune B.
	themNguoiCoMa(t, db, xaB, "nd-b-"+xaB, "CB-CUA-B", "Vũ Thị F", true)

	ra, err := dungCanBoStore(db).GiaoViecDuoc(ctxXa(xaA),
		[]string{"CB-DUOC", "CB-KHOA", "CB-XOA", "CB-KHONG-TK", "CB-CUA-B", "CB-KHONG-TON-TAI"})
	if err != nil {
		t.Fatalf("GiaoViecDuoc: %v", err)
	}
	sort.Strings(ra)
	if strings.Join(ra, ",") != "CB-DUOC" {
		t.Fatalf("mã giao việc được = %v, muốn chỉ CB-DUOC", ra)
	}
}
