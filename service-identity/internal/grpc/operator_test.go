package grpc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/token"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	"github.com/vihat/vigov/service-identity/internal/operatorauth"
)

// Credential markers: unique strings that must never appear in any log line. Fake by construction.
const (
	opMarkEmail    = "op-marker-email@example.test"
	opMarkPassword = "OP-MARKER-PASSWORD-not-real"
	opMarkNewPw    = "OP-MARKER-NEW-PASSWORD-not-real"
	opMarkTOTP     = "918273"
	opMarkRecovery = "OP-MARKER-RECOVERY-not-real"
	opMarkToken    = "op1.OP-MARKER-TOKEN.not-real"
	opMarkUA       = "OP-MARKER-USER-AGENT"
)

// operatorAuthFake answers every use case with the configured err (nil = success) and records
// what it was given.
type operatorAuthFake struct {
	err       error
	calls     int
	principal app.OperatorPrincipal

	gotLogin      app.OperatorLoginRequest
	gotChange     app.OperatorPasswordChange
	gotBegin      app.OperatorEnrollmentRequest
	gotComplete   app.OperatorEnrollmentCompletion
	gotToken      string
	gotIP, gotOTP string
}

var opExpires = time.Date(2026, 10, 1, 16, 0, 0, 0, time.UTC)

func (f *operatorAuthFake) Login(_ context.Context, in app.OperatorLoginRequest) (app.OperatorSessionIssued, error) {
	f.calls++
	f.gotLogin = in
	if f.err != nil {
		return app.OperatorSessionIssued{}, f.err
	}
	return app.OperatorSessionIssued{Token: "op1.tok", ExpiresAt: opExpires, AccountCode: "VH-00001"}, nil
}

func (f *operatorAuthFake) ResolveSession(_ context.Context, tok string) (app.OperatorPrincipal, error) {
	f.calls++
	f.gotToken = tok
	return f.principal, f.err
}

func (f *operatorAuthFake) Logout(_ context.Context, tok, ip string) error {
	f.calls++
	f.gotToken, f.gotIP = tok, ip
	return f.err
}

func (f *operatorAuthFake) ChangePassword(_ context.Context, in app.OperatorPasswordChange) error {
	f.calls++
	f.gotChange = in
	return f.err
}

func (f *operatorAuthFake) RegenerateRecoveryCodes(_ context.Context, tok, otp, ip string) ([]secret.Secret, error) {
	f.calls++
	f.gotToken, f.gotOTP, f.gotIP = tok, otp, ip
	if f.err != nil {
		return nil, f.err
	}
	return []secret.Secret{secret.Secret("code-1"), secret.Secret("code-2")}, nil
}

func (f *operatorAuthFake) BeginEnrollment(_ context.Context, in app.OperatorEnrollmentRequest) (app.OperatorEnrollmentStart, error) {
	f.calls++
	f.gotBegin = in
	if f.err != nil {
		return app.OperatorEnrollmentStart{}, f.err
	}
	return app.OperatorEnrollmentStart{AccountCode: "VH-00001",
		ProvisioningURI: secret.Secret("otpauth://totp/ViGov:VH-00001?secret=X"), ManualEntryKey: secret.Secret("X")}, nil
}

func (f *operatorAuthFake) CompleteEnrollment(_ context.Context, in app.OperatorEnrollmentCompletion) (app.OperatorEnrollmentResult, error) {
	f.calls++
	f.gotComplete = in
	if f.err != nil {
		return app.OperatorEnrollmentResult{}, f.err
	}
	return app.OperatorEnrollmentResult{
		OperatorSessionIssued: app.OperatorSessionIssued{Token: "op1.tok", ExpiresAt: opExpires, AccountCode: "VH-00001"},
		RecoveryCodes:         []secret.Secret{secret.Secret("code-1")},
	}, nil
}

// opRPC calls one RPC with every credential field set and returns its outcome (UNSPECIFIED for
// Resolve, which has none) and its error.
type opRPC func(s *OperatorServer) (identityv1.OperatorAuthOutcome, error)

