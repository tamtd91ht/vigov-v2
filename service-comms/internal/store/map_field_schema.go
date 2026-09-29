package store

// The commune's map field schema — migrations/0007_map_field_schema.sql.
//
// FOUR THINGS HOLD ACROSS EVERY METHOD, the same four as the type catalogue's write path:
//
//  1. THE COMMUNE IS $1 IN EVERY STATEMENT, from Scoped / tx.TenantID(), never a parameter.
//  2. `asset_type_code`, `field_code` AND `value_type` APPEAR IN NO UPDATE. They are immutable; the
//     trigger refuses a change too, and their absence here keeps that floor unreachable from this
//     service.
//  3. EVERY READ EXCLUDES SOFT-DELETED ROWS except FieldCodeState, which exists to see them.
//  4. NOTHING HERE OPENS A TRANSACTION. The use case opens it and writes the audit entry inside.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// MapFieldSchemaStore reads and writes map_field_schema. Built from *store.DB and reaching the
// database only through Scoped / ScopedTx (rule 1, invariant 5).
type MapFieldSchemaStore struct {
	db *store.DB
}

func NewMapFieldSchemaStore(db *store.DB) *MapFieldSchemaStore {
	return &MapFieldSchemaStore{db: db}
}

// MapFieldSchemaCeiling bounds one commune's live rows, for the same reason the type catalogue has
// MapAssetTypeCeiling: the list is returned whole, so the bound cannot be a `limit`.
//
// 1000 is a few dozen fields for each of about a dozen groups, many times over — past it the
// content is an import run twice or a loop, not a form.
const MapFieldSchemaCeiling = 1000

var (
	// ErrMapFieldSchemaNotFound — no live row with that id in THIS commune. 404, and deliberately
	// the same answer for "another commune's row" as for "no such row".
	ErrMapFieldSchemaNotFound = errors.New("map_field_schema: không tồn tại trong xã này")

	// ErrFieldCodeTaken — a LIVE field of this type already has the key.
	ErrFieldCodeTaken = errors.New("map_field_schema: mã trường đã có ở nhóm này")

	// ErrFieldCodeRetired — the key belonged to a field that was DELETED. Kept separate from
	// ErrFieldCodeTaken because the caller's next step differs: a taken key is on the screen; a
	// retired one is not, and without its own sentence the refusal reads as a bug. An issued key
	// is never reissued (rule 7, invariant 3): assets that stored a value under it would silently
	// read the new field's meaning.
	ErrFieldCodeRetired = errors.New("map_field_schema: mã trường đã dùng cho một trường đã xoá, không cấp lại")

	// ErrAssetTypeMissing — the type code is not a live row of this commune's catalogue.
	ErrAssetTypeMissing = errors.New("map_field_schema: nhóm tài nguyên không có trong danh mục của xã")

	// ErrMapFieldSchemaFull — the commune is at MapFieldSchemaCeiling.
	ErrMapFieldSchemaFull = errors.New("map_field_schema: đã đạt số trường tối đa")

	// ErrTooManyMapFieldSchemas — the READ found more than the ceiling. Refused, never truncated:
	// a silently short list is a field missing from a form.
	ErrTooManyMapFieldSchemas = errors.New("map_field_schema: vượt trần")
)

// mapFieldSchemaColumns IS READ BY POSITION in scanMapFieldSchema. Three adjacent TEXT columns and
// two adjacent BOOLEANs: a swap here or there produces no error, only wrong data.
const mapFieldSchemaColumns = `id, asset_type_code, field_code, label, value_type, options, ` +
	`is_required, sort_order, is_active`

func scanMapFieldSchema(scan func(...any) error) (domain.MapFieldSchema, error) {
	var (
		m    domain.MapFieldSchema
		opts []byte
	)
	if err := scan(&m.ID, &m.AssetTypeCode, &m.FieldCode, &m.Label, &m.ValueType, &opts,
		&m.IsRequired, &m.SortOrder, &m.IsActive); err != nil {
		return domain.MapFieldSchema{}, err
	}
	var err error
	if m.Options, err = decodeOptions(opts); err != nil {
		return domain.MapFieldSchema{}, err
	}
	return m, nil
}

// optionJSON is the stored shape of one option. Tags written out so the column's shape does not
// silently follow a rename of the Go field.
type optionJSON struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

