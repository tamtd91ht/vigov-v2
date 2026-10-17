package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/service-identity/internal/app"
)

// FindCitizenContactByPhone and GetCitizenContactMatchPhone — the directory side of the derived
// petition ↔ directory link (ADR 0102). The contract, including WHY each refusal has the code it has,
// is on the two RPCs in identity.proto; this file implements their status-code tables.
//
// NEVER LOG EITHER REQUEST OR RESPONSE: FindCitizenContactByPhoneRequest and
// GetCitizenContactMatchPhoneResponse both carry a full number, generated String() prints it, and
// `debug_redact` is a no-op in protobuf-go v1.36.12. The one line this file can emit is Server.loi's,
// whose error carries the commune and no value (rule 3).

// CitizenContactMatchService is the use case behind both RPCs — *app.CitizenContactMatch in production.
// A USE CASE, not a store: MatchPhone's read and its audit entry share one transaction, and the number
// leaves only after the entry commits (rule 6, invariants 3 and 7).
type CitizenContactMatchService interface {
	FindByPhone(ctx context.Context, raw string) (contactID string, found bool, err error)
	MatchPhone(ctx context.Context, contactID, actorCode string) (phone string, found bool, err error)
}

// FindCitizenContactByPhone:
//
//	OK + found          the live row's id
//	OK + !found         no live row for this number here, OR a number that does not normalise — one answer
//	INVALID_ARGUMENT    `phone` empty after trimming (no "x-tenant-id" is refused earlier, by core/grpcx)
//	INTERNAL            the read failed — NOT "not in the directory": the caller must not offer "Thêm"
func (s *Server) FindCitizenContactByPhone(ctx context.Context, req *identityv1.FindCitizenContactByPhoneRequest) (
	*identityv1.FindCitizenContactByPhoneResponse, error) {

	if _, err := s.xa(ctx); err != nil {
		return nil, err
	}
	id, found, err := s.d.ContactMatch.FindByPhone(ctx, req.GetPhone())
	if err != nil {
		if errors.Is(err, app.ErrCitizenContactMatchInvalid) {
			// A fixed sentence, never err.Error() widened with the input — the input is a phone number.
			return nil, status.Error(codes.InvalidArgument, "phone là bắt buộc")
		}
		return nil, s.loi(ctx, err, "FindCitizenContactByPhone")
	}
	if !found {
		// NEVER NotFound: absence is an answer, not a failure (identity.proto). No log line either — an
		// ordinary negative must not become a line per lookup.
		return &identityv1.FindCitizenContactByPhoneResponse{}, nil
	}
	return &identityv1.FindCitizenContactByPhoneResponse{Found: true, ContactId: id}, nil
}

// GetCitizenContactMatchPhone:
//
//	OK + found          the number, disclosed AND audited — the entry committed before this returns
//	OK + !found         unknown / soft-deleted / other commune's row — one answer, nothing written
//	INVALID_ARGUMENT    contact_id empty; actor_code empty or not a live staff code of this commune
//	INTERNAL            the read or the entry failed — nothing disclosed
func (s *Server) GetCitizenContactMatchPhone(ctx context.Context, req *identityv1.GetCitizenContactMatchPhoneRequest) (
	*identityv1.GetCitizenContactMatchPhoneResponse, error) {

	if _, err := s.xa(ctx); err != nil {
		return nil, err
	}
	number, found, err := s.d.ContactMatch.MatchPhone(ctx, req.GetContactId(), req.GetActorCode())
	if err != nil {
		if errors.Is(err, app.ErrCitizenContactMatchInvalid) {
			// The use case's reason names the FIELD at fault, never its value (app.ErrCitizenContactMatchInvalid).
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, s.loi(ctx, err, "GetCitizenContactMatchPhone")
	}
	if !found {
		return &identityv1.GetCitizenContactMatchPhoneResponse{}, nil
	}
	return &identityv1.GetCitizenContactMatchPhoneResponse{Found: true, Phone: number}, nil
}
