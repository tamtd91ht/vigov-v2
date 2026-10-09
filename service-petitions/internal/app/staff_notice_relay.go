package app

// StaffNoticeRelay drains `staff_notice_outbox` into comms' DeliverStaffNotifications (ADR 0086 A1;
// transaction boundary `thong_bao_can_bo_khi_giao_viec`). It runs in this process, beside the
// automation runner, for the reason ADR 0058 §1 gives: no broker exists, and the day one does only this
// file is replaced — the acts and their outbox rows stay as they are.
//
// ONE TICK:
//
//  1. an advisory lock in this service's own database — ONE replica drains (ADR 0058 §1). Losing it
//     costs a skipped tick; the idempotency keys still deliver each notice once if two ever raced;
//  2. the communes owing a notice (a `// @cross-tenant:` read of identifiers, store/crosstenant);
//  3. per commune, IN THAT COMMUNE'S CONTEXT — so the gRPC client puts its id in "x-tenant-id" (rule 1,
//     invariant 8; rule 2, invariant 8) — batches of at most 100 rows, oldest act first.
//
// WHAT EACH ANSWER OF COMMS DOES TO A ROW (the transaction boundary's compensation):
//
//	OK                  delivered_at set. Comms' answer per key (created / already delivered) is not
//	                    needed: both mean the person has the notice.
//	UNAVAILABLE etc.    the rows stay owed, attempts+1, and the relay BACKS OFF — every commune shares
//	                    one comms, so the tick stops rather than hammering it commune after commune.
//	INVALID_ARGUMENT    comms refuses a WHOLE call; the batch is retried one row at a time so the one
//	                    bad row is found and set aside (failed_class = invalid_argument) and the others
//	                    leave. A set-aside row never blocks the rows behind it, and is re-queued only by
//	                    a deliberate act (clearing failed_class) once comms knows the kind.
//	anything else       treated as an outage: rows stay owed, attempts+1, back off.
//
// A row whose payload cannot be read is set aside as `invalid_payload` — it can never be delivered as
// it is, and retrying it would block nothing but fill the log.
//
// NEVER LOGGED: a payload, a title, a recipient. Counts, the commune id and status codes only (rule 3).

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/vihat/vigov/core/commsclient"
	commsv1 "github.com/vihat/vigov/core/gen/vigov/comms/v1"
	"github.com/vihat/vigov/core/tenant"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// StaffNoticeRelayInterval is the relay's beat when comms is healthy — the boundary's "≤ 1 minute"
// target with room for one failed beat.
const StaffNoticeRelayInterval = 20 * time.Second

const (
	// staffNoticeRelayLock names the relay's advisory lock (crosstenant.LockKey derives the key).
	staffNoticeRelayLock = "staff_notice_relay"
	// staffNoticeBatch is comms' bound on one call (DeliverStaffNotificationsRequest: 1 to 100).
	staffNoticeBatch = commsclient.MaxNoticesPerCall
	// staffNoticeBatchesPerTick bounds one commune's share of a tick, so a commune with a backlog cannot
	// starve the others; what is left goes at the next beat.
	staffNoticeBatchesPerTick = 10
	// staffNoticeMaxBackoff caps the wait after repeated outages.
	staffNoticeMaxBackoff = 5 * time.Minute
)

// FailedClassInvalidPayload sets aside a row whose stored payload is not a readable StaffNotification.
const FailedClassInvalidPayload = "invalid_payload"

// PendingNoticeCommunes lists the communes owing a notice. *crosstenant.Automation.
type PendingNoticeCommunes interface {
	CommunesWithPendingStaffNotices(ctx context.Context) ([]tenant.ID, error)
}

// StaffNoticeQueue is the relay's half of the outbox, in the commune of ctx. *petstore.StaffNoticeOutboxStore.
type StaffNoticeQueue interface {
	Undelivered(ctx context.Context, limit int) ([]petstore.StaffNoticeRow, error)
	MarkDelivered(ctx context.Context, ids []string, at time.Time) error
	RecordFailedAttempt(ctx context.Context, ids []string) error
	MarkFailed(ctx context.Context, ids []string, class string) error
}

// StaffNoticeRelayDeps wires the relay. Every field but Log and Now is required.
type StaffNoticeRelayDeps struct {
	Communes PendingNoticeCommunes
	Locks    JobLocker
	Queue    StaffNoticeQueue
	Comms    NoticeDeliverer
	Log      *slog.Logger
	// Now stamps delivered_at. nil = the real clock.
	Now func() time.Time
}

