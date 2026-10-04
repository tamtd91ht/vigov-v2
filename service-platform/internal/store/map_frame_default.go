package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	corestore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-platform/internal/domain"
)

// MapFrameDefaultStore is the only path to `commune_map_frame_default` (migration 0020, ADR 0072
// amendment 2 K1).
//
// BUILT FROM *corestore.DB AND READ ONLY THROUGH Scoped, like HoSoHienThiStore: the default belongs to
// ONE commune, so the commune comes from the context and is $1 of every statement (rule 1, invariants 4
// and 5). No method takes a commune as an argument — the gRPC read gets it from "x-tenant-id", the
// operator routes from the path (targetCommune), both through tenant.Into.
//
// THE WRITE IS ONE TRANSACTION WITH ITS AUDIT ENTRY (rule 6 invariant 3), filed in this service's
// audit_log under the TARGET commune with actor_kind 'operator' — the convention of RegistryWriter, so
// the operator log screen (OperatorLog) lists it with its before/after and reason.
type MapFrameDefaultStore struct {
	db *corestore.DB
}

// NewMapFrameDefaultStore builds the store over the scoped handle.
func NewMapFrameDefaultStore(db *corestore.DB) *MapFrameDefaultStore {
	return &MapFrameDefaultStore{db: db}
}

// ActionSetMapFrameDefault is the audit_log verb of a change. A Vietnamese snake_case VALUE, as every
// operator act filed in a commune's audit_log (operator_writes.go; ADR 0011 for values).
const ActionSetMapFrameDefault = "dat_khung_ban_do_mac_dinh"

const mapFrameDefaultColumns = `center_lat, center_lng, radius_km, created_at, created_by, updated_at, updated_by`

func scanMapFrameDefault(rows *sql.Rows) (domain.MapFrameDefault, error) {
	var f domain.MapFrameDefault
	err := rows.Scan(&f.CenterLat, &f.CenterLng, &f.RadiusKm, &f.CreatedAt, &f.CreatedBy, &f.UpdatedAt, &f.UpdatedBy)
	return f, err
}

// MapFrameDefault reads the default frame of the commune in ctx. ok=false when none is set — an
// ordinary state, never an error. It does NOT check that the commune exists or is active: the gRPC
// caller has already resolved its commune, and "no row" is the contract's "not configured" either way.
// Panics when ctx carries no commune — tenant.MustFrom, deliberately.
func (s *MapFrameDefaultStore) MapFrameDefault(ctx context.Context) (domain.MapFrameDefault, bool, error) {
	rows, err := s.db.For(ctx).Query(ctx, mapFrameDefaultColumns, "commune_map_frame_default", "")
	if err != nil {
		return domain.MapFrameDefault{}, false, fmt.Errorf("commune_map_frame_default: read: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.MapFrameDefault{}, false, fmt.Errorf("commune_map_frame_default: read: %w", err)
		}
		return domain.MapFrameDefault{}, false, nil
	}
	f, err := scanMapFrameDefault(rows)
	if err != nil {
		return domain.MapFrameDefault{}, false, fmt.Errorf("commune_map_frame_default: scan: %w", err)
	}
	// tenant_id is the primary key: a second row is impossible.
	return f, true, nil
}

// operatorMapFrameDefaultStmt reads the commune's registry row with its default, if any. The registry
// row is what turns an unknown commune into ErrCommuneNotFound (404) rather than "not configured".
// `tenant` has no tenant_id column — its id IS the commune — so `t.id = $1` is the scope, and the joined
// table is constrained to $1 in its ON clause (corestore.QueryJoin's contract).
const operatorMapFrameDefaultStmt = `SELECT f.center_lat, f.center_lng, f.radius_km,
	f.created_at, f.created_by, f.updated_at, f.updated_by
	FROM tenant t
	LEFT JOIN commune_map_frame_default f ON f.tenant_id = $1
	WHERE t.id = $1`

// OperatorMapFrameDefault is the operator console's read: the commune in ctx must exist
// (ErrCommuneNotFound otherwise); ok=false when it has no default. An INACTIVE commune is read like any
// other — rule 7 keeps its configuration, and the console shows what it was.
func (s *MapFrameDefaultStore) OperatorMapFrameDefault(ctx context.Context) (domain.MapFrameDefault, bool, error) {
	rows, err := s.db.For(ctx).QueryJoin(ctx, operatorMapFrameDefaultStmt)
	if err != nil {
		return domain.MapFrameDefault{}, false, fmt.Errorf("commune_map_frame_default: operator read: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.MapFrameDefault{}, false, fmt.Errorf("commune_map_frame_default: operator read: %w", err)
		}
		return domain.MapFrameDefault{}, false, ErrCommuneNotFound
	}
	var (
		lat, lng, r      sql.NullFloat64
		createdAt, updAt sql.NullTime
		createdBy, updBy sql.NullString
	)
	if err := rows.Scan(&lat, &lng, &r, &createdAt, &createdBy, &updAt, &updBy); err != nil {
		return domain.MapFrameDefault{}, false, fmt.Errorf("commune_map_frame_default: operator scan: %w", err)
	}
	if !r.Valid {
		// The LEFT JOIN found the commune and no default. Every column is NOT NULL in the table, so a
		// NULL radius can only mean "no row".
		return domain.MapFrameDefault{}, false, nil
	}
	return domain.MapFrameDefault{CenterLat: lat.Float64, CenterLng: lng.Float64, RadiusKm: r.Float64,
		CreatedAt: createdAt.Time, CreatedBy: createdBy.String, UpdatedAt: updAt.Time, UpdatedBy: updBy.String}, true, nil
}

