package http

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// WHAT THIS FILE IS FOR: the three routes of Cấu hình → Tự động hoá. Rule 5, invariant 7 on each —
// 401 no token · 403 wrong permission (the harness default `admin.user`) · 403 `admin.sla` in the
// WRONG commune · 2xx — plus what the handler itself owns: the wire shape, the error mapping, and that
// the audit actor is the staff code. The schedule, the claim and the no-op rule are the use case's,
// proved in app/automation_test.go.

type automationFake struct {
	mu sync.Mutex

	calls     int
	lastJob   domain.AutomationJob
	lastReq   app.AutomationSettingRequest
	lastActor app.NguoiThucHien
	lastXa    tenant.ID

	views []app.AutomationJobView
	err   error
}

func automationSample() *automationFake {
	at := time.Date(2026, 9, 29, 0, 1, 0, 0, time.UTC)
	return &automationFake{views: []app.AutomationJobView{
		{Setting: domain.SuggestedAutomationSetting(domain.JobSLAReminders), LastRuns: []domain.AutomationRun{}},
		{Setting: domain.AutomationSetting{Job: domain.JobEscalation, Enabled: true, RunHour: 7, RunMinute: 0, EnabledAt: at.Add(-72 * time.Hour)},
			Configured: true,
			LastRuns: []domain.AutomationRun{
				{ID: "01JRUN0000000000000000000A", Scope: domain.AutomationScope{Job: domain.JobEscalation, WorkKind: domain.LoaiViecNhiemVu},
					Trigger: domain.TriggerSchedule, ClaimedAt: at,
					Report:     domain.RunReport{Outcome: domain.OutcomeSucceeded, RecordsExamined: 40, NoticesDelivered: 3, RecordsWithoutRecipient: 1},
					RecordedAt: at.Add(time.Minute)},
				{ID: "01JRUN0000000000000000000B", Scope: domain.AutomationScope{Job: domain.JobEscalation, WorkKind: domain.LoaiViecVanBanDen},
					Trigger: domain.TriggerRequest, ClaimedAt: at},
			}},
		{Setting: domain.SuggestedAutomationSetting(domain.JobWeeklyDigest), LastRuns: []domain.AutomationRun{}},
		{Setting: domain.SuggestedAutomationSetting(domain.JobScheduledReports), LastRuns: []domain.AutomationRun{}},
	}}
}

func (f *automationFake) record(ctx context.Context, job domain.AutomationJob, actor app.NguoiThucHien) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.lastJob, f.lastActor, f.lastXa = job, actor, tenant.MustFrom(ctx)
}

func (f *automationFake) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *automationFake) Overview(ctx context.Context) ([]app.AutomationJobView, error) {
	f.record(ctx, "", app.NguoiThucHien{})
	return f.views, f.err
}

func (f *automationFake) SaveSetting(ctx context.Context, job domain.AutomationJob, req app.AutomationSettingRequest,
	actor app.NguoiThucHien) (app.AutomationJobView, error) {
	f.record(ctx, job, actor)
	f.lastReq = req
	return f.views[1], f.err
}

func (f *automationFake) RequestRun(ctx context.Context, job domain.AutomationJob, actor app.NguoiThucHien) (app.AutomationJobView, error) {
	f.record(ctx, job, actor)
	return f.views[1], f.err
}

type automationRoute struct {
	name, method, path, body string
	ok                       int
}

func automationRoutes() []automationRoute {
	return []automationRoute{
		{"list", "GET", "/api/v1/automation-jobs", "", http.StatusOK},
		{"save", "PUT", "/api/v1/automation-jobs/escalation", `{"enabled":true,"run_hour":7,"run_minute":0}`, http.StatusOK},
		{"run now", "POST", "/api/v1/automation-jobs/escalation/runs", "", http.StatusAccepted},
		// The fourth job (ADR 0086 B) on the same two write routes — same key, same four answers.
		{"save scheduled_reports", "PUT", "/api/v1/automation-jobs/scheduled_reports",
			`{"enabled":true,"weekday":1,"run_hour":7,"run_minute":45}`, http.StatusOK},
		{"run now scheduled_reports", "POST", "/api/v1/automation-jobs/scheduled_reports/runs", "", http.StatusAccepted},
	}
}

