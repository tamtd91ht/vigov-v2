package grpc

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/vihat/vigov/core/audit"
	commsv1 "github.com/vihat/vigov/core/gen/vigov/comms/v1"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

const testTenant = tenant.ID("01JD8ZQK9M3NPXR7TVWYB2C4EF")

// fakeDeliverer runs the REAL domain validation — so the status table below is the contract's, not
// a fake's — and then answers what the test asks for.
type fakeDeliverer struct {
	calls     int
	lastIn    []domain.NotificationDelivery
	lastActor audit.Actor
	already   bool
	fail      error
}

func (f *fakeDeliverer) Deliver(ctx context.Context, in []domain.NotificationDelivery, actor audit.Actor) (
	[]domain.DeliveryOutcome, error) {
	f.calls++
	f.lastIn, f.lastActor = in, actor
	_ = tenant.MustFrom(ctx)
	clean, err := domain.ValidateDeliveries(in)
	if err != nil {
		return nil, err
	}
	if f.fail != nil {
		return nil, f.fail
	}
	out := make([]domain.DeliveryOutcome, 0, len(clean))
	for _, n := range clean {
		o := domain.DeliveryOutcome{IdempotencyKey: n.IdempotencyKey, Created: len(n.RecipientCodes)}
		if f.already {
			o = domain.DeliveryOutcome{IdempotencyKey: n.IdempotencyKey, AlreadyDelivered: len(n.RecipientCodes)}
		}
		out = append(out, o)
	}
	return out, nil
}

func newTestServer(f *fakeDeliverer) *Server {
	return NewServer(Deps{Notifications: f, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
}

func goodRequest() *commsv1.DeliverStaffNotificationsRequest {
	return &commsv1.DeliverStaffNotificationsRequest{Notifications: []*commsv1.StaffNotification{
		{IdempotencyKey: "sla_reminders:due_soon:NHIEM_VU:2026-09-29",
			Kind:        commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DUE_SOON,
			RecipientMa: []string{"CB-001", "CB-002"}, Title: "Bạn có 2 việc sắp đến hạn", Link: "/nhiem-vu?soon=true"},
		{IdempotencyKey: "weekly_digest:NHIEM_VU:2026-W40",
			Kind:        commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_WEEKLY_DIGEST,
			RecipientMa: []string{"CB-100"}, Title: "Bản tin đầu tuần"},
	}}
}

func ctxWithPeer() context.Context {
	ctx := tenant.Into(context.Background(), testTenant)
	return peer.NewContext(ctx, &peer.Peer{Addr: &net.TCPAddr{IP: net.ParseIP("10.4.5.6"), Port: 50123}})
}

func TestDeliverStatusTable(t *testing.T) {
	mut := func(f func(*commsv1.DeliverStaffNotificationsRequest)) *commsv1.DeliverStaffNotificationsRequest {
		r := goodRequest()
		f(r)
		return r
	}
	cases := []struct {
		name string
		req  *commsv1.DeliverStaffNotificationsRequest
		fail error
		want codes.Code
	}{
		{"ok", goodRequest(), nil, codes.OK},
		{"empty", &commsv1.DeliverStaffNotificationsRequest{}, nil, codes.InvalidArgument},
		{"kind unspecified", mut(func(r *commsv1.DeliverStaffNotificationsRequest) {
			r.Notifications[1].Kind = commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_UNSPECIFIED
		}), nil, codes.InvalidArgument},
		{"kind from a newer build", mut(func(r *commsv1.DeliverStaffNotificationsRequest) {
			r.Notifications[0].Kind = commsv1.StaffNotificationKind(99)
		}), nil, codes.InvalidArgument},
		{"absolute link", mut(func(r *commsv1.DeliverStaffNotificationsRequest) {
			r.Notifications[0].Link = "https://evil.example"
		}), nil, codes.InvalidArgument},
		{"no recipient", mut(func(r *commsv1.DeliverStaffNotificationsRequest) {
			r.Notifications[1].RecipientMa = nil
		}), nil, codes.InvalidArgument},
		{"key twice", mut(func(r *commsv1.DeliverStaffNotificationsRequest) {
			r.Notifications[1].IdempotencyKey = r.Notifications[0].IdempotencyKey
		}), nil, codes.InvalidArgument},
		{"store down", goodRequest(), errors.New("fake: pool exhausted"), codes.Internal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeDeliverer{fail: tc.fail}
			_, err := newTestServer(f).DeliverStaffNotifications(ctxWithPeer(), tc.req)
			if got := status.Code(err); got != tc.want {
				t.Fatalf("mã = %v, muốn %v (lỗi: %v)", got, tc.want, err)
			}
			if tc.fail != nil && strings.Contains(err.Error(), "pool exhausted") {
				t.Error("nguyên nhân nội bộ lọt qua ranh giới dịch vụ")
			}
		})
	}
}

func TestDeliverInvalidMessageNeverEchoesTitle(t *testing.T) {
	r := goodRequest()
	r.Notifications[0].Title = strings.Repeat("NOI-DUNG-BEN-GOI ", 20)
	_, err := newTestServer(&fakeDeliverer{}).DeliverStaffNotifications(ctxWithPeer(), r)
	if status.Code(err) != codes.InvalidArgument || strings.Contains(err.Error(), "NOI-DUNG-BEN-GOI") {
		t.Fatalf("lỗi = %v", err)
	}
}

func TestDeliverMapsOutcomesAndKindsAndUsesSystemActor(t *testing.T) {
	f := &fakeDeliverer{}
	res, err := newTestServer(f).DeliverStaffNotifications(ctxWithPeer(), goodRequest())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.GetItems()) != 2 || res.GetItems()[0].GetCreated() != 2 || res.GetItems()[1].GetCreated() != 1 ||
		res.GetItems()[0].GetIdempotencyKey() != "sla_reminders:due_soon:NHIEM_VU:2026-09-29" {
		t.Fatalf("= %+v", res.GetItems())
	}
	if f.lastIn[0].Kind != domain.StaffNotificationDueSoon || f.lastIn[1].Kind != domain.StaffNotificationWeeklyDigest {
		t.Errorf("ánh xạ loại sai: %q, %q", f.lastIn[0].Kind, f.lastIn[1].Kind)
	}
	if a := f.lastActor; a.ID != audit.SystemActor || a.Kind != "system" || a.IP != "10.4.5.6" {
		t.Errorf("chủ thể = %+v, muốn chủ thể hệ thống kèm IP của bên gọi", a)
	}
}

