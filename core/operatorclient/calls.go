package operatorclient

import (
	"context"
	"fmt"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/secret"
)

// Short names for the wire outcomes, so each RPC's allowed subset reads as the proto states it.
const (
	wAccepted   = identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_ACCEPTED
	wRefused    = identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_REFUSED
	wEnrollment = identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_ENROLLMENT_REQUIRED
	wNotLive    = identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_SESSION_NOT_LIVE
	wRejected   = identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_NEW_PASSWORD_REJECTED
	wDenied     = identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_PERMISSION_DENIED
)

// OpenRequest signs an operator in: password AND exactly one second factor.
type OpenRequest struct {
	// Email as typed — PERSONAL DATA (rule 3): never logged by the caller either.
	Email        string
	Password     secret.Secret
	TOTPCode     secret.Secret
	RecoveryCode secret.Secret
	// ClientIP is what the edge observed on its own socket (httpx.ClientIP), never X-Forwarded-For.
	ClientIP  string
	UserAgent string
}

// OpenResult is ACCEPTED (Session set), REFUSED or ENROLLMENT_REQUIRED.
type OpenResult struct {
	Outcome Outcome
	Session SessionGrant
}

// Open is OpenOperatorSession.
//
// Both second factors set is ErrInvalidRequest, refused HERE: a property of the request, decided
// before any lookup (the server answers INVALID_ARGUMENT for the same thing) — the caller answers 400.
// Neither set is sent: the server answers REFUSED and COUNTS it, which is the contract.
func (c *Client) Open(ctx context.Context, req OpenRequest) (OpenResult, error) {
	const rpc = "OpenOperatorSession"
	if !req.TOTPCode.Rong() && !req.RecoveryCode.Rong() {
		return OpenResult{}, fmt.Errorf("%w: %s with both a TOTP code and a recovery code", ErrInvalidRequest, rpc)
	}
	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	ra, err := c.cl.OpenOperatorSession(ctx, &identityv1.OpenOperatorSessionRequest{
		Email:        req.Email,
		Password:     raw(req.Password),
		TotpCode:     raw(req.TOTPCode),
		RecoveryCode: raw(req.RecoveryCode),
		ClientIp:     req.ClientIP,
		UserAgent:    req.UserAgent,
	})
	if err != nil {
		return OpenResult{}, c.callFailed(ctx, rpc, err)
	}
	o, err := outcome(rpc, ra.GetOutcome(), wAccepted, wRefused, wEnrollment)
	if err != nil {
		return OpenResult{}, err
	}
	res := OpenResult{Outcome: o}
	if o == OutcomeAccepted {
		if res.Session, err = grant(rpc, ra.GetSession()); err != nil {
			return OpenResult{}, err
		}
	}
	return res, nil
}

// Resolve is ResolveOperatorSession — call it on EVERY operator request a PERSON made, never from a
// background poll: it refreshes the idle timer (operator.proto).
//
//	err != nil   the call did not happen, or identity broke the contract → 503. NEVER "signed out".
//	ok == false  not usable, for any of the reasons the contract deliberately merges → 401, clear
//	             the cookie.
//	ok == true   a live session. PermissionKeys may be empty: signed in, every guarded route 403.
func (c *Client) Resolve(ctx context.Context, token secret.Secret) (Principal, bool, error) {
	const rpc = "ResolveOperatorSession"
	if token.Rong() {
		return Principal{}, false, fmt.Errorf("%w: %s with an empty token — the caller must not call without a cookie", ErrInvalidRequest, rpc)
	}
	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	ra, err := c.cl.ResolveOperatorSession(ctx, &identityv1.ResolveOperatorSessionRequest{SessionToken: raw(token)})
	if err != nil {
		return Principal{}, false, c.callFailed(ctx, rpc, err)
	}
	p := ra.GetPrincipal()
	if p == nil {
		// The ordinary negative — an idle-expired cookie is a daily event. Logs nothing.
		return Principal{}, false, nil
	}
	if p.GetOperatorId() == "" || p.GetOperatorCode() == "" {
		// The contract: a present principal carries both. An empty code would leave every audited
		// write with no "who" (rule 6, invariant 8 — no fallback to the id), so it is a contract
		// fault and an error, never served as a principal.
		c.log.WarnContext(ctx, "CẢNH BÁO HỢP ĐỒNG: ResolveOperatorSession trả chủ thể thiếu operator_id hoặc operator_code")
		return Principal{}, false, fmt.Errorf("%w: %s principal without operator_id or operator_code", ErrContract, rpc)
	}
	// Copied, never aliased; blank keys dropped — dropping can only narrow access, never widen it.
	keys := make([]string, 0, len(p.GetPermissionKeys()))
	for _, k := range p.GetPermissionKeys() {
		if k != "" {
			keys = append(keys, k)
		}
	}
	return Principal{OperatorID: p.GetOperatorId(), OperatorCode: p.GetOperatorCode(), PermissionKeys: keys}, true, nil
}

