// Package grpc serves the platform contract to the other services.
//
// It answers exactly two questions — "which commune owns this Host" and "what is this
// commune called" — and it is deliberately incapable of answering a third. ADR 0003: the
// vendor operating the platform cannot read a commune's petitions or documents because THERE
// IS NO PATH, not because a flag is switched off. A flag can be flipped; a path has to be
// written, and writing one shows up in review.
//
// Neither RPC here is audited, and that is a decision rather than an omission. Rule 6 covers
// WRITES to business data; these two read registry metadata, and ResolveHost runs on the edge
// of every single request. An entry per Host resolution would add hundreds of rows a second
// that record nothing anybody would ever look for, and bury the entries that carry legal
// weight. The first RPC here that WRITES anything changes that answer.
package grpc

import (
	"context"
	"errors"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	platformv1 "github.com/vihat/vigov/gen/vigov/platform/v1"
	"github.com/vihat/vigov/pkg/tenant"
	"github.com/vihat/vigov/services/platform/internal/domain"
	"github.com/vihat/vigov/services/platform/internal/store"
)

// Directory is the registry this server reads, declared here at the point of use so the
// server can be tested without a database.
//
// Both methods return an error rather than a bool on purpose. tenant.Directory's ByHost
// collapses "no such commune" and "the database is down" into one false, which is right for
// the HTTP edge — it needs a yes or a no. It is wrong here: those two produce different gRPC
// codes, and collapsing them turns an infrastructure incident into "that commune does not
// exist", which is the kind of answer that sends an operator looking at DNS for an afternoon.
type Directory interface {
	ByHostErr(ctx context.Context, host string) (tenant.Tenant, error)
	ByID(ctx context.Context, id tenant.ID) (tenant.Tenant, error)
}

// Server implements platformv1.PlatformServiceServer.
type Server struct {
	platformv1.UnimplementedPlatformServiceServer

	dir Directory
	log *slog.Logger
}

func NewServer(dir Directory, log *slog.Logger) *Server {
	return &Server{dir: dir, log: log}
}

// ResolveHost maps an incoming Host to a commune.
//
// This runs with NO commune in context — it is the call that establishes one, and it is on
// grpcx's exemption list for that reason. Nothing below may therefore reach for
// tenant.MustFrom.
func (s *Server) ResolveHost(ctx context.Context, req *platformv1.ResolveHostRequest) (
	*platformv1.ResolveHostResponse, error) {

	t, err := s.dir.ByHostErr(ctx, req.GetHost())
	if err != nil {
		return nil, s.loi(ctx, err, "ResolveHost")
	}
	return &platformv1.ResolveHostResponse{Tenant: sangProto(t)}, nil
}

// GetTenant returns registry metadata for one commune: name, host, whether it still operates.
// Nothing else — the response carries only what `message Tenant` declares, and every field
// added there has to survive the ADR 0003 question first.
//
// @cross-tenant: the id is the SUBJECT being looked up, not a claim by the caller about who
// it is. The platform console lists communes, and a commune's own edge renders the name
// attached to an archival record — both need to name a commune other than the caller's. This
// is safe only because the answer is metadata: the identifier is already public (it appears
// in the QR deep link, ADR 0005) and there is no path from here to business content. Any RPC
// added to this service that returns more than metadata invalidates this comment.
func (s *Server) GetTenant(ctx context.Context, req *platformv1.GetTenantRequest) (
	*platformv1.GetTenantResponse, error) {

	id := tenant.ID(req.GetId())
	if !id.Valid() {
		// Checked before touching the database: an identifier that is not a ULID means the
		// caller is using an administrative code or a name, which rule 1 invariant 2 forbids.
		// Saying so is more useful than a NotFound the caller will read as "commune gone".
		return nil, status.Error(codes.InvalidArgument, "id phải là ULID 26 ký tự")
	}

	t, err := s.dir.ByID(ctx, id)
	if err != nil {
		return nil, s.loi(ctx, err, "GetTenant")
	}
	// A deactivated commune IS returned, with Active=false. Rule 7 keeps a merged commune's
	// data and address; hiding it here would leave archival records referring to a commune
	// nothing can name.
	return &platformv1.GetTenantResponse{Tenant: sangProto(t)}, nil
}

// loi maps a directory failure to a gRPC code.
//
// THE DISTINCTION THIS FUNCTION EXISTS FOR: "no commune matches" and "the database is
// unreachable" must never arrive at the caller as the same answer. The first is a routine
// negative the edge turns into a 404; the second is an outage that has to page somebody.
//
// The message returned to the caller never carries the cause. An internal failure text can
// hold a DSN fragment or a query, and this response crosses a service boundary — the cause
// goes to this service's log, where the operator is.
func (s *Server) loi(ctx context.Context, err error, rpc string) error {
	switch {
	case errors.Is(err, store.ErrKhongCoXa):
		return status.Error(codes.NotFound, "không có xã nào ứng với yêu cầu này")

	case errors.Is(err, store.ErrXaNgungHoatDong):
		// Deactivated is NOT "database is fine but commune missing" and not an error either;
		// for a Host lookup it is simply not servable, and the edge must produce the same 404
		// it produces for an unknown Host. Distinguishing the two to an outside caller would
		// disclose which communes exist on the platform.
		return status.Error(codes.NotFound, "không có xã nào ứng với yêu cầu này")

	case errors.Is(err, domain.ErrHostTrong),
		errors.Is(err, domain.ErrHostCoGiaoThuc),
		errors.Is(err, domain.ErrIDKhongHopLe):
		return status.Error(codes.InvalidArgument, "tham số tra cứu không hợp lệ")

	default:
		s.log.ErrorContext(ctx, "tra cứu danh bạ xã thất bại", "rpc", rpc, "err", err)
		return status.Error(codes.Internal, "lỗi nội bộ, vui lòng thử lại")
	}
}

// sangProto copies the registry view onto the wire type.
//
// Field by field, never by reflection or a generic mapper: this is the boundary ADR 0003
// draws, and a mapper that copies "whatever is on the struct" is how a business field
// eventually leaves this service without anybody deciding that it should.
func sangProto(t tenant.Tenant) *platformv1.Tenant {
	return &platformv1.Tenant{
		Id:          t.ID.String(),
		Host:        t.Host,
		DisplayName: t.Name,
		Active:      t.Active,
	}
}
