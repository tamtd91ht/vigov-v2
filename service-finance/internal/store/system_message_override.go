package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// SystemMessageOverrideStore reads and writes a commune's rewording of this service's system
// sentences (migration 0010). SQL only: which key exists, what the default is and how the two
// combine are internal/domain's.
//
// It holds *store.DB, never a *sql.DB: the commune is $1 in every statement, taken from the
// context (rule 1, invariants 4 and 5), and no method takes it as a parameter.
type SystemMessageOverrideStore struct {
	db *store.DB
}

func NewSystemMessageOverrideStore(db *store.DB) *SystemMessageOverrideStore {
	return &SystemMessageOverrideStore{db: db}
}

// ErrTooManyMessageOverrides — more live rows than the catalogue has keys. UNIQUE (tenant_id,
// live_key) and the key CHECK make that impossible; reaching it means the schema and the code
// disagree, and the read refuses rather than guessing which row is in force.
var ErrTooManyMessageOverrides = errors.New("system_message_override: số dòng đang dùng vượt số khoá")

// Read by position, in lockstep with every Scan below.
const systemMessageOverrideCols = `id, message_key, message_text, updated_at, updated_by, is_active`

// scanOverride is the ONE Scan of systemMessageOverrideCols. `is_active` is read and stored as its
// negation (domain.MessageOverride.Inactive) so the zero value of the struct means "in force", as the
// column's default does.
func scanOverride(scan func(...any) error) (domain.MessageOverride, error) {
	var o domain.MessageOverride
	var active bool
	err := scan(&o.ID, &o.Key, &o.Text, &o.UpdatedAt, &o.UpdatedBy, &active)
	o.Inactive = !active
	return o, err
}

// ListLive reads the commune's live overrides. SOFT-DELETED ROWS ARE EXCLUDED (rule 7, invariant
// 2): a reverted wording is history, not the sentence in force.
//
// BOUNDED BY THE CATALOGUE, not paginated: at most one live row per key, so the ceiling is the
// number of keys, plus one so that "too many" is detectable instead of silently truncated.
func (s *SystemMessageOverrideStore) ListLive(ctx context.Context) ([]domain.MessageOverride, error) {
	limit := len(domain.ShippedMessages())
	rows, err := s.db.For(ctx).Query(ctx, systemMessageOverrideCols, "system_message_override",
		`AND deleted_at IS NULL ORDER BY message_key LIMIT $2`, limit+1)
	if err != nil {
		return nil, fmt.Errorf("system_message_override: đọc danh sách: %w", err)
	}
	defer rows.Close()

	out := make([]domain.MessageOverride, 0, limit)
	for rows.Next() {
		o, err := scanOverride(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("system_message_override: đọc dòng: %w", err)
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("system_message_override: duyệt kết quả: %w", err)
	}
	if len(out) > limit {
		return nil, ErrTooManyMessageOverrides
	}
	return out, nil
}

// LiveForUpdate reads the live override of one key inside the transaction and LOCKS it. nil means
// the commune is on the default.
//
// THE LOCK IS WHAT MAKES read-decide-write SAFE for an existing row: two administrators saving the
// same card both read the old text, and without it the audit entry of the second would name a
// "before" that was already gone. It cannot lock a row that does not exist yet — two concurrent
// first saves both see nil and the second INSERT fails on UNIQUE (tenant_id, live_key): one row,
// one 500, never two live wordings.
func (s *SystemMessageOverrideStore) LiveForUpdate(ctx context.Context, tx *store.ScopedTx, key string) (*domain.MessageOverride, error) {
	const stmt = `SELECT ` + systemMessageOverrideCols + ` FROM system_message_override ` +
		`WHERE tenant_id = $1 AND message_key = $2 AND deleted_at IS NULL FOR UPDATE`

	o, err := scanOverride(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), key).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("system_message_override: đọc dòng để sửa: %w", err)
	}
	return &o, nil
}

// AddOverride writes the commune's first wording of a key (or its first after a revert).
func (s *SystemMessageOverrideStore) AddOverride(ctx context.Context, tx *store.ScopedTx, o domain.MessageOverride) error {
	const stmt = `INSERT INTO system_message_override
		(tenant_id, id, message_key, message_text, created_at, created_by, updated_at, updated_by, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $5, $6, $7)`
	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID()), o.ID, o.Key, o.Text, o.UpdatedAt, o.UpdatedBy, !o.Inactive); err != nil {
		return fmt.Errorf("system_message_override: chèn: %w", err)
	}
	return nil
}

// UpdateText rewrites the live wording. `message_key` is not in the statement: a row never moves
// to another key.
func (s *SystemMessageOverrideStore) UpdateText(ctx context.Context, tx *store.ScopedTx, o domain.MessageOverride) error {
	const stmt = `UPDATE system_message_override SET message_text = $3, updated_at = $4, updated_by = $5 ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), o.ID, o.Text, o.UpdatedAt, o.UpdatedBy)
	if err != nil {
		return fmt.Errorf("system_message_override: cập nhật: %w", err)
	}
	return doiMotDong(res, "cập nhật câu hệ thống")
}

// SoftDelete retires the live wording — "Khôi phục câu mặc định". All three of rule 7 invariant
// 1's columns in one statement; `AND deleted_at IS NULL` so a second revert cannot rewrite who
// reverted it or when.
func (s *SystemMessageOverrideStore) SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string, at time.Time) error {
	const stmt = `UPDATE system_message_override SET deleted_at = $3, deleted_by = $4, delete_reason = $5 ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, at, by, reason)
	if err != nil {
		return fmt.Errorf("system_message_override: xoá mềm: %w", err)
	}
	return doiMotDong(res, "khôi phục câu mặc định")
}

// SetActive is "Tắt / Bật lại" (migration 0017 §A): the switch and who/when, nothing else — the wording stays
// exactly as it was, which is what makes "Bật lại" bring the same words back.
func (s *SystemMessageOverrideStore) SetActive(ctx context.Context, tx *store.ScopedTx, id string, active bool, by string, at time.Time) error {
	const stmt = `UPDATE system_message_override SET is_active = $3, updated_at = $4, updated_by = $5 ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, active, at, by)
	if err != nil {
		return fmt.Errorf("system_message_override: tắt/bật: %w", err)
	}
	return doiMotDong(res, "tắt/bật câu hệ thống")
}
