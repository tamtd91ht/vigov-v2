package domain

import (
	"errors"
	"time"
)

// The background jobs of Cấu hình → Tự động hoá (docs/ui-ux/14-cau-hinh.md §9, ADR 0058) — their
// per-commune settings and the ONE implementation of "is this scope due now".
//
// WHY THE SCHEDULE LIVES HERE AND NOWHERE ELSE: the jobs run in two services (petitions, documents),
// each with several replicas. The contract (ClaimDueAutomationRuns) keeps the schedule inside identity
// so there is one implementation of the edge cases — the minute a slot opens, a restart inside it, the
// day a job is switched on — instead of one per runner. Nothing here reads a clock, a database or the
// process zone: `now` and the zone are arguments, which is what makes every case testable.

// AutomationJob is the software key of one job. English values, following the contract's
// AutomationJob comment: keys of the software's jobs, identical in every commune, not values written
// into an archival record. FOUR: the user's decision of 2026-09-29 built three (`refresh_dashboards`
// dropped — ADR 0058 §4), and the user un-deferred `send_scheduled_reports` on 2026-10-09 as
// `scheduled_reports` (ADR 0086 B).
type AutomationJob string

const (
	JobSLAReminders     AutomationJob = "sla_reminders"
	JobEscalation       AutomationJob = "escalation"
	JobWeeklyDigest     AutomationJob = "weekly_digest"
	JobScheduledReports AutomationJob = "scheduled_reports"
)

// AutomationJobs lists the jobs in the order the configuration screen shows them (§9's order).
// A fresh slice per call: a package-level one would be mutable by any caller.
func AutomationJobs() []AutomationJob {
	return []AutomationJob{JobSLAReminders, JobEscalation, JobWeeklyDigest, JobScheduledReports}
}

// Valid reports whether j is one of the four keys. A caller meeting false refuses — an unknown job
// is not "off", it is a request for something the software does not run.
func (j AutomationJob) Valid() bool {
	switch j {
	case JobSLAReminders, JobEscalation, JobWeeklyDigest, JobScheduledReports:
		return true
	}
	return false
}

// ScheduleKind is how a job repeats, and so which cadence fields its setting carries.
type ScheduleKind string

const (
	ScheduleInterval ScheduleKind = "interval" // every N minutes
	ScheduleDaily    ScheduleKind = "daily"    // once a day at HH:MM
	ScheduleWeekly   ScheduleKind = "weekly"   // once a week, on a weekday, at HH:MM

	// ScheduleMonthlyAndWeekly — on a weekday at HH:MM AND on the 1st of every month at the same HH:MM;
	// one slot on a day that is both. The reference system's own kind name (automation.py:95). Same
	// cadence fields as ScheduleWeekly; only the slots differ (LatestSlot).
	ScheduleMonthlyAndWeekly ScheduleKind = "monthly_and_weekly"
)

// HasWeekday reports whether the kind's cadence carries a weekday (and HH:MM) — the shape the two
// weekday kinds share, which `automation_job_setting_shape` pins (migration 0029).
func (k ScheduleKind) HasWeekday() bool {
	return k == ScheduleWeekly || k == ScheduleMonthlyAndWeekly
}

// Schedule returns the job's one schedule kind — FIXED BY THE JOB, never chosen by the commune
// (contract: "ONE SCHEDULE PER JOB, fixed by the job, set by the commune"). "" for an unknown job.
func (j AutomationJob) Schedule() ScheduleKind {
	switch j {
	case JobSLAReminders:
		return ScheduleInterval
	case JobEscalation:
		return ScheduleDaily
	case JobWeeklyDigest:
		return ScheduleWeekly
	case JobScheduledReports:
		return ScheduleMonthlyAndWeekly
	}
	return ""
}

// The cadence bounds. The minimum is §9's own (../vigov-require automation.py:57-58, `min_interval=5`);
// the maximum is a week, the reference system's input bound (schemas.py:216), past which "every N
// minutes" has stopped describing a reminder.
const (
	MinIntervalMinutes = 5
	MaxIntervalMinutes = 10080
)

