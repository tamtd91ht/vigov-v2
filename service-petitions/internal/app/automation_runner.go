package app

// The automation runner of the Tự động hoá tab (docs/ui-ux/14-cau-hinh.md §9), in the service owning
// the data — ADR 0058 (user decision 2026-09-29). This service runs the three jobs over the two kinds
// of work it owns: tasks (NHIEM_VU) and petitions (PHAN_ANH).
//
// ONE TICK A MINUTE (ADR 0058 §7: "chạy ngay" is only a mark in identity, picked up at the next tick,
// ≤ 1 minute). Per tick:
//
//  1. the job locks — one PostgreSQL advisory lock PER JOB in this service's own database (ADR 0058
//     §1), so ONE replica does a tick's work for that job;
//  2. the communes this service holds records for (a `// @cross-tenant:` read of identifiers,
//     ADR 0058 §2b), skipping every commune platform's GetTenant does not report `active`;
//  3. per commune, IN THAT COMMUNE'S CONTEXT, identity's ClaimDueAutomationRuns for the scopes whose
//     lock this replica holds; each claimed run is executed and its outcome recorded.
//
// WHY THE LOCK IS KEPT although the claim already has a single winner: the claim makes each
// (commune, scope, slot) run ONCE across every replica of every service — that is correctness, and it
// does not depend on the lock. The lock is the ADR's "một pod chạy": without it every replica asks
// identity about every commune every minute for scopes another replica will win anyway. Losing the
// lock (a replica dies mid-tick) costs nothing but a skipped tick; the claim still decides who runs.
//
// ONE COMMUNE FAILING NEVER STOPS ANOTHER: every commune runs behind its own recover, and an error
// is logged by code and commune — the commune's identifier is not personal data — never by content.
//
// NOTHING HERE HAS A CLOCK OF ITS OWN. The run's `now` is identity's `claimed_at`, for every
// comparison, every date in a key, every window (identity.proto, ClaimDueAutomationRuns). The ticker
// only decides WHEN to ask.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/vihat/vigov/core/commsclient"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// AutomationTickInterval is how often the runner asks. ADR 0058 §7 promises "run now" within a
// minute; the shortest schedule a commune can set is five minutes, so a minute loses nothing.
const AutomationTickInterval = time.Minute

// CommuneLister lists the communes this service holds records for. *crosstenant.Automation.
type CommuneLister interface {
	CommunesWithRecords(ctx context.Context) ([]tenant.ID, error)
}

// JobLocker holds one advisory lock per job for a tick. *crosstenant.Automation.
type JobLocker interface {
	TryLockJobs(ctx context.Context, jobs []string) (held map[string]bool, release func(), err error)
}

// CommuneRegistry reads the commune in ctx from platform. *platformclient.Directory.
type CommuneRegistry interface {
	// vi-name-ok: the method name of the existing core/platformclient.Directory this interface must match
	XaTrongNguCanh(ctx context.Context) (tenant.Tenant, bool, error)
}

// AutomationIdentity is what a run asks identity. *identityclient.Client satisfies it.
type AutomationIdentity interface {
	ClaimDueAutomationRuns(ctx context.Context, scopes []*identityv1.AutomationRunScope) ([]identityclient.AutomationRun, error)
	RecordAutomationRunOutcome(ctx context.Context, runID string, o identityclient.AutomationOutcome) error
	DueSoonCutoff(ctx context.Context, kind identityv1.WorkKind, linhVuc string, asOf time.Time) (time.Time, error)
	EscalationInstants(ctx context.Context, kind identityv1.WorkKind, linhVuc string,
		missed []time.Time) (map[time.Time]identityclient.EscalationInstants, error)
	UnassignedHoldInstants(ctx context.Context, kind identityv1.WorkKind, linhVuc string,
		starts []time.Time) (bool, map[time.Time]time.Time, error)
	OrgUnitPermissionHolders(ctx context.Context, orgUnitIDs []string, permissionKey string) (map[string][]string, error)
	LeadershipStaff(ctx context.Context) ([]string, error)
}