func opRPCs() map[string]opRPC {
	ctx := context.Background()
	return map[string]opRPC{
		"OpenOperatorSession": func(s *OperatorServer) (identityv1.OperatorAuthOutcome, error) {
			r, err := s.OpenOperatorSession(ctx, &identityv1.OpenOperatorSessionRequest{Email: opMarkEmail,
				Password: opMarkPassword, TotpCode: opMarkTOTP, ClientIp: "10.0.0.9", UserAgent: opMarkUA})
			return r.GetOutcome(), err
		},
		"ResolveOperatorSession": func(s *OperatorServer) (identityv1.OperatorAuthOutcome, error) {
			_, err := s.ResolveOperatorSession(ctx, &identityv1.ResolveOperatorSessionRequest{SessionToken: opMarkToken})
			return identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_UNSPECIFIED, err
		},
		"RevokeOperatorSession": func(s *OperatorServer) (identityv1.OperatorAuthOutcome, error) {
			r, err := s.RevokeOperatorSession(ctx, &identityv1.RevokeOperatorSessionRequest{SessionToken: opMarkToken, ClientIp: "10.0.0.9"})
			return r.GetOutcome(), err
		},
		"ChangeOperatorPassword": func(s *OperatorServer) (identityv1.OperatorAuthOutcome, error) {
			r, err := s.ChangeOperatorPassword(ctx, &identityv1.ChangeOperatorPasswordRequest{SessionToken: opMarkToken,
				CurrentPassword: opMarkPassword, NewPassword: opMarkNewPw, TotpCode: opMarkTOTP, ClientIp: "10.0.0.9"})
			return r.GetOutcome(), err
		},
		"RegenerateOperatorRecoveryCodes": func(s *OperatorServer) (identityv1.OperatorAuthOutcome, error) {
			r, err := s.RegenerateOperatorRecoveryCodes(ctx, &identityv1.RegenerateOperatorRecoveryCodesRequest{
				SessionToken: opMarkToken, TotpCode: opMarkTOTP, ClientIp: "10.0.0.9"})
			return r.GetOutcome(), err
		},
		"BeginOperatorEnrollment": func(s *OperatorServer) (identityv1.OperatorAuthOutcome, error) {
			r, err := s.BeginOperatorEnrollment(ctx, &identityv1.BeginOperatorEnrollmentRequest{Email: opMarkEmail,
				TemporaryPassword: opMarkPassword, ClientIp: "10.0.0.9"})
			return r.GetOutcome(), err
		},
		"CompleteOperatorEnrollment": func(s *OperatorServer) (identityv1.OperatorAuthOutcome, error) {
			r, err := s.CompleteOperatorEnrollment(ctx, &identityv1.CompleteOperatorEnrollmentRequest{Email: opMarkEmail,
				TemporaryPassword: opMarkPassword, NewPassword: opMarkNewPw, TotpCode: opMarkTOTP, ClientIp: "10.0.0.9",
				UserAgent: opMarkUA})
			return r.GetOutcome(), err
		},
	}
}

