package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/vihat/vigov/core/password"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/service-identity/internal/domain"
	"github.com/vihat/vigov/service-identity/internal/operatorauth"
	"github.com/vihat/vigov/service-identity/internal/store/operatorstore"
)

// The sign-in side of the Vihat OPERATOR realm (ADR 0048 step 1, owner's decisions 2026-09-28).
//
// WHAT THIS IS NOT: the staff sign-in (dang_nhap.go). A staff member signs in to ONE commune,
// resolved from Host, with a v1 token that carries tenant_id. An operator belongs to no commune:
// the token is `op1.` with no tenant field, signed with OPERATOR_SESSION_SIGNING_KEYS, the session
// lives in operator_session and the trail in operator_audit_log. Nothing here calls tenant.From —
// there is no commune to read, and inventing one would be a default on the isolation path.
//
// THE SHAPE OF EVERY CREDENTIAL CHECK BELOW, and why it commits on failure: a failed attempt is
// COUNTED (5 consecutive → locked 15 minutes, owner's decision). The counter lives in the database,
// so the transaction that reads the account must COMMIT the increment even though the sign-in
// fails. Each flow therefore runs one transaction whose closure returns nil for a refused attempt
// and records the outcome in a variable; the error returned to the caller is chosen after commit.
// Returning the refusal as the closure's error would roll the counter back and make the lockout a
// decoration.

// Errors of the operator realm. Handlers (step 2) map them to status codes.
var (
	// ErrOperatorRealmNotConfigured — OPERATOR_SESSION_SIGNING_KEYS or OPERATOR_TOTP_ENCRYPTION_KEY
	// is absent. Every sign-in flow refuses; nothing falls back (fail closed, core/config operator.go).
	ErrOperatorRealmNotConfigured = errors.New("operator: operator realm is not configured")

	// ErrOperatorLoginFailed is THE ONE ANSWER for every refused credential: unknown email, wrong
	// password, wrong or replayed TOTP code, used recovery code, missing second factor, locked or
	// disabled account. One error, one log line — see the staff precedent ErrDangNhapThatBai for why
	// two sentences rebuild the directory. An operator account reaches every commune's platform
	// metadata, so the directory it would leak is worth more than a commune's.
	ErrOperatorLoginFailed = errors.New("operator: sign-in refused")

	// ErrOperatorEnrollmentRequired — the password was RIGHT, and the account has no active second
	// factor yet (first sign-in, or after an MFA reset). Returned only after the password verified:
	// before that, it would tell anybody which addresses are fresh operator accounts.
	ErrOperatorEnrollmentRequired = errors.New("operator: second factor enrolment required")

	// ErrOperatorSecondFactorAmbiguous — both a TOTP code and a recovery code were sent. A property
	// of the REQUEST, decided before any lookup, so it says nothing about any account.
	ErrOperatorSecondFactorAmbiguous = errors.New("operator: send a TOTP code or a recovery code, not both")

	// ErrOperatorUnauthenticated — the bearer token is missing, malformed, from another realm,
	// expired, or names a session that is revoked, expired or whose account is disabled. One answer.
	ErrOperatorUnauthenticated = errors.New("operator: not signed in")
)

// totpIssuer is the issuer shown in the authenticator app. Fixed by the task card: "ViGov".
const totpIssuer = "ViGov"

// Second-factor labels written into the login_succeeded entry, so the trail says HOW the operator
// proved it — a sign-in by recovery code is the one an inspection asks about.
const (
	secondFactorTOTP         = "totp"
	secondFactorRecoveryCode = "recovery_code"
	secondFactorEnrollment   = "totp_enrollment"
)

// lockoutReason is the reason on the SystemActor entry a lockout writes. The lock is an act of the
// system, not of the person guessing, and a system entry must say why (domain Validate).
const lockoutReason = "automatic: consecutive failed sign-in attempts reached the limit"

// logoutReason is operator_session.revoked_reason for a sign-out.
const logoutReason = "logout"

// OperatorAuth is the operator realm's sign-in, session and self-service use cases.
type OperatorAuth struct {
	store  OperatorStore
	signer *operatorauth.TokenSigner
	sealer *operatorauth.Sealer
	now    func() time.Time
	log    *slog.Logger // the SECURITY log (skills/security-logging): events, never values

	timingOnce sync.Once
	timingHash string
}

