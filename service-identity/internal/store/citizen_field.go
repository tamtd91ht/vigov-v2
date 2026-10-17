package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// CitizenFieldStore reads and writes `citizen_field` (migration 0036, ADR 0101) — a commune's dynamic
// attributes of the citizen directory. The CitizenDirectoryStore properties hold here too: the commune is
// $1 in every statement, nothing here opens a transaction (every write takes the one its audit entry is
// written in), no hard delete (retire = soft; the 0036 trigger refuses one anyway), nothing logs.
type CitizenFieldStore struct {
	db *store.DB
}

func NewCitizenFieldStore(db *store.DB) *CitizenFieldStore { return &CitizenFieldStore{db: db} }

var (
	// ErrCitizenFieldNotFound — no field of THIS commune has this code (one answer for every case).
	ErrCitizenFieldNotFound = errors.New("thuộc tính danh bạ: không tìm thấy thuộc tính")
	// ErrCitizenFieldCodeTaken — the commune already has a field (live or retired) with this code. A
	// retired code is never reused: stored values are keyed by it (migration 0036 header).
	ErrCitizenFieldCodeTaken = errors.New("thuộc tính danh bạ: mã này đã được dùng trong xã (kể cả thuộc tính đã ngừng dùng)")
)

const citizenFieldCols = `code, label, field_type, coalesce(placeholder, ''), coalesce(options::text, ''), ` +
	`coalesce(number_kind, ''), coalesce(date_pattern, ''), required, show_in_list, sort_order, ` +
	`created_at, updated_at, retired_at, coalesce(retired_by, ''), coalesce(retire_reason, ''), ` +
	`display_mode, layout_width`

func scanCitizenField(scan func(...any) error) (domain.CitizenField, error) {
	var f domain.CitizenField
	var typ, opts string
	var retired sql.NullTime
	if err := scan(&f.Code, &f.Label, &typ, &f.Placeholder, &opts, &f.NumberKind, &f.DatePattern,
		&f.Required, &f.ShowInList, &f.SortOrder, &f.CreatedAt, &f.UpdatedAt, &retired, &f.RetiredBy,
		&f.RetireReason, &f.DisplayMode, &f.LayoutWidth); err != nil {
		return domain.CitizenField{}, fmt.Errorf("thuộc tính danh bạ: đọc dòng: %w", err)
	}
	f.Type = domain.CitizenFieldType(typ)
	if retired.Valid {
		t := retired.Time
		f.RetiredAt = &t
	}
	if opts != "" {
		if err := json.Unmarshal([]byte(opts), &f.Options); err != nil {
			return domain.CitizenField{}, errors.New("thuộc tính danh bạ: lựa chọn đã lưu không đọc được")
		}
	}
	return f, nil
}

func readCitizenFields(rows *sql.Rows, err error) ([]domain.CitizenField, error) {
	if err != nil {
		return nil, fmt.Errorf("thuộc tính danh bạ: đọc danh sách: %w", err)
	}
	defer rows.Close()
	out := []domain.CitizenField{}
	for rows.Next() {
		f, err := scanCitizenField(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("thuộc tính danh bạ: duyệt danh sách: %w", err)
	}
	return out, nil
}

// List reads the commune's fields in the configured order; retired ones only when asked.
func (s *CitizenFieldStore) List(ctx context.Context, includeRetired bool) ([]domain.CitizenField, error) {
	tail := `ORDER BY sort_order, code`
	if !includeRetired {
		tail = `AND retired_at IS NULL ` + tail
	}
	return readCitizenFields(s.db.For(ctx).Query(ctx, citizenFieldCols, "citizen_field", tail))
}

// Get reads one field (live or retired) of this commune.
func (s *CitizenFieldStore) Get(ctx context.Context, code string) (domain.CitizenField, bool, error) {
	fs, err := readCitizenFields(s.db.For(ctx).Query(ctx, citizenFieldCols, "citizen_field", `AND code = $2`, code))
	if err != nil || len(fs) == 0 {
		return domain.CitizenField{}, false, err
	}
	return fs[0], true, nil
}

// LockAndListLive takes the commune's field-configuration lock for the rest of the transaction and reads
// the live fields — so "≤ 50 live" (ADR 0101 #1) and the reorder's "exactly the live set" are decided on a
// set no concurrent write of the same commune can change before the commit.
func (s *CitizenFieldStore) LockAndListLive(ctx context.Context, tx *store.ScopedTx) ([]domain.CitizenField, error) {
	if tx == nil {
		return nil, ErrThieuGiaoDich
	}
	// A transaction-scoped advisory lock keyed by the commune (tx.TenantID()): released by the commit or
	// the rollback.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
		"citizen_field:"+string(tx.TenantID())); err != nil {
		return nil, fmt.Errorf("thuộc tính danh bạ: khoá cấu hình: %w", err)
	}
	// ScopedTx.Query binds tenant_id = $1 from the transaction's commune (rule 1, invariant 5).
	return readCitizenFields(tx.Query(ctx, citizenFieldCols, "citizen_field", `AND retired_at IS NULL ORDER BY sort_order, code`))
}

