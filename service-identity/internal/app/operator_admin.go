package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/vihat/vigov/core/password"
	"github.com/vihat/vigov/core/privacy"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/service-identity/internal/domain"
	"github.com/vihat/vigov/service-identity/internal/store/operatorstore"
)

// The administrative side of the operator realm — what the server-side CLI (cmd/operatorctl)
// does, by the owner's decision of 2026-09-28: devops creates accounts, grants and revokes `ops.*`,
// disables and enables, resets MFA. There is NO operator-management permission and no screen:
// adding one is ADR 0048 stop condition #1.
//
// EVERY ACT HERE IS WRITTEN WITH actor = domain.SystemActor AND reason = "ticket:<n>". The person
// at the terminal is not an account of this system, so the trail cannot name them; the ticket is
// what links the entry to a human decision somebody can be asked about. No ticket, no act — refused
// before any transaction opens.
//
// NO SIGNER, NO SEALER: none of these acts issues a token or touches a TOTP secret in the clear, so
// they work whether or not the realm's sign-in variables are set. Setting up the first operator
// before the keys are rolled out is a legitimate order of work.

// Errors of the administrative acts.
var (
	ErrTicketRequired          = errors.New("operator admin: a ticket number is required")
	ErrInvalidTicket           = errors.New("operator admin: ticket must be 1-64 printable characters with no spaces")
	ErrReasonRequired          = errors.New("operator admin: a reason is required")
	ErrInvalidOperatorEmail    = errors.New("operator admin: email is not a usable address")
	ErrInvalidOperatorName     = errors.New("operator admin: display name is required (at most 200 characters)")
	ErrOperatorNotFound        = errors.New("operator admin: no operator with this code")
	ErrOperatorAlreadyDisabled = errors.New("operator admin: operator is already disabled")
	ErrOperatorNotDisabled     = errors.New("operator admin: operator is not disabled")
)

// ticketReason renders the audit reason the owner fixed: "ticket:<n>", plus the free-text reason
// when the act carries one. Validated: an empty or blank ticket would make "system" say nobody.
func ticketReason(ticket, reason string) (string, error) {
	t := strings.TrimSpace(ticket)
	if t == "" {
		return "", ErrTicketRequired
	}
	if utf8.RuneCountInString(t) > 64 {
		return "", ErrInvalidTicket
	}
	for _, r := range t {
		if unicode.IsSpace(r) || !unicode.IsPrint(r) {
			return "", ErrInvalidTicket
		}
	}
	out := "ticket:" + t
	if r := strings.TrimSpace(reason); r != "" {
		out += " " + r
	}
	return out, nil
}

// OperatorAdmin is the CLI's use cases.
type OperatorAdmin struct {
	store OperatorStore
	now   func() time.Time
	log   *slog.Logger
}

// NewOperatorAdmin builds the administrative use cases. store and log are required.
func NewOperatorAdmin(store OperatorStore, now func() time.Time, log *slog.Logger) *OperatorAdmin {
	if store == nil || log == nil {
		panic("app.NewOperatorAdmin: store and log are required")
	}
	if now == nil {
		now = time.Now
	}
	return &OperatorAdmin{store: store, now: now, log: log}
}

func (o *OperatorAdmin) clock() time.Time { return o.now().UTC() }

// adminLog is the security-log line every administrative act writes (skills/security-logging:
// "role or permission granted / revoked", "account locked / unlocked", "MFA reset").
func (o *OperatorAdmin) adminLog(event, subject, reason string, extra ...any) {
	args := append([]any{"event", event, "outcome", "success", "actor", domain.SystemActor,
		"subject", subject, "reason", reason}, extra...)
	o.log.Info("operator admin act", args...)
}

// OperatorCreated is returned ONCE. TemporaryPassword is the only copy anywhere: the database
// holds its argon2id hash.
type OperatorCreated struct {
	Code              string
	TemporaryPassword secret.Secret
}

