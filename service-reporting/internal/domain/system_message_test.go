package domain

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

// wantCatalogue is the agreed catalogue written out a second time, BY HAND, on purpose: a default
// changed in system_message.go without this line changing is a sentence on a signed report that
// moved with nobody deciding it. Source of each text: messages.py in ../vigov-require (see the
// comment on shippedMessages).
var wantCatalogue = []struct{ key, text string }{
	{"report.title", "Báo cáo điều hành"},
	{"report.block.tasks", "Nhiệm vụ"},
	{"report.block.register", "Văn bản và đơn thư"},
	{"report.block.budget", "Giải ngân ngân sách"},
	{"report.block.feedback", "Phản ánh của người dân"},
	{"report.block.economy", "Kinh tế và tài nguyên"},
	{"report.block.alerts", "Cần xử lý ngay"},
	{"report.block.ranking", "Xếp hạng bộ phận"},
	{"report.block.fiscal", "Thu - chi ngân sách xã"},
	{"report.metric.tasks.open", "Đang thực hiện"},
	{"report.metric.tasks.overdue", "Quá hạn"},
	{"report.metric.tasks.done", "Hoàn thành trong kỳ"},
	{"report.metric.tasks.on_time_percent", "Tỷ lệ đúng hạn"},
	{"report.metric.register.arrived", "Đến trong kỳ"},
	{"report.metric.register.open", "Chưa xử lý xong"},
	{"report.metric.register.overdue", "Quá hạn xử lý"},
	{"report.metric.register.petition_arrived", "Đơn thư đến trong kỳ"},
	{"report.metric.budget.disbursed_percent", "Tỷ lệ giải ngân"},
	{"report.metric.budget.time_percent", "Thời gian đã trôi qua"},
	{"report.metric.budget.behind", "Dự án chậm tiến độ"},
	{"report.metric.budget.open_issues", "Vướng mắc chưa gỡ"},
	{"report.metric.budget.disbursed_amount", "Đã giải ngân"},
	{"report.metric.feedback.received", "Tiếp nhận"},
	{"report.metric.feedback.open", "Đang xử lý"},
	{"report.metric.feedback.on_time_percent", "Đúng hạn"},
	{"report.metric.feedback.late", "Trễ hạn"},
	{"report.metric.feedback.rating", "Điểm hài lòng"},
	{"report.metric.economy.enterprises", "Doanh nghiệp"},
	{"report.metric.economy.household_businesses", "Hộ kinh doanh"},
	{"report.metric.economy.new_in_period", "Thành lập mới trong kỳ"},
	{"report.metric.economy.total", "Tổng tài nguyên trên bản đồ"},
	{"report.metric.fiscal.revenue_percent", "Thu đạt so với dự toán"},
	{"report.metric.fiscal.revenue_amount", "Tổng thu ngân sách"},
	{"report.metric.fiscal.expense_percent", "Chi đạt so với dự toán"},
	{"report.metric.fiscal.expense_amount", "Tổng chi ngân sách"},
	{"report.metric.fiscal.balance", "Cân đối thu - chi"},
	{"report.notification.week", "Báo cáo điều hành tuần đã sẵn sàng"},
	{"report.notification.month", "Báo cáo điều hành tháng đã sẵn sàng"},
}

func TestShippedMessagesHoldsThe38ReportKeysWithTheAgreedDefaults(t *testing.T) {
	// 38 = the specification's 32 + the requirement's six fiscal keys (ADR 0024, Bổ sung 29/09/2026).
	// No `zalo.*`, no `feedback.*`, no `budget.scope_notice`: those are not this service's sentences.
	got := ShippedMessages()
	if len(got) != 38 || len(wantCatalogue) != 38 {
		t.Fatalf("catalogue has %d keys (want list %d), want 38", len(got), len(wantCatalogue))
	}
	seen := map[string]bool{}
	for i, w := range wantCatalogue {
		if got[i].Key != w.key || got[i].DefaultText != w.text {
			t.Errorf("row %d = %q / %q, want %q / %q", i, got[i].Key, got[i].DefaultText, w.key, w.text)
		}
		if !strings.HasPrefix(got[i].Key, "report.") {
			t.Errorf("%s is not a report.* key", got[i].Key)
		}
		if seen[got[i].Key] {
			t.Errorf("%s listed twice", got[i].Key)
		}
		seen[got[i].Key] = true
		if got[i].Description == "" {
			t.Errorf("%s has no description — the card would print nothing under the key", w.key)
		}
		// The shipped default must itself pass the rule a commune's wording is held to.
		if _, err := NormalizeMessageText(got[i].DefaultText); err != nil {
			t.Errorf("%s: shipped default fails validation: %v", w.key, err)
		}
	}
}

