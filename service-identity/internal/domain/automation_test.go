package domain

import (
	"testing"
	"time"
)

// What these tests defend: the ONE implementation of "is this scope due" (contract
// ClaimDueAutomationRuns §WHAT "DUE" MEANS), in Asia/Ho_Chi_Minh, with `now` handed in.
//
// THE EXPECTATIONS ARE BUILT IN A FIXED +07 ZONE, not domain.MuiGio(): an oracle computed by the code
// under test agrees with it by construction. Vietnam has had no daylight saving since 1975.
var vnFixed = time.FixedZone("ICT-oracle", 7*3600)

func vn(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.ParseInLocation("2006-01-02 15:04", s, vnFixed)
	if err != nil {
		t.Fatalf("bad instant %q: %v", s, err)
	}
	return v
}

func zoneVN(t *testing.T) *time.Location {
	t.Helper()
	z, err := MuiGio()
	if err != nil {
		t.Fatalf("zone: %v", err)
	}
	return z
}

// 2026-09-28 is a Monday; 2026-10-02 a Friday.

func TestDueDisabledOrNoRowIsNeverDue(t *testing.T) {
	z := zoneVN(t)
	off := AutomationSetting{Job: JobEscalation, Enabled: false, RunHour: 7, EnabledAt: vn(t, "2026-09-01 00:00")}
	if _, due := DueTrigger(off, ScopeState{}, vn(t, "2026-09-29 08:00"), z); due {
		t.Fatal("a switched-off job was due")
	}
	// Enabled without a switch-on instant cannot come from the store (CHECK), and is refused here too.
	broken := AutomationSetting{Job: JobEscalation, Enabled: true, RunHour: 7}
	if _, due := DueTrigger(broken, ScopeState{}, vn(t, "2026-09-29 08:00"), z); due {
		t.Fatal("enabled with no switch-on instant counted every past slot")
	}
	// A request on a switched-off job is NOT a run: off means off.
	off.RunRequestedAt = vn(t, "2026-09-29 07:59")
	if _, due := DueTrigger(off, ScopeState{}, vn(t, "2026-09-29 08:00"), z); due {
		t.Fatal("run-now ran a switched-off job")
	}
}

func TestDueDailyInVietnamTimeAndSwitchOnSkipsTodaysPastSlot(t *testing.T) {
	z := zoneVN(t)
	s := AutomationSetting{Job: JobEscalation, Enabled: true, RunHour: 7, RunMinute: 0,
		EnabledAt: vn(t, "2026-09-28 15:00")}

	// Switched on at 15:00 Monday: Monday's 07:00 opened BEFORE the switch-on and never counts.
	if _, due := DueTrigger(s, ScopeState{}, vn(t, "2026-09-28 16:00"), z); due {
		t.Fatal("enabling at 15:00 ran today's 07:00 slot")
	}
	// Tuesday 06:59 local = Monday 23:59 UTC: not yet. 07:00 local = 00:00 UTC: due.
	if _, due := DueTrigger(s, ScopeState{}, vn(t, "2026-09-29 06:59").UTC(), z); due {
		t.Fatal("due one minute before the slot")
	}
	tr, due := DueTrigger(s, ScopeState{}, vn(t, "2026-09-29 07:00").UTC(), z)
	if !due || tr != TriggerSchedule {
		t.Fatalf("slot 07:00 Tuesday not due (trigger %q)", tr)
	}
	// Claimed at 07:01: the rest of Tuesday is not due again.
	st := ScopeState{LastRunID: "r1", LastClaimedAt: vn(t, "2026-09-29 07:01")}
	if _, due := DueTrigger(s, st, vn(t, "2026-09-29 18:00"), z); due {
		t.Fatal("daily job due twice in one day")
	}
}

