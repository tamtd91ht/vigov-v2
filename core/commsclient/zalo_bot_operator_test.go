package commsclient

// What these tests defend for the Zalo Bot operator client: the six RPCs pass the REAL server chain
// with no commune at all (they are exempt by full name — a commune in ctx is not sent either), the
// caller key travels, a call gets a deadline, and an empty address is refused by name.

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	commsv1 "github.com/vihat/vigov/core/gen/vigov/comms/v1"
	"github.com/vihat/vigov/core/grpcx"
	"github.com/vihat/vigov/core/tenant"
)

type fakeZaloBotServer struct {
	commsv1.UnimplementedZaloBotOperatorServiceServer

	calls       int
	sawTenant   [][]string
	sawKey      [][]string
	sawDeadline []bool
}

func (s *fakeZaloBotServer) record(ctx context.Context) {
	s.calls++
	md, _ := metadata.FromIncomingContext(ctx)
	s.sawTenant = append(s.sawTenant, md.Get(grpcx.MetadataTenantKey))
	s.sawKey = append(s.sawKey, md.Get(grpcx.MetadataCallerKey))
	_, has := ctx.Deadline()
	s.sawDeadline = append(s.sawDeadline, has)
}

func (s *fakeZaloBotServer) SetSharedZaloBot(ctx context.Context, _ *commsv1.SetSharedZaloBotRequest) (*commsv1.SetSharedZaloBotResponse, error) {
	s.record(ctx)
	return &commsv1.SetSharedZaloBotResponse{}, nil
}

func (s *fakeZaloBotServer) GetSharedZaloBot(ctx context.Context, _ *commsv1.GetSharedZaloBotRequest) (*commsv1.GetSharedZaloBotResponse, error) {
	s.record(ctx)
	return &commsv1.GetSharedZaloBotResponse{}, nil
}

func (s *fakeZaloBotServer) CheckSharedZaloBot(ctx context.Context, _ *commsv1.CheckSharedZaloBotRequest) (*commsv1.CheckSharedZaloBotResponse, error) {
	s.record(ctx)
	return &commsv1.CheckSharedZaloBotResponse{}, nil
}

func (s *fakeZaloBotServer) SetSharedZaloBotWebhook(ctx context.Context, _ *commsv1.SetSharedZaloBotWebhookRequest) (*commsv1.SetSharedZaloBotWebhookResponse, error) {
	s.record(ctx)
	return &commsv1.SetSharedZaloBotWebhookResponse{}, nil
}

func (s *fakeZaloBotServer) GetSharedZaloBotWebhook(ctx context.Context, _ *commsv1.GetSharedZaloBotWebhookRequest) (*commsv1.GetSharedZaloBotWebhookResponse, error) {
	s.record(ctx)
	return &commsv1.GetSharedZaloBotWebhookResponse{}, nil
}

func (s *fakeZaloBotServer) ListZaloBotCommuneStats(ctx context.Context, _ *commsv1.ListZaloBotCommuneStatsRequest) (*commsv1.ListZaloBotCommuneStatsResponse, error) {
	s.record(ctx)
	return &commsv1.ListZaloBotCommuneStatsResponse{}, nil
}

func startZaloBot(t *testing.T, srv commsv1.ZaloBotOperatorServiceServer) *ZaloBotOperatorClient {
	t.Helper()
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	lis := bufconn.Listen(1 << 20)
	gs := grpc.NewServer(grpc.ChainUnaryInterceptor(
		grpcx.UnaryServerCallerAuth(fakeCallerKey, quiet),
		grpcx.UnaryServerInterceptor(),
	))
	commsv1.RegisterZaloBotOperatorServiceServer(gs, srv)
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithChainUnaryInterceptor(ZaloBotOperatorInterceptors(fakeCallerKey)...),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return NewZaloBotOperatorFromConn(conn)
}

func TestZaloBotOperatorRPCsTravelWithoutCommune(t *testing.T) {
	srv := &fakeZaloBotServer{}
	c := startZaloBot(t, srv)
	for name, ctx := range map[string]context.Context{
		"no commune":         context.Background(),
		"commune in context": tenant.Into(context.Background(), communeA),
	} {
		srv.calls, srv.sawTenant, srv.sawKey, srv.sawDeadline = 0, nil, nil, nil
		calls := []func() error{
			func() error { _, err := c.SetSharedZaloBot(ctx, &commsv1.SetSharedZaloBotRequest{}); return err },
			func() error { _, err := c.GetSharedZaloBot(ctx, &commsv1.GetSharedZaloBotRequest{}); return err },
			func() error { _, err := c.CheckSharedZaloBot(ctx, &commsv1.CheckSharedZaloBotRequest{}); return err },
			func() error {
				_, err := c.SetSharedZaloBotWebhook(ctx, &commsv1.SetSharedZaloBotWebhookRequest{})
				return err
			},
			func() error {
				_, err := c.GetSharedZaloBotWebhook(ctx, &commsv1.GetSharedZaloBotWebhookRequest{})
				return err
			},
			func() error {
				_, err := c.ListZaloBotCommuneStats(ctx, &commsv1.ListZaloBotCommuneStatsRequest{})
				return err
			},
		}
		for i, call := range calls {
			if err := call(); err != nil {
				t.Fatalf("%s: call %d refused: %v — every RPC of the service is exempt by full name", name, i+1, err)
			}
		}
		if srv.calls != 6 {
			t.Fatalf("%s: %d calls reached the server, want 6", name, srv.calls)
		}
		for i := range srv.sawTenant {
			if len(srv.sawTenant[i]) != 0 {
				t.Errorf("%s: call %d carried x-tenant-id %v — a platform-scope call names no commune", name, i+1, srv.sawTenant[i])
			}
			if len(srv.sawKey[i]) != 1 {
				t.Errorf("%s: call %d carried caller key %d times, want 1", name, i+1, len(srv.sawKey[i]))
			}
			if !srv.sawDeadline[i] {
				t.Errorf("%s: call %d had no deadline", name, i+1)
			}
		}
	}
}

func TestBoundCallKeepsAnEarlierDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	want, _ := ctx.Deadline()
	err := boundCall(ctx, "/m", nil, nil, nil, func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		if got, _ := ctx.Deadline(); !got.Equal(want) {
			return errors.New("the caller's earlier deadline was extended")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDialZaloBotOperatorRefusesEmptyAddress(t *testing.T) {
	if _, err := DialZaloBotOperator("", fakeCallerKey); err == nil {
		t.Fatal("an empty COMMS_GRPC_ADDR must be refused at wiring time")
	}
	var c *ZaloBotOperatorClient
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}
