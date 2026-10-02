package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
)

// PetitionSettingsStore reads one commune's petition lifecycle settings (`petition_settings`,
// migration 0028 — ADR 0008's per-commune switches).
//
// It holds *store.DB, never a *sql.DB: the commune is $1, bound by Scoped.Query from the context
// (rule 1, invariants 4 and 5), and no method takes it as a parameter.
//
// READ ONLY TODAY. The write path, its permission and its audit entry are a later card; so is wiring
// this read into the closing act (app/xu_ly_phan_anh.go Dong).
type PetitionSettingsStore struct {
	db *store.DB
}

func NewPetitionSettingsStore(db *store.DB) *PetitionSettingsStore {
	return &PetitionSettingsStore{db: db}
}

// verificationPhotoRequiredByDefault is ADR 0008 "Đóng phiếu" decision 3's decided default
// (`bat_buoc_anh_nghiem_thu`, TRUE — kept by the project owner on 02/10/2026). It MUST equal the column
// DEFAULT in migration 0028: a row written with the default and no row must mean the same thing.
const verificationPhotoRequiredByDefault = true

// ErrPetitionSettingsDuplicate — more than one row for one commune. PRIMARY KEY (tenant_id) makes that
// impossible; reaching it means the schema and the code disagree, and the read refuses rather than
// guessing which row is in force.
var ErrPetitionSettingsDuplicate = errors.New("petition_settings: more than one row for this commune")

// VerificationPhotoRequired reports whether this commune requires at least one verification photo
// before a petition can be closed.
//
// NO ROW IS NOT AN ERROR — it is every commune's state until the settings screen exists — and it is
// answered with the decided default, TRUE. A driver failure is returned, never answered with a
// default: the caller is the closing act, and closing on a guess is the one outcome that cannot be
// taken back (fail closed).
func (s *PetitionSettingsStore) VerificationPhotoRequired(ctx context.Context) (bool, error) {
	rows, err := s.db.For(ctx).Query(ctx, "verification_photo_required", "petition_settings", "LIMIT 2")
	if err != nil {
		return false, fmt.Errorf("petition_settings: read: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, fmt.Errorf("petition_settings: read: %w", err)
		}
		return verificationPhotoRequiredByDefault, nil
	}
	var required bool
	if err := rows.Scan(&required); err != nil {
		return false, fmt.Errorf("petition_settings: scan: %w", err)
	}
	if rows.Next() {
		return false, ErrPetitionSettingsDuplicate
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("petition_settings: read: %w", err)
	}
	return required, nil
}
