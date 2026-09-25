package grpcx

// Caller authentication on the CITIZEN-SESSION BRIDGE listener of service-identity (ADR 0045
// §Tin cậy) — a different boundary from the inter-service port of caller_auth.go, with a different
// caller, a different key and a different metadata name.
//
// WHY IT LIVES HERE AND NOT IN service-identity: this package is where every out-of-band value of a
// gRPC boundary is named and read (MetadataTenantKey, MetadataCallerKey). A third key declared
// somewhere else is a key a reviewer of this package never sees next to the other two — and the
// three must stay visibly distinct, because a holder of one must never be able to try it where
// another is read.
//
// WHAT THE BRIDGE KEY PROVES: the caller is the vihat-miniapp backend. NOTHING about the citizen —
// app_id, zalo_user_id, verified_phone, client_ip are that caller's claims (ADR 0045 §Tin cậy).

import (
	"context"
	"crypto/subtle"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/vihat/vigov/core/secret"
)

// MetadataBridgeKey is the metadata name the bridge caller sends its key under.
//
// PART OF THE CONTRACT WITH vihat-miniapp. DELIBERATELY NOT MetadataCallerKey: a process holding
// the bridge key must get Unauthenticated on the inter-service port by NAME, not merely by value,
// and an alert line must say which boundary was probed. Outside the "x-tenant" prefix for the
// reason MetadataCallerKey gives.
const MetadataBridgeKey = "x-vigov-bridge-key"

// BridgeKeyMinLen is the minimum bridge key length in bytes — what `openssl rand -base64 32` yields
// before encoding. A shorter configured key is refused at construction.
const BridgeKeyMinLen = 32

// UnaryServerBridgeKey refuses any call not carrying one of the configured bridge keys.
//
// A LIST, ANY ENTRY ACCEPTED — the rotation shape ADR 0045 asks for, because vihat-miniapp and
// identity deploy independently: add the new key, switch vihat-miniapp, drop the old one. Every
// entry is compared in constant time with no early exit, so the response time says neither how many
// bytes matched nor WHICH key was close. Exactly ONE offered value is accepted, for the reason
// UnaryServerCallerAuth gives: accepting a call because one of several offered keys matched is an
// invitation to offer several.
//
// IT PANICS AT CONSTRUCTION on an empty list or a short key: a bridge started without a usable key
// is a port nobody can use, one edit away from a port everybody can.
func UnaryServerBridgeKey(khoa []secret.Secret, log *slog.Logger) grpc.UnaryServerInterceptor {
	if len(khoa) == 0 {
		panic("grpcx: thiếu khoá cầu (CITIZEN_SESSION_BRIDGE_KEYS) — cổng cầu không được mở không khoá")
	}
	for _, k := range khoa {
		if len(k.Lo()) < BridgeKeyMinLen {
			// Neither the value nor its position is printed: neither helps anyone but a guesser.
			panic("grpcx: một khoá cầu ngắn hơn 32 byte — sinh bằng `openssl rand -base64 32`")
		}
	}
	if log == nil {
		log = slog.Default()
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (any, error) {

		var vals []string
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			vals = md.Get(MetadataBridgeKey)
		}
		khop := 0
		if len(vals) == 1 {
			nhan := []byte(vals[0])
			for _, k := range khoa {
				khop |= subtle.ConstantTimeCompare(nhan, k.Lo())
			}
		}
		if khop != 1 {
			// Cause to THIS log, never a value — neither the offered one nor an expected one (rule 8).
			log.WarnContext(ctx, "CẢNH BÁO AN NINH: từ chối lời gọi cổng cầu phiên không có khoá cầu hợp lệ",
				"method", info.FullMethod,
				"so_khoa_nhan_duoc", len(vals),
				"tu", diaChiGoi(ctx))
			return nil, status.Error(codes.Unauthenticated, tuChoi)
		}
		return handler(ctx, req)
	}
}
