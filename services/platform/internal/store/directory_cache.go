package store

import (
	"context"
	"sync"
	"time"

	"github.com/vihat/vigov/pkg/tenant"
)

// CachedDirectory wraps a Directory with a short TTL.
//
// WHY THIS IS NOT PREMATURE: this lookup runs on EVERY request of EVERY commune. At 200+
// communes an uncached directory turns one database round trip into the busiest query in the
// system, and it answers the same question all day — the Host -> commune mapping changes only
// when a commune is renamed or a domain is reassigned (ADR 0004, decision 5).
//
// WHY THE TTL IS SHORT AND NOT ZERO: this cache is keyed by HOST, not by commune, so it holds
// no commune's business data and cannot serve one commune's rows to another — the failure mode
// rule 1 forbids. What it can do is keep serving a commune that was just deactivated, or miss
// a domain that was just reassigned. A short TTL bounds that window; Invalidate closes it on
// the events that matter.
//
// This is also why the cache lives HERE and not in pkg/: a process-wide cache keyed without
// tenant_id is exactly the mistake rule 1 forbids, and keeping this one next to the only table
// that legitimately has no tenant_id keeps that distinction visible.
type CachedDirectory struct {
	inner tenant.Directory
	ttl   time.Duration
	now   func() time.Time // injectable so tests do not sleep

	mu      sync.RWMutex
	entries map[string]cacheEntry
}

type cacheEntry struct {
	t      tenant.Tenant
	ok     bool // a miss is remembered too — see NewCachedDirectory
	hetHan time.Time
}

// NewCachedDirectory wraps inner. A ttl of zero disables caching, which is what tests that
// exercise the underlying query should use.
//
// MISSES ARE REMEMBERED ON PURPOSE: an unknown Host is the shape of a scan looking for
// communes that exist. Forgetting misses means every probe reaches the database, and a scanner
// sets the query rate. The negative entry is cheap and bounded by the same TTL.
func NewCachedDirectory(inner tenant.Directory, ttl time.Duration) *CachedDirectory {
	return &CachedDirectory{
		inner:   inner,
		ttl:     ttl,
		now:     time.Now,
		entries: make(map[string]cacheEntry),
	}
}

func (c *CachedDirectory) ByHost(ctx context.Context, host string) (tenant.Tenant, bool) {
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
// This touches an in-memory map only. Nothing here reaches the database, and no commune's
// record is affected: rule 7 is about archival data, and a cache holds none.
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