// Revoke is RevokeOperatorSession: signs out the session NAMED BY THE TOKEN. ACCEPTED or
// SESSION_NOT_LIVE — either way the caller clears the cookie and answers success.
func (c *Client) Revoke(ctx context.Context, token secret.Secret, clientIP string) (Outcome, error) {
	const rpc = "RevokeOperatorSession"
	if token.Rong() {
		return OutcomeRefused, fmt.Errorf("%w: %s with an empty token", ErrInvalidRequest, rpc)
	}
	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	ra, err := c.cl.RevokeOperatorSession(ctx, &identityv1.RevokeOperatorSessionRequest{
		SessionToken: raw(token),
		ClientIp:     clientIP,
	})
	if err != nil {
		return OutcomeRefused, c.callFailed(ctx, rpc, err)
	}
	return outcome(rpc, ra.GetOutcome(), wAccepted, wNotLive)
}

// ChangePasswordRequest re-proves the current password AND a TOTP code.
type ChangePasswordRequest struct {
	Token           secret.Secret
	CurrentPassword secret.Secret
	NewPassword     secret.Secret
	TOTPCode        secret.Secret
	ClientIP        string
}

// ChangePasswordResult: on ACCEPTED every session is revoked — clear the cookie, sign in again.
type ChangePasswordResult struct {
	Outcome Outcome
	// Refusal is set exactly when Outcome is OutcomeNewPasswordRejected.
	Refusal NewPasswordRefusal
}

// ChangePassword is ChangeOperatorPassword.
func (c *Client) ChangePassword(ctx context.Context, req ChangePasswordRequest) (ChangePasswordResult, error) {
	const rpc = "ChangeOperatorPassword"
	if req.Token.Rong() {
		return ChangePasswordResult{}, fmt.Errorf("%w: %s with an empty token", ErrInvalidRequest, rpc)
	}
	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	ra, err := c.cl.ChangeOperatorPassword(ctx, &identityv1.ChangeOperatorPasswordRequest{
		SessionToken:    raw(req.Token),
		CurrentPassword: raw(req.CurrentPassword),
		NewPassword:     raw(req.NewPassword),
		TotpCode:        raw(req.TOTPCode),
		ClientIp:        req.ClientIP,
	})
	if err != nil {
		return ChangePasswordResult{}, c.callFailed(ctx, rpc, err)
	}
	o, err := outcome(rpc, ra.GetOutcome(), wAccepted, wRejected, wRefused, wNotLive)
	if err != nil {
		return ChangePasswordResult{}, err
	}
	res := ChangePasswordResult{Outcome: o}
	if o == OutcomeNewPasswordRejected {
		if res.Refusal, err = refusal(rpc, ra.GetNewPasswordRefusal()); err != nil {
			return ChangePasswordResult{}, err
		}
	}
	return res, nil
}

// RecoveryCodesResult: Codes are set exactly when ACCEPTED — shown ONCE, never logged, cached or
// persisted by the caller.
type RecoveryCodesResult struct {
	Outcome Outcome
	Codes   []secret.Secret
}

// RegenerateRecoveryCodes is RegenerateOperatorRecoveryCodes. Needs a current TOTP code.
func (c *Client) RegenerateRecoveryCodes(ctx context.Context, token, totpCode secret.Secret, clientIP string) (RecoveryCodesResult, error) {
	const rpc = "RegenerateOperatorRecoveryCodes"
	if token.Rong() {
		return RecoveryCodesResult{}, fmt.Errorf("%w: %s with an empty token", ErrInvalidRequest, rpc)
	}
	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	ra, err := c.cl.RegenerateOperatorRecoveryCodes(ctx, &identityv1.RegenerateOperatorRecoveryCodesRequest{
		SessionToken: raw(token),
		TotpCode:     raw(totpCode),
		ClientIp:     clientIP,
	})
	if err != nil {
		return RecoveryCodesResult{}, c.callFailed(ctx, rpc, err)
	}
	o, err := outcome(rpc, ra.GetOutcome(), wAccepted, wRefused, wNotLive)
	if err != nil {
		return RecoveryCodesResult{}, err
	}
	res := RecoveryCodesResult{Outcome: o}
	if o == OutcomeAccepted {
		if res.Codes, err = codes2secrets(rpc, ra.GetRecoveryCodes()); err != nil {
			return RecoveryCodesResult{}, err
		}
	}
	return res, nil
}