// The refusals of a setting. Sentences a person reads on the configuration screen; the handler
// returns them as they are.
var (
	ErrIntervalOutOfRange = errors.New("tự động hoá: nhịp nhắc việc phải từ 5 đến 10080 phút")
	ErrHourOutOfRange     = errors.New("tự động hoá: giờ chạy phải từ 0 đến 23")
	ErrMinuteOutOfRange   = errors.New("tự động hoá: phút chạy phải từ 0 đến 59")
	ErrWeekdayOutOfRange  = errors.New("tự động hoá: thứ trong tuần phải từ 1 (thứ Hai) đến 7 (Chủ nhật)")
)

// AutomationSetting is one commune's choice for one job — one row of `automation_job_setting`
// (migration 0017). NO ROW MEANS OFF; there is no default setting anywhere on the claim path.
type AutomationSetting struct {
	Job     AutomationJob
	Enabled bool

	// The cadence. Only the fields of the job's ScheduleKind are meaningful; the others are zero and
	// the store writes them as NULL (the table's shape CHECK refuses anything else).
	IntervalMinutes int // ScheduleInterval
	RunHour         int // every kind but ScheduleInterval — local hour in Asia/Ho_Chi_Minh
	RunMinute       int // every kind but ScheduleInterval
	Weekday         int // ScheduleKind.HasWeekday — ISO 1 = Monday … 7 = Sunday, like `lich_lam_viec.thu`

	// EnabledAt is when the job was last switched ON. No slot opened before it counts. Zero = never.
	EnabledAt time.Time

	// RunRequestedAt is the "run now" mark (ADR 0058 §7): due for every scope whose last claim is
	// older than it. Zero = never requested. RunRequestedBy is the staff code that pressed it.
	RunRequestedAt time.Time
	RunRequestedBy string

	UpdatedAt time.Time
	UpdatedBy string // staff business code (rule 6, invariant 8)
}

// ValidateAutomationSetting refuses a cadence outside its bounds, for the fields of the job's kind.
// Kept even while the job is OFF: the cadence is kept for the next switch-on and must be usable then.
func ValidateAutomationSetting(s AutomationSetting) error {
	switch s.Job.Schedule() {
	case ScheduleInterval:
		if s.IntervalMinutes < MinIntervalMinutes || s.IntervalMinutes > MaxIntervalMinutes {
			return ErrIntervalOutOfRange
		}
	case ScheduleDaily, ScheduleWeekly, ScheduleMonthlyAndWeekly:
		if s.RunHour < 0 || s.RunHour > 23 {
			return ErrHourOutOfRange
		}
		if s.RunMinute < 0 || s.RunMinute > 59 {
			return ErrMinuteOutOfRange
		}
		if s.Job.Schedule().HasWeekday() && (s.Weekday < 1 || s.Weekday > 7) {
			return ErrWeekdayOutOfRange
		}
	default:
		return errors.New("tự động hoá: việc không có trong danh sách")
	}
	return nil
}

// SuggestedAutomationSetting is the FORM PREFILL for a job with no row: switched off, with the
// user's suggested cadence (2026-09-29) — 15 minutes; daily 07:00; Monday 07:30 — and, for
// `scheduled_reports`, the reference system's Monday 07:45 (automation.py:96-98; ADR 0086 "theo
// prototype").
//
// IT IS NEVER READ BY A CLAIM. A job with no row is off, and DueTrigger is only ever handed a row the
// commune saved. Returning these numbers on GET is what lets the screen show a cadence to switch on
// with; a claim path reading them would be a default schedule, which the contract forbids.
func SuggestedAutomationSetting(j AutomationJob) AutomationSetting {
	s := AutomationSetting{Job: j}
	switch j {
	case JobSLAReminders:
		s.IntervalMinutes = 15
	case JobEscalation:
		s.RunHour, s.RunMinute = 7, 0
	case JobWeeklyDigest:
		s.Weekday, s.RunHour, s.RunMinute = 1, 7, 30
	case JobScheduledReports:
		s.Weekday, s.RunHour, s.RunMinute = 1, 7, 45
	}
	return s
}

