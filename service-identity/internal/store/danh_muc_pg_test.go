package store

import (
	"database/sql"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
)

// Integration tests for the two reference catalogues against a real PostgreSQL, sharing the harness
// in checker_pg_test.go (moKetNoi, xaRieng, ctxXa).
//
// WHY A REAL DATABASE: docDanhMuc's three load-bearing decisions are all in the SQL —
//
//	deleted_at IS NULL    soft-deleted rows disappear everywhere (rule 7, invariant 2)
//	NO dang_dung filter   a row out of use is RETURNED with its flag, because the catalogue screen
//	                      must show it while a picker filters it out
//	ORDER BY thu_tu, ma   the commune's own order, made TOTAL by a tie-break on a column that is
//	                      unique per commune
//
// BOTH TABLES ARE EXERCISED, not one as a stand-in for the other: they share a statement, so a
// typo'd table name in one of the two stores would be invisible if only one were tested.
//
// Skipped unless VIGOV_TEST_DSN is set.

func themKhoiNhiemVu(t *testing.T, db *sql.DB, tenantID, id, ma, nhan string, thuTu int) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO khoi_nhiem_vu (tenant_id, id, ma, nhan, thu_tu) VALUES ($1,$2,$3,$4,$5)`,
		tenantID, id, ma, nhan, thuTu)
	if err != nil {
		t.Fatalf("thêm khối nhiệm vụ: %v", err)
	}
}

func TestPgDanhMucLoaiDonViDanCu(t *testing.T) {
	db := moKetNoi(t)
	xa, xaKhac := xaRieng(t)

	// thu_tu PUT IN DELIBERATELY OUT OF ALPHABETICAL ORDER: `to-dan-pho` sorts first by code and
	// LAST by the commune's own arrangement. A query that lost `thu_tu` would still look right if
	// the fixture had been alphabetical.
	themLoaiDonViDanCu(t, db, xa, "ldv-1-"+xa, "thon", "Thôn", 1)
	themLoaiDonViDanCu(t, db, xa, "ldv-2-"+xa, "to-dan-pho", "Tổ dân phố", 2)
	// Another commune's rows must not appear. Same codes on purpose — that is the normal case
	// (rule 1, forbidden #4), and it is also what a missing tenant predicate would reveal.
	themLoaiDonViDanCu(t, db, xaKhac, "ldv-1-"+xaKhac, "thon", "THÔN CỦA XÃ KHÁC", 1)

	// One row taken out of use, one row soft-deleted: the first must come back carrying
	// DangDung = false, the second must not come back at all.
	if _, err := db.Exec(
		`UPDATE loai_don_vi_dan_cu SET dang_dung = false WHERE tenant_id = $1 AND id = $2`,
		xa, "ldv-2-"+xa); err != nil {
		t.Fatalf("tắt mục: %v", err)
	}
	themLoaiDonViDanCu(t, db, xa, "ldv-3-"+xa, "khu-pho", "Khu phố", 3)
	if _, err := db.Exec(
		`UPDATE loai_don_vi_dan_cu SET deleted_at = now(), delete_reason = $3
		 WHERE tenant_id = $1 AND id = $2`, xa, "ldv-3-"+xa, "nhập nhầm"); err != nil {
		t.Fatalf("xoá mềm mục: %v", err)
	}

	ds, err := NewLoaiDonViDanCuStore(pkgstore.New(db)).DanhSach(ctxXa(xa))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(ds) != 2 {
		t.Fatalf("nhận %d mục, muốn 2 (mục xoá mềm phải biến mất, mục đã tắt thì không): %+v", len(ds), ds)
	}
	if ds[0].Ma != "thon" || ds[1].Ma != "to-dan-pho" {
		t.Errorf("sai thứ tự thu_tu: %q, %q", ds[0].Ma, ds[1].Ma)
	}
	if ds[0].Nhan != "Thôn" {
		t.Errorf("RÒ RỈ hoặc sai nhãn: %q", ds[0].Nhan)
	}
	if !ds[0].DangDung {
		t.Error("mục đang dùng bị báo là đã tắt")
	}
	if ds[1].DangDung {
		t.Error("mục đã tắt bị báo là đang dùng — ô chọn sẽ mời một lựa chọn xã đã tắt")
	}
}

func TestPgDanhMucKhoiNhiemVu(t *testing.T) {
	// THE SECOND TABLE, READ THROUGH THE SAME STATEMENT. It is tested separately rather than trusted
	// to the first: the shared reader takes the table name as an argument, and a wrong constant in
	// one store is exactly the failure a single-table test cannot show.
	db := moKetNoi(t)
	xa, xaKhac := xaRieng(t)

	themKhoiNhiemVu(t, db, xa, "knv-1-"+xa, "khoi-uy-ban", "Khối Uỷ ban", 1)
	themKhoiNhiemVu(t, db, xa, "knv-2-"+xa, "khoi-dang", "Khối Đảng", 2)
	themKhoiNhiemVu(t, db, xaKhac, "knv-1-"+xaKhac, "khoi-uy-ban", "KHỐI CỦA XÃ KHÁC", 1)

	ds, err := NewKhoiNhiemVuStore(pkgstore.New(db)).DanhSach(ctxXa(xa))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(ds) != 2 {
		t.Fatalf("nhận %d mục, muốn 2: %+v", len(ds), ds)
	}
	// `thu_tu` again, not the alphabet: "khoi-dang" would sort first by code.
	if ds[0].Ma != "khoi-uy-ban" || ds[1].Ma != "khoi-dang" {
		t.Errorf("sai thứ tự thu_tu: %q, %q", ds[0].Ma, ds[1].Ma)
	}
	if ds[0].Nhan != "Khối Uỷ ban" {
		t.Errorf("RÒ RỈ hoặc sai nhãn: %q", ds[0].Nhan)
	}
}

func TestPgDanhMucMacDinhChiMotDongMoiXa(t *testing.T) {
	// THE FLAG A FORM PRE-SELECTS ON, AND THE SCHEMA'S GUARANTEE BEHIND IT. `moc_mac_dinh` is
	// GENERATED plus UNIQUE (tenant_id, moc_mac_dinh), so a second default in the same commune is
	// refused by the database rather than by a promise in Go (migration 0005:244). Both halves are
	// asserted: the flag really travels out, and the second insert really fails.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themKhoiNhiemVu(t, db, xa, "knv-1-"+xa, "khoi-uy-ban", "Khối Uỷ ban", 1)
	themKhoiNhiemVu(t, db, xa, "knv-2-"+xa, "khoi-dang", "Khối Đảng", 2)
	if _, err := db.Exec(
		`UPDATE khoi_nhiem_vu SET la_mac_dinh = true WHERE tenant_id = $1 AND id = $2`,
		xa, "knv-1-"+xa); err != nil {
		t.Fatalf("đặt mặc định: %v", err)
	}

	ds, err := NewKhoiNhiemVuStore(pkgstore.New(db)).DanhSach(ctxXa(xa))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if !ds[0].LaMacDinh || ds[1].LaMacDinh {
		t.Errorf("cờ mặc định sai: %+v", ds)
	}

	if _, err := db.Exec(
		`UPDATE khoi_nhiem_vu SET la_mac_dinh = true WHERE tenant_id = $1 AND id = $2`,
		xa, "knv-2-"+xa); err == nil {
		t.Error("hai mục cùng là mặc định mà cơ sở dữ liệu không từ chối — ô chọn sẽ chọn sẵn ngẫu nhiên")
	}
}
