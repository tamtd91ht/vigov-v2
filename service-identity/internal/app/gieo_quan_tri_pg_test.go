package app

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/vihat/vigov/core/password"
	"github.com/vihat/vigov/core/secret"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// THE CASES ONLY A REAL POSTGRESQL CAN DECIDE for the default-administrator seed:
//
//	`ON CONFLICT (tenant_id, email|ma)` on a table PARTITIONED BY HASH really resolves
//	`INSERT … SELECT … FROM quyen` really passes the FK to the catalogue for every key
//	the CHECK of migration 0009 §3 accepts the row (account with a non-empty hash)
//	TWO CONCURRENT FIRST SIGN-INS end with exactly one account — the property the fake driver can
//	only simulate, and the one a read-then-insert would get wrong
//
// Skipped unless VIGOV_TEST_DSN is set. TestMain in danh_ba_can_bo_pg_test.go builds the schema.

// ucDangNhapGieoPg builds the sign-in use case over a real pool, seed switched on.
func ucDangNhapGieoPg(t *testing.T, db *sql.DB) *DangNhap {
	t.Helper()
	kho := pkgstore.New(db)
	uc := NewDangNhap(kho, idstore.NewCanBoStore(kho), idstore.NewPhienStore(kho), kyGq{}, slogBoQua())
	if err := uc.BatGieoQuanTri(secret.Secret(matKhauGieoGia)); err != nil {
		t.Fatalf("BatGieoQuanTri: %v", err)
	}
	return uc
}

func slogBoQua() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// moPoolNhieuKetNoi is moPool with several physical connections. `search_path` travels as a
// connection parameter, so every connection the pool opens lands in the test schema — `SET` on a
// pool would reach one connection only.
func moPoolNhieuKetNoi(t *testing.T) *sql.DB {
	t.Helper()
	if dsnChung == "" {
		t.Skip("VIGOV_TEST_DSN chưa đặt — bỏ qua test tích hợp")
	}
	dsn := dsnChung
	switch {
	case strings.Contains(dsn, "://") && strings.Contains(dsn, "?"):
		dsn += "&search_path=" + schemaChung
	case strings.Contains(dsn, "://"):
		dsn += "?search_path=" + schemaChung
	default:
		dsn += " search_path=" + schemaChung
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("mở kết nối: %v", err)
	}
	db.SetMaxOpenConns(4)
	t.Cleanup(func() { db.Close() })
	return db
}

type demGieoPg struct {
	taiKhoan, vaiTro, oQuyen, catalogue, vetGieo int
}

func demPg(t *testing.T, db *sql.DB, xa string) demGieoPg {
	t.Helper()
	var d demGieoPg
	for _, c := range []struct {
		dich *int
		sql  string
		args []any
	}{
		{&d.taiKhoan, `SELECT count(*) FROM nguoi_dung WHERE tenant_id = $1 AND email = 'admin'`, []any{xa}},
		{&d.vaiTro, `SELECT count(*) FROM vai_tro WHERE tenant_id = $1 AND ma = 'quan-tri-he-thong'`, []any{xa}},
		{&d.oQuyen, `SELECT count(*) FROM vai_tro_quyen WHERE tenant_id = $1`, []any{xa}},
		{&d.catalogue, `SELECT count(*) FROM quyen`, nil},
		{&d.vetGieo, `SELECT count(*) FROM audit_log WHERE tenant_id = $1 AND action = 'gieo_quan_tri_mac_dinh'`, []any{xa}},
	} {
		if err := db.QueryRow(c.sql, c.args...).Scan(c.dich); err != nil {
			t.Fatalf("đếm: %v", err)
		}
	}
	return d
}

