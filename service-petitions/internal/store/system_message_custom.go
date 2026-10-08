package store

// The commune-sentence half of "Lời hệ thống" — `custom_system_message` (migration 0033 §B). Methods on
// SystemMessageOverrideStore so ONE value carries both halves of the screen (app.SystemMessageStore).
//
// SAME FOUR PROPERTIES AS EVERY WRITE PATH HERE: the commune is $1 from the transaction / context
// (rule 1, invariants 4 and 5); every read excludes soft-deleted rows except CustomKeyTaken, which
// exists to see them; there is no DELETE (the trigger refuses one anyway); nothing opens a transaction.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// ErrTooManyCustomMessages — more live commune sentences than domain.TranCustomMessages. The create
// path refuses at the ceiling, so reaching this means a writer bypassed it; the list refuses rather
// than truncates (a short list looks complete).
var ErrTooManyCustomMessages = errors.New("custom_system_message: số câu đang dùng vượt trần")

// Read by position, in lockstep with scanCustom.
const customMessageCols = `id, group_code, message_key, message_text, description, is_active, created_at, created_by, updated_at, updated_by`

func scanCustom(scan func(...any) error) (domain.CustomMessage, error) {
	var c domain.CustomMessage
	var desc sql.NullString
	err := scan(&c.ID, &c.Group, &c.Key, &c.Text, &desc, &c.Active, &c.CreatedAt, &c.CreatedBy, &c.UpdatedAt, &c.UpdatedBy)
	c.Description = desc.String
	return c, err
}

// isCustomKeyUniqueViolation — SQLSTATE 23505. The constraint name is not compared: on a partitioned
// table PostgreSQL reports the PARTITION's index name (custom_system_message_p07_…), which differs by
// commune.
func isCustomKeyUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// nullableText writes "" as NULL — the CHECK refuses a blank description.
func nullableText(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ListCustom reads the commune's live sentences, ordered by group then key (the partial index
// `custom_system_message_list`). LIMIT is the ceiling plus one, so "too many" is detectable.
func (s *SystemMessageOverrideStore) ListCustom(ctx context.Context) ([]domain.CustomMessage, error) {
	rows, err := s.db.For(ctx).Query(ctx, customMessageCols, "custom_system_message",
		`AND deleted_at IS NULL ORDER BY group_code, message_key LIMIT $2`, domain.TranCustomMessages+1)
	if err != nil {
		return nil, fmt.Errorf("custom_system_message: đọc danh sách: %w", err)
	}
	defer rows.Close()

	out := make([]domain.CustomMessage, 0, 8)
	for rows.Next() {
		c, err := scanCustom(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("custom_system_message: đọc dòng: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("custom_system_message: duyệt kết quả: %w", err)
	}
	if len(out) > domain.TranCustomMessages {
		return nil, ErrTooManyCustomMessages
	}
	return out, nil
}

// CustomForUpdate reads one LIVE sentence by key and locks it. nil = none (or deleted).
func (s *SystemMessageOverrideStore) CustomForUpdate(ctx context.Context, tx *store.ScopedTx, key string) (*domain.CustomMessage, error) {
	const stmt = `SELECT ` + customMessageCols + ` FROM custom_system_message ` +
		`WHERE tenant_id = $1 AND message_key = $2 AND deleted_at IS NULL FOR UPDATE`
	c, err := scanCustom(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), key).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("custom_system_message: đọc dòng để sửa: %w", err)
	}
	return &c, nil
}

// CustomKeyTaken reports whether the key exists in this commune — SOFT-DELETED ROWS INCLUDED (rule 7,
// invariant 3). The UNIQUE key is the real guard; this is the readable 409.
func (s *SystemMessageOverrideStore) CustomKeyTaken(ctx context.Context, tx *store.ScopedTx, key string) (bool, error) {
	const stmt = `SELECT count(*) FROM custom_system_message WHERE tenant_id = $1 AND message_key = $2`
	var n int
	if err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), key).Scan(&n); err != nil {
		return false, fmt.Errorf("custom_system_message: kiểm tra mã trùng: %w", err)
	}
	return n > 0, nil
}

// CountLiveCustom counts live sentences for the ceiling — the same set ListCustom returns.
func (s *SystemMessageOverrideStore) CountLiveCustom(ctx context.Context, tx *store.ScopedTx) (int, error) {
	const stmt = `SELECT count(*) FROM custom_system_message WHERE tenant_id = $1 AND deleted_at IS NULL`
	var n int
	if err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID())).Scan(&n); err != nil {
		return 0, fmt.Errorf("custom_system_message: đếm dòng đang sống: %w", err)
	}
	return n, nil
}

// AddCustom inserts one sentence.
func (s *SystemMessageOverrideStore) AddCustom(ctx context.Context, tx *store.ScopedTx, c domain.CustomMessage) error {
	const stmt = `INSERT INTO custom_system_message
		(tenant_id, id, group_code, message_key, message_text, description, is_active,
		 created_at, created_by, updated_at, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID()), c.ID, c.Group, c.Key, c.Text, nullableText(c.Description),
		c.Active, c.CreatedAt, c.CreatedBy, c.UpdatedAt, c.UpdatedBy); err != nil {
		// THE UNIQUE KEY IS THE REAL GUARD. CreateCustom counts before it inserts, so two concurrent
		// creates of one key both pass the count and the second meets UNIQUE (tenant_id, message_key)
		// here. That is the same refusal as the count's — a 409 the administrator can act on, not a
		// 500. The only other unique key on this table is the primary key, on a fresh ULID.
		if isCustomKeyUniqueViolation(err) {
			return fmt.Errorf("custom_system_message: chèn: %w", domain.ErrMessageCodeTaken)
		}
		return fmt.Errorf("custom_system_message: chèn: %w", err)
	}
	return nil
}

// UpdateCustom writes the three editable fields and who/when. `message_key`, `group_code` and the
// creation columns appear nowhere here; the trigger refuses them too (0033).
func (s *SystemMessageOverrideStore) UpdateCustom(ctx context.Context, tx *store.ScopedTx, c domain.CustomMessage) error {
	const stmt = `UPDATE custom_system_message SET message_text = $3, description = $4, is_active = $5, ` +
		`updated_at = $6, updated_by = $7 WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), c.ID, c.Text, nullableText(c.Description), c.Active,
		c.UpdatedAt, c.UpdatedBy)
	if err != nil {
		return fmt.Errorf("custom_system_message: cập nhật: %w", err)
	}
	return doiMotDong(res, "cập nhật câu xã tự thêm")
}

// SoftDeleteCustom writes all three of rule 7 invariant 1's columns in one statement. `AND deleted_at
// IS NULL` so a second delete cannot rewrite who deleted it or why.
func (s *SystemMessageOverrideStore) SoftDeleteCustom(ctx context.Context, tx *store.ScopedTx, id, by, reason string, at time.Time) error {
	const stmt = `UPDATE custom_system_message SET deleted_at = $3, deleted_by = $4, delete_reason = $5 ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, at, by, reason)
	if err != nil {
		return fmt.Errorf("custom_system_message: xoá mềm: %w", err)
	}
	return doiMotDong(res, "xoá câu xã tự thêm")
}