// SetMapFrameDefault creates or replaces the default frame of the commune in ctx, with its audit entry,
// in one transaction. next must come from domain.NormalizeMapFrameDefault and have passed
// domain.CheckUnusualRadius (the handler does both); acknowledgedUnusual is recorded in the entry so a
// later reader sees the operator confirmed an unusual radius. changed=false when the values are
// identical — no write and no entry, because nothing happened (RegistryWriter's convention).
//
// Refused, writing nothing: no actor code (domain.ErrNoActor); commune unknown (ErrCommuneNotFound);
// commune inactive (ErrCommuneInactive — rule 7 invariant 6 keeps a merged commune's registry
// unchanged, as every other registry write here does).
//
// The tenant row is locked first (lockCommune), so a concurrent deactivation and this write serialise.
func (s *MapFrameDefaultStore) SetMapFrameDefault(ctx context.Context, next domain.MapFrameDefault,
	acknowledgedUnusual bool, reason string, by domain.OperatorActor) (out domain.MapFrameDefault, changed bool, err error) {
	if err := by.Validate(); err != nil {
		return domain.MapFrameDefault{}, false, err
	}
	act := audit.Actor{ID: by.Code, Kind: domain.AuditKindOperator, IP: by.IP}
	err = s.db.For(ctx).Tx(ctx, func(tx *corestore.ScopedTx) error {
		c, err := lockCommune(ctx, tx)
		if err != nil {
			return err
		}
		if !c.active {
			return ErrCommuneInactive
		}
		rows, err := tx.Query(ctx, mapFrameDefaultColumns, "commune_map_frame_default", "FOR UPDATE")
		if err != nil {
			return fmt.Errorf("commune_map_frame_default: lock: %w", err)
		}
		var (
			before domain.MapFrameDefault
			had    bool
		)
		if rows.Next() {
			if before, err = scanMapFrameDefault(rows); err != nil {
				rows.Close()
				return fmt.Errorf("commune_map_frame_default: lock: %w", err)
			}
			had = true
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf("commune_map_frame_default: lock: %w", err)
		}
		if err := rows.Close(); err != nil {
			return fmt.Errorf("commune_map_frame_default: lock: %w", err)
		}
		if had && domain.SameMapFrameDefault(before, next) {
			out = before
			return nil
		}

		id := tx.TenantID().String()
		var row *sql.Row
		if had {
			row = tx.Underlying().QueryRowContext(ctx,
				`UPDATE commune_map_frame_default
				    SET center_lat = $2, center_lng = $3, radius_km = $4, updated_at = now(), updated_by = $5
				  WHERE tenant_id = $1
				 RETURNING `+mapFrameDefaultColumns,
				id, next.CenterLat, next.CenterLng, next.RadiusKm, by.Code)
		} else {
			row = tx.Underlying().QueryRowContext(ctx,
				`INSERT INTO commune_map_frame_default
				    (tenant_id, center_lat, center_lng, radius_km, created_by, updated_by)
				 VALUES ($1, $2, $3, $4, $5, $5)
				 RETURNING `+mapFrameDefaultColumns,
				id, next.CenterLat, next.CenterLng, next.RadiusKm, by.Code)
		}
		if err := row.Scan(&out.CenterLat, &out.CenterLng, &out.RadiusKm, &out.CreatedAt, &out.CreatedBy,
			&out.UpdatedAt, &out.UpdatedBy); err != nil {
			return fmt.Errorf("commune_map_frame_default: write: %w", err)
		}
		changed = true

		// truoc / sau / ly_do: the keys OperatorLog reads for before, after and reason. No personal data
		// (a commune's centre is a public place — migration 0020).
		var prev any
		if had {
			prev = frameDelta(before)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: act, Action: ActionSetMapFrameDefault, Subject: domain.MapFrameDefaultSubject,
			Delta: delta(map[string]any{
				"truoc":                prev,
				"sau":                  frameDelta(out),
				"ly_do":                reason,
				"xa":                   c.name,
				"acknowledged_unusual": acknowledgedUnusual,
			}),
		})
	})
	if err != nil {
		return domain.MapFrameDefault{}, false, err
	}
	return out, changed, nil
}

func frameDelta(f domain.MapFrameDefault) map[string]any {
	return map[string]any{"center_lat": f.CenterLat, "center_lng": f.CenterLng, "radius_km": f.RadiusKm}
}
