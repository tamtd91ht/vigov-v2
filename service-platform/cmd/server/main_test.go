package main

// What this test defends: THE WIRING, not the interceptors.
//
// core/grpcx proves that UnaryServerCallerAuth refuses a call with no key and that
// UnaryServerInterceptor refuses a call with no commune. Neither of those tests can see whether
// this binary actually installs them. An interceptor deleted from the chain in buildGRPCServer
// leaves a server that starts, serves, answers — and accepts every unauthenticated call on the
// inter-service port. Nothing else in this repository turns red for that.
//
// So this test speaks to the real server over a real connection, with the real chain.

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
	"github.com/vihat/vigov/core/grpcx"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-platform/internal/domain"
	svcgrpc "github.com/vihat/vigov/service-platform/internal/grpc"
)

// fakeCallerKey is fake key material — the text says so in full (rule 8, forbidden #1).
var fakeCallerKey = secret.Secret("khoa-goi-noi-bo-GIA-KHONG-PHAI-KHOA-THAT")

const (
	testHost     = "thangbinh.example.gov.vn"
	testTenantID = tenant.ID("01JD8ZQK9M3NPXR7TVWYB2C4EF")
)

// fakeDirectory is the registry, absent. The test is about the chain in front of the handlers, so
// the handlers only have to answer something.
type fakeDirectory struct{}

func (fakeDirectory) ByHostErr(context.Context, string) (tenant.Tenant, error) {
	return tenant.Tenant{ID: testTenantID, Host: testHost, Active: true}, nil
}

func (fakeDirectory) ByID(_ context.Context, id tenant.ID) (tenant.Tenant, error) {
	return tenant.Tenant{ID: id, Host: testHost, Active: true}, nil
}

func (fakeDirectory) MiniApp(_ context.Context, appID string) (domain.MiniApp, error) {
	return domain.MiniApp{AppID: appID, Mode: domain.MiniAppModeMain}, nil
}

// fakeProfiles answers only when a commune is in the context — which is what the chain must put there.
type fakeProfiles struct{}

func (fakeProfiles) Read(ctx context.Context) (domain.CommuneProfile, error) {
	_ = tenant.MustFrom(ctx)
	return domain.CommuneProfile{OfficeAddress: "Trụ sở thử"}, nil
}

// policiesFake answers one policy whatever the commune — like the real store, which has none.
type policiesFake struct{}

func (policiesFake) ListUploadPolicies(context.Context) ([]domain.UploadPolicy, error) {
	return []domain.UploadPolicy{{Purpose: "tenant-logo", MaxBytes: 10485760,
		AllowedMIMETypes: []string{"image/png"}}}, nil
}

// fieldsFake answers one code whatever the commune — like the real store, which has none.
type fieldsFake struct{}

func (fieldsFake) ListCitizenReportFields(context.Context) ([]domain.CitizenReportField, error) {
	return []domain.CitizenReportField{{Code: "khac", DefaultLabel: "Khác", SortOrder: 12, IsActive: true}}, nil
}

func fakeDeps() svcgrpc.Deps {
	return svcgrpc.Deps{Dir: fakeDirectory{}, Apps: fakeDirectory{}, Profiles: fakeProfiles{}, Policies: policiesFake{},
		Fields: fieldsFake{}}
}

// startServer starts the real server on an in-memory connection and returns a client dialled with the
// given interceptors — so a test can choose to dial like a correctly configured service, or like
// something that just opened a socket to the port.
func startServer(t *testing.T, opts ...grpc.DialOption) platformv1.PlatformServiceClient {
	t.Helper()

	lis := bufconn.Listen(1 << 20)
	srv := buildGRPCServer(fakeCallerKey, fakeDeps(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	go func() {
		if err := srv.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Errorf("Serve: %v", err)
		}
	}()
	t.Cleanup(srv.Stop)

	opts = append(opts,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}))
	conn, err := grpc.NewClient("passthrough:///bufnet", opts...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	return platformv1.NewPlatformServiceClient(conn)
}

// A caller that only opened a socket gets nothing — INCLUDING ResolveHost, which is exempt from
// carrying a commune because it is the call that establishes one. That exemption says nothing
// about the caller, and this is the assertion that keeps the two axes apart at the binary level.
func TestGRPCPortRefusesCallerWithoutKey(t *testing.T) {
	cl := startServer(t) // no interceptors at all: the naked caller

	if _, err := cl.ResolveHost(context.Background(),
		&platformv1.ResolveHostRequest{Host: testHost}); status.Code(err) != codes.Unauthenticated {
		t.Errorf("ResolveHost không khoá: mã = %v, muốn Unauthenticated (lỗi: %v)",
			status.Code(err), err)
	}

	if _, err := cl.GetTenant(context.Background(),
		&platformv1.GetTenantRequest{Id: testTenantID.String()}); status.Code(err) != codes.Unauthenticated {
		t.Errorf("GetTenant không khoá: mã = %v, muốn Unauthenticated (lỗi: %v)",
			status.Code(err), err)
	}
}

