package grpc

// What these tests defend: THE STATUS CODES of the five automation RPCs, exactly as the contract's
// tables in identity.proto state them, and the one piece of arithmetic joined here — escalation
// instants counted FORWARD through the commune's calendar from the missed deadline. The schedule and
// the conditional write are defended in internal/domain and internal/app.

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// --- fakes ------------------------------------------------------------------------------------------

type recipientsFake struct {
	holders  map[string][]string
	leaders  []string
	err      error
	gotUnits []string
	gotKey   string
	gotLimit int
	calls    int
}

func (f *recipientsFake) OrgUnitPermissionHolders(_ context.Context, units []string, key string) (map[string][]string, error) {
	f.calls++
	f.gotUnits, f.gotKey = units, key
	return f.holders, f.err
}

func (f *recipientsFake) LeadershipCodes(_ context.Context, limit int) ([]string, error) {
	f.calls++
	f.gotLimit = limit
	if len(f.leaders) > limit {
		return f.leaders[:limit], f.err
	}
	return f.leaders, f.err
}

type keysFake struct {
	known map[string]bool
	err   error
}

func (f *keysFake) PermissionKeyExists(_ context.Context, key string) (bool, error) {
	return f.known[key], f.err
}

type automationFake struct {
	runs      []domain.AutomationRun
	err       error
	gotScopes []domain.AutomationScope
	gotRunID  string
	gotReport domain.RunReport
	calls     int
}

func (f *automationFake) ClaimDue(_ context.Context, scopes []domain.AutomationScope) ([]domain.AutomationRun, error) {
	f.calls++
	f.gotScopes = scopes
	return f.runs, f.err
}

func (f *automationFake) RecordOutcome(_ context.Context, id string, rep domain.RunReport) error {
	f.calls++
	f.gotRunID, f.gotReport = id, rep
	return f.err
}

// --- ResolveEscalationInstants ---------------------------------------------------------------------

// escalationRow is a default `nhiem-vu` row: unit head after 2 working hours, chairman after 8, and the
// other three columns DIFFERENT so a wrong column shows up as a wrong instant.
func escalationRow() domain.DongSLA {
	return domain.DongSLA{ID: "sla-nv", LoaiViec: domain.LoaiViecNhiemVu,
		GioTiepNhan: 3, GioXuLyXong: 16, GioSapDenHan: 5, GioBaoLanhDao: 2, GioBaoChuTich: 8}
}

func escalationReq(t *testing.T, missed ...string) *identityv1.ResolveEscalationInstantsRequest {
	t.Helper()
	req := &identityv1.ResolveEscalationInstantsRequest{WorkKind: identityv1.WorkKind_WORK_KIND_NHIEM_VU}
	for _, m := range missed {
		req.MissedDeadlines = append(req.MissedDeadlines, timestamppb.New(mocVN(t, m)))
	}
	return req
}

