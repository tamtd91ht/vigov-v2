package app

// StaffNoticeRelay drains `staff_notice_outbox` into comms (ADR 0086 A1; transaction boundary
// `thong_bao_can_bo_khi_giao_viec`). It runs in THIS process because this service owns the outbox —
// there is no broker, and the day there is one only this file changes.
//
// PER TICK:
//
//  1. one PostgreSQL advisory lock (`staff_notice_relay`, crosstenant.LockKey), so ONE replica drains —
//     the shape of the automation runner's per-job locks (ADR 0058 §1);
//  2. the communes owing a notice — a `// @cross-tenant:` read of identifiers only (store/crosstenant);
//  3. per commune, IN THAT COMMUNE'S CONTEXT (so "x-tenant-id" on the comms call is that commune —
//     rule 1, invariant 8; core/grpcx sets it from the context), pages of at most 100 rows, oldest
//     first, each sent as one DeliverStaffNotifications call.
//
// THE STATUS CODES DECIDE WHAT HAPPENS TO A ROW (comms.proto, "THE STATUS CODES ARE PART OF THE
// CONTRACT"; transaction boundary compensation):
//
//	OK                 rows stamped delivered. A retry of an already-accepted key answers OK with
//	                   already_delivered — the key is the act, so a resend notifies nobody twice.
//	UNAVAILABLE        nothing was written: rows stay owed, the attempt is counted, and the relay BACKS
//	(ErrCommsUnavailable) OFF — the next ticks are skipped for a doubling delay up to maxRelayBackoff,
//	                   because comms down for one commune is comms down for all of them.
//	INVALID_ARGUMENT   comms refuses the WHOLE batch for one bad notice. The page is retried row by row
//	                   so only the refused row is set aside (failed_class `invalid_argument`) and the rows
//	                   behind it still leave. A set-aside row is never retried until somebody clears it.
//	anything else      a contract or local fault: rows stay owed, the attempt is counted, this commune
//	                   stops for this tick, the others continue.
//
// A ROW WHOSE PAYLOAD DOES NOT DECODE is set aside as `payload_invalid` — sending a guess of it would
// put words nobody wrote into a bell.
//
// NOTHING IS LOGGED BUT COUNTS, CODES AND THE COMMUNE: a payload carries a title (rule 3 — not personal
// data by contract, but this file does not get to decide that for every future producer).

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
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

// Relay pacing. The transaction boundary promises "one relay beat, aim <= 1 minute" while comms is up.
const (
	StaffNoticeRelayInterval = 15 * time.Second
	// maxRelayBatch is comms' bound on one call (MaxNoticesPerCall). Each row holds one notice of at
	// most 200 recipients, so 100 rows can exceed the 1 000-recipient bound only in theory — every row
	// this service writes has ONE recipient; relayPages still splits on it.
	maxRelayBatch = commsclient.MaxNoticesPerCall
	// maxRelayPagesPerCommune bounds one commune's share of a tick, so a commune with a large backlog
	// cannot starve the others; what is left leaves on the next tick.
	maxRelayPagesPerCommune = 10
	maxRelayBackoff         = 5 * time.Minute
)

// staffNoticeRelayLock is the advisory-lock name. Not a job of identity's — a lock name only.
const staffNoticeRelayLock = "staff_notice_relay"

// PendingNoticeCommunes lists the communes owing a notice. *crosstenant.Automation.
type PendingNoticeCommunes interface {
	CommunesWithPendingStaffNotices(ctx context.Context) ([]tenant.ID, error)
}

// OutboxDrain is the relay half of the outbox, in the commune of ctx. *docstore.StaffNoticeOutboxStore.
type OutboxDrain interface {
	Undelivered(ctx context.Context, limit int) ([]docstore.StaffNoticeRow, error)
	MarkDelivered(ctx context.Context, ids []string, at time.Time) error
	RecordFailedAttempt(ctx context.Context, ids []string) error
	MarkFailed(ctx context.Context, ids []string, class string) error
}

// StaffNoticeRelayDeps wires the relay. Every field but Log is required.
type StaffNoticeRelayDeps struct {
	Communes PendingNoticeCommunes
	Locks    JobLocker
	Outbox   OutboxDrain
	Comms    NoticeDeliverer
	Log      *slog.Logger
}

// StaffNoticeRelay delivers the outbox. Run blocks until ctx is done. One goroutine: the backoff state
// below is not shared.
type StaffNoticeRelay struct {
	d        StaffNoticeRelayDeps
	interval time.Duration
	now      func() time.Time

	backoff   time.Duration
	nextTryAt time.Time
}

