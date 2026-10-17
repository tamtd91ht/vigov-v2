package kafka

// Consumer is the Kafka consumer of events.Envelope — the other half of Publisher (ADR 0010, ADR 0101 #11).
//
// THE SHAPE A SERVICE SEES IS ONE FUNCTION: an events.Handler. Everything Kafka-specific stays here, because
// service_boundary_guard check 5 blocks a Kafka client import anywhere but core/events — and because the two
// names the broker knows are built ONLY from the profile:
//
//	group   events.ConsumerGroup(profile, service, purpose) — "prod.identity.citizen-search". dev and prod share
//	        one cluster (owner, 17/10/2026); a group without the prefix would let a dev pod take prod's partitions
//	topics  events.Topic(profile, name) for every event name the consumer declares
//
// AT LEAST ONCE, NEVER AT MOST ONCE:
//
//   - auto-commit is OFF. An offset is committed only for a record whose handler returned nil (or whose
//     message can never be processed — below), and only after the handler returned. A crash between the
//     handler and the commit redelivers the record, which is why every handler must be idempotent
//     (rule 2, invariant 5).
//   - a handler error is RETRIED, in place, with a capped backoff, until it succeeds or the process stops.
//     Moving past it would commit an offset beyond a message that was never processed — a silent loss.
//     The cost is stated: one message that fails forever stops its partition. That is visible (the warn
//     line repeats, the consumer lag grows); a skipped message is not.
//   - THREE cases are skipped and committed, because no retry can ever change them: a record whose value
//     is not an envelope, an envelope with no commune (events.Dispatch refuses it, rule 1 invariant 9), and
//     a handler error wrapping events.ErrPermanent. Each is logged by topic, partition, offset, envelope id
//     and name — never the payload, which a producer elsewhere may have filled with personal data (rule 3).
//
// A NEW GROUP STARTS AT THE EARLIEST RETAINED OFFSET: a consumer deployed after its producer must still see
// what the producer already published, and the handlers are idempotent, so reading from the start costs
// time, never correctness.
//
// TOPICS ARE NOT CREATED HERE, for Publisher's reason (kafka.go): partition count and retention are
// operations' decisions.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/vihat/vigov/core/config"
	"github.com/vihat/vigov/core/events"
)

// consumerMaxBackoff caps the wait between two attempts at one failing record.
const consumerMaxBackoff = time.Minute

// fetcher is the part of *kgo.Client the loop uses — an interface so the loop is testable without a broker.
type fetcher interface {
	PollFetches(ctx context.Context) kgo.Fetches
	CommitRecords(ctx context.Context, rs ...*kgo.Record) error
	Close()
}

// Consumer reads the declared events of one consumer group. Build it with NewConsumer; Run blocks.
type Consumer struct {
	cl    fetcher
	group string
	log   *slog.Logger
	// wait is the pause before attempt n+1 of one record. Injected so a test does not sleep.
	wait func(attempt int) time.Duration
}

// NewConsumer builds the consumer of `eventNames` (each "<service>.<fact>.v<N>") for the group
// events.ConsumerGroup(profile, service, purpose). An invalid profile, service, purpose or event name is
// refused HERE, at startup. No connection is opened — franz-go dials on first poll.
func NewConsumer(clientID, profile, service, purpose string, eventNames []string, b config.KafkaEventBroker, log *slog.Logger) (*Consumer, error) {
	group, err := events.ConsumerGroup(profile, service, purpose)
	if err != nil {
		return nil, fmt.Errorf("kafka: %w", err)
	}
	if len(eventNames) == 0 {
		return nil, errors.New("kafka: a consumer must declare at least one event")
	}
	topics := make([]string, 0, len(eventNames))
	for _, n := range eventNames {
		t, err := events.Topic(profile, n)
		if err != nil {
			return nil, fmt.Errorf("kafka: %w", err)
		}
		topics = append(topics, t)
	}
	opts, err := clientOptions(clientID, b)
	if err != nil {
		return nil, err
	}
	opts = append(opts,
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topics...),
		kgo.DisableAutoCommit(),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("kafka: build the consumer: %w", err)
	}
	return newConsumer(cl, group, log), nil
}

func newConsumer(cl fetcher, group string, log *slog.Logger) *Consumer {
	if log == nil {
		log = slog.Default()
	}
	return &Consumer{cl: cl, group: group, log: log, wait: backoff}
}

// Group is the consumer group's name — for the startup log line.
func (c *Consumer) Group() string { return c.group }

// backoff doubles from one second, capped at consumerMaxBackoff.
func backoff(attempt int) time.Duration {
	d := time.Second
	for i := 1; i < attempt && d < consumerMaxBackoff; i++ {
		d *= 2
	}
	return min(d, consumerMaxBackoff)
}

// Run polls until ctx is cancelled (returns nil) or the client is closed. h runs with the envelope's commune
// in its context (events.Dispatch).
func (c *Consumer) Run(ctx context.Context, h events.Handler) error {
	for {
		fs := c.cl.PollFetches(ctx)
		if ctx.Err() != nil || fs.IsClientClosed() {
			return nil
		}
		fs.EachError(func(topic string, partition int32, err error) {
			// A fetch error is the broker's, never a message's: retried by the next poll.
			c.log.Warn("kafka: lỗi đọc", "group", c.group, "topic", topic, "partition", partition, "err", err)
		})
		var done []*kgo.Record
		fs.EachRecord(func(r *kgo.Record) {
			if c.handle(ctx, r, h) {
				done = append(done, r)
			}
		})
		if len(done) == 0 {
			continue
		}
		// A commit failure redelivers these records later — at least once, never lost.
		if err := c.cl.CommitRecords(context.WithoutCancel(ctx), done...); err != nil {
			c.log.Warn("kafka: không ghi được offset", "group", c.group, "so_ban_ghi", len(done), "err", err)
		}
	}
}

// Close releases the connections.
func (c *Consumer) Close() { c.cl.Close() }

// handle processes one record and reports whether its offset may be committed.
func (c *Consumer) handle(ctx context.Context, r *kgo.Record, h events.Handler) bool {
	if ctx.Err() != nil {
		return false
	}
	var e events.Envelope
	if err := json.Unmarshal(r.Value, &e); err != nil || e.Name == "" {
		// Not an envelope: no retry can parse it. The error of json.Unmarshal may quote the bytes — not logged.
		c.log.Error("kafka: bản ghi không phải phong bì sự kiện — bỏ qua", "group", c.group,
			"topic", r.Topic, "partition", r.Partition, "offset", r.Offset)
		return true
	}
	for attempt := 1; ; attempt++ {
		err := events.Dispatch(ctx, e, h)
		switch {
		case err == nil:
			return true
		case errors.Is(err, events.ErrNoTenant), errors.Is(err, events.ErrPermanent):
			c.log.Error("kafka: sự kiện không xử lý được — bỏ qua", "group", c.group, "topic", r.Topic,
				"partition", r.Partition, "offset", r.Offset, "event_id", e.ID, "event", e.Name, "err", err)
			return true
		}
		c.log.Warn("kafka: xử lý sự kiện lỗi — thử lại", "group", c.group, "event_id", e.ID, "event", e.Name,
			"lan", attempt, "err", err)
		t := time.NewTimer(c.wait(attempt))
		select {
		case <-ctx.Done():
			t.Stop()
			return false
		case <-t.C:
		}
	}
}
