package store

// The two statements the UNVERIFIED-petition ceiling needs (ADR 0080 decision 7). SQL only: the
// number and the day boundary are domain.UnverifiedDailyCeiling / UnverifiedCountingDayStart, and the
// decision to refuse is the use case's (app/gui_phan_anh.go).

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vihat/vigov/core/store"
)

// ErrNoZaloAccount — a ceiling statement was asked for with no account id. A wiring fault in the
// caller, never "zero petitions": an empty account counted as one shared bucket would let every
// unverified sender of the commune exhaust one another's allowance, or none.
var ErrNoZaloAccount = errors.New("phieu_phan_anh: thiếu tài khoản Zalo cho trần phiếu chưa xác thực")

// LockZaloAccountIntake serialises the intakes of ONE Zalo account in ONE commune until the caller's
// transaction ends (COMMIT or ROLLBACK — no path forgets it).
//
// WHY A LOCK AND NOT ONLY THE COUNT: the ceiling is a read-decide-write. Two sends of the same account
// at the same instant would both count 9 and both insert, so the 10 would be a soft target exactly when
// it is being hammered. Per (commune, account), so it never blocks another sender. The key is a hash of
// the commune and the opaque account id — no personal data (rule 3, forbidden #4).
func (s *PhieuPhanAnhStore) LockZaloAccountIntake(ctx context.Context, tx *store.ScopedTx, zaloAccountID string) error {
	if strings.TrimSpace(zaloAccountID) == "" {
		return ErrNoZaloAccount
	}
	const stmt = `SELECT pg_advisory_xact_lock(hashtextextended('phieu_phan_anh:zalo-account:' || $1 || ':' || $2, 0))`
	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID()), zaloAccountID); err != nil {
		// The account id is NOT in the message (rule 3): it links to a person's Zalo account.
		return fmt.Errorf("phieu_phan_anh: khoá lượt gửi của tài khoản Zalo: %w", err)
	}
	return nil
}

// CountZaloAccountPetitionsSince counts the petitions ONE Zalo account sent in THIS commune at or after
// `since`, inside the caller's transaction (after LockZaloAccountIntake).
//
// SOFT-DELETED ROWS ARE COUNTED, ON PURPOSE. This is an ABUSE COUNTER, not a business read: a spam
// petition that staff soft-delete must NOT hand its sender the allowance back, or deleting spam would
// invite more of it. Rule 7, invariant 2 ("every read path excludes deleted rows") governs what the
// commune's register, lists, statistics and reports SHOW — none of which this is; nothing here is
// shown to anybody, only compared with domain.UnverifiedDailyCeiling.
//
// `vao_so_luc` AND NOT `goc_dem_han`: the row's register instant, which on the Mini App channel is the
// instant of the send (gui_phan_anh.go writes both from one clock read). Served by migration 0032's
// partial index `phieu_phan_anh_zalo_account` (tenant_id, zalo_account_id, vao_so_luc DESC).
func (s *PhieuPhanAnhStore) CountZaloAccountPetitionsSince(ctx context.Context, tx *store.ScopedTx,
	zaloAccountID string, since time.Time) (int, error) {

	if strings.TrimSpace(zaloAccountID) == "" {
		return 0, ErrNoZaloAccount
	}
	const stmt = `SELECT count(*) FROM phieu_phan_anh
		WHERE tenant_id = $1 AND zalo_account_id = $2 AND vao_so_luc >= $3`
	var n int
	if err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), zaloAccountID, since).Scan(&n); err != nil {
		return 0, fmt.Errorf("phieu_phan_anh: đếm phiếu chưa xác thực trong ngày: %w", err)
	}
	return n, nil
}
