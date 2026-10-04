package app

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// The default-groups seed over the recording driver and an in-memory catalogue. The rules under test are
// ADR 0055 §2's, which ADR 0072 §3 names: existing codes untouched, soft-deleted codes NOT revived, one
// transaction, a run that creates nothing writes and audits nothing. The SQL half — `nguon = 'he-thong'`
// as a literal, the commune as $1 — is internal/store/map_asset_test.go.

type fakeCatalogue struct {
	codes    map[string]bool // code -> deleted
	live     int
	locked   int
	inserted []domain.MapAssetTypeDefault
	commune  tenant.ID
}

func (f *fakeCatalogue) LockCatalogueForSeed(_ context.Context, tx *store.ScopedTx) error {
	f.locked++
	f.commune = tx.TenantID()
	return nil
}

func (f *fakeCatalogue) CodeStates(_ context.Context, _ *store.ScopedTx, codes []string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, c := range codes {
		if d, ok := f.codes[c]; ok {
			out[c] = d
		}
	}
	return out, nil
}

func (f *fakeCatalogue) DemDangSong(context.Context, *store.ScopedTx) (int, error) {
	return f.live, nil
} // vi-name-ok: implements the existing store method's name

func (f *fakeCatalogue) InsertSystemRow(_ context.Context, _ *store.ScopedTx, _ string, d domain.MapAssetTypeDefault) error {
	f.inserted = append(f.inserted, d)
	f.codes[d.Code] = false
	f.live++
	return nil
}

func newSeeder(t *testing.T, cat *fakeCatalogue) (*MapAssetTypeDefaults, *recDB, context.Context) {
	t.Helper()
	d := &recDB{}
	uc := NewMapAssetTypeDefaults(openRec(t, d), cat)
	n := 0
	uc.newID = func() (string, error) { n++; return fmt.Sprintf("01JSEED%019d", n), nil }
	return uc, d, tenant.Into(context.Background(), xaA)
}

func TestSeedMapAssetTypesIdempotentAndNeverRevives(t *testing.T) {
	// The commune already has `cho` (live, maybe relabelled) and DELETED `ocop` itself.
	cat := &fakeCatalogue{codes: map[string]bool{"cho": false, "ocop": true, "rieng-cua-xa": false}, live: 2}
	uc, d, ctx := newSeeder(t, cat)

	res, err := uc.SeedDefaults(ctx, assetActor())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Created) != 9 || len(res.SkippedExisting) != 1 || len(res.SkippedDeleted) != 1 {
		t.Fatalf("created=%d existing=%d deleted=%d", len(res.Created), len(res.SkippedExisting), len(res.SkippedDeleted))
	}
	if res.SkippedExisting[0].Code != "cho" || res.SkippedDeleted[0].Code != "ocop" {
		t.Errorf("skipped = %+v / %+v", res.SkippedExisting, res.SkippedDeleted)
	}
	for _, ins := range cat.inserted {
		if ins.Code == "ocop" || ins.Code == "cho" {
			t.Errorf("seed wrote %q — an existing or deleted code", ins.Code)
		}
	}
	if cat.locked != 1 || cat.commune != xaA || d.committed != 1 {
		t.Errorf("locked=%d commune=%q committed=%d", cat.locked, cat.commune, d.committed)
	}
	entries := d.audits(t)
	if len(entries) != 1 || entries[0].action != ActionSeedMapAssetTypes || entries[0].actor != assetActorCode {
		t.Fatalf("entries = %+v", entries)
	}
	if got := entries[0].delta["bo_qua_da_xoa"].([]any); len(got) != 1 || got[0] != "ocop" {
		t.Errorf("delta skipped-deleted = %v", got)
	}

	// SECOND RUN: everything present; nothing written, nothing audited.
	res, err = uc.SeedDefaults(ctx, assetActor())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Created) != 0 || len(res.SkippedExisting) != 10 || len(res.SkippedDeleted) != 1 {
		t.Errorf("second run = %d/%d/%d", len(res.Created), len(res.SkippedExisting), len(res.SkippedDeleted))
	}
	if len(d.audits(t)) != 1 || len(cat.inserted) != 9 {
		t.Errorf("second run wrote: audits=%d inserts=%d", len(d.audits(t)), len(cat.inserted))
	}
}

func TestSeedMapAssetTypesRespectsTheCatalogueCeiling(t *testing.T) {
	cat := &fakeCatalogue{codes: map[string]bool{}, live: commsstore.TranDanhMucLoaiTaiNguyen - 5}
	uc, d, ctx := newSeeder(t, cat)
	if _, err := uc.SeedDefaults(ctx, assetActor()); !errors.Is(err, commsstore.ErrDanhMucDayTran) {
		t.Fatalf("err = %v", err)
	}
	if len(cat.inserted) != 0 || len(d.audits(t)) != 0 || d.committed != 0 {
		t.Error("a refused seed wrote")
	}
}

func TestSeedMapAssetTypesNeedsAnActor(t *testing.T) {
	uc, d, ctx := newSeeder(t, &fakeCatalogue{codes: map[string]bool{}})
	if _, err := uc.SeedDefaults(ctx, audit.Actor{}); !errors.Is(err, ErrMissingActor) {
		t.Fatalf("err = %v", err)
	}
	if d.begun != 0 {
		t.Error("opened a transaction with no actor")
	}
}
