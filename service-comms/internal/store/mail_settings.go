package store

// The commune's mail server — migrations/0008_mail_settings.sql, `mail_settings`.
//
// THREE THINGS HOLD ACROSS EVERY METHOD:
//
//  1. THE COMMUNE IS $1 IN EVERY STATEMENT, from Scoped / tx.TenantID(), never a parameter.
//  2. THE READ THE SCREEN USES CANNOT RETURN THE PASSWORD: Get selects `password_sealed IS NOT
//     NULL`, a boolean, and never the column itself. Only the two reads that exist to hand the
//     sealed bytes to the use case select them — ForUpdate (to keep them on save) and Account (to
//     open them for a test message).
//  3. NOTHING HERE OPENS A TRANSACTION. The use case opens it and writes the audit entry inside.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// mailSettingsColumns IS READ BY POSITION in scanMailSettings. Four adjacent TEXT columns: a swap
// produces no error, only wrong data.
const mailSettingsColumns = `host, port, security, username, from_address, from_name, is_enabled`

// mailLastTestColumns — 0019's four columns, read by position in scanLastTest.
const mailLastTestColumns = `last_test_at, last_test_to_masked, last_test_ok, last_test_error_class`

const selectMailSettingsForUpdate = `SELECT ` + mailSettingsColumns + `, password_sealed, ` + mailLastTestColumns +
	` FROM mail_settings WHERE tenant_id = $1 FOR UPDATE`

// upsertMailSettings — one row per commune. `tenant_id` appears in no SET list: the trigger refuses
// a change too, and its absence here keeps that floor unreachable from this service.
const upsertMailSettings = `INSERT INTO mail_settings
	(tenant_id, host, port, security, username, from_address, from_name, is_enabled, password_sealed, updated_by)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	ON CONFLICT (tenant_id) DO UPDATE SET
	host = EXCLUDED.host, port = EXCLUDED.port, security = EXCLUDED.security,
	username = EXCLUDED.username, from_address = EXCLUDED.from_address, from_name = EXCLUDED.from_name,
	is_enabled = EXCLUDED.is_enabled, password_sealed = EXCLUDED.password_sealed,
	updated_by = EXCLUDED.updated_by, updated_at = now()`

// ErrMailSettingsNotFound — the commune has not configured a mail server.
var ErrMailSettingsNotFound = errors.New("mail_settings: xã chưa khai máy chủ thư")

// MailSettingsStore reads and writes mail_settings. Built from *store.DB and reaching the database
// only through Scoped / ScopedTx (rule 1, invariant 5).
type MailSettingsStore struct {
	db *store.DB
}

func NewMailSettingsStore(db *store.DB) *MailSettingsStore {
	return &MailSettingsStore{db: db}
}

func scanMailSettings(scan func(...any) error, extra ...any) (domain.MailSettings, error) {
	var m domain.MailSettings
	dest := append([]any{&m.Host, &m.Port, &m.Security, &m.Username, &m.FromAddress, &m.FromName,
		&m.IsEnabled}, extra...)
	if err := scan(dest...); err != nil {
		return domain.MailSettings{}, err
	}
	return m, nil
}

// Get reads the commune's settings for the screen, or ErrMailSettingsNotFound. PasswordSet is
// computed in SQL; the sealed bytes never leave the database on this path (point 2 above).
func (s *MailSettingsStore) Get(ctx context.Context) (domain.MailSettings, error) {
	// Scoped: Query adds `WHERE tenant_id = $1` from the context.
	rows, err := s.db.For(ctx).Query(ctx, mailSettingsColumns+`, password_sealed IS NOT NULL, `+mailLastTestColumns,
		"mail_settings", "")
	if err != nil {
		return domain.MailSettings{}, fmt.Errorf("mail_settings: read: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.MailSettings{}, fmt.Errorf("mail_settings: read: %w", err)
		}
		return domain.MailSettings{}, ErrMailSettingsNotFound
	}
	var set bool
	var t lastTestScan
	m, err := scanMailSettings(rows.Scan, append([]any{&set}, t.dest()...)...)
	if err != nil {
		return domain.MailSettings{}, fmt.Errorf("mail_settings: scan: %w", err)
	}
	m.PasswordSet = set
	m.LastTest = t.result()
	return m, nil
}

// lastTestScan holds 0019's four nullable columns while a row is scanned.
type lastTestScan struct {
	at    sql.NullTime
	to    sql.NullString
	ok    sql.NullBool
	class sql.NullString
}

func (t *lastTestScan) dest() []any { return []any{&t.at, &t.to, &t.ok, &t.class} }

// result is nil for "never tested" — 0019's CHECK makes the three NOT-NULL-together columns agree.
func (t *lastTestScan) result() *domain.MailTestResult {
	if !t.at.Valid {
		return nil
	}
	return &domain.MailTestResult{At: t.at.Time, ToMasked: t.to.String, OK: t.ok.Bool, ErrorClass: t.class.String}
}

