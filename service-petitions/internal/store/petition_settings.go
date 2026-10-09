package store

import (
	"context"
	"database/sql"
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
// READ ONLY TODAY. The write path, its permission and its audit entry are a later card. The closing
// act (app/xu_ly_phan_anh.go Dong) reads it through VerificationPhotoRequiredTx.
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
	return readVerificationPhotoRequired(s.db.For(ctx).Query(ctx, "verification_photo_required",
		"petition_settings", "LIMIT 2"))
}

// VerificationPhotoRequiredTx is VerificationPhotoRequired INSIDE the caller's transaction — the
// closing act (app.XuLyPhanAnh.Dong), so the switch it obeys and the close it gates are read and
// written in one transaction (rule 2, invariant 6). Same answers: no row is TRUE, a failure is an error.
func (s *PetitionSettingsStore) VerificationPhotoRequiredTx(ctx context.Context, tx *store.ScopedTx) (bool, error) {
	// ScopedTx.Query prefixes `WHERE tenant_id = $1` and binds the commune from the transaction.
	return readVerificationPhotoRequired(tx.Query(ctx, "verification_photo_required",
		"petition_settings", "LIMIT 2"))
}

// The suspected-duplicate thresholds' decided defaults (ADR 0087 §6: 50 m, 7 days). They MUST equal the
// column DEFAULTs of migration 0038, whose header asks exactly this of the reader: "no row" answers 50 and
// 7, as a row written with the defaults does — the verificationPhotoRequiredByDefault precedent. They are
// NOT constants of the rule (ADR 0087 stop condition #5): a commune's own row overrides both.
const (
	duplicateRadiusMetersByDefault = 50
	duplicateWindowDaysByDefault   = 7
)

// DuplicateThresholds is one commune's suspected-duplicate radius (metres) and window (calendar days).
type DuplicateThresholds struct {
	RadiusMeters int
	WindowDays   int
}

// DuplicateThresholds reads this commune's thresholds. NO ROW IS THE DECIDED DEFAULTS; a driver failure
// is an error and never answered with a default — a suggestion list built on a guessed radius would look
// complete while it is not.
func (s *PetitionSettingsStore) DuplicateThresholds(ctx context.Context) (DuplicateThresholds, error) {
	rows, err := s.db.For(ctx).Query(ctx, "duplicate_radius_meters, duplicate_window_days",
		"petition_settings", "LIMIT 2")
	if err != nil {
		return DuplicateThresholds{}, fmt.Errorf("petition_settings: read thresholds: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return DuplicateThresholds{}, fmt.Errorf("petition_settings: read thresholds: %w", err)
		}
		return DuplicateThresholds{RadiusMeters: duplicateRadiusMetersByDefault,
			WindowDays: duplicateWindowDaysByDefault}, nil
	}
	var t DuplicateThresholds
	if err := rows.Scan(&t.RadiusMeters, &t.WindowDays); err != nil {
		return DuplicateThresholds{}, fmt.Errorf("petition_settings: scan thresholds: %w", err)
	}
	if rows.Next() {
		return DuplicateThresholds{}, ErrPetitionSettingsDuplicate
	}
	if err := rows.Err(); err != nil {
		return DuplicateThresholds{}, fmt.Errorf("petition_settings: read thresholds: %w", err)
	}
	return t, nil
}

func readVerificationPhotoRequired(rows *sql.Rows, err error) (bool, error) {
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
