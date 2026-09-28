package grpc

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// MaxOrgUnitsPerCall is the ceiling on ResolveLiveOrgUnitsRequest.ids, declared where it is enforced.
// 50, the same derivation as TranMaGiaoViecMotLo: one assignment names a unit and perhaps a lead unit.
const MaxOrgUnitsPerCall = 50

// OrgUnitReader is the read behind ResolveLiveOrgUnits and ResolveStaffOrgUnits. Declared HERE at the
// point of use so every branch is checkable without a PostgreSQL; *idstore.BoPhanStore satisfies it.
//
// BOTH METHODS SHARE ONE "LIVE UNIT" PREDICATE in the store (liveOrgUnit), which is the org chart's.
// A unit one of them treats as live and the other does not is a tab showing work a write would refuse.
type OrgUnitReader interface {
	LiveIDs(ctx context.Context, ids []string) ([]string, error)
	UnitsOfStaff(ctx context.Context, ma string) ([]string, error)
	// LiveIDsByCode is the read behind ResolveLiveOrgUnitCodes (reference_names.go). HERE and not
	// on OrgUnitNamer because the caller writes the id it returns: it must share the live predicate.
	LiveIDsByCode(ctx context.Context, codes []string) ([]domain.OrgUnitCodeMatch, error)
}

// ResolveLiveOrgUnits answers which of the requested `bo_phan` ids are live units of the commune named
// by "x-tenant-id". The full contract is on the RPC in identity.proto.
//
// ORDER: ceiling (on what was SENT, before dedup) → commune present → empty answers empty → read. The
// order of ResolveAssignableStaff, for its reasons: an unbounded request is refused before any read,
// and store.Scoped (which panics) is never reached without a commune.
//
// ONE ANSWER FOR EVERY REFUSAL — absent. NEVER NOT_FOUND. Log counts, never ids. Not audited: the
// caller's write of the unit is the audited act.
func (s *Server) ResolveLiveOrgUnits(ctx context.Context, req *identityv1.ResolveLiveOrgUnitsRequest) (
	*identityv1.ResolveLiveOrgUnitsResponse, error) {

	ids := req.GetIds()
	if len(ids) > MaxOrgUnitsPerCall {
		return nil, status.Errorf(codes.InvalidArgument,
			"ids vượt trần %d cho một lời gọi — một lần giao việc không cần chừng ấy bộ phận", MaxOrgUnitsPerCall)
	}

	if _, err := s.xa(ctx); err != nil {
		return nil, err
	}

	asked := locID(ids)
	if len(asked) == 0 {
		return &identityv1.ResolveLiveOrgUnitsResponse{}, nil
	}

	live, err := s.d.OrgUnits.LiveIDs(ctx, asked)
	if err != nil {
		return nil, s.loi(ctx, err, "ResolveLiveOrgUnits")
	}

	// THE ANSWER IS A SUBSET OF THE QUESTION, EACH ID ONCE — enforced here rather than trusted to the
	// SQL, because the caller DECIDES from this set.
	pending := make(map[string]struct{}, len(asked))
	for _, id := range asked {
		pending[id] = struct{}{}
	}
	out := make([]string, 0, len(live))
	for _, id := range live {
		if _, ok := pending[id]; !ok {
			continue
		}
		delete(pending, id)
		out = append(out, id)
	}
	return &identityv1.ResolveLiveOrgUnitsResponse{LiveIds: out}, nil
}

// ResolveStaffOrgUnits answers which live unit(s) the staff record with this business code sits in,
// in the commune named by "x-tenant-id". The full contract — including why this is NOT a field on
// StaffPrincipal — is on the RPC in identity.proto.
//
// EMPTY IS ONE ANSWER FOR FOUR CASES (unknown code, removed record, no unit, removed unit); nothing
// here tells them apart and nothing may. An empty `ma` is a wiring fault in the caller — its staff
// principal always carries a code (rule 6, invariant 8) — so it is INVALID_ARGUMENT, loudly.
//
// NEVER LOG THE CODE: it identifies a person to anyone holding the directory.
func (s *Server) ResolveStaffOrgUnits(ctx context.Context, req *identityv1.ResolveStaffOrgUnitsRequest) (
	*identityv1.ResolveStaffOrgUnitsResponse, error) {

	ma := strings.TrimSpace(req.GetMa())
	if ma == "" {
		return nil, status.Error(codes.InvalidArgument,
			"thiếu ma — bên gọi phải gửi mã cán bộ của chính chủ thể đang đăng nhập")
	}

	if _, err := s.xa(ctx); err != nil {
		return nil, err
	}

	units, err := s.d.OrgUnits.UnitsOfStaff(ctx, ma)
	if err != nil {
		return nil, s.loi(ctx, err, "ResolveStaffOrgUnits")
	}

	// Duplicates and blanks dropped: an empty string in this list would be a unit whose id is
	// nothing, and a caller filtering `bo_phan_id = ANY(...)` on it would match any row storing "".
	return &identityv1.ResolveStaffOrgUnitsResponse{OrgUnitIds: locID(units)}, nil
}