func optionsJSON(f domain.CitizenField) (any, error) {
	if !f.Type.HasOptions() {
		return nil, nil
	}
	b, err := json.Marshal(f.Options)
	if err != nil {
		return nil, fmt.Errorf("thuộc tính danh bạ: mã hoá lựa chọn: %w", err)
	}
	return string(b), nil
}

// Insert writes one new field. CreatedAt is the caller's instant.
func (s *CitizenFieldStore) Insert(ctx context.Context, tx *store.ScopedTx, f domain.CitizenField) error {
	if tx == nil {
		return ErrThieuGiaoDich
	}
	opts, err := optionsJSON(f)
	if err != nil {
		return err
	}
	const stmt = `INSERT INTO citizen_field (tenant_id, code, label, field_type, placeholder, options,
			number_kind, date_pattern, required, show_in_list, sort_order, created_at, updated_at,
			display_mode, layout_width)
		VALUES ($1, $2, $3, $4, nullif($5, ''), $6::jsonb, nullif($7, ''), nullif($8, ''), $9, $10, $11, $12, $12,
			$13, $14)`
	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID()), f.Code, f.Label, string(f.Type), f.Placeholder, opts,
		f.NumberKind, f.DatePattern, f.Required, f.ShowInList, f.SortOrder, f.CreatedAt,
		f.DisplayMode, f.LayoutWidth); err != nil {
		if strings.Contains(err.Error(), "citizen_field_pkey") {
			return ErrCitizenFieldCodeTaken
		}
		return fmt.Errorf("thuộc tính danh bạ: thêm: %w", err)
	}
	return nil
}

// Update rewrites the editable columns of one field (label, placeholder, options, date_pattern, required,
// show_in_list, display_mode, layout_width) and updated_at. Code and type are not in the statement — the
// trigger refuses them anyway.
func (s *CitizenFieldStore) Update(ctx context.Context, tx *store.ScopedTx, f domain.CitizenField) error {
	if tx == nil {
		return ErrThieuGiaoDich
	}
	opts, err := optionsJSON(f)
	if err != nil {
		return err
	}
	const stmt = `UPDATE citizen_field SET label = $3, placeholder = nullif($4, ''), options = $5::jsonb,
			date_pattern = nullif($6, ''), required = $7, show_in_list = $8, updated_at = $9,
			display_mode = $10, layout_width = $11
		WHERE tenant_id = $1 AND code = $2`
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), f.Code, f.Label, f.Placeholder, opts, f.DatePattern,
		f.Required, f.ShowInList, f.UpdatedAt, f.DisplayMode, f.LayoutWidth)
	if err != nil {
		return fmt.Errorf("thuộc tính danh bạ: sửa: %w", err)
	}
	return exactlyOneField(res)
}

// Retire soft-retires one LIVE field. `by` is the staff business code (rule 6, invariant 8). A second
// retire is not-found, never a rewrite of who retired it and why.
func (s *CitizenFieldStore) Retire(ctx context.Context, tx *store.ScopedTx, code, by, reason string, at time.Time) error {
	if tx == nil {
		return ErrThieuGiaoDich
	}
	if strings.TrimSpace(by) == "" {
		return ErrCitizenDirectoryActorMissing
	}
	const stmt = `UPDATE citizen_field SET retired_at = $3, retired_by = $4, retire_reason = $5, updated_at = $3
		WHERE tenant_id = $1 AND code = $2 AND retired_at IS NULL`
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), code, at, by, reason)
	if err != nil {
		return fmt.Errorf("thuộc tính danh bạ: ngừng dùng: %w", err)
	}
	return exactlyOneField(res)
}

// SetOrder writes sort_order = position for each code, in one statement. Codes not listed keep theirs.
func (s *CitizenFieldStore) SetOrder(ctx context.Context, tx *store.ScopedTx, codes []string, at time.Time) error {
	if tx == nil {
		return ErrThieuGiaoDich
	}
	if len(codes) == 0 {
		return nil
	}
	const stmt = `UPDATE citizen_field f SET sort_order = o.pos - 1, updated_at = $3
		FROM unnest($2::text[]) WITH ORDINALITY AS o(code, pos)
		WHERE f.tenant_id = $1 AND f.code = o.code AND f.sort_order <> o.pos - 1`
	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID()), codes, at); err != nil {
		return fmt.Errorf("thuộc tính danh bạ: sắp xếp: %w", err)
	}
	return nil
}

func exactlyOneField(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("thuộc tính danh bạ: đếm dòng đã ghi: %w", err)
	}
	if n == 0 {
		return ErrCitizenFieldNotFound
	}
	return nil
}
