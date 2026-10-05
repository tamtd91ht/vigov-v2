package grpc

// ZaloBotOperatorService (proto/vigov/comms/v1/zalo_bot_operator.proto) — the six platform-scope RPCs
// service-platform's operator routes call for the ONE shared Zalo Bot (ADR 0074). The contract, its
// status table and every step are the proto's comments; this file only translates.
//
// NO COMMUNE. All six are in core/grpcx.methodsWithoutTenant (owner, 05/10/2026), so the interceptor
// lets them through without "x-tenant-id" — and no handler here reads tenant.From. The caller key is
// still required: the exemption is from the commune, not from the caller.
//
// NEVER LOG A REQUEST HERE. SetSharedZaloBotRequest.String() prints the token. Errors from the use case
// are logged by class; the status message names a rule or a state, never a value.

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/vihat/vigov/core/crypto"
	commsv1 "github.com/vihat/vigov/core/gen/vigov/comms/v1"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// ZaloBotOperator is the use case, declared at the point of use. *app.ZaloBotOperator satisfies it.
type ZaloBotOperator interface {
	SetSharedZaloBot(ctx context.Context, in app.SetSharedZaloBotInput) (app.SetSharedZaloBotOutput, error)
	GetSharedZaloBot(ctx context.Context) (domain.SharedZaloBot, bool, error)
	CheckSharedZaloBot(ctx context.Context, actorCode, actorIP string) (app.CheckOutput, error)
	SetSharedZaloBotWebhook(ctx context.Context, actorCode, actorIP string) (app.SetWebhookOutput, error)
	GetSharedZaloBotWebhook(ctx context.Context) (app.GetWebhookOutput, error)
	ListZaloBotCommuneStats(ctx context.Context, actorCode, actorIP string) ([]domain.ZaloBotCommuneStat, error)
}

// ZaloBotServer implements commsv1.ZaloBotOperatorServiceServer.
type ZaloBotServer struct {
	commsv1.UnimplementedZaloBotOperatorServiceServer
	uc  ZaloBotOperator
	log *slog.Logger
}

// NewZaloBotServer panics on a missing dependency, at construction.
func NewZaloBotServer(uc ZaloBotOperator, log *slog.Logger) *ZaloBotServer {
	if uc == nil || log == nil {
		panic("comms grpc: NewZaloBotServer thiếu phụ thuộc")
	}
	return &ZaloBotServer{uc: uc, log: log}
}

// SetSharedZaloBot — see the proto.
func (s *ZaloBotServer) SetSharedZaloBot(ctx context.Context, req *commsv1.SetSharedZaloBotRequest) (*commsv1.SetSharedZaloBotResponse, error) {
	token := secret.Secret(req.GetToken())
	defer clear(token)
	out, err := s.uc.SetSharedZaloBot(ctx, app.SetSharedZaloBotInput{
		Token: token, BotName: req.GetBotName(), ChatURL: req.GetChatUrl(),
		ActorCode: req.GetActorCode(), ActorIP: req.GetActorIp(), ExpectedRelinkCount: int(req.GetExpectedRelinkCount()),
	})
	if err != nil {
		return nil, s.status(ctx, "SetSharedZaloBot", err)
	}
	res := &commsv1.SetSharedZaloBotResponse{ZaloOutcome: outcomeToWire(out.ZaloOutcome)}
	switch out.Result {
	case app.SetSaved:
		res.Result = commsv1.SetSharedZaloBotResult_SET_SHARED_ZALO_BOT_RESULT_SAVED
		res.Bot = statusToWire(out.Bot)
		res.EndedLinkCount = clampInt32(out.EndedLinkCount)
	case app.SetTokenCheckFailed:
		res.Result = commsv1.SetSharedZaloBotResult_SET_SHARED_ZALO_BOT_RESULT_TOKEN_CHECK_FAILED
	case app.SetRelinkConfirmationRequired:
		res.Result = commsv1.SetSharedZaloBotResult_SET_SHARED_ZALO_BOT_RESULT_RELINK_CONFIRMATION_REQUIRED
		res.LiveLinkCount = clampInt32(out.LiveLinkCount)
	default:
		// Never UNSPECIFIED on the wire (the proto): an unknown result is a defect here.
		s.log.ErrorContext(ctx, "comms/grpc: kết quả đặt token Zalo Bot không xác định", "rpc", "SetSharedZaloBot")
		return nil, status.Error(codes.Internal, "lỗi nội bộ, vui lòng thử lại")
	}
	return res, nil
}

