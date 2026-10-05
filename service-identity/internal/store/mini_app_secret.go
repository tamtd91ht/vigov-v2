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
//
// demo_identity_enabled IS WRITTEN, NEVER READ (owner decision 05/10/2026, ADR 0066 §Sửa đổi): the
// column stays because dropping a populated column is rule 7 stop condition #2, and the history rows
// that carry `true` are the answer to "which App ID opened sessions without a verified phone, and
// since when". Every new version writes false; nothing in Go reads it back, so no code path can
// re-enable the identity from a stale row.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/store"
)

// MiniAppSecret is one version row. Sealed is nil only on a row written while the demo identity
// existed (CHECK mini_app_secret_sealed_or_demo); Insert refuses a nil one, and sign-in answers "not
// ready" for a live row that has none.
type MiniAppSecret struct {
	ID     string
	AppID  string
	Sealed []byte
	SetAt  time.Time
	SetBy  string
}

// MiniAppSecretStatus is one live version WITHOUT its sealed bytes: the status list computes
// `app_secret_sealed IS NOT NULL` in SQL, so the ciphertext never even leaves the database on that
// path (operator.proto: nothing derived from the secret — no length, no hash).
type MiniAppSecretStatus struct {
	ID        string
	AppID     string
	SecretSet bool
	SetAt     time.Time
	SetBy     string
}

const miniAppSecretColumns = "id, app_id, app_secret_sealed, set_at, set_by"

const miniAppSecretStatusColumns = "id, app_id, app_secret_sealed IS NOT NULL, set_at, set_by"

const retireMiniAppSecret = `UPDATE mini_app_secret
	SET deleted_at = now(), deleted_by = $3, delete_reason = $4, updated_at = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

// demo_identity_enabled is the literal false: see the file comment.
const insertMiniAppSecret = `INSERT INTO mini_app_secret
	(tenant_id, id, app_id, app_secret_sealed, demo_identity_enabled, set_at, set_by)
	VALUES ($1, $2, $3, $4, false, $5, $6)`

// ErrMiniAppSecretRetired is a retire that matched no live row — another run retired it first.
var ErrMiniAppSecretRetired = errors.New("mini_app_secret: the version is no longer live")

// ErrMiniAppSecretNoSecret is an Insert with no sealed secret. Refused here, before the database:
// the schema would admit it only with demo on, and no version is written with demo on any more.
var ErrMiniAppSecretNoSecret = errors.New("mini_app_secret: a version must carry a sealed secret")

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

// LiveStatuses lists every live version of the commune in ctx, ordered by app_id — the operator
// screen's "đặt lúc … bởi …" read. Live = deleted_at IS NULL, i.e. live_app_id IS NOT NULL (rule 7,
// invariant 2).
func (s *MiniAppSecretStore) LiveStatuses(ctx context.Context) ([]MiniAppSecretStatus, error) {
	// Scoped: Query adds `WHERE tenant_id = $1`.
	rows, err := s.db.For(ctx).Query(ctx, miniAppSecretStatusColumns, "mini_app_secret",
		"AND deleted_at IS NULL ORDER BY app_id")
	if err != nil {
		return nil, fmt.Errorf("mini_app_secret: list live: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []MiniAppSecretStatus
	for rows.Next() {
		var v MiniAppSecretStatus
		if err := rows.Scan(&v.ID, &v.AppID, &v.SecretSet, &v.SetAt, &v.SetBy); err != nil {
			return nil, fmt.Errorf("mini_app_secret: list live: scan: %w", err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mini_app_secret: list live: %w", err)
	}
	return out, nil
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
	if len(v.Sealed) == 0 {
		return ErrMiniAppSecretNoSecret
	}
	if _, err := tx.Exec(ctx, insertMiniAppSecret, string(tx.TenantID()), v.ID, v.AppID, v.Sealed,
		v.SetAt, v.SetBy); err != nil {
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
	if err := rows.Scan(&v.ID, &v.AppID, &v.Sealed, &v.SetAt, &v.SetBy); err != nil {
		return MiniAppSecret{}, false, fmt.Errorf("mini_app_secret: scan: %w", err)
	}
	if len(v.Sealed) == 0 {
		v.Sealed = nil
	}
	return v, true, nil
}
