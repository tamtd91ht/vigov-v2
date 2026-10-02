package main

// OperatorService's WIRING, over a real connection with the real chain — for the reason the note at
// the top of main_test.go gives. internal/grpc proves the status table; only this file can see that
// buildOperatorGRPCServer registers the service behind the caller key, that the commune interceptor
// lets its RPCs through with no commune (core/grpcx.methodsWithoutTenant), and that NEITHER the
// inter-service port NOR the bridge port serves it (user decision 01/10/2026: a port of its own).

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
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/grpcx"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/token"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	svcgrpc "github.com/vihat/vigov/service-identity/internal/grpc"
)

// Fake operator key material (rule 8, forbidden #1).
var (
	operatorSigningKeyFake = secret.Secret("FAKE-OPERATOR-SIGNING-KEY-NOT-REAL-32-bytes")
	operatorTOTPKeyFake    = secret.Secret("FAKE-AES-KEY-32-bytes-NOT-REAL!!")
)

// operatorStoreStub answers every transaction with an error. These tests send tokens the signature
// check refuses, so a call reaching it means a refusal happened too late.
type operatorStoreStub struct{ txs int }

func (s *operatorStoreStub) InTx(context.Context, func(app.OperatorTx) error) error {
	s.txs++
	return errors.New("operator store stub")
}
func (s *operatorStoreStub) ListAccounts(context.Context) ([]domain.OperatorAccount, error) {
	return nil, nil
}
func (s *operatorStoreStub) ActivePermissions(context.Context, string) ([]domain.OperatorPermission, error) {
	return nil, nil
}

// operatorServerOff is the dev state with no operator keys, built by the SAME function run() uses.
func operatorServerOff(t *testing.T) *svcgrpc.OperatorServer {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	uc, err := buildOperatorAuth(nil, nil, &operatorStoreStub{}, log)
	if err != nil {
		t.Fatalf("buildOperatorAuth without keys: %v", err)
	}
	return svcgrpc.NewOperatorServer(uc, log)
}

func operatorServerOn(t *testing.T, store app.OperatorStore, log *slog.Logger) *svcgrpc.OperatorServer {
	t.Helper()
	uc, err := buildOperatorAuth([]secret.Secret{operatorSigningKeyFake}, []secret.Secret{operatorTOTPKeyFake}, store, log)
	if err != nil {
		t.Fatalf("buildOperatorAuth: %v", err)
	}
	return svcgrpc.NewOperatorServer(uc, log)
}

// dialOperatorService starts the REAL operator listener's server (buildOperatorGRPCServer) and dials
// it with the given client options.
func dialOperatorService(t *testing.T, op *svcgrpc.OperatorServer, opts ...grpc.DialOption) identityv1.OperatorServiceClient {
	t.Helper()
	srv := buildOperatorGRPCServer(khoaGoiGia, op, slog.New(slog.NewTextHandler(io.Discard, nil)))
	lis := bufconn.Listen(1 << 20)
	go func() {
		if err := srv.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Errorf("Serve: %v", err)
		}
	}()
	t.Cleanup(srv.Stop)
	opts = append(opts,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }))
	conn, err := grpc.NewClient("passthrough:///bufnet", opts...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return identityv1.NewOperatorServiceClient(conn)
}

// No caller key → Unauthenticated, on the operator RPCs as on every other: the commune exemption is
// never a caller-key exemption.
func TestOperatorServiceRefusesCallerWithoutKey(t *testing.T) {
	store := &operatorStoreStub{}
	cl := dialOperatorService(t, operatorServerOn(t, store, slog.New(slog.NewTextHandler(io.Discard, nil))))
	if _, err := cl.ResolveOperatorSession(context.Background(),
		&identityv1.ResolveOperatorSessionRequest{SessionToken: "op1.x.y"}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("ResolveOperatorSession without key: %v, want Unauthenticated", status.Code(err))
	}
	if _, err := cl.OpenOperatorSession(context.Background(),
		&identityv1.OpenOperatorSessionRequest{Email: "a@example.test", Password: "x", TotpCode: "000000"}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("OpenOperatorSession without key: %v, want Unauthenticated", status.Code(err))
	}
	if store.txs != 0 {
		t.Fatal("a caller without the key reached the operator store")
	}
}

