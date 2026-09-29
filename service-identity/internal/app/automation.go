package app

// The use cases behind Cấu hình → Tự động hoá (docs/ui-ux/14-cau-hinh.md §9, ADR 0058): the three
// staff operations (overview, save a job's settings, request a run now) and the two runner
// operations (claim due runs, record an outcome) served over gRPC.
//
// WHY THIS LAYER: rule 6, invariant 3 — every write that is a business act shares one transaction
// with its audit entry, and core/audit.Write takes the *store.ScopedTx this layer opens.
//
// WHAT IS AUDITED AND WHAT IS NOT, stated because the contract decides it:
//
//	save settings        audited, actor = the staff code (rule 6, invariant 8)
//	request a run now    audited, actor = the staff code
//	claim a run          NOT audited — a lease written up to every minute per scope; the contract
//	                     (ClaimDueAutomationRuns §WHAT THE CLAIM WRITES) says an entry per lease would
//	                     bury the entries that carry legal weight
//	record an outcome    audited ONCE per run, actor = core/audit.SystemActor (rule 6, invariant 6:
//	                     background jobs are system actions)

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// AutomationRepo is the store, declared at the point of use. Every mutating method takes the
// transaction, so no write can land outside the one its audit entry shares. The three reads without a
// transaction are scoped to the commune by the store (store.DB.For(ctx)).
type AutomationRepo interface {
	Settings(ctx context.Context) ([]domain.AutomationSetting, error)
	LastRuns(ctx context.Context) ([]domain.AutomationRun, error)
	ScopeStates(ctx context.Context) (map[domain.AutomationScope]domain.ScopeState, error)

	SettingForUpdate(ctx context.Context, tx *store.ScopedTx, job domain.AutomationJob) (domain.AutomationSetting, bool, error)
	InsertSetting(ctx context.Context, tx *store.ScopedTx, s domain.AutomationSetting) error
	UpdateSetting(ctx context.Context, tx *store.ScopedTx, s domain.AutomationSetting) error
	MarkRunRequested(ctx context.Context, tx *store.ScopedTx, job domain.AutomationJob, at time.Time, by string) error

	ClaimScope(ctx context.Context, tx *store.ScopedTx, prevRunID string, run domain.AutomationRun) (bool, error)
	InsertRun(ctx context.Context, tx *store.ScopedTx, run domain.AutomationRun) error
	RunForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (domain.AutomationRun, error)
	RecordRun(ctx context.Context, tx *store.ScopedTx, run domain.AutomationRun) error
}

// The business verbs written into the trail. English constant names, Vietnamese values — the
// convention of role_template.go (`gieo_vai_tro_mau`): an inspection reads these strings.
const (
	ActionUpdateAutomationJob  = "sua_cau_hinh_tu_dong_hoa"
	ActionRequestAutomationRun = "yeu_cau_chay_ngay_tu_dong_hoa"
	ActionRecordAutomationRun  = "ghi_ket_qua_luot_chay_tu_dong"
)

// The refusals this layer owns. The handler maps them to status codes.
var (
	// ErrAutomationJobUnknown — a job key that is not one of the three. 404 on the REST surface.
	ErrAutomationJobUnknown = errors.New("tự động hoá: không có việc này")

	// ErrAutomationJobDisabled — "run now" on a job that is off or was never saved. NO ROW MEANS OFF,
	// and a run request is not a way round a switch the commune left off (ADR 0058 stop condition #3).
	ErrAutomationJobDisabled = errors.New("tự động hoá: việc đang tắt — bật việc trước khi yêu cầu chạy ngay")

	// ErrAutomationEnabledMissing — PUT without `enabled`. There is no default switch position.
	ErrAutomationEnabledMissing = errors.New("tự động hoá: thiếu trạng thái bật/tắt")

	// ErrAutomationFieldMissing / ErrAutomationFieldNotApplicable — a cadence field of the job's kind is
	// absent, or a field of another kind is present. Both are refused rather than defaulted or ignored:
	// a silently dropped `weekday` on a daily job is a setting the screen shows and nothing uses.
	ErrAutomationFieldMissing       = errors.New("tự động hoá: thiếu nhịp chạy cho việc này")
	ErrAutomationFieldNotApplicable = errors.New("tự động hoá: có trường nhịp chạy không dùng cho việc này")

	// ErrRunReportInvalid — an outcome that is not one of the four, or a negative count.
	ErrRunReportInvalid = errors.New("tự động hoá: kết quả lượt chạy không hợp lệ")
)

