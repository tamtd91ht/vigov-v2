package tenant

// CachedCommune — the citizen-session commune check's cache (user finding 01/10/2026: deactivating a
// commune did not stop citizen sessions already open). The property that matters is the TTL bound:
// a commune deactivated in the registry is refused within one TENANT_CACHE_TTL, never later.

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// registryFake is a registry whose answer for one commune the test can flip mid-run.
type registryFake struct {
	mu     sync.Mutex
	calls  int
	active bool
	known  bool
	err    error
}

func (r *registryFake) Current(ctx context.Context) (Tenant, bool, error) {
	id := MustFrom(ctx)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.err != nil {
		return Tenant{}, false, r.err
	}
	if !r.known {
		return Tenant{}, false, nil
	}
	return Tenant{ID: id, Active: r.active}, true, nil
}

// fakeClock is moved by the test; nothing sleeps.
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func cachedWithClock(inner CommuneLookup, ttl time.Duration) (*CachedCommune, *fakeClock) {
	clk := &fakeClock{t: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)}
	c := NewCachedCommune(inner, ttl)
	c.now = clk.now
	return c, clk
}

// Active ⇒ answered from cache after the first call; deactivated in the registry ⇒ still answered
// active INSIDE the TTL (that is the bounded window), refused the moment the TTL has passed.
func TestCachedCommuneDeactivationTakesEffectWithinTTL(t *testing.T) {
	reg := &registryFake{known: true, active: true}
	c, clk := cachedWithClock(reg, 30*time.Second)
	ctx := thangBinh()

	for range 3 {
		got, ok, err := c.Current(ctx)
		if err != nil || !ok || !got.Active {
			t.Fatalf("active commune: %+v ok=%v err=%v", got, ok, err)
		}
	}
	if reg.calls != 1 {
		t.Fatalf("registry reached %d times for one commune inside the TTL, want 1", reg.calls)
	}

	reg.mu.Lock()
	reg.active = false // the operator deactivates the commune
	reg.mu.Unlock()

	clk.t = clk.t.Add(29 * time.Second)
	if got, _, _ := c.Current(ctx); !got.Active {
		t.Fatal("the cache answered before its TTL — the test's premise is wrong")
	}

	clk.t = clk.t.Add(2 * time.Second) // 31 s after the first read: past the TTL
	got, ok, err := c.Current(ctx)
	if err != nil || !ok {
		t.Fatalf("after the TTL: ok=%v err=%v", ok, err)
	}
	if got.Active {
		t.Fatal("a commune deactivated in the registry is still answered active after one TTL")
	}
}

// An error is never cached: the next request asks again, so recovery is immediate and an outage is
// never remembered as "commune gone".
func TestCachedCommuneDoesNotCacheErrors(t *testing.T) {
	reg := &registryFake{known: true, active: true, err: errors.New("unavailable")}
	c, _ := cachedWithClock(reg, time.Minute)
	ctx := thangBinh()

	if _, ok, err := c.Current(ctx); err == nil || ok {
		t.Fatalf("an unreachable registry must surface as an error, got ok=%v err=%v", ok, err)
	}
	reg.mu.Lock()
	reg.err = nil
	reg.mu.Unlock()
	if got, ok, err := c.Current(ctx); err != nil || !ok || !got.Active {
		t.Fatalf("after recovery: %+v ok=%v err=%v — the error was cached", got, ok, err)
	}
	if reg.calls != 2 {
		t.Fatalf("registry reached %d times, want 2", reg.calls)
	}
}

// Unknown is remembered like a host miss (bounded by the same TTL).
func TestCachedCommuneRemembersUnknown(t *testing.T) {
	reg := &registryFake{}
	c, _ := cachedWithClock(reg, time.Minute)
	for range 3 {
		if _, ok, err := c.Current(thangBinh()); ok || err != nil {
			t.Fatalf("unknown commune: ok=%v err=%v", ok, err)
		}
	}
	if reg.calls != 1 {
		t.Fatalf("registry reached %d times, want 1", reg.calls)
	}
}

// TTL zero disables caching — every call reaches the registry.
func TestCachedCommuneZeroTTLDisablesCache(t *testing.T) {
	reg := &registryFake{known: true, active: true}
	c := NewCachedCommune(reg, 0)
	for range 3 {
		_, _, _ = c.Current(thangBinh())
	}
	if reg.calls != 3 {
		t.Fatalf("registry reached %d times with TTL 0, want 3", reg.calls)
	}
}

func TestNewCachedCommuneRefusesNil(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("built a CachedCommune over a nil lookup")
		}
	}()
	NewCachedCommune(nil, time.Minute)
}

func thangBinh() context.Context { return Into(context.Background(), ID(ulidThangBinh)) }

// No commune in ctx is an error before the registry is asked — never a lookup of "".
func TestCachedCommuneWithoutCommuneIsAnError(t *testing.T) {
	reg := &registryFake{known: true, active: true}
	c := NewCachedCommune(reg, time.Minute)
	if _, ok, err := c.Current(context.Background()); !errors.Is(err, ErrNoTenant) || ok {
		t.Fatalf("no commune in ctx: ok=%v err=%v, want ErrNoTenant", ok, err)
	}
	if reg.calls != 0 {
		t.Fatal("the registry was asked about no commune")
	}
}

// Two communes are two entries: one commune's answer is never served for another.
func TestCachedCommuneKeysByCommune(t *testing.T) {
	reg := &registryFake{known: true, active: true}
	c := NewCachedCommune(reg, time.Minute)
	const other = "01JD8ZQK9M3NPXR7TVWYB2C4EG"
	a, _, _ := c.Current(thangBinh())
	b, _, _ := c.Current(Into(context.Background(), ID(other)))
	if a.ID != ID(ulidThangBinh) || b.ID != ID(other) || reg.calls != 2 {
		t.Fatalf("a=%v b=%v calls=%d — the cache mixed two communes", a.ID, b.ID, reg.calls)
	}
}
