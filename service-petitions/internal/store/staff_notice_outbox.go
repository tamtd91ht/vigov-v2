package store

// `staff_notice_outbox` (migration 0036, ADR 0086 A1) — staff notices owed by an act, waiting for the
// relay. SQL, and nothing else.
//
// THE WRITE TAKES THE ACT'S TRANSACTION. Record has no other signature, so a notice cannot be recorded
// in one transaction and its act committed in another — the shape SuKienDiStore has for the citizen
// channel.
//
// THE RELAY'S STATEMENTS run in ONE commune each (the context's), like every statement here: the list
// of communes owing a notice is store/crosstenant's, and it returns identifiers only.
//
// NOT AUDITED. Marking a row delivered or refused is infrastructure state, not business data (the
// migration's header); the notice's trail is the entry comms writes in its own delivery transaction.

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/vihat/vigov/core/store"
)

// StaffNoticeOutboxStore is the only path to `staff_notice_outbox`.
type StaffNoticeOutboxStore struct {
	db *store.DB
}

func NewStaffNoticeOutboxStore(db *store.DB) *StaffNoticeOutboxStore {
	return &StaffNoticeOutboxStore{db: db}
}

// StaffNoticeRow is one notice waiting to leave this service.
type StaffNoticeRow struct {
	ID string
	// Name is the fact with its version — `tasks.assigned.v1` (rule 2, invariant 4).
	Name string
	// IdempotencyKey is comms' key, `<stored kind>:<act row id>`.
	IdempotencyKey string
	// Payload is the protojson of comms.v1.StaffNotification. Never logged: it carries the title.
	Payload    []byte
	OccurredAt time.Time
	Attempts   int
}

// FailedClassInvalidArgument is the one refusal class the relay records today: comms answered
// INVALID_ARGUMENT for that row alone.
const FailedClassInvalidArgument = "invalid_argument"

// Record writes one notice INSIDE the act's transaction. `delivered_at` is not written: NULL is "owed".
func (s *StaffNoticeOutboxStore) Record(ctx context.Context, tx *store.ScopedTx, r StaffNoticeRow) error {
	const stmt = `INSERT INTO staff_notice_outbox (tenant_id, id, name, idempotency_key, payload, occurred_at)
		VALUES ($1,$2,$3,$4,$5,$6)`
	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID()), r.ID, r.Name, r.IdempotencyKey, r.Payload,
		r.OccurredAt); err != nil {
		// NOT the payload: an INSERT error can quote the row on some drivers, and it carries a title.
		return fmt.Errorf("staff_notice_outbox: ghi thông báo chờ gửi: %w", err)
	}
	return nil
}

// Undelivered reads at most `limit` rows still owed in the commune of ctx, oldest act first. A row comms
// refused (failed_class set) is not read again until somebody clears its class.
func (s *StaffNoticeOutboxStore) Undelivered(ctx context.Context, limit int) ([]StaffNoticeRow, error) {
	rows, err := s.db.For(ctx).Query(ctx,
		"id, name, idempotency_key, payload, occurred_at, attempts", "staff_notice_outbox",
		"AND delivered_at IS NULL AND failed_class IS NULL ORDER BY occurred_at, id LIMIT $2", limit)
	if err != nil {
		return nil, fmt.Errorf("staff_notice_outbox: đọc thông báo chờ gửi: %w", err)
	}
	defer rows.Close()
	var out []StaffNoticeRow
	for rows.Next() {
		var r StaffNoticeRow
		if err := rows.Scan(&r.ID, &r.Name, &r.IdempotencyKey, &r.Payload, &r.OccurredAt, &r.Attempts); err != nil {
			return nil, fmt.Errorf("staff_notice_outbox: đọc dòng: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("staff_notice_outbox: đọc thông báo chờ gửi: %w", err)
	}
	return out, nil
}

// MarkDelivered stamps the rows comms accepted. `delivered_at IS NULL` in the WHERE clause keeps the
// FIRST delivery instant if a second replica ever raced this one.
func (s *StaffNoticeOutboxStore) MarkDelivered(ctx context.Context, ids []string, at time.Time) error {
	return s.update(ctx, ids, "delivered_at = $2", "AND delivered_at IS NULL", at)
}

// RecordFailedAttempt counts one more unsuccessful attempt on rows that stay owed.
func (s *StaffNoticeOutboxStore) RecordFailedAttempt(ctx context.Context, ids []string) error {
	return s.update(ctx, ids, "attempts = attempts + 1", "AND delivered_at IS NULL")
}

// MarkFailed sets rows comms refused aside under `class`, counting the attempt.
func (s *StaffNoticeOutboxStore) MarkFailed(ctx context.Context, ids []string, class string) error {
	if class == "" {
		return fmt.Errorf("staff_notice_outbox: thiếu lớp lỗi")
	}
	return s.update(ctx, ids, "attempts = attempts + 1, failed_class = $2", "AND delivered_at IS NULL", class)
}

// update runs one UPDATE over a set of ids in the commune of ctx. NEVER WITHOUT A FILTER (rule 7,
// forbidden #2): the commune is $1 and an empty id list writes nothing at all.
func (s *StaffNoticeOutboxStore) update(ctx context.Context, ids []string, set, extra string, args ...any) error {
	if len(ids) == 0 {
		return nil
	}
	all := append([]any{}, args...)
	marks := make([]string, len(ids))
	for i, id := range ids {
		all = append(all, id)
		marks[i] = "$" + strconv.Itoa(len(all)+1)
	}
	stmt := "UPDATE staff_notice_outbox SET " + set + " WHERE tenant_id = $1 AND id IN (" +
		strings.Join(marks, ",") + ") " + extra
	return s.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if _, err := tx.Exec(ctx, stmt, append([]any{string(tx.TenantID())}, all...)...); err != nil {
			return fmt.Errorf("staff_notice_outbox: cập nhật trạng thái gửi: %w", err)
		}
		return nil
	})
}