// IsAutomationInputError reports whether err is a refusal of what the client sent (400), as opposed
// to a failure. Listed, never defaulted — the discipline of LaLoiDauVaoSLA.
func IsAutomationInputError(err error) bool {
	for _, e := range []error{ErrAutomationEnabledMissing, ErrAutomationFieldMissing,
		ErrAutomationFieldNotApplicable, domain.ErrIntervalOutOfRange, domain.ErrHourOutOfRange,
		domain.ErrMinuteOutOfRange, domain.ErrWeekdayOutOfRange} {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}

// IsRunNotFound reports the contract's NOT_FOUND case, so the gRPC layer need not import the store.
func IsRunNotFound(err error) bool { return errors.Is(err, idstore.ErrAutomationRunNotFound) }

// Automation owns the settings and run state of one commune's background jobs.
type Automation struct {
	db   *store.DB
	repo AutomationRepo

	// now is IDENTITY'S clock — the contract's "claimed_at comes from this service". Injected so tests
	// pin it. Truncated to microseconds, so the value handed back is the value PostgreSQL keeps.
	now   func() time.Time
	newID func() (string, error)
}

func NewAutomation(db *store.DB, repo AutomationRepo) *Automation {
	return &Automation{db: db, repo: repo, now: time.Now, newID: ulid.Moi}
}

func (uc *Automation) clock() time.Time { return uc.now().UTC().Truncate(time.Microsecond) }

// AutomationJobView is one job as the configuration screen shows it.
type AutomationJobView struct {
	// Setting is the saved row, or — when Configured is false — the FORM PREFILL
	// (domain.SuggestedAutomationSetting), switched off. Never read by a claim.
	Setting    domain.AutomationSetting
	Configured bool

	// LastRuns is the latest run of every scope of this job that has ever been claimed, one per kind
	// of work — a job runs in several services, so there is no single "last run".
	LastRuns []domain.AutomationRun
}

// Overview returns the three jobs of the commune in the context, in the screen's order.
func (uc *Automation) Overview(ctx context.Context) ([]AutomationJobView, error) {
	settings, err := uc.repo.Settings(ctx)
	if err != nil {
		return nil, err
	}
	runs, err := uc.repo.LastRuns(ctx)
	if err != nil {
		return nil, err
	}
	saved := make(map[domain.AutomationJob]domain.AutomationSetting, len(settings))
	for _, s := range settings {
		saved[s.Job] = s
	}
	out := make([]AutomationJobView, 0, 3)
	for _, j := range domain.AutomationJobs() {
		v := AutomationJobView{Setting: domain.SuggestedAutomationSetting(j), LastRuns: []domain.AutomationRun{}}
		if s, ok := saved[j]; ok {
			v.Setting, v.Configured = s, true
		}
		for _, r := range runs {
			if r.Scope.Job == j {
				v.LastRuns = append(v.LastRuns, r)
			}
		}
		out = append(out, v)
	}
	return out, nil
}

// jobView is Overview narrowed to one job — the body the two write routes answer with.
func (uc *Automation) jobView(ctx context.Context, job domain.AutomationJob) (AutomationJobView, error) {
	views, err := uc.Overview(ctx)
	if err != nil {
		return AutomationJobView{}, err
	}
	for _, v := range views {
		if v.Setting.Job == job {
			return v, nil
		}
	}
	return AutomationJobView{}, ErrAutomationJobUnknown
}

// AutomationSettingRequest is a PUT body: the switch, and the cadence fields of the job's kind.
// Pointers, so "absent" is distinguishable from 0 — 0 is a real hour and a real minute.
type AutomationSettingRequest struct {
	Enabled         *bool
	IntervalMinutes *int
	RunHour         *int
	RunMinute       *int
	Weekday         *int
}

// buildSetting applies a request to the job's current setting, refusing missing and foreign fields.
func buildSetting(job domain.AutomationJob, base domain.AutomationSetting, req AutomationSettingRequest) (domain.AutomationSetting, error) {
	if req.Enabled == nil {
		return domain.AutomationSetting{}, ErrAutomationEnabledMissing
	}
	next := base
	next.Job, next.Enabled = job, *req.Enabled
	next.IntervalMinutes, next.RunHour, next.RunMinute, next.Weekday = 0, 0, 0, 0

	var need, foreign []*int
	switch job.Schedule() {
	case domain.ScheduleInterval:
		need = []*int{req.IntervalMinutes}
		foreign = []*int{req.RunHour, req.RunMinute, req.Weekday}
	case domain.ScheduleDaily:
		need = []*int{req.RunHour, req.RunMinute}
		foreign = []*int{req.IntervalMinutes, req.Weekday}
	case domain.ScheduleWeekly:
		need = []*int{req.RunHour, req.RunMinute, req.Weekday}
		foreign = []*int{req.IntervalMinutes}
	default:
		return domain.AutomationSetting{}, ErrAutomationJobUnknown
	}
	for _, p := range need {
		if p == nil {
			return domain.AutomationSetting{}, ErrAutomationFieldMissing
		}
	}
	for _, p := range foreign {
		if p != nil {
			return domain.AutomationSetting{}, ErrAutomationFieldNotApplicable
		}
	}
	if p := req.IntervalMinutes; p != nil {
		next.IntervalMinutes = *p
	}
	if p := req.RunHour; p != nil {
		next.RunHour = *p
	}
	if p := req.RunMinute; p != nil {
		next.RunMinute = *p
	}
	if p := req.Weekday; p != nil {
		next.Weekday = *p
	}
	return next, domain.ValidateAutomationSetting(next)
}

// sameChoice compares what a person chose, ignoring bookkeeping (who, when, the run-now mark).
func sameChoice(a, b domain.AutomationSetting) bool {
	return a.Job == b.Job && a.Enabled == b.Enabled && a.IntervalMinutes == b.IntervalMinutes &&
		a.RunHour == b.RunHour && a.RunMinute == b.RunMinute && a.Weekday == b.Weekday
}

// SaveSetting saves one job's settings (PUT). A save that changes nothing writes nothing and audits
// nothing — which is also what makes the route idempotent.
//
// SWITCHING ON STAMPS `enabled_at` = now, and only an off→on change does: that instant is what stops a
// slot opened earlier today from counting (contract: "enabling escalation at 15:00 runs it tomorrow").
// A cadence change on an enabled job keeps the old instant, so it applies from the next slot.
func (uc *Automation) SaveSetting(ctx context.Context, job domain.AutomationJob, req AutomationSettingRequest,
	actor NguoiThucHien) (AutomationJobView, error) {

	if !job.Valid() {
		return AutomationJobView{}, ErrAutomationJobUnknown
	}
	if err := actor.hopLe(); err != nil {
		return AutomationJobView{}, err
	}
	// Caller faults before any transaction: a misshapen body must not take a row lock.
	if _, err := buildSetting(job, domain.AutomationSetting{}, req); err != nil {
		return AutomationJobView{}, err
	}

	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		prev, exists, err := uc.repo.SettingForUpdate(ctx, tx, job)
		if err != nil {
			return err
		}
		next, err := buildSetting(job, prev, req)
		if err != nil {
			return err
		}
		if exists && sameChoice(prev, next) {
			return nil
		}
		now := uc.clock()
		if next.Enabled && (!exists || !prev.Enabled) {
			next.EnabledAt = now
		}
		next.UpdatedAt, next.UpdatedBy = now, actor.Vet.ID

		if exists {
			err = uc.repo.UpdateSetting(ctx, tx, next)
		} else {
			err = uc.repo.InsertSetting(ctx, tx, next)
		}
		if err != nil {
			return err
		}
		var before any
		if exists {
			before = settingDelta(prev)
		}
		// SAME TRANSACTION AS THE WRITE (rule 6, invariant 3). TenantID is filled from the tx.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor.Vet,
			Action:  ActionUpdateAutomationJob,
			Subject: automationSubject(job),
			Delta:   automationDelta(map[string]any{"truoc": before, "sau": settingDelta(next)}),
		})
	})
	if err != nil {
		return AutomationJobView{}, err
	}
	return uc.jobView(ctx, job)
}

