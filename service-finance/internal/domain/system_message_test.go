package domain

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestShippedMessagesHoldsOnlyFinanceKeyWithTheAgreedDefault(t *testing.T) {
	// The finance half is ONE key (ADR 0024 §`loi_he_thong`). A `feedback.*` key here would be a
	// sentence this service never raises; a `report.*` key would decide an open question.
	got := ShippedMessages()
	if len(got) != 1 || got[0].Key != KeyBudgetScopeNotice {
		t.Fatalf("catalogue = %+v, want exactly budget.scope_notice", got)
	}
	const want = "Hệ thống là công cụ theo dõi và điều hành, không phải phần mềm kế toán. Số liệu phục vụ chỉ đạo, không thay thế sổ sách kế toán và không đối chiếu với Kho bạc."
	if got[0].DefaultText != want {
		t.Errorf("default = %q, want the sentence of docs/ui-ux/06-giai-ngan.md:13 with the product name dropped (ADR 0068 §13)", got[0].DefaultText)
	}
	// The shipped default must itself pass the rule a commune's wording is held to.
	if _, err := NormalizeMessageText(got[0].DefaultText); err != nil {
		t.Errorf("shipped default fails validation: %v", err)
	}
}

func TestShippedMessagesIsACopy(t *testing.T) {
	a := ShippedMessages()
	a[0].DefaultText = "changed"
	if m, _ := LookupShippedMessage(KeyBudgetScopeNotice); m.DefaultText == "changed" {
		t.Fatal("a caller edited the shipped default in place")
	}
}

func TestCatalogueAgreesWithMigrationCheck(t *testing.T) {
	// The CHECK in migration 0010 and the catalogue here are two lists of one fact. Reading the
	// migration is what keeps them from drifting: a key added here only would be refused by the
	// database on the first save; a key added there only would be stored and never read.
	b, err := os.ReadFile("../../migrations/0010_system_message_override.sql")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`message_key IN \(([^)]*)\)`).FindSubmatch(b)
	if m == nil {
		t.Fatal("CHECK (message_key IN (...)) not found in migration 0010")
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
		t.Error("length bound differs between migration 0010 and MessageTextMax")
	}
}

func TestLookupShippedMessageUnknownKey(t *testing.T) {
	for _, k := range []string{"", "feedback.reason_required", "report.title", "budget.scope_notice "} {
		if _, ok := LookupShippedMessage(k); ok {
			t.Errorf("key %q accepted — finance does not raise it", k)
		}
	}
}

func TestResolveMessageFallsBackToDefault(t *testing.T) {
	m, _ := LookupShippedMessage(KeyBudgetScopeNotice)

	got := ResolveMessage(m, nil)
	if got.CurrentText != m.DefaultText || got.Overridden || got.UpdatedAt != nil || got.UpdatedBy != "" {
		t.Errorf("no override: %+v, want the default and overridden=false", got)
	}

	// An override for ANOTHER key never leaks onto this one.
	got = ResolveMessage(m, &MessageOverride{Key: "other.key", Text: "x"})
	if got.CurrentText != m.DefaultText || got.Overridden {
		t.Errorf("foreign override applied: %+v", got)
	}

	at := time.Date(2026, 9, 28, 3, 0, 0, 0, time.UTC)
	got = ResolveMessage(m, &MessageOverride{ID: "o1", Key: m.Key, Text: "Câu của xã.", UpdatedAt: at, UpdatedBy: "CB-00123"})
	if got.CurrentText != "Câu của xã." || !got.Overridden || got.DefaultText != m.DefaultText ||
		got.UpdatedAt == nil || !got.UpdatedAt.Equal(at) || got.UpdatedBy != "CB-00123" {
		t.Errorf("override: %+v", got)
	}
}

func TestNormalizeMessageText(t *testing.T) {
	ok, err := NormalizeMessageText("  Câu của xã.  ")
	if err != nil || ok != "Câu của xã." {
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
