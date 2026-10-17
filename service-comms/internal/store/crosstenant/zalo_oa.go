package crosstenant

// The unscoped statements of a COMMUNE'S ZALO OA (ADR 0098 #4, #6; migration 0031). The authorization
// callback has no commune Host: Zalo registers ONE callback URL per app, so the request lands on the
// platform's callback host and the commune is found FROM `state` — the one lookup 0031 declares
// cross-commune BY DESIGN. And the refresh job and the expiry sweep need to know WHICH communes have
// work, the dispatcher's shape (zalo_bot.go). So:
//
//  1. OpenAuthorizationByStateHash — state hash → (commune, request id). Identifiers only; the row's
//     verifier, the token exchange, the OA row and the audit entry all happen afterwards, IN that
//     commune's context through core/store.
//  2. CommunesWithOAToRefresh / CommunesWithOpenAuthorizations — tenant identifiers only.
//  3. The refresh job's scheduler lock, so ONE replica refreshes.
//  4. TenantByLiveOAID — live oa_id → commune, for the OA webhook (owner, 17/10/2026). Identifier only,
//     and only for an event whose signature already verified.
//
// WHAT THIS ADDS TO THE PACKAGE'S RULE ("never a read returning business rows of more than one commune"):
// 1 RESOLVES ONE commune from an unguessable key (32 CSPRNG bytes, hashed) that Zalo's redirect
// presented; a replayed state meets a USED row and is refused, never a fresh one (0031: the index is
// total, not partial on used_at).
//
// NEVER LOGGED HERE OR ANYWHERE: the state, the hash, a callback URL (ADR 0098 #13).

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/vihat/vigov/core/tenant"
)

// ZaloOA is the commune OA's unscoped handle. It holds the pool and hands none of it out.
type ZaloOA struct{ db *sql.DB }

// NewZaloOA wraps the process pool.
func NewZaloOA(db *sql.DB) *ZaloOA { return &ZaloOA{db: db} }

// openAuthorizationByStateHash uses 0031's zalo_oa_authorization_state_hash. Open AND unexpired by the
// DATABASE clock; the use case re-checks under its lock with its own.
const openAuthorizationByStateHash = `SELECT tenant_id, id FROM zalo_oa_authorization
	WHERE state_hash = $1 AND used_at IS NULL AND expires_at > now()`

// OpenAuthorizationByStateHash returns the commune and the id of the open request whose state hash is
// hash. found=false: no such open request — unknown, used or expired alike, one answer (ADR 0098 #4).
func (z *ZaloOA) OpenAuthorizationByStateHash(ctx context.Context, hash []byte) (tenant.ID, string, bool, error) {
	var tid, id string
	// @cross-tenant: ADR 0098 #4 — Zalo's OA authorization callback arrives on the platform host with no
	// commune in it; the commune is found FROM the `state` of the request it answers (0031
	// zalo_oa_authorization_state_hash, platform-wide unique). Identifiers only — the commune and the
	// row id; everything after runs in that commune's context.
	err := z.db.QueryRowContext(ctx, openAuthorizationByStateHash, hash).Scan(&tid, &id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, fmt.Errorf("crosstenant: find the authorization request: %w", err)
	}
	t := tenant.ID(tid)
	if !t.Valid() {
		return "", "", false, nil
	}
	return t, id, true, nil
}

// tenantByLiveOAID uses 0031's zalo_commune_oa_one_live_per_oa: at most ONE live row per oa_id across the
// platform, so the answer is one commune or none.
const tenantByLiveOAID = `SELECT tenant_id FROM zalo_commune_oa WHERE oa_id = $1 AND retired_at IS NULL`