// AutomationScope is the unit of run state: one job over one kind of work (contract:
// AutomationRunScope). One job runs in two services, so run state cannot be per job alone.
type AutomationScope struct {
	Job      AutomationJob
	WorkKind LoaiViec
}

// ScopeState is the lease of one scope: the last run claimed and when. Zero = never claimed.
type ScopeState struct {
	LastRunID     string
	LastClaimedAt time.Time
}

// RunTrigger names what made a scope due.
type RunTrigger string

const (
	TriggerSchedule RunTrigger = "schedule"
	TriggerRequest  RunTrigger = "request" // "run now"
)

// DueTrigger answers whether one scope is due at `now`, and why. THE ONE IMPLEMENTATION of the
// contract's "WHAT DUE MEANS":
//
//	off, or never switched on                        never due — NO ROW / OFF MEANS OFF
//	a "run now" mark later than the scope's last     due (TriggerRequest), regardless of the schedule —
//	claim, not before the last switch-on, not after  each scope consumes the one job-level mark once,
//	`now`                                            by its own claim moving past it
//	the latest scheduled slot at or before `now`     due (TriggerSchedule) when it opened after the
//	                                                 scope's last claim AND not before the switch-on —
//	                                                 so a missed slot runs ONCE, and a slot that opened
//	                                                 before the job was switched on never counts
//
// A mark AFTER `now` is not due yet: claimed at `now`, the claim would be older than the mark and the
// scope would run a second time on the next tick.
func DueTrigger(s AutomationSetting, st ScopeState, now time.Time, zone *time.Location) (RunTrigger, bool) {
	if !s.Enabled || s.EnabledAt.IsZero() || zone == nil {
		return "", false
	}
	if req := s.RunRequestedAt; !req.IsZero() && !req.Before(s.EnabledAt) && !req.After(now) &&
		(st.LastClaimedAt.IsZero() || req.After(st.LastClaimedAt)) {
		return TriggerRequest, true
	}
	slot, ok := LatestSlot(s, st, now, zone)
	if !ok || slot.Before(s.EnabledAt) {
		return "", false
	}
	if !st.LastClaimedAt.IsZero() && !slot.After(st.LastClaimedAt) {
		return "", false
	}
	return TriggerSchedule, true
}

// LatestSlot is the latest scheduled instant at or before `now`, in the commune's zone. False when no
// slot has opened yet (an interval job whose first interval has not elapsed).
//
//	interval  counted from the later of the switch-on and the scope's last claim — the reference
//	          system's rule (scheduler.py:65-69), so a changed interval applies from the next tick and a
//	          run-now claim restarts the count
//	daily     today at HH:MM, or yesterday's when today's has not come
//	weekly    the most recent weekday at HH:MM, looking back at most a week
//	monthly_and_weekly
//	          the most recent day that is the weekday OR the 1st, at HH:MM — at most a week back, since
//	          the weekday recurs within seven days. ONE SLOT PER DAY: both rules name the same HH:MM, so
//	          a 1st that is also the weekday is one instant, not two
//
// Built with time.Date in the zone, never by truncating an instant: Truncate works on UTC and would
// put a 07:00 slot at 14:00 local.
func LatestSlot(s AutomationSetting, st ScopeState, now time.Time, zone *time.Location) (time.Time, bool) {
	switch s.Job.Schedule() {
	case ScheduleInterval:
		if s.IntervalMinutes < MinIntervalMinutes {
			return time.Time{}, false
		}
		base := s.EnabledAt
		if st.LastClaimedAt.After(base) {
			base = st.LastClaimedAt
		}
		if base.IsZero() {
			return time.Time{}, false
		}
		slot := base.Add(time.Duration(s.IntervalMinutes) * time.Minute)
		if slot.After(now) {
			return time.Time{}, false
		}
		return slot, true

	case ScheduleDaily:
		l := now.In(zone)
		for back := 0; back <= 1; back++ {
			slot := time.Date(l.Year(), l.Month(), l.Day()-back, s.RunHour, s.RunMinute, 0, 0, zone)
			if !slot.After(now) {
				return slot, true
			}
		}

	case ScheduleWeekly:
		l := now.In(zone)
		for back := 0; back <= 7; back++ {
			slot := time.Date(l.Year(), l.Month(), l.Day()-back, s.RunHour, s.RunMinute, 0, 0, zone)
			if thuISO(slot) == s.Weekday && !slot.After(now) {
				return slot, true
			}
		}

	case ScheduleMonthlyAndWeekly:
		l := now.In(zone)
		for back := 0; back <= 7; back++ {
			slot := time.Date(l.Year(), l.Month(), l.Day()-back, s.RunHour, s.RunMinute, 0, 0, zone)
			if (slot.Day() == 1 || thuISO(slot) == s.Weekday) && !slot.After(now) {
				return slot, true
			}
		}
	}
	return time.Time{}, false
}