// buildAutomationServer grants `admin.sla` in commune A and nothing in commune B.
func buildAutomationServer(t *testing.T) *mayChu {
	t.Helper()
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idNoiBo: {authz.Perm("admin.sla"): true}},
			xaB: {},
		}}
	})
	return m
}

func TestAutomation_401NoToken(t *testing.T) {
	m := buildAutomationServer(t)
	for _, r := range automationRoutes() {
		if w := m.goi(t, r.method, hostA, r.path, r.body, ""); w.Code != http.StatusUnauthorized {
			t.Errorf("%s: code = %d, want 401", r.name, w.Code)
		}
	}
	if n := m.automation.count(); n != 0 {
		t.Errorf("use case ran %d times without a session", n)
	}
}

// `admin.user` — a real key, the shipped harness default — does not open the automation tab.
func TestAutomation_403WrongPermission(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)
	for _, r := range automationRoutes() {
		if w := m.goi(t, r.method, hostA, r.path, r.body, tok); w.Code != http.StatusForbidden {
			t.Errorf("%s: code = %d, want 403", r.name, w.Code)
		}
	}
	if n := m.automation.count(); n != 0 {
		t.Errorf("use case ran %d times with the wrong permission", n)
	}
}

// `admin.sla` held in commune A, signed in at commune B: the grant does not exist there (rule 5 inv 3).
func TestAutomation_403RightPermissionWrongCommune(t *testing.T) {
	m := buildAutomationServer(t)
	tok := m.tokenCho(t, xaB, sidB)
	for _, r := range automationRoutes() {
		if w := m.goi(t, r.method, hostB, r.path, r.body, tok); w.Code != http.StatusForbidden {
			t.Errorf("%s: code = %d, want 403", r.name, w.Code)
		}
	}
	if n := m.automation.count(); n != 0 {
		t.Errorf("use case ran %d times in the wrong commune", n)
	}
}

func TestAutomation_2xxBothCorrect(t *testing.T) {
	m := buildAutomationServer(t)
	tok := m.tokenCho(t, xaA, sidA)
	for _, r := range automationRoutes() {
		if w := m.goi(t, r.method, hostA, r.path, r.body, tok); w.Code != r.ok {
			t.Errorf("%s: code = %d, want %d — body %s", r.name, w.Code, r.ok, w.Body.String())
		}
		if m.automation.lastXa != xaA {
			t.Errorf("%s: use case ran in commune %q, want %q", r.name, m.automation.lastXa, xaA)
		}
	}
}

// Rule 6, invariant 8: the two write routes hand the STAFF CODE to the trail, the internal id to decide.
func TestAutomationActorIsStaffCode(t *testing.T) {
	m := buildAutomationServer(t)
	tok := m.tokenCho(t, xaA, sidA)
	for _, r := range automationRoutes()[1:] {
		m.automation.lastActor = app.NguoiThucHien{}
		if w := m.goi(t, r.method, hostA, r.path, r.body, tok); w.Code != r.ok {
			t.Fatalf("%s: %d", r.name, w.Code)
		}
		a := m.automation.lastActor
		if a.Vet.ID != maCanBo || a.ID != idNoiBo {
			t.Errorf("%s: actor = %+v, want trail %q and decider %q", r.name, a, maCanBo, idNoiBo)
		}
	}
}

