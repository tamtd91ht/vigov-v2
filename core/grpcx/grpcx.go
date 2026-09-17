// Package grpcx carries the commune across a gRPC hop.
//
// A gRPC call is the one place where the commune has to leave context.Context and travel as
// data. pkg/tenant keeps tenant_id out of every function signature precisely so it cannot be
// passed wrong; this package is the single, narrow exception, and it exists so that the
// exception is written ONCE instead of at every call site.
//
// Two halves of one rule, and they must stay symmetric:
//
//	client → reads the commune from context, writes it into outgoing metadata
//	server → reads it back out of incoming metadata, puts it into context
//
// Between those two points the commune is never a business argument, never a message field,
// and never something the caller can name for itself (rule 1, forbidden #2).
//
// Only unary calls are handled, because only unary RPCs exist today. THE FIRST STREAMING RPC
// NEEDS A STREAM INTERCEPTOR ADDED HERE BEFORE IT IS REGISTERED: without one the stream path
// runs with no commune in context and nothing turns red — the server handler simply sees an
// empty context and pkg/store refuses or, worse, some future code defaults.
package grpcx

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/vihat/vigov/core/tenant"
)

// MetadataTenantKey is the ONE name the commune travels under on the wire.
//
// Every .proto file states this key in its contract comment; this constant is where it
// actually lives. Both interceptors below read it from here, so the string is never typed
// twice and the two ends cannot drift apart.
//
// WHY THE "x-tenant" PREFIX IS LOAD-BEARING, NOT COSMETIC: pkg/httpx.StripTenantHeaders
// (pkg/httpx/edge.go:39) deletes every inbound header whose name begins with "x-tenant",
// because a client naming its own commune is a client granting itself access. gRPC carries
// metadata as HTTP/2 headers, so a key inside that prefix is already inside the sweep that
// exists to stop exactly this. Renaming it to something "tidier" moves the key OUT of that
// sweep, and the protection disappears without a single test turning red.
//
// Lower-case because gRPC lower-cases every metadata key. A constant in any other case
// compares unequal and silently never matches.
const MetadataTenantKey = "x-tenant-id"

// MethodResolveHost is the one RPC that answers "which commune", and therefore the one that
// cannot itself carry a commune. Declared as a constant so the exemption list below is a list
// of names a reviewer can read, not a string literal buried in a condition.
const MethodResolveHost = "/vigov.platform.v1.PlatformService/ResolveHost"

// methodsWithoutTenant is the WHITELIST of RPCs allowed to travel without a commune.
//
// EXEMPTION IS BY LIST, NEVER BY DEFAULT. An implicit exemption — "no metadata, so probably
// an infrastructure call, let it through" — is an exemption nobody can audit: it applies to
// every RPC written afterwards, including the ones that read business data. This map is the
// complete set, and adding to it is a visible diff that has to be justified.
//
// ResolveHost is exempt for a reason that cannot be designed away: it runs BEFORE any commune
// is known, so requiring the commune would mean knowing the commune in order to find the
// commune. The platform contract declares this exception explicitly
// (proto/vigov/platform/v1/platform.proto) and ADR 0003 is what keeps it harmless — that
// service has no path to business content at all.
var methodsWithoutTenant = map[string]struct{}{
	MethodResolveHost: {},
}

// ExemptFromTenant reports whether fullMethod may be called without a commune.
//
// Exported so a test can assert the list, and so /review-isolation has one function to point
// at rather than a condition to re-derive.
func ExemptFromTenant(fullMethod string) bool {
	_, ok := methodsWithoutTenant[fullMethod]
	return ok
}

// UnaryClientInterceptor writes the commune from context into outgoing metadata.
//
// A call with no commune in context is refused HERE, before anything leaves the process. The
// receiver would refuse it anyway, but refusing locally keeps a request that can only fail off
// the network, and points the stack trace at the caller that forgot rather than at a remote
// service that is behaving correctly.
//
// It uses tenant.From and NOT tenant.MustFrom on purpose. MustFrom panics by design, which is
// right inside an HTTP handler where httpx.Recover turns it into a traceable 500. An outgoing
// call can just as easily be made from a background worker with no recover above it, and
// taking the process down there converts one mis-wired call into an outage.
func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any,
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {

		if ExemptFromTenant(method) {
			// Deliberately sends nothing. Attaching a commune "if one happens to be there"
			// would make the exemption depend on the caller's state instead of on this list.
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		id, ok := tenant.From(ctx)
		if !ok {
			return status.Errorf(codes.InvalidArgument,
				"grpcx: %v — refusing to call %s without a commune", tenant.ErrNoTenant, method)
		}

		// Append, not overwrite. If a key is somehow already present the receiver sees two
		// values and refuses the call loudly; overwriting would silently discard whichever
		// claim was there, and silence is how an isolation defect survives.
		ctx = metadata.AppendToOutgoingContext(ctx, MetadataTenantKey, id.String())
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// UnaryServerInterceptor lifts the commune out of incoming metadata and into the context.
//
// Missing, empty, malformed or ambiguous — all four are refused with InvalidArgument. None of
// them is repaired, and none falls back to a default: a default on the isolation path serves
// one commune's data under another commune's name, silently, with every test still green
// (rule 1, forbidden #1).
//
// It sets the commune BEFORE the handler runs, so no handler in any service ever reads
// metadata itself. A handler that does is a handler that can forget to.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (any, error) {

		if ExemptFromTenant(info.FullMethod) {
			// Runs with NO commune in context, deliberately. An exempt handler that needs one
			// is a handler that does not belong on this list.
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument,
				"grpcx: %s requires metadata %q — none present", info.FullMethod, MetadataTenantKey)
		}

		vals := md.Get(MetadataTenantKey)
		switch {
		case len(vals) == 0:
			return nil, status.Errorf(codes.InvalidArgument,
				"grpcx: %s requires metadata %q", info.FullMethod, MetadataTenantKey)
		case len(vals) > 1:
			// Two communes named in one call. Picking either one is picking which government
			// body's data this request reads — that is a decision no interceptor may make.
			return nil, status.Errorf(codes.InvalidArgument,
				"grpcx: metadata %q carries %d values — ambiguous, refusing",
				MetadataTenantKey, len(vals))
		}

		id := tenant.ID(strings.TrimSpace(vals[0]))
		if !id.Valid() {
			// Length-checked, not merely non-empty: tenant_id is an opaque ULID, and an
			// administrative code or a commune name arriving here means some caller is using
			// a meaningful identifier the merger rules forbid (rule 1, invariant 2).
			return nil, status.Errorf(codes.InvalidArgument,
				"grpcx: metadata %q is not a ULID", MetadataTenantKey)
		}

		return handler(tenant.Into(ctx, id), req)
	}
}