// Account reads the settings AND the sealed password, for the one use that needs to open it: a
// test message. ErrMailSettingsNotFound when there is no row.
func (s *MailSettingsStore) Account(ctx context.Context) (domain.MailSettings, []byte, error) {
	rows, err := s.db.For(ctx).Query(ctx, mailSettingsColumns+`, password_sealed`, "mail_settings", "")
	if err != nil {
		return domain.MailSettings{}, nil, fmt.Errorf("mail_settings: read account: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.MailSettings{}, nil, fmt.Errorf("mail_settings: read account: %w", err)
		}
		return domain.MailSettings{}, nil, ErrMailSettingsNotFound
	}
	var sealed []byte
	m, err := scanMailSettings(rows.Scan, &sealed)
	if err != nil {
		return domain.MailSettings{}, nil, fmt.Errorf("mail_settings: scan account: %w", err)
	}
	m.PasswordSet = len(sealed) > 0
	return m, sealed, nil
}

// ForUpdate reads the row and its sealed password and locks it until the transaction ends. found
// is false when the commune has no row yet. Without the lock two administrators saving at once both
// decide "destination unchanged, keep the password" against the old row, and the second overwrites
// the first's host with the old password still attached.
func (s *MailSettingsStore) ForUpdate(ctx context.Context, tx *store.ScopedTx) (
	m domain.MailSettings, sealed []byte, found bool, err error) {

	var t lastTestScan
	m, err = scanMailSettings(tx.Underlying().QueryRowContext(ctx, selectMailSettingsForUpdate,
		string(tx.TenantID())).Scan, append([]any{&sealed}, t.dest()...)...)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.MailSettings{}, nil, false, nil
	}
	if err != nil {
		return domain.MailSettings{}, nil, false, fmt.Errorf("mail_settings: read for update: %w", err)
	}
	m.PasswordSet = len(sealed) > 0
	m.LastTest = t.result()
	return m, sealed, true, nil
}

// Upsert writes the whole row. sealed is the FINAL sealed password — newly sealed, or the bytes
// ForUpdate returned — never empty (the column is NOT NULL and the use case refuses first). by is
// the actor's BUSINESS CODE (rule 6, invariant 8).
func (s *MailSettingsStore) Upsert(ctx context.Context, tx *store.ScopedTx, m domain.MailSettings,
	sealed []byte, by string) error {

	if len(sealed) == 0 {
		return errors.New("mail_settings: upsert without a sealed password")
	}
	if _, err := tx.Exec(ctx, upsertMailSettings, string(tx.TenantID()), m.Host, m.Port, m.Security,
		m.Username, m.FromAddress, m.FromName, m.IsEnabled, sealed, by); err != nil {
		return fmt.Errorf("mail_settings: upsert: %w", err)
	}
	return nil
}

// recordMailTest writes 0019's four columns and NOTHING ELSE — not updated_at, not updated_by: recording
// a test is not a change of the configuration, and bumping updated_by would show the tester as the last
// person to save the server settings (0019 "OWED BY GO").
const recordMailTest = `UPDATE mail_settings SET last_test_at = $2, last_test_to_masked = $3, last_test_ok = $4,
	last_test_error_class = $5 WHERE tenant_id = $1`

// RecordTest stores the last test message's result. errorClass "" when ok. ErrMailSettingsNotFound when
// the commune has no row (the use case read it a moment before; the guard refuses DELETE).
func (s *MailSettingsStore) RecordTest(ctx context.Context, tx *store.ScopedTx, r domain.MailTestResult) error {
	var class any
	if r.ErrorClass != "" {
		class = r.ErrorClass
	}
	res, err := tx.Exec(ctx, recordMailTest, string(tx.TenantID()), r.At.UTC(), r.ToMasked, r.OK, class)
	if err != nil {
		return fmt.Errorf("mail_settings: record test: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mail_settings: record test: %w", err)
	}
	if n == 0 {
		return ErrMailSettingsNotFound
	}
	return nil
}

// clearMailTest empties the four columns — all NULL is what 0019's CHECKs allow for "never tested".
const clearMailTest = `UPDATE mail_settings SET last_test_at = NULL, last_test_to_masked = NULL,
	last_test_ok = NULL, last_test_error_class = NULL WHERE tenant_id = $1`

// ClearTest forgets the last test result: a save pointed the configuration at another destination.
func (s *MailSettingsStore) ClearTest(ctx context.Context, tx *store.ScopedTx) error {
	if _, err := tx.Exec(ctx, clearMailTest, string(tx.TenantID())); err != nil {
		return fmt.Errorf("mail_settings: clear test: %w", err)
	}
	return nil
}
