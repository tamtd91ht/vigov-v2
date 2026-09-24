package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/vihat/vigov/core/page"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// THE SOFT DELETE OF #10 AGAINST A REAL POSTGRESQL. Skipped unless VIGOV_TEST_DSN is set (see
// TestMain in danh_ba_can_bo_pg_test.go).
//
// What only a real server can decide, and why each one matters:
//
//	0010's CHECKs       the UPDATE clears the publication flag AND both consent marks as
//	                    literals; a statement that cleared one and not the others would be refused
//	                    by `nguoi_dung_rut_cong_khai_xoa_dong_y`, and only a server says so.
//	the read paths      list, detail and search hide the row; ResolveStaffNames still names it
//	                    (ADR 0034) — the four predicates live in four SQL strings.
//	the code            `UNIQUE (tenant_id, ma)` is not partial: the deleted row's code stays
//	                    taken, and the create path mints another rather than reusing it.

// chenDongDanhBaPg inserts one DIRECTORY-ONLY row (no account), optionally published with consent.
func chenDongDanhBaPg(t *testing.T, db *sql.DB, xa, id, ma string, congKhai bool) {
	t.Helper()
	var luc any
	ghiBoi := ""
	if congKhai {
		luc = time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
		ghiBoi = "CB-2026-GHI001"
	}
	if _, err := db.Exec(
		`INSERT INTO nguoi_dung (tenant_id, id, ma, ho_ten, email, di_dong_ca_nhan, co_tai_khoan, mat_khau_hash,
		                         hien_tren_mini_app, dong_y_cong_khai_luc, dong_y_cong_khai_ghi_boi)
		 VALUES ($1,$2,$3,$4,$5,$6,false,'',$7,$8,$9)`,
		xa, id, ma, "Trần Thị B", id+"@xa.danang.gov.vn", "0900000000", congKhai, luc, ghiBoi); err != nil {
		t.Fatalf("thêm dòng danh bạ: %v", err)
	}
}

func TestPgXoaDongTrungGoCongKhaiAnKhoiDocVanTraTenVaGhiVet(t *testing.T) {
	db := moPool(t)
	xa := xaRiengPg(t)
	ctx := tenant.Into(context.Background(), tenant.ID(xa))
	nguoi := nguoiPg("nd-quan-tri")
	const id, ma = "nd-trung-01", "CB-2026-TRUNG1"

	chenDongDanhBaPg(t, db, xa, id, ma, true)

	if err := ucThat(db).Xoa(ctx, id, "  Nhập trùng do Excel  ", nguoi); err != nil {
		t.Fatalf("Xoa: %v", err)
	}

	// The row, read straight off the table: deleted with who/why, and OFF the Mini App.
	var (
		daXoa         bool
		xoaBoi, lyDo  string
		hien          bool
		dongYLuc      sql.NullTime
		ghiBoi, hoTen string
		maSau         string
	)
	if err := db.QueryRow(
		`SELECT deleted_at IS NOT NULL, deleted_by, delete_reason,
		        hien_tren_mini_app, dong_y_cong_khai_luc, dong_y_cong_khai_ghi_boi, ho_ten, ma
		   FROM nguoi_dung WHERE tenant_id=$1 AND id=$2`, xa, id).
		Scan(&daXoa, &xoaBoi, &lyDo, &hien, &dongYLuc, &ghiBoi, &hoTen, &maSau); err != nil {
		t.Fatalf("đọc lại dòng: %v", err)
	}
	if !daXoa || xoaBoi != nguoi.Vet.ID || lyDo != "Nhập trùng do Excel" {
		t.Errorf("xoá mềm sai: da_xoa=%v deleted_by=%q delete_reason=%q", daXoa, xoaBoi, lyDo)
	}
	if xoaBoi == nguoi.ID {
		t.Error("deleted_by mang ĐỊNH DANH NỘI BỘ (luật 6 bất biến 8)")
	}
	if hien || dongYLuc.Valid || ghiBoi != "" {
		t.Errorf("dòng đã xoá vẫn còn công khai hoặc còn dấu đồng ý: hien=%v luc=%v ghi_boi=%q", hien, dongYLuc, ghiBoi)
	}
	if maSau != ma || hoTen != "Trần Thị B" {
		t.Errorf("xoá mềm đã chạm mã/họ tên: ma=%q", maSau)
	}

	kho := idstore.NewCanBoStore(pkgstore.New(db))

	// Detail: 404-equivalent.
	if _, err := kho.ChiTiet(ctx, id); !errors.Is(err, idstore.ErrCanBoKhongTonTai) {
		t.Errorf("chi tiết dòng đã xoá: lỗi = %v, muốn ErrCanBoKhongTonTai", err)
	}
	// List and search: absent.
	yc, err := page.Parse(url.Values{}, idstore.SapXepCanBo)
	if err != nil {
		t.Fatal(err)
	}
	for ten, loc := range map[string]domain.LocCanBo{
		"danh sách": {},
		"tìm kiếm":  {TuKhoa: "Trần Thị B"},
	} {
		kq, err := kho.DanhSach(ctx, loc, yc)
		if err != nil {
			t.Fatalf("%s: %v", ten, err)
		}
		for _, cb := range kq.Items {
			if cb.ID == id {
				t.Errorf("%s vẫn trả dòng đã xoá mềm", ten)
			}
		}
	}
	// ResolveStaffNames: still named, flagged as no longer in the directory (#10, ADR 0034).
	ten, err := kho.TenTheoNhieuMa(ctx, []string{ma})
	if err != nil {
		t.Fatal(err)
	}
	if len(ten) != 1 || ten[0].HoTen != "Trần Thị B" || ten[0].ConTrongDanhBa {
		t.Errorf("tên của dòng đã xoá không còn tra được đúng: %+v", ten)
	}

	// The trail: one entry, the staff code as actor.
	var actor, hanhVi string
	if err := db.QueryRow(`SELECT actor_id, action FROM audit_log WHERE tenant_id=$1 AND subject=$2`,
		xa, ma).Scan(&actor, &hanhVi); err != nil {
		t.Fatalf("không đọc được vết: %v", err)
	}
	if actor != nguoi.Vet.ID || hanhVi != HanhViXoaCanBoNhapTrung {
		t.Errorf("vết: actor=%q action=%q", actor, hanhVi)
	}

	// A second delete is "not found" and cannot overwrite who removed it.
	if err := ucThat(db).Xoa(ctx, id, "lý do khác", nguoiPg("nd-nguoi-khac")); !errors.Is(err, idstore.ErrCanBoKhongTonTai) {
		t.Errorf("xoá lần hai: lỗi = %v, muốn ErrCanBoKhongTonTai", err)
	}
	var xoaBoiSau string
	if err := db.QueryRow(`SELECT deleted_by FROM nguoi_dung WHERE tenant_id=$1 AND id=$2`, xa, id).
		Scan(&xoaBoiSau); err != nil {
		t.Fatal(err)
	}
	if xoaBoiSau != nguoi.Vet.ID {
		t.Errorf("lần xoá thứ hai đã ghi đè deleted_by: %q", xoaBoiSau)
	}
}

