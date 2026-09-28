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

const selectMailSettingsForUpdate = `SELECT ` + mailSettingsColumns + `, password_sealed ` +
	`FROM mail_settings WHERE tenant_id = $1 FOR UPDATE`

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
	rows, err := s.db.For(ctx).Query(ctx, mailSettingsColumns+`, password_sealed IS NOT NULL`,
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
	m, err := scanMailSettings(rows.Scan, &set)
	if err != nil {
		return domain.MailSettings{}, fmt.Errorf("mail_settings: scan: %w", err)
	}
	m.PasswordSet = set
	return m, nil
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

	m, err = scanMailSettings(tx.Underlying().QueryRowContext(ctx, selectMailSettingsForUpdate,
		string(tx.TenantID())).Scan, &sealed)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.MailSettings{}, nil, false, nil
	}
	if err != nil {
		return domain.MailSettings{}, nil, false, fmt.Errorf("mail_settings: read for update: %w", err)
	}
	m.PasswordSet = len(sealed) > 0
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
