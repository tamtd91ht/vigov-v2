package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// WHAT THIS FILE PROVES: the decisions of app.Automation — no row / off means nothing is claimed,
// the schedule and run-now make scopes due, two concurrent claims have ONE winner, the outcome is
// written once with ONE audit entry by the system principal, staff writes are audited with the staff
// code — all through REAL transactions of core/store over a recording driver. The repository is an
// in-memory fake WITH THE SAME COMPARE-AND-SET the SQL has; the SQL itself is proved in
// store/automation_test.go and (against PostgreSQL) store/automation_pg_test.go.

// --- recording driver: transactions and the audit INSERT ------------------------------------------

type txRecorder struct {
	mu                    sync.Mutex
	begun, commit, rollbk int
	execs                 []recordedExec
	failOn                string // fail the first Exec containing this
}

type recordedExec struct {
	sql  string
	args []driver.Value
}

func (r *txRecorder) Connect(context.Context) (driver.Conn, error) { return &recConn{r: r}, nil }
func (r *txRecorder) Driver() driver.Driver                        { return trinhGia{} }

func (r *txRecorder) audits() []recordedExec {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []recordedExec
	for _, e := range r.execs {
		if strings.Contains(e.sql, "audit_log") {
			out = append(out, e)
		}
	}
	return out
}

type recConn struct{ r *txRecorder }

func (c *recConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("no Prepare") }
func (c *recConn) Close() error                        { return nil }
func (c *recConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *recConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.r.mu.Lock()
	c.r.begun++
	c.r.mu.Unlock()
	return &recTx{r: c.r}, nil
}
func (c *recConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.r.mu.Lock()
	defer c.r.mu.Unlock()
	if c.r.failOn != "" && strings.Contains(q, c.r.failOn) {
		c.r.failOn = ""
		return nil, errors.New("fake: statement built to fail")
	}
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	c.r.execs = append(c.r.execs, recordedExec{sql: q, args: vals})
	return driver.RowsAffected(1), nil
}

type recTx struct{ r *txRecorder }

func (t *recTx) Commit() error   { t.r.mu.Lock(); t.r.commit++; t.r.mu.Unlock(); return nil }
func (t *recTx) Rollback() error { t.r.mu.Lock(); t.r.rollbk++; t.r.mu.Unlock(); return nil }

// --- in-memory repository with the SQL's compare-and-set ------------------------------------------

type memAutomation struct {
	mu       sync.Mutex
	settings map[domain.AutomationJob]domain.AutomationSetting
	states   map[domain.AutomationScope]domain.ScopeState
	runs     map[string]domain.AutomationRun

	// barrier, when set, holds every ScopeStates read until `barrierN` callers have read — which is
	// how two claims are forced to decide on the SAME lease before either writes.
	barrier  *sync.WaitGroup
	barrierN int
}

func newMem() *memAutomation {
	return &memAutomation{
		settings: map[domain.AutomationJob]domain.AutomationSetting{},
		states:   map[domain.AutomationScope]domain.ScopeState{},
		runs:     map[string]domain.AutomationRun{},
	}
}

func (m *memAutomation) Settings(context.Context) ([]domain.AutomationSetting, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.AutomationSetting
	for _, s := range m.settings {
		out = append(out, s)
	}
	return out, nil
}

func (m *memAutomation) LastRuns(context.Context) ([]domain.AutomationRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.AutomationRun
	for _, st := range m.states {
		out = append(out, m.runs[st.LastRunID])
	}
	return out, nil
}

func (m *memAutomation) ScopeStates(context.Context) (map[domain.AutomationScope]domain.ScopeState, error) {
	m.mu.Lock()
	cp := make(map[domain.AutomationScope]domain.ScopeState, len(m.states))
	for k, v := range m.states {
		cp[k] = v
	}
	b := m.barrier
	m.mu.Unlock()
	if b != nil {
		b.Done()
		b.Wait()
	}
	return cp, nil
}

func (m *memAutomation) SettingForUpdate(_ context.Context, _ *store.ScopedTx, j domain.AutomationJob) (domain.AutomationSetting, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.settings[j]
	return s, ok, nil
}

func (m *memAutomation) InsertSetting(_ context.Context, _ *store.ScopedTx, s domain.AutomationSetting) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.settings[s.Job]; ok {
		return idstore.ErrAutomationSettingExists
	}
	m.settings[s.Job] = s
	return nil
}

