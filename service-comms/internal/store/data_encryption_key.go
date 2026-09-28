package store

// comms' implementation of core/crypto.DEKStore — migrations/0008_mail_settings.sql,
// `data_encryption_key`.
//
// ONE ROW PER COMMUNE, the commune taken from the context through Scoped / ScopedTx and bound as $1
// of every statement (rule 1, invariants 4 and 5). There is no delete here and the trigger refuses
// one: a deleted data key is every sealed secret of that commune destroyed (ADR 0009).
//
// EACH WRITE OPENS ITS OWN SHORT TRANSACTION. crypto.DEKStore takes only a context, so a DEK is
// created outside the transaction of the business write that first needed it. That is harmless in
// the one direction it can go wrong: a business write that then fails leaves a DEK and no secret,
// and the next seal simply reuses it. The reverse — a secret without its DEK — cannot happen, because
// Seal returns only after CreateDEK has committed.
//
// BOTH WRITES ARE AUDITED, IN THEIR OWN TRANSACTION, UNDER THE SYSTEM PRINCIPAL. ADR 0009's
// amendment left this open ("Còn nợ" #4) for the owning service to decide; the choice made here is
// the conservative one of rule 6, invariant 6 — a system action is audited too. The actor is
// audit.SystemActor because the interface carries no person: the DEK is created by the platform on
// the first seal, and the person whose save triggered it is named by THAT save's own entry. The
// delta holds the KEK id only — 8 hex characters derived by HMAC, which say nothing about any key.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/store"
)

// The business verbs and subject written into the trail — VALUES, Vietnamese like every other
// action of this service (ADR 0011).
const (
	ActionCreateDataKey = "tao_khoa_du_lieu_xa"
	ActionRewrapDataKey = "boc_lai_khoa_du_lieu_xa"
	DataKeySubject      = "khoa_du_lieu_xa"
)

// The two statements, kept above every function.
const insertDEK = `INSERT INTO data_encryption_key (tenant_id, kek_id, wrapped) VALUES ($1, $2, $3) ` +
	`ON CONFLICT (tenant_id) DO NOTHING`

// The compare-and-swap: the row must still be exactly the one the caller read.
const replaceDEK = `UPDATE data_encryption_key SET kek_id = $4, wrapped = $5, updated_at = now() ` +
	`WHERE tenant_id = $1 AND kek_id = $2 AND wrapped = $3`

// systemActor is the principal of a write no person made directly (rule 6, invariant 6).
var systemActor = audit.Actor{ID: audit.SystemActor, Kind: "system"}

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
	defer rows.Close()
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

// CreateDEK inserts the commune's first DEK, or reports crypto.ErrDEKExists — NEVER overwrites.
// The losing side of a race writes no audit entry: nothing was created.
func (s *DataEncryptionKeyStore) CreateDEK(ctx context.Context, dek crypto.WrappedDEK) error {
	var inserted int64
	err := s.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// Scoped: tenant_id is $1 from tx.TenantID().
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

// errDEKUnchanged rolls the transaction back when the compare-and-swap matched nothing, so no
// entry is left claiming a re-wrap that did not happen.
var errDEKUnchanged = crypto.ErrDEKChanged

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
			return errDEKUnchanged
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
		Actor:   systemActor,
		Action:  action,
		Subject: DataKeySubject,
		Delta:   b,
	})
}