// ACROSS A WEEKEND AND A HOLIDAY. Missed Friday 2026-10-02 16:00; the week is Mon–Fri 07:30–11:30,
// 13:30–17:00; Monday 2026-10-05 is declared a holiday.
//
//	unit head, 2h: Fri 16:00–17:00 = 1h, weekend and Monday skipped, Tue 07:30 + 1h = 08:30
//	chairman,  8h: 1h Friday, Tue 07:30–11:30 = 4h, 13:30 + 3h = 16:30
//
// Wall-clock arithmetic would say Fri 18:00 and Sat 00:00 — the answer rule 10 forbids.
func TestEscalationInstantsCountWorkingHoursAcrossWeekendAndHoliday(t *testing.T) {
	s, _ := may(t, func(d *Deps) {
		d.SLA = &slaGia{ds: []domain.DongSLA{escalationRow()}}
		d.NghiLe = &nghiLeGia{ds: []domain.NgayNghiLe{{ID: "nl-1", Ngay: "2026-10-05"}}}
	})
	ra, err := s.ResolveEscalationInstants(ctxXa(xaA), escalationReq(t, "2026-10-02 16:00", "2026-10-02 16:00"))
	if err != nil {
		t.Fatalf("ResolveEscalationInstants: %v", err)
	}
	if len(ra.GetItems()) != 1 {
		t.Fatalf("%d items, want 1 — duplicates collapse", len(ra.GetItems()))
	}
	it := ra.GetItems()[0]
	if !it.GetMissedDeadline().AsTime().Equal(mocVN(t, "2026-10-02 16:00")) {
		t.Errorf("missed_deadline not echoed: %v", it.GetMissedDeadline().AsTime())
	}
	if got, want := it.GetUnitHeadDueAt().AsTime(), mocVN(t, "2026-10-06 08:30"); !got.Equal(want) {
		t.Errorf("unit_head_due_at = %s, want %s", got.In(muiDoiChungVN), want)
	}
	if got, want := it.GetChairmanDueAt().AsTime(), mocVN(t, "2026-10-06 16:30"); !got.Equal(want) {
		t.Errorf("chairman_due_at = %s, want %s", got.In(muiDoiChungVN), want)
	}
}

func TestEscalationInstantsCallerFaultsAreInvalidArgumentAndReadNothing(t *testing.T) {
	tooMany := escalationReq(t)
	for i := 0; i <= MaxMissedDeadlines; i++ {
		tooMany.MissedDeadlines = append(tooMany.MissedDeadlines, timestamppb.New(mocVN(t, "2026-10-02 16:00").Add(time.Duration(i)*time.Minute)))
	}
	cases := map[string]*identityv1.ResolveEscalationInstantsRequest{
		"no work_kind": {MissedDeadlines: []*timestamppb.Timestamp{timestamppb.New(mocVN(t, "2026-10-02 16:00"))}},
		"empty":        escalationReq(t),
		"over 500":     tooMany,
		"nil entry":    {WorkKind: identityv1.WorkKind_WORK_KIND_NHIEM_VU, MissedDeadlines: []*timestamppb.Timestamp{nil}},
		"invalid":      {WorkKind: identityv1.WorkKind_WORK_KIND_NHIEM_VU, MissedDeadlines: []*timestamppb.Timestamp{{Seconds: 1, Nanos: -1}}},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			sla := &slaGia{ds: []domain.DongSLA{escalationRow()}}
			s, _ := may(t, func(d *Deps) { d.SLA = sla })
			if _, err := s.ResolveEscalationInstants(ctxXa(xaA), req); status.Code(err) != codes.InvalidArgument {
				t.Fatalf("code = %v, want InvalidArgument", status.Code(err))
			}
			if sla.soLanGoi != 0 {
				t.Error("store read for a misshapen call")
			}
		})
	}
}

// Nothing configured, a bad threshold, a calendar that cannot count: FAILED_PRECONDITION — never 24.
func TestEscalationInstantsConfigurationFaultsAreFailedPrecondition(t *testing.T) {
	bad := escalationRow()
	bad.GioBaoChuTich = 0
	for name, sua := range map[string]func(*Deps){
		"empty sla":      func(d *Deps) { d.SLA = &slaGia{} },
		"bad threshold":  func(d *Deps) { d.SLA = &slaGia{ds: []domain.DongSLA{bad}} },
		"empty calendar": func(d *Deps) { d.SLA = &slaGia{ds: []domain.DongSLA{escalationRow()}}; d.Lich = &lichGia{} },
	} {
		s, _ := may(t, sua)
		if _, err := s.ResolveEscalationInstants(ctxXa(xaA), escalationReq(t, "2026-10-02 16:00")); status.Code(err) != codes.FailedPrecondition {
			t.Errorf("%s: code = %v, want FailedPrecondition", name, status.Code(err))
		}
	}
	s, _ := may(t, func(d *Deps) { d.SLA = &slaGia{err: loiKho} })
	if _, err := s.ResolveEscalationInstants(ctxXa(xaA), escalationReq(t, "2026-10-02 16:00")); status.Code(err) != codes.Internal {
		t.Errorf("store outage: code = %v, want Internal (not 'nothing crossed')", status.Code(err))
	}
}