func TestDueMissedSlotsRunOnceNotReplayed(t *testing.T) {
	z := zoneVN(t)
	s := AutomationSetting{Job: JobEscalation, Enabled: true, RunHour: 7, EnabledAt: vn(t, "2026-09-01 00:00")}
	// Last claimed Monday; every runner was down Tuesday to Thursday; ticking again Friday 09:00.
	st := ScopeState{LastRunID: "r1", LastClaimedAt: vn(t, "2026-09-28 07:01")}
	now := vn(t, "2026-10-02 09:00")
	slot, ok := LatestSlot(s, st, now, z)
	if !ok || !slot.Equal(vn(t, "2026-10-02 07:00")) {
		t.Fatalf("latest slot = %v, want Friday 07:00 (not a replay of Tuesday)", slot)
	}
	if _, due := DueTrigger(s, st, now, z); !due {
		t.Fatal("missed slot was skipped for the day (the reference system's five-minute window defect)")
	}
	// After that one claim, nothing more is due on Friday: three missed slots, one run.
	st = ScopeState{LastRunID: "r2", LastClaimedAt: now}
	if _, due := DueTrigger(s, st, vn(t, "2026-10-02 09:01"), z); due {
		t.Fatal("missed slots replayed one by one")
	}
}

func TestDueWeeklyOnItsWeekdayOnly(t *testing.T) {
	z := zoneVN(t)
	s := AutomationSetting{Job: JobWeeklyDigest, Enabled: true, Weekday: 1, RunHour: 7, RunMinute: 30,
		EnabledAt: vn(t, "2026-09-27 12:00")} // Sunday
	if _, due := DueTrigger(s, ScopeState{}, vn(t, "2026-09-28 07:29"), z); due {
		t.Fatal("due before Monday 07:30")
	}
	if _, due := DueTrigger(s, ScopeState{}, vn(t, "2026-09-28 07:30"), z); !due {
		t.Fatal("Monday 07:30 not due")
	}
	st := ScopeState{LastRunID: "r1", LastClaimedAt: vn(t, "2026-09-28 07:31")}
	for _, at := range []string{"2026-09-29 07:30", "2026-10-04 23:59"} {
		if _, due := DueTrigger(s, st, vn(t, at), z); due {
			t.Fatalf("weekly job due again at %s", at)
		}
	}
	if _, due := DueTrigger(s, st, vn(t, "2026-10-05 07:30"), z); !due {
		t.Fatal("next Monday 07:30 not due")
	}
}

// --- scheduled_reports (ADR 0086 B) ------------------------------------------------------------------
//
// Calendar: 2026-06-01 is a Monday (day 1 ON the weekday); 2026-10-01 a Thursday; 2026-10-05 a Monday.

func reportsSetting(t *testing.T, enabledAt string) AutomationSetting {
	return AutomationSetting{Job: JobScheduledReports, Enabled: true, Weekday: 1, RunHour: 7, RunMinute: 45,
		EnabledAt: vn(t, enabledAt)}
}

// The 1st falling on the configured Monday is ONE slot, a MONTH slot — never a week run and a month run.
func TestScheduledReportsFirstOnTheWeekdayIsOneMonthSlot(t *testing.T) {
	z := zoneVN(t)
	s := reportsSetting(t, "2026-05-20 09:00")
	now := vn(t, "2026-06-01 07:45")
	tr, due := DueTrigger(s, ScopeState{}, now, z)
	if !due || tr != TriggerSchedule {
		t.Fatalf("Monday the 1st 07:45 not due (trigger %q)", tr)
	}
	if p := ScheduledReportPeriod(s, ScopeState{}, tr, now, z); p != ReportPeriodMonth {
		t.Fatalf("period = %q, want month — the 1st on the weekday is the month report only", p)
	}
	// Claimed: nothing more that day — the weekday rule does not open a second slot.
	st := ScopeState{LastRunID: "r1", LastClaimedAt: vn(t, "2026-06-01 07:46")}
	for _, at := range []string{"2026-06-01 07:50", "2026-06-01 23:59", "2026-06-07 23:59"} {
		if _, due := DueTrigger(s, st, vn(t, at), z); due {
			t.Fatalf("second slot at %s after the month run", at)
		}
	}
	// The next Monday is an ordinary week slot.
	if tr, due := DueTrigger(s, st, vn(t, "2026-06-08 07:45"), z); !due ||
		ScheduledReportPeriod(s, st, tr, vn(t, "2026-06-08 07:45"), z) != ReportPeriodWeek {
		t.Fatal("next Monday is not a week slot")
	}
}

