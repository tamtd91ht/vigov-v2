package store

// THE HEADER-BELL INBOX — `staff_notification` (migration 0010). SQL, and nothing else.
//
// THREE THINGS HOLD IN EVERY STATEMENT HERE:
//
//  1. THE COMMUNE IS $1, from the context or the transaction (rule 1, invariants 4 and 5).
//  2. EVERY READ AND EVERY UPDATE CARRIES `recipient_code = <the caller's code>`. The inbox is ONE
//     person's: no method here reads or marks another person's notice, and none takes a code from
//     anywhere but its caller — which, on the HTTP path, is the session's principal and nothing else.
//  3. `deleted_at IS NULL` on every read and update (rule 7, invariant 2). Nothing writes the
//     soft-delete columns yet; the predicate is there so the day something does, no read changes
//     meaning.
//
// NO audit.Write HERE — same answer as store/thong_bao_noi_bo.go: internal/app opens the transaction
// and writes the entry inside it; the actor and the verb do not exist at the SQL layer.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// StaffNotificationStore is the only path to `staff_notification`. Built from *store.DB and reaching
// the database only through Scoped / ScopedTx (rule 1, invariant 5).
type StaffNotificationStore struct {
	db *store.DB
}

func NewStaffNotificationStore(db *store.DB) *StaffNotificationStore {
	return &StaffNotificationStore{db: db}
}

// ErrStaffNotificationNotFound says the id addressed no live notice OF THIS PERSON IN THIS COMMUNE.
// "No such notice", "another person's notice" and "another commune's notice" are deliberately one
// answer: telling them apart would confirm what somebody else's bell holds (rule 4, forbidden #2).
var ErrStaffNotificationNotFound = errors.New("staff_notification: không có bản ghi")

// staffNotificationColumns IS READ BY POSITION in scanStaffNotification.
const staffNotificationColumns = `id, recipient_code, idempotency_key, kind, title, body, link, read_at, created_at`

// SortStaffNotifications is the one sort the bell offers: newest first. `created_at` is NOT NULL, which
// a cursor column must be (core/page).
var SortStaffNotifications = page.NewAllowlist(page.Desc,
	page.Col("created_at", "created_at", page.KindTime),
)

var anchorStaffNotification = store.NewMoc[domain.StaffNotification](SortStaffNotifications,
	map[string]func(domain.StaffNotification) page.Key{
		"created_at": func(n domain.StaffNotification) page.Key { return page.TimeKey(n.CreatedAt) },
	})

// ListOwn reads one page of ONE person's inbox. recipientCode is bound to $2 — see (2) above.
func (s *StaffNotificationStore) ListOwn(ctx context.Context, recipientCode string, req page.Request) (
	page.Result[domain.StaffNotification], error) {

	return store.QueryPage(ctx, s.db.For(ctx), store.PageSpec{
		Columns: staffNotificationColumns,
		Table:   "staff_notification",
		Filter:  `AND deleted_at IS NULL AND recipient_code = $2`,
		Args:    []any{recipientCode},
	}, req, anchorStaffNotification, func(rows *sql.Rows) (domain.StaffNotification, string, error) {
		n, err := scanStaffNotification(rows.Scan)
		if err != nil {
			return domain.StaffNotification{}, "", err
		}
		return n, n.ID, nil
	})
}

