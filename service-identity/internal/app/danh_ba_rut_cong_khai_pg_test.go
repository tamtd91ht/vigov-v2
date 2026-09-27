package app

import (
	"context"
	"database/sql"
	"testing"

	"github.com/vihat/vigov/core/tenant"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The automatic unpublish (owner decision 2026-09-28, #12) against a REAL PostgreSQL. What the fake
// driver cannot say: that the profile UPDATE and the unpublish UPDATE, one after the other on the
// same row in one transaction, both pass migration 0010 §3's CHECKs; and that the PUBLIC read
// (store.DanhBaCongKhai — the query behind GET /api/v1/commune-staff) stops returning the person.
//
// Skipped unless VIGOV_TEST_DSN is set (rule 8). Harness: danh_ba_can_bo_pg_test.go.
func TestPgDoiDiDongVaKhoaThiRoiKhoiDanhBaCongKhai(t *testing.T) {
	db := moPool(t)
	xa := xaRiengPg(t)
	quanTri, dich := "nd-rt-quantri", "nd-rt-dich"
	dungXaPg(t, db, xa, "vt-rt", []string{"admin.user"}, quanTri, dich)

	ctx := tenant.Into(context.Background(), tenant.ID(xa))
	uc := ucThat(db)
	kho := idstore.NewCanBoStore(uc.db)

	congKhai := func() {
		t.Helper()
		if _, err := uc.DatCongKhai(ctx, dich,
			YeuCauCongKhai{CongKhai: true, DaXacNhanDongY: true}, nguoiPg(quanTri)); err != nil {
			t.Fatalf("công khai: %v", err)
		}
		if n := demCongKhai(t, kho, ctx); n != 1 {
			t.Fatalf("vừa công khai mà danh bạ công khai có %d người, muốn 1", n)
		}
	}
	chuaCongKhai := func(buoc string) {
		t.Helper()
		cb, err := kho.ChiTiet(ctx, dich)
		if err != nil {
			t.Fatalf("%s: đọc lại: %v", buoc, err)
		}
		if cb.HienTrenMiniApp || cb.DongYCongKhaiLuc != nil || cb.DongYCongKhaiGhiBoi != "" {
			t.Fatalf("%s: còn công khai hoặc còn dấu: hien=%v luc=%v ghi_boi=%q",
				buoc, cb.HienTrenMiniApp, cb.DongYCongKhaiLuc, cb.DongYCongKhaiGhiBoi)
		}
		if n := demCongKhai(t, kho, ctx); n != 0 {
			t.Fatalf("%s: danh bạ công khai vẫn trả %d người", buoc, n)
		}
	}

	// 1. PATCH the mobile of a published person.
	congKhai()
	moi := "0900000000"
	if _, err := uc.Sua(ctx, dich, YeuCauSuaCanBo{DiDongCaNhan: &moi}, nguoiPg(quanTri)); err != nil {
		t.Fatalf("SỬA SỐ DI ĐỘNG BỊ TỪ CHỐI (hai câu UPDATE cùng dòng qua CHECK 0010 §3?): %v", err)
	}
	chuaCongKhai("sau khi đổi số di động")
	if n := demVet(t, db, xa, HanhViSuaCanBo, dich); n != 1 {
		t.Fatalf("có %d vết sửa hồ sơ, muốn 1", n)
	}
	if n := demVet(t, db, xa, HanhViRutCongKhaiMiniApp, dich); n != 1 {
		t.Fatalf("có %d vết rút công khai, muốn 1", n)
	}

	// 2. Lock a published person, then unlock: still unpublished.
	congKhai()
	if _, err := uc.DatKhoa(ctx, dich, true, nguoiPg(quanTri)); err != nil {
		t.Fatalf("khoá: %v", err)
	}
	chuaCongKhai("sau khi khoá")
	if _, err := uc.DatKhoa(ctx, dich, false, nguoiPg(quanTri)); err != nil {
		t.Fatalf("mở khoá: %v", err)
	}
	chuaCongKhai("sau khi mở khoá")
	if n := demVet(t, db, xa, HanhViRutCongKhaiMiniApp, dich); n != 2 {
		t.Fatalf("có %d vết rút công khai, muốn 2 (đổi số + khoá; mở khoá không thêm)", n)
	}
}

func demCongKhai(t *testing.T, kho *idstore.CanBoStore, ctx context.Context) int {
	t.Helper()
	ds, err := kho.DanhBaCongKhai(ctx)
	if err != nil {
		t.Fatalf("đọc danh bạ công khai: %v", err)
	}
	return len(ds)
}

func demVet(t *testing.T, db *sql.DB, xa, hanhVi, dich string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM audit_log
	                        WHERE tenant_id = $1 AND action = $2 AND subject = $3`,
		xa, hanhVi, "CB-2026-"+dich).Scan(&n); err != nil {
		t.Fatalf("đếm vết: %v", err)
	}
	return n
}