// --- ResolveOrgUnitPermissionHolders -----------------------------------------------------------------

func TestHoldersAnswersRequestedUnitsWithHoldersOnly(t *testing.T) {
	rec := &recipientsFake{holders: map[string][]string{
		"U1": {"CB-0001", "CB-0002", "CB-0001"},
		"U9": {"CB-0099"}, // not requested — must never be answered
	}}
	s, _ := may(t, func(d *Deps) { d.Recipients = rec })
	ra, err := s.ResolveOrgUnitPermissionHolders(ctxXa(xaA), &identityv1.ResolveOrgUnitPermissionHoldersRequest{
		OrgUnitIds: []string{"U1", "U2", "U1", ""}, PermissionKey: "task.assign"})
	if err != nil {
		t.Fatalf("holders: %v", err)
	}
	if len(ra.GetItems()) != 1 || ra.GetItems()[0].GetOrgUnitId() != "U1" || len(ra.GetItems()[0].GetStaffMa()) != 2 {
		t.Fatalf("items = %v, want only U1 with two distinct codes", ra.GetItems())
	}
	if len(rec.gotUnits) != 2 || rec.gotKey != "task.assign" {
		t.Errorf("store got units %v key %q — duplicates and blanks must be dropped", rec.gotUnits, rec.gotKey)
	}
}

func TestHoldersStatusCodes(t *testing.T) {
	many := make([]string, MaxHolderUnits+1)
	for i := range many {
		many[i] = "U" + string(rune('A'+i%26)) + string(rune('a'+i/26))
	}
	for name, c := range map[string]struct {
		req  *identityv1.ResolveOrgUnitPermissionHoldersRequest
		want codes.Code
	}{
		"empty key":   {&identityv1.ResolveOrgUnitPermissionHoldersRequest{OrgUnitIds: []string{"U1"}}, codes.InvalidArgument},
		"unknown key": {&identityv1.ResolveOrgUnitPermissionHoldersRequest{OrgUnitIds: []string{"U1"}, PermissionKey: "task.asign"}, codes.InvalidArgument},
		"over 50":     {&identityv1.ResolveOrgUnitPermissionHoldersRequest{OrgUnitIds: many, PermissionKey: "task.assign"}, codes.InvalidArgument},
		"empty list":  {&identityv1.ResolveOrgUnitPermissionHoldersRequest{PermissionKey: "task.assign"}, codes.OK},
	} {
		rec := &recipientsFake{}
		s, _ := may(t, func(d *Deps) { d.Recipients = rec })
		ra, err := s.ResolveOrgUnitPermissionHolders(ctxXa(xaA), c.req)
		if status.Code(err) != c.want {
			t.Errorf("%s: code = %v, want %v", name, status.Code(err), c.want)
		}
		if c.want == codes.OK && (len(ra.GetItems()) != 0 || rec.calls != 0) {
			t.Errorf("%s: empty list answered %v after %d reads — never 'every unit'", name, ra.GetItems(), rec.calls)
		}
	}
	s, _ := may(t, func(d *Deps) { d.PermissionKeys = &keysFake{err: loiKho} })
	if _, err := s.ResolveOrgUnitPermissionHolders(ctxXa(xaA), &identityv1.ResolveOrgUnitPermissionHoldersRequest{
		OrgUnitIds: []string{"U1"}, PermissionKey: "task.assign"}); status.Code(err) != codes.Internal {
		t.Errorf("catalogue outage: code = %v, want Internal", status.Code(err))
	}
}

// --- ResolveLeadershipStaff --------------------------------------------------------------------------

