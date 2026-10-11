package app

// ADR 0092 fields 8 and 9 on every path a notice leaves this service: the act outbox (written, stored,
// read back by the relay) and the automation runner (planned, paged, handed to comms). What each
// builder DECIDES is proved in domain/group_posting_test.go; here, that the decision reaches the wire.

import (
	"context"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	commsv1 "github.com/vihat/vigov/core/gen/vigov/comms/v1"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// groupUnitA / groupUnitB are ULID-shaped fake `bo_phan.id`s — comms refuses any other shape.
var (
	groupUnitA = "01JBPA" + strings.Repeat("0", 19) + "1"
	groupUnitB = "01JBPB" + strings.Repeat("0", 19) + "2"
)

func TestOutboxCarriesGroupFieldsToTheRelay(t *testing.T) {
	for _, c := range []struct {
		name string
		n    domain.ActNotice
		unit string
		gp   commsv1.GroupPosting
	}{
		{"phiếu thường", domain.PetitionAssignedNotice(domain.PhieuPhanAnh{MaTraCuu: "PA-XYZ", LinhVuc: "moi-truong",
			BoPhanID: groupUnitA, CanBoXuLyID: "CB-9"}, "lg-1"), groupUnitA, commsv1.GroupPosting_GROUP_POSTING_ALLOWED},
		{"phiếu hạn chế", domain.PetitionAssignedNotice(domain.PhieuPhanAnh{MaTraCuu: "PA-XYZ", LinhVuc: domain.LinhVucHanChe,
			BoPhanID: groupUnitA, CanBoXuLyID: "CB-9"}, "lg-2"), groupUnitA, commsv1.GroupPosting_GROUP_POSTING_RESTRICTED},
		{"nhắc tên", domain.TaskMentionNotice(domain.NhiemVu{Ma: "NV1", TieuDe: "x", BoPhanID: groupUnitA}, "nk-1", "ghi chú",
			[]string{"CB-3"}), "", commsv1.GroupPosting_GROUP_POSTING_UNSPECIFIED},
		{"nhiệm vụ không bộ phận", domain.TaskAssignedNotice(domain.NhiemVu{Ma: "NV2", TieuDe: "y"}, "nk-2", []string{"CB-2"}),
			"", commsv1.GroupPosting_GROUP_POSTING_ALLOWED},
	} {
		t.Run(c.name, func(t *testing.T) {
			out := &outboxFake{}
			if err := writeStaffNotice(context.Background(), nil, out, seqIDs(), c.n, "CB-ACTOR", time.Now()); err != nil {
				t.Fatal(err)
			}
			if len(out.rows) != 1 {
				t.Fatalf("%d dòng, muốn 1", len(out.rows))
			}
			var m commsv1.StaffNotification
			if err := protojson.Unmarshal(out.rows[0].Payload, &m); err != nil {
				t.Fatal(err)
			}
			if m.GetHoldingOrgUnitId() != c.unit || m.GetGroupPosting() != c.gp {
				t.Errorf("payload: %q / %v, muốn %q / %v", m.GetHoldingOrgUnitId(), m.GetGroupPosting(), c.unit, c.gp)
			}
			n, ok := noticeFromPayload(out.rows[0])
			if !ok || n.HoldingOrgUnitID != c.unit || n.GroupPosting != c.gp {
				t.Errorf("relay đọc lại: %v %q / %v", ok, n.HoldingOrgUnitID, n.GroupPosting)
			}
		})
	}
}

// A row written before ADR 0092 carries neither field: the relay sends UNSPECIFIED and "" — no group.
func TestRelayReadsAPreGroupRowAsUnspecified(t *testing.T) {
	payload := []byte(`{"idempotency_key":"phan-anh.phan-cong:lg-9","kind":"STAFF_NOTIFICATION_KIND_PETITION_ASSIGNED",` +
		`"recipient_ma":["CB-9"],"title":"t"}`)
	n, ok := noticeFromPayload(petstore.StaffNoticeRow{IdempotencyKey: "phan-anh.phan-cong:lg-9", Payload: payload})
	if !ok || n.GroupPosting != commsv1.GroupPosting_GROUP_POSTING_UNSPECIFIED || n.HoldingOrgUnitID != "" {
		t.Errorf("dòng cũ: %v %q / %v", ok, n.HoldingOrgUnitID, n.GroupPosting)
	}
}

// End to end through Tick: what comms receives from the runner, per notice shape.
func TestAutomationGroupFieldsReachComms(t *testing.T) {
	h := newHarness(t, communeA)
	h.identity.holders[communeA] = map[string]map[string][]string{
		permTaskAssign:         {groupUnitA: {"CB-ROUTE"}},
		permFeedbackAssign:     {groupUnitB: {"CB-PROUTE", "CB-DEPUTY"}},
		permFeedbackRestricted: {groupUnitB: {"CB-DEPUTY"}},
	}
	h.identity.leaders[communeA] = []string{"CB-LD"}
	h.tasks.byCommune[communeA] = []domain.AutomationRecord{
		{ID: "nv-unit", Code: "NV02", Deadline: missed, OrgUnitID: groupUnitA, AssigneeMa: "CB-001"},
		{ID: "nv-none", Code: "NV03", Deadline: missed, AssigneeMa: "CB-001"},
		{ID: "nv-soon", Code: "NV04", Deadline: soonDue, OrgUnitID: groupUnitA, AssigneeMa: "CB-001"},
	}
	h.reports.byCommune[communeA] = []domain.AutomationRecord{
		{ID: "pa-open", Code: "PA-AAAA1111", Field: "moi-truong", Deadline: missed, OrgUnitID: groupUnitB},
		{ID: "pa-restr", Code: "PA-RRRR0000", Field: domain.LinhVucHanChe, Deadline: missed, OrgUnitID: groupUnitB,
			Restricted: true},
	}
	h.claim(communeA, "sla-t", jobSLA, kindTask)
	h.claim(communeA, "sla-p", jobSLA, kindReport)
	h.claim(communeA, "dig-t", jobDigest, kindTask)
	h.runner.Tick(context.Background())

	type fields struct {
		unit string
		gp   commsv1.GroupPosting
	}
	want := map[string]fields{
		"sla_reminders:overdue:nhiem-vu:nv-unit:2026-09-29":  {groupUnitA, commsv1.GroupPosting_GROUP_POSTING_ALLOWED},
		"sla_reminders:overdue:nhiem-vu:nv-none:2026-09-29":  {"", commsv1.GroupPosting_GROUP_POSTING_ALLOWED},
		"sla_reminders:due_soon:nhiem-vu:2026-09-29:CB-001":  {"", commsv1.GroupPosting_GROUP_POSTING_UNSPECIFIED},
		"sla_reminders:overdue:phan-anh:pa-open:2026-09-29":  {groupUnitB, commsv1.GroupPosting_GROUP_POSTING_ALLOWED},
		"sla_reminders:overdue:phan-anh:pa-restr:2026-09-29": {groupUnitB, commsv1.GroupPosting_GROUP_POSTING_RESTRICTED},
		"weekly_digest:nhiem-vu:2026-W40":                    {"", commsv1.GroupPosting_GROUP_POSTING_UNSPECIFIED},
	}
	got := map[string]fields{}
	for _, d := range h.comms.delivered {
		got[d.n.IdempotencyKey] = fields{d.n.HoldingOrgUnitID, d.n.GroupPosting}
	}
	for key, w := range want {
		if g, ok := got[key]; !ok || g != w {
			t.Errorf("%s: %+v (có=%v), muốn %+v", key, g, ok, w)
		}
	}
	if len(got) != len(want) {
		t.Errorf("giao %d khoá, muốn %d: %v", len(got), len(want), got)
	}
}

// Through the real act and the fake drivers: the unit is the task's own column as the act wrote it.
func TestCreateTaskNoticeCarriesTheTaskUnit(t *testing.T) {
	k := khoNVMau()
	k.soLonNhat = 18
	uc, ctx := dungGhiNhiemVu(t, k)
	uc.giaoViec = &giaoViecGia{duocTatCa: true}
	req := taoMau()
	req.NguoiThucHienMa = maNguoiThucHien
	req.BoPhanID = groupUnitA

	if _, err := uc.Tao(ctx, req, canBoThu()); err != nil {
		t.Fatalf("giao việc: %v", err)
	}
	r := oneOutboxRow(t, k)
	if r.n.GetHoldingOrgUnitId() != groupUnitA || r.n.GetGroupPosting() != commsv1.GroupPosting_GROUP_POSTING_ALLOWED {
		t.Errorf("payload: %q / %v", r.n.GetHoldingOrgUnitId(), r.n.GetGroupPosting())
	}
}

// The petition's unit AFTER the assignment — the unit the act handed it to — never the one before.
func TestPetitionAssignmentNoticeCarriesTheNewUnit(t *testing.T) {
	k := khoPhieuMau()
	phieuChoPhanCong(k)
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	uc.giaoViec = &giaoViecGia{duocTatCa: true}

	if _, err := uc.PhanCong(ctx, maPhieuThu, YeuCauPhanCong{BoPhan: groupUnitB, CanBo: "CB-00999"},
		canBoThu(), khongQuyenHanChe); err != nil {
		t.Fatalf("phân công: %v", err)
	}
	rows := decodeOutbox(t, k.cau(outboxInsert))
	if len(rows) != 1 {
		t.Fatalf("outbox=%d, muốn 1", len(rows))
	}
	if rows[0].n.GetHoldingOrgUnitId() != groupUnitB ||
		rows[0].n.GetGroupPosting() != commsv1.GroupPosting_GROUP_POSTING_ALLOWED {
		t.Errorf("payload: %q / %v", rows[0].n.GetHoldingOrgUnitId(), rows[0].n.GetGroupPosting())
	}
}
