package operatorclient

// What these tests defend: THE MAPPING from a gRPC answer to an Outcome / error, and the wire facts
// the owner decided on 2026-10-01 — no commune travels, the caller key does. They speak to a real
// gRPC server over bufconn with the real server interceptors, because the commune exemption lives
// in that chain: a non-exempt method would be refused by grpcx.UnaryServerInterceptor there.

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/grpcx"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
)

// Fake material — the text says so in full (rule 8, forbidden #1). The email is not a real address.
var callerKeyFake = secret.Secret("caller-key-FAKE-NOT-A-REAL-KEY-for-tests-only")

const (
	tokenFake    = "op1.token-FAKE-NOT-A-REAL-TOKEN"
	passwordFake = "  password FAKE not real  " // leading/trailing spaces: must travel verbatim
	totpFake     = "000000"
	emailFake    = "operator@example.invalid"
)

// fakeServer is identity, absent. It records what arrived on the wire.
type fakeServer struct {
	identityv1.UnimplementedOperatorServiceServer

	err error

	open    *identityv1.OpenOperatorSessionResponse
	resolve *identityv1.ResolveOperatorSessionResponse
	revoke  *identityv1.RevokeOperatorSessionResponse
	change  *identityv1.ChangeOperatorPasswordResponse
	regen   *identityv1.RegenerateOperatorRecoveryCodesResponse
	begin   *identityv1.BeginOperatorEnrollmentResponse
	done    *identityv1.CompleteOperatorEnrollmentResponse
	// The two Mini App secret RPCs (mini_app_secret_test.go).
	setSecret    *identityv1.SetMiniAppSecretResponse
	retireSecret *identityv1.RetireMiniAppSecretResponse
	sawSet       *identityv1.SetMiniAppSecretRequest
	statuses     *identityv1.ListMiniAppSecretStatusesResponse

	calls     int
	sawTenant [][]string
	sawOpen   *identityv1.OpenOperatorSessionRequest
}

func (f *fakeServer) record(ctx context.Context) {
	f.calls++
	md, _ := metadata.FromIncomingContext(ctx)
	f.sawTenant = append(f.sawTenant, md.Get(grpcx.MetadataTenantKey))
}

func (f *fakeServer) OpenOperatorSession(ctx context.Context, r *identityv1.OpenOperatorSessionRequest) (*identityv1.OpenOperatorSessionResponse, error) {
	f.record(ctx)
	f.sawOpen = r
	return f.open, f.err
}

func (f *fakeServer) ResolveOperatorSession(ctx context.Context, _ *identityv1.ResolveOperatorSessionRequest) (*identityv1.ResolveOperatorSessionResponse, error) {
	f.record(ctx)
	return f.resolve, f.err
}

func (f *fakeServer) RevokeOperatorSession(ctx context.Context, _ *identityv1.RevokeOperatorSessionRequest) (*identityv1.RevokeOperatorSessionResponse, error) {
	f.record(ctx)
	return f.revoke, f.err
}

func (f *fakeServer) ChangeOperatorPassword(ctx context.Context, _ *identityv1.ChangeOperatorPasswordRequest) (*identityv1.ChangeOperatorPasswordResponse, error) {
	f.record(ctx)
	return f.change, f.err
}

func (f *fakeServer) RegenerateOperatorRecoveryCodes(ctx context.Context, _ *identityv1.RegenerateOperatorRecoveryCodesRequest) (*identityv1.RegenerateOperatorRecoveryCodesResponse, error) {
	f.record(ctx)
	return f.regen, f.err
}

func (f *fakeServer) BeginOperatorEnrollment(ctx context.Context, _ *identityv1.BeginOperatorEnrollmentRequest) (*identityv1.BeginOperatorEnrollmentResponse, error) {
	f.record(ctx)
	return f.begin, f.err
}

func (f *fakeServer) CompleteOperatorEnrollment(ctx context.Context, _ *identityv1.CompleteOperatorEnrollmentRequest) (*identityv1.CompleteOperatorEnrollmentResponse, error) {
	f.record(ctx)
	return f.done, f.err
}

