package grpc

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"unicode/utf8"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// OperatorService — the Vihat OPERATOR realm, served on identity's inter-service port to one caller,
// service-platform (proto/vigov/identity/v1/operator.proto, ADR 0048 §01/10). The use cases are
// app.OperatorAuth (operator_auth.go, operator_enrollment.go); this file only translates.
//
// # WHY A SEPARATE SERVER TYPE AND NOT METHODS ON Server
//
// Server serves the staff and citizen realms of ONE commune and every handler but one starts with
// s.xa(ctx). Nothing here may: an operator belongs to no commune, every RPC of this service is on
// core/grpcx.methodsWithoutTenant, and there is no commune to read — inventing one would be a
// default on the isolation path (rule 1, forbidden #1). A type of its own has no s.xa to call and
// no staff store to reach, so a staff token cannot be resolved here by a careless edit either (ADR
// 0048 stop condition #6).
//
// # NEVER LOG A MESSAGE OF THIS SERVICE
//
// The generated String() prints every field (debug_redact is a no-op, measured 2026-09-20): requests
// carry passwords, TOTP codes, recovery codes, bearer tokens and an email; responses carry bearer
// tokens, TOTP secrets and recovery codes. The use case logs the business code and the outcome; this
// file logs ONE thing, the cause of an Internal answer, and never a field of the request.
//
// # NOTHING IS AUDITED HERE
//
// Every write these RPCs cause is audited by the use case, in operator_audit_log, in the transaction
// of the write (rule 6, invariant 3), with the operator's VH- code as actor. A second entry written
// here could not even name a "who" (ADR 0025: the caller key proves the deployment, not the service).

// userAgentMaxRunes is the contract's bound on operator_session.user_agent (operator.proto,
// OpenOperatorSessionRequest.user_agent): the first 256 runes are KEPT, the rest dropped, never
// refused — a diagnostic field must not fail a sign-in.
const userAgentMaxRunes = 256

// OperatorAuthenticator is the use case behind the seven RPCs. *app.OperatorAuth satisfies it as it
// stands; declared here, at the point of use, so every branch of the status table is testable
// without a PostgreSQL.
type OperatorAuthenticator interface {
	Login(ctx context.Context, in app.OperatorLoginRequest) (app.OperatorSessionIssued, error)
	ResolveSession(ctx context.Context, token string) (app.OperatorPrincipal, error)
	Logout(ctx context.Context, token, ip string) error
	ChangePassword(ctx context.Context, in app.OperatorPasswordChange) error
	RegenerateRecoveryCodes(ctx context.Context, token, totpCode, ip string) ([]secret.Secret, error)
	BeginEnrollment(ctx context.Context, in app.OperatorEnrollmentRequest) (app.OperatorEnrollmentStart, error)
	CompleteEnrollment(ctx context.Context, in app.OperatorEnrollmentCompletion) (app.OperatorEnrollmentResult, error)
}

// OperatorServer implements identityv1.OperatorServiceServer. The embedded Unimplemented server is
// the seam an eighth RPC arrives through: Unimplemented, never a stub that answers something.
type OperatorServer struct {
	identityv1.UnimplementedOperatorServiceServer

	uc  OperatorAuthenticator
	log *slog.Logger
}

// NewOperatorServer refuses a missing use case at construction, where a human watches a process
// start. An UNCONFIGURED realm is not a missing use case: app.OperatorAuth built without keys is a
// valid value whose every flow answers ErrOperatorRealmNotConfigured → FAILED_PRECONDITION.
func NewOperatorServer(uc OperatorAuthenticator, log *slog.Logger) *OperatorServer {
	if uc == nil {
		panic("identity/grpc: NewOperatorServer without a use case — every OperatorService RPC would panic")
	}
	if log == nil {
		log = slog.Default()
	}
	return &OperatorServer{uc: uc, log: log}
}

// Short names for the wire outcomes, so each RPC reads as the subset the proto lists for it.
const (
	outAccepted   = identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_ACCEPTED
	outRefused    = identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_REFUSED
	outEnrollment = identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_ENROLLMENT_REQUIRED
	outNotLive    = identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_SESSION_NOT_LIVE
	outRejected   = identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_NEW_PASSWORD_REJECTED
)

// errEmptyToken is the wiring fault every session-bound RPC refuses before any lookup. One value, so
// the four RPCs cannot drift into four different answers for the same fault.
var errEmptyToken = status.Error(codes.InvalidArgument, "session_token trống")

