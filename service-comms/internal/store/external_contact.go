package store

// The commune's external contacts — migrations/0014_external_contacts.sql.
//
// FOUR THINGS HOLD ACROSS EVERY METHOD, the same four as map_field_schema.go:
//
//  1. THE COMMUNE IS $1 IN EVERY STATEMENT, from Scoped / tx.TenantID(), never a parameter.
//  2. `tenant_id`, `id`, `created_at` AND `created_by` APPEAR IN NO UPDATE. The trigger refuses a change
//     too; their absence here keeps that floor unreachable from this service.
//  3. EVERY READ EXCLUDES SOFT-DELETED ROWS (rule 7, invariant 2) — the staff list and the public read
//     alike. There is no publish flag: a live row IS a public row (0014, VENDOR CHOICES).
//  4. NOTHING HERE OPENS A TRANSACTION. The use case opens it and writes the audit entry inside.
//
// `phone` AND `address` MAY BE PERSONAL DATA (0014, PERSONAL DATA): nothing here logs a row or puts a
// value into an error message.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// ExternalContactStore reads and writes external_contacts. Built from *store.DB and reaching the
// database only through Scoped / ScopedTx (rule 1, invariant 5).
type ExternalContactStore struct {
	db *store.DB
}

func NewExternalContactStore(db *store.DB) *ExternalContactStore {
	return &ExternalContactStore{db: db}
}

// ExternalContactCeiling bounds one commune's live rows. The list is returned WHOLE — to staff and to
// residents — so the bound cannot be a `limit`, and the create refuses past it so the commune cannot
// break its own list.
//
// 500 is ten times the "few dozen per commune" migration 0014 expects; past it the content is an
// import run twice or a loop, not a directory.
const ExternalContactCeiling = 500

var (
	// ErrExternalContactNotFound — no LIVE row with that id in THIS commune. 404, deliberately the same
	// answer for "another commune's row", "deleted" and "no such row".
	ErrExternalContactNotFound = errors.New("external_contact: không tồn tại trong xã này")

	// ErrExternalContactsFull — the commune is at ExternalContactCeiling.
	ErrExternalContactsFull = errors.New("external_contact: đã đạt số liên hệ tối đa")

	// ErrTooManyExternalContacts — the READ found more than the ceiling. Refused, never truncated: a
	// silently short list is a number a resident cannot find.
	ErrTooManyExternalContacts = errors.New("external_contact: vượt trần")
)

// externalContactColumns IS READ BY POSITION in scanExternalContact. Four adjacent TEXT columns: a swap
// produces no error, only a phone number printed as a heading.
const externalContactColumns = `id, name, category, phone, address, display_order`

func scanExternalContact(scan func(...any) error) (domain.ExternalContact, error) {
	var (
		c       domain.ExternalContact
		address sql.NullString
		order   sql.NullInt64
	)
	if err := scan(&c.ID, &c.Name, &c.Category, &c.Phone, &address, &order); err != nil {
		return domain.ExternalContact{}, err
	}
	c.Address = address.String
	if order.Valid {
		n := int(order.Int64)
		c.DisplayOrder = &n
	}
	return c, nil
}

// List reads the commune's live contacts in display order, NULLs last, id as the tie-break — the
// order the partial index `external_contacts_list` serves as is (btree ASC puts NULLs last).
//
// ONE METHOD FOR THE STAFF LIST AND THE PUBLIC READ, on purpose: with no publish flag the two read the
// same rows, and two queries for one predicate are two predicates that can drift.
func (s *ExternalContactStore) List(ctx context.Context) ([]domain.ExternalContact, error) {
	// LIMIT is the ceiling PLUS ONE, so "too many" is detectable rather than indistinguishable from a
	// complete list of exactly that size.
	rows, err := s.db.For(ctx).Query(ctx, externalContactColumns, "external_contacts",
		`AND deleted_at IS NULL ORDER BY display_order, id LIMIT $2`, ExternalContactCeiling+1)
	if err != nil {
		return nil, fmt.Errorf("external_contact: đọc danh sách: %w", err)
	}
	defer rows.Close()

	out := make([]domain.ExternalContact, 0, 32)
	for rows.Next() {
		c, err := scanExternalContact(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("external_contact: đọc dòng: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("external_contact: duyệt kết quả: %w", err)
	}
	if len(out) > ExternalContactCeiling {
		return nil, ErrTooManyExternalContacts
	}
	return out, nil
}

// CountLive counts the commune's live rows — the set List returns — for the ceiling check.
func (s *ExternalContactStore) CountLive(ctx context.Context, tx *store.ScopedTx) (int, error) {
	const stmt = `SELECT count(*) FROM external_contacts WHERE tenant_id = $1 AND deleted_at IS NULL`

	var n int
	if err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID())).Scan(&n); err != nil {
		return 0, fmt.Errorf("external_contact: đếm dòng đang sống: %w", err)
	}
	return n, nil
}