// start serves f behind the REAL server chain and returns a client on a connection carrying the
// REAL client chain (UnaryInterceptors), logging into logBuf.
func start(t *testing.T, f *fakeServer) (*Client, *bytes.Buffer) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	gs := grpc.NewServer(grpc.ChainUnaryInterceptor(
		grpcx.UnaryServerCallerAuth(callerKeyFake, slog.New(slog.NewTextHandler(io.Discard, nil))),
		grpcx.UnaryServerInterceptor(),
	))
	identityv1.RegisterOperatorServiceServer(gs, f)
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithChainUnaryInterceptor(UnaryInterceptors(callerKeyFake)...),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	var buf bytes.Buffer
	return NewFromConn(conn, slog.New(slog.NewTextHandler(&buf, nil))), &buf
}

// A context that DOES hold a commune — the exemption must not depend on the caller's state.
func ctxWithTenant() context.Context {
	return tenant.Into(context.Background(), tenant.ID("01JA"+strings.Repeat("A", 22)))
}

func grantFake() *identityv1.OperatorSessionGrant {
	return &identityv1.OperatorSessionGrant{
		SessionToken: tokenFake,
		ExpiresAt:    timestamppb.New(time.Unix(1_800_000_000, 0)),
		OperatorCode: "VH-00001",
	}
}