// THE STATUS TABLE of operator.proto, one row per (RPC, use-case answer). A sentinel an RPC does not
// list for itself (ENROLLMENT_REQUIRED from Begin, "not signed in" from a sign-in) is Internal —
// fail closed, never guessed into an outcome the caller would act on.
func TestOperatorServerOutcomeTable(t *testing.T) {
	const (
		ok          = codes.OK
		accepted    = identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_ACCEPTED
		refused     = identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_REFUSED
		enrollment  = identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_ENROLLMENT_REQUIRED
		notLive     = identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_SESSION_NOT_LIVE
		rejected    = identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_NEW_PASSWORD_REJECTED
		unspecified = identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_UNSPECIFIED
	)
	wrap := func(e error) error { return fmt.Errorf("operator x: %w", e) }
	type row struct {
		err  error
		code codes.Code
		out  identityv1.OperatorAuthOutcome
	}
	table := map[string][]row{
		"OpenOperatorSession": {
			{nil, ok, accepted},
			{wrap(app.ErrOperatorLoginFailed), ok, refused},
			{wrap(app.ErrOperatorEnrollmentRequired), ok, enrollment},
			{wrap(app.ErrOperatorSecondFactorAmbiguous), codes.InvalidArgument, unspecified},
			{wrap(app.ErrOperatorUnauthenticated), codes.Internal, unspecified},
			{errors.New("disk full"), codes.Internal, unspecified},
		},
		"RevokeOperatorSession": {
			{nil, ok, accepted},
			{wrap(app.ErrOperatorUnauthenticated), ok, notLive},
			{wrap(app.ErrOperatorLoginFailed), codes.Internal, unspecified},
			{errors.New("disk full"), codes.Internal, unspecified},
		},
		"ChangeOperatorPassword": {
			{nil, ok, accepted},
			{wrap(domain.ErrMatKhauQuaNgan), ok, rejected},
			{wrap(app.ErrOperatorLoginFailed), ok, refused},
			{wrap(app.ErrOperatorUnauthenticated), ok, notLive},
			{wrap(app.ErrOperatorEnrollmentRequired), codes.Internal, unspecified},
			{errors.New("disk full"), codes.Internal, unspecified},
		},
		"RegenerateOperatorRecoveryCodes": {
			{nil, ok, accepted},
			{wrap(app.ErrOperatorLoginFailed), ok, refused},
			{wrap(app.ErrOperatorUnauthenticated), ok, notLive},
			{errors.New("disk full"), codes.Internal, unspecified},
		},
		"BeginOperatorEnrollment": {
			{nil, ok, accepted},
			{wrap(app.ErrOperatorLoginFailed), ok, refused},
			{wrap(app.ErrOperatorEnrollmentRequired), codes.Internal, unspecified},
			{errors.New("disk full"), codes.Internal, unspecified},
		},
		"CompleteOperatorEnrollment": {
			{nil, ok, accepted},
			{wrap(domain.ErrMatKhauMoiTrungCu), ok, rejected},
			{wrap(app.ErrOperatorLoginFailed), ok, refused},
			{wrap(app.ErrOperatorUnauthenticated), codes.Internal, unspecified},
			{errors.New("disk full"), codes.Internal, unspecified},
		},
	}
	rpcs := opRPCs()
	for name, rows := range table {
		for _, r := range rows {
			t.Run(fmt.Sprintf("%s/%v", name, r.err), func(t *testing.T) {
				s := NewOperatorServer(&operatorAuthFake{err: r.err}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
				out, err := rpcs[name](s)
				if status.Code(err) != r.code || out != r.out {
					t.Fatalf("got (%v, %v), want (%v, %v) — err %v", status.Code(err), out, r.code, r.out, err)
				}
				// The cause never crosses the boundary.
				if err != nil && strings.Contains(status.Convert(err).Message(), "disk full") {
					t.Fatalf("message carries the internal cause: %q", status.Convert(err).Message())
				}
				// UNAUTHENTICATED and RESOURCE_EXHAUSTED are never produced by these RPCs.
				if c := status.Code(err); c == codes.Unauthenticated || c == codes.ResourceExhausted {
					t.Fatalf("produced %v, which belongs to the interceptor / the caller's rate limit", c)
				}
			})
		}
	}
}

// FAILED_PRECONDITION on all seven when identity has no operator keys — a deployment fault, never
// REFUSED (the caller answers 503 and alerts).
func TestOperatorServerRealmNotConfiguredIsFailedPrecondition(t *testing.T) {
	for name, call := range opRPCs() {
		s := NewOperatorServer(&operatorAuthFake{err: fmt.Errorf("x: %w", app.ErrOperatorRealmNotConfigured)}, nil)
		if out, err := call(s); status.Code(err) != codes.FailedPrecondition || out != outUnspecified() {
			t.Errorf("%s: got (%v, %v), want FailedPrecondition", name, status.Code(err), out)
		}
	}
}

func outUnspecified() identityv1.OperatorAuthOutcome {
	return identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_UNSPECIFIED
}

// An empty session_token is a wiring fault decided from the request alone: INVALID_ARGUMENT, and the
// use case is never entered — nothing counted, nothing written, the idle timer not touched.
func TestOperatorServerEmptyTokenIsInvalidArgument(t *testing.T) {
	ctx := context.Background()
	f := &operatorAuthFake{}
	s := NewOperatorServer(f, nil)
	calls := map[string]func() error{
		"Resolve": func() error {
			_, err := s.ResolveOperatorSession(ctx, &identityv1.ResolveOperatorSessionRequest{})
			return err
		},
		"Revoke": func() error {
			_, err := s.RevokeOperatorSession(ctx, &identityv1.RevokeOperatorSessionRequest{ClientIp: "10.0.0.9"})
			return err
		},
		"ChangePassword": func() error {
			_, err := s.ChangeOperatorPassword(ctx, &identityv1.ChangeOperatorPasswordRequest{NewPassword: opMarkNewPw})
			return err
		},
		"Regenerate": func() error {
			_, err := s.RegenerateOperatorRecoveryCodes(ctx, &identityv1.RegenerateOperatorRecoveryCodesRequest{TotpCode: opMarkTOTP})
			return err
		},
	}
	for name, call := range calls {
		if err := call(); status.Code(err) != codes.InvalidArgument {
			t.Errorf("%s with empty token: %v, want InvalidArgument", name, status.Code(err))
		}
	}
	if f.calls != 0 {
		t.Fatalf("use case entered %d times for an empty token", f.calls)
	}
}

// Both factors: INVALID_ARGUMENT before the use case — even when the realm is unconfigured, because
// it is a property of the request and says nothing about the deployment.
func TestOperatorServerBothFactorsIsInvalidArgumentBeforeUseCase(t *testing.T) {
	f := &operatorAuthFake{err: app.ErrOperatorRealmNotConfigured}
	s := NewOperatorServer(f, nil)
	_, err := s.OpenOperatorSession(context.Background(), &identityv1.OpenOperatorSessionRequest{
		Email: opMarkEmail, Password: opMarkPassword, TotpCode: opMarkTOTP, RecoveryCode: opMarkRecovery})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("both factors: %v, want InvalidArgument", status.Code(err))
	}
	if f.calls != 0 {
		t.Fatal("use case entered for a request with both factors")
	}
}