// OpenOperatorSession — ACCEPTED, REFUSED or ENROLLMENT_REQUIRED.
func (s *OperatorServer) OpenOperatorSession(ctx context.Context, req *identityv1.OpenOperatorSessionRequest) (
	*identityv1.OpenOperatorSessionResponse, error) {

	// NEVER LOG req.
	//
	// Both factors is decided HERE, from the request alone and before the use case, so the answer
	// is INVALID_ARGUMENT even when the realm is not configured: it says nothing about the
	// deployment or any account. The use case checks it again (ErrOperatorSecondFactorAmbiguous).
	if req.GetTotpCode() != "" && req.GetRecoveryCode() != "" {
		return nil, status.Error(codes.InvalidArgument, "gửi mã TOTP hoặc mã khôi phục, không gửi cả hai")
	}
	issued, err := s.uc.Login(ctx, app.OperatorLoginRequest{
		Email:        req.GetEmail(),
		Password:     req.GetPassword(),
		TOTPCode:     req.GetTotpCode(),
		RecoveryCode: req.GetRecoveryCode(),
		IP:           req.GetClientIp(),
		UserAgent:    clipUserAgent(req.GetUserAgent()),
	})
	switch {
	case err == nil:
		return &identityv1.OpenOperatorSessionResponse{Outcome: outAccepted, Session: sessionGrant(issued)}, nil
	case errors.Is(err, app.ErrOperatorLoginFailed):
		return &identityv1.OpenOperatorSessionResponse{Outcome: outRefused}, nil
	case errors.Is(err, app.ErrOperatorEnrollmentRequired):
		return &identityv1.OpenOperatorSessionResponse{Outcome: outEnrollment}, nil
	}
	return nil, s.fault(ctx, "OpenOperatorSession", err)
}

// ResolveOperatorSession — OK + principal, or OK + no principal for every unusable token.
func (s *OperatorServer) ResolveOperatorSession(ctx context.Context, req *identityv1.ResolveOperatorSessionRequest) (
	*identityv1.ResolveOperatorSessionResponse, error) {

	tok := req.GetSessionToken()
	if tok == "" {
		return nil, errEmptyToken
	}
	p, err := s.uc.ResolveSession(ctx, tok)
	if errors.Is(err, app.ErrOperatorUnauthenticated) {
		// ONE ANSWER for a bad signature, a staff `v1.` token, expiry, idle expiry, revocation and a
		// disabled account. No log line: an idle-expired cookie is a daily event, and a line per
		// probe is a line per probe.
		return &identityv1.ResolveOperatorSessionResponse{}, nil
	}
	if err != nil {
		return nil, s.fault(ctx, "ResolveOperatorSession", err)
	}
	if p.ID == "" || p.Code == "" {
		// A contract fault in THIS service, never served as a principal: a principal with no VH-
		// code leaves every audited write downstream with no "who" (rule 6, invariant 8, no
		// fallback to the id). The use case cannot produce it today; checked because serving it
		// would be silent.
		s.log.ErrorContext(ctx, "CẢNH BÁO HỢP ĐỒNG: phiên vận hành không có operator_id hoặc operator_code")
		return nil, status.Error(codes.Internal, "lỗi nội bộ, vui lòng thử lại")
	}
	keys := make([]string, 0, len(p.Permissions))
	for _, k := range p.Permissions {
		keys = append(keys, string(k))
	}
	return &identityv1.ResolveOperatorSessionResponse{Principal: &identityv1.OperatorPrincipal{
		OperatorId:     p.ID,
		OperatorCode:   p.Code,
		PermissionKeys: keys,
	}}, nil
}

// RevokeOperatorSession — ACCEPTED or SESSION_NOT_LIVE.
func (s *OperatorServer) RevokeOperatorSession(ctx context.Context, req *identityv1.RevokeOperatorSessionRequest) (
	*identityv1.RevokeOperatorSessionResponse, error) {

	if req.GetSessionToken() == "" {
		return nil, errEmptyToken
	}
	err := s.uc.Logout(ctx, req.GetSessionToken(), req.GetClientIp())
	switch {
	case err == nil:
		return &identityv1.RevokeOperatorSessionResponse{Outcome: outAccepted}, nil
	case errors.Is(err, app.ErrOperatorUnauthenticated):
		return &identityv1.RevokeOperatorSessionResponse{Outcome: outNotLive}, nil
	}
	return nil, s.fault(ctx, "RevokeOperatorSession", err)
}

