package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// ClaimDueAutomationRuns and RecordAutomationRunOutcome — the runner side of Cấu hình → Tự động hoá
// (ADR 0058). The full contract is on the two RPCs in identity.proto. THIS FILE TRANSLATES: the schedule
// is domain.DueTrigger, the conditional write and the audit entry are app.Automation.

// AutomationRuns is the use case behind both RPCs, declared at the point of use. *app.Automation
// satisfies it.
type AutomationRuns interface {
	ClaimDue(ctx context.Context, scopes []domain.AutomationScope) ([]domain.AutomationRun, error)
	RecordOutcome(ctx context.Context, runID string, rep domain.RunReport) error
}

// MaxClaimScopes — 1 to 10 scopes per claim, checked on what was sent (the contract: three jobs × three
// kinds = nine, plus one).
const MaxClaimScopes = 10

func (s *Server) ClaimDueAutomationRuns(ctx context.Context, req *identityv1.ClaimDueAutomationRunsRequest) (
	*identityv1.ClaimDueAutomationRunsResponse, error) {

	sent := req.GetScopes()
	if len(sent) == 0 {
		return nil, status.Error(codes.InvalidArgument, "thiếu scopes")
	}
	if len(sent) > MaxClaimScopes {
		return nil, status.Errorf(codes.InvalidArgument, "scopes có %d mục, vượt trần %d", len(sent), MaxClaimScopes)
	}
	scopes := make([]domain.AutomationScope, 0, len(sent))
	seen := make(map[domain.AutomationScope]struct{}, len(sent))
	for _, sc := range sent {
		if sc == nil {
			return nil, status.Error(codes.InvalidArgument, "scopes có mục trống")
		}
		job, err := automationJobFrom(sc.GetJob())
		if err != nil {
			return nil, err
		}
		kind, err := loaiViecTu(sc.GetWorkKind())
		if err != nil {
			return nil, err
		}
		k := domain.AutomationScope{Job: job, WorkKind: kind}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		scopes = append(scopes, k)
	}

	if _, err := s.xa(ctx); err != nil {
		return nil, err
	}

	won, err := s.d.Automation.ClaimDue(ctx, scopes)
	if err != nil {
		// NOTHING CLAIMED as far as the runner may assume: it runs nothing on this tick (the contract).
		return nil, s.loi(ctx, err, "ClaimDueAutomationRuns")
	}
	runs := make([]*identityv1.AutomationRun, 0, len(won))
	for _, r := range won {
		pj, ok := automationJobTo(r.Scope.Job)
		pk, ok2 := workKindTo(r.Scope.WorkKind)
		if !ok || !ok2 {
			// UNREACHABLE — the scope came from this request. Checked because a run with an unset job
			// would be one the runner cannot route.
			return nil, status.Error(codes.Internal, "lỗi nội bộ, vui lòng thử lại")
		}
		runs = append(runs, &identityv1.AutomationRun{
			RunId:     r.ID,
			Scope:     &identityv1.AutomationRunScope{Job: pj, WorkKind: pk},
			ClaimedAt: timestamppb.New(r.ClaimedAt),
		})
	}
	return &identityv1.ClaimDueAutomationRunsResponse{Runs: runs}, nil
}

func (s *Server) RecordAutomationRunOutcome(ctx context.Context, req *identityv1.RecordAutomationRunOutcomeRequest) (
	*identityv1.RecordAutomationRunOutcomeResponse, error) {

	if req.GetRunId() == "" {
		return nil, status.Error(codes.InvalidArgument, "thiếu run_id")
	}
	outcome, err := runOutcomeFrom(req.GetOutcome())
	if err != nil {
		return nil, err
	}
	if _, err := s.xa(ctx); err != nil {
		return nil, err
	}
	err = s.d.Automation.RecordOutcome(ctx, req.GetRunId(), domain.RunReport{
		Outcome:                 outcome,
		RecordsExamined:         int(req.GetRecordsExamined()),
		NoticesDelivered:        int(req.GetNoticesDelivered()),
		RecordsWithoutRecipient: int(req.GetRecordsWithoutRecipient()),
	})
	switch {
	case err == nil:
		return &identityv1.RecordAutomationRunOutcomeResponse{}, nil
	case app.IsRunNotFound(err):
		// Unknown and another commune's run are ONE answer (rule 1).
		return nil, status.Error(codes.NotFound, "không có lượt chạy này ở xã")
	case errors.Is(err, app.ErrRunReportInvalid):
		return nil, status.Error(codes.InvalidArgument, "kết quả lượt chạy không hợp lệ")
	}
	return nil, s.loi(ctx, err, "RecordAutomationRunOutcome")
}