// validEmail is a SHAPE check, not a deliverability check: one '@', something on both sides, no
// whitespace, at most 254 bytes (RFC 5321). The address is the operator's sign-in name and nothing
// sends mail to it.
func validEmail(s string) bool {
	if s == "" || len(s) > 254 || strings.IndexFunc(s, unicode.IsSpace) >= 0 {
		return false
	}
	local, host, ok := strings.Cut(s, "@")
	return ok && local != "" && host != "" && !strings.Contains(host, "@") && strings.Contains(host, ".")
}

// CreateOperator creates an account with a minted temporary password (80 bits, domain.SinhMatKhauTam
// — the staff generator, reused) and must_change_password = true: the operator sets their own
// password and enrols TOTP at the first sign-in (operator_enrollment.go).
func (o *OperatorAdmin) CreateOperator(ctx context.Context, email, displayName, ticket string) (OperatorCreated, error) {
	reason, err := ticketReason(ticket, "")
	if err != nil {
		return OperatorCreated{}, err
	}
	email, displayName = strings.TrimSpace(email), strings.TrimSpace(displayName)
	if !validEmail(email) {
		return OperatorCreated{}, ErrInvalidOperatorEmail
	}
	if displayName == "" || utf8.RuneCountInString(displayName) > 200 || !utf8.ValidString(displayName) {
		return OperatorCreated{}, ErrInvalidOperatorName
	}
	temp, hash, err := mintTemporaryPassword()
	if err != nil {
		return OperatorCreated{}, fmt.Errorf("operator create: %w", err)
	}
	now := o.clock()

	var acc domain.OperatorAccount
	err = o.store.InTx(ctx, func(tx OperatorTx) error {
		var err error
		acc, err = tx.CreateAccount(ctx, operatorstore.NewAccount{
			Email: email, DisplayName: displayName, PasswordHash: hash, CreatedBy: domain.SystemActor,
		}, now)
		if err != nil {
			return err
		}
		// No email, no name in the entry: personal data does not go into the trail (rule 6,
		// forbidden #4). The code is the operator everywhere.
		return tx.AppendAudit(ctx, domain.OperatorAuditEntry{
			OccurredAt: now, Actor: domain.SystemActor, Action: domain.OperatorAuditAccountCreated,
			Subject: acc.Code, After: map[string]any{"must_change_password": true}, Reason: reason,
		})
	})
	if err != nil {
		return OperatorCreated{}, fmt.Errorf("operator create: %w", err)
	}
	o.adminLog("operator.account_created", acc.Code, reason)
	return OperatorCreated{Code: acc.Code, TemporaryPassword: temp}, nil
}

// mintTemporaryPassword returns a fresh temporary password and its hash.
func mintTemporaryPassword() (secret.Secret, string, error) {
	temp, err := domain.SinhMatKhauTam()
	if err != nil {
		return nil, "", err
	}
	hash, err := password.Bam(temp)
	if err != nil {
		return nil, "", err
	}
	return secret.Secret(temp), hash, nil
}

// byCode reads the account an act targets, mapping "no such code" to ErrOperatorNotFound.
func byCode(ctx context.Context, tx OperatorTx, code string) (domain.OperatorAccount, error) {
	code = strings.TrimSpace(code)
	if !domain.ValidOperatorCode(code) {
		return domain.OperatorAccount{}, ErrOperatorNotFound
	}
	acc, err := tx.ByCode(ctx, code)
	if errors.Is(err, operatorstore.ErrNotFound) {
		return domain.OperatorAccount{}, ErrOperatorNotFound
	}
	return acc, err
}

// Grant gives one `ops.*` key. The store revokes the operator's live sessions in the same
// transaction, so the new right takes effect at the next sign-in, never inside a session that was
// issued without it.
func (o *OperatorAdmin) Grant(ctx context.Context, code, permission, ticket string) error {
	return o.changeGrant(ctx, code, permission, ticket, true)
}

