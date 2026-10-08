package domain

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// What these tests defend: the keys are the contract's recipes and depend only on the business item
// and the run's local date (never the run id); the sentences carry the record's CODE and nothing else
// from the record; the windows are calendar windows, not hour arithmetic.

var claimedAt = time.Date(2026, 9, 28, 23, 30, 0, 0, time.UTC) // 06:30 on Tuesday 29/09 in Viet Nam

func TestLocalDayAndWeekUseAdministrativeZone(t *testing.T) {
	if got := LocalDay(claimedAt); got != "2026-09-29" {
		t.Errorf("LocalDay = %s, muốn ngày Việt Nam 2026-09-29 (UTC còn là 28)", got)
	}
	if got := ISOWeek(claimedAt); got != "2026-W40" {
		t.Errorf("ISOWeek = %s, muốn 2026-W40", got)
	}
}

func TestKeysFollowTheRecipes(t *testing.T) {
	hold := time.Date(2026, 9, 25, 1, 0, 0, 0, time.UTC)
	for got, want := range map[string]string{
		DueSoonKey(AutomationTask, "2026-09-29", "CB-00123"):           "sla_reminders:due_soon:nhiem-vu:2026-09-29:CB-00123",
		OverdueKey(AutomationCitizenReport, "pa-1", "2026-09-29"):      "sla_reminders:overdue:phan-anh:pa-1:2026-09-29",
		EscalationKey(AutomationTask, "nv-1", EscalationUnitHead):      "escalation:nhiem-vu:nv-1:unit_head",
		EscalationKey(AutomationTask, "nv-1", EscalationChairman):      "escalation:nhiem-vu:nv-1:chairman",
		WeeklyDigestKey(AutomationCitizenReport, "2026-W40"):           "weekly_digest:phan-anh:2026-W40",
		UnassignedKey(AutomationTask, "nv-1", hold.In(AutomationZone)): "sla_reminders:unassigned:nhiem-vu:nv-1:1790298000",
	} {
		if got != want {
			t.Errorf("khoá = %q, muốn %q", got, want)
		}
		if !ValidNoticeKey(got) {
			t.Errorf("khoá %q không hợp lệ với comms", got)
		}
	}
}

// THE SENTENCES CARRY THE CODE, NOT THE INTERNAL ID, and nothing else from the record.
func TestNoticesCarryCodeOnly(t *testing.T) {
	r := AutomationRecord{ID: "01JINTERNALIDXXXXXXXXXXXXX", Code: "PA-7K3M9QX2", Field: "moi-truong",
		Deadline: claimedAt, OrgUnitID: "bp-internal-id", AssigneeMa: "CB-00123", HoldStartedAt: claimedAt}
	notices := []StaffNotice{
		OverdueNotice(AutomationCitizenReport, r, "2026-09-29", []string{"CB-1"}),
		UnassignedNotice(AutomationCitizenReport, r, []string{"CB-1"}),
		EscalationNotice(AutomationCitizenReport, r, EscalationChairman, []string{"CB-1"}),
		DueSoonNotice(AutomationCitizenReport, "2026-09-29", "CB-1", []AutomationRecord{r}),
	}
	for _, n := range notices {
		text := n.Title + " " + n.Body + " " + n.Link
		for _, leak := range []string{r.ID, r.OrgUnitID, r.AssigneeMa, r.Field} {
			if strings.Contains(text, leak) {
				t.Errorf("thông báo %q lộ %q", n.Title, leak)
			}
		}
		if !strings.Contains(n.Title+n.Body, r.Code) {
			t.Errorf("thông báo %q không nêu mã %s", n.Title, r.Code)
		}
		if !strings.HasPrefix(n.Link, "/") || strings.HasPrefix(n.Link, "//") {
			t.Errorf("đường dẫn %q không tương đối", n.Link)
		}
		if n.Title == "" || len([]rune(n.Title)) > noticeTitleMax || len([]rune(n.Body)) > noticeBodyMax {
			t.Errorf("độ dài tiêu đề/nội dung sai: %q", n.Title)
		}
	}
	if got := notices[0].Body; got != "Hạn xử lý: 06:30 ngày 29/09/2026." {
		t.Errorf("hạn hiển thị = %q — phải là giờ Việt Nam", got)
	}
}

