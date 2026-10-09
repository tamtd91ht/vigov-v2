package identityclient

// What these tests defend: no automation answer is read as "nothing to do" or "nobody" when it is
// in fact a fault — an unset instant, a missing deadline in the reply, a run for a scope nobody asked
// for — and FAILED_PRECONDITION is the configuration sentinel, never a number.

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/grpcx"
)

type fakeAutomationServer struct {
	identityv1.UnimplementedIdentityServiceServer

	err       error
	runs      []*identityv1.AutomationRun
	escalate  func(*identityv1.ResolveEscalationInstantsRequest) *identityv1.ResolveEscalationInstantsResponse
	hold      *identityv1.ResolveUnassignedHoldInstantsResponse
	holders   *identityv1.ResolveOrgUnitPermissionHoldersResponse
	leaders   []string
	recorded  *identityv1.RecordAutomationRunOutcomeRequest
	calls     int
	sawTenant []string
}

func (s *fakeAutomationServer) see(ctx context.Context) error {
	s.calls++
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.sawTenant = md.Get(grpcx.MetadataTenantKey)
	}
	return s.err
}

func (s *fakeAutomationServer) ClaimDueAutomationRuns(ctx context.Context, _ *identityv1.ClaimDueAutomationRunsRequest) (
	*identityv1.ClaimDueAutomationRunsResponse, error) {
	if err := s.see(ctx); err != nil {
		return nil, err
	}
	return &identityv1.ClaimDueAutomationRunsResponse{Runs: s.runs}, nil
}

func (s *fakeAutomationServer) RecordAutomationRunOutcome(ctx context.Context, in *identityv1.RecordAutomationRunOutcomeRequest) (
	*identityv1.RecordAutomationRunOutcomeResponse, error) {
	if err := s.see(ctx); err != nil {
		return nil, err
	}
	s.recorded = in
	return &identityv1.RecordAutomationRunOutcomeResponse{}, nil
}

func (s *fakeAutomationServer) ResolveEscalationInstants(ctx context.Context, in *identityv1.ResolveEscalationInstantsRequest) (
	*identityv1.ResolveEscalationInstantsResponse, error) {
	if err := s.see(ctx); err != nil {
		return nil, err
	}
	return s.escalate(in), nil
}

func (s *fakeAutomationServer) ResolveUnassignedHoldInstants(ctx context.Context, _ *identityv1.ResolveUnassignedHoldInstantsRequest) (
	*identityv1.ResolveUnassignedHoldInstantsResponse, error) {
	if err := s.see(ctx); err != nil {
		return nil, err
	}
	return s.hold, nil
}

func (s *fakeAutomationServer) ResolveOrgUnitPermissionHolders(ctx context.Context, _ *identityv1.ResolveOrgUnitPermissionHoldersRequest) (
	*identityv1.ResolveOrgUnitPermissionHoldersResponse, error) {
	if err := s.see(ctx); err != nil {
		return nil, err
	}
	return s.holders, nil
}

func (s *fakeAutomationServer) ResolveLeadershipStaff(ctx context.Context, _ *identityv1.ResolveLeadershipStaffRequest) (
	*identityv1.ResolveLeadershipStaffResponse, error) {
	if err := s.see(ctx); err != nil {
		return nil, err
	}
	return &identityv1.ResolveLeadershipStaffResponse{StaffMa: s.leaders}, nil
}

// Fixed instants from the fake server — these tests do no hour arithmetic of their own.
var (
	missedAt   = time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	missedAt2  = time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	unitHeadAt = time.Date(2026, 9, 28, 2, 0, 0, 0, time.UTC)
	chairAt    = time.Date(2026, 9, 30, 2, 0, 0, 0, time.UTC)
)

func scopeSLA() *identityv1.AutomationRunScope {
	return &identityv1.AutomationRunScope{Job: identityv1.AutomationJob_AUTOMATION_JOB_SLA_REMINDERS,
		WorkKind: identityv1.WorkKind_WORK_KIND_NHIEM_VU}
}

func TestClaimReturnsRunsAndCarriesTenant(t *testing.T) {
	srv := &fakeAutomationServer{runs: []*identityv1.AutomationRun{{RunId: "run-1", Scope: scopeSLA(),
		ClaimedAt: timestamppb.New(missedAt)}}}
	c := moMay(t, srv)
	runs, err := c.ClaimDueAutomationRuns(ngucCanh(), []*identityv1.AutomationRunScope{scopeSLA()})
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if len(runs) != 1 || runs[0].RunID != "run-1" || !runs[0].ClaimedAt.Equal(missedAt) ||
		runs[0].Job != identityv1.AutomationJob_AUTOMATION_JOB_SLA_REMINDERS {
		t.Errorf("runs = %+v", runs)
	}
	assertTenantOnWire(t, srv.sawTenant)
}

