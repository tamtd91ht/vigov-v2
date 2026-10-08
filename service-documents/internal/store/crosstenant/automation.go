// Package crosstenant holds service-documents' database statements that do NOT carry a commune —
// and nothing else. Same purpose as service-petitions/internal/store/crosstenant (ADR 0021): an
// unscoped statement must step outside core/store.For(ctx), and that step has to be COUNTABLE, in a
// directory named after what it does. Every statement here carries its own `// @cross-tenant:`.
//
// TWO STATEMENTS TODAY, both for the automation runner (ADR 0058):
//
//  1. the list of communes this service holds incoming documents or citizen letters for — tenant
//     IDENTIFIERS only, never a row of business data (ADR 0058 §2b; identity.proto
//     ClaimDueAutomationRuns, "HOW THE RUNNER FINDS COMMUNES TO ASK ABOUT");
//  2. the advisory locks that keep one replica per job (ADR 0058 §1) — they read no table at all.
//
// WHAT MAY NOT BE ADDED HERE: any read returning business rows of more than one commune (open
// question #4 on rollups is still open), and any handle to the raw *sql.DB leaving this package.
package crosstenant

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"hash/fnv"

	"github.com/vihat/vigov/core/tenant"
)

// Automation is the runner's unscoped handle. It holds the pool and hands none of it out.
type Automation struct{ db *sql.DB }

// NewAutomation wraps the process pool.
func NewAutomation(db *sql.DB) *Automation { return &Automation{db: db} }

// communesQuery walks the leading column of the register's primary key (tenant_id, id) as a loose
// index scan — one index probe per commune — instead of a DISTINCT over every row, which would read the
// whole register once a minute.
//
// `van_ban_den` AND `citizen_letter`: the two registers here with a commitment a job speaks about
// (ADR 0079 lô 5 Q18 added the letters' clerk-set deadline). The outgoing register carries no deadline.
// One loose scan per register, merged: a commune booking only letters must still be asked about —
// otherwise its letter reminders would never run, silently. A commune with neither gets no documents
// part of the digest — the cost ADR 0058 §2b states.
//
// SOFT-DELETED ROWS ARE NOT FILTERED HERE: this answers "which identifiers occur", nothing more. Every
// per-commune read excludes deleted rows (rule 7, invariant 2).
const communesQuery = `WITH RECURSIVE
	d(id) AS (
		SELECT min(tenant_id) FROM van_ban_den
		UNION ALL
		SELECT (SELECT min(tenant_id) FROM van_ban_den WHERE tenant_id > d.id) FROM d WHERE d.id IS NOT NULL),
	l(id) AS (
		SELECT min(tenant_id) FROM citizen_letter
		UNION ALL
		SELECT (SELECT min(tenant_id) FROM citizen_letter WHERE tenant_id > l.id) FROM l WHERE l.id IS NOT NULL)
	SELECT id FROM d WHERE id IS NOT NULL
	UNION
	SELECT id FROM l WHERE id IS NOT NULL
	ORDER BY 1`

// CommunesWithRecords lists the communes holding at least one incoming document or citizen letter
// here. Identifiers that are not ULID-shaped are dropped: they cannot name a commune.
func (a *Automation) CommunesWithRecords(ctx context.Context) ([]tenant.ID, error) {
	// @cross-tenant: ADR 0058 §2b — the automation runner lists the communes it holds records for
	// (tenant identifiers only, no business row), then asks identity about each one IN that commune's
	// context; there is deliberately no "which communes are enabled" RPC (ADR 0012 decision 1).
	rows, err := a.db.QueryContext(ctx, communesQuery)
	if err != nil {
		return nil, fmt.Errorf("crosstenant: liệt kê xã có văn bản đến hoặc đơn thư: %w", err)
	}
	defer rows.Close()
	var out []tenant.ID
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("crosstenant: liệt kê xã có văn bản đến hoặc đơn thư: đọc dòng: %w", err)
		}
		if t := tenant.ID(id); t.Valid() {
			out = append(out, t)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("crosstenant: liệt kê xã có văn bản đến hoặc đơn thư: %w", err)
	}
	return out, nil
}

// LockKey is the advisory-lock key of one job in THIS service. The prefix differs from the petitions
// runner's, so the two can never contend even if they ever shared a database. FNV-1a, not a
// cryptographic hash — it names a lock, it protects nothing.
func LockKey(job string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte("vigov:documents:automation:" + job))
	return int64(h.Sum64())
}

// TryLockJobs tries, without waiting, the advisory lock of each job, on ONE pinned connection held
// until release is called (pg_try_advisory_lock is session-scoped: on a pooled handle the unlock could
// land on another connection — core/migrate/migrate.go says the same).
//
// held[job] is true for the jobs this replica won; another replica holding one is an ordinary answer.
// release unlocks what was won and returns the connection; it is never nil, and it runs on a context
// that cannot be cancelled, so a shutdown mid-tick does not leave a lock held by a pooled connection.
//
// NOT AUDITED, and not a write of business data: a session lock is a lease for one tick. The run's
// trail is identity's RecordAutomationRunOutcome entry, and every notice is audited where comms writes
// it (rule 6, invariant 6).
func (a *Automation) TryLockJobs(ctx context.Context, jobs []string) (map[string]bool, func(), error) {
	noop := func() {}
	if len(jobs) == 0 {
		return map[string]bool{}, noop, errors.New("crosstenant: không có việc nào để khoá")
	}
	conn, err := a.db.Conn(ctx)
	if err != nil {
		return nil, noop, fmt.Errorf("crosstenant: lấy kết nối riêng cho khoá việc nền: %w", err)
	}
	held := make(map[string]bool, len(jobs))
	release := func() {
		bg := context.WithoutCancel(ctx)
		for job, ok := range held {
			if !ok {
				continue
			}
			// @cross-tenant: an advisory lock names a JOB, reads no table and no commune (ADR 0058 §1).
			_, _ = conn.ExecContext(bg, "SELECT pg_advisory_unlock($1)", LockKey(job))
		}
		_ = conn.Close()
	}
	for _, job := range jobs {
		var ok bool
		// @cross-tenant: an advisory lock names a JOB, reads no table and no commune (ADR 0058 §1).
		if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", LockKey(job)).Scan(&ok); err != nil {
			release()
			return nil, noop, fmt.Errorf("crosstenant: thử khoá việc nền %s: %w", job, err)
		}
		held[job] = ok
	}
	return held, release, nil
}
