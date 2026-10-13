package operatorclient

// The seven OWNER-ONLY RPCs of OperatorService (proto/vigov/identity/v1/operator.proto; ADR 0048
// §"Sửa đổi 13/10/2026 — owner, …"): the operator accounts (four, NO commune) and a commune's staff
// accounts (three, THE TARGET COMMUNE in metadata).
//
// WHO IS THE OWNER IS IDENTITY'S TO DECIDE, on the session it resolves from the token — this package
// sends no actor and no owner flag. service-platform's opauth.RequireOwner is the first check, never
// the only one.
//
// THE COMMUNE: the four operator-account RPCs are on core/grpcx.methodsWithoutTenant by full name, so
// a commune in ctx is not sent. The three staff-account RPCs are NOT, so the commune travels from ctx
// like SetMiniAppSecret's, and a ctx without one is refused HERE (requireCommune) before anything is
// sent.
//
// PASSWORDS cross this API as secret.Secret and are taken out with raw() only at the line that builds
// the request. Responses carry operators' and staff members' names and sign-in emails — personal data
// (rule 3): nothing here logs a request, a response or a status message; only the RPC name and the
// gRPC code are ever logged.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/secret"
)

var (
	// ErrAccountNotFound is NOT_FOUND: no operator account has the target code
	// (SetOperatorPermissions / ResetOperatorPassword), or the target commune has no live sign-in
	// account with the user id (ResetStaffPassword — another commune's id is "none here"). Answered by
	// identity only AFTER its owner check. The caller answers 404.
	ErrAccountNotFound = errors.New("operatorclient: no such account")

	// ErrAccountExists is ALREADY_EXISTS: the email already holds an account — an operator account
	// (CreateOperatorAccount) or a sign-in account of the target commune (CreateCommuneAdminAccount).
	// Nothing was written. The caller answers 409.
	ErrAccountExists = errors.New("operatorclient: an account already has this email")

	// ErrCommuneNotLive is FAILED_PRECONDITION on CreateCommuneAdminAccount: identity re-read the
	// commune from the platform and it does not exist or is inactive. Nothing written. The caller
	// answers 409. Not ErrRealmNotConfigured although the code is the same — the realm meaning is
	// excluded by the caller having resolved the session on the same request (operator.proto, the
	// argument made on SetMiniAppSecret).
	ErrCommuneNotLive = errors.New("operatorclient: the target commune is not live on the platform")

	// ErrStaffAccountLocked is FAILED_PRECONDITION on ResetStaffPassword: the account is
	// administratively locked (dang_hoat_dong = false) — the commune's decision, which this RPC does
	// not lift. Nothing written. The caller answers 409.
	ErrStaffAccountLocked = errors.New("operatorclient: the staff account is administratively locked")
)

// OperatorAccount is one operator account as the owner's screen shows it — never a password hash, a
// TOTP secret, a recovery code or a session (operator.proto, OperatorAccount).
type OperatorAccount struct {
	// Code is `VH-00001` — names the account in every request; may be logged.
	Code string
	// Email and DisplayName are PERSONAL DATA (rule 3): never logged.
	Email       string
	DisplayName string
	// PermissionKeys are the granted `ops.*` keys, read live. Never nil on a mapped account.
	PermissionKeys     []string
	IsOwner            bool
	Disabled           bool
	MustChangePassword bool
	// LockedUntil is the failed-sign-in lockout's end; nil means not locked. A past instant also means
	// not locked — the lock is derived from it, never a flag.
	LockedUntil *time.Time
	CreatedAt   time.Time
}

// CommuneStaffAccount is one sign-in account of the target commune — the narrowest shape the owner's
// screen needs (operator.proto, CommuneStaffAccount).
type CommuneStaffAccount struct {
	// UserID names the account in ResetStaffPassword. Authorises nothing.
	UserID string
	// StaffCode is `CB-…`; may be logged.
	StaffCode string
	// Email (the sign-in name) and FullName are PERSONAL DATA (rule 3): never logged.
	Email    string
	FullName string
	// RoleCode / RoleName: the one role; both empty when the account holds none.
	RoleCode string
	RoleName string
	// IsAdmin is identity's statement that RoleCode is the commune administrator role — the screen
	// holds no copy of that literal.
	IsAdmin            bool
	Disabled           bool
	MustChangePassword bool
	// SignInLockedUntil: nil means not locked; a past instant also means not locked.
	SignInLockedUntil *time.Time
}

