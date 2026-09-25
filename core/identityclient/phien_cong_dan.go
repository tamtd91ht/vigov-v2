package identityclient

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
)

// The citizen half of this client — how a service that is NOT identity mounts a citizen edge.
//
// WHY IT HAS TO EXIST. core/httpx.CitizenEdge needs a core/httpx.CitizenSessions, and the only
// implementation is *store.PhienCongDanStore inside service-identity's `internal/`, which rule 2,
// forbidden #1 forbids any other service from importing. Until this file existed, service-petitions
// — which owns the ONE object a citizen creates and then watches (rule 10) — could not build a
// citizen edge at all, and wrote that fact into its own source rather than ship a second session
// reader. A second reader is the thing that must not happen: two implementations of "is this
// session still usable" disagree on the day a session is revoked, and the one that says yes is the
// one that serves.
//
// ⚠ THIS PARAGRAPH USED TO SAY "NOTHING HERE WORKS YET", AND THAT IS NO LONGER TRUE — 2026-09-21.
// "/vigov.identity.v1.IdentityService/ResolveCitizenSession" WAS absent from
// core/grpcx.methodsWithoutTenant, so grpcx.UnaryClientInterceptor refused every call below with
// InvalidArgument before anything left this process. The user was asked — it is a STOP CONDITION
// under ADR 0012 decision 1 — and answered yes: the name is on that list now
// (core/grpcx/grpcx.go:177, with the reasoning at :158). The calls below travel.
//
// THE REASON IT IS RECORDED RATHER THAN DELETED: the decision, not the code, is what a reader
// needs. It also fixes the shape of the NEXT request of this kind — adding a further name there is
// still the user's call, and the answer for this one does not answer any other (grpcx.go:172).
//
// WHAT REMAINS FORBIDDEN, UNCHANGED BY THE EXEMPTION. Putting any commune into the context to
// satisfy the interceptor is rule 1, forbidden #1 wearing a disguise; dialling a second connection
// without the interceptor is deleting the isolation check for every call that connection ever
// makes. Neither was ever the way past the block, and neither becomes acceptable now.

// SoPhienCongDan resolves a citizen bearer token to a session, over gRPC, for a service that does
// not own the registry. It implements core/httpx.CitizenSessions.
//
// A TYPE OF ITS OWN RATHER THAN A METHOD ON Client, AND THE SEPARATION IS LOAD-BEARING. Client's
// staff method returns three outcomes and lets the middleware answer 503 on the third
// (staffauth.Resolver). httpx.CitizenSessions has TWO outcomes by contract, so the third has to be
// collapsed somewhere — and a collapse of that kind must happen in a named place a reviewer can
// find, not inside a method that also serves the staff path. See TraCuu for what the collapse
// costs and what it would take to stop paying it.
type SoPhienCongDan struct {
	c   *Client
	log *slog.Logger
}

// NewSoPhienCongDan wraps a dialled client as the citizen edge's session registry.
//
// It refuses a nil client at construction rather than at the first citizen request: a nil here
// becomes a panic inside the edge middleware, on the one code path that runs before every other.
func NewSoPhienCongDan(c *Client) *SoPhienCongDan {
	if c == nil {
		panic("identityclient: NewSoPhienCongDan cần một *Client đã kết nối")
	}
	log := c.log
	if log == nil {
		log = slog.Default()
	}
	return &SoPhienCongDan{c: c, log: log}
}

// TraCuu implements core/httpx.CitizenSessions.
//
// # THE COLLAPSE MADE HERE, NAMED BECAUSE IT COSTS SOMETHING
//
// This interface has ONE negative answer, and that is right where it was written: at the HTTP edge
// INSIDE identity, unknown / expired / revoked must be indistinguishable, and an outage of the
// session registry was an outage of the same process, self-evident on every other route at the same
// moment. ACROSS THIS HOP THAT IS NO LONGER TRUE. identity can be down while service-petitions is
// perfectly healthy, and this method then answers ok=false — so httpx.XaTuPhien replies 401 and the
// Mini App tells the citizen "Phiên không hợp lệ hoặc đã kết thúc. Vui lòng mở lại ứng dụng." That
// sentence is FALSE during an outage: the session row is untouched and the token works again the
// moment identity returns.
//
// THE STAFF PATH DOES NOT PAY THIS, AND THE DIFFERENCE IS THE INTERFACE, NOT THE ARGUMENT.
// staffauth.Resolver carries an error, and staffauth's middleware turns it into 503 with a comment
// explaining that 401 would send everybody to sign in again through the service that is down. The
// identical argument applies to citizens; the citizen interface simply predates the network hop.
//
// SO THE FIX IS KNOWN AND IS NOT TAKEN HERE: widen httpx.CitizenSessions.TraCuu to return an error,
// carry it from CitizenEdge to XaTuPhien, and give XaTuPhien a 503 branch beside its 401. That
// changes a core edge contract whose "one negative answer" discipline is argued at length in
// core/httpx/citizen.go, and it belongs to whoever owns that decision — it is written down here
// rather than done quietly, and rather than left for somebody to find during an outage.
//
// WHAT IS NOT GIVEN UP: the distinction survives everywhere it can be acted on. The gRPC contract
// keeps it (ResolveCitizenSession: "UNAVAILABLE etc. — the call did not happen. IT IS NOT 'no
// session'"), the store keeps it (PhienCongDanStore.TraCuuCoLoi), and the log line below says which
// of the two happened. Only the return value cannot say it.
//
// NOTHING IS LOGGED ON THE ORDINARY NEGATIVE. An expired citizen session is a daily event, and this
// runs on every citizen request: a line per failure would be the highest-volume log in the system.
func (s *SoPhienCongDan) TraCuu(ctx context.Context, token string) (httpx.CitizenSession, bool) {
	p, ok, err := s.c.TraCuuPhienCongDan(ctx, token)
	if err != nil {
		// WARN AND SAY WHICH FAILURE IT WAS. This is the only trace left of the collapse above, so
		// it names the consequence rather than just the error: an operator seeing 401s across the
		// whole citizen channel needs to reach this line and not a citizen's session.
		//
		// NO TOKEN, NO CITIZEN IDENTIFIER, NO COMMUNE (rule 3, rule 8).
		s.log.WarnContext(ctx, "CẢNH BÁO HẠ TẦNG: không phân giải được phiên công dân — "+
			"công dân sẽ nhận 401 như thể phiên đã hết hạn",
			"ma_loi", status.Code(err).String(), "err", err)
		return httpx.CitizenSession{}, false
	}
	return p, ok
}

