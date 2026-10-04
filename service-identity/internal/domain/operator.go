package domain

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

// The Vihat operator realm (ADR 0048 §Chốt của chủ dự án — 28/09/2026).
//
// AN OPERATOR IS NOT A STAFF MEMBER OF ANY COMMUNE, AND NOTHING HERE MAY BORROW FROM THAT MODEL.
// A staff member acts inside ONE commune, carries a tenant_id, holds rights through roles in the
// per-commune `quyen` table, and is audited in audit_log under that commune. An operator touches
// PLATFORM metadata for every commune — domains, profiles, upload limits — so every one of those
// four properties is deliberately different here: no tenant_id, a closed list of `ops.*` keys
// granted straight to the account, a separate session registry, a separate trail. Mixing the two
// would give a cross-commune right a seat in a commune's own table (rule 5, forbidden #2).

// OperatorPermission is one right of the operator realm.
//
// NOT authz.Perm, AND NOT ANY TYPE NAMED `Perm`. tools/quyen_keys.py demands that every such key
// exist in the per-commune `quyen` table (rule 5, invariant 3c), and these keys must NEVER be
// there: `quyen` is a commune's register, and a key in it is a key a commune administrator could
// grant. The `ops.` prefix guarantees an operator key can never collide with a commune key.
type OperatorPermission string

// THE CLOSED LIST decided by the owner on 2026-09-28, grown to seven on 2026-10-04 (ADR 0073 #3:
// `ops.petition_field.manage`, migration 0023). Adding a key is ADR 0048 stop condition #1
// (authority beyond one commune) — an owner decision, never a line added here in passing. The
// migration's CHECK on operator_permission_grant.permission_key is the database's copy of this
// list; internal/store/operatorstore tests compare the two.
const (
	OperatorPermissionTenantManage       OperatorPermission = "ops.tenant.manage"
	OperatorPermissionDomainManage       OperatorPermission = "ops.domain.manage"
	OperatorPermissionProfileManage      OperatorPermission = "ops.profile.manage"
	OperatorPermissionMiniAppManage      OperatorPermission = "ops.mini_app.manage"
	OperatorPermissionUploadPolicyManage OperatorPermission = "ops.upload_policy.manage"
	OperatorPermissionQRIssue            OperatorPermission = "ops.qr.issue"
	// The tier-1 petition field codes (ADR 0060), edited in the operator area (ADR 0073 #3).
	OperatorPermissionPetitionFieldManage OperatorPermission = "ops.petition_field.manage"
)

// OperatorPermissions returns the closed list, in a stable order. A fresh slice every call, so no
// caller can append to the list the rest of the process reads.
func OperatorPermissions() []OperatorPermission {
	return []OperatorPermission{
		OperatorPermissionTenantManage,
		OperatorPermissionDomainManage,
		OperatorPermissionProfileManage,
		OperatorPermissionMiniAppManage,
		OperatorPermissionUploadPolicyManage,
		OperatorPermissionQRIssue,
		OperatorPermissionPetitionFieldManage,
	}
}

// Valid reports whether p is on the closed list. EXACT match: no trimming, no case folding — a key
// that only matches after normalising is a key somebody typed differently, and the grant table
// would then hold a spelling no route checks for.
func (p OperatorPermission) Valid() bool {
	for _, k := range OperatorPermissions() {
		if p == k {
			return true
		}
	}
	return false
}

// ErrUnknownOperatorPermission is returned for any key off the closed list.
var ErrUnknownOperatorPermission = errors.New("operator: unknown permission key")

// ParseOperatorPermission converts an input string into a permission, refusing anything off the list.
func ParseOperatorPermission(s string) (OperatorPermission, error) {
	p := OperatorPermission(s)
	if !p.Valid() {
		return "", fmt.Errorf("%w: %q", ErrUnknownOperatorPermission, s)
	}
	return p, nil
}

// operatorCodePrefix — "ViHAT". A different prefix from the staff code (`CB-…`) so that the "who"
// column of any trail tells the realm at a glance (rule 6, invariant 8; ADR 0048 §Thiết kế #6).
const operatorCodePrefix = "VH-"