// NoticeDeliverer is comms' bell inbox. *commsclient.Client satisfies it.
type NoticeDeliverer interface {
	DeliverStaffNotifications(ctx context.Context, notices []commsclient.Notice) (map[string]commsclient.Delivery, error)
}

// TaskAutomationReader is the task register's read for the jobs. *store.NhiemVuStore.
type TaskAutomationReader interface {
	OpenTasksForAutomation(ctx context.Context) ([]domain.AutomationRecord, error)
}

// CitizenReportAutomationReader is the petition register's reads for the jobs. *store.PhieuPhanAnhStore.
type CitizenReportAutomationReader interface {
	OpenCitizenReportsForAutomation(ctx context.Context) ([]domain.AutomationRecord, error)
	CitizenReportDigestCounts(ctx context.Context, asOf, ratedFrom time.Time) (domain.CitizenReportDigestCounts, error)
}

// AutomationDeps wires the runner.
type AutomationDeps struct {
	Communes       CommuneLister
	Locks          JobLocker
	Registry       CommuneRegistry
	Identity       AutomationIdentity
	Comms          NoticeDeliverer
	Tasks          TaskAutomationReader
	CitizenReports CitizenReportAutomationReader
	Log            *slog.Logger
}

// AutomationRunner runs the jobs. Build it with NewAutomationRunner; Run blocks until ctx is done.
type AutomationRunner struct {
	d        AutomationDeps
	interval time.Duration
}

// NewAutomationRunner builds the runner. Every dependency is required: a runner missing one would
// claim runs it cannot execute, and a claimed slot is not given back.
func NewAutomationRunner(d AutomationDeps) (*AutomationRunner, error) {
	if d.Communes == nil || d.Locks == nil || d.Registry == nil || d.Identity == nil || d.Comms == nil ||
		d.Tasks == nil || d.CitizenReports == nil {
		return nil, errors.New("bộ chạy tự động hoá: thiếu phụ thuộc")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &AutomationRunner{d: d, interval: AutomationTickInterval}, nil
}

// automationJob pairs the contract's enum with the lock name (the settings table's lower-case key).
type automationJob struct {
	name string
	job  identityv1.AutomationJob
}

var automationJobs = []automationJob{
	{"sla_reminders", identityv1.AutomationJob_AUTOMATION_JOB_SLA_REMINDERS},
	{"escalation", identityv1.AutomationJob_AUTOMATION_JOB_ESCALATION},
	{"weekly_digest", identityv1.AutomationJob_AUTOMATION_JOB_WEEKLY_DIGEST},
}

// automationKinds are the WorkKinds THIS service owns — and the only ones it may claim: a runner
// claiming another service's scope silently takes its runs away (identity.proto, ClaimDueAutomationRuns).
var automationKinds = []identityv1.WorkKind{
	identityv1.WorkKind_WORK_KIND_NHIEM_VU,
	identityv1.WorkKind_WORK_KIND_PHAN_ANH,
}

// Run ticks until ctx is cancelled — once at start, then every interval. It returns only when ctx is
// done; a tick in progress sees the cancellation through its context and stops at the next commune.
func (r *AutomationRunner) Run(ctx context.Context) {
	r.Tick(ctx)
	t := time.NewTicker(r.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.Tick(ctx)
		}
	}
}