// OperatorAccountsResult: Accounts is meaningful exactly when Outcome is OutcomeAccepted.
type OperatorAccountsResult struct {
	Outcome  Outcome
	Accounts []OperatorAccount
}

// OperatorAccountResult: Account is set exactly when ACCEPTED; Refusal exactly when
// OutcomeNewPasswordRejected.
type OperatorAccountResult struct {
	Outcome Outcome
	Account OperatorAccount
	Refusal NewPasswordRefusal
}

// StaffAccountsResult: Accounts is meaningful exactly when ACCEPTED; empty is a valid answer.
type StaffAccountsResult struct {
	Outcome  Outcome
	Accounts []CommuneStaffAccount
}

// StaffAccountResult: Account is set exactly when ACCEPTED; Refusal exactly when
// OutcomeNewPasswordRejected.
type StaffAccountResult struct {
	Outcome Outcome
	Account CommuneStaffAccount
	Refusal NewPasswordRefusal
}

// CreateOperatorAccountRequest — no actor field: identity resolves the owner from Token.
type CreateOperatorAccountRequest struct {
	Token secret.Secret
	// Email, DisplayName — PERSONAL DATA (rule 3).
	Email       string
	DisplayName string
	// TemporaryPassword is write-only, verbatim (never trimmed).
	TemporaryPassword secret.Secret
	// PermissionKeys may be empty: an account holding no key.
	PermissionKeys []string
	ClientIP       string
}

// SetOperatorPermissionsRequest — PermissionKeys is the COMPLETE desired set; empty revokes every key.
type SetOperatorPermissionsRequest struct {
	Token          secret.Secret
	TargetCode     string
	PermissionKeys []string
	Reason         string
	ClientIP       string
}

// ResetOperatorPasswordRequest gives the target a new TEMPORARY password; its TOTP is kept.
type ResetOperatorPasswordRequest struct {
	Token             secret.Secret
	TargetCode        string
	TemporaryPassword secret.Secret
	Reason            string
	ClientIP          string
}

// CreateCommuneAdminAccountRequest — the target commune is the context's, never a field.
type CreateCommuneAdminAccountRequest struct {
	Token secret.Secret
	// Email (the sign-in name) and FullName — PERSONAL DATA (rule 3).
	Email             string
	FullName          string
	TemporaryPassword secret.Secret
	ClientIP          string
}

// ResetStaffPasswordRequest — the target commune is the context's; UserID must be one of its accounts.
type ResetStaffPasswordRequest struct {
	Token             secret.Secret
	UserID            string
	TemporaryPassword secret.Secret
	Reason            string
	ClientIP          string
}

// ownerCallFailed reads the status of an owner-only RPC that returned an error. The typed answers
// (NOT_FOUND, ALREADY_EXISTS, the two second meanings of FAILED_PRECONDITION) log nothing — they are
// ordinary refusals; INVALID_ARGUMENT logs the RPC and the code only, as on the secret RPCs.
func (c *Client) ownerCallFailed(ctx context.Context, rpc string, err error) error {
	code := status.Code(err)
	switch {
	case code == codes.Unimplemented:
		// An identity older than the RPC — expected while platform ships first. Not an outage.
		return fmt.Errorf("%w: %s", ErrNotSupported, rpc)
	case code == codes.NotFound && (rpc == "SetOperatorPermissions" || rpc == "ResetOperatorPassword" || rpc == "ResetStaffPassword"):
		return fmt.Errorf("%w: %s", ErrAccountNotFound, rpc)
	case code == codes.AlreadyExists && (rpc == "CreateOperatorAccount" || rpc == "CreateCommuneAdminAccount"):
		return fmt.Errorf("%w: %s", ErrAccountExists, rpc)
	case code == codes.FailedPrecondition && rpc == "CreateCommuneAdminAccount":
		return fmt.Errorf("%w: %s", ErrCommuneNotLive, rpc)
	case code == codes.FailedPrecondition && rpc == "ResetStaffPassword":
		return fmt.Errorf("%w: %s", ErrStaffAccountLocked, rpc)
	case code == codes.InvalidArgument:
		c.log.WarnContext(ctx, "OperatorService từ chối dạng yêu cầu", "rpc", rpc, "ma_loi", code.String())
		return fmt.Errorf("%w: %s", ErrArgumentRefused, rpc)
	}
	// Everything else — FAILED_PRECONDITION on the four no-commune RPCs (realm not configured)
	// included — is a call that did not happen.
	return c.callFailed(ctx, rpc, err)
}

