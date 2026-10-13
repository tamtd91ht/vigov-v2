package app

// The OWNER's management of operator accounts — the four owner-only OperatorService RPCs without a
// commune (operator.proto §"THE OWNER-ONLY RPCs"; ADR 0048 §"Sửa đổi 13/10/2026 — owner, …" #2–#4,
// #6). Who may call them is decided in internal/grpc (the principal identity resolved from the `op1.`
// token, is_owner read live); these use cases take that owner as an already-checked MiniAppOperator
// and attribute every write to its VH- code (rule 6, invariant 8 — no fallback to the internal id).
//
// SAME STORE, SAME STATEMENTS AS operatorctl (operator_admin.go): CreateAccount, Grant, Revoke,
// SetPassword, Unlock — so a key granted from the screen and a key granted by the CLI are the same
// row and the same trail entry, differing only in the actor (VH- code + the owner's reason, versus
// "system" + "ticket:<n>").
//
// PASSWORDS: the owner TYPES the temporary password (operator.proto). It is checked against the
// staff policy (domain.KiemTraMatKhauMoi — the one the operator realm already reuses), hashed with
// argon2id before any transaction opens, and from then on exists only as that hash. It never reaches
// a log line, an error, a trail entry or a return value.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/vihat/vigov/core/password"
	"github.com/vihat/vigov/service-identity/internal/domain"
	"github.com/vihat/vigov/service-identity/internal/store/operatorstore"
)

// Refusals of the owner's operator-account acts. Each maps to one status in internal/grpc.
var (
	// ErrOwnerReasonInvalid — `reason` outside the contract's shape (1-500 runes, no control
	// character but a line break). INVALID_ARGUMENT, decided before any lookup.
	ErrOwnerReasonInvalid = errors.New("owner admin: reason must be 1-500 characters, no control characters but line breaks")
	// ErrOperatorEmailTaken — CreateOperatorAccount on an email an account already has (case-folded,
	// operator_account_email_unique). ALREADY_EXISTS.
	ErrOperatorEmailTaken = errors.New("owner admin: an operator account already has this email")
	// ErrOwnerTargetIsOwner — SetOperatorPermissions aimed at the owner. The owner holds every right
	// by the flag (ADR 0048 amendment #2), so a key set on that account would be a second, contradicting
	// source of what the owner may do. FAILED_PRECONDITION.
	ErrOwnerTargetIsOwner = errors.New("owner admin: the owner's rights come from the owner flag, not from keys")
	// ErrOwnerTargetIsSelf — ResetOperatorPassword aimed at the caller. A reset does not ask for the
	// current password; the self path is ChangeOperatorPassword, which re-proves both factors — the
	// staff side's #14 (tai_khoan_can_bo.go DatLai) for the same reason. FAILED_PRECONDITION.
	ErrOwnerTargetIsSelf = errors.New("owner admin: change your own password with ChangeOperatorPassword")
)

// OperatorAccountView is one operator account with its live keys, as the owner's screen shows it.
// The account's Email and DisplayName are personal data (rule 3): the grpc layer puts them on the
// wire and nowhere else.
type OperatorAccountView struct {
	Account     domain.OperatorAccount
	Permissions []domain.OperatorPermission
}

// OwnerAdmin is the owner's operator-account use cases.
type OwnerAdmin struct {
	store OperatorStore
	now   func() time.Time
	log   *slog.Logger // the SECURITY log: events and codes, never values
	hash  func(string) (string, error)
}

// NewOwnerAdmin builds the use cases. store and log are required.
func NewOwnerAdmin(store OperatorStore, now func() time.Time, log *slog.Logger) *OwnerAdmin {
	if store == nil || log == nil {
		panic("app.NewOwnerAdmin: store and log are required")
	}
	if now == nil {
		now = time.Now
	}
	return &OwnerAdmin{store: store, now: now, log: log, hash: password.Bam}
}

func (o *OwnerAdmin) clock() time.Time { return o.now().UTC() }

// CheckOwnerReason is the contract's `reason` rule (operatorReason, shared with the Mini App RPCs).
func CheckOwnerReason(raw string) (string, error) {
	r, ok := operatorReason(raw)
	if !ok {
		return "", ErrOwnerReasonInvalid
	}
	return r, nil
}

