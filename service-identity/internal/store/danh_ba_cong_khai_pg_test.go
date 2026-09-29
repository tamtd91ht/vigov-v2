package store

import (
	"database/sql"
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
)

// The public Mini App directory against a REAL PostgreSQL — the one place the publication predicate,
// the per-commune LEFT JOIN and NULLS LAST are executed by the engine that runs them in production.
// Skipped without VIGOV_TEST_DSN (rule 8); the statement-level half always runs, in
// danh_ba_cong_khai_test.go.
//
// EACH ROW OF COMMUNE A EXISTS TO MAKE ONE DEFECT VISIBLE — its name says which:
//
//	cong-khai-1   published, consent, order 1, unit U1           must appear FIRST
//	cong-khai-2   published, consent, NO order, no unit          must appear, LAST (NULLS LAST)
//	chua-cong-khai  not published                                drop `hien_tren_mini_app` → appears
//	bi-khoa       published, consent, locked                     drop `dang_hoat_dong` → appears
//	da-xoa        published, consent, soft-deleted               drop `deleted_at IS NULL` → appears
//
// "Published without consent" cannot be inserted at all — migration 0010 §3 refuses it, proven in
// danh_ba_mini_app_pg_test.go — so the read's own consent clauses are covered by the statement test.
//
// Commune B holds a published person AND a unit with the SAME id as A's U1, so a JOIN that forgot the
// commune would print B's unit name on A's row.

func themCongKhaiPg(t *testing.T, db *sql.DB, xa, id, boPhan string, hien, khoa, xoa bool, thuTu any) {
	t.Helper()
	var luc any
	ghiBoi := ""
	if hien {
		luc, ghiBoi = "2026-09-24T08:00:00Z", "CB-2026-GHI001"
	}
	var bp any
	if boPhan != "" {
		bp = boPhan
	}
	if _, err := db.Exec(
		`INSERT INTO nguoi_dung
		     (tenant_id, id, ma, ho_ten, email, chuc_vu, bo_phan_id, co_tai_khoan, mat_khau_hash,
		      dien_thoai_co_quan, di_dong_ca_nhan, co_zalo, dang_hoat_dong,
		      hien_tren_mini_app, dong_y_cong_khai_luc, dong_y_cong_khai_ghi_boi, thu_tu_danh_ba)
		 VALUES ($1,$2,$3,$4,$5,'Công chức',$6,false,'',
		         '0900000000','0900000000',true,$7,$8,$9,$10,$11)`,
		xa, id, "CB-"+id, id, id+"@example.gov.vn", bp, !khoa, hien, luc, ghiBoi, thuTu); err != nil {
		t.Fatalf("thêm %s: %v", id, err)
	}
	if xoa {
		if _, err := db.Exec(
			`UPDATE nguoi_dung SET deleted_at = now(), deleted_by = 'CB-TEST', delete_reason = 'nhập trùng'
			  WHERE tenant_id = $1 AND id = $2`, xa, id); err != nil {
			t.Fatalf("xoá mềm %s: %v", id, err)
		}
	}
}