// ownerOutcome maps an owner-only RPC's outcome. UNSPECIFIED is a contract fault here, never
// "refused": there is no credential in these requests for "refused" to be about.
func ownerOutcome(rpc string, o identityv1.OperatorAuthOutcome, allowed ...identityv1.OperatorAuthOutcome) (Outcome, error) {
	got, err := outcome(rpc, o, allowed...)
	if err != nil {
		return OutcomeRefused, err
	}
	if got == OutcomeRefused {
		return OutcomeRefused, fmt.Errorf("%w: %s answered no outcome", ErrContract, rpc)
	}
	return got, nil
}

func optionalTime(t *timestamppb.Timestamp) *time.Time {
	if t == nil {
		return nil
	}
	v := t.AsTime()
	return &v
}

// toOperatorAccount refuses an account without its code or creation time: an account the screen
// cannot name cannot be acted on, and a request naming "" would be refused by identity anyway.
func toOperatorAccount(rpc string, a *identityv1.OperatorAccount) (OperatorAccount, error) {
	if a == nil || a.GetCode() == "" || a.GetCreatedAt() == nil {
		return OperatorAccount{}, fmt.Errorf("%w: %s account without code or created_at", ErrContract, rpc)
	}
	keys := make([]string, 0, len(a.GetPermissionKeys()))
	for _, k := range a.GetPermissionKeys() {
		if k != "" {
			keys = append(keys, k)
		}
	}
	return OperatorAccount{
		Code: a.GetCode(), Email: a.GetEmail(), DisplayName: a.GetDisplayName(), PermissionKeys: keys,
		IsOwner: a.GetIsOwner(), Disabled: a.GetDisabled(), MustChangePassword: a.GetMustChangePassword(),
		LockedUntil: optionalTime(a.GetLockedUntil()), CreatedAt: a.GetCreatedAt().AsTime(),
	}, nil
}

func toStaffAccount(rpc string, a *identityv1.CommuneStaffAccount) (CommuneStaffAccount, error) {
	if a == nil || a.GetUserId() == "" {
		return CommuneStaffAccount{}, fmt.Errorf("%w: %s staff account without user_id", ErrContract, rpc)
	}
	return CommuneStaffAccount{
		UserID: a.GetUserId(), StaffCode: a.GetStaffCode(), Email: a.GetEmail(), FullName: a.GetFullName(),
		RoleCode: a.GetRoleCode(), RoleName: a.GetRoleName(), IsAdmin: a.GetIsAdmin(), Disabled: a.GetDisabled(),
		MustChangePassword: a.GetMustChangePassword(), SignInLockedUntil: optionalTime(a.GetSignInLockedUntil()),
	}, nil
}

// --- operator accounts (no commune) -----------------------------------------------------------------

