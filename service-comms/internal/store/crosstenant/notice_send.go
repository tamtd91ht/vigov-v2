package crosstenant

// The unscoped statements of the NOTICE CAMPAIGN SENDER (ADR 0099 #8; migration 0032), the dispatcher's
// shape (zalo_bot.go): the sender needs to know WHICH communes have owed recipient rows, and ONE replica
// must send at a time. So:
//
//  1. CommunesWithDueNoticeRecipients — tenant identifiers only, never a business row.
//  2. The sender's scheduler lock — its own key, so it never contends with the Zalo dispatcher, the OA
//     refresh job, the portal sync or the sweep.
//
// Every claim, provider call, result and audit entry happens afterwards IN that commune's context through
// core/store (app/notice_sender.go).

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/vihat/vigov/core/tenant"
)

// NoticeSend is the campaign sender's unscoped handle. It holds the pool and hands none of it out.
type NoticeSend struct{ db *sql.DB }

// NewNoticeSend wraps the process pool.
func NewNoticeSend(db *sql.DB) *NoticeSend { return &NoticeSend{db: db} }

// communesWithDueNoticeRecipients — owed rows whose lease (or back-off) has run out: 0032's
// notice_recipient_pending index (tenant_id, status, lease_until) WHERE status = 'cho-gui'.
const communesWithDueNoticeRecipients = `SELECT DISTINCT tenant_id FROM notice_recipient
	WHERE status = 'cho-gui' AND (lease_until IS NULL OR lease_until <= $1)
	ORDER BY tenant_id`

// CommunesWithDueNoticeRecipients lists the communes holding at least one notice_recipient row that is
// owed and not leased at `now`. Identifiers only.
func (n *NoticeSend) CommunesWithDueNoticeRecipients(ctx context.Context, now time.Time) ([]tenant.ID, error) {
	// @cross-tenant: ADR 0099 #8 / ADR 0058 §2b — the campaign sender lists the communes with owed
	// recipient rows (tenant identifiers only, no business row, no ref, no number), then claims and sends
	// each commune's rows IN that commune's context through core/store.
	rows, err := n.db.QueryContext(ctx, communesWithDueNoticeRecipients, now.UTC())
	if err != nil {
		return nil, wrapList(err)
	}
	defer rows.Close()
	var out []tenant.ID
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, wrapList(err)
		}
		if t := tenant.ID(id); t.Valid() {
			out = append(out, t)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, wrapList(err)
	}
	return out, nil
}

// wrapList names the statement; the driver's error never carries a row of ours (identifiers only).
func wrapList(err error) error {
	return fmt.Errorf("crosstenant: list communes with owed notice recipients: %w", err)
}

// NoticeSendLockKey is the sender's scheduler lock — its own prefix.
func NoticeSendLockKey() int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte("vigov:comms:notice-send:scheduler"))
	return int64(h.Sum64())
}

// TryLockNoticeSendScheduler takes the sender's lock for one tick, without waiting. ok=false: another
// replica is sending. release is never nil.
func (n *NoticeSend) TryLockNoticeSendScheduler(ctx context.Context) (release func(), ok bool, err error) {
	return tryAdvisoryLock(ctx, n.db, NoticeSendLockKey())
}
