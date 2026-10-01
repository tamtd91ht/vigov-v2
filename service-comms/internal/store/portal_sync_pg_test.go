package store

// The portal sync store against a REAL PostgreSQL (migration 0013's tables and the sync's INSERT into
// noi_dung_mini_app). SKIPPED UNLESS VIGOV_TEST_DSN IS SET — read noi_dung_mini_app_pg_test.go's header:
// a green run without the DSN means this file COMPILES, nothing more.
//
// What only the database can say, and this file asks: every column named here exists as 0013 creates
// it; the CHECKs accept what the writers send (the actor/trigger pairing, the all-or-none finish, the
// `only for synced` portal category); `UNIQUE (tenant_id, nguon_id_ngoai)` is what ErrPortalItemExists
// is made of; a SOFT-DELETED item still holds its portal id; and every read stays in its commune.

import (
	"context"
	"errors"
	"testing"
	"time"

	pkgpage "github.com/vihat/vigov/core/page"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

func TestPgPortalSyncStore(t *testing.T) {
	db := moKetNoi(t)
	a, b := xaRieng(t)
	kho := pkgstore.New(db)
	s := NewPortalSyncStore(kho)
	ctxA := tenant.Into(context.Background(), tenant.ID(a))
	ctxB := tenant.Into(context.Background(), tenant.ID(b))
	inTx := func(ctx context.Context, fn func(*pkgstore.ScopedTx) error) {
		t.Helper()
		if err := kho.For(ctx).Tx(ctx, fn); err != nil {
			t.Fatal(err)
		}
	}
	sealed := make([]byte, 40)

	// --- settings
	st := domain.DefaultPortalSyncSettings()
	st.APIURL, st.IsEnabled = "https://portal.example.gov.vn/api", true
	inTx(ctxA, func(tx *pkgstore.ScopedTx) error { return s.UpsertSettings(ctxA, tx, st, sealed, "CB-00123") })
	got, err := s.Settings(ctxA)
	if err != nil || !got.APIKeySet || got.APIURL != st.APIURL || got.UpdatedBy != "CB-00123" || !got.LastRunAt.IsZero() {
		t.Fatalf("settings = %+v, %v", got, err)
	}
	if _, err := s.Settings(ctxB); !errors.Is(err, ErrPortalSyncSettingsNotFound) {
		t.Fatalf("another commune reads A's settings: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	inTx(ctxA, func(tx *pkgstore.ScopedTx) error { return s.MarkRunStarted(ctxA, tx, now) })
	if got, _ := s.Settings(ctxA); !got.LastRunAt.Equal(now) || got.UpdatedBy != "CB-00123" {
		t.Errorf("after MarkRunStarted = %+v", got)
	}

	// --- categories
	cat := domain.PortalCategory{ID: "01JCAT" + a[6:26], ExternalID: "120", Name: "Tin tức", TargetKind: "tin-tuc", IsSelected: true}
	inTx(ctxA, func(tx *pkgstore.ScopedTx) error { return s.InsertCategory(ctxA, tx, cat, "CB-00123") })
	cat.TargetKind, cat.IsSelected = "su-kien", false
	inTx(ctxA, func(tx *pkgstore.ScopedTx) error { return s.UpdateCategory(ctxA, tx, cat, "CB-00123") })
	cats, err := s.Categories(ctxA)
	if err != nil || len(cats) != 1 || cats[0].TargetKind != "su-kien" || cats[0].IsSelected {
		t.Fatalf("categories = %+v, %v", cats, err)
	}

	// --- runs
	run := domain.PortalSyncRun{ID: "01JRUN" + a[6:26], TriggerKind: domain.PortalRunScheduled, Actor: "system", StartedAt: now}
	inTx(ctxA, func(tx *pkgstore.ScopedTx) error { return s.InsertRun(ctxA, tx, run) })
	run.FinishedAt, run.Outcome = now.Add(time.Second), domain.PortalRunPartial
	run.Counts = domain.PortalRunCounts{Fetched: 5, Imported: 2, SkippedExisting: 1, SkippedDeleted: 1, Failed: 1}
	run.Errors = []domain.PortalRunError{{CategoryExternalID: "144", CategoryName: "Sự kiện", Error: "timeout", Count: 1}}
	inTx(ctxA, func(tx *pkgstore.ScopedTx) error { return s.FinishRun(ctxA, tx, run) })
	err = kho.For(ctxA).Tx(ctxA, func(tx *pkgstore.ScopedTx) error { return s.FinishRun(ctxA, tx, run) })
	if !errors.Is(err, ErrPortalSyncRunFinished) {
		t.Fatalf("second finish = %v, want ErrPortalSyncRunFinished", err)
	}
	stuck := domain.PortalSyncRun{ID: "01JOLD" + a[6:26], TriggerKind: domain.PortalRunManual, Actor: "CB-00123",
		StartedAt: now.Add(-2 * time.Hour)}
	inTx(ctxA, func(tx *pkgstore.ScopedTx) error { return s.InsertRun(ctxA, tx, stuck) })
	var reaped []string
	inTx(ctxA, func(tx *pkgstore.ScopedTx) error {
		var err error
		reaped, err = s.ReapStuckRuns(ctxA, tx, now.Add(-time.Hour), now)
		return err
	})
	if len(reaped) != 1 || reaped[0] != stuck.ID {
		t.Fatalf("reaped = %v", reaped)
	}
	req, err := pkgpage.New(SortPortalSyncRuns, "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	page, err := s.Runs(ctxA, req)
	if err != nil || len(page.Items) != 2 || page.Items[0].ID != run.ID || page.Items[0].Counts.Imported != 2 ||
		len(page.Items[0].Errors) != 1 || page.Items[1].Outcome != domain.PortalRunFailed {
		t.Fatalf("runs = %+v, %v", page.Items, err)
	}
	if other, _ := s.Runs(ctxB, req); len(other.Items) != 0 {
		t.Fatal("another commune reads A's runs")
	}

	// --- the sync's item write
	item := domain.NoiDungMiniApp{ID: "01JITEM" + a[7:26], Loai: domain.LoaiTinTuc, TieuDe: "Tin", NoiDung: "<p>x</p>",
		NgayDang: time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC), TrangThai: domain.TrangThaiChoDuyet,
		NguonIDNgoai: "cttdt-danang:9001", NguoiTaoMa: "system"}
	inTx(ctxA, func(tx *pkgstore.ScopedTx) error { return s.InsertSyncedItem(ctxA, tx, item, cat.ID) })
	dup := item
	dup.ID = "01JDUP" + a[6:26]
	err = kho.For(ctxA).Tx(ctxA, func(tx *pkgstore.ScopedTx) error { return s.InsertSyncedItem(ctxA, tx, dup, cat.ID) })
	if !errors.Is(err, ErrPortalItemExists) {
		t.Fatalf("duplicate portal id = %v, want ErrPortalItemExists", err)
	}
	held, err := s.ExistingPortalItems(ctxA, []string{"cttdt-danang:9001", "cttdt-danang:9002"})
	if err != nil || len(held) != 1 || held["cttdt-danang:9001"] {
		t.Fatalf("held = %v, %v", held, err)
	}
	if _, err := db.Exec(`UPDATE noi_dung_mini_app SET deleted_at = now(), deleted_by = 'CB-00123',
		delete_reason = 'test' WHERE tenant_id = $1 AND id = $2`, a, item.ID); err != nil {
		t.Fatal(err)
	}
	if held, _ := s.ExistingPortalItems(ctxA, []string{"cttdt-danang:9001"}); !held["cttdt-danang:9001"] {
		t.Fatal("a soft-deleted item no longer holds its portal id — the next run would bring it back")
	}
	if held, _ := s.ExistingPortalItems(ctxB, []string{"cttdt-danang:9001"}); len(held) != 0 {
		t.Fatal("another commune sees A's portal ids")
	}
}
