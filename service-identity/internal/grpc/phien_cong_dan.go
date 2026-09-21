package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/httpx"
)

// The citizen half of this contract — ONE RPC, and it is the citizen channel's ResolveHost.
//
// IT IS IN A FILE OF ITS OWN, NOT BESIDE ResolveStaffPrincipal, and the separation is the same
// one internal/store makes between `phien` and `phien_cong_dan`: two populations at two trust
// levels (rule 4). A staff session is issued to an accountable account and carries a grant set; a
// citizen session is a phone number that passed an OTP and carries NO authority at all. One file
// holding both is one careless edit away from the weaker one being handed the stronger one's
// shape.
//
// # THIS HANDLER RUNS WITH NO COMMUNE IN CONTEXT, BY DESIGN
//
// Everything else on this server calls s.xa(ctx) first. This one must not, and the reason cannot
// be designed away: it is the call that ESTABLISHES the commune of a citizen request (ADR 0022),
// so requiring one would mean knowing the commune in order to find the commune. Nothing here may
// therefore touch core/store.For(ctx) — which calls tenant.MustFrom and panics — and nothing here
// does: the one read it makes is *store.PhienCongDanStore.TraCuuCoLoi, whose unscoped exemption is
// argued at its own definition and whose query returns AT MOST ONE row.
//
// AND IT IS NOT REACHABLE YET. core/grpcx.methodsWithoutTenant does not name
// "/vigov.identity.v1.IdentityService/ResolveCitizenSession", so the server interceptor refuses
// the call with InvalidArgument before this file is entered. That is deliberate, not an oversight:
// ADR 0012 decision 1 makes ADDING A SECOND NAME TO THAT LIST A STOP CONDITION for the user, and
// core/grpcx/grpcx_test.go pins the absence so the answer cannot be given by accident. The
// implementation is written so that the decision, when it is taken, is one line and not a project.

// PhienCongDanDoc is the citizen session registry.
//
// IT IS core/httpx.CitizenSessions' RICHER TWIN, ON PURPOSE. The edge interface has one negative
// answer — unknown, expired and revoked must be indistinguishable — and across THIS boundary that
// collapse is wrong: a caller that reads "identity is down" as "no session" signs every citizen in
// every commune out, through the one channel a commune is judged on (rule 10). So the third
// answer is carried here and nowhere else. *idstore.PhienCongDanStore satisfies both as it is.
type PhienCongDanDoc interface {
	// TraCuuCoLoi returns the session for a bearer token, ok=false when the token is unusable,
	// and a non-nil error ONLY when the registry could not be read.
	TraCuuCoLoi(ctx context.Context, token string) (httpx.CitizenSession, bool, error)
}

// ResolveCitizenSession turns the bearer token a Mini App holds into the three opaque identifiers
// the citizen edge needs: which session, which citizen, which commune.
//
// # IT RESOLVES A SESSION. IT GRANTS NOTHING.
//
// There is no permission lookup in this function and there must never be one. A citizen identity
// is a phone number plus an OTP, which rule 4 calls deliberately weak; what a citizen may see is
// decided by rule 4's isolation at the point of the query — their own records, in one commune —
// never by anything handed back here. The contrast with ResolveStaffPrincipal, which reads the
// grant set, is the design and not an unfinished half.
//
// # NOTHING IS LOGGED ON THE ORDINARY PATHS
//
// Not the token (it substitutes for a whole session, rule 8), not the request message (the
// generated String() prints session_token in full — measured, not assumed), not the citizen id and
// not the commune. An expired session is an ordinary daily event; a line per citizen request would
// be the highest-volume log in the system and would carry the identifiers of every citizen using
// the channel (rule 3). The ONE line this RPC can emit is Server.loi's, on a registry failure, and
// it carries neither.
//
// # The commune is an ANSWER, never an input
//
// There is no commune in the request and there cannot be one: a caller asking "is this session
// valid IN COMMUNE X" is a caller naming its own commune (rule 1, forbidden #2) wearing the
// clothes of a stricter check. An EMPTY tenant_id in the response is a REAL ANSWER — the citizen
// is signed in and has chosen no commune yet (ADR 0005) — and it is passed straight through, never
// defaulted (rule 1, forbidden #1). core/httpx.XaTuPhien answers 401 for it on every business
// route, which is where that state is refused.
func (s *Server) ResolveCitizenSession(ctx context.Context, req *identityv1.ResolveCitizenSessionRequest) (
	*identityv1.ResolveCitizenSessionResponse, error) {

	// NEVER LOG req. See the paragraph above.

	tok := req.GetSessionToken()
	if tok == "" {
		// LOUD, not "no session". The citizen edge only calls when an `Authorization: Bearer`
		// header was present, so an empty token is a wiring fault in the caller. Answering it
		// quietly would cost a round trip on every anonymous request of the citizen channel,
		// forever, with nothing to report the fault.
		return nil, status.Error(codes.InvalidArgument, "session_token trống")
	}

	p, ok, err := s.d.PhienCongDan.TraCuuCoLoi(ctx, tok)
	if err != nil {
		// THE REGISTRY COULD NOT BE READ. Internal, never "no session": the caller turns an error
		// into 503 and leaves the citizen's session alone. Server.loi logs the cause HERE, where
		// the operator is, and returns a message carrying none of it across the boundary.
		return nil, s.loi(ctx, err, "ResolveCitizenSession/phien_cong_dan")
	}
	if !ok {
		// ONE ANSWER FOR ALL OF THEM — unknown token, malformed token, expired session, revoked
		// session, a token the registry cannot attribute to a single session. No reason code and
		// no log line: a response that varies with the reason tells whoever is probing how close
		// they are (rule 4, forbidden #2), and a log line per failed probe is a log line per probe.
		return khongCoPhienCongDan(), nil
	}

	if p.ID == "" || p.CitizenID == "" {
		// A CONTRACT FAULT IN THIS SERVICE, not an unusable token, and it is worth an error rather
		// than a quiet negative. A session with no sid cannot be revoked and a session with no
		// citizen id cannot filter a citizen-path query (rule 4, invariant 3) — so serving it would
		// hand the edge a session that looks valid and isolates nothing. The registry's own scan
		// cannot produce this today; it is checked because the cost of being wrong is a citizen
		// reading another citizen's petition.
		s.d.Log.ErrorContext(ctx, "CẢNH BÁO HỢP ĐỒNG: sổ phiên công dân trả về phiên thiếu định danh",
			// Neither identifier is logged — one of them is the thing that is missing and the other
			// is a citizen identifier (rule 3). Which one is absent is enough to find the row.
			"thieu_sid", p.ID == "", "thieu_cong_dan", p.CitizenID == "")
		return nil, status.Error(codes.Internal, "lỗi nội bộ, vui lòng thử lại")
	}

	return &identityv1.ResolveCitizenSessionResponse{
		Session: &identityv1.CitizenSessionPrincipal{
			SessionId: p.ID,
			CitizenId: p.CitizenID,
			// STRAIGHT THROUGH, INCLUDING EMPTY. See the note on the RPC: "" is the citizen who has
			// not chosen a commune, and substituting anything for it is rule 1, forbidden #1.
			TenantId: string(p.TenantID),
		},
	}, nil
}

// khongCoPhienCongDan is the ONE answer every unusable token receives.
//
// A named function and not an inline literal, for the reason khongCoPrincipal gives: the branches
// that produce it are then provably the same answer, and the moment one grows a field the others
// do not have, the response has started varying with the reason.
func khongCoPhienCongDan() *identityv1.ResolveCitizenSessionResponse {
	return &identityv1.ResolveCitizenSessionResponse{}
}