const insertExternalContact = `INSERT INTO external_contacts
	(tenant_id, id, name, category, phone, address, display_order, created_by, updated_by)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)`

// Insert adds one row. `by` is the actor's BUSINESS CODE (rule 6, invariant 8) — the same value the
// audit entry carries, so the column and the trail name the same person.
func (s *ExternalContactStore) Insert(ctx context.Context, tx *store.ScopedTx, c domain.ExternalContact, by string) error {
	if _, err := tx.Exec(ctx, insertExternalContact, string(tx.TenantID()), c.ID, c.Name, c.Category,
		c.Phone, nullText(c.Address), nullOrder(c.DisplayOrder), by); err != nil {
		return fmt.Errorf("external_contact: chèn: %w", err)
	}
	return nil
}

// ByIDForUpdate reads one LIVE row and locks it until the transaction ends: every write is
// read-decide-write, and without the lock two editors both decide against the old state and the audit
// entry of the second records a "before" that was never the row's state.
func (s *ExternalContactStore) ByIDForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (domain.ExternalContact, error) {
	const stmt = `SELECT ` + externalContactColumns + ` FROM external_contacts ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

	c, err := scanExternalContact(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ExternalContact{}, ErrExternalContactNotFound
	}
	if err != nil {
		return domain.ExternalContact{}, fmt.Errorf("external_contact: đọc dòng để sửa: %w", err)
	}
	return c, nil
}

// updateExternalContact — the five fields a commune may change, and who changed them (point 2 above).
const updateExternalContact = `UPDATE external_contacts ` +
	`SET name = $3, category = $4, phone = $5, address = $6, display_order = $7, ` +
	`updated_at = now(), updated_by = $8 WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

func (s *ExternalContactStore) Update(ctx context.Context, tx *store.ScopedTx, c domain.ExternalContact, by string) error {
	res, err := tx.Exec(ctx, updateExternalContact, string(tx.TenantID()), c.ID, c.Name, c.Category,
		c.Phone, nullText(c.Address), nullOrder(c.DisplayOrder), by)
	if err != nil {
		return fmt.Errorf("external_contact: cập nhật: %w", err)
	}
	return externalContactOneRow(res, "cập nhật")
}

// softDeleteExternalContact writes all three columns of rule 7 invariant 1 in one statement.
// `AND deleted_at IS NULL` makes a second delete a 404 rather than a rewrite of who deleted it — and
// the trigger refuses any edit of a deleted row underneath.
const softDeleteExternalContact = `UPDATE external_contacts ` +
	`SET deleted_at = now(), deleted_by = $3, delete_reason = $4, updated_at = now(), updated_by = $3 ` +
	`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

// SoftDelete retires one row. There is no hard delete in this package; the trigger refuses one.
func (s *ExternalContactStore) SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string) error {
	res, err := tx.Exec(ctx, softDeleteExternalContact, string(tx.TenantID()), id, by, reason)
	if err != nil {
		return fmt.Errorf("external_contact: xoá mềm: %w", err)
	}
	return externalContactOneRow(res, "xoá mềm")
}

func externalContactOneRow(res sql.Result, op string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("external_contact: %s: đọc số dòng: %w", op, err)
	}
	if n == 0 {
		return ErrExternalContactNotFound
	}
	return nil
}

// nullText maps "" to NULL: 0014 refuses an all-blank address as a second spelling of NULL.
func nullText(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullOrder(n *int) any {
	if n == nil {
		return nil
	}
	return int64(*n)
}
