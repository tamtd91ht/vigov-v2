package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// OrgUnitNamer is the display read behind ResolveOrgUnitNames. SEPARATE FROM OrgUnitReader on
// purpose, for the reason CanBoTen is separate from CanBoLo: this one answers REMOVED units, and one
// interface carrying both would put "also returns removed units" one careless edit away from the
// read an assignment write decides from. *idstore.BoPhanStore satisfies both.
type OrgUnitNamer interface {
	NamesByID(ctx context.Context, ids []string) ([]domain.OrgUnitName, error)
}

// TaskBlocLabeler is the display read behind ResolveTaskBlocLabels; *idstore.KhoiNhiemVuStore
// satisfies it. It answers removed catalogue rows, flagged.
type TaskBlocLabeler interface {
	LabelsByCode(ctx context.Context, codes []string) ([]domain.TaskBlocLabel, error)
}

// ResolveOrgUnitNames turns `bo_phan` ids an already-stored record carries into names, REMOVED
// UNITS INCLUDED. The full contract is on the RPC in identity.proto.
//
// ORDER — the order of ResolveStaffNames, for its reasons: ceiling (on what was SENT, before dedup)
// → commune present (store.Scoped panics without one) → empty answers empty without a read → read.
// The ceiling is TranIDMotLo, ResolveStaffNames' number, so one register export batches identically
// for people and for units.
//
// IT RESOLVES A NAME. IT DECIDES NOTHING. NOT_FOUND is never produced; an unresolved id is absent.
func (s *Server) ResolveOrgUnitNames(ctx context.Context, req *identityv1.ResolveOrgUnitNamesRequest) (
	*identityv1.ResolveOrgUnitNamesResponse, error) {

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
		return &identityv1.ResolveOrgUnitNamesResponse{}, nil
	}

	rows, err := s.d.OrgUnitNames.NamesByID(ctx, asked)
	if err != nil {
		return nil, s.loi(ctx, err, "ResolveOrgUnitNames")
	}

	// THE ANSWER IS A SUBSET OF THE QUESTION, EACH KEY ONCE — enforced here rather than trusted to
	// the SQL: anti-enumeration property 3 is held at the boundary the contract names.
	pending := keySet(asked)
	out := make([]*identityv1.OrgUnitName, 0, len(rows))
	for _, r := range rows {
		if _, ok := pending[r.ID]; !ok {
			continue
		}
		delete(pending, r.ID)
		out = append(out, &identityv1.OrgUnitName{Id: r.ID, Name: r.Name, Standing: recordStanding(r.Live)})
	}
	return &identityv1.ResolveOrgUnitNamesResponse{Items: out}, nil
}

// ResolveTaskBlocLabels turns task-bloc codes an already-stored record carries into labels, REMOVED
// ROWS INCLUDED. Same order and same discipline as ResolveOrgUnitNames; contract in identity.proto.
func (s *Server) ResolveTaskBlocLabels(ctx context.Context, req *identityv1.ResolveTaskBlocLabelsRequest) (
	*identityv1.ResolveTaskBlocLabelsResponse, error) {

	ma := req.GetMa()
	if len(ma) > TranIDMotLo {
		return nil, status.Errorf(codes.InvalidArgument,
			"ma vượt trần %d cho một lời gọi — bên gọi phải tự chia lô", TranIDMotLo)
	}
	if _, err := s.xa(ctx); err != nil {
		return nil, err
	}
	asked := locID(ma)
	if len(asked) == 0 {
		return &identityv1.ResolveTaskBlocLabelsResponse{}, nil
	}

	rows, err := s.d.TaskBlocLabels.LabelsByCode(ctx, asked)
	if err != nil {
		return nil, s.loi(ctx, err, "ResolveTaskBlocLabels")
	}

	pending := keySet(asked)
	out := make([]*identityv1.TaskBlocLabel, 0, len(rows))
	for _, r := range rows {
		if _, ok := pending[r.Ma]; !ok {
			continue
		}
		delete(pending, r.Ma)
		out = append(out, &identityv1.TaskBlocLabel{Ma: r.Ma, Label: r.Label, Standing: recordStanding(r.Live)})
	}
	return &identityv1.ResolveTaskBlocLabelsResponse{Items: out}, nil
}

// ResolveLiveOrgUnitCodes translates `bo_phan.ma` codes into the ids of LIVE units of this commune.
// The caller DECIDES from this (it writes the id), so the read is OrgUnitReader's — the live
// predicate shared with ResolveLiveOrgUnits — never OrgUnitNamer's. Contract in identity.proto.
//
// The ceiling is TranIDMotLo (200), not MaxOrgUnitsPerCall (50): an import is many assignments at
// once, and its distinct unit codes are bounded by the org chart. Codes are logged by count only.
func (s *Server) ResolveLiveOrgUnitCodes(ctx context.Context, req *identityv1.ResolveLiveOrgUnitCodesRequest) (
	*identityv1.ResolveLiveOrgUnitCodesResponse, error) {

	ma := req.GetMa()
	if len(ma) > TranIDMotLo {
		return nil, status.Errorf(codes.InvalidArgument,
			"ma vượt trần %d cho một lời gọi — bên gọi phải tự chia lô", TranIDMotLo)
	}
	if _, err := s.xa(ctx); err != nil {
		return nil, err
	}
	asked := locID(ma)
	if len(asked) == 0 {
		return &identityv1.ResolveLiveOrgUnitCodesResponse{}, nil
	}

	rows, err := s.d.OrgUnits.LiveIDsByCode(ctx, asked)
	if err != nil {
		return nil, s.loi(ctx, err, "ResolveLiveOrgUnitCodes")
	}

	pending := keySet(asked)
	out := make([]*identityv1.OrgUnitCodeMatch, 0, len(rows))
	for _, r := range rows {
		if r.ID == "" {
			// `bo_phan.id` is part of the primary key and never empty; an empty id would be written
			// onto a task as "no unit". Dropped: absent is the fail-closed answer.
			continue
		}
		if _, ok := pending[r.Ma]; !ok {
			continue
		}
		delete(pending, r.Ma)
		out = append(out, &identityv1.OrgUnitCodeMatch{Ma: r.Ma, Id: r.ID})
	}
	return &identityv1.ResolveLiveOrgUnitCodesResponse{Items: out}, nil
}

// recordStanding maps `deleted_at IS NULL` onto the enum. It never produces UNSPECIFIED — the
// server answers it from one column it has already read.
func recordStanding(live bool) identityv1.RecordStanding {
	if live {
		return identityv1.RecordStanding_RECORD_STANDING_LIVE
	}
	return identityv1.RecordStanding_RECORD_STANDING_REMOVED
}

func keySet(keys []string) map[string]struct{} {
	m := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		m[k] = struct{}{}
	}
	return m
}