// All seven RPCs reach the server through the real chains, with NO commune in metadata even
// though the caller's context holds one.
func TestEveryRPCTravelsWithoutACommune(t *testing.T) {
	f := &fakeServer{
		open:    &identityv1.OpenOperatorSessionResponse{Outcome: wRefused},
		resolve: &identityv1.ResolveOperatorSessionResponse{},
		revoke:  &identityv1.RevokeOperatorSessionResponse{Outcome: wNotLive},
		change:  &identityv1.ChangeOperatorPasswordResponse{Outcome: wNotLive},
		regen:   &identityv1.RegenerateOperatorRecoveryCodesResponse{Outcome: wNotLive},
		begin:   &identityv1.BeginOperatorEnrollmentResponse{Outcome: wRefused},
		done:    &identityv1.CompleteOperatorEnrollmentResponse{Outcome: wRefused},
	}
	c, _ := start(t, f)
	ctx := ctxWithTenant()
	tok := secret.Secret(tokenFake)

	if _, err := c.Open(ctx, OpenRequest{Email: emailFake, Password: secret.Secret(passwordFake), TOTPCode: secret.Secret(totpFake)}); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, _, err := c.Resolve(ctx, tok); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if _, err := c.Revoke(ctx, tok, "203.0.113.7"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if _, err := c.ChangePassword(ctx, ChangePasswordRequest{Token: tok}); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if _, err := c.RegenerateRecoveryCodes(ctx, tok, secret.Secret(totpFake), ""); err != nil {
		t.Fatalf("RegenerateRecoveryCodes: %v", err)
	}
	if _, err := c.BeginEnrollment(ctx, BeginEnrollmentRequest{Email: emailFake}); err != nil {
		t.Fatalf("BeginEnrollment: %v", err)
	}
	if _, err := c.CompleteEnrollment(ctx, CompleteEnrollmentRequest{Email: emailFake}); err != nil {
		t.Fatalf("CompleteEnrollment: %v", err)
	}
	if f.calls != 7 {
		t.Fatalf("server saw %d calls, want 7", f.calls)
	}
	for i, v := range f.sawTenant {
		if len(v) != 0 {
			t.Errorf("call %d carried %s=%v — an operator call must carry no commune", i+1, grpcx.MetadataTenantKey, v)
		}
	}
}

func TestOpenAcceptedMapsTheGrantAndSendsCredentialsVerbatim(t *testing.T) {
	f := &fakeServer{open: &identityv1.OpenOperatorSessionResponse{Outcome: wAccepted, Session: grantFake()}}
	c, _ := start(t, f)
	res, err := c.Open(context.Background(), OpenRequest{
		Email: emailFake, Password: secret.Secret(passwordFake), TOTPCode: secret.Secret(totpFake),
		ClientIP: "203.0.113.7", UserAgent: "test-agent",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != OutcomeAccepted || string(res.Session.Token.Lo()) != tokenFake ||
		res.Session.OperatorCode != "VH-00001" || res.Session.ExpiresAt.Unix() != 1_800_000_000 {
		t.Fatalf("grant mapped wrongly: outcome=%v code=%q exp=%v", res.Outcome, res.Session.OperatorCode, res.Session.ExpiresAt)
	}
	if f.sawOpen.GetPassword() != passwordFake || f.sawOpen.GetTotpCode() != totpFake ||
		f.sawOpen.GetRecoveryCode() != "" || f.sawOpen.GetClientIp() != "203.0.113.7" {
		t.Fatal("request fields did not travel verbatim")
	}
}

// UNSPECIFIED is REFUSED (operator.proto); a value outside the RPC's subset is a contract fault.
func TestOutcomeMapping(t *testing.T) {
	f := &fakeServer{open: &identityv1.OpenOperatorSessionResponse{}}
	c, _ := start(t, f)
	res, err := c.Open(context.Background(), OpenRequest{Password: secret.Secret(passwordFake)})
	if err != nil || res.Outcome != OutcomeRefused {
		t.Fatalf("UNSPECIFIED: outcome=%v err=%v, want refused", res.Outcome, err)
	}

	f.open = &identityv1.OpenOperatorSessionResponse{Outcome: wNotLive}
	if _, err := c.Open(context.Background(), OpenRequest{}); !errors.Is(err, ErrContract) {
		t.Fatalf("SESSION_NOT_LIVE on Open: want ErrContract, got %v", err)
	}

	f.open = &identityv1.OpenOperatorSessionResponse{Outcome: wAccepted}
	if _, err := c.Open(context.Background(), OpenRequest{}); !errors.Is(err, ErrContract) {
		t.Fatalf("ACCEPTED without a session: want ErrContract, got %v", err)
	}

	f.open = &identityv1.OpenOperatorSessionResponse{Outcome: wEnrollment}
	if res, err := c.Open(context.Background(), OpenRequest{}); err != nil || res.Outcome != OutcomeEnrollmentRequired || !res.Session.Token.Rong() {
		t.Fatalf("ENROLLMENT_REQUIRED: %v %v", res.Outcome, err)
	}
}

// A call that did not happen is an ERROR — never "refused", never "signed out".
func TestTransportFailureIsAnErrorNeverAnAnswer(t *testing.T) {
	f := &fakeServer{err: status.Error(codes.Unavailable, "down")}
	c, logBuf := start(t, f)
	ctx := context.Background()

	if res, err := c.Open(ctx, OpenRequest{Email: emailFake, Password: secret.Secret(passwordFake)}); err == nil {
		t.Fatalf("Open on an outage answered %v with no error", res.Outcome)
	}
	if _, ok, err := c.Resolve(ctx, secret.Secret(tokenFake)); err == nil || ok {
		t.Fatalf("Resolve on an outage: ok=%v err=%v — must be an error, never signed-out", ok, err)
	}
	// Nothing credential-shaped reached the log.
	for _, leak := range []string{tokenFake, strings.TrimSpace(passwordFake), emailFake} {
		if strings.Contains(logBuf.String(), leak) {
			t.Fatalf("the log carries a credential or personal data: %s", logBuf.String())
		}
	}
	if !strings.Contains(logBuf.String(), "Unavailable") {
		t.Errorf("the log does not name the gRPC code: %s", logBuf.String())
	}

	f.err = status.Error(codes.FailedPrecondition, "realm not configured")
	if _, _, err := c.Resolve(ctx, secret.Secret(tokenFake)); !errors.Is(err, ErrRealmNotConfigured) {
		t.Fatalf("FAILED_PRECONDITION: want ErrRealmNotConfigured, got %v", err)
	}
}

func TestResolveOutcomes(t *testing.T) {
	f := &fakeServer{resolve: &identityv1.ResolveOperatorSessionResponse{}}
	c, _ := start(t, f)
	ctx := context.Background()
	tok := secret.Secret(tokenFake)

	if _, ok, err := c.Resolve(ctx, tok); ok || err != nil {
		t.Fatalf("absent principal: ok=%v err=%v, want not signed in", ok, err)
	}

	f.resolve = &identityv1.ResolveOperatorSessionResponse{Principal: &identityv1.OperatorPrincipal{OperatorId: "id"}}
	if _, _, err := c.Resolve(ctx, tok); !errors.Is(err, ErrContract) {
		t.Fatalf("principal without operator_code: want ErrContract (no fallback to the id), got %v", err)
	}

	f.resolve = &identityv1.ResolveOperatorSessionResponse{Principal: &identityv1.OperatorPrincipal{
		OperatorId: "id", OperatorCode: "VH-00001", PermissionKeys: []string{"ops.tenant.manage", ""},
	}}
	p, ok, err := c.Resolve(ctx, tok)
	if err != nil || !ok || p.OperatorCode != "VH-00001" || len(p.PermissionKeys) != 1 {
		t.Fatalf("live principal: %+v ok=%v err=%v", p, ok, err)
	}
}

// Refused locally, nothing sent.
func TestInvalidRequestsNeverLeave(t *testing.T) {
	f := &fakeServer{}
	c, _ := start(t, f)
	ctx := context.Background()
	if _, _, err := c.Resolve(ctx, nil); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("empty token: %v", err)
	}
	if _, err := c.Revoke(ctx, nil, ""); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("empty token: %v", err)
	}
	if _, err := c.Open(ctx, OpenRequest{TOTPCode: secret.Secret(totpFake), RecoveryCode: secret.Secret("rc")}); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("both second factors: %v", err)
	}
	if f.calls != 0 {
		t.Fatalf("%d invalid requests reached the server", f.calls)
	}
}