// THE DELETED ROW'S CODE IS NEVER HANDED TO A NEW PERSON (rule 7, invariant 3), through the real
// delete path rather than a hand-written `deleted_at`.
func TestPgMaCuaDongTrungDaXoaKhongCapLai(t *testing.T) {
	db := moPool(t)
	xa := xaRiengPg(t)
	ctx := tenant.Into(context.Background(), tenant.ID(xa))
	const id, ma = "nd-trung-02", "CB-2026-TRUNG2"

	chenDongDanhBaPg(t, db, xa, id, ma, false)
	uc := ucThat(db)
	if err := uc.Xoa(ctx, id, "Nhập trùng", nguoiPg("nd-quan-tri")); err != nil {
		t.Fatalf("Xoa: %v", err)
	}

	lan := 0
	uc.sinhMa = func(time.Time) (string, error) {
		lan++
		if lan == 1 {
			return ma, nil // the generator offers the deleted code first
		}
		return fmt.Sprintf("CB-2026-MOI%03d", lan), nil
	}
	cb, err := uc.Them(ctx, YeuCauThemCanBo{HoTen: "Lê Văn C", Email: "moi.c@xa.danang.gov.vn"}, nguoiPg("nd-quan-tri"))
	if err != nil {
		t.Fatalf("Them: %v", err)
	}
	if cb.Ma == ma {
		t.Fatal("MÃ CỦA DÒNG ĐÃ XOÁ MỀM ĐƯỢC CẤP LẠI — luật 7 bất biến 3")
	}
}

// A ROW WITH AN ACCOUNT IS REFUSED AND STAYS EXACTLY AS IT WAS.
func TestPgXoaDongCoTaiKhoanBiTuChoiVaDongGiuNguyen(t *testing.T) {
	db := moPool(t)
	xa := xaRiengPg(t)
	ctx := tenant.Into(context.Background(), tenant.ID(xa))
	dungXaPg(t, db, xa, "vt-thuong", []string{"document.read"}, "nd-co-tk")

	if err := ucThat(db).Xoa(ctx, "nd-co-tk", "Nhập trùng", nguoiPg("nd-quan-tri")); !errors.Is(err, ErrCanBoCoTaiKhoan) {
		t.Fatalf("lỗi = %v, muốn ErrCanBoCoTaiKhoan", err)
	}
	var daXoa bool
	if err := db.QueryRow(`SELECT deleted_at IS NOT NULL FROM nguoi_dung WHERE tenant_id=$1 AND id=$2`,
		xa, "nd-co-tk").Scan(&daXoa); err != nil {
		t.Fatal(err)
	}
	if daXoa {
		t.Error("dòng có tài khoản đã bị xoá mềm")
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM audit_log WHERE tenant_id=$1`, xa).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("thao tác bị từ chối vẫn để lại %d vết", n)
	}
}
