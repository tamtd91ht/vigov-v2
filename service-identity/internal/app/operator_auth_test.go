package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/password"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/token"
	"github.com/vihat/vigov/service-identity/internal/domain"
	"github.com/vihat/vigov/service-identity/internal/operatorauth"
)

func login(h *opHarness, email, pw, totp, recovery string) (OperatorSessionIssued, error) {
	return h.auth.Login(context.Background(), OperatorLoginRequest{
		Email: email, Password: pw, TOTPCode: totp, RecoveryCode: recovery, IP: opIP, UserAgent: opUA,
	})
}

func TestOperatorLoginHappyPathAndResolve(t *testing.T) {
	h := newOpHarness(t)
	code, pw, _, _ := h.enrolled(t)
	if err := h.admin.Grant(context.Background(), code, "ops.domain.manage", "OPS-2"); err != nil {
		t.Fatal(err)
	}
	res, err := login(h, opEmail, pw, h.currentCode(t, code), "")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if !strings.HasPrefix(res.Token, "op1.") || res.AccountCode != code ||
		!res.ExpiresAt.Equal(h.clock.t.Add(domain.SessionLifetime)) {
		t.Fatalf("result = %+v", res)
	}
	// Session, token and trail in ONE committed transaction, trail after the session.
	tx := h.store.lastTx()
	if !tx.committed {
		t.Fatal("login transaction not committed")
	}
	if i, j := slices.Index(tx.ops, "CreateSession"), slices.Index(tx.ops, "AppendAudit:operator.login_succeeded"); i < 0 || j < i {
		t.Fatalf("session and login_succeeded not in one tx, in order: %v", tx.ops)
	}

	p, err := h.auth.ResolveSession(context.Background(), res.Token)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if p.Code != code || !p.Has(domain.OperatorPermissionDomainManage) || p.Has(domain.OperatorPermissionQRIssue) {
		t.Fatalf("principal = %+v", p)
	}
}

// Every refused cause answers the SAME error, and writes the SAME log line shape.
func TestOperatorLoginUniformFailure(t *testing.T) {
	h := newOpHarness(t)
	code, pw, recovery, _ := h.enrolled(t)
	used := string(recovery[0].Lo())
	if _, err := login(h, opEmail, pw, "", used); err != nil {
		t.Fatalf("recovery login: %v", err)
	}
	replayed := h.currentCode(t, code)
	if _, err := login(h, opEmail, pw, replayed, ""); err != nil {
		t.Fatalf("totp login: %v", err)
	}

	cases := map[string]func() error{
		"unknown email": func() error { _, err := login(h, "nobody@example.test", pw, replayed, ""); return err },
		"wrong password": func() error {
			_, err := login(h, opEmail, "wrong-password-xyz", h.currentCode(t, code), "")
			return err
		},
		"wrong totp":       func() error { _, err := login(h, opEmail, pw, h.wrongCode(t, code), ""); return err },
		"replayed totp":    func() error { _, err := login(h, opEmail, pw, replayed, ""); return err },
		"used recovery":    func() error { _, err := login(h, opEmail, pw, "", used); return err },
		"no second factor": func() error { _, err := login(h, opEmail, pw, "", ""); return err },
		"malformed totp":   func() error { _, err := login(h, opEmail, pw, "12ab56", ""); return err },
		"unknown recovery": func() error { _, err := login(h, opEmail, pw, "", "AAAA-BBBB-CCCC-DDDD"); return err },
		"empty everything": func() error { _, err := login(h, "", "", "", ""); return err },
		"upper-case email": func() error {
			_, err := login(h, strings.ToUpper(opEmail), "wrong-password-xyz", replayed, "")
			return err
		},
	}
	var msg string
	for name, fn := range cases {
		// Reset the counter between cases so no case is refused merely for being locked.
		acc := h.store.account(t, code)
		acc.FailedAttempts, acc.LockedUntil = 0, nil
		h.store.state.accounts[acc.ID] = acc
		h.logs.Reset()

		err := fn()
		if !errors.Is(err, ErrOperatorLoginFailed) {
			t.Errorf("%s: err = %v, want ErrOperatorLoginFailed", name, err)
			continue
		}
		if msg == "" {
			msg = err.Error()
		} else if err.Error() != msg {
			t.Errorf("%s: message %q differs from %q — the difference is the oracle", name, err.Error(), msg)
		}
		line := h.logs.String()
		if !strings.Contains(line, `"msg":"operator credential refused"`) || !strings.Contains(line, `"email_fingerprint"`) {
			t.Errorf("%s: refusal log line has another shape: %s", name, line)
		}
		if strings.Contains(line, `"actor"`) || strings.Contains(line, code) {
			t.Errorf("%s: refusal log names the account — tells a log reader the address exists: %s", name, line)
		}
	}

	// Locked and disabled answer the same, too.
	for i := 0; i < domain.MaxFailedAttempts; i++ {
		_, _ = login(h, opEmail, "wrong-password-xyz", "", "")
	}
	if _, err := login(h, opEmail, pw, h.currentCode(t, code), ""); !errors.Is(err, ErrOperatorLoginFailed) || err.Error() != msg {
		t.Errorf("locked: %v", err)
	}
	h.clock.t = h.clock.t.Add(domain.LockoutDuration + time.Minute)
	if err := h.admin.Disable(context.Background(), code, "left", "OPS-3"); err != nil {
		t.Fatal(err)
	}
	if _, err := login(h, opEmail, pw, h.currentCode(t, code), ""); !errors.Is(err, ErrOperatorLoginFailed) || err.Error() != msg {
		t.Errorf("disabled: %v", err)
	}
}

