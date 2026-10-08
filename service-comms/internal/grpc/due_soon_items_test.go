package grpc

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	commsv1 "github.com/vihat/vigov/core/gen/vigov/comms/v1"
)

// StaffNotification.due_soon_items (comms.proto, ADR 0079 lô 5 Q13) through the whole RPC: the items reach
// the use case as sent, and every refusal of the contract's VALIDATION is INVALID_ARGUMENT for the WHOLE
// batch, with nothing delivered and no code echoed.

var itemDeadline = time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)

func withItems(items ...*commsv1.DueSoonItem) *commsv1.DeliverStaffNotificationsRequest {
	r := goodRequest()
	r.Notifications[0].DueSoonItems = items
	return r
}

func item(code string) *commsv1.DueSoonItem {
	return &commsv1.DueSoonItem{Code: code, Deadline: timestamppb.New(itemDeadline)}
}

func TestDeliverCarriesDueSoonItemsToTheUseCase(t *testing.T) {
	f := &fakeDeliverer{}
	if _, err := newTestServer(f).DeliverStaffNotifications(ctxWithPeer(), withItems(item("NV-0001"), item("NV-0002"))); err != nil {
		t.Fatal(err)
	}
	got := f.lastIn[0].DueSoonItems
	if len(got) != 2 || got[0].Code != "NV-0001" || !got[1].Deadline.Equal(itemDeadline) {
		t.Fatalf("items = %+v", got)
	}
	if f.lastIn[1].DueSoonItems != nil {
		t.Errorf("a notice without items received %+v", f.lastIn[1].DueSoonItems)
	}
}

func TestDeliverRefusesBadDueSoonItemsWholeBatch(t *testing.T) {
	tooMany := make([]*commsv1.DueSoonItem, 501)
	for i := range tooMany {
		tooMany[i] = item(fmt.Sprintf("NV-%04d", i))
	}
	cases := []struct {
		name string
		req  *commsv1.DeliverStaffNotificationsRequest
	}{
		{"on an overdue kind", func() *commsv1.DeliverStaffNotificationsRequest {
			r := withItems(item("NV-0001"))
			r.Notifications[0].Kind = commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_OVERDUE
			return r
		}()},
		{"on the weekly digest", func() *commsv1.DeliverStaffNotificationsRequest {
			r := goodRequest()
			r.Notifications[1].DueSoonItems = []*commsv1.DueSoonItem{item("NV-0001")}
			return r
		}()},
		{"501 items", withItems(tooMany...)},
		{"empty code", withItems(item(""))},
		{"101-character code", withItems(item(strings.Repeat("A", 101)))},
		{"control character in code", withItems(item("NV-\n0001"))},
		{"repeated code", withItems(item("NV-0001"), item("NV-0001"))},
		{"deadline unset", withItems(&commsv1.DueSoonItem{Code: "NV-0001"})},
		{"deadline out of range", withItems(&commsv1.DueSoonItem{Code: "NV-0001",
			Deadline: &timestamppb.Timestamp{Seconds: 1, Nanos: -1}})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newTestServer(&fakeDeliverer{}).DeliverStaffNotifications(ctxWithPeer(), tc.req)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("code = %v (%v)", status.Code(err), err)
			}
			if strings.Contains(err.Error(), "NV-") {
				t.Errorf("the refusal echoes a code: %v", err)
			}
		})
	}
}

// Every due-soon wire kind admits items: DUE_SOON 1, TASK 5, DOCUMENT 9, PETITION 13 (comms.proto).
func TestDeliverAcceptsDueSoonItemsOnEveryDueSoonKind(t *testing.T) {
	for _, k := range []commsv1.StaffNotificationKind{
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DUE_SOON,
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_DUE_SOON,
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DOCUMENT_DUE_SOON,
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_PETITION_DUE_SOON,
	} {
		r := withItems(item("NV-0001"))
		r.Notifications[0].Kind = k
		if _, err := newTestServer(&fakeDeliverer{}).DeliverStaffNotifications(ctxWithPeer(), r); err != nil {
			t.Errorf("%v: %v", k, err)
		}
	}
}
