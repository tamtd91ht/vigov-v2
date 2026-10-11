package domain

// ADR 0092: every notice builder decides holding_org_unit_id and group_posting — an ordinary record
// with a unit, a restricted record (a denunciation letter), a record with no unit, a digest or
// per-person notice.

import (
	"strings"
	"testing"
	"time"
)

// fakeUnit is a ULID-shaped fake `bo_phan.id`.
var fakeUnit = "01JBP" + strings.Repeat("0", 20) + "1"

func TestGroupUnitKeepsOnlyAULID(t *testing.T) {
	for in, want := range map[string]string{
		fakeUnit:                  fakeUnit,
		"":                        "",
		"bp-van-phong":            "",
		strings.ToLower(fakeUnit): "",
		fakeUnit + "0":            "",
	} {
		if got := GroupUnit(in); got != want {
			t.Errorf("GroupUnit(%q) = %q, muốn %q", in, got, want)
		}
	}
}

func TestAutomationNoticesGroupFields(t *testing.T) {
	late := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	withUnit := AutomationRecord{ID: "vb-1", Code: MaVanBanDen(2026, 1), Deadline: late, OrgUnitID: fakeUnit, HoldStartedAt: late}
	noUnit := AutomationRecord{ID: "vb-2", Code: MaVanBanDen(2026, 2), Deadline: late, AssigneeMa: "CB-1"}
	denunciation := withUnit
	denunciation.Restricted = true

	type want struct {
		unit string
		gp   GroupPosting
	}
	for name, c := range map[string]struct {
		n    StaffNotice
		want want
	}{
		"văn bản đến quá hạn":           {OverdueNotice(withUnit, "2026-09-29", []string{"CB-1"}), want{fakeUnit, GroupPostingAllowed}},
		"văn bản đến chưa phân công":    {UnassignedNotice(withUnit, []string{"CB-1"}), want{fakeUnit, GroupPostingAllowed}},
		"văn bản đến báo trễ hạn":       {EscalationNotice(withUnit, EscalationUnitHead, []string{"CB-1"}), want{fakeUnit, GroupPostingAllowed}},
		"đơn thư quá hạn":               {LetterNotices.Overdue(withUnit, "2026-09-29", []string{"CB-1"}), want{fakeUnit, GroupPostingAllowed}},
		"đơn thư báo trễ hạn":           {LetterNotices.Escalation(withUnit, EscalationChairman, []string{"CB-1"}), want{fakeUnit, GroupPostingAllowed}},
		"đơn thư tố cáo quá hạn":        {LetterNotices.Overdue(denunciation, "2026-09-29", []string{"CB-1"}), want{fakeUnit, GroupPostingRestricted}},
		"đơn thư tố cáo chưa phân công": {LetterNotices.Unassigned(denunciation, []string{"CB-1"}), want{fakeUnit, GroupPostingRestricted}},
		"đơn thư tố cáo báo trễ hạn":    {LetterNotices.Escalation(denunciation, EscalationUnitHead, []string{"CB-1"}), want{fakeUnit, GroupPostingRestricted}},
		"văn bản đến không bộ phận":     {OverdueNotice(noUnit, "2026-09-29", []string{"CB-1"}), want{"", GroupPostingAllowed}},
		"bản tin sắp đến hạn":           {DueSoonNotice("2026-09-29", "CB-1", []AutomationRecord{withUnit}), want{"", GroupPostingUnspecified}},
		"bản tin tuần":                  {IncomingDigestNotice("2026-W40", 1, 2, []string{"CB-LD"}), want{"", GroupPostingUnspecified}},
		"bản tin tuần đơn thư":          {LetterNotices.Digest("2026-W40", 1, 2, []string{"CB-LD"}), want{"", GroupPostingUnspecified}},
		"báo cáo định kỳ":               {ScheduledReportNotice(ReportWeek, "2026-09-28", 3, []string{"CB-LD"}), want{"", GroupPostingUnspecified}},
	} {
		if got := (want{c.n.HoldingOrgUnitID, c.n.GroupPosting}); got != c.want {
			t.Errorf("%s: %+v, muốn %+v", name, got, c.want)
		}
	}
}

func TestActNoticesGroupFields(t *testing.T) {
	doc := VanBanDen{SoVaoSo: 7, Nam: 2026, BoPhanDangGiu: fakeUnit}
	docNoUnit := VanBanDen{SoVaoSo: 8, Nam: 2026}
	letter := CitizenLetter{Number: 5, Year: 2026, Type: LetterTypeComplaint, HoldingUnitID: fakeUnit}
	letterNoUnit := CitizenLetter{Number: 6, Year: 2026, Type: LetterTypeComplaint}
	denunciation := CitizenLetter{Number: 7, Year: 2026, Type: LetterTypeDenunciation, HoldingUnitID: fakeUnit}

	type want struct {
		unit string
		gp   GroupPosting
	}
	for name, c := range map[string]struct {
		n    ActNotice
		want want
	}{
		"văn bản đến":               {IncomingAssignedNotice(doc, "ls-1", "CB-1"), want{fakeUnit, GroupPostingAllowed}},
		"văn bản đến không bộ phận": {IncomingAssignedNotice(docNoUnit, "ls-2", "CB-1"), want{"", GroupPostingAllowed}},
		"đơn thư":                   {LetterAssignedNotice(letter, "lg-1", "CB-1"), want{fakeUnit, GroupPostingAllowed}},
		"đơn thư tố cáo":            {LetterAssignedNotice(denunciation, "lg-3", "CB-1"), want{fakeUnit, GroupPostingRestricted}},
		"đơn thư không bộ phận":     {LetterAssignedNotice(letterNoUnit, "lg-2", "CB-1"), want{"", GroupPostingAllowed}},
	} {
		if got := (want{c.n.HoldingOrgUnitID, c.n.GroupPosting}); got != c.want {
			t.Errorf("%s: %+v, muốn %+v", name, got, c.want)
		}
	}
}

// Only a letter whose type protects the sender is restricted; a complaint stays allowed.
func TestOnlyDenunciationLettersAreRestricted(t *testing.T) {
	if !LetterTypeDenunciation.ProtectsIdentity() || LetterTypeComplaint.ProtectsIdentity() {
		t.Fatal("ProtectsIdentity đổi nghĩa — ca kiểm này không còn chứng minh gì")
	}
	for typ, want := range map[LetterType]GroupPosting{
		LetterTypeDenunciation: GroupPostingRestricted,
		LetterTypeComplaint:    GroupPostingAllowed,
	} {
		if got := LetterAssignedNotice(CitizenLetter{Number: 1, Year: 2026, Type: typ}, "lg-1", "CB-1").GroupPosting; got != want {
			t.Errorf("%s: %v, muốn %v", typ, got, want)
		}
	}
}
