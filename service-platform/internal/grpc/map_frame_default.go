package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
)

// GetMapFrameDefault returns the default map frame of THE COMMUNE IN "x-tenant-id" (ADR 0072 amendment
// 2, K3). The status table is on the RPC in platform.proto; this follows it.
//
// NOT EXEMPT from "x-tenant-id": the interceptor has already refused a call without one, and the store
// reads the commune from ctx (corestore.For). The request has no field naming a commune, so there is no
// way to ask for another commune's frame.
//
// Not configured is OK with no `frame` — never NOT_FOUND. A store failure is Internal, never an empty
// OK: the caller would read that as "not configured" and tell staff the commune has no frame during
// an outage.
//
// Not audited: commune configuration, no personal data, no caller identity on this port (ADR 0025).
func (s *Server) GetMapFrameDefault(ctx context.Context, _ *platformv1.GetMapFrameDefaultRequest) (
	*platformv1.GetMapFrameDefaultResponse, error) {

	f, ok, err := s.frames.MapFrameDefault(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "đọc khung bản đồ mặc định thất bại", "rpc", "GetMapFrameDefault", "err", err)
		return nil, status.Error(codes.Internal, "lỗi nội bộ, vui lòng thử lại")
	}
	if !ok {
		return &platformv1.GetMapFrameDefaultResponse{}, nil
	}
	// Field by field, like sangProto — ADR 0003's boundary. updated_by does not cross: the contract has
	// no field for it, and an operator's code is nothing comms decides from.
	out := &platformv1.MapFrameDefault{
		CenterLat: f.CenterLat,
		CenterLng: f.CenterLng,
		RadiusKm:  f.RadiusKm,
	}
	if !f.UpdatedAt.IsZero() {
		out.UpdatedAt = timestamppb.New(f.UpdatedAt)
	}
	return &platformv1.GetMapFrameDefaultResponse{Frame: out}, nil
}
