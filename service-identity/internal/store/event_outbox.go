package store

// `event_outbox` (migration 0036, ADR 0101 #11) — facts identity owes to Kafka. SQL, and nothing else.
//
// THE WRITE TAKES THE ACT'S TRANSACTION. Record has no other signature, so a fact cannot be recorded in
// one transaction and its act committed in another (rule 2 invariant 6; skills/events-and-queues #5).
//
// THE RELAY'S STATEMENTS run in ONE commune each (the context's). Which communes owe a fact is
// store/crosstenant's question, answered with identifiers only.
//
// NOT AUDITED. A row published is infrastructure state, not business data; the act's own entry was
// written with the act.

import (
	"context"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/store"
)

// EventOutboxStore is the only path to `event_outbox`.
type EventOutboxStore struct {
	db *store.DB
}

func NewEventOutboxStore(db *store.DB) *EventOutboxStore { return &EventOutboxStore{db: db} }

// OutboxEvent is one fact waiting to leave identity.
type OutboxEvent struct {
	ID         string // ULID, also the envelope id
	Name       string // "identity.citizen_contact_changed.v1"
	Payload    []byte // protojson — no personal data by contract
	OccurredAt time.Time
}

// Record writes facts INSIDE the act's transaction — one statement for the whole slice (an Excel import
// writes one per row).
func (s *EventOutboxStore) Record(ctx context.Context, tx *store.ScopedTx, evs []OutboxEvent) error {
	if tx == nil {
		return ErrThieuGiaoDich
	}
	if len(evs) == 0 {
		return nil
	}
	ids := make([]string, len(evs))
	names := make([]string, len(evs))
	payloads := make([]string, len(evs))
	at := make([]time.Time, len(evs))
	for i, e := range evs {
		ids[i], names[i], payloads[i], at[i] = e.ID, e.Name, string(e.Payload), e.OccurredAt
	}
	const stmt = `INSERT INTO event_outbox (tenant_id, id, name, payload, occurred_at)
		SELECT $1, u.id, u.name, u.payload::jsonb, u.at
		FROM unnest($2::text[], $3::text[], $4::text[], $5::timestamptz[]) AS u(id, name, payload, at)`
	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID()), ids, names, payloads, at); err != nil {
		// Not the payload: an INSERT error can quote the row on some drivers.
		return fmt.Errorf("event_outbox: ghi sự kiện chờ gửi: %w", err)
	}
	return nil
}

// Unpublished reads at most `limit` rows still owed in the commune of ctx, oldest act first.
func (s *EventOutboxStore) Unpublished(ctx context.Context, limit int) ([]OutboxEvent, error) {
	rows, err := s.db.For(ctx).Query(ctx, "id, name, payload::text, occurred_at", "event_outbox",
		"AND published_at IS NULL ORDER BY occurred_at, id LIMIT $2", limit)
	if err != nil {
		return nil, fmt.Errorf("event_outbox: đọc sự kiện chờ gửi: %w", err)
	}
	defer rows.Close()
	var out []OutboxEvent
	for rows.Next() {
		var e OutboxEvent
		var payload string
		if err := rows.Scan(&e.ID, &e.Name, &payload, &e.OccurredAt); err != nil {
			return nil, fmt.Errorf("event_outbox: đọc dòng: %w", err)
		}
		e.Payload = []byte(payload)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("event_outbox: duyệt sự kiện chờ gửi: %w", err)
	}
	return out, nil
}

// MarkPublished stamps rows the broker acknowledged. `published_at IS NULL` keeps the FIRST instant if two
// replicas ever raced; a second publish of the same envelope id is harmless (consumers deduplicate).
func (s *EventOutboxStore) MarkPublished(ctx context.Context, ids []string, at time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	return s.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// NEVER WITHOUT A FILTER (rule 7, forbidden #2): tenant_id = $1 and an id list.
		_, err := tx.Exec(ctx, `UPDATE event_outbox SET published_at = $3
			WHERE tenant_id = $1 AND id = ANY($2) AND published_at IS NULL`, string(tx.TenantID()), ids, at)
		if err != nil {
			return fmt.Errorf("event_outbox: đánh dấu đã gửi: %w", err)
		}
		return nil
	})
}

// RecordFailedAttempt counts one more unsuccessful attempt on a row that stays owed.
func (s *EventOutboxStore) RecordFailedAttempt(ctx context.Context, id string) error {
	return s.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		_, err := tx.Exec(ctx, `UPDATE event_outbox SET attempts = attempts + 1
			WHERE tenant_id = $1 AND id = $2 AND published_at IS NULL`, string(tx.TenantID()), id)
		if err != nil {
			return fmt.Errorf("event_outbox: đếm lần gửi lỗi: %w", err)
		}
		return nil
	})
}
