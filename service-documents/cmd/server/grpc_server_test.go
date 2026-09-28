package main

// THE gRPC WIRING, over a real in-memory connection with the real chain — the platform
// cmd/server/main_test.go shape. An interceptor deleted from buildGRPCServer leaves a server that
// answers every unauthenticated or commune-less call; only this file turns red for that.

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

	documentsv1 "github.com/vihat/vigov/core/gen/vigov/documents/v1"
	"github.com/vihat/vigov/core/grpcx"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	svcgrpc "github.com/vihat/vigov/service-documents/internal/grpc"
)

// testCallerKey is fake key material — the text says so (rule 8, forbidden #1).
var testCallerKey = secret.Secret("khoa-goi-noi-bo-GIA-KHONG-PHAI-KHOA-THAT")

const grpcTestTenant = tenant.ID("01JD8ZQK9M3NPXR7TVWYB2C4EF")

// tenantCounter panics without a commune, like the real store (store.DB.For → tenant.MustFrom).
type tenantCounter struct{ n int }

func (c tenantCounter) CountOpenHeldByOrgUnit(ctx context.Context, _ string) (int, error) {
	_ = tenant.MustFrom(ctx)
	return c.n, nil
}

func startGRPC(t *testing.T, opts ...grpc.DialOption) documentsv1.DocumentsServiceClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := buildGRPCServer(testCallerKey, svcgrpc.Deps{Incoming: tenantCounter{n: 6},
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
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
	return documentsv1.NewDocumentsServiceClient(conn)
}

var countReq = &documentsv1.CountOrgUnitHoldingsRequest{OrgUnitId: "bp-1"}

func TestGRPCRefusesCallerWithoutKey(t *testing.T) {
	cl := startGRPC(t)
	if _, err := cl.CountOrgUnitHoldings(tenant.Into(context.Background(), grpcTestTenant), countReq); status.Code(err) != codes.Unauthenticated {
		t.Errorf("code = %v, want Unauthenticated (err: %v)", status.Code(err), err)
	}
}

// Key yes, commune no — the SERVER's own interceptor must refuse.
func TestGRPCServerRefusesCallWithoutCommune(t *testing.T) {
	cl := startGRPC(t, grpc.WithChainUnaryInterceptor(grpcx.UnaryClientCallerAuth(testCallerKey)))
	if _, err := cl.CountOrgUnitHoldings(context.Background(), countReq); status.Code(err) != codes.InvalidArgument {
		t.Errorf("code = %v, want InvalidArgument (err: %v)", status.Code(err), err)
	}
}

func TestGRPCAnswersWithKeyAndCommune(t *testing.T) {
	cl := startGRPC(t, grpc.WithChainUnaryInterceptor(
		grpcx.UnaryClientCallerAuth(testCallerKey),
		grpcx.UnaryClientInterceptor(),
	))
	res, err := cl.CountOrgUnitHoldings(tenant.Into(context.Background(), grpcTestTenant), countReq)
	if err != nil {
		t.Fatalf("refused with key and commune: %v", err)
	}
	if res.GetOpenIncomingDocuments() != 6 {
		t.Errorf("= %+v", res)
	}
}

func TestGRPCServerWithoutCallerKeyDoesNotBuild(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("built a gRPC server with an empty GRPC_CALLER_KEY")
		}
	}()
	_ = buildGRPCServer(nil, svcgrpc.Deps{Incoming: tenantCounter{},
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
}
