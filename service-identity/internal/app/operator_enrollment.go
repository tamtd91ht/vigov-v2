package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/password"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/service-identity/internal/domain"
	"github.com/vihat/vigov/service-identity/internal/operatorauth"
	"github.com/vihat/vigov/service-identity/internal/store/operatorstore"
)

// Second-factor enrolment: the first sign-in of an operator the CLI created, and the first sign-in
// after an MFA reset (which also issues a fresh temporary password — operator_admin.go ResetMFA).
//
// TWO CALLS, because the operator must scan the secret between them:
//
//	BeginEnrollment    temporary password → a pending TOTP secret, shown once as a provisioning URI
//	CompleteEnrollment temporary password + new password + a code from the app → active factor,
//	                   own password, 10 recovery codes, and a session
//
// BOTH COUNT FAILED PASSWORDS toward the lockout. An enrolment route that did not would be an
// unlimited oracle for the temporary password — the one credential that, on a fresh account, is
// everything.

// OperatorEnrollmentRequest starts an enrolment.
type OperatorEnrollmentRequest struct {
	Email             string
	TemporaryPassword string
	IP                string
}

// OperatorEnrollmentStart is shown ONCE. Both fields CONTAIN THE TOTP SECRET, hence secret.Secret:
// ProvisioningURI is what the QR code encodes; ManualEntryKey is the base32 form for typing in.
type OperatorEnrollmentStart struct {
	AccountCode     string
	ProvisioningURI secret.Secret
	ManualEntryKey  secret.Secret
}

// BeginEnrollment verifies the temporary password and stores a fresh TOTP secret, sealed with
// AES-256-GCM under the account id as associated data (so a sealed value copied onto another row
// does not open). Calling it again replaces the pending secret: an operator who lost the first QR
// code simply starts over.
//
// Only for an account with NO active factor. Replacing an active factor is an MFA reset — a CLI act
// with a ticket, audited — never something a password alone can do.
//
// AUDITED as totp_enrollment_started (owner's decision 28/09/2026, TASK-04), in the same
// transaction as the pending secret. The pending secret expires after domain.PendingTOTPLifetime
// (10 minutes) and the temporary password after domain.TemporaryPasswordLifetime (24 hours); past
// either, enrolment is refused.
func (a *OperatorAuth) BeginEnrollment(ctx context.Context, in OperatorEnrollmentRequest) (OperatorEnrollmentStart, error) {
	if err := a.configured(); err != nil {
		return OperatorEnrollmentStart{}, err
	}
	now := a.clock()
	totpSecret, err := operatorauth.GenerateSecret()
	if err != nil {
		return OperatorEnrollmentStart{}, fmt.Errorf("operator enrolment: %w", err)
	}

	var start OperatorEnrollmentStart
	var code string
	decided := outcomeRefused
	err = a.store.InTx(ctx, func(tx OperatorTx) error {
		decided, start = outcomeRefused, OperatorEnrollmentStart{}
		acc, _, passwordOK, found, err := a.checkPassword(ctx, tx, in.Email, in.TemporaryPassword)
		if err != nil || !found {
			return err
		}
		code = acc.Code
		if acc.Disabled() || acc.LockedAt(now) {
			return nil
		}
		if !passwordOK {
			return a.registerFailure(ctx, tx, acc, in.IP, now)
		}
		// Enrolment is reachable only with a USABLE temporary password: an account that already has
		// a factor, or whose temporary password is past its 24 hours (not counted — not a guess).
		if acc.TOTPEnrolled() || !acc.MustChangePassword || acc.TemporaryPasswordExpiredAt(now) {
			return nil
		}
		sealed, err := a.sealer.Seal(totpSecret, []byte(acc.ID))
		if err != nil {
			return fmt.Errorf("seal TOTP secret: %w", err)
		}
		if err := tx.SetPendingTOTP(ctx, acc.ID, sealed, now); err != nil {
			if errors.Is(err, operatorstore.ErrStateConflict) {
				return nil // enrolled, disabled or expired between the read and the write
			}
			return err
		}
		// Owner's decision 28/09/2026 (TASK-04): the start of an enrolment is a write to credential
		// state and is audited, in this transaction, by the operator who started it.
		if err := tx.AppendAudit(ctx, domain.OperatorAuditEntry{
			OccurredAt: now, Actor: acc.Code, ActorIP: in.IP,
			Action: domain.OperatorAuditTOTPEnrollmentStarted, Subject: acc.Code,
			After: map[string]any{"pending_expires_at": now.Add(domain.PendingTOTPLifetime).UTC().Format(time.RFC3339)},
		}); err != nil {
			return err
		}
		// The label is the operator CODE — never the email, which would put personal data into a
		// URL and a QR image (rule 3, forbidden #4; operatorauth refuses an '@' anyway).
		uri, err := operatorauth.ProvisioningURI(totpIssuer, acc.Code, totpSecret)
		if err != nil {
			return fmt.Errorf("provisioning uri: %w", err)
		}
		start = OperatorEnrollmentStart{
			AccountCode: acc.Code, ProvisioningURI: uri, ManualEntryKey: operatorauth.EncodeSecret(totpSecret),
		}
		decided = outcomeAccepted
		return nil
	})
	if err != nil {
		return OperatorEnrollmentStart{}, fmt.Errorf("operator enrolment: %w", err)
	}
	if decided != outcomeAccepted {
		a.refusedLog("operator.enrollment_begin", in.Email, in.IP)
		return OperatorEnrollmentStart{}, ErrOperatorLoginFailed
	}
	a.log.Info("operator enrolment started", "event", "operator.enrollment_begin", "outcome", "success",
		"actor", code, "ip", in.IP)
	return start, nil
}