func decodeOptions(raw []byte) ([]domain.FieldOption, error) {
	var in []optionJSON
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, fmt.Errorf("map_field_schema: đọc options: %w", err)
		}
	}
	out := make([]domain.FieldOption, 0, len(in))
	for _, o := range in {
		out = append(out, domain.FieldOption{Value: o.Value, Label: o.Label})
	}
	return out, nil
}

func encodeOptions(opts []domain.FieldOption) (string, error) {
	out := make([]optionJSON, 0, len(opts))
	for _, o := range opts {
		out = append(out, optionJSON{Value: o.Value, Label: o.Label})
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("map_field_schema: mã hoá options: %w", err)
	}
	return string(b), nil
}

// List reads the commune's live fields, optionally of one asset type, ordered by type, display
// order and key — a TOTAL order, since (type, key) is unique.
//
// Disabled rows are returned (the tab shows them as `Tắt` and offers to re-enable); only
// soft-deleted rows drop out, here (rule 7, invariant 2).
func (s *MapFieldSchemaStore) List(ctx context.Context, assetTypeCode string) ([]domain.MapFieldSchema, error) {
	var (
		rows *sql.Rows
		err  error
	)
	// LIMIT is the ceiling PLUS ONE, so "too many" is detectable rather than indistinguishable from
	// a complete list of exactly that size.
	if assetTypeCode == "" {
		rows, err = s.db.For(ctx).Query(ctx, mapFieldSchemaColumns, "map_field_schema",
			`AND deleted_at IS NULL ORDER BY asset_type_code, sort_order, field_code LIMIT $2`,
			MapFieldSchemaCeiling+1)
	} else {
		rows, err = s.db.For(ctx).Query(ctx, mapFieldSchemaColumns, "map_field_schema",
			`AND asset_type_code = $2 AND deleted_at IS NULL ORDER BY sort_order, field_code LIMIT $3`,
			assetTypeCode, MapFieldSchemaCeiling+1)
	}
	if err != nil {
		return nil, fmt.Errorf("map_field_schema: đọc danh sách: %w", err)
	}
	defer rows.Close()

	out := make([]domain.MapFieldSchema, 0, 16)
	for rows.Next() {
		m, err := scanMapFieldSchema(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("map_field_schema: đọc dòng: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("map_field_schema: duyệt kết quả: %w", err)
	}
	if len(out) > MapFieldSchemaCeiling {
		return nil, ErrTooManyMapFieldSchemas
	}
	return out, nil
}

// AssetTypeLive reports whether the code is a live (not soft-deleted) row of THIS commune's type
// catalogue, and holds a share lock on it until the transaction ends.
//
// FOR SHARE, so a concurrent soft delete of the type (which reads the row FOR UPDATE) waits for
// this create to commit or roll back instead of racing it. A DISABLED type is accepted: it still
// has assets that render its fields, and configuring a form before re-enabling a group is
// legitimate.
func (s *MapFieldSchemaStore) AssetTypeLive(ctx context.Context, tx *store.ScopedTx, code string) (bool, error) {
	const stmt = `SELECT 1 FROM loai_tai_nguyen_ban_do ` +
		`WHERE tenant_id = $1 AND ma = $2 AND deleted_at IS NULL FOR SHARE`

	var one int
	err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), code).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("map_field_schema: kiểm tra nhóm tài nguyên: %w", err)
	}
	return true, nil
}

// FieldCodeState reports whether (type, key) is held by a live row, and whether by a retired one —
// INCLUDING soft-deleted rows, which is its whole purpose. The unique key is the real guard; this
// turns the ordinary case into a sentence.
func (s *MapFieldSchemaStore) FieldCodeState(ctx context.Context, tx *store.ScopedTx,
	assetTypeCode, fieldCode string) (live, retired bool, err error) {
	const stmt = `SELECT count(*) FILTER (WHERE deleted_at IS NULL), ` +
		`count(*) FILTER (WHERE deleted_at IS NOT NULL) FROM map_field_schema ` +
		`WHERE tenant_id = $1 AND asset_type_code = $2 AND field_code = $3`

	var nLive, nRetired int
	if err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()),
		assetTypeCode, fieldCode).Scan(&nLive, &nRetired); err != nil {
		return false, false, fmt.Errorf("map_field_schema: kiểm tra mã trường: %w", err)
	}
	return nLive > 0, nRetired > 0, nil
}

