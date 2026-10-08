package domain

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// A SWITCHED-OFF OVERRIDE RESOLVES TO THE DEFAULT and still carries the commune's words for "Bật lại"
// (migration 0017 §A).
func TestResolveMessageSwitchedOff(t *testing.T) {
	m, _ := LookupShippedMessage(KeyBudgetScopeNotice)
	at := time.Date(2026, 10, 8, 3, 0, 0, 0, time.UTC)
	got := ResolveMessage(m, &MessageOverride{ID: "o", Key: m.Key, Text: "Câu của xã.", UpdatedAt: at, UpdatedBy: "CB-1", Inactive: true})
	if got.CurrentText != m.DefaultText || got.Active || !got.Overridden || got.OverrideText != "Câu của xã." {
		t.Errorf("switched off: %+v", got)
	}
	if got.Group != GroupDisbursement || got.Origin != OriginShipped {
		t.Errorf("group/origin = %q/%q", got.Group, got.Origin)
	}
	on := ResolveMessage(m, &MessageOverride{ID: "o", Key: m.Key, Text: "Câu của xã.", UpdatedAt: at})
	if on.CurrentText != "Câu của xã." || !on.Active {
		t.Errorf("on: %+v", on)
	}
	if def := ResolveMessage(m, nil); !def.Active || def.OverrideText != "" {
		t.Errorf("default: %+v", def)
	}
}

func TestNormalizeCustomKey(t *testing.T) {
	g, k, err := NormalizeCustomKey(" giai-ngan ", " giai-ngan.loi-nhac.sang ")
	if err != nil || g != "giai-ngan" || k != "giai-ngan.loi-nhac.sang" {
		t.Fatalf("%q %q %v", g, k, err)
	}
	for name, c := range map[string]struct {
		group, key string
		want       error
	}{
		"petitions group": {"phan-anh", "phan-anh.x", ErrCustomGroupUnknown},
		"shared group":    {"chung", "chung.x", ErrCustomGroupUnknown},
		"report group":    {"bao-cao", "bao-cao.x", ErrCustomGroupUnknown},
		"no prefix":       {"giai-ngan", "x", ErrCustomKeyPrefix},
		"other prefix":    {"giai-ngan", "phan-anh.x", ErrCustomKeyPrefix},
		"prefix only":     {"giai-ngan", "giai-ngan.", ErrCustomKeyShape},
		"double dot":      {"giai-ngan", "giai-ngan..x", ErrCustomKeyShape},
		"space":           {"giai-ngan", "giai-ngan.loi nhac", ErrCustomKeyShape},
		"diacritic":       {"giai-ngan", "giai-ngan.lời", ErrCustomKeyShape},
		"too long":        {"giai-ngan", "giai-ngan." + strings.Repeat("a", CustomMessageKeyMax), ErrCustomKeyShape},
		"shipped budget":  {"giai-ngan", KeyBudgetScopeNotice, ErrCustomKeyShape}, // `_` is outside the shape
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := NormalizeCustomKey(c.group, c.key)
			if !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
			if !IsCustomMessageInputError(err) {
				t.Error("not classified as a 400")
			}
		})
	}
	// A shipped key is "taken", which is a 409, not a 400.
	if IsCustomMessageInputError(ErrMessageCodeTaken) {
		t.Error("a taken code classified as a 400")
	}
}

// The group set and the key rule agree with migration 0017's CHECKs.
func TestCustomGroupsAgreeWithMigration(t *testing.T) {
	b, e := os.ReadFile("../../migrations/0017_system_message_switch_custom_messages_catalogue_color.sql")
	if e != nil {
		t.Fatal(e)
	}
	sql := string(b)
	if !strings.Contains(sql, "CHECK (group_code IN ('giai-ngan'))") {
		t.Error("group CHECK changed — customGroups must follow it")
	}
	if !strings.Contains(sql, "message_key ~ '"+customKeyShape.String()+"'") {
		t.Error("key shape CHECK and customKeyShape disagree")
	}
}

func TestNormalizeDescriptionAndReason(t *testing.T) {
	if d, e := NormalizeDescription("   "); e != nil || d != "" {
		t.Errorf("blank description = %q, %v — want none", d, e)
	}
	if _, e := NormalizeDescription("a\nb"); !errors.Is(e, ErrDescriptionInvalid) {
		t.Errorf("control: %v", e)
	}
	if r, e := NormalizeCustomDeleteReason("  lý do  "); e != nil || r != "lý do" {
		t.Errorf("reason = %q, %v", r, e)
	}
	if _, e := NormalizeCustomDeleteReason(strings.Repeat("a", CustomMessageDeleteReasonMax+1)); !errors.Is(e, ErrCustomDeleteReasonTooLong) {
		t.Errorf("too long: %v", e)
	}
}