// RequestRun marks a job "run requested" (POST …/runs). Each scope of the job then claims it once at
// its next tick, regardless of the schedule (ADR 0058 §7; domain.DueTrigger).
//
// A SECOND REQUEST BEFORE THE NEXT TICK MOVES THE MARK AND STILL PRODUCES ONE RUN PER SCOPE — the mark
// is compared with each scope's last claim, not counted. It is audited each time: each press is an act
// somebody performed.
func (uc *Automation) RequestRun(ctx context.Context, job domain.AutomationJob, actor NguoiThucHien) (AutomationJobView, error) {
	if !job.Valid() {
		return AutomationJobView{}, ErrAutomationJobUnknown
	}
	if err := actor.hopLe(); err != nil {
		return AutomationJobView{}, err
	}
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		cur, exists, err := uc.repo.SettingForUpdate(ctx, tx, job)
		if err != nil {
			return err
		}
		if !exists || !cur.Enabled {
			return ErrAutomationJobDisabled
		}
		now := uc.clock()
		if err := uc.repo.MarkRunRequested(ctx, tx, job, now, actor.Vet.ID); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor.Vet,
			Action:  ActionRequestAutomationRun,
			Subject: automationSubject(job),
			Delta:   automationDelta(map[string]any{"yeu_cau_luc": now.Format(time.RFC3339)}),
		})
	})
	if err != nil {
		return AutomationJobView{}, err
	}
	return uc.jobView(ctx, job)
}

