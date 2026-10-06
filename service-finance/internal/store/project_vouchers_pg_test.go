package store

// Integration tests for the project's voucher list and the voucher write path's share lock
// (decision 06/10/2026), against a real PostgreSQL.
//
// SKIPPED UNLESS VIGOV_TEST_DSN IS SET — a green run without it means the code COMPILES, nothing more.

import (
	"context"
	"errors"
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// Newest payment date first, ties by newest id; soft-deleted excluded; the source NAME carried; another
// commune's voucher on a colliding project id never appears; an unknown project is ErrKhongThayDuAn.
func TestPgVouchersOfProject(t *testing.T) {
	db := moKetNoi(t)
	commune, otherCommune := xaRieng(t)

	themNguonVon(t, db, commune, "nv-xa", "Ngân sách xã", 1)
	themDuAnToiThieu(t, db, commune, "da-1", "DA01", 2026, 100_000_000)
	themDuAnToiThieu(t, db, otherCommune, "da-1", "DA01", 2026, 100_000_000)

	insert := func(tenantID, id, date string, source any, deleted bool) {
		t.Helper()
		_, err := db.Exec(`INSERT INTO chung_tu_giai_ngan
			   (tenant_id, id, du_an_id, ngay_chi, so_tien, noi_dung, nguoi_nhap_id, nguon_von_id)
			 VALUES ($1,$2,'da-1',$3::date,1000,'Thanh toán','CB-TEST',$4)`, tenantID, id, date, source)
		if err != nil {
			t.Fatalf("insert voucher %q: %v", id, err)
		}
		if deleted {
			if _, err := db.Exec(`UPDATE chung_tu_giai_ngan SET deleted_at = now(), deleted_by = 'CB-TEST',
				delete_reason = 'thử' WHERE tenant_id = $1 AND id = $2`, tenantID, id); err != nil {
				t.Fatalf("soft delete %q: %v", id, err)
			}
		}
	}
	insert(commune, "ct-a", "2026-03-01", "nv-xa", false)
	insert(commune, "ct-b", "2026-05-01", nil, false)
	insert(commune, "ct-c", "2026-05-01", nil, false) // same day as ct-b, larger id -> first
	insert(commune, "ct-gone", "2026-06-01", nil, true)
	insert(otherCommune, "ct-other", "2026-07-01", nil, false)

	s := NewDuAnStore(pkgstore.New(db))
	ctx := ctxXa(tenant.ID(commune))

	got, err := s.VouchersOfProject(ctx, "da-1")
	if err != nil {
		t.Fatalf("VouchersOfProject: %v", err)
	}
	var ids []string
	for _, v := range got {
		ids = append(ids, v.Voucher.ID)
	}
	if strings.Join(ids, ",") != "ct-c,ct-b,ct-a" {
		t.Fatalf("order = %v, want ct-c,ct-b,ct-a (soft-deleted and other commune excluded)", ids)
	}
	if got[2].FundingSourceName != "Ngân sách xã" || got[0].FundingSourceName != "" {
		t.Fatalf("source names = %q / %q", got[2].FundingSourceName, got[0].FundingSourceName)
	}

	if _, err := s.VouchersOfProject(ctx, "da-unknown"); !errors.Is(err, ErrKhongThayDuAn) {
		t.Fatalf("unknown project: err = %v, want ErrKhongThayDuAn", err)
	}
}

// ProjectForVoucherWrite returns the allocated sources and HOLDS the project row: an allocation edit's
// FOR UPDATE cannot be taken while it is held (the race of 8245698b).
func TestPgProjectForVoucherWriteSharesProjectLock(t *testing.T) {
	db := moKetNoi(t)
	commune, _ := xaRieng(t)
	themNguonVon(t, db, commune, "nv-xa", "Ngân sách xã", 1)
	themDuAnToiThieu(t, db, commune, "da-1", "DA01", 2026, 100_000_000)
	themPhanBo(t, db, commune, "pb-xa", "da-1", "nv-xa", 40_000_000)

	scoped := pkgstore.New(db)
	s := NewChungTuGiaiNganStore(scoped)
	ctx := ctxXa(tenant.ID(commune))

	err := scoped.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		code, allocated, err := s.ProjectForVoucherWrite(ctx, tx, "da-1")
		if err != nil {
			return err
		}
		if code != "DA01" || len(allocated) != 1 || allocated[0] != "nv-xa" {
			t.Errorf("code %q, allocated %v", code, allocated)
		}
		// A second connection trying the edit path's lock must be refused while this one is held.
		_, lockErr := db.ExecContext(context.Background(),
			`SELECT 1 FROM du_an WHERE tenant_id = $1 AND id = 'da-1' FOR UPDATE NOWAIT`, commune)
		if lockErr == nil {
			t.Error("FOR UPDATE succeeded while the voucher path held the project FOR SHARE — the race is open")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("tx: %v", err)
	}

	if err := scoped.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		_, _, err := s.ProjectForVoucherWrite(ctx, tx, "da-unknown")
		return err
	}); !errors.Is(err, ErrKhongThayDuAnCuaChungTu) {
		t.Fatalf("unknown project: err = %v, want ErrKhongThayDuAnCuaChungTu", err)
	}
}