func TestOperatorLoginRefusesBothFactors(t *testing.T) {
	h := newOpHarness(t)
	before := len(h.store.txs)
	if _, err := login(h, opEmail, "x", "123456", "AAAA-BBBB-CCCC-DDDD"); !errors.Is(err, ErrOperatorSecondFactorAmbiguous) {
		t.Fatalf("err = %v", err)
	}
	if len(h.store.txs) != before {
		t.Fatal("an ambiguous request reached the store — the refusal must not depend on any account")
	}
}

func TestOperatorTOTPReplayRefused(t *testing.T) {
	h := newOpHarness(t)
	code, pw, _, _ := h.enrolled(t)
	c := h.currentCode(t, code)
	if _, err := login(h, opEmail, pw, c, ""); err != nil {
		t.Fatalf("first use: %v", err)
	}
	if _, err := login(h, opEmail, pw, c, ""); !errors.Is(err, ErrOperatorLoginFailed) {
		t.Fatalf("replay within the window accepted: %v", err)
	}
	// Still refused 30 s later, while the code remains inside the ±1 step window.
	h.clock.t = h.clock.t.Add(operatorauth.TOTPPeriod)
	if _, err := login(h, opEmail, pw, c, ""); !errors.Is(err, ErrOperatorLoginFailed) {
		t.Fatalf("replay one step later accepted: %v", err)
	}
	if got := h.store.account(t, code).FailedAttempts; got != 2 {
		t.Fatalf("a replay must count as a failure: failed_attempts = %d", got)
	}
}

func TestOperatorLockoutAfterFiveAndGenericWhileLocked(t *testing.T) {
	h := newOpHarness(t)
	code, pw, _, _ := h.enrolled(t)
	for i := 1; i <= domain.MaxFailedAttempts; i++ {
		if _, err := login(h, opEmail, "wrong-password-xyz", "", ""); !errors.Is(err, ErrOperatorLoginFailed) {
			t.Fatalf("attempt %d: %v", i, err)
		}
		// The count must COMMIT even though the sign-in failed.
		if tx := h.store.lastTx(); !tx.committed {
			t.Fatalf("attempt %d: the failure count rolled back — lockout is decoration", i)
		}
	}
	acc := h.store.account(t, code)
	if !acc.LockedAt(h.clock.t) {
		t.Fatal("not locked after 5 failures")
	}
	// The lock is in the trail, written by the system with a reason, in the tx that locked it.
	tx := h.store.lastTx()
	if !slices.Contains(tx.ops, "AppendAudit:operator.login_locked_out") {
		t.Fatalf("lock not audited in the locking tx: %v", tx.ops)
	}
	last := h.store.state.audit[len(h.store.state.audit)-1]
	if last.Action != domain.OperatorAuditLoginLockedOut || last.Actor != domain.SystemActor ||
		last.Subject != code || last.Reason == "" {
		t.Fatalf("lock entry = %+v", last)
	}
	if !strings.Contains(h.logs.String(), `"event":"operator.login_locked_out"`) {
		t.Error("lock not in the security log")
	}

	// Correct credentials while locked: the same generic refusal, and the lock is not extended.
	until := *acc.LockedUntil
	if _, err := login(h, opEmail, pw, h.currentCode(t, code), ""); !errors.Is(err, ErrOperatorLoginFailed) {
		t.Fatalf("locked account signed in: %v", err)
	}
	if got := h.store.account(t, code); !got.LockedUntil.Equal(until) || got.FailedAttempts != 0 {
		t.Fatalf("a refused attempt during the lock changed it: %+v", got)
	}

	// After 15 minutes the lock has expired by itself.
	h.clock.t = until.Add(time.Second)
	if _, err := login(h, opEmail, pw, h.currentCode(t, code), ""); err != nil {
		t.Fatalf("after the lock: %v", err)
	}
}