// Every wire value of comms.proto maps to exactly the value 0021 admits; 1–4 keep their old values
// (the read-time map routes them), 5–16 are stored per-domain as-is, 17 is the bell-only mention (0023),
// 18–25 the act notices (0025).
func TestKindFromWireIsTheProtoTable(t *testing.T) {
	want := map[commsv1.StaffNotificationKind]string{
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DUE_SOON:             "sap-den-han",
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_OVERDUE:              "qua-han",
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_ESCALATION:           "leo-thang",
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_WEEKLY_DIGEST:        "ban-tin-tuan",
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_DUE_SOON:        "nhiem-vu.sap-den-han",
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_OVERDUE:         "nhiem-vu.qua-han",
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_UNASSIGNED:      "nhiem-vu.chua-cu-nguoi",
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_ESCALATION:      "nhiem-vu.leo-thang",
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DOCUMENT_DUE_SOON:    "van-ban.sap-den-han",
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DOCUMENT_OVERDUE:     "van-ban.qua-han",
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DOCUMENT_UNASSIGNED:  "van-ban.chua-cu-nguoi",
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DOCUMENT_ESCALATION:  "van-ban.leo-thang",
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_PETITION_DUE_SOON:    "phan-anh.sap-den-han",
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_PETITION_OVERDUE:     "phan-anh.qua-han",
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_PETITION_UNASSIGNED:  "phan-anh.chua-cu-nguoi",
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_PETITION_ESCALATION:  "phan-anh.leo-thang",
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DISBURSEMENT_MENTION: "giai-ngan.nhac-ten",
		// 18–25: the act notices of 0025 (ADR 0086), stored as their value as-is.
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_ASSIGNED:            "nhiem-vu.giao-moi",
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_EXTENSION_REQUESTED: "nhiem-vu.de-nghi-lui-han",
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_APPROVAL_REQUESTED:  "nhiem-vu.cho-duyet",
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_MENTION:             "nhiem-vu.nhac-ten",
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DOCUMENT_ASSIGNED:        "van-ban.chuyen-toi",
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_PETITION_ASSIGNED:        "phan-anh.phan-cong",
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_PETITION_REOPENED:        "phan-anh.mo-lai",
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_REPORT_READY:             "bao-cao.san-sang",
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_UNSPECIFIED:              "",
		commsv1.StaffNotificationKind(26):                                              "",
		commsv1.StaffNotificationKind(-1):                                              "",
	}
	// `thong-bao.moi` has no wire value: no input maps to it (comms issues announcements itself).
	for v := range commsv1.StaffNotificationKind_name {
		if kindFromWire(commsv1.StaffNotificationKind(v)) == domain.ZaloKindAnnouncementPublished {
			t.Errorf("wire value %d maps to thong-bao.moi — another service could forge an announcement notice", v)
		}
	}
	// Every enum value the generated code knows is in the table — an 18th added to the proto without a
	// mapping here turns this red instead of being refused in production.
	for v := range commsv1.StaffNotificationKind_name {
		if _, ok := want[commsv1.StaffNotificationKind(v)]; !ok {
			t.Errorf("giá trị %d của proto không có trong bảng kiểm", v)
		}
	}
	for k, w := range want {
		if got := kindFromWire(k); got != w {
			t.Errorf("kindFromWire(%v) = %q, muốn %q", k, got, w)
		}
	}
}

