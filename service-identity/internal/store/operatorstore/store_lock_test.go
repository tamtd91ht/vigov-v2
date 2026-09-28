package operatorstore

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/vihat/vigov/service-identity/internal/domain"
)

// Security review of TASK-03, findings 1 and 2: the lockout must hold under CONCURRENT attempts,
// and activation must promote only the ciphertext that was proved.

// --- recording driver: the statements say what they must ---------------------------------------

func TestForUpdateReadsLockTheRow(t *testing.T) {
	for name, fn := range map[string]func(tx *Tx) error{
		"ByEmailForUpdate": func(tx *Tx) error {
			_, err := tx.ByEmailForUpdate(context.Background(), "a@example.test")
			return err
		},
		"ByIDForUpdate": func(tx *Tx) error {
			_, err := tx.ByIDForUpdate(context.Background(), accountID)
			return err
		},
	} {
		r := &recorder{}
		db := sql.OpenDB(recConnector{r: r})
		_ = New(db).InTx(context.Background(), fn) // ErrNotFound from the empty recorder is expected
		db.Close()
		if len(r.stmts) != 1 || !strings.HasSuffix(strings.TrimSpace(r.stmts[0].query), "FOR UPDATE") {
			t.Errorf("%s does not end in FOR UPDATE: %v", name, r.stmts)
		}
	}
	// The plain reads stay plain: they serve paths that must not queue behind a sign-in.
	if strings.Contains(selectAccountByEmail, "FOR UPDATE") || strings.Contains(selectAccountByID, "FOR UPDATE") {
		t.Error("the plain account reads took a row lock")
	}
}

func TestFactorAcceptingStatementsCarryTheLockGuard(t *testing.T) {
	norm := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	for name, c := range map[string]struct{ stmt, guard string }{
		"recordTOTPStep":  {recordTOTPStep, "(locked_until IS NULL OR locked_until <= $3)"},
		"useRecoveryCode": {useRecoveryCode, "(a.locked_until IS NULL OR a.locked_until <= $3)"},
		"resetFailures":   {resetFailures, "(locked_until IS NULL OR locked_until <= $2)"},
	} {
		if !strings.Contains(norm(c.stmt), c.guard) {
			t.Errorf("%s lacks the lock guard %q: %s", name, c.guard, norm(c.stmt))
		}
	}
	// RecordTOTPStep binds now at $3; UseRecoveryCode's $3 is the same now it writes into used_at.
	stmts := runTx(t, &recorder{}, func(tx *Tx) error {
		_, err := tx.RecordTOTPStep(context.Background(), accountID, 7, t0)
		return err
	})
	if len(stmts[0].args) != 3 || !stmts[0].args[2].(time.Time).Equal(t0) {
		t.Errorf("RecordTOTPStep args = %#v, want now at $3", stmts[0].args)
	}
}

func TestResetFailuresRefusesWhileLocked(t *testing.T) {
	enrolled := t0.Add(-time.Hour)
	lockedRow := []driver.Value{accountID, "VH-00001", "a@example.test", "Name", false, enrolled, false,
		int64(0), t0.Add(10 * time.Minute), nil, "", "", t0, "system", t0}
	r := &recorder{zeroRows: true, queryRows: map[string][][]driver.Value{"WHERE id = $1": {lockedRow}}}
	db := sql.OpenDB(recConnector{r: r})
	defer db.Close()
	err := New(db).InTx(context.Background(), func(tx *Tx) error {
		return tx.ResetFailures(context.Background(), accountID, t0)
	})
	if !errors.Is(err, ErrAccountLocked) {
		t.Fatalf("reset on a locked account: err = %v, want ErrAccountLocked", err)
	}
}