// The other half of the same statement: with the key, the chain lets the call through and the
// commune interceptor behind it still does its own job. A server that refused everything would
// pass the test above and be just as broken.
func TestGRPCPortAdmitsWhenBothConditionsMet(t *testing.T) {
	cl := startServer(t, grpc.WithChainUnaryInterceptor(
		grpcx.UnaryClientCallerAuth(fakeCallerKey),
		grpcx.UnaryClientInterceptor(),
	))

	// ResolveHost: key yes, commune no — it is the call that establishes one.
	res, err := cl.ResolveHost(context.Background(), &platformv1.ResolveHostRequest{Host: testHost})
	if err != nil {
		t.Fatalf("ResolveHost với khoá đúng vẫn bị từ chối: %v", err)
	}
	if res.GetTenant().GetId() != testTenantID.String() {
		t.Errorf("ResolveHost trả xã = %q", res.GetTenant().GetId())
	}

	// GetTenant: key AND commune. The commune comes from the context, never from the body.
	ctx := tenant.Into(context.Background(), testTenantID)
	if _, err := cl.GetTenant(ctx, &platformv1.GetTenantRequest{Id: testTenantID.String()}); err != nil {
		t.Fatalf("GetTenant với khoá và xã đầy đủ vẫn bị từ chối: %v", err)
	}

	// ...and a call with no commune is refused. THIS ONE PROVES THE CLIENT INTERCEPTOR ONLY —
	// the refusal happens before anything leaves the process, so it says nothing about the
	// server's chain. Kept because the client half is worth pinning; see the test below for the
	// server half, which this test was once believed to cover.
	if _, err := cl.GetTenant(context.Background(),
		&platformv1.GetTenantRequest{Id: testTenantID.String()}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("GetTenant thiếu xã (phía client): mã = %v, muốn InvalidArgument (lỗi: %v)",
			status.Code(err), err)
	}
}

// THE SERVER'S OWN COMMUNE CHECK, and this test exists because its absence was measured.
//
// Deleting grpcx.UnaryServerInterceptor from buildGRPCServer turned NOTHING red in this package
// (checked 2026-09-20). Every other test here dials with the client interceptor attached, so
// `x-tenant-id` is on the wire whatever the server does with it — the suite could not tell a
// server that enforces the commune from one that ignores it.
//
// So this test dials WITH the caller key and deliberately WITHOUT the client commune
// interceptor. Nothing puts a commune on the wire, and GetTenant is not on
// grpcx.methodsWithoutTenant — so an InvalidArgument here can only have come from the server's
// own chain. The caller key is attached on purpose: without it the call is refused earlier with
// Unauthenticated, and the test would pass for the wrong reason a second time.
//
// WHY IT MATTERS FOR THIS SERVICE SPECIFICALLY, stated so nobody downgrades it later: platform's
// two RPCs do not currently read the commune from context, so a missing interceptor leaks
// nothing TODAY. What it removes is the declaration that a non-exempt RPC must carry a commune
// at all — rule 1, forbidden #1. The first RPC added here that does scope by commune would
// inherit a port with no such guarantee, and nothing would say so.
func TestServerRefusesRPCWithoutTenantEvenWithKey(t *testing.T) {
	cl := startServer(t, grpc.WithChainUnaryInterceptor(grpcx.UnaryClientCallerAuth(fakeCallerKey)))

	_, err := cl.GetTenant(context.Background(), &platformv1.GetTenantRequest{Id: testTenantID.String()})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("GetTenant có khoá nhưng không mang xã: mã = %v, muốn InvalidArgument (lỗi: %v)",
			status.Code(err), err)
	}

	// ResolveHost is the control: it IS exempt from carrying a commune, so the same dial must
	// succeed. Without this line the test above would also pass on a server that refused
	// everything, which is the failure shape the exemption list exists to make visible.
	if _, err := cl.ResolveHost(context.Background(),
		&platformv1.ResolveHostRequest{Host: testHost}); err != nil {
		t.Errorf("ResolveHost được miễn xã mà vẫn bị từ chối: %v", err)
	}
}