// OperatorEnrollmentCompletion finishes an enrolment.
type OperatorEnrollmentCompletion struct {
	Email             string
	TemporaryPassword string
	NewPassword       string
	TOTPCode          string
	IP                string
	UserAgent         string
}

// OperatorEnrollmentResult carries the session AND the recovery codes, which are returned HERE AND
// NOWHERE ELSE: only their SHA-256 is stored.
type OperatorEnrollmentResult struct {
	OperatorSessionIssued
	RecoveryCodes []secret.Secret
}

// CompleteEnrollment verifies the temporary password and a code from the pending secret, then in
// ONE transaction: activates the factor (recording the code's step, so that very code cannot be
// replayed), sets the operator's own password, issues domain.RecoveryCodeCount recovery codes,
// opens a session and writes four entries — totp_enrolled, password_changed,
// recovery_codes_regenerated, login_succeeded — all with the operator's code as actor.
func (a *OperatorAuth) CompleteEnrollment(ctx context.Context, in OperatorEnrollmentCompletion) (OperatorEnrollmentResult, error) {
	if err := a.configured(); err != nil {
		return OperatorEnrollmentResult{}, err
	}
	// The staff password policy, reused (domain/mat_khau.go): minimum 12, as the owner decided.
	if err := domain.KiemTraMatKhauMoi(in.NewPassword); err != nil {
		return OperatorEnrollmentResult{}, err
	}
	if in.NewPassword == in.TemporaryPassword {
		return OperatorEnrollmentResult{}, domain.ErrMatKhauMoiTrungCu
	}
	newHash, err := password.Bam(in.NewPassword)
	if err != nil {
		return OperatorEnrollmentResult{}, fmt.Errorf("operator enrolment: %w", err)
	}
	codes, digests, err := operatorauth.GenerateRecoveryCodes(domain.RecoveryCodeCount)
	if err != nil {
		return OperatorEnrollmentResult{}, fmt.Errorf("operator enrolment: %w", err)
	}
	now := a.clock()

	var result OperatorEnrollmentResult
	var code string
	decided := outcomeRefused
	err = a.store.InTx(ctx, func(tx OperatorTx) error {
		decided, result = outcomeRefused, OperatorEnrollmentResult{}
		acc, creds, passwordOK, found, err := a.checkPassword(ctx, tx, in.Email, in.TemporaryPassword)
		if err != nil || !found {
			return err
		}
		code = acc.Code
		if acc.Disabled() || acc.LockedAt(now) {
			return nil
		}
		if !passwordOK {
			return a.registerFailure(ctx, tx, acc, in.IP, now)
		}
		if acc.TOTPEnrolled() || len(creds.PendingTOTPSecretSealed) == 0 {
			return nil // nothing to complete: BeginEnrollment was not called, or already done
		}
		// Past 24 hours for the temporary password, or 10 minutes for the pending secret: refused,
		// not counted. The operator starts the enrolment again (or devops reissues the password).
		// The store's activateTOTP enforces both limits again in SQL.
		if !acc.MustChangePassword || acc.TemporaryPasswordExpiredAt(now) || acc.PendingTOTPExpiredAt(now) {
			return nil
		}
		pending, err := a.sealer.Open(creds.PendingTOTPSecretSealed, []byte(acc.ID))
		if err != nil {
			a.log.Error("operator pending TOTP secret cannot be opened", "event", "operator.totp_unreadable",
				"subject", acc.Code, "err", err)
			return nil
		}
		step, ok := operatorauth.Verify(pending, in.TOTPCode, now)
		if !ok {
			return a.registerFailure(ctx, tx, acc, in.IP, now)
		}

		// The EXACT ciphertext just verified goes with the activation: if a parallel BeginEnrollment
		// replaced the pending secret, the store activates nothing (ErrStateConflict) and this attempt
		// is refused — never a factor nobody proved. Nothing has been written yet in this
		// transaction, so committing the refusal commits nothing.
		if err := tx.ActivateTOTP(ctx, acc.ID, newHash, step, creds.PendingTOTPSecretSealed, now); err != nil {
			if errors.Is(err, operatorstore.ErrStateConflict) {
				return nil
			}
			return err
		}
		if _, err := tx.ReplaceRecoveryCodes(ctx, acc.ID, digests, now); err != nil {
			return err
		}
		// A lock found here ROLLS BACK the activation above (errLockedMidAttempt).
		if err := resetFailures(ctx, tx, acc.ID, now); err != nil {
			return err
		}
		for _, e := range []domain.OperatorAuditEntry{
			{Action: domain.OperatorAuditTOTPEnrolled},
			{Action: domain.OperatorAuditPasswordChanged, After: map[string]any{"must_change_password": false}},
			{Action: domain.OperatorAuditRecoveryCodesRegenerated, After: map[string]any{"count": domain.RecoveryCodeCount}},
		} {
			e.OccurredAt, e.Actor, e.ActorIP, e.Subject = now, acc.Code, in.IP, acc.Code
			if err := tx.AppendAudit(ctx, e); err != nil {
				return err
			}
		}
		issued, err := a.openSession(ctx, tx, acc, in.IP, in.UserAgent, secondFactorEnrollment, now)
		if err != nil {
			return err
		}
		result = OperatorEnrollmentResult{OperatorSessionIssued: issued, RecoveryCodes: codes}
		decided = outcomeAccepted
		return nil
	})
	if errors.Is(err, errLockedMidAttempt) {
		decided, err = outcomeRefused, nil
	}
	if err != nil {
		return OperatorEnrollmentResult{}, fmt.Errorf("operator enrolment: %w", err)
	}
	if decided != outcomeAccepted {
		a.refusedLog("operator.enrollment_complete", in.Email, in.IP)
		return OperatorEnrollmentResult{}, ErrOperatorLoginFailed
	}
	a.log.Info("operator enrolment completed", "event", "operator.totp_enrolled", "outcome", "success",
		"actor", code, "ip", in.IP)
	return result, nil
}
