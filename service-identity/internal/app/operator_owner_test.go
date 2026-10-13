package app

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/password"
	"github.com/vihat/vigov/service-identity/internal/domain"
	"github.com/vihat/vigov/service-identity/internal/operatorauth"
)

// Fake material: example.test addresses, a marker temporary password that must never reach a log.
const (
	ownerTwoEmail = "operator.two@example.test"
	ownerTemp     = "OWNER-TEMP-MARKER-not-real"
	ownerIP       = "198.51.100.4"
)

type ownerFixture struct {
	h         *opHarness
	owner     *OwnerAdmin
	ownerCode string
	token     string
	actor     MiniAppOperator
}

// newOwnerFixture: an enrolled operator (opEmail) made THE owner through the real SetOwner.
func newOwnerFixture(t *testing.T) *ownerFixture {
	t.Helper()
	h := newOpHarness(t)
	code, _, _, tok := h.enrolled(t)
	if _, err := h.admin.SetOwner(context.Background(), opEmail, "OPS-OWNER"); err != nil {
		t.Fatalf("set owner: %v", err)
	}
	log := slog.New(slog.NewJSONHandler(h.logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return &ownerFixture{h: h, owner: NewOwnerAdmin(h.store, h.clock.now, log), ownerCode: code, token: tok,
		actor: MiniAppOperator{Code: code, IP: ownerIP}}
}

// second creates another operator through the CLI path and gives it a live session.
func (f *ownerFixture) second(t *testing.T) domain.OperatorAccount {
	t.Helper()
	c, err := f.h.admin.CreateOperator(context.Background(), ownerTwoEmail, "Operator Two", "OPS-2")
	if err != nil {
		t.Fatal(err)
	}
	acc := f.h.store.account(t, c.Code)
	now := f.h.clock.t
	f.h.store.state.sessions["sid-two"] = fakeSession{accountID: acc.ID, created: now,
		expires: now.Add(domain.SessionLifetime), lastSeen: now}
	return acc
}

func (f *ownerFixture) noSecretsLogged(t *testing.T) {
	t.Helper()
	for _, s := range []string{ownerTemp, ownerTwoEmail, opEmail} {
		if strings.Contains(f.h.logs.String(), s) {
			t.Fatalf("log carries %q: %s", s, f.h.logs.String())
		}
	}
}

func TestResolveSessionCarriesIsOwner(t *testing.T) {
	h := newOpHarness(t)
	_, _, _, tok := h.enrolled(t)
	p, err := h.auth.ResolveSession(context.Background(), tok)
	if err != nil || p.IsOwner {
		t.Fatalf("before set-owner: IsOwner %v err %v", p.IsOwner, err)
	}
	if _, err := h.admin.SetOwner(context.Background(), opEmail, "OPS-1"); err != nil {
		t.Fatal(err)
	}
	p, err = h.auth.ResolveSession(context.Background(), tok)
	if err != nil || !p.IsOwner {
		t.Fatalf("after set-owner: IsOwner %v err %v — the flag must be read live, no re-login", p.IsOwner, err)
	}
}

func TestSetOwnerSwapsInOneTransactionAndTrailsBoth(t *testing.T) {
	f := newOwnerFixture(t)
	two := f.second(t)
	auditBefore := len(f.h.store.state.audit)

	code, err := f.h.admin.SetOwner(context.Background(), " "+strings.ToUpper(ownerTwoEmail)+" ", "OPS-9")
	if err != nil || code != two.Code {
		t.Fatalf("set-owner: %q %v", code, err)
	}
	if f.h.store.account(t, f.ownerCode).IsOwner || !f.h.store.account(t, two.Code).IsOwner {
		t.Fatal("the flag did not move")
	}
	entries := f.h.store.state.audit[auditBefore:]
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2 (removed + designated)", len(entries))
	}
	if entries[0].Action != domain.OperatorAuditOwnerRemoved || entries[0].Subject != f.ownerCode ||
		entries[1].Action != domain.OperatorAuditOwnerDesignated || entries[1].Subject != two.Code ||
		entries[1].Before["previous_owner"] != f.ownerCode {
		t.Fatalf("entries = %+v", entries)
	}
	for _, e := range entries {
		if e.Actor != domain.SystemActor || e.Reason != "ticket:OPS-9" {
			t.Fatalf("entry actor/reason = %q/%q", e.Actor, e.Reason)
		}
	}
	ops := f.h.store.lastTx().ops
	if !f.h.store.lastTx().committed || ops[len(ops)-1] != "AppendAudit:"+string(domain.OperatorAuditOwnerDesignated) {
		t.Fatalf("one transaction, audit last: %v", ops)
	}
}

func TestSetOwnerRefusals(t *testing.T) {
	f := newOwnerFixture(t)
	two := f.second(t)
	ctx := context.Background()
	if err := f.h.admin.Disable(ctx, two.Code, "left", "OPS-3"); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		email, ticket string
		want          error
	}{
		{ownerTwoEmail, "OPS-4", ErrOwnerMustBeEnabled},
		{opEmail, "OPS-5", ErrOperatorAlreadyOwner},
		{"nobody@example.test", "OPS-6", ErrOperatorEmailNotFound},
		{opEmail, "", ErrTicketRequired},
		{"not-an-email", "OPS-7", ErrInvalidOperatorEmail},
	}
	for _, c := range cases {
		before := len(f.h.store.state.audit)
		if _, err := f.h.admin.SetOwner(ctx, c.email, c.ticket); !errors.Is(err, c.want) {
			t.Errorf("%s: err %v, want %v", c.email, err, c.want)
		}
		if len(f.h.store.state.audit) != before {
			t.Errorf("%s: a refused set-owner wrote an entry", c.email)
		}
	}
	if !f.h.store.account(t, f.ownerCode).IsOwner {
		t.Fatal("a refused set-owner moved the flag")
	}
	if err := f.h.admin.Disable(ctx, f.ownerCode, "x", "OPS-8"); !errors.Is(err, ErrOwnerCannotBeDisabled) {
		t.Fatalf("disabling the owner: %v", err)
	}
}

