package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// ADR 0079 lô 5 Q13 — the Zalo "nhắc trước" lead and the due-soon items it narrows.

func dueSoonNotice(kind string, items ...DueSoonItem) NotificationDelivery {
	return NotificationDelivery{IdempotencyKey: "sla_reminders:due_soon:nhiem-vu:2026-10-08", Kind: kind,
		RecipientCodes: []string{"CB-00001"}, Title: "Bạn có 2 nhiệm vụ sắp đến hạn xử lý", DueSoonItems: items}
}

var leadNow = time.Date(2026, 10, 8, 1, 0, 0, 0, time.UTC) // 08:00 in Vietnam

func TestValidateDeliveriesKeepsDueSoonItems(t *testing.T) {
	vn := time.Date(2026, 10, 9, 17, 0, 0, 0, VietnamTime)
	out, err := ValidateDeliveries([]NotificationDelivery{dueSoonNotice(ZaloKindTaskDueSoon,
		DueSoonItem{Code: "NV-0001", Deadline: vn})})
	if err != nil {
		t.Fatal(err)
	}
	got := out[0].DueSoonItems
	if len(got) != 1 || got[0].Code != "NV-0001" || !got[0].Deadline.Equal(vn) || got[0].Deadline.Location() != time.UTC {
		t.Fatalf("items = %+v — kept as sent, in UTC", got)
	}
	// No items on any kind: valid, and nil (an old producer).
	if out, err := ValidateDeliveries([]NotificationDelivery{dueSoonNotice(ZaloKindTaskOverdue)}); err != nil ||
		out[0].DueSoonItems != nil {
		t.Fatalf("no items: %+v %v", out, err)
	}
}

func TestValidateDeliveriesRefusesBadDueSoonItems(t *testing.T) {
	ok := DueSoonItem{Code: "NV-0001", Deadline: leadNow}
	many := make([]DueSoonItem, MaxDueSoonItems+1)
	for i := range many {
		many[i] = DueSoonItem{Code: strings.Repeat("A", 1+i%90) + string(rune('a'+i/90)), Deadline: leadNow}
	}
	cases := map[string]NotificationDelivery{
		"overdue kind":     dueSoonNotice(ZaloKindPetitionOverdue, ok),
		"escalation kind":  dueSoonNotice(StaffNotificationEscalation, ok),
		"weekly digest":    dueSoonNotice(StaffNotificationWeeklyDigest, ok),
		"bell-only kind":   dueSoonNotice(StaffNotificationDisbursementMention, ok),
		"501 items":        dueSoonNotice(ZaloKindTaskDueSoon, many...),
		"empty code":       dueSoonNotice(ZaloKindTaskDueSoon, DueSoonItem{Deadline: leadNow}),
		"101 characters":   dueSoonNotice(ZaloKindTaskDueSoon, DueSoonItem{Code: strings.Repeat("Đ", 101), Deadline: leadNow}),
		"tab in code":      dueSoonNotice(ZaloKindTaskDueSoon, DueSoonItem{Code: "NV\t1", Deadline: leadNow}),
		"invalid UTF-8":    dueSoonNotice(ZaloKindTaskDueSoon, DueSoonItem{Code: "NV-\xff", Deadline: leadNow}),
		"repeated code":    dueSoonNotice(ZaloKindTaskDueSoon, ok, ok),
		"deadline not set": dueSoonNotice(ZaloKindTaskDueSoon, DueSoonItem{Code: "NV-0001"}),
	}
	for name, n := range cases {
		_, err := ValidateDeliveries([]NotificationDelivery{dueSoonNotice(ZaloKindDocumentDueSoon), n})
		if !errors.Is(err, ErrInvalidDelivery) {
			t.Errorf("%s: %v, want ErrInvalidDelivery for the whole batch", name, err)
			continue
		}
		if strings.Contains(err.Error(), "NV-") || strings.Contains(err.Error(), "ĐĐ") {
			t.Errorf("%s: the refusal echoes a code: %v", name, err)
		}
	}
	// The bounds themselves pass: 500 items, a 100-character code.
	edge := many[:MaxDueSoonItems]
	edge[0] = DueSoonItem{Code: strings.Repeat("Đ", MaxDueSoonItemCodeLen), Deadline: leadNow}
	if _, err := ValidateDeliveries([]NotificationDelivery{dueSoonNotice(StaffNotificationDueSoon, edge...)}); err != nil {
		t.Errorf("500 items with a 100-character code refused: %v", err)
	}
}

