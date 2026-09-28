package operatorstore

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// Errors. Each names the operation's outcome, never the row — an error travels into logs, and the
// row is an operator's personal data (rule 3, forbidden #3).
var (
	ErrNoDB              = errors.New("operatorstore: no database handle")
	ErrNotFound          = errors.New("operatorstore: not found")
	ErrStateConflict     = errors.New("operatorstore: the account is not in the state this change requires")
	ErrAccountLocked     = errors.New("operatorstore: account is locked after repeated failures")
	ErrEmailTaken        = errors.New("operatorstore: an operator account with this email already exists")
	ErrAlreadyGranted    = errors.New("operatorstore: permission already granted")
	ErrNotGranted        = errors.New("operatorstore: permission is not currently granted")
	ErrRecoveryCodeSet   = errors.New("operatorstore: a recovery batch must hold exactly domain.RecoveryCodeCount distinct codes")
	ErrMissingField      = errors.New("operatorstore: required field missing")
	ErrActorShape        = errors.New("operatorstore: actor must be an operator code or \"system\"")
	ErrUnknownPermission = domain.ErrUnknownOperatorPermission
)

// Revoke reasons written by the invariants this package enforces itself.
const (
	RevokeReasonPasswordChanged = "password_changed"
	RevokeReasonDisabled        = "account_disabled"
	RevokeReasonGrantChanged    = "permission_changed"
	RevokeReasonMFAReset        = "mfa_reset"
)

// querier is what both *sql.DB and *sql.Tx offer. Reads are written once against it and exposed on
// both Store (outside a transaction) and Tx (inside one).
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Store is the operator realm's store. It holds the raw handle and never gives it away.
type Store struct{ db *sql.DB }

// New returns a Store over db. Unscoped by design — see doc.go.
func New(db *sql.DB) *Store { return &Store{db: db} }

// Tx is one transaction over the operator tables. Every write lives here so that the business
// change and its audit entry share the transaction (rule 6, invariant 3).
type Tx struct{ tx *sql.Tx }

// InTx runs fn in one transaction: commit when fn returns nil, roll back otherwise.
func (s *Store) InTx(ctx context.Context, fn func(tx *Tx) error) error {
	if s == nil || s.db == nil {
		return ErrNoDB
	}
	sqlTx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("operatorstore: begin: %w", err)
	}
	if err := fn(&Tx{tx: sqlTx}); err != nil {
		if rbErr := sqlTx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			return errors.Join(err, fmt.Errorf("operatorstore: rollback: %w", rbErr))
		}
		return err
	}
	if err := sqlTx.Commit(); err != nil {
		return fmt.Errorf("operatorstore: commit: %w", err)
	}
	return nil
}

// hashSecret is the SHA-256 hex of the session id. Done HERE so that no caller can forget it: the
// raw sid never reaches a statement. (Recovery codes arrive already hashed — ReplaceRecoveryCodes.)
func hashSecret(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func validActor(a string) bool { return a == domain.SystemActor || domain.ValidOperatorCode(a) }

// uniqueViolation returns the constraint name of a unique violation, or "" for any other error.
func uniqueViolation(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return pgErr.ConstraintName
	}
	return ""
}

func requireOne(res sql.Result, op string, none error) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("operatorstore: %s: rows affected: %w", op, err)
	}
	if n == 0 {
		return none
	}
	return nil
}

// --- accounts -------------------------------------------------------------------------------------

// accountColumns — no secret column is selected here. Secrets are read only by Credentials.
const accountColumns = `id, code, email, display_name, must_change_password, totp_enrolled_at,
	pending_totp_secret_sealed IS NOT NULL, failed_attempts, locked_until, disabled_at,
	coalesce(disabled_by, ''), coalesce(disabled_reason, ''), created_at, created_by, updated_at`

const (
	selectAccountByID    = `SELECT ` + accountColumns + ` FROM operator_account WHERE id = $1`
	selectAccountByCode  = `SELECT ` + accountColumns + ` FROM operator_account WHERE code = $1`
	selectAccountByEmail = `SELECT ` + accountColumns + ` FROM operator_account WHERE lower(email) = lower($1)`
)

func scanAccount(row *sql.Row) (domain.OperatorAccount, error) {
	var a domain.OperatorAccount
	var enrolled, locked, disabled sql.NullTime
	err := row.Scan(&a.ID, &a.Code, &a.Email, &a.DisplayName, &a.MustChangePassword, &enrolled,
		&a.TOTPPending, &a.FailedAttempts, &locked, &disabled, &a.DisabledBy, &a.DisabledReason,
		&a.CreatedAt, &a.CreatedBy, &a.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.OperatorAccount{}, ErrNotFound
	}
	if err != nil {
		return domain.OperatorAccount{}, fmt.Errorf("operatorstore: read account: %w", err)
	}
	a.TOTPEnrolledAt = timePtr(enrolled)
	a.LockedUntil = timePtr(locked)
	a.DisabledAt = timePtr(disabled)
	return a, nil
}

