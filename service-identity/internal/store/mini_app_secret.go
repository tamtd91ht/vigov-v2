package store

// mini_app_secret — the sign-in settings of a commune's OWN Mini App, one version per row
// (migration 0021, ADR 0066).
//
// EVERY CHANGE IS A NEW ROW: the live row is retired (soft-delete trio) and the next version
// inserted, in the CALLER's transaction, which also carries the audit entry (rule 6, invariant 3).
// The table's guard trigger refuses any other UPDATE and every DELETE.
//
// Every statement binds the commune as $1 from the scoped handle (rule 1, invariants 4 and 5), and
// every read of the current settings filters on live_app_id — NULL on retired rows — so a
// superseded version is never served (rule 7, invariant 2).
//
// THE SEALED BYTES ARE OPAQUE HERE. This file never opens them; app/mini_app_secret.go owns the
// envelope and its AAD.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/store"
)

// MiniAppSecret is one version row. Sealed is nil when no secret is set (allowed only with
// DemoIdentity — CHECK mini_app_secret_sealed_or_demo).
type MiniAppSecret struct {
	ID           string
	AppID        string
	Sealed       []byte
	DemoIdentity bool
	SetAt        time.Time
	SetBy        string
}

const miniAppSecretColumns = "id, app_id, app_secret_sealed, demo_identity_enabled, set_at, set_by"

const retireMiniAppSecret = `UPDATE mini_app_secret
	SET deleted_at = now(), deleted_by = $3, delete_reason = $4, updated_at = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

const insertMiniAppSecret = `INSERT INTO mini_app_secret
	(tenant_id, id, app_id, app_secret_sealed, demo_identity_enabled, set_at, set_by)
	VALUES ($1, $2, $3, $4, $5, $6, $7)`

// ErrMiniAppSecretRetired is a retire that matched no live row — another run retired it first.
var ErrMiniAppSecretRetired = errors.New("mini_app_secret: the version is no longer live")

// MiniAppSecretStore reads and writes mini_app_secret.
type MiniAppSecretStore struct {
	db *store.DB
}

func NewMiniAppSecretStore(db *store.DB) *MiniAppSecretStore { return &MiniAppSecretStore{db: db} }

// Live returns the live version for appID in the commune of ctx, outside any transaction — the
// sign-in read.
func (s *MiniAppSecretStore) Live(ctx context.Context, appID string) (MiniAppSecret, bool, error) {
	// Scoped: Query adds `WHERE tenant_id = $1`.
	rows, err := s.db.For(ctx).Query(ctx, miniAppSecretColumns, "mini_app_secret", "AND live_app_id = $2", appID)
	if err != nil {
		return MiniAppSecret{}, false, fmt.Errorf("mini_app_secret: read: %w", err)
	}
	return scanOneMiniAppSecret(rows)
}

// LiveForUpdate is Live inside the caller's transaction, with the row locked until it ends, so two
// concurrent changes of one App ID serialise instead of both retiring the same version.
func (s *MiniAppSecretStore) LiveForUpdate(ctx context.Context, tx *store.ScopedTx, appID string) (MiniAppSecret, bool, error) {
	// Scoped: tx.Query adds `WHERE tenant_id = $1`.
	rows, err := tx.Query(ctx, miniAppSecretColumns, "mini_app_secret", "AND live_app_id = $2 FOR UPDATE", appID)
	if err != nil {
		return MiniAppSecret{}, false, fmt.Errorf("mini_app_secret: read for update: %w", err)
	}
	return scanOneMiniAppSecret(rows)
}

// Retire soft-deletes the live version id. by is the business code of who retired it, reason the
// operator's ticket (CHECK mini_app_secret_soft_delete_complete wants both non-blank).
func (s *MiniAppSecretStore) Retire(ctx context.Context, tx *store.ScopedTx, id, by, reason string) error {
	res, err := tx.Exec(ctx, retireMiniAppSecret, string(tx.TenantID()), id, by, reason)
	if err != nil {
		return fmt.Errorf("mini_app_secret: retire: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mini_app_secret: retire: rows affected: %w", err)
	}
	if n != 1 {
		return ErrMiniAppSecretRetired
	}
	return nil
}

// Insert writes a new version. The live-row key (tenant_id, live_app_id) refuses a second live row
// for one App ID, so Retire the current one first in the same transaction.
func (s *MiniAppSecretStore) Insert(ctx context.Context, tx *store.ScopedTx, v MiniAppSecret) error {
	// A nil slice must reach the driver as SQL NULL, not as an empty BYTEA: the CHECK on the
	// envelope length would otherwise refuse a demo-only row.
	var sealed any
	if v.Sealed != nil {
		sealed = v.Sealed
	}
	if _, err := tx.Exec(ctx, insertMiniAppSecret, string(tx.TenantID()), v.ID, v.AppID, sealed,
		v.DemoIdentity, v.SetAt, v.SetBy); err != nil {
		return fmt.Errorf("mini_app_secret: insert: %w", err)
	}
	return nil
}

func scanOneMiniAppSecret(rows *sql.Rows) (MiniAppSecret, bool, error) {
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return MiniAppSecret{}, false, fmt.Errorf("mini_app_secret: read: %w", err)
		}
		return MiniAppSecret{}, false, nil
	}
	var v MiniAppSecret
	if err := rows.Scan(&v.ID, &v.AppID, &v.Sealed, &v.DemoIdentity, &v.SetAt, &v.SetBy); err != nil {
		return MiniAppSecret{}, false, fmt.Errorf("mini_app_secret: scan: %w", err)
	}
	if len(v.Sealed) == 0 {
		v.Sealed = nil
	}
	return v, true, nil
}
