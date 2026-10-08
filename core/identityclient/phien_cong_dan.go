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
// A TYPE OF ITS OWN RATHER THAN A METHOD ON Client: it is the one place the citizen edge's contract
// (httpx.CitizenSessions) meets this transport, and it is where an outage is said out loud.
//
// SINCE 01/10/2026 httpx.CitizenSessions CARRIES THE THIRD OUTCOME, so nothing is collapsed here any
// more: an outage reaches CitizenEdge as an error and the citizen gets 503, exactly as staff do
// through staffauth.Resolver. That outage now includes "identity could not check the session's
// commune with platform" — ResolveCitizenSession answers UNAVAILABLE for it.
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
// THE THREE OUTCOMES PASS THROUGH (01/10/2026). Until then this method collapsed an outage into
// ok=false, so httpx.XaTuPhien replied 401 and the Mini App told the citizen "Phiên không hợp lệ hoặc
// đã kết thúc" while the session row was untouched — the known fix recorded here was to widen the
// interface, and it was taken when the deactivated-commune check made an outage of platform a
// per-request dependency too: collapsed, "could not check the commune" would have read as "no
// session" instead of as a refusal. CitizenEdge now answers the error with 503, as staffauth does.
//
// NOTHING IS LOGGED ON THE ORDINARY NEGATIVE. An expired citizen session is a daily event, and this
// runs on every citizen request: a line per failure would be the highest-volume log in the system.
func (s *SoPhienCongDan) TraCuu(ctx context.Context, token string) (httpx.CitizenSession, bool, error) { // vi-name-ok: implements the existing httpx.CitizenSessions method
	p, ok, err := s.c.TraCuuPhienCongDan(ctx, token)
	if err != nil {
		// WARN AND SAY WHICH FAILURE IT WAS: the edge has no logger, so this is the line an operator
		// seeing 503 across the citizen channel needs to reach. UNAVAILABLE here includes identity
		// being unable to check the session's commune with platform.
		//
		// NO TOKEN, NO CITIZEN IDENTIFIER, NO COMMUNE (rule 3, rule 8).
		s.log.WarnContext(ctx, "CẢNH BÁO HẠ TẦNG: không phân giải được phiên công dân — "+
			"công dân sẽ nhận 503 cho tới khi khôi phục",
			"ma_loi", status.Code(err).String(), "err", err)
		return httpx.CitizenSession{}, false, err
	}
	return p, ok, nil
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
// EXPORTED SEPARATELY FROM SoPhienCongDan.TraCuu so a caller can use the transport without the
// edge's logging; SoPhienCongDan.TraCuu is the one caller today.
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
	// ZALO ACCOUNT COPIED STRAIGHT THROUGH, INCLUDING EMPTY: "" is a session that did not come
	// through the Mini App bridge — and, until identity's handler fills the field, every session.
	// Refusing it here would break the view-only screens of every unverified session. Whoever needs
	// an owner refuses when both it and CitizenID are empty; nothing here fills one from the other.
	return httpx.CitizenSession{
		ID:            p.GetSessionId(),
		CitizenID:     p.GetCitizenId(),
		TenantID:      tenant.ID(p.GetTenantId()),
		ZaloAccountID: p.GetZaloAccountId(),
	}, true, nil
}
