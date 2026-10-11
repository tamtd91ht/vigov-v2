package domain

import (
	"errors"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The group kind list is written twice — here, and in 0029's three CHECKs. They must be the same fourteen,
// or Go would offer a box the database refuses (or the reverse).
func TestZaloGroupKindsMatchMigration0029(t *testing.T) {
	sql, err := os.ReadFile("../../migrations/0029_zalo_group_link.sql")
	if err != nil {
		t.Fatal(err)
	}
	block := regexp.MustCompile(`(?s)CONSTRAINT zalo_group_link_kinds_known\s+CHECK \(kinds <@ ARRAY\[(.*?)\]::text\[\]\)`).
		FindSubmatch(sql)
	if block == nil {
		t.Fatal("zalo_group_link_kinds_known not found in 0029")
	}
	var inSQL []string
	for _, m := range regexp.MustCompile(`'([a-z.-]+)'`).FindAllSubmatch(block[1], -1) {
		inSQL = append(inSQL, string(m[1]))
	}
	got := slices.Clone(ZaloGroupKinds)
	slices.Sort(got)
	slices.Sort(inSQL)
	if !slices.Equal(got, inSQL) {
		t.Fatalf("Go %v ≠ 0029 %v", got, inSQL)
	}
	for _, k := range ZaloGroupKinds {
		if !slices.Contains(ZaloReminderKinds, k) {
			t.Errorf("%q is not a kind a commune can switch on", k)
		}
	}
	for _, never := range []string{ZaloKindTaskDueSoon, ZaloKindWeeklyDigest, ZaloKindTaskMention,
		ZaloKindTaskExtensionRequested, ZaloKindReportReady, ZaloKindAnnouncementPublished,
		StaffNotificationDisbursementMention, StaffNotificationOverdue} {
		if IsZaloGroupKind(never) {
			t.Errorf("%q reaches a group", never)
		}
	}
}

func TestOrgUnitID(t *testing.T) {
	for s, want := range map[string]bool{
		"01JBPHAN00000000000000000A": true,
		"01jbphan00000000000000000a": false, // ValidOrgUnitID is the column's upper-case shape
		"01JBPHAN0000000000000000I0": false, // I is not Crockford
		"01JBPHAN00000000000000000":  false,
		"bp-1":                       false,
		"":                           false,
	} {
		if ValidOrgUnitID(s) != want {
			t.Errorf("ValidOrgUnitID(%q) = %v", s, !want)
		}
	}
	if u, ok := NormalizeOrgUnitID("01jbphan00000000000000000a"); !ok || u != "01JBPHAN00000000000000000A" {
		t.Errorf("normalise lower case → %q %v", u, ok)
	}
}

func TestNormalizeZaloGroupKinds(t *testing.T) {
	on := ZaloChannelSetting{Saved: true, IsEnabled: true, Kinds: []string{ZaloKindTaskAssigned, ZaloKindPetitionAssigned,
		ZaloKindTaskDueSoon}}
	got, err := NormalizeZaloGroupKinds([]string{ZaloKindPetitionAssigned, ZaloKindTaskAssigned, ZaloKindTaskAssigned}, on)
	if err != nil || strings.Join(got, ",") != ZaloKindTaskAssigned+","+ZaloKindPetitionAssigned {
		t.Fatalf("= %v %v", got, err)
	}
	for _, c := range []struct {
		kinds []string
		s     ZaloChannelSetting
		want  error
	}{
		{nil, on, ErrZaloGroupKindsRequired},
		{[]string{ZaloKindTaskDueSoon}, on, ErrZaloGroupKindUnknown},
		{[]string{ZaloKindDocumentAssigned}, on, ErrZaloGroupKindNotEnabled},
		{[]string{ZaloKindTaskAssigned}, ZaloChannelSetting{Saved: true, Kinds: on.Kinds}, ErrZaloGroupKindNotEnabled},
	} {
		if _, err := NormalizeZaloGroupKinds(c.kinds, c.s); !errors.Is(err, c.want) || !errors.Is(err, ErrZaloGroupInvalid) {
			t.Errorf("%v → %v, want %v", c.kinds, err, c.want)
		}
	}
}

func TestValidateDeliveriesHoldingOrgUnit(t *testing.T) {
	n := NotificationDelivery{IdempotencyKey: "k", Kind: ZaloKindTaskAssigned, RecipientCodes: []string{"CB-1"},
		Title: "t", HoldingOrgUnitID: "01jbphan00000000000000000a", GroupPosting: GroupPostingAllowed}
	out, err := ValidateDeliveries([]NotificationDelivery{n})
	if err != nil || out[0].HoldingOrgUnitID != "01JBPHAN00000000000000000A" || out[0].GroupPosting != GroupPostingAllowed {
		t.Fatalf("= %+v %v", out, err)
	}
	n.HoldingOrgUnitID = "bp-van-phong"
	if _, err := ValidateDeliveries([]NotificationDelivery{n}); !errors.Is(err, ErrInvalidDelivery) ||
		strings.Contains(err.Error(), "bp-van-phong") {
		t.Fatalf("not a ULID → %v (and never echoed)", err)
	}
}

func TestZaloGroupRepliesNameNoOne(t *testing.T) {
	for _, s := range []string{ZaloReplyGroupCodeRefused, ZaloReplyGroupAlreadyLinked, ZaloReplyGroupPairedPersonally,
		ZaloReplyPrivateGroupCode, ZaloReplyGroupLinked(true, ""), ZaloReplyGroupLinked(false, "Văn phòng")} {
		if s == "" || strings.ContainsAny(s, "`") {
			t.Errorf("reply %q", s)
		}
	}
	if len([]rune(ZaloGroupEndedByBotChange)) > 200 {
		t.Error("the bot-change reason exceeds 0029's 200 characters")
	}
}
