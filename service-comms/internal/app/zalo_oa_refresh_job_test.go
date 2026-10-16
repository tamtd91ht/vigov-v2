package app

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

type fakeOALocks struct {
	locked      bool
	lockErr     error
	refreshIDs  []tenant.ID
	openIDs     []tenant.ID
	askedBefore time.Time
	released    int
}

func (l *fakeOALocks) CommunesWithOAToRefresh(_ context.Context, before time.Time) ([]tenant.ID, error) {
	l.askedBefore = before
	return l.refreshIDs, nil
}
func (l *fakeOALocks) CommunesWithOpenAuthorizations(context.Context, time.Time) ([]tenant.ID, error) {
	return l.openIDs, nil
}
func (l *fakeOALocks) TryLockRefreshScheduler(context.Context) (func(), bool, error) {
	return func() { l.released++ }, l.locked, l.lockErr
}

type fakeKeeper struct {
	refreshed, expired []tenant.ID
	panicIn            tenant.ID
}

func (k *fakeKeeper) RefreshIfDue(ctx context.Context) (bool, error) {
	id := tenant.MustFrom(ctx)
	if id == k.panicIn {
		panic("boom")
	}
	k.refreshed = append(k.refreshed, id)
	return true, nil
}
func (k *fakeKeeper) ExpireOpenAuthorizations(ctx context.Context, _ time.Time) (int, error) {
	k.expired = append(k.expired, tenant.MustFrom(ctx))
	return 1, nil
}

// oaJobRegistry answers active for every commune but `merged`.
type oaJobRegistry struct{ merged tenant.ID }

func (g oaJobRegistry) XaTrongNguCanh(ctx context.Context) (tenant.Tenant, bool, error) { // vi-name-ok: the method of the existing PortalCommuneRegistry interface
	id := tenant.MustFrom(ctx)
	return tenant.Tenant{ID: id, Host: "x.example", Active: id != g.merged}, true, nil
}

func newJob(t *testing.T, l *fakeOALocks, k *fakeKeeper, merged tenant.ID) (*ZaloOARefreshJob, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	j, err := NewZaloOARefreshJob(ZaloOARefreshJobDeps{Locks: l, Keeper: k, Registry: oaJobRegistry{merged: merged},
		Log: slog.New(slog.NewTextHandler(&buf, nil))})
	if err != nil {
		t.Fatal(err)
	}
	j.now = func() time.Time { return oaClock }
	return j, &buf
}

func TestRefreshJobRefreshesEachCommuneInItsOwnContextAndSweeps(t *testing.T) {
	l := &fakeOALocks{locked: true, refreshIDs: []tenant.ID{xaA, xaB}, openIDs: []tenant.ID{xaB}}
	k := &fakeKeeper{}
	j, _ := newJob(t, l, k, "")
	j.Tick(context.Background())
	if len(k.refreshed) != 2 || k.refreshed[0] != xaA || k.refreshed[1] != xaB {
		t.Fatalf("refreshed = %v", k.refreshed)
	}
	if len(k.expired) != 1 || k.expired[0] != xaB {
		t.Fatalf("expired = %v", k.expired)
	}
	if !l.askedBefore.Equal(oaClock.Add(domain.AccessRefreshLead)) {
		t.Fatalf("asked before %v, want now + the 2-hour lead", l.askedBefore)
	}
	if l.released != 1 {
		t.Fatal("the scheduler lock was not released")
	}
}

func TestRefreshJobDoesNothingWithoutTheLock(t *testing.T) {
	for name, l := range map[string]*fakeOALocks{
		"held elsewhere": {locked: false, refreshIDs: []tenant.ID{xaA}},
		"lock error":     {lockErr: errors.New("db down"), refreshIDs: []tenant.ID{xaA}},
	} {
		t.Run(name, func(t *testing.T) {
			k := &fakeKeeper{}
			j, _ := newJob(t, l, k, "")
			j.Tick(context.Background())
			if len(k.refreshed) != 0 {
				t.Fatalf("refreshed without the lock: %v", k.refreshed)
			}
		})
	}
}

func TestRefreshJobSkipsAMergedCommuneAndSurvivesAPanic(t *testing.T) {
	l := &fakeOALocks{locked: true, refreshIDs: []tenant.ID{xaA, xaB}}
	k := &fakeKeeper{panicIn: xaA}
	j, logs := newJob(t, l, k, "")
	j.Tick(context.Background())
	if len(k.refreshed) != 1 || k.refreshed[0] != xaB || !strings.Contains(logs.String(), "panic") {
		t.Fatalf("refreshed %v logs %q", k.refreshed, logs.String())
	}
	k2 := &fakeKeeper{}
	j2, _ := newJob(t, &fakeOALocks{locked: true, refreshIDs: []tenant.ID{xaA, xaB}}, k2, xaA)
	j2.Tick(context.Background())
	if len(k2.refreshed) != 1 || k2.refreshed[0] != xaB {
		t.Fatalf("a merged commune was touched: %v", k2.refreshed)
	}
}

func TestRefreshJobRequiresItsDependencies(t *testing.T) {
	if _, err := NewZaloOARefreshJob(ZaloOARefreshJobDeps{Locks: &fakeOALocks{}, Keeper: &fakeKeeper{}}); err == nil {
		t.Fatal("a job without a registry was built")
	}
}
