package http

// ipRateLimiter — "10 lượt / 5 phút / IP, đếm trong bộ nhớ từng pod" (ADR 0066 decision 2, a
// security threshold the user approved). It guards POST /api/v1/citizen-sessions, the one public
// route here that makes OUTBOUND calls (Zalo) and writes (a session) on an unauthenticated request.
//
// IN MEMORY, PER POD — the decision, and its cost stated: with N replicas a client spread across
// them gets up to N×10 attempts per window, and a restart forgets every counter. A shared limiter
// would need Redis on the public chain; that is not what was decided. There is no core/ratelimit to
// reuse yet (rule 13, invariant 7); this stays private to the route until one exists.
//
// SLIDING LOG, NOT A FIXED WINDOW: a fixed window lets 20 attempts through across a boundary.
//
// A REFUSED ATTEMPT IS NOT COUNTED, so a client that retries while blocked is unblocked when its
// oldest counted attempt leaves the window — not kept out for as long as it keeps knocking.
//
// BOUNDED MEMORY, FAIL CLOSED: the key table is swept once per window; past maxKeys distinct live
// clients a NEW client is refused (429) rather than the table growing without bound. An attacker
// with that many addresses then blocks new sign-ins on this pod for at most one window — the
// alternative is the pod running out of memory, which blocks everybody.

import (
	"sync"
	"time"
)

const (
	citizenSessionLimit  = 10
	citizenSessionWindow = 5 * time.Minute
	rateLimitMaxKeys     = 100_000
)

type ipRateLimiter struct {
	mu        sync.Mutex
	limit     int
	window    time.Duration
	maxKeys   int
	now       func() time.Time
	hits      map[string][]time.Time
	lastSweep time.Time
}

func newIPRateLimiter(limit int, window time.Duration, maxKeys int, now func() time.Time) *ipRateLimiter {
	if limit <= 0 || window <= 0 || maxKeys <= 0 {
		panic("identity/http: giới hạn tần suất cần số lượt, cửa sổ và trần khoá dương")
	}
	if now == nil {
		now = time.Now
	}
	return &ipRateLimiter{limit: limit, window: window, maxKeys: maxKeys, now: now,
		hits: map[string][]time.Time{}, lastSweep: now()}
}

// allow counts one attempt for key, or refuses it and says how long until one is allowed.
func (l *ipRateLimiter) allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	cutoff := now.Add(-l.window)
	if now.Sub(l.lastSweep) >= l.window {
		for k, ts := range l.hits {
			if len(ts) == 0 || !ts[len(ts)-1].After(cutoff) {
				delete(l.hits, k)
			}
		}
		l.lastSweep = now
	}

	ts, known := l.hits[key]
	if !known && len(l.hits) >= l.maxKeys {
		return false, l.window
	}
	i := 0
	for i < len(ts) && !ts[i].After(cutoff) {
		i++
	}
	ts = ts[i:]
	if len(ts) >= l.limit {
		l.hits[key] = ts
		return false, ts[0].Add(l.window).Sub(now)
	}
	l.hits[key] = append(ts, now)
	return true, 0
}
