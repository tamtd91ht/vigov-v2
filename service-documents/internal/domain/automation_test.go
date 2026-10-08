package domain

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// What these tests defend: the keys are the contract's recipes and depend only on the document and the
// run's local date (never the run id); the sentences carry the register CODE and nothing else from the
// document; the overdue comparison is this register's own (VanBanDen.QuaHan); windows are calendar
// windows.

var claimedAt = time.Date(2026, 9, 28, 23, 30, 0, 0, time.UTC) // 06:30 on Tuesday 29/09 in Viet Nam

func TestAutomationLocalDayAndWeek(t *testing.T) {
	if got := LocalDay(claimedAt); got != "2026-09-29" {
		t.Errorf("LocalDay = %s, muốn ngày Việt Nam 2026-09-29 (UTC còn là 28)", got)
	}
	if got := ISOWeek(claimedAt); got != "2026-W40" {
		t.Errorf("ISOWeek = %s, muốn 2026-W40", got)
	}
}

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
	r := AutomationRecord{ID: "01JINTERNALIDXXXXXXXXXXXXX", Code: MaVanBanDen(2026, 7),
		Deadline: claimedAt, OrgUnitID: "bp-internal-id", AssigneeMa: "CB-00123", HoldStartedAt: claimedAt}
	notices := []StaffNotice{
		OverdueNotice(r, "2026-09-29", []string{"CB-1"}),
		UnassignedNotice(r, []string{"CB-1"}),
		EscalationNotice(r, EscalationChairman, []string{"CB-1"}),
		DueSoonNotice("2026-09-29", "CB-1", []AutomationRecord{r}),
	}
	for _, n := range notices {
		text := n.Title + " " + n.Body + " " + n.Link
		for _, leak := range []string{r.ID, r.OrgUnitID, r.AssigneeMa} {
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
	recs := make([]AutomationRecord, 200)
	for i := range recs {
		recs[i] = AutomationRecord{Code: MaVanBanDen(2026, i+1), Deadline: claimedAt}
	}
	n := DueSoonNotice("2026-09-29", "CB-1", recs)
	if len([]rune(n.Body)) > noticeBodyMax || !strings.Contains(n.Body, "mục khác") {
		t.Errorf("nội dung %d ký tự, muốn ≤ %d và có phần 'mục khác'", len([]rune(n.Body)), noticeBodyMax)
	}
	if n.Title != "Bạn có 200 văn bản đến sắp đến hạn xử lý" || n.Link != "/van-ban?metric=open" {
		t.Errorf("tiêu đề/đường dẫn = %q %q", n.Title, n.Link)
	}
}

// The items are the documents the body counts, each with the deadline AS STORED (same value, location
// included — nothing computed), one per code, earliest first; other kinds carry none.
func TestDueSoonItemsCopyStoredDeadlines(t *testing.T) {
	d1 := time.Date(2026, 9, 30, 9, 15, 0, 0, time.UTC)
	d2 := time.Date(2026, 9, 29, 10, 0, 0, 0, AutomationZone) // a non-UTC location is kept as read
	recs := []AutomationRecord{
		{ID: "vb-b", Code: "VB-DEN-2026-0002", Deadline: d1},
		{ID: "vb-a", Code: "VB-DEN-2026-0001", Deadline: d2},
	}
	n := DueSoonNotice("2026-09-29", "CB-1", recs)
	if n.Key != "sla_reminders:due_soon:van-ban-den:2026-09-29:CB-1" {
		t.Errorf("khoá = %q — các mục không được đổi khoá", n.Key)
	}
	want := []DueSoonItem{{Code: "VB-DEN-2026-0001", Deadline: d2}, {Code: "VB-DEN-2026-0002", Deadline: d1}}
	if len(n.DueSoonItems) != len(want) {
		t.Fatalf("mục = %+v, muốn %+v", n.DueSoonItems, want)
	}
	for i, w := range want {
		g := n.DueSoonItems[i]
		if g.Code != w.Code || g.Deadline != w.Deadline { // == on purpose: the very value stored, not an equal instant
			t.Errorf("mục %d = %+v, muốn %+v", i, g, w)
		}
	}
	if n.Title != "Bạn có 2 văn bản đến sắp đến hạn xử lý" || n.Body != "Gồm: VB-DEN-2026-0001, VB-DEN-2026-0002." {
		t.Errorf("tiêu đề/nội dung = %q %q", n.Title, n.Body)
	}

	// A code twice: one item, the earliest stored deadline of the two.
	dup := DueSoonNotice("2026-09-29", "CB-1", append(recs, AutomationRecord{ID: "vb-b2", Code: "VB-DEN-2026-0002", Deadline: d2}))
	if len(dup.DueSoonItems) != 2 || dup.DueSoonItems[1].Code != "VB-DEN-2026-0002" || !dup.DueSoonItems[1].Deadline.Equal(d2) {
		t.Errorf("mã lặp = %+v", dup.DueSoonItems)
	}

	// Only due-soon notices carry items (comms: a non-empty list on another kind is INVALID_ARGUMENT).
	r := AutomationRecord{ID: "vb-b", Code: "VB-DEN-2026-0002", Deadline: d1, HoldStartedAt: d2}
	for _, other := range []StaffNotice{
		OverdueNotice(r, "2026-09-29", []string{"CB-1"}),
		UnassignedNotice(r, []string{"CB-1"}),
		EscalationNotice(r, EscalationUnitHead, []string{"CB-1"}),
		IncomingDigestNotice("2026-W40", 1, 1, []string{"CB-1"}),
	} {
		if len(other.DueSoonItems) != 0 {
			t.Errorf("%q mang mục sắp đến hạn", other.Title)
		}
	}
}

// Past the contract's 500 the EARLIEST deadlines are kept; the title still counts every document.
func TestDueSoonItemsCapKeepsEarliest(t *testing.T) {
	const n = MaxDueSoonItems + 37
	recs := make([]AutomationRecord, 0, n)
	for i := n - 1; i >= 0; i-- { // fed latest first, so the cap cannot pass by input order
		recs = append(recs, AutomationRecord{ID: fmt.Sprintf("vb-%04d", i), Code: MaVanBanDen(2026, i+1),
			Deadline: claimedAt.Add(time.Duration(i) * time.Minute)})
	}
	got := DueSoonNotice("2026-09-29", "CB-1", recs)
	if len(got.DueSoonItems) != MaxDueSoonItems {
		t.Fatalf("%d mục, muốn %d", len(got.DueSoonItems), MaxDueSoonItems)
	}
	for i, it := range got.DueSoonItems {
		if it.Code != MaVanBanDen(2026, i+1) {
			t.Fatalf("mục %d = %s — không phải hạn sớm nhất", i, it.Code)
		}
	}
	if got.Title != fmt.Sprintf("Bạn có %d văn bản đến sắp đến hạn xử lý", n) {
		t.Errorf("tiêu đề phải đếm đủ %d: %q", n, got.Title)
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
		v := VanBanDen{HanXuLyXong: claimedAt, TrangThai: VanBanDangXuLy}
		if r.PastDeadlineAt(asOf) != v.QuaHan(asOf) {
			t.Errorf("tại %s: việc nền nói %v, sổ nói %v", asOf, r.PastDeadlineAt(asOf), v.QuaHan(asOf))
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