func TestOperatorServerNewPasswordRefusalNamesTheRule(t *testing.T) {
	for e, want := range map[error]identityv1.NewPasswordProblem{
		domain.ErrThieuMatKhau:      identityv1.NewPasswordProblem_NEW_PASSWORD_PROBLEM_EMPTY,
		domain.ErrMatKhauKhongDoc:   identityv1.NewPasswordProblem_NEW_PASSWORD_PROBLEM_NOT_UTF8,
		domain.ErrMatKhauQuaNgan:    identityv1.NewPasswordProblem_NEW_PASSWORD_PROBLEM_TOO_SHORT,
		domain.ErrMatKhauQuaDai:     identityv1.NewPasswordProblem_NEW_PASSWORD_PROBLEM_TOO_LONG,
		domain.ErrMatKhauMoiTrungCu: identityv1.NewPasswordProblem_NEW_PASSWORD_PROBLEM_SAME_AS_CURRENT,
	} {
		s := NewOperatorServer(&operatorAuthFake{err: e}, nil)
		ch, err := s.ChangeOperatorPassword(context.Background(), &identityv1.ChangeOperatorPasswordRequest{SessionToken: "op1.x"})
		if err != nil {
			t.Fatal(err)
		}
		co, err := s.CompleteOperatorEnrollment(context.Background(), &identityv1.CompleteOperatorEnrollmentRequest{})
		if err != nil {
			t.Fatal(err)
		}
		for rpc, r := range map[string]*identityv1.NewPasswordRefusal{
			"Change": ch.GetNewPasswordRefusal(), "Complete": co.GetNewPasswordRefusal(),
		} {
			if r.GetProblem() != want || r.GetMinLength() != domain.DaiMatKhauToiThieu || r.GetMaxLength() != domain.DaiMatKhauToiDa {
				t.Errorf("%s %v: refusal %v, want %v [%d,%d]", rpc, e, r, want, domain.DaiMatKhauToiThieu, domain.DaiMatKhauToiDa)
			}
		}
		if ch.GetOutcome() != outRejected || co.GetOutcome() != outRejected || co.GetSession() != nil || len(co.GetRecoveryCodes()) != 0 {
			t.Errorf("%v: outcomes %v / %v, a rejection must carry nothing else", e, ch.GetOutcome(), co.GetOutcome())
		}
	}
}