func TestOperatorRecoveryCodeSingleUse(t *testing.T) {
	h := newOpHarness(t)
	code, pw, recovery, _ := h.enrolled(t)
	if len(recovery) != domain.RecoveryCodeCount {
		t.Fatalf("%d recovery codes, want %d", len(recovery), domain.RecoveryCodeCount)
	}
	rc := string(recovery[3].Lo())
	if _, err := login(h, opEmail, pw, "", strings.ToLower(rc)); err != nil {
		t.Fatalf("recovery login (typed in lower case): %v", err)
	}
	tx := h.store.lastTx()
	i, j := slices.Index(tx.ops, "AppendAudit:operator.recovery_code_used"), slices.Index(tx.ops, "AppendAudit:operator.login_succeeded")
	if i < 0 || j < i {
		t.Fatalf("recovery_code_used not audited in the sign-in tx: %v", tx.ops)
	}
	if _, err := login(h, opEmail, pw, "", rc); !errors.Is(err, ErrOperatorLoginFailed) {
		t.Fatalf("recovery code reused: %v", err)
	}
	_ = code
}

func TestOperatorEnrollmentHappyPath(t *testing.T) {
	h := newOpHarness(t)
	code, temp := h.create(t)
	ctx := context.Background()

	// Before enrolment, the right temporary password says "enrol"; a wrong one says nothing.
	if _, err := login(h, opEmail, temp, "", ""); !errors.Is(err, ErrOperatorEnrollmentRequired) {
		t.Fatalf("fresh account, right password: %v", err)
	}
	if _, err := login(h, opEmail, "wrong-password-xyz", "", ""); !errors.Is(err, ErrOperatorLoginFailed) {
		t.Fatalf("fresh account, wrong password: %v", err)
	}

	start, err := h.auth.BeginEnrollment(ctx, OperatorEnrollmentRequest{Email: opEmail, TemporaryPassword: temp, IP: opIP})
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	uri := string(start.ProvisioningURI.Lo())
	if !strings.HasPrefix(uri, "otpauth://totp/ViGov:"+code+"?") || strings.Contains(uri, "example.test") ||
		strings.Contains(uri, "%40") {
		t.Fatalf("provisioning URI must be labelled with the code, never the email: %s", uri)
	}
	if start.ProvisioningURI.String() != secret.Che || start.ManualEntryKey.String() != secret.Che {
		t.Fatal("the enrolment secret must not render")
	}
	// Sealed under the account id: it opens with that AAD only.
	acc := h.store.account(t, code)
	if _, err := h.auth.sealer.Open(h.store.state.creds[acc.ID].pendingTOTP, []byte("01JOTHERACCOUNT00000000000")); err == nil {
		t.Fatal("pending secret opens under another account's id")
	}
	// Re-calling replaces the pending secret.
	first := h.totpSecretOf(t, code, true)
	if _, err := h.auth.BeginEnrollment(ctx, OperatorEnrollmentRequest{Email: opEmail, TemporaryPassword: temp, IP: opIP}); err != nil {
		t.Fatal(err)
	}
	pending := h.totpSecretOf(t, code, true)
	if string(first.Lo()) == string(pending.Lo()) {
		t.Fatal("a second BeginEnrollment did not replace the pending secret")
	}

	complete := OperatorEnrollmentCompletion{Email: opEmail, TemporaryPassword: temp, NewPassword: opNewPw,
		TOTPCode: operatorauth.CodeAt(first, h.clock.t), IP: opIP, UserAgent: opUA}
	if _, err := h.auth.CompleteEnrollment(ctx, complete); !errors.Is(err, ErrOperatorLoginFailed) {
		t.Fatalf("code from the replaced secret accepted: %v", err)
	}
	// Password policy is the staff one.
	short := complete
	short.NewPassword = "short"
	if _, err := h.auth.CompleteEnrollment(ctx, short); !errors.Is(err, domain.ErrMatKhauQuaNgan) {
		t.Fatalf("short password: %v", err)
	}
	same := complete
	same.NewPassword = temp
	if _, err := h.auth.CompleteEnrollment(ctx, same); !errors.Is(err, domain.ErrMatKhauMoiTrungCu) {
		t.Fatalf("reused temporary password: %v", err)
	}

	complete.TOTPCode = operatorauth.CodeAt(pending, h.clock.t)
	auditBefore := len(h.store.state.audit)
	res, err := h.auth.CompleteEnrollment(ctx, complete)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if len(res.RecoveryCodes) != domain.RecoveryCodeCount || !strings.HasPrefix(res.Token, "op1.") {
		t.Fatalf("result: %d codes, token %q", len(res.RecoveryCodes), res.Token[:4])
	}
	tx := h.store.lastTx()
	wantOrder := []string{"ActivateTOTP", "ReplaceRecoveryCodes", "AppendAudit:operator.totp_enrolled",
		"AppendAudit:operator.password_changed", "AppendAudit:operator.recovery_codes_regenerated",
		"CreateSession", "AppendAudit:operator.login_succeeded"}
	last := -1
	for _, w := range wantOrder {
		i := slices.Index(tx.ops, w)
		if i <= last {
			t.Fatalf("enrolment tx lacks %q in order: %v", w, tx.ops)
		}
		last = i
	}
	for _, e := range h.store.state.audit[auditBefore:] {
		if e.Actor != code || e.Subject != code {
			t.Errorf("enrolment entry %s actor=%q subject=%q, want the operator code", e.Action, e.Actor, e.Subject)
		}
	}
	got := h.store.account(t, code)
	if !got.TOTPEnrolled() || got.MustChangePassword {
		t.Fatalf("account after enrolment: %+v", got)
	}
	// The enrolment code's step is recorded: it cannot be replayed at login.
	if _, err := login(h, opEmail, opNewPw, complete.TOTPCode, ""); !errors.Is(err, ErrOperatorLoginFailed) {
		t.Fatalf("enrolment code replayed at login: %v", err)
	}
	// The temporary password is dead.
	h.clock.t = h.clock.t.Add(operatorauth.TOTPPeriod)
	if _, err := login(h, opEmail, temp, h.currentCode(t, code), ""); !errors.Is(err, ErrOperatorLoginFailed) {
		t.Fatalf("temporary password still works: %v", err)
	}
	// An enrolled account cannot start another enrolment with its password.
	if _, err := h.auth.BeginEnrollment(ctx, OperatorEnrollmentRequest{Email: opEmail, TemporaryPassword: opNewPw, IP: opIP}); !errors.Is(err, ErrOperatorLoginFailed) {
		t.Fatalf("re-enrolment over an active factor: %v", err)
	}
}

