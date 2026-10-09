package app

// What these tests defend (ADR 0086 B1/B2 — `scheduled_reports`):
//   - the period is IDENTITY's (AutomationRun.scheduled_report_period); UNSPECIFIED sends nothing and
//     records FAILED, never a guess from this runner's clock;
//   - the period bounds are the CURRENT week (Monday 00:00) / month (day 1) in Asia/Ho_Chi_Minh;
//   - the figures: tasks overdue as of claimed_at (stock), petitions late in the period (the overview's
//     predicate, restricted field excluded); a figure with no data is left out, never 0-filled;
//   - one notice per work kind per period, to the leadership, kind REPORT_READY, stable key across runs.

import (
	"context"
	"strings"
	"testing"
	"time"

	commsv1 "github.com/vihat/vigov/core/gen/vigov/comms/v1"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

const jobReports = identityv1.AutomationJob_AUTOMATION_JOB_SCHEDULED_REPORTS

func (h *harness) claimReport(id tenant.ID, runID string, kind identityv1.WorkKind, at time.Time,
	period identityv1.ScheduledReportPeriod) {
	h.identity.runs[id] = append(h.identity.runs[id], identityclient.AutomationRun{RunID: runID, Job: jobReports,
		WorkKind: kind, ClaimedAt: at, ScheduledReportPeriod: period})
}

func TestScheduledReportTasksWeek(t *testing.T) {
	h := newHarness(t, communeA)
	h.identity.leaders[communeA] = []string{"CB-LD"}
	h.tasks.byCommune[communeA] = []domain.AutomationRecord{
		{ID: "nv-1", Code: "NV01", Deadline: missed},
		{ID: "nv-2", Code: "NV02", Deadline: runAt}, // at claimed_at = overdue (identity.proto)
		{ID: "nv-3", Code: "NV03", Deadline: soonDue},
		{ID: "nv-4", Code: "NV04"},
	}
	// runAt is Tuesday 29/09/2026 08:00 local → the week began Monday 28/09.
	h.claimReport(communeA, "run-r", kindTask, runAt, identityv1.ScheduledReportPeriod_SCHEDULED_REPORT_PERIOD_WEEK)
	h.runner.Tick(context.Background())

	if len(h.comms.delivered) != 1 {
		t.Fatalf("giao %d, muốn 1", len(h.comms.delivered))
	}
	n := h.comms.delivered[0].n
	if n.IdempotencyKey != "scheduled_reports:nhiem-vu:week:2026-09-28" ||
		n.Kind != commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_REPORT_READY ||
		strings.Join(n.RecipientMa, ",") != "CB-LD" {
		t.Errorf("thông báo = %+v", n)
	}
	if n.Title != "Báo cáo điều hành tuần đã sẵn sàng" || n.Body != "Nhiệm vụ quá hạn: 2" || n.Link != "/bao-cao" {
		t.Errorf("câu chữ = %q / %q / %q", n.Title, n.Body, n.Link)
	}
	if o := h.identity.recorded["run-r"]; o.Outcome != succeeded || o.RecordsExamined != 4 || o.NoticesDelivered != 1 {
		t.Errorf("kết quả = %+v", o)
	}
}

func TestScheduledReportPetitionsMonthUsesTheCurrentLocalMonth(t *testing.T) {
	h := newHarness(t, communeA)
	h.identity.leaders[communeA] = []string{"CB-LD"}
	h.reports.summary = domain.CitizenReportSummary{OnTimeSample: 9, Late: 3}
	// 30/09 18:30 UTC is ALREADY 01/10 01:30 in Viet Nam: the month is October, not September.
	at := time.Date(2026, 9, 30, 18, 30, 0, 0, time.UTC)
	h.claimReport(communeA, "run-m", kindReport, at, identityv1.ScheduledReportPeriod_SCHEDULED_REPORT_PERIOD_MONTH)
	h.runner.Tick(context.Background())

	wantFrom := time.Date(2026, 9, 30, 17, 0, 0, 0, time.UTC) // 01/10 00:00 +07
	if !h.reports.sawPeriod.From.Equal(wantFrom) || !h.reports.sawPeriod.To.Equal(at) {
		t.Errorf("kỳ hỏi = %v → %v, muốn %v → %v", h.reports.sawPeriod.From, h.reports.sawPeriod.To, wantFrom, at)
	}
	if h.reports.sawRestricted {
		t.Error("đếm cả lĩnh vực hạn chế cho danh sách lãnh đạo không mang quyền feedback.restricted")
	}
	n := h.comms.delivered[0].n
	if n.IdempotencyKey != "scheduled_reports:phan-anh:month:2026-10-01" || n.Body != "Phản ánh trễ hạn trong kỳ: 3" ||
		n.Title != "Báo cáo điều hành tháng đã sẵn sàng" {
		t.Errorf("thông báo = %+v", n)
	}
}

// A run at exactly the first instant of its period has no elapsed time to count: the petition figure is
// LEFT OUT, never sent as 0 (ADR 0053 §6). The notice still says the report is ready.
func TestScheduledReportEmptyPeriodOmitsTheFigure(t *testing.T) {
	h := newHarness(t, communeA)
	h.identity.leaders[communeA] = []string{"CB-LD"}
	monday := time.Date(2026, 9, 27, 17, 0, 0, 0, time.UTC) // Monday 28/09 00:00 +07
	h.claimReport(communeA, "run-0", kindReport, monday, identityv1.ScheduledReportPeriod_SCHEDULED_REPORT_PERIOD_WEEK)
	h.runner.Tick(context.Background())

	if h.reports.summaryCalls != 0 {
		t.Error("đếm một kỳ chưa trôi giây nào")
	}
	if n := h.comms.delivered[0].n; n.Body != "" || strings.Contains(n.Body, "0") {
		t.Errorf("nội dung = %q, muốn trống", n.Body)
	}
}

func TestScheduledReportWithoutPeriodSendsNothingAndFails(t *testing.T) {
	h := newHarness(t, communeA)
	h.identity.leaders[communeA] = []string{"CB-LD"}
	h.claimReport(communeA, "run-x", kindTask, runAt, identityv1.ScheduledReportPeriod_SCHEDULED_REPORT_PERIOD_UNSPECIFIED)
	h.runner.Tick(context.Background())

	if len(h.comms.delivered) != 0 {
		t.Error("gửi báo cáo khi identity không nói kỳ nào — điều kiện dừng #4")
	}
	if o := h.identity.recorded["run-x"]; o.Outcome != identityv1.AutomationRunOutcome_AUTOMATION_RUN_OUTCOME_FAILED {
		t.Errorf("kết quả = %v, muốn FAILED", o.Outcome)
	}
}

func TestScheduledReportKeyStableAcrossRuns(t *testing.T) {
	h := newHarness(t, communeA)
	h.identity.leaders[communeA] = []string{"CB-LD"}
	h.claimReport(communeA, "run-1", kindTask, runAt, identityv1.ScheduledReportPeriod_SCHEDULED_REPORT_PERIOD_WEEK)
	h.runner.Tick(context.Background())
	// A later run in the SAME week (an identity redo) sends the same key; comms creates nothing new.
	h.claimReport(communeA, "run-2", kindTask, runAt.Add(3*time.Hour), identityv1.ScheduledReportPeriod_SCHEDULED_REPORT_PERIOD_WEEK)
	h.runner.Tick(context.Background())

	keys := h.keys(communeA)
	if len(keys) != 2 || keys[0] != keys[1] {
		t.Errorf("khoá = %v", keys)
	}
	if o := h.identity.recorded["run-2"]; o.NoticesDelivered != 0 {
		t.Errorf("lượt làm lại tạo thêm %d thông báo", o.NoticesDelivered)
	}
}

func TestScheduledReportNoLeadershipTellsNobody(t *testing.T) {
	h := newHarness(t, communeA)
	h.tasks.byCommune[communeA] = []domain.AutomationRecord{{ID: "nv-1", Code: "NV01", Deadline: missed}}
	h.claimReport(communeA, "run-n", kindTask, runAt, identityv1.ScheduledReportPeriod_SCHEDULED_REPORT_PERIOD_WEEK)
	h.runner.Tick(context.Background())

	if len(h.comms.delivered) != 0 {
		t.Error("không có lãnh đạo mà vẫn gửi")
	}
	if o := h.identity.recorded["run-n"]; o.RecordsWithoutRecipient != 1 {
		t.Errorf("không người nhận = %d, muốn 1", o.RecordsWithoutRecipient)
	}
}

func TestReportPeriodStartLocalBoundaries(t *testing.T) {
	for _, c := range []struct {
		at   time.Time
		p    domain.ReportPeriod
		want time.Time
	}{
		// Sunday 04/10 23:59 local is still the week of Monday 28/09.
		{time.Date(2026, 10, 4, 16, 59, 0, 0, time.UTC), domain.ReportWeek, time.Date(2026, 9, 27, 17, 0, 0, 0, time.UTC)},
		// Monday 05/10 00:00 local starts a new week.
		{time.Date(2026, 10, 4, 17, 0, 0, 0, time.UTC), domain.ReportWeek, time.Date(2026, 10, 4, 17, 0, 0, 0, time.UTC)},
		{time.Date(2026, 10, 9, 3, 0, 0, 0, time.UTC), domain.ReportMonth, time.Date(2026, 9, 30, 17, 0, 0, 0, time.UTC)},
	} {
		if got := domain.ReportPeriodStart(c.at, c.p); !got.Equal(c.want) {
			t.Errorf("%v %s → %v, muốn %v", c.at, c.p, got, c.want)
		}
	}
	if !domain.ReportPeriodStart(runAt, "quy").IsZero() {
		t.Error("kỳ không biết vẫn có mốc")
	}
}