func TestLeadershipStatusCodes(t *testing.T) {
	rec := &recipientsFake{leaders: []string{"CB-0001", "CB-0002"}}
	s, _ := may(t, func(d *Deps) { d.Recipients = rec })
	ra, err := s.ResolveLeadershipStaff(ctxXa(xaA), &identityv1.ResolveLeadershipStaffRequest{})
	if err != nil || len(ra.GetStaffMa()) != 2 || rec.gotLimit != MaxLeadershipCodes+1 {
		t.Fatalf("ok case: %v %v (limit %d)", ra, err, rec.gotLimit)
	}

	var many []string
	for i := 0; i < MaxLeadershipCodes+1; i++ {
		many = append(many, "CB-"+string(rune('A'+i%26))+string(rune('a'+i/26)))
	}
	s, _ = may(t, func(d *Deps) { d.Recipients = &recipientsFake{leaders: many} })
	if _, err := s.ResolveLeadershipStaff(ctxXa(xaA), &identityv1.ResolveLeadershipStaffRequest{}); status.Code(err) != codes.FailedPrecondition {
		t.Errorf("51 leaders: code = %v, want FailedPrecondition — refused, never truncated", status.Code(err))
	}

	s, _ = may(t, func(d *Deps) { d.Recipients = &recipientsFake{} })
	if ra, err := s.ResolveLeadershipStaff(ctxXa(xaA), &identityv1.ResolveLeadershipStaffRequest{}); err != nil || len(ra.GetStaffMa()) != 0 {
		t.Errorf("no leadership: %v %v — empty is ordinary", ra, err)
	}
}

// --- ClaimDueAutomationRuns -------------------------------------------------------------------------

func scope(job identityv1.AutomationJob, kind identityv1.WorkKind) *identityv1.AutomationRunScope {
	return &identityv1.AutomationRunScope{Job: job, WorkKind: kind}
}

func TestClaimMapsScopesAndRuns(t *testing.T) {
	at := time.Date(2026, 9, 29, 3, 0, 0, 0, time.UTC)
	fake := &automationFake{runs: []domain.AutomationRun{{ID: "01JRUN0000000000000000000A",
		Scope: domain.AutomationScope{Job: domain.JobEscalation, WorkKind: domain.LoaiViecPhanAnh}, ClaimedAt: at}}}
	s, _ := may(t, func(d *Deps) { d.Automation = fake })
	ra, err := s.ClaimDueAutomationRuns(ctxXa(xaA), &identityv1.ClaimDueAutomationRunsRequest{Scopes: []*identityv1.AutomationRunScope{
		scope(identityv1.AutomationJob_AUTOMATION_JOB_ESCALATION, identityv1.WorkKind_WORK_KIND_PHAN_ANH),
		scope(identityv1.AutomationJob_AUTOMATION_JOB_ESCALATION, identityv1.WorkKind_WORK_KIND_PHAN_ANH),
		scope(identityv1.AutomationJob_AUTOMATION_JOB_SLA_REMINDERS, identityv1.WorkKind_WORK_KIND_NHIEM_VU),
	}})
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(fake.gotScopes) != 2 {
		t.Errorf("use case got %v — duplicates collapse", fake.gotScopes)
	}
	r := ra.GetRuns()[0]
	if r.GetRunId() != "01JRUN0000000000000000000A" || !r.GetClaimedAt().AsTime().Equal(at) ||
		r.GetScope().GetJob() != identityv1.AutomationJob_AUTOMATION_JOB_ESCALATION ||
		r.GetScope().GetWorkKind() != identityv1.WorkKind_WORK_KIND_PHAN_ANH {
		t.Errorf("run = %v", r)
	}
}

