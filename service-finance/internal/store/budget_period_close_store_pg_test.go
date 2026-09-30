package store

import (
	"context"
	"errors"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// The store half of budget period close (budget_period_close.go) against a real PostgreSQL: the
// advisory lock statement the server must accept (hashtextextended, the casts), the NULL-month revision
// sequence, the unique violation mapped to ErrPeriodAlreadyClosed THROUGH A PARTITION'S INDEX, and the
// one-time reopen fill. SKIPPED UNLESS VIGOV_TEST_DSN IS SET (see budget_period_close_pg_test.go).
func TestBudgetPeriodCloseStore(t *testing.T) {
	db := moKetNoi(t)
	a, b := xaRieng(t)
	kho := pkgstore.New(db)
	s := NewNganSachStore(kho)
	ctxA := tenant.Into(context.Background(), tenant.ID(a))
	ctxB := tenant.Into(context.Background(), tenant.ID(b))

	insert := func(ctx context.Context, id string, year, month int) error {
		return kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
			if err := s.LockBudgetYears(ctx, tx, year, year+1); err != nil {
				return err
			}
			rev, err := s.NextBudgetPeriodCloseRevision(ctx, tx, year, month)
			if err != nil {
				return err
			}
			return s.InsertBudgetPeriodClose(ctx, tx, domain.BudgetPeriodClose{
				ID: id, Code: domain.BudgetPeriodCloseCode(year, month, rev), Year: year, Month: month,
				Revision: rev, ClosedBy: "CB-00012"})
		})
	}

	mustOK(t, "close 09/2026", insert(ctxA, "k1", 2026, 9))
	mustOK(t, "close year 2026", insert(ctxA, "y1", 2026, 0))
	if err := insert(ctxA, "k2", 2026, 9); !errors.Is(err, domain.ErrPeriodAlreadyClosed) {
		t.Fatalf("second active close = %v, want ErrPeriodAlreadyClosed", err)
	}
	mustOK(t, "commune B closes the same month", insert(ctxB, "k1", 2026, 9))

	err := kho.For(ctxA).Tx(ctxA, func(tx *pkgstore.ScopedTx) error {
		active, err := s.ActiveBudgetPeriodCloses(ctxA, tx, 2026, 2026)
		if err != nil {
			return err
		}
		if len(active) != 2 {
			t.Errorf("active closes of 2026 in A = %d, want 2", len(active))
		}
		c, err := s.BudgetPeriodCloseByCode(ctxA, tx, "CK-2026-09-01")
		if err != nil {
			return err
		}
		if err := s.ReopenBudgetPeriodClose(ctxA, tx, c.ID, "CB-00003", "Ghi sót đợt thu"); err != nil {
			return err
		}
		if err := s.ReopenBudgetPeriodClose(ctxA, tx, c.ID, "CB-00003", "lần hai"); !errors.Is(err, domain.ErrCloseAlreadyReopened) {
			t.Errorf("second reopen = %v, want ErrCloseAlreadyReopened", err)
		}
		return nil
	})
	mustOK(t, "reopen", err)
	mustOK(t, "re-close 09/2026 as revision 2", insert(ctxA, "k3", 2026, 9))

	all, err := s.BudgetPeriodClosesOfYear(ctxA, 2026)
	mustOK(t, "list", err)
	if len(all) != 3 || all[0].Code != "CK-2026-CN-01" || all[2].Code != "CK-2026-09-02" || all[1].Active() {
		t.Fatalf("history = %+v", all)
	}
}
