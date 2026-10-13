package operatorstore

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Migration 0030 against a real PostgreSQL: at most one owner (operator_account_single_owner), never a
// disabled owner (operator_account_owner_not_disabled), and SetOwner's clear-then-set order inside one
// transaction. Skipped without VIGOV_TEST_DSN (TestMain in store_pg_test.go).

func ownerOf(t *testing.T, s *Store, id string) bool {
	t.Helper()
	a, err := s.ByID(context.Background(), id)
	if err != nil {
		t.Fatalf("read %s: %v", id, err)
	}
	return a.IsOwner
}

func designateOwner(s *Store, id string) (string, error) {
	var prev string
	err := s.InTx(context.Background(), func(tx *Tx) error {
		var err error
		prev, err = tx.SetOwner(context.Background(), id, time.Now())
		return err
	})
	return prev, err
}

func TestPGSetOwnerMovesTheSingleFlag(t *testing.T) {
	s := pgStore(t)
	ctx := context.Background()
	a, b := mustCreate(t, s), mustCreate(t, s)

	// Whoever holds the flag from an earlier test in this schema is cleared by the first call.
	if _, err := designateOwner(s, a.ID); err != nil {
		t.Fatalf("set owner a: %v", err)
	}
	if !ownerOf(t, s, a.ID) {
		t.Fatal("a is not the owner")
	}
	prev, err := designateOwner(s, b.ID)
	if err != nil || prev != a.Code {
		t.Fatalf("set owner b: prev %q err %v, want %q", prev, err, a.Code)
	}
	if ownerOf(t, s, a.ID) || !ownerOf(t, s, b.ID) {
		t.Fatal("the flag did not move from a to b")
	}
	if _, err := designateOwner(s, b.ID); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("already owner: %v", err)
	}

	// A disabled account cannot take the flag — and the failed transaction leaves b the owner (the
	// clear ran first and rolled back with the refused set).
	c := mustCreate(t, s)
	if err := s.InTx(ctx, func(tx *Tx) error { return tx.Disable(ctx, c.ID, "system", "test", time.Now()) }); err != nil {
		t.Fatal(err)
	}
	if _, err := designateOwner(s, c.ID); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("disabled account: %v", err)
	}
	if !ownerOf(t, s, b.ID) {
		t.Fatal("a refused set-owner cleared the current owner")
	}

	// The database itself refuses a second owner and a disabled owner, whatever path writes them.
	if _, err := sharedDB.ExecContext(ctx, `UPDATE operator_account SET is_owner = true WHERE id = $1`, a.ID); err == nil {
		t.Fatal("a second owner was accepted by the database")
	}
	if err := s.InTx(ctx, func(tx *Tx) error { return tx.Disable(ctx, b.ID, "system", "test", time.Now()) }); err == nil {
		t.Fatal("disabling the owner was accepted by the database")
	}
}
