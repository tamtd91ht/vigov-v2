package operatorclient

import (
	"fmt"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/secret"
)

// Dial opens the platform → identity OperatorService channel to IDENTITY_OPERATOR_GRPC_ADDR —
// identity's OPERATOR listener (:9093), never the staff port 9090. Identity no longer registers
// OperatorService on 9090 (user decision 01/10/2026): five staff services reach that port with the
// shared caller key, and a NetworkPolicy can only admit platform alone if the operator RPCs sit on a
// port of their own.
//
// FAIL CLOSED AND BY NAME on an empty address, the identityclient.Dial shape: a client with no
// address resolves no operator session, and every operator request would answer 503 for a reason
// the logs never name.
//
// THE TRANSPORT IS PLAINTEXT, AND THAT IS A RECORDED DEBT, NOT A DEFAULT. User's decision of
// 2026-10-01: this channel carries the same debt as the existing in-cluster gRPC channels (ADR 0025,
// core/identityclient, core/platformclient — tools/security_debt.json, expiry 2026-12-28) and is a
// go-live blocker. What has to be true before OPERATOR_HOST is set anywhere real: the NetworkPolicy
// rule platform → identity 9093 (deploy/base/mang/netpol.yaml, rule 11) applied on the live cluster.
// Replacing insecure.NewCredentials with TLS here is the fix, and it closes the debt for all three.
func Dial(addr string, key secret.Secret, log *slog.Logger) (*Client, error) {
	if addr == "" {
		return nil, fmt.Errorf("operatorclient: thiếu địa chỉ IDENTITY_OPERATOR_GRPC_ADDR (cổng vận hành của identity, :9093)")
	}
	conn, err := grpc.NewClient(addr,
		// @security-exception: debt=grpc-plaintext-operator-channel — user decision 2026-10-01, same in-cluster plaintext gRPC debt as core/identityclient (tools/security_debt.json tracks the expiry); go-live blocker until the platform→identity:9093 NetworkPolicy is applied
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(UnaryInterceptors(key)...),
	)
	if err != nil {
		return nil, fmt.Errorf("operatorclient: mở kết nối tới %s: %w", addr, err)
	}
	c := New(identityv1.NewOperatorServiceClient(conn), log)
	c.conn = conn
	return c, nil
}

// Close releases the connection Dial opened. Safe on an injected client.
func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}
