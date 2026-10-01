package tenant

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// countingResolver counts calls and can fail like an unreachable registry.
type countingResolver struct {
	mu    sync.Mutex
	calls int
	known map[string]Tenant
	fail  error
}

func (r *countingResolver) ByHost(ctx context.Context, host string) (Tenant, bool) {
	t, ok, _ := r.XaTheoHost(ctx, host)
	return t, ok
}

// vi-name-ok: implements HostResolver, whose method name is platformclient's
func (r *countingResolver) XaTheoHost(_ context.Context, host string) (Tenant, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.fail != nil {
		return Tenant{}, false, r.fail
	}
	t, ok := r.known[host]
	return t, ok, nil
}

const resolverCommune = ID("01JHOSTRESOLVER00000000000")

// N lookups of the same UNKNOWN host → one registry call within the TTL.
func TestHostResolverCachesAMissOnce(t *testing.T) {
	r := &countingResolver{known: map[string]Tenant{}}
	c := NewCachedDirectory(r, time.Minute)
	for i := 0; i < 50; i++ {
		if _, ok, err := c.XaTheoHost(context.Background(), "khong-co.vigov.vn"); ok || err != nil {
			t.Fatalf("lookup %d: ok=%v err=%v", i, ok, err)
		}
	}
	if r.calls != 1 {
		t.Fatalf("registry calls = %d, want 1", r.calls)
	}
}

func TestHostResolverCachesAHitAndExpires(t *testing.T) {
	r := &countingResolver{known: map[string]Tenant{"xa-a.vigov.vn": {ID: resolverCommune, Active: true}}}
	c := NewCachedDirectory(r, time.Minute)
	clock := time.Unix(1_800_000_000, 0)
	c.now = func() time.Time { return clock }
	for i := 0; i < 3; i++ {
		if got, ok, err := c.XaTheoHost(context.Background(), "xa-a.vigov.vn"); !ok || err != nil || got.ID != resolverCommune {
			t.Fatalf("got %+v ok=%v err=%v", got, ok, err)
		}
	}
	clock = clock.Add(time.Minute)
	_, _, _ = c.XaTheoHost(context.Background(), "xa-a.vigov.vn")
	if r.calls != 2 {
		t.Fatalf("registry calls = %d, want 2 (one per TTL)", r.calls)
	}
}

// An outage is reported and NEVER remembered: the next lookup asks again.
func TestHostResolverNeverCachesAnError(t *testing.T) {
	r := &countingResolver{known: map[string]Tenant{"xa-a.vigov.vn": {ID: resolverCommune, Active: true}},
		fail: errors.New("Unavailable")}
	c := NewCachedDirectory(r, time.Minute)
	if _, _, err := c.XaTheoHost(context.Background(), "xa-a.vigov.vn"); err == nil {
		t.Fatal("an outage was not reported")
	}
	r.fail = nil
	if _, ok, err := c.XaTheoHost(context.Background(), "xa-a.vigov.vn"); !ok || err != nil {
		t.Fatalf("after recovery: ok=%v err=%v — an error was cached", ok, err)
	}
}

type byHostOnly struct{}

func (byHostOnly) ByHost(context.Context, string) (Tenant, bool) { return Tenant{}, false }

func TestHostResolverRefusesADirectoryThatCannotReportOutages(t *testing.T) {
	c := NewCachedDirectory(byHostOnly{}, time.Minute)
	if _, _, err := c.XaTheoHost(context.Background(), "x.vigov.vn"); !errors.Is(err, ErrNoHostResolver) {
		t.Fatalf("err = %v, want ErrNoHostResolver", err)
	}
}

// The same ceiling as ByHost: a scan of random hosts does not grow the map past TranMuc.
func TestHostResolverRespectsTheCeiling(t *testing.T) {
	r := &countingResolver{known: map[string]Tenant{}}
	c := NewCachedDirectory(r, time.Hour)
	for i := 0; i < TranMuc+100; i++ {
		_, _, _ = c.XaTheoHost(context.Background(), fmt.Sprintf("h%d.vigov.vn", i))
	}
	if n := len(c.entries); n > TranMuc {
		t.Fatalf("entries = %d, ceiling %d", n, TranMuc)
	}
}
