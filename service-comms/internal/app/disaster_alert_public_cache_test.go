package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// countingInForce is the store: per-commune rows, a read counter per commune, an optional failure.
type countingInForce struct {
	mu    sync.Mutex
	rows  map[tenant.ID][]domain.DisasterAlert
	reads map[tenant.ID]int
	err   error
	// during, when set, runs inside the read — a test uses it to act while a read is in flight.
	during func()
}

func (s *countingInForce) InForce(ctx context.Context, _ time.Time) ([]domain.DisasterAlert, error) {
	id := tenant.MustFrom(ctx)
	s.mu.Lock()
	s.reads[id]++
	rows, err, during := s.rows[id], s.err, s.during
	s.mu.Unlock()
	if during != nil {
		during()
	}
	return rows, err
}

func (s *countingInForce) readsOf(id tenant.ID) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reads[id]
}

var cacheT0 = time.Date(2026, 10, 11, 3, 0, 0, 0, time.UTC)

func newCacheRig() (*InForceAlertCache, *countingInForce, *time.Time) {
	src := &countingInForce{
		rows: map[tenant.ID][]domain.DisasterAlert{
			xaA: {{ID: "a-1", Kind: domain.DisasterKindFlood, Title: "Lũ", EffectiveFrom: cacheT0.Add(-time.Hour)}},
			xaB: {{ID: "b-1", Kind: domain.DisasterKindStorm, Title: "Bão", EffectiveFrom: cacheT0.Add(-time.Hour)}},
		},
		reads: map[tenant.ID]int{},
	}
	c := NewInForceAlertCache(src)
	clock := cacheT0
	c.clock = func() time.Time { return clock }
	return c, src, &clock
}

func ids(rows []domain.DisasterAlert) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out
}

// The owner's numbers: 30 seconds, and an entry is a hit before it and a miss at it.
func TestInForceCacheHitThenExpiryAt30s(t *testing.T) {
	if PublicDisasterAlertCacheTTL != 30*time.Second {
		t.Fatalf("TTL = %v — the owner accepted 30 s of staleness, no more", PublicDisasterAlertCacheTTL)
	}
	c, src, clock := newCacheRig()
	ctx := tenant.Into(context.Background(), xaA)

	for i := 0; i < 5; i++ {
		got, err := c.InForce(ctx, *clock)
		if err != nil || len(got) != 1 || got[0].ID != "a-1" {
			t.Fatalf("read %d: %v %v", i+1, ids(got), err)
		}
	}
	if n := src.readsOf(xaA); n != 1 {
		t.Fatalf("store reads within the TTL = %d, want 1", n)
	}
	*clock = cacheT0.Add(PublicDisasterAlertCacheTTL - time.Nanosecond)
	_, _ = c.InForce(ctx, *clock)
	if n := src.readsOf(xaA); n != 1 {
		t.Fatalf("a hit just before 30 s read the store (%d)", n)
	}
	*clock = cacheT0.Add(PublicDisasterAlertCacheTTL)
	_, _ = c.InForce(ctx, *clock)
	if n := src.readsOf(xaA); n != 2 {
		t.Fatalf("at 30 s the entry must be gone — store reads %d, want 2", n)
	}
}

// Two communes, two entries: commune B's read never answers from A's entry (rule 1, invariant 7).
func TestInForceCacheIsPerCommune(t *testing.T) {
	c, src, clock := newCacheRig()
	a := tenant.Into(context.Background(), xaA)
	b := tenant.Into(context.Background(), xaB)
	gotA, _ := c.InForce(a, *clock)
	gotB, _ := c.InForce(b, *clock)
	if len(gotA) != 1 || gotA[0].ID != "a-1" || len(gotB) != 1 || gotB[0].ID != "b-1" {
		t.Fatalf("A=%v B=%v", ids(gotA), ids(gotB))
	}
	if src.readsOf(xaA) != 1 || src.readsOf(xaB) != 1 {
		t.Fatalf("reads A=%d B=%d", src.readsOf(xaA), src.readsOf(xaB))
	}
	if _, ok := c.entries["t:"+string(xaA)]; !ok {
		t.Fatalf("entry keys = %v, want t:<tenant_id>", c.entries)
	}
}

// No commune in ctx is a refusal, never a shared entry — and the store is not asked.
func TestInForceCacheRefusesWithoutCommune(t *testing.T) {
	c, src, clock := newCacheRig()
	if _, err := c.InForce(context.Background(), *clock); !errors.Is(err, ErrInForceCacheNoCommune) {
		t.Fatalf("err = %v", err)
	}
	if len(src.reads) != 0 || len(c.entries) != 0 {
		t.Fatal("asked the store or cached without a commune")
	}
}