func TestActivateTOTPRefusesWithoutProvedCiphertext(t *testing.T) {
	r := &recorder{}
	db := sql.OpenDB(recConnector{r: r})
	defer db.Close()
	err := New(db).InTx(context.Background(), func(tx *Tx) error {
		return tx.ActivateTOTP(context.Background(), accountID, "hash", 1, nil, t0)
	})
	if !errors.Is(err, ErrMissingField) || len(r.stmts) != 0 {
		t.Fatalf("activation with no proved ciphertext: err=%v statements=%d", err, len(r.stmts))
	}
	r = &recorder{zeroRows: true}
	db2 := sql.OpenDB(recConnector{r: r})
	defer db2.Close()
	err = New(db2).InTx(context.Background(), func(tx *Tx) error {
		return tx.ActivateTOTP(context.Background(), accountID, "hash", 1, []byte("stale"), t0)
	})
	if !errors.Is(err, ErrStateConflict) {
		t.Fatalf("pending secret replaced (0 rows): err = %v, want ErrStateConflict", err)
	}
}

// --- PostgreSQL: the database enforces it --------------------------------------------------------

// secondPool opens an independent pool on the test schema, so two transactions really run at once.
func secondPool(t *testing.T) *sql.DB {
	t.Helper()
	if testDSN == "" {
		t.Skip("VIGOV_TEST_DSN not set — skipping integration test")
	}
	cfg, err := pgx.ParseConfig(testDSN)
	if err != nil {
		t.Fatal(err)
	}
	schema := testSchema
	db := stdlib.OpenDB(*cfg, stdlib.OptionAfterConnect(func(ctx context.Context, c *pgx.Conn) error {
		_, err := c.Exec(ctx, "SET search_path TO "+schema)
		return err
	}))
	db.SetMaxOpenConns(4)
	t.Cleanup(func() { db.Close() })
	return db
}