func TestScheduledReportsOrdinaryWeekdayIsWeekAndFirstOffWeekdayIsMonth(t *testing.T) {
	z := zoneVN(t)
	s := reportsSetting(t, "2026-09-20 09:00")
	mon := ScopeState{LastRunID: "r0", LastClaimedAt: vn(t, "2026-09-28 07:46")} // Monday's week run

	// Thursday 1 October: the 1st, not the weekday — a MONTH slot.
	if _, due := DueTrigger(s, mon, vn(t, "2026-10-01 07:44"), z); due {
		t.Fatal("due one minute before the 1st's slot")
	}
	now := vn(t, "2026-10-01 07:45")
	tr, due := DueTrigger(s, mon, now, z)
	if !due || ScheduledReportPeriod(s, mon, tr, now, z) != ReportPeriodMonth {
		t.Fatalf("Thursday the 1st: due %v period %q, want month", due, ScheduledReportPeriod(s, mon, tr, now, z))
	}
	st := ScopeState{LastRunID: "r1", LastClaimedAt: vn(t, "2026-10-01 07:46")}
	// Friday to Sunday: neither the weekday nor the 1st.
	for _, at := range []string{"2026-10-02 07:45", "2026-10-04 23:59", "2026-10-05 07:44"} {
		if _, due := DueTrigger(s, st, vn(t, at), z); due {
			t.Fatalf("due at %s — not the weekday and not the 1st", at)
		}
	}
	// Monday 5 October: an ordinary weekday — a WEEK slot.
	now = vn(t, "2026-10-05 07:45").UTC() // the zone is the commune's, not the instant's
	tr, due = DueTrigger(s, st, now, z)
	if !due || ScheduledReportPeriod(s, st, tr, now, z) != ReportPeriodWeek {
		t.Fatalf("ordinary Monday: due %v period %q, want week", due, ScheduledReportPeriod(s, st, tr, now, z))
	}
}

// A restart inside the slot: the replica that comes back at 07:47 finds the 07:45 slot already claimed.
func TestScheduledReportsRestartInsideTheSlotDoesNotDoubleClaim(t *testing.T) {
	z := zoneVN(t)
	s := reportsSetting(t, "2026-09-20 09:00")
	st := ScopeState{LastRunID: "r1", LastClaimedAt: vn(t, "2026-10-05 07:45")}
	for _, at := range []string{"2026-10-05 07:45", "2026-10-05 07:47", "2026-10-05 12:00"} {
		if _, due := DueTrigger(s, st, vn(t, at), z); due {
			t.Fatalf("slot claimed at 07:45 due again at %s", at)
		}
	}
}

// Switched on at 09:00 on the 1st: that day's 07:45 month slot opened BEFORE the switch-on and never
// counts — the commune's first report is the next weekday's.
func TestScheduledReportsSlotBeforeSwitchOnDoesNotCount(t *testing.T) {
	z := zoneVN(t)
	s := reportsSetting(t, "2026-10-01 09:00")
	if _, due := DueTrigger(s, ScopeState{}, vn(t, "2026-10-01 10:00"), z); due {
		t.Fatal("enabling at 09:00 on the 1st ran that day's 07:45 month slot")
	}
	now := vn(t, "2026-10-05 07:45")
	tr, due := DueTrigger(s, ScopeState{}, now, z)
	if !due || ScheduledReportPeriod(s, ScopeState{}, tr, now, z) != ReportPeriodWeek {
		t.Fatal("first slot after the switch-on is not Monday's week report")
	}
}

// A month slot claimed late (every runner down overnight) stays a MONTH run: the period follows the
// slot, not the clock at the claim. A run-now follows the claim's own local date.
func TestScheduledReportsPeriodFollowsSlotAndRunNowFollowsClaimDate(t *testing.T) {
	z := zoneVN(t)
	s := reportsSetting(t, "2026-09-20 09:00")
	st := ScopeState{LastRunID: "r0", LastClaimedAt: vn(t, "2026-09-28 07:45")}
	late := vn(t, "2026-10-02 03:00")
	tr, due := DueTrigger(s, st, late, z)
	if !due || ScheduledReportPeriod(s, st, tr, late, z) != ReportPeriodMonth {
		t.Fatal("the 1st's slot claimed on the 2nd lost its month period")
	}
	if p := ScheduledReportPeriod(s, st, TriggerRequest, vn(t, "2026-10-07 10:00"), z); p != ReportPeriodWeek {
		t.Errorf("run-now on a Wednesday = %q, want week", p)
	}
	// 00:30 on the 1st in Vietnam is still the 30th in UTC — the commune's date decides.
	if p := ScheduledReportPeriod(s, st, TriggerRequest, vn(t, "2026-10-01 00:30").UTC(), z); p != ReportPeriodMonth {
		t.Errorf("run-now at 00:30 local on the 1st = %q, want month", p)
	}
	other := AutomationSetting{Job: JobWeeklyDigest, Enabled: true, Weekday: 1, RunHour: 7, EnabledAt: s.EnabledAt}
	if p := ScheduledReportPeriod(other, st, TriggerSchedule, vn(t, "2026-06-01 08:00"), z); p != "" {
		t.Errorf("weekly_digest got period %q — only scheduled_reports carries one", p)
	}
}