// The period identity decided for a SCHEDULED_REPORTS run reaches the runner unchanged, and UNSPECIFIED
// stays UNSPECIFIED — the runner refuses that run rather than guessing (ADR 0086 B2). A wrapper that
// dropped the field would make every scheduled report a FAILED run with nothing saying why.
func TestClaimCarriesTheScheduledReportPeriod(t *testing.T) {
	scope := &identityv1.AutomationRunScope{Job: identityv1.AutomationJob_AUTOMATION_JOB_SCHEDULED_REPORTS,
		WorkKind: identityv1.WorkKind_WORK_KIND_VAN_BAN_DEN}
	for _, p := range []identityv1.ScheduledReportPeriod{
		identityv1.ScheduledReportPeriod_SCHEDULED_REPORT_PERIOD_MONTH,
		identityv1.ScheduledReportPeriod_SCHEDULED_REPORT_PERIOD_UNSPECIFIED,
	} {
		srv := &fakeAutomationServer{runs: []*identityv1.AutomationRun{{RunId: "run-1", Scope: scope,
			ClaimedAt: timestamppb.New(missedAt), ScheduledReportPeriod: p}}}
		runs, err := moMay(t, srv).ClaimDueAutomationRuns(ngucCanh(), []*identityv1.AutomationRunScope{scope})
		if err != nil {
			t.Fatalf("Claim: %v", err)
		}
		if len(runs) != 1 || runs[0].ScheduledReportPeriod != p {
			t.Errorf("period = %+v, want %v", runs, p)
		}
	}
}

func TestClaimRefusesUnaskedScopeAndUnsetClaimedAt(t *testing.T) {
	other := &identityv1.AutomationRunScope{Job: identityv1.AutomationJob_AUTOMATION_JOB_SLA_REMINDERS,
		WorkKind: identityv1.WorkKind_WORK_KIND_VAN_BAN_DEN}
	for name, run := range map[string]*identityv1.AutomationRun{
		"phạm vi không hỏi": {RunId: "r", Scope: other, ClaimedAt: timestamppb.New(missedAt)},
		"thiếu claimed_at":  {RunId: "r", Scope: scopeSLA()},
	} {
		c := moMay(t, &fakeAutomationServer{runs: []*identityv1.AutomationRun{run}})
		if _, err := c.ClaimDueAutomationRuns(ngucCanh(), []*identityv1.AutomationRunScope{scopeSLA()}); err == nil {
			t.Errorf("%s: không bị từ chối", name)
		}
	}
	c := moMay(t, &fakeAutomationServer{})
	if _, err := c.ClaimDueAutomationRuns(ngucCanh(), nil); err == nil {
		t.Error("không phạm vi mà không bị từ chối")
	}
}

func TestRecordOutcomeSendsCounts(t *testing.T) {
	srv := &fakeAutomationServer{}
	c := moMay(t, srv)
	err := c.RecordAutomationRunOutcome(ngucCanh(), "run-1", AutomationOutcome{
		Outcome: identityv1.AutomationRunOutcome_AUTOMATION_RUN_OUTCOME_SUCCEEDED, RecordsExamined: 4,
		NoticesDelivered: 3, RecordsWithoutRecipient: 1})
	if err != nil {
		t.Fatal(err)
	}
	if r := srv.recorded; r.GetRunId() != "run-1" || r.GetRecordsExamined() != 4 || r.GetNoticesDelivered() != 3 ||
		r.GetRecordsWithoutRecipient() != 1 {
		t.Errorf("recorded = %v", r)
	}
	if err := c.RecordAutomationRunOutcome(ngucCanh(), "run-1", AutomationOutcome{}); err == nil {
		t.Error("kết quả UNSPECIFIED mà không bị từ chối")
	}
}

func TestEscalationInstantsMapsByInstantAndRefusesGaps(t *testing.T) {
	full := func(in *identityv1.ResolveEscalationInstantsRequest) *identityv1.ResolveEscalationInstantsResponse {
		out := &identityv1.ResolveEscalationInstantsResponse{}
		for _, d := range in.GetMissedDeadlines() {
			out.Items = append(out.Items, &identityv1.EscalationInstants{MissedDeadline: d,
				UnitHeadDueAt: timestamppb.New(unitHeadAt), ChairmanDueAt: timestamppb.New(chairAt)})
		}
		return out
	}
	srv := &fakeAutomationServer{escalate: full}
	c := moMay(t, srv)
	// Duplicates collapse; another zone for the same instant is the same key.
	got, err := c.EscalationInstants(ngucCanh(), identityv1.WorkKind_WORK_KIND_NHIEM_VU, "",
		[]time.Time{missedAt, missedAt.In(time.FixedZone("ICT", 7*3600)), missedAt2})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got[missedAt].UnitHeadDueAt.Equal(unitHeadAt) || !got[missedAt2].ChairmanDueAt.Equal(chairAt) {
		t.Errorf("got = %+v", got)
	}
	assertTenantOnWire(t, srv.sawTenant)

	short := func(in *identityv1.ResolveEscalationInstantsRequest) *identityv1.ResolveEscalationInstantsResponse {
		o := full(in)
		o.Items = o.Items[:1]
		return o
	}
	c = moMay(t, &fakeAutomationServer{escalate: short})
	if _, err := c.EscalationInstants(ngucCanh(), identityv1.WorkKind_WORK_KIND_NHIEM_VU, "",
		[]time.Time{missedAt, missedAt2}); err == nil {
		t.Error("trả thiếu mốc mà không báo lỗi")
	}
	unset := func(in *identityv1.ResolveEscalationInstantsRequest) *identityv1.ResolveEscalationInstantsResponse {
		o := full(in)
		o.Items[0].ChairmanDueAt = nil
		return o
	}
	c = moMay(t, &fakeAutomationServer{escalate: unset})
	if _, err := c.EscalationInstants(ngucCanh(), identityv1.WorkKind_WORK_KIND_NHIEM_VU, "",
		[]time.Time{missedAt}); err == nil {
		t.Error("mốc chủ tịch rỗng mà không báo lỗi — sẽ đọc thành năm 1970")
	}
}

