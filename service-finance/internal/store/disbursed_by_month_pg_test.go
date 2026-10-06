package store

// Integration test for the month buckets behind §4's year curve and §8.3's project curve, against a
// real PostgreSQL.
//
// SKIPPED UNLESS VIGOV_TEST_DSN IS SET — a green run without it means the code COMPILES, nothing more.

import (
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// Buckets by month of ngay_chi; a payment before 01/01 lands in January and one after 31/12 in
// AfterYear; soft-deleted vouchers, other years' projects and another commune's colliding project are
// all excluded; the project form reads one project only.
func TestPgDisbursedByMonth(t *testing.T) {
	db := moKetNoi(t)
	commune, otherCommune := xaRieng(t)

	themDuAnToiThieu(t, db, commune, "da-1", "DA01", 2026, 100_000_000)
	themDuAnToiThieu(t, db, commune, "da-2", "DA02", 2026, 100_000_000)
	themDuAnToiThieu(t, db, commune, "da-old", "DA00", 2025, 100_000_000)
	themDuAnToiThieu(t, db, otherCommune, "da-1", "DA01", 2026, 100_000_000)

	insert := func(tenantID, id, project, date string, amount int64, deleted bool) {
		t.Helper()
		_, err := db.Exec(`INSERT INTO chung_tu_giai_ngan
			   (tenant_id, id, du_an_id, ngay_chi, so_tien, noi_dung, nguoi_nhap_id)
			 VALUES ($1,$2,$3,$4::date,$5,'Thanh toán','CB-TEST')`, tenantID, id, project, date, amount)
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
	insert(commune, "ct-advance", "da-1", "2025-12-20", 1, false)
	insert(commune, "ct-jan", "da-1", "2026-01-31", 10, false)
	insert(commune, "ct-mar-1", "da-1", "2026-03-01", 100, false)
	insert(commune, "ct-mar-2", "da-2", "2026-03-31", 1000, false)
	insert(commune, "ct-dec", "da-2", "2026-12-31", 10000, false)
	insert(commune, "ct-next", "da-2", "2027-01-15", 100000, false)
	insert(commune, "ct-gone", "da-1", "2026-05-01", 7, true)
	insert(commune, "ct-old", "da-old", "2026-04-01", 9, false)
	insert(otherCommune, "ct-other", "da-1", "2026-03-01", 5, false)

	s := NewDuAnStore(pkgstore.New(db))
	ctx := ctxXa(tenant.ID(commune))

	all, err := s.DisbursedByMonth(ctx, 2026, "")
	if err != nil {
		t.Fatalf("DisbursedByMonth: %v", err)
	}
	if all.Months[0] != 11 || all.Months[2] != 1100 || all.Months[11] != 10000 || all.AfterYear != 100000 {
		t.Fatalf("commune buckets = %+v", all)
	}
	if all.Months[3] != 0 || all.Months[4] != 0 {
		t.Fatalf("other year's project or soft-deleted voucher leaked in: %+v", all)
	}

	one, err := s.DisbursedByMonth(ctx, 2026, "da-1")
	if err != nil {
		t.Fatalf("DisbursedByMonth(da-1): %v", err)
	}
	if one.Months[0] != 11 || one.Months[2] != 100 || one.Months[11] != 0 || one.AfterYear != 0 {
		t.Fatalf("project buckets = %+v (the other commune's colliding da-1 must not appear)", one)
	}
}