// DON_THU is claimable and round-trips: the request's wire value reaches the use case as `don-thu`, and
// the won run's domain kind is answered as DON_THU. Without workKindTo's case the handler would find a
// won run it cannot encode — the claim is committed and the runner is told Internal, so that slot is
// lost for the citizen-letter register every time.
func TestClaimCitizenLetterKindRoundTrips(t *testing.T) {
	at := time.Date(2026, 9, 29, 3, 0, 0, 0, time.UTC)
	fake := &automationFake{runs: []domain.AutomationRun{{ID: "01JRUN0000000000000000000B",
		Scope: domain.AutomationScope{Job: domain.JobSLAReminders, WorkKind: domain.LoaiViecDonThu}, ClaimedAt: at}}}
	s, _ := may(t, func(d *Deps) { d.Automation = fake })
	ra, err := s.ClaimDueAutomationRuns(ctxXa(xaA), &identityv1.ClaimDueAutomationRunsRequest{Scopes: []*identityv1.AutomationRunScope{
		scope(identityv1.AutomationJob_AUTOMATION_JOB_SLA_REMINDERS, identityv1.WorkKind_WORK_KIND_DON_THU),
	}})
	if err != nil {
		t.Fatalf("claim don-thu: %v", err)
	}
	if len(fake.gotScopes) != 1 || fake.gotScopes[0].WorkKind != domain.LoaiViecDonThu {
		t.Errorf("use case got %v, want one scope of kind don-thu", fake.gotScopes)
	}
	if len(ra.GetRuns()) != 1 || ra.GetRuns()[0].GetScope().GetWorkKind() != identityv1.WorkKind_WORK_KIND_DON_THU {
		t.Errorf("runs = %v, want one run of kind DON_THU", ra.GetRuns())
	}
}

// SCHEDULED_REPORTS is claimable (identity ships first — the contract's ROLLOUT note) and its run carries
// the period identity decided; every other job's run carries UNSPECIFIED.
func TestClaimScheduledReportsCarriesPeriod(t *testing.T) {
	at := time.Date(2026, 10, 1, 0, 45, 0, 0, time.UTC)
	fake := &automationFake{runs: []domain.AutomationRun{
		{ID: "01JRUN0000000000000000000C", Scope: domain.AutomationScope{Job: domain.JobScheduledReports,
			WorkKind: domain.LoaiViecNhiemVu}, ClaimedAt: at, ReportPeriod: domain.ReportPeriodMonth},
		{ID: "01JRUN0000000000000000000D", Scope: domain.AutomationScope{Job: domain.JobScheduledReports,
			WorkKind: domain.LoaiViecVanBanDen}, ClaimedAt: at, ReportPeriod: domain.ReportPeriodWeek},
		{ID: "01JRUN0000000000000000000E", Scope: domain.AutomationScope{Job: domain.JobEscalation,
			WorkKind: domain.LoaiViecNhiemVu}, ClaimedAt: at},
	}}
	s, _ := may(t, func(d *Deps) { d.Automation = fake })
	ra, err := s.ClaimDueAutomationRuns(ctxXa(xaA), &identityv1.ClaimDueAutomationRunsRequest{Scopes: []*identityv1.AutomationRunScope{
		scope(identityv1.AutomationJob_AUTOMATION_JOB_SCHEDULED_REPORTS, identityv1.WorkKind_WORK_KIND_NHIEM_VU),
	}})
	if err != nil {
		t.Fatalf("claim scheduled_reports: %v", err)
	}
	if len(fake.gotScopes) != 1 || fake.gotScopes[0].Job != domain.JobScheduledReports {
		t.Errorf("use case got %v, want one scheduled_reports scope", fake.gotScopes)
	}
	runs := ra.GetRuns()
	if len(runs) != 3 ||
		runs[0].GetScope().GetJob() != identityv1.AutomationJob_AUTOMATION_JOB_SCHEDULED_REPORTS ||
		runs[0].GetScheduledReportPeriod() != identityv1.ScheduledReportPeriod_SCHEDULED_REPORT_PERIOD_MONTH ||
		runs[1].GetScheduledReportPeriod() != identityv1.ScheduledReportPeriod_SCHEDULED_REPORT_PERIOD_WEEK ||
		runs[2].GetScheduledReportPeriod() != identityv1.ScheduledReportPeriod_SCHEDULED_REPORT_PERIOD_UNSPECIFIED {
		t.Errorf("runs = %v", runs)
	}
}

