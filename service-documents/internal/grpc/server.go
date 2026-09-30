// Package grpc serves the documents contract (proto/vigov/documents/v1) to the other services.
//
// ONE question today: CountOrgUnitHoldings, asked by identity before it soft-deletes an org unit.
// It writes nothing, publishes nothing and is not audited — the audited act is identity's delete
// (documents.proto, which defers to petitions.proto for the reasoning).
//
// Health is not implemented, like every other gRPC server in this repository: liveness is the
// HTTP /healthz, outside the commune chain.
package grpc

import (
	"context"
	"log/slog"
	"math"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	documentsv1 "github.com/vihat/vigov/core/gen/vigov/documents/v1"
	"github.com/vihat/vigov/core/tenant"
)

// OrgUnitHoldingsCounter is "open incoming documents this unit holds", in the commune of ctx.
// *store.VanBanDenStore satisfies it.
type OrgUnitHoldingsCounter interface {
	CountOpenHeldByOrgUnit(ctx context.Context, orgUnitID string) (int, error)
}

// Deps are what the server reads. Every field is required.
type Deps struct {
	Incoming OrgUnitHoldingsCounter
	Log      *slog.Logger
}

// Server implements documentsv1.DocumentsServiceServer.
type Server struct {
	documentsv1.UnimplementedDocumentsServiceServer
	d Deps
}

// NewServer panics on a missing dependency, at construction rather than on the first call.
func NewServer(d Deps) *Server {
	if d.Incoming == nil || d.Log == nil {
		panic("documents grpc: NewServer thiếu phụ thuộc")
	}
	return &Server{d: d}
}

// CountOrgUnitHoldings answers how many open incoming documents the unit holds in the commune named
// by "x-tenant-id". Contract and status codes: documents.proto / petitions.proto.
//
// ORDER: blank id → commune present → read. The org-unit id is not logged.
func (s *Server) CountOrgUnitHoldings(ctx context.Context, req *documentsv1.CountOrgUnitHoldingsRequest) (
	*documentsv1.CountOrgUnitHoldingsResponse, error) {

	id := strings.TrimSpace(req.GetOrgUnitId())
	if id == "" {
		return nil, status.Error(codes.InvalidArgument,
			"thiếu org_unit_id — bên gọi phải gửi bo_phan.id của bộ phận sắp xoá")
	}
	// The interceptor already refused a call without a commune; reaching here without one means it
	// is missing from the chain. Refused, never defaulted (rule 1, forbidden #1).
	if _, ok := tenant.From(ctx); !ok {
		s.d.Log.ErrorContext(ctx, "CẢNH BÁO: handler gRPC chạy mà không có xã trong context — "+
			"grpcx.UnaryServerInterceptor không nằm trong chuỗi", "rpc", "CountOrgUnitHoldings")
		return nil, status.Error(codes.Internal, "lỗi cấu hình máy chủ")
	}

	n, err := s.d.Incoming.CountOpenHeldByOrgUnit(ctx, id)
	if err != nil {
		// The cause stays in THIS service's log; the response crosses a boundary (rule 3,
		// forbidden #3). The caller refuses the delete on any error.
		s.d.Log.ErrorContext(ctx, "documents/grpc: đếm văn bản bộ phận đang giữ thất bại",
			"rpc", "CountOrgUnitHoldings", "err", err)
		return nil, status.Error(codes.Internal, "lỗi nội bộ, vui lòng thử lại")
	}
	return &documentsv1.CountOrgUnitHoldingsResponse{OpenIncomingDocuments: clampUint32(n)}, nil
}

// clampUint32 saturates rather than wraps: a wrapped count could read as zero and let a delete
// through.
func clampUint32(n int) uint32 {
	switch {
	case n <= 0:
		return 0
	case uint64(n) > math.MaxUint32:
		return math.MaxUint32
	default:
		return uint32(n)
	}
}
