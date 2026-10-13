package store

import (
	"context"
	"fmt"

	corestore "github.com/vihat/vigov/core/store"
)

// CommuneLaunchStore is the COMMUNE-HOST read of what the commune's one QR link needs (owner
// 13/10/2026, GET /api/v1/commune-launch-link): whether the commune is active, its launch code, and its
// live own Mini App(s).
//
// BUILT FROM *corestore.DB AND READ ONLY THROUGH Scoped: the commune is the one the edge resolved from
// Host and put in ctx (rule 1, invariants 4 and 5). No method takes a commune — the operator console's
// equivalent (SharedMiniAppStore.CommuneLaunchTarget) takes the PATH's id on the operator realm, which a
// commune-host route must never reach.
//
// The shared app is NOT read here: its row has no commune (migration 0006), so it stays on
// SharedMiniAppStore.SharedMiniApp.
type CommuneLaunchStore struct {
	db *corestore.DB
}

// NewCommuneLaunchStore builds the store over the scoped handle.
func NewCommuneLaunchStore(db *corestore.DB) *CommuneLaunchStore { return &CommuneLaunchStore{db: db} }

// CommuneLaunch is what the commune's QR link is chosen from.
type CommuneLaunch struct {
	Active bool
	// LaunchCode is tenant.launch_code (migration 0024) — what the shared app's `x=` link carries.
	LaunchCode string
	// LiveOwnAppIDs are the commune's running, not-deleted 'rieng' App IDs, at most TWO (the second only
	// to see ambiguity: ADR 0070 #1 forbids it, no constraint prevents it).
	LiveOwnAppIDs []string
}

// tenant has no tenant_id column — its id IS the commune — so `t.id = $1` is the scope (QueryJoin's
// contract: $1 is the commune).
const communeLaunchStmt = `SELECT t.dang_hoat_dong, t.launch_code FROM tenant t WHERE t.id = $1`

// CommuneLaunch reads the commune in ctx. ErrCommuneNotFound when the registry has no row for it — not
// reachable behind TenantMiddleware, which resolved it a moment ago, so it is a 500 to the caller, never
// a "no link". Panics when ctx carries no commune — tenant.MustFrom, deliberately.
func (s *CommuneLaunchStore) CommuneLaunch(ctx context.Context) (CommuneLaunch, error) {
	scoped := s.db.For(ctx)
	rows, err := scoped.QueryJoin(ctx, communeLaunchStmt)
	if err != nil {
		return CommuneLaunch{}, fmt.Errorf("commune launch: read commune: %w", err)
	}
	var out CommuneLaunch
	found := rows.Next()
	if found {
		if err := rows.Scan(&out.Active, &out.LaunchCode); err != nil {
			_ = rows.Close()
			return CommuneLaunch{}, fmt.Errorf("commune launch: scan commune: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return CommuneLaunch{}, fmt.Errorf("commune launch: read commune: %w", err)
	}
	if err := rows.Close(); err != nil {
		return CommuneLaunch{}, fmt.Errorf("commune launch: read commune: %w", err)
	}
	if !found {
		return CommuneLaunch{}, ErrCommuneNotFound
	}

	apps, err := scoped.Query(ctx, "app_id", "mini_app",
		`AND che_do = 'rieng' AND dang_hoat_dong AND deleted_at IS NULL ORDER BY app_id LIMIT 2`)
	if err != nil {
		return CommuneLaunch{}, fmt.Errorf("commune launch: read own mini app: %w", err)
	}
	defer apps.Close()
	for apps.Next() {
		var id string
		if err := apps.Scan(&id); err != nil {
			return CommuneLaunch{}, fmt.Errorf("commune launch: scan own mini app: %w", err)
		}
		out.LiveOwnAppIDs = append(out.LiveOwnAppIDs, id)
	}
	if err := apps.Err(); err != nil {
		return CommuneLaunch{}, fmt.Errorf("commune launch: read own mini app: %w", err)
	}
	return out, nil
}
