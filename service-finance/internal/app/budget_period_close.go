package app

// Budget period close (chốt kỳ), migration 0012 — the close and reopen use cases, and the two guards
// every budget write runs inside its transaction. User decision 30/09/2026 (ledger service-finance
// `thu-chi-ngan-sach-82` (g), `thu-chi-chot-ky-va-cong-khai`).
//
// THE SHAPE OF EVERY GUARDED WRITE, in this order, all in ONE transaction:
//
//	1. the sheet's FOR UPDATE lock (khoaVaDocCay / BangTheoIDDeSua) — as before
//	2. the (tenant, year) advisory lock(s), ascending (store.LockBudgetYears)
//	3. read the ACTIVE closes of those years, ask domain whether one locks this write
//	4. the write and its audit entry — as before
//
// Step 2 before step 3 is what makes the check hold: a close takes the same advisory lock before it
// inserts, so a write that saw "no close" commits before the close can start, and a close that
// committed first is seen by the write. Without it the two could interleave and an entry would land
// in a month that reads as closed (0012's RACE note).

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// guardSheetYear refuses a write on a sheet of `sheetYear` — the sheet itself, its lines, its
// hand-entered values — when that year has an active YEAR close. A month close does not lock these
// (hand values carry no date; domain file header).
func (uc *NganSach) guardSheetYear(ctx context.Context, tx *store.ScopedTx, sheetYear int) error {
	if err := uc.kho.LockBudgetYears(ctx, tx, sheetYear); err != nil {
		return err
	}
	closes, err := uc.kho.ActiveBudgetPeriodCloses(ctx, tx, sheetYear, sheetYear)
	if err != nil {
		return err
	}
	if c, locked := domain.SheetLockingClose(closes, sheetYear); locked {
		return domain.SheetYearClosedError(c)
	}
	return nil
}

// guardEntryPeriod refuses adding or removing an entry dated `date` on a sheet of `sheetYear` when
// the date's month, the date's year, or the sheet's year is closed (domain.EntryLockingClose). When
// the two years differ both locks are taken, ascending.
func (uc *NganSach) guardEntryPeriod(ctx context.Context, tx *store.ScopedTx, sheetYear int,
	date time.Time) error {

	if err := uc.kho.LockBudgetYears(ctx, tx, sheetYear, date.Year()); err != nil {
		return err
	}
	closes, err := uc.kho.ActiveBudgetPeriodCloses(ctx, tx, sheetYear, date.Year())
	if err != nil {
		return err
	}
	if c, locked := domain.EntryLockingClose(closes, date, sheetYear); locked {
		return domain.EntryPeriodClosedError(c)
	}
	return nil
}

// BudgetPeriodCloseRequest is one close as the form sends it. Month 0 = the WHOLE YEAR.
//
// THERE IS NO Code AND NO Revision: both are issued here, from what is already in the table.
type BudgetPeriodCloseRequest struct {
	Year  int
	Month int
}

// CloseBudgetPeriod closes one month, or one whole year.
//
// REFUSED WITH 409 when the period already has an active close — checked under the advisory lock so
// the sentence can name the existing close; the unique index is the floor under a writer that did
// not take the lock.
//
// ⚠ A FUTURE PERIOD IS NOT REFUSED. Nothing in the user's decision says a month must have ended
// before it is closed, and a close only ever restricts writes; stated as an assumption in the
// hand-back rather than decided silently in either direction.
func (uc *NganSach) CloseBudgetPeriod(ctx context.Context, req BudgetPeriodCloseRequest,
	nguoi audit.Actor) (domain.BudgetPeriodClose, error) {

	if err := domain.ValidateClosePeriod(req.Year, req.Month); err != nil {
		return domain.BudgetPeriodClose{}, err
	}
	if err := coNguoiThucHien(nguoi); err != nil {
		return domain.BudgetPeriodClose{}, err
	}
	id, err := uc.sinhID()
	if err != nil {
		return domain.BudgetPeriodClose{}, fmt.Errorf("ngan_sach: sinh mã chốt kỳ: %w", err)
	}

	var closed domain.BudgetPeriodClose
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if err := uc.kho.LockBudgetYears(ctx, tx, req.Year); err != nil {
			return err
		}
		active, err := uc.kho.ActiveBudgetPeriodCloses(ctx, tx, req.Year, req.Year)
		if err != nil {
			return err
		}
		if c, found := domain.FindActiveClose(active, req.Year, req.Month); found {
			return domain.AlreadyClosedError(c)
		}
		rev, err := uc.kho.NextBudgetPeriodCloseRevision(ctx, tx, req.Year, req.Month)
		if err != nil {
			return err
		}
		closed = domain.BudgetPeriodClose{
			ID: id, Code: domain.BudgetPeriodCloseCode(req.Year, req.Month, rev),
			Year: req.Year, Month: req.Month, Revision: rev,
			// THE STAFF BUSINESS CODE — audit.Actor.ID is Principal.Ma (http.nguoiThucHien).
			ClosedBy: nguoi.ID,
		}
		if err := uc.kho.InsertBudgetPeriodClose(ctx, tx, closed); err != nil {
			return err
		}
		delta, err := json.Marshal(map[string]any{"sau": periodCloseSummary(closed)})
		if err != nil {
			return fmt.Errorf("ngan_sach: mã hoá delta: %w", err)
		}
		// SAME TRANSACTION AS THE INSERT (rule 6, invariant 3). Subject = the close's own code.
		return audit.Write(ctx, tx, audit.Entry{
			Actor: nguoi, Action: ActionBudgetPeriodClose, Subject: closed.Code, Delta: delta,
		})
	})
	if err != nil {
		return domain.BudgetPeriodClose{}, bocNganSach(ctx, "chốt kỳ", err)
	}
	return closed, nil
}