func TestDueSoonBodyStaysInBound(t *testing.T) {
	recs := make([]AutomationRecord, 200)
	for i := range recs {
		recs[i] = AutomationRecord{Code: "NV-2026-" + strings.Repeat("9", 6), Deadline: claimedAt}
	}
	n := DueSoonNotice(AutomationTask, "2026-09-29", "CB-1", recs)
	if len([]rune(n.Body)) > noticeBodyMax || !strings.Contains(n.Body, "mục khác") {
		t.Errorf("nội dung %d ký tự, muốn ≤ %d và có phần 'mục khác': %q", len([]rune(n.Body)), noticeBodyMax, n.Body)
	}
	if n.Title != "Bạn có 200 nhiệm vụ sắp đến hạn xử lý" || n.Link != "/nhiem-vu?soon=true" {
		t.Errorf("tiêu đề/đường dẫn = %q %q", n.Title, n.Link)
	}
}

// THE ITEMS ARE THE BODY'S RECORDS WITH THEIR STORED DEADLINES — copied, never computed (rule 10,
// invariant 2); one per code; and the key is the recipe's, unchanged by them.
func TestDueSoonItemsCopyStoredDeadlines(t *testing.T) {
	d1 := time.Date(2026, 9, 30, 9, 15, 0, 0, time.UTC)
	d2 := time.Date(2026, 9, 29, 10, 0, 0, 0, AutomationZone) // a non-UTC location is kept as read
	recs := []AutomationRecord{
		{ID: "nv-b", Code: "NV-B", Deadline: d1},
		{ID: "nv-a", Code: "NV-A", Deadline: d2},
	}
	n := DueSoonNotice(AutomationTask, "2026-09-29", "CB-1", recs)
	if n.Key != "sla_reminders:due_soon:nhiem-vu:2026-09-29:CB-1" {
		t.Errorf("khoá = %q — các mục không được đổi khoá", n.Key)
	}
	want := []DueSoonItem{{Code: "NV-A", Deadline: d2}, {Code: "NV-B", Deadline: d1}}
	if len(n.DueSoonItems) != len(want) {
		t.Fatalf("mục = %+v, muốn %+v", n.DueSoonItems, want)
	}
	for i, w := range want {
		g := n.DueSoonItems[i]
		if g.Code != w.Code || g.Deadline != w.Deadline { // == on purpose: the very value stored, not an equal instant
			t.Errorf("mục %d = %+v, muốn %+v", i, g, w)
		}
	}
	if n.Title != "Bạn có 2 nhiệm vụ sắp đến hạn xử lý" || n.Body != "Gồm: NV-A, NV-B." {
		t.Errorf("tiêu đề/nội dung = %q %q", n.Title, n.Body)
	}

	// A code twice (one recipient through two paths can never happen today, but comms refuses a repeat
	// outright): one item, the earliest stored deadline of the two.
	dup := DueSoonNotice(AutomationTask, "2026-09-29", "CB-1", append(recs, AutomationRecord{ID: "nv-b2", Code: "NV-B", Deadline: d2}))
	if len(dup.DueSoonItems) != 2 || dup.DueSoonItems[1].Code != "NV-B" || !dup.DueSoonItems[1].Deadline.Equal(d2) {
		t.Errorf("mã lặp = %+v", dup.DueSoonItems)
	}

	// Only due-soon notices carry items (comms: a non-empty list on another kind is INVALID_ARGUMENT).
	r := recs[0]
	for _, other := range []StaffNotice{
		OverdueNotice(AutomationTask, r, "2026-09-29", []string{"CB-1"}),
		UnassignedNotice(AutomationTask, r, []string{"CB-1"}),
		EscalationNotice(AutomationTask, r, EscalationUnitHead, []string{"CB-1"}),
		TaskDigestNotice("2026-W40", 1, 1, 1, []string{"CB-1"}),
	} {
		if len(other.DueSoonItems) != 0 {
			t.Errorf("%q mang mục sắp đến hạn", other.Title)
		}
	}
}