func TestPgGieoQuanTriHaiXaMoiXaMotAdminLanHaiKhongGieoLai(t *testing.T) {
	db := moPool(t)
	uc := ucDangNhapGieoPg(t, db)
	// Derived, not a second xaRiengPg call: two clock reads in a row can return the same
	// nanosecond on a coarse clock, and xaRiengPg digits never include 'Z'.
	xaA := xaRiengPg(t)
	xaB := xaA[:25] + "Z"

	for _, xa := range []string{xaA, xaB} {
		ctx := tenant.Into(context.Background(), tenant.ID(xa))
		kq, err := uc.Chay(ctx, yeuCauAdmin(matKhauGieoGia))
		if err != nil {
			t.Fatalf("xã %s: đăng nhập admin lần đầu lỗi: %v", xa, err)
		}
		if !kq.CanBo.PhaiDoiMatKhau {
			t.Errorf("xã %s: tài khoản mặc định không bị bắt đổi mật khẩu", xa)
		}

		d := demPg(t, db, xa)
		if d.taiKhoan != 1 || d.vaiTro != 1 || d.vetGieo != 1 {
			t.Errorf("xã %s: sau lần đầu %+v, muốn 1 tài khoản, 1 vai trò, 1 vết gieo", xa, d)
		}
		if d.catalogue == 0 || d.oQuyen != d.catalogue {
			t.Errorf("xã %s: %d ô quyền, danh mục có %d khoá — phải cấp đủ", xa, d.oQuyen, d.catalogue)
		}

		var bam string
		var coTK, hoatDong, phaiDoi bool
		if err := db.QueryRow(
			`SELECT mat_khau_hash, co_tai_khoan, dang_hoat_dong, phai_doi_mat_khau
			   FROM nguoi_dung WHERE tenant_id = $1 AND email = 'admin'`, xa).
			Scan(&bam, &coTK, &hoatDong, &phaiDoi); err != nil {
			t.Fatalf("đọc lại tài khoản: %v", err)
		}
		if err := password.KiemTra(matKhauGieoGia, bam); err != nil || !coTK || !hoatDong || !phaiDoi {
			t.Errorf("xã %s: hash khớp=%v co_tai_khoan=%v dang_hoat_dong=%v phai_doi=%v",
				xa, err == nil, coTK, hoatDong, phaiDoi)
		}
		for _, v := range vetCuaXaPg(t, db, xa) {
			if strings.Contains(v, matKhauGieoGia) || strings.Contains(v, "$argon2id") {
				t.Errorf("xã %s: vết mang mật khẩu hoặc hash: %s", xa, v)
			}
		}

		// Second sign-in: the ordinary path, no second seed.
		if _, err := uc.Chay(ctx, yeuCauAdmin(matKhauGieoGia)); err != nil {
			t.Fatalf("xã %s: lần hai lỗi: %v", xa, err)
		}
		if d2 := demPg(t, db, xa); d2 != d {
			t.Errorf("xã %s: lần hai thay đổi dữ liệu gieo: %+v → %+v", xa, d, d2)
		}
	}
}

func TestPgGieoQuanTriHaiLanDauDongThoiChiMotTaiKhoan(t *testing.T) {
	db := moPoolNhieuKetNoi(t)
	uc := ucDangNhapGieoPg(t, db)
	xa := xaRiengPg(t)
	ctx := tenant.Into(context.Background(), tenant.ID(xa))

	const n = 3
	var wg sync.WaitGroup
	loi := make([]error, n)
	batDau := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-batDau
			_, loi[i] = uc.Chay(ctx, yeuCauAdmin(matKhauGieoGia))
		}(i)
	}
	close(batDau)
	wg.Wait()

	for i, err := range loi {
		if err != nil {
			t.Errorf("lượt %d lỗi: %v", i, err)
		}
	}
	d := demPg(t, db, xa)
	if d.taiKhoan != 1 || d.vaiTro != 1 || d.vetGieo != 1 {
		t.Errorf("sau %d lần đầu đồng thời: %+v — muốn đúng 1 tài khoản, 1 vai trò, 1 vết gieo", n, d)
	}
}

func TestPgGieoQuanTriKhongHoiSinhAdminDaXoaMem(t *testing.T) {
	db := moPool(t)
	uc := ucDangNhapGieoPg(t, db)
	xa := xaRiengPg(t)
	if _, err := db.Exec(
		`INSERT INTO nguoi_dung (tenant_id, id, ma, ho_ten, email, co_tai_khoan, mat_khau_hash,
		                         deleted_at, deleted_by, delete_reason)
		 VALUES ($1, 'nd-admin-cu', 'CB-2026-ADMCU1', 'Quản trị cũ', 'admin', true, $2,
		         now(), 'CB-2026-NGUOIX', 'thử')`, xa, bamGiaPg); err != nil {
		t.Fatalf("thêm admin đã xoá mềm: %v", err)
	}
	ctx := tenant.Into(context.Background(), tenant.ID(xa))
	if _, err := uc.Chay(ctx, yeuCauAdmin(matKhauGieoGia)); !errors.Is(err, ErrDangNhapThatBai) {
		t.Fatalf("muốn ErrDangNhapThatBai, nhận %v", err)
	}
	d := demPg(t, db, xa)
	if d.taiKhoan != 1 || d.vaiTro != 0 || d.oQuyen != 0 || d.vetGieo != 0 {
		t.Errorf("xã đã có admin xoá mềm mà vẫn gieo: %+v", d)
	}
	var bam string
	if err := db.QueryRow(`SELECT mat_khau_hash FROM nguoi_dung WHERE tenant_id = $1 AND email = 'admin'`, xa).
		Scan(&bam); err != nil || bam != bamGiaPg {
		t.Errorf("hash của admin cũ bị ghi đè (err=%v)", err)
	}
}
