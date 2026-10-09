package store

import (
	"sync"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// Integration tests for migration 0017 against a real PostgreSQL, sharing the harness in
// checker_pg_test.go (moKetNoi, xaRieng, ctxXa). Skipped unless VIGOV_TEST_DSN is set.
//
// THE ONE PROPERTY ONLY A SERVER CAN SHOW: two claimers that both read the same lease and both
// decided "due" — concurrently, on two connections — produce exactly ONE winner, for the first
// claim (INSERT vs INSERT) and for a later one (UPDATE … WHERE last_run_id = read value).

func TestPgAutomationClaimRaceOneWinner(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	ctx := ctxXa(xa)
	kho := pkgstore.New(db)
	s := NewAutomationStore(kho)
	scope := domain.AutomationScope{Job: domain.JobSLAReminders, WorkKind: domain.LoaiViecNhiemVu}

	race := func(prev string, ids [2]string) int {
		var wg sync.WaitGroup
		var mu sync.Mutex
		won := 0
		for _, id := range ids {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				run := domain.AutomationRun{ID: id, Scope: scope, Trigger: domain.TriggerSchedule,
					ClaimedAt: time.Now().UTC().Truncate(time.Microsecond)}
				err := kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
					ok, err := s.ClaimScope(ctx, tx, prev, run)
					if err != nil || !ok {
						return err
					}
					mu.Lock()
					won++
					mu.Unlock()
					return s.InsertRun(ctx, tx, run)
				})
				if err != nil {
					t.Errorf("claim %s: %v", id, err)
				}
			}(id)
		}
		wg.Wait()
		return won
	}

	if n := race("", [2]string{idSLA("RUNA"), idSLA("RUNB")}); n != 1 {
		t.Fatalf("first claim: %d winners, want exactly 1", n)
	}
	states, err := s.ScopeStates(ctx)
	if err != nil {
		t.Fatalf("ScopeStates: %v", err)
	}
	prev := states[scope].LastRunID
	if n := race(prev, [2]string{idSLA("RUNC"), idSLA("RUND")}); n != 1 {
		t.Fatalf("later claim: %d winners, want exactly 1", n)
	}
	// A claimer still holding the OLD run id loses, whatever else it does.
	if n := race(prev, [2]string{idSLA("RUNE"), idSLA("RUNF")}); n != 0 {
		t.Fatalf("stale claim: %d winners, want 0", n)
	}
}

// Migration 0029: a `scheduled_reports` run is admitted with its period and read back with it; a period
// on another job, or a scheduled_reports run without one, is refused by the server.
func TestPgScheduledReportsRunCarriesPeriod(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	ctx := ctxXa(xa)
	kho := pkgstore.New(db)
	s := NewAutomationStore(kho)
	at := time.Now().UTC().Truncate(time.Microsecond)

	insert := func(run domain.AutomationRun) error {
		return kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error { return s.InsertRun(ctx, tx, run) })
	}
	good := domain.AutomationRun{ID: idSLA("REPA"), Scope: domain.AutomationScope{Job: domain.JobScheduledReports,
		WorkKind: domain.LoaiViecNhiemVu}, Trigger: domain.TriggerSchedule, ClaimedAt: at, ReportPeriod: domain.ReportPeriodMonth}
	if err := insert(good); err != nil {
		t.Fatalf("scheduled_reports run refused: %v", err)
	}
	err := kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		r, err := s.RunForUpdate(ctx, tx, good.ID)
		if err == nil && r.ReportPeriod != domain.ReportPeriodMonth {
			t.Errorf("period read back as %q", r.ReportPeriod)
		}
		return err
	})
	if err != nil {
		t.Fatalf("RunForUpdate: %v", err)
	}
	noPeriod := good
	noPeriod.ID, noPeriod.ReportPeriod = idSLA("REPB"), ""
	if insert(noPeriod) == nil {
		t.Error("scheduled_reports run without a period admitted")
	}
	foreign := domain.AutomationRun{ID: idSLA("REPC"), Scope: domain.AutomationScope{Job: domain.JobEscalation,
		WorkKind: domain.LoaiViecNhiemVu}, Trigger: domain.TriggerSchedule, ClaimedAt: at, ReportPeriod: domain.ReportPeriodWeek}
	if insert(foreign) == nil {
		t.Error("escalation run carrying a report period admitted")
	}
}