// Refused and enrolment-required carry NOTHING beside the outcome: one shape for the one answer.
func TestOperatorServerRefusalCarriesNothing(t *testing.T) {
	s := NewOperatorServer(&operatorAuthFake{err: app.ErrOperatorLoginFailed}, nil)
	ctx := context.Background()
	o, _ := s.OpenOperatorSession(ctx, &identityv1.OpenOperatorSessionRequest{})
	b, _ := s.BeginOperatorEnrollment(ctx, &identityv1.BeginOperatorEnrollmentRequest{})
	c, _ := s.CompleteOperatorEnrollment(ctx, &identityv1.CompleteOperatorEnrollmentRequest{})
	r, _ := s.RegenerateOperatorRecoveryCodes(ctx, &identityv1.RegenerateOperatorRecoveryCodesRequest{SessionToken: "op1.x"})
	if o.GetSession() != nil || b.GetOperatorCode() != "" || b.GetProvisioningUri() != "" || b.GetManualEntryKey() != "" ||
		c.GetSession() != nil || len(c.GetRecoveryCodes()) != 0 || c.GetNewPasswordRefusal() != nil || len(r.GetRecoveryCodes()) != 0 {
		t.Fatalf("a refusal carried data: %v %v %v %v", o, b, c, r)
	}
}