// FormatOperatorCode renders the business code of the n-th operator: `VH-00001`.
//
// n COMES FROM THE PLATFORM-WIDE SEQUENCE operator_code_seq AND NOTHING ELSE. A sequence never
// hands back a value, even when the transaction that drew it rolls back, so a code is issued once
// and never reissued (rule 7, invariant 3) — unlike MAX(code)+1, which reissues the code of the
// newest row the moment that row stops being counted. The UNIQUE key on operator_account.code is
// the second half of the guarantee.
//
// FIVE DIGITS IS A MINIMUM, NOT A WIDTH: past 99999 the number simply grows (`VH-100000`). A
// fixed-width format would either truncate — two operators, one code — or refuse the 100000th
// account; both are worse than a longer string. The migration's CHECK is `^VH-[0-9]{5,}$` for
// the same reason.
func FormatOperatorCode(n int64) (string, error) {
	if n < 1 {
		// A sequence starting at 1 never yields this. Seeing it means the caller did not read the
		// sequence, and a code minted from anything else can collide with one already issued.
		return "", fmt.Errorf("operator: code number must be positive, got %d", n)
	}
	return fmt.Sprintf("%s%05d", operatorCodePrefix, n), nil
}

// ValidOperatorCode reports whether s has the shape FormatOperatorCode produces.
func ValidOperatorCode(s string) bool {
	rest, ok := strings.CutPrefix(s, operatorCodePrefix)
	if !ok || len(rest) < 5 {
		return false
	}
	for _, r := range rest {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// SystemActor is the audit "who" of an act no person performed at a screen: the server-side CLI
// that creates the first operator and performs every grant, revoke, lock and MFA reset (owner's
// decision 2026-09-28). The ticket number that authorised it goes in the entry's reason (rule 6,
// invariant 6).
const SystemActor = "system"

// The security figures decided by the owner on 2026-09-28. Platform-wide constants, not
// per-commune configuration: the operator realm belongs to no commune (rule 1, invariant 10 does
// not apply).
const (
	// SessionLifetime — 8 hours ABSOLUTE, no refresh. One working day. An operator session reaches
	// every commune's platform metadata, so it is not renewed by activity: a stolen cookie is good
	// for at most one day, and a forgotten browser tab signs itself out overnight. This is a
	// security lifetime, not an administrative deadline — rule 10's working-hours clock does not
	// apply to it.
	SessionLifetime = 8 * time.Hour

	// MaxFailedAttempts — 5 CONSECUTIVE failures, password and TOTP counted together, lock the
	// account. Counted together because a split counter would give an attacker who already holds
	// the password five free TOTP guesses per lockout window on top of the password guesses.
	MaxFailedAttempts = 5

	// LockoutDuration — 12 HOURS (owner's decision 28/09/2026, TASK-04, replacing the 15 minutes of
	// ADR 0048 §"Chốt bước 1"; TCVN 14423:2026 §5.5.2.2). Five guesses per half-day makes online
	// guessing of a password + 6-digit TOTP hopeless. The cost is deliberate: an operator who
	// mistyped five times waits, or asks devops for `operatorctl unlock` — a ticketed, audited act,
	// which is exactly the human check an account that reaches every commune should get.
	LockoutDuration = 12 * time.Hour

	// SessionIdleTimeout — 5 MINUTES without a request ends the session (owner's decision
	// 28/09/2026, TASK-04; TCVN 14423 caps an ADMINISTRATIVE session's idle time at 5 minutes). It
	// sits UNDER SessionLifetime, never instead of it: activity keeps a session alive for at most 8
	// hours in total. Enforced in operatorstore's checkSession SQL against last_seen_at, which every
	// successful ResolveSession moves forward.
	SessionIdleTimeout = 5 * time.Minute

	// TemporaryPasswordLifetime — 24 HOURS from issue (CreateOperator, ResetMFA) for the operator to
	// sign in, choose their own password and enrol TOTP (owner's decision 28/09/2026, TASK-04). A
	// temporary password travels by hand — chat, paper, phone — so it must stop working on its own
	// soon after the person it was meant for had their chance. Expired → the uniform refusal; devops
	// reissues with `operatorctl reset-mfa`.
	TemporaryPasswordLifetime = 24 * time.Hour

	// PendingTOTPLifetime — 10 MINUTES between BeginEnrollment and CompleteEnrollment (owner's
	// decision 28/09/2026, TASK-04). A QR code left on a screen, or a provisioning URI in a
	// screenshot, stops being bindable soon after it is shown. Expired → start the enrolment again.
	PendingTOTPLifetime = 10 * time.Minute

	// RecoveryCodeCount — 10 single-use codes per batch. Regenerating voids the whole previous
	// batch, so there is never more than one live set to lose.
	RecoveryCodeCount = 10
)

// OperatorAccount is one Vihat operator. No tenant_id, by decision (ADR 0048 §Chốt #1).
//
// NO SECRET FIELD IS HERE. The password hash and the sealed TOTP secret stay in the store layer
// and are handed only to the code that verifies them, so a `%+v` of an account can never print a
// credential. Email and DisplayName ARE personal data (rule 3): never log them, never put them in
// an error or an audit entry — the business code is what identifies an operator everywhere.
type OperatorAccount struct {
	ID                 string
	Code               string
	Email              string
	DisplayName        string
	MustChangePassword bool
	TOTPEnrolledAt     *time.Time
	TOTPPending        bool
	FailedAttempts     int
	LockedUntil        *time.Time
	DisabledAt         *time.Time
	DisabledBy         string
	DisabledReason     string
	CreatedAt          time.Time
	CreatedBy          string
	UpdatedAt          time.Time

	// TemporaryPasswordExpiresAt is when the temporary password stops working; set whenever
	// MustChangePassword is. PendingTOTPCreatedAt is when BeginEnrollment stored the pending secret.
	TemporaryPasswordExpiresAt *time.Time
	PendingTOTPCreatedAt       *time.Time
}

// TemporaryPasswordExpiredAt reports whether the account's password is a temporary one that can no
// longer be used at now. NIL COUNTS AS EXPIRED: a temporary password with no recorded expiry is one
// that would work forever, and "no value" must never be the permissive answer on a security path
// (migration 0014's CHECK makes the state unreachable; this makes it harmless if it is reached).
func (a OperatorAccount) TemporaryPasswordExpiredAt(now time.Time) bool {
	if !a.MustChangePassword {
		return false
	}
	return a.TemporaryPasswordExpiresAt == nil || !now.Before(*a.TemporaryPasswordExpiresAt)
}

// PendingTOTPExpiredAt reports whether the pending enrolment secret can no longer be completed at
// now. No pending secret, or no recorded creation time, counts as expired (same reason).
func (a OperatorAccount) PendingTOTPExpiredAt(now time.Time) bool {
	if !a.TOTPPending || a.PendingTOTPCreatedAt == nil {
		return true
	}
	return !now.Before(a.PendingTOTPCreatedAt.Add(PendingTOTPLifetime))
}

// String renders an account WITHOUT its personal fields (rule 3; security review of TASK-03,
// finding 3): only the id, the business code and state flags, then a withheld marker. The fields are
// exported because the store fills them and the CLI lists them, so fmt would print them by
// reflection on any `%+v` — and slog would too. Format, GoString and LogValue close every path;
// Format is the load-bearing one (fmt consults String only for some verbs, so a `%d` would
// otherwise walk the fields).
func (a OperatorAccount) String() string {
	return "domain.OperatorAccount{id:" + a.ID + " code:" + a.Code +
		" must_change_password:" + strconv.FormatBool(a.MustChangePassword) +
		" totp_enrolled:" + strconv.FormatBool(a.TOTPEnrolled()) +
		" totp_pending:" + strconv.FormatBool(a.TOTPPending) +
		" failed_attempts:" + strconv.Itoa(a.FailedAttempts) +
		" locked:" + strconv.FormatBool(a.LockedUntil != nil) +
		" disabled:" + strconv.FormatBool(a.Disabled()) + " personal_fields:withheld}"
}

// GoString keeps %#v from printing the personal fields.
func (a OperatorAccount) GoString() string { return a.String() }

// Format renders String on every verb.
func (a OperatorAccount) Format(f fmt.State, verb rune) {
	if verb == 'q' {
		_, _ = io.WriteString(f, strconv.Quote(a.String()))
		return
	}
	_, _ = io.WriteString(f, a.String())
}

// LogValue keeps slog from printing the personal fields.
func (a OperatorAccount) LogValue() slog.Value { return slog.StringValue(a.String()) }

// Disabled reports an administrative lock (rule 7: soft — the row and its history stay).
func (a OperatorAccount) Disabled() bool { return a.DisabledAt != nil }

// LockedAt reports whether the failure lockout is in force at now. The lock EXPIRES by itself: it
// is a comparison against a stored instant, never a flag somebody must remember to clear.
func (a OperatorAccount) LockedAt(now time.Time) bool {
	return a.LockedUntil != nil && now.Before(*a.LockedUntil)
}

// TOTPEnrolled reports whether the second factor is active.
func (a OperatorAccount) TOTPEnrolled() bool { return a.TOTPEnrolledAt != nil }

// OperatorAuditAction is the verb of one entry in operator_audit_log.
type OperatorAuditAction string

// The events the owner listed on 2026-09-28, plus RecoveryCodesRegenerated — see its note.
const (
	OperatorAuditAccountCreated    OperatorAuditAction = "operator.account_created"
	OperatorAuditLoginSucceeded    OperatorAuditAction = "operator.login_succeeded"
	OperatorAuditLoginLockedOut    OperatorAuditAction = "operator.login_locked_out"
	OperatorAuditLoggedOut         OperatorAuditAction = "operator.logged_out"
	OperatorAuditTOTPEnrolled      OperatorAuditAction = "operator.totp_enrolled"
	OperatorAuditRecoveryCodeUsed  OperatorAuditAction = "operator.recovery_code_used"
	OperatorAuditPermissionGranted OperatorAuditAction = "operator.permission_granted"
	OperatorAuditPermissionRevoked OperatorAuditAction = "operator.permission_revoked"
	OperatorAuditAccountDisabled   OperatorAuditAction = "operator.account_disabled"
	OperatorAuditAccountEnabled    OperatorAuditAction = "operator.account_enabled"
	OperatorAuditMFAReset          OperatorAuditAction = "operator.mfa_reset"
	OperatorAuditPasswordChanged   OperatorAuditAction = "operator.password_changed"
	// RecoveryCodesRegenerated is NOT on the owner's list. It is here because regenerating voids
	// every live code of the previous batch — a write to credential state — and rule 6, invariant 1
	// asks every write to leave an entry. Stated as an assumption in the TASK-02 report.
	OperatorAuditRecoveryCodesRegenerated OperatorAuditAction = "operator.recovery_codes_regenerated"
	// Owner's decisions 28/09/2026 (TASK-04): lifting a failure lockout by CLI, and the start of an
	// enrolment (a pending secret stored — a write to credential state, audited like the rest).
	OperatorAuditUnlocked              OperatorAuditAction = "operator.unlocked"
	OperatorAuditTOTPEnrollmentStarted OperatorAuditAction = "operator.totp_enrollment_started"
)

// Valid reports whether a is one of the declared actions. The database only checks the SHAPE of
// the verb; the closed list lives here so that adding an event is a code change, reviewed.
func (a OperatorAuditAction) Valid() bool {
	switch a {
	case OperatorAuditAccountCreated, OperatorAuditLoginSucceeded, OperatorAuditLoginLockedOut,
		OperatorAuditLoggedOut, OperatorAuditTOTPEnrolled, OperatorAuditRecoveryCodeUsed,
		OperatorAuditPermissionGranted, OperatorAuditPermissionRevoked, OperatorAuditAccountDisabled,
		OperatorAuditAccountEnabled, OperatorAuditMFAReset, OperatorAuditPasswordChanged,
		OperatorAuditRecoveryCodesRegenerated, OperatorAuditUnlocked, OperatorAuditTOTPEnrollmentStarted:
		return true
	}
	return false
}

// OperatorAuditEntry is one row of operator_audit_log.
//
// Actor is a BUSINESS CODE (`VH-00001`) or SystemActor — never an internal id (rule 6, invariant
// 8). Subject is the operator code the act was about. Before/After hold significant fields ONLY,
// and never personal data: no email, no display name, no secret (rule 6, forbidden #4).
type OperatorAuditEntry struct {
	OccurredAt time.Time
	Actor      string
	ActorIP    string
	Action     OperatorAuditAction
	Subject    string
	Before     map[string]any
	After      map[string]any
	Reason     string
}

// Audit entry refusals. Returned BEFORE anything is written, so the business change in the same
// transaction rolls back with it — an act that cannot be attributed does not happen.
var (
	ErrOperatorAuditActor   = errors.New("operator audit: actor must be an operator code or \"system\"")
	ErrOperatorAuditAction  = errors.New("operator audit: unknown action")
	ErrOperatorAuditSubject = errors.New("operator audit: subject must be an operator code")
	ErrOperatorAuditReason  = errors.New("operator audit: a system act must carry its reason (ticket number)")
)

// Validate checks the entry. NO FALLBACK: an empty actor is refused, never replaced by "system"
// (rule 6, invariant 8) — a system act that nobody declared as one is exactly the entry an
// inspection cannot defend.
func (e OperatorAuditEntry) Validate() error {
	if e.Actor != SystemActor && !ValidOperatorCode(e.Actor) {
		return ErrOperatorAuditActor
	}
	if !e.Action.Valid() {
		return fmt.Errorf("%w: %q", ErrOperatorAuditAction, e.Action)
	}
	if !ValidOperatorCode(e.Subject) {
		return ErrOperatorAuditSubject
	}
	// The owner's decision: the CLI acts as "system" WITH the ticket number in the reason. Without
	// it, "system" says nobody.
	if e.Actor == SystemActor && strings.TrimSpace(e.Reason) == "" {
		return ErrOperatorAuditReason
	}
	return nil
}