// ReportPeriod is which period a `scheduled_reports` run reports (contract: ScheduledReportPeriod),
// stored in `automation_run.scheduled_report_period` (migration 0029). "" on every other job. Only
// WEEK or MONTH: the BOUNDS of the period are the runners' (ADR 0086 B1 — the current week/month just
// started, in Vietnam time).
type ReportPeriod string

const (
	ReportPeriodWeek  ReportPeriod = "week"
	ReportPeriodMonth ReportPeriod = "month"
)

// ScheduledReportPeriod decides the period of one claimed `scheduled_reports` run — THE ONE
// IMPLEMENTATION of "is today the 1st" (ADR 0086 B2, stop condition #4: a runner never decides it).
//
//	scheduled run   the SLOT's local date: the 1st → MONTH, any other day → WEEK. A slot that opened on
//	                the 1st and is claimed on the 2nd (every runner down overnight) is still the month
//	                report it was due as
//	"run now"       the claim's local date, by the same rule — the reference system's run for one
//	                commune (reports.py:191-198: "the 1st sends the month, other days the week"). A
//	                press on the 1st therefore reports the month
//
// "" for any other job, or when no slot exists (unreachable after DueTrigger answered TriggerSchedule).
func ScheduledReportPeriod(s AutomationSetting, st ScopeState, trigger RunTrigger, now time.Time,
	zone *time.Location) ReportPeriod {

	if s.Job != JobScheduledReports || zone == nil {
		return ""
	}
	day := now
	if trigger == TriggerSchedule {
		slot, ok := LatestSlot(s, st, now, zone)
		if !ok {
			return ""
		}
		day = slot
	}
	if day.In(zone).Day() == 1 {
		return ReportPeriodMonth
	}
	return ReportPeriodWeek
}

// RunOutcome is how one run ended (contract: AutomationRunOutcome). Closed; no free text crosses.
type RunOutcome string

const (
	OutcomeSucceeded             RunOutcome = "succeeded"
	OutcomeConfigurationMissing  RunOutcome = "configuration_missing"
	OutcomeDependencyUnavailable RunOutcome = "dependency_unavailable"
	OutcomeFailed                RunOutcome = "failed"
)

// Valid reports whether o is one of the four recorded outcomes.
func (o RunOutcome) Valid() bool {
	switch o {
	case OutcomeSucceeded, OutcomeConfigurationMissing, OutcomeDependencyUnavailable, OutcomeFailed:
		return true
	}
	return false
}

// RunReport is what a runner sends back for one run: the outcome and three counts — never a list of
// records and never a message (rule 3: a runner's error text is where personal data ends up).
type RunReport struct {
	Outcome                 RunOutcome
	RecordsExamined         int
	NoticesDelivered        int
	RecordsWithoutRecipient int
}

// AutomationRun is one claimed run and, once recorded, its outcome (`automation_run`, migration 0017).
// Outcome "" means claimed and not recorded yet — a runner that died shows as a run with no result,
// which is what it is.
type AutomationRun struct {
	ID        string
	Scope     AutomationScope
	Trigger   RunTrigger
	ClaimedAt time.Time

	// ReportPeriod — set on a `scheduled_reports` run only (ScheduledReportPeriod), "" otherwise.
	ReportPeriod ReportPeriod

	Report     RunReport
	RecordedAt time.Time
}

// Recorded reports whether the run's outcome has been written.
func (r AutomationRun) Recorded() bool { return r.Report.Outcome != "" }