// ReopenBudgetPeriodClose reopens one close, once, with a mandatory reason. The row is not deleted
// and nothing but the reopen trio moves (0012's trigger); closing the period again is a NEW close.
func (uc *NganSach) ReopenBudgetPeriodClose(ctx context.Context, code, reasonRaw string,
	nguoi audit.Actor) (domain.BudgetPeriodClose, error) {

	if code == "" {
		return domain.BudgetPeriodClose{}, domain.ErrBudgetPeriodCloseNotFound
	}
	reason, err := domain.NormaliseReopenReason(reasonRaw)
	if err != nil {
		return domain.BudgetPeriodClose{}, err
	}
	if err := coNguoiThucHien(nguoi); err != nil {
		return domain.BudgetPeriodClose{}, err
	}

	var after domain.BudgetPeriodClose
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.kho.BudgetPeriodCloseByCode(ctx, tx, code)
		if err != nil {
			return err
		}
		// The year is only known from the row, so the lock follows the read. A concurrent reopen of
		// the same close is caught by the UPDATE's `AND reopened_at IS NULL` either way.
		if err := uc.kho.LockBudgetYears(ctx, tx, before.Year); err != nil {
			return err
		}
		if !before.Active() {
			return domain.AlreadyReopenedError(before)
		}
		// `reopened_by` HOLDS THE STAFF BUSINESS CODE, the same value the entry's actor holds.
		if err := uc.kho.ReopenBudgetPeriodClose(ctx, tx, before.ID, nguoi.ID, reason); err != nil {
			return err
		}
		after, err = uc.kho.BudgetPeriodCloseByCode(ctx, tx, code)
		if err != nil {
			return err
		}

		// THE REASON IS IN THE ENTRY AS WELL AS IN THE ROW — the reason GoKhoanMuc gives: the entry is
		// the append-only record of the act.
		delta, err := json.Marshal(map[string]any{
			"truoc": periodCloseSummary(before),
			"ly_do": reason,
		})
		if err != nil {
			return fmt.Errorf("ngan_sach: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: nguoi, Action: ActionBudgetPeriodReopen, Subject: before.Code, Delta: delta,
		})
	})
	if err != nil {
		return domain.BudgetPeriodClose{}, bocNganSach(ctx, "mở chốt kỳ", err)
	}
	return after, nil
}

// periodCloseSummary is the audit delta's view of one close. Nothing here is personal data.
func periodCloseSummary(c domain.BudgetPeriodClose) map[string]any {
	var month any
	if c.Month != 0 {
		month = c.Month
	}
	ra := map[string]any{
		"close_id":  c.ID,
		"code":      c.Code,
		"year":      c.Year,
		"month":     month, // null = the whole year
		"revision":  c.Revision,
		"closed_by": c.ClosedBy,
	}
	if !c.ClosedAt.IsZero() {
		ra["closed_at"] = c.ClosedAt.UTC().Format(time.RFC3339)
	}
	return ra
}
