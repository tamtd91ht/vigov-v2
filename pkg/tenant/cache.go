package tenant

import (
	"context"
	"sync"
	"time"
)

// CachedDirectory wraps a Directory with a short TTL.
//
// WHY THIS IS NOT PREMATURE: this lookup runs on EVERY request of EVERY commune. At 200+
// communes an uncached directory turns one round trip into the busiest call in the system, and
// it answers the same question all day — the Host -> commune mapping changes only when a
// commune is renamed or a domain is reassigned (ADR 0004, decision 5). For the seven services
// that are NOT the platform, that round trip is a gRPC call to another process, so the cost of
// missing is higher still.
//
// WHY THE TTL IS SHORT AND NOT ZERO: this cache is keyed by HOST, not by commune, so it holds
// no commune's business data and cannot serve one commune's rows to another — the failure mode
// rule 1 forbids. What it can do is keep serving a commune that was just deactivated, or miss a
// domain that was just reassigned. A short TTL bounds that window; Forget closes it on the
// events that matter.
//
// WHY IT LIVES IN pkg/ AND NOT IN THE PLATFORM SERVICE, which is where it used to live: it is a
// decorator over tenant.Directory with no platform logic in it at all, and every service edge
// needs one — the platform reads the registry from its own tables, the other seven read it over
// gRPC, and both wrap the result here. Left inside services/platform/internal it was unreachable
// from anywhere else (rule 2, forbidden #1), so the second service to need it would have written
// a second copy, and two caches with two invalidation rules is how a deactivated commune keeps
// being served in one process after it stopped being served in another.
//
// THE ONE THING THIS CACHE MUST NEVER BECOME: a cache keyed without the commune holding business
// data. That is rule 1, invariant 7, and it is a different thing from this. What makes THIS
// one safe is that the key IS the host and the value is only the registry answer.
type CachedDirectory struct {
	inner Directory
	ttl   time.Duration
	now   func() time.Time // injectable so tests do not sleep

	mu      sync.RWMutex
	entries map[string]cacheEntry
}

type cacheEntry struct {
	t      Tenant
	ok     bool // a miss is remembered too — see NewCachedDirectory
	hetHan time.Time
}

// NewCachedDirectory wraps inner. A ttl of zero disables caching, which is what tests that
// exercise the underlying lookup should use.
//
// MISSES ARE REMEMBERED ON PURPOSE: an unknown Host is the shape of a scan looking for communes
// that exist. Forgetting misses means every probe reaches the registry, and a scanner sets the
// query rate. The negative entry is cheap and bounded by the same TTL.
//
// THE COST OF THAT CHOICE, STATED: a lookup that failed for a TRANSPORT reason is indistinguish-
// able here from "no such commune" — Directory.ByHost returns a bool and nothing else — so a
// blip while the registry is unreachable is remembered as a miss for up to one TTL. That is
// fail-closed, which is the right direction (rule 1, invariant 3), and the implementation of
// ByHost is where the alarm has to be raised so the 404s are not silent. Telling the two apart
// inside the cache needs a Directory that reports why, and that is a wider change than this.
func NewCachedDirectory(inner Directory, ttl time.Duration) *CachedDirectory {
	return &CachedDirectory{
		inner:   inner,
		ttl:     ttl,
		now:     time.Now,
		entries: make(map[string]cacheEntry),
	}
}

func (c *CachedDirectory) ByHost(ctx context.Context, host string) (Tenant, bool) {
	if c.ttl <= 0 {
		return c.inner.ByHost(ctx, host)
	}

	c.mu.RLock()
	e, found := c.entries[host]
	c.mu.RUnlock()
	if found && c.now().Before(e.hetHan) {
		return e.t, e.ok
	}

	t, ok := c.inner.ByHost(ctx, host)

	c.mu.Lock()
	c.entries[host] = cacheEntry{t: t, ok: ok, hetHan: c.now().Add(c.ttl)}
	c.mu.Unlock()
	return t, ok
}

// Forget drops one host from the cache. Call it when a domain is reassigned or a commune is
// renamed — the TTL is a safety net, not the mechanism.
//
// This touches an in-memory map only. Nothing here reaches storage, and no commune's record is
// affected: rule 7 is about archival data, and a cache holds none.
func (c *CachedDirectory) Forget(host string) {
	c.mu.Lock()
	delete(c.entries, host)
	c.mu.Unlock()
}

// ForgetAll empties the cache. Call it when a commune is deactivated: that commune may hold
// several hosts, and serving a dissolved commune is worse than a moment of extra queries.
func (c *CachedDirectory) ForgetAll() {
	c.mu.Lock()
	c.entries = make(map[string]cacheEntry)
	c.mu.Unlock()
}
