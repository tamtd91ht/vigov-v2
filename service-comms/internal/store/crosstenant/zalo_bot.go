package crosstenant

// The unscoped statements of the shared Zalo Bot's COMMUNE side (ADR 0074 #5; migration 0018). The webhook
// has no commune Host: "xã lấy từ mã ghép rồi từ liên kết; không ra xã thì bỏ gói, không đoán". So:
//
//  1. OpenPairingCodesByHash — hash → (commune, code id). At most TWO rows are read, and the caller treats
//     more than one as NO match (0018: "MORE THAN ONE open match is answered as NO match").
//  2. CommuneOfLiveChat — chat → commune, through the platform-wide one-live-link-per-chat index.
//  3. EndChatLinksInOtherCommunes — a pairing takes a chat that is live in ANOTHER commune: that link is
//     ended in the SAME transaction as the new one (0018: "pairing a chat that is live elsewhere ends the
//     old link (unlinked_by 'system') in the same transaction, before inserting the new one"), and the end
//     is audited in THAT commune's audit_log, never in the pairing commune's.
//  4. CommunesWithDueZaloDeliveries + the dispatcher's scheduler lock — identifiers only, the portal sync
//     runner's shape.
//
// WHAT THIS ADDS TO THE PACKAGE'S RULE ("never a read returning business rows of more than one commune"):
// 1 and 2 each RESOLVE ONE commune from an unguessable key a chat presented; they return identifiers and
// never a row of a second commune. 3 is a write across communes, and it takes the CALLER's transaction —
// service-identity's crosstenant.DinhDanhStore precedent — so it commits with the new link or not at all.
//
// chat_id is personal data (ADR 0074): it is a bind parameter here and nothing else, never in an error.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// ZaloBot is the shared bot's unscoped handle. It holds the pool and hands none of it out.
type ZaloBot struct{ db *sql.DB }

// NewZaloBot wraps the process pool.
func NewZaloBot(db *sql.DB) *ZaloBot { return &ZaloBot{db: db} }

// PairingCodeMatch is one open code found by its hash: which commune, which row.
type PairingCodeMatch struct {
	TenantID tenant.ID
	CodeID   string
}

// ErrNoTransaction — a cross-commune write was asked for with no caller transaction to share.
var ErrNoTransaction = errors.New("crosstenant: the caller's transaction is required")

// openPairingCodesByHash uses 0018's zalo_pairing_code_open_by_hash. LIMIT 2: the caller needs to know
// only "exactly one" versus "not exactly one".
const openPairingCodesByHash = `SELECT tenant_id, id FROM zalo_pairing_code
	WHERE code_hash = $1 AND used_at IS NULL AND cancelled_at IS NULL
	ORDER BY tenant_id, id LIMIT 2`

// OpenPairingCodesByHash returns the open codes whose hash is hash — at most two.
func (z *ZaloBot) OpenPairingCodesByHash(ctx context.Context, hash []byte) ([]PairingCodeMatch, error) {
	// @cross-tenant: ADR 0074 #5 — the webhook has no commune Host; the commune is found FROM the pairing
	// code a chat typed (0018 zalo_pairing_code_open_by_hash). Identifiers only, at most two rows, and the
	// caller refuses anything but exactly one.
	rows, err := z.db.QueryContext(ctx, openPairingCodesByHash, hash)
	if err != nil {
		return nil, fmt.Errorf("crosstenant: find pairing code: %w", err)
	}
	defer rows.Close()
	var out []PairingCodeMatch
	for rows.Next() {
		var tid, id string
		if err := rows.Scan(&tid, &id); err != nil {
			return nil, fmt.Errorf("crosstenant: find pairing code: %w", err)
		}
		if t := tenant.ID(tid); t.Valid() {
			out = append(out, PairingCodeMatch{TenantID: t, CodeID: id})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("crosstenant: find pairing code: %w", err)
	}
	return out, nil
}

// CommuneOfLiveChat returns the commune holding the live link of chatID on the shared bot. found=false:
// the chat is not linked anywhere.
func (z *ZaloBot) CommuneOfLiveChat(ctx context.Context, chatID string) (tenant.ID, bool, error) {
	var tid string
	// @cross-tenant: ADR 0074 #5 — a command from a chat (/dung) names no commune; the commune is found
	// FROM the link, through 0018's platform-wide zalo_link_one_live_per_chat. One identifier.
	err := z.db.QueryRowContext(ctx, `SELECT tenant_id FROM zalo_link
		WHERE bot_ref = $1 AND chat_id = $2 AND unlinked_at IS NULL`, domain.SharedZaloBotRef, chatID).Scan(&tid)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("crosstenant: find the commune of a chat: %w", err)
	}
	t := tenant.ID(tid)
	if !t.Valid() {
		return "", false, nil
	}
	return t, true, nil
}