// NewStaffNoticeRelay builds the relay; a missing dependency is refused at construction.
func NewStaffNoticeRelay(d StaffNoticeRelayDeps) (*StaffNoticeRelay, error) {
	if d.Communes == nil || d.Locks == nil || d.Outbox == nil || d.Comms == nil {
		return nil, errors.New("bộ chuyển thông báo cán bộ: thiếu phụ thuộc")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &StaffNoticeRelay{d: d, interval: StaffNoticeRelayInterval, now: func() time.Time { return time.Now().UTC() }}, nil
}

// Run ticks until ctx is cancelled — once at start, then every interval.
func (r *StaffNoticeRelay) Run(ctx context.Context) {
	r.Tick(ctx)
	t := time.NewTicker(r.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.Tick(ctx)
		}
	}
}

// errRelayBackOff stops a tick after comms answered UNAVAILABLE.
var errRelayBackOff = errors.New("bộ chuyển: comms tạm không trả lời")

// Tick does one pass. Exported for tests.
func (r *StaffNoticeRelay) Tick(ctx context.Context) {
	if ctx.Err() != nil || r.now().Before(r.nextTryAt) {
		return
	}
	held, release, err := r.d.Locks.TryLockJobs(ctx, []string{staffNoticeRelayLock})
	if err != nil {
		r.d.Log.WarnContext(ctx, "CẢNH BÁO: không thử được khoá bộ chuyển thông báo, bỏ nhịp này", "service", "documents", "err", err)
		return
	}
	defer release()
	if !held[staffNoticeRelayLock] {
		return // another replica drains this tick
	}
	communes, err := r.d.Communes.CommunesWithPendingStaffNotices(ctx)
	if err != nil {
		r.d.Log.WarnContext(ctx, "CẢNH BÁO: không liệt kê được xã còn thông báo chờ gửi", "service", "documents", "err", err)
		return
	}
	for _, id := range communes {
		if ctx.Err() != nil {
			return
		}
		if err := r.drainCommune(ctx, id); errors.Is(err, errRelayBackOff) {
			r.backOff()
			return
		}
	}
	r.backoff, r.nextTryAt = 0, time.Time{}
}

// backOff doubles the pause after an outage, from one interval up to maxRelayBackoff.
func (r *StaffNoticeRelay) backOff() {
	if r.backoff == 0 {
		r.backoff = r.interval
	} else {
		r.backoff = min(2*r.backoff, maxRelayBackoff)
	}
	r.nextTryAt = r.now().Add(r.backoff)
}

// drainCommune sends what one commune owes. Nothing it does stops the next commune except a comms outage.
func (r *StaffNoticeRelay) drainCommune(ctx context.Context, id tenant.ID) (err error) {
	defer func() {
		if p := recover(); p != nil {
			r.d.Log.ErrorContext(ctx, "LỖI: bộ chuyển thông báo panic ở một xã, chuyển sang xã kế", "service", "documents",
				"xa", string(id), "panic", fmt.Sprint(p))
			err = nil
		}
	}()
	cctx := tenant.Into(ctx, id)
	for page := 0; page < maxRelayPagesPerCommune; page++ {
		rows, err := r.d.Outbox.Undelivered(cctx, maxRelayBatch)
		if err != nil {
			r.d.Log.WarnContext(cctx, "CẢNH BÁO: không đọc được thông báo chờ gửi", "service", "documents", "xa", string(id), "err", err)
			return nil
		}
		if len(rows) == 0 {
			return nil
		}
		if err := r.sendRows(cctx, id, rows); err != nil {
			return err
		}
		if len(rows) < maxRelayBatch {
			return nil
		}
	}
	return nil
}

// relayItem is one decodable row.
type relayItem struct {
	id     string
	notice commsclient.Notice
}

// sendRows decodes one page, sets undecodable rows aside, and delivers the rest.
func (r *StaffNoticeRelay) sendRows(ctx context.Context, id tenant.ID, rows []docstore.StaffNoticeRow) error {
	var (
		items []relayItem
		bad   []string
	)
	for _, row := range rows {
		n, ok := decodeStaffNotice(row.Payload)
		if !ok {
			bad = append(bad, row.ID)
			continue
		}
		items = append(items, relayItem{id: row.ID, notice: n})
	}
	if len(bad) > 0 {
		r.d.Log.WarnContext(ctx, "CẢNH BÁO: dòng thông báo chờ gửi không giải mã được, đặt sang bên", "service", "documents",
			"xa", string(id), "so_dong", len(bad))
		if err := r.d.Outbox.MarkFailed(ctx, bad, docstore.FailedClassPayloadInvalid); err != nil {
			r.d.Log.WarnContext(ctx, "CẢNH BÁO: không đánh dấu được dòng hỏng", "service", "documents", "xa", string(id), "err", err)
		}
	}
	for _, page := range relayPages(items) {
		if err := r.deliver(ctx, id, page, true); err != nil {
			return err
		}
	}
	return nil
}

