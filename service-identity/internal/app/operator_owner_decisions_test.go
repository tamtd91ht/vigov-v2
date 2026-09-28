package app

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/vihat/vigov/service-identity/internal/domain"
	"github.com/vihat/vigov/service-identity/internal/operatorauth"
)

// The owner's hardening decisions of 28/09/2026 (TASK-04).

// Decision 1: 5 failures lock for 12 hours; only the ticketed `unlock` lifts it early; Enable does not.
func TestOperatorLockoutTwelveHoursAndUnlock(t *testing.T) {
	h := newOpHarness(t)
	code, pw, _, _ := h.enrolled(t)
	ctx := context.Background()
	for i := 0; i < domain.MaxFailedAttempts; i++ {
		_, _ = login(h, opEmail, "wrong-password-xyz", "", "")
	}
	start := h.clock.t
	h.clock.t = start.Add(12*time.Hour - time.Minute)
	if _, err := login(h, opEmail, pw, h.currentCode(t, code), ""); !errors.Is(err, ErrOperatorLoginFailed) {
		t.Fatalf("still inside the 12-hour lock, sign-in: %v", err)
	}

	// Enable does NOT lift a failure lockout (disable/enable is the other lock).
	if err := h.admin.Disable(ctx, code, "check", "OPS-50"); err != nil {
		t.Fatal(err)
	}
	if err := h.admin.Enable(ctx, code, "checked", "OPS-51"); err != nil {
		t.Fatal(err)
	}
	if !h.store.account(t, code).LockedAt(h.clock.t) {
		t.Fatal("Enable silently cleared the failure lockout")
	}

	if err := h.admin.Unlock(ctx, code, " "); !errors.Is(err, ErrTicketRequired) {
		t.Fatalf("unlock without a ticket: %v", err)
	}
	if err := h.admin.Unlock(ctx, code, "OPS-52"); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	tx := h.store.lastTx()
	if !slices.Equal(tx.ops, []string{"ByCode", "Unlock", "AppendAudit:operator.unlocked"}) || !tx.committed {
		t.Fatalf("unlock ops: %v", tx.ops)
	}
	e := h.store.state.audit[len(h.store.state.audit)-1]
	if e.Actor != domain.SystemActor || e.Reason != "ticket:OPS-52" || e.Subject != code || e.Before["locked"] != true {
		t.Fatalf("unlock entry = %+v", e)
	}
	if acc := h.store.account(t, code); acc.LockedUntil != nil || acc.FailedAttempts != 0 {
		t.Fatalf("after unlock: %+v", acc)
	}
	if _, err := login(h, opEmail, pw, h.currentCode(t, code), ""); err != nil {
		t.Fatalf("sign-in after unlock: %v", err)
	}
	if err := h.admin.Unlock(ctx, code, "OPS-53"); !errors.Is(err, ErrOperatorNotLocked) {
		t.Fatalf("unlock of an unlocked account: %v", err)
	}
}

// Decision 2: 5 minutes idle ends a session; activity (ResolveSession) keeps it alive; the 8-hour
// absolute cap still holds.
func TestOperatorSessionIdleTimeout(t *testing.T) {
	h := newOpHarness(t)
	_, _, _, tok := h.enrolled(t) // the clock is 30 s past the session's creation
	ctx := context.Background()

	h.clock.t = h.clock.t.Add(4 * time.Minute)
	if _, err := h.auth.ResolveSession(ctx, tok); err != nil {
		t.Fatalf("4.5 min idle: %v", err)
	}
	h.clock.t = h.clock.t.Add(4 * time.Minute) // 8.5 min since creation, 4 since last activity
	if _, err := h.auth.ResolveSession(ctx, tok); err != nil {
		t.Fatalf("activity did not keep the session alive: %v", err)
	}
	h.clock.t = h.clock.t.Add(domain.SessionIdleTimeout)
	if _, err := h.auth.ResolveSession(ctx, tok); !errors.Is(err, ErrOperatorUnauthenticated) {
		t.Fatalf("5 min idle: %v", err)
	}

	// Absolute cap: active every 4 minutes, still over at 8 hours.
	h2 := newOpHarness(t)
	_, _, _, tok2 := h2.enrolled(t)
	created := h2.clock.t.Add(-operatorauth.TOTPPeriod)
	for h2.clock.t.Before(created.Add(domain.SessionLifetime - 4*time.Minute)) {
		h2.clock.t = h2.clock.t.Add(4 * time.Minute)
		if _, err := h2.auth.ResolveSession(ctx, tok2); err != nil {
			t.Fatalf("active session refused at %v: %v", h2.clock.t.Sub(created), err)
		}
	}
	h2.clock.t = created.Add(domain.SessionLifetime)
	if _, err := h2.auth.ResolveSession(ctx, tok2); !errors.Is(err, ErrOperatorUnauthenticated) {
		t.Fatalf("activity extended the session past 8 hours: %v", err)
	}
}

