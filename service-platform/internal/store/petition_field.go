package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/service-platform/internal/domain"
)

// PetitionFieldStore is the only path to `petition_field` (migration 0011), tier 1 of the petition
// field catalogue (ADR 0026, ADR 0060).
//
// A RAW *sql.DB, LIKE UploadPolicyStore: the table has no tenant_id, so core/store.Scoped — which
// injects WHERE tenant_id = $1 — cannot read it. It holds one code set shared by every commune, no
// business content and no personal data. The write path is Vihat's operations zone under
// `ops.petition_field.manage` (ADR 0073 #3): every write below changes the row and appends its
// platform_audit_log entry in ONE transaction (rule 6, invariant 3).
//
// WHAT NO METHOD HERE CAN DO, by construction rather than by check: change a code, or remove a row.
// Archival petitions (`phieu_phan_anh.linh_vuc`), a commune's SLA rows (identity `sla.linh_vuc`) and
// its tier-2 labels (`nhan_linh_vuc`) hold the code AS A VALUE across the service boundary, so a
// renamed or removed code would orphan all three (ADR 0060 §4, stop condition #5). Migration 0011's
// trigger refuses both as a second copy. Retirement is `active = false`, which hides the code from
// NEW intake and reclassification only; tier-2 rows keep their meaning.
type PetitionFieldStore struct {
	db *sql.DB
}

func NewPetitionFieldStore(db *sql.DB) *PetitionFieldStore { return &PetitionFieldStore{db: db} }

// listPetitionFields reads EVERY code, retired ones included.
//
// THERE IS DELIBERATELY NO `WHERE active`: a retired code is still the code on every petition filed
// under it before retirement, and those petitions need its label for as long as they exist. Hiding
// retired codes is the caller's decision on its WRITE paths only (ADR 0060 §4; platform.proto).
//
// The order is total (sort_order, then the primary key) so two rows sharing a sort_order cannot swap
// places between two reads.
const listPetitionFields = `
	SELECT code, default_label, sort_order, icon, tone, active
	FROM petition_field
	ORDER BY sort_order, code`