func TestOperatorListShowsOwner(t *testing.T) {
	f := newOwnerFixture(t)
	f.second(t)
	rows, err := f.h.admin.List(context.Background())
	if err != nil || len(rows) != 2 || !rows[0].Owner || rows[1].Owner {
		t.Fatalf("list = %+v, %v", rows, err)
	}
}

func TestOwnerCreateAccount(t *testing.T) {
	f := newOwnerFixture(t)
	ctx := context.Background()
	before := len(f.h.store.state.audit)
	keys := []domain.OperatorPermission{domain.OperatorPermissionDomainManage, domain.OperatorPermissionQRIssue}
	v, err := f.owner.CreateAccount(ctx, f.actor, NewOperatorByOwner{Email: ownerTwoEmail, DisplayName: "Two",
		TemporaryPassword: ownerTemp, Permissions: keys})
	if err != nil {
		t.Fatal(err)
	}
	acc := f.h.store.account(t, v.Account.Code)
	if !acc.MustChangePassword || acc.TOTPEnrolled() || acc.CreatedBy != f.ownerCode || acc.IsOwner ||
		acc.TemporaryPasswordExpiresAt == nil || !acc.TemporaryPasswordExpiresAt.Equal(f.h.clock.t.Add(domain.TemporaryPasswordLifetime)) {
		t.Fatalf("account = %v created_by %q", acc, acc.CreatedBy)
	}
	if password.KiemTra(ownerTemp, f.h.store.state.creds[acc.ID].passwordHash) != nil {
		t.Fatal("the stored hash is not the owner's temporary password")
	}
	if len(v.Permissions) != 2 {
		t.Fatalf("keys = %v", v.Permissions)
	}
	entries := f.h.store.state.audit[before:]
	want := []domain.OperatorAuditAction{domain.OperatorAuditAccountCreated, domain.OperatorAuditPermissionGranted,
		domain.OperatorAuditPermissionGranted}
	if len(entries) != len(want) {
		t.Fatalf("entries = %+v", entries)
	}
	for i, e := range entries {
		if e.Action != want[i] || e.Actor != f.ownerCode || e.ActorIP != ownerIP || e.Subject != acc.Code {
			t.Fatalf("entry %d = %+v", i, e)
		}
	}
	if !f.h.store.lastTx().committed {
		t.Fatal("not committed")
	}

	// Email taken (case-folded): nothing written.
	before = len(f.h.store.state.audit)
	if _, err := f.owner.CreateAccount(ctx, f.actor, NewOperatorByOwner{Email: strings.ToUpper(ownerTwoEmail),
		DisplayName: "Dup", TemporaryPassword: ownerTemp}); !errors.Is(err, ErrOperatorEmailTaken) {
		t.Fatalf("duplicate email: %v", err)
	}
	// Policy refusal: decided before any transaction.
	txs := len(f.h.store.txs)
	if _, err := f.owner.CreateAccount(ctx, f.actor, NewOperatorByOwner{Email: "three@example.test",
		DisplayName: "Three", TemporaryPassword: "short"}); !errors.Is(err, domain.ErrMatKhauQuaNgan) {
		t.Fatalf("short password: %v", err)
	}
	if len(f.h.store.txs) != txs || len(f.h.store.state.audit) != before {
		t.Fatal("a refused create opened a transaction or wrote an entry")
	}
	// No VH- code: refused, never a fallback to the id.
	if _, err := f.owner.CreateAccount(ctx, MiniAppOperator{}, NewOperatorByOwner{}); !errors.Is(err, ErrMiniAppNoOperatorCode) {
		t.Fatalf("empty actor: %v", err)
	}
	f.noSecretsLogged(t)
}