// BeginEnrollmentRequest is the first step of a first sign-in: the temporary password.
type BeginEnrollmentRequest struct {
	// Email — PERSONAL DATA (rule 3).
	Email             string
	TemporaryPassword secret.Secret
	ClientIP          string
}

// BeginEnrollmentResult: on ACCEPTED, ProvisioningURI (contains the TOTP secret) and ManualEntryKey
// are rendered to the operator ONCE and never logged, cached or persisted.
type BeginEnrollmentResult struct {
	Outcome         Outcome
	OperatorCode    string
	ProvisioningURI secret.Secret
	ManualEntryKey  secret.Secret
}

// BeginEnrollment is BeginOperatorEnrollment.
func (c *Client) BeginEnrollment(ctx context.Context, req BeginEnrollmentRequest) (BeginEnrollmentResult, error) {
	const rpc = "BeginOperatorEnrollment"
	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	ra, err := c.cl.BeginOperatorEnrollment(ctx, &identityv1.BeginOperatorEnrollmentRequest{
		Email:             req.Email,
		TemporaryPassword: raw(req.TemporaryPassword),
		ClientIp:          req.ClientIP,
	})
	if err != nil {
		return BeginEnrollmentResult{}, c.callFailed(ctx, rpc, err)
	}
	o, err := outcome(rpc, ra.GetOutcome(), wAccepted, wRefused)
	if err != nil {
		return BeginEnrollmentResult{}, err
	}
	res := BeginEnrollmentResult{Outcome: o}
	if o == OutcomeAccepted {
		if ra.GetOperatorCode() == "" || ra.GetProvisioningUri() == "" || ra.GetManualEntryKey() == "" {
			return BeginEnrollmentResult{}, fmt.Errorf("%w: %s ACCEPTED without its enrolment data", ErrContract, rpc)
		}
		res.OperatorCode = ra.GetOperatorCode()
		res.ProvisioningURI = secret.Secret(ra.GetProvisioningUri())
		res.ManualEntryKey = secret.Secret(ra.GetManualEntryKey())
	}
	return res, nil
}

// CompleteEnrollmentRequest is the second step: temporary password + new password + a code from the
// pending secret.
type CompleteEnrollmentRequest struct {
	// Email — PERSONAL DATA (rule 3).
	Email             string
	TemporaryPassword secret.Secret
	NewPassword       secret.Secret
	TOTPCode          secret.Secret
	ClientIP          string
	UserAgent         string
}

// CompleteEnrollmentResult: ACCEPTED sets Session and RecoveryCodes; NEW_PASSWORD_REJECTED sets
// Refusal.
type CompleteEnrollmentResult struct {
	Outcome       Outcome
	Session       SessionGrant
	RecoveryCodes []secret.Secret
	Refusal       NewPasswordRefusal
}

// CompleteEnrollment is CompleteOperatorEnrollment.
func (c *Client) CompleteEnrollment(ctx context.Context, req CompleteEnrollmentRequest) (CompleteEnrollmentResult, error) {
	const rpc = "CompleteOperatorEnrollment"
	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	ra, err := c.cl.CompleteOperatorEnrollment(ctx, &identityv1.CompleteOperatorEnrollmentRequest{
		Email:             req.Email,
		TemporaryPassword: raw(req.TemporaryPassword),
		NewPassword:       raw(req.NewPassword),
		TotpCode:          raw(req.TOTPCode),
		ClientIp:          req.ClientIP,
		UserAgent:         req.UserAgent,
	})
	if err != nil {
		return CompleteEnrollmentResult{}, c.callFailed(ctx, rpc, err)
	}
	o, err := outcome(rpc, ra.GetOutcome(), wAccepted, wRejected, wRefused)
	if err != nil {
		return CompleteEnrollmentResult{}, err
	}
	res := CompleteEnrollmentResult{Outcome: o}
	switch o {
	case OutcomeAccepted:
		if res.Session, err = grant(rpc, ra.GetSession()); err != nil {
			return CompleteEnrollmentResult{}, err
		}
		if res.RecoveryCodes, err = codes2secrets(rpc, ra.GetRecoveryCodes()); err != nil {
			return CompleteEnrollmentResult{}, err
		}
	case OutcomeNewPasswordRejected:
		if res.Refusal, err = refusal(rpc, ra.GetNewPasswordRefusal()); err != nil {
			return CompleteEnrollmentResult{}, err
		}
	}
	return res, nil
}
