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
// empty context and pkg/store refuses or, worse, some future code defaults. The same sentence
// applies to caller authentication below: a stream interceptor is a SECOND place to forget it.
//
// # Caller authentication — and what it deliberately does not buy
//
// Every RPC arriving here must carry MetadataCallerKey holding the one value every service on
// the deployment shares (UnaryServerCallerAuth, caller_auth.go). THE DEFENCE IS TWO LAYERS,
// and a reader who sees only the first badly overestimates it:
//
//  1. this shared key, on EVERY call — including the RPCs exempt from carrying a commune
//  2. the gRPC port confined to the cluster's internal network, never published
//
// Layer 2 is not a footnote. The key is what stops a process that has reached the port; the
// network confinement is what stops a process reaching it at all. Whoever removes either one
// is not removing "one of several" measures — they are removing half of the whole protection.
//
// THREE THINGS THIS DOES NOT BUY. Written down because the next reader will otherwise assume
// them, and each is expensive to assume wrongly:
//
//	IT DOES NOT SAY WHICH SERVICE CALLED. One shared key proves the caller is inside the
//	deployment and nothing more. So an audit entry cannot attribute an inter-service call to a
//	caller — rule 6, invariant 2 wants a "who", and this boundary cannot supply one. And
//	MetadataTenantKey remains a CLAIM MADE BY THE CALLER rather than evidence: it is worth
//	exactly as much as the trust placed in every holder of the key.
//
//	IT DOES NOT BOUND THE BLAST RADIUS INSIDE THE CLUSTER. Anything holding the key may call
//	anything — every RPC of every service, not only the ones it has business with. What bounds
//	the damage is layer 2, and nothing else. This is a trade accepted knowingly (ADR 0025), for
//	a stated reason: maintenance. One key that nobody has to keep working beats per-service
//	identities that rot unnoticed. It is NOT an oversight for the next person to improvise away.
//
//	IT DOES NOT ROTATE GRACEFULLY. There is exactly ONE key, so changing it means changing it
//	everywhere AT ONCE. Every service is a holder, and a rolling restart with half the fleet on
//	the new value is a fleet answering Unauthenticated to half its own calls. Rotation is a
//	coordinated deployment, not a per-service action (rule 8, invariant 6) — whoever schedules
//	one needs to know that BEFORE scheduling it, not halfway through.
//
// Per-service identity (mTLS or a service mesh) is what removes all three, and it remains the
// answer ADR 0012, decision 3 names for a real deployment. ADR 0025 records what was built
// here and what it leaves open; the first item still open is attribution.
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
// WHY THE "x-tenant" PREFIX IS LOAD-BEARING, NOT COSMETIC: core/httpx.StripTenantHeaders
// (core/httpx/edge.go:56) deletes every inbound header whose name begins with "x-tenant",
// because a client naming its own commune is a client granting itself access. gRPC carries
// metadata as HTTP/2 headers, so a key inside that prefix is already inside the sweep that
// exists to stop exactly this. Renaming it to something "tidier" moves the key OUT of that
// sweep, and the protection disappears without a single test turning red.
//
// Lower-case because gRPC lower-cases every metadata key. A constant in any other case
// compares unequal and silently never matches.
const MetadataTenantKey = "x-tenant-id"

// MetadataCallerKey is the ONE name the inter-service caller key travels under.
//
// It sits beside MetadataTenantKey because the two are the complete set of things this
// boundary carries out of band, and they answer DIFFERENT questions — "which commune is this
// request for" and "is the caller inside the deployment at all". Neither implies the other,
// and the interceptors that read them are deliberately separate (caller_auth.go).
//
// ONE NAME, ONE VALUE, FOR THE WHOLE SYSTEM — not one key per service, and no key id for
// rotation. That is a choice of maintenance over precision: these are local calls between
// services inside one cluster, and a scheme with more moving parts is a scheme nobody keeps
// working. What the choice costs is written out in the package doc; read it before reasoning
// about this key as if it were the only thing standing between this port and the internet.
//
// Lower-case for the same protocol reason as MetadataTenantKey: gRPC lower-cases every
// metadata key, so a constant in any other case never matches and every call is refused.
//
// DELIBERATELY OUTSIDE THE "x-tenant" PREFIX. That prefix is swept by
// httpx.StripTenantHeaders, and the sweep exists to answer a different question — a client
// naming its own commune. Sharing the prefix would invite a future edit aimed at one key to
// land on the other, and these two must be able to change independently.
const MetadataCallerKey = "x-vigov-caller-key"

// MethodResolveHost is the one RPC that answers "which commune", and therefore the one that
// cannot itself carry a commune. Declared as a constant so the exemption list below is a list
// of names a reviewer can read, not a string literal buried in a condition.
const MethodResolveHost = "/vigov.platform.v1.PlatformService/ResolveHost"