// StaffNoticeRelay delivers the outbox. Build it with NewStaffNoticeRelay; Run blocks until ctx is done.
type StaffNoticeRelay struct {
	d        StaffNoticeRelayDeps
	interval time.Duration
}

// NewStaffNoticeRelay builds the relay.
func NewStaffNoticeRelay(d StaffNoticeRelayDeps) (*StaffNoticeRelay, error) {
	if d.Communes == nil || d.Locks == nil || d.Queue == nil || d.Comms == nil {
		return nil, errors.New("bộ chuyển thông báo cán bộ: thiếu phụ thuộc")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	if d.Now == nil {
		d.Now = func() time.Time { return time.Now().UTC() }
	}
	return &StaffNoticeRelay{d: d, interval: StaffNoticeRelayInterval}, nil
}

// Run ticks until ctx is cancelled, backing off after a tick that met an outage.
func (r *StaffNoticeRelay) Run(ctx context.Context) {
	failures := 0
	for {
		if r.Tick(ctx) {
			failures = 0
		} else {
			failures++
		}
		t := time.NewTimer(relayWait(r.interval, failures))
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
	}
}

// relayWait doubles the beat per consecutive failed tick, capped.
func relayWait(base time.Duration, failures int) time.Duration {
	wait := base
	for i := 0; i < failures && wait < staffNoticeMaxBackoff; i++ {
		wait *= 2
	}
	return min(wait, staffNoticeMaxBackoff)
}

// errRelayOutage stops a tick: comms (or this service's own database) did not answer.
var errRelayOutage = errors.New("bộ chuyển thông báo cán bộ: phụ thuộc không trả lời")

// Tick does one pass and reports whether it met no outage (false → back off).
func (r *StaffNoticeRelay) Tick(ctx context.Context) bool {
	if ctx.Err() != nil {
		return true
	}
	held, release, err := r.d.Locks.TryLockJobs(ctx, []string{staffNoticeRelayLock})
	if err != nil {
		r.d.Log.WarnContext(ctx, "CẢNH BÁO: không thử được khoá bộ chuyển thông báo cán bộ, bỏ nhịp này",
			"service", "petitions", "err", err)
		return false
	}
	defer release()
	if !held[staffNoticeRelayLock] {
		return true // another replica drains this beat
	}
	communes, err := r.d.Communes.CommunesWithPendingStaffNotices(ctx)
	if err != nil {
		r.d.Log.WarnContext(ctx, "CẢNH BÁO: không liệt kê được xã còn thông báo chờ gửi, bỏ nhịp này",
			"service", "petitions", "err", err)
		return false
	}
	for _, id := range communes {
		if ctx.Err() != nil {
			return true
		}
		if err := r.drainCommune(tenant.Into(ctx, id)); err != nil {
			r.d.Log.WarnContext(ctx, "CẢNH BÁO: dừng chuyển thông báo cán bộ ở nhịp này", "service", "petitions",
				"xa", string(id), "err", err)
			return false
		}
	}
	return true
}

// drainCommune sends the commune's owed rows, batch by batch. An outage stops it with an error.
func (r *StaffNoticeRelay) drainCommune(ctx context.Context) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("%w: panic: %v", errRelayOutage, p)
		}
	}()
	for i := 0; i < staffNoticeBatchesPerTick; i++ {
		rows, err := r.d.Queue.Undelivered(ctx, staffNoticeBatch)
		if err != nil {
			return fmt.Errorf("%w: %w", errRelayOutage, err)
		}
		if len(rows) == 0 {
			return nil
		}
		if err := r.deliverBatch(ctx, rows); err != nil {
			return err
		}
		if len(rows) < staffNoticeBatch {
			return nil
		}
	}
	return nil
}

// relayItem is one readable row ready for the wire.
type relayItem struct {
	id     string
	notice commsclient.Notice
}

