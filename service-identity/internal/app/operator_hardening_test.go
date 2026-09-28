package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/service-identity/internal/domain"
	"github.com/vihat/vigov/service-identity/internal/operatorauth"
)

// Security review of TASK-03, findings 1–3.

// lockAll stands in for a PARALLEL attempt that locked the account and committed.
func lockAll(until time.Time) func(st *fakeState) {
	return func(st *fakeState) {
		for id, a := range st.accounts {
			u := until
			a.LockedUntil = &u
			st.accounts[id] = a
		}
	}
}

// Finding 1: every credential flow reads the account with a ROW LOCK before anything else.
func TestOperatorCredentialFlowsLockTheAccountRow(t *testing.T) {
	h := newOpHarness(t)
	code, pw, _, tok := h.enrolled(t)
	ctx := context.Background()

	firstRead := func(name string) {
		t.Helper()
		ops := h.store.lastTx().ops
		if len(ops) == 0 || ops[0] != "ByEmailForUpdate" {
			t.Errorf("%s: first operation is %v, want ByEmailForUpdate", name, ops)
		}
	}
	_, _ = login(h, opEmail, pw, h.currentCode(t, code), "")
	firstRead("login")
	_, _ = h.auth.BeginEnrollment(ctx, OperatorEnrollmentRequest{Email: opEmail, TemporaryPassword: pw, IP: opIP})
	firstRead("begin enrolment")
	_, _ = h.auth.CompleteEnrollment(ctx, OperatorEnrollmentCompletion{Email: opEmail, TemporaryPassword: pw,
		NewPassword: "yet-another-password-x", TOTPCode: "000000", IP: opIP})
	firstRead("complete enrolment")

	lockedBeforeCreds := func(name string) {
		t.Helper()
		ops := h.store.lastTx().ops
		i, j := slices.Index(ops, "ByIDForUpdate"), slices.Index(ops, "Credentials")
		if i < 0 || j < i {
			t.Errorf("%s: credentials read without a row lock first: %v", name, ops)
		}
	}
	_, _ = h.auth.RegenerateRecoveryCodes(ctx, tok, "000000", opIP)
	lockedBeforeCreds("regenerate")
	_ = h.auth.ChangePassword(ctx, OperatorPasswordChange{Token: tok, CurrentPassword: pw,
		NewPassword: "another-password-for-tests", TOTPCode: "000000", IP: opIP})
	lockedBeforeCreds("change password")
}

// Finding 1: a lock committed by a parallel attempt while this one was in flight must win. The
// success path's ResetFailures no longer clears it, and the attempt ROLLS BACK — so the recovery
// code it consumed is still live and no session exists.
func TestOperatorLockDuringAttemptWinsAndRollsBack(t *testing.T) {
	h := newOpHarness(t)
	code, pw, recovery, _ := h.enrolled(t)
	acc := h.store.account(t, code)
	sessionsBefore := len(h.store.state.sessions)

	h.store.before = map[string]func(*fakeState){"ResetFailures": lockAll(h.clock.t.Add(domain.LockoutDuration))}
	rc := string(recovery[0].Lo())
	if _, err := login(h, opEmail, pw, "", rc); !errors.Is(err, ErrOperatorLoginFailed) {
		t.Fatalf("attempt that met a lock: %v", err)
	}
	if h.store.lastTx().committed {
		t.Fatal("the attempt committed — its consumed recovery code and trail entry would stand without a sign-in")
	}
	if len(h.store.state.sessions) != sessionsBefore {
		t.Fatal("a session was opened although the account was locked")
	}
	h.store.before = nil
	// The code was not burnt: once the (hook-free) account is usable again, it still works.
	if _, err := login(h, opEmail, pw, "", rc); err != nil {
		t.Fatalf("recovery code lost to a rolled-back attempt: %v", err)
	}

	// A lock that lands before the TOTP step is claimed: the code is refused outright.
	h.clock.t = h.clock.t.Add(operatorauth.TOTPPeriod)
	h.store.before = map[string]func(*fakeState){"RecordTOTPStep": lockAll(h.clock.t.Add(domain.LockoutDuration))}
	if _, err := login(h, opEmail, pw, h.currentCode(t, code), ""); !errors.Is(err, ErrOperatorLoginFailed) {
		t.Fatalf("TOTP accepted on a locked account: %v", err)
	}
	if got := h.store.account(t, code); got.LockedUntil == nil {
		t.Fatal("the parallel lock was cleared")
	}
	_ = acc
}

