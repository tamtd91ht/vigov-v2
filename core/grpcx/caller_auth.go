package grpcx

// Caller authentication on the inter-service gRPC port.
//
// WHAT THIS FILE IS FOR, IN ONE SENTENCE: without it, anything that can open a TCP connection
// to the gRPC port can call every RPC on it. The package doc above states the shape of the
// scheme, the second layer it depends on, and the three things it deliberately does not buy.
// Read it before changing anything here.
//
// THE TWO AXES OF THIS BOUNDARY ARE ORTHOGONAL, and confusing them is the single most likely
// way to break this file:
//
//	"which commune"  -> UnaryServerInterceptor  -> exempt for ResolveHost (grpcx.go)
//	"who is calling" -> UnaryServerCallerAuth   -> EXEMPT FOR NOBODY
//
// ResolveHost is excused from carrying a COMMUNE because it is the call that establishes one —
// requiring the commune would mean knowing the commune in order to find the commune. That
// exemption says nothing whatever about the CALLER. Nothing in this file reads
// methodsWithoutTenant or calls ExemptFromTenant, and that absence is the design, not an
// omission: the moment the tenant exemption list doubles as a list of RPCs that skip
// authentication, the one RPC every unauthenticated process reaches is the registry lookup.

import (
	"context"
	"crypto/subtle"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/vihat/vigov/core/secret"
)

// tuChoi is the ONE message a refused caller ever receives.
//
// It is deliberately identical for "no key", "wrong key" and "two keys". Telling the three
// apart is free diagnostic help for whoever is guessing, and it is help the legitimate caller
// does not need — a misconfigured service is diagnosed from the SERVER's log line, which names
// the method and the peer and never the value. Same reasoning as rule 4, forbidden #2: a
// response that varies with the reason leaks the reason.
const tuChoi = "unauthenticated"

// UnaryServerCallerAuth refuses any RPC that does not carry the deployment's caller key.
//
// IT PANICS ON AN EMPTY KEY, at construction, and that is the whole point of checking here
// rather than per request. A server wired with no key would accept every call it is supposed
// to refuse, on every RPC, and NOTHING WOULD LOOK WRONG: no error, no failed request, no
// metric moving. The first person to find out would be nobody. Same discipline as
// identity/http.Register — refuse incomplete wiring at construction, where a human is watching
// a process fail to start, rather than at request time where the failure is silence.
//
// There is no dev exemption and no env check. A default on this path is rule 1, forbidden #1
// applied one boundary over: the one configuration that must not be possible is "running
// without the check".
//
// Wire it FIRST in the chain, before UnaryServerInterceptor: an unauthenticated caller must
// not reach the commune logic at all, or the errors it receives start describing what the
// server expected next.
func UnaryServerCallerAuth(khoa secret.Secret, log *slog.Logger) grpc.UnaryServerInterceptor {
	if khoa.Rong() {
		panic("grpcx: thiếu khoá gọi nội bộ (GRPC_CALLER_KEY) — " +
			"máy chủ gRPC khởi động được sẽ nhận MỌI lời gọi đáng lẽ phải từ chối")
	}
	if log == nil {
		log = slog.Default()
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (any, error) {

		// No method check. See the note at the top of this file: every RPC, including the ones
		// exempt from carrying a commune.
		var vals []string
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			vals = md.Get(MetadataCallerKey)
		}

		// CONSTANT TIME, NOT ==. This comparison is reachable by anything that can open a TCP
		// connection to the port, and a byte-by-byte comparison that returns early is a timing
		// oracle: it answers "how many leading bytes were right", which recovers the key one
		// byte at a time instead of by brute force. ConstantTimeCompare returns 0 immediately
		// when the lengths differ, so the LENGTH of the key is observable — accepted: the
		// length is not the secret, the bytes are.
		//
		// The short-circuit on len(vals) is safe to read early because no secret is involved
		// in it, and two values must be refused rather than searched: accepting a call because
		// ONE of several offered keys matched is an invitation to offer several.
		if len(vals) != 1 || subtle.ConstantTimeCompare([]byte(vals[0]), khoa.Lo()) != 1 {
			// The cause goes to THIS service's log, never to the caller, and NEVER with the
			// value — neither the expected one nor the one offered. A rejected value is
			// usually a real key from a stale deployment, and a log line is exactly the place
			// rule 8 says a secret must never reach.
			log.WarnContext(ctx, "CẢNH BÁO AN NINH: từ chối lời gọi gRPC không có khoá gọi nội bộ hợp lệ",
				"method", info.FullMethod,
				"so_khoa_nhan_duoc", len(vals),
				"tu", diaChiGoi(ctx))
			return nil, status.Error(codes.Unauthenticated, tuChoi)
		}

		return handler(ctx, req)
	}
}

// UnaryClientCallerAuth attaches the deployment's caller key to every outgoing call.
//
// EVERY outgoing call, with no method list, mirroring the server exactly. An asymmetry here
// does not fail safe or unsafe — it fails CONFUSING: one RPC refused in production while the
// rest work, diagnosed as a network fault for as long as it takes somebody to read both lists.
//
// It panics on an empty key for the same reason the server does, with a different
// consequence: this end would send every call without a key and have every one refused. Loud
// either way, but at construction it names the missing variable instead of producing eight
// services' worth of Unauthenticated.
func UnaryClientCallerAuth(khoa secret.Secret) grpc.UnaryClientInterceptor {
	if khoa.Rong() {
		panic("grpcx: thiếu khoá gọi nội bộ (GRPC_CALLER_KEY) — " +
			"mọi lời gọi gRPC đi ra sẽ bị đầu kia từ chối")
	}

	return func(ctx context.Context, method string, req, reply any,
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {

		// khoa.Lo() is called HERE and not hoisted into a variable above on purpose: the raw
		// material exists only as the argument of the call that consumes it. A string holding
		// the key in the closure is a plain string again, and the next person to print the
		// interceptor while debugging gets the deployment's key (pkg/secret, Lo).
		ctx = metadata.AppendToOutgoingContext(ctx, MetadataCallerKey, string(khoa.Lo()))
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// diaChiGoi names where a refused call came from, so a rejection can be told apart from an
// intrusion attempt without turning on anything extra.
//
// A pod IP is not personal data (rule 3): it identifies a process inside the cluster, and it
// is the only handle an operator has when the answer is "some caller, somewhere, is stale".
func diaChiGoi(ctx context.Context) string {
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		return p.Addr.String()
	}
	return "không rõ"
}
