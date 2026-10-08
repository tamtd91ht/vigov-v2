package main

// THE gRPC WIRING, over a real in-memory connection with the real chain — the platform
// cmd/server/main_test.go shape. An interceptor deleted from buildGRPCServer leaves a server that
// starts and answers every unauthenticated or commune-less call; only this file turns red for that.
//
// Its own file rather than main_test.go so the two can be edited independently.

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

	petitionsv1 "github.com/vihat/vigov/core/gen/vigov/petitions/v1"
	"github.com/vihat/vigov/core/grpcx"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	svcgrpc "github.com/vihat/vigov/service-petitions/internal/grpc"
)

// testCallerKey is fake key material — the text says so (rule 8, forbidden #1).
var testCallerKey = secret.Secret("khoa-goi-noi-bo-GIA-KHONG-PHAI-KHOA-THAT")

const grpcTestTenant = tenant.ID("01JD8ZQK9M3NPXR7TVWYB2C4EF")

// tenantCounter answers only when the chain put a commune in the context — and panics otherwise,
// like the real store (store.DB.For → tenant.MustFrom).
type tenantCounter struct{ n int }

func (c tenantCounter) CountOpenHeldByOrgUnit(ctx context.Context, _ string) (int, error) {
	_ = tenant.MustFrom(ctx)
	return c.n, nil
}

// tenantPriorities answers every asked code as an active priority — and panics without a commune,
// like the real store.
type tenantPriorities struct{}

func (tenantPriorities) StatesByCode(ctx context.Context, codes []string) ([]domain.TaskPriorityCodeState, error) {
	_ = tenant.MustFrom(ctx)
	out := make([]domain.TaskPriorityCodeState, 0, len(codes))
	for _, c := range codes {
		out = append(out, domain.TaskPriorityCodeState{Code: c, Active: true})
	}
	return out, nil
}

func startGRPC(t *testing.T, opts ...grpc.DialOption) petitionsv1.PetitionsServiceClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := buildGRPCServer(testCallerKey, svcgrpc.Deps{
		Petitions: tenantCounter{n: 4}, Tasks: tenantCounter{n: 5}, TaskPriorities: tenantPriorities{},
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
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
	return petitionsv1.NewPetitionsServiceClient(conn)
}

var countReq = &petitionsv1.CountOrgUnitHoldingsRequest{OrgUnitId: "bp-1"}

func TestGRPCRefusesCallerWithoutKey(t *testing.T) {
	cl := startGRPC(t)
	if _, err := cl.CountOrgUnitHoldings(tenant.Into(context.Background(), grpcTestTenant), countReq); status.Code(err) != codes.Unauthenticated {
		t.Errorf("mã = %v, muốn Unauthenticated (lỗi: %v)", status.Code(err), err)
	}
}

// Key yes, commune no — the SERVER's own interceptor must refuse. The client commune interceptor
// is deliberately absent, so nothing puts "x-tenant-id" on the wire.
func TestGRPCServerRefusesCallWithoutCommune(t *testing.T) {
	cl := startGRPC(t, grpc.WithChainUnaryInterceptor(grpcx.UnaryClientCallerAuth(testCallerKey)))
	if _, err := cl.CountOrgUnitHoldings(context.Background(), countReq); status.Code(err) != codes.InvalidArgument {
		t.Errorf("mã = %v, muốn InvalidArgument (lỗi: %v)", status.Code(err), err)
	}
}

func TestGRPCAnswersWithKeyAndCommune(t *testing.T) {
	cl := startGRPC(t, grpc.WithChainUnaryInterceptor(
		grpcx.UnaryClientCallerAuth(testCallerKey),
		grpcx.UnaryClientInterceptor(),
	))
	res, err := cl.CountOrgUnitHoldings(tenant.Into(context.Background(), grpcTestTenant), countReq)
	if err != nil {
		t.Fatalf("có khoá và xã mà vẫn bị từ chối: %v", err)
	}
	if res.GetOpenPetitions() != 4 || res.GetOpenTasks() != 5 {
		t.Errorf("= %+v", res)
	}
}

func TestGRPCServerWithoutCallerKeyDoesNotBuild(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("dựng được máy chủ gRPC với GRPC_CALLER_KEY rỗng")
		}
	}()
	_ = buildGRPCServer(nil, svcgrpc.Deps{Petitions: tenantCounter{}, Tasks: tenantCounter{}, TaskPriorities: tenantPriorities{},
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
}

// The new RPC rides the SAME chain: with key and commune it answers, without the commune it is refused
// by the interceptor before the handler runs.
func TestGRPCResolveTaskPriorityCodesThroughChain(t *testing.T) {
	cl := startGRPC(t, grpc.WithChainUnaryInterceptor(
		grpcx.UnaryClientCallerAuth(testCallerKey),
		grpcx.UnaryClientInterceptor(),
	))
	req := &petitionsv1.ResolveTaskPriorityCodesRequest{Codes: []string{"khan"}}
	res, err := cl.ResolveTaskPriorityCodes(tenant.Into(context.Background(), grpcTestTenant), req)
	if err != nil {
		t.Fatalf("refused with key and commune: %v", err)
	}
	if len(res.GetItems()) != 1 || res.GetItems()[0].GetCode() != "khan" || !res.GetItems()[0].GetActive() {
		t.Errorf("= %+v", res)
	}
	if _, err := cl.ResolveTaskPriorityCodes(context.Background(), req); status.Code(err) != codes.InvalidArgument {
		t.Errorf("no commune: code = %v, want InvalidArgument", status.Code(err))
	}
}
