package operatorstore

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/vihat/vigov/core/migrate"
	"github.com/vihat/vigov/service-identity/internal/domain"
	"github.com/vihat/vigov/service-identity/migrations"
)

// Integration tests against a real PostgreSQL: the append-only triggers, the sequence, the partial
// unique grant, the strict TOTP step compare and the lockout all live in SQL, and a mock would agree
// with whatever the Go code believes.
//
// THEY SKIP WITHOUT VIGOV_TEST_DSN AND THE PACKAGE STILL PRINTS `ok`. A green gate here does NOT
// mean these ran — read the skip lines. The DSN carries a password and lives only in the environment
// (rule 8).

var sharedDB *sql.DB

func TestMain(m *testing.M) {
	dsn := os.Getenv("VIGOV_TEST_DSN")
	if dsn == "" {
		os.Exit(m.Run()) // each pg test skips itself; the recording tests still run
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	schema := fmt.Sprintf("vigov_op_test_%d", time.Now().UnixNano())
	// ONE physical connection: `SET search_path` is session state (see crosstenant's TestMain).
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		fmt.Fprintln(os.Stderr, "create schema:", err)
		os.Exit(1)
	}
	if _, err := db.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		fmt.Fprintln(os.Stderr, "search_path:", err)
		os.Exit(1)
	}
	if _, err := migrate.Chay(ctx, db, migrations.FS, "identity"); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
	sharedDB = db
	code := m.Run()
	// A schema this suite created in a test database; it holds no archival data.
	if _, err := db.ExecContext(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); err != nil {
		fmt.Fprintln(os.Stderr, "drop schema:", err)
	}
	db.Close()
	os.Exit(code)
}

func pgStore(t *testing.T) *Store {
	t.Helper()
	if sharedDB == nil {
		t.Skip("VIGOV_TEST_DSN not set — skipping integration test")
	}
	return New(sharedDB)
}

var emailSeq atomic.Int64

func uniqueEmail() string {
	return fmt.Sprintf("op%d-%d@example.test", time.Now().UnixNano(), emailSeq.Add(1))
}