func TestOperatorServerAcceptedCarriesTheGrant(t *testing.T) {
	ctx := context.Background()
	f := &operatorAuthFake{}
	s := NewOperatorServer(f, nil)

	o, err := s.OpenOperatorSession(ctx, &identityv1.OpenOperatorSessionRequest{Email: "a@example.test",
		Password: "pw", RecoveryCode: "rc", ClientIp: "10.0.0.9", UserAgent: "ua"})
	if err != nil {
		t.Fatal(err)
	}
	if g := o.GetSession(); g.GetSessionToken() != "op1.tok" || g.GetOperatorCode() != "VH-00001" || !g.GetExpiresAt().AsTime().Equal(opExpires) {
		t.Fatalf("session grant %v", g)
	}
	if f.gotLogin != (app.OperatorLoginRequest{Email: "a@example.test", Password: "pw", RecoveryCode: "rc", IP: "10.0.0.9", UserAgent: "ua"}) {
		t.Fatalf("use case got %+v", f.gotLogin)
	}

	b, err := s.BeginOperatorEnrollment(ctx, &identityv1.BeginOperatorEnrollmentRequest{Email: "a@example.test", TemporaryPassword: "tmp", ClientIp: "10.0.0.9"})
	if err != nil {
		t.Fatal(err)
	}
	if b.GetOperatorCode() != "VH-00001" || !strings.HasPrefix(b.GetProvisioningUri(), "otpauth://") || b.GetManualEntryKey() != "X" {
		t.Fatalf("begin %v", b)
	}
	if f.gotBegin != (app.OperatorEnrollmentRequest{Email: "a@example.test", TemporaryPassword: "tmp", IP: "10.0.0.9"}) {
		t.Fatalf("begin got %+v", f.gotBegin)
	}

	c, err := s.CompleteOperatorEnrollment(ctx, &identityv1.CompleteOperatorEnrollmentRequest{Email: "a@example.test",
		TemporaryPassword: "tmp", NewPassword: "new", TotpCode: "123456", ClientIp: "10.0.0.9", UserAgent: "ua"})
	if err != nil {
		t.Fatal(err)
	}
	if c.GetSession().GetSessionToken() != "op1.tok" || len(c.GetRecoveryCodes()) != 1 || c.GetRecoveryCodes()[0] != "code-1" {
		t.Fatalf("complete %v", c)
	}
	if f.gotComplete != (app.OperatorEnrollmentCompletion{Email: "a@example.test", TemporaryPassword: "tmp",
		NewPassword: "new", TOTPCode: "123456", IP: "10.0.0.9", UserAgent: "ua"}) {
		t.Fatalf("complete got %+v", f.gotComplete)
	}

	r, err := s.RegenerateOperatorRecoveryCodes(ctx, &identityv1.RegenerateOperatorRecoveryCodesRequest{SessionToken: "op1.t", TotpCode: "654321", ClientIp: "10.0.0.8"})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.GetRecoveryCodes()) != 2 || f.gotToken != "op1.t" || f.gotOTP != "654321" || f.gotIP != "10.0.0.8" {
		t.Fatalf("regenerate %v, got token %q otp %q ip %q", r, f.gotToken, f.gotOTP, f.gotIP)
	}

	if _, err := s.ChangeOperatorPassword(ctx, &identityv1.ChangeOperatorPasswordRequest{SessionToken: "op1.t",
		CurrentPassword: "cur", NewPassword: "new", TotpCode: "111111", ClientIp: "10.0.0.7"}); err != nil {
		t.Fatal(err)
	}
	if f.gotChange != (app.OperatorPasswordChange{Token: "op1.t", CurrentPassword: "cur", NewPassword: "new", TOTPCode: "111111", IP: "10.0.0.7"}) {
		t.Fatalf("change got %+v", f.gotChange)
	}

	if _, err := s.RevokeOperatorSession(ctx, &identityv1.RevokeOperatorSessionRequest{SessionToken: "op1.r", ClientIp: "10.0.0.6"}); err != nil {
		t.Fatal(err)
	}
	if f.gotToken != "op1.r" || f.gotIP != "10.0.0.6" {
		t.Fatalf("revoke got token %q ip %q", f.gotToken, f.gotIP)
	}
}

func TestOperatorServerResolvePrincipal(t *testing.T) {
	ctx := context.Background()
	f := &operatorAuthFake{principal: app.OperatorPrincipal{ID: "01JDOPERATORAAAAAAAAAAAAAA", Code: "VH-00001",
		Permissions: []domain.OperatorPermission{"ops.tenant.read"}, ExpiresAt: opExpires}}
	s := NewOperatorServer(f, nil)
	r, err := s.ResolveOperatorSession(ctx, &identityv1.ResolveOperatorSessionRequest{SessionToken: "op1.t"})
	if err != nil {
		t.Fatal(err)
	}
	p := r.GetPrincipal()
	if p.GetOperatorId() != "01JDOPERATORAAAAAAAAAAAAAA" || p.GetOperatorCode() != "VH-00001" ||
		len(p.GetPermissionKeys()) != 1 || p.GetPermissionKeys()[0] != "ops.tenant.read" {
		t.Fatalf("principal %v", p)
	}

	// Live session, no key: PRESENT with empty keys — signed in, every guarded route 403.
	f.principal.Permissions = nil
	r, err = s.ResolveOperatorSession(ctx, &identityv1.ResolveOperatorSessionRequest{SessionToken: "op1.t"})
	if err != nil || r.GetPrincipal() == nil || len(r.GetPrincipal().GetPermissionKeys()) != 0 {
		t.Fatalf("keyless principal: %v, %v", r, err)
	}

	// Not usable: OK and NO principal.
	f.err = fmt.Errorf("x: %w", app.ErrOperatorUnauthenticated)
	r, err = s.ResolveOperatorSession(ctx, &identityv1.ResolveOperatorSessionRequest{SessionToken: "op1.t"})
	if err != nil || r.GetPrincipal() != nil {
		t.Fatalf("unusable session: %v, %v — want OK with no principal", r, err)
	}

	// A principal without its VH- code is a contract fault, never served.
	f.err = nil
	f.principal = app.OperatorPrincipal{ID: "01JDOPERATORAAAAAAAAAAAAAA"}
	if _, err := s.ResolveOperatorSession(ctx, &identityv1.ResolveOperatorSessionRequest{SessionToken: "op1.t"}); status.Code(err) != codes.Internal {
		t.Fatalf("principal without code: %v, want Internal", status.Code(err))
	}
}

