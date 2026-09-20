package main

// What this test defends: THE WIRING, not the interceptors.
//
// core/grpcx proves that UnaryServerCallerAuth refuses a call with no key and that
// UnaryServerInterceptor refuses a call with no commune. Neither of those tests can see whether
// this binary actually installs them. An interceptor deleted from the chain in dungGRPCServer
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
)

// khoaGoiGia is fake key material — the text says so in full (rule 8, forbidden #1).
var khoaGoiGia = secret.Secret("khoa-goi-noi-bo-GIA-KHONG-PHAI-KHOA-THAT")

const (
	hostThu = "thangbinh.example.gov.vn"
	ulidThu = tenant.ID("01JD8ZQK9M3NPXR7TVWYB2C4EF")
)

// danhBaGia is the registry, absent. The test is about the chain in front of the handlers, so
// the handlers only have to answer something.
type danhBaGia struct{}

func (danhBaGia) ByHostErr(context.Context, string) (tenant.Tenant, error) {
	return tenant.Tenant{ID: ulidThu, Host: hostThu, Active: true}, nil
}

func (danhBaGia) ByID(_ context.Context, id tenant.ID) (tenant.Tenant, error) {
	return tenant.Tenant{ID: id, Host: hostThu, Active: true}, nil
}

// moMay starts the real server on an in-memory connection and returns a client dialled with the
// given interceptors — so a test can choose to dial like a correctly configured service, or like
// something that just opened a socket to the port.
func moMay(t *testing.T, opts ...grpc.DialOption) platformv1.PlatformServiceClient {
	t.Helper()

	lis := bufconn.Listen(1 << 20)
	srv := dungGRPCServer(khoaGoiGia, danhBaGia{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
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
func TestCongGRPCTuChoiBenGoiKhongCoKhoa(t *testing.T) {
	cl := moMay(t) // no interceptors at all: the naked caller

	if _, err := cl.ResolveHost(context.Background(),
		&platformv1.ResolveHostRequest{Host: hostThu}); status.Code(err) != codes.Unauthenticated {
		t.Errorf("ResolveHost không khoá: mã = %v, muốn Unauthenticated (lỗi: %v)",
			status.Code(err), err)
	}

	if _, err := cl.GetTenant(context.Background(),
		&platformv1.GetTenantRequest{Id: ulidThu.String()}); status.Code(err) != codes.Unauthenticated {
		t.Errorf("GetTenant không khoá: mã = %v, muốn Unauthenticated (lỗi: %v)",
			status.Code(err), err)
	}
}

// The other half of the same statement: with the key, the chain lets the call through and the
// commune interceptor behind it still does its own job. A server that refused everything would
// pass the test above and be just as broken.
func TestCongGRPCChoQuaKhiDungCaHaiDieuKien(t *testing.T) {
	cl := moMay(t, grpc.WithChainUnaryInterceptor(
		grpcx.UnaryClientCallerAuth(khoaGoiGia),
		grpcx.UnaryClientInterceptor(),
	))

	// ResolveHost: key yes, commune no — it is the call that establishes one.
	ra, err := cl.ResolveHost(context.Background(), &platformv1.ResolveHostRequest{Host: hostThu})
	if err != nil {
		t.Fatalf("ResolveHost với khoá đúng vẫn bị từ chối: %v", err)
	}
	if ra.GetTenant().GetId() != ulidThu.String() {
		t.Errorf("ResolveHost trả xã = %q", ra.GetTenant().GetId())
	}

	// GetTenant: key AND commune. The commune comes from the context, never from the body.
	ctx := tenant.Into(context.Background(), ulidThu)
	if _, err := cl.GetTenant(ctx, &platformv1.GetTenantRequest{Id: ulidThu.String()}); err != nil {
		t.Fatalf("GetTenant với khoá và xã đầy đủ vẫn bị từ chối: %v", err)
	}

	// ...and the commune interceptor is still in the chain behind the key: the key alone does
	// not buy a commune. Refused by the CLIENT interceptor, before it leaves the process.
	if _, err := cl.GetTenant(context.Background(),
		&platformv1.GetTenantRequest{Id: ulidThu.String()}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("GetTenant thiếu xã: mã = %v, muốn InvalidArgument (lỗi: %v)",
			status.Code(err), err)
	}
}

// A server that starts without the key accepts every call it should refuse, and the first person
// to find out would be nobody. Refusing at construction is the only failure anybody sees.
func TestKhongCoKhoaGoiThiKhongDungDuocMayChu(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("dựng được máy chủ gRPC với GRPC_CALLER_KEY rỗng")
		}
	}()
	_ = dungGRPCServer(nil, danhBaGia{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}
