package store

import (
	"database/sql"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
)

// Integration tests for the residential-unit read against a real PostgreSQL, sharing the harness in
// checker_pg_test.go (moKetNoi, xaRieng, ctxXa).
//
// WHY A REAL DATABASE, AND WHY THESE PROPERTIES SPECIFICALLY: every one of them lives entirely in
// the SQL, where nothing on the Go side can see it and no fake can disagree with it —
//
//	the LEFT join        a unit whose classification has not been entered must still come back. An
//	                     inner join drops it, and the Go code looks identical either way.
//	the commune on the   two communes both have a `thon`. Joining on the code alone labels one
//	                     join                 commune's hamlet with another's catalogue row, and a
//	                     single-commune test can never show it.
//	`deleted_at` in the  a soft-deleted type must empty the LABEL, not remove the UNIT.
//	                     JOIN
//	`deleted_at` in the  a soft-deleted unit must disappear everywhere (rule 7, invariant 2).
//	                     WHERE
//
// Skipped unless VIGOV_TEST_DSN is set. The DSN carries a password and lives only in the
// environment (rule 8).

func themLoaiDonViDanCu(t *testing.T, db *sql.DB, tenantID, id, ma, nhan string, thuTu int) {
	t.Helper()
	// `moc_mac_dinh` IS NEVER INSERTED: it is GENERATED ALWAYS, and naming it in an INSERT is an
	// error rather than an override.
	_, err := db.Exec(
		`INSERT INTO loai_don_vi_dan_cu (tenant_id, id, ma, nhan, thu_tu) VALUES ($1,$2,$3,$4,$5)`,
		tenantID, id, ma, nhan, thuTu)
	if err != nil {
		t.Fatalf("thêm loại đơn vị dân cư: %v", err)
	}
}

// themThonToDanPho inserts one residential unit. `loai` may be "" to mean NULL — the unit whose
// classification has not been entered, which is the whole reason the column is nullable.
func themThonToDanPho(t *testing.T, db *sql.DB, tenantID, id, ma, ten, loai string) {
	t.Helper()
	var l any
	if loai != "" {
		l = loai
	}
	_, err := db.Exec(
		`INSERT INTO thon_to_dan_pho (tenant_id, id, ma, ten, loai) VALUES ($1,$2,$3,$4,$5)`,
		tenantID, id, ma, ten, l)
	if err != nil {
		t.Fatalf("thêm thôn/tổ dân phố: %v", err)
	}
}

func dungThonToDanPhoStore(db *sql.DB) *ThonToDanPhoStore {
	return NewThonToDanPhoStore(pkgstore.New(db))
}

func TestPgThonToDanPhoKemNhanLoai(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themLoaiDonViDanCu(t, db, xa, "ldv-"+xa, "thon", "Thôn", 1)
	themThonToDanPho(t, db, xa, "tt-"+xa, "thon-binh-an", "Thôn Bình An", "thon")

	ds, err := dungThonToDanPhoStore(db).DanhSach(ctxXa(xa))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(ds) != 1 {
		t.Fatalf("nhận %d đơn vị, muốn 1", len(ds))
	}
	if ds[0].LoaiMa != "thon" || ds[0].LoaiNhan != "Thôn" {
		t.Errorf("nhãn loại không được nối: %+v", ds[0])
	}
	// Not entered is NULL, and NULL must not arrive as a counted zero.
	if ds[0].SoHo != nil || ds[0].NhanKhau != nil {
		t.Errorf("số liệu chưa nhập lại thành số: so_ho=%v nhan_khau=%v", ds[0].SoHo, ds[0].NhanKhau)
	}
}

func TestPgThonToDanPhoChuaPhanLoaiVanTraVe(t *testing.T) {
	// `loai` IS NULLABLE and `—` is a legitimate value on the commune's own screen. An INNER JOIN
	// would drop this row, which is a hamlet missing from the commune's list with nothing to say so.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themThonToDanPho(t, db, xa, "tt-"+xa, "thon-chua-phan-loai", "Thôn Chưa Phân Loại", "")

	ds, err := dungThonToDanPhoStore(db).DanhSach(ctxXa(xa))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(ds) != 1 {
		t.Fatalf("đơn vị chưa phân loại biến mất: nhận %d dòng", len(ds))
	}
	if ds[0].LoaiMa != "" || ds[0].LoaiNhan != "" {
		t.Errorf("đơn vị chưa phân loại lại có loại: %+v", ds[0])
	}
}

func TestPgLoaiXoaMemGiuLaiDonViVaMa(t *testing.T) {
	// `l.deleted_at IS NULL` SITS IN THE JOIN, NOT IN THE WHERE. In the WHERE it would filter out
	// the UNIT along with the deleted type — untidy catalogue data turning into "this hamlet does
	// not exist", and a 500 on a screen with nothing wrong with it.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themLoaiDonViDanCu(t, db, xa, "ldv-"+xa, "to-dan-pho", "Tổ dân phố", 1)
	themThonToDanPho(t, db, xa, "tt-"+xa, "to-dan-pho-so-1", "Tổ dân phố số 1", "to-dan-pho")
	if _, err := db.Exec(
		`UPDATE loai_don_vi_dan_cu SET deleted_at = now(), delete_reason = $3
		 WHERE tenant_id = $1 AND id = $2`, xa, "ldv-"+xa, "sắp xếp lại danh mục"); err != nil {
		t.Fatalf("xoá mềm loại: %v", err)
	}

	ds, err := dungThonToDanPhoStore(db).DanhSach(ctxXa(xa))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(ds) != 1 {
		t.Fatalf("đơn vị rơi mất cùng loại đã xoá mềm: nhận %d dòng", len(ds))
	}
	// The code survives so the screen can fall back to it; only the label empties.
	if ds[0].LoaiMa != "to-dan-pho" {
		t.Errorf("mã loại mất theo nhãn: %+v", ds[0])
	}
	if ds[0].LoaiNhan != "" {
		t.Errorf("nhãn của loại đã xoá mềm vẫn hiện: %q", ds[0].LoaiNhan)
	}
}

