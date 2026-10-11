package store

// The staff-recorded rating (migration 0042; owner rule 11/10/2026, bug sheet row 60) — SQL, and nothing
// else. The commune is $1 in every statement; nothing here opens a transaction (the use case does, and
// writes the audit entry inside it); the table is append-only, so there is no UPDATE and no soft delete.
//
// ⚠ NOTHING HERE READS OR WRITES `phieu_phan_anh.diem_hai_long`. That column is the citizen's own verdict
// and every satisfaction figure reads it; keeping this record out of it is the whole design (0042 header).

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// PetitionStaffRatingStore reads and appends `petition_staff_recorded_rating`.
type PetitionStaffRatingStore struct {
	db *store.DB
}

func NewPetitionStaffRatingStore(db *store.DB) *PetitionStaffRatingStore {
	return &PetitionStaffRatingStore{db: db}
}

const staffRatingColumns = "id, petition_id, stars, channel, note, source, recorded_by, recorded_at"

// ForPetitionTx reads the staff record of petition `petitionID` (internal id) inside the caller's
// transaction — the use case's "already recorded?" check, made while it holds the petition's row lock, so
// two officers recording at once serialise on that lock (the UNIQUE key is the floor under it).
func (s *PetitionStaffRatingStore) ForPetitionTx(ctx context.Context, tx *store.ScopedTx, petitionID string) (
	domain.StaffRecordedRating, bool, error) {

	// ScopedTx.Query prefixes `WHERE tenant_id = $1`, bound from the transaction's commune.
	rows, err := tx.Query(ctx, staffRatingColumns, "petition_staff_recorded_rating",
		"AND petition_id = $2 LIMIT 1", petitionID)
	if err != nil {
		return domain.StaffRecordedRating{}, false, fmt.Errorf("petition_staff_recorded_rating: đọc: %w", err)
	}
	return scanOneStaffRating(rows)
}

// ForPetition is ForPetitionTx outside a transaction — the staff detail's read. Scoped to the context's
// commune like every read here.
func (s *PetitionStaffRatingStore) ForPetition(ctx context.Context, petitionID string) (
	domain.StaffRecordedRating, bool, error) {

	rows, err := s.db.For(ctx).Query(ctx, staffRatingColumns, "petition_staff_recorded_rating",
		"AND petition_id = $2 LIMIT 1", petitionID)
	if err != nil {
		return domain.StaffRecordedRating{}, false, fmt.Errorf("petition_staff_recorded_rating: đọc: %w", err)
	}
	return scanOneStaffRating(rows)
}

func scanOneStaffRating(rows *sql.Rows) (domain.StaffRecordedRating, bool, error) {
	defer rows.Close()
	var (
		r       domain.StaffRecordedRating
		channel string
		note    sql.NullString
		at      time.Time
	)
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return r, false, fmt.Errorf("petition_staff_recorded_rating: đọc: %w", err)
		}
		return r, false, nil
	}
	if err := rows.Scan(&r.ID, &r.PetitionID, &r.Stars, &channel, &note, &r.Source, &r.RecordedBy, &at); err != nil {
		// NOT the note: staff free text that may name people (rule 3).
		return domain.StaffRecordedRating{}, false, fmt.Errorf("petition_staff_recorded_rating: quét: %w", err)
	}
	if err := rows.Err(); err != nil {
		return domain.StaffRecordedRating{}, false, fmt.Errorf("petition_staff_recorded_rating: đọc: %w", err)
	}
	r.Channel = domain.StaffRatingChannel(channel)
	r.Note = note.String
	r.RecordedAt = at.UTC()
	return r, true, nil
}

// Append adds one record, in the caller's commune-scoped transaction ($1 is tx.TenantID()). `source` is
// written EXPLICITLY (and the schema CHECK admits nothing else), so the row says what it is without relying
// on a column default. "" note becomes NULL — "no note" — which the schema requires rather than a blank
// string.
func (s *PetitionStaffRatingStore) Append(ctx context.Context, tx *store.ScopedTx, r domain.StaffRecordedRating) error {
	const stmt = `INSERT INTO petition_staff_recorded_rating
		(tenant_id, id, petition_id, stars, channel, note, source, recorded_by, recorded_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID()), r.ID, r.PetitionID, r.Stars, string(r.Channel),
		rongThanhNull(r.Note), domain.StaffRecordedSource, r.RecordedBy, r.RecordedAt); err != nil {
		// NOT the note (rule 3).
		return fmt.Errorf("petition_staff_recorded_rating: ghi: %w", err)
	}
	return nil
}