func (m *memAutomation) UpdateSetting(_ context.Context, _ *store.ScopedTx, s domain.AutomationSetting) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur := m.settings[s.Job]
	s.RunRequestedAt, s.RunRequestedBy = cur.RunRequestedAt, cur.RunRequestedBy // not in the SET clause
	m.settings[s.Job] = s
	return nil
}

func (m *memAutomation) MarkRunRequested(_ context.Context, _ *store.ScopedTx, j domain.AutomationJob, at time.Time, by string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.settings[j]
	s.RunRequestedAt, s.RunRequestedBy = at, by
	m.settings[j] = s
	return nil
}

func (m *memAutomation) ClaimScope(_ context.Context, _ *store.ScopedTx, prev string, run domain.AutomationRun) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.states[run.Scope].LastRunID != prev { // the WHERE of claimScopeStmt
		return false, nil
	}
	m.states[run.Scope] = domain.ScopeState{LastRunID: run.ID, LastClaimedAt: run.ClaimedAt}
	return true, nil
}

func (m *memAutomation) InsertRun(_ context.Context, _ *store.ScopedTx, run domain.AutomationRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runs[run.ID] = run
	return nil
}

func (m *memAutomation) RunForUpdate(_ context.Context, _ *store.ScopedTx, id string) (domain.AutomationRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.runs[id]
	if !ok {
		return domain.AutomationRun{}, idstore.ErrAutomationRunNotFound
	}
	return r, nil
}

func (m *memAutomation) RecordRun(_ context.Context, _ *store.ScopedTx, run domain.AutomationRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runs[run.ID] = run
	return nil
}

// --- harness ----------------------------------------------------------------------------------------

const xaAuto = tenant.ID("01JAUTO0000000000000000000")

var nowAuto = time.Date(2026, 9, 29, 3, 0, 0, 0, time.UTC) // 10:00 Tuesday, Asia/Ho_Chi_Minh

func buildAutomation(t *testing.T) (*Automation, *memAutomation, *txRecorder, context.Context, *time.Time) {
	t.Helper()
	rec := &txRecorder{}
	db := sql.OpenDB(rec)
	t.Cleanup(func() { db.Close() })
	mem := newMem()
	uc := NewAutomation(store.New(db), mem)
	clock := nowAuto
	uc.now = func() time.Time { return clock }
	var n int
	var mu sync.Mutex
	uc.newID = func() (string, error) {
		mu.Lock()
		defer mu.Unlock()
		n++
		return fmt.Sprintf("01JRUN%020d", n), nil
	}
	return uc, mem, rec, tenant.Into(context.Background(), xaAuto), &clock
}

func staffActor() NguoiThucHien {
	return NguoiThucHien{ID: "nd-01JINTERNALIDOFTHEADMIN", Vet: audit.Actor{ID: "CB-0042", Kind: "staff", IP: "10.0.0.7"}}
}

var (
	scopeTasks     = domain.AutomationScope{Job: domain.JobSLAReminders, WorkKind: domain.LoaiViecNhiemVu}
	scopeDocuments = domain.AutomationScope{Job: domain.JobSLAReminders, WorkKind: domain.LoaiViecVanBanDen}
)

func boolp(b bool) *bool { return &b }

// --- claim ------------------------------------------------------------------------------------------

func TestClaimNoRowOrDisabledClaimsNothing(t *testing.T) {
	uc, mem, rec, ctx, _ := buildAutomation(t)

	runs, err := uc.ClaimDue(ctx, []domain.AutomationScope{scopeTasks})
	if err != nil || len(runs) != 0 {
		t.Fatalf("no row: runs = %v, err = %v — no row means off", runs, err)
	}
	mem.settings[domain.JobSLAReminders] = domain.AutomationSetting{Job: domain.JobSLAReminders,
		Enabled: false, IntervalMinutes: 5, EnabledAt: nowAuto.Add(-24 * time.Hour)}
	runs, err = uc.ClaimDue(ctx, []domain.AutomationScope{scopeTasks})
	if err != nil || len(runs) != 0 {
		t.Fatalf("disabled: runs = %v, err = %v", runs, err)
	}
	if rec.begun != 0 {
		t.Errorf("%d transactions opened for nothing due — a one-minute tick must stay read-only", rec.begun)
	}
}

