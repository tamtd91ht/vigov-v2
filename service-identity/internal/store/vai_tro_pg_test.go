package store

import (
	"database/sql"
	"errors"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
)

// Integration tests for VaiTroCuaCanBo against a real PostgreSQL, sharing the harness in
// checker_pg_test.go (moKetNoi, xaRieng, ctxXa).
//
// WHY A REAL DATABASE: every property here belongs to the query, and a fake would only agree with
// whatever the query already does. The LEFT JOIN that separates "no role assigned" from "no such
// person", the `deleted_at` clause that sits in the JOIN rather than in the WHERE, and the
// commune constraint on the join — none of those are visible from the Go side.
//
// Skipped unless VIGOV_TEST_DSN is set.

// themVaiTro inserts one role. la_lanh_dao IS PASSED EXPLICITLY: the schema defaults it to false,
// and a test that relied on the default could not tell "the flag is read" from "the flag is
// always false".
func themVaiTro(t *testing.T, db *sql.DB, tenantID, id, ma, ten string, laLanhDao bool) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO vai_tro (tenant_id, id, ten, ma, la_lanh_dao) VALUES ($1,$2,$3,$4,$5)`,
		tenantID, id, ten, ma, laLanhDao)
	if err != nil {
		t.Fatalf("thêm vai trò: %v", err)
	}
}

// themNguoi inserts one staff row. vaiTroID may be "" to mean NULL — the directory entry nobody
// has assigned a role to yet, which is the whole reason the column is nullable.
func themNguoi(t *testing.T, db *sql.DB, tenantID, id, vaiTroID string) {
	t.Helper()
	var vt any
	if vaiTroID != "" {
		vt = vaiTroID
	}
	_, err := db.Exec(
		`INSERT INTO nguoi_dung (tenant_id, id, ma, ho_ten, email, vai_tro_id, co_tai_khoan)
		 VALUES ($1,$2,$3,$4,$5,$6,true)`,
		tenantID, id, "CB-"+id, "Nguyễn Văn A", id+"@xa.danang.gov.vn", vt)
	if err != nil {
		t.Fatalf("thêm cán bộ: %v", err)
	}
}

func dungVaiTroStore(db *sql.DB) *VaiTroStore { return NewVaiTroStore(pkgstore.New(db)) }

func TestPgVaiTroCuaCanBo(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themVaiTro(t, db, xa, "vt-"+xa, "chu-tich-ubnd", "Chủ tịch UBND", true)
	themNguoi(t, db, xa, "nd-"+xa, "vt-"+xa)

	vt, co, err := dungVaiTroStore(db).VaiTroCuaCanBo(ctxXa(xa), "nd-"+xa)
	if err != nil {
		t.Fatalf("VaiTroCuaCanBo: %v", err)
	}
	if !co {
		t.Fatal("cán bộ CÓ vai trò mà báo là chưa gán")
	}
	if vt.Ma != "chu-tich-ubnd" || vt.Ten != "Chủ tịch UBND" {
		t.Errorf("vai trò sai: %+v", vt)
	}
	if !vt.LaLanhDao {
		t.Error("la_lanh_dao = false với vai trò lãnh đạo — màn hình mặc định sẽ mở sai")
	}
}

func TestPgChuaGanVaiTroKhongPhaiLoi(t *testing.T) {
	// `nguoi_dung.vai_tro_id` IS NULLABLE: a person can be in the commune's directory before
	// anybody decides what they do. An inner join would return no row for them, which the caller
	// could only read as "this person does not exist" — and the route would answer 500 for an
	// ordinary state.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themNguoi(t, db, xa, "nd-"+xa, "")

	vt, co, err := dungVaiTroStore(db).VaiTroCuaCanBo(ctxXa(xa), "nd-"+xa)
	if err != nil {
		t.Fatalf("chưa gán vai trò lại thành lỗi: %v", err)
	}
	if co {
		t.Fatalf("báo là có vai trò: %+v", vt)
	}
}

func TestPgVaiTroXoaMemDocNhuChuaGan(t *testing.T) {
	// The person must still come back. `vt.deleted_at IS NULL` sits in the JOIN, not in the WHERE:
	// in the WHERE it would filter out the PERSON along with the role, turning untidy data into
	// "this account does not exist" and a 500 on a screen that has nothing wrong with it.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themVaiTro(t, db, xa, "vt-"+xa, "vai-tro-cu", "Vai trò cũ", true)
	themNguoi(t, db, xa, "nd-"+xa, "vt-"+xa)
	if _, err := db.Exec(
		`UPDATE vai_tro SET deleted_at = now(), delete_reason = $3
		 WHERE tenant_id = $1 AND id = $2`, xa, "vt-"+xa, "sắp xếp lại tổ chức"); err != nil {
		t.Fatalf("xoá mềm vai trò: %v", err)
	}

	vt, co, err := dungVaiTroStore(db).VaiTroCuaCanBo(ctxXa(xa), "nd-"+xa)
	if err != nil {
		t.Fatalf("vai trò đã xoá mềm lại thành lỗi: %v", err)
	}
	if co {
		t.Fatalf("vai trò đã xoá mềm vẫn hiện lên màn hình: %+v", vt)
	}
	// And the row is still there — rule 7 keeps it, this read simply does not name it.
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM vai_tro WHERE tenant_id = $1 AND id = $2`,
		xa, "vt-"+xa).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Error("dòng vai trò đã xoá mềm bị mất — xoá mềm phải giữ dữ liệu (luật 7)")
	}
}

