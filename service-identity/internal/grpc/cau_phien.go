package grpc

import (
	"context"
	"errors"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The citizen-session bridge — CitizenSessionBridgeService, served on identity's SECOND gRPC
// listener to exactly one caller, the vihat-miniapp backend (ADR 0045 §Tin cậy). The key check in
// front of it is core/grpcx.UnaryServerBridgeKey.
//
// # WHY A SEPARATE SERVER TYPE AND NOT A METHOD ON Server
//
// Server is registered on the inter-service port behind GRPC_CALLER_KEY. This one is registered on
// the bridge port behind the bridge key, and on NOTHING else. "The bridge key opens exactly one
// RPC" is then a property of which server object sits on which listener — not a method list that
// somebody has to keep right. cmd/server/main_test.go pins both halves: this service is absent from
// the inter-service port, and IdentityService is absent from the bridge port.
//
// # NEVER LOG A MESSAGE OF THIS SERVICE
//
// The generated String() prints every field: the request carries a Zalo account id and possibly a
// phone number (rule 3), the response a working bearer token (rule 8). Fields chosen one by one, or
// nothing.

// MoPhienCau is the use case behind the RPC. *app.CauPhienCongDan satisfies it.
type MoPhienCau interface {
	Mo(ctx context.Context, yc app.YeuCauMoPhienCau) (app.KetQuaMoPhienCau, error)
}

// CauServer implements identityv1.CitizenSessionBridgeServiceServer. Its handler runs with NO
// commune in context — OpenCitizenSession is on core/grpcx.methodsWithoutTenant because it is the
// call that DECIDES the commune.
type CauServer struct {
	identityv1.UnimplementedCitizenSessionBridgeServiceServer

	uc  MoPhienCau
	log *slog.Logger
}

func NewCauServer(uc MoPhienCau, log *slog.Logger) *CauServer {
	if uc == nil {
		panic("identity/grpc: thiếu use case cầu phiên — OpenCitizenSession sẽ panic ở lần mở app đầu tiên")
	}
	if log == nil {
		log = slog.Default()
	}
	return &CauServer{uc: uc, log: log}
}

// OpenCitizenSession follows the status table of citizen_session_bridge.proto.
func (s *CauServer) OpenCitizenSession(ctx context.Context, req *identityv1.OpenCitizenSessionRequest) (
	*identityv1.OpenCitizenSessionResponse, error) {

	// NEVER LOG req. See the note above.
	kq, err := s.uc.Mo(ctx, app.YeuCauMoPhienCau{
		AppID:       req.GetAppId(),
		MaZalo:      req.GetZaloUserId(),
		GoiYXa:      req.GetTenantHint(),
		DaXacNhanXa: req.GetCommuneConfirmed(),
		SoDaXacThuc: req.GetVerifiedPhone(),
		IP:          req.GetClientIp(),
		ThietBi:     req.GetDevice(),
	})
	if err != nil {
		return nil, s.maLoi(ctx, err)
	}

	// "" tenant_id with no token is a real answer (no commune): the Mini App shows the introduction
	// screen only. expires_at is set exactly when a token is.
	ra := &identityv1.OpenCitizenSessionResponse{
		SessionToken:      kq.Token,
		SessionId:         kq.Sid,
		TenantId:          string(kq.Xa),
		TenantDisplayName: kq.TenXa,
		PhoneVerified:     kq.DaCoSo,
		AppMode:           sangMiniAppMode(kq.CheDo),
	}
	if kq.Token != "" {
		ra.ExpiresAt = timestamppb.New(kq.HetHan)
	}
	return ra, nil
}

// maLoi maps the use case's error classes onto the contract. The message never carries the cause
// — the cause is in THIS service's log, without personal data.
func (s *CauServer) maLoi(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, app.ErrCauYeuCauSai):
		return status.Error(codes.InvalidArgument, "yêu cầu không hợp lệ")
	case errors.Is(err, app.ErrCauAppChuaSanSang):
		return status.Error(codes.FailedPrecondition, "ứng dụng chưa sẵn sàng")
	case errors.Is(err, app.ErrCauXaKhongHoatDong):
		return status.Error(codes.FailedPrecondition, "xã được chọn không hoạt động")
	case errors.Is(err, app.ErrCauNenTang):
		return status.Error(codes.Unavailable, "tạm thời không phát được phiên, vui lòng thử lại")
	case errors.Is(err, app.ErrCauXungDot):
		// "UNAVAILABLE etc. — nothing was issued": the caller retries once, which then follows the
		// commune the concurrent switch just remembered.
		return status.Error(codes.Aborted, "vui lòng thử lại")
	}
	s.log.ErrorContext(ctx, "identity/grpc: OpenCitizenSession thất bại", "err", err)
	return status.Error(codes.Internal, "lỗi nội bộ, vui lòng thử lại")
}

func sangMiniAppMode(c domain.CheDoApp) identityv1.MiniAppMode {
	switch c {
	case domain.CheDoAppChinh:
		return identityv1.MiniAppMode_MINI_APP_MODE_MAIN
	case domain.CheDoAppRieng:
		return identityv1.MiniAppMode_MINI_APP_MODE_COMMUNE
	}
	return identityv1.MiniAppMode_MINI_APP_MODE_UNSPECIFIED
}