// NewOperatorAuth builds the use cases. signer and sealer MAY be nil — that is the "not
// configured" state, and every flow then answers ErrOperatorRealmNotConfigured. store and log may
// not: a missing collaborator panics here, where somebody is watching a process start.
func NewOperatorAuth(store OperatorStore, signer *operatorauth.TokenSigner, sealer *operatorauth.Sealer,
	now func() time.Time, log *slog.Logger) *OperatorAuth {
	if store == nil || log == nil {
		panic("app.NewOperatorAuth: store and log are required")
	}
	if now == nil {
		now = time.Now
	}
	return &OperatorAuth{store: store, signer: signer, sealer: sealer, now: now, log: log}
}

func (a *OperatorAuth) configured() error {
	if a.signer == nil || a.sealer == nil {
		return ErrOperatorRealmNotConfigured
	}
	return nil
}

func (a *OperatorAuth) clock() time.Time { return a.now().UTC() }

// OperatorLoginRequest is one sign-in attempt. Exactly one of TOTPCode / RecoveryCode proves the
// second factor.
type OperatorLoginRequest struct {
	Email        string
	Password     string
	TOTPCode     string
	RecoveryCode string
	IP           string
	UserAgent    string
}

// OperatorSessionIssued is what a successful sign-in returns. Token is the `op1.` bearer value,
// signed INSIDE the transaction that wrote the session and its audit entry.
type OperatorSessionIssued struct {
	Token       string
	ExpiresAt   time.Time
	AccountCode string
}

// outcome is what a credential transaction decided, read after it committed.
type outcome int

const (
	outcomeRefused outcome = iota
	outcomeEnrollmentRequired
	outcomeUnauthenticated
	outcomeAccepted
)

// Login signs an operator in with password + one second factor.
func (a *OperatorAuth) Login(ctx context.Context, in OperatorLoginRequest) (OperatorSessionIssued, error) {
	if err := a.configured(); err != nil {
		return OperatorSessionIssued{}, err
	}
	if in.TOTPCode != "" && in.RecoveryCode != "" {
		return OperatorSessionIssued{}, ErrOperatorSecondFactorAmbiguous
	}
	now := a.clock()

	var (
		result   OperatorSessionIssued
		decided  outcome
		code     string
		oldHash  string
		accID    string
		upgrade  bool
		viaCodes bool
	)
	err := a.store.InTx(ctx, func(tx OperatorTx) error {
		decided, result = outcomeRefused, OperatorSessionIssued{}
		acc, creds, passwordOK, found, err := a.checkPassword(ctx, tx, in.Email, in.Password)
		if err != nil || !found {
			return err
		}
		code = acc.Code
		if acc.Disabled() || acc.LockedAt(now) {
			return nil // refused, not counted: a lock must not be extended by the guesses it stops
		}
		if !passwordOK {
			return a.registerFailure(ctx, tx, acc, in.IP, now)
		}
		if acc.MustChangePassword || !acc.TOTPEnrolled() {
			decided = outcomeEnrollmentRequired
			return nil
		}

		var factor string
		switch {
		case in.TOTPCode != "":
			v, err := a.verifyActiveTOTP(ctx, tx, acc, creds, in.TOTPCode, now)
			if err != nil {
				return err
			}
			if v == totpUnreadable {
				return nil
			}
			if v != totpAccepted {
				return a.registerFailure(ctx, tx, acc, in.IP, now)
			}
			factor = secondFactorTOTP
		case in.RecoveryCode != "":
			used, err := tx.UseRecoveryCode(ctx, acc.ID, operatorauth.HashRecoveryCode(in.RecoveryCode), now)
			if err != nil {
				return err
			}
			if !used {
				return a.registerFailure(ctx, tx, acc, in.IP, now)
			}
			if err := tx.AppendAudit(ctx, domain.OperatorAuditEntry{
				OccurredAt: now, Actor: acc.Code, ActorIP: in.IP,
				Action: domain.OperatorAuditRecoveryCodeUsed, Subject: acc.Code,
			}); err != nil {
				return err
			}
			factor, viaCodes = secondFactorRecoveryCode, true
		default:
			// Password right, no second factor sent: counted like a wrong code. Answering anything
			// else would turn "password only" into a free password oracle.
			return a.registerFailure(ctx, tx, acc, in.IP, now)
		}

		if err := resetFailures(ctx, tx, acc.ID, now); err != nil {
			return err
		}
		issued, err := a.openSession(ctx, tx, acc, in.IP, in.UserAgent, factor, now)
		if err != nil {
			return err
		}
		result, decided = issued, outcomeAccepted
		oldHash, accID, upgrade = creds.PasswordHash, acc.ID, password.CanBamLai(creds.PasswordHash)
		return nil
	})
	if errors.Is(err, errLockedMidAttempt) {
		decided, err = outcomeRefused, nil
	}
	if err != nil {
		// Nothing committed: no session, no trail, no token — the three agree.
		return OperatorSessionIssued{}, fmt.Errorf("operator login: %w", err)
	}

	switch decided {
	case outcomeAccepted:
		a.log.Info("operator sign-in", "event", "operator.login", "outcome", "success",
			"actor", code, "second_factor_recovery_code", viaCodes, "ip", in.IP)
		if upgrade {
			a.upgradePasswordHash(ctx, accID, code, oldHash, in.Password)
		}
		return result, nil
	case outcomeEnrollmentRequired:
		a.log.Info("operator sign-in", "event", "operator.login", "outcome", "enrollment_required",
			"actor", code, "ip", in.IP)
		return OperatorSessionIssued{}, ErrOperatorEnrollmentRequired
	default:
		a.refusedLog("operator.login", in.Email, in.IP)
		return OperatorSessionIssued{}, ErrOperatorLoginFailed
	}
}

