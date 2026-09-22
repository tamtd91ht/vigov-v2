// Package identityclient is how a service asks identity "who is holding this session".
//
// WHY IT LIVES IN core/ AND NOT INSIDE ONE SERVICE: four service edges have to rebuild a staff
// principal before any guard runs, and the only thing that can answer is identity — whose session
// registry sits in its own `internal/`, unreachable by rule 2, forbidden #1. Buried in one of the
// four, the second service to need it would write a second one: two clients, two timeout values,
// two readings of what a failure means, and eventually two answers to the question every guarded
// route in the system rests on.
//
// WHY IT IS NOT IN core/staffauth, where Resolver is declared: same split as platformclient and
// tenant.Directory. The interface belongs beside the middleware that consumes it; the transport
// lives here, so a test of the middleware needs no gRPC and a test of the transport needs no HTTP.
//
// WHY NOT A DATABASE READ: the session registry and the grants belong to identity. A second
// service opening a connection to them is rule 2, forbidden #2 — the read path around the contract
// that turns eight services into a distributed monolith with no symptoms. It would also be a
// second implementation of "is this session still valid", and the two would disagree on the day a
// session is revoked.
package identityclient

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/vihat/vigov/core/authz"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/grpcx"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/staffauth"
)

// HanGoi bounds one principal resolution.
//
// IT EXISTS BECAUSE THIS CALL IS ON THE PATH OF EVERY STAFF REQUEST. Without a deadline, an
// identity service that accepts connections but never answers holds every in-flight request of
// four other services open until their own clients give up — one slow dependency becomes five
// unresponsive services. Three seconds is long for a session read plus a grant read, and short
// enough that a stall shows up as a 503 rather than as a hang. The same number as
// platformclient.HanGoi, deliberately: two different deadlines on two hops of the same request
// would make the observed timeout depend on which dependency was slow.
const HanGoi = 3 * time.Second

// Client resolves a session credential into a staff principal over gRPC. It implements
// staffauth.Resolver.
//
// THERE IS NO CACHE AND THERE MUST NOT BE ONE. The argument is written out in full on
// ResolveStaffPrincipal in proto/vigov/identity/v1/identity.proto and on
// staffauth.StaffPrincipal.PermissionKeys: identity runs several replicas, ADR 0010 fixes the
// data infrastructure at PostgreSQL only so there is no shared cache, and a revocation served by
// one replica has no channel on which to reach a copy held by another. A TTL here would keep a
// revoked session, a locked account or a withdrawn role working for the length of that TTL, and
// nothing would turn red.
type Client struct {
	cl   identityv1.IdentityServiceClient
	conn *grpc.ClientConn // nil when the client was injected directly, e.g. in a test
	log  *slog.Logger
}

// Dial opens the connection to the identity service.
//
// grpc.NewClient does not connect eagerly, and that is wanted: a service must still start when
// identity is momentarily down. What it must NOT do is pretend a session resolved, and
// ResolveStaff below is where that is held.
//
// INSECURE CREDENTIALS, STATED RATHER THAN HIDDEN, and the stakes are higher on this hop than on
// the platform one: what travels here is a WORKING SESSION TOKEN. The transport is not encrypted
// and there is no per-service identity. What guards it is two layers, and neither is TLS — the
// shared caller key attached below, and the gRPC port being confined to the cluster's internal
// network (ADR 0025). So this client must be pointed at a cluster-internal address ONLY; an
// address that leaves the cluster puts every staff session on the wire in the clear. When
// per-service identity arrives — mTLS or a mesh — the credentials change HERE, in one place.
//
// khoa IS THE DEPLOYMENT'S ONE CALLER KEY (config.GRPCCallerKey). An empty one panics inside
// grpcx.UnaryClientCallerAuth, at construction: this end would otherwise send every call without
// a key and have every one refused, and "503 on every staff request" is a much slower read than a
// startup message naming the variable.
func Dial(addr string, khoa secret.Secret, log *slog.Logger) (*Client, error) {
	if addr == "" {
		// Fail closed and BY NAME, the same shape as platformclient.Dial. A client with no address
		// resolves no session, which turns into 503 for every member of staff of every commune —
		// a failure that looks like "identity is down" and sends somebody to inspect a service
		// that is running perfectly well.
		return nil, fmt.Errorf("identityclient: thiếu địa chỉ IDENTITY_GRPC_ADDR")
	}
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// core/grpcx, never a hand-written copy: it is the one place that decides how the commune
		// travels on the wire, and which RPCs may travel without one.
		//
		// ResolveStaffPrincipal IS NOT ON THE EXEMPTION LIST and must never be added to it. The
		// commune in that metadata is what the server compares against the commune inside the
		// credential — the comparison rule 1, invariant 8 asks for, made once for all four
		// services. Exempt it and the comparison has nothing to compare.
		//
		// CHAINED, and the caller key goes FIRST, mirroring the server: "who is calling" is
		// answered before "which commune", so a caller with no key never reaches the commune logic.
		grpc.WithChainUnaryInterceptor(
			grpcx.UnaryClientCallerAuth(khoa),
			grpcx.UnaryClientInterceptor(),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("identityclient: mở kết nối tới %s: %w", addr, err)
	}
	return New(identityv1.NewIdentityServiceClient(conn), log).voi(conn), nil
}