// THE WIRE SHAPE: three items; the unconfigured job is off with its prefill; cadence fields of other
// kinds are null; a recorded run carries counts, an unrecorded one carries nulls.
func TestListAutomationJobsWireShape(t *testing.T) {
	m := buildAutomationServer(t)
	w := m.goi(t, "GET", hostA, "/api/v1/automation-jobs", "", m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}
	var out struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || len(out.Items) != 4 {
		t.Fatalf("body = %s (%v)", w.Body.String(), err)
	}
	sla := out.Items[0]
	for k, want := range map[string]string{"job": `"sla_reminders"`, "schedule_kind": `"interval"`, "configured": "false",
		"enabled": "false", "interval_minutes": "15", "min_interval_minutes": "5", "run_hour": "null",
		"weekday": "null", "timezone": `"Asia/Ho_Chi_Minh"`, "last_runs": "[]", "run_requested_at": "null"} {
		if got := string(sla[k]); got != want {
			t.Errorf("sla_reminders.%s = %s, want %s", k, got, want)
		}
	}
	esc := out.Items[1]
	if string(esc["run_hour"]) != "7" || string(esc["interval_minutes"]) != "null" || string(esc["configured"]) != "true" {
		t.Errorf("escalation = %v", esc)
	}
	var runs []map[string]json.RawMessage
	_ = json.Unmarshal(esc["last_runs"], &runs)
	if len(runs) != 2 || string(runs[0]["outcome"]) != `"succeeded"` || string(runs[0]["records_without_recipient"]) != "1" ||
		string(runs[1]["outcome"]) != "null" || string(runs[1]["trigger"]) != `"request"` || string(runs[1]["records_examined"]) != "null" {
		t.Errorf("last_runs = %s", esc["last_runs"])
	}
	if string(out.Items[2]["weekday"]) != "1" {
		t.Errorf("weekly_digest prefill weekday = %s, want ISO Monday 1", out.Items[2]["weekday"])
	}
	rep := out.Items[3]
	for k, want := range map[string]string{"job": `"scheduled_reports"`, "schedule_kind": `"monthly_and_weekly"`,
		"configured": "false", "enabled": "false", "weekday": "1", "run_hour": "7", "run_minute": "45",
		"interval_minutes": "null", "min_interval_minutes": "null"} {
		if got := string(rep[k]); got != want {
			t.Errorf("scheduled_reports.%s = %s, want %s", k, got, want)
		}
	}
}

// The new job's PUT reaches the use case with the weekly shape (weekday + HH:MM, no interval).
func TestSaveScheduledReportsPassesWeekdayShape(t *testing.T) {
	m := buildAutomationServer(t)
	w := m.goi(t, "PUT", hostA, "/api/v1/automation-jobs/scheduled_reports",
		`{"enabled":true,"weekday":3,"run_hour":8,"run_minute":15}`, m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}
	r := m.automation.lastReq
	if m.automation.lastJob != domain.JobScheduledReports || r.Enabled == nil || !*r.Enabled || r.Weekday == nil ||
		*r.Weekday != 3 || r.RunHour == nil || *r.RunHour != 8 || r.RunMinute == nil || *r.RunMinute != 15 ||
		r.IntervalMinutes != nil {
		t.Errorf("request reached the use case as %+v (job %q)", r, m.automation.lastJob)
	}
}

func TestSaveAutomationJobPassesFieldsAndMapsErrors(t *testing.T) {
	m := buildAutomationServer(t)
	tok := m.tokenCho(t, xaA, sidA)
	w := m.goi(t, "PUT", hostA, "/api/v1/automation-jobs/weekly_digest",
		`{"enabled":false,"weekday":1,"run_hour":7,"run_minute":30}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}
	r := m.automation.lastReq
	if m.automation.lastJob != domain.JobWeeklyDigest || r.Enabled == nil || *r.Enabled || r.Weekday == nil || *r.Weekday != 1 ||
		r.RunMinute == nil || *r.RunMinute != 30 || r.IntervalMinutes != nil {
		t.Errorf("request reached the use case as %+v (job %q)", r, m.automation.lastJob)
	}

	for name, c := range map[string]struct {
		err  error
		body string
		want int
	}{
		"not json":    {nil, `{`, http.StatusBadRequest},
		"input fault": {app.ErrAutomationFieldMissing, `{"enabled":true}`, http.StatusBadRequest},
		"range":       {domain.ErrIntervalOutOfRange, `{"enabled":true,"interval_minutes":1}`, http.StatusBadRequest},
		"unknown job": {app.ErrAutomationJobUnknown, `{"enabled":true}`, http.StatusNotFound},
		"outage":      {context.DeadlineExceeded, `{"enabled":true}`, http.StatusInternalServerError},
	} {
		m.automation.err = c.err
		if w := m.goi(t, "PUT", hostA, "/api/v1/automation-jobs/escalation", c.body, tok); w.Code != c.want {
			t.Errorf("%s: code = %d, want %d", name, w.Code, c.want)
		}
	}
}

func TestRequestAutomationRunDisabledIs409(t *testing.T) {
	m := buildAutomationServer(t)
	m.automation.err = app.ErrAutomationJobDisabled
	w := m.goi(t, "POST", hostA, "/api/v1/automation-jobs/escalation/runs", "", m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusConflict {
		t.Fatalf("code = %d, want 409", w.Code)
	}
	var e struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &e)
	if e.Code != "automation_job_disabled" {
		t.Errorf("error code = %q", e.Code)
	}
}