// Finding 2: the factor activated is the one the code proved. A pending secret replaced between
// verification and activation activates nothing.
func TestOperatorEnrollmentActivatesOnlyTheProvedSecret(t *testing.T) {
	h := newOpHarness(t)
	code, temp := h.create(t)
	ctx := context.Background()
	if _, err := h.auth.BeginEnrollment(ctx, OperatorEnrollmentRequest{Email: opEmail, TemporaryPassword: temp, IP: opIP}); err != nil {
		t.Fatal(err)
	}
	proved := h.totpSecretOf(t, code, true)
	acc := h.store.account(t, code)
	h.store.before = map[string]func(*fakeState){"ActivateTOTP": func(st *fakeState) {
		c := st.creds[acc.ID]
		c.pendingTOTP = []byte("a-different-sealed-secret-from-a-parallel-begin")
		st.creds[acc.ID] = c
	}}
	_, err := h.auth.CompleteEnrollment(ctx, OperatorEnrollmentCompletion{Email: opEmail, TemporaryPassword: temp,
		NewPassword: opNewPw, TOTPCode: operatorauth.CodeAt(proved, h.clock.t), IP: opIP, UserAgent: opUA})
	if !errors.Is(err, ErrOperatorLoginFailed) {
		t.Fatalf("enrolment with a swapped pending secret: %v", err)
	}
	got := h.store.account(t, code)
	if got.TOTPEnrolled() || len(h.store.state.sessions) != 0 {
		t.Fatalf("an unproved factor was activated: enrolled=%v sessions=%d", got.TOTPEnrolled(), len(h.store.state.sessions))
	}
	for _, e := range h.store.state.audit {
		if e.Action == domain.OperatorAuditTOTPEnrolled {
			t.Fatal("totp_enrolled written for an activation that did not happen")
		}
	}
}

// Finding 3: no rendering path — any fmt verb, slog JSON or text — prints a credential or an email.
func TestOperatorStructsNeverRenderCredentials(t *testing.T) {
	const (
		email = "leaky.operator@example.test"
		pw    = "plain-password-must-not-render"
		pw2   = "second-password-must-not-render"
		totp  = "493817"
		rc    = "QWER-TYUI-OPAS-DFGH"
		tok   = "op1.eyJmYWtlIjoidG9rZW4ifQ.c2lnbmF0dXJl"
		name  = "Leaky Display Name"
	)
	secrets := []string{email, pw, pw2, totp, rc, tok, name, "otpauth://", "MANUALKEY"}
	now := time.Date(2026, 9, 28, 8, 0, 0, 0, time.UTC)
	values := map[string]any{
		"login":      OperatorLoginRequest{Email: email, Password: pw, TOTPCode: totp, RecoveryCode: rc, IP: opIP},
		"issued":     OperatorSessionIssued{Token: tok, ExpiresAt: now, AccountCode: "VH-00001"},
		"change":     OperatorPasswordChange{Token: tok, CurrentPassword: pw, NewPassword: pw2, TOTPCode: totp, IP: opIP},
		"begin":      OperatorEnrollmentRequest{Email: email, TemporaryPassword: pw, IP: opIP},
		"start":      OperatorEnrollmentStart{AccountCode: "VH-00001", ProvisioningURI: secret.Secret("otpauth://totp/x"), ManualEntryKey: secret.Secret("MANUALKEY")},
		"completion": OperatorEnrollmentCompletion{Email: email, TemporaryPassword: pw, NewPassword: pw2, TOTPCode: totp, IP: opIP},
		"result": OperatorEnrollmentResult{OperatorSessionIssued: OperatorSessionIssued{Token: tok, ExpiresAt: now, AccountCode: "VH-00001"},
			RecoveryCodes: []secret.Secret{secret.Secret(rc)}},
		"account": domain.OperatorAccount{ID: "01JFAKE", Code: "VH-00001", Email: email, DisplayName: name},
	}
	for label, v := range values {
		var outs []string
		for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%d", "%x", "%q"} {
			outs = append(outs, fmt.Sprintf(verb, v))
		}
		outs = append(outs, fmt.Sprint(v), fmt.Sprint([]any{v}), fmt.Sprintf("%+v", struct{ Inner any }{v}))
		var jb, tb bytes.Buffer
		slog.New(slog.NewJSONHandler(&jb, nil)).Info("x", "v", v)
		slog.New(slog.NewTextHandler(&tb, nil)).Info("x", "v", v)
		outs = append(outs, jb.String(), tb.String())
		for _, o := range outs {
			for _, s := range secrets {
				if strings.Contains(o, s) || strings.Contains(o, fmt.Sprintf("%x", s)) {
					t.Errorf("%s: a rendering contains a withheld value (%d chars): %s", label, len(s), o)
				}
			}
		}
		if !strings.Contains(fmt.Sprintf("%+v", v), "withheld") {
			t.Errorf("%s: rendering does not mark what it withheld: %+v", label, v)
		}
	}
}