// The bell-only disbursement mention (ADR 0081 #5) passes the whole RPC, validation included — a comms
// that maps it but whose domain refuses it would fail every finance mention call.
func TestDeliverAcceptsDisbursementMention(t *testing.T) {
	r := goodRequest()
	r.Notifications[0].Kind = commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DISBURSEMENT_MENTION
	f := &fakeDeliverer{}
	if _, err := newTestServer(f).DeliverStaffNotifications(ctxWithPeer(), r); err != nil {
		t.Fatal(err)
	}
	if f.lastIn[0].Kind != domain.StaffNotificationDisbursementMention {
		t.Errorf("loại = %q", f.lastIn[0].Kind)
	}
}

// A per-domain kind passes the whole RPC (validation included) and reaches the use case unchanged.
func TestDeliverAcceptsPerDomainKinds(t *testing.T) {
	r := goodRequest()
	r.Notifications[0].Kind = commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_PETITION_OVERDUE
	r.Notifications[1].Kind = commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DOCUMENT_UNASSIGNED
	f := &fakeDeliverer{}
	if _, err := newTestServer(f).DeliverStaffNotifications(ctxWithPeer(), r); err != nil {
		t.Fatal(err)
	}
	if f.lastIn[0].Kind != domain.ZaloKindPetitionOverdue || f.lastIn[1].Kind != domain.ZaloKindDocumentUnassigned {
		t.Errorf("loại = %q, %q", f.lastIn[0].Kind, f.lastIn[1].Kind)
	}
}

// Every act notice 18–25 passes the whole RPC, validation included, and reaches the use case as its
// stored value — a comms that maps them but whose domain refused one would fail every relay drain.
func TestDeliverAcceptsActNoticeKinds(t *testing.T) {
	for wire, stored := range map[commsv1.StaffNotificationKind]string{
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_ASSIGNED:            domain.ZaloKindTaskAssigned,
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_EXTENSION_REQUESTED: domain.ZaloKindTaskExtensionRequested,
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_APPROVAL_REQUESTED:  domain.ZaloKindTaskApprovalRequested,
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_MENTION:             domain.ZaloKindTaskMention,
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DOCUMENT_ASSIGNED:        domain.ZaloKindDocumentAssigned,
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_PETITION_ASSIGNED:        domain.ZaloKindPetitionAssigned,
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_PETITION_REOPENED:        domain.ZaloKindPetitionReopened,
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_REPORT_READY:             domain.ZaloKindReportReady,
	} {
		r := goodRequest()
		r.Notifications[0].Kind = wire
		f := &fakeDeliverer{}
		if _, err := newTestServer(f).DeliverStaffNotifications(ctxWithPeer(), r); err != nil {
			t.Fatalf("%v: %v", wire, err)
		}
		if f.lastIn[0].Kind != stored {
			t.Errorf("%v → %q, want %q", wire, f.lastIn[0].Kind, stored)
		}
	}
}

// An unknown wire value next to a valid act notice still refuses the WHOLE batch: the use case writes
// nothing, and the relay keeps both rows undelivered until comms catches up (comms.proto ROLLOUT).
func TestDeliverUnknownWireValueRefusesWholeBatch(t *testing.T) {
	r := goodRequest()
	r.Notifications[0].Kind = commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_ASSIGNED
	r.Notifications[1].Kind = commsv1.StaffNotificationKind(26)
	f := &fakeDeliverer{}
	res, err := newTestServer(f).DeliverStaffNotifications(ctxWithPeer(), r)
	if status.Code(err) != codes.InvalidArgument || res != nil {
		t.Fatalf("code = %v, res = %v; want INVALID_ARGUMENT and nothing", status.Code(err), res)
	}
}

func TestDeliverRetryReportsAlreadyDelivered(t *testing.T) {
	res, err := newTestServer(&fakeDeliverer{already: true}).DeliverStaffNotifications(ctxWithPeer(), goodRequest())
	if err != nil {
		t.Fatal(err)
	}
	if it := res.GetItems()[0]; it.GetCreated() != 0 || it.GetAlreadyDelivered() != 2 {
		t.Fatalf("= %+v", it)
	}
}

func TestDeliverWithoutCommuneIsInternalAndCallsNothing(t *testing.T) {
	f := &fakeDeliverer{}
	_, err := newTestServer(f).DeliverStaffNotifications(context.Background(), goodRequest())
	if status.Code(err) != codes.Internal || f.calls != 0 {
		t.Fatalf("mã = %v, gọi = %d", status.Code(err), f.calls)
	}
}

func TestNewServerRefusesMissingDeps(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("dựng được máy chủ thiếu phụ thuộc")
		}
	}()
	NewServer(Deps{})
}
