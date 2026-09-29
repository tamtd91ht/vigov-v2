package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
)

// ResolveOrgUnitPermissionHolders and ResolveLeadershipStaff — WHO IS TOLD about late work. The full
// contract (why by permission, why leadership is a roster here, the predicates, the status tables) is
// on the two RPCs in identity.proto. Codes only; nothing here is personal data; nothing is audited —
// the audited act is the notification, written where it is sent.

// NoticeRecipientReader is the staff read behind both RPCs, declared at the point of use.
// *idstore.CanBoStore satisfies it.
type NoticeRecipientReader interface {
	OrgUnitPermissionHolders(ctx context.Context, unitIDs []string, key string) (map[string][]string, error)
	LeadershipCodes(ctx context.Context, limit int) ([]string, error)
}

// PermissionKeyChecker asks the `quyen` catalogue whether a key exists. *idstore.QuyenStore satisfies it.
type PermissionKeyChecker interface {
	PermissionKeyExists(ctx context.Context, key string) (bool, error)
}

// The ceilings the contract fixes: at most 50 unit ids per call (ResolveLiveOrgUnits' ceiling), and at
// most 50 leadership accounts — above that is a configuration anomaly, refused rather than truncated.
const (
	MaxHolderUnits     = 50
	MaxLeadershipCodes = 50
)

func (s *Server) ResolveOrgUnitPermissionHolders(ctx context.Context, req *identityv1.ResolveOrgUnitPermissionHoldersRequest) (
	*identityv1.ResolveOrgUnitPermissionHoldersResponse, error) {

	key := req.GetPermissionKey()
	if key == "" {
		return nil, status.Error(codes.InvalidArgument, "thiếu permission_key")
	}
	if n := len(req.GetOrgUnitIds()); n > MaxHolderUnits {
		return nil, status.Errorf(codes.InvalidArgument,
			"org_unit_ids có %d mục, vượt trần %d — bên gọi phải chia lô", n, MaxHolderUnits)
	}

	if _, err := s.xa(ctx); err != nil {
		return nil, err
	}

	// THE KEY MUST BE A ROW OF `quyen` (rule 5, invariant 3c) — checked before the empty-list answer, so
	// a typo is refused even on a call that asks about no unit: answered "nobody" instead, it would make
	// a job that notifies nobody, forever, with every test green.
	exists, err := s.d.PermissionKeys.PermissionKeyExists(ctx, key)
	if err != nil {
		return nil, s.loi(ctx, err, "ResolveOrgUnitPermissionHolders/quyen")
	}
	if !exists {
		return nil, status.Error(codes.InvalidArgument, "permission_key không có trong danh mục quyền")
	}

	units := locID(req.GetOrgUnitIds())
	if len(units) == 0 {
		return &identityv1.ResolveOrgUnitPermissionHoldersResponse{}, nil
	}
	holders, err := s.d.Recipients.OrgUnitPermissionHolders(ctx, units, key)
	if err != nil {
		return nil, s.loi(ctx, err, "ResolveOrgUnitPermissionHolders")
	}
	// One item per REQUESTED unit with at least one holder — never an id that was not asked for, even
	// if the read returned one.
	items := make([]*identityv1.OrgUnitPermissionHolders, 0, len(units))
	for _, u := range units {
		staff := locID(holders[u])
		if len(staff) == 0 {
			continue
		}
		items = append(items, &identityv1.OrgUnitPermissionHolders{OrgUnitId: u, StaffMa: staff})
	}
	return &identityv1.ResolveOrgUnitPermissionHoldersResponse{Items: items}, nil
}

func (s *Server) ResolveLeadershipStaff(ctx context.Context, _ *identityv1.ResolveLeadershipStaffRequest) (
	*identityv1.ResolveLeadershipStaffResponse, error) {

	xa, err := s.xa(ctx)
	if err != nil {
		return nil, err
	}
	// Ceiling PLUS ONE, so "too many" is detectable rather than a silently full list.
	leaders, err := s.d.Recipients.LeadershipCodes(ctx, MaxLeadershipCodes+1)
	if err != nil {
		return nil, s.loi(ctx, err, "ResolveLeadershipStaff")
	}
	leaders = locID(leaders)
	if len(leaders) > MaxLeadershipCodes {
		// A truncated list tells some leaders and not others with nothing reporting which. Logged with
		// the commune and the ceiling — never the codes.
		s.d.Log.WarnContext(ctx, "ResolveLeadershipStaff: xã có quá nhiều tài khoản lãnh đạo — TỪ CHỐI",
			"xa", string(xa), "tran", MaxLeadershipCodes)
		return nil, status.Errorf(codes.FailedPrecondition,
			"xã có hơn %d tài khoản thuộc vai trò lãnh đạo — kiểm lại nhãn Lãnh đạo trên màn Phân quyền", MaxLeadershipCodes)
	}
	return &identityv1.ResolveLeadershipStaffResponse{StaffMa: leaders}, nil
}
