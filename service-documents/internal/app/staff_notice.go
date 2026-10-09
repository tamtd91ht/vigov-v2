package app

// writeStaffNotice — the ONE way an act of this service records a staff notice (ADR 0086 A1): an outbox
// row in the act's own transaction, drained later by StaffNoticeRelay. Same design as
// service-petitions/internal/app/staff_notice.go: drop the actor, map the kind onto the wire, store the
// protojson of comms.v1.StaffNotification.
//
// WHAT IT DOES NOT DO: call comms. Calling inside the transaction would make routing fail whenever comms
// does and hold the register row lock over a network call; calling after commit loses the notice if the
// process dies in between (ADR 0086, stop condition #1). The relay is the only caller of comms for these.

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	commsv1 "github.com/vihat/vigov/core/gen/vigov/comms/v1"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

// StaffNoticeOutbox is the write half of the outbox as an act needs it. *docstore.StaffNoticeOutboxStore.
// The commune is the transaction's (tx.TenantID()), never an argument.
type StaffNoticeOutbox interface {
	Record(ctx context.Context, tx *store.ScopedTx, r docstore.StaffNoticeRow) error
}

// actNoticeWireKind maps the stored kind onto comms' enum. An unknown kind is refused, never defaulted:
// a notice under the wrong kind lands in the wrong Zalo box of the commune.
func actNoticeWireKind(k domain.ActNoticeKind) (commsv1.StaffNotificationKind, bool) {
	if k == domain.ActNoticeDocumentAssigned {
		return commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DOCUMENT_ASSIGNED, true
	}
	return commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_UNSPECIFIED, false
}

// staffNoticeMarshal is the payload encoding — proto field names, so the stored JSON reads like the
// contract. The relay decodes with protojson, which accepts either spelling.
var staffNoticeMarshal = protojson.MarshalOptions{UseProtoNames: true}

// writeStaffNotice records notice `n` inside `tx`. actorCode is the acting STAFF code (audit.Actor.ID —
// Principal.Ma); it is removed from the recipients (A3 #1). Nobody left → NO ROW, and no error: an
// officer routing a document to themselves owes nobody a notice (transaction boundary compensation (c)).
//
// A FAILURE HERE FAILS THE ACT. That is the outbox's point: the act and its notice commit together or
// not at all.
func writeStaffNotice(ctx context.Context, tx *store.ScopedTx, out StaffNoticeOutbox, newID func() (string, error),
	n domain.ActNotice, actorCode string, at time.Time) error {

	to := domain.DropActor(n.Recipients, actorCode)
	if len(to) == 0 {
		return nil
	}
	if out == nil {
		// FAIL CLOSED: an act that owes a notice with no outbox wired must not commit without it. Every
		// constructor call in cmd/server wires one; reaching here is a wiring fault.
		return fmt.Errorf("thông báo cán bộ: chưa nối outbox")
	}
	kind, ok := actNoticeWireKind(n.Kind)
	if !ok || n.Event == "" {
		return fmt.Errorf("thông báo cán bộ: loại %q không biết", n.Kind)
	}
	if n.ActRowID == "" {
		// THE KEY IS THE ACT (A3 #2). With no act row there is no key a retry could repeat.
		return fmt.Errorf("thông báo cán bộ: thiếu mã dòng hành vi")
	}
	key := n.Key()
	if !domain.ValidNoticeKey(key) || len(to) > maxRecipientsPerNotice {
		return fmt.Errorf("thông báo cán bộ: khoá hoặc danh sách người nhận sai hợp đồng")
	}
	payload, err := staffNoticeMarshal.Marshal(&commsv1.StaffNotification{
		IdempotencyKey: key, Kind: kind, RecipientMa: to, Title: n.Title, Body: n.Body, Link: n.Link,
	})
	if err != nil {
		return fmt.Errorf("thông báo cán bộ: mã hoá: %w", err)
	}
	id, err := newID()
	if err != nil {
		return fmt.Errorf("thông báo cán bộ: sinh mã nội bộ: %w", err)
	}
	return out.Record(ctx, tx, docstore.StaffNoticeRow{ID: id, Name: n.Event, IdempotencyKey: key,
		Payload: payload, OccurredAt: at})
}