// ListOperatorAccounts is OperatorService.ListOperatorAccounts.
//
//	err == nil        ACCEPTED (Accounts), SESSION_NOT_LIVE (401) or PERMISSION_DENIED (403)
//	ErrNotSupported   identity predates the RPC
//	ErrContract       UNSPECIFIED, an outcome outside the subset, an empty list (the owner reading it
//	                  is one of them), an account without code, a code twice
//	ErrInvalidRequest empty token — nothing sent
//	any other error   the call did not happen
func (c *Client) ListOperatorAccounts(ctx context.Context, token secret.Secret) (OperatorAccountsResult, error) {
	const rpc = "ListOperatorAccounts"
	if token.Rong() {
		return OperatorAccountsResult{}, fmt.Errorf("%w: %s with an empty token", ErrInvalidRequest, rpc)
	}
	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	ra, err := c.cl.ListOperatorAccounts(ctx, &identityv1.ListOperatorAccountsRequest{SessionToken: raw(token)})
	if err != nil {
		return OperatorAccountsResult{}, c.ownerCallFailed(ctx, rpc, err)
	}
	o, err := ownerOutcome(rpc, ra.GetOutcome(), wAccepted, wNotLive, wDenied)
	if err != nil {
		return OperatorAccountsResult{}, err
	}
	res := OperatorAccountsResult{Outcome: o}
	if o != OutcomeAccepted {
		return res, nil
	}
	if len(ra.GetAccounts()) == 0 {
		return OperatorAccountsResult{}, fmt.Errorf("%w: %s ACCEPTED with no account", ErrContract, rpc)
	}
	seen := make(map[string]bool, len(ra.GetAccounts()))
	for _, a := range ra.GetAccounts() {
		acc, err := toOperatorAccount(rpc, a)
		if err != nil {
			return OperatorAccountsResult{}, err
		}
		if seen[acc.Code] {
			return OperatorAccountsResult{}, fmt.Errorf("%w: %s named one account twice", ErrContract, rpc)
		}
		seen[acc.Code] = true
		res.Accounts = append(res.Accounts, acc)
	}
	return res, nil
}

// CreateOperatorAccount is OperatorService.CreateOperatorAccount.
//
//	err == nil          ACCEPTED (Account), SESSION_NOT_LIVE, PERMISSION_DENIED, NEW_PASSWORD_REJECTED
//	ErrAccountExists    409
//	ErrArgumentRefused  422 (email / display name shape, a key outside the realm's list)
//	ErrNotSupported · ErrContract · ErrInvalidRequest · other → 503
func (c *Client) CreateOperatorAccount(ctx context.Context, req CreateOperatorAccountRequest) (OperatorAccountResult, error) {
	const rpc = "CreateOperatorAccount"
	if req.Token.Rong() {
		return OperatorAccountResult{}, fmt.Errorf("%w: %s with an empty token", ErrInvalidRequest, rpc)
	}
	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	ra, err := c.cl.CreateOperatorAccount(ctx, &identityv1.CreateOperatorAccountRequest{
		SessionToken:      raw(req.Token),
		Email:             req.Email,
		DisplayName:       req.DisplayName,
		TemporaryPassword: raw(req.TemporaryPassword),
		PermissionKeys:    req.PermissionKeys,
		ClientIp:          req.ClientIP,
	})
	if err != nil {
		return OperatorAccountResult{}, c.ownerCallFailed(ctx, rpc, err)
	}
	return operatorAccountAnswer(rpc, ra.GetOutcome(), ra.GetAccount(), ra.GetNewPasswordRefusal(), true)
}

// SetOperatorPermissions is OperatorService.SetOperatorPermissions — the COMPLETE key set.
//
//	err == nil          ACCEPTED (Account), SESSION_NOT_LIVE, PERMISSION_DENIED
//	ErrAccountNotFound  404 · ErrArgumentRefused 422 · otherwise 503
func (c *Client) SetOperatorPermissions(ctx context.Context, req SetOperatorPermissionsRequest) (OperatorAccountResult, error) {
	const rpc = "SetOperatorPermissions"
	if req.Token.Rong() || req.TargetCode == "" {
		return OperatorAccountResult{}, fmt.Errorf("%w: %s with an empty token or target", ErrInvalidRequest, rpc)
	}
	keys := req.PermissionKeys
	if keys == nil {
		keys = []string{} // empty IS meaningful (revoke every key); nil and empty travel the same
	}
	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	ra, err := c.cl.SetOperatorPermissions(ctx, &identityv1.SetOperatorPermissionsRequest{
		SessionToken:   raw(req.Token),
		TargetCode:     req.TargetCode,
		PermissionKeys: keys,
		Reason:         req.Reason,
		ClientIp:       req.ClientIP,
	})
	if err != nil {
		return OperatorAccountResult{}, c.ownerCallFailed(ctx, rpc, err)
	}
	return operatorAccountAnswer(rpc, ra.GetOutcome(), ra.GetAccount(), nil, false)
}

