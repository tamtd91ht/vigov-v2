package main

// THE gRPC WIRING, over a real in-memory connection with the real chain — the petitions
// cmd/server/grpc_server_test.go shape. An interceptor deleted from buildGRPCServer leaves a server
// that starts and answers every unauthenticated or commune-less call; only this file turns red.

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

	"github.com/vihat/vigov/core/audit"
	commsv1 "github.com/vihat/vigov/core/gen/vigov/comms/v1"
	"github.com/vihat/vigov/core/grpcx"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	svcgrpc "github.com/vihat/vigov/service-comms/internal/grpc"
)

// testCallerKey is fake key material — the text says so (rule 8, forbidden #1).
var testCallerKey = secret.Secret("khoa-goi-noi-bo-GIA-KHONG-PHAI-KHOA-THAT")

const grpcTestTenant = tenant.ID("01JD8ZQK9M3NPXR7TVWYB2C4EF")

// tenantDeliverer answers only when the chain put a commune in the context — and panics otherwise,
// like the real store (store.DB.For → tenant.MustFrom).
type tenantDeliverer struct{ commune tenant.ID }

func (d *tenantDeliverer) Deliver(ctx context.Context, in []domain.NotificationDelivery, _ audit.Actor) (
	[]domain.DeliveryOutcome, error) {
	d.commune = tenant.MustFrom(ctx)
	out := make([]domain.DeliveryOutcome, 0, len(in))
	for _, n := range in {
		out = append(out, domain.DeliveryOutcome{IdempotencyKey: n.IdempotencyKey, Created: len(n.RecipientCodes)})
	}
	return out, nil
}

// zaloBotProbe stands in for the Zalo Bot operator service and records whether the chain let a call
// through — and whether a commune reached it (it must not: the six RPCs are platform-scope).
type zaloBotProbe struct {
	commsv1.UnimplementedZaloBotOperatorServiceServer
	called     bool
	sawCommune bool
}

func (z *zaloBotProbe) GetSharedZaloBot(ctx context.Context, _ *commsv1.GetSharedZaloBotRequest) (*commsv1.GetSharedZaloBotResponse, error) {
	z.called = true
	_, z.sawCommune = tenant.From(ctx)
	return &commsv1.GetSharedZaloBotResponse{Configured: false}, nil
}

func startGRPC(t *testing.T, d *tenantDeliverer, opts ...grpc.DialOption) commsv1.CommsServiceClient {
	t.Helper()
	conn := dialGRPC(t, d, &zaloBotProbe{}, opts...)
	return commsv1.NewCommsServiceClient(conn)
}

func dialGRPC(t *testing.T, d *tenantDeliverer, z *zaloBotProbe, opts ...grpc.DialOption) *grpc.ClientConn {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := buildGRPCServer(testCallerKey, svcgrpc.Deps{
		Notifications: d, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, z)
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
	return conn
}

var deliverReq = &commsv1.DeliverStaffNotificationsRequest{Notifications: []*commsv1.StaffNotification{{
	IdempotencyKey: "escalation:NHIEM_VU:01JREC:unit_head",
	Kind:           commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_ESCALATION,
	RecipientMa:    []string{"CB-001"}, Title: "Việc trễ hạn cần xử lý",
}}}

func TestGRPCRefusesCallerWithoutKey(t *testing.T) {
	cl := startGRPC(t, &tenantDeliverer{})
	_, err := cl.DeliverStaffNotifications(tenant.Into(context.Background(), grpcTestTenant), deliverReq)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("mã = %v, muốn Unauthenticated (lỗi: %v)", status.Code(err), err)
	}
}

// Key yes, commune no — the SERVER's own interceptor must refuse.
func TestGRPCServerRefusesCallWithoutCommune(t *testing.T) {
	cl := startGRPC(t, &tenantDeliverer{}, grpc.WithChainUnaryInterceptor(grpcx.UnaryClientCallerAuth(testCallerKey)))
	if _, err := cl.DeliverStaffNotifications(context.Background(), deliverReq); status.Code(err) != codes.InvalidArgument {
		t.Errorf("mã = %v, muốn InvalidArgument (lỗi: %v)", status.Code(err), err)
	}
}

func TestGRPCAnswersWithKeyAndCommune(t *testing.T) {
	d := &tenantDeliverer{}
	cl := startGRPC(t, d, grpc.WithChainUnaryInterceptor(
		grpcx.UnaryClientCallerAuth(testCallerKey),
		grpcx.UnaryClientInterceptor(),
	))
	res, err := cl.DeliverStaffNotifications(tenant.Into(context.Background(), grpcTestTenant), deliverReq)
	if err != nil {
		t.Fatalf("có khoá và xã mà vẫn bị từ chối: %v", err)
	}
	if len(res.GetItems()) != 1 || res.GetItems()[0].GetCreated() != 1 || d.commune != grpcTestTenant {
		t.Errorf("= %+v, xã = %q", res.GetItems(), d.commune)
	}
}

func TestGRPCServerWithoutCallerKeyDoesNotBuild(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("dựng được máy chủ gRPC với GRPC_CALLER_KEY rỗng")
		}
	}()
	_ = buildGRPCServer(nil, svcgrpc.Deps{Notifications: &tenantDeliverer{},
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))}, &zaloBotProbe{})
}

// ZaloBotOperatorService is on THIS port: refused without the caller key…
func TestGRPCZaloBotRefusesCallerWithoutKey(t *testing.T) {
	z := &zaloBotProbe{}
	cl := commsv1.NewZaloBotOperatorServiceClient(dialGRPC(t, &tenantDeliverer{}, z))
	if _, err := cl.GetSharedZaloBot(context.Background(), &commsv1.GetSharedZaloBotRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Errorf("mã = %v, muốn Unauthenticated (lỗi: %v)", status.Code(err), err)
	}
	if z.called {
		t.Error("the handler ran without the caller key")
	}
}

// …and answered WITHOUT a commune (core/grpcx.methodsWithoutTenant, owner 05/10/2026), with none in
// the handler's context.
func TestGRPCZaloBotAnswersWithKeyAndNoCommune(t *testing.T) {
	z := &zaloBotProbe{}
	cl := commsv1.NewZaloBotOperatorServiceClient(dialGRPC(t, &tenantDeliverer{}, z,
		grpc.WithChainUnaryInterceptor(grpcx.UnaryClientCallerAuth(testCallerKey))))
	if _, err := cl.GetSharedZaloBot(context.Background(), &commsv1.GetSharedZaloBotRequest{}); err != nil {
		t.Fatalf("with the key and no commune, refused: %v", err)
	}
	if !z.called || z.sawCommune {
		t.Errorf("called=%v sawCommune=%v", z.called, z.sawCommune)
	}
}
