package operatorstore

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io/fs"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/service-identity/internal/domain"
	"github.com/vihat/vigov/service-identity/migrations"
)

// The owner's hardening decisions of 28/09/2026 (TASK-04), at the statement level: idle timeout,
// temporary-password and pending-enrolment expiry, the CLI unlock.

func norm(s string) string { return strings.Join(strings.Fields(s), " ") }

func TestCreateAccountSetsTemporaryPasswordExpiry(t *testing.T) {
	r := &recorder{queryRows: map[string][][]driver.Value{"nextval('operator_code_seq')": {{int64(3)}}}}
	var got domain.OperatorAccount
	stmts := runTx(t, r, func(tx *Tx) error {
		var err error
		got, err = tx.CreateAccount(context.Background(), NewAccount{
			Email: "op@example.test", DisplayName: "Operator", PasswordHash: "h", CreatedBy: "system",
		}, t0)
		return err
	})
	want := t0.Add(24 * time.Hour)
	expect(t, "CreateAccount", insertBindings(t, stmts[1]), map[string]driver.Value{"temporary_password_expires_at": want})
	if got.TemporaryPasswordExpiresAt == nil || !got.TemporaryPasswordExpiresAt.Equal(want) {
		t.Errorf("returned expiry = %v, want %v", got.TemporaryPasswordExpiresAt, want)
	}
}

func TestSetPasswordExpiryFollowsMustChange(t *testing.T) {
	for _, mustChange := range []bool{true, false} {
		stmts := runTx(t, &recorder{}, func(tx *Tx) error {
			return tx.SetPassword(context.Background(), accountID, "hash", mustChange, t0)
		})
		got := assignments(t, stmts[0])["temporary_password_expires_at"]
		if mustChange {
			if tv, ok := got.(time.Time); !ok || !tv.Equal(t0.Add(24*time.Hour)) {
				t.Errorf("temporary password: expiry bound as %#v, want now+24h", got)
			}
		} else if got != nil {
			t.Errorf("own password: expiry bound as %#v, want NULL", got)
		}
	}
}

func TestPendingTOTPIsDatedAndGuarded(t *testing.T) {
	stmts := runTx(t, &recorder{}, func(tx *Tx) error {
		return tx.SetPendingTOTP(context.Background(), accountID, []byte("sealed"), t0)
	})
	expect(t, "SetPendingTOTP", assignments(t, stmts[0]), map[string]driver.Value{"pending_totp_created_at": t0})
	if q := norm(setPendingTOTP); !strings.Contains(q, "AND must_change_password AND temporary_password_expires_at > $3") {
		t.Errorf("SetPendingTOTP accepts an expired temporary password: %s", q)
	}

	stmts = runTx(t, &recorder{}, func(tx *Tx) error {
		return tx.ActivateTOTP(context.Background(), accountID, "hash", 5, []byte("sealed"), t0)
	})
	q := norm(stmts[0].query)
	for _, want := range []string{"pending_totp_created_at = NULL", "temporary_password_expires_at = NULL",
		"AND pending_totp_created_at > $6", "AND must_change_password AND temporary_password_expires_at > $2"} {
		if !strings.Contains(q, want) {
			t.Errorf("activateTOTP lacks %q: %s", want, q)
		}
	}
	if cut, ok := stmts[0].args[5].(time.Time); !ok || !cut.Equal(t0.Add(-10*time.Minute)) {
		t.Errorf("$6 = %#v, want now − 10 min", stmts[0].args[5])
	}
	if !strings.Contains(norm(resetMFA), "pending_totp_created_at = NULL") {
		t.Error("ResetMFA leaves a pending creation time behind")
	}
}