// ResetOperatorPassword is OperatorService.ResetOperatorPassword.
//
//	err == nil          ACCEPTED (Account), SESSION_NOT_LIVE, PERMISSION_DENIED, NEW_PASSWORD_REJECTED
//	ErrAccountNotFound  404 · ErrArgumentRefused 422 · otherwise 503
func (c *Client) ResetOperatorPassword(ctx context.Context, req ResetOperatorPasswordRequest) (OperatorAccountResult, error) {
	const rpc = "ResetOperatorPassword"
	if req.Token.Rong() || req.TargetCode == "" {
		return OperatorAccountResult{}, fmt.Errorf("%w: %s with an empty token or target", ErrInvalidRequest, rpc)
	}
	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	ra, err := c.cl.ResetOperatorPassword(ctx, &identityv1.ResetOperatorPasswordRequest{
		SessionToken:      raw(req.Token),
		TargetCode:        req.TargetCode,
		TemporaryPassword: raw(req.TemporaryPassword),
		Reason:            req.Reason,
		ClientIp:          req.ClientIP,
	})
	if err != nil {
		return OperatorAccountResult{}, c.ownerCallFailed(ctx, rpc, err)
	}
	return operatorAccountAnswer(rpc, ra.GetOutcome(), ra.GetAccount(), ra.GetNewPasswordRefusal(), true)
}

func operatorAccountAnswer(rpc string, wire identityv1.OperatorAuthOutcome, a *identityv1.OperatorAccount,
	ref *identityv1.NewPasswordRefusal, setsPassword bool) (OperatorAccountResult, error) {
	allowed := []identityv1.OperatorAuthOutcome{wAccepted, wNotLive, wDenied}
	if setsPassword {
		allowed = append(allowed, wRejected)
	}
	o, err := ownerOutcome(rpc, wire, allowed...)
	if err != nil {
		return OperatorAccountResult{}, err
	}
	res := OperatorAccountResult{Outcome: o}
	switch o {
	case OutcomeAccepted:
		if res.Account, err = toOperatorAccount(rpc, a); err != nil {
			return OperatorAccountResult{}, err
		}
	case OutcomeNewPasswordRejected:
		if res.Refusal, err = refusal(rpc, ref); err != nil {
			return OperatorAccountResult{}, err
		}
	}
	return res, nil
}

// --- a commune's staff accounts (the commune in ctx) ---------------------------------------------------