// errLockedMidAttempt: the account was found locked at the moment a success was about to clear the
// failure state. The row lock makes this unreachable in practice; the store's own guard reports it
// anyway, and the attempt is then ROLLED BACK — not committed as a refusal — because by then a
// factor may already have been consumed or activated in this transaction, and none of that may
// stand without the sign-in it belonged to.
var errLockedMidAttempt = errors.New("operator: account locked during the attempt")

// resetFailures clears the failure state after a success, mapping a lock in force to
// errLockedMidAttempt.
func resetFailures(ctx context.Context, tx OperatorTx, accountID string, now time.Time) error {
	err := tx.ResetFailures(ctx, accountID, now)
	if errors.Is(err, operatorstore.ErrAccountLocked) {
		return errLockedMidAttempt
	}
	return err
}

// refusedLog writes THE ONE LINE a refused credential produces, whatever the reason: the email's
// fingerprint (vanTay — never the address) and the IP. No code, no reason, no hint.
func (a *OperatorAuth) refusedLog(event, email, ip string) {
	a.log.Info("operator credential refused", "event", event, "outcome", "failure",
		"email_fingerprint", vanTay(email), "ip", ip)
}

// checkPassword reads the account and its credentials and verifies the password. found = false
// for an unknown address — after spending one argon2 verification anyway, so the response time
// does not say which addresses exist (the staff path does not do this; the operator path is worth
// more to enumerate and costs one hash to protect).
//
// THE ACCOUNT ROW IS LOCKED (FOR UPDATE) until the transaction ends, so parallel attempts on one
// account are serialised and each sees the failure count and lock the previous one committed.
func (a *OperatorAuth) checkPassword(ctx context.Context, tx OperatorTx, email, plain string) (
	acc domain.OperatorAccount, creds operatorstore.Credentials, ok, found bool, err error) {
	acc, err = tx.ByEmailForUpdate(ctx, email)
	if errors.Is(err, operatorstore.ErrNotFound) {
		_ = password.KiemTra(plain, a.timingEqualiser()) // result deliberately unused: timing only
		return domain.OperatorAccount{}, operatorstore.Credentials{}, false, false, nil
	}
	if err != nil {
		return domain.OperatorAccount{}, operatorstore.Credentials{}, false, false, err
	}
	creds, err = tx.Credentials(ctx, acc.ID)
	if err != nil {
		return domain.OperatorAccount{}, operatorstore.Credentials{}, false, false, err
	}
	// Verified EVEN for a disabled or locked account, for the same timing reason; the caller
	// decides what the answer means.
	return acc, creds, password.KiemTra(plain, creds.PasswordHash) == nil, true, nil
}

// timingEqualiser is an argon2id hash of a random value nobody knows, made once, lazily, with the
// current parameters — so the unknown-address path costs what the known-address path costs.
func (a *OperatorAuth) timingEqualiser() string {
	a.timingOnce.Do(func() {
		h, err := password.Bam(rand.Text())
		if err != nil {
			// Unreachable (rand.Text is 26 characters); an empty string still refuses in KiemTra.
			a.log.Warn("operator timing equaliser unavailable", "err", err)
		}
		a.timingHash = h
	})
	return a.timingHash
}