func TestIdleTimeoutInSessionStatements(t *testing.T) {
	for name, stmt := range map[string]string{"checkSession": checkSession, "touchSession": touchSession} {
		if !strings.Contains(norm(stmt), "coalesce(") || !strings.Contains(norm(stmt), "last_seen_at") ||
			!strings.Contains(norm(stmt), "created_at) > $3") {
			t.Errorf("%s has no idle predicate: %s", name, norm(stmt))
		}
	}
	cut := t0.Add(-5 * time.Minute)
	r := &recorder{}
	db := sql.OpenDB(recConnector{r: r})
	defer db.Close()
	_ = New(db).InTx(context.Background(), func(tx *Tx) error {
		_, _ = tx.CheckSession(context.Background(), "sid", t0)
		_ = tx.TouchSession(context.Background(), "sid", t0)
		return nil
	})
	for i, s := range r.stmts {
		if len(s.args) != 3 || !s.args[2].(time.Time).Equal(cut) {
			t.Errorf("statement %d: $3 = %#v, want now − 5 min", i, s.args)
		}
	}
}

func TestUnlockClearsTheLockAndNothingElse(t *testing.T) {
	stmts := runTx(t, &recorder{}, func(tx *Tx) error { return tx.Unlock(context.Background(), accountID, t0) })
	if len(stmts) != 1 {
		t.Fatalf("unlock sent %d statements — it must not revoke sessions or touch anything else", len(stmts))
	}
	q := norm(stmts[0].query)
	if !strings.Contains(q, "SET failed_attempts = 0, locked_until = NULL") || strings.Contains(q, "disabled_at") ||
		strings.Contains(q, "locked_until <=") {
		t.Errorf("unlock statement: %s", q)
	}
	r := &recorder{zeroRows: true}
	db := sql.OpenDB(recConnector{r: r})
	defer db.Close()
	if err := New(db).InTx(context.Background(), func(tx *Tx) error {
		return tx.Unlock(context.Background(), accountID, t0)
	}); !errors.Is(err, ErrNotFound) {
		t.Errorf("unlock of no account: %v", err)
	}
}

func TestAccountScanReadsExpiryColumns(t *testing.T) {
	exp, pend := t0.Add(24*time.Hour), t0.Add(-time.Minute)
	r := &recorder{queryRows: map[string][][]driver.Value{"WHERE id = $1": {{accountID, "VH-00001", "a@example.test",
		"Name", true, nil, true, int64(0), nil, nil, "", "", t0, "system", t0, exp, pend}}}}
	db := sql.OpenDB(recConnector{r: r})
	defer db.Close()
	a, err := New(db).ByID(context.Background(), accountID)
	if err != nil {
		t.Fatal(err)
	}
	if a.TemporaryPasswordExpiresAt == nil || !a.TemporaryPasswordExpiresAt.Equal(exp) ||
		a.PendingTOTPCreatedAt == nil || !a.PendingTOTPCreatedAt.Equal(pend) {
		t.Fatalf("expiry columns not read: %+v %v %v", a, a.TemporaryPasswordExpiresAt, a.PendingTOTPCreatedAt)
	}
}

// Migration 0014 backfills with an interval of its own; it must equal the constant the application
// sets the column from.
func TestMigration0014BackfillMatchesDomain(t *testing.T) {
	b, err := fs.ReadFile(migrations.FS, "0014_operator_credential_expiry.sql")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`SET temporary_password_expires_at = now\(\) \+ interval '(\d+) hours'`).FindSubmatch(b)
	if m == nil {
		t.Fatal("0014 has no temporary-password backfill in the expected shape")
	}
	h, _ := strconv.Atoi(string(m[1]))
	if time.Duration(h)*time.Hour != domain.TemporaryPasswordLifetime {
		t.Errorf("0014 backfills %dh, domain.TemporaryPasswordLifetime is %v", h, domain.TemporaryPasswordLifetime)
	}
}

// --- PostgreSQL ---------------------------------------------------------------------------------