// GetSharedZaloBot — see the proto.
func (s *ZaloBotServer) GetSharedZaloBot(ctx context.Context, _ *commsv1.GetSharedZaloBotRequest) (*commsv1.GetSharedZaloBotResponse, error) {
	b, found, err := s.uc.GetSharedZaloBot(ctx)
	if err != nil {
		return nil, s.status(ctx, "GetSharedZaloBot", err)
	}
	if !found {
		return &commsv1.GetSharedZaloBotResponse{Configured: false}, nil
	}
	res := &commsv1.GetSharedZaloBotResponse{Configured: true, Bot: statusToWire(b)}
	if !b.LastCheckAt.IsZero() {
		res.LastCheck = &commsv1.SharedZaloBotCheck{
			CheckedAt: timestamppb.New(b.LastCheckAt), Outcome: outcomeToWire(b.LastCheckResult),
		}
	}
	return res, nil
}

// CheckSharedZaloBot — see the proto.
func (s *ZaloBotServer) CheckSharedZaloBot(ctx context.Context, req *commsv1.CheckSharedZaloBotRequest) (*commsv1.CheckSharedZaloBotResponse, error) {
	out, err := s.uc.CheckSharedZaloBot(ctx, req.GetActorCode(), req.GetActorIp())
	if err != nil {
		return nil, s.status(ctx, "CheckSharedZaloBot", err)
	}
	res := &commsv1.CheckSharedZaloBotResponse{Outcome: outcomeToWire(out.Outcome), AccountName: out.AccountName}
	if !out.CheckedAt.IsZero() {
		res.Check = &commsv1.SharedZaloBotCheck{CheckedAt: timestamppb.New(out.CheckedAt), Outcome: outcomeToWire(out.Outcome)}
	}
	return res, nil
}

// SetSharedZaloBotWebhook — see the proto.
func (s *ZaloBotServer) SetSharedZaloBotWebhook(ctx context.Context, req *commsv1.SetSharedZaloBotWebhookRequest) (*commsv1.SetSharedZaloBotWebhookResponse, error) {
	out, err := s.uc.SetSharedZaloBotWebhook(ctx, req.GetActorCode(), req.GetActorIp())
	if err != nil {
		return nil, s.status(ctx, "SetSharedZaloBotWebhook", err)
	}
	res := &commsv1.SetSharedZaloBotWebhookResponse{Outcome: outcomeToWire(out.Outcome), Url: out.URL}
	if !out.SetAt.IsZero() {
		res.SetAt, res.SetBy = timestamppb.New(out.SetAt), out.SetBy
	}
	return res, nil
}

// GetSharedZaloBotWebhook — see the proto.
func (s *ZaloBotServer) GetSharedZaloBotWebhook(ctx context.Context, _ *commsv1.GetSharedZaloBotWebhookRequest) (*commsv1.GetSharedZaloBotWebhookResponse, error) {
	out, err := s.uc.GetSharedZaloBotWebhook(ctx)
	if err != nil {
		return nil, s.status(ctx, "GetSharedZaloBotWebhook", err)
	}
	res := &commsv1.GetSharedZaloBotWebhookResponse{Outcome: outcomeToWire(out.Outcome)}
	if out.Outcome != domain.ZaloCallOK {
		return res, nil
	}
	res.Url, res.UrlMatches = out.URL, out.URLMatches
	res.SecretSetAt, res.SecretSetBy = timestampOrNil(out.SecretSetAt), out.SecretSetBy
	res.ZaloUpdatedAt = timestampOrNil(out.ZaloUpdatedAt)
	return res, nil
}