func TestAutomationFailedPreconditionIsNotConfigured(t *testing.T) {
	srv := &fakeAutomationServer{err: status.Error(codes.FailedPrecondition, "xã chưa cấu hình sla")}
	c := moMay(t, srv)
	if _, err := c.EscalationInstants(ngucCanh(), identityv1.WorkKind_WORK_KIND_NHIEM_VU, "",
		[]time.Time{missedAt}); !errors.Is(err, ErrAutomationNotConfigured) {
		t.Errorf("escalation: %v", err)
	}
	if _, _, err := c.UnassignedHoldInstants(ngucCanh(), identityv1.WorkKind_WORK_KIND_NHIEM_VU, "",
		[]time.Time{missedAt}); !errors.Is(err, ErrAutomationNotConfigured) {
		t.Errorf("hold: %v", err)
	}
	if _, err := c.LeadershipStaff(ngucCanh()); !errors.Is(err, ErrAutomationNotConfigured) {
		t.Errorf("leadership: %v", err)
	}
	c = moMay(t, &fakeAutomationServer{err: status.Error(codes.Unavailable, "xuống")})
	if _, err := c.OrgUnitPermissionHolders(ngucCanh(), []string{"bp-1"}, "task.assign"); !errors.Is(err, ErrIdentityUnavailable) {
		t.Errorf("holders unavailable: %v", err)
	}
}

func TestUnassignedHoldDisabledIsAnAnswer(t *testing.T) {
	c := moMay(t, &fakeAutomationServer{hold: &identityv1.ResolveUnassignedHoldInstantsResponse{ReportingDisabled: true}})
	disabled, due, err := c.UnassignedHoldInstants(ngucCanh(), identityv1.WorkKind_WORK_KIND_PHAN_ANH, "moi-truong",
		[]time.Time{missedAt})
	if err != nil || !disabled || due != nil {
		t.Errorf("disabled=%v due=%v err=%v", disabled, due, err)
	}
	c = moMay(t, &fakeAutomationServer{hold: &identityv1.ResolveUnassignedHoldInstantsResponse{}})
	if _, _, err := c.UnassignedHoldInstants(ngucCanh(), identityv1.WorkKind_WORK_KIND_PHAN_ANH, "",
		[]time.Time{missedAt}); err == nil {
		t.Error("không tắt mà không có mốc — phải là lỗi hợp đồng")
	}
}

func TestInstantBoundsRefusedBeforeWire(t *testing.T) {
	srv := &fakeAutomationServer{}
	c := moMay(t, srv)
	many := make([]time.Time, MaxInstantsPerCall+1)
	for i := range many {
		many[i] = time.Date(2026, 9, 1, 0, 0, i, 0, time.UTC)
	}
	if _, err := c.EscalationInstants(ngucCanh(), identityv1.WorkKind_WORK_KIND_NHIEM_VU, "", many); err == nil {
		t.Error("501 mốc mà không bị từ chối")
	}
	if _, err := c.EscalationInstants(ngucCanh(), identityv1.WorkKind_WORK_KIND_UNSPECIFIED, "",
		[]time.Time{missedAt}); err == nil {
		t.Error("loại việc UNSPECIFIED mà không bị từ chối")
	}
	units := make([]string, MaxOrgUnitsPerCall+1)
	if _, err := c.OrgUnitPermissionHolders(ngucCanh(), units, "task.assign"); err == nil {
		t.Error("51 bộ phận mà không bị từ chối")
	}
	if srv.calls != 0 {
		t.Errorf("đã gửi %d yêu cầu chỉ có thể thất bại", srv.calls)
	}
}

func TestOrgUnitHoldersRefusesUnaskedUnit(t *testing.T) {
	c := moMay(t, &fakeAutomationServer{holders: &identityv1.ResolveOrgUnitPermissionHoldersResponse{
		Items: []*identityv1.OrgUnitPermissionHolders{{OrgUnitId: "bp-khac", StaffMa: []string{"CB-1"}}}}})
	if _, err := c.OrgUnitPermissionHolders(ngucCanh(), []string{"bp-1"}, "task.assign"); err == nil {
		t.Error("bộ phận không hỏi mà không bị từ chối")
	}
}