// A server that starts without the key accepts every call it should refuse, and the first person
// to find out would be nobody. Refusing at construction is the only failure anybody sees.
func TestServerWithoutCallerKeyCannotBeBuilt(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("dựng được máy chủ gRPC với GRPC_CALLER_KEY rỗng")
		}
	}()
	_ = buildGRPCServer(nil, fakeDeps(), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// GetTenantProfile is the first RPC on this port that DOES read the commune from context — the
// case the comment above said would inherit whatever the chain guarantees. With the key and no
// commune the server's own interceptor must refuse it; with both it answers.
func TestGetTenantProfileThroughRealChain(t *testing.T) {
	keyOnly := startServer(t, grpc.WithChainUnaryInterceptor(grpcx.UnaryClientCallerAuth(fakeCallerKey)))
	if _, err := keyOnly.GetTenantProfile(context.Background(),
		&platformv1.GetTenantProfileRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("GetTenantProfile không mang xã: mã = %v, muốn InvalidArgument (lỗi: %v)",
			status.Code(err), err)
	}

	full := startServer(t, grpc.WithChainUnaryInterceptor(
		grpcx.UnaryClientCallerAuth(fakeCallerKey),
		grpcx.UnaryClientInterceptor(),
	))
	res, err := full.GetTenantProfile(tenant.Into(context.Background(), testTenantID),
		&platformv1.GetTenantProfileRequest{})
	if err != nil {
		t.Fatalf("GetTenantProfile với khoá và xã vẫn bị từ chối: %v", err)
	}
	if res.GetProfile().GetOfficeAddress() != "Trụ sở thử" {
		t.Errorf("hồ sơ = %+v", res.GetProfile())
	}
}

// ListUploadPolicies is NOT exempt from "x-tenant-id" (platform.proto: "not a candidate for
// exemption"). Through the real chain: key without commune is refused by the SERVER's interceptor;
// key and commune gets the answer; no key is refused before anything else.
func TestListUploadPoliciesThroughRealChain(t *testing.T) {
	keyOnly := startServer(t, grpc.WithChainUnaryInterceptor(grpcx.UnaryClientCallerAuth(fakeCallerKey)))
	if _, err := keyOnly.ListUploadPolicies(context.Background(),
		&platformv1.ListUploadPoliciesRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("no commune: code = %v, want InvalidArgument (err: %v)", status.Code(err), err)
	}

	full := startServer(t, grpc.WithChainUnaryInterceptor(
		grpcx.UnaryClientCallerAuth(fakeCallerKey),
		grpcx.UnaryClientInterceptor(),
	))
	res, err := full.ListUploadPolicies(tenant.Into(context.Background(), testTenantID),
		&platformv1.ListUploadPoliciesRequest{})
	if err != nil {
		t.Fatalf("key and commune still refused: %v", err)
	}
	if len(res.GetPolicies()) != 1 ||
		res.GetPolicies()[0].GetPurpose() != platformv1.UploadPurpose_UPLOAD_PURPOSE_TENANT_LOGO {
		t.Errorf("policies = %+v", res.GetPolicies())
	}

	noKey := startServer(t)
	if _, err := noKey.ListUploadPolicies(tenant.Into(context.Background(), testTenantID),
		&platformv1.ListUploadPoliciesRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Errorf("no key: code = %v, want Unauthenticated", status.Code(err))
	}
}

// ResolveMiniApp IS NOW ON core/grpcx.methodsWithoutTenant (owner's answer to ADR 0045 CÒN MỞ #1,
// added 2026-09-25). This test used to pin the refusal; it now pins the grant — through the REAL
// server chain, dialled with the caller key and deliberately WITHOUT the commune interceptor, so
// nothing puts "x-tenant-id" on the wire. Success here can only mean the server's own chain lets
// the exempt RPC through with no commune.
//
// The caller key is still required: the commune exemption is not an authentication exemption.
func TestResolveMiniAppExemptFromTenantButNeedsKey(t *testing.T) {
	withKey := startServer(t, grpc.WithChainUnaryInterceptor(grpcx.UnaryClientCallerAuth(fakeCallerKey)))
	res, err := withKey.ResolveMiniApp(context.Background(), &platformv1.ResolveMiniAppRequest{AppId: "1234567890"})
	if err != nil {
		t.Fatalf("ResolveMiniApp có khoá, không mang xã: bị từ chối (%v) — miễn xã chưa có hiệu lực", err)
	}
	if res.GetApp().GetAppId() != "1234567890" {
		t.Errorf("app trả về = %+v", res.GetApp())
	}

	noKey := startServer(t)
	if _, err := noKey.ResolveMiniApp(context.Background(),
		&platformv1.ResolveMiniAppRequest{AppId: "1234567890"}); status.Code(err) != codes.Unauthenticated {
		t.Errorf("ResolveMiniApp không khoá: mã = %v, muốn Unauthenticated", status.Code(err))
	}
}