// automationJobFrom maps the wire enum. UNSPECIFIED and unknown values are INVALID_ARGUMENT — never a
// default job. The switch is exhaustive; a value added to the contract arrives here as unknown and is
// refused until a case is added.
func automationJobFrom(j identityv1.AutomationJob) (domain.AutomationJob, error) {
	switch j {
	case identityv1.AutomationJob_AUTOMATION_JOB_SLA_REMINDERS:
		return domain.JobSLAReminders, nil
	case identityv1.AutomationJob_AUTOMATION_JOB_ESCALATION:
		return domain.JobEscalation, nil
	case identityv1.AutomationJob_AUTOMATION_JOB_WEEKLY_DIGEST:
		return domain.JobWeeklyDigest, nil
	}
	return "", status.Errorf(codes.InvalidArgument, "job %d không phải một việc tự động hoá hợp lệ", int32(j))
}

func automationJobTo(j domain.AutomationJob) (identityv1.AutomationJob, bool) {
	switch j {
	case domain.JobSLAReminders:
		return identityv1.AutomationJob_AUTOMATION_JOB_SLA_REMINDERS, true
	case domain.JobEscalation:
		return identityv1.AutomationJob_AUTOMATION_JOB_ESCALATION, true
	case domain.JobWeeklyDigest:
		return identityv1.AutomationJob_AUTOMATION_JOB_WEEKLY_DIGEST, true
	}
	return identityv1.AutomationJob_AUTOMATION_JOB_UNSPECIFIED, false
}

// workKindTo is loaiViecTu's inverse. `don-thu` has no wire value yet (contract-designer adds it); a
// domain kind with no wire value is answered false, which the caller treats as unreachable because the
// kind came from the request's own loaiViecTu.
func workKindTo(k domain.LoaiViec) (identityv1.WorkKind, bool) {
	switch k {
	case domain.LoaiViecVanBanDen:
		return identityv1.WorkKind_WORK_KIND_VAN_BAN_DEN, true
	case domain.LoaiViecPhanAnh:
		return identityv1.WorkKind_WORK_KIND_PHAN_ANH, true
	case domain.LoaiViecNhiemVu:
		return identityv1.WorkKind_WORK_KIND_NHIEM_VU, true
	}
	return identityv1.WorkKind_WORK_KIND_UNSPECIFIED, false
}

func runOutcomeFrom(o identityv1.AutomationRunOutcome) (domain.RunOutcome, error) {
	switch o {
	case identityv1.AutomationRunOutcome_AUTOMATION_RUN_OUTCOME_SUCCEEDED:
		return domain.OutcomeSucceeded, nil
	case identityv1.AutomationRunOutcome_AUTOMATION_RUN_OUTCOME_CONFIGURATION_MISSING:
		return domain.OutcomeConfigurationMissing, nil
	case identityv1.AutomationRunOutcome_AUTOMATION_RUN_OUTCOME_DEPENDENCY_UNAVAILABLE:
		return domain.OutcomeDependencyUnavailable, nil
	case identityv1.AutomationRunOutcome_AUTOMATION_RUN_OUTCOME_FAILED:
		return domain.OutcomeFailed, nil
	}
	return "", status.Error(codes.InvalidArgument, "thiếu outcome hoặc outcome không hợp lệ")
}
