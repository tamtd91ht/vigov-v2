package store

// The commune's map frame — migrations/0016_map_frame.sql, `map_frame` (ADR 0072 H3).
//
// THREE THINGS HOLD ACROSS EVERY METHOD, as in mail_settings.go:
//
//  1. THE COMMUNE IS $1 IN EVERY STATEMENT, from Scoped / tx.TenantID(), never a parameter.
//  2. `tenant_id`, `created_at` AND `created_by` APPEAR IN NO SET LIST: the guard trigger refuses a
//     change too, and their absence here keeps that floor unreachable from this service.
//  3. NOTHING HERE OPENS A TRANSACTION. The use case opens it and writes the audit entry inside.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// mapFrameColumns IS READ BY POSITION in readOneMapFrame. Three adjacent numerics: a swap of lat and
// lng produces no error, only a frame in the wrong place.
const mapFrameColumns = `center_lat, center_lng, radius_km, created_at, created_by, updated_at, updated_by`

// upsertMapFrame — one row per commune. On the first save `created_by` and `updated_by` are both the
// actor ($5); afterwards only `updated_by` moves.
const upsertMapFrame = `INSERT INTO map_frame
	(tenant_id, center_lat, center_lng, radius_km, created_by, updated_by)
	VALUES ($1, $2, $3, $4, $5, $5)
	ON CONFLICT (tenant_id) DO UPDATE SET
	center_lat = EXCLUDED.center_lat, center_lng = EXCLUDED.center_lng, radius_km = EXCLUDED.radius_km,
	updated_by = EXCLUDED.updated_by, updated_at = now()`

// ErrMapFrameNotFound — the commune has not set its frame (H3 "Chưa đặt": no map is drawn).
var ErrMapFrameNotFound = errors.New("map_frame: xã chưa đặt khung bản đồ")

// MapFrameStore reads and writes map_frame. Built from *store.DB and reaching the database only
// through Scoped / ScopedTx (rule 1, invariant 5).
type MapFrameStore struct {
	db *store.DB
}

func NewMapFrameStore(db *store.DB) *MapFrameStore {
	return &MapFrameStore{db: db}
}

// readOneMapFrame reads at most one row; found is false when there is none.
func readOneMapFrame(rows *sql.Rows) (f domain.MapFrame, found bool, err error) {
	defer rows.Close()
	if !rows.Next() {
		return domain.MapFrame{}, false, rows.Err()
	}
	if err := rows.Scan(&f.CenterLat, &f.CenterLng, &f.RadiusKm, &f.CreatedAt, &f.CreatedBy,
		&f.UpdatedAt, &f.UpdatedBy); err != nil {
		return domain.MapFrame{}, false, err
	}
	return f, true, rows.Err()
}

// Get reads the commune's frame, or ErrMapFrameNotFound.
func (s *MapFrameStore) Get(ctx context.Context) (domain.MapFrame, error) {
	// Scoped: Query adds `WHERE tenant_id = $1` from the context.
	rows, err := s.db.For(ctx).Query(ctx, mapFrameColumns, "map_frame", "")
	if err != nil {
		return domain.MapFrame{}, fmt.Errorf("map_frame: read: %w", err)
	}
	f, found, err := readOneMapFrame(rows)
	if err != nil {
		return domain.MapFrame{}, fmt.Errorf("map_frame: scan: %w", err)
	}
	if !found {
		return domain.MapFrame{}, ErrMapFrameNotFound
	}
	return f, nil
}

// LockCommuneFrame takes the commune's map-frame lock for the rest of the transaction, keyed
// `t:<tenant_id>:map-frame` (rule 1, invariant 7).
//
// WHY AN ADVISORY LOCK AND NOT ONLY FOR UPDATE: FOR UPDATE locks a row that EXISTS. On the commune's
// first save there is none, so two administrators saving at once both read "not found", both upsert,
// and the second entry has no `truoc` — the trail says the frame was set from nothing when it overwrote
// the first one's values. Taken BEFORE ForUpdate, this lock makes the second save wait for the first
// to commit, and then its read sees the row. Transaction-scoped: released at commit or rollback, never
// held across requests, never touching another commune. Same shape as LockCatalogueForSeed.
func (s *MapFrameStore) LockCommuneFrame(ctx context.Context, tx *store.ScopedTx) error {
	const stmt = `SELECT pg_advisory_xact_lock(hashtextextended('t:' || $1 || ':map-frame', 0))`
	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID())); err != nil {
		return fmt.Errorf("map_frame: khoá khung bản đồ của xã: %w", err)
	}
	return nil
}

// ForUpdate reads the row and locks it until the transaction ends; found is false when the commune
// has no row yet. The lock is what makes "nothing moved → no write, no entry" and the audit's
// `truoc` side true for two administrators saving at once — once the row exists; the first save needs
// LockCommuneFrame before it.
func (s *MapFrameStore) ForUpdate(ctx context.Context, tx *store.ScopedTx) (domain.MapFrame, bool, error) {
	// Scoped: tx.Query adds `WHERE tenant_id = $1`, bound from tx.TenantID().
	rows, err := tx.Query(ctx, mapFrameColumns, "map_frame", "FOR UPDATE")
	if err != nil {
		return domain.MapFrame{}, false, fmt.Errorf("map_frame: read for update: %w", err)
	}
	f, found, err := readOneMapFrame(rows)
	if err != nil {
		return domain.MapFrame{}, false, fmt.Errorf("map_frame: scan for update: %w", err)
	}
	return f, found, nil
}

// Upsert writes the three values and returns the row as stored (timestamps and signers included). by
// is the actor's BUSINESS CODE (rule 6, invariant 8).
func (s *MapFrameStore) Upsert(ctx context.Context, tx *store.ScopedTx, f domain.MapFrame, by string) (
	domain.MapFrame, error) {

	if by == "" {
		return domain.MapFrame{}, errors.New("map_frame: upsert without a business code")
	}
	if _, err := tx.Exec(ctx, upsertMapFrame, string(tx.TenantID()), f.CenterLat, f.CenterLng,
		f.RadiusKm, by); err != nil {
		return domain.MapFrame{}, fmt.Errorf("map_frame: upsert: %w", err)
	}
	// Read back on the same connection: the reply carries what the database holds, not what was sent.
	// Scoped: tx.Query adds `WHERE tenant_id = $1`, bound from tx.TenantID().
	rows, err := tx.Query(ctx, mapFrameColumns, "map_frame", "")
	if err != nil {
		return domain.MapFrame{}, fmt.Errorf("map_frame: read back: %w", err)
	}
	out, found, err := readOneMapFrame(rows)
	if err != nil {
		return domain.MapFrame{}, fmt.Errorf("map_frame: scan back: %w", err)
	}
	if !found {
		return domain.MapFrame{}, errors.New("map_frame: row missing right after its upsert")
	}
	return out, nil
}
