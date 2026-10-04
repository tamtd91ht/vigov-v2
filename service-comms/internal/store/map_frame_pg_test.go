package store

import (
	"errors"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// map_frame against a REAL PostgreSQL — the half internal/app/map_frame_test.go's fake driver cannot
// see: migration 0016's columns as the store names them, numeric(10,6)/numeric(4,1) round-tripping
// through float64, the ON CONFLICT upsert, the guard trigger keeping created_by, and the two named
// CHECKs refusing what the domain would have refused first. SKIPPED WITHOUT VIGOV_TEST_DSN (shared
// harness: loai_tai_nguyen_ban_do_pg_test.go) — a green run without the DSN means this file COMPILES,
// nothing more.

func TestPgMapFrameUpsertAndIsolation(t *testing.T) {
	db := moKetNoi(t)
	c1, c2 := xaRieng(t)
	h := pkgstore.New(db)
	s := NewMapFrameStore(h)
	ctx := ctxXa(tenant.ID(c1))
	run := func(fn func(tx *pkgstore.ScopedTx) error) error { return h.For(ctx).Tx(ctx, fn) }

	if _, err := s.Get(ctx); !errors.Is(err, ErrMapFrameNotFound) {
		t.Fatalf("unset frame: %v, want ErrMapFrameNotFound", err)
	}
	if err := run(func(tx *pkgstore.ScopedTx) error {
		_, found, err := s.ForUpdate(ctx, tx)
		if found {
			t.Error("ForUpdate found a row that does not exist")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}

	var first domain.MapFrame
	if err := run(func(tx *pkgstore.ScopedTx) error {
		var err error
		first, err = s.Upsert(ctx, tx, domain.MapFrame{CenterLat: 15.730507, CenterLng: 108.37811, RadiusKm: 12.3}, "CB-00123")
		return err
	}); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if first.CenterLat != 15.730507 || first.CenterLng != 108.37811 || first.RadiusKm != 12.3 ||
		first.CreatedBy != "CB-00123" || first.UpdatedBy != "CB-00123" || first.CreatedAt.IsZero() {
		t.Errorf("first = %+v", first)
	}

	if err := run(func(tx *pkgstore.ScopedTx) error {
		before, found, err := s.ForUpdate(ctx, tx)
		if err != nil || !found || before.RadiusKm != 12.3 {
			t.Fatalf("ForUpdate = %+v, %v, %v", before, found, err)
		}
		_, err = s.Upsert(ctx, tx, domain.MapFrame{CenterLat: 16, CenterLng: 108, RadiusKm: 30}, "CB-00456")
		return err
	}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	got, err := s.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.RadiusKm != 30 || got.CreatedBy != "CB-00123" || got.UpdatedBy != "CB-00456" ||
		!got.CreatedAt.Equal(first.CreatedAt) {
		t.Errorf("after move = %+v — created_* fixed, updated_by moves", got)
	}

	// Another commune: nothing.
	if _, err := s.Get(ctxXa(tenant.ID(c2))); !errors.Is(err, ErrMapFrameNotFound) {
		t.Errorf("commune 2 sees commune 1's frame: %v", err)
	}
}

func TestPgMapFrameChecksRefuseWhatTheDomainRefuses(t *testing.T) {
	db := moKetNoi(t)
	c1, _ := xaRieng(t)
	h := pkgstore.New(db)
	s := NewMapFrameStore(h)
	ctx := ctxXa(tenant.ID(c1))

	// Bypassing domain.NormalizeMapFrame on purpose: the CHECKs are the floor under it.
	for name, f := range map[string]domain.MapFrame{
		"east of 109.5": {CenterLat: 16, CenterLng: 109.6, RadiusKm: 10},
		"radius 30.1":   {CenterLat: 16, CenterLng: 108, RadiusKm: 30.1},
		"radius 0.9":    {CenterLat: 16, CenterLng: 108, RadiusKm: 0.9},
	} {
		t.Run(name, func(t *testing.T) {
			err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
				_, err := s.Upsert(ctx, tx, f, "CB-00123")
				return err
			})
			if err == nil {
				t.Fatal("the database accepted a frame its CHECK forbids")
			}
		})
	}
	// The domain's edges are the CHECK's edges: both accept 8.4 / 109.5 / radius 1 and 30.
	for _, f := range []domain.MapFrame{
		{CenterLat: 8.4, CenterLng: 109.5, RadiusKm: 1},
		{CenterLat: 23.4, CenterLng: 102.1, RadiusKm: 30},
	} {
		if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
			_, err := s.Upsert(ctx, tx, f, "CB-00123")
			return err
		}); err != nil {
			t.Errorf("edge %+v refused by the database: %v", f, err)
		}
	}
}
