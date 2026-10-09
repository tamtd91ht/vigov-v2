package app

// writeStaffNotice — the ONE way an act of this service records a staff notice (ADR 0086 A1): an outbox
// row in the act's own transaction, drained later by StaffNoticeRelay. Every act calls it with the
// notice the domain built; this file drops the actor, splits an over-long recipient list, maps the kind
// onto the wire and stores the protojson of comms.v1.StaffNotification.
//
// WHAT IT DOES NOT DO: call comms. Calling inside the transaction would make the act fail whenever comms
// does and hold a register row lock over a network call; calling after commit loses the notice if the
// process dies in between (ADR 0086, stop condition #1). The relay is the only caller of comms here.

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	commsv1 "github.com/vihat/vigov/core/gen/vigov/comms/v1"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// StaffNoticeOutbox is the write half of the outbox as an act needs it. *petstore.StaffNoticeOutboxStore.
// The commune is the transaction's (tx.TenantID()), never an argument.
type StaffNoticeOutbox interface {
	Record(ctx context.Context, tx *store.ScopedTx, r petstore.StaffNoticeRow) error
}

// OrgUnitHolders answers which staff in each unit hold a permission there — identity's
// ResolveOrgUnitPermissionHolders. *identityclient.Client satisfies it. Grants nothing: kind 18's
// recipient list for a task handed to a unit with nobody named, never a guard.
type OrgUnitHolders interface {
	OrgUnitPermissionHolders(ctx context.Context, orgUnitIDs []string, permissionKey string) (map[string][]string, error)
}

// actNoticeWireKind maps the stored kind onto comms' enum (18–24). An unknown kind is refused, never
// defaulted: a notice under the wrong kind lands in the wrong Zalo box of the commune.
func actNoticeWireKind(k domain.ActNoticeKind) (commsv1.StaffNotificationKind, bool) {
	switch k {
	case domain.ActNoticeTaskAssigned:
		return commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_ASSIGNED, true
	case domain.ActNoticeTaskExtensionRequested:
		return commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_EXTENSION_REQUESTED, true
	case domain.ActNoticeTaskApprovalRequested:
		return commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_APPROVAL_REQUESTED, true
	case domain.ActNoticeTaskMention:
		return commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_MENTION, true
	case domain.ActNoticePetitionAssigned:
		return commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_PETITION_ASSIGNED, true
	case domain.ActNoticePetitionReopened:
		return commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_PETITION_REOPENED, true
	}
	return commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_UNSPECIFIED, false
}

// writeStaffNotice records notice `n` inside `tx`. actorCode is the acting STAFF code (audit.Actor.ID —
// Principal.Ma); it is removed from the recipients (A3 #1). Nobody left → NO ROW, and no error: an
// officer assigning work to themselves owes nobody a notice (transaction boundary compensation (c)).
//
// A FAILURE HERE FAILS THE ACT. That is the outbox's point: the act and its notice commit together or
// not at all. Rows are never partly written — a list over 200 recipients becomes several rows, all in
// this transaction.
func writeStaffNotice(ctx context.Context, tx *store.ScopedTx, out StaffNoticeOutbox, newID func() (string, error),
	n domain.ActNotice, actorCode string, at time.Time) error {

	to := domain.DropActor(n.Recipients, actorCode)
	if len(to) == 0 {
		return nil
	}
	if out == nil {
		// FAIL CLOSED: an act that owes a notice with no outbox wired must not commit without it. Every
		// constructor wires one; reaching here is a wiring fault.
		return fmt.Errorf("thông báo cán bộ: chưa nối outbox")
	}
	kind, ok := actNoticeWireKind(n.Kind)
	name := n.Kind.EventName()
	if !ok || name == "" {
		return fmt.Errorf("thông báo cán bộ: loại %q không biết", n.Kind)
	}
	if n.ActRowID == "" {
		// THE KEY IS THE ACT (A3 #2). With no act row there is no key a retry could repeat.
		return fmt.Errorf("thông báo cán bộ: thiếu mã dòng hành vi")
	}
	keys, parts := domain.SplitRecipients(n.Key(), to)
	for i, key := range keys {
		if !domain.ValidNoticeKey(key) {
			return fmt.Errorf("thông báo cán bộ: khoá chống trùng không hợp lệ")
		}
		payload, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(&commsv1.StaffNotification{
			IdempotencyKey: key, Kind: kind, RecipientMa: parts[i], Title: n.Title, Body: n.Body, Link: n.Link,
		})
		if err != nil {
			return fmt.Errorf("thông báo cán bộ: mã hoá: %w", err)
		}
		id, err := newID()
		if err != nil {
			return fmt.Errorf("thông báo cán bộ: sinh mã nội bộ: %w", err)
		}
		row := petstore.StaffNoticeRow{ID: id, Name: name, IdempotencyKey: key, Payload: payload, OccurredAt: at}
		if err := out.Record(ctx, tx, row); err != nil {
			return err
		}
	}
	return nil
}

// resolveUnitHolders asks identity, BEFORE the act's transaction, who holds `task.assign` in each unit
// that may need kind 18 sent to its assigners. Never inside the transaction (no gRPC call while a
// register row is locked).
//
// A FAILURE DOES NOT STOP THE ACT: handing out work must not fail because the notice's recipient list
// could not be read (ADR 0086 A1 — "một hồ sơ hành chính không được dừng vì kênh nhắn tin"). It returns
// an empty map, the act commits, and the unit's assigners are not told — logged by count, never by
// person. nil holders (not wired) behaves the same.
func resolveUnitHolders(ctx context.Context, holders OrgUnitHolders, units []string) map[string][]string {
	out := map[string][]string{}
	var ask []string
	seen := map[string]bool{}
	for _, u := range units {
		if u != "" && !seen[u] {
			seen[u] = true
			ask = append(ask, u)
		}
	}
	if len(ask) == 0 || holders == nil {
		return out
	}
	for _, page := range pageStrings(ask, maxOrgUnitsPerHoldersCall) {
		got, err := holders.OrgUnitPermissionHolders(ctx, page, permTaskAssign)
		if err != nil {
			slog.Default().WarnContext(ctx, "CẢNH BÁO: không lấy được người giao việc của bộ phận — thông báo giao việc bỏ qua người nhận theo bộ phận",
				"service", "petitions", "so_bo_phan", len(page), "err", err)
			return map[string][]string{}
		}
		for u, codes := range got {
			out[u] = codes
		}
	}
	return out
}

// maxOrgUnitsPerHoldersCall is identity's bound on one ResolveOrgUnitPermissionHolders call
// (identityclient.MaxOrgUnitsPerCall).
const maxOrgUnitsPerHoldersCall = 50
