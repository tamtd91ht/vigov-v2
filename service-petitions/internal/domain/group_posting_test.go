package domain

// ADR 0092: every notice builder decides holding_org_unit_id and group_posting. The four cases the
// contract separates — an ordinary record with a unit, a restricted petition, a digest or per-person
// notice, a record with no unit — for both builder families (automation runner, act outbox).

import (
	"strings"
	"testing"
	"time"
)

// fakeUnit is a ULID-shaped fake `bo_phan.id`.
var fakeUnit = "01JBP" + strings.Repeat("0", 20) + "1"

func TestGroupUnitKeepsOnlyAULID(t *testing.T) {
	for in, want := range map[string]string{
		fakeUnit:                                fakeUnit,
		"":                                      "",
		"bp-1":                                  "",
		strings.ToLower(fakeUnit):               "", // comms is sent the canonical spelling only
		"01JBP" + strings.Repeat("0", 22):       "",
		"01JBP" + strings.Repeat("0", 20) + "U": "", // U is not Crockford base32
	} {
		if got := GroupUnit(in); got != want {
			t.Errorf("GroupUnit(%q) = %q, muốn %q", in, got, want)
		}
	}
}

func TestAutomationNoticesGroupFields(t *testing.T) {
	late := time.Date(2026, 9, 28, 3, 0, 0, 0, time.UTC)
	ordinary := AutomationRecord{ID: "pa-1", Code: "PA-AAAA1111", Field: "moi-truong", Deadline: late,
		OrgUnitID: fakeUnit, HoldStartedAt: late}
	restricted := ordinary
	restricted.Field, restricted.Restricted = LinhVucHanChe, true
	noUnit := AutomationRecord{ID: "nv-1", Code: "NV01", Deadline: late, AssigneeMa: "CB-1"}
	malformed := noUnit
	malformed.OrgUnitID = "bp-1"

	type want struct {
		unit string
		gp   GroupPosting
	}
	for name, c := range map[string]struct {
		n    StaffNotice
		want want
	}{
		"quá hạn, phiếu thường":           {OverdueNotice(AutomationCitizenReport, ordinary, "2026-09-29", []string{"CB-1"}), want{fakeUnit, GroupPostingAllowed}},
		"chưa phân công, phiếu thường":    {UnassignedNotice(AutomationCitizenReport, ordinary, []string{"CB-1"}), want{fakeUnit, GroupPostingAllowed}},
		"báo trễ hạn, phiếu thường":       {EscalationNotice(AutomationCitizenReport, ordinary, EscalationUnitHead, []string{"CB-1"}), want{fakeUnit, GroupPostingAllowed}},
		"quá hạn, phiếu hạn chế":          {OverdueNotice(AutomationCitizenReport, restricted, "2026-09-29", []string{"CB-1"}), want{fakeUnit, GroupPostingRestricted}},
		"chưa phân công, phiếu hạn chế":   {UnassignedNotice(AutomationCitizenReport, restricted, []string{"CB-1"}), want{fakeUnit, GroupPostingRestricted}},
		"báo trễ hạn, phiếu hạn chế":      {EscalationNotice(AutomationCitizenReport, restricted, EscalationChairman, []string{"CB-1"}), want{fakeUnit, GroupPostingRestricted}},
		"quá hạn, nhiệm vụ không bộ phận": {OverdueNotice(AutomationTask, noUnit, "2026-09-29", []string{"CB-1"}), want{"", GroupPostingAllowed}},
		"quá hạn, mã bộ phận sai dạng":    {OverdueNotice(AutomationTask, malformed, "2026-09-29", []string{"CB-1"}), want{"", GroupPostingAllowed}},
		"bản tin sắp đến hạn":             {DueSoonNotice(AutomationTask, "2026-09-29", "CB-1", []AutomationRecord{ordinary}), want{"", GroupPostingUnspecified}},
		"bản tin tuần nhiệm vụ":           {TaskDigestNotice("2026-W40", 1, 2, 3, []string{"CB-LD"}), want{"", GroupPostingUnspecified}},
		"bản tin tuần phản ánh":           {CitizenReportDigestNotice("2026-W40", CitizenReportDigestCounts{}, []string{"CB-LD"}), want{"", GroupPostingUnspecified}},
		"báo cáo định kỳ":                 {ReportReadyNotice(AutomationTask, ReportWeek, late, nil, []string{"CB-LD"}), want{"", GroupPostingUnspecified}},
	} {
		if got := (want{c.n.HoldingOrgUnitID, c.n.GroupPosting}); got != c.want {
			t.Errorf("%s: %+v, muốn %+v", name, got, c.want)
		}
	}
}

func TestActNoticesGroupFields(t *testing.T) {
	task := NhiemVu{Ma: "NV19", TieuDe: "Rà soát hồ sơ", BoPhanID: fakeUnit, NguoiThucHienMa: "CB-2", LanhDaoGiaoViecMa: "CB-LD"}
	taskNoUnit := task
	taskNoUnit.BoPhanID = ""
	petition := PhieuPhanAnh{MaTraCuu: "PA-XYZ", LinhVuc: "moi-truong", BoPhanID: fakeUnit, CanBoXuLyID: "CB-9"}
	restricted := petition
	restricted.LinhVuc = LinhVucHanChe
	petitionNoUnit := petition
	petitionNoUnit.BoPhanID = ""
	due := time.Date(2026, 10, 20, 3, 0, 0, 0, time.UTC)

	type want struct {
		unit string
		gp   GroupPosting
	}
	for name, c := range map[string]struct {
		n    ActNotice
		want want
	}{
		"18 giao nhiệm vụ":                 {TaskAssignedNotice(task, "nk-1", []string{"CB-2"}), want{fakeUnit, GroupPostingAllowed}},
		"18 giao nhiệm vụ không bộ phận":   {TaskAssignedNotice(taskNoUnit, "nk-1", []string{"CB-2"}), want{"", GroupPostingAllowed}},
		"20 chờ duyệt":                     {TaskApprovalRequestedNotice(task, "nk-2"), want{fakeUnit, GroupPostingAllowed}},
		"19 đề nghị lùi hạn — cá nhân":     {TaskExtensionRequestedNotice(task, "dn-1", due), want{"", GroupPostingUnspecified}},
		"21 nhắc tên — cá nhân":            {TaskMentionNotice(task, "nk-3", "ghi chú", []string{"CB-3"}), want{"", GroupPostingUnspecified}},
		"23 phân công phiếu":               {PetitionAssignedNotice(petition, "lg-1"), want{fakeUnit, GroupPostingAllowed}},
		"23 phân công phiếu hạn chế":       {PetitionAssignedNotice(restricted, "lg-1"), want{fakeUnit, GroupPostingRestricted}},
		"23 phân công phiếu không bộ phận": {PetitionAssignedNotice(petitionNoUnit, "lg-1"), want{"", GroupPostingAllowed}},
		"24 mở lại phiếu":                  {PetitionReopenedNotice(petition, "lg-2"), want{fakeUnit, GroupPostingAllowed}},
		"24 mở lại phiếu hạn chế":          {PetitionReopenedNotice(restricted, "lg-2"), want{fakeUnit, GroupPostingRestricted}},
	} {
		if got := (want{c.n.HoldingOrgUnitID, c.n.GroupPosting}); got != c.want {
			t.Errorf("%s: %+v, muốn %+v", name, got, c.want)
		}
	}
}