func TestPgNhanLoaiKhongLayNhamCuaXaKhac(t *testing.T) {
	// THE LEAK THIS JOIN COULD CAUSE, AND THE ONLY TEST THAT CAN SHOW IT. Every commune in the
	// country has a `thon`. Joining on the code alone — without `l.tenant_id = tt.tenant_id` —
	// labels commune A's hamlet with commune B's catalogue row, and no single-commune test would
	// ever notice.
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)

	themLoaiDonViDanCu(t, db, xaA, "ldv-a-"+xaA, "thon", "Thôn", 1)
	themLoaiDonViDanCu(t, db, xaB, "ldv-b-"+xaB, "thon", "THÔN CỦA XÃ B", 1)
	themThonToDanPho(t, db, xaA, "tt-a-"+xaA, "thon-binh-an", "Thôn Bình An", "thon")

	ds, err := dungThonToDanPhoStore(db).DanhSach(ctxXa(xaA))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(ds) != 1 {
		t.Fatalf("nhận %d đơn vị, muốn 1", len(ds))
	}
	if ds[0].LoaiNhan != "Thôn" {
		t.Fatalf("RÒ RỈ: nhãn loại lấy từ xã khác: %q", ds[0].LoaiNhan)
	}
}

func TestPgThonToDanPhoXoaMemBienMat(t *testing.T) {
	// Rule 7, invariant 2: every read path excludes deleted rows — and the row itself stays, which
	// is the other half of the same rule.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themThonToDanPho(t, db, xa, "tt-"+xa, "thon-cu", "Thôn Cũ", "")
	if _, err := db.Exec(
		`UPDATE thon_to_dan_pho SET deleted_at = now(), delete_reason = $3
		 WHERE tenant_id = $1 AND id = $2`, xa, "tt-"+xa, "sáp nhập địa bàn"); err != nil {
		t.Fatalf("xoá mềm đơn vị: %v", err)
	}

	ds, err := dungThonToDanPhoStore(db).DanhSach(ctxXa(xa))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(ds) != 0 {
		t.Fatalf("đơn vị đã xoá mềm vẫn lên danh sách: %+v", ds)
	}

	var n int
	if err := db.QueryRow(`SELECT count(*) FROM thon_to_dan_pho WHERE tenant_id = $1 AND id = $2`,
		xa, "tt-"+xa).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("bản ghi đã bị xoá thật: count = %d", n)
	}
}

func TestPgThonToDanPhoKhongDocSangXaKhac(t *testing.T) {
	// The commune is bound from the context by Scoped.QueryJoin and cannot be passed in (rule 1,
	// invariant 5). This is the assertion that the $1 in the statement really is that binding.
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)

	themThonToDanPho(t, db, xaA, "tt-a-"+xaA, "thon-binh-an", "Thôn Bình An", "")
	themThonToDanPho(t, db, xaB, "tt-b-"+xaB, "thon-binh-duong", "Thôn Bình Dương", "")

	ds, err := dungThonToDanPhoStore(db).DanhSach(ctxXa(xaB))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(ds) != 1 || ds[0].Ma != "thon-binh-duong" {
		t.Fatalf("RÒ RỈ hoặc thiếu: %+v", ds)
	}
}

func TestPgSoKhongKhacVoiChuaNhap(t *testing.T) {
	// 0 IS A STATEMENT, NULL IS THE ABSENCE OF ONE, and the difference has to survive the scan. A
	// plain int target would turn "not entered" into a counted zero, and a zero travels onward into
	// a report as a number.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themThonToDanPho(t, db, xa, "tt-0-"+xa, "thon-khong-ho", "Thôn A", "")
	if _, err := db.Exec(
		`UPDATE thon_to_dan_pho SET so_ho = 0 WHERE tenant_id = $1 AND id = $2`,
		xa, "tt-0-"+xa); err != nil {
		t.Fatalf("đặt số hộ = 0: %v", err)
	}
	themThonToDanPho(t, db, xa, "tt-null-"+xa, "thon-chua-nhap", "Thôn B", "")

	ds, err := dungThonToDanPhoStore(db).DanhSach(ctxXa(xa))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(ds) != 2 {
		t.Fatalf("nhận %d đơn vị, muốn 2", len(ds))
	}
	// ORDER BY ten: "Thôn A" trước "Thôn B".
	if ds[0].SoHo == nil || *ds[0].SoHo != 0 {
		t.Errorf("số hộ đã nhập bằng 0 bị mất: %v", ds[0].SoHo)
	}
	if ds[1].SoHo != nil {
		t.Errorf("số hộ chưa nhập lại thành %v", *ds[1].SoHo)
	}
}