// Tick does one pass over every commune.
func (r *AutomationRunner) Tick(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	names := make([]string, len(automationJobs))
	for i, j := range automationJobs {
		names[i] = j.name
	}
	held, release, err := r.d.Locks.TryLockJobs(ctx, names)
	if err != nil {
		r.d.Log.WarnContext(ctx, "CẢNH BÁO: không thử được khoá việc nền, bỏ nhịp này", "service", "petitions", "err", err)
		return
	}
	defer release()

	var scopes []*identityv1.AutomationRunScope
	for _, j := range automationJobs {
		if !held[j.name] {
			continue // another replica runs this job this tick
		}
		for _, k := range automationKinds {
			scopes = append(scopes, &identityv1.AutomationRunScope{Job: j.job, WorkKind: k})
		}
	}
	if len(scopes) == 0 {
		return
	}

	communes, err := r.d.Communes.CommunesWithRecords(ctx)
	if err != nil {
		r.d.Log.WarnContext(ctx, "CẢNH BÁO: không liệt kê được xã cho việc nền, bỏ nhịp này", "service", "petitions", "err", err)
		return
	}
	for _, id := range communes {
		if ctx.Err() != nil {
			return
		}
		r.runCommune(ctx, id, scopes)
	}
}

// runCommune claims and runs the due scopes of one commune. Nothing it does can stop the next one.
func (r *AutomationRunner) runCommune(ctx context.Context, id tenant.ID, scopes []*identityv1.AutomationRunScope) {
	defer func() {
		if p := recover(); p != nil {
			r.d.Log.ErrorContext(ctx, "LỖI: việc nền của một xã bị panic, chuyển sang xã kế", "service", "petitions",
				"xa", string(id), "panic", fmt.Sprint(p))
		}
	}()
	cctx := tenant.Into(ctx, id)

	// A merged commune is inactive and keeps its data (rule 7, invariant 6): nothing runs for it.
	// Unknown, or platform not answering: skipped this tick — never assumed active.
	t, ok, err := r.d.Registry.XaTrongNguCanh(cctx)
	if err != nil {
		r.d.Log.WarnContext(cctx, "CẢNH BÁO: không đọc được trạng thái xã, bỏ xã này ở nhịp này", "service", "petitions",
			"xa", string(id), "err", err)
		return
	}
	if !ok || !t.Active {
		return
	}

	// NEVER RETRIED: a retry after a lost reply finds the slot claimed and answers absent (contract).
	runs, err := r.d.Identity.ClaimDueAutomationRuns(cctx, scopes)
	if err != nil {
		r.d.Log.WarnContext(cctx, "CẢNH BÁO: không nhận được lượt chạy, thử ở nhịp kế", "service", "petitions",
			"xa", string(id), "err", err)
		return
	}
	for _, run := range runs {
		o := r.execute(cctx, run)
		r.record(cctx, id, run, o)
	}
}

// execute runs one claimed run and says how it ended. A panic is a FAILED run, never an unrecorded one.
func (r *AutomationRunner) execute(ctx context.Context, run identityclient.AutomationRun) (out identityclient.AutomationOutcome) {
	defer func() {
		if p := recover(); p != nil {
			r.d.Log.ErrorContext(ctx, "LỖI: lượt việc nền bị panic", "service", "petitions", "run_id", run.RunID,
				"panic", fmt.Sprint(p))
			out.Outcome = identityv1.AutomationRunOutcome_AUTOMATION_RUN_OUTCOME_FAILED
		}
	}()

	kind, ok := automationWorkKind(run.WorkKind)
	if !ok || run.ClaimedAt.IsZero() {
		out.Outcome = identityv1.AutomationRunOutcome_AUTOMATION_RUN_OUTCOME_FAILED
		return out
	}
	var (
		p   automationPlan
		err error
	)
	switch run.Job {
	case identityv1.AutomationJob_AUTOMATION_JOB_SLA_REMINDERS:
		p, err = r.slaReminders(ctx, kind, run.ClaimedAt)
	case identityv1.AutomationJob_AUTOMATION_JOB_ESCALATION:
		p, err = r.escalation(ctx, kind, run.ClaimedAt)
	case identityv1.AutomationJob_AUTOMATION_JOB_WEEKLY_DIGEST:
		p, err = r.weeklyDigest(ctx, kind, run.ClaimedAt)
	default:
		err = fmt.Errorf("việc nền không biết: %v", run.Job)
	}
	out.RecordsExamined = uint32(p.examined)
	out.RecordsWithoutRecipient = uint32(p.withoutRecipientCount())
	if err != nil {
		// NOTHING IS DELIVERED FROM A HALF-BUILT PLAN: the next run redoes it under the same keys.
		out.Outcome = automationOutcomeFor(err)
		r.d.Log.WarnContext(ctx, "CẢNH BÁO: lượt việc nền dừng giữa chừng", "service", "petitions",
			"run_id", run.RunID, "err", err)
		return out
	}
	created, err := r.deliver(ctx, p.notices)
	out.NoticesDelivered = created
	if err != nil {
		out.Outcome = automationOutcomeFor(err)
		r.d.Log.WarnContext(ctx, "CẢNH BÁO: không giao hết thông báo của lượt việc nền", "service", "petitions",
			"run_id", run.RunID, "err", err)
		return out
	}
	if p.configMissing {
		out.Outcome = identityv1.AutomationRunOutcome_AUTOMATION_RUN_OUTCOME_CONFIGURATION_MISSING
	} else {
		out.Outcome = identityv1.AutomationRunOutcome_AUTOMATION_RUN_OUTCOME_SUCCEEDED
	}
	return out
}

