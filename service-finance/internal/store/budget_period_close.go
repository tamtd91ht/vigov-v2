package store

// Budget period closes (migration 0012) — reads, the one insert, the one reopen fill, and the
// (tenant, year) advisory lock every guarded write takes. The five properties at the top of
// thu_chi_ngan_sach.go hold here unchanged; two more are specific to this table:
//
//  1. HISTORY IS NEVER EDITED. The only UPDATE is the one-time reopen fill, carrying
//     `AND reopened_at IS NULL`; 0012's trigger refuses anything else. No DELETE exists.
//  2. THE ADVISORY LOCK KEY CARRIES THE COMMUNE FROM THE TRANSACTION, never from a caller: a key
//     without it would serialise every commune's budget writes on one year, and a key taken from a
//     request parameter would let one commune hold another's lock.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// ErrTooManyPeriodCloses — one year holds more closes than MaxPeriodClosesPerYear. REFUSED, NEVER
// TRUNCATED: the list is the history of who declared which figures final, and a short list hides one.
var ErrTooManyPeriodCloses = errors.New("ngan_sach: năm vượt trần số lần chốt kỳ")

// MaxPeriodClosesPerYear — 12 month closes + 1 year close per year is the ordinary ceiling (0012,
// question 1); every re-close adds one. 1000 is far past any real history of one year.
const MaxPeriodClosesPerYear = 1000

// lockBudgetYear — pg_advisory_xact_lock, TRANSACTION-scoped: released by COMMIT or ROLLBACK, so no
// path can leak it. hashtextextended(…, 0) turns the text key into the bigint the lock takes. Every
// operand is cast: `unknown || unknown` has no unique operator in PostgreSQL. $1 is the commune,
// bound from the transaction.
const lockBudgetYear = `SELECT pg_advisory_xact_lock(hashtextextended(
	'budget-period:'::text || $1::text || ':'::text || ($2::int)::text, 0))`

// LockBudgetYears takes the (tenant, year) advisory lock for each distinct year, in ASCENDING order
// (domain.LockYears) — two writers needing the same two years then queue instead of deadlocking.
//
// WHY IT EXISTS: a guarded write checks an ABSENCE ("no active close"), which no row lock can hold.
// Close, reopen and every guarded write take this lock BEFORE they check, so a write and a close of
// the same (commune, year) are serialised and the write cannot land in a period that reads as closed.
//
// LOCK ORDER, stated once: writers on a sheet take the sheet's FOR UPDATE row lock first and this
// lock second; close and reopen take ONLY this lock. Nothing holds this lock while waiting for a
// sheet row, so the two kinds cannot form a cycle.
func (s *NganSachStore) LockBudgetYears(ctx context.Context, tx *store.ScopedTx, years ...int) error {
	for _, y := range domain.LockYears(years...) {
		if _, err := tx.Exec(ctx, lockBudgetYear, string(tx.TenantID()), y); err != nil {
			return fmt.Errorf("ngan_sach: khoá kỳ ngân sách năm %d: %w", y, err)
		}
	}
	return nil
}

const colsPeriodClose = `id, code, year, COALESCE(month, 0), revision, closed_at, closed_by, ` +
	`reopened_at, COALESCE(reopened_by, ''), COALESCE(reopen_reason, '')`