// A canceled call is Canceled — the transaction rolled back, nothing decided — never Internal.
func TestOperatorServerCanceledCallIsNotInternal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := NewOperatorServer(&operatorAuthFake{err: context.Canceled}, nil)
	_, err := s.RevokeOperatorSession(ctx, &identityv1.RevokeOperatorSessionRequest{SessionToken: "op1.t"})
	if status.Code(err) != codes.Canceled {
		t.Fatalf("canceled call: %v, want Canceled", status.Code(err))
	}
}

func TestClipUserAgent(t *testing.T) {
	long := strings.Repeat("ệ", 300) // three bytes per rune: a byte cut would split one
	got := clipUserAgent(long)
	if utf8.RuneCountInString(got) != userAgentMaxRunes || !utf8.ValidString(got) {
		t.Fatalf("clipped to %d runes (valid %v), want %d", utf8.RuneCountInString(got), utf8.ValidString(got), userAgentMaxRunes)
	}
	if exact := strings.Repeat("a", userAgentMaxRunes); clipUserAgent(exact) != exact {
		t.Fatal("a 256-rune value must be kept whole")
	}
	if got := clipUserAgent("Mozilla\x00/5.0 \xff"); got != "Mozilla/5.0 �" {
		t.Fatalf("NUL / invalid UTF-8 not repaired: %q", got)
	}

	f := &operatorAuthFake{}
	s := NewOperatorServer(f, nil)
	_, _ = s.OpenOperatorSession(context.Background(), &identityv1.OpenOperatorSessionRequest{UserAgent: long})
	_, _ = s.CompleteOperatorEnrollment(context.Background(), &identityv1.CompleteOperatorEnrollmentRequest{UserAgent: long})
	if utf8.RuneCountInString(f.gotLogin.UserAgent) != userAgentMaxRunes || utf8.RuneCountInString(f.gotComplete.UserAgent) != userAgentMaxRunes {
		t.Fatal("user_agent reached the use case unclipped")
	}
}

func TestNewOperatorServerRefusesMissingUseCase(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewOperatorServer(nil) did not panic")
		}
	}()
	NewOperatorServer(nil, nil)
}

// untouchedOperatorStore fails the test if a use case reaches the store: a token refused at the
// signature must never cost a database round trip, nor touch an idle timer.
type untouchedOperatorStore struct{ t *testing.T }

func (s untouchedOperatorStore) InTx(context.Context, func(app.OperatorTx) error) error {
	s.t.Error("operator store reached for a token the signature check should have refused")
	return errors.New("untouched store")
}
func (s untouchedOperatorStore) ListAccounts(context.Context) ([]domain.OperatorAccount, error) {
	s.t.Error("ListAccounts reached")
	return nil, nil
}
func (s untouchedOperatorStore) ActivePermissions(context.Context, string) ([]domain.OperatorPermission, error) {
	s.t.Error("ActivePermissions reached")
	return nil, nil
}

// Fake key material — the text says so (rule 8, forbidden #1). The SAME bytes sign the staff token
// below: even with a key pasted into both lists, the realms do not cross (operatorauth macKeyLabel).
var (
	opSharedKeyFake = secret.Secret("FAKE-KEY-shared-by-both-realms-NOT-REAL-32B")
	opSealKeyFake   = secret.Secret("FAKE-AES-KEY-32-bytes-NOT-REAL!!")
)