// registerFailure counts one failed credential attempt and, when this failure is the one that
// locks the account, writes the lock into the trail IN THE SAME TRANSACTION. Returns nil for a
// refusal so the count commits (see the note at the top of this file).
func (a *OperatorAuth) registerFailure(ctx context.Context, tx OperatorTx, acc domain.OperatorAccount,
	ip string, now time.Time) error {
	lockedNow, until, err := tx.RegisterFailure(ctx, acc.ID, now)
	if errors.Is(err, operatorstore.ErrAccountLocked) {
		return nil // a concurrent attempt locked it first; that attempt wrote the entry
	}
	if err != nil {
		return err
	}
	if !lockedNow {
		return nil
	}
	if err := tx.AppendAudit(ctx, domain.OperatorAuditEntry{
		OccurredAt: now, Actor: domain.SystemActor, ActorIP: ip,
		Action: domain.OperatorAuditLoginLockedOut, Subject: acc.Code,
		After:  map[string]any{"locked_until": until.UTC().Format(time.RFC3339)},
		Reason: lockoutReason,
	}); err != nil {
		return err
	}
	// The business code, not the fingerprint: the account is known here, and the audit trail
	// already names it. An alert on this line is what tells somebody an operator is being attacked.
	a.log.Warn("operator account locked", "event", "operator.login_locked_out", "outcome", "locked",
		"actor", domain.SystemActor, "subject", acc.Code, "ip", ip,
		"locked_until", until.UTC().Format(time.RFC3339))
	return nil
}

type totpVerdict int

const (
	totpRejected totpVerdict = iota
	totpAccepted
	totpUnreadable
)

// verifyActiveTOTP checks a code against the ACTIVE factor and claims its step atomically, so a
// replayed code (same or older step) is refused even under two concurrent sign-ins.
//
// totpUnreadable: the sealed secret does not open — a key missing from OPERATOR_TOTP_ENCRYPTION_KEY
// or a corrupted row. That is the server's fault, not a guess: it is NOT counted (counting would
// lock every operator out during a botched key rotation), and it is logged at Error with the code
// so somebody can act. The caller still answers the uniform refusal, because a distinct answer here
// would confirm the password to whoever typed it.
func (a *OperatorAuth) verifyActiveTOTP(ctx context.Context, tx OperatorTx, acc domain.OperatorAccount,
	creds operatorstore.Credentials, code string, now time.Time) (totpVerdict, error) {
	s, err := a.sealer.Open(creds.TOTPSecretSealed, []byte(acc.ID))
	if err != nil {
		a.log.Error("operator TOTP secret cannot be opened", "event", "operator.totp_unreadable",
			"subject", acc.Code, "err", err)
		return totpUnreadable, nil
	}
	step, ok := operatorauth.Verify(s, code, now)
	if !ok {
		return totpRejected, nil
	}
	fresh, err := tx.RecordTOTPStep(ctx, acc.ID, step, now)
	if err != nil {
		return totpRejected, err
	}
	if !fresh {
		return totpRejected, nil
	}
	return totpAccepted, nil
}

// openSession registers a session, signs its token INSIDE the transaction, and writes
// login_succeeded — the staff precedent (dang_nhap.go): a signing failure after commit would leave
// a trail stating a sign-in for which no token was ever issued, and the trail is append-only.
func (a *OperatorAuth) openSession(ctx context.Context, tx OperatorTx, acc domain.OperatorAccount,
	ip, userAgent, factor string, now time.Time) (OperatorSessionIssued, error) {
	sid, err := newOperatorSessionID()
	if err != nil {
		return OperatorSessionIssued{}, err
	}
	expires, err := tx.CreateSession(ctx, sid, acc.ID, ip, userAgent, now)
	if err != nil {
		return OperatorSessionIssued{}, err
	}
	tok, err := a.signer.Sign(operatorauth.TokenClaims{SessionID: sid, ExpiresAt: expires})
	if err != nil {
		return OperatorSessionIssued{}, fmt.Errorf("sign operator token: %w", err)
	}
	if err := tx.AppendAudit(ctx, domain.OperatorAuditEntry{
		OccurredAt: now, Actor: acc.Code, ActorIP: ip,
		Action: domain.OperatorAuditLoginSucceeded, Subject: acc.Code,
		After: map[string]any{"second_factor": factor},
	}); err != nil {
		return OperatorSessionIssued{}, err
	}
	return OperatorSessionIssued{Token: tok, ExpiresAt: expires, AccountCode: acc.Code}, nil
}