func mustCreate(t *testing.T, s *Store) domain.OperatorAccount {
	t.Helper()
	var a domain.OperatorAccount
	if err := s.InTx(context.Background(), func(tx *Tx) error {
		var err error
		a, err = tx.CreateAccount(context.Background(), NewAccount{
			Email: uniqueEmail(), DisplayName: "Test Operator", PasswordHash: "hash", CreatedBy: "system",
		}, time.Now())
		return err
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	return a
}

func TestPGSequenceCodesAreNeverReissued(t *testing.T) {
	s := pgStore(t)
	first := mustCreate(t, s)

	// A rolled-back creation burns its number.
	rollback := errors.New("rollback on purpose")
	_ = s.InTx(context.Background(), func(tx *Tx) error {
		if _, err := tx.CreateAccount(context.Background(), NewAccount{
			Email: uniqueEmail(), DisplayName: "x", PasswordHash: "h", CreatedBy: "system",
		}, time.Now()); err != nil {
			return err
		}
		return rollback
	})
	second := mustCreate(t, s)

	var n1, n2 int
	fmt.Sscanf(first.Code, "VH-%d", &n1)
	fmt.Sscanf(second.Code, "VH-%d", &n2)
	if !domain.ValidOperatorCode(first.Code) || n2 <= n1+1 {
		t.Errorf("codes %s then %s: the rolled-back number must be skipped, never handed out", first.Code, second.Code)
	}
}

func TestPGEmailUniqueCaseInsensitive(t *testing.T) {
	s := pgStore(t)
	a := mustCreate(t, s)
	err := s.InTx(context.Background(), func(tx *Tx) error {
		_, err := tx.CreateAccount(context.Background(), NewAccount{
			Email: strings.ToUpper(a.Email), DisplayName: "x", PasswordHash: "h", CreatedBy: "system",
		}, time.Now())
		return err
	})
	if !errors.Is(err, ErrEmailTaken) {
		t.Errorf("same email in another case: err = %v, want ErrEmailTaken", err)
	}
	got, err := s.ByEmail(context.Background(), strings.ToUpper(a.Email))
	if err != nil || got.ID != a.ID {
		t.Errorf("ByEmail case-folded: %v", err)
	}
}

func TestPGAuditLogIsAppendOnly(t *testing.T) {
	s := pgStore(t)
	a := mustCreate(t, s)
	if err := s.InTx(context.Background(), func(tx *Tx) error {
		return tx.AppendAudit(context.Background(), domain.OperatorAuditEntry{
			Actor: domain.SystemActor, Action: domain.OperatorAuditAccountCreated, Subject: a.Code,
			Reason: "ticket OPS-1",
		})
	}); err != nil {
		t.Fatalf("append: %v", err)
	}
	for _, stmt := range []string{
		"UPDATE operator_audit_log SET reason = 'x' WHERE subject = '" + a.Code + "'",
		"DELETE FROM operator_audit_log WHERE subject = '" + a.Code + "'",
		"TRUNCATE operator_audit_log",
	} {
		if _, err := sharedDB.ExecContext(context.Background(), stmt); err == nil ||
			!strings.Contains(err.Error(), "append-only") {
			t.Errorf("%q: err = %v, want the append-only refusal", strings.Fields(stmt)[0], err)
		}
	}
}

func TestPGAuditRefusesInternalIDActorAtTheDatabase(t *testing.T) {
	s := pgStore(t)
	a := mustCreate(t, s)
	_, err := sharedDB.ExecContext(context.Background(),
		`INSERT INTO operator_audit_log (actor, action, subject) VALUES ($1, 'operator.logged_out', $2)`, a.ID, a.Code)
	if err == nil {
		t.Error("the database accepted an internal id as the audit actor")
	}
}

func TestPGGrantPartialUnique(t *testing.T) {
	s := pgStore(t)
	a := mustCreate(t, s)
	ctx := context.Background()
	grant := func() error {
		return s.InTx(ctx, func(tx *Tx) error {
			return tx.Grant(ctx, a.ID, domain.OperatorPermissionTenantManage, "system", "ticket OPS-2", time.Now())
		})
	}
	if err := grant(); err != nil {
		t.Fatalf("first grant: %v", err)
	}
	if err := grant(); !errors.Is(err, ErrAlreadyGranted) {
		t.Errorf("second live grant: err = %v, want ErrAlreadyGranted", err)
	}
	if err := s.InTx(ctx, func(tx *Tx) error {
		return tx.Revoke(ctx, a.ID, domain.OperatorPermissionTenantManage, "system", "ticket OPS-3", time.Now())
	}); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if err := grant(); err != nil {
		t.Errorf("grant after revoke: %v — revoked rows must not block a new grant", err)
	}
	var total int
	if err := sharedDB.QueryRow(`SELECT count(*) FROM operator_permission_grant WHERE operator_account_id = $1`,
		a.ID).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Errorf("%d grant rows, want 2 — the revoked one is history and stays", total)
	}
	perms, err := s.ActivePermissions(ctx, a.ID)
	if err != nil || len(perms) != 1 || perms[0] != domain.OperatorPermissionTenantManage {
		t.Errorf("active = %v, %v", perms, err)
	}
}

func TestPGTOTPReplayStepCompare(t *testing.T) {
	s := pgStore(t)
	a := mustCreate(t, s)
	ctx := context.Background()
	now := time.Now()
	if err := s.InTx(ctx, func(tx *Tx) error {
		if err := tx.SetPendingTOTP(ctx, a.ID, []byte("sealed-secret"), now); err != nil {
			return err
		}
		return tx.ActivateTOTP(ctx, a.ID, "own-hash", 1000, now)
	}); err != nil {
		t.Fatalf("enrol: %v", err)
	}
	for _, c := range []struct {
		step int64
		want bool
	}{{1000, false}, {999, false}, {1001, true}, {1001, false}, {1003, true}} {
		var ok bool
		if err := s.InTx(ctx, func(tx *Tx) error {
			var err error
			ok, err = tx.RecordTOTPStep(ctx, a.ID, c.step)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		if ok != c.want {
			t.Errorf("step %d accepted=%v, want %v", c.step, ok, c.want)
		}
	}
	got, _ := s.ByID(ctx, a.ID)
	if !got.TOTPEnrolled() || got.MustChangePassword || got.TOTPPending {
		t.Errorf("after activation: %+v", got)
	}
}

func TestPGLockoutAfterFiveFailures(t *testing.T) {
	s := pgStore(t)
	a := mustCreate(t, s)
	ctx := context.Background()
	now := time.Now()
	for i := 1; i <= domain.MaxFailedAttempts; i++ {
		var locked bool
		if err := s.InTx(ctx, func(tx *Tx) error {
			var err error
			locked, _, err = tx.RegisterFailure(ctx, a.ID, now)
			return err
		}); err != nil {
			t.Fatalf("failure %d: %v", i, err)
		}
		if locked != (i == domain.MaxFailedAttempts) {
			t.Errorf("failure %d: locked=%v", i, locked)
		}
	}
	got, _ := s.ByID(ctx, a.ID)
	if !got.LockedAt(now) || got.LockedAt(now.Add(domain.LockoutDuration)) {
		t.Errorf("lock window wrong: %v", got.LockedUntil)
	}
	// A failure during the lock neither counts nor extends it.
	err := s.InTx(ctx, func(tx *Tx) error {
		_, _, err := tx.RegisterFailure(ctx, a.ID, now.Add(time.Minute))
		return err
	})
	if !errors.Is(err, ErrAccountLocked) {
		t.Errorf("failure during lock: err = %v, want ErrAccountLocked", err)
	}
	// After the window a fresh count starts.
	var locked bool
	_ = s.InTx(ctx, func(tx *Tx) error {
		var err error
		locked, _, err = tx.RegisterFailure(ctx, a.ID, now.Add(domain.LockoutDuration+time.Second))
		return err
	})
	if locked {
		t.Error("the first failure after the window locked again")
	}
}

func TestPGSessionLifecycle(t *testing.T) {
	s := pgStore(t)
	a := mustCreate(t, s)
	ctx := context.Background()
	now := time.Now()
	const sid = "pg-test-session-id"
	sidA, sidB := sid+"-a-"+a.ID, sid+"-b-"+a.ID
	if err := s.InTx(ctx, func(tx *Tx) error {
		if _, err := tx.CreateSession(ctx, sidA, a.ID, "203.0.113.1", "UA", now); err != nil {
			return err
		}
		_, err := tx.CreateSession(ctx, sidB, a.ID, "", "", now)
		return err
	}); err != nil {
		t.Fatalf("create sessions: %v", err)
	}
	got, err := s.CheckSession(ctx, sidA, now.Add(time.Hour))
	if err != nil || got.AccountCode != a.Code {
		t.Fatalf("live session: %+v %v", got, err)
	}
	if _, err := s.CheckSession(ctx, sidA, now.Add(domain.SessionLifetime)); !errors.Is(err, ErrNotFound) {
		t.Errorf("at 8h: err = %v, want ErrNotFound (absolute lifetime)", err)
	}
	var stored int
	_ = sharedDB.QueryRow(`SELECT count(*) FROM operator_session WHERE id = $1`, sidA).Scan(&stored)
	if stored != 0 {
		t.Error("the raw sid is stored")
	}

	// A password change revokes every session.
	if err := s.InTx(ctx, func(tx *Tx) error {
		return tx.SetPassword(ctx, a.ID, "new-hash", false, now)
	}); err != nil {
		t.Fatal(err)
	}
	for _, x := range []string{sidA, sidB} {
		if _, err := s.CheckSession(ctx, x, now.Add(time.Minute)); !errors.Is(err, ErrNotFound) {
			t.Errorf("session survived a password change: %v", err)
		}
	}

	// A disabled account's session is refused even if not yet revoked.
	const sidC = "pg-test-session-c"
	_ = s.InTx(ctx, func(tx *Tx) error {
		_, err := tx.CreateSession(ctx, sidC+a.ID, a.ID, "", "", now)
		return err
	})
	if err := s.InTx(ctx, func(tx *Tx) error {
		return tx.Disable(ctx, a.ID, "system", "ticket OPS-4", now)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CheckSession(ctx, sidC+a.ID, now.Add(time.Minute)); !errors.Is(err, ErrNotFound) {
		t.Errorf("disabled account's session accepted: %v", err)
	}
}

func TestPGRecoveryCodesSingleUseAndVoidedOnRegenerate(t *testing.T) {
	s := pgStore(t)
	a := mustCreate(t, s)
	ctx := context.Background()
	now := time.Now()
	// Digests, the shape operatorauth.HashRecoveryCode produces (this package does not import it).
	batch := func(prefix string) [][]byte {
		out := make([][]byte, domain.RecoveryCodeCount)
		for i := range out {
			sum := sha256.Sum256([]byte(fmt.Sprintf("%s-%s-%02d", prefix, a.ID, i)))
			out[i] = sum[:]
		}
		return out
	}
	use := func(code []byte) bool {
		var ok bool
		if err := s.InTx(ctx, func(tx *Tx) error {
			var err error
			ok, err = tx.UseRecoveryCode(ctx, a.ID, code, now)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return ok
	}
	first := batch("first")
	if err := s.InTx(ctx, func(tx *Tx) error {
		_, err := tx.ReplaceRecoveryCodes(ctx, a.ID, first, now)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if !use(first[0]) {
		t.Fatal("a fresh code was refused")
	}
	if use(first[0]) {
		t.Error("a code worked twice")
	}
	if err := s.InTx(ctx, func(tx *Tx) error {
		_, err := tx.ReplaceRecoveryCodes(ctx, a.ID, batch("second"), now)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if use(first[1]) {
		t.Error("a code of the voided batch still works")
	}
	if !use(batch("second")[0]) {
		t.Error("a code of the new batch was refused")
	}
}
