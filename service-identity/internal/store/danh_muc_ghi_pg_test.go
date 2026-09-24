package store

import (
	"errors"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The DATABASE FLOOR under the catalogue write path, against a real PostgreSQL: the statements in
// danh_muc_ghi.go really run, the trigger `danh_muc_ba_tang` (migration 0005:157) really refuses what
// the use case refuses first, and dichLoiGhiDanhMuc turns those refusals into the same sentinels.
// The app-level refusals are proved without a database in app/danh_muc_ghi_test.go.
//
// Skipped unless VIGOV_TEST_DSN is set.

func TestPgDanhMucGhiVaSanTrigger(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	kho := pkgstore.New(db)
	s := NewKhoiNhiemVuStore(kho)
	ctx := ctxXa(xa)

	// A system row, seeded directly: no code path in this service writes `he-thong`.
	if _, err := db.Exec(`INSERT INTO khoi_nhiem_vu (tenant_id, id, ma, nhan, nguon, ma_nguon_re_nhanh)
		VALUES ($1, $2, 'khoi-uy-ban', 'Khối Uỷ ban', 'he-thong', true)`, xa, "knv-ht-"+xa); err != nil {
		t.Fatalf("gieo dòng hệ thống: %v", err)
	}

	err := kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		// Chen writes a tier-1 row whatever the value carries in Nguon.
		if err := s.Chen(ctx, tx, domain.KhoiNhiemVu{ID: "knv-1-" + xa, Ma: "khoi-mat-tran", Nhan: "Khối Mặt trận",
			DangDung: true, Nguon: domain.NguonHeThong, MaNguonReNhanh: true}); err != nil {
			return err
		}
		k, err := s.TheoIDDeSua(ctx, tx, "knv-1-"+xa)
		if err != nil {
			return err
		}
		if k.Tang() != domain.TangDonVi {
			t.Errorf("dòng xã thêm có tầng %d — nguon phải là HẰNG 'don-vi'", k.Tang())
		}
		if err := s.XoaMem(ctx, tx, k.ID, "CB-0001", "nhập nhầm"); err != nil {
			return err
		}
		// The code stays taken after the soft delete.
		if daDung, err := s.MaDaDung(ctx, tx, "khoi-mat-tran"); err != nil || !daDung {
			t.Errorf("mã của dòng đã xoá mềm phải còn bị chiếm: %v %v", daDung, err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("giao dịch tầng 1: %v", err)
	}

	// THE FLOOR: soft-deleting and disabling the tier-3 row are refused BY THE TRIGGER, reaching the
	// caller as the same sentinels the use case returns.
	err = kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.XoaMem(ctx, tx, "knv-ht-"+xa, "CB-0001", "thử")
	})
	if !errors.Is(err, domain.ErrKhongXoaDuocMucHeThong) {
		t.Errorf("xoá mềm dòng hệ thống: lỗi = %v, muốn ErrKhongXoaDuocMucHeThong từ trigger", err)
	}
	err = kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		k, err := s.TheoIDDeSua(ctx, tx, "knv-ht-"+xa)
		if err != nil {
			return err
		}
		k.DangDung = false
		return s.CapNhat(ctx, tx, k)
	})
	if !errors.Is(err, domain.ErrKhongTatDuocMucReNhanh) {
		t.Errorf("tắt dòng tầng 3: lỗi = %v, muốn ErrKhongTatDuocMucReNhanh từ trigger", err)
	}
}