// More than 500 for one person: the 500 EARLIEST deadlines go as items, while title and body still
// count every record.
func TestDueSoonItemsCapKeepsEarliest(t *testing.T) {
	const n = MaxDueSoonItems + 37
	recs := make([]AutomationRecord, 0, n)
	for i := n - 1; i >= 0; i-- { // fed latest first, so the cap cannot pass by input order
		recs = append(recs, AutomationRecord{ID: fmt.Sprintf("nv-%04d", i), Code: fmt.Sprintf("NV-%04d", i),
			Deadline: claimedAt.Add(time.Duration(i) * time.Minute)})
	}
	got := DueSoonNotice(AutomationTask, "2026-09-29", "CB-1", recs)
	if len(got.DueSoonItems) != MaxDueSoonItems {
		t.Fatalf("%d mục, muốn %d", len(got.DueSoonItems), MaxDueSoonItems)
	}
	for i, it := range got.DueSoonItems {
		if it.Code != fmt.Sprintf("NV-%04d", i) {
			t.Fatalf("mục %d = %s — không phải hạn sớm nhất", i, it.Code)
		}
	}
	if got.Title != fmt.Sprintf("Bạn có %d nhiệm vụ sắp đến hạn xử lý", n) {
		t.Errorf("tiêu đề phải đếm đủ %d: %q", n, got.Title)
	}
}

func TestDigestSentences(t *testing.T) {
	n := TaskDigestNotice("2026-W40", 3, 2, 1, []string{"CB-LD"})
	if n.Body != "3 nhiệm vụ quá hạn, 2 nhiệm vụ đến hạn trong 7 ngày tới, 1 nhiệm vụ chờ duyệt." {
		t.Errorf("bản tin nhiệm vụ = %q", n.Body)
	}
	c := CitizenReportDigestNotice("2026-W40", CitizenReportDigestCounts{PastDeadline: 2, LowRated: 3, Hot: 4}, []string{"CB-LD"})
	if c.Body != "4 phản ánh nóng: 2 phiếu quá hạn xử lý, 3 phiếu bị đánh giá 1–2 sao trong 7 ngày qua." {
		t.Errorf("bản tin phản ánh = %q", c.Body)
	}
}

// A calendar window, the same local clock time — across a month end.
func TestSameClockDaysLater(t *testing.T) {
	got := SameClockDaysLater(claimedAt, 7)
	if want := time.Date(2026, 10, 5, 23, 30, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("sau 7 ngày = %s, muốn %s", got, want)
	}
	if back := SameClockDaysLater(claimedAt, -7); !back.Equal(time.Date(2026, 9, 21, 23, 30, 0, 0, time.UTC)) {
		t.Errorf("trước 7 ngày = %s", back)
	}
}

func TestComparisonsAreDerived(t *testing.T) {
	r := AutomationRecord{Deadline: claimedAt}
	if !r.PastDeadlineAt(claimedAt) {
		t.Error("hạn đúng bằng mốc chạy phải là quá hạn (identity.proto)")
	}
	if r.DueSoonAt(claimedAt, claimedAt.Add(time.Second)) {
		t.Error("hạn đúng bằng mốc chạy không được là sắp đến hạn")
	}
	if (AutomationRecord{}).PastDeadlineAt(claimedAt) {
		t.Error("không có hạn mà quá hạn")
	}
}

func TestCleanRecipients(t *testing.T) {
	got := CleanRecipients([]string{"CB-2", "", "CB-1", "CB-2", "CB đổi", "CB-1"})
	if strings.Join(got, ",") != "CB-1,CB-2" {
		t.Errorf("CleanRecipients = %v", got)
	}
}
