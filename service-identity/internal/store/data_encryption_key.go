package store

// identity's implementation of core/crypto.DEKStore — migrations/0021_mini_app_secret.sql,
// `data_encryption_key` (entity IdentityDataEncryptionKey). It seals each commune's own-Mini-App
// secret (ADR 0066 câu 6, ADR 0009).
//
// THE SAME SHAPE AS service-comms/internal/store/data_encryption_key.go, AND A SECOND COPY ON
// PURPOSE: each service owns its own DEK table (core/crypto DEKStore: "in the owning service's own
// database"), and importing another service's internal/ is rule 2 forbidden #1. A fix to the
// contract below goes into both.
//
// ONE ROW PER COMMUNE, the commune taken from the context and bound as $1 (rule 1, invariants 4 and
// 5). No delete here, and the trigger refuses one: a deleted data key is every sealed secret of that
// commune destroyed.
//
// EACH WRITE OPENS ITS OWN SHORT TRANSACTION, because crypto.DEKStore takes only a context. Harmless
// in the one direction it can go wrong: a business write that then fails leaves a DEK and no secret,
// and the next seal reuses it. A secret without its DEK cannot happen — Seal returns only after
// CreateDEK committed.
//
// BOTH WRITES ARE AUDITED, IN THEIR OWN TRANSACTION, UNDER THE SYSTEM PRINCIPAL (rule 6, invariant
// 6) — the choice comms made, for its reason: the interface carries no person, and the act that
// triggered the first seal is named by its own entry. The delta holds the KEK id only.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/store"
)

// Business verbs and subject written into the trail — values, Vietnamese (ADR 0011), the same
// strings comms writes for the same act.
const (
	ActionCreateDataKey = "tao_khoa_du_lieu_xa"
	ActionRewrapDataKey = "boc_lai_khoa_du_lieu_xa"
	DataKeySubject      = "khoa_du_lieu_xa"
)

const insertDEK = `INSERT INTO data_encryption_key (tenant_id, kek_id, wrapped) VALUES ($1, $2, $3) ` +
	`ON CONFLICT (tenant_id) DO NOTHING`

// The compare-and-swap: the row must still be exactly the one the caller read.
const replaceDEK = `UPDATE data_encryption_key SET kek_id = $4, wrapped = $5, updated_at = now() ` +
	`WHERE tenant_id = $1 AND kek_id = $2 AND wrapped = $3`

// DataEncryptionKeyStore reads and writes data_encryption_key.
type DataEncryptionKeyStore struct {
	db *store.DB
}

func NewDataEncryptionKeyStore(db *store.DB) *DataEncryptionKeyStore {
	return &DataEncryptionKeyStore{db: db}
}

var _ crypto.DEKStore = (*DataEncryptionKeyStore)(nil)

// GetDEK returns the commune's wrapped DEK, or crypto.ErrDEKNotFound.
func (s *DataEncryptionKeyStore) GetDEK(ctx context.Context) (crypto.WrappedDEK, error) {
	// Scoped: Query adds `WHERE tenant_id = $1` from the context.
	rows, err := s.db.For(ctx).Query(ctx, "kek_id, wrapped", "data_encryption_key", "")
	if err != nil {
		return crypto.WrappedDEK{}, fmt.Errorf("data_encryption_key: read: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return crypto.WrappedDEK{}, fmt.Errorf("data_encryption_key: read: %w", err)
		}
		return crypto.WrappedDEK{}, crypto.ErrDEKNotFound
	}
	var w crypto.WrappedDEK
	if err := rows.Scan(&w.KEKID, &w.Wrapped); err != nil {
		return crypto.WrappedDEK{}, fmt.Errorf("data_encryption_key: scan: %w", err)
	}
	return w, nil
}

// CreateDEK inserts the commune's first DEK, or reports crypto.ErrDEKExists — never overwrites.
func (s *DataEncryptionKeyStore) CreateDEK(ctx context.Context, dek crypto.WrappedDEK) error {
	var inserted int64
	err := s.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		res, err := tx.Exec(ctx, insertDEK, string(tx.TenantID()), dek.KEKID, dek.Wrapped)
		if err != nil {
			return fmt.Errorf("data_encryption_key: insert: %w", err)
		}
		if inserted, err = res.RowsAffected(); err != nil {
			return fmt.Errorf("data_encryption_key: insert: rows affected: %w", err)
		}
		if inserted == 0 {
			return nil
		}
		return recordDataKeyAudit(ctx, tx, ActionCreateDataKey, map[string]any{"kek_id": dek.KEKID})
	})
	if err != nil {
		return err
	}
	if inserted == 0 {
		return crypto.ErrDEKExists
	}
	return nil
}

// ReplaceDEK swaps oldDEK for newDEK only if the stored row still equals oldDEK, else
// crypto.ErrDEKChanged. Used by Envelope.RewrapDEK only.
func (s *DataEncryptionKeyStore) ReplaceDEK(ctx context.Context, oldDEK, newDEK crypto.WrappedDEK) error {
	if oldDEK.KEKID == newDEK.KEKID && bytes.Equal(oldDEK.Wrapped, newDEK.Wrapped) {
		return nil
	}
	return s.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		res, err := tx.Exec(ctx, replaceDEK, string(tx.TenantID()), oldDEK.KEKID, oldDEK.Wrapped,
			newDEK.KEKID, newDEK.Wrapped)
		if err != nil {
			return fmt.Errorf("data_encryption_key: re-wrap: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("data_encryption_key: re-wrap: rows affected: %w", err)
		}
		if n == 0 {
			// Rolls the transaction back, so no entry claims a re-wrap that did not happen.
			return crypto.ErrDEKChanged
		}
		return recordDataKeyAudit(ctx, tx, ActionRewrapDataKey, map[string]any{
			"truoc": map[string]string{"kek_id": oldDEK.KEKID},
			"sau":   map[string]string{"kek_id": newDEK.KEKID},
		})
	})
}

// recordDataKeyAudit — SAME TRANSACTION as the write (rule 6, invariant 3). Never the wrapped bytes.
func recordDataKeyAudit(ctx context.Context, tx *store.ScopedTx, action string, delta map[string]any) error {
	b, err := json.Marshal(delta)
	if err != nil {
		return fmt.Errorf("data_encryption_key: encode delta: %w", err)
	}
	return audit.Write(ctx, tx, audit.Entry{
		Actor:   audit.Actor{ID: audit.SystemActor, Kind: "system"},
		Action:  action,
		Subject: DataKeySubject,
		Delta:   b,
	})
}