func TestPgVaiTroKhongVuotSangXaKhac(t *testing.T) {
	// THE ONE THAT MATTERS MOST. Two communes hold roles with THE SAME id — which is exactly what
	// happens when ids are generated independently — and the join is constrained to the commune,
	// not merely to the matching id. Joining on id alone would name commune B's role on commune A's
	// screen, and no single-commune test could ever show it.
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)
	idVaiTro := "vt-chung-" + xaA

	themVaiTro(t, db, xaA, idVaiTro, "chu-tich-ubnd", "Chủ tịch UBND xã A", true)
	themVaiTro(t, db, xaB, idVaiTro, "ke-toan", "Kế toán xã B", false)
	themNguoi(t, db, xaA, "nd-"+xaA, idVaiTro)

	kho := dungVaiTroStore(db)

	vt, co, err := kho.VaiTroCuaCanBo(ctxXa(xaA), "nd-"+xaA)
	if err != nil || !co {
		t.Fatalf("đọc vai trò ở xã A: co=%v err=%v", co, err)
	}
	if vt.Ten != "Chủ tịch UBND xã A" {
		t.Fatalf("RÒ RỈ: xã A nhận vai trò %q", vt.Ten)
	}

	// The same person's id, asked in commune B's context: no row at all, because Scoped binds the
	// commune to $1. Not a different role — nothing.
	if _, _, err := kho.VaiTroCuaCanBo(ctxXa(xaB), "nd-"+xaA); !errors.Is(err, ErrCanBoKhongTonTai) {
		t.Fatalf("RÒ RỈ: đọc được cán bộ của xã A từ ngữ cảnh xã B (err = %v)", err)
	}
}

func TestPgKhongCoCanBoThiBaoKhongTonTai(t *testing.T) {
	// The third outcome, and it must be distinguishable from the other two: "no such person" is a
	// defect by the time this route runs — the middleware read this account one step earlier — so
	// it is an error, never a null role.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	_, _, err := dungVaiTroStore(db).VaiTroCuaCanBo(ctxXa(xa), "nd-khong-co-that")
	if !errors.Is(err, ErrCanBoKhongTonTai) {
		t.Fatalf("err = %v, muốn ErrCanBoKhongTonTai", err)
	}
}

func TestPgCanBoXoaMemBaoKhongTonTai(t *testing.T) {
	// A soft-deleted person is kept for the audit trail (rule 7) and must not be readable as a
	// current account. Unlike the deleted ROLE above, this one filters the row out entirely: there
	// is no person left to describe.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themVaiTro(t, db, xa, "vt-"+xa, "chuyen-vien", "Chuyên viên", false)
	themNguoi(t, db, xa, "nd-"+xa, "vt-"+xa)
	if _, err := db.Exec(
		`UPDATE nguoi_dung SET deleted_at = now() WHERE tenant_id = $1 AND id = $2`,
		xa, "nd-"+xa); err != nil {
		t.Fatalf("xoá mềm cán bộ: %v", err)
	}

	_, _, err := dungVaiTroStore(db).VaiTroCuaCanBo(ctxXa(xa), "nd-"+xa)
	if !errors.Is(err, ErrCanBoKhongTonTai) {
		t.Fatalf("err = %v, muốn ErrCanBoKhongTonTai", err)
	}
}

func TestPgIDRongKhongChayTruyVan(t *testing.T) {
	// Fail closed before the statement runs. An empty id would otherwise match whatever row has an
	// empty id, and "whatever row" is never an acceptable answer on this path.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	if _, _, err := dungVaiTroStore(db).VaiTroCuaCanBo(ctxXa(xa), ""); !errors.Is(err, ErrCanBoKhongTonTai) {
		t.Fatalf("err = %v, muốn ErrCanBoKhongTonTai", err)
	}
}