func enrolAndLock(t *testing.T, s *Store, a domain.OperatorAccount, now time.Time) {
	t.Helper()
	ctx := context.Background()
	if err := s.InTx(ctx, func(tx *Tx) error {
		if err := tx.SetPendingTOTP(ctx, a.ID, []byte("sealed-secret"), now); err != nil {
			return err
		}
		if err := tx.ActivateTOTP(ctx, a.ID, "own-hash", 1000, []byte("sealed-secret"), now); err != nil {
			return err
		}
		for i := 0; i < domain.MaxFailedAttempts; i++ {
			if _, _, err := tx.RegisterFailure(ctx, a.ID, now); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("enrol and lock: %v", err)
	}
}

// While a lock is in force, no statement accepts a factor or clears the failure state — even
// when the caller did not take the row lock.
func TestPGLockGuardRefusesFactorsWhileLocked(t *testing.T) {
	s := pgStore(t)
	a := mustCreate(t, s)
	ctx := context.Background()
	now := time.Now()
	enrolAndLock(t, s, a, now)
	digest := make([]byte, 32)
	digest[0] = 1
	digests := make([][]byte, domain.RecoveryCodeCount)
	for i := range digests {
		d := make([]byte, 32)
		d[0], d[1] = 9, byte(i)
		digests[i] = d
	}
	if err := s.InTx(ctx, func(tx *Tx) error {
		_, err := tx.ReplaceRecoveryCodes(ctx, a.ID, digests, now)
		return err
	}); err != nil {
		t.Fatal(err)
	}

	during := now.Add(time.Minute)
	_ = s.InTx(ctx, func(tx *Tx) error {
		if ok, err := tx.RecordTOTPStep(ctx, a.ID, 2000, during); err != nil || ok {
			t.Errorf("TOTP step accepted while locked: ok=%v err=%v", ok, err)
		}
		if ok, err := tx.UseRecoveryCode(ctx, a.ID, digests[0], during); err != nil || ok {
			t.Errorf("recovery code consumed while locked: ok=%v err=%v", ok, err)
		}
		if err := tx.ResetFailures(ctx, a.ID, during); !errors.Is(err, ErrAccountLocked) {
			t.Errorf("reset while locked: err = %v, want ErrAccountLocked", err)
		}
		return nil
	})
	after := now.Add(domain.LockoutDuration + time.Second)
	_ = s.InTx(ctx, func(tx *Tx) error {
		if ok, err := tx.UseRecoveryCode(ctx, a.ID, digests[0], after); err != nil || !ok {
			t.Errorf("recovery code refused after the lock expired: ok=%v err=%v", ok, err)
		}
		if err := tx.ResetFailures(ctx, a.ID, after); err != nil {
			t.Errorf("reset after the lock expired: %v", err)
		}
		return nil
	})
}

// THE RACE THE REVIEW CONFIRMED: attempt A holds the account row (FOR UPDATE) and locks the account;
// attempt B, started meanwhile, must WAIT for A and then see A's lock — not read a stale "unlocked"
// row and go on to accept a code and clear the lock.
func TestPGForUpdateSerialisesParallelAttempts(t *testing.T) {
	pgStore(t) // skips without the DSN
	s := New(secondPool(t))
	a := mustCreate(t, s)
	ctx := context.Background()
	now := time.Now()

	aHasRow, releaseA := make(chan struct{}), make(chan struct{})
	aDone := make(chan error, 1)
	go func() {
		aDone <- s.InTx(ctx, func(tx *Tx) error {
			if _, err := tx.ByIDForUpdate(ctx, a.ID); err != nil {
				return err
			}
			close(aHasRow)
			<-releaseA
			for i := 0; i < domain.MaxFailedAttempts; i++ {
				if _, _, err := tx.RegisterFailure(ctx, a.ID, now); err != nil {
					return err
				}
			}
			return nil
		})
	}()
	<-aHasRow

	bSaw := make(chan domain.OperatorAccount, 1)
	bDone := make(chan error, 1)
	go func() {
		bDone <- s.InTx(ctx, func(tx *Tx) error {
			acc, err := tx.ByIDForUpdate(ctx, a.ID)
			if err != nil {
				return err
			}
			bSaw <- acc
			if err := tx.ResetFailures(ctx, a.ID, now); !errors.Is(err, ErrAccountLocked) {
				t.Errorf("B cleared A's lock: err = %v", err)
			}
			return nil
		})
	}()

	select {
	case <-bSaw:
		t.Fatal("B read the account row while A held it — the attempts are not serialised")
	case <-time.After(300 * time.Millisecond):
	}
	close(releaseA)
	if err := <-aDone; err != nil {
		t.Fatalf("A: %v", err)
	}
	select {
	case acc := <-bSaw:
		if !acc.LockedAt(now) {
			t.Error("B did not see the lock A committed — it would go on to check a code")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("B never proceeded after A committed")
	}
	if err := <-bDone; err != nil {
		t.Fatalf("B: %v", err)
	}
}

// Activation promotes only the ciphertext that was proved.
func TestPGActivateTOTPRequiresTheProvedCiphertext(t *testing.T) {
	s := pgStore(t)
	a := mustCreate(t, s)
	ctx := context.Background()
	now := time.Now()
	if err := s.InTx(ctx, func(tx *Tx) error {
		return tx.SetPendingTOTP(ctx, a.ID, []byte("replaced-by-a-parallel-begin"), now)
	}); err != nil {
		t.Fatal(err)
	}
	err := s.InTx(ctx, func(tx *Tx) error {
		return tx.ActivateTOTP(ctx, a.ID, "own-hash", 1000, []byte("the-one-that-was-proved"), now)
	})
	if !errors.Is(err, ErrStateConflict) {
		t.Fatalf("activated an unproved ciphertext: err = %v", err)
	}
	got, _ := s.ByID(ctx, a.ID)
	if got.TOTPEnrolled() {
		t.Fatal("factor active after a refused activation")
	}
}

// The database refuses a session longer than 8 hours (migration 0013).
func TestPGSessionLifetimeCap(t *testing.T) {
	s := pgStore(t)
	a := mustCreate(t, s)
	_, err := sharedDB.ExecContext(context.Background(), `INSERT INTO operator_session
		(id, operator_account_id, created_at, expires_at) VALUES ($1, $2, now(), now() + interval '9 hours')`,
		strings.Repeat("a", 64), a.ID)
	if err == nil || !strings.Contains(err.Error(), "operator_session_lifetime_cap") {
		t.Fatalf("a 9-hour session was accepted: %v", err)
	}
	_ = s
}
