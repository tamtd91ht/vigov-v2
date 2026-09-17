// Package platformclient is how a service asks the platform registry "which commune is this".
//
// WHY IT LIVES IN pkg/ AND NOT INSIDE ONE SERVICE: every service edge has to resolve Host ->
// commune before any handler runs (rule 1, invariant 3), so all eight need this client. Buried
// in identity/internal it would be unreachable from the other seven (rule 2,
// forbidden #1), and the second service to need it would write a second one — two clients, two
// timeout values, two ideas of what a failure means, and eventually two answers to the one
// question the whole isolation model rests on.
//
// WHY IT IS NOT IN pkg/tenant, which is where tenant.Directory is declared: this client needs
// pkg/grpcx for the interceptor, and pkg/grpcx imports pkg/tenant. Putting it there would be an
// import cycle. The interface stays in pkg/tenant, the transport lives here.
//
// WHY NOT A DATABASE READ: the registry tables belong to the platform service. A second service
// opening a connection to them is rule 2, forbidden #2 — the read path around the contract that
// turns eight services into a distributed monolith with no symptoms.
package platformclient

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/vihat/vigov/core/grpcx"
	"github.com/vihat/vigov/core/tenant"
	platformv1 "github.com/vihat/vigov/gen/vigov/platform/v1"
)

// HanGoi bounds one Host resolution.
//
// IT EXISTS BECAUSE THIS CALL IS ON THE PATH OF EVERY REQUEST. Without a deadline, a platform
// service that accepts connections but never answers holds every in-flight request of every
// other service open until their own clients give up — one slow dependency becomes eight
// unresponsive services. Three seconds is long for a single indexed lookup behind a cached
// directory, and short enough that a stall is visible rather than fatal.
const HanGoi = 3 * time.Second

// Directory resolves a Host to a commune over gRPC. It implements tenant.Directory.
//
// WRAP IT IN tenant.NewCachedDirectory. On its own this makes a network call per request; the
// cache is what makes that acceptable at 200+ communes (ADR 0004, decision 5).
type Directory struct {
	cl   platformv1.PlatformServiceClient
	conn *grpc.ClientConn // nil when the client was injected directly, e.g. in a test
	log  *slog.Logger
}

// Dial opens the connection to the platform service.
//
// grpc.NewClient does not connect eagerly, and that is wanted: a service must still start when
// the platform is momentarily down. What it must NOT do is pretend it resolved a commune, and
// ByHost below is where that is held.
//
// INSECURE CREDENTIALS, STATED RATHER THAN HIDDEN: there is no mTLS and no caller identity on
// this hop yet — the gRPC port is protected by network isolation alone, which is the named gap
// written out in platform/cmd/server/main.go. This client must be pointed at a
// cluster-internal address only. When that gap is closed, the credentials are changed HERE, in
// one place, for every service.
func Dial(addr string, log *slog.Logger) (*Directory, error) {
	if addr == "" {
		// Fail closed and by name. A client with no address resolves no Host, which turns into
		// 404 for every commune — a failure that looks like "the domain is misconfigured" and
		// sends somebody to DNS for an afternoon.
		return nil, fmt.Errorf("platformclient: thiếu địa chỉ PLATFORM_GRPC_ADDR")
	}
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// pkg/grpcx, never a hand-written copy: it is the one place that decides how the commune
		// travels on the wire, and which RPCs may travel without one. ResolveHost is on that
		// exemption list — it is the call that ESTABLISHES the commune, so it cannot carry one.
		grpc.WithUnaryInterceptor(grpcx.UnaryClientInterceptor()),
	)
	if err != nil {
		return nil, fmt.Errorf("platformclient: mở kết nối tới %s: %w", addr, err)
	}
	return NewDirectory(platformv1.NewPlatformServiceClient(conn), log).voi(conn), nil
}

// NewDirectory wraps an already-built client. Exported so the failure behaviour below can be
// tested without a running platform service — a test that needs infrastructure is a test that
// stops being run.
func NewDirectory(cl platformv1.PlatformServiceClient, log *slog.Logger) *Directory {
	if log == nil {
		log = slog.Default()
	}
	return &Directory{cl: cl, log: log}
}

func (d *Directory) voi(conn *grpc.ClientConn) *Directory {
	d.conn = conn
	return d
}

// Close releases the connection. Safe on an injected client.
func (d *Directory) Close() error {
	if d.conn == nil {
		return nil
	}
	return d.conn.Close()
}

// ByHost implements tenant.Directory.
//
// A HOST THAT DOES NOT RESOLVE RETURNS false, AND THE EDGE TURNS THAT INTO 404 — never a
// fallback commune (rule 1, invariant 3; ADR 0012, decision 4). That is the right answer even
// when the reason is that the platform service is unreachable: serving a request whose commune
// is unknown is the one outcome that is never acceptable.
//
// BUT A TRANSPORT FAILURE MUST NOT BE SILENT, and that is what the Warn below is for. ByHost
// returns a bool, so an outage and an unknown Host collapse into the same answer: with the
// platform down, EVERY Host stops resolving and every member of staff sees a 404 — their own
// commune appearing not to exist. Without a loud line at the moment it happens, an
// infrastructure incident looks exactly like a batch of ordinary 404s, and the first real
// diagnosis comes from a commune telephoning to say the system is gone.
func (d *Directory) ByHost(ctx context.Context, host string) (tenant.Tenant, bool) {
	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()

	ra, err := d.cl.ResolveHost(ctx, &platformv1.ResolveHostRequest{Host: host})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound, codes.InvalidArgument:
			// A routine negative: no commune holds this Host, or the Host is not a host at all.
			// This is an ordinary daily event on any public address and must not raise an alarm,
			// or the alarm stops meaning anything.
			d.log.DebugContext(ctx, "không có xã nào ứng với host", "host", host)
		default:
			// Everything else is infrastructure: unreachable, timed out, refused, broken
			// contract. Warn and not Error because the service itself is still healthy and
			// answering — what has failed is the dependency every request needs.
			d.log.WarnContext(ctx, "CẢNH BÁO HẠ TẦNG: không phân giải được xã vì không gọi được dịch vụ nền tảng "+
				"— MỌI Host sẽ trả 404 cho tới khi khôi phục",
				"host", host, "ma_loi", status.Code(err).String(), "err", err)
		}
		return tenant.Tenant{}, false
	}

	t := ra.GetTenant()
	if t == nil {
		// The contract says a successful ResolveHost carries a commune. An empty one means the
		// two ends disagree about the contract, which is worth as much noise as an outage.
		d.log.WarnContext(ctx, "CẢNH BÁO HỢP ĐỒNG: ResolveHost trả về thành công nhưng không có xã",
			"host", host)
		return tenant.Tenant{}, false
	}

	id := tenant.ID(t.GetId())
	if !id.Valid() {
		// Length-checked, not merely non-empty: tenant_id is an opaque ULID. Anything else
		// arriving here means some end is using an administrative code or a commune name, which
		// rule 1, invariant 2 forbids — and accepting it would put a meaningful identifier onto
		// archival records that a merger then cannot rewrite.
		d.log.WarnContext(ctx, "CẢNH BÁO HỢP ĐỒNG: ResolveHost trả về mã xã không phải ULID",
			"host", host)
		return tenant.Tenant{}, false
	}

	// Active is passed through rather than folded into the bool: httpx.TenantMiddleware refuses a
	// deactivated commune itself, and a merged commune still has to be describable (rule 7).
	return tenant.Tenant{
		ID:     id,
		Host:   t.GetHost(),
		Name:   t.GetDisplayName(),
		Active: t.GetActive(),
	}, true
}