// ChangeOperatorPassword — ACCEPTED, NEW_PASSWORD_REJECTED, REFUSED or SESSION_NOT_LIVE.
func (s *OperatorServer) ChangeOperatorPassword(ctx context.Context, req *identityv1.ChangeOperatorPasswordRequest) (
	*identityv1.ChangeOperatorPasswordResponse, error) {

	if req.GetSessionToken() == "" {
		return nil, errEmptyToken
	}
	err := s.uc.ChangePassword(ctx, app.OperatorPasswordChange{
		Token:           req.GetSessionToken(),
		CurrentPassword: req.GetCurrentPassword(),
		NewPassword:     req.GetNewPassword(),
		TOTPCode:        req.GetTotpCode(),
		IP:              req.GetClientIp(),
	})
	if r, ok := newPasswordRefusal(err); ok {
		return &identityv1.ChangeOperatorPasswordResponse{Outcome: outRejected, NewPasswordRefusal: r}, nil
	}
	switch {
	case err == nil:
		return &identityv1.ChangeOperatorPasswordResponse{Outcome: outAccepted}, nil
	case errors.Is(err, app.ErrOperatorLoginFailed):
		return &identityv1.ChangeOperatorPasswordResponse{Outcome: outRefused}, nil
	case errors.Is(err, app.ErrOperatorUnauthenticated):
		return &identityv1.ChangeOperatorPasswordResponse{Outcome: outNotLive}, nil
	}
	return nil, s.fault(ctx, "ChangeOperatorPassword", err)
}

// RegenerateOperatorRecoveryCodes — ACCEPTED, REFUSED or SESSION_NOT_LIVE.
func (s *OperatorServer) RegenerateOperatorRecoveryCodes(ctx context.Context, req *identityv1.RegenerateOperatorRecoveryCodesRequest) (
	*identityv1.RegenerateOperatorRecoveryCodesResponse, error) {

	if req.GetSessionToken() == "" {
		return nil, errEmptyToken
	}
	codesIssued, err := s.uc.RegenerateRecoveryCodes(ctx, req.GetSessionToken(), req.GetTotpCode(), req.GetClientIp())
	switch {
	case err == nil:
		return &identityv1.RegenerateOperatorRecoveryCodesResponse{
			Outcome: outAccepted, RecoveryCodes: rawCodes(codesIssued),
		}, nil
	case errors.Is(err, app.ErrOperatorLoginFailed):
		return &identityv1.RegenerateOperatorRecoveryCodesResponse{Outcome: outRefused}, nil
	case errors.Is(err, app.ErrOperatorUnauthenticated):
		return &identityv1.RegenerateOperatorRecoveryCodesResponse{Outcome: outNotLive}, nil
	}
	return nil, s.fault(ctx, "RegenerateOperatorRecoveryCodes", err)
}

// BeginOperatorEnrollment — ACCEPTED or REFUSED.
func (s *OperatorServer) BeginOperatorEnrollment(ctx context.Context, req *identityv1.BeginOperatorEnrollmentRequest) (
	*identityv1.BeginOperatorEnrollmentResponse, error) {

	start, err := s.uc.BeginEnrollment(ctx, app.OperatorEnrollmentRequest{
		Email:             req.GetEmail(),
		TemporaryPassword: req.GetTemporaryPassword(),
		IP:                req.GetClientIp(),
	})
	switch {
	case err == nil:
		// .Lo() here and nowhere else: the wire is the one place the secret must leave in clear,
		// to be rendered to the operator once (operator.proto, provisioning_uri).
		return &identityv1.BeginOperatorEnrollmentResponse{
			Outcome:         outAccepted,
			OperatorCode:    start.AccountCode,
			ProvisioningUri: string(start.ProvisioningURI.Lo()),
			ManualEntryKey:  string(start.ManualEntryKey.Lo()),
		}, nil
	case errors.Is(err, app.ErrOperatorLoginFailed):
		return &identityv1.BeginOperatorEnrollmentResponse{Outcome: outRefused}, nil
	}
	return nil, s.fault(ctx, "BeginOperatorEnrollment", err)
}

// CompleteOperatorEnrollment — ACCEPTED, NEW_PASSWORD_REJECTED or REFUSED.
func (s *OperatorServer) CompleteOperatorEnrollment(ctx context.Context, req *identityv1.CompleteOperatorEnrollmentRequest) (
	*identityv1.CompleteOperatorEnrollmentResponse, error) {

	res, err := s.uc.CompleteEnrollment(ctx, app.OperatorEnrollmentCompletion{
		Email:             req.GetEmail(),
		TemporaryPassword: req.GetTemporaryPassword(),
		NewPassword:       req.GetNewPassword(),
		TOTPCode:          req.GetTotpCode(),
		IP:                req.GetClientIp(),
		UserAgent:         clipUserAgent(req.GetUserAgent()),
	})
	if r, ok := newPasswordRefusal(err); ok {
		return &identityv1.CompleteOperatorEnrollmentResponse{Outcome: outRejected, NewPasswordRefusal: r}, nil
	}
	switch {
	case err == nil:
		return &identityv1.CompleteOperatorEnrollmentResponse{
			Outcome:       outAccepted,
			Session:       sessionGrant(res.OperatorSessionIssued),
			RecoveryCodes: rawCodes(res.RecoveryCodes),
		}, nil
	case errors.Is(err, app.ErrOperatorLoginFailed):
		return &identityv1.CompleteOperatorEnrollmentResponse{Outcome: outRefused}, nil
	}
	return nil, s.fault(ctx, "CompleteOperatorEnrollment", err)
}