func TestDueIntervalCountsFromSwitchOnThenFromLastClaim(t *testing.T) {
	z := zoneVN(t)
	s := AutomationSetting{Job: JobSLAReminders, Enabled: true, IntervalMinutes: 15,
		EnabledAt: vn(t, "2026-09-29 10:00")}
	if _, due := DueTrigger(s, ScopeState{}, vn(t, "2026-09-29 10:14"), z); due {
		t.Fatal("due before the first interval elapsed — a setting applies from the NEXT slot")
	}
	if _, due := DueTrigger(s, ScopeState{}, vn(t, "2026-09-29 10:15"), z); !due {
		t.Fatal("first interval elapsed and not due")
	}
	st := ScopeState{LastRunID: "r1", LastClaimedAt: vn(t, "2026-09-29 10:16")}
	if _, due := DueTrigger(s, st, vn(t, "2026-09-29 10:30"), z); due {
		t.Fatal("due 14 minutes after the last claim")
	}
	if _, due := DueTrigger(s, st, vn(t, "2026-09-29 10:31"), z); !due {
		t.Fatal("not due 15 minutes after the last claim")
	}
	// Re-enabled after a switch-off: the old claim is before the switch-on, so the count restarts there.
	s.EnabledAt = vn(t, "2026-09-29 12:00")
	if _, due := DueTrigger(s, st, vn(t, "2026-09-29 12:10"), z); due {
		t.Fatal("re-enabled job ran before its first interval")
	}
}

func TestDueRunRequestConsumedOncePerScope(t *testing.T) {
	z := zoneVN(t)
	s := AutomationSetting{Job: JobEscalation, Enabled: true, RunHour: 7,
		EnabledAt: vn(t, "2026-09-01 00:00"), RunRequestedAt: vn(t, "2026-09-29 10:00")}
	// The daily slot was already claimed at 07:01; the request makes BOTH scopes due regardless.
	claimed := ScopeState{LastRunID: "r1", LastClaimedAt: vn(t, "2026-09-29 07:01")}
	for _, st := range []ScopeState{claimed, {}} {
		tr, due := DueTrigger(s, st, vn(t, "2026-09-29 10:01"), z)
		if !due || tr != TriggerRequest {
			t.Fatalf("run-now not due for scope %+v (trigger %q)", st, tr)
		}
	}
	// One scope claims it at 10:01: that scope is done; the OTHER scope's pending mark is untouched.
	done := ScopeState{LastRunID: "r2", LastClaimedAt: vn(t, "2026-09-29 10:01")}
	if _, due := DueTrigger(s, done, vn(t, "2026-09-29 10:02"), z); due {
		t.Fatal("run-now consumed twice by one scope")
	}
	if _, due := DueTrigger(s, claimed, vn(t, "2026-09-29 10:02"), z); !due {
		t.Fatal("one scope's claim cleared the other scope's run-now")
	}
}