func TestClaimDueScopeIsClaimedOnceWithIdentitysClock(t *testing.T) {
	uc, mem, rec, ctx, clock := buildAutomation(t)
	mem.settings[domain.JobSLAReminders] = domain.AutomationSetting{Job: domain.JobSLAReminders,
		Enabled: true, IntervalMinutes: 15, EnabledAt: nowAuto.Add(-time.Hour)}

	runs, err := uc.ClaimDue(ctx, []domain.AutomationScope{scopeTasks, scopeDocuments})
	if err != nil || len(runs) != 2 {
		t.Fatalf("runs = %v, err = %v; want both scopes", runs, err)
	}
	for _, r := range runs {
		if !r.ClaimedAt.Equal(nowAuto) || r.Trigger != domain.TriggerSchedule {
			t.Errorf("run %+v: claimed_at must be identity's clock", r)
		}
	}
	if len(rec.audits()) != 0 {
		t.Error("a claim wrote an audit entry — the contract says a lease is not audited")
	}
	// The next tick, one minute later: nothing.
	*clock = nowAuto.Add(time.Minute)
	if again, _ := uc.ClaimDue(ctx, []domain.AutomationScope{scopeTasks, scopeDocuments}); len(again) != 0 {
		t.Fatalf("claimed again one minute later: %v", again)
	}
}

// TWO CLAIMS, ONE WINNER: both read the same lease (the barrier holds them until both have), both
// decide "due", both open a transaction — the compare-and-set lets exactly one write.
func TestClaimRaceOneWins(t *testing.T) {
	uc, mem, _, ctx, _ := buildAutomation(t)
	mem.settings[domain.JobEscalation] = domain.AutomationSetting{Job: domain.JobEscalation,
		Enabled: true, RunHour: 7, EnabledAt: nowAuto.Add(-48 * time.Hour)}
	scope := domain.AutomationScope{Job: domain.JobEscalation, WorkKind: domain.LoaiViecPhanAnh}
	mem.barrier = &sync.WaitGroup{}
	mem.barrier.Add(2)

	var wg sync.WaitGroup
	results := make([][]domain.AutomationRun, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r, err := uc.ClaimDue(ctx, []domain.AutomationScope{scope})
			if err != nil {
				t.Errorf("claim %d: %v", i, err)
			}
			results[i] = r
		}(i)
	}
	wg.Wait()
	if total := len(results[0]) + len(results[1]); total != 1 {
		t.Fatalf("%d winners, want exactly 1 (%v)", total, results)
	}
	if len(mem.runs) != 1 {
		t.Errorf("%d runs recorded, want 1 — the loser must not insert a run", len(mem.runs))
	}
}

// RUN NOW: requested at 10:00 on an escalation already claimed at 07:00 today — claimed at the NEXT
// tick regardless of the schedule, once per scope, each scope consuming only its own pending mark.
func TestRunNowClaimedAtNextTickOncePerScope(t *testing.T) {
	uc, mem, _, ctx, clock := buildAutomation(t)
	mem.settings[domain.JobEscalation] = domain.AutomationSetting{Job: domain.JobEscalation,
		Enabled: true, RunHour: 7, EnabledAt: nowAuto.Add(-72 * time.Hour)}
	tasks := domain.AutomationScope{Job: domain.JobEscalation, WorkKind: domain.LoaiViecNhiemVu}
	docs := domain.AutomationScope{Job: domain.JobEscalation, WorkKind: domain.LoaiViecVanBanDen}
	sevenAM := time.Date(2026, 9, 29, 0, 0, 30, 0, time.UTC) // 07:00:30 local
	mem.states[tasks] = domain.ScopeState{LastRunID: "01JOLD000000000000000000TA", LastClaimedAt: sevenAM}
	mem.states[docs] = domain.ScopeState{LastRunID: "01JOLD000000000000000000DO", LastClaimedAt: sevenAM}

	if _, err := uc.RequestRun(ctx, domain.JobEscalation, staffActor()); err != nil {
		t.Fatalf("RequestRun: %v", err)
	}
	*clock = nowAuto.Add(time.Minute)

	// petitions ticks first and claims only its own scope.
	runs, err := uc.ClaimDue(ctx, []domain.AutomationScope{tasks})
	if err != nil || len(runs) != 1 || runs[0].Trigger != domain.TriggerRequest {
		t.Fatalf("petitions tick: %v (err %v), want one run triggered by the request", runs, err)
	}
	// documents ticks next: its mark is still pending.
	runs, err = uc.ClaimDue(ctx, []domain.AutomationScope{docs})
	if err != nil || len(runs) != 1 {
		t.Fatalf("documents tick: %v (err %v) — one scope's claim cleared the other's run-now", runs, err)
	}
	// And neither runs again.
	*clock = nowAuto.Add(2 * time.Minute)
	if again, _ := uc.ClaimDue(ctx, []domain.AutomationScope{tasks, docs}); len(again) != 0 {
		t.Fatalf("run-now consumed twice: %v", again)
	}
}