func TestPasswordRefusalAndRecoveryCodes(t *testing.T) {
	refusalWire := &identityv1.NewPasswordRefusal{Problem: identityv1.NewPasswordProblem_NEW_PASSWORD_PROBLEM_TOO_SHORT, MinLength: 12, MaxLength: 128}
	f := &fakeServer{
		change: &identityv1.ChangeOperatorPasswordResponse{Outcome: wRejected, NewPasswordRefusal: refusalWire},
		regen:  &identityv1.RegenerateOperatorRecoveryCodesResponse{Outcome: wAccepted, RecoveryCodes: []string{"rc-FAKE-1", "rc-FAKE-2"}},
		done: &identityv1.CompleteOperatorEnrollmentResponse{
			Outcome: wAccepted, Session: grantFake(), RecoveryCodes: []string{"rc-FAKE-1"},
		},
		begin: &identityv1.BeginOperatorEnrollmentResponse{Outcome: wAccepted, OperatorCode: "VH-00001",
			ProvisioningUri: "otpauth://totp/FAKE", ManualEntryKey: "FAKEKEY"},
	}
	c, _ := start(t, f)
	ctx := context.Background()
	tok := secret.Secret(tokenFake)

	ch, err := c.ChangePassword(ctx, ChangePasswordRequest{Token: tok})
	if err != nil || ch.Outcome != OutcomeNewPasswordRejected || ch.Refusal.MinLength != 12 {
		t.Fatalf("ChangePassword: %+v %v", ch, err)
	}
	f.change = &identityv1.ChangeOperatorPasswordResponse{Outcome: wRejected}
	if _, err := c.ChangePassword(ctx, ChangePasswordRequest{Token: tok}); !errors.Is(err, ErrContract) {
		t.Fatalf("NEW_PASSWORD_REJECTED without a refusal: %v", err)
	}

	rc, err := c.RegenerateRecoveryCodes(ctx, tok, secret.Secret(totpFake), "")
	if err != nil || len(rc.Codes) != 2 || rc.Codes[0].String() != secret.Che {
		t.Fatalf("RegenerateRecoveryCodes: %v %v", len(rc.Codes), err)
	}
	f.regen = &identityv1.RegenerateOperatorRecoveryCodesResponse{Outcome: wAccepted}
	if _, err := c.RegenerateRecoveryCodes(ctx, tok, secret.Secret(totpFake), ""); !errors.Is(err, ErrContract) {
		t.Fatalf("ACCEPTED without codes: %v", err)
	}

	be, err := c.BeginEnrollment(ctx, BeginEnrollmentRequest{Email: emailFake})
	if err != nil || be.Outcome != OutcomeAccepted || be.OperatorCode != "VH-00001" || be.ProvisioningURI.Rong() {
		t.Fatalf("BeginEnrollment: %v %v", be.Outcome, err)
	}

	ce, err := c.CompleteEnrollment(ctx, CompleteEnrollmentRequest{Email: emailFake})
	if err != nil || ce.Outcome != OutcomeAccepted || ce.Session.Token.Rong() || len(ce.RecoveryCodes) != 1 {
		t.Fatalf("CompleteEnrollment: %v %v", ce.Outcome, err)
	}
}

// Result structs are safe to log by accident: every credential renders as ***.
func TestResultsDoNotRenderCredentials(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	log.Info("x", "res", OpenResult{Outcome: OutcomeAccepted, Session: SessionGrant{Token: secret.Secret(tokenFake), OperatorCode: "VH-00001"}})
	if strings.Contains(buf.String(), tokenFake) {
		t.Fatalf("a logged result printed the token: %s", buf.String())
	}
}