func TestOwnerSetPermissionsFullSet(t *testing.T) {
	f := newOwnerFixture(t)
	ctx := context.Background()
	two := f.second(t)
	if err := f.h.admin.Grant(ctx, two.Code, string(domain.OperatorPermissionQRIssue), "OPS-G"); err != nil {
		t.Fatal(err)
	}
	f.h.store.state.sessions["sid-two"] = fakeSession{accountID: two.ID, created: f.h.clock.t,
		expires: f.h.clock.t.Add(domain.SessionLifetime), lastSeen: f.h.clock.t}
	before := len(f.h.store.state.audit)

	v, err := f.owner.SetPermissions(ctx, f.actor, two.Code, []domain.OperatorPermission{domain.OperatorPermissionDomainManage}, " moved team ")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Permissions) != 1 || v.Permissions[0] != domain.OperatorPermissionDomainManage {
		t.Fatalf("keys after = %v", v.Permissions)
	}
	if !f.h.store.state.sessions["sid-two"].revoked {
		t.Fatal("a key removed must revoke the target's sessions")
	}
	entries := f.h.store.state.audit[before:]
	if len(entries) != 2 || entries[0].Action != domain.OperatorAuditPermissionRevoked ||
		entries[1].Action != domain.OperatorAuditPermissionGranted {
		t.Fatalf("entries = %+v", entries)
	}
	for _, e := range entries {
		if e.Actor != f.ownerCode || e.Reason != "moved team" || e.Subject != two.Code {
			t.Fatalf("entry = %+v", e)
		}
	}

	// The same set again: no change, no entry.
	before = len(f.h.store.state.audit)
	if _, err := f.owner.SetPermissions(ctx, f.actor, two.Code, []domain.OperatorPermission{domain.OperatorPermissionDomainManage}, "again"); err != nil {
		t.Fatal(err)
	}
	if len(f.h.store.state.audit) != before {
		t.Fatal("an unchanged set wrote an entry")
	}
	// A key listed twice is one key — one grant, one entry, no ErrAlreadyGranted.
	before = len(f.h.store.state.audit)
	dup := []domain.OperatorPermission{domain.OperatorPermissionQRIssue, domain.OperatorPermissionQRIssue,
		domain.OperatorPermissionDomainManage}
	if v, err := f.owner.SetPermissions(ctx, f.actor, two.Code, dup, "dup"); err != nil || len(v.Permissions) != 2 {
		t.Fatalf("duplicated key: %v %v", v.Permissions, err)
	}
	if n := len(f.h.store.state.audit) - before; n != 1 {
		t.Fatalf("duplicated key wrote %d entries, want 1", n)
	}
	// Empty set revokes everything.
	if v, err := f.owner.SetPermissions(ctx, f.actor, two.Code, nil, "all off"); err != nil || len(v.Permissions) != 0 {
		t.Fatalf("empty set: %v %v", v.Permissions, err)
	}

	if _, err := f.owner.SetPermissions(ctx, f.actor, f.ownerCode, nil, "x"); !errors.Is(err, ErrOwnerTargetIsOwner) {
		t.Fatalf("owner target: %v", err)
	}
	if _, err := f.owner.SetPermissions(ctx, f.actor, "VH-99999", nil, "x"); !errors.Is(err, ErrOperatorNotFound) {
		t.Fatalf("unknown target: %v", err)
	}
	if _, err := f.owner.SetPermissions(ctx, f.actor, two.Code, nil, "  "); !errors.Is(err, ErrOwnerReasonInvalid) {
		t.Fatalf("blank reason: %v", err)
	}
}