// Caller key, NO commune — not on the client, not in metadata: accepted, because every RPC of this
// service is on methodsWithoutTenant. A staff `v1.` token (signed with the staff key this binary
// holds) and garbage both resolve to OK + no principal, and neither reaches the store.
func TestOperatorServiceAnswersWithoutCommune(t *testing.T) {
	var logs bytes.Buffer
	store := &operatorStoreStub{}
	log := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	cl := dialOperatorService(t, operatorServerOn(t, store, log),
		grpc.WithChainUnaryInterceptor(grpcx.UnaryClientCallerAuth(khoaGoiGia)))

	staff, err := token.NewSigner([]secret.Secret{khoaKyGia})
	if err != nil {
		t.Fatal(err)
	}
	staffTok, err := staff.Ky(token.Claims{TenantID: ulidThu, Sid: "sid-gia", ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	for _, tok := range []string{staffTok, "op1.garbage.garbage"} {
		r, err := cl.ResolveOperatorSession(context.Background(), &identityv1.ResolveOperatorSessionRequest{SessionToken: tok})
		if err != nil {
			t.Fatalf("ResolveOperatorSession with no commune refused: %v", err)
		}
		if r.GetPrincipal() != nil {
			t.Fatalf("token %.6q… resolved to an operator: %v", tok, r.GetPrincipal())
		}
		if strings.Contains(logs.String(), tok) {
			t.Fatal("a bearer token reached the log")
		}
	}
	rv, err := cl.RevokeOperatorSession(context.Background(), &identityv1.RevokeOperatorSessionRequest{SessionToken: staffTok})
	if err != nil || rv.GetOutcome() != identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_SESSION_NOT_LIVE {
		t.Fatalf("revoke of a staff token: %v, %v — want SESSION_NOT_LIVE", rv, err)
	}
	if _, err := cl.ResolveOperatorSession(context.Background(), &identityv1.ResolveOperatorSessionRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("empty token through the chain: %v, want InvalidArgument", status.Code(err))
	}
	if store.txs != 0 {
		t.Fatalf("store reached %d times for tokens the signature refuses", store.txs)
	}
}

// miniAppsUntouched fails the test if a Mini App secret act runs: the calls below must be refused first.
type miniAppsUntouched struct{ t *testing.T }

func (m miniAppsUntouched) SetSecretAsOperator(context.Context, app.MiniAppOperator, string, secret.Secret, string) (app.MiniAppSecretSet, error) {
	m.t.Error("SetSecretAsOperator ran")
	return app.MiniAppSecretSet{}, errors.New("untouched")
}

func (m miniAppsUntouched) RetireAsOperator(context.Context, app.MiniAppOperator, string, string) (app.MiniAppSecretRetired, error) {
	m.t.Error("RetireAsOperator ran")
	return app.MiniAppSecretRetired{}, errors.New("untouched")
}

// The two RPCs that act ON a commune are NOT exempt from the commune (operator.proto): through the
// real chain, no x-tenant-id is INVALID_ARGUMENT before the handler; WITH it, the handler runs and a
// token the signature refuses is SESSION_NOT_LIVE — nothing reaches the operator store or the use case.
func TestOperatorServiceMiniAppSecretRPCsNeedTheCommune(t *testing.T) {
	store := &operatorStoreStub{}
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	op := operatorServerOn(t, store, log).WithMiniAppSecrets(miniAppsUntouched{t})
	cl := dialOperatorService(t, op,
		grpc.WithChainUnaryInterceptor(grpcx.UnaryClientCallerAuth(khoaGoiGia), grpcx.UnaryClientInterceptor()))

	const fakeSecret = "FAKE-APP-SECRET-WIRING-0000"
	set := &identityv1.SetMiniAppSecretRequest{SessionToken: "op1.garbage.garbage", AppId: "1234567890",
		AppSecret: fakeSecret, Reason: "kiểm thử"}
	ret := &identityv1.RetireMiniAppSecretRequest{SessionToken: "op1.garbage.garbage", AppId: "1234567890", Reason: "kiểm thử"}

	if _, err := cl.SetMiniAppSecret(context.Background(), set); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("SetMiniAppSecret without x-tenant-id: %v, want InvalidArgument", status.Code(err))
	}
	if _, err := cl.RetireMiniAppSecret(context.Background(), ret); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("RetireMiniAppSecret without x-tenant-id: %v, want InvalidArgument", status.Code(err))
	}

	ctx := tenant.Into(context.Background(), ulidThu)
	sr, err := cl.SetMiniAppSecret(ctx, set)
	if err != nil || sr.GetOutcome() != identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_SESSION_NOT_LIVE {
		t.Fatalf("SetMiniAppSecret with commune, bad token: %v %v, want SESSION_NOT_LIVE", sr, err)
	}
	rr, err := cl.RetireMiniAppSecret(ctx, ret)
	if err != nil || rr.GetOutcome() != identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_SESSION_NOT_LIVE {
		t.Fatalf("RetireMiniAppSecret with commune, bad token: %v %v, want SESSION_NOT_LIVE", rr, err)
	}
	if store.txs != 0 {
		t.Fatalf("operator store reached %d times", store.txs)
	}
	if strings.Contains(logs.String(), fakeSecret) {
		t.Fatal("the app secret reached the log")
	}
}

