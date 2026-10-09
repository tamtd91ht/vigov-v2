package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// ActiveResidentialUnitReader is the DECISION read behind ResolveActiveResidentialUnits;
// *idstore.ThonToDanPhoStore satisfies it.
//
// SEPARATE FROM ResidentialUnitNamer on purpose, for the reason OrgUnitReader is separate from
// OrgUnitNamer: the namer answers out-of-use and removed units, and one interface carrying both puts
// "also returns units no longer receiving petitions" one careless edit away from the read a petition
// write decides from.
type ActiveResidentialUnitReader interface {
	ActiveUnitsByID(ctx context.Context, ids []string) ([]domain.ActiveResidentialUnit, error)
}

// ResidentialUnitNamer is the DISPLAY read behind ResolveResidentialUnitNames;
// *idstore.ThonToDanPhoStore satisfies it. It answers out-of-use units as live and removed ones flagged.
type ResidentialUnitNamer interface {
	UnitNamesByID(ctx context.Context, ids []string) ([]domain.ResidentialUnitName, error)
}

// ResolveActiveResidentialUnits answers which of the requested `thon_to_dan_pho` ids are ACTIVE units
// (live and in use) of the commune named by "x-tenant-id", with today's name. Contract in
// identity.proto (ADR 0088).
//
// ORDER — ResolveOrgUnitNames', for its reasons: ceiling (on what was SENT, before dedup) → commune
// present (store.Scoped panics without one) → empty answers empty without a read → read. The ceiling
// is TranIDMotLo, the ResolveResidentialUnitNames number, so a bulk caller batches identically.
//
// ONE ANSWER FOR EVERY REFUSAL — absent. NEVER NOT_FOUND. Ids are never logged. Not audited: the
// caller's write of the petition is the audited act.
func (s *Server) ResolveActiveResidentialUnits(ctx context.Context, req *identityv1.ResolveActiveResidentialUnitsRequest) (
	*identityv1.ResolveActiveResidentialUnitsResponse, error) {

	ids := req.GetIds()
	if len(ids) > TranIDMotLo {
		return nil, status.Errorf(codes.InvalidArgument,
			"ids vượt trần %d cho một lời gọi — bên gọi phải tự chia lô", TranIDMotLo)
	}
	if _, err := s.xa(ctx); err != nil {
		return nil, err
	}
	asked := locID(ids)
	if len(asked) == 0 {
		return &identityv1.ResolveActiveResidentialUnitsResponse{}, nil
	}

	rows, err := s.d.ResidentialUnits.ActiveUnitsByID(ctx, asked)
	if err != nil {
		return nil, s.loi(ctx, err, "ResolveActiveResidentialUnits")
	}

	// THE ANSWER IS A SUBSET OF THE QUESTION, EACH ID ONCE — enforced here rather than trusted to the
	// SQL, because the caller DECIDES from it.
	pending := keySet(asked)
	out := make([]*identityv1.ActiveResidentialUnit, 0, len(rows))
	for _, r := range rows {
		if r.Name == "" {
			// `ten` is NOT NULL; an empty one is a broken row, and the contract promises a name. Absent
			// is the fail-closed answer — the caller refuses rather than writes.
			continue
		}
		if _, ok := pending[r.ID]; !ok {
			continue
		}
		delete(pending, r.ID)
		out = append(out, &identityv1.ActiveResidentialUnit{Id: r.ID, Name: r.Name})
	}
	return &identityv1.ResolveActiveResidentialUnitsResponse{Items: out}, nil
}

// ResolveResidentialUnitNames turns `thon_to_dan_pho` ids already-stored records carry into names,
// OUT-OF-USE AND REMOVED UNITS INCLUDED. Same order and discipline as ResolveOrgUnitNames; contract in
// identity.proto (ADR 0088).
//
// IT RESOLVES A NAME. IT DECIDES NOTHING. NOT_FOUND is never produced; an unresolved id is absent.
func (s *Server) ResolveResidentialUnitNames(ctx context.Context, req *identityv1.ResolveResidentialUnitNamesRequest) (
	*identityv1.ResolveResidentialUnitNamesResponse, error) {

	ids := req.GetIds()
	if len(ids) > TranIDMotLo {
		return nil, status.Errorf(codes.InvalidArgument,
			"ids vượt trần %d cho một lời gọi — bên gọi phải tự chia lô", TranIDMotLo)
	}
	if _, err := s.xa(ctx); err != nil {
		return nil, err
	}
	asked := locID(ids)
	if len(asked) == 0 {
		return &identityv1.ResolveResidentialUnitNamesResponse{}, nil
	}

	rows, err := s.d.ResidentialUnitNames.UnitNamesByID(ctx, asked)
	if err != nil {
		return nil, s.loi(ctx, err, "ResolveResidentialUnitNames")
	}

	pending := keySet(asked)
	out := make([]*identityv1.ResidentialUnitName, 0, len(rows))
	for _, r := range rows {
		if r.Name == "" {
			// The contract promises a name (NOT NULL column). Absent prints "unknown" honestly; an
			// empty name would print a blank the reader takes as fact.
			continue
		}
		if _, ok := pending[r.ID]; !ok {
			continue
		}
		delete(pending, r.ID)
		out = append(out, &identityv1.ResidentialUnitName{Id: r.ID, Name: r.Name, Standing: recordStanding(r.Live)})
	}
	return &identityv1.ResolveResidentialUnitNamesResponse{Items: out}, nil
}