// MethodResolveCitizenSession is the citizen channel's counterpart to MethodResolveHost: the RPC
// that answers "which commune" when there is no domain to answer it. Same reason for a constant.
const MethodResolveCitizenSession = "/vigov.identity.v1.IdentityService/ResolveCitizenSession"

// MethodOpenCitizenSession is the citizen-session bridge (ADR 0045): the RPC that DECIDES the
// commune of a new citizen session, so it cannot carry one. Served ONLY on identity's separate
// bridge listener, behind the bridge key — never on the inter-service port.
const MethodOpenCitizenSession = "/vigov.identity.v1.CitizenSessionBridgeService/OpenCitizenSession"

// MethodResolveMiniApp answers which commune a dedicated Mini App is bound to (ADR 0044, 0045).
// ONE app per call, never a list — the shape of ResolveHost, not of ListTenants.
const MethodResolveMiniApp = "/vigov.platform.v1.PlatformService/ResolveMiniApp"

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
//
// DELIBERATELY ABSENT: ListTenants and ResolveTenantSuccession. Both belong here eventually — the
// citizen channel has no domain, so both are asked before any commune is known, exactly like
// ResolveHost (ADR 0005, ADR 0022). They are not here YET, and the order matters more than the
// destination:
//
//	ResolveHost answers about ONE commune the caller already names. ListTenants answers about
//	ALL of them, in a few calls, and the ULIDs it returns are the prefix of every cache key,
//	queue, realtime room and file path in the system (rule 1, invariant 7). service-identity
//	deliberately does not return that ULID on its public route for the same reason.
//
// THE PRECONDITION NAMED HERE HAS NOW BEEN BUILT — caller authentication exists on this
// boundary (caller_auth.go, ADR 0025), so the clause this paragraph used to carry, "the gRPC
// port has no caller authentication at all today", is no longer true. THE CONCLUSION IS
// UNCHANGED, and the distinction matters: one condition being met does not lift a stop
// condition that was never conditional on it. Adding a name to this list is a STOP CONDITION
// in its own right (ADR 0012, decision 1) — ask the user, do not decide it while writing code.
//
// What did change is the size of the exposure, from "any process that can open a TCP
// connection" to "any holder of the one shared key", which is every service in the cluster.
// That is smaller, not small: the package doc says exactly why one shared key is a weaker
// statement than it looks. Until the question is asked and answered, the interceptor refuses
// these two with InvalidArgument. grpcx_test.go pins their absence so that editing this
// comment is not enough to undo the decision.
//
// ResolveCitizenSession WAS asked, and answered on 2026-09-21: the user said yes. It is here
// because it passes the ADR's own test — "can this RPC know the commune at call time?" — with a
// no that is structural rather than inconvenient. It is the call that ESTABLISHES the commune of
// a citizen request (ADR 0022): the Mini App has no domain, so there is nothing to derive one
// from until this answers. Requiring a commune would mean knowing the commune in order to find
// it, the identical shape that exempts ResolveHost.
//
// WHY THIS DOES NOT WIDEN THE HOLE, which is the question a reviewer should ask next: the
// contract gives it nothing to widen. Its request carries ONLY `session_token` — there is no
// commune field in it, so a caller cannot ask "is this session valid in commune X" and have the
// exemption launder a claim into an answer (rule 1, forbidden #2). The commune comes back in the
// RESPONSE, derived from the session registry alone. And the reply carries no permission key and
// no personal data: it resolves a session, it grants nothing (rule 4).
//
// OpenCitizenSession AND ResolveMiniApp WERE asked, and answered on 2026-09-25: the owner said
// yes (ADR 0045, answer to CÒN MỞ #1). Both pass the same test with the same structural no: the
// first DECIDES the commune of a citizen session, the second ANSWERS which commune a dedicated app
// belongs to — neither can know it at call time. What keeps them from widening the hole:
//
//	OpenCitizenSession  its request has NO commune field declared for the caller (tenant_hint is
//	                    a QR parameter that only counts with the citizen's explicit confirmation,
//	                    in the main app); the commune comes back as an ANSWER. And it is served on
//	                    identity's bridge listener only — the inter-service port does not register
//	                    CitizenSessionBridgeService at all.
//	ResolveMiniApp      one app per call, metadata only (ADR 0003), no listing — the asymmetry that
//	                    keeps ListTenants off this list does not apply to it.
//
// ListTenants and ResolveTenantSuccession STAY ABSENT. One question being answered does not
// answer the others — theirs is a different question with a different exposure, spelled out
// above, and it has not been put to the user.
var methodsWithoutTenant = map[string]struct{}{
	MethodResolveHost:           {},
	MethodResolveCitizenSession: {},
	MethodOpenCitizenSession:    {},
	MethodResolveMiniApp:        {},
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