// New wraps an already-built client. Exported so the failure behaviour below can be tested against
// a fake server — a test that needs a running identity service is a test that stops being run.
func New(cl identityv1.IdentityServiceClient, log *slog.Logger) *Client {
	if log == nil {
		log = slog.Default()
	}
	return &Client{cl: cl, log: log}
}

func (c *Client) voi(conn *grpc.ClientConn) *Client {
	c.conn = conn
	return c
}

// Close releases the connection. Safe on an injected client.
func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// ResolveStaff implements staffauth.Resolver.
//
// THE THREE OUTCOMES ARE KEPT APART HERE AND NOWHERE ELSE, because this is the only place that can
// see a gRPC status code:
//
//	err != nil        the call did not happen — unreachable, timed out, refused, deadline,
//	                  broken contract. The middleware turns it into 503. It is NEVER folded into
//	                  "no principal": a caller that reads an outage as anonymity tells every member
//	                  of staff to sign in again, through the service that is down.
//	ok == false       OK with no principal. The credential is not usable, and the contract
//	                  deliberately does not say why — unknown, malformed, expired, revoked, locked,
//	                  deleted, or issued for another commune all answer the same, so a probe cannot
//	                  learn how close it is.
//	ok == true        a usable credential. The key set MAY BE EMPTY and that is an ordinary answer:
//	                  a live session held by somebody who currently holds no permission.
//
// NOTHING HERE LOGS THE TOKEN, AT ANY LEVEL (rule 3, rule 8). Not the value, not the request
// message — the generated String() prints session_token in full, so a single `"req", req` would
// put a working credential into the log pipeline, from where it cannot be recalled. The fields
// below are chosen one at a time for that reason.
func (c *Client) ResolveStaff(ctx context.Context, sessionToken, clientIP string) (staffauth.StaffPrincipal, bool, error) {
	if sessionToken == "" {
		// The middleware never calls without a credential — no cookie means no call. Reaching here
		// is a wiring fault in some other caller, and it is refused locally rather than sent: the
		// server answers INVALID_ARGUMENT for exactly this, and a request that can only fail has no
		// business on the network.
		return staffauth.StaffPrincipal{}, false, fmt.Errorf(
			"identityclient: gọi ResolveStaffPrincipal với phiếu phiên rỗng — bên gọi phải bỏ qua khi không có cookie")
	}

	ctx, huy := context.WithTimeout(ctx, HanGoi)
	defer huy()

	ra, err := c.cl.ResolveStaffPrincipal(ctx, &identityv1.ResolveStaffPrincipalRequest{
		SessionToken: sessionToken,
		// A CLAIM, and it decides nothing. It exists for one line of output at the other end — the
		// alert raised when a credential's commune disagrees with the commune in metadata — because
		// across this hop the server can otherwise see only a pod address inside the cluster.
		ClientIp: clientIP,
	})
	if err != nil {
		// Warn, not Error: this service is still healthy and answering. What has failed is the
		// dependency every staff request needs, and the middleware says so again with the commune
		// and the path. The gRPC code is here because it is how an operator tells a genuine outage
		// (UNAVAILABLE, DeadlineExceeded) from a misconfiguration (Unauthenticated: the caller key;
		// InvalidArgument: the contract) — all three answer 503 to the client on purpose.
		c.log.WarnContext(ctx, "CẢNH BÁO HẠ TẦNG: không gọi được ResolveStaffPrincipal",
			"ma_loi", status.Code(err).String(), "err", err)
		return staffauth.StaffPrincipal{}, false, fmt.Errorf("identityclient: ResolveStaffPrincipal: %w", err)
	}

	p := ra.GetPrincipal()
	if p == nil {
		// ABSENT MEANS NO PRINCIPAL. This is the ordinary negative — a stale cookie is a daily
		// event — so it raises nothing and logs nothing.
		return staffauth.StaffPrincipal{}, false, nil
	}

	if p.GetStaffId() == "" {
		// The contract says a principal that is present carries an id: the message exists so the
		// two facts cannot be half-set. An empty id means the two ends disagree about the contract,
		// and it is worth as much noise as an outage — a principal with no id would make every
		// permission check answer false and every guarded route 403, with nothing to point at.
		//
		// AN ERROR, NOT "no principal": a contract fault must not be served as an ordinary expired
		// session, or it is invisible for as long as the two ends disagree.
		c.log.WarnContext(ctx, "CẢNH BÁO HỢP ĐỒNG: ResolveStaffPrincipal trả về chủ thể không có staff_id")
		return staffauth.StaffPrincipal{}, false, fmt.Errorf(
			"identityclient: chủ thể trả về không có staff_id")
	}

	// Copied into our own slice rather than aliasing the response's: the message is reachable from
	// the caller only through what is returned here, and a shared backing array is a set two
	// requests could come to share.
	//
	// EMPTY STAYS EMPTY, and is not turned into nil-and-therefore-suspicious. An account whose role
	// was withdrawn holds no key, and the middleware must build a principal for them all the same.
	khoa := make([]authz.Perm, 0, len(p.GetPermissionKeys()))
	for _, k := range p.GetPermissionKeys() {
		if k == "" {
			// A blank key matches no route declaration and can only be noise on the wire. Dropping
			// it is safe in the one direction that matters: it can never widen access.
			continue
		}
		khoa = append(khoa, authz.Perm(k))
	}

	// `ma` IS CARRIED BUT NOT REQUIRED, and the asymmetry with staff_id above is deliberate.
	// staff_id absent is a contract fault that breaks every permission check, so it errors. `ma`
	// absent is what an OLD identity answers during a rolling deploy (the field was added
	// 2026-09-22, optional by rule 2, forbidden #4), and erroring here would turn that into a
	// total outage of this service instead of a refusal of its audited writes alone. It is also
	// NEVER substituted with staff_id: see staffauth.StaffPrincipal.Ma.
	return staffauth.StaffPrincipal{
		StaffID:        p.GetStaffId(),
		Ma:             p.GetMa(),
		PermissionKeys: khoa,
	}, true, nil
}

