package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The automatic sign-in lockout's two columns (migration 0020, open question #39).
//
// A READ-LOCK-THEN-WRITE PAIR, and the pair is only safe because of the FOR UPDATE: two parallel
// wrong passwords for one account queue on the row, so the second reads the count the first
// committed. Without it both read 4, both write 5, and the lock is decided twice. The state machine
// itself lives in domain.SignInLock, where a test can walk it without a database.
//
// Neither method touches `cap_nhat_luc`: that column means "a person last edited this record" (the
// reading of migrations 0003 and 0019), and a failed sign-in is not an edit.

// SignInLockForUpdate reads one account's lock state and holds its row until the transaction ends.
// ErrCanBoKhongTonTai when the row is gone.
func (s *CanBoStore) SignInLockForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (domain.SignInLock, error) {
	const stmt = `SELECT failed_sign_in_count, sign_in_locked_until FROM nguoi_dung ` +
		`WHERE tenant_id = $1 AND id = $2 FOR UPDATE`
	var l domain.SignInLock
	var until *time.Time
	err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan(&l.FailedCount, &until)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.SignInLock{}, ErrCanBoKhongTonTai
	}
	if err != nil {
		return domain.SignInLock{}, fmt.Errorf("can_bo: read sign-in lock: %w", err)
	}
	if until != nil {
		u := until.UTC()
		l.LockedUntil = &u
	}
	return l, nil
}

// SetSignInLock writes one account's lock state. The caller holds the row (SignInLockForUpdate) and
// writes the audit entry for a lock or an unlock in the same transaction (rule 6, invariant 3).
func (s *CanBoStore) SetSignInLock(ctx context.Context, tx *store.ScopedTx, id string, l domain.SignInLock) error {
	const stmt = `UPDATE nguoi_dung SET failed_sign_in_count = $3, sign_in_locked_until = $4
	              WHERE tenant_id = $1 AND id = $2`
	var until any
	if l.LockedUntil != nil {
		until = l.LockedUntil.UTC()
	}
	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, l.FailedCount, until)
	if err != nil {
		return fmt.Errorf("can_bo: write sign-in lock: %w", err)
	}
	return doiMotDong(kq, "write sign-in lock")
}