func TestOwnerResetOperatorPassword(t *testing.T) {
	f := newOwnerFixture(t)
	ctx := context.Background()
	two := f.second(t)
	// A locked account with an active second factor.
	until := f.h.clock.t.Add(time.Hour)
	enrolled := f.h.clock.t
	a := f.h.store.state.accounts[two.ID]
	a.LockedUntil, a.FailedAttempts, a.TOTPEnrolledAt = &until, 3, &enrolled
	f.h.store.state.accounts[two.ID] = a
	c := f.h.store.state.creds[two.ID]
	c.totpSealed = []byte("sealed-totp-kept")
	f.h.store.state.creds[two.ID] = c
	oldHash := c.passwordHash
	before := len(f.h.store.state.audit)

	v, err := f.owner.ResetPassword(ctx, f.actor, two.Code, ownerTemp, "forgot it")
	if err != nil {
		t.Fatal(err)
	}
	got := f.h.store.state.accounts[two.ID]
	creds := f.h.store.state.creds[two.ID]
	switch {
	case !got.MustChangePassword || !v.Account.MustChangePassword:
		t.Fatal("must_change_password not set")
	case got.TemporaryPasswordExpiresAt == nil || !got.TemporaryPasswordExpiresAt.Equal(f.h.clock.t.Add(domain.TemporaryPasswordLifetime)):
		t.Fatal("temporary expiry not 24 h")
	case got.LockedUntil != nil || got.FailedAttempts != 0:
		t.Fatal("lockout not cleared")
	case !got.TOTPEnrolled() || string(creds.totpSealed) != "sealed-totp-kept":
		t.Fatal("the second factor must be KEPT (ADR 0048 amendment #6)")
	case password.KiemTra(ownerTemp, creds.passwordHash) != nil || creds.passwordHash == oldHash:
		t.Fatal("password not replaced by the owner's value")
	case !f.h.store.state.sessions["sid-two"].revoked:
		t.Fatal("sessions not revoked")
	}
	entries := f.h.store.state.audit[before:]
	if len(entries) != 1 || entries[0].Action != domain.OperatorAuditPasswordReset || entries[0].Actor != f.ownerCode ||
		entries[0].ActorIP != ownerIP || entries[0].Reason != "forgot it" || entries[0].Before["locked"] != true {
		t.Fatalf("entries = %+v", entries)
	}

	if _, err := f.owner.ResetPassword(ctx, f.actor, f.ownerCode, ownerTemp, "self"); !errors.Is(err, ErrOwnerTargetIsSelf) {
		t.Fatalf("self reset: %v", err)
	}
	if _, err := f.owner.ResetPassword(ctx, f.actor, "VH-99999", ownerTemp, "x"); !errors.Is(err, ErrOperatorNotFound) {
		t.Fatalf("unknown: %v", err)
	}

	// The trail entry fails: the reset rolls back with it.
	f.h.store.failAudit = domain.OperatorAuditPasswordReset
	hashBefore := f.h.store.state.creds[two.ID].passwordHash
	if _, err := f.owner.ResetPassword(ctx, f.actor, two.Code, ownerTemp+"-2", "again"); err == nil {
		t.Fatal("a failed entry must fail the act")
	}
	if f.h.store.state.creds[two.ID].passwordHash != hashBefore || f.h.store.lastTx().committed {
		t.Fatal("the reset committed without its entry")
	}
	f.noSecretsLogged(t)
}