func TestPGIdleTimeout(t *testing.T) {
	s := pgStore(t)
	a := mustCreate(t, s)
	ctx := context.Background()
	now := time.Now()
	sid := "pg-idle-" + a.ID
	if err := s.InTx(ctx, func(tx *Tx) error {
		_, err := tx.CreateSession(ctx, sid, a.ID, "", "", now)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CheckSession(ctx, sid, now.Add(4*time.Minute)); err != nil {
		t.Fatalf("4 min idle: %v", err)
	}
	if err := s.InTx(ctx, func(tx *Tx) error { return tx.TouchSession(ctx, sid, now.Add(4*time.Minute)) }); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CheckSession(ctx, sid, now.Add(8*time.Minute)); err != nil {
		t.Fatalf("touched 4 min ago: %v", err)
	}
	if _, err := s.CheckSession(ctx, sid, now.Add(9*time.Minute)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("5 min idle: err = %v, want ErrNotFound", err)
	}
	if err := s.InTx(ctx, func(tx *Tx) error { return tx.TouchSession(ctx, sid, now.Add(9*time.Minute)) }); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an idle session was revived by a touch: %v", err)
	}
}

func TestPGCredentialExpiryEnforced(t *testing.T) {
	s := pgStore(t)
	a := mustCreate(t, s) // created at time.Now(): temporary password expires in 24h
	ctx := context.Background()
	now := time.Now()
	late := now.Add(25 * time.Hour)
	if err := s.InTx(ctx, func(tx *Tx) error {
		return tx.SetPendingTOTP(ctx, a.ID, []byte("sealed-secret"), late)
	}); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("pending stored with an expired temporary password: %v", err)
	}
	if err := s.InTx(ctx, func(tx *Tx) error {
		return tx.SetPendingTOTP(ctx, a.ID, []byte("sealed-secret"), now)
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.InTx(ctx, func(tx *Tx) error {
		return tx.ActivateTOTP(ctx, a.ID, "own-hash", 1, []byte("sealed-secret"), now.Add(11*time.Minute))
	}); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("activated an 11-minute-old pending secret: %v", err)
	}
	got, _ := s.ByID(ctx, a.ID)
	if got.TOTPEnrolled() || got.PendingTOTPCreatedAt == nil || got.TemporaryPasswordExpiresAt == nil {
		t.Fatalf("after refused activation: %+v", got)
	}
}

// Migration 0014's CHECKs: no temporary password without an expiry, no pending secret undated.
func TestPGNoNullMeansForever(t *testing.T) {
	s := pgStore(t)
	a := mustCreate(t, s)
	for name, stmt := range map[string]string{
		"temporary password without expiry": `UPDATE operator_account SET temporary_password_expires_at = NULL WHERE id = $1`,
		"pending secret without date":       `UPDATE operator_account SET pending_totp_secret_sealed = '\x01', pending_totp_created_at = NULL WHERE id = $1`,
	} {
		if _, err := sharedDB.ExecContext(context.Background(), stmt, a.ID); err == nil {
			t.Errorf("%s: accepted by the database", name)
		}
	}
	_ = s
}

func TestPGUnlock(t *testing.T) {
	s := pgStore(t)
	a := mustCreate(t, s)
	ctx := context.Background()
	now := time.Now()
	_ = s.InTx(ctx, func(tx *Tx) error {
		for i := 0; i < domain.MaxFailedAttempts; i++ {
			if _, _, err := tx.RegisterFailure(ctx, a.ID, now); err != nil {
				return err
			}
		}
		return nil
	})
	if got, _ := s.ByID(ctx, a.ID); !got.LockedAt(now) || !got.LockedUntil.After(now.Add(11*time.Hour)) {
		t.Fatalf("lock is not 12h: %v", got.LockedUntil)
	}
	if err := s.InTx(ctx, func(tx *Tx) error { return tx.Unlock(ctx, a.ID, now) }); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.ByID(ctx, a.ID); got.LockedUntil != nil || got.FailedAttempts != 0 {
		t.Fatalf("after unlock: %+v", got)
	}
}