// A hit re-filters at the caller's now: an alert whose end passed leaves the reply ON TIME.
func TestInForceCacheDropsAnAlertThatEndedWhileCached(t *testing.T) {
	c, src, clock := newCacheRig()
	src.rows[xaA] = []domain.DisasterAlert{
		{ID: "ends", Kind: domain.DisasterKindFlood, EffectiveFrom: cacheT0.Add(-time.Hour), EffectiveTo: cacheT0.Add(10 * time.Second)},
		{ID: "stays", Kind: domain.DisasterKindFlood, EffectiveFrom: cacheT0.Add(-time.Hour)},
	}
	ctx := tenant.Into(context.Background(), xaA)
	if got, _ := c.InForce(ctx, *clock); len(got) != 2 {
		t.Fatalf("first read %v", ids(got))
	}
	*clock = cacheT0.Add(10 * time.Second)
	got, _ := c.InForce(ctx, *clock)
	if len(got) != 1 || got[0].ID != "stays" || src.readsOf(xaA) != 1 {
		t.Fatalf("from the cache at the end instant: %v (reads %d)", ids(got), src.readsOf(xaA))
	}
}

// Errors are never cached; the over-ceiling answer is, with its sentinel.
func TestInForceCacheErrorsAndCeiling(t *testing.T) {
	c, src, clock := newCacheRig()
	ctx := tenant.Into(context.Background(), xaA)
	src.err = errors.New("pq: boom")
	if _, err := c.InForce(ctx, *clock); err == nil {
		t.Fatal("store failure swallowed")
	}
	src.err = commsstore.ErrTooManyAlertsInForce
	for i := 0; i < 2; i++ {
		got, err := c.InForce(ctx, *clock)
		if !errors.Is(err, commsstore.ErrTooManyAlertsInForce) || len(got) != 1 {
			t.Fatalf("read %d over the ceiling: %v %v", i+1, ids(got), err)
		}
	}
	if n := src.readsOf(xaA); n != 2 {
		t.Fatalf("store reads = %d, want 2 (the failure not cached, the ceiling answer cached)", n)
	}
}

// Invalidate drops the entry, and a read that STARTED before it cannot put the old list back.
func TestInForceCacheInvalidateBeatsAnInFlightRead(t *testing.T) {
	c, src, clock := newCacheRig()
	ctx := tenant.Into(context.Background(), xaA)
	_, _ = c.InForce(ctx, *clock)
	c.Invalidate(ctx)
	_, _ = c.InForce(ctx, *clock)
	if n := src.readsOf(xaA); n != 2 {
		t.Fatalf("after Invalidate the next read must reach the store (reads %d)", n)
	}

	// In flight: the read starts at t, the staff write invalidates at t+1s, the read finishes after.
	*clock = cacheT0.Add(time.Minute)
	src.during = func() {
		*clock = cacheT0.Add(time.Minute + time.Second)
		c.Invalidate(ctx)
	}
	_, _ = c.InForce(ctx, cacheT0.Add(time.Minute))
	src.during = nil
	_, _ = c.InForce(ctx, *clock)
	if n := src.readsOf(xaA); n != 4 {
		t.Fatalf("the pre-write read was cached over the invalidation (reads %d, want 4)", n)
	}
}

// Bounded: past max the cache evicts rather than grows.
func TestInForceCacheIsBounded(t *testing.T) {
	c, src, clock := newCacheRig()
	c.max = 2
	for _, id := range []tenant.ID{xaA, xaB, tenant.ID("01JC" + "CCCCCCCCCCCCCCCCCCCCCC")} {
		src.rows[id] = nil
		_, _ = c.InForce(tenant.Into(context.Background(), id), *clock)
	}
	if len(c.entries) > 2 {
		t.Fatalf("entries = %d, max 2", len(c.entries))
	}
}

// Concurrent readers and invalidations across communes: run under -race; every answer is its own
// commune's.
func TestInForceCacheConcurrent(t *testing.T) {
	src := &countingInForce{
		rows: map[tenant.ID][]domain.DisasterAlert{
			xaA: {{ID: "a-1", EffectiveFrom: cacheT0.Add(-time.Hour)}},
			xaB: {{ID: "b-1", EffectiveFrom: cacheT0.Add(-time.Hour)}},
		},
		reads: map[tenant.ID]int{},
	}
	c := NewInForceAlertCache(src)
	var wg sync.WaitGroup
	errs := make(chan string, 400)
	for i := 0; i < 200; i++ {
		id, want := xaA, "a-1"
		if i%2 == 1 {
			id, want = xaB, "b-1"
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := tenant.Into(context.Background(), id)
			if i%10 == 0 {
				c.Invalidate(ctx)
			}
			got, err := c.InForce(ctx, cacheT0)
			if err != nil || len(got) != 1 || got[0].ID != want {
				errs <- want
			}
		}()
	}
	wg.Wait()
	close(errs)
	for w := range errs {
		t.Errorf("a reader of %s got another answer", w)
	}
}
