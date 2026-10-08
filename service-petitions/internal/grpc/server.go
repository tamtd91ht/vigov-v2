// Package grpc serves the petitions contract (proto/vigov/petitions/v1) to the other services.
//
// Two questions, both from identity, both read-only: CountOrgUnitHoldings, asked before it soft-deletes
// an org unit, and ResolveTaskPriorityCodes (task_priority_codes.go), asked before it writes an SLA row
// for one task priority. Neither writes, publishes or audits — the audited act is identity's, inside
// identity's own transaction (petitions.proto).
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

	petitionsv1 "github.com/vihat/vigov/core/gen/vigov/petitions/v1"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// OrgUnitHoldingsCounter is one register's "open records this unit holds", in the commune of ctx.
// Declared here at the point of use; *store.PhieuPhanAnhStore and *store.NhiemVuStore satisfy it.
type OrgUnitHoldingsCounter interface {
	CountOpenHeldByOrgUnit(ctx context.Context, orgUnitID string) (int, error)
}

// TaskPriorityCodeReader is "the live task priorities of the commune in ctx carrying these codes".
// *store.MucUuTienNhiemVuStore satisfies it.
type TaskPriorityCodeReader interface {
	StatesByCode(ctx context.Context, codes []string) ([]domain.TaskPriorityCodeState, error)
}

// Deps are what the server reads. Every field is required.
type Deps struct {
	Petitions      OrgUnitHoldingsCounter
	Tasks          OrgUnitHoldingsCounter
	TaskPriorities TaskPriorityCodeReader
	Log            *slog.Logger
}

// Server implements petitionsv1.PetitionsServiceServer.
type Server struct {
	petitionsv1.UnimplementedPetitionsServiceServer
	d Deps
}

// NewServer panics on a missing dependency, at construction: a nil one would otherwise surface as
// a panic on the first call, in production, read by identity as "try again".
func NewServer(d Deps) *Server {
	if d.Petitions == nil || d.Tasks == nil || d.TaskPriorities == nil || d.Log == nil {
		panic("petitions grpc: NewServer thiếu phụ thuộc")
	}
	return &Server{d: d}
}

// CountOrgUnitHoldings answers how many open petitions and tasks the unit holds in the commune named
// by "x-tenant-id". The whole contract — both predicates, the status codes — is on the RPC in
// petitions.proto.
//
// ORDER: blank id → commune present → two reads. A blank id is refused before anything else
// because it is the caller's wiring fault whatever the commune. The org-unit id is not logged: not
// personal data, but not needed to tell an outage from a misconfiguration either.
func (s *Server) CountOrgUnitHoldings(ctx context.Context, req *petitionsv1.CountOrgUnitHoldingsRequest) (
	*petitionsv1.CountOrgUnitHoldingsResponse, error) {

	id := strings.TrimSpace(req.GetOrgUnitId())
	if id == "" {
		return nil, status.Error(codes.InvalidArgument,
			"thiếu org_unit_id — bên gọi phải gửi bo_phan.id của bộ phận sắp xoá")
	}
	// The interceptor has already refused a call without a commune (this RPC is not exempt). Reaching
	// here without one means the interceptor is not in the chain; refused rather than letting
	// store.DB.For panic, and NEVER defaulted (rule 1, forbidden #1).
	if _, ok := tenant.From(ctx); !ok {
		s.d.Log.ErrorContext(ctx, "CẢNH BÁO: handler gRPC chạy mà không có xã trong context — "+
			"grpcx.UnaryServerInterceptor không nằm trong chuỗi", "rpc", "CountOrgUnitHoldings")
		return nil, status.Error(codes.Internal, "lỗi cấu hình máy chủ")
	}

	petitions, err := s.d.Petitions.CountOpenHeldByOrgUnit(ctx, id)
	if err != nil {
		return nil, s.internal(ctx, err, "phieu_phan_anh")
	}
	tasks, err := s.d.Tasks.CountOpenHeldByOrgUnit(ctx, id)
	if err != nil {
		return nil, s.internal(ctx, err, "nhiem_vu")
	}
	return &petitionsv1.CountOrgUnitHoldingsResponse{
		OpenPetitions: clampUint32(petitions),
		OpenTasks:     clampUint32(tasks),
	}, nil
}

// internal logs the cause HERE and returns a bare INTERNAL: the cause can carry a query or a DSN
// fragment, and this response crosses a service boundary (rule 3, forbidden #3). The caller refuses
// the delete on any error — never reads it as zero.
func (s *Server) internal(ctx context.Context, err error, table string) error {
	s.d.Log.ErrorContext(ctx, "petitions/grpc: đếm hồ sơ bộ phận đang giữ thất bại",
		"rpc", "CountOrgUnitHoldings", "bang", table, "err", err)
	return status.Error(codes.Internal, "lỗi nội bộ, vui lòng thử lại")
}

// clampUint32 maps a count onto the wire type. A negative count is impossible from count(*); an
// overflow saturates rather than wraps, because a wrapped count could read as zero and let a delete
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