func scanPeriodCloses(rows *sql.Rows) ([]domain.BudgetPeriodClose, error) {
	defer rows.Close()
	ra := make([]domain.BudgetPeriodClose, 0, 4)
	for rows.Next() {
		var (
			c        domain.BudgetPeriodClose
			reopened sql.NullTime
		)
		if err := rows.Scan(&c.ID, &c.Code, &c.Year, &c.Month, &c.Revision, &c.ClosedAt, &c.ClosedBy,
			&reopened, &c.ReopenedBy, &c.ReopenReason); err != nil {
			return nil, fmt.Errorf("ngan_sach: đọc dòng chốt kỳ: %w", err)
		}
		if reopened.Valid {
			c.ReopenedAt = reopened.Time
		}
		ra = append(ra, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ngan_sach: duyệt chốt kỳ: %w", err)
	}
	return ra, nil
}

// BudgetPeriodClosesOfYear lists every close of one year — active AND reopened, the whole history —
// year close first, then by month, then by revision. GET /api/v1/budget-period-closes?year=
//
// LIMIT IS THE CEILING PLUS ONE, for the reason KhoanMucCuaBang gives.
func (s *NganSachStore) BudgetPeriodClosesOfYear(ctx context.Context,
	year int) ([]domain.BudgetPeriodClose, error) {

	rows, err := s.db.For(ctx).Query(ctx, colsPeriodClose, "budget_period_closes",
		`AND year = $2 ORDER BY month NULLS FIRST, revision, id LIMIT $3`,
		year, MaxPeriodClosesPerYear+1)
	if err != nil {
		return nil, fmt.Errorf("ngan_sach: đọc chốt kỳ: %w", err)
	}
	ra, err := scanPeriodCloses(rows)
	if err != nil {
		return nil, err
	}
	if len(ra) > MaxPeriodClosesPerYear {
		return nil, ErrTooManyPeriodCloses
	}
	return ra, nil
}

// activePeriodCloses — served by the unique index `budget_period_closes_one_active`, whose predicate
// is exactly `reopened_at IS NULL`.
const activePeriodCloses = `SELECT ` + colsPeriodClose + ` FROM budget_period_closes ` +
	`WHERE tenant_id = $1 AND year IN ($2, $3) AND reopened_at IS NULL ORDER BY year, month NULLS FIRST`

// ActiveBudgetPeriodCloses reads the ACTIVE closes of two years (pass one year twice for one), inside
// the write transaction, AFTER LockBudgetYears.
func (s *NganSachStore) ActiveBudgetPeriodCloses(ctx context.Context, tx *store.ScopedTx,
	yearA, yearB int) ([]domain.BudgetPeriodClose, error) {

	rows, err := tx.Underlying().QueryContext(ctx, activePeriodCloses, string(tx.TenantID()), yearA, yearB)
	if err != nil {
		return nil, fmt.Errorf("ngan_sach: đọc chốt kỳ đang hiệu lực: %w", err)
	}
	return scanPeriodCloses(rows)
}

const periodCloseByCode = `SELECT ` + colsPeriodClose + ` FROM budget_period_closes ` +
	`WHERE tenant_id = $1 AND code = $2`

// BudgetPeriodCloseByCode reads one close of this commune by its business code, inside the
// transaction. Another commune's code and a code never issued are one answer.
func (s *NganSachStore) BudgetPeriodCloseByCode(ctx context.Context, tx *store.ScopedTx,
	code string) (domain.BudgetPeriodClose, error) {

	rows, err := tx.Underlying().QueryContext(ctx, periodCloseByCode, string(tx.TenantID()), code)
	if err != nil {
		return domain.BudgetPeriodClose{}, fmt.Errorf("ngan_sach: đọc lần chốt kỳ: %w", err)
	}
	ra, err := scanPeriodCloses(rows)
	if err != nil {
		return domain.BudgetPeriodClose{}, err
	}
	if len(ra) == 0 {
		return domain.BudgetPeriodClose{}, domain.ErrBudgetPeriodCloseNotFound
	}
	return ra[0], nil
}

// nextPeriodCloseRevision COUNTS REOPENED ROWS ON PURPOSE — the reason LanKeTiep gives: a revision,
// and the code built from it, is never reissued. `IS NOT DISTINCT FROM` so the year close (NULL
// month) has its own sequence.
const nextPeriodCloseRevision = `SELECT COALESCE(MAX(revision), 0) + 1 FROM budget_period_closes
	WHERE tenant_id = $1 AND year = $2 AND month IS NOT DISTINCT FROM $3::int`

// NextBudgetPeriodCloseRevision — month 0 = the year close. Runs under LockBudgetYears, so two
// closes of one period cannot read the same MAX; `budget_period_closes_revision_once` is the floor.
func (s *NganSachStore) NextBudgetPeriodCloseRevision(ctx context.Context, tx *store.ScopedTx,
	year, month int) (int, error) {

	var rev int
	err := tx.Underlying().QueryRowContext(ctx, nextPeriodCloseRevision,
		string(tx.TenantID()), year, monthOrNil(month)).Scan(&rev)
	if err != nil {
		return 0, fmt.Errorf("ngan_sach: đọc lần chốt kế tiếp: %w", err)
	}
	return rev, nil
}

// insertPeriodClose — `closed_at` takes its default and the reopen trio is absent: a close is born
// active.
const insertPeriodClose = `INSERT INTO budget_period_closes
	(tenant_id, id, code, year, month, revision, closed_by)
	VALUES ($1, $2, $3, $4, $5, $6, $7)`

// InsertBudgetPeriodClose records one close.
//
// A UNIQUE VIOLATION IS domain.ErrPeriodAlreadyClosed, whichever of the three keys fired (one active
// per period, revision once, code once). Under LockBudgetYears the app has already checked, so the
// only way here is a writer that did not take the lock — an old replica during a rolling deploy
// (0012, question 4) — and "this period was just closed by somebody else" is the truth in every case.
func (s *NganSachStore) InsertBudgetPeriodClose(ctx context.Context, tx *store.ScopedTx,
	c domain.BudgetPeriodClose) error {

	_, err := tx.Exec(ctx, insertPeriodClose, string(tx.TenantID()),
		c.ID, c.Code, c.Year, monthOrNil(c.Month), c.Revision, c.ClosedBy)
	if isUniqueViolation(err) {
		return fmt.Errorf("ngan_sach: chèn chốt kỳ: %w", domain.ErrPeriodAlreadyClosed)
	}
	if err != nil {
		return fmt.Errorf("ngan_sach: chèn chốt kỳ: %w", err)
	}
	return nil
}

// reopenPeriodClose fills the reopen trio ONCE — the one UPDATE 0012's trigger admits.
// `AND reopened_at IS NULL` makes a second reopen a zero-row update (409) rather than a statement the
// trigger refuses with a 500.
const reopenPeriodClose = `UPDATE budget_period_closes
	SET reopened_at = now(), reopened_by = $3, reopen_reason = $4
	WHERE tenant_id = $1 AND id = $2 AND reopened_at IS NULL`

// ReopenBudgetPeriodClose — `by` is the staff business code (rule 6, invariant 8).
func (s *NganSachStore) ReopenBudgetPeriodClose(ctx context.Context, tx *store.ScopedTx,
	id, by, reason string) error {

	kq, err := tx.Exec(ctx, reopenPeriodClose, string(tx.TenantID()), id, by, reason)
	if err != nil {
		return fmt.Errorf("ngan_sach: mở chốt kỳ: %w", err)
	}
	return doiMotDongNganSach(kq, "mở chốt kỳ", domain.ErrCloseAlreadyReopened)
}

// monthOrNil writes the year close's month as NULL — 0 is not a month, and 0012's CHECK refuses it.
func monthOrNil(month int) any {
	if month == 0 {
		return nil
	}
	return month
}

// isUniqueViolation — SQLSTATE 23505. The constraint name is NOT compared: on a partitioned table the
// server reports the PARTITION's index name (`budget_period_closes_p03_…`), not the parent's.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