// fault maps everything that is not a decision onto the status table. ANY sentinel an RPC does not
// list for itself lands in the default branch — Internal, fail closed — rather than being guessed
// into an outcome the caller would act on.
//
// The message never carries the cause: it crosses into another process's logs. The cause is logged
// HERE, with the RPC name; it is a wrapped store or signing error and holds no field of the request.
func (s *OperatorServer) fault(ctx context.Context, rpc string, err error) error {
	switch {
	case errors.Is(err, app.ErrOperatorRealmNotConfigured):
		// A deployment fault, reported once at startup by cmd/server. Not logged per call: the caller
		// answers 503 and alerts (operator.proto, FAILED_PRECONDITION).
		return status.Error(codes.FailedPrecondition, "miền vận hành chưa được cấu hình")
	case errors.Is(err, app.ErrOperatorSecondFactorAmbiguous):
		return status.Error(codes.InvalidArgument, "gửi mã TOTP hoặc mã khôi phục, không gửi cả hai")
	case ctx.Err() != nil:
		// The caller went away or its deadline passed: the transaction rolled back, nothing was
		// decided. Canceled / DeadlineExceeded, never Internal.
		return status.FromContextError(ctx.Err()).Err()
	}
	s.log.ErrorContext(ctx, "identity/grpc: OperatorService thất bại", "rpc", rpc, "err", err)
	return status.Error(codes.Internal, "lỗi nội bộ, vui lòng thử lại")
}

// newPasswordRefusal maps the staff password policy's errors (domain/mat_khau.go) onto the wire.
// The bounds are sent from the domain constants so the console never holds its own "12".
func newPasswordRefusal(err error) (*identityv1.NewPasswordRefusal, bool) {
	var p identityv1.NewPasswordProblem
	switch {
	case err == nil:
		return nil, false
	case errors.Is(err, domain.ErrThieuMatKhau):
		p = identityv1.NewPasswordProblem_NEW_PASSWORD_PROBLEM_EMPTY
	case errors.Is(err, domain.ErrMatKhauKhongDoc):
		p = identityv1.NewPasswordProblem_NEW_PASSWORD_PROBLEM_NOT_UTF8
	case errors.Is(err, domain.ErrMatKhauQuaNgan):
		p = identityv1.NewPasswordProblem_NEW_PASSWORD_PROBLEM_TOO_SHORT
	case errors.Is(err, domain.ErrMatKhauQuaDai):
		p = identityv1.NewPasswordProblem_NEW_PASSWORD_PROBLEM_TOO_LONG
	case errors.Is(err, domain.ErrMatKhauMoiTrungCu):
		p = identityv1.NewPasswordProblem_NEW_PASSWORD_PROBLEM_SAME_AS_CURRENT
	default:
		return nil, false
	}
	return &identityv1.NewPasswordRefusal{
		Problem:   p,
		MinLength: domain.DaiMatKhauToiThieu,
		MaxLength: domain.DaiMatKhauToiDa,
	}, true
}

func sessionGrant(in app.OperatorSessionIssued) *identityv1.OperatorSessionGrant {
	return &identityv1.OperatorSessionGrant{
		SessionToken: in.Token,
		ExpiresAt:    timestamppb.New(in.ExpiresAt),
		OperatorCode: in.AccountCode,
	}
}

// rawCodes unwraps the recovery codes for the wire — the one place they leave as plain strings.
func rawCodes(in []secret.Secret) []string {
	out := make([]string, 0, len(in))
	for _, c := range in {
		out = append(out, string(c.Lo()))
	}
	return out
}

// clipUserAgent keeps the first userAgentMaxRunes runes of a diagnostic field.
//
// Also repairs what PostgreSQL TEXT refuses — invalid UTF-8 and NUL — because the contract says
// this field must not fail a sign-in, and either byte would turn the session INSERT into an
// Internal answer: a sign-in lost to a browser string.
func clipUserAgent(ua string) string {
	ua = strings.ToValidUTF8(ua, "�")
	ua = strings.ReplaceAll(ua, "\x00", "")
	if utf8.RuneCountInString(ua) <= userAgentMaxRunes {
		return ua
	}
	n := 0
	for i := range ua {
		if n == userAgentMaxRunes {
			return ua[:i]
		}
		n++
	}
	return ua
}