// --- record outcome ---------------------------------------------------------------------------------

func TestRecordOutcomeFirstWriteWinsOneSystemAuditEntry(t *testing.T) {
	uc, mem, rec, ctx, _ := buildAutomation(t)
	mem.runs["01JRUN00000000000000000001"] = domain.AutomationRun{ID: "01JRUN00000000000000000001",
		Scope: scopeTasks, Trigger: domain.TriggerSchedule, ClaimedAt: nowAuto}

	first := domain.RunReport{Outcome: domain.OutcomeSucceeded, RecordsExamined: 40, NoticesDelivered: 3, RecordsWithoutRecipient: 1}
	if err := uc.RecordOutcome(ctx, "01JRUN00000000000000000001", first); err != nil {
		t.Fatalf("RecordOutcome: %v", err)
	}
	second := domain.RunReport{Outcome: domain.OutcomeFailed}
	if err := uc.RecordOutcome(ctx, "01JRUN00000000000000000001", second); err != nil {
		t.Fatalf("second RecordOutcome must answer OK: %v", err)
	}
	if got := mem.runs["01JRUN00000000000000000001"].Report; got != first {
		t.Errorf("stored %+v, want the FIRST outcome %+v", got, first)
	}
	a := rec.audits()
	if len(a) != 1 {
		t.Fatalf("%d audit entries, want exactly one per run", len(a))
	}
	// audit_log: tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta
	if a[0].args[0] != string(xaAuto) || a[0].args[1] != audit.SystemActor || a[0].args[2] != "system" ||
		a[0].args[4] != ActionRecordAutomationRun || a[0].args[5] != "tu-dong-hoa/sla_reminders/nhiem-vu" {
		t.Errorf("audit entry = %v", a[0].args)
	}
	var delta map[string]any
	if err := json.Unmarshal(a[0].args[7].([]byte), &delta); err != nil || delta["run_id"] != "01JRUN00000000000000000001" {
		t.Errorf("delta = %s (%v)", a[0].args[7], err)
	}
}

func TestRecordOutcomeRefusals(t *testing.T) {
	uc, _, rec, ctx, _ := buildAutomation(t)
	if err := uc.RecordOutcome(ctx, "01JNOSUCHRUN00000000000000", domain.RunReport{Outcome: domain.OutcomeSucceeded}); !IsRunNotFound(err) {
		t.Errorf("unknown run: %v, want not found", err)
	}
	if err := uc.RecordOutcome(ctx, "x", domain.RunReport{Outcome: ""}); !errors.Is(err, ErrRunReportInvalid) {
		t.Errorf("empty outcome: %v", err)
	}
	if err := uc.RecordOutcome(ctx, "x", domain.RunReport{Outcome: domain.OutcomeSucceeded, RecordsExamined: -1}); !errors.Is(err, ErrRunReportInvalid) {
		t.Errorf("negative count: %v", err)
	}
	if len(rec.audits()) != 0 {
		t.Error("a refused record wrote an audit entry")
	}
}

// AN AUDIT FAILURE TAKES THE OUTCOME DOWN WITH IT: rolled back, nothing committed (rule 6, forbidden #2).
func TestRecordOutcomeAuditFailureRollsBack(t *testing.T) {
	uc, mem, rec, ctx, _ := buildAutomation(t)
	mem.runs["01JRUN00000000000000000001"] = domain.AutomationRun{ID: "01JRUN00000000000000000001", Scope: scopeTasks, ClaimedAt: nowAuto}
	rec.failOn = "audit_log"
	if err := uc.RecordOutcome(ctx, "01JRUN00000000000000000001", domain.RunReport{Outcome: domain.OutcomeSucceeded}); err == nil {
		t.Fatal("audit failed and the record reported success")
	}
	if rec.commit != 0 || rec.rollbk != 1 {
		t.Errorf("commit %d rollback %d, want 0/1", rec.commit, rec.rollbk)
	}
}

// --- staff writes -----------------------------------------------------------------------------------

