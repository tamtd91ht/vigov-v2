package domain

import (
	"strings"
	"testing"
	"time"
)

// What these tests defend: the keys are the contract's recipes and depend only on the document and the
// run's local date (never the run id); the sentences carry the register CODE and nothing else from the
// document; the overdue comparison is this register's own (IncomingDocument.IsOverdue); windows are
// calendar windows.

var claimedAt = time.Date(2026, 9, 28, 23, 30, 0, 0, time.UTC) // 06:30 on Tuesday 29/09 in Viet Nam

func TestAutomationLocalDayAndWeek(t *testing.T) {
	if got := LocalDay(claimedAt); got != "2026-09-29" {
		t.Errorf("LocalDay = %s, muốn ngày Việt Nam 2026-09-29 (UTC còn là 28)", got)
	}
	if got := ISOWeek(claimedAt); got != "2026-W40" {
		t.Errorf("ISOWeek = %s, muốn 2026-W40", got)
	}
}

// TestAutomationKeysFollowTheRecipes PINS THE STAFF-BELL IDEMPOTENCY AND ESCALATION KEY RECIPES. comms
// deduplicates on these exact strings, so the English rename campaign renamed the functions and must
// never move a byte of what they produce (ADR 0061) — `van-ban-den` included, a wire value (layer C).
func TestAutomationKeysFollowTheRecipes(t *testing.T) {
	hold := time.Date(2026, 9, 25, 1, 0, 0, 0, time.UTC)
	k := AutomationIncomingDocument
	for got, want := range map[string]string{
		DueSoonKey(k, "2026-09-29", "CB-00123"):           "sla_reminders:due_soon:van-ban-den:2026-09-29:CB-00123",
		OverdueKey(k, "vb-1", "2026-09-29"):               "sla_reminders:overdue:van-ban-den:vb-1:2026-09-29",
		EscalationKey(k, "vb-1", EscalationUnitHead):      "escalation:van-ban-den:vb-1:unit_head",
		EscalationKey(k, "vb-1", EscalationChairman):      "escalation:van-ban-den:vb-1:chairman",
		WeeklyDigestKey(k, "2026-W40"):                    "weekly_digest:van-ban-den:2026-W40",
		UnassignedKey(k, "vb-1", hold.In(AutomationZone)): "sla_reminders:unassigned:van-ban-den:vb-1:1790298000",
	} {
		if got != want {
			t.Errorf("khoá = %q, muốn %q", got, want)
		}
		if !ValidNoticeKey(got) {
			t.Errorf("khoá %q không hợp lệ với comms", got)
		}
	}
}

// THE SENTENCES CARRY THE CODE, NOT THE INTERNAL ID, and nothing else from the document.
func TestAutomationNoticesCarryCodeOnly(t *testing.T) {
	r := AutomationRecord{ID: "01JINTERNALIDXXXXXXXXXXXXX", Code: IncomingDocumentCode(2026, 7),
		Deadline: claimedAt, OrgUnitID: "bp-internal-id", AssigneeCode: "CB-00123", HoldStartedAt: claimedAt}
	notices := []StaffNotice{
		OverdueNotice(r, "2026-09-29", []string{"CB-1"}),
		UnassignedNotice(r, []string{"CB-1"}),
		EscalationNotice(r, EscalationChairman, []string{"CB-1"}),
		DueSoonNotice("2026-09-29", "CB-1", []string{r.Code}),
	}
	for _, n := range notices {
		text := n.Title + " " + n.Body + " " + n.Link
		for _, leak := range []string{r.ID, r.OrgUnitID, r.AssigneeCode} {
			if strings.Contains(text, leak) {
				t.Errorf("thông báo %q lộ %q", n.Title, leak)
			}
		}
		if !strings.Contains(n.Title+n.Body, "VB-DEN-2026-0007") {
			t.Errorf("thông báo %q không nêu mã sổ", n.Title)
		}
		if !strings.HasPrefix(n.Link, "/van-ban") {
			t.Errorf("đường dẫn %q không vào sổ văn bản", n.Link)
		}
		if n.Title == "" || len([]rune(n.Title)) > noticeTitleMax || len([]rune(n.Body)) > noticeBodyMax {
			t.Errorf("độ dài tiêu đề/nội dung sai: %q", n.Title)
		}
	}
	if got := notices[0].Body; got != "Hạn xử lý: 06:30 ngày 29/09/2026." {
		t.Errorf("hạn hiển thị = %q — phải là giờ Việt Nam", got)
	}
}

func TestAutomationDueSoonBodyStaysInBound(t *testing.T) {
	codes := make([]string, 200)
	for i := range codes {
		codes[i] = IncomingDocumentCode(2026, i+1)
	}
	n := DueSoonNotice("2026-09-29", "CB-1", codes)
	if len([]rune(n.Body)) > noticeBodyMax || !strings.Contains(n.Body, "mục khác") {
		t.Errorf("nội dung %d ký tự, muốn ≤ %d và có phần 'mục khác'", len([]rune(n.Body)), noticeBodyMax)
	}
	if n.Title != "Bạn có 200 văn bản đến sắp đến hạn xử lý" || n.Link != "/van-ban?metric=open" {
		t.Errorf("tiêu đề/đường dẫn = %q %q", n.Title, n.Link)
	}
}

func TestIncomingDigestSentence(t *testing.T) {
	n := IncomingDigestNotice("2026-W40", 3, 2, []string{"CB-LD"})
	if n.Body != "3 văn bản đến quá hạn xử lý, 2 văn bản đến sắp đến hạn trong 7 ngày tới." ||
		n.Key != "weekly_digest:van-ban-den:2026-W40" || n.Link != "/van-ban?metric=overdue" {
		t.Errorf("bản tin văn bản = %+v", n)
	}
}

func TestAutomationSameClockDaysLater(t *testing.T) {
	if got, want := SameClockDaysLater(claimedAt, 7), time.Date(2026, 10, 5, 23, 30, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("sau 7 ngày = %s, muốn %s", got, want)
	}
}

// The overdue comparison is the register's own (strictly after), so the bell and /tong-quan agree on
// every document.
func TestAutomationOverdueMatchesRegister(t *testing.T) {
	for _, asOf := range []time.Time{claimedAt.Add(-time.Second), claimedAt, claimedAt.Add(time.Second)} {
		r := AutomationRecord{Deadline: claimedAt}
		d := IncomingDocument{DueAt: claimedAt, Status: IncomingStatusInProgress}
		if r.PastDeadlineAt(asOf) != d.IsOverdue(asOf) {
			t.Errorf("tại %s: việc nền nói %v, sổ nói %v", asOf, r.PastDeadlineAt(asOf), d.IsOverdue(asOf))
		}
	}
	r := AutomationRecord{Deadline: claimedAt}
	if r.DueSoonAt(claimedAt, claimedAt.Add(time.Hour)) {
		t.Error("hạn đúng bằng mốc chạy không được là sắp đến hạn")
	}
	if (AutomationRecord{}).PastDeadlineAt(claimedAt) || (AutomationRecord{}).DueSoonAt(claimedAt, claimedAt) {
		t.Error("không có hạn mà quá hạn / sắp đến hạn")
	}
}

func TestAutomationCleanRecipients(t *testing.T) {
	if got := CleanRecipients([]string{"CB-2", "", "CB-1", "CB-2", "CB đổi", "CB-1"}); strings.Join(got, ",") != "CB-1,CB-2" {
		t.Errorf("CleanRecipients = %v", got)
	}
}
