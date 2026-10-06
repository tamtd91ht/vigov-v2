package store

// Integration tests for the auto-issued project code's counter (migration 0014), against a real
// PostgreSQL. SKIPPED UNLESS VIGOV_TEST_DSN IS SET — a green run without it means the code COMPILES.
//
// WHAT ONLY A SERVER CAN SAY, and therefore what this file is for: that `INSERT … ON CONFLICT … DO
// UPDATE` really holds the counter row until commit (a second transaction BLOCKS), that the advance
// never moves the counter backwards, and that one commune's series is not another's.

import (
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

func TestPgProjectSerialLockBlocksSecondTransaction(t *testing.T) {
	db := moKetNoi(t)
	commune, otherCommune := xaRieng(t)
	scoped := pkgstore.New(db)
	w := NewDuAnGhiStore(scoped)
	ctx := ctxXa(tenant.ID(commune))

	firstHolds := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- scoped.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
			n, err := w.LockProjectSerial(ctx, tx)
			if err != nil {
				return err
			}
			if n != 1 {
				t.Errorf("lần đầu next_number = %d, muốn 1", n)
			}
			close(firstHolds)
			<-release
			return w.AdvanceProjectSerial(ctx, tx, n+1)
		})
	}()
	<-firstHolds

	secondGot := make(chan int64, 1)
	secondErr := make(chan error, 1)
	go func() {
		secondErr <- scoped.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
			n, err := w.LockProjectSerial(ctx, tx)
			secondGot <- n
			return err
		})
	}()

	select {
	case n := <-secondGot:
		t.Fatalf("giao dịch thứ hai đọc được %d khi giao dịch đầu còn giữ khoá — upsert KHÔNG khoá dòng", n)
	case <-time.After(300 * time.Millisecond):
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("giao dịch đầu: %v", err)
	}
	if n := <-secondGot; n != 2 {
		t.Errorf("giao dịch thứ hai đọc %d, muốn 2 (số đã được giao dịch đầu tăng)", n)
	}
	if err := <-secondErr; err != nil {
		t.Fatalf("giao dịch thứ hai: %v", err)
	}

	// Another commune's series is its own: it starts at 1.
	other := ctxXa(tenant.ID(otherCommune))
	err := scoped.For(other).Tx(other, func(tx *pkgstore.ScopedTx) error {
		n, err := w.LockProjectSerial(other, tx)
		if err == nil && n != 1 {
			t.Errorf("xã khác next_number = %d, muốn 1", n)
		}
		return err
	})
	if err != nil {
		t.Fatalf("xã khác: %v", err)
	}
}

func TestPgProjectSerialNeverMovesBackwards(t *testing.T) {
	db := moKetNoi(t)
	commune, _ := xaRieng(t)
	scoped := pkgstore.New(db)
	w := NewDuAnGhiStore(scoped)
	ctx := ctxXa(tenant.ID(commune))

	run := func(f func(tx *pkgstore.ScopedTx) error) {
		t.Helper()
		if err := scoped.For(ctx).Tx(ctx, f); err != nil {
			t.Fatalf("tx: %v", err)
		}
	}
	run(func(tx *pkgstore.ScopedTx) error {
		if _, err := w.LockProjectSerial(ctx, tx); err != nil {
			return err
		}
		return w.AdvanceProjectSerial(ctx, tx, 10)
	})
	run(func(tx *pkgstore.ScopedTx) error { return w.AdvanceProjectSerial(ctx, tx, 3) })
	run(func(tx *pkgstore.ScopedTx) error {
		n, err := w.LockProjectSerial(ctx, tx)
		if err == nil && n != 10 {
			t.Errorf("next_number = %d, muốn 10 — bộ đếm không được lùi", n)
		}
		return err
	})
}
