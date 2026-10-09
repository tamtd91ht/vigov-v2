package grpc

import (
	"context"
	"errors"
	"log/slog"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
)

// CitizenEdgeSessions is identity's OWN citizen HTTP edge's session registry (core/httpx.CitizenSessions)
// — for GET /api/v1/my-residential-units, the first citizen route of this service since 2026-09-27.
//
// IT CALLS ResolveCitizenSession IN-PROCESS, AND THAT IS THE WHOLE POINT. Every other service's citizen
// edge reaches a session through that RPC (core/identityclient), and the RPC is where the
// deactivated-commune check lives, so one check covers every edge. A second reader built here from the
// store would be a second answer to "is this session still usable" — the two disagree on the day a
// commune is deactivated or a session revoked, and the one that says yes is the one that serves
// (core/identityclient/phien_cong_dan.go). Calling the method is one implementation, not two.
//
// The method is called directly, not over the network: no caller key and no commune interceptor are
// involved, and neither is needed — the RPC is on core/grpcx.methodsWithoutTenant precisely because it
// RESOLVES the commune, and the caller is this process.
//
// THE RESPONSE IS MAPPED EXACTLY AS core/identityclient.TraCuuPhienCongDan MAPS IT: absent = no session;
// a session without a sid = an error (it could not be revoked); everything else straight through,
// empty tenant and empty citizen included — httpx.XaTuPhien* refuses those, in one place.
type CitizenEdgeSessions struct {
	s *Server
}

// NewCitizenEdgeSessions builds the registry over the SAME two collaborators the RPC is given in
// cmd/server: the citizen session store and the commune lookup. A nil one is refused here, at startup,
// not as a nil dereference on the first citizen request.
func NewCitizenEdgeSessions(reg PhienCongDanDoc, communes tenant.CommuneLookup, log *slog.Logger) *CitizenEdgeSessions {
	if reg == nil || communes == nil {
		panic("identity/grpc: NewCitizenEdgeSessions cần sổ phiên công dân và sổ xã — rìa công dân không dựng được")
	}
	if log == nil {
		log = slog.Default()
	}
	// Only the three fields ResolveCitizenSession reads. NewServer is NOT used: it demands every
	// collaborator of every RPC, and this value serves exactly one method.
	return &CitizenEdgeSessions{s: &Server{d: Deps{PhienCongDan: reg, Communes: communes, Log: log}}}
}

// TraCuu implements core/httpx.CitizenSessions.
//
// NOTHING IS LOGGED HERE: the RPC already logs a registry failure and an unreachable registry (without
// token, citizen or commune — rule 3), and a second line per citizen request during an outage would be
// a second copy of the busiest log in the system.
func (c *CitizenEdgeSessions) TraCuu(ctx context.Context, token string) (httpx.CitizenSession, bool, error) { // vi-name-ok: implements the existing httpx.CitizenSessions method
	if token == "" {
		// CitizenEdge only calls with a non-empty bearer token; an empty one is a wiring fault, and the
		// RPC would answer InvalidArgument for it anyway.
		return httpx.CitizenSession{}, false, errors.New("identity/grpc: citizen edge asked with an empty token")
	}
	ra, err := c.s.ResolveCitizenSession(ctx, &identityv1.ResolveCitizenSessionRequest{SessionToken: token})
	if err != nil {
		// Registry unreadable or the commune check could not be made: CitizenEdge answers 503 and the
		// session is left alone — never "no session" (fail closed, never a sign-out).
		return httpx.CitizenSession{}, false, err
	}
	p := ra.GetSession()
	if p == nil {
		return httpx.CitizenSession{}, false, nil
	}
	if p.GetSessionId() == "" {
		return httpx.CitizenSession{}, false, errors.New("identity/grpc: ResolveCitizenSession answered a session without a sid")
	}
	return httpx.CitizenSession{
		ID:            p.GetSessionId(),
		CitizenID:     p.GetCitizenId(),
		TenantID:      tenant.ID(p.GetTenantId()),
		ZaloAccountID: p.GetZaloAccountId(),
	}, true, nil
}

var _ httpx.CitizenSessions = (*CitizenEdgeSessions)(nil)