// deliverBatch sends ≤ 100 rows in calls of ≤ 1 000 recipients.
func (r *StaffNoticeRelay) deliverBatch(ctx context.Context, rows []petstore.StaffNoticeRow) error {
	var items []relayItem
	var unreadable []string
	for _, row := range rows {
		n, ok := noticeFromPayload(row)
		if !ok {
			unreadable = append(unreadable, row.ID)
			continue
		}
		items = append(items, relayItem{id: row.ID, notice: n})
	}
	if len(unreadable) > 0 {
		if err := r.d.Queue.MarkFailed(ctx, unreadable, FailedClassInvalidPayload); err != nil {
			return fmt.Errorf("%w: %w", errRelayOutage, err)
		}
		r.d.Log.WarnContext(ctx, "CẢNH BÁO: dòng thông báo cán bộ không đọc được, đã gác lại", "service", "petitions",
			"xa", string(tenant.MustFrom(ctx)), "so_dong", len(unreadable))
	}
	for _, page := range relayPages(items) {
		if err := r.send(ctx, page, true); err != nil {
			return err
		}
	}
	return nil
}

// send delivers one call and applies comms' answer to its rows. isolate=true lets an INVALID_ARGUMENT
// on several rows be retried row by row.
func (r *StaffNoticeRelay) send(ctx context.Context, page []relayItem, isolate bool) error {
	ids := make([]string, len(page))
	notices := make([]commsclient.Notice, len(page))
	for i, it := range page {
		ids[i], notices[i] = it.id, it.notice
	}
	_, err := r.d.Comms.DeliverStaffNotifications(ctx, notices)
	switch {
	case err == nil:
		if err := r.d.Queue.MarkDelivered(ctx, ids, r.d.Now()); err != nil {
			// Comms has them; the rows stay owed and the next beat sends the same keys — delivered once.
			return fmt.Errorf("%w: %w", errRelayOutage, err)
		}
		return nil
	case status.Code(err) == codes.InvalidArgument:
		if isolate && len(page) > 1 {
			for _, it := range page {
				if err := r.send(ctx, []relayItem{it}, false); err != nil {
					return err
				}
			}
			return nil
		}
		if err := r.d.Queue.MarkFailed(ctx, ids, petstore.FailedClassInvalidArgument); err != nil {
			return fmt.Errorf("%w: %w", errRelayOutage, err)
		}
		r.d.Log.WarnContext(ctx, "CẢNH BÁO: comms từ chối thông báo cán bộ, đã gác lại để chạy lại có chủ đích",
			"service", "petitions", "xa", string(tenant.MustFrom(ctx)), "so_dong", len(ids))
		return nil
	default:
		// UNAVAILABLE / DEADLINE_EXCEEDED (commsclient.ErrCommsUnavailable) and every other answer: nothing
		// was written by comms; the rows stay owed and the relay backs off.
		if rerr := r.d.Queue.RecordFailedAttempt(ctx, ids); rerr != nil {
			return fmt.Errorf("%w: %w", errRelayOutage, rerr)
		}
		return fmt.Errorf("%w: %w", errRelayOutage, err)
	}
}

// relayPages splits items into calls of ≤ 100 notices and ≤ 1 000 recipients, in the queue's order.
// Keys are unique per commune by the table's constraint, so no call repeats one.
func relayPages(items []relayItem) [][]relayItem {
	var pages [][]relayItem
	var cur []relayItem
	recipients := 0
	for _, it := range items {
		if len(cur) == commsclient.MaxNoticesPerCall ||
			(len(cur) > 0 && recipients+len(it.notice.RecipientMa) > commsclient.MaxRecipientsPerCall) {
			pages = append(pages, cur)
			cur, recipients = nil, 0
		}
		cur = append(cur, it)
		recipients += len(it.notice.RecipientMa)
	}
	if len(cur) > 0 {
		pages = append(pages, cur)
	}
	return pages
}

// noticeFromPayload reads a stored row back into the client's shape. The key on the wire is the ROW's
// key column — the one the UNIQUE constraint guards — and a payload naming another key is unreadable.
func noticeFromPayload(row petstore.StaffNoticeRow) (commsclient.Notice, bool) {
	var m commsv1.StaffNotification
	if err := protojson.Unmarshal(row.Payload, &m); err != nil {
		return commsclient.Notice{}, false
	}
	if m.GetIdempotencyKey() != row.IdempotencyKey || m.GetKind() == commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_UNSPECIFIED ||
		len(m.GetRecipientMa()) == 0 {
		return commsclient.Notice{}, false
	}
	return commsclient.Notice{
		IdempotencyKey: row.IdempotencyKey, Kind: m.GetKind(), RecipientMa: m.GetRecipientMa(),
		Title: m.GetTitle(), Body: m.GetBody(), Link: m.GetLink(),
	}, true
}