// CheckOperatorPermissionKeys parses every key against the realm's closed list, folds duplicates
// and sorts. Any key off the list refuses the whole request (INVALID_ARGUMENT): granting "the valid
// half" of what the owner ticked would be a state nobody chose.
func CheckOperatorPermissionKeys(raw []string) ([]domain.OperatorPermission, error) {
	seen := make(map[domain.OperatorPermission]bool, len(raw))
	out := make([]domain.OperatorPermission, 0, len(raw))
	for _, k := range raw {
		p, err := domain.ParseOperatorPermission(k)
		if err != nil {
			return nil, err
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

// CheckNewOperatorShape is CreateOperatorAccount's shape: the email and name rules operatorctl's
// `create` applies (validEmail, 1-200 runes), plus the keys. Returns the trimmed values.
func CheckNewOperatorShape(email, displayName string, keys []string) (string, string, []domain.OperatorPermission, error) {
	email, displayName = strings.TrimSpace(email), strings.TrimSpace(displayName)
	if !validEmail(email) {
		return "", "", nil, ErrInvalidOperatorEmail
	}
	if displayName == "" || !utf8.ValidString(displayName) || utf8.RuneCountInString(displayName) > 200 {
		return "", "", nil, ErrInvalidOperatorName
	}
	perms, err := CheckOperatorPermissionKeys(keys)
	if err != nil {
		return "", "", nil, err
	}
	return email, displayName, perms, nil
}

// hashTemporary applies the password policy, then hashes — both before any transaction.
func (o *OwnerAdmin) hashTemporary(temp string) (string, error) {
	if err := domain.KiemTraMatKhauMoi(temp); err != nil {
		return "", err
	}
	h, err := o.hash(temp)
	if err != nil {
		return "", fmt.Errorf("owner admin: hash temporary password: %w", err)
	}
	return h, nil
}

// ListAccounts reads every operator account, disabled ones included, with its live keys, ordered by
// code. NOT AUDITED (operator.proto, ListOperatorAccounts): the realm's own roster read by the one
// person who manages it, the same reasoning as operatorctl's `list`.
func (o *OwnerAdmin) ListAccounts(ctx context.Context) ([]OperatorAccountView, error) {
	accounts, err := o.store.ListAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("owner list operators: %w", err)
	}
	out := make([]OperatorAccountView, 0, len(accounts))
	for _, a := range accounts {
		perms, err := o.store.ActivePermissions(ctx, a.ID)
		if err != nil {
			return nil, fmt.Errorf("owner list operators: %w", err)
		}
		out = append(out, OperatorAccountView{Account: a, Permissions: perms})
	}
	return out, nil
}

// NewOperatorByOwner is CreateAccount's input, already shape-checked by CheckNewOperatorShape.
type NewOperatorByOwner struct {
	Email             string
	DisplayName       string
	TemporaryPassword string // SECRET — hashed here, never stored or logged
	Permissions       []domain.OperatorPermission
}

// CreateAccount creates an operator with the owner's temporary password (must_change_password,
// the 24-hour expiry, no second factor — the first sign-in enrols TOTP exactly as after
// `operatorctl create`, ADR 0048 amendment #3) and grants the given keys: ONE transaction with one
// account_created entry and one permission_granted entry per key, all by the owner's code.
func (o *OwnerAdmin) CreateAccount(ctx context.Context, owner MiniAppOperator, in NewOperatorByOwner) (OperatorAccountView, error) {
	actor, err := operatorActor(owner)
	if err != nil {
		return OperatorAccountView{}, err
	}
	hash, err := o.hashTemporary(in.TemporaryPassword)
	if err != nil {
		return OperatorAccountView{}, err
	}
	now := o.clock()
	var view OperatorAccountView
	err = o.store.InTx(ctx, func(tx OperatorTx) error {
		acc, err := tx.CreateAccount(ctx, operatorstore.NewAccount{
			Email: in.Email, DisplayName: in.DisplayName, PasswordHash: hash, CreatedBy: actor.code,
		}, now)
		if errors.Is(err, operatorstore.ErrEmailTaken) {
			return ErrOperatorEmailTaken
		}
		if err != nil {
			return err
		}
		// No email, no name in the entry (rule 6, forbidden #4) — the code is the operator.
		if err := tx.AppendAudit(ctx, domain.OperatorAuditEntry{
			OccurredAt: now, Actor: actor.code, ActorIP: actor.ip, Action: domain.OperatorAuditAccountCreated,
			Subject: acc.Code, After: map[string]any{"must_change_password": true},
		}); err != nil {
			return err
		}
		for _, k := range in.Permissions {
			// grant_reason is NOT NULL-and-not-blank in the store; the act's own words are the reason.
			if err := tx.Grant(ctx, acc.ID, k, actor.code, "granted at account creation", now); err != nil {
				return err
			}
			if err := tx.AppendAudit(ctx, domain.OperatorAuditEntry{
				OccurredAt: now, Actor: actor.code, ActorIP: actor.ip, Action: domain.OperatorAuditPermissionGranted,
				Subject: acc.Code, After: map[string]any{"permission_key": string(k)},
			}); err != nil {
				return err
			}
		}
		perms, err := tx.ActivePermissions(ctx, acc.ID)
		if err != nil {
			return err
		}
		view = OperatorAccountView{Account: acc, Permissions: perms}
		return nil
	})
	if err != nil {
		return OperatorAccountView{}, fmt.Errorf("owner create operator: %w", err)
	}
	o.ownerLog("operator.account_created", actor, view.Account.Code, "permissions_granted", len(in.Permissions))
	return view, nil
}

// SetPermissions makes keys the COMPLETE live set of the target: missing keys revoked (the grant row
// stamped revoked_*, kept as history), new keys granted, one entry per key changed, one transaction.
// Every change revokes every session of the target (the store's Grant/Revoke do it in this
// transaction — ADR 0048 §Chốt bước 1: "đổi quyền → thu hồi mọi phiên"). The same set twice is no
// change: nothing written, no entry.
func (o *OwnerAdmin) SetPermissions(ctx context.Context, owner MiniAppOperator, targetCode string,
	keys []domain.OperatorPermission, reason string) (OperatorAccountView, error) {
	actor, err := operatorActor(owner)
	if err != nil {
		return OperatorAccountView{}, err
	}
	if reason, err = CheckOwnerReason(reason); err != nil {
		return OperatorAccountView{}, err
	}
	want := make(map[domain.OperatorPermission]bool, len(keys))
	for _, k := range keys {
		if !k.Valid() {
			return OperatorAccountView{}, fmt.Errorf("%w: %q", domain.ErrUnknownOperatorPermission, k)
		}
		want[k] = true
	}
	now := o.clock()
	var view OperatorAccountView
	var granted, revoked int
	err = o.store.InTx(ctx, func(tx OperatorTx) error {
		granted, revoked = 0, 0
		acc, err := byCode(ctx, tx, targetCode)
		if err != nil {
			return err
		}
		if acc.IsOwner {
			return ErrOwnerTargetIsOwner
		}
		current, err := tx.ActivePermissions(ctx, acc.ID)
		if err != nil {
			return err
		}
		have := make(map[domain.OperatorPermission]bool, len(current))
		for _, k := range current {
			have[k] = true
			if want[k] {
				continue
			}
			if err := tx.Revoke(ctx, acc.ID, k, actor.code, reason, now); err != nil {
				return err
			}
			if err := tx.AppendAudit(ctx, domain.OperatorAuditEntry{
				OccurredAt: now, Actor: actor.code, ActorIP: actor.ip, Action: domain.OperatorAuditPermissionRevoked,
				Subject: acc.Code, Before: map[string]any{"permission_key": string(k)}, Reason: reason,
			}); err != nil {
				return err
			}
			revoked++
		}
		for _, k := range keys {
			if have[k] {
				continue
			}
			if err := tx.Grant(ctx, acc.ID, k, actor.code, reason, now); err != nil {
				return err
			}
			if err := tx.AppendAudit(ctx, domain.OperatorAuditEntry{
				OccurredAt: now, Actor: actor.code, ActorIP: actor.ip, Action: domain.OperatorAuditPermissionGranted,
				Subject: acc.Code, After: map[string]any{"permission_key": string(k)}, Reason: reason,
			}); err != nil {
				return err
			}
			have[k] = true // a key listed twice is one key (operator.proto: "Duplicates are one key")
			granted++
		}
		perms, err := tx.ActivePermissions(ctx, acc.ID)
		if err != nil {
			return err
		}
		view = OperatorAccountView{Account: acc, Permissions: perms}
		return nil
	})
	if err != nil {
		return OperatorAccountView{}, fmt.Errorf("owner set operator permissions: %w", err)
	}
	if granted+revoked > 0 {
		o.ownerLog("operator.permissions_set", actor, view.Account.Code, "granted", granted, "revoked", revoked)
	}
	return view, nil
}

// ResetPassword gives the target a TEMPORARY password typed by the owner: must_change_password, the
// 24-hour expiry, every session revoked (SetPassword does it in this transaction), the failure
// lockout and count CLEARED, the second factor and recovery codes KEPT (ADR 0048 amendment #6), one
// operator.password_reset entry. Refused on the caller's own account (ErrOwnerTargetIsSelf).
//
// THE LOCKOUT IS CLEARED ON PURPOSE (task decision 13/10/2026): a person who asks the owner for a new
// password is very often the person who locked themselves out guessing the old one; a reset that left
// a 12-hour lock in force would hand over a password that cannot be used until tomorrow. The entry
// records the lock state before, so an inspection sees that a lock was lifted and by whom.
func (o *OwnerAdmin) ResetPassword(ctx context.Context, owner MiniAppOperator, targetCode, temp, reason string) (OperatorAccountView, error) {
	actor, err := operatorActor(owner)
	if err != nil {
		return OperatorAccountView{}, err
	}
	if reason, err = CheckOwnerReason(reason); err != nil {
		return OperatorAccountView{}, err
	}
	hash, err := o.hashTemporary(temp)
	if err != nil {
		return OperatorAccountView{}, err
	}
	now := o.clock()
	var view OperatorAccountView
	err = o.store.InTx(ctx, func(tx OperatorTx) error {
		acc, err := byCode(ctx, tx, targetCode)
		if err != nil {
			return err
		}
		if acc.Code == actor.code {
			return ErrOwnerTargetIsSelf
		}
		locked := acc.LockedAt(now)
		if err := tx.SetPassword(ctx, acc.ID, hash, true, now); err != nil {
			return err
		}
		if err := tx.Unlock(ctx, acc.ID, now); err != nil {
			return err
		}
		if err := tx.AppendAudit(ctx, domain.OperatorAuditEntry{
			OccurredAt: now, Actor: actor.code, ActorIP: actor.ip, Action: domain.OperatorAuditPasswordReset,
			Subject: acc.Code,
			Before:  map[string]any{"locked": locked, "failed_attempts": acc.FailedAttempts},
			After: map[string]any{"must_change_password": true, "sessions_revoked": true, "locked": false,
				"failed_attempts": 0, "totp_kept": acc.TOTPEnrolled()},
			Reason: reason,
		}); err != nil {
			return err
		}
		after, err := tx.ByID(ctx, acc.ID)
		if err != nil {
			return err
		}
		perms, err := tx.ActivePermissions(ctx, acc.ID)
		if err != nil {
			return err
		}
		view = OperatorAccountView{Account: after, Permissions: perms}
		return nil
	})
	if err != nil {
		return OperatorAccountView{}, fmt.Errorf("owner reset operator password: %w", err)
	}
	o.ownerLog("operator.password_reset", actor, view.Account.Code)
	return view, nil
}

// ownerLog is the security-log line of an owner act (skills/security-logging): event, actor code,
// subject code, IP — never an email, a name, a reason text or a password.
func (o *OwnerAdmin) ownerLog(event string, actor changeActor, subject string, extra ...any) {
	args := append([]any{"event", event, "outcome", "success", "actor", actor.code,
		"subject", subject, "ip", actor.ip}, extra...)
	o.log.Info("operator owner act", args...)
}