// deliver sends one page and settles its rows. splitOnRefusal is false for a single-row retry.
func (r *StaffNoticeRelay) deliver(ctx context.Context, id tenant.ID, page []relayItem, splitOnRefusal bool) error {
	notices := make([]commsclient.Notice, len(page))
	ids := make([]string, len(page))
	for i, it := range page {
		notices[i], ids[i] = it.notice, it.id
	}
	_, err := r.d.Comms.DeliverStaffNotifications(ctx, notices)
	switch {
	case err == nil:
		if err := r.d.Outbox.MarkDelivered(ctx, ids, r.now()); err != nil {
			// The rows stay owed and are sent again next tick under the SAME keys: comms answers
			// already_delivered and nobody is told twice.
			r.d.Log.WarnContext(ctx, "CẢNH BÁO: comms đã nhận nhưng không ghi được trạng thái đã gửi", "service", "documents",
				"xa", string(id), "so_dong", len(ids), "err", err)
			// Stop this commune: reading the page again now would only resend the same keys.
			return errRelayCommuneStop
		}
		return nil
	case errors.Is(err, commsclient.ErrCommsUnavailable):
		r.countAttempt(ctx, id, ids)
		return errRelayBackOff
	case status.Code(err) == codes.InvalidArgument:
		if splitOnRefusal && len(page) > 1 {
			for _, it := range page {
				if err := r.deliver(ctx, id, []relayItem{it}, false); err != nil {
					return err
				}
			}
			return nil
		}
		r.d.Log.WarnContext(ctx, "CẢNH BÁO: comms từ chối thông báo, đặt sang bên", "service", "documents",
			"xa", string(id), "so_dong", len(ids))
		if err := r.d.Outbox.MarkFailed(ctx, ids, docstore.FailedClassInvalidArgument); err != nil {
			r.d.Log.WarnContext(ctx, "CẢNH BÁO: không đánh dấu được dòng bị từ chối", "service", "documents", "xa", string(id), "err", err)
		}
		return nil
	default:
		r.countAttempt(ctx, id, ids)
		r.d.Log.WarnContext(ctx, "CẢNH BÁO: không giao được thông báo cán bộ, thử lại nhịp sau", "service", "documents",
			"xa", string(id), "ma_loi", status.Code(err).String(), "so_dong", len(ids))
		return errRelayCommuneStop
	}
}

// errRelayCommuneStop stops one commune for this tick; Tick moves on to the next.
var errRelayCommuneStop = errors.New("bộ chuyển: dừng xã này ở nhịp này")

func (r *StaffNoticeRelay) countAttempt(ctx context.Context, id tenant.ID, ids []string) {
	if err := r.d.Outbox.RecordFailedAttempt(ctx, ids); err != nil {
		r.d.Log.WarnContext(ctx, "CẢNH BÁO: không đếm được lần gửi hỏng", "service", "documents", "xa", string(id), "err", err)
	}
}

// decodeStaffNotice reads a stored payload. False when it is not a StaffNotification with a key.
func decodeStaffNotice(payload []byte) (commsclient.Notice, bool) {
	var m commsv1.StaffNotification
	if err := protojson.Unmarshal(payload, &m); err != nil || m.GetIdempotencyKey() == "" {
		return commsclient.Notice{}, false
	}
	return commsclient.Notice{IdempotencyKey: m.GetIdempotencyKey(), Kind: m.GetKind(), RecipientMa: m.GetRecipientMa(),
		Title: m.GetTitle(), Body: m.GetBody(), Link: m.GetLink()}, true
}

// relayPages cuts a page into calls comms accepts: <= 100 notices, <= 1 000 recipients, a key once.
func relayPages(items []relayItem) [][]relayItem {
	var (
		pages      [][]relayItem
		cur        []relayItem
		keys       = map[string]bool{}
		recipients int
	)
	for _, it := range items {
		if len(cur) > 0 && (len(cur) == commsclient.MaxNoticesPerCall || keys[it.notice.IdempotencyKey] ||
			recipients+len(it.notice.RecipientMa) > commsclient.MaxRecipientsPerCall) {
			pages = append(pages, cur)
			cur, keys, recipients = nil, map[string]bool{}, 0
		}
		cur = append(cur, it)
		keys[it.notice.IdempotencyKey] = true
		recipients += len(it.notice.RecipientMa)
	}
	if len(cur) > 0 {
		pages = append(pages, cur)
	}
	return pages
}