// CountUnread is the badge: ONE person's unread, live notices. Served by the partial index
// `staff_notification_unread`.
func (s *StaffNotificationStore) CountUnread(ctx context.Context, recipientCode string) (int, error) {
	rows, err := s.db.For(ctx).Query(ctx, "count(*)", "staff_notification",
		`AND deleted_at IS NULL AND read_at IS NULL AND recipient_code = $2`, recipientCode)
	if err != nil {
		return 0, fmt.Errorf("staff_notification: đếm chưa đọc: %w", err)
	}
	defer rows.Close()
	n := 0
	if rows.Next() {
		if err := rows.Scan(&n); err != nil {
			return 0, fmt.Errorf("staff_notification: đọc số chưa đọc: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("staff_notification: duyệt số chưa đọc: %w", err)
	}
	return n, nil
}

// --- the write path ------------------------------------------------------------------------------

// AddDelivery writes one notice for each of its recipients, SKIPPING every (key, recipient) pair already
// present — `ON CONFLICT (tenant_id, idempotency_key, recipient_code) DO NOTHING`, the unique key of
// migration 0010. It returns how many rows were CREATED; the rest already had the key.
//
// The ids come from the caller, one per recipient in order, so a test can pin them. A pair that
// conflicts burns its id, which is harmless: an id is internal and never a business code.
//
// `read_at` and the soft-delete columns appear nowhere: a notice is not born read, nor deleted.
func (s *StaffNotificationStore) AddDelivery(ctx context.Context, tx *store.ScopedTx,
	n domain.NotificationDelivery, ids []string, at time.Time) (int, error) {

	if len(n.RecipientCodes) == 0 {
		return 0, nil
	}
	if len(ids) != len(n.RecipientCodes) {
		return 0, fmt.Errorf("staff_notification: %d id cho %d người nhận", len(ids), len(n.RecipientCodes))
	}
	var b strings.Builder
	b.WriteString(`INSERT INTO staff_notification ` +
		`(tenant_id, idempotency_key, kind, title, body, link, created_at, updated_at, id, recipient_code) VALUES `)
	// $1 commune · $2 key · $3 kind · $4 title · $5 body · $6 link · $7 time, then (id, code) pairs.
	args := make([]any, 0, 7+2*len(ids))
	args = append(args, string(tx.TenantID()), n.IdempotencyKey, n.Kind, n.Title, n.Body, n.Link, at.UTC())
	for i, code := range n.RecipientCodes {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("($1, $2, $3, $4, $5, $6, $7, $7, $" + strconv.Itoa(8+2*i) + ", $" + strconv.Itoa(9+2*i) + ")")
		args = append(args, ids[i], code)
	}
	b.WriteString(" ON CONFLICT (tenant_id, idempotency_key, recipient_code) DO NOTHING")

	res, err := tx.Exec(ctx, b.String(), args...)
	if err != nil {
		return 0, fmt.Errorf("staff_notification: chèn: %w", err)
	}
	created, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("staff_notification: đếm dòng chèn: %w", err)
	}
	return int(created), nil
}

// LockOwn reads one live notice OF THIS PERSON inside the transaction, FOR UPDATE, so two clicks on
// the same item cannot both decide it is unread and both file an audit entry.
func (s *StaffNotificationStore) LockOwn(ctx context.Context, tx *store.ScopedTx, id, recipientCode string) (
	domain.StaffNotification, error) {

	const stmt = `SELECT ` + staffNotificationColumns + ` FROM staff_notification ` +
		`WHERE tenant_id = $1 AND id = $2 AND recipient_code = $3 AND deleted_at IS NULL FOR UPDATE`
	n, err := scanStaffNotification(tx.Underlying().QueryRowContext(ctx, stmt,
		string(tx.TenantID()), id, recipientCode).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.StaffNotification{}, ErrStaffNotificationNotFound
	}
	if err != nil {
		return domain.StaffNotification{}, err
	}
	return n, nil
}

// MarkRead sets `read_at` on one unread notice of this person. `AND read_at IS NULL` makes a second
// mark a no-op rather than a moved timestamp (the trigger refuses that too).
func (s *StaffNotificationStore) MarkRead(ctx context.Context, tx *store.ScopedTx, id, recipientCode string,
	at time.Time) error {

	const stmt = `UPDATE staff_notification SET read_at = $4, updated_at = $4 ` +
		`WHERE tenant_id = $1 AND id = $2 AND recipient_code = $3 AND read_at IS NULL AND deleted_at IS NULL`
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, recipientCode, at.UTC())
	if err != nil {
		return fmt.Errorf("staff_notification: đánh dấu đã đọc: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("staff_notification: đếm dòng đánh dấu: %w", err)
	}
	if n == 0 {
		return ErrStaffNotificationNotFound
	}
	return nil
}

// MarkAllRead sets `read_at` on EVERY unread live notice of this person — the panel's `Đọc hết`.
// Returns how many rows moved.
func (s *StaffNotificationStore) MarkAllRead(ctx context.Context, tx *store.ScopedTx, recipientCode string,
	at time.Time) (int, error) {

	const stmt = `UPDATE staff_notification SET read_at = $3, updated_at = $3 ` +
		`WHERE tenant_id = $1 AND recipient_code = $2 AND read_at IS NULL AND deleted_at IS NULL`
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), recipientCode, at.UTC())
	if err != nil {
		return 0, fmt.Errorf("staff_notification: đọc hết: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("staff_notification: đếm dòng đọc hết: %w", err)
	}
	return int(n), nil
}

// scanStaffNotification reads one row of staffNotificationColumns, positionally.
func scanStaffNotification(scan func(...any) error) (domain.StaffNotification, error) {
	var (
		n    domain.StaffNotification
		read sql.NullTime
	)
	if err := scan(&n.ID, &n.RecipientCode, &n.IdempotencyKey, &n.Kind, &n.Title, &n.Body, &n.Link,
		&read, &n.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.StaffNotification{}, err
		}
		return domain.StaffNotification{}, fmt.Errorf("staff_notification: đọc dòng: %w", err)
	}
	n.ReadAt = read.Time
	return n, nil
}