func TestShippedMessagesIsACopy(t *testing.T) {
	a := ShippedMessages()
	a[0].DefaultText = "changed"
	if m, _ := LookupShippedMessage(a[0].Key); m.DefaultText == "changed" {
		t.Fatal("a caller edited the shipped default in place")
	}
}

func TestCatalogueAgreesWithMigrationCheck(t *testing.T) {
	// The CHECK in migration 0003 and the catalogue here are two lists of one fact. Reading the
	// migration is what keeps them from drifting: a key added here only would be refused by the
	// database on the first save; a key added there only would be stored and never read.
	b, err := os.ReadFile("../../migrations/0003_system_message_override.sql")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`message_key IN \(([^)]*)\)`).FindSubmatch(b)
	if m == nil {
		t.Fatal("CHECK (message_key IN (...)) not found in migration 0003")
	}
	var inSQL []string
	for _, p := range strings.Split(string(m[1]), ",") {
		inSQL = append(inSQL, strings.Trim(strings.TrimSpace(p), "'"))
	}
	var inGo []string
	for _, s := range ShippedMessages() {
		inGo = append(inGo, s.Key)
	}
	if strings.Join(inSQL, ",") != strings.Join(inGo, ",") {
		t.Errorf("migration keys %v != catalogue keys %v", inSQL, inGo)
	}
	if !strings.Contains(string(b), "BETWEEN 1 AND 1000") || MessageTextMax != 1000 {
		t.Error("length bound differs between migration 0003 and MessageTextMax")
	}
}

func TestLookupShippedMessageUnknownKey(t *testing.T) {
	for _, k := range []string{"", "budget.scope_notice", "feedback.reason_required", "zalo.header", "report.title ", "report"} {
		if _, ok := LookupShippedMessage(k); ok {
			t.Errorf("key %q accepted — reporting does not raise it", k)
		}
	}
}

func TestResolveMessageFallsBackToDefault(t *testing.T) {
	m, _ := LookupShippedMessage("report.title")

	got := ResolveMessage(m, nil)
	if got.CurrentText != m.DefaultText || got.Overridden || got.UpdatedAt != nil || got.UpdatedBy != "" {
		t.Errorf("no override: %+v, want the default and overridden=false", got)
	}

	// An override for ANOTHER key never leaks onto this one.
	got = ResolveMessage(m, &MessageOverride{Key: "report.block.tasks", Text: "x"})
	if got.CurrentText != m.DefaultText || got.Overridden {
		t.Errorf("foreign override applied: %+v", got)
	}

	at := time.Date(2026, 9, 29, 3, 0, 0, 0, time.UTC)
	got = ResolveMessage(m, &MessageOverride{ID: "o1", Key: m.Key, Text: "Báo cáo của xã.", UpdatedAt: at, UpdatedBy: "CB-00123"})
	if got.CurrentText != "Báo cáo của xã." || !got.Overridden || got.DefaultText != m.DefaultText ||
		got.UpdatedAt == nil || !got.UpdatedAt.Equal(at) || got.UpdatedBy != "CB-00123" {
		t.Errorf("override: %+v", got)
	}
}

func TestNormalizeMessageText(t *testing.T) {
	ok, err := NormalizeMessageText("  Báo cáo của xã.  ")
	if err != nil || ok != "Báo cáo của xã." {
		t.Fatalf("trim: %q, %v", ok, err)
	}
	// Counted in RUNES: 1000 Vietnamese characters are 3000 bytes and must pass.
	if _, err := NormalizeMessageText(strings.Repeat("ế", MessageTextMax)); err != nil {
		t.Errorf("%d runes refused — the bound is counting bytes: %v", MessageTextMax, err)
	}
	for name, c := range map[string]struct {
		in   string
		want error
	}{
		"empty":        {"", ErrMessageTextEmpty},
		"spaces":       {"   \t ", ErrMessageTextEmpty},
		"too long":     {strings.Repeat("a", MessageTextMax+1), ErrMessageTextTooLong},
		"newline":      {"một\nhai", ErrMessageTextControl},
		"escape":       {"a\x1b[31mb", ErrMessageTextControl},
		"zero width":   {"a​b", ErrMessageTextControl},
		"bidi":         {"a‮b", ErrMessageTextControl},
		"tag":          {"<b>ViGov</b>", ErrMessageTextMarkup},
		"greater than": {"a > b", ErrMessageTextMarkup},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NormalizeMessageText(c.in)
			if !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
			if !IsMessageInputError(err) {
				t.Error("refusal not classified as an input error — it would answer 500")
			}
		})
	}
	if IsMessageInputError(errors.New("db down")) || IsMessageInputError(ErrUnknownMessageKey) {
		t.Error("a failure or an unknown key classified as a 400")
	}
}