func timePtr(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func accountBy(ctx context.Context, q querier, stmt string, key string) (domain.OperatorAccount, error) {
	if strings.TrimSpace(key) == "" {
		return domain.OperatorAccount{}, ErrNotFound
	}
	// @cross-tenant: operator realm has no commune (ADR 0048 §Chốt #1). At most one row, keyed on
	// one account's id, code or sign-in email; never a list.
	return scanAccount(q.QueryRowContext(ctx, stmt, strings.TrimSpace(key)))
}

// ByID reads one account by its internal id.
func (s *Store) ByID(ctx context.Context, id string) (domain.OperatorAccount, error) {
	if s == nil || s.db == nil {
		return domain.OperatorAccount{}, ErrNoDB
	}
	return accountBy(ctx, s.db, selectAccountByID, id)
}

// ByCode reads one account by its business code.
func (s *Store) ByCode(ctx context.Context, code string) (domain.OperatorAccount, error) {
	if s == nil || s.db == nil {
		return domain.OperatorAccount{}, ErrNoDB
	}
	return accountBy(ctx, s.db, selectAccountByCode, code)
}

// ByEmail reads one account by sign-in email, case-folded like the unique index.
func (s *Store) ByEmail(ctx context.Context, email string) (domain.OperatorAccount, error) {
	if s == nil || s.db == nil {
		return domain.OperatorAccount{}, ErrNoDB
	}
	return accountBy(ctx, s.db, selectAccountByEmail, email)
}

// ByID reads one account inside the transaction.
func (t *Tx) ByID(ctx context.Context, id string) (domain.OperatorAccount, error) {
	return accountBy(ctx, t.tx, selectAccountByID, id)
}

// ByCode reads one account inside the transaction.
func (t *Tx) ByCode(ctx context.Context, code string) (domain.OperatorAccount, error) {
	return accountBy(ctx, t.tx, selectAccountByCode, code)
}

// ByEmail reads one account inside the transaction.
func (t *Tx) ByEmail(ctx context.Context, email string) (domain.OperatorAccount, error) {
	return accountBy(ctx, t.tx, selectAccountByEmail, email)
}

// Credentials is the secret material of one account, handed only to the code that verifies it.
// Kept out of domain.OperatorAccount so that no `%+v` of an account can print a credential.
type Credentials struct {
	PasswordHash            string
	TOTPSecretSealed        []byte
	PendingTOTPSecretSealed []byte
	TOTPLastStep            *int64
}

// String keeps a credential out of any log line that formats the struct by accident.
func (Credentials) String() string { return "operatorstore.Credentials{redacted}" }

// GoString does the same for %#v.
func (Credentials) GoString() string { return "operatorstore.Credentials{redacted}" }

const selectCredentials = `SELECT password_hash, totp_secret_sealed, pending_totp_secret_sealed, totp_last_step
	FROM operator_account WHERE id = $1`

// Credentials reads the secret material of one account, inside the transaction.
func (t *Tx) Credentials(ctx context.Context, accountID string) (Credentials, error) {
	var c Credentials
	var step sql.NullInt64
	// @cross-tenant: operator realm has no commune (ADR 0048 §Chốt #1). One row, keyed on one
	// account id.
	err := t.tx.QueryRowContext(ctx, selectCredentials, accountID).Scan(
		&c.PasswordHash, &c.TOTPSecretSealed, &c.PendingTOTPSecretSealed, &step)
	if errors.Is(err, sql.ErrNoRows) {
		return Credentials{}, ErrNotFound
	}
	if err != nil {
		return Credentials{}, fmt.Errorf("operatorstore: read credentials: %w", err)
	}
	if step.Valid {
		v := step.Int64
		c.TOTPLastStep = &v
	}
	return c, nil
}

// NewAccount is what the CLI supplies to create an operator. The password is a temporary one, so
// the account starts with must_change_password = true.
type NewAccount struct {
	Email        string
	DisplayName  string
	PasswordHash string
	CreatedBy    string // an operator code or domain.SystemActor
}

const (
	nextOperatorCode = `SELECT nextval('operator_code_seq')`
	insertAccount    = `INSERT INTO operator_account
	(id, code, email, display_name, password_hash, must_change_password, created_at, created_by, updated_at)
	VALUES ($1, $2, $3, $4, $5, true, $6, $7, $6)`
)

// CreateAccount inserts a new operator, allocating its code from operator_code_seq.
//
// TWO STATEMENTS, AND THE ORDER IS THE GUARANTEE: the number is drawn from the sequence first and
// formatted in Go (domain.FormatOperatorCode) — not lpad() in SQL, which truncates past five digits.
// A rolled-back transaction burns its number; a gap is harmless, a reissued code is not (rule 7,
// invariant 3).
func (t *Tx) CreateAccount(ctx context.Context, in NewAccount, now time.Time) (domain.OperatorAccount, error) {
	email := strings.TrimSpace(in.Email)
	name := strings.TrimSpace(in.DisplayName)
	if email == "" || name == "" || in.PasswordHash == "" {
		return domain.OperatorAccount{}, ErrMissingField
	}
	if !validActor(in.CreatedBy) {
		return domain.OperatorAccount{}, ErrActorShape
	}
	id, err := ulid.Moi()
	if err != nil {
		return domain.OperatorAccount{}, fmt.Errorf("operatorstore: create account: %w", err)
	}

	var n int64
	// @cross-tenant: operator realm has no commune (ADR 0048 §Chốt #1). The sequence is
	// platform-wide by decision: one numbering for all operators.
	if err := t.tx.QueryRowContext(ctx, nextOperatorCode).Scan(&n); err != nil {
		return domain.OperatorAccount{}, fmt.Errorf("operatorstore: draw operator code: %w", err)
	}
	code, err := domain.FormatOperatorCode(n)
	if err != nil {
		return domain.OperatorAccount{}, err
	}

	// @cross-tenant: operator realm has no commune (ADR 0048 §Chốt #1). Inserts one account row.
	_, err = t.tx.ExecContext(ctx, insertAccount, id, code, email, name, in.PasswordHash, now, in.CreatedBy)
	if err != nil {
		if uniqueViolation(err) == "operator_account_email_unique" {
			return domain.OperatorAccount{}, ErrEmailTaken
		}
		return domain.OperatorAccount{}, fmt.Errorf("operatorstore: create account: %w", err)
	}
	return domain.OperatorAccount{
		ID: id, Code: code, Email: email, DisplayName: name, MustChangePassword: true,
		CreatedAt: now, CreatedBy: in.CreatedBy, UpdatedAt: now,
	}, nil
}

const setPendingTOTP = `UPDATE operator_account
	SET pending_totp_secret_sealed = $2, updated_at = $3
	WHERE id = $1 AND totp_secret_sealed IS NULL AND disabled_at IS NULL`

// SetPendingTOTP stores the sealed secret of an enrolment in progress. Refused (ErrStateConflict)
// when a factor is already active — replacing an active factor goes through ResetMFA, audited.
func (t *Tx) SetPendingTOTP(ctx context.Context, accountID string, sealed []byte, now time.Time) error {
	if len(sealed) == 0 {
		return ErrMissingField
	}
	// @cross-tenant: operator realm has no commune (ADR 0048 §Chốt #1). One row, keyed on one
	// account id.
	res, err := t.tx.ExecContext(ctx, setPendingTOTP, accountID, sealed, now)
	if err != nil {
		return fmt.Errorf("operatorstore: set pending totp: %w", err)
	}
	return requireOne(res, "set pending totp", ErrStateConflict)
}

const activateTOTP = `UPDATE operator_account
	SET totp_secret_sealed = pending_totp_secret_sealed, pending_totp_secret_sealed = NULL,
	    totp_enrolled_at = $2, totp_last_step = $3, password_hash = $4,
	    must_change_password = false, updated_at = $2
	WHERE id = $1 AND pending_totp_secret_sealed IS NOT NULL AND disabled_at IS NULL`

// ActivateTOTP promotes the pending secret to the active factor, records the step of the code that
// proved it (so that very code cannot be replayed), sets the operator's own password and clears
// must_change_password. It is a password change, so every live session is revoked.
func (t *Tx) ActivateTOTP(ctx context.Context, accountID, passwordHash string, step int64, now time.Time) error {
	if passwordHash == "" {
		return ErrMissingField
	}
	// @cross-tenant: operator realm has no commune (ADR 0048 §Chốt #1). One row, keyed on one
	// account id.
	res, err := t.tx.ExecContext(ctx, activateTOTP, accountID, now, step, passwordHash)
	if err != nil {
		return fmt.Errorf("operatorstore: activate totp: %w", err)
	}
	if err := requireOne(res, "activate totp", ErrStateConflict); err != nil {
		return err
	}
	_, err = t.RevokeAllSessions(ctx, accountID, RevokeReasonPasswordChanged, now)
	return err
}

const recordTOTPStep = `UPDATE operator_account
	SET totp_last_step = $2
	WHERE id = $1 AND totp_secret_sealed IS NOT NULL
	  AND (totp_last_step IS NULL OR totp_last_step < $2)`

// RecordTOTPStep accepts a verified TOTP step ONLY IF it is strictly later than the last one
// accepted. One atomic UPDATE, so two concurrent sign-ins with the same code cannot both pass:
// the second finds the condition false. Returns false for a replayed (or older) step.
func (t *Tx) RecordTOTPStep(ctx context.Context, accountID string, step int64) (bool, error) {
	// @cross-tenant: operator realm has no commune (ADR 0048 §Chốt #1). One row, keyed on one
	// account id.
	res, err := t.tx.ExecContext(ctx, recordTOTPStep, accountID, step)
	if err != nil {
		return false, fmt.Errorf("operatorstore: record totp step: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("operatorstore: record totp step: rows affected: %w", err)
	}
	return n == 1, nil
}

// registerFailure — ONE statement: increment, and on reaching the threshold set locked_until and
// restart the count at 0, so the next window starts fresh. Refused while a lock is in force, so
// failures during a lock neither extend it nor pile up. Because a locking failure is the ONLY way
// the new count is 0, RETURNING failed_attempts tells the caller whether this call locked.
const registerFailure = `UPDATE operator_account
	SET failed_attempts = CASE WHEN failed_attempts + 1 >= $3 THEN 0 ELSE failed_attempts + 1 END,
	    locked_until    = CASE WHEN failed_attempts + 1 >= $3 THEN $4 ELSE locked_until END,
	    updated_at      = $2
	WHERE id = $1 AND (locked_until IS NULL OR locked_until <= $2)
	RETURNING failed_attempts`

// RegisterFailure records one failed password or TOTP attempt. It returns lockedNow = true when
// this failure reached domain.MaxFailedAttempts and locked the account until lockedUntil — the
// event the owner asked to audit. ErrAccountLocked when a lock is already in force.
func (t *Tx) RegisterFailure(ctx context.Context, accountID string, now time.Time) (lockedNow bool, lockedUntil time.Time, err error) {
	// Microsecond precision, PostgreSQL's, so the value returned is the value stored.
	until := now.Add(domain.LockoutDuration).Truncate(time.Microsecond)
	var count int
	// @cross-tenant: operator realm has no commune (ADR 0048 §Chốt #1). One row, keyed on one
	// account id.
	err = t.tx.QueryRowContext(ctx, registerFailure, accountID, now, domain.MaxFailedAttempts, until).Scan(&count)
	if errors.Is(err, sql.ErrNoRows) {
		// Either no such account or a lock in force. Tell them apart without a second statement
		// the caller might forget: read the row.
		a, rerr := t.ByID(ctx, accountID)
		if rerr != nil {
			return false, time.Time{}, rerr
		}
		if a.LockedAt(now) {
			return false, *a.LockedUntil, ErrAccountLocked
		}
		return false, time.Time{}, ErrNotFound
	}
	if err != nil {
		return false, time.Time{}, fmt.Errorf("operatorstore: register failure: %w", err)
	}
	if count == 0 {
		return true, until, nil
	}
	return false, time.Time{}, nil
}

const resetFailures = `UPDATE operator_account
	SET failed_attempts = 0, locked_until = NULL, updated_at = $2
	WHERE id = $1`

// ResetFailures clears the failure count and any lockout — after a successful sign-in.
func (t *Tx) ResetFailures(ctx context.Context, accountID string, now time.Time) error {
	// @cross-tenant: operator realm has no commune (ADR 0048 §Chốt #1). One row, keyed on one
	// account id.
	res, err := t.tx.ExecContext(ctx, resetFailures, accountID, now)
	if err != nil {
		return fmt.Errorf("operatorstore: reset failures: %w", err)
	}
	return requireOne(res, "reset failures", ErrNotFound)
}

const setPassword = `UPDATE operator_account
	SET password_hash = $2, must_change_password = $3, updated_at = $4
	WHERE id = $1`

// SetPassword replaces the password hash and revokes every live session (owner's decision).
// mustChange = true when the CLI sets a temporary password.
func (t *Tx) SetPassword(ctx context.Context, accountID, passwordHash string, mustChange bool, now time.Time) error {
	if passwordHash == "" {
		return ErrMissingField
	}
	// @cross-tenant: operator realm has no commune (ADR 0048 §Chốt #1). One row, keyed on one
	// account id.
	res, err := t.tx.ExecContext(ctx, setPassword, accountID, passwordHash, mustChange, now)
	if err != nil {
		return fmt.Errorf("operatorstore: set password: %w", err)
	}
	if err := requireOne(res, "set password", ErrNotFound); err != nil {
		return err
	}
	_, err = t.RevokeAllSessions(ctx, accountID, RevokeReasonPasswordChanged, now)
	return err
}

const disableAccount = `UPDATE operator_account
	SET disabled_at = $2, disabled_by = $3, disabled_reason = $4, updated_at = $2
	WHERE id = $1 AND disabled_at IS NULL`

// Disable is the administrative lock (soft — rule 7). Revokes every live session.
func (t *Tx) Disable(ctx context.Context, accountID, by, reason string, now time.Time) error {
	if !validActor(by) {
		return ErrActorShape
	}
	if strings.TrimSpace(reason) == "" {
		return ErrMissingField
	}
	// @cross-tenant: operator realm has no commune (ADR 0048 §Chốt #1). One row, keyed on one
	// account id.
	res, err := t.tx.ExecContext(ctx, disableAccount, accountID, now, by, reason)
	if err != nil {
		return fmt.Errorf("operatorstore: disable: %w", err)
	}
	if err := requireOne(res, "disable", ErrStateConflict); err != nil {
		return err
	}
	_, err = t.RevokeAllSessions(ctx, accountID, RevokeReasonDisabled, now)
	return err
}

const enableAccount = `UPDATE operator_account
	SET disabled_at = NULL, disabled_by = NULL, disabled_reason = NULL, updated_at = $2
	WHERE id = $1 AND disabled_at IS NOT NULL`

// Enable lifts the administrative lock. Who lifted it and why belongs in the audit entry the
// caller writes in this transaction; the row only holds the current state.
func (t *Tx) Enable(ctx context.Context, accountID string, now time.Time) error {
	// @cross-tenant: operator realm has no commune (ADR 0048 §Chốt #1). One row, keyed on one
	// account id.
	res, err := t.tx.ExecContext(ctx, enableAccount, accountID, now)
	if err != nil {
		return fmt.Errorf("operatorstore: enable: %w", err)
	}
	return requireOne(res, "enable", ErrStateConflict)
}

const (
	resetMFA = `UPDATE operator_account
	SET totp_secret_sealed = NULL, totp_enrolled_at = NULL, totp_last_step = NULL,
	    pending_totp_secret_sealed = NULL, updated_at = $2
	WHERE id = $1`
	voidLiveRecoveryCodes = `UPDATE operator_recovery_code
	SET voided_at = $2
	WHERE operator_account_id = $1 AND used_at IS NULL AND voided_at IS NULL`
)

// ResetMFA removes the TOTP factor (active and pending), voids every live recovery code and
// revokes every live session, so the operator must enrol again at the next sign-in.
func (t *Tx) ResetMFA(ctx context.Context, accountID string, now time.Time) error {
	// @cross-tenant: operator realm has no commune (ADR 0048 §Chốt #1). One row, keyed on one
	// account id.
	res, err := t.tx.ExecContext(ctx, resetMFA, accountID, now)
	if err != nil {
		return fmt.Errorf("operatorstore: reset mfa: %w", err)
	}
	if err := requireOne(res, "reset mfa", ErrNotFound); err != nil {
		return err
	}
	// @cross-tenant: operator realm has no commune (ADR 0048 §Chốt #1). Rows of one account only.
	if _, err := t.tx.ExecContext(ctx, voidLiveRecoveryCodes, accountID, now); err != nil {
		return fmt.Errorf("operatorstore: reset mfa: void recovery codes: %w", err)
	}
	_, err = t.RevokeAllSessions(ctx, accountID, RevokeReasonMFAReset, now)
	return err
}

// --- sessions -------------------------------------------------------------------------------------

// OperatorSession is a live session as CheckSession sees it.
type OperatorSession struct {
	AccountID   string
	AccountCode string
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

const insertSession = `INSERT INTO operator_session
	(id, operator_account_id, created_at, expires_at, created_ip, user_agent, last_seen_at)
	VALUES ($1, $2, $3, $4, $5, $6, $3)`

// CreateSession registers a session for the raw sid. Only its SHA-256 is stored. The lifetime is
// domain.SessionLifetime, absolute. Returns the expiry.
func (t *Tx) CreateSession(ctx context.Context, sid, accountID, ip, userAgent string, now time.Time) (time.Time, error) {
	if sid == "" || accountID == "" {
		return time.Time{}, ErrMissingField
	}
	expires := now.Add(domain.SessionLifetime)
	// @cross-tenant: operator realm has no commune (ADR 0048 §Chốt #1). Inserts one session row.
	_, err := t.tx.ExecContext(ctx, insertSession, hashSecret(sid), accountID, now, expires, ip, userAgent)
	if err != nil {
		return time.Time{}, fmt.Errorf("operatorstore: create session: %w", err)
	}
	return expires, nil
}

// checkSession — a session is live when it is not revoked, not expired, and its account is not
// disabled. One answer for every failure, so the caller cannot leak WHICH condition failed.
const checkSession = `SELECT s.operator_account_id, a.code, s.created_at, s.expires_at
	FROM operator_session s
	JOIN operator_account a ON a.id = s.operator_account_id
	WHERE s.id = $1 AND s.revoked_at IS NULL AND s.expires_at > $2 AND a.disabled_at IS NULL`

func checkSessionOn(ctx context.Context, q querier, sid string, now time.Time) (OperatorSession, error) {
	if sid == "" {
		return OperatorSession{}, ErrNotFound
	}
	var s OperatorSession
	// @cross-tenant: operator realm has no commune (ADR 0048 §Chốt #1). At most one row, keyed on
	// the hash of the bearer's own sid.
	err := q.QueryRowContext(ctx, checkSession, hashSecret(sid), now).Scan(
		&s.AccountID, &s.AccountCode, &s.CreatedAt, &s.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return OperatorSession{}, ErrNotFound
	}
	if err != nil {
		return OperatorSession{}, fmt.Errorf("operatorstore: check session: %w", err)
	}
	return s, nil
}

// CheckSession resolves a raw sid to a live session — on every operator request.
func (s *Store) CheckSession(ctx context.Context, sid string, now time.Time) (OperatorSession, error) {
	if s == nil || s.db == nil {
		return OperatorSession{}, ErrNoDB
	}
	return checkSessionOn(ctx, s.db, sid, now)
}

// CheckSession resolves a raw sid inside the transaction.
func (t *Tx) CheckSession(ctx context.Context, sid string, now time.Time) (OperatorSession, error) {
	return checkSessionOn(ctx, t.tx, sid, now)
}

const revokeSession = `UPDATE operator_session
	SET revoked_at = $2, revoked_reason = $3
	WHERE id = $1 AND revoked_at IS NULL`

// RevokeSession revokes one session by raw sid (sign-out). ErrNotFound when it is unknown or
// already revoked.
func (t *Tx) RevokeSession(ctx context.Context, sid, reason string, now time.Time) error {
	if strings.TrimSpace(reason) == "" {
		return ErrMissingField
	}
	// @cross-tenant: operator realm has no commune (ADR 0048 §Chốt #1). One row, keyed on the hash
	// of the bearer's own sid.
	res, err := t.tx.ExecContext(ctx, revokeSession, hashSecret(sid), now, reason)
	if err != nil {
		return fmt.Errorf("operatorstore: revoke session: %w", err)
	}
	return requireOne(res, "revoke session", ErrNotFound)
}

const revokeAllSessions = `UPDATE operator_session
	SET revoked_at = $2, revoked_reason = $3
	WHERE operator_account_id = $1 AND revoked_at IS NULL`

// RevokeAllSessions revokes every live session of one account and returns how many.
func (t *Tx) RevokeAllSessions(ctx context.Context, accountID, reason string, now time.Time) (int64, error) {
	if accountID == "" || strings.TrimSpace(reason) == "" {
		return 0, ErrMissingField
	}
	// @cross-tenant: operator realm has no commune (ADR 0048 §Chốt #1). Rows of one account only.
	res, err := t.tx.ExecContext(ctx, revokeAllSessions, accountID, now, reason)
	if err != nil {
		return 0, fmt.Errorf("operatorstore: revoke all sessions: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("operatorstore: revoke all sessions: rows affected: %w", err)
	}
	return n, nil
}

const touchSession = `UPDATE operator_session
	SET last_seen_at = $2
	WHERE id = $1 AND revoked_at IS NULL AND expires_at > $2`

// TouchSession records activity. It NEVER extends expires_at — the lifetime is absolute.
func (t *Tx) TouchSession(ctx context.Context, sid string, now time.Time) error {
	// @cross-tenant: operator realm has no commune (ADR 0048 §Chốt #1). One row, keyed on the hash
	// of the bearer's own sid.
	res, err := t.tx.ExecContext(ctx, touchSession, hashSecret(sid), now)
	if err != nil {
		return fmt.Errorf("operatorstore: touch session: %w", err)
	}
	return requireOne(res, "touch session", ErrNotFound)
}

// --- permission grants ----------------------------------------------------------------------------

const insertGrant = `INSERT INTO operator_permission_grant
	(id, operator_account_id, permission_key, granted_at, granted_by, grant_reason)
	VALUES ($1, $2, $3, $4, $5, $6)`

// Grant gives one `ops.*` key to one account and revokes its live sessions, so the next session is
// issued with the new rights. ErrAlreadyGranted when a live grant of that key exists.
func (t *Tx) Grant(ctx context.Context, accountID string, key domain.OperatorPermission, by, reason string, now time.Time) error {
	if !key.Valid() {
		return fmt.Errorf("%w: %q", ErrUnknownPermission, key)
	}
	if !validActor(by) {
		return ErrActorShape
	}
	if accountID == "" || strings.TrimSpace(reason) == "" {
		return ErrMissingField
	}
	id, err := ulid.Moi()
	if err != nil {
		return fmt.Errorf("operatorstore: grant: %w", err)
	}
	// @cross-tenant: operator realm has no commune (ADR 0048 §Chốt #1). Inserts one grant row for
	// one account.
	_, err = t.tx.ExecContext(ctx, insertGrant, id, accountID, string(key), now, by, reason)
	if err != nil {
		if uniqueViolation(err) == "operator_permission_grant_live_unique" {
			return ErrAlreadyGranted
		}
		return fmt.Errorf("operatorstore: grant: %w", err)
	}
	_, err = t.RevokeAllSessions(ctx, accountID, RevokeReasonGrantChanged, now)
	return err
}

const revokeGrant = `UPDATE operator_permission_grant
	SET revoked_at = $3, revoked_by = $4, revoke_reason = $5
	WHERE operator_account_id = $1 AND permission_key = $2 AND revoked_at IS NULL`

// Revoke withdraws a live grant (the row stays — it is the history) and revokes live sessions.
func (t *Tx) Revoke(ctx context.Context, accountID string, key domain.OperatorPermission, by, reason string, now time.Time) error {
	if !key.Valid() {
		return fmt.Errorf("%w: %q", ErrUnknownPermission, key)
	}
	if !validActor(by) {
		return ErrActorShape
	}
	if strings.TrimSpace(reason) == "" {
		return ErrMissingField
	}
	// @cross-tenant: operator realm has no commune (ADR 0048 §Chốt #1). At most one live grant row
	// of one account.
	res, err := t.tx.ExecContext(ctx, revokeGrant, accountID, string(key), now, by, reason)
	if err != nil {
		return fmt.Errorf("operatorstore: revoke grant: %w", err)
	}
	if err := requireOne(res, "revoke grant", ErrNotGranted); err != nil {
		return err
	}
	_, err = t.RevokeAllSessions(ctx, accountID, RevokeReasonGrantChanged, now)
	return err
}

const selectActivePermissions = `SELECT permission_key FROM operator_permission_grant
	WHERE operator_account_id = $1 AND revoked_at IS NULL
	ORDER BY permission_key`

func activePermissionsOn(ctx context.Context, q querier, accountID string) ([]domain.OperatorPermission, error) {
	// @cross-tenant: operator realm has no commune (ADR 0048 §Chốt #1). Rows of one account only.
	rows, err := q.QueryContext(ctx, selectActivePermissions, accountID)
	if err != nil {
		return nil, fmt.Errorf("operatorstore: active permissions: %w", err)
	}
	defer rows.Close()
	var out []domain.OperatorPermission
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, fmt.Errorf("operatorstore: active permissions: scan: %w", err)
		}
		p, err := domain.ParseOperatorPermission(k)
		if err != nil {
			// The CHECK makes this unreachable; if it happens anyway, FAIL CLOSED — an unknown key
			// is never silently dropped or silently honoured.
			return nil, fmt.Errorf("operatorstore: active permissions: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("operatorstore: active permissions: %w", err)
	}
	return out, nil
}

// ActivePermissions lists the live keys of one account.
func (s *Store) ActivePermissions(ctx context.Context, accountID string) ([]domain.OperatorPermission, error) {
	if s == nil || s.db == nil {
		return nil, ErrNoDB
	}
	return activePermissionsOn(ctx, s.db, accountID)
}

// ActivePermissions lists the live keys of one account inside the transaction.
func (t *Tx) ActivePermissions(ctx context.Context, accountID string) ([]domain.OperatorPermission, error) {
	return activePermissionsOn(ctx, t.tx, accountID)
}

// --- recovery codes -------------------------------------------------------------------------------

const insertRecoveryCode = `INSERT INTO operator_recovery_code
	(id, operator_account_id, code_hash, batch_id, created_at)
	VALUES ($1, $2, $3, $4, $5)`

// ReplaceRecoveryCodes voids every live code of the account and stores a new batch of exactly
// domain.RecoveryCodeCount distinct codes. Returns the batch id.
//
// DIGESTS IN, NOT PLAINTEXT — unlike the sid. A recovery code must be NORMALISED before hashing
// (case, dashes, look-alike characters), and that normalisation belongs to one place:
// operatorauth.HashRecoveryCode, which returns the 32-byte SHA-256 this method takes. Hashing again
// here would store a hash of a hash that no verifier computes. Anything that is not exactly 32 bytes
// is refused, so plaintext cannot reach the column by mistake (the CHECK refuses it too).
func (t *Tx) ReplaceRecoveryCodes(ctx context.Context, accountID string, digests [][]byte, now time.Time) (string, error) {
	if accountID == "" {
		return "", ErrMissingField
	}
	if len(digests) != domain.RecoveryCodeCount {
		return "", ErrRecoveryCodeSet
	}
	seen := make(map[string]bool, len(digests))
	for _, d := range digests {
		if len(d) != sha256.Size || seen[string(d)] {
			return "", ErrRecoveryCodeSet
		}
		seen[string(d)] = true
	}
	batch, err := ulid.Moi()
	if err != nil {
		return "", fmt.Errorf("operatorstore: recovery codes: %w", err)
	}
	// @cross-tenant: operator realm has no commune (ADR 0048 §Chốt #1). Rows of one account only.
	if _, err := t.tx.ExecContext(ctx, voidLiveRecoveryCodes, accountID, now); err != nil {
		return "", fmt.Errorf("operatorstore: recovery codes: void old batch: %w", err)
	}
	for _, d := range digests {
		id, err := ulid.Moi()
		if err != nil {
			return "", fmt.Errorf("operatorstore: recovery codes: %w", err)
		}
		// @cross-tenant: operator realm has no commune (ADR 0048 §Chốt #1). Inserts one code row
		// for one account.
		if _, err := t.tx.ExecContext(ctx, insertRecoveryCode, id, accountID, hex.EncodeToString(d), batch, now); err != nil {
			return "", fmt.Errorf("operatorstore: recovery codes: insert: %w", err)
		}
	}
	return batch, nil
}

const useRecoveryCode = `UPDATE operator_recovery_code
	SET used_at = $3
	WHERE operator_account_id = $1 AND code_hash = $2 AND used_at IS NULL AND voided_at IS NULL`

// UseRecoveryCode consumes one code atomically, given its digest (operatorauth.HashRecoveryCode of
// what the operator typed): true when a live code of THIS account matched. A code already used,
// voided, or belonging to another account returns false — one answer for all.
func (t *Tx) UseRecoveryCode(ctx context.Context, accountID string, digest []byte, now time.Time) (bool, error) {
	if accountID == "" || len(digest) != sha256.Size {
		return false, nil
	}
	// @cross-tenant: operator realm has no commune (ADR 0048 §Chốt #1). At most one row of one
	// account.
	res, err := t.tx.ExecContext(ctx, useRecoveryCode, accountID, hex.EncodeToString(digest), now)
	if err != nil {
		return false, fmt.Errorf("operatorstore: use recovery code: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("operatorstore: use recovery code: rows affected: %w", err)
	}
	return n == 1, nil
}

// --- audit ----------------------------------------------------------------------------------------

const insertAudit = `INSERT INTO operator_audit_log
	(occurred_at, actor, actor_ip, action, subject, before, after, reason)
	VALUES (coalesce($1, now()), $2, nullif($3, ''), $4, $5, $6, $7, nullif($8, ''))`

// AppendAudit writes one operator_audit_log row IN THIS TRANSACTION (rule 6, invariant 3). The
// entry is validated first (domain.OperatorAuditEntry.Validate — no fallback actor), so a refused
// entry rolls the business change back with it.
func (t *Tx) AppendAudit(ctx context.Context, e domain.OperatorAuditEntry) error {
	if err := e.Validate(); err != nil {
		return err
	}
	before, err := jsonOrNull(e.Before)
	if err != nil {
		return fmt.Errorf("operatorstore: audit before: %w", err)
	}
	after, err := jsonOrNull(e.After)
	if err != nil {
		return fmt.Errorf("operatorstore: audit after: %w", err)
	}
	var at sql.NullTime
	if !e.OccurredAt.IsZero() {
		at = sql.NullTime{Time: e.OccurredAt, Valid: true}
	}
	// @cross-tenant: operator realm has no commune (ADR 0048 §Chốt #1). Appends one trail row.
	_, err = t.tx.ExecContext(ctx, insertAudit, at, e.Actor, e.ActorIP, string(e.Action), e.Subject,
		before, after, e.Reason)
	if err != nil {
		return fmt.Errorf("operatorstore: append audit: %w", err)
	}
	return nil
}

// jsonOrNull marshals m, or returns nil (SQL NULL) for an empty map.
func jsonOrNull(m map[string]any) (any, error) {
	if len(m) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}