// enrolSecond creates a second operator and completes its enrolment, so it holds an active TOTP.
// Returns its code and its own password; the clock is moved one TOTP step on.
func (f *ownerFixture) enrolSecond(t *testing.T) (string, string) {
	t.Helper()
	ctx := context.Background()
	c, err := f.h.admin.CreateOperator(ctx, ownerTwoEmail, "Operator Two", "OPS-2")
	if err != nil {
		t.Fatal(err)
	}
	temp := string(c.TemporaryPassword.Lo())
	if _, err := f.h.auth.BeginEnrollment(ctx, OperatorEnrollmentRequest{Email: ownerTwoEmail, TemporaryPassword: temp, IP: opIP}); err != nil {
		t.Fatalf("begin: %v", err)
	}
	const own = "operator-two-own-password"
	s := f.h.totpSecretOf(t, c.Code, true)
	if _, err := f.h.auth.CompleteEnrollment(ctx, OperatorEnrollmentCompletion{Email: ownerTwoEmail, TemporaryPassword: temp,
		NewPassword: own, TOTPCode: operatorauth.CodeAt(s, f.h.clock.t), IP: opIP, UserAgent: opUA}); err != nil {
		t.Fatalf("complete: %v", err)
	}
	f.h.clock.t = f.h.clock.t.Add(operatorauth.TOTPPeriod)
	return c.Code, own
}

// Owner decision 13/10/2026 (ADR 0048 §13/10 #6): reset keeps the TOTP; the person signs in with
// the temporary password + their own TOTP, gets a RESTRICTED principal, changes the password, and
// the next session is unrestricted.
func TestResetKeepsTOTPAndRestrictsUntilPasswordChanged(t *testing.T) {
	f := newOwnerFixture(t)
	ctx := context.Background()
	two, own := f.enrolSecond(t)
	if _, err := f.owner.ResetPassword(ctx, f.actor, two, ownerTemp, "forgot"); err != nil {
		t.Fatal(err)
	}
	if _, err := login(f.h, ownerTwoEmail, own, f.h.currentCode(t, two), ""); !errors.Is(err, ErrOperatorLoginFailed) {
		t.Fatalf("the old password must stop working: %v", err)
	}
	f.h.clock.t = f.h.clock.t.Add(operatorauth.TOTPPeriod)

	sess, err := login(f.h, ownerTwoEmail, ownerTemp, f.h.currentCode(t, two), "")
	if err != nil {
		t.Fatalf("temporary password + own TOTP must sign in (not ENROLLMENT_REQUIRED): %v", err)
	}
	p, err := f.h.auth.ResolveSession(ctx, sess.Token)
	if err != nil || !p.MustChangePassword {
		t.Fatalf("restricted principal expected: %+v %v", p, err)
	}

	f.h.clock.t = f.h.clock.t.Add(operatorauth.TOTPPeriod)
	const fresh = "operator-two-fresh-password"
	if err := f.h.auth.ChangePassword(ctx, OperatorPasswordChange{Token: sess.Token, CurrentPassword: ownerTemp,
		NewPassword: fresh, TOTPCode: f.h.currentCode(t, two), IP: opIP}); err != nil {
		t.Fatalf("change password: %v", err)
	}
	acc := f.h.store.account(t, two)
	if acc.MustChangePassword || acc.TemporaryPasswordExpiresAt != nil || !acc.TOTPEnrolled() {
		t.Fatalf("after change: %v expires %v", acc, acc.TemporaryPasswordExpiresAt)
	}
	if _, err := f.h.auth.ResolveSession(ctx, sess.Token); !errors.Is(err, ErrOperatorUnauthenticated) {
		t.Fatalf("the restricted session must end with the change: %v", err)
	}

	f.h.clock.t = f.h.clock.t.Add(operatorauth.TOTPPeriod)
	sess, err = login(f.h, ownerTwoEmail, fresh, f.h.currentCode(t, two), "")
	if err != nil {
		t.Fatal(err)
	}
	if p, err := f.h.auth.ResolveSession(ctx, sess.Token); err != nil || p.MustChangePassword {
		t.Fatalf("the next session must be unrestricted: %+v %v", p, err)
	}
}