// Revoke withdraws one `ops.*` key (the grant row stays as history) and revokes live sessions.
func (o *OperatorAdmin) Revoke(ctx context.Context, code, permission, ticket string) error {
	return o.changeGrant(ctx, code, permission, ticket, false)
}

func (o *OperatorAdmin) changeGrant(ctx context.Context, code, permission, ticket string, grant bool) error {
	reason, err := ticketReason(ticket, "")
	if err != nil {
		return err
	}
	key, err := domain.ParseOperatorPermission(strings.TrimSpace(permission))
	if err != nil {
		return err
	}
	now := o.clock()
	event := "operator.permission_granted"
	if !grant {
		event = "operator.permission_revoked"
	}
	var subject string
	err = o.store.InTx(ctx, func(tx OperatorTx) error {
		acc, err := byCode(ctx, tx, code)
		if err != nil {
			return err
		}
		subject = acc.Code
		entry := domain.OperatorAuditEntry{
			OccurredAt: now, Actor: domain.SystemActor, Subject: acc.Code, Reason: reason,
		}
		if grant {
			if err := tx.Grant(ctx, acc.ID, key, domain.SystemActor, reason, now); err != nil {
				return err
			}
			entry.Action, entry.After = domain.OperatorAuditPermissionGranted, map[string]any{"permission_key": string(key)}
		} else {
			if err := tx.Revoke(ctx, acc.ID, key, domain.SystemActor, reason, now); err != nil {
				return err
			}
			entry.Action, entry.Before = domain.OperatorAuditPermissionRevoked, map[string]any{"permission_key": string(key)}
		}
		return tx.AppendAudit(ctx, entry)
	})
	if err != nil {
		return fmt.Errorf("%s: %w", event, err)
	}
	o.adminLog(event, subject, reason, "permission_key", string(key))
	return nil
}

// Disable is the administrative lock (soft — the row and its history stay, rule 7). The store
// revokes every live session in the same transaction.
func (o *OperatorAdmin) Disable(ctx context.Context, code, reason, ticket string) error {
	return o.setDisabled(ctx, code, reason, ticket, true)
}

// Enable lifts the administrative lock. It does not touch the failure lockout, which expires by
// itself.
func (o *OperatorAdmin) Enable(ctx context.Context, code, reason, ticket string) error {
	return o.setDisabled(ctx, code, reason, ticket, false)
}

func (o *OperatorAdmin) setDisabled(ctx context.Context, code, reason, ticket string, disable bool) error {
	if strings.TrimSpace(reason) == "" {
		return ErrReasonRequired
	}
	full, err := ticketReason(ticket, reason)
	if err != nil {
		return err
	}
	now := o.clock()
	event := "operator.account_disabled"
	if !disable {
		event = "operator.account_enabled"
	}
	var subject string
	err = o.store.InTx(ctx, func(tx OperatorTx) error {
		acc, err := byCode(ctx, tx, code)
		if err != nil {
			return err
		}
		subject = acc.Code
		entry := domain.OperatorAuditEntry{
			OccurredAt: now, Actor: domain.SystemActor, Subject: acc.Code, Reason: full,
			Before: map[string]any{"disabled": acc.Disabled()},
		}
		if disable {
			if err := tx.Disable(ctx, acc.ID, domain.SystemActor, full, now); err != nil {
				if errors.Is(err, operatorstore.ErrStateConflict) {
					return ErrOperatorAlreadyDisabled
				}
				return err
			}
			entry.Action, entry.After = domain.OperatorAuditAccountDisabled, map[string]any{"disabled": true}
		} else {
			if err := tx.Enable(ctx, acc.ID, now); err != nil {
				if errors.Is(err, operatorstore.ErrStateConflict) {
					return ErrOperatorNotDisabled
				}
				return err
			}
			entry.Action, entry.After = domain.OperatorAuditAccountEnabled, map[string]any{"disabled": false}
		}
		return tx.AppendAudit(ctx, entry)
	})
	if err != nil {
		return fmt.Errorf("%s: %w", event, err)
	}
	o.adminLog(event, subject, full)
	return nil
}

