package app

import (
	"context"
	"errors"
	"testing"

	"github.com/vihat/vigov/core/tenant"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// DatCongKhai against a REAL PostgreSQL — the one thing the fake driver cannot say: that the UPDATE
// the use case writes is accepted by the CHECKs of migration 0010 §3 in BOTH directions, and that
// it reads back through the same column list the screens use. A publish that forgot the recorder,
// or an unpublish that left the marks, is refused HERE and nowhere else.
//
// Skipped unless VIGOV_TEST_DSN is set (rule 8). Harness: danh_ba_can_bo_pg_test.go.
func TestPgCongKhaiRoiRutCongKhaiQuaRangBuocThat(t *testing.T) {
	db := moPool(t)
	xa := xaRiengPg(t)
	quanTri, dich := "nd-ck-quantri", "nd-ck-dich"
	dungXaPg(t, db, xa, "vt-ck", []string{"admin.user"}, quanTri, dich)

	ctx := tenant.Into(context.Background(), tenant.ID(xa))
	uc := ucThat(db)
	kho := idstore.NewCanBoStore(uc.db)
	ba := 3

	if _, err := uc.DatCongKhai(ctx, dich,
		YeuCauCongKhai{CongKhai: true, DaXacNhanDongY: true, ThuTu: &ba}, nguoiPg(quanTri)); err != nil {
		t.Fatalf("CÔNG KHAI CÓ XÁC NHẬN ĐỒNG Ý BỊ TỪ CHỐI: %v", err)
	}
	cb, err := kho.ChiTiet(ctx, dich)
	if err != nil {
		t.Fatalf("đọc lại: %v", err)
	}
	if !cb.HienTrenMiniApp || cb.DongYCongKhaiLuc == nil || cb.DongYCongKhaiGhiBoi != "CB-2026-"+quanTri ||
		cb.ThuTuDanhBa == nil || *cb.ThuTuDanhBa != 3 {
		t.Fatalf("đọc lại sai: hien=%v luc=%v ghi_boi=%q thu_tu=%v",
			cb.HienTrenMiniApp, cb.DongYCongKhaiLuc, cb.DongYCongKhaiGhiBoi, cb.ThuTuDanhBa)
	}

	var soVet int
	if err := db.QueryRow(`SELECT count(*) FROM audit_log
	                        WHERE tenant_id = $1 AND action = $2 AND actor_id = $3 AND subject = $4`,
		xa, HanhViCongKhaiMiniApp, "CB-2026-"+quanTri, "CB-2026-"+dich).Scan(&soVet); err != nil {
		t.Fatalf("đếm vết: %v", err)
	}
	if soVet != 1 {
		t.Fatalf("có %d vết công khai, muốn 1", soVet)
	}

	if _, err := uc.DatCongKhai(ctx, dich, YeuCauCongKhai{CongKhai: false}, nguoiPg(quanTri)); err != nil {
		t.Fatalf("THÔI CÔNG KHAI BỊ TỪ CHỐI (dấu đồng ý chưa được xoá cùng câu?): %v", err)
	}
	cb, err = kho.ChiTiet(ctx, dich)
	if err != nil {
		t.Fatalf("đọc lại: %v", err)
	}
	if cb.HienTrenMiniApp || cb.DongYCongKhaiLuc != nil || cb.DongYCongKhaiGhiBoi != "" || cb.ThuTuDanhBa != nil {
		t.Fatalf("thôi công khai mà còn dấu: hien=%v luc=%v ghi_boi=%q thu_tu=%v",
			cb.HienTrenMiniApp, cb.DongYCongKhaiLuc, cb.DongYCongKhaiGhiBoi, cb.ThuTuDanhBa)
	}

	// ANOTHER COMMUNE'S CONTEXT CANNOT REACH THE ROW — one answer with an invented id.
	khac := tenant.Into(context.Background(), tenant.ID(xaRiengPg(t)))
	if _, err := uc.DatCongKhai(khac, dich,
		YeuCauCongKhai{CongKhai: true, DaXacNhanDongY: true}, nguoiPg(quanTri)); !errors.Is(err, idstore.ErrCanBoKhongTonTai) {
		t.Fatalf("xã khác công khai được người của xã này: %v", err)
	}
}
