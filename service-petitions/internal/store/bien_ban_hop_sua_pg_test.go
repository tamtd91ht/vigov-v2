package store

import (
	"errors"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// Integration test for the lifecycle UPDATEs of bien_ban_hop_sua.go against migration 0012.
//
// ⚠ SKIPS UNLESS VIGOV_TEST_DSN IS SET — and it was NOT set where this was written, so this has never
// run. It is here because the statements' column lists, the COALESCE over NULL date parameters, and
// the P0001 translation are only provable against a real server.
func TestPgVongDoiBienBanSuaKyThongBao(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	if err := themBienBan(db, xa, "bb-vd", "Giao ban tháng 8", mocNgayHopPg); err != nil {
		t.Fatalf("thêm biên bản: %v", err)
	}
	s := NewBienBanHopStore(pkgstore.New(db))
	ctx := ctxXa(tenant.ID(xa))
	kho := pkgstore.New(db)

	// 1. edit the draft (secretary set, body cleared)
	err := kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		b, err := s.BienBanDayDuDeSua(ctx, tx, "bb-vd")
		if err != nil {
			return err
		}
		b.ThuKyMa, b.NoiDung = "CB-00042", ""
		return s.SuaBienBan(ctx, tx, b)
	})
	if err != nil {
		t.Fatalf("sửa bản nháp: %v", err)
	}

	// 2. sign, without a notice
	luc := time.Now().UTC().Truncate(time.Second)
	if err := kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.KyBienBan(ctx, tx, "bb-vd", luc, "CB-00031", "", time.Time{})
	}); err != nil {
		t.Fatalf("ký: %v", err)
	}

	// 3. signing again matches no row
	err = kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.KyBienBan(ctx, tx, "bb-vd", luc, "CB-00031", "", time.Time{})
	})
	if !errors.Is(err, domain.ErrBienBanDaKy) {
		t.Errorf("ký lần hai: %v, muốn ErrBienBanDaKy", err)
	}

	// 4. editing the signed record is refused BY THE TRIGGER when the guard is bypassed — and the
	//    refusal reads as the 409 sentinel. (A raw UPDATE, since SuaBienBan's WHERE would not match.)
	_, err = db.Exec(`UPDATE bien_ban_hop SET noi_dung = 'x' WHERE tenant_id = $1 AND id = $2`, xa, "bb-vd")
	if !errors.Is(boiTuChoiHoSoDaKy(err), domain.ErrBienBanDaKy) {
		t.Errorf("trigger không khoá / không dịch được: %v", err)
	}

	// 5. the notice, once
	ngay := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	for i, muon := range []error{nil, domain.ErrDaCoThongBao} {
		err := kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
			return s.GhiThongBao(ctx, tx, "bb-vd", "12/TB-UBND", ngay)
		})
		if !errors.Is(err, muon) && !(muon == nil && err == nil) {
			t.Errorf("lần ghi thông báo %d: %v, muốn %v", i+1, err, muon)
		}
	}

	b, err := s.TheoID(ctx, "bb-vd")
	if err != nil {
		t.Fatalf("đọc lại: %v", err)
	}
	if b.TrangThai != domain.TrangThaiBienBanDaKy || b.KyBoiMa != "CB-00031" || b.ThuKyMa != "CB-00042" ||
		b.TbSoKyHieu != "12/TB-UBND" || b.TbNgay.Format("2006-01-02") != "2026-08-12" {
		t.Errorf("biên bản sau vòng đời = %+v", b)
	}
}