// newOperatorSessionID is 256 bits from crypto/rand. The store keeps only its SHA-256.
func newOperatorSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("operator session id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// upgradePasswordHash re-hashes a password stored under weaker argon2 parameters. A failure is
// logged and swallowed, like the staff path: the sign-in already succeeded, and refusing it because
// a background upgrade failed would lock out the very accounts being protected.
//
// NOT AUDITED, like the staff path: the credential does not change — the same password under a
// stronger cost — and nobody acted. Stated as an assumption in the TASK-03 report.
func (a *OperatorAuth) upgradePasswordHash(ctx context.Context, accountID, code, oldHash, plain string) {
	newHash, err := password.Bam(plain)
	if err != nil {
		a.log.Warn("operator password rehash failed", "subject", code, "err", err)
		return
	}
	err = a.store.InTx(ctx, func(tx OperatorTx) error {
		return tx.UpgradePasswordHash(ctx, accountID, oldHash, newHash, a.clock())
	})
	if err != nil && !errors.Is(err, operatorstore.ErrStateConflict) {
		a.log.Warn("operator password rehash not stored", "subject", code, "err", err)
	}
}

// OperatorPrincipal is the operator behind a live session. ID authorises, Code is the "who" of
// every trail entry (rule 6, invariant 8).
type OperatorPrincipal struct {
	ID          string
	Code        string
	Permissions []domain.OperatorPermission
	ExpiresAt   time.Time
}

// Has reports whether the principal holds p. Exact match, the closed list's rule.
func (p OperatorPrincipal) Has(perm domain.OperatorPermission) bool {
	for _, k := range p.Permissions {
		if k == perm {
			return true
		}
	}
	return false
}

// ResolveSession turns an `op1.` bearer token into the operator behind it, on every operator
// request (step 2 wires it). A staff `v1.` token fails at the prefix and, were the prefix forged,
// at the MAC — the realms share neither format nor key (ADR 0048 stop condition #6).
func (a *OperatorAuth) ResolveSession(ctx context.Context, token string) (OperatorPrincipal, error) {
	if err := a.configured(); err != nil {
		return OperatorPrincipal{}, err
	}
	now := a.clock()
	claims, err := a.signer.Verify(token, now)
	if err != nil {
		return OperatorPrincipal{}, ErrOperatorUnauthenticated
	}
	var p OperatorPrincipal
	decided := outcomeUnauthenticated
	err = a.store.InTx(ctx, func(tx OperatorTx) error {
		decided = outcomeUnauthenticated
		sess, acc, ok, err := liveSession(ctx, tx, claims.SessionID, now)
		if err != nil || !ok {
			return err
		}
		perms, err := tx.ActivePermissions(ctx, acc.ID)
		if err != nil {
			return err
		}
		if err := tx.TouchSession(ctx, claims.SessionID, now); err != nil {
			if errors.Is(err, operatorstore.ErrNotFound) {
				return nil // expired or revoked between the two statements
			}
			return err
		}
		p = OperatorPrincipal{ID: acc.ID, Code: acc.Code, Permissions: perms, ExpiresAt: sess.ExpiresAt}
		decided = outcomeAccepted
		return nil
	})
	if err != nil {
		return OperatorPrincipal{}, fmt.Errorf("operator session: %w", err)
	}
	if decided != outcomeAccepted {
		return OperatorPrincipal{}, ErrOperatorUnauthenticated
	}
	return p, nil
}

// liveSession resolves a sid to its live session and account. ok = false for every reason a
// session is not live — one answer (the store's CheckSession already joins disabled accounts; the
// second check is here so this function does not depend on that join staying).
func liveSession(ctx context.Context, tx OperatorTx, sid string, now time.Time) (
	operatorstore.OperatorSession, domain.OperatorAccount, bool, error) {
	sess, err := tx.CheckSession(ctx, sid, now)
	if errors.Is(err, operatorstore.ErrNotFound) {
		return operatorstore.OperatorSession{}, domain.OperatorAccount{}, false, nil
	}
	if err != nil {
		return operatorstore.OperatorSession{}, domain.OperatorAccount{}, false, err
	}
	acc, err := tx.ByID(ctx, sess.AccountID)
	if errors.Is(err, operatorstore.ErrNotFound) {
		return operatorstore.OperatorSession{}, domain.OperatorAccount{}, false, nil
	}
	if err != nil {
		return operatorstore.OperatorSession{}, domain.OperatorAccount{}, false, err
	}
	if acc.Disabled() {
		return operatorstore.OperatorSession{}, domain.OperatorAccount{}, false, nil
	}
	return sess, acc, true, nil
}

// Logout revokes the bearer's OWN session — the sid comes from the verified token, never from a
// parameter — and writes logged_out in the same transaction.
func (a *OperatorAuth) Logout(ctx context.Context, token, ip string) error {
	if err := a.configured(); err != nil {
		return err
	}
	now := a.clock()
	claims, err := a.signer.Verify(token, now)
	if err != nil {
		return ErrOperatorUnauthenticated
	}
	var code string
	decided := outcomeUnauthenticated
	err = a.store.InTx(ctx, func(tx OperatorTx) error {
		decided = outcomeUnauthenticated
		_, acc, ok, err := liveSession(ctx, tx, claims.SessionID, now)
		if err != nil || !ok {
			return err
		}
		if err := tx.RevokeSession(ctx, claims.SessionID, logoutReason, now); err != nil {
			return err
		}
		if err := tx.AppendAudit(ctx, domain.OperatorAuditEntry{
			OccurredAt: now, Actor: acc.Code, ActorIP: ip,
			Action: domain.OperatorAuditLoggedOut, Subject: acc.Code,
		}); err != nil {
			return err
		}
		code, decided = acc.Code, outcomeAccepted
		return nil
	})
	if err != nil {
		return fmt.Errorf("operator logout: %w", err)
	}
	if decided != outcomeAccepted {
		return ErrOperatorUnauthenticated
	}
	a.log.Info("operator sign-out", "event", "operator.logout", "outcome", "success", "actor", code, "ip", ip)
	return nil
}

// OperatorPasswordChange is a signed-in operator changing their own password. It re-proves both
// factors: a session alone — a stolen cookie — must not be enough to take the account over.
type OperatorPasswordChange struct {
	Token           string
	CurrentPassword string
	NewPassword     string
	TOTPCode        string
	IP              string
}

// ChangePassword sets a new password and revokes EVERY session of the account, the current one
// included (owner's decision: a password change revokes all sessions). The caller signs in again.
func (a *OperatorAuth) ChangePassword(ctx context.Context, in OperatorPasswordChange) error {
	if err := a.configured(); err != nil {
		return err
	}
	now := a.clock()
	claims, err := a.signer.Verify(in.Token, now)
	if err != nil {
		return ErrOperatorUnauthenticated
	}
	// The policy is the staff one, reused — not copied (domain/mat_khau.go). Checked before any
	// hash or transaction: it depends on nothing but the new value.
	if err := domain.KiemTraMatKhauMoi(in.NewPassword); err != nil {
		return err
	}
	if in.NewPassword == in.CurrentPassword {
		return domain.ErrMatKhauMoiTrungCu
	}
	newHash, err := password.Bam(in.NewPassword)
	if err != nil {
		return fmt.Errorf("operator change password: %w", err)
	}

	var code string
	decided := outcomeUnauthenticated
	err = a.store.InTx(ctx, func(tx OperatorTx) error {
		decided = outcomeUnauthenticated
		_, acc, ok, err := liveSession(ctx, tx, claims.SessionID, now)
		if err != nil || !ok {
			return err
		}
		code, decided = acc.Code, outcomeRefused
		v, err := a.reauthenticate(ctx, tx, acc.ID, true, in.CurrentPassword, in.TOTPCode, in.IP, now)
		if err != nil || v != totpAccepted {
			return err
		}
		if err := tx.SetPassword(ctx, acc.ID, newHash, false, now); err != nil {
			return err
		}
		if err := tx.AppendAudit(ctx, domain.OperatorAuditEntry{
			OccurredAt: now, Actor: acc.Code, ActorIP: in.IP,
			Action: domain.OperatorAuditPasswordChanged, Subject: acc.Code,
			After: map[string]any{"sessions_revoked": true},
		}); err != nil {
			return err
		}
		decided = outcomeAccepted
		return nil
	})
	if err != nil {
		return fmt.Errorf("operator change password: %w", err)
	}
	switch decided {
	case outcomeAccepted:
		a.log.Info("operator password changed", "event", "operator.password_changed", "outcome", "success",
			"actor", code, "ip", in.IP)
		return nil
	case outcomeRefused:
		a.log.Info("operator credential refused", "event", "operator.password_change", "outcome", "failure",
			"actor", code, "ip", in.IP)
		return ErrOperatorLoginFailed
	default:
		return ErrOperatorUnauthenticated
	}
}

// reauthenticate re-proves the password (when requirePassword) and TOTP for a signed-in operator.
// Every failure is counted toward the lockout — a stolen session must not buy unlimited guesses —
// and answers totpRejected; the caller then returns ErrOperatorLoginFailed.
//
// IT RE-READS THE ACCOUNT WITH A ROW LOCK (ByIDForUpdate) rather than trusting the session read:
// the lock state that decides this attempt must be the one no parallel attempt can change until
// this transaction ends — the same serialisation the sign-in path gets from ByEmailForUpdate.
func (a *OperatorAuth) reauthenticate(ctx context.Context, tx OperatorTx, accountID string,
	requirePassword bool, currentPassword, totpCode, ip string, now time.Time) (totpVerdict, error) {
	acc, err := tx.ByIDForUpdate(ctx, accountID)
	if err != nil {
		return totpRejected, err
	}
	if acc.Disabled() || acc.LockedAt(now) {
		return totpUnreadable, nil // refused, not counted — same rule as sign-in
	}
	creds, err := tx.Credentials(ctx, acc.ID)
	if err != nil {
		return totpRejected, err
	}
	if requirePassword && password.KiemTra(currentPassword, creds.PasswordHash) != nil {
		return totpRejected, a.registerFailure(ctx, tx, acc, ip, now)
	}
	v, err := a.verifyActiveTOTP(ctx, tx, acc, creds, totpCode, now)
	if err != nil {
		return totpRejected, err
	}
	if v == totpRejected {
		return totpRejected, a.registerFailure(ctx, tx, acc, ip, now)
	}
	return v, nil
}

// RegenerateRecoveryCodes voids every live recovery code and returns a fresh batch of
// domain.RecoveryCodeCount — shown ONCE; only their SHA-256 is stored. Needs a current TOTP code:
// minting a new way in on the strength of a session alone would outlive the session.
func (a *OperatorAuth) RegenerateRecoveryCodes(ctx context.Context, token, totpCode, ip string) ([]secret.Secret, error) {
	if err := a.configured(); err != nil {
		return nil, err
	}
	now := a.clock()
	claims, err := a.signer.Verify(token, now)
	if err != nil {
		return nil, ErrOperatorUnauthenticated
	}
	codes, digests, err := operatorauth.GenerateRecoveryCodes(domain.RecoveryCodeCount)
	if err != nil {
		return nil, fmt.Errorf("operator recovery codes: %w", err)
	}

	var code string
	decided := outcomeUnauthenticated
	err = a.store.InTx(ctx, func(tx OperatorTx) error {
		decided = outcomeUnauthenticated
		_, acc, ok, err := liveSession(ctx, tx, claims.SessionID, now)
		if err != nil || !ok {
			return err
		}
		code, decided = acc.Code, outcomeRefused
		v, err := a.reauthenticate(ctx, tx, acc.ID, false, "", totpCode, ip, now)
		if err != nil || v != totpAccepted {
			return err
		}
		if _, err := tx.ReplaceRecoveryCodes(ctx, acc.ID, digests, now); err != nil {
			return err
		}
		if err := tx.AppendAudit(ctx, domain.OperatorAuditEntry{
			OccurredAt: now, Actor: acc.Code, ActorIP: ip,
			Action: domain.OperatorAuditRecoveryCodesRegenerated, Subject: acc.Code,
			After: map[string]any{"count": domain.RecoveryCodeCount},
		}); err != nil {
			return err
		}
		decided = outcomeAccepted
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("operator recovery codes: %w", err)
	}
	switch decided {
	case outcomeAccepted:
		a.log.Info("operator recovery codes regenerated", "event", "operator.recovery_codes_regenerated",
			"outcome", "success", "actor", code, "ip", ip)
		return codes, nil
	case outcomeRefused:
		a.log.Info("operator credential refused", "event", "operator.recovery_codes_regenerate",
			"outcome", "failure", "actor", code, "ip", ip)
		return nil, ErrOperatorLoginFailed
	default:
		return nil, ErrOperatorUnauthenticated
	}
}