// TenantByLiveOAID returns the commune holding oaID LIVE. found=false: no live row — unknown, retired or
// never connected alike, one answer.
//
// NEW USE OF THE INDEX (owner decision 17/10/2026, TASK W-1): migration 0031's comment on
// zalo_commune_oa_one_live_per_oa says nothing reads oa_id → tenant_id through it. That was true of the
// authorization callback; the OA WEBHOOK has no other key — Zalo posts every OA's events to one URL, and
// the event names its OA, never a commune. The commune is resolved from a SIGNED event (the handler
// verifies X-ZEvent-Signature before this runs), and only its identifier is returned.
func (z *ZaloOA) TenantByLiveOAID(ctx context.Context, oaID string) (tenant.ID, bool, error) {
	var tid string
	// @cross-tenant: owner decision 17/10/2026 (TASK W-1) — the Zalo OA webhook arrives on the platform host
	// with no commune in it; the commune is the one holding the event's OA live (0031
	// zalo_commune_oa_one_live_per_oa, platform-wide unique). Identifier only, after the signature check.
	err := z.db.QueryRowContext(ctx, tenantByLiveOAID, oaID).Scan(&tid)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("crosstenant: find the commune of a live OA: %w", err)
	}
	t := tenant.ID(tid)
	if !t.Valid() {
		return "", false, nil
	}
	return t, true, nil
}

// communesWithOAToRefresh — live OA rows not flagged for re-authorization whose access token expires
// before the instant given (the job passes now + the lead, ADR 0098 #6).
const communesWithOAToRefresh = `SELECT DISTINCT tenant_id FROM zalo_commune_oa
	WHERE retired_at IS NULL AND needs_reauthorization_since IS NULL AND access_expires_at < $1
	ORDER BY tenant_id`

// CommunesWithOAToRefresh lists the communes holding at least one live OA whose access token expires
// before `before` — DISTINCT, so a commune with several due OAs (ADR 0099 #1) is listed once and walks its
// own OAs in its own context. Identifiers only.
func (z *ZaloOA) CommunesWithOAToRefresh(ctx context.Context, before time.Time) ([]tenant.ID, error) {
	// @cross-tenant: ADR 0098 #6 / ADR 0058 §2b — the refresh job lists the communes whose OA access
	// token is about to expire (tenant identifiers only, no business row), then refreshes each one IN
	// that commune's context through core/store.
	return z.communes(ctx, communesWithOAToRefresh, "list communes with an OA token to refresh", before.UTC())
}

// communesWithOpenAuthorizations — open requests already past their expiry (0031
// zalo_oa_authorization_open_by_expiry), to be closed as het-han per commune.
const communesWithOpenAuthorizations = `SELECT DISTINCT tenant_id FROM zalo_oa_authorization
	WHERE used_at IS NULL AND expires_at < $1
	ORDER BY tenant_id`

// CommunesWithOpenAuthorizations lists the communes holding an open, expired authorization request.
// Identifiers only.
func (z *ZaloOA) CommunesWithOpenAuthorizations(ctx context.Context, now time.Time) ([]tenant.ID, error) {
	// @cross-tenant: ADR 0098 #4 / ADR 0058 §2b — the sweep lists the communes with an expired open
	// authorization request (tenant identifiers only), then closes each one IN that commune's context.
	return z.communes(ctx, communesWithOpenAuthorizations, "list communes with expired authorization requests", now.UTC())
}

func (z *ZaloOA) communes(ctx context.Context, stmt, what string, args ...any) ([]tenant.ID, error) {
	// @cross-tenant: the two callers above each carry their own reason; this is their shared scan of
	// tenant identifiers, never a business row.
	rows, err := z.db.QueryContext(ctx, stmt, args...)
	if err != nil {
		return nil, fmt.Errorf("crosstenant: %s: %w", what, err)
	}
	defer rows.Close()
	var out []tenant.ID
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("crosstenant: %s: %w", what, err)
		}
		if t := tenant.ID(id); t.Valid() {
			out = append(out, t)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("crosstenant: %s: %w", what, err)
	}
	return out, nil
}

// ZaloOARefreshLockKey is the refresh job's scheduler lock — its own prefix, so it never contends with
// the dispatcher's, the portal sync's or the sweep's.
func ZaloOARefreshLockKey() int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte("vigov:comms:zalo-oa-refresh:scheduler"))
	return int64(h.Sum64())
}

// TryLockRefreshScheduler takes the refresh job's lock for one tick, without waiting. ok=false: another
// replica is refreshing. release is never nil.
func (z *ZaloOA) TryLockRefreshScheduler(ctx context.Context) (release func(), ok bool, err error) {
	return tryAdvisoryLock(ctx, z.db, ZaloOARefreshLockKey())
}