var _ staffauth.Resolver = (*Client)(nil)

// TienGioLamViec asks identity when a number of WORKING hours has elapsed from an instant, against
// ONE commune's calendar. It is the only correct way to produce an administrative deadline.
//
// WHY A SECOND METHOD HERE AND NOT A SECOND PACKAGE: the argument at the top of this file applies
// unchanged and the count makes it sharper. `petitions` needs two deadlines at intake (ADR 0028),
// `documents` needs one for `Văn bản đến` (ADR 0007). Left to each
// service, that is two clients and — far worse — two readings of what a failure means, on a
// value that is a COMMITMENT MADE TO A CITIZEN by a public authority.
//
// THE CALLER MUST NEVER FALL BACK TO A LOCAL DURATION when this returns an error. Adding hours in
// the caller walks straight through nights, weekends, `ngay_nghi_le` and `ngay_lam_bu` while
// looking like it respected the unit — the two answers differ only on the days a citizen notices.
// Rule 10, forbidden #2 says so and `hooks/citizen_commitment_guard.py` blocks the shape outside
// `service-identity`. An error here means FAIL THE INTAKE, not "no deadline".
//
// KEYED BY THE AMOUNT, NEVER BY INDEX. The contract collapses duplicates, so `len` of the answer
// can be smaller than `len(gio)` for a request that fully succeeded. A caller reading
// `items[0]` and `items[1]` after asking for `{8, 8}` would silently take the acknowledge
// deadline as the resolve deadline — a shift no test of the happy path would catch.
//
// THERE IS NO PARTIAL SUCCESS. An amount asked for and not answered is a contract fault, not a
// missing optional: it would otherwise reach the caller as a zero `time.Time`, i.e. a deadline in
// year 1, i.e. a petition overdue the moment it is received.
func (c *Client) TienGioLamViec(ctx context.Context, tuLuc time.Time, gio []uint32) (map[uint32]time.Time, error) {
	if tuLuc.IsZero() {
		// REFUSED LOCALLY, and the message names the cause rather than the symptom. Sent, a zero
		// instant comes back as a horizon failure about the year 1 — an operator reading that
		// would go looking at the commune's calendar for a fault that is in the caller's wiring.
		return nil, fmt.Errorf("identityclient: TienGioLamViec với mốc khởi động rỗng — bên gọi chưa đặt thời điểm tiếp nhận")
	}
	if len(gio) == 0 {
		// A request that asks nothing can only come back empty, and has no business on the network.
		return nil, fmt.Errorf("identityclient: TienGioLamViec không có số giờ nào để tính")
	}

	// THE SAME DEADLINE AS ResolveStaff, on purpose. This is a read of configuration — at most two
	// years of calendar rows, all indexed — not a different class of work, and two numbers would
	// make the observed timeout depend on which call was slow. It is also NOT on every request:
	// it runs once, at intake.
	ctx, huy := context.WithTimeout(ctx, HanGoi)
	defer huy()

	ra, err := c.cl.AdvanceWorkingHours(ctx, &identityv1.AdvanceWorkingHoursRequest{
		CountFrom:    timestamppb.New(tuLuc),
		WorkingHours: gio,
	})
	if err != nil {
		// FAILED_PRECONDITION is the ordinary answer today and it is the contract working, not a
		// bug to route around: no commune has a working calendar yet, because migration 0006 seeds
		// nothing and onboarding does not exist. Logged at Warn with the code so an operator can
		// tell "nobody filled in the commune's hours" from "identity is down" — both must fail the
		// intake, but only one of them is fixed on a configuration screen.
		c.log.WarnContext(ctx, "CẢNH BÁO: không tính được hạn theo giờ làm việc",
			"ma_loi", status.Code(err).String(), "err", err)
		return nil, fmt.Errorf("identityclient: AdvanceWorkingHours: %w", err)
	}

	moc := make(map[uint32]time.Time, len(gio))
	for _, it := range ra.GetItems() {
		t := it.GetReachedAt()
		if t == nil {
			return nil, fmt.Errorf(
				"identityclient: AdvanceWorkingHours trả về mốc rỗng cho %d giờ làm việc", it.GetWorkingHours())
		}
		moc[it.GetWorkingHours()] = t.AsTime()
	}

	for _, g := range gio {
		if _, co := moc[g]; !co {
			// A contract fault, and it is worth as much noise as an outage: the two ends disagree
			// about what a complete answer is, and every deadline produced from here on would be
			// short by whichever amount went missing.
			c.log.WarnContext(ctx, "CẢNH BÁO HỢP ĐỒNG: AdvanceWorkingHours thiếu một mốc đã hỏi",
				"so_gio", g)
			return nil, fmt.Errorf(
				"identityclient: AdvanceWorkingHours không trả mốc cho %d giờ làm việc — không có thành công một phần", g)
		}
	}

	return moc, nil
}
