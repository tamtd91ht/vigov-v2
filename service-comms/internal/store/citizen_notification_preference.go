package store

// Citizen notification preferences — migrations/0027_citizen_notification_preferences.sql.
//
// THE COMMUNE IS $1 IN EVERY STATEMENT, from Scoped / tx.TenantID(); the citizen is $2, passed by the
// caller from the SESSION (the citizen routes) or from the event (the send path) — never from a request
// body (rule 4, invariant 2).
//
// NO ROW = ENABLED, and this file is where that reading lives for the whole service: a read finding no
// row returns domain.DefaultCitizenNotificationPreference(). There is deliberately no JOIN here — a JOIN
// filtered on `zalo_enabled = true` drops every citizen with no row and silences them (0027, question 4).
//
// NOTHING HERE DELETES (the trigger refuses it: an absent row means ON, so a delete would turn an
// opt-out back on without the citizen's act) and NOTHING HERE OPENS A TRANSACTION for the write — the
// use case does, and writes the audit entry inside it.
//
// The citizen id is a handle on a person: never logged, never in an error message.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// CitizenNotificationPreferenceStore reads and writes citizen_notification_preferences through Scoped /
// ScopedTx only (rule 1, invariant 5).
type CitizenNotificationPreferenceStore struct {
	db *store.DB
}

func NewCitizenNotificationPreferenceStore(db *store.DB) *CitizenNotificationPreferenceStore {
	return &CitizenNotificationPreferenceStore{db: db}
}

// Get reads one citizen's switch in the commune of ctx. No row → enabled (see the file comment).
func (s *CitizenNotificationPreferenceStore) Get(ctx context.Context, citizenID string) (
	domain.CitizenNotificationPreference, error) {

	// Scoped: db.For(ctx).Query binds the commune as $1 (`WHERE tenant_id = $1`).
	rows, err := s.db.For(ctx).Query(ctx, "zalo_enabled", "citizen_notification_preferences",
		`AND citizen_id = $2`, citizenID)
	if err != nil {
		return domain.CitizenNotificationPreference{}, fmt.Errorf("citizen_notification_preference: đọc: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.CitizenNotificationPreference{}, fmt.Errorf("citizen_notification_preference: duyệt: %w", err)
		}
		return domain.DefaultCitizenNotificationPreference(), nil
	}
	p := domain.CitizenNotificationPreference{Stored: true}
	if err := rows.Scan(&p.ZaloEnabled); err != nil {
		return domain.CitizenNotificationPreference{}, fmt.Errorf("citizen_notification_preference: đọc dòng: %w", err)
	}
	if err := rows.Err(); err != nil {
		return domain.CitizenNotificationPreference{}, fmt.Errorf("citizen_notification_preference: duyệt: %w", err)
	}
	return p, nil
}

// ForUpdate reads one citizen's switch inside the transaction and locks the row when it exists, so two
// taps on the switch cannot both record the same "before" in the trail.
func (s *CitizenNotificationPreferenceStore) ForUpdate(ctx context.Context, tx *store.ScopedTx, citizenID string) (
	domain.CitizenNotificationPreference, error) {

	const stmt = `SELECT zalo_enabled FROM citizen_notification_preferences ` +
		`WHERE tenant_id = $1 AND citizen_id = $2 FOR UPDATE`

	p := domain.CitizenNotificationPreference{Stored: true}
	err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), citizenID).Scan(&p.ZaloEnabled)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DefaultCitizenNotificationPreference(), nil
	}
	if err != nil {
		return domain.CitizenNotificationPreference{}, fmt.Errorf("citizen_notification_preference: đọc để sửa: %w", err)
	}
	return p, nil
}

// upsertPreference — the key never moves (the trigger refuses it too); only the value and updated_at.
// ON CONFLICT covers two first taps racing past ForUpdate (which locks nothing while no row exists).
const upsertPreference = `INSERT INTO citizen_notification_preferences (tenant_id, citizen_id, zalo_enabled)
	VALUES ($1, $2, $3)
	ON CONFLICT (tenant_id, citizen_id) DO UPDATE SET zalo_enabled = EXCLUDED.zalo_enabled, updated_at = now()`

// Set records the citizen's choice. The caller opened the transaction and writes the audit entry in it.
func (s *CitizenNotificationPreferenceStore) Set(ctx context.Context, tx *store.ScopedTx, citizenID string,
	zaloEnabled bool) error {

	// Scoped: tenant_id is $1 from tx.TenantID().
	if _, err := tx.Exec(ctx, upsertPreference, string(tx.TenantID()), citizenID, zaloEnabled); err != nil {
		return fmt.Errorf("citizen_notification_preference: ghi: %w", err)
	}
	return nil
}
