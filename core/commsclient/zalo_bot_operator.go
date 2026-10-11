package commsclient

// The second comms surface a caller reaches from here: ZaloBotOperatorService
// (proto/vigov/comms/v1/zalo_bot_operator.proto, ADR 0074). ONE CALLER: service-platform's operator
// routes `zalo-bots/shared…`, for a request a person made, after platform's own
// ResolveOperatorSession and `ops.zalo_bot.manage` check.
//
// A SEPARATE CLIENT FROM Client, ON PURPOSE — the proto's own reason: a caller holding a CommsService
// client cannot reach a bot-token RPC by a typo, and a caller wired for the bot screen holds nothing
// that delivers staff notices.
//
// NO COMMUNE TRAVELS. Every RPC of the service is on core/grpcx.methodsWithoutTenant by full name
// (owner, 2026-10-05), so grpcx.UnaryClientInterceptor sends no x-tenant-id for them. It is kept in
// the chain anyway, exactly as operatorclient keeps it: a seventh RPC added to the service but not to
// that list must be refused locally, not sent bare.
//
// NEVER LOG A REQUEST OF THIS SERVICE. SetSharedZaloBotRequest.String() prints the bot token. This
// file logs nothing at all; the caller logs status codes only.

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	commsv1 "github.com/vihat/vigov/core/gen/vigov/comms/v1"
	"github.com/vihat/vigov/core/grpcx"
	"github.com/vihat/vigov/core/secret"
)

// ZaloBotCallTimeout bounds one call when the caller's context has no earlier deadline. Longer than
// comms' own Zalo call budget (15 s, service-comms/internal/zalobot.DefaultTimeout) so that comms —
// not this hop — is the one that gives up on Zalo, and still gets to record the outcome class it
// promises (SetSharedZaloBotWebhook: a transaction before and after the Zalo call).
const ZaloBotCallTimeout = 25 * time.Second

// ZaloBotOperatorClient is the generated client plus the connection Dial opened.
type ZaloBotOperatorClient struct {
	commsv1.ZaloBotOperatorServiceClient
	conn *grpc.ClientConn // nil when built with NewZaloBotOperatorFromConn
}

// ZaloBotOperatorInterceptors is the chain the connection MUST carry: caller key first, then the
// commune interceptor (which sends nothing for these exempt methods), then the call deadline.
// key is GRPC_CALLER_KEY; an empty one panics inside grpcx.UnaryClientCallerAuth, at wiring time.
func ZaloBotOperatorInterceptors(key secret.Secret) []grpc.UnaryClientInterceptor {
	return []grpc.UnaryClientInterceptor{grpcx.UnaryClientCallerAuth(key), grpcx.UnaryClientInterceptor(), boundCall}
}

// boundCall applies ZaloBotCallTimeout. context.WithTimeout keeps an earlier deadline the caller set.
func boundCall(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	ctx, cancel := context.WithTimeout(ctx, ZaloBotCallTimeout)
	defer cancel()
	return invoker(ctx, method, req, reply, cc, opts...)
}

// NewZaloBotOperatorFromConn builds a client on a connection the caller opened with
// ZaloBotOperatorInterceptors — for tests over bufconn.
func NewZaloBotOperatorFromConn(cc grpc.ClientConnInterface) *ZaloBotOperatorClient {
	return &ZaloBotOperatorClient{ZaloBotOperatorServiceClient: commsv1.NewZaloBotOperatorServiceClient(cc)}
}

// DialZaloBotOperator opens the platform → comms channel to COMMS_GRPC_ADDR (comms' inter-service
// port, where ZaloBotOperatorService is registered beside CommsService). Fails closed and by name on
// an empty address, the shape of Dial: a client with no address answers every bot route 503 for a
// reason the logs would never name.
func DialZaloBotOperator(addr string, key secret.Secret) (*ZaloBotOperatorClient, error) {
	if addr == "" {
		return nil, fmt.Errorf("commsclient: thiếu địa chỉ COMMS_GRPC_ADDR (bot Zalo dùng chung, ADR 0074)")
	}
	conn, err := grpc.NewClient(addr,
		// @security-exception: service-to-service gRPC has no TLS yet — same in-cluster plaintext channel debt as Dial/identityclient (tools/security_debt.json, expires 2026-12-28), converted together when internal TLS lands; carries the shared Zalo bot TOKEN on SetSharedZaloBot (write-only, never echoed), operator business codes and client IPs, and the deployment-wide GRPC_CALLER_KEY — readable by anyone on the cluster network until TLS lands, so only the platform→comms:9090 NetworkPolicy narrows it
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(ZaloBotOperatorInterceptors(key)...),
	)
	if err != nil {
		return nil, fmt.Errorf("commsclient: mở kết nối tới %s: %w", addr, err)
	}
	return &ZaloBotOperatorClient{ZaloBotOperatorServiceClient: commsv1.NewZaloBotOperatorServiceClient(conn), conn: conn}, nil
}

// Close releases the connection. Safe on a client built with NewZaloBotOperatorFromConn and on nil.
func (c *ZaloBotOperatorClient) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}