// A scheduled_reports run reaching the handler without a period is answered Internal — never encoded
// as UNSPECIFIED, which the runner would record as FAILED with nothing reporting why.
func TestClaimScheduledReportsWithoutPeriodIsInternal(t *testing.T) {
	fake := &automationFake{runs: []domain.AutomationRun{{ID: "01JRUN0000000000000000000F",
		Scope:     domain.AutomationScope{Job: domain.JobScheduledReports, WorkKind: domain.LoaiViecNhiemVu},
		ClaimedAt: time.Date(2026, 10, 5, 0, 45, 0, 0, time.UTC)}}}
	s, _ := may(t, func(d *Deps) { d.Automation = fake })
	_, err := s.ClaimDueAutomationRuns(ctxXa(xaA), &identityv1.ClaimDueAutomationRunsRequest{Scopes: []*identityv1.AutomationRunScope{
		scope(identityv1.AutomationJob_AUTOMATION_JOB_SCHEDULED_REPORTS, identityv1.WorkKind_WORK_KIND_NHIEM_VU),
	}})
	if status.Code(err) != codes.Internal {
		t.Errorf("code = %v, want Internal", status.Code(err))
	}
}

func TestClaimStatusCodes(t *testing.T) {
	ok := scope(identityv1.AutomationJob_AUTOMATION_JOB_ESCALATION, identityv1.WorkKind_WORK_KIND_NHIEM_VU)
	var eleven []*identityv1.AutomationRunScope
	for i := 0; i <= MaxClaimScopes; i++ {
		eleven = append(eleven, ok)
	}
	for name, scopes := range map[string][]*identityv1.AutomationRunScope{
		"empty":       nil,
		"over 10":     eleven,
		"no job":      {scope(0, identityv1.WorkKind_WORK_KIND_NHIEM_VU)},
		"no kind":     {scope(identityv1.AutomationJob_AUTOMATION_JOB_ESCALATION, 0)},
		"unknown job": {scope(99, identityv1.WorkKind_WORK_KIND_NHIEM_VU)},
		"nil scope":   {nil},
	} {
		fake := &automationFake{}
		s, _ := may(t, func(d *Deps) { d.Automation = fake })
		if _, err := s.ClaimDueAutomationRuns(ctxXa(xaA), &identityv1.ClaimDueAutomationRunsRequest{Scopes: scopes}); status.Code(err) != codes.InvalidArgument {
			t.Errorf("%s: code = %v, want InvalidArgument", name, status.Code(err))
		}
		if fake.calls != 0 {
			t.Errorf("%s: use case ran for a misshapen call", name)
		}
	}
	s, _ := may(t, func(d *Deps) { d.Automation = &automationFake{err: errors.New("db down")} })
	if _, err := s.ClaimDueAutomationRuns(ctxXa(xaA), &identityv1.ClaimDueAutomationRunsRequest{Scopes: []*identityv1.AutomationRunScope{ok}}); status.Code(err) != codes.Internal {
		t.Errorf("outage: code = %v — never an empty OK the runner reads as 'nothing due'", status.Code(err))
	}
}

// --- RecordAutomationRunOutcome ---------------------------------------------------------------------