func TestOperatorEnrollmentWrongCodeCountsAndCommitsNothing(t *testing.T) {
	h := newOpHarness(t)
	code, temp := h.create(t)
	ctx := context.Background()
	if _, err := h.auth.BeginEnrollment(ctx, OperatorEnrollmentRequest{Email: opEmail, TemporaryPassword: temp, IP: opIP}); err != nil {
		t.Fatal(err)
	}
	pending := h.totpSecretOf(t, code, true)
	var wrong string
	for i := 0; ; i++ {
		c := strings.Repeat("0", 6-len(itoa(i))) + itoa(i)
		if _, ok := operatorauth.Verify(pending, c, h.clock.t); !ok {
			wrong = c
			break
		}
	}
	_, err := h.auth.CompleteEnrollment(ctx, OperatorEnrollmentCompletion{Email: opEmail, TemporaryPassword: temp,
		NewPassword: opNewPw, TOTPCode: wrong, IP: opIP, UserAgent: opUA})
	if !errors.Is(err, ErrOperatorLoginFailed) {
		t.Fatalf("wrong code: %v", err)
	}
	acc := h.store.account(t, code)
	if acc.TOTPEnrolled() || acc.FailedAttempts != 1 || len(h.store.state.sessions) != 0 {
		t.Fatalf("wrong code: enrolled=%v failed=%d sessions=%d", acc.TOTPEnrolled(), acc.FailedAttempts, len(h.store.state.sessions))
	}
	// Wrong temporary password on BeginEnrollment counts too — no free oracle.
	if _, err := h.auth.BeginEnrollment(ctx, OperatorEnrollmentRequest{Email: opEmail, TemporaryPassword: "wrong-password-xyz", IP: opIP}); !errors.Is(err, ErrOperatorLoginFailed) {
		t.Fatal(err)
	}
	if got := h.store.account(t, code).FailedAttempts; got != 2 {
		t.Fatalf("BeginEnrollment wrong password not counted: %d", got)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for ; i > 0; i /= 10 {
		b = append([]byte{byte('0' + i%10)}, b...)
	}
	return string(b)
}

func TestOperatorResetMFARequiresReonboarding(t *testing.T) {
	h := newOpHarness(t)
	code, pw, recovery, tok := h.enrolled(t)
	ctx := context.Background()

	temp, err := h.admin.ResetMFA(ctx, code, "OPS-9")
	if err != nil {
		t.Fatal(err)
	}
	if len(temp) < 16 || temp.String() != secret.Che {
		t.Fatal("reset must return a fresh, non-rendering temporary password of at least 16 characters")
	}
	// Old session dead, old password dead, old recovery codes dead.
	if _, err := h.auth.ResolveSession(ctx, tok); !errors.Is(err, ErrOperatorUnauthenticated) {
		t.Fatalf("session survived an MFA reset: %v", err)
	}
	if _, err := login(h, opEmail, pw, "", string(recovery[0].Lo())); !errors.Is(err, ErrOperatorLoginFailed) {
		t.Fatalf("old password + recovery code after reset: %v", err)
	}
	// The stolen-password case: the OLD password cannot enrol a new authenticator.
	if _, err := h.auth.BeginEnrollment(ctx, OperatorEnrollmentRequest{Email: opEmail, TemporaryPassword: pw, IP: opIP}); !errors.Is(err, ErrOperatorLoginFailed) {
		t.Fatalf("old password began enrolment after reset: %v", err)
	}
	// The new temporary password leads to enrolment, and only there.
	if _, err := login(h, opEmail, string(temp.Lo()), "", ""); !errors.Is(err, ErrOperatorEnrollmentRequired) {
		t.Fatalf("new temporary password: %v", err)
	}
	acc := h.store.account(t, code)
	if acc.TOTPEnrolled() || !acc.MustChangePassword {
		t.Fatalf("after reset: %+v", acc)
	}
	acts := h.store.auditActions()
	if !slices.Contains(acts, domain.OperatorAuditMFAReset) {
		t.Fatalf("reset not audited: %v", acts)
	}
	for _, e := range h.store.state.audit {
		if e.Action == domain.OperatorAuditMFAReset && (e.Actor != domain.SystemActor || e.Reason != "ticket:OPS-9") {
			t.Fatalf("reset entry = %+v", e)
		}
	}
}

func TestOperatorResolveSessionRefusesStaffToken(t *testing.T) {
	h := newOpHarness(t)
	h.enrolled(t)
	ctx := context.Background()

	staff, err := token.NewSigner([]secret.Secret{secret.Secret("staff-key-FAKE-NOT-A-REAL-KEY-for-app-tests")})
	if err != nil {
		t.Fatal(err)
	}
	// A staff v1 token for a session id that IS live in the operator registry — only the realm
	// separation stands between it and a principal.
	var liveSid string
	for sid, s := range h.store.state.sessions {
		if !s.revoked {
			liveSid = sid
		}
	}
	v1, err := staff.Ky(token.Claims{TenantID: tenant.ID("01JTENANTFAKE0000000000000"), Sid: liveSid,
		ExpiresAt: h.clock.t.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	for name, tok := range map[string]string{
		"staff v1":           v1,
		"staff v1 relabeled": "op1." + strings.TrimPrefix(v1, strings.SplitN(v1, ".", 2)[0]+"."),
		"empty":              "",
		"garbage":            "op1.x.y",
	} {
		if _, err := h.auth.ResolveSession(ctx, tok); !errors.Is(err, ErrOperatorUnauthenticated) {
			t.Errorf("%s: err = %v, want ErrOperatorUnauthenticated", name, err)
		}
	}
}

func TestOperatorResolveSessionExpiryAndDisable(t *testing.T) {
	h := newOpHarness(t)
	code, _, _, tok := h.enrolled(t)
	ctx := context.Background()
	if _, err := h.auth.ResolveSession(ctx, tok); err != nil {
		t.Fatal(err)
	}
	if err := h.admin.Disable(ctx, code, "suspended", "OPS-4"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.auth.ResolveSession(ctx, tok); !errors.Is(err, ErrOperatorUnauthenticated) {
		t.Fatalf("disabled account's session resolved: %v", err)
	}

	h2 := newOpHarness(t)
	_, _, _, tok2 := h2.enrolled(t)
	h2.clock.t = h2.clock.t.Add(domain.SessionLifetime)
	if _, err := h2.auth.ResolveSession(ctx, tok2); !errors.Is(err, ErrOperatorUnauthenticated) {
		t.Fatalf("session past 8h resolved: %v", err)
	}
}

func TestOperatorLogoutRevokesOwnSession(t *testing.T) {
	h := newOpHarness(t)
	code, _, _, tok := h.enrolled(t)
	ctx := context.Background()
	if err := h.auth.Logout(ctx, tok, opIP); err != nil {
		t.Fatal(err)
	}
	tx := h.store.lastTx()
	if i, j := slices.Index(tx.ops, "RevokeSession"), slices.Index(tx.ops, "AppendAudit:operator.logged_out"); i < 0 || j < i {
		t.Fatalf("logout ops: %v", tx.ops)
	}
	last := h.store.state.audit[len(h.store.state.audit)-1]
	if last.Actor != code || last.Subject != code {
		t.Fatalf("logout entry = %+v", last)
	}
	if _, err := h.auth.ResolveSession(ctx, tok); !errors.Is(err, ErrOperatorUnauthenticated) {
		t.Fatal("session alive after logout")
	}
	if err := h.auth.Logout(ctx, tok, opIP); !errors.Is(err, ErrOperatorUnauthenticated) {
		t.Fatalf("second logout: %v", err)
	}
}

func TestOperatorChangePassword(t *testing.T) {
	h := newOpHarness(t)
	code, pw, _, tok := h.enrolled(t)
	ctx := context.Background()
	const next = "another-password-for-tests"

	err := h.auth.ChangePassword(ctx, OperatorPasswordChange{Token: tok, CurrentPassword: "wrong-password-xyz",
		NewPassword: next, TOTPCode: h.currentCode(t, code), IP: opIP})
	if !errors.Is(err, ErrOperatorLoginFailed) {
		t.Fatalf("wrong current password: %v", err)
	}
	if h.store.account(t, code).FailedAttempts != 1 {
		t.Fatal("wrong current password not counted — a stolen session buys unlimited guesses")
	}
	err = h.auth.ChangePassword(ctx, OperatorPasswordChange{Token: tok, CurrentPassword: pw,
		NewPassword: next, TOTPCode: h.wrongCode(t, code), IP: opIP})
	if !errors.Is(err, ErrOperatorLoginFailed) {
		t.Fatalf("wrong totp: %v", err)
	}

	if err := h.auth.ChangePassword(ctx, OperatorPasswordChange{Token: tok, CurrentPassword: pw,
		NewPassword: next, TOTPCode: h.currentCode(t, code), IP: opIP}); err != nil {
		t.Fatalf("change: %v", err)
	}
	tx := h.store.lastTx()
	if i, j := slices.Index(tx.ops, "SetPassword"), slices.Index(tx.ops, "AppendAudit:operator.password_changed"); i < 0 || j < i {
		t.Fatalf("change ops: %v", tx.ops)
	}
	if _, err := h.auth.ResolveSession(ctx, tok); !errors.Is(err, ErrOperatorUnauthenticated) {
		t.Fatal("the current session survived a password change")
	}
	h.clock.t = h.clock.t.Add(operatorauth.TOTPPeriod)
	if _, err := login(h, opEmail, next, h.currentCode(t, code), ""); err != nil {
		t.Fatalf("new password: %v", err)
	}
}

func TestOperatorRegenerateRecoveryCodes(t *testing.T) {
	h := newOpHarness(t)
	code, pw, old, tok := h.enrolled(t)
	ctx := context.Background()
	codes, err := h.auth.RegenerateRecoveryCodes(ctx, tok, h.currentCode(t, code), opIP)
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != domain.RecoveryCodeCount {
		t.Fatalf("%d codes", len(codes))
	}
	if !slices.Contains(h.store.lastTx().ops, "AppendAudit:operator.recovery_codes_regenerated") {
		t.Fatal("regeneration not audited in its tx")
	}
	h.clock.t = h.clock.t.Add(operatorauth.TOTPPeriod)
	if _, err := login(h, opEmail, pw, "", string(old[0].Lo())); !errors.Is(err, ErrOperatorLoginFailed) {
		t.Fatal("a code of the voided batch still works")
	}
	if _, err := login(h, opEmail, pw, "", string(codes[0].Lo())); err != nil {
		t.Fatalf("new code: %v", err)
	}
	// Without a valid TOTP code: refused.
	if _, err := h.auth.RegenerateRecoveryCodes(ctx, tok, "000000", opIP); err == nil {
		t.Fatal("regenerated without a valid code")
	}
}

// An audit write that fails takes the business change down with it (rule 6, invariant 3).
func TestOperatorAuditFailureRollsBackTheChange(t *testing.T) {
	h := newOpHarness(t)
	code, _, _, tok := h.enrolled(t)
	ctx := context.Background()
	h.store.failAudit = domain.OperatorAuditLoggedOut
	if err := h.auth.Logout(ctx, tok, opIP); err == nil {
		t.Fatal("logout succeeded although its audit entry failed")
	}
	if h.store.lastTx().committed {
		t.Fatal("tx committed without its audit entry")
	}
	if _, err := h.auth.ResolveSession(ctx, tok); err != nil {
		t.Fatalf("session was revoked although the tx rolled back: %v", err)
	}

	h.store.failAudit = domain.OperatorAuditPermissionGranted
	if err := h.admin.Grant(ctx, code, "ops.qr.issue", "OPS-5"); err == nil {
		t.Fatal("grant succeeded although its audit entry failed")
	}
	if len(activeKeys(h.store.state, h.store.account(t, code).ID)) != 0 {
		t.Fatal("grant persisted without its audit entry")
	}
}

func TestOperatorRealmNotConfiguredFailsClosed(t *testing.T) {
	store := newFakeOperatorStore()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()
	signer, _ := operatorauth.NewTokenSigner([]secret.Secret{opSignKey})
	sealer, _ := operatorauth.NewSealer([]secret.Secret{opSealKey})
	for name, a := range map[string]*OperatorAuth{
		"neither":   NewOperatorAuth(store, nil, nil, nil, log),
		"no sealer": NewOperatorAuth(store, signer, nil, nil, log),
		"no signer": NewOperatorAuth(store, nil, sealer, nil, log),
	} {
		errs := []error{}
		_, err := a.Login(ctx, OperatorLoginRequest{Email: opEmail, Password: "x"})
		errs = append(errs, err)
		_, err = a.BeginEnrollment(ctx, OperatorEnrollmentRequest{Email: opEmail})
		errs = append(errs, err)
		_, err = a.CompleteEnrollment(ctx, OperatorEnrollmentCompletion{Email: opEmail})
		errs = append(errs, err)
		_, err = a.ResolveSession(ctx, "op1.x.y")
		errs = append(errs, err)
		errs = append(errs, a.Logout(ctx, "op1.x.y", opIP))
		errs = append(errs, a.ChangePassword(ctx, OperatorPasswordChange{}))
		_, err = a.RegenerateRecoveryCodes(ctx, "op1.x.y", "000000", opIP)
		errs = append(errs, err)
		for i, err := range errs {
			if !errors.Is(err, ErrOperatorRealmNotConfigured) {
				t.Errorf("%s: flow %d: err = %v", name, i, err)
			}
		}
	}
	if len(store.txs) != 0 {
		t.Fatal("an unconfigured realm opened a transaction")
	}
	// The CLI's acts do not need the sign-in keys.
	if _, err := NewOperatorAdmin(store, nil, log).CreateOperator(ctx, opEmail, opName, "OPS-1"); err != nil {
		t.Fatalf("admin without keys: %v", err)
	}
}

// Nothing secret reaches the security log across every flow: no email, no password, no temporary
// password, no TOTP code or secret, no recovery code, no token.
func TestOperatorLogsCarryNoSecrets(t *testing.T) {
	h := newOpHarness(t)
	ctx := context.Background()
	created, err := h.admin.CreateOperator(ctx, opEmail, opName, "OPS-1")
	if err != nil {
		t.Fatal(err)
	}
	temp := string(created.TemporaryPassword.Lo())
	secrets := []string{opEmail, strings.ToUpper(opEmail), opName, temp, opNewPw, "wrong-password-xyz"}

	_, _ = login(h, opEmail, temp, "", "")
	_, _ = login(h, strings.ToUpper(opEmail), "wrong-password-xyz", "", "")
	start, err := h.auth.BeginEnrollment(ctx, OperatorEnrollmentRequest{Email: opEmail, TemporaryPassword: temp, IP: opIP})
	if err != nil {
		t.Fatal(err)
	}
	pending := h.totpSecretOf(t, created.Code, true)
	enrolCode := operatorauth.CodeAt(pending, h.clock.t)
	secrets = append(secrets, string(start.ProvisioningURI.Lo()), string(start.ManualEntryKey.Lo()))
	res, err := h.auth.CompleteEnrollment(ctx, OperatorEnrollmentCompletion{Email: opEmail, TemporaryPassword: temp,
		NewPassword: opNewPw, TOTPCode: enrolCode, IP: opIP, UserAgent: opUA})
	if err != nil {
		t.Fatal(err)
	}
	secrets = append(secrets, res.Token, `"`+enrolCode+`"`)
	for _, c := range res.RecoveryCodes {
		secrets = append(secrets, string(c.Lo()))
	}
	h.clock.t = h.clock.t.Add(operatorauth.TOTPPeriod)
	loginCode := h.currentCode(t, created.Code)
	secrets = append(secrets, `"`+loginCode+`"`)
	s2, _ := login(h, opEmail, opNewPw, loginCode, "")
	_, _ = login(h, opEmail, opNewPw, "", string(res.RecoveryCodes[0].Lo()))
	secrets = append(secrets, s2.Token)
	for i := 0; i < domain.MaxFailedAttempts; i++ {
		_, _ = login(h, opEmail, "wrong-password-xyz", "", "")
	}
	_ = h.auth.Logout(ctx, s2.Token, opIP)
	reset, _ := h.admin.ResetMFA(ctx, created.Code, "OPS-2")
	secrets = append(secrets, string(reset.Lo()))
	_, _ = h.admin.List(ctx)
	for _, sid := range func() []string {
		var out []string
		for sid := range h.store.state.sessions {
			out = append(out, sid)
		}
		return out
	}() {
		secrets = append(secrets, sid)
	}

	out := h.logs.String()
	if !strings.Contains(out, "operator.login") || !strings.Contains(out, "operator.mfa_reset") {
		t.Fatalf("expected security events missing; log:\n%s", out)
	}
	for _, s := range secrets {
		if s != "" && strings.Contains(out, s) {
			t.Errorf("log contains a secret or personal value (%d chars)", len(s))
		}
	}
}

func TestOperatorLoginRehashesWeakHash(t *testing.T) {
	h := newOpHarness(t)
	code, pw, _, _ := h.enrolled(t)
	acc := h.store.account(t, code)
	// A hash with a weaker memory parameter than current: re-hash of the same password.
	weak := strings.Replace(h.store.state.creds[acc.ID].passwordHash, "m=19456", "m=8192", 1)
	c := h.store.state.creds[acc.ID]
	c.passwordHash = weak
	h.store.state.creds[acc.ID] = c
	// The weakened string no longer verifies (parameters are part of the hash), so this test can
	// only assert the upgrade path is NOT taken on failure; the positive path needs a real weak
	// hash, which core/password cannot mint. Assert the negative, which is the dangerous one.
	if _, err := login(h, opEmail, pw, h.currentCode(t, code), ""); !errors.Is(err, ErrOperatorLoginFailed) {
		t.Fatalf("login against an unverifiable hash: %v", err)
	}
	if h.store.state.creds[acc.ID].passwordHash != weak {
		t.Fatal("hash rewritten after a FAILED sign-in")
	}

	// The upgrade itself: conditional on the old hash, never revoking sessions.
	c.passwordHash = "old-hash-placeholder"
	h.store.state.creds[acc.ID] = c
	revoked := func() (n int) {
		for _, s := range h.store.state.sessions {
			if s.revoked {
				n++
			}
		}
		return n
	}
	revokedBefore := revoked()
	h.auth.upgradePasswordHash(context.Background(), acc.ID, code, "a-stale-hash", pw)
	if h.store.state.creds[acc.ID].passwordHash != "old-hash-placeholder" {
		t.Fatal("upgrade overwrote a hash that changed in between")
	}
	h.auth.upgradePasswordHash(context.Background(), acc.ID, code, "old-hash-placeholder", pw)
	if err := password.KiemTra(pw, h.store.state.creds[acc.ID].passwordHash); err != nil {
		t.Fatalf("upgraded hash does not verify the same password: %v", err)
	}
	if revoked() != revokedBefore {
		t.Fatal("a rehash revoked a session — it would sign out the session the sign-in just opened")
	}
}