// ClaimDue claims, for the commune in the context, every requested scope that is due now, and returns
// the runs this call won. Absent = not enabled, not due, or won by another caller (the contract's
// three indistinguishable reasons).
//
// CHEAP ENOUGH FOR A ONE-MINUTE TICK: two small scoped SELECTs (settings ≤ 3 rows, leases ≤ 12) when
// nothing is due, and one short transaction per due scope. The decision is made on those reads OUTSIDE
// a transaction; correctness comes from the compare-and-set in ClaimScope, which refuses if the lease
// moved after the read — so two replicas that both decide "due" produce one winner.
//
// NOW IS IDENTITY'S CLOCK, taken once per call: every run of one call shares one `claimed_at`.
func (uc *Automation) ClaimDue(ctx context.Context, scopes []domain.AutomationScope) ([]domain.AutomationRun, error) {
	zone, err := domain.MuiGio()
	if err != nil {
		return nil, err
	}
	settings, err := uc.repo.Settings(ctx)
	if err != nil {
		return nil, err
	}
	states, err := uc.repo.ScopeStates(ctx)
	if err != nil {
		return nil, err
	}
	byJob := make(map[domain.AutomationJob]domain.AutomationSetting, len(settings))
	for _, s := range settings {
		byJob[s.Job] = s
	}

	now := uc.clock()
	var won []domain.AutomationRun
	for _, sc := range scopes {
		setting, ok := byJob[sc.Job]
		if !ok {
			continue // NO ROW MEANS OFF
		}
		st := states[sc]
		trigger, due := domain.DueTrigger(setting, st, now, zone)
		if !due {
			continue
		}
		id, err := uc.newID()
		if err != nil {
			return nil, fmt.Errorf("tự động hoá: sinh mã lượt chạy: %w", err)
		}
		run := domain.AutomationRun{ID: id, Scope: sc, Trigger: trigger, ClaimedAt: now}
		var claimed bool
		err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
			ok, err := uc.repo.ClaimScope(ctx, tx, st.LastRunID, run)
			if err != nil || !ok {
				return err
			}
			claimed = true
			return uc.repo.InsertRun(ctx, tx, run)
		})
		if err != nil {
			return nil, err
		}
		if claimed {
			won = append(won, run)
		}
	}
	return won, nil
}

