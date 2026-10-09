package app

// scheduled_reports over the incoming register (ADR 0086 B1/B2), through the real runner Tick:
//   - the period is identity's (WEEK/MONTH); UNSPECIFIED sends NOTHING and records FAILED — never a
//     period read from this runner's clock (stop condition #4);
//   - the key names the CURRENT period's first day in Asia/Ho_Chi_Minh, so a retried or redone run
//     sends the same key and comms creates nothing twice;
//   - the one figure is the overdue backlog at claimed_at, derived from each STORED deadline;
//   - recipients are the leadership; none → nothing sent, the backlog counted as without recipient.

import (
	"context"
	"strings"
	"testing"
	"time"

	commsv1 "github.com/vihat/vigov/core/gen/vigov/comms/v1"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

func (h *harness) claimReport(runID string, period identityv1.ScheduledReportPeriod, at time.Time) {
	h.identity.runs[communeA] = append(h.identity.runs[communeA], identityclient.AutomationRun{RunID: runID,
		Job: jobReports, WorkKind: kindIncoming, ClaimedAt: at, ScheduledReportPeriod: period})
}

const (
	periodWeek  = identityv1.ScheduledReportPeriod_SCHEDULED_REPORT_PERIOD_WEEK
	periodMonth = identityv1.ScheduledReportPeriod_SCHEDULED_REPORT_PERIOD_MONTH
)

// reportHarness: two leaders, one document past its deadline, one exactly AT runAt (not yet past —
// the strict comparison of PastDeadlineAt), one ahead.
func reportHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t, communeA)
	h.identity.leaders[communeA] = []string{"CB-LD2", "CB-LD1"}
	h.incoming.byCommune[communeA] = []domain.AutomationRecord{
		doc("vb-late", 1, missed), doc("vb-now", 2, runAt), doc("vb-ahead", 3, soonDue),
	}
	return h
}

func TestScheduledReportWeekAndMonth(t *testing.T) {
	for _, c := range []struct {
		period     identityv1.ScheduledReportPeriod
		key, title string
	}{
		// runAt is Tuesday 29/09/2026 08:00 in Viet Nam: the current week began Monday 28/09.
		{periodWeek, "scheduled_reports:van-ban-den:week:2026-09-28", "Báo cáo điều hành tuần đã sẵn sàng"},
		{periodMonth, "scheduled_reports:van-ban-den:month:2026-09-01", "Báo cáo điều hành tháng đã sẵn sàng"},
	} {
		h := reportHarness(t)
		h.claimReport("run-r", c.period, runAt)
		h.runner.Tick(context.Background())

		if len(h.comms.delivered) != 1 {
			t.Fatalf("%v: giao %d thông báo, muốn 1", c.period, len(h.comms.delivered))
		}
		n := h.comms.delivered[0].n
		if n.IdempotencyKey != c.key || n.Title != c.title || n.Body != "Văn bản đến quá hạn: 1" ||
			n.Kind != commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_REPORT_READY ||
			strings.Join(n.RecipientMa, ",") != "CB-LD1,CB-LD2" || n.Link != "/van-ban?metric=overdue" {
			t.Errorf("%v: thông báo = %+v", c.period, n)
		}
		if o := h.identity.recorded["run-r"]; o.Outcome != succeeded || o.RecordsExamined != 3 || o.NoticesDelivered != 2 {
			t.Errorf("%v: kết quả = %+v", c.period, o)
		}
		if h.letters.reads != 0 {
			t.Errorf("%v: báo cáo định kỳ đọc sổ đơn thư", c.period)
		}
	}
}

// The period boundary is the VIETNAMESE day (ADR 0053 §3): 18:00 UTC on 30/09 is already 01/10 in Viet
// Nam, so the month is October and the week is the one that began Monday 28/09. A UTC boundary would
// key September.
func TestScheduledReportPeriodStartIsVietnameseDate(t *testing.T) {
	at := time.Date(2026, 9, 30, 18, 0, 0, 0, time.UTC)
	for period, want := range map[identityv1.ScheduledReportPeriod]string{
		periodMonth: "scheduled_reports:van-ban-den:month:2026-10-01",
		periodWeek:  "scheduled_reports:van-ban-den:week:2026-09-28",
	} {
		h := reportHarness(t)
		h.claimReport("run-r", period, at)
		h.runner.Tick(context.Background())
		if keys := h.keys(communeA); len(keys) != 1 || keys[0] != want {
			t.Errorf("%v: khoá = %v, muốn %s", period, keys, want)
		}
	}
}

// A redone run (another run id, same period, a later instant) sends the SAME key: comms creates nothing.
func TestScheduledReportIsIdempotentAcrossRuns(t *testing.T) {
	h := reportHarness(t)
	h.claimReport("run-1", periodWeek, runAt)
	h.runner.Tick(context.Background())
	h.claimReport("run-2", periodWeek, runAt.Add(3*time.Hour))
	h.runner.Tick(context.Background())

	keys := h.keys(communeA)
	if len(keys) != 2 || keys[0] != keys[1] {
		t.Fatalf("khoá qua hai lượt = %v", keys)
	}
	if o := h.identity.recorded["run-2"]; o.Outcome != succeeded || o.NoticesDelivered != 0 {
		t.Errorf("lượt làm lại tạo thêm thông báo: %+v", o)
	}
}

// identity did not say WEEK or MONTH: nothing is read, nothing sent, FAILED — never a guessed period.
func TestScheduledReportWithoutPeriodSendsNothing(t *testing.T) {
	h := reportHarness(t)
	h.claimReport("run-x", identityv1.ScheduledReportPeriod_SCHEDULED_REPORT_PERIOD_UNSPECIFIED, runAt)
	h.runner.Tick(context.Background())
	if o := h.identity.recorded["run-x"]; o.Outcome != identityv1.AutomationRunOutcome_AUTOMATION_RUN_OUTCOME_FAILED ||
		h.comms.calls != 0 {
		t.Errorf("không có kỳ: %+v, comms gọi %d lần", o, h.comms.calls)
	}
}

// No leadership flagged: nobody is told (never "everyone"), and the backlog is counted.
func TestScheduledReportWithoutLeadershipIsCounted(t *testing.T) {
	h := reportHarness(t)
	delete(h.identity.leaders, communeA)
	h.claimReport("run-r", periodMonth, runAt)
	h.runner.Tick(context.Background())
	if o := h.identity.recorded["run-r"]; h.comms.calls != 0 || o.RecordsWithoutRecipient != 1 || o.Outcome != succeeded {
		t.Errorf("không có lãnh đạo: comms %d lần, kết quả %+v", h.comms.calls, o)
	}
}

// A SCHEDULED_REPORTS run for the letter register (identity should never hand one out: it is not
// claimed) fails rather than reporting letters under the documents' key.
func TestScheduledReportForLettersIsFailed(t *testing.T) {
	h := reportHarness(t)
	h.identity.runs[communeA] = []identityclient.AutomationRun{{RunID: "run-l", Job: jobReports, WorkKind: kindLetter,
		ClaimedAt: runAt, ScheduledReportPeriod: periodWeek}}
	h.runner.Tick(context.Background())
	if o := h.identity.recorded["run-l"]; o.Outcome != identityv1.AutomationRunOutcome_AUTOMATION_RUN_OUTCOME_FAILED ||
		h.comms.calls != 0 {
		t.Errorf("lượt báo cáo cho đơn thư: %+v", o)
	}
}
