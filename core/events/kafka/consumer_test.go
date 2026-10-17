package kafka

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/vihat/vigov/core/config"
	"github.com/vihat/vigov/core/events"
	"github.com/vihat/vigov/core/tenant"
)

// fakeFetcher hands out one batch, then blocks until the context ends.
type fakeFetcher struct {
	mu        sync.Mutex
	batches   []kgo.Fetches
	committed []*kgo.Record
	closed    bool
}

func (f *fakeFetcher) PollFetches(ctx context.Context) kgo.Fetches {
	f.mu.Lock()
	if len(f.batches) > 0 {
		b := f.batches[0]
		f.batches = f.batches[1:]
		f.mu.Unlock()
		return b
	}
	f.mu.Unlock()
	<-ctx.Done()
	return nil
}

func (f *fakeFetcher) CommitRecords(_ context.Context, rs ...*kgo.Record) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.committed = append(f.committed, rs...)
	return nil
}

func (f *fakeFetcher) Close() { f.closed = true }

func batch(recs ...*kgo.Record) kgo.Fetches {
	return kgo.Fetches{{Topics: []kgo.FetchTopic{{Topic: "dev.identity.citizen_contact_changed.v1",
		Partitions: []kgo.FetchPartition{{Partition: 0, Records: recs}}}}}}
}

func envelopeRecord(offset int64, id string, commune tenant.ID) *kgo.Record {
	v := fmt.Sprintf(`{"id":%q,"name":"identity.citizen_contact_changed.v1","tenant_id":%q,"at":"2026-10-17T00:00:00Z","payload":{"contact_id":"x","op":"upsert"}}`, id, commune)
	return &kgo.Record{Topic: "dev.identity.citizen_contact_changed.v1", Offset: offset, Value: []byte(v)}
}

func runUntilCommitted(t *testing.T, f *fakeFetcher, h events.Handler, want int) {
	t.Helper()
	c := newConsumer(f, "dev.identity.citizen-search", nil)
	c.wait = func(int) time.Duration { return time.Millisecond }
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = c.Run(ctx, h); close(done) }()
	deadline := time.After(5 * time.Second)
	for {
		f.mu.Lock()
		n := len(f.committed)
		f.mu.Unlock()
		if n >= want {
			break
		}
		select {
		case <-deadline:
			cancel()
			t.Fatalf("committed %d records, want %d", n, want)
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	<-done
}

// The handler runs with the envelope's commune in context, and the offset is committed after it.
func TestConsumerDispatchesWithCommuneAndCommitsAfterSuccess(t *testing.T) {
	f := &fakeFetcher{batches: []kgo.Fetches{batch(envelopeRecord(1, "e1", testCommune))}}
	var got tenant.ID
	runUntilCommitted(t, f, func(ctx context.Context, e events.Envelope) error {
		got = tenant.MustFrom(ctx)
		return nil
	}, 1)
	if got != testCommune {
		t.Errorf("commune in context = %q, want %q", got, testCommune)
	}
}

// A transient error is retried in place; the offset is committed only once the handler succeeds.
func TestConsumerRetriesTransientErrorBeforeCommitting(t *testing.T) {
	f := &fakeFetcher{batches: []kgo.Fetches{batch(envelopeRecord(1, "e1", testCommune))}}
	calls := 0
	runUntilCommitted(t, f, func(context.Context, events.Envelope) error {
		calls++
		if calls < 3 {
			return errors.New("es down")
		}
		return nil
	}, 1)
	if calls != 3 {
		t.Errorf("handler ran %d times, want 3 (two failures, one success)", calls)
	}
}

// No commune, not an envelope, and ErrPermanent are skipped AND committed — no retry can change them.
func TestConsumerSkipsWhatCanNeverSucceed(t *testing.T) {
	bad := &kgo.Record{Offset: 2, Value: []byte("not json")}
	f := &fakeFetcher{batches: []kgo.Fetches{batch(
		envelopeRecord(1, "e1", ""), bad, envelopeRecord(3, "e3", testCommune),
	)}}
	calls := 0
	runUntilCommitted(t, f, func(context.Context, events.Envelope) error {
		calls++
		return fmt.Errorf("payload: %w", events.ErrPermanent)
	}, 3)
	if calls != 1 {
		t.Errorf("handler ran %d times, want 1 — a message without a commune never reaches it", calls)
	}
}

// A cancelled context leaves the failing record UNcommitted: it is redelivered after the restart.
func TestConsumerDoesNotCommitWhenStoppedMidRetry(t *testing.T) {
	f := &fakeFetcher{batches: []kgo.Fetches{batch(envelopeRecord(1, "e1", testCommune))}}
	c := newConsumer(f, "g", nil)
	c.wait = func(int) time.Duration { return time.Hour }
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		_ = c.Run(ctx, func(context.Context, events.Envelope) error {
			select {
			case started <- struct{}{}:
			default:
			}
			return errors.New("transient")
		})
		close(done)
	}()
	<-started
	cancel()
	<-done
	if len(f.committed) != 0 {
		t.Errorf("committed %d records after a stop mid-retry, want 0", len(f.committed))
	}
}

// The names come from the profile: a bad profile or event name is refused before any client exists.
func TestNewConsumerRefusesBadNames(t *testing.T) {
	b := config.KafkaEventBroker{Addresses: []string{"kafka-0:9092"}}
	if _, err := NewConsumer("identity", "qa", "identity", "citizen-search", []string{"identity.citizen_contact_changed.v1"}, b, nil); !errors.Is(err, events.ErrNoProfile) {
		t.Errorf("unknown profile: err = %v", err)
	}
	if _, err := NewConsumer("identity", "dev", "identity", "citizen-search", []string{"no-version"}, b, nil); err == nil || !strings.Contains(err.Error(), "event name") {
		t.Errorf("unversioned event: err = %v", err)
	}
	if _, err := NewConsumer("identity", "dev", "identity", "citizen-search", nil, b, nil); err == nil {
		t.Error("no events: want an error")
	}
	if _, err := NewConsumer("identity", "dev", "identity", "citizen-search", []string{"identity.citizen_contact_changed.v1"}, config.KafkaEventBroker{}, nil); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("no broker: err = %v", err)
	}
	c, err := NewConsumer("identity", "dev", "identity", "citizen-search", []string{"identity.citizen_contact_changed.v1"}, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if c.Group() != "dev.identity.citizen-search" {
		t.Errorf("group = %q", c.Group())
	}
}