// ListPetitionFields returns every tier-1 code. An empty slice is an ordinary answer (no code is
// valid), not an error.
func (s *PetitionFieldStore) ListPetitionFields(ctx context.Context) ([]domain.PetitionField, error) {
	// No tenant filter: one code set for every commune (see the type comment).
	rows, err := s.db.QueryContext(ctx, listPetitionFields)
	if err != nil {
		return nil, fmt.Errorf("petition_field: query: %w", err)
	}
	defer rows.Close()

	var out []domain.PetitionField
	for rows.Next() {
		var f domain.PetitionField
		if err := rows.Scan(&f.Code, &f.DefaultLabel, &f.SortOrder, &f.Icon, &f.Tone, &f.Active); err != nil {
			return nil, fmt.Errorf("petition_field: scan: %w", err)
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("petition_field: read: %w", err)
	}
	return out, nil
}

// Errors of the write path.
var (
	// ErrPetitionFieldNotFound — no row with this code. A code that is absent was never issued, and
	// issuing it is CreatePetitionField's explicit act, never a side effect of an edit.
	ErrPetitionFieldNotFound = errors.New("petition_field: không có mã lĩnh vực này")
	// ErrPetitionFieldCodeTaken — the code already has a row, active or retired. A retired code is
	// brought back by SetPetitionFieldActive, never by a second create (rule 7 invariant 3: an issued
	// code is never reissued).
	ErrPetitionFieldCodeTaken = errors.New("petition_field: mã lĩnh vực đã có")
)

// The platform_audit_log verbs of the operator's acts on a tier-1 row. Subject = the code.
const (
	ActionPetitionFieldCreated     = "petition_field.created"
	ActionPetitionFieldChanged     = "petition_field.changed"
	ActionPetitionFieldDeactivated = "petition_field.deactivated"
	ActionPetitionFieldReactivated = "petition_field.reactivated"
)

// fieldTrail is the before/after shape 0011's seed entries already have.
func fieldTrail(f domain.PetitionField) string {
	return string(delta(map[string]any{
		"default_label": f.DefaultLabel, "sort_order": f.SortOrder, "icon": f.Icon, "tone": f.Tone, "active": f.Active,
	}))
}

const fieldColumns = `code, default_label, sort_order, icon, tone, active`

func scanField(scan func(...any) error) (domain.PetitionField, error) {
	var f domain.PetitionField
	if err := scan(&f.Code, &f.DefaultLabel, &f.SortOrder, &f.Icon, &f.Tone, &f.Active); err != nil {
		return domain.PetitionField{}, err
	}
	return f, nil
}

func appendFieldTrail(ctx context.Context, tx *sql.Tx, by domain.OperatorActor, action, code string,
	before *domain.PetitionField, after domain.PetitionField, reason string) error {
	var b any
	if before != nil {
		b = fieldTrail(*before)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO platform_audit_log (actor, actor_ip, action, subject, before, after, reason)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7)`,
		by.Code, by.IP, action, code, b, fieldTrail(after), reason); err != nil {
		return fmt.Errorf("petition_field: trail: %w", err)
	}
	return nil
}

// fieldTx runs fn in one transaction and commits only when fn returns nil AND reports a change; an
// unchanged row writes nothing and appends no entry (the convention of every operator write).
func (s *PetitionFieldStore) fieldTx(ctx context.Context, by domain.OperatorActor,
	fn func(tx *sql.Tx) (bool, error)) (changed bool, err error) {
	if err := by.Validate(); err != nil {
		return false, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("petition_field: begin: %w", err)
	}
	defer func() {
		if err != nil || !changed {
			_ = tx.Rollback()
		}
	}()
	if changed, err = fn(tx); err != nil || !changed {
		return changed, err
	}
	if err = tx.Commit(); err != nil {
		return false, fmt.Errorf("petition_field: commit: %w", err)
	}
	return true, nil
}

// CreatePetitionField issues a NEW tier-1 code, active, with its presentation (validated by the
// caller: domain.ValidatePetitionFieldCode + ValidatePetitionFieldPresentation). Refused with
// ErrPetitionFieldCodeTaken when the code has any row.
//
// WHAT A NEW CODE MEANS FOR COMMUNES (ADR 0060 §2, "Cái giá"): every business service sees it within
// one 60-second TTL; a commune with no tier-2 label shows the default label; a commune with no SLA
// row for it falls to ITS OWN default SLA row (identity ResolveDeadlines) — that commune's
// configuration, not a value chosen here.
func (s *PetitionFieldStore) CreatePetitionField(ctx context.Context, f domain.PetitionField, reason string,
	by domain.OperatorActor) (out domain.PetitionField, err error) {
	_, err = s.fieldTx(ctx, by, func(tx *sql.Tx) (bool, error) {
		row, err := scanField(tx.QueryRowContext(ctx, `
			INSERT INTO petition_field (code, default_label, sort_order, icon, tone, active, created_by, updated_by)
			VALUES ($1, $2, $3, $4, $5, true, $6, $6)
			ON CONFLICT (code) DO NOTHING
			RETURNING `+fieldColumns,
			f.Code, f.DefaultLabel, f.SortOrder, f.Icon, f.Tone, by.Code).Scan)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return false, ErrPetitionFieldCodeTaken
		case err != nil:
			return false, fmt.Errorf("petition_field: insert: %w", err)
		}
		out = row
		return true, appendFieldTrail(ctx, tx, by, ActionPetitionFieldCreated, row.Code, nil, row, reason)
	})
	if err != nil {
		return domain.PetitionField{}, err
	}
	return out, nil
}

func lockField(ctx context.Context, tx *sql.Tx, code string) (domain.PetitionField, error) {
	f, err := scanField(tx.QueryRowContext(ctx,
		`SELECT `+fieldColumns+` FROM petition_field WHERE code = $1 FOR UPDATE`, code).Scan)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return domain.PetitionField{}, ErrPetitionFieldNotFound
	case err != nil:
		return domain.PetitionField{}, fmt.Errorf("petition_field: lock: %w", err)
	}
	return f, nil
}

// EditPetitionField sets the default label, order, icon and tone of one code (validated by the
// caller). The code is the WHERE, never a SET — no statement here could change it. changed=false,
// nothing written, when the values are the ones the row holds.
func (s *PetitionFieldStore) EditPetitionField(ctx context.Context, code string, next domain.PetitionField,
	reason string, by domain.OperatorActor) (out domain.PetitionField, changed bool, err error) {
	changed, err = s.fieldTx(ctx, by, func(tx *sql.Tx) (bool, error) {
		before, err := lockField(ctx, tx, code)
		if err != nil {
			return false, err
		}
		if domain.SamePetitionFieldPresentation(before, next) {
			out = before
			return false, nil
		}
		out, err = scanField(tx.QueryRowContext(ctx, `
			UPDATE petition_field
			   SET default_label = $2, sort_order = $3, icon = $4, tone = $5, updated_at = now(), updated_by = $6
			 WHERE code = $1
			RETURNING `+fieldColumns,
			code, next.DefaultLabel, next.SortOrder, next.Icon, next.Tone, by.Code).Scan)
		if err != nil {
			return false, fmt.Errorf("petition_field: update: %w", err)
		}
		return true, appendFieldTrail(ctx, tx, by, ActionPetitionFieldChanged, code, &before, out, reason)
	})
	if err != nil {
		return domain.PetitionField{}, false, err
	}
	return out, changed, nil
}

// SetPetitionFieldActive retires (active=false) or brings back (true) one code, platform-wide.
// changed=false, nothing written, when it is already in that state.
//
// RETIRING NEVER BREAKS A COMMUNE'S DATA: no read path filters on `active` (ADR 0060 §4), so old
// petitions keep their label, tier-2 labels keep their row, SLA rows keep their key; only NEW intake
// and reclassification stop offering the code, in every commune, within one 60-second TTL.
func (s *PetitionFieldStore) SetPetitionFieldActive(ctx context.Context, code string, active bool,
	reason string, by domain.OperatorActor) (out domain.PetitionField, changed bool, err error) {
	changed, err = s.fieldTx(ctx, by, func(tx *sql.Tx) (bool, error) {
		before, err := lockField(ctx, tx, code)
		if err != nil {
			return false, err
		}
		if before.Active == active {
			out = before
			return false, nil
		}
		out, err = scanField(tx.QueryRowContext(ctx, `
			UPDATE petition_field SET active = $2, updated_at = now(), updated_by = $3
			 WHERE code = $1
			RETURNING `+fieldColumns,
			code, active, by.Code).Scan)
		if err != nil {
			return false, fmt.Errorf("petition_field: update: %w", err)
		}
		action := ActionPetitionFieldDeactivated
		if active {
			action = ActionPetitionFieldReactivated
		}
		return true, appendFieldTrail(ctx, tx, by, action, code, &before, out, reason)
	})
	if err != nil {
		return domain.PetitionField{}, false, err
	}
	return out, changed, nil
}