// ResetMFA removes the TOTP factor, voids every recovery code, revokes every session — AND sets a
// fresh temporary password with must_change_password = true, returned once.
//
// WHY THE PASSWORD TOO (task card): after a reset, BeginEnrollment accepts the account's password
// to bind a new authenticator. If the old password survived the reset, whoever stole it — the most
// common reason an MFA reset is asked for — could enrol THEIR authenticator before the real
// operator does. A reset is a full re-onboarding, through a value only the ticket's requester gets.
func (o *OperatorAdmin) ResetMFA(ctx context.Context, code, ticket string) (secret.Secret, error) {
	reason, err := ticketReason(ticket, "")
	if err != nil {
		return nil, err
	}
	temp, hash, err := mintTemporaryPassword()
	if err != nil {
		return nil, fmt.Errorf("operator reset mfa: %w", err)
	}
	now := o.clock()
	var subject string
	err = o.store.InTx(ctx, func(tx OperatorTx) error {
		acc, err := byCode(ctx, tx, code)
		if err != nil {
			return err
		}
		subject = acc.Code
		if err := tx.ResetMFA(ctx, acc.ID, now); err != nil {
			return err
		}
		if err := tx.SetPassword(ctx, acc.ID, hash, true, now); err != nil {
			return err
		}
		if err := tx.AppendAudit(ctx, domain.OperatorAuditEntry{
			OccurredAt: now, Actor: domain.SystemActor, Action: domain.OperatorAuditMFAReset, Subject: acc.Code,
			Before: map[string]any{"totp_enrolled": acc.TOTPEnrolled()},
			After:  map[string]any{"totp_enrolled": false, "recovery_codes_voided": true, "sessions_revoked": true},
			Reason: reason,
		}); err != nil {
			return err
		}
		return tx.AppendAudit(ctx, domain.OperatorAuditEntry{
			OccurredAt: now, Actor: domain.SystemActor, Action: domain.OperatorAuditPasswordChanged,
			Subject: acc.Code, After: map[string]any{"must_change_password": true}, Reason: reason,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("operator reset mfa: %w", err)
	}
	o.adminLog("operator.mfa_reset", subject, reason)
	return temp, nil
}

// Operator statuses shown by List, most severe first.
const (
	OperatorStatusDisabled           = "disabled"
	OperatorStatusLocked             = "locked"
	OperatorStatusEnrollmentRequired = "enrollment_required"
	OperatorStatusActive             = "active"
)

// OperatorListing is one line of the CLI's `list`. The email is MASKED (core/privacy.MaskEmail):
// the listing is printed on a terminal and lands in shell scrollback and screenshots.
type OperatorListing struct {
	Code        string
	MaskedEmail string
	DisplayName string
	Status      string
	Permissions []domain.OperatorPermission
}

// List reads every operator with its live permissions. A read: no audit entry, no ticket.
func (o *OperatorAdmin) List(ctx context.Context) ([]OperatorListing, error) {
	accounts, err := o.store.ListAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("operator list: %w", err)
	}
	now := o.clock()
	out := make([]OperatorListing, 0, len(accounts))
	for _, acc := range accounts {
		perms, err := o.store.ActivePermissions(ctx, acc.ID)
		if err != nil {
			return nil, fmt.Errorf("operator list: %w", err)
		}
		status := OperatorStatusActive
		switch {
		case acc.Disabled():
			status = OperatorStatusDisabled
		case acc.LockedAt(now):
			status = OperatorStatusLocked
		case acc.MustChangePassword || !acc.TOTPEnrolled():
			status = OperatorStatusEnrollmentRequired
		}
		out = append(out, OperatorListing{
			Code: acc.Code, MaskedEmail: privacy.MaskEmail(acc.Email), DisplayName: acc.DisplayName,
			Status: status, Permissions: perms,
		})
	}
	return out, nil
}