func TestRecordStatusCodes(t *testing.T) {
	good := &identityv1.RecordAutomationRunOutcomeRequest{RunId: "01JRUN0000000000000000000A",
		Outcome:         identityv1.AutomationRunOutcome_AUTOMATION_RUN_OUTCOME_CONFIGURATION_MISSING,
		RecordsExamined: 12, NoticesDelivered: 0, RecordsWithoutRecipient: 3}

	fake := &automationFake{}
	s, _ := may(t, func(d *Deps) { d.Automation = fake })
	if _, err := s.RecordAutomationRunOutcome(ctxXa(xaA), good); err != nil {
		t.Fatalf("record: %v", err)
	}
	want := domain.RunReport{Outcome: domain.OutcomeConfigurationMissing, RecordsExamined: 12, RecordsWithoutRecipient: 3}
	if fake.gotRunID != good.RunId || fake.gotReport != want {
		t.Errorf("use case got %q %+v, want %+v", fake.gotRunID, fake.gotReport, want)
	}

	for name, req := range map[string]*identityv1.RecordAutomationRunOutcomeRequest{
		"no run id":  {Outcome: identityv1.AutomationRunOutcome_AUTOMATION_RUN_OUTCOME_SUCCEEDED},
		"no outcome": {RunId: "01JRUN0000000000000000000A"},
	} {
		f := &automationFake{}
		s, _ := may(t, func(d *Deps) { d.Automation = f })
		if _, err := s.RecordAutomationRunOutcome(ctxXa(xaA), req); status.Code(err) != codes.InvalidArgument || f.calls != 0 {
			t.Errorf("%s: code = %v (calls %d), want InvalidArgument and no write", name, status.Code(err), f.calls)
		}
	}

	s, _ = may(t, func(d *Deps) { d.Automation = &automationFake{err: idstore.ErrAutomationRunNotFound} })
	if _, err := s.RecordAutomationRunOutcome(ctxXa(xaA), good); status.Code(err) != codes.NotFound {
		t.Errorf("unknown run: code = %v, want NotFound", status.Code(err))
	}
	s, _ = may(t, func(d *Deps) { d.Automation = &automationFake{err: errors.New("db down")} })
	if _, err := s.RecordAutomationRunOutcome(ctxXa(xaA), good); status.Code(err) != codes.Internal {
		t.Errorf("outage: code = %v, want Internal (retry is safe)", status.Code(err))
	}
}

// Every automation RPC refuses with Internal when the commune interceptor is not in the chain — never
// a panic through store.Scoped.
func TestAutomationRPCsNeedCommuneInContext(t *testing.T) {
	s, _ := may(t, func(d *Deps) { d.SLA = &slaGia{ds: []domain.DongSLA{escalationRow()}} })
	ctx := context.Background()
	calls := map[string]func() error{
		"escalation": func() error { _, e := s.ResolveEscalationInstants(ctx, escalationReq(t, "2026-10-02 16:00")); return e },
		"holders": func() error {
			_, e := s.ResolveOrgUnitPermissionHolders(ctx, &identityv1.ResolveOrgUnitPermissionHoldersRequest{PermissionKey: "task.assign"})
			return e
		},
		"leadership": func() error {
			_, e := s.ResolveLeadershipStaff(ctx, &identityv1.ResolveLeadershipStaffRequest{})
			return e
		},
		"claim": func() error {
			_, e := s.ClaimDueAutomationRuns(ctx, &identityv1.ClaimDueAutomationRunsRequest{Scopes: []*identityv1.AutomationRunScope{
				scope(identityv1.AutomationJob_AUTOMATION_JOB_ESCALATION, identityv1.WorkKind_WORK_KIND_NHIEM_VU)}})
			return e
		},
		"record": func() error {
			_, e := s.RecordAutomationRunOutcome(ctx, &identityv1.RecordAutomationRunOutcomeRequest{RunId: "x",
				Outcome: identityv1.AutomationRunOutcome_AUTOMATION_RUN_OUTCOME_SUCCEEDED})
			return e
		},
	}
	for name, call := range calls {
		if code := status.Code(call()); code != codes.Internal {
			t.Errorf("%s without commune: code = %v, want Internal", name, code)
		}
	}
}