// endChatLinksElsewhere ends the live link of one chat in every commune but the pairing one, and writes,
// IN THE SAME STATEMENT, one entry per ended link into THAT link's own commune audit_log — attributed to
// the system (0018: "core/audit.SystemActor ('system') when the chat was taken by a pairing in ANOTHER
// commune — that commune's staff code is never written into this commune's row").
//
// $1 bot · $2 chat · $3 the pairing commune (kept) · $4 time · $5 reason · $6 actor · $7 kind ·
// $8 action · $9 delta
const endChatLinksElsewhere = `WITH ended AS (
		UPDATE zalo_link SET unlinked_at = $4, unlinked_by = $6, unlink_reason = $5, updated_at = $4
		WHERE bot_ref = $1 AND chat_id = $2 AND tenant_id <> $3 AND unlinked_at IS NULL
		RETURNING tenant_id, staff_code
	), trail AS (
		INSERT INTO audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
		SELECT tenant_id, $6, $7, '', $8, staff_code, $4, $9::jsonb FROM ended
		RETURNING tenant_id
	)
	SELECT count(*) FROM trail`

// EndChatLinksInOtherCommunes runs endChatLinksElsewhere inside the CALLER's transaction — the pairing
// commune's ScopedTx, whose commune is the one KEPT — so the end and the new link commit together.
// Returns how many links it ended.
func (z *ZaloBot) EndChatLinksInOtherCommunes(ctx context.Context, tx *store.ScopedTx, chatID string,
	at time.Time) (int, error) {

	if tx == nil {
		return 0, ErrNoTransaction
	}
	keep := tx.TenantID()
	delta, err := json.Marshal(map[string]any{
		"ly_do":   domain.ZaloLinkEndedByChatTaken,
		"nguon":   "ghep_o_xa_khac",
		"bot_ref": domain.SharedZaloBotRef,
	})
	if err != nil {
		return 0, fmt.Errorf("crosstenant: encode the link-end delta: %w", err)
	}
	var n int
	// @cross-tenant: 0018 zalo_link_one_live_per_chat (ADR 0074 "một chat một tài khoản") — a chat paired
	// in this commune ends its live link in any OTHER commune, in the same transaction, each end audited
	// in its own commune (tenant_id of the row it ends) by the system principal.
	err = tx.Underlying().QueryRowContext(ctx, endChatLinksElsewhere, domain.SharedZaloBotRef, chatID, string(keep), at.UTC(),
		domain.ZaloLinkEndedByChatTaken, audit.SystemActor, "system", domain.ActionEndZaloLink, delta).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("crosstenant: end the chat's links elsewhere: %w", err)
	}
	return n, nil
}

// dueZaloCommunes — the communes holding an owed, due message. Served by 0018's zalo_delivery_due.
const dueZaloCommunes = `SELECT DISTINCT tenant_id FROM zalo_delivery
	WHERE status = 'cho-gui' AND deleted_at IS NULL AND next_attempt_at <= $1
	ORDER BY tenant_id`

// CommunesWithDueZaloDeliveries lists the communes the dispatcher has work in. Identifiers only.
func (z *ZaloBot) CommunesWithDueZaloDeliveries(ctx context.Context, now time.Time) ([]tenant.ID, error) {
	// @cross-tenant: ADR 0074 / ADR 0058 §2b — the Zalo dispatcher lists the communes holding an owed
	// message (tenant identifiers only, no business row), then sends each one IN that commune's context
	// through core/store.
	rows, err := z.db.QueryContext(ctx, dueZaloCommunes, now.UTC())
	if err != nil {
		return nil, fmt.Errorf("crosstenant: list communes with Zalo deliveries due: %w", err)
	}
	defer rows.Close()
	var out []tenant.ID
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("crosstenant: list communes with Zalo deliveries due: %w", err)
		}
		if t := tenant.ID(id); t.Valid() {
			out = append(out, t)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("crosstenant: list communes with Zalo deliveries due: %w", err)
	}
	return out, nil
}

// ZaloDispatchLockKey is the dispatcher scheduler's lock — its own prefix, so it never contends with the
// portal sync's or the sweep's.
func ZaloDispatchLockKey() int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte("vigov:comms:zalo-dispatch:scheduler"))
	return int64(h.Sum64())
}

// TryLockScheduler takes the dispatcher's lock for one tick, without waiting. ok=false: another replica
// is sending. release is never nil.
func (z *ZaloBot) TryLockScheduler(ctx context.Context) (release func(), ok bool, err error) {
	return tryAdvisoryLock(ctx, z.db, ZaloDispatchLockKey())
}