var _ httpx.CitizenSessions = (*SoPhienCongDan)(nil)

// TraCuuPhienCongDan asks identity whose citizen session a bearer token is, and in which commune.
//
// THE THREE OUTCOMES ARE KEPT APART HERE, exactly as ResolveStaff keeps them, because this is the
// only place that can see a gRPC status code:
//
//	err != nil        the call did not happen — unreachable, timed out, refused, refused by the
//	                  commune interceptor, broken contract. It is NEVER "no session".
//	ok == false       OK with no session. The token is not usable, and the contract deliberately
//	                  does not say why — unknown, malformed, expired and revoked all answer the
//	                  same, so a probe cannot learn how close it is.
//	ok == true        a usable session. TenantID MAY BE EMPTY and that is an ordinary answer: a
//	                  citizen who is signed in and has not chosen a commune yet (ADR 0005).
//
// EXPORTED SEPARATELY FROM SoPhienCongDan.TraCuu so a caller that CAN act on the third outcome is
// able to. Nothing calls it that way today; it exists so that the day httpx.CitizenSessions is
// widened, the transport does not have to be rewritten to supply what it already knows.
//
// NOTHING HERE LOGS THE TOKEN, AT ANY LEVEL (rule 3, rule 8), and nothing logs the request message:
// the generated String() prints session_token in full, so one `"req", req` would put a working
// citizen session into the log pipeline, from where it cannot be recalled.
func (c *Client) TraCuuPhienCongDan(ctx context.Context, token string) (httpx.CitizenSession, bool, error) {
	if token == "" {
		// The edge only calls when an `Authorization: Bearer` header was present, so an empty token
		// is a wiring fault in the caller. Refused locally: the server answers INVALID_ARGUMENT for
		// exactly this, and a request that can only fail has no business on the network.
		return httpx.CitizenSession{}, false, fmt.Errorf(
			"identityclient: gọi ResolveCitizenSession với token rỗng — bên gọi phải bỏ qua khi không có Bearer")
	}

	// THE SAME DEADLINE AS ResolveStaff, on purpose. This call sits on the path of every citizen
	// request, which is the same class of work as ResolveStaff on every staff request; two numbers
	// would make the observed timeout depend on which dependency was slow.
	ctx, huy := context.WithTimeout(ctx, HanGoi)
	defer huy()

	ra, err := c.cl.ResolveCitizenSession(ctx, &identityv1.ResolveCitizenSessionRequest{
		SessionToken: token,
	})
	if err != nil {
		// NOT LOGGED HERE. The one caller that exists logs it with the consequence attached, and a
		// second line per citizen request during an outage is a second copy of the busiest log in
		// the system. The gRPC code travels in the error so that caller can print it.
		return httpx.CitizenSession{}, false, fmt.Errorf("identityclient: ResolveCitizenSession: %w", err)
	}

	p := ra.GetSession()
	if p == nil {
		// ABSENT MEANS NO SESSION. The ordinary negative — an expired session is a daily event — so
		// it raises nothing and logs nothing.
		return httpx.CitizenSession{}, false, nil
	}

	if p.GetSessionId() == "" {
		// A CONTRACT FAULT, AND AN ERROR RATHER THAN "no session": a session with no sid cannot be
		// revoked (rule 5, invariant 4). Served quietly it would be invisible for as long as the two
		// ends disagree.
		return httpx.CitizenSession{}, false, fmt.Errorf(
			"identityclient: ResolveCitizenSession trả về phiên thiếu sid")
	}

	// AN EMPTY citizen_id IS A REAL ANSWER SINCE ADR 0045, and it is copied through like an empty
	// tenant_id: a bridge session whose phone is not verified yet. It used to be refused here as a
	// contract fault. The wall that refuses it is httpx.XaTuPhien, on every route that reads or
	// writes the citizen's own records — one wall in one place, changed in the same commit as this
	// line and as identity's handler (ADR 0045 §Phiên chưa có số, point 5).

	// TENANT ID COPIED STRAIGHT THROUGH, INCLUDING EMPTY, AND THERE IS NO VALIDATION HERE.
	//
	// "" is a real answer: signed in, no commune chosen (ADR 0005). httpx.XaTuPhien is what refuses
	// it, with 401, on every business route — that is the wall, and it is one wall in one place.
	// Substituting anything for an empty value here is rule 1, forbidden #1, and rejecting the
	// session because of it would break the commune-picker screen, which is called WITH exactly
	// this session.
	return httpx.CitizenSession{
		ID:        p.GetSessionId(),
		CitizenID: p.GetCitizenId(),
		TenantID:  tenant.ID(p.GetTenantId()),
	}, true, nil
}