func TestNormalizeZaloChannelSettingDueSoonDays(t *testing.T) {
	base := ZaloChannelSetting{QuietStartMinute: 21 * 60, QuietEndMinute: 6 * 60}
	for _, d := range []int{MinDueSoonDays, 7, MaxDueSoonDays} {
		in := base
		v := d
		in.DueSoonDays = &v
		if out, err := NormalizeZaloChannelSetting(in); err != nil || *out.DueSoonDays != d {
			t.Errorf("%d days: %+v %v", d, out, err)
		}
	}
	if out, err := NormalizeZaloChannelSetting(base); err != nil || out.DueSoonDays != nil {
		t.Errorf("no lead: %+v %v", out, err)
	}
	for _, d := range []int{0, -1, 15} {
		in := base
		v := d
		in.DueSoonDays = &v
		var se *ZaloSettingError
		if _, err := NormalizeZaloChannelSetting(in); !errors.As(err, &se) || se.Code != "due_soon_days_out_of_range" {
			t.Errorf("%d days: %v", d, err)
		}
	}
	// A changed lead is not a no-op save.
	a, b := base, base
	three := 3
	b.DueSoonDays = &three
	if a.Same(b) || b.Same(a) {
		t.Error("Same ignores due_soon_days — a lead change would write no row and no audit entry")
	}
}

func TestZaloDueSoonCutoffAndNarrowing(t *testing.T) {
	cut := ZaloDueSoonCutoff(leadNow, 2)
	if want := time.Date(2026, 10, 10, 1, 0, 0, 0, time.UTC); !cut.Equal(want) {
		t.Fatalf("cut-off = %v, want %v (two calendar days, Vietnam)", cut, want)
	}
	items := []DueSoonItem{
		{Code: "past", Deadline: leadNow.Add(-time.Minute)},      // passed while the message waited: kept, not relabelled
		{Code: "at", Deadline: cut},                              // exactly the cut-off: kept
		{Code: "after", Deadline: cut.Add(time.Second)},          // one second beyond: dropped
		{Code: "inside", Deadline: leadNow.Add(5 * time.Minute)}, // kept
	}
	got := NarrowDueSoonItems(items, cut)
	if len(got) != 3 || got[0].Code != "past" || got[1].Code != "at" || got[2].Code != "inside" {
		t.Fatalf("kept = %+v", got)
	}
	if len(NarrowDueSoonItems(items[2:3], cut)) != 0 {
		t.Error("an item beyond the lead was kept")
	}
}

func TestZaloDueSoonTextWordsTheKeptItemsOnly(t *testing.T) {
	kept := []DueSoonItem{{Code: "NV-0003", Deadline: leadNow}, {Code: "NV-0001", Deadline: leadNow}}
	got := ZaloDueSoonText(ZaloKindTaskDueSoon, kept, "https://xa.example.gov.vn/nhiem-vu?soon=true")
	want := "Bạn có 2 nhiệm vụ sắp đến hạn xử lý\nGồm: NV-0001, NV-0003.\nhttps://xa.example.gov.vn/nhiem-vu?soon=true"
	if got != want {
		t.Fatalf("text =\n%q\nwant\n%q", got, want)
	}
	for kind, noun := range map[string]string{
		ZaloKindPetitionDueSoon:  "phiếu phản ánh",
		ZaloKindDocumentDueSoon:  "văn bản",
		StaffNotificationDueSoon: "việc",
	} {
		if s := ZaloDueSoonText(kind, kept[:1], ""); s != "Bạn có 1 "+noun+" sắp đến hạn xử lý\nGồm: NV-0003." {
			t.Errorf("%s: %q", kind, s)
		}
	}
	// Past the bell body's length the list is cut the producers' way.
	long := make([]DueSoonItem, 60)
	for i := range long {
		long[i] = DueSoonItem{Code: "PA-" + strings.Repeat("X", 10) + string(rune('A'+i%26)) + string(rune('a'+i/26)), Deadline: leadNow}
	}
	s := ZaloDueSoonText(ZaloKindPetitionDueSoon, long, "")
	body := s[strings.Index(s, "\n")+1:]
	if !strings.HasPrefix(s, "Bạn có 60 phiếu phản ánh") || !strings.Contains(body, " mục khác.") ||
		len([]rune(body)) > MaxNotificationBodyLen {
		t.Fatalf("long list: %q", s)
	}
}