func TestUpdateSwitchOnStampsEnabledAtAndAuditsStaffCode(t *testing.T) {
	uc, mem, rec, ctx, clock := buildAutomation(t)
	h, m := 7, 0
	view, err := uc.SaveSetting(ctx, domain.JobEscalation, AutomationSettingRequest{Enabled: boolp(true), RunHour: &h, RunMinute: &m}, staffActor())
	if err != nil {
		t.Fatalf("SaveSetting: %v", err)
	}
	s := mem.settings[domain.JobEscalation]
	if !s.EnabledAt.Equal(nowAuto) || !view.Configured || !view.Setting.Enabled {
		t.Fatalf("switch-on: %+v (view %+v)", s, view)
	}
	a := rec.audits()
	if len(a) != 1 || a[0].args[1] != "CB-0042" || a[0].args[4] != ActionUpdateAutomationJob {
		t.Fatalf("audit = %v — actor must be the staff code (rule 6 inv 8)", a)
	}

	// The same body again: no write, no audit.
	*clock = nowAuto.Add(time.Hour)
	if _, err := uc.SaveSetting(ctx, domain.JobEscalation, AutomationSettingRequest{Enabled: boolp(true), RunHour: &h, RunMinute: &m}, staffActor()); err != nil {
		t.Fatal(err)
	}
	if len(rec.audits()) != 1 {
		t.Error("a no-op save was audited")
	}
	// A cadence change while enabled keeps the switch-on instant.
	h = 8
	if _, err := uc.SaveSetting(ctx, domain.JobEscalation, AutomationSettingRequest{Enabled: boolp(true), RunHour: &h, RunMinute: &m}, staffActor()); err != nil {
		t.Fatal(err)
	}
	if s := mem.settings[domain.JobEscalation]; !s.EnabledAt.Equal(nowAuto) || s.RunHour != 8 {
		t.Errorf("cadence change moved enabled_at or lost the hour: %+v", s)
	}
}

func TestUpdateRefusesMissingAndForeignFields(t *testing.T) {
	uc, _, rec, ctx, _ := buildAutomation(t)
	five, seven, one := 5, 7, 1
	cases := map[string]struct {
		job domain.AutomationJob
		req AutomationSettingRequest
		err error
	}{
		"no enabled":           {domain.JobSLAReminders, AutomationSettingRequest{IntervalMinutes: &five}, ErrAutomationEnabledMissing},
		"interval missing":     {domain.JobSLAReminders, AutomationSettingRequest{Enabled: boolp(true)}, ErrAutomationFieldMissing},
		"hour on interval job": {domain.JobSLAReminders, AutomationSettingRequest{Enabled: boolp(true), IntervalMinutes: &five, RunHour: &seven}, ErrAutomationFieldNotApplicable},
		"weekday on daily":     {domain.JobEscalation, AutomationSettingRequest{Enabled: boolp(true), RunHour: &seven, RunMinute: &five, Weekday: &one}, ErrAutomationFieldNotApplicable},
		"interval under 5":     {domain.JobSLAReminders, AutomationSettingRequest{Enabled: boolp(true), IntervalMinutes: &one}, domain.ErrIntervalOutOfRange},
		"unknown job":          {"refresh_dashboards", AutomationSettingRequest{Enabled: boolp(true)}, ErrAutomationJobUnknown},
	}
	for name, c := range cases {
		if _, err := uc.SaveSetting(ctx, c.job, c.req, staffActor()); !errors.Is(err, c.err) {
			t.Errorf("%s: err = %v, want %v", name, err, c.err)
		}
	}
	if rec.begun != 0 {
		t.Errorf("%d transactions opened for refused bodies", rec.begun)
	}
}

func TestRequestRunRefusedWhenOffOrMissing(t *testing.T) {
	uc, mem, rec, ctx, _ := buildAutomation(t)
	if _, err := uc.RequestRun(ctx, domain.JobWeeklyDigest, staffActor()); !errors.Is(err, ErrAutomationJobDisabled) {
		t.Errorf("no row: %v", err)
	}
	mem.settings[domain.JobWeeklyDigest] = domain.AutomationSetting{Job: domain.JobWeeklyDigest, Weekday: 1, RunHour: 7}
	if _, err := uc.RequestRun(ctx, domain.JobWeeklyDigest, staffActor()); !errors.Is(err, ErrAutomationJobDisabled) {
		t.Errorf("off: %v", err)
	}
	if len(rec.audits()) != 0 {
		t.Error("a refused run request was audited")
	}
}

func TestOverviewListsThreeJobsWithPrefillWhenUnconfigured(t *testing.T) {
	uc, _, _, ctx, _ := buildAutomation(t)
	views, err := uc.Overview(ctx)
	if err != nil || len(views) != 3 {
		t.Fatalf("views = %v, err = %v", views, err)
	}
	for i, j := range domain.AutomationJobs() {
		v := views[i]
		if v.Setting.Job != j || v.Configured || v.Setting.Enabled || v.LastRuns == nil {
			t.Errorf("%s: %+v — unconfigured must be off with a prefill and an empty run list", j, v)
		}
	}
}
