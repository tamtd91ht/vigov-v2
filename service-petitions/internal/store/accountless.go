package store

// The statements of the ACCOUNTLESS petition path — ADR 0083, TEMPORARY (remove with its routes, ADR
// 0083 §Gỡ bỏ). SQL only: the predicate's meaning is domain.PhieuPhanAnh.Accountless, the number is
// domain.AccountlessDailyCeiling, and the decision to refuse is the use case's.

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// accountlessWhere is domain.PhieuPhanAnh.Accountless in SQL, with the channel bound as $2 from
// domain.AccountlessChannel. THE SAME TEXT IN BOTH STATEMENTS BELOW, so the ceiling counts exactly the
// petitions the lookup can read. Owner columns are NULL — never ” (migration 0032), so IS NULL is exact.
const accountlessWhere = `kenh_tiep_nhan = $2 AND cong_dan_id IS NULL AND zalo_account_id IS NULL`

// LockAccountlessIntake serialises the accountless intakes of ONE commune until the caller's transaction
// ends. The ceiling is a read-decide-write; without the lock two sends at the 199th would both pass.
// Per commune, so it never blocks a session intake or another commune.
func (s *PhieuPhanAnhStore) LockAccountlessIntake(ctx context.Context, tx *store.ScopedTx) error {
	const stmt = `SELECT pg_advisory_xact_lock(hashtextextended('phieu_phan_anh:accountless:' || $1, 0))`
	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID())); err != nil {
		return fmt.Errorf("phieu_phan_anh: khoá lượt gửi không tài khoản của xã: %w", err)
	}
	return nil
}

// CountAccountlessPetitionsSince counts THIS commune's accountless petitions registered at or after
// `since`, inside the caller's transaction (after LockAccountlessIntake).
//
// SOFT-DELETED ROWS ARE COUNTED, ON PURPOSE (ADR 0083 row 12, as ADR 0080): an abuse counter, not a
// business read — deleting spam must not refill the allowance. CountZaloAccountPetitionsSince says the
// rest of why rule 7 invariant 2 does not govern it.
func (s *PhieuPhanAnhStore) CountAccountlessPetitionsSince(ctx context.Context, tx *store.ScopedTx,
	since time.Time) (int, error) {

	const stmt = `SELECT count(*) FROM phieu_phan_anh
		WHERE tenant_id = $1 AND ` + accountlessWhere + ` AND vao_so_luc >= $3`
	var n int
	if err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()),
		string(domain.AccountlessChannel), since).Scan(&n); err != nil {
		return 0, fmt.Errorf("phieu_phan_anh: đếm phiếu không tài khoản trong ngày: %w", err)
	}
	return n, nil
}

// AccountlessByCode reads ONE live ACCOUNTLESS petition of THIS commune by its lookup code — the public
// lookup of ADR 0083 row 4.
//
// RESTRICTED TO ACCOUNTLESS ROWS, and that restriction is the isolation: a citizen's or a Zalo account's
// petition holding the same code is ErrPhieuKhongTonTai — the same answer as no such code, another
// commune's code, or a soft-deleted petition (rule 4, forbidden #2). Only the code opens it; that is ADR
// 0083 cost #4, and the handler returns only status, deadlines, result and reason in compensation.
//
// TWO WALLS: the WHERE clause, and domain.Accountless on the scanned row — so the day the SQL and the
// domain definition drift, the read fails closed instead of serving an owned petition.
func (s *PhieuPhanAnhStore) AccountlessByCode(ctx context.Context, ma string) (domain.PhieuPhanAnh, error) {
	if strings.TrimSpace(ma) == "" {
		return domain.PhieuPhanAnh{}, ErrPhieuKhongTonTai
	}
	rows, err := s.db.For(ctx).Query(ctx, cotPhieu, "phieu_phan_anh",
		`AND `+accountlessWhere+` AND ma_tra_cuu = $3 AND deleted_at IS NULL`,
		string(domain.AccountlessChannel), ma)
	if err != nil {
		// The code is NOT in the message: it is the one string that opens the petition (rule 3).
		return domain.PhieuPhanAnh{}, fmt.Errorf("phieu_phan_anh: đọc phiếu không tài khoản: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.PhieuPhanAnh{}, fmt.Errorf("phieu_phan_anh: đọc phiếu không tài khoản: %w", err)
		}
		return domain.PhieuPhanAnh{}, ErrPhieuKhongTonTai
	}
	p, err := quetPhieu(rows)
	if err != nil {
		return domain.PhieuPhanAnh{}, err
	}
	if err := rows.Err(); err != nil {
		return domain.PhieuPhanAnh{}, fmt.Errorf("phieu_phan_anh: duyệt kết quả: %w", err)
	}
	if !p.Accountless() {
		return domain.PhieuPhanAnh{}, ErrPhieuKhongTonTai
	}
	return p, nil
}