// No keys in dev: identity still starts and the RPCs answer FAILED_PRECONDITION — through the chain.
func TestOperatorServiceUnconfiguredAnswersFailedPrecondition(t *testing.T) {
	cl := dialOperatorService(t, operatorServerOff(t), grpc.WithChainUnaryInterceptor(grpcx.UnaryClientCallerAuth(khoaGoiGia)))
	if _, err := cl.ResolveOperatorSession(context.Background(),
		&identityv1.ResolveOperatorSessionRequest{SessionToken: "op1.x.y"}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("unconfigured realm: %v, want FailedPrecondition", status.Code(err))
	}
	if _, err := cl.BeginOperatorEnrollment(context.Background(),
		&identityv1.BeginOperatorEnrollmentRequest{Email: "a@example.test", TemporaryPassword: "x"}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("unconfigured realm: %v, want FailedPrecondition", status.Code(err))
	}
}

// Absent keys are "off"; a key that is SET but unusable stops the process.
func TestBuildOperatorAuthRefusesUnusableKeys(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cases := map[string]struct {
		signing, totp []secret.Secret
		wantErr       bool
	}{
		"both absent":        {nil, nil, false},
		"signing only":       {[]secret.Secret{operatorSigningKeyFake}, nil, false},
		"both usable":        {[]secret.Secret{operatorSigningKeyFake}, []secret.Secret{operatorTOTPKeyFake}, false},
		"short signing key":  {[]secret.Secret{secret.Secret("too-short")}, []secret.Secret{operatorTOTPKeyFake}, true},
		"wrong-size AES key": {[]secret.Secret{operatorSigningKeyFake}, []secret.Secret{secret.Secret("not-32-bytes")}, true},
	}
	for name, c := range cases {
		_, err := buildOperatorAuth(c.signing, c.totp, &operatorStoreStub{}, log)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err = %v, wantErr %v", name, err, c.wantErr)
		}
	}
}

// The bridge key opens nothing of OperatorService: it is not registered on the bridge listener.
func TestOperatorServiceAbsentFromBridgePort(t *testing.T) {
	conn, _ := moCongCau(t, khoaCauGia)
	if _, err := identityv1.NewOperatorServiceClient(conn).ResolveOperatorSession(voiKhoaCau(khoaCauGia),
		&identityv1.ResolveOperatorSessionRequest{SessionToken: "op1.x.y"}); status.Code(err) != codes.Unimplemented {
		t.Fatalf("OperatorService on the bridge port: %v, want Unimplemented", status.Code(err))
	}
}

func TestBuildOperatorGRPCServerRefusesMissingOperatorServer(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("built the operator gRPC server with no OperatorServer")
		}
	}()
	_ = buildOperatorGRPCServer(khoaGoiGia, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// The operator listener refuses to be built without the caller key, like the inter-service port: a
// server without it answers every call that reaches it.
func TestBuildOperatorGRPCServerRefusesEmptyCallerKey(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("built the operator gRPC server with an empty GRPC_CALLER_KEY")
		}
	}()
	_ = buildOperatorGRPCServer(nil, operatorServerOff(t), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// THE TEST THE SEPARATE PORT EXISTS FOR (user decision 01/10/2026). The inter-service port — which
// five staff services reach with the shared caller key — must NOT serve OperatorService: a call there
// WITH the key answers Unimplemented. Re-registering the service on dungGRPCServer turns this red.
func TestOperatorServiceAbsentFromInterServicePort(t *testing.T) {
	lis := bufconn.Listen(1 << 20)
	srv := dungGRPCServer(khoaGoiGia, noiDayGia(t), slog.New(slog.NewTextHandler(io.Discard, nil)))
	go func() {
		if err := srv.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Errorf("Serve: %v", err)
		}
	}()
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithChainUnaryInterceptor(grpcx.UnaryClientCallerAuth(khoaGoiGia)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	cl := identityv1.NewOperatorServiceClient(conn)
	if _, err := cl.OpenOperatorSession(context.Background(),
		&identityv1.OpenOperatorSessionRequest{Email: "a@example.test", Password: "x", TotpCode: "000000"}); status.Code(err) != codes.Unimplemented {
		t.Fatalf("OpenOperatorSession on the inter-service port: %v, want Unimplemented", status.Code(err))
	}
	if _, err := cl.ResolveOperatorSession(context.Background(),
		&identityv1.ResolveOperatorSessionRequest{SessionToken: "op1.x.y"}); status.Code(err) != codes.Unimplemented {
		t.Fatalf("ResolveOperatorSession on the inter-service port: %v, want Unimplemented", status.Code(err))
	}
}

// And the operator listener serves ONLY OperatorService: the staff RPCs are absent there, so the port
// platform alone can reach does not double as a second door to ResolveStaffPrincipal.
func TestOperatorPortServesNothingElse(t *testing.T) {
	srv := buildOperatorGRPCServer(khoaGoiGia, operatorServerOff(t), slog.New(slog.NewTextHandler(io.Discard, nil)))
	services := srv.GetServiceInfo()
	if len(services) != 1 {
		t.Fatalf("operator listener registers %d services, want exactly 1: %v", len(services), services)
	}
	if _, ok := services[identityv1.OperatorService_ServiceDesc.ServiceName]; !ok {
		t.Fatalf("operator listener does not register OperatorService: %v", services)
	}
}
