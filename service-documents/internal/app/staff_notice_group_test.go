package app

// ADR 0092 fields 8 and 9 on every path a notice leaves this service: the act outbox (written inside
// the routing, read back by the relay) and the automation runner. What each builder DECIDES is proved
// in domain/group_posting_test.go; here, that the decision reaches the wire, and that the unit is the
// one the act moved the record INTO.

import (
	"context"
	"strings"
	"testing"

	commsv1 "github.com/vihat/vigov/core/gen/vigov/comms/v1"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

// groupUnitA / groupUnitB are ULID-shaped fake `bo_phan.id`s — comms refuses any other shape.
var (
	groupUnitA = "01JBPA" + strings.Repeat("0", 19) + "1"
	groupUnitB = "01JBPB" + strings.Repeat("0", 19) + "2"
)

// The fixture document sits in a non-ULID unit; it is routed to groupUnitA. The notice must carry the
// DESTINATION — a notice scoped to the unit the document just left reaches the wrong group.
func TestIncomingRoutingNoticeCarriesTheDestinationUnit(t *testing.T) {
	k := khoVBMau()
	k.hangDen = incomingRow()
	uc, _, ctx := dungUseCaseVanBanDen(t, k, 1)

	if _, err := uc.Chuyen(ctx, "vbd-001", YeuCauChuyenVanBan{
		DenBoPhan: groupUnitA, CanBoXuLyMa: "CB-00456", LyDo: "Thuộc thẩm quyền Địa chính",
	}, nguoiMau); err != nil {
		t.Fatalf("Chuyen: %v", err)
	}
	n, _ := outboxPayload(t, k, 0)
	if n.GetHoldingOrgUnitId() != groupUnitA || n.GetGroupPosting() != commsv1.GroupPosting_GROUP_POSTING_ALLOWED {
		t.Errorf("payload: %q / %v, muốn %q / ALLOWED", n.GetHoldingOrgUnitId(), n.GetGroupPosting(), groupUnitA)
	}
	if n.GetTitle() != "Bạn được giao xử lý văn bản đến số 7/2026" {
		t.Errorf("câu chữ đổi theo thứ tự dựng: %q", n.GetTitle())
	}
	got, ok := decodeStaffNotice(k.cau(outboxInsert)[0].args[4].([]byte))
	if !ok || got.HoldingOrgUnitID != groupUnitA || got.GroupPosting != commsv1.GroupPosting_GROUP_POSTING_ALLOWED {
		t.Errorf("relay đọc lại: %v %q / %v", ok, got.HoldingOrgUnitID, got.GroupPosting)
	}
}

// The fixture letter is held by "bp-tu-phap"; it is routed to groupUnitB.
func TestRouteLetterNoticeCarriesTheDestinationUnit(t *testing.T) {
	k := khoVBMau()
	repo := newLetterRepo(letterIn("dt-1", domain.LetterStatusNew, ""))
	dir := liveDirectory()
	dir.units[groupUnitB] = true
	uc, ctx := buildLetters(t, k, repo, dir)

	if _, err := uc.Route(ctx, "dt-1", RouteLetterRequest{ToUnitID: groupUnitB, AssigneeCode: "CB-00777",
		Reason: "Thẩm quyền Địa chính"}, clerk); err != nil {
		t.Fatalf("Route: %v", err)
	}
	n, _ := outboxPayload(t, k, 0)
	if n.GetHoldingOrgUnitId() != groupUnitB || n.GetGroupPosting() != commsv1.GroupPosting_GROUP_POSTING_ALLOWED {
		t.Errorf("payload: %q / %v, muốn %q / ALLOWED", n.GetHoldingOrgUnitId(), n.GetGroupPosting(), groupUnitB)
	}
}

// A denunciation routed to an officer: its notice is RESTRICTED — never posted to a Zalo group — while
// a complaint routed the same way stays ALLOWED.
func TestRouteDenunciationNoticeIsRestricted(t *testing.T) {
	for typ, want := range map[domain.LetterType]commsv1.GroupPosting{
		domain.LetterTypeDenunciation: commsv1.GroupPosting_GROUP_POSTING_RESTRICTED,
		domain.LetterTypeComplaint:    commsv1.GroupPosting_GROUP_POSTING_ALLOWED,
	} {
		t.Run(string(typ), func(t *testing.T) {
			k := khoVBMau()
			l := letterIn("dt-1", domain.LetterStatusNew, "")
			l.Type = typ
			repo := newLetterRepo(l)
			dir := liveDirectory()
			dir.units[groupUnitB] = true
			uc, ctx := buildLetters(t, k, repo, dir)
			if _, err := uc.Route(ctx, "dt-1", RouteLetterRequest{ToUnitID: groupUnitB, AssigneeCode: "CB-00777",
				Reason: "x"}, clerk); err != nil {
				t.Fatalf("Route: %v", err)
			}
			n, _ := outboxPayload(t, k, 0)
			if n.GetGroupPosting() != want || n.GetHoldingOrgUnitId() != groupUnitB {
				t.Errorf("payload: %v / %q, muốn %v / %q", n.GetGroupPosting(), n.GetHoldingOrgUnitId(), want, groupUnitB)
			}
		})
	}
}

// A row written before ADR 0092 carries neither field: the relay sends UNSPECIFIED and "" — no group.
func TestRelayReadsAPreGroupRowAsUnspecified(t *testing.T) {
	n, ok := decodeStaffNotice([]byte(`{"idempotency_key":"van-ban.chuyen-toi:ls-9",` +
		`"kind":"STAFF_NOTIFICATION_KIND_DOCUMENT_ASSIGNED","recipient_ma":["CB-9"],"title":"t"}`))
	if !ok || n.GroupPosting != commsv1.GroupPosting_GROUP_POSTING_UNSPECIFIED || n.HoldingOrgUnitID != "" {
		t.Errorf("dòng cũ: %v %q / %v", ok, n.HoldingOrgUnitID, n.GroupPosting)
	}
}

// End to end through Tick: what comms receives from the runner, per notice shape.
func TestAutomationGroupFieldsReachComms(t *testing.T) {
	h := newHarness(t, communeA)
	h.identity.holders[communeA] = map[string][]string{groupUnitA: {"CB-ROUTE"}}
	h.identity.leaders[communeA] = []string{"CB-LD"}
	withUnit := doc("vb-unit", 1, missed)
	withUnit.OrgUnitID, withUnit.AssigneeMa = groupUnitA, "CB-001"
	noUnit := doc("vb-none", 2, missed)
	noUnit.AssigneeMa = "CB-001"
	soon := doc("vb-soon", 3, soonDue)
	soon.OrgUnitID, soon.AssigneeMa = groupUnitA, "CB-001"
	h.incoming.byCommune[communeA] = []domain.AutomationRecord{withUnit, noUnit, soon}
	denounced := letter("dt-to-cao", 4, missed)
	denounced.OrgUnitID, denounced.AssigneeMa, denounced.Restricted = groupUnitA, "CB-001", true
	complaint := letter("dt-khieu-nai", 5, missed)
	complaint.OrgUnitID, complaint.AssigneeMa = groupUnitA, "CB-001"
	h.letters.byCommune[communeA] = []domain.AutomationRecord{denounced, complaint}
	h.claim(communeA, "sla", jobSLA)
	h.claim(communeA, "dig", jobDigest)
	h.claimLetters(communeA, "sla-letters", jobSLA)
	h.runner.Tick(context.Background())

	type fields struct {
		unit string
		gp   commsv1.GroupPosting
	}
	want := map[string]fields{
		"sla_reminders:overdue:van-ban-den:vb-unit:2026-09-29":  {groupUnitA, commsv1.GroupPosting_GROUP_POSTING_ALLOWED},
		"sla_reminders:overdue:van-ban-den:vb-none:2026-09-29":  {"", commsv1.GroupPosting_GROUP_POSTING_ALLOWED},
		"sla_reminders:due_soon:van-ban-den:2026-09-29:CB-001":  {"", commsv1.GroupPosting_GROUP_POSTING_UNSPECIFIED},
		"weekly_digest:van-ban-den:2026-W40":                    {"", commsv1.GroupPosting_GROUP_POSTING_UNSPECIFIED},
		"sla_reminders:overdue:don-thu:dt-to-cao:2026-09-29":    {groupUnitA, commsv1.GroupPosting_GROUP_POSTING_RESTRICTED},
		"sla_reminders:overdue:don-thu:dt-khieu-nai:2026-09-29": {groupUnitA, commsv1.GroupPosting_GROUP_POSTING_ALLOWED},
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
