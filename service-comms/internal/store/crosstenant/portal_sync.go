// Package crosstenant holds service-comms' database statements that do NOT carry a commune through
// core/store — and nothing else. The same purpose as service-documents' and service-petitions'
// packages of this name (ADR 0021, ADR 0058): an unscoped statement must step outside
// core/store.For(ctx), and that step has to be COUNTABLE, in a directory named after what it does.
// Every statement here carries its own `// @cross-tenant:`.
//
// THREE THINGS, all for the portal sync runner (ADR 0067 §2 "Việc nền", ADR 0058's pattern):
//
//  1. the communes whose sync is due — tenant IDENTIFIERS only, never a row of business data;
//  2. the scheduler's advisory lock, so ONE replica runs a tick;
//  3. the per-commune advisory lock 0013 requires BEFORE a run's row is inserted (its "WHAT THIS FILE
//     DOES NOT STOP" block): two runs at once for one commune — scheduler and `⟳ Đồng bộ ngay`, or two
//     pods — would double-count, and a "one unfinished run" unique index would lock a commune out for
//     ever after one crash.
//
// The locks are SESSION locks held on ONE pinned connection for as long as a run lasts: a run is
// minutes of network I/O, which no transaction may span, and a crashed pod's connection closes and
// frees the lock with it — no lease to expire, no row to reap before the next run can start.
//
// WHAT MAY NOT BE ADDED HERE: any read returning business rows of more than one commune, and any
// handle to the raw *sql.DB leaving this package.
package crosstenant

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"

	"github.com/vihat/vigov/core/tenant"
)

// PortalSync is the runner's unscoped handle. It holds the pool and hands none of it out.
type PortalSync struct{ db *sql.DB }

// NewPortalSync wraps the process pool.
func NewPortalSync(db *sql.DB) *PortalSync { return &PortalSync{db: db} }

// dueCommunesQuery — enabled, scheduled (interval > 0), and never run or last started at least one
// interval ago, by the DATABASE clock (the one every replica shares). One row per commune by key.
//
// MOST OVERDUE FIRST (R1, 02/10/2026): never-run communes, then the oldest last_run_at; tenant_id only
// breaks ties. A tick cut by its time budget leaves the rest due, and they lead the next tick — ordered
// by tenant_id alone, the same communes at the end would be the ones cut every time.
const dueCommunesQuery = `SELECT tenant_id FROM portal_sync_settings
	WHERE is_enabled AND interval_hours > 0
	  AND (last_run_at IS NULL OR last_run_at + make_interval(hours => interval_hours) <= now())
	ORDER BY last_run_at NULLS FIRST, tenant_id`

// DueCommunes lists the communes whose scheduled sync is due. Identifiers that are not ULID-shaped
// are dropped: they cannot name a commune.
func (p *PortalSync) DueCommunes(ctx context.Context) ([]tenant.ID, error) {
	// @cross-tenant: ADR 0067 §2 / ADR 0058 §2b — the portal sync runner lists the communes whose
	// sync is due (tenant identifiers only, no business row), then runs each one IN that commune's
	// context through core/store.
	rows, err := p.db.QueryContext(ctx, dueCommunesQuery)
	if err != nil {
		return nil, fmt.Errorf("crosstenant: list communes due a portal sync: %w", err)
	}
	defer rows.Close()
	var out []tenant.ID
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("crosstenant: list communes due a portal sync: %w", err)
		}
		if t := tenant.ID(id); t.Valid() {
			out = append(out, t)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("crosstenant: list communes due a portal sync: %w", err)
	}
	return out, nil
}

// lockKey names one advisory lock of THIS service's portal sync. The prefix differs from every other
// runner's, so they never contend. FNV-1a names a lock; it protects nothing.
func lockKey(name string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte("vigov:comms:portal-sync:" + name))
	return int64(h.Sum64())
}

// SchedulerLockKey / CommuneLockKey are exported for the tests that assert the two never coincide.
func SchedulerLockKey() int64           { return lockKey("scheduler") }
func CommuneLockKey(id tenant.ID) int64 { return lockKey("commune:" + string(id)) }

// TryLockScheduler takes the scheduler's lock for one tick, without waiting. ok=false: another replica
// is ticking. release is never nil and runs on a context that cannot be cancelled.
func (p *PortalSync) TryLockScheduler(ctx context.Context) (release func(), ok bool, err error) {
	return p.tryLock(ctx, SchedulerLockKey())
}

// TryLockCommune takes the commune's run lock, without waiting. ok=false: a run is in progress for
// this commune somewhere (409 for `⟳ Đồng bộ ngay`, skip for the scheduler).
func (p *PortalSync) TryLockCommune(ctx context.Context, id tenant.ID) (release func(), ok bool, err error) {
	return p.tryLock(ctx, CommuneLockKey(id))
}

// tryLock holds a session advisory lock on ONE pinned connection until release. pg_try_advisory_lock
// is session-scoped: on a pooled handle the unlock could land on another connection
// (service-documents/internal/store/crosstenant/automation.go says the same).
//
// NOT AUDITED, and not a write of business data: a session lock is a lease. What a run did is its
// `portal_sync_runs` row and the audit entries of its imports.
func (p *PortalSync) tryLock(ctx context.Context, key int64) (func(), bool, error) {
	noop := func() {}
	conn, err := p.db.Conn(ctx)
	if err != nil {
		return noop, false, fmt.Errorf("crosstenant: pin a connection for the portal sync lock: %w", err)
	}
	var ok bool
	// @cross-tenant: an advisory lock names a job or a commune's run, reads no table (ADR 0058 §1).
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&ok); err != nil {
		_ = conn.Close()
		return noop, false, fmt.Errorf("crosstenant: try the portal sync lock: %w", err)
	}
	if !ok {
		_ = conn.Close()
		return noop, false, nil
	}
	release := func() {
		bg := context.WithoutCancel(ctx)
		// @cross-tenant: an advisory lock names a job or a commune's run, reads no table (ADR 0058 §1).
		_, _ = conn.ExecContext(bg, "SELECT pg_advisory_unlock($1)", key)
		_ = conn.Close()
	}
	return release, true, nil
}