// Decision 3a: a temporary password stops working after 24 hours; `reset-mfa` reissues.
func TestOperatorTemporaryPasswordExpires(t *testing.T) {
	h := newOpHarness(t)
	code, temp := h.create(t)
	ctx := context.Background()

	h.clock.t = h.clock.t.Add(domain.TemporaryPasswordLifetime - time.Minute)
	if _, err := login(h, opEmail, temp, "", ""); !errors.Is(err, ErrOperatorEnrollmentRequired) {
		t.Fatalf("inside 24h: %v", err)
	}
	h.clock.t = h.clock.t.Add(time.Minute)
	if _, err := login(h, opEmail, temp, "", ""); !errors.Is(err, ErrOperatorLoginFailed) {
		t.Fatalf("at 24h the temporary password must be refused like any wrong credential: %v", err)
	}
	if _, err := h.auth.BeginEnrollment(ctx, OperatorEnrollmentRequest{Email: opEmail, TemporaryPassword: temp, IP: opIP}); !errors.Is(err, ErrOperatorLoginFailed) {
		t.Fatalf("enrolment with an expired temporary password: %v", err)
	}
	if got := h.store.account(t, code).FailedAttempts; got != 0 {
		t.Fatalf("an expired temporary password was counted as a guess: %d", got)
	}
	list, _ := h.admin.List(ctx)
	if list[0].Status != OperatorStatusTemporaryPasswordExpired {
		t.Fatalf("list status = %s", list[0].Status)
	}

	fresh, err := h.admin.ResetMFA(ctx, code, "OPS-60")
	if err != nil {
		t.Fatal(err)
	}
	if exp := h.store.account(t, code).TemporaryPasswordExpiresAt; exp == nil || !exp.Equal(h.clock.t.Add(domain.TemporaryPasswordLifetime)) {
		t.Fatalf("reset-mfa did not set a fresh 24h expiry: %v", exp)
	}
	if _, err := login(h, opEmail, string(fresh.Lo()), "", ""); !errors.Is(err, ErrOperatorEnrollmentRequired) {
		t.Fatalf("reissued temporary password: %v", err)
	}
}

// Decision 3b: a pending enrolment secret can be completed for 10 minutes only.
func TestOperatorPendingEnrollmentExpires(t *testing.T) {
	h := newOpHarness(t)
	code, temp := h.create(t)
	ctx := context.Background()
	begin := func() {
		t.Helper()
		if _, err := h.auth.BeginEnrollment(ctx, OperatorEnrollmentRequest{Email: opEmail, TemporaryPassword: temp, IP: opIP}); err != nil {
			t.Fatalf("begin: %v", err)
		}
	}
	complete := func() error {
		s := h.totpSecretOf(t, code, true)
		_, err := h.auth.CompleteEnrollment(ctx, OperatorEnrollmentCompletion{Email: opEmail, TemporaryPassword: temp,
			NewPassword: opNewPw, TOTPCode: operatorauth.CodeAt(s, h.clock.t), IP: opIP, UserAgent: opUA})
		return err
	}
	begin()
	h.clock.t = h.clock.t.Add(domain.PendingTOTPLifetime)
	if err := complete(); !errors.Is(err, ErrOperatorLoginFailed) {
		t.Fatalf("completing a 10-minute-old pending secret: %v", err)
	}
	acc := h.store.account(t, code)
	if acc.TOTPEnrolled() || acc.FailedAttempts != 0 {
		t.Fatalf("expired pending: enrolled=%v failed=%d", acc.TOTPEnrolled(), acc.FailedAttempts)
	}
	begin()
	h.clock.t = h.clock.t.Add(domain.PendingTOTPLifetime - time.Second)
	if err := complete(); err != nil {
		t.Fatalf("inside 10 minutes: %v", err)
	}
	got := h.store.account(t, code)
	if got.TemporaryPasswordExpiresAt != nil || got.PendingTOTPCreatedAt != nil {
		t.Fatalf("enrolment left expiry columns behind: %+v", got)
	}
}

