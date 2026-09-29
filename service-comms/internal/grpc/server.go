// Package grpc serves the comms contract (proto/vigov/comms/v1) to the other services.
//
// ONE business RPC: DeliverStaffNotifications, called by the automation jobs of service-petitions and
// service-documents (ADR 0058 §3). The whole contract — limits, idempotency, status codes — is the
// RPC's comment in comms.proto and is not restated here (rule 9).
//
// Health is not implemented, like every other gRPC server in this repository: liveness is the HTTP
// /healthz, outside the commune chain.
//
// NEVER LOG A REQUEST HERE. The generated String() prints `title` and `body` in full, and those are
// free text another service composed. Log counts.
package grpc

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/vihat/vigov/core/audit"
	commsv1 "github.com/vihat/vigov/core/gen/vigov/comms/v1"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// StaffNotificationDeliverer is the use case, declared at the point of use. *app.StaffNotifications
// satisfies it.
type StaffNotificationDeliverer interface {
	Deliver(ctx context.Context, in []domain.NotificationDelivery, actor audit.Actor) ([]domain.DeliveryOutcome, error)
}

// Deps are what the server reads. Every field is required.
type Deps struct {
	Notifications StaffNotificationDeliverer
	Log           *slog.Logger
}

// Server implements commsv1.CommsServiceServer.
type Server struct {
	commsv1.UnimplementedCommsServiceServer
	d Deps
}

// NewServer panics on a missing dependency, at construction — a nil one would otherwise surface as a
// panic on the first call, read by the caller as "try again".
func NewServer(d Deps) *Server {
	if d.Notifications == nil || d.Log == nil {
		panic("comms grpc: NewServer thiếu phụ thuộc")
	}
	return &Server{d: d}
}

// kindFromWire maps the proto enum onto the value stored (ADR 0011). UNSPECIFIED and any value this
// build does not know map to "", which domain.ValidateDeliveries refuses — never a default kind.
func kindFromWire(k commsv1.StaffNotificationKind) string {
	switch k {
	case commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DUE_SOON:
		return domain.StaffNotificationDueSoon
	case commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_OVERDUE:
		return domain.StaffNotificationOverdue
	case commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_ESCALATION:
		return domain.StaffNotificationEscalation
	case commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_WEEKLY_DIGEST:
		return domain.StaffNotificationWeeklyDigest
	}
	return ""
}

// DeliverStaffNotifications — see comms.proto.
//
//	INVALID_ARGUMENT   the batch is refused whole; the message names the field and the index, never
//	                   the caller's text
//	INTERNAL           a wiring fault (no commune in context) or a store failure; nothing written,
//	                   and retrying the same request is safe
func (s *Server) DeliverStaffNotifications(ctx context.Context, req *commsv1.DeliverStaffNotificationsRequest) (
	*commsv1.DeliverStaffNotificationsResponse, error) {

	// The interceptor has already refused a call without a commune (this RPC is not exempt). Reaching
	// here without one means the interceptor is missing from the chain — refused, never defaulted.
	if _, ok := tenant.From(ctx); !ok {
		s.d.Log.ErrorContext(ctx, "CẢNH BÁO: handler gRPC chạy mà không có xã trong context — "+
			"grpcx.UnaryServerInterceptor không nằm trong chuỗi", "rpc", "DeliverStaffNotifications")
		return nil, status.Error(codes.Internal, "lỗi cấu hình máy chủ")
	}

	in := make([]domain.NotificationDelivery, 0, len(req.GetNotifications()))
	for _, n := range req.GetNotifications() {
		in = append(in, domain.NotificationDelivery{
			IdempotencyKey: n.GetIdempotencyKey(),
			Kind:           kindFromWire(n.GetKind()),
			RecipientCodes: n.GetRecipientMa(),
			Title:          n.GetTitle(),
			Body:           n.GetBody(),
			Link:           n.GetLink(),
		})
	}

	out, err := s.d.Notifications.Deliver(ctx, in, systemActor(ctx))
	if err != nil {
		if errors.Is(err, domain.ErrInvalidDelivery) {
			// The domain's sentence names a field and an index and nothing the caller wrote.
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		// The cause can carry a statement fragment; it stays in this log and never crosses the
		// boundary (rule 3, forbidden #3). Counts only — never the request.
		s.d.Log.ErrorContext(ctx, "comms/grpc: giao thông báo chuông thất bại",
			"rpc", "DeliverStaffNotifications", "so_thong_bao", len(in),
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		return nil, status.Error(codes.Internal, "lỗi nội bộ, vui lòng thử lại")
	}

	res := &commsv1.DeliverStaffNotificationsResponse{
		Items: make([]*commsv1.StaffNotificationDelivery, 0, len(out)),
	}
	created := 0
	for _, o := range out {
		created += o.Created
		res.Items = append(res.Items, &commsv1.StaffNotificationDelivery{
			IdempotencyKey:   o.IdempotencyKey,
			Created:          clampUint32(o.Created),
			AlreadyDelivered: clampUint32(o.AlreadyDelivered),
		})
	}
	s.d.Log.InfoContext(ctx, "comms/grpc: đã giao thông báo chuông",
		"xa", string(tenant.MustFrom(ctx)), "so_thong_bao", len(out), "tao_moi", created)
	return res, nil
}

// systemActor is the "who" of a delivery: the system principal (core/audit.SystemActor), because the
// shared caller key cannot say which service called (core/grpcx package doc) — and the IP of the
// peer, which is the one attribution this boundary CAN give (rule 6, invariant 2).
//
// ADR 0058 open question #9 — whether "system" satisfies rule 6 invariant 8 — is the contract's
// choice (comms.proto: "ONE audit entry with the system principal"), recorded here, not decided here.
func systemActor(ctx context.Context) audit.Actor {
	a := audit.Actor{ID: audit.SystemActor, Kind: "system"}
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		addr := p.Addr.String()
		if host, _, err := net.SplitHostPort(addr); err == nil {
			addr = host
		}
		a.IP = strings.TrimSpace(addr)
	}
	return a
}

func clampUint32(n int) uint32 {
	switch {
	case n <= 0:
		return 0
	case n > int(^uint32(0)):
		return ^uint32(0)
	default:
		return uint32(n)
	}
}