// A staff `v1.` token — signed with the very key the operator realm holds — resolves to NO principal,
// through the real use case, without touching the store (ADR 0048 stop condition #6).
func TestResolveOperatorSessionRefusesStaffToken(t *testing.T) {
	staff, err := token.NewSigner([]secret.Secret{opSharedKeyFake})
	if err != nil {
		t.Fatal(err)
	}
	staffTok, err := staff.Ky(token.Claims{TenantID: tenant.ID("01JD8ZQK9M3NPXR7TVWYB2C4EF"), Sid: "sid-staff",
		ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	signer, err := operatorauth.NewTokenSigner([]secret.Secret{opSharedKeyFake})
	if err != nil {
		t.Fatal(err)
	}
	sealer, err := operatorauth.NewSealer([]secret.Secret{opSealKeyFake})
	if err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	s := NewOperatorServer(app.NewOperatorAuth(untouchedOperatorStore{t}, signer, sealer, nil, log), log)

	for _, tok := range []string{staffTok, "op1." + strings.TrimPrefix(staffTok, "v1."), "garbage"} {
		r, err := s.ResolveOperatorSession(context.Background(), &identityv1.ResolveOperatorSessionRequest{SessionToken: tok})
		if err != nil || r.GetPrincipal() != nil {
			t.Fatalf("token %.8q…: %v, %v — want OK with no principal", tok, r, err)
		}
		if strings.Contains(logs.String(), tok) {
			t.Fatal("a bearer token reached the log")
		}
	}
	// Sign-out of a staff token: nothing to revoke, store untouched.
	if r, err := s.RevokeOperatorSession(context.Background(), &identityv1.RevokeOperatorSessionRequest{SessionToken: staffTok}); err != nil || r.GetOutcome() != outNotLive {
		t.Fatalf("revoke staff token: %v, %v — want SESSION_NOT_LIVE", r, err)
	}
}

// The real use case with no keys — the dev state of a deployment that has not set them — answers
// FAILED_PRECONDITION on every RPC, never a refusal and never a crash.
func TestOperatorServerUnconfiguredRealUseCase(t *testing.T) {
	log := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	s := NewOperatorServer(app.NewOperatorAuth(untouchedOperatorStore{t}, nil, nil, nil, log), log)
	for name, call := range opRPCs() {
		if _, err := call(s); status.Code(err) != codes.FailedPrecondition {
			t.Errorf("%s with no keys: %v, want FailedPrecondition", name, status.Code(err))
		}
	}
}

// No credential field reaches any log line, on ANY path — success, refusal, not-live, rejection,
// unconfigured and Internal — at Debug level.
func TestOperatorServerNeverLogsCredentials(t *testing.T) {
	var logs bytes.Buffer
	// No timestamp: its fractional seconds could spell the six-digit TOTP marker by chance.
	log := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		}}))
	for _, e := range []error{nil, app.ErrOperatorLoginFailed, app.ErrOperatorEnrollmentRequired,
		app.ErrOperatorUnauthenticated, domain.ErrMatKhauQuaNgan, app.ErrOperatorRealmNotConfigured,
		errors.New("store failure")} {
		for _, call := range opRPCs() {
			_, _ = call(NewOperatorServer(&operatorAuthFake{err: e, principal: app.OperatorPrincipal{ID: "x", Code: "VH-00001"}}, log))
		}
	}
	if !strings.Contains(logs.String(), "OperatorService") {
		t.Fatal("the Internal path logged nothing — this test would pass with logging switched off")
	}
	for _, m := range []string{opMarkEmail, opMarkPassword, opMarkNewPw, opMarkTOTP, opMarkRecovery, opMarkToken, "OP-MARKER"} {
		if strings.Contains(logs.String(), m) {
			t.Fatalf("log carries a credential or personal field (%q):\n%s", m, logs.String())
		}
	}
}