func TestRestrictedSessionEndsWithTheTemporaryPassword(t *testing.T) {
	f := newOwnerFixture(t)
	ctx := context.Background()
	two, _ := f.enrolSecond(t)
	if _, err := f.owner.ResetPassword(ctx, f.actor, two, ownerTemp, "forgot"); err != nil {
		t.Fatal(err)
	}
	sess, err := login(f.h, ownerTwoEmail, ownerTemp, f.h.currentCode(t, two), "")
	if err != nil {
		t.Fatal(err)
	}
	// The 24 hours run out while the session is still inside its idle window.
	acc := f.h.store.account(t, two)
	past := f.h.clock.t.Add(-time.Second)
	acc.TemporaryPasswordExpiresAt = &past
	f.h.store.state.accounts[acc.ID] = acc
	if _, err := f.h.auth.ResolveSession(ctx, sess.Token); !errors.Is(err, ErrOperatorUnauthenticated) {
		t.Fatalf("a restricted session must not outlive its temporary password: %v", err)
	}
	f.h.clock.t = f.h.clock.t.Add(operatorauth.TOTPPeriod)
	if _, err := login(f.h, ownerTwoEmail, ownerTemp, f.h.currentCode(t, two), ""); !errors.Is(err, ErrOperatorLoginFailed) {
		t.Fatalf("an expired temporary password must be REFUSED: %v", err)
	}
}

func TestNeverEnrolledAccountStillNeedsEnrollment(t *testing.T) {
	f := newOwnerFixture(t)
	ctx := context.Background()
	v, err := f.owner.CreateAccount(ctx, f.actor, NewOperatorByOwner{Email: ownerTwoEmail, DisplayName: "Two",
		TemporaryPassword: ownerTemp})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := login(f.h, ownerTwoEmail, ownerTemp, "123456", ""); !errors.Is(err, ErrOperatorEnrollmentRequired) {
		t.Fatalf("no factor yet: %v, want ErrOperatorEnrollmentRequired", err)
	}
	// Reset of a never-enrolled account: still the enrolment path.
	if _, err := f.owner.ResetPassword(ctx, f.actor, v.Account.Code, ownerTemp+"-2", "lost it"); err != nil {
		t.Fatal(err)
	}
	if _, err := login(f.h, ownerTwoEmail, ownerTemp+"-2", "", ""); !errors.Is(err, ErrOperatorEnrollmentRequired) {
		t.Fatalf("after reset, no factor: %v, want ErrOperatorEnrollmentRequired", err)
	}
}

func TestOwnerListAccountsIncludesDisabled(t *testing.T) {
	f := newOwnerFixture(t)
	two := f.second(t)
	if err := f.h.admin.Disable(context.Background(), two.Code, "left", "OPS-D"); err != nil {
		t.Fatal(err)
	}
	views, err := f.owner.ListAccounts(context.Background())
	if err != nil || len(views) != 2 {
		t.Fatalf("views = %v, %v", views, err)
	}
	if !views[0].Account.IsOwner || !views[1].Account.Disabled() {
		t.Fatalf("owner/disabled flags lost: %v %v", views[0].Account, views[1].Account)
	}
}

func TestCheckOperatorPermissionKeys(t *testing.T) {
	keys, err := CheckOperatorPermissionKeys([]string{"ops.qr.issue", "ops.domain.manage", "ops.qr.issue"})
	if err != nil || len(keys) != 2 || keys[0] != domain.OperatorPermissionDomainManage {
		t.Fatalf("keys = %v, %v", keys, err)
	}
	if _, err := CheckOperatorPermissionKeys([]string{"ops.qr.issue", "admin.user"}); err == nil {
		t.Fatal("a key off the closed list must refuse the whole set")
	}
}