// ListCommuneStaffAccounts is OperatorService.ListCommuneStaffAccounts, for the commune in ctx. Audited
// by identity (a read of personal data by an operator): call it only for a request a person made.
//
//	err == nil        ACCEPTED (Accounts — empty is valid), SESSION_NOT_LIVE, PERMISSION_DENIED
//	ErrContract       UNSPECIFIED, outside the subset, an account without user_id, a user_id twice
//	ErrInvalidRequest empty token or no commune in ctx — nothing sent
//	ErrNotSupported · any other error → 503
func (c *Client) ListCommuneStaffAccounts(ctx context.Context, token secret.Secret, clientIP string) (StaffAccountsResult, error) {
	const rpc = "ListCommuneStaffAccounts"
	if token.Rong() {
		return StaffAccountsResult{}, fmt.Errorf("%w: %s with an empty token", ErrInvalidRequest, rpc)
	}
	if err := requireCommune(ctx, rpc); err != nil {
		return StaffAccountsResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	ra, err := c.cl.ListCommuneStaffAccounts(ctx, &identityv1.ListCommuneStaffAccountsRequest{
		SessionToken: raw(token),
		ClientIp:     clientIP,
	})
	if err != nil {
		return StaffAccountsResult{}, c.ownerCallFailed(ctx, rpc, err)
	}
	o, err := ownerOutcome(rpc, ra.GetOutcome(), wAccepted, wNotLive, wDenied)
	if err != nil {
		return StaffAccountsResult{}, err
	}
	res := StaffAccountsResult{Outcome: o}
	if o != OutcomeAccepted {
		return res, nil
	}
	res.Accounts = make([]CommuneStaffAccount, 0, len(ra.GetAccounts()))
	seen := make(map[string]bool, len(ra.GetAccounts()))
	for _, a := range ra.GetAccounts() {
		acc, err := toStaffAccount(rpc, a)
		if err != nil {
			return StaffAccountsResult{}, err
		}
		if seen[acc.UserID] {
			return StaffAccountsResult{}, fmt.Errorf("%w: %s named one account twice", ErrContract, rpc)
		}
		seen[acc.UserID] = true
		res.Accounts = append(res.Accounts, acc)
	}
	return res, nil
}

// CreateCommuneAdminAccount is OperatorService.CreateCommuneAdminAccount, for the commune in ctx.
//
//	err == nil          ACCEPTED (Account), SESSION_NOT_LIVE, PERMISSION_DENIED, NEW_PASSWORD_REJECTED
//	ErrAccountExists    409 · ErrCommuneNotLive 409 · ErrArgumentRefused 422
//	ErrInvalidRequest   empty token or no commune — nothing sent
//	ErrNotSupported · other → 503
func (c *Client) CreateCommuneAdminAccount(ctx context.Context, req CreateCommuneAdminAccountRequest) (StaffAccountResult, error) {
	const rpc = "CreateCommuneAdminAccount"
	if req.Token.Rong() {
		return StaffAccountResult{}, fmt.Errorf("%w: %s with an empty token", ErrInvalidRequest, rpc)
	}
	if err := requireCommune(ctx, rpc); err != nil {
		return StaffAccountResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	ra, err := c.cl.CreateCommuneAdminAccount(ctx, &identityv1.CreateCommuneAdminAccountRequest{
		SessionToken:      raw(req.Token),
		Email:             req.Email,
		FullName:          req.FullName,
		TemporaryPassword: raw(req.TemporaryPassword),
		ClientIp:          req.ClientIP,
	})
	if err != nil {
		return StaffAccountResult{}, c.ownerCallFailed(ctx, rpc, err)
	}
	return staffAccountAnswer(rpc, ra.GetOutcome(), ra.GetAccount(), ra.GetNewPasswordRefusal())
}

// ResetStaffPassword is OperatorService.ResetStaffPassword, for the commune in ctx.
//
//	err == nil             ACCEPTED (Account), SESSION_NOT_LIVE, PERMISSION_DENIED, NEW_PASSWORD_REJECTED
//	ErrAccountNotFound     404 · ErrStaffAccountLocked 409 · ErrArgumentRefused 422
//	ErrInvalidRequest      empty token, empty user id or no commune — nothing sent
//	ErrNotSupported · other → 503
func (c *Client) ResetStaffPassword(ctx context.Context, req ResetStaffPasswordRequest) (StaffAccountResult, error) {
	const rpc = "ResetStaffPassword"
	if req.Token.Rong() || req.UserID == "" {
		return StaffAccountResult{}, fmt.Errorf("%w: %s with an empty token or user id", ErrInvalidRequest, rpc)
	}
	if err := requireCommune(ctx, rpc); err != nil {
		return StaffAccountResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	ra, err := c.cl.ResetStaffPassword(ctx, &identityv1.ResetStaffPasswordRequest{
		SessionToken:      raw(req.Token),
		UserId:            req.UserID,
		TemporaryPassword: raw(req.TemporaryPassword),
		Reason:            req.Reason,
		ClientIp:          req.ClientIP,
	})
	if err != nil {
		return StaffAccountResult{}, c.ownerCallFailed(ctx, rpc, err)
	}
	return staffAccountAnswer(rpc, ra.GetOutcome(), ra.GetAccount(), ra.GetNewPasswordRefusal())
}

func staffAccountAnswer(rpc string, wire identityv1.OperatorAuthOutcome, a *identityv1.CommuneStaffAccount,
	ref *identityv1.NewPasswordRefusal) (StaffAccountResult, error) {
	o, err := ownerOutcome(rpc, wire, wAccepted, wNotLive, wDenied, wRejected)
	if err != nil {
		return StaffAccountResult{}, err
	}
	res := StaffAccountResult{Outcome: o}
	switch o {
	case OutcomeAccepted:
		if res.Account, err = toStaffAccount(rpc, a); err != nil {
			return StaffAccountResult{}, err
		}
	case OutcomeNewPasswordRejected:
		if res.Refusal, err = refusal(rpc, ref); err != nil {
			return StaffAccountResult{}, err
		}
	}
	return res, nil
}
