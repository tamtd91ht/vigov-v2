package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

// CitizenLetterTypeStore reads and writes `citizen_letter_type` (migration 0010) — the commune's labels
// for C4's four letter types. Same discipline as LoaiVanBanStore: the commune is $1 in every statement,
// every write takes the caller's transaction, every read but CodeTaken excludes soft-deleted rows.
//
// NO CEILING CONSTANT: the unique key and the code CHECK admit at most four rows per commune.
type CitizenLetterTypeStore struct {
	db *store.DB
}

func NewCitizenLetterTypeStore(db *store.DB) *CitizenLetterTypeStore {
	return &CitizenLetterTypeStore{db: db}
}

// citizenLetterTypeColumns is read BY POSITION (scanCitizenLetterType).
const citizenLetterTypeColumns = `id, code, label, sort_order, active, color`

func scanCitizenLetterType(scan func(...any) error) (domain.CitizenLetterTypeEntry, error) {
	var (
		e     domain.CitizenLetterTypeEntry
		code  string
		color sql.NullString
	)
	err := scan(&e.ID, &code, &e.Label, &e.Order, &e.Active, &color)
	e.Code, e.Color = domain.LetterType(code), color.String
	return e, err
}

// ListEntries reads the commune's live rows, in the commune's order (`sort_order`, then `code` so two rows
// sharing a rank never swap between two reads). Rows switched off ARE returned, carrying `active`.
func (s *CitizenLetterTypeStore) ListEntries(ctx context.Context) ([]domain.CitizenLetterTypeEntry, error) {
	rows, err := s.db.For(ctx).Query(ctx, citizenLetterTypeColumns, "citizen_letter_type",
		`AND deleted_at IS NULL ORDER BY sort_order, code`)
	if err != nil {
		return nil, fmt.Errorf("citizen_letter_type: đọc danh mục: %w", err)
	}
	defer rows.Close()
	out := make([]domain.CitizenLetterTypeEntry, 0, len(domain.LetterTypes))
	for rows.Next() {
		e, err := scanCitizenLetterType(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("citizen_letter_type: đọc dòng: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("citizen_letter_type: duyệt kết quả: %w", err)
	}
	return out, nil
}

// ByIDForUpdate reads one live row and locks it until the transaction ends (read-decide-write).
func (s *CitizenLetterTypeStore) ByIDForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (domain.CitizenLetterTypeEntry, error) {
	const stmt = `SELECT ` + citizenLetterTypeColumns + ` FROM citizen_letter_type ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`
	e, err := scanCitizenLetterType(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CitizenLetterTypeEntry{}, ErrDanhMucKhongTonTai
	}
	if err != nil {
		return domain.CitizenLetterTypeEntry{}, fmt.Errorf("citizen_letter_type: đọc dòng để sửa: %w", err)
	}
	return e, nil
}

// CodeTaken reports whether the code already has a row in this commune — SOFT-DELETED ROWS INCLUDED,
// as `UNIQUE (tenant_id, code)` counts them (rule 7, invariant 3). The key is the real guard; this is
// the readable sentence.
func (s *CitizenLetterTypeStore) CodeTaken(ctx context.Context, tx *store.ScopedTx, code domain.LetterType) (bool, error) {
	const stmt = `SELECT count(*) FROM citizen_letter_type WHERE tenant_id = $1 AND code = $2`
	var n int
	if err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), string(code)).Scan(&n); err != nil {
		return false, fmt.Errorf("citizen_letter_type: kiểm tra mã trùng: %w", err)
	}
	return n > 0, nil
}

const insertCitizenLetterType = `INSERT INTO citizen_letter_type
	(tenant_id, id, code, label, sort_order, active, color)
	VALUES ($1, $2, $3, $4, $5, $6, $7)`

// AddEntry adds one row.
func (s *CitizenLetterTypeStore) AddEntry(ctx context.Context, tx *store.ScopedTx, e domain.CitizenLetterTypeEntry) error {
	if _, err := tx.Exec(ctx, insertCitizenLetterType, string(tx.TenantID()),
		e.ID, string(e.Code), e.Label, e.Order, e.Active, colorArg(e.Color)); err != nil {
		return fmt.Errorf("citizen_letter_type: chèn: %w", err)
	}
	return nil
}

// updateCitizenLetterType — `code` APPEARS NOWHERE: an issued code is never renumbered (rule 7,
// invariant 3).
const updateCitizenLetterType = `UPDATE citizen_letter_type ` +
	`SET label = $3, sort_order = $4, active = $5, color = $6, updated_at = now() ` +
	`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

// RewriteEntry writes the four fields a commune may change.
func (s *CitizenLetterTypeStore) RewriteEntry(ctx context.Context, tx *store.ScopedTx, e domain.CitizenLetterTypeEntry) error {
	res, err := tx.Exec(ctx, updateCitizenLetterType, string(tx.TenantID()),
		e.ID, e.Label, e.Order, e.Active, colorArg(e.Color))
	if err != nil {
		return fmt.Errorf("citizen_letter_type: cập nhật: %w", err)
	}
	return doiMotDong(res, "cập nhật")
}

// softDeleteCitizenLetterType writes the three columns of rule 7, invariant 1 in one statement.
// `AND deleted_at IS NULL` makes a second delete a 404 rather than a rewrite of the first one's reason.
const softDeleteCitizenLetterType = `UPDATE citizen_letter_type ` +
	`SET deleted_at = now(), deleted_by = $3, delete_reason = $4, updated_at = now() ` +
	`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

// SoftDelete soft deletes one row. There is no hard delete; the trigger refuses one anyway.
func (s *CitizenLetterTypeStore) SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string) error {
	res, err := tx.Exec(ctx, softDeleteCitizenLetterType, string(tx.TenantID()), id, by, reason)
	if err != nil {
		return fmt.Errorf("citizen_letter_type: xoá mềm: %w", err)
	}
	return doiMotDong(res, "xoá mềm")
}