// ListZaloBotCommuneStats — see the proto.
func (s *ZaloBotServer) ListZaloBotCommuneStats(ctx context.Context, req *commsv1.ListZaloBotCommuneStatsRequest) (*commsv1.ListZaloBotCommuneStatsResponse, error) {
	stats, err := s.uc.ListZaloBotCommuneStats(ctx, req.GetActorCode(), req.GetActorIp())
	if err != nil {
		return nil, s.status(ctx, "ListZaloBotCommuneStats", err)
	}
	res := &commsv1.ListZaloBotCommuneStatsResponse{Communes: make([]*commsv1.ZaloBotCommuneStats, 0, len(stats))}
	for _, st := range stats {
		res.Communes = append(res.Communes, &commsv1.ZaloBotCommuneStats{
			TenantId: st.TenantID, ChannelEnabled: st.ChannelEnabled, LinkedStaffCount: clampUint32(st.LinkedStaffCount),
		})
	}
	return res, nil
}

// status is the proto's one status table:
//
//	INVALID_ARGUMENT     a caller fault decided from the request alone; the message names the RULE
//	FAILED_PRECONDITION  the platform data key (SECRET_ENCRYPTION_KEYS) — or, dev only, the webhook
//	                     host — is not configured: an operator-facing 503
//	UNAVAILABLE          comms' own store failed, or the row moved underneath; nothing half-written,
//	                     retrying is safe
//
// The cause is logged by CLASS here and never crosses the boundary. It can never hold the token: the
// use case wraps store and crypto errors only, and the Zalo adapter returns no error at all.
func (s *ZaloBotServer) status(ctx context.Context, rpc string, err error) error {
	switch {
	case errors.Is(err, domain.ErrZaloBotInvalidArgument):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, crypto.ErrNotConfigured):
		return status.Error(codes.FailedPrecondition, "khoá dữ liệu nền tảng chưa cấu hình (SECRET_ENCRYPTION_KEYS) — không niêm, không mở được token Zalo Bot")
	case errors.Is(err, app.ErrZaloBotWebhookHostNotConfigured):
		return status.Error(codes.FailedPrecondition, "ZALO_BOT_WEBHOOK_HOST chưa đặt — webhook chưa có host để trỏ tới")
	}
	s.log.ErrorContext(ctx, "comms/grpc: thao tác vận hành Zalo Bot thất bại", "rpc", rpc, "err", err)
	return status.Error(codes.Unavailable, "chưa ghi được, vui lòng thử lại")
}

func statusToWire(b domain.SharedZaloBot) *commsv1.SharedZaloBotStatus {
	return &commsv1.SharedZaloBotStatus{
		SetAt: timestamppb.New(b.SetAt), SetBy: b.SetBy, HasToken: true, BotName: b.BotName, ChatUrl: b.ChatURL,
	}
}

// outcomeToWire maps the stored vocabulary (ADR 0011) onto the contract enum. An unknown value maps to
// UNSPECIFIED, which the proto says every caller shows as a failure of unknown cause — never success.
func outcomeToWire(v string) commsv1.ZaloBotCallOutcome {
	switch v {
	case domain.ZaloCallOK:
		return commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_OK
	case domain.ZaloCallNotConfigured:
		return commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_NOT_CONFIGURED
	case domain.ZaloCallTokenRejected:
		return commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_TOKEN_REJECTED
	case domain.ZaloCallRateLimited:
		return commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_RATE_LIMITED
	case domain.ZaloCallUnavailable:
		return commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_UNAVAILABLE
	case domain.ZaloCallRejected:
		return commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_REJECTED
	case domain.ZaloCallMalformedResponse:
		return commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_MALFORMED_RESPONSE
	}
	return commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_UNSPECIFIED
}

func timestampOrNil(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

func clampInt32(n int) int32 {
	switch {
	case n <= 0:
		return 0
	case n > int(^uint32(0)>>1):
		return int32(^uint32(0) >> 1)
	default:
		return int32(n)
	}
}