// CountLive counts the commune's live rows, the same set List returns, for the ceiling check.
func (s *MapFieldSchemaStore) CountLive(ctx context.Context, tx *store.ScopedTx) (int, error) {
	const stmt = `SELECT count(*) FROM map_field_schema WHERE tenant_id = $1 AND deleted_at IS NULL`

	var n int
	if err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID())).Scan(&n); err != nil {
		return 0, fmt.Errorf("map_field_schema: đếm dòng đang sống: %w", err)
	}
	return n, nil
}

const insertMapFieldSchema = `INSERT INTO map_field_schema
	(tenant_id, id, asset_type_code, field_code, label, value_type, options, is_required, sort_order, is_active)
	VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, true)`

// Insert adds one row, active. A field created disabled is two requests, and the second one is the
// one that leaves a trail saying somebody turned it off.
func (s *MapFieldSchemaStore) Insert(ctx context.Context, tx *store.ScopedTx, m domain.MapFieldSchema) error {
	opts, err := encodeOptions(m.Options)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, insertMapFieldSchema, string(tx.TenantID()), m.ID, m.AssetTypeCode,
		m.FieldCode, m.Label, m.ValueType, opts, m.IsRequired, m.SortOrder); err != nil {
		return fmt.Errorf("map_field_schema: chèn: %w", err)
	}
	return nil
}

// ByIDForUpdate reads one LIVE row and locks it until the transaction ends: every write is
// read-decide-write, and without the lock two editors both decide against the old state and the
// second silently overwrites the first — including one removing an option the other just added.
func (s *MapFieldSchemaStore) ByIDForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (domain.MapFieldSchema, error) {
	const stmt = `SELECT ` + mapFieldSchemaColumns + ` FROM map_field_schema ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

	m, err := scanMapFieldSchema(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.MapFieldSchema{}, ErrMapFieldSchemaNotFound
	}
	if err != nil {
		return domain.MapFieldSchema{}, fmt.Errorf("map_field_schema: đọc dòng để sửa: %w", err)
	}
	return m, nil
}

// updateMapFieldSchema — the five fields a commune may change, and nothing else (point 2 above).
const updateMapFieldSchema = `UPDATE map_field_schema ` +
	`SET label = $3, options = $4::jsonb, is_required = $5, sort_order = $6, is_active = $7, ` +
	`updated_at = now() WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

func (s *MapFieldSchemaStore) Update(ctx context.Context, tx *store.ScopedTx, m domain.MapFieldSchema) error {
	opts, err := encodeOptions(m.Options)
	if err != nil {
		return err
	}
	res, err := tx.Exec(ctx, updateMapFieldSchema, string(tx.TenantID()), m.ID, m.Label, opts,
		m.IsRequired, m.SortOrder, m.IsActive)
	if err != nil {
		return fmt.Errorf("map_field_schema: cập nhật: %w", err)
	}
	return oneRowOrNotFound(res, "cập nhật")
}

// softDeleteMapFieldSchema writes all three columns of rule 7 invariant 1 in one statement.
// `AND deleted_at IS NULL` makes a second delete a 404 rather than a rewrite of who deleted it.
const softDeleteMapFieldSchema = `UPDATE map_field_schema ` +
	`SET deleted_at = now(), deleted_by = $3, delete_reason = $4, updated_at = now() ` +
	`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

// SoftDelete retires one row. There is no hard delete in this package; the trigger refuses one.
func (s *MapFieldSchemaStore) SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string) error {
	res, err := tx.Exec(ctx, softDeleteMapFieldSchema, string(tx.TenantID()), id, by, reason)
	if err != nil {
		return fmt.Errorf("map_field_schema: xoá mềm: %w", err)
	}
	return oneRowOrNotFound(res, "xoá mềm")
}

// oneRowOrNotFound: an UPDATE touching zero rows is not an error to PostgreSQL, only to us — see
// doiMotDong, whose error this cannot reuse because it names the other catalogue's refusal.
func oneRowOrNotFound(res sql.Result, op string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("map_field_schema: %s: đọc số dòng: %w", op, err)
	}
	if n == 0 {
		return ErrMapFieldSchemaNotFound
	}
	return nil
}
