package app

// THE IN-FORCE CACHE behind the public disaster-alert read (owner decision 11/10/2026, task TASK-08):
// GET /api/v1/commune-disaster-alerts serves residents at up to 1200 requests per minute per (host,
// network) — the whole hamlet opening the Mini App during a storm — and without a cache every one of them
// is a database statement. The owner ACCEPTED A MAXIMUM STALENESS OF 30 SECONDS: an alert posted (or
// started) reaches the Mini App up to 30 s later on a pod that already holds the commune's list.
//
// WHAT KEEPS IT CORRECT WITHOUT INVALIDATION — the owner's condition ("correctness must not depend on it"):
//
//	an entry lives 30 s from the moment its read STARTED, never from when it finished, so a slow read can
//	not stretch the bound; and every hit RE-FILTERS the cached rows with domain.DisasterAlert.InForce at
//	the caller's `now`, so an alert whose `effective_to` passes leaves the reply ON TIME even from the
//	cache. Only what the cache cannot know — a new row, an edit, an early end, a delete — waits for expiry.
//
// Invalidate is a courtesy for the pod that took the staff write (the officer who posts sees it at once
// there); other pods expire within the bound. It leaves a stamp, so a read that STARTED before the write
// and finishes after it cannot put the pre-write list back.
//
// ISOLATION (rule 1, invariant 7): one entry per commune, keyed `t:<tenant_id>`, the commune taken from
// ctx (invariant 4). No commune in ctx is a REFUSAL — never an unscoped entry every caller would share.
// An unknown host never reaches here: the handler answers it before asking for a list.
//
// BOUNDED (maxEntries): one entry per commune that was asked for. When full, expired entries are dropped
// first; if none is expired, an arbitrary one is — a dropped entry costs one database read, never a wrong
// answer. Errors are never cached: a failing store is asked again on the next request.

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// PublicDisasterAlertCacheTTL is the owner's accepted staleness (11/10/2026). Not configuration: it is a
// promise about how late a warning may appear, and a deployment must not lengthen it silently.
const PublicDisasterAlertCacheTTL = 30 * time.Second

// publicDisasterAlertCacheMax bounds the entries — one per commune asked about. Sized above the number of
// communes one deployment serves; past it the cache degrades to more reads, never to more memory.
const publicDisasterAlertCacheMax = 4096

// InForceAlertReader is the store's public read. *commsstore.DisasterAlertStore satisfies it.
type InForceAlertReader interface {
	InForce(ctx context.Context, now time.Time) ([]domain.DisasterAlert, error)
}

// ErrInForceCacheNoCommune — the cache was asked with no commune in ctx: a wiring fault on the isolation
// path, refused rather than answered from a shared entry (rule 1, forbidden #1).
var ErrInForceCacheNoCommune = errors.New("disaster_alert: bộ đệm cảnh báo đang hiệu lực được hỏi khi không có xã trong ngữ cảnh")

type inForceEntry struct {
	// stamp is when the read behind rows STARTED, or when the entry was invalidated (valid=false).
	stamp time.Time
	valid bool
	rows  []domain.DisasterAlert
	// overCeiling: the store returned ErrTooManyAlertsInForce beside rows; a hit answers the same.
	overCeiling bool
}

// InForceAlertCache is a per-commune, 30-second cache over InForceAlertReader. Safe for concurrent use.
type InForceAlertCache struct {
	next InForceAlertReader
	ttl  time.Duration
	max  int
	// clock decides expiry — injected so a test can move it. In production time.Now.
	clock func() time.Time

	mu      sync.Mutex
	entries map[string]inForceEntry
}

// NewInForceAlertCache wraps next. A nil reader is a wiring fault and panics at startup.
func NewInForceAlertCache(next InForceAlertReader) *InForceAlertCache {
	if next == nil {
		panic("comms/app: bộ đệm cảnh báo đang hiệu lực thiếu kho đọc")
	}
	return &InForceAlertCache{next: next, ttl: PublicDisasterAlertCacheTTL, max: publicDisasterAlertCacheMax,
		clock: time.Now, entries: map[string]inForceEntry{}}
}

// cacheKey is `t:<tenant_id>` from ctx (rule 1, invariants 4 and 7).
func inForceCacheKey(ctx context.Context) (string, error) {
	id, ok := tenant.From(ctx)
	if !ok || !id.Valid() {
		return "", ErrInForceCacheNoCommune
	}
	return "t:" + string(id), nil
}

// InForce answers the commune's alerts in force at now — from the cache when its entry is younger than the
// TTL, re-filtered at now; otherwise from the store, remembered. Same contract as the store's InForce,
// ErrTooManyAlertsInForce beside the rows included.
func (c *InForceAlertCache) InForce(ctx context.Context, now time.Time) ([]domain.DisasterAlert, error) {
	key, err := inForceCacheKey(ctx)
	if err != nil {
		return nil, err
	}
	started := c.clock()

	c.mu.Lock()
	e, ok := c.entries[key]
	c.mu.Unlock()
	if ok && e.valid && started.Sub(e.stamp) < c.ttl {
		return answerInForce(e.rows, e.overCeiling, now)
	}

	rows, err := c.next.InForce(ctx, now)
	over := errors.Is(err, commsstore.ErrTooManyAlertsInForce)
	if err != nil && !over {
		return nil, fmt.Errorf("disaster_alert: đọc cảnh báo đang hiệu lực (bộ đệm): %w", err)
	}
	c.store(key, inForceEntry{stamp: started, valid: true, rows: rows, overCeiling: over})
	return answerInForce(rows, over, now)
}

// answerInForce copies the cached slice (a caller may not alias the entry) keeping only what is still in
// force at now.
func answerInForce(rows []domain.DisasterAlert, over bool, now time.Time) ([]domain.DisasterAlert, error) {
	out := make([]domain.DisasterAlert, 0, len(rows))
	for _, a := range rows {
		if a.InForce(now) {
			out = append(out, a)
		}
	}
	if over {
		return out, commsstore.ErrTooManyAlertsInForce
	}
	return out, nil
}

// store keeps e unless the entry already there is NEWER — a read that started earlier, or an invalidation
// that came after this read began. Last-STARTED wins, never last-finished.
func (c *InForceAlertCache) store(key string, e inForceEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if old, ok := c.entries[key]; ok && old.stamp.After(e.stamp) {
		return
	}
	if _, ok := c.entries[key]; !ok && len(c.entries) >= c.max {
		c.evictLocked(e.stamp)
	}
	c.entries[key] = e
}

// evictLocked frees at least one slot: every expired entry or invalidation stamp, and if none, one
// arbitrary entry.
func (c *InForceAlertCache) evictLocked(now time.Time) {
	for k, e := range c.entries {
		if now.Sub(e.stamp) >= c.ttl {
			delete(c.entries, k)
		}
	}
	if len(c.entries) < c.max {
		return
	}
	for k := range c.entries {
		delete(c.entries, k)
		return
	}
}

// Invalidate drops the commune's entry on THIS pod after a staff write, leaving a stamp so that a read
// already in flight cannot restore the older list. No commune in ctx: nothing to drop.
func (c *InForceAlertCache) Invalidate(ctx context.Context) {
	key, err := inForceCacheKey(ctx)
	if err != nil {
		return
	}
	c.store(key, inForceEntry{stamp: c.clock(), valid: false})
}
