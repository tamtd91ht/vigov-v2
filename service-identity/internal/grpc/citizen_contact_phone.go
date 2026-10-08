package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// CitizenContactPhoneReveal is the use case behind ResolveCitizenContactPhone — *app.CitizenContact-
// PhoneReveal in production. A USE CASE, not a store: the read and its audit entry share one
// transaction, and the number is returned only after the entry commits (rule 6, invariants 3 and 7).
//
// It answers idstore.ErrNoContactPhone for every session that cannot disclose, and any other error
// only when the read or the entry could not be completed.
type CitizenContactPhoneReveal interface {
	Reveal(ctx context.Context, sessionID, citizenID string) (string, error)
}

// ResolveCitizenContactPhone hands the full verified phone of the citizen behind one LIVE session to
// the service filing that citizen's petition. The contract (identity.proto) carries the full argument;
// this handler implements its status-code table exactly:
//
//	OK + verified_phone   disclosed AND audited — the entry committed before this returns
//	OK + ""               unknown / revoked / expired / another commune's / unverified / another
//	                      citizen's session — one answer, nothing audited, nothing logged
//	INVALID_ARGUMENT      session_id or citizen_id empty (no "x-tenant-id" is refused earlier, by
//	                      core/grpcx's interceptor)
//	INTERNAL              the read or the audit entry failed — nothing disclosed
//
// NEVER LOG req OR THE RESPONSE: the generated String() of the response prints the number in full,
// and `debug_redact` is a no-op in protobuf-go v1.36.12. The one line this handler can emit is
// Server.loi's, whose error carries the commune and no identifier (rule 3).
func (s *Server) ResolveCitizenContactPhone(ctx context.Context, req *identityv1.ResolveCitizenContactPhoneRequest) (
	*identityv1.ResolveCitizenContactPhoneResponse, error) {

	if _, err := s.xa(ctx); err != nil {
		return nil, err
	}

	sid, citizen := req.GetSessionId(), req.GetCitizenId()
	if sid == "" || citizen == "" {
		// LOUD, not "no phone": the caller must not call without a verified session, so an empty key
		// is a wiring fault in the caller (identity.proto).
		return nil, status.Error(codes.InvalidArgument, "session_id và citizen_id là bắt buộc")
	}

	phone, err := s.d.ContactPhones.Reveal(ctx, sid, citizen)
	if errors.Is(err, idstore.ErrNoContactPhone) {
		// ONE ANSWER for all six cases — NOT NotFound, which would vary with whether a sid exists
		// (rule 4, forbidden #2). No log line: a probe must not become a line per probe.
		return &identityv1.ResolveCitizenContactPhoneResponse{}, nil
	}
	if err != nil {
		// Internal, never "no phone": the caller fails the intake with 503 rather than filing a
		// petition without the number the citizen's screen promised (identity.proto).
		return nil, s.loi(ctx, err, "ResolveCitizenContactPhone")
	}
	return &identityv1.ResolveCitizenContactPhoneResponse{VerifiedPhone: phone}, nil
}
