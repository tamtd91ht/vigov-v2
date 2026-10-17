package app

// EventRelay drains identity's `event_outbox` into Kafka (ADR 0101 #11; the outbox shape of petitions'
// StaffNoticeRelay). Business writes record their facts in the same transaction; this loop publishes them
// AFTER the commit — never inside it (skills/events-and-queues #5).
//
// ONE TICK: the communes owing an event (a `@cross-tenant` read of identifiers, store/crosstenant); then per
// commune, IN THAT COMMUNE'S CONTEXT, up to relayBatch rows oldest first, published one by one through
// events.Publisher (synchronous — the broker acknowledged before the row is stamped).
//
// AT LEAST ONCE, ON PURPOSE: a crash between the acknowledgement and the stamp, or two replicas draining
// the same row, publishes it twice under the SAME envelope id. The consumer is idempotent (it re-reads the
// row), so a duplicate costs one extra document write; an ordering lock would cost more than that.
//
// A FAILURE STOPS THE TICK and the loop backs off: every commune shares one broker, so hammering it commune
// after commune helps nobody. The row stays owed with attempts+1.
//
// NEVER LOGGED: a payload. Ids, names, counts and the commune id only (rule 3) — and the payload holds
// nothing personal anyway (events.proto).

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/vihat/vigov/core/events"
	"github.com/vihat/vigov/core/tenant"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

const (
	// EventRelayInterval is the beat when the broker is healthy: the search index lags a write by about
	// this much.
	EventRelayInterval = 2 * time.Second
	relayBatch         = 100
	relayMaxBackoff    = time.Minute
)

// PendingEventCommunes lists the communes owing an event. *crosstenant.EventOutboxCommunes.
type PendingEventCommunes interface {
	CommunesWithUnpublishedEvents(ctx context.Context) ([]tenant.ID, error)
}

// EventQueue is the relay's half of the outbox, in the commune of ctx. *idstore.EventOutboxStore.
type EventQueue interface {
	Unpublished(ctx context.Context, limit int) ([]idstore.OutboxEvent, error)
	MarkPublished(ctx context.Context, ids []string, at time.Time) error
	RecordFailedAttempt(ctx context.Context, id string) error
}

// EventRelay publishes the outbox. Run blocks until ctx is done.
type EventRelay struct {
	communes  PendingEventCommunes
	queue     EventQueue
	publisher events.Publisher
	log       *slog.Logger
	now       func() time.Time
	interval  time.Duration
}

func NewEventRelay(communes PendingEventCommunes, queue EventQueue, publisher events.Publisher, log *slog.Logger) *EventRelay {
	if communes == nil || queue == nil || publisher == nil {
		panic("app.NewEventRelay: communes, queue and publisher are required")
	}
	if log == nil {
		log = slog.Default()
	}
	return &EventRelay{communes: communes, queue: queue, publisher: publisher, log: log,
		now: func() time.Time { return time.Now().UTC() }, interval: EventRelayInterval}
}

// Run ticks until ctx is cancelled, backing off after a tick that met an outage.
func (r *EventRelay) Run(ctx context.Context) {
	failures := 0
	for {
		if r.Tick(ctx) {
			failures = 0
		} else {
			failures++
		}
		wait := r.interval
		for i := 0; i < failures && wait < relayMaxBackoff; i++ {
			wait *= 2
		}
		t := time.NewTimer(min(wait, relayMaxBackoff))
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
	}
}

var errRelayStop = errors.New("event relay: publish failed")

// Tick does one pass and reports whether it met no outage.
func (r *EventRelay) Tick(ctx context.Context) bool {
	if ctx.Err() != nil {
		return true
	}
	list, err := r.communes.CommunesWithUnpublishedEvents(ctx)
	if err != nil {
		r.log.Warn("chuyển sự kiện identity: không đọc được danh sách xã", "err", err)
		return false
	}
	for _, t := range list {
		if err := r.drain(tenant.Into(ctx, t)); err != nil {
			if !errors.Is(err, errRelayStop) {
				r.log.Warn("chuyển sự kiện identity: lỗi", "xa", string(t), "err", err)
			}
			return false
		}
	}
	return true
}

// drain publishes one batch of the commune of ctx.
func (r *EventRelay) drain(ctx context.Context) error {
	t := tenant.MustFrom(ctx)
	rows, err := r.queue.Unpublished(ctx, relayBatch)
	if err != nil {
		return err
	}
	done := make([]string, 0, len(rows))
	var failed error
	for _, row := range rows {
		env := events.Envelope{ID: row.ID, Name: row.Name, TenantID: t, At: row.OccurredAt, Payload: row.Payload}
		if err := r.publisher.Publish(ctx, env); err != nil {
			r.log.Warn("chuyển sự kiện identity: broker không nhận", "xa", string(t), "event_id", row.ID, "event", row.Name, "err", err)
			if aerr := r.queue.RecordFailedAttempt(ctx, row.ID); aerr != nil {
				r.log.Warn("chuyển sự kiện identity: không đếm được lần lỗi", "xa", string(t), "event_id", row.ID, "err", aerr)
			}
			failed = errRelayStop
			break
		}
		done = append(done, row.ID)
	}
	if len(done) > 0 {
		if err := r.queue.MarkPublished(ctx, done, r.now()); err != nil {
			// Published but not stamped: republished next tick under the same id — a duplicate, never a loss.
			return err
		}
	}
	return failed
}