// record writes the outcome. Retried on an outage (first write wins on the server), on a context
// that outlives a shutdown, so a run interrupted by SIGTERM is still recorded rather than left as a
// lease with no result.
func (r *AutomationRunner) record(ctx context.Context, id tenant.ID, run identityclient.AutomationRun,
	o identityclient.AutomationOutcome) {

	rctx := context.WithoutCancel(ctx)
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if err = r.d.Identity.RecordAutomationRunOutcome(rctx, run.RunID, o); err == nil ||
			!errors.Is(err, identityclient.ErrIdentityUnavailable) {
			break
		}
		time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
	}
	if err != nil {
		r.d.Log.WarnContext(ctx, "CẢNH BÁO: không ghi được kết quả lượt việc nền", "service", "petitions",
			"xa", string(id), "run_id", run.RunID, "err", err)
		return
	}
	// Counts and codes only (rule 3).
	r.d.Log.InfoContext(ctx, "lượt việc nền xong", "service", "petitions", "xa", string(id), "run_id", run.RunID,
		"viec", run.Job.String(), "loai_viec", run.WorkKind.String(), "ket_qua", o.Outcome.String(),
		"so_ho_so", o.RecordsExamined, "so_thong_bao", o.NoticesDelivered, "khong_nguoi_nhan", o.RecordsWithoutRecipient)
}

// errAutomationStore marks a failure of THIS service's own database — a dependency (the outcome
// enum lists "the runner's own database" under DEPENDENCY_UNAVAILABLE).
var errAutomationStore = errors.New("việc nền: không đọc được sổ của petitions")

// automationOutcomeFor names who acts next for a run that stopped.
func automationOutcomeFor(err error) identityv1.AutomationRunOutcome {
	switch {
	case errors.Is(err, identityclient.ErrIdentityUnavailable), errors.Is(err, commsclient.ErrCommsUnavailable),
		errors.Is(err, errAutomationStore), errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return identityv1.AutomationRunOutcome_AUTOMATION_RUN_OUTCOME_DEPENDENCY_UNAVAILABLE
	default:
		return identityv1.AutomationRunOutcome_AUTOMATION_RUN_OUTCOME_FAILED
	}
}

func automationWorkKind(k identityv1.WorkKind) (domain.AutomationWorkKind, bool) {
	switch k {
	case identityv1.WorkKind_WORK_KIND_NHIEM_VU:
		return domain.AutomationTask, true
	case identityv1.WorkKind_WORK_KIND_PHAN_ANH:
		return domain.AutomationCitizenReport, true
	}
	return "", false
}

func identityWorkKind(k domain.AutomationWorkKind) identityv1.WorkKind {
	if k == domain.AutomationCitizenReport {
		return identityv1.WorkKind_WORK_KIND_PHAN_ANH
	}
	return identityv1.WorkKind_WORK_KIND_NHIEM_VU
}