// Decision 4: BeginEnrollment writes totp_enrollment_started in its own transaction.
func TestOperatorEnrollmentStartIsAudited(t *testing.T) {
	h := newOpHarness(t)
	code, temp := h.create(t)
	if _, err := h.auth.BeginEnrollment(context.Background(), OperatorEnrollmentRequest{Email: opEmail, TemporaryPassword: temp, IP: opIP}); err != nil {
		t.Fatal(err)
	}
	tx := h.store.lastTx()
	i, j := slices.Index(tx.ops, "SetPendingTOTP"), slices.Index(tx.ops, "AppendAudit:operator.totp_enrollment_started")
	if i < 0 || j < i || !tx.committed {
		t.Fatalf("enrolment start not audited in its tx: %v", tx.ops)
	}
	e := h.store.state.audit[len(h.store.state.audit)-1]
	if e.Actor != code || e.Subject != code || e.ActorIP != opIP || e.After["pending_expires_at"] == nil {
		t.Fatalf("entry = %+v", e)
	}

	// The entry failing takes the pending secret down with it.
	h2 := newOpHarness(t)
	code2, temp2 := h2.create(t)
	h2.store.failAudit = domain.OperatorAuditTOTPEnrollmentStarted
	if _, err := h2.auth.BeginEnrollment(context.Background(), OperatorEnrollmentRequest{Email: opEmail, TemporaryPassword: temp2, IP: opIP}); err == nil {
		t.Fatal("enrolment started although its audit entry failed")
	}
	if h2.store.account(t, code2).TOTPPending {
		t.Fatal("pending secret stored without its audit entry")
	}
}

// Decision 5: a lockout caused by a SESSION HOLDER revokes every session; one caused from outside
// does not.
func TestOperatorLockoutBySessionHolderRevokesSessions(t *testing.T) {
	h := newOpHarness(t)
	code, pw, _, tokA := h.enrolled(t)
	ctx := context.Background()
	b, err := login(h, opEmail, pw, h.currentCode(t, code), "")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < domain.MaxFailedAttempts; i++ {
		err := h.auth.ChangePassword(ctx, OperatorPasswordChange{Token: tokA, CurrentPassword: "wrong-password-xyz",
			NewPassword: "another-password-for-tests", TOTPCode: "000000", IP: opIP})
		if !errors.Is(err, ErrOperatorLoginFailed) {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
	}
	tx := h.store.lastTx()
	i, j := slices.Index(tx.ops, "RevokeAllSessions"), slices.Index(tx.ops, "AppendAudit:operator.login_locked_out")
	if i < 0 || j < i || !tx.committed {
		t.Fatalf("session-holder lockout did not revoke sessions in the locking tx: %v", tx.ops)
	}
	for name, tok := range map[string]string{"guessing session": tokA, "other session": b.Token} {
		if _, err := h.auth.ResolveSession(ctx, tok); !errors.Is(err, ErrOperatorUnauthenticated) {
			t.Errorf("%s survived a session-holder lockout: %v", name, err)
		}
	}
	e := h.store.state.audit[len(h.store.state.audit)-1]
	if e.Action != domain.OperatorAuditLoginLockedOut || e.After["sessions_revoked"] != int64(2) {
		t.Fatalf("lock entry = %+v", e)
	}

	// From outside: the lockout leaves the operator's own session alone.
	h2 := newOpHarness(t)
	_, _, _, tok := h2.enrolled(t)
	for i := 0; i < domain.MaxFailedAttempts; i++ {
		_, _ = login(h2, opEmail, "wrong-password-xyz", "", "")
	}
	if slices.Contains(h2.store.lastTx().ops, "RevokeAllSessions") {
		t.Fatal("an outside lockout revoked sessions — anybody knowing the email could sign the operator out")
	}
	if _, err := h2.auth.ResolveSession(ctx, tok); err != nil {
		t.Fatalf("outside lockout ended the operator's session: %v", err)
	}
}