// RecordOutcome writes the result of one claimed run and its ONE audit entry, in one transaction.
// FIRST WRITE WINS: a second call for the same run changes nothing and answers success.
func (uc *Automation) RecordOutcome(ctx context.Context, runID string, rep domain.RunReport) error {
	if !rep.Outcome.Valid() || rep.RecordsExamined < 0 || rep.NoticesDelivered < 0 || rep.RecordsWithoutRecipient < 0 {
		return ErrRunReportInvalid
	}
	return uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		run, err := uc.repo.RunForUpdate(ctx, tx, runID)
		if err != nil {
			return err // idstore.ErrAutomationRunNotFound included — NOT_FOUND at the contract
		}
		if run.Recorded() {
			return nil
		}
		run.Report, run.RecordedAt = rep, uc.clock()
		if err := uc.repo.RecordRun(ctx, tx, run); err != nil {
			return err
		}
		// THE SYSTEM PRINCIPAL (core/audit.SystemActor, audit.go:35-37): a background job is a system
		// action (rule 6, invariant 6). No IP — there is no request from a person to observe one from.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   audit.Actor{ID: audit.SystemActor, Kind: "system"},
			Action:  ActionRecordAutomationRun,
			Subject: automationSubject(run.Scope.Job) + "/" + string(run.Scope.WorkKind),
			Delta: automationDelta(map[string]any{
				"run_id":                    run.ID,
				"trigger":                   string(run.Trigger),
				"claimed_at":                run.ClaimedAt.Format(time.RFC3339),
				"outcome":                   string(rep.Outcome),
				"records_examined":          rep.RecordsExamined,
				"notices_delivered":         rep.NoticesDelivered,
				"records_without_recipient": rep.RecordsWithoutRecipient,
			}),
		})
	})
}

// automationSubject names what was configured: `tu-dong-hoa/<job>`. The job key is the software's own
// identifier, the same string the screen and the contract use; no row id names anything years later.
func automationSubject(job domain.AutomationJob) string { return "tu-dong-hoa/" + string(job) }

// settingDelta is the before/after payload of a settings save: what a person chose, nothing else.
// Nothing here is personal data (rule 3).
func settingDelta(s domain.AutomationSetting) map[string]any {
	d := map[string]any{"enabled": s.Enabled}
	switch s.Job.Schedule() {
	case domain.ScheduleInterval:
		d["interval_minutes"] = s.IntervalMinutes
	case domain.ScheduleDaily:
		d["run_hour"], d["run_minute"] = s.RunHour, s.RunMinute
	case domain.ScheduleWeekly:
		d["weekday"], d["run_hour"], d["run_minute"] = s.Weekday, s.RunHour, s.RunMinute
	}
	return d
}

// automationDelta marshals an audit payload; a failure yields an explicit marker, never a nil delta
// (the discipline of deltaSLA).
func automationDelta(v map[string]any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{"loi":"khong_dung_duoc_delta"}`)
	}
	return b
}