func TestDueRunRequestEdges(t *testing.T) {
	z := zoneVN(t)
	base := AutomationSetting{Job: JobEscalation, Enabled: true, RunHour: 7, EnabledAt: vn(t, "2026-09-29 12:00")}
	st := ScopeState{LastRunID: "r1", LastClaimedAt: vn(t, "2026-09-29 11:00")}

	// A request made before the last switch-on belongs to a previous enablement.
	s := base
	s.RunRequestedAt = vn(t, "2026-09-29 11:30")
	if _, due := DueTrigger(s, st, vn(t, "2026-09-29 12:05"), z); due {
		t.Fatal("a request older than the switch-on ran")
	}
	// A request stamped after `now` (clock skew between replicas) waits until now reaches it —
	// otherwise the claim would be older than the mark and run again next tick.
	s = base
	s.RunRequestedAt = vn(t, "2026-09-29 12:10")
	if _, due := DueTrigger(s, st, vn(t, "2026-09-29 12:09"), z); due {
		t.Fatal("a request in the future was due")
	}
	if tr, due := DueTrigger(s, st, vn(t, "2026-09-29 12:10"), z); !due || tr != TriggerRequest {
		t.Fatal("a request was not due once now reached it")
	}
}

func TestValidateAutomationSetting(t *testing.T) {
	ok := []AutomationSetting{
		{Job: JobSLAReminders, IntervalMinutes: 5},
		{Job: JobSLAReminders, IntervalMinutes: MaxIntervalMinutes},
		{Job: JobEscalation, RunHour: 0, RunMinute: 0},
		{Job: JobEscalation, RunHour: 23, RunMinute: 59},
		{Job: JobWeeklyDigest, Weekday: 7, RunHour: 7, RunMinute: 30},
		{Job: JobScheduledReports, Weekday: 1, RunHour: 7, RunMinute: 45},
		{Job: JobScheduledReports, Weekday: 7, RunHour: 23, RunMinute: 59},
	}
	for _, s := range ok {
		if err := ValidateAutomationSetting(s); err != nil {
			t.Errorf("%+v refused: %v", s, err)
		}
	}
	bad := map[string]AutomationSetting{
		"interval 4":   {Job: JobSLAReminders, IntervalMinutes: 4},
		"interval max": {Job: JobSLAReminders, IntervalMinutes: MaxIntervalMinutes + 1},
		"hour 24":      {Job: JobEscalation, RunHour: 24},
		"minute 60":    {Job: JobEscalation, RunMinute: 60},
		"weekday 0":    {Job: JobWeeklyDigest, Weekday: 0, RunHour: 7},
		"weekday 8":    {Job: JobWeeklyDigest, Weekday: 8, RunHour: 7},
		"unknown job":  {Job: "refresh_dashboards"},
		// The reference system's key is not ours: stored and routed as `scheduled_reports`.
		"reference key":     {Job: "send_scheduled_reports", Weekday: 1, RunHour: 7},
		"reports weekday 0": {Job: JobScheduledReports, Weekday: 0, RunHour: 7},
		"reports weekday 8": {Job: JobScheduledReports, Weekday: 8, RunHour: 7},
		"reports hour 24":   {Job: JobScheduledReports, Weekday: 1, RunHour: 24},
		"reports minute 60": {Job: JobScheduledReports, Weekday: 1, RunHour: 7, RunMinute: 60},
	}
	for name, s := range bad {
		if err := ValidateAutomationSetting(s); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestSuggestedSettingIsOffWithUsersCadence(t *testing.T) {
	want := map[AutomationJob]AutomationSetting{
		JobSLAReminders: {Job: JobSLAReminders, IntervalMinutes: 15},
		JobEscalation:   {Job: JobEscalation, RunHour: 7, RunMinute: 0},
		JobWeeklyDigest: {Job: JobWeeklyDigest, Weekday: 1, RunHour: 7, RunMinute: 30},
		// The reference system's prefill (automation.py:96-98): Monday 07:45, off.
		JobScheduledReports: {Job: JobScheduledReports, Weekday: 1, RunHour: 7, RunMinute: 45},
	}
	if len(AutomationJobs()) != 4 {
		t.Fatalf("AutomationJobs = %v, want four", AutomationJobs())
	}
	for _, j := range AutomationJobs() {
		got := SuggestedAutomationSetting(j)
		if got.Enabled {
			t.Errorf("%s suggested ON — no row means off (§9 Mặc định tắt hết)", j)
		}
		if got != want[j] {
			t.Errorf("%s suggested %+v, want %+v", j, got, want[j])
		}
		if err := ValidateAutomationSetting(got); err != nil {
			t.Errorf("%s suggestion invalid: %v", j, err)
		}
	}
}
