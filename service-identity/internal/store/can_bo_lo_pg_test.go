package store

import (
	"database/sql"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
)

// Integration tests for TheoNhieuID — the read behind BatchGetStaff — against a real PostgreSQL,
// sharing the harness in checker_pg_test.go (moKetNoi, xaRieng, ctxXa) and the row helpers in
// vai_tro_pg_test.go (themVaiTro, themNguoi).
//
// WHY A REAL DATABASE, and here it is not a formality: FOUR of these properties live entirely in
// the SQL, and a fake would simply agree with whatever the query already does.
//
//  1. THE LEFT JOIN. An inner join would DROP every staff member who holds no role — turning
//     "this person has no role" into "this id does not exist", which the caller renders as a
//     deleted record. `nguoi_dung.vai_tro_id` is nullable precisely so that case exists.
//  2. THE COMMUNE PREDICATE ON THE JOIN (`vt.tenant_id = nd.tenant_id`). Joining on role id alone
//     would pick up another commune's role wherever two ids collide, and no single-commune test
//     can ever show it.
//  3. THE COMMUNE PREDICATE ON THE ROW. An id belonging to another commune must come back ABSENT,
//     indistinguishable from "does not exist" — never as an error that confirms it exists
//     somewhere.
//  4. `nd.deleted_at IS NULL`. A soft-deleted person is kept for the audit trail (rule 7) and must
//     not be served.
//
// Skipped unless VIGOV_TEST_DSN is set. The DSN carries a password and lives only in the
// environment (rule 8).

func dungCanBoStore(db *sql.DB) *CanBoStore { return NewCanBoStore(pkgstore.New(db)) }

// maVaiTro reads back what the batch query returned for one id, or reports it absent.
func maVaiTro(t *testing.T, db *sql.DB, xa string, ids []string, tim string) (string, bool) {
	t.Helper()
	hang, err := dungCanBoStore(db).TheoNhieuID(ctxXa(xa), ids)
	if err != nil {
		t.Fatalf("TheoNhieuID: %v", err)
	}
	for _, cb := range hang {
		if cb.ID == tim {
			return cb.VaiTroMa, true
		}
	}
	return "", false
}

// Property 1: the person with no role is RETURNED, with an empty slug. This is the assertion that
// turns red if the LEFT JOIN is ever tightened to an inner join.
func TestPgLoTraCaNguoiKhongCoVaiTro(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themVaiTro(t, db, xa, "vt-"+xa, "chu-tich-ubnd", "Chủ tịch UBND", true)
	themNguoi(t, db, xa, "co-vt-"+xa, "vt-"+xa)
	themNguoi(t, db, xa, "khong-vt-"+xa, "") // vai_tro_id NULL

	ids := []string{"co-vt-" + xa, "khong-vt-" + xa}

	if ma, co := maVaiTro(t, db, xa, ids, "co-vt-"+xa); !co || ma != "chu-tich-ubnd" {
		t.Errorf("người có vai trò: ma = %q, có = %v; muốn slug vai trò", ma, co)
	}
	ma, co := maVaiTro(t, db, xa, ids, "khong-vt-"+xa)
	if !co {
		t.Fatal("NGƯỜI KHÔNG CÓ VAI TRÒ BỊ RỚT KHỎI KẾT QUẢ — bên gọi sẽ đọc là bản ghi đã xoá")
	}
	if ma != "" {
		t.Errorf("người không có vai trò trả ma = %q, muốn rỗng", ma)
	}
}

// Properties 2 and 3: a second commune's row and a second commune's role are both invisible, and
// the absence is silent — no error, nothing confirming the record exists elsewhere.
func TestPgLoKhongVuotSangXaKhac(t *testing.T) {
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)

	// The SAME role id in both communes, holding different slugs. If the join dropped
	// `vt.tenant_id = nd.tenant_id`, commune A's person would come back wearing commune B's role.
	themVaiTro(t, db, xaA, "vt-chung", "vai-tro-cua-A", "Vai trò A", false)
	themVaiTro(t, db, xaB, "vt-chung", "vai-tro-cua-B", "Vai trò B", false)
	themNguoi(t, db, xaA, "nd-a-"+xaA, "vt-chung")
	themNguoi(t, db, xaB, "nd-b-"+xaB, "vt-chung")

	ids := []string{"nd-a-" + xaA, "nd-b-" + xaB}

	ma, co := maVaiTro(t, db, xaA, ids, "nd-a-"+xaA)
	if !co {
		t.Fatal("không đọc được người của chính xã mình")
	}
	if ma != "vai-tro-cua-A" {
		t.Errorf("ma vai trò = %q — join đã bắt sang vai trò của xã khác", ma)
	}
	if _, co := maVaiTro(t, db, xaA, ids, "nd-b-"+xaB); co {
		t.Error("ĐỌC ĐƯỢC CÁN BỘ CỦA XÃ KHÁC — đây là rò rỉ giữa hai cơ quan nhà nước (luật 1)")
	}
}

// Property 4: a soft-deleted person is kept for the audit trail and is not served.
func TestPgLoBoQuaNguoiDaXoaMem(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themNguoi(t, db, xa, "da-xoa-"+xa, "")
	if _, err := db.Exec(
		`UPDATE nguoi_dung SET deleted_at = now() WHERE tenant_id = $1 AND id = $2`,
		xa, "da-xoa-"+xa); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}

	if _, co := maVaiTro(t, db, xa, []string{"da-xoa-" + xa}, "da-xoa-"+xa); co {
		t.Error("người đã xoá mềm vẫn được trả về")
	}
}

// A batch of ids that match nothing is an empty answer, not an error. The caller maps by id and
// renders what it got; ADR 0012 decision 2 makes "fewer items than ids" the normal case.
func TestPgLoKhongKhopGiThiRong(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	hang, err := dungCanBoStore(db).TheoNhieuID(ctxXa(xa), []string{"khong-ton-tai"})
	if err != nil {
		t.Fatalf("TheoNhieuID: %v", err)
	}
	if len(hang) != 0 {
		t.Errorf("số dòng = %d, muốn 0", len(hang))
	}
}
