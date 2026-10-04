package store

import (
	"context"
	"errors"
	"testing"

	"github.com/vihat/vigov/core/page"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// map_asset against a REAL PostgreSQL — the half the fake driver in map_asset_test.go cannot see:
// migration 0015's columns as the store names them, the type foreign key, the generated live_tax_code
// key, the verified CHECK, the guard trigger, and the soft-delete predicate actually evaluated. SKIPPED
// WITHOUT VIGOV_TEST_DSN (shared harness: loai_tai_nguyen_ban_do_pg_test.go) — a green run without the
// DSN means this file COMPILES, nothing more.

func TestPgMapAssetRegisterEndToEnd(t *testing.T) {
	db := moKetNoi(t)
	c1, c2 := xaRieng(t)
	themLoai(t, db, c1, "t-dn", "doanh-nghiep", "Doanh nghiệp", 1, false, true)
	themLoai(t, db, c1, "t-cho", "cho", "Chợ", 2, false, true)
	themLoai(t, db, c2, "t-dn2", "doanh-nghiep", "Doanh nghiệp", 1, false, true)

	h := pkgstore.New(db)
	s := NewMapAssetStore(h)
	ctx := ctxXa(tenant.ID(c1))
	run := func(fn func(tx *pkgstore.ScopedTx) error) error { return h.For(ctx).Tx(ctx, fn) }

	n := 3
	insert := func(id, typ, name, tax string) error {
		return run(func(tx *pkgstore.ScopedTx) error {
			ok, err := s.AssetTypeAvailable(ctx, tx, typ)
			if err != nil || !ok {
				t.Fatalf("type %s available=%v err=%v", typ, ok, err)
			}
			_, err = s.Insert(ctx, tx, domain.MapAsset{ID: id, AssetTypeCode: typ, Name: name, Lat: 15.730507,
				Lng: 108.37811, Status: domain.MapAssetStatusActive, TaxCode: tax, EmployeeCount: &n,
				EstablishedOn: "2020-01-31", Phone: "0900000000"}, "CB-00123")
			return err
		})
	}
	if err := insert("a1", "doanh-nghiep", "Công ty A", "0101234567"); err != nil {
		t.Fatalf("insert a1: %v", err)
	}
	if err := insert("a2", "cho", "Chợ Bình Trị", ""); err != nil {
		t.Fatalf("insert a2: %v", err)
	}
	if err := insert("a3", "doanh-nghiep", "Công ty B", "0101234567"); !errors.Is(err, ErrMapAssetTaxCodeTaken) {
		t.Fatalf("duplicate live tax code: %v, want ErrMapAssetTaxCodeTaken", err)
	}

	// Another commune's row with the same tax code is NOT a conflict, and never visible here.
	ctx2 := ctxXa(tenant.ID(c2))
	if err := h.For(ctx2).Tx(ctx2, func(tx *pkgstore.ScopedTx) error {
		_, err := s.Insert(ctx2, tx, domain.MapAsset{ID: "b1", AssetTypeCode: "doanh-nghiep", Name: "Công ty xã B",
			Lat: 1, Lng: 2, Status: domain.MapAssetStatusActive, TaxCode: "0101234567"}, "CB-00999")
		return err
	}); err != nil {
		t.Fatalf("insert in commune 2: %v", err)
	}

	pts, err := s.Points(ctx, domain.MapAssetFilter{})
	if err != nil || len(pts) != 2 {
		t.Fatalf("points = %d, %v", len(pts), err)
	}
	if pts[0].Lat != 15.730507 || pts[0].Lng != 108.37811 {
		t.Errorf("coordinates round-trip = %v,%v", pts[0].Lat, pts[0].Lng)
	}

	// Verify, then soft delete a1: it leaves points, list, summary — and its tax code is free again.
	if err := run(func(tx *pkgstore.ScopedTx) error {
		a, err := s.ByIDForUpdate(ctx, tx, "a1")
		if err != nil {
			return err
		}
		_, err = s.SetConfirmation(ctx, tx, a, true, "CB-00123")
		return err
	}); err != nil {
		t.Fatalf("verify: %v", err)
	}
	sum, err := s.Summary(ctx)
	if err != nil || sum.Total != 2 || sum.Verified != 1 {
		t.Fatalf("summary = %+v, %v", sum, err)
	}
	if err := run(func(tx *pkgstore.ScopedTx) error { return s.SoftDelete(ctx, tx, "a1", "CB-00123", "trùng") }); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	pts, _ = s.Points(ctx, domain.MapAssetFilter{})
	req, _ := page.New(MapAssetListSort, "", "", "", "")
	list, _ := s.List(ctx, domain.MapAssetFilter{}, req)
	sum, _ = s.Summary(ctx)
	if len(pts) != 1 || len(list.Items) != 1 || sum.Total != 1 || sum.Verified != 0 {
		t.Errorf("after delete: points=%d list=%d summary=%+v", len(pts), len(list.Items), sum)
	}
	if _, err := s.ByID(ctx, "a1"); !errors.Is(err, ErrMapAssetNotFound) {
		t.Errorf("deleted asset still readable: %v", err)
	}
	if err := insert("a4", "doanh-nghiep", "Công ty A (nhập lại)", "0101234567"); err != nil {
		t.Errorf("the tax code of a deleted asset did not free: %v", err)
	}

	// The guard trigger: no edit of a deleted row, no hard delete.
	if _, err := db.ExecContext(context.Background(),
		`UPDATE map_asset SET name = 'sửa lén' WHERE tenant_id = $1 AND id = 'a1'`, c1); err == nil {
		t.Error("the trigger let a deleted row be edited")
	}
	if _, err := db.ExecContext(context.Background(),
		`DELETE FROM map_asset WHERE tenant_id = $1 AND id = 'a2'`, c1); err == nil {
		t.Error("the trigger let a hard DELETE through")
	}
}

// Another commune's id, named from THIS commune, is not-found on every path that takes an id — the read,
// the locking read and the three writes. The predicate is `tenant_id = $1` from the context's commune,
// so a guessed or leaked id reads nothing and changes nothing (rule 1, invariant 5). Afterwards the
// other commune's row is checked untouched from its own commune: a write that matched zero rows and still
// reported success would leave it changed while this test stayed green on the error alone.
func TestPgMapAssetAnotherCommunesIDIsNotFound(t *testing.T) {
	db := moKetNoi(t)
	c1, c2 := xaRieng(t)
	themLoai(t, db, c1, "t-dn", "doanh-nghiep", "Doanh nghiệp", 1, false, true)
	themLoai(t, db, c2, "t-dn2", "doanh-nghiep", "Doanh nghiệp", 1, false, true)

	h := pkgstore.New(db)
	s := NewMapAssetStore(h)
	ctx1, ctx2 := ctxXa(tenant.ID(c1)), ctxXa(tenant.ID(c2))
	if err := h.For(ctx2).Tx(ctx2, func(tx *pkgstore.ScopedTx) error {
		_, err := s.Insert(ctx2, tx, domain.MapAsset{ID: "b1", AssetTypeCode: "doanh-nghiep", Name: "Công ty xã B",
			Lat: 1, Lng: 2, Status: domain.MapAssetStatusActive, Phone: "0900000000"}, "CB-00999")
		return err
	}); err != nil {
		t.Fatalf("insert in commune 2: %v", err)
	}
	theirs, err := s.ByID(ctx2, "b1")
	if err != nil {
		t.Fatalf("commune 2 reads its own row: %v", err)
	}

	if _, err := s.ByID(ctx1, "b1"); !errors.Is(err, ErrMapAssetNotFound) {
		t.Errorf("ByID across communes: %v, want ErrMapAssetNotFound", err)
	}
	inTx := func(name string, fn func(tx *pkgstore.ScopedTx) error) {
		t.Helper()
		var got error
		if err := h.For(ctx1).Tx(ctx1, func(tx *pkgstore.ScopedTx) error {
			got = fn(tx)
			return nil
		}); err != nil {
			t.Fatalf("%s: tx: %v", name, err)
		}
		if !errors.Is(got, ErrMapAssetNotFound) {
			t.Errorf("%s across communes: %v, want ErrMapAssetNotFound", name, got)
		}
	}
	inTx("ByIDForUpdate", func(tx *pkgstore.ScopedTx) error {
		_, err := s.ByIDForUpdate(ctx1, tx, "b1")
		return err
	})
	inTx("Update", func(tx *pkgstore.ScopedTx) error {
		edited := theirs
		edited.Name = "sửa từ xã khác"
		_, err := s.Update(ctx1, tx, edited, "CB-00123")
		return err
	})
	inTx("SetConfirmation", func(tx *pkgstore.ScopedTx) error {
		_, err := s.SetConfirmation(ctx1, tx, theirs, true, "CB-00123")
		return err
	})
	inTx("SoftDelete", func(tx *pkgstore.ScopedTx) error {
		return s.SoftDelete(ctx1, tx, "b1", "CB-00123", "xoá từ xã khác")
	})

	after, err := s.ByID(ctx2, "b1")
	if err != nil {
		t.Fatalf("commune 2's row gone after commune 1's attempts: %v", err)
	}
	if after.Name != theirs.Name || after.Verified || !after.UpdatedAt.Equal(theirs.UpdatedAt) {
		t.Errorf("commune 2's row changed from commune 1: %+v", after)
	}
}

func TestPgMapAssetTypeSeedRowsAreTierTwo(t *testing.T) {
	db := moKetNoi(t)
	c1, _ := xaRieng(t)
	h := pkgstore.New(db)
	cat := NewLoaiTaiNguyenBanDoStore(h)
	ctx := ctxXa(tenant.ID(c1))
	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		if err := cat.LockCatalogueForSeed(ctx, tx); err != nil {
			return err
		}
		return cat.InsertSystemRow(ctx, tx, "seed-1", domain.MapAssetTypeDefault{Code: "ocop", Label: "Sản phẩm OCOP", Order: 9})
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var nguon string
	if err := db.QueryRow(`SELECT nguon FROM loai_tai_nguyen_ban_do WHERE tenant_id = $1 AND ma = 'ocop'`, c1).Scan(&nguon); err != nil || nguon != "he-thong" {
		t.Fatalf("nguon = %q, %v", nguon, err)
	}
	// Tier 2: the tier trigger refuses a soft delete of a system row.
	if _, err := db.Exec(`UPDATE loai_tai_nguyen_ban_do SET deleted_at = now(), deleted_by = 'CB-1', delete_reason = 'x'
		WHERE tenant_id = $1 AND ma = 'ocop'`, c1); err == nil {
		t.Error("a seeded system row was soft deleted")
	}
	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		states, err := cat.CodeStates(ctx, tx, []string{"ocop", "cho"})
		if err != nil {
			return err
		}
		if d, ok := states["ocop"]; !ok || d {
			t.Errorf("states = %v", states)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
