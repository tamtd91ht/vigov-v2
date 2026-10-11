package store

import (
	"context"
	"errors"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// disaster_alerts against a REAL PostgreSQL — what the fake driver in internal/app cannot see: 0026's
// columns as the store names them, the JSONB round trip, the in-force predicate evaluated, and the guard
// trigger on an ended alert. SKIPPED WITHOUT VIGOV_TEST_DSN (shared harness: moKetNoi / xaRieng) — a
// green run without the DSN means this file COMPILES, nothing more.
//
// ROWS (xã 1) AT now = 12:00, EACH MAKING ONE DEFECT VISIBLE:
//
//	live      from 10:00, no end              in force
//	later     from 11:00, to 13:00            in force, and FIRST (newest start)
//	future    from 13:00                      drop `effective_from <= now` → appears
//	past      from 08:00, to 12:00            the window is [from, to): drop the strictness → appears
//	ended     from 09:00, ended               drop `ended_at IS NULL` → appears
//	gone      from 09:30, soft-deleted        drop `deleted_at IS NULL` → appears
//	xã 2's x  from 10:00                      drop the commune → appears in xã 1
func TestPgDisasterAlertsInForceAndGuard(t *testing.T) {
	db := moKetNoi(t)
	c1, c2 := xaRieng(t)
	day := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	at := func(h int) time.Time { return day.Add(time.Duration(h) * time.Hour) }
	s := NewDisasterAlertStore(pkgstore.New(db))

	insert := func(commune, id string, from time.Time, to time.Time) {
		t.Helper()
		ctx := ctxXa(tenant.ID(commune))
		err := pkgstore.New(db).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
			return s.Insert(ctx, tx, domain.DisasterAlert{ID: id, Kind: domain.DisasterKindFlood, Title: "Lũ " + id,
				Detail: "Sơ tán", EffectiveFrom: from, EffectiveTo: to,
				DangerZone:        []domain.DangerZonePoint{{X: 1, Y: 1}, {X: 9, Y: 1}, {X: 5, Y: 9}},
				EmergencyContacts: []domain.EmergencyContact{{Role: "Trực ban", Name: "A", Phone: "0900000000"}},
			}, "CB-TEST")
		})
		if err != nil {
			t.Fatalf("insert %s: %v", id, err)
		}
	}
	insert(c1, "live", at(10), time.Time{})
	insert(c1, "later", at(11), at(13))
	insert(c1, "future", at(13), time.Time{})
	insert(c1, "past", at(8), at(12))
	insert(c1, "ended", at(9), time.Time{})
	insert(c1, "gone", at(9).Add(30*time.Minute), time.Time{})
	insert(c2, "x", at(10), time.Time{})

	ctx := ctxXa(tenant.ID(c1))
	write := func(fn func(tx *pkgstore.ScopedTx) error) error {
		return pkgstore.New(db).For(ctx).Tx(ctx, fn)
	}
	if err := write(func(tx *pkgstore.ScopedTx) error { return s.End(ctx, tx, "ended", "CB-TEST") }); err != nil {
		t.Fatalf("end: %v", err)
	}
	if err := write(func(tx *pkgstore.ScopedTx) error { return s.SoftDelete(ctx, tx, "gone", "CB-TEST", "thử") }); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	got, err := s.InForce(ctx, at(12))
	if err != nil {
		t.Fatalf("InForce: %v", err)
	}
	if len(got) != 2 || got[0].ID != "later" || got[1].ID != "live" {
		t.Fatalf("in force = %+v", got)
	}
	if len(got[0].DangerZone) != 3 || got[0].EmergencyContacts[0].Phone != "0900000000" || got[0].EvacuationPoints == nil {
		t.Errorf("JSONB round trip = %+v", got[0])
	}

	// The guard: an ended alert is a historical record — the UPDATE matches no row here (ended_at IS
	// NULL), and the trigger refuses it underneath when the predicate is bypassed.
	err = write(func(tx *pkgstore.ScopedTx) error {
		a, _ := s.ByIDForUpdate(ctx, tx, "ended")
		a.Title = "Sửa"
		return s.Update(ctx, tx, a, "CB-TEST")
	})
	if !errors.Is(err, ErrDisasterAlertNotFound) {
		t.Errorf("editing an ended alert: %v", err)
	}
	if _, err := db.ExecContext(context.Background(),
		`UPDATE disaster_alerts SET title = 'x' WHERE tenant_id = $1 AND id = 'ended'`, c1); err == nil {
		t.Error("the guard let an ended alert be edited")
	}
	if _, err := db.ExecContext(context.Background(),
		`DELETE FROM disaster_alerts WHERE tenant_id = $1 AND id = 'live'`, c1); err == nil {
		t.Error("the guard let a hard delete through")
	}
}
