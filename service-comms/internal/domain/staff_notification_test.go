package domain

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func okNotice(key string) NotificationDelivery {
	return NotificationDelivery{IdempotencyKey: key, Kind: StaffNotificationOverdue,
		RecipientCodes: []string{"CB-001"}, Title: "Việc quá hạn", Link: "/nhiem-vu/01JREC"}
}

func TestValidateDeliveries_ContractLimits(t *testing.T) {
	many := func(n int) []NotificationDelivery {
		out := make([]NotificationDelivery, n)
		for i := range out {
			out[i] = okNotice(fmt.Sprintf("k-%d", i))
		}
		return out
	}
	codes := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = fmt.Sprintf("CB-%04d", i)
		}
		return out
	}
	with := func(f func(*NotificationDelivery)) []NotificationDelivery {
		n := okNotice("k")
		f(&n)
		return []NotificationDelivery{n}
	}

	cases := []struct {
		name string
		in   []NotificationDelivery
		ok   bool
	}{
		{"one", many(1), true},
		{"hundred", many(100), true},
		{"empty", nil, false},
		{"hundred-and-one", many(101), false},
		{"key empty", with(func(n *NotificationDelivery) { n.IdempotencyKey = "" }), false},
		{"key 200", with(func(n *NotificationDelivery) { n.IdempotencyKey = strings.Repeat("k", 200) }), true},
		{"key 201", with(func(n *NotificationDelivery) { n.IdempotencyKey = strings.Repeat("k", 201) }), false},
		{"key non-ascii", with(func(n *NotificationDelivery) { n.IdempotencyKey = "khoá" }), false},
		{"key control", with(func(n *NotificationDelivery) { n.IdempotencyKey = "a\nb" }), false},
		{"key repeated", []NotificationDelivery{okNotice("same"), okNotice("same")}, false},
		{"kind unspecified", with(func(n *NotificationDelivery) { n.Kind = "" }), false},
		{"kind unknown", with(func(n *NotificationDelivery) { n.Kind = "khac" }), false},
		{"kind per-domain", with(func(n *NotificationDelivery) { n.Kind = ZaloKindPetitionOverdue }), true},
		{"kind per-domain unassigned", with(func(n *NotificationDelivery) { n.Kind = ZaloKindTaskUnassigned }), true},
		{"kind unknown domain", with(func(n *NotificationDelivery) { n.Kind = "ho-so.qua-han" }), false},
		{"kind test message", with(func(n *NotificationDelivery) { n.Kind = ZaloKindTest }), false},
		{"no recipient", with(func(n *NotificationDelivery) { n.RecipientCodes = nil }), false},
		{"blank recipient", with(func(n *NotificationDelivery) { n.RecipientCodes = []string{" "} }), false},
		{"200 recipients", with(func(n *NotificationDelivery) { n.RecipientCodes = codes(200) }), true},
		{"201 recipients", with(func(n *NotificationDelivery) { n.RecipientCodes = codes(201) }), false},
		{"201 with duplicates collapsing to 200", with(func(n *NotificationDelivery) {
			n.RecipientCodes = append(codes(200), "CB-0000")
		}), true},
		{"title empty", with(func(n *NotificationDelivery) { n.Title = "  " }), false},
		{"title 200", with(func(n *NotificationDelivery) { n.Title = strings.Repeat("ạ", 200) }), true},
		{"title 201", with(func(n *NotificationDelivery) { n.Title = strings.Repeat("ạ", 201) }), false},
		{"body 500", with(func(n *NotificationDelivery) { n.Body = strings.Repeat("ạ", 500) }), true},
		{"body 501", with(func(n *NotificationDelivery) { n.Body = strings.Repeat("ạ", 501) }), false},
		{"body newline", with(func(n *NotificationDelivery) { n.Body = "dòng một\ndòng hai" }), true},
		{"link empty", with(func(n *NotificationDelivery) { n.Link = "" }), true},
		{"link query", with(func(n *NotificationDelivery) { n.Link = "/nhiem-vu?soon=true" }), true},
		{"link absolute", with(func(n *NotificationDelivery) { n.Link = "https://evil.example/" }), false},
		{"link scheme-relative", with(func(n *NotificationDelivery) { n.Link = "//evil.example/" }), false},
		{"link backslash", with(func(n *NotificationDelivery) { n.Link = "/\\evil.example" }), false},
		{"link javascript", with(func(n *NotificationDelivery) { n.Link = "javascript:alert(1)" }), false},
		{"link relative no slash", with(func(n *NotificationDelivery) { n.Link = "nhiem-vu" }), false},
		{"link 301", with(func(n *NotificationDelivery) { n.Link = "/" + strings.Repeat("a", 300) }), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ValidateDeliveries(tc.in)
			if tc.ok && err != nil {
				t.Fatalf("bị từ chối: %v", err)
			}
			if !tc.ok && !errors.Is(err, ErrInvalidDelivery) {
				t.Fatalf("lỗi = %v, muốn ErrInvalidDelivery", err)
			}
		})
	}
}

// Every kind 0021 admits on staff_notification is accepted, and kept exactly as sent.
func TestValidateDeliveries_AcceptsEvery0021Kind(t *testing.T) {
	kinds := append([]string{StaffNotificationDueSoon, StaffNotificationOverdue, StaffNotificationEscalation,
		StaffNotificationWeeklyDigest}, ZaloReminderKinds...)
	for _, k := range kinds {
		n := okNotice("k")
		n.Kind = k
		out, err := ValidateDeliveries([]NotificationDelivery{n})
		if err != nil || out[0].Kind != k {
			t.Errorf("%s: %v / %+v", k, err, out)
		}
	}
}

func TestValidateDeliveries_TotalRecipientsBound(t *testing.T) {
	in := make([]NotificationDelivery, 6)
	for i := range in {
		n := okNotice(fmt.Sprintf("k-%d", i))
		n.RecipientCodes = make([]string, 200)
		for j := range n.RecipientCodes {
			n.RecipientCodes[j] = fmt.Sprintf("CB-%d-%d", i, j)
		}
		in[i] = n
	}
	if _, err := ValidateDeliveries(in[:5]); err != nil {
		t.Fatalf("1000 người nhận bị từ chối: %v", err)
	}
	if _, err := ValidateDeliveries(in); !errors.Is(err, ErrInvalidDelivery) {
		t.Fatalf("1200 người nhận mà không bị từ chối: %v", err)
	}
}

func TestValidateDeliveries_MessageNeverEchoesText(t *testing.T) {
	n := okNotice("k")
	n.Title = strings.Repeat("BI-MAT ", 40)
	_, err := ValidateDeliveries([]NotificationDelivery{n})
	if err == nil || strings.Contains(err.Error(), "BI-MAT") {
		t.Fatalf("lỗi = %v — không được nhắc lại nội dung bên gọi viết", err)
	}
}

func TestValidateDeliveries_CollapsesDuplicatesInOrder(t *testing.T) {
	n := okNotice("k")
	n.RecipientCodes = []string{" CB-2", "CB-1", "CB-2 "}
	out, err := ValidateDeliveries([]NotificationDelivery{n})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(out[0].RecipientCodes, ","); got != "CB-2,CB-1" {
		t.Errorf("= %q", got)
	}
}