func themBoPhanCongKhaiPg(t *testing.T, db *sql.DB, xa, id, ten string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO bo_phan (tenant_id, id, ten, ma) VALUES ($1,$2,$3,$4)`,
		xa, id, ten, "bp-"+strings.ToLower(id)); err != nil {
		t.Fatalf("thêm bộ phận %s: %v", id, err)
	}
}

func TestDanhBaCongKhaiPg(t *testing.T) {
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)

	themBoPhanCongKhaiPg(t, db, xaA, "u1", "VĂN PHÒNG A")
	themBoPhanCongKhaiPg(t, db, xaB, "u1", "VĂN PHÒNG CỦA XÃ B")

	themCongKhaiPg(t, db, xaA, "cong-khai-2", "", true, false, false, nil)
	themCongKhaiPg(t, db, xaA, "cong-khai-1", "u1", true, false, false, 1)
	themCongKhaiPg(t, db, xaA, "chua-cong-khai", "u1", false, false, false, 0)
	themCongKhaiPg(t, db, xaA, "bi-khoa", "u1", true, true, false, 0)
	themCongKhaiPg(t, db, xaA, "da-xoa", "u1", true, false, true, 0)
	themCongKhaiPg(t, db, xaB, "cua-xa-b", "u1", true, false, false, 0)

	kho := NewCanBoStore(pkgstore.New(db))
	doc := func(xa string) (string, string) {
		t.Helper()
		ds, err := kho.DanhBaCongKhai(ctxXa(xa))
		if err != nil {
			t.Fatalf("DanhBaCongKhai: %v", err)
		}
		var ten, bp []string
		for _, cb := range ds {
			ten = append(ten, cb.HoTen)
			bp = append(bp, cb.TenBoPhan)
		}
		return strings.Join(ten, ","), strings.Join(bp, ",")
	}

	ten, bp := doc(xaA)
	if ten != "cong-khai-1,cong-khai-2" {
		t.Fatalf("xã A = %q, muốn cong-khai-1,cong-khai-2 (chưa công khai / bị khoá / đã xoá không được hiện, "+
			"người không có thứ tự xếp sau)", ten)
	}
	if bp != "VĂN PHÒNG A," {
		t.Fatalf("tên bộ phận xã A = %q, muốn \"VĂN PHÒNG A,\" — RÒ RỈ GIỮA HAI XÃ nếu thấy tên của xã B", bp)
	}

	if ten, bp := doc(xaB); ten != "cua-xa-b" || bp != "VĂN PHÒNG CỦA XÃ B" {
		t.Fatalf("xã B = %q / %q, muốn cua-xa-b / VĂN PHÒNG CỦA XÃ B", ten, bp)
	}
}

// addUnitPg inserts one residential unit headed by `head` ("" = nobody).
func addUnitPg(t *testing.T, db *sql.DB, commune, id, name, head string, inUse bool, order int) {
	t.Helper()
	var h any
	if head != "" {
		h = head
	}
	if _, err := db.Exec(
		`INSERT INTO thon_to_dan_pho (tenant_id, id, ma, ten, dang_dung, head_staff_id, sort_order)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`, commune, id, "ma-"+id, name, inUse, h, order); err != nil {
		t.Fatalf("thêm thôn %s: %v", id, err)
	}
}

// Order and units headed against a real PostgreSQL. EACH UNIT MAKES ONE DEFECT VISIBLE:
//
//	t1 / t2     A, in use, head p1, ranks 1 / 2   must appear, in rank order
//	t-off       A, taken out of use, head p1      drop `tt.dang_dung` → appears
//	t-del       A, soft-deleted, head p1          drop `tt.deleted_at IS NULL` → appears
//	t-hidden    A, head = an UNPUBLISHED person   the person must stay absent, and so must the unit
//	b-unit      B, head = B's row with p1's id    drop the commune from the subquery → appears on A's p1
func TestPgPublicDirectoryOrderAndUnitsHeaded(t *testing.T) {
	db := moKetNoi(t)
	a, b := xaRieng(t)

	themCongKhaiPg(t, db, a, "p1", "", true, false, false, 4)
	themCongKhaiPg(t, db, a, "p2", "", true, false, false, nil)
	themCongKhaiPg(t, db, a, "hidden", "", false, false, false, 0)
	themCongKhaiPg(t, db, b, "p1", "", false, false, false, 0) // same id as A's p1, unpublished
	themCongKhaiPg(t, db, b, "cua-xa-b", "", true, false, false, 0)

	addUnitPg(t, db, a, "t2", "Thôn Hai", "p1", true, 2)
	addUnitPg(t, db, a, "t1", "Thôn Một", "p1", true, 1)
	addUnitPg(t, db, a, "t-off", "Thôn Đã Ngừng", "p1", false, 0)
	addUnitPg(t, db, a, "t-del", "Thôn Đã Xoá", "p1", true, 0)
	addUnitPg(t, db, a, "t-hidden", "Thôn Của Người Ẩn", "hidden", true, 0)
	addUnitPg(t, db, b, "b-unit", "Thôn Của Xã B", "p1", true, 0)
	if _, err := db.Exec(
		`UPDATE thon_to_dan_pho SET deleted_at = now(), deleted_by = 'CB-TEST', delete_reason = 'nhập trùng'
		  WHERE tenant_id = $1 AND id = 't-del'`, a); err != nil {
		t.Fatalf("xoá mềm thôn: %v", err)
	}

	got, err := NewCanBoStore(pkgstore.New(db)).DanhBaCongKhai(ctxXa(a))
	if err != nil {
		t.Fatalf("DanhBaCongKhai: %v", err)
	}
	if len(got) != 2 || got[0].HoTen != "p1" || got[1].HoTen != "p2" {
		t.Fatalf("xã A = %+v, muốn đúng [p1, p2]", got)
	}
	if got[0].DisplayOrder == nil || *got[0].DisplayOrder != 4 || got[1].DisplayOrder != nil {
		t.Fatalf("thứ tự = %v / %v, muốn 4 / nil", got[0].DisplayOrder, got[1].DisplayOrder)
	}
	if units := strings.Join(got[0].ResidentialUnitsHeaded, "|"); units != "Thôn Một|Thôn Hai" {
		t.Fatalf("thôn của p1 = %q, muốn \"Thôn Một|Thôn Hai\" — RÒ RỈ GIỮA HAI XÃ nếu thấy Thôn Của Xã B", units)
	}
	if len(got[1].ResidentialUnitsHeaded) != 0 {
		t.Fatalf("p2 không phụ trách thôn nào mà nhận %v", got[1].ResidentialUnitsHeaded)
	}
}
