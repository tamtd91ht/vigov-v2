package app

// The use case behind "⬆ Nhập từ Excel" on the residential-unit tab (14-cau-hinh.md §2; user decision
// 2026-09-29, ADR 0059 §2): create only, ALL OR NOTHING, under `admin.org`.
//
//	Preview  plan the file against the commune and report — WRITES NOTHING, audits nothing
//	Import   the same plan; any error = nothing written; else every unit, one audit entry per unit,
//	         all in ONE transaction (rule 6, invariant 3)
//
// THE ROW RULES ARE domain.PlanResidentialUnitImport's and are not restated here (rule 9). What this
// layer owns is the transaction: the register lock and the snapshot the plan is made against are taken
// inside the same transaction the inserts run in, and a failure anywhere — an insert, an audit entry,
// the unique key refusing a code — rolls the WHOLE file back. The shape is app.OrgUnitImporter's.

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// ResidentialUnitImportRepo is the store, declared at the point of use. EVERY METHOD TAKES THE
// TRANSACTION. *idstore.ThonToDanPhoStore satisfies it; the insert is the statement the create form
// runs, so the import cannot write a row the form could not.
type ResidentialUnitImportRepo interface {
	LockRegister(ctx context.Context, tx *store.ScopedTx) error
	ImportSnapshot(ctx context.Context, tx *store.ScopedTx) (domain.ResidentialUnitImportSnapshot, error)
	InsertUnit(ctx context.Context, tx *store.ScopedTx, u domain.ThonToDanPho) error

	// TemplateChoices is the one read outside a transaction: the template's two dropdowns, built with
	// the same predicates as ImportSnapshot so the file never offers a value the import refuses.
	TemplateChoices(ctx context.Context) ([]domain.ResidentialUnitTypeChoice, []domain.HeadStaffChoice, error)
}

// ResidentialUnitImportRejected carries every error of a file that was refused. Nothing was written.
type ResidentialUnitImportRejected struct {
	Errors []domain.ResidentialUnitImportError
}

func (e *ResidentialUnitImportRejected) Error() string {
	return fmt.Sprintf("thon_to_dan_pho: tệp nhập có %d lỗi — không ghi gì", len(e.Errors))
}

// ResidentialUnitImportResult is what a run planned (Preview) or created (Import), in file order.
type ResidentialUnitImportResult struct {
	Units  []domain.PlannedResidentialUnit
	Errors []domain.ResidentialUnitImportError // Preview only; Import returns *ResidentialUnitImportRejected
}

// residentialUnitImportSource marks, in the delta, that a creation came from a file. The ACTION stays
// ActionCreateResidentialUnit so "every unit ever created" is one query on the trail.
const residentialUnitImportSource = "nhap_excel"

// errResidentialUnitPreviewRollback ends the preview's transaction: a preview must not COMMIT anything,
// even by accident, so it always rolls back.
var errResidentialUnitPreviewRollback = errors.New("thon_to_dan_pho: xem trước — huỷ giao dịch")

// ResidentialUnitImporter owns the import.
type ResidentialUnitImporter struct {
	db   *store.DB
	repo ResidentialUnitImportRepo

	// Injected so a test can pin it. In production: ulid.Moi.
	newID func() (string, error)
}

func NewResidentialUnitImporter(db *store.DB, repo ResidentialUnitImportRepo) *ResidentialUnitImporter {
	return &ResidentialUnitImporter{db: db, repo: repo, newID: ulid.Moi}
}

// Preview plans the file and reports. The returned error is a system failure only; a file with errors
// is a SUCCESSFUL preview whose Errors is non-empty. No lock is taken: the preview writes nothing, and
// the import re-plans under the lock anyway.
func (uc *ResidentialUnitImporter) Preview(ctx context.Context, rows []domain.ResidentialUnitImportRow) (ResidentialUnitImportResult, error) {
	var res ResidentialUnitImportResult
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		snap, err := uc.repo.ImportSnapshot(ctx, tx)
		if err != nil {
			return err
		}
		res.Units, res.Errors = domain.PlanResidentialUnitImport(rows, snap, idstore.TranDanhSachThonToDanPho)
		return errResidentialUnitPreviewRollback
	})
	if err != nil && !errors.Is(err, errResidentialUnitPreviewRollback) {
		return ResidentialUnitImportResult{}, err
	}
	return res, nil
}

// Import writes the file, or nothing.
//
// ONE ENTRY PER UNIT, EACH CARRYING THE BATCH: every entry names the unit's code as its subject — so a
// unit's history reads the same whichever way it was made — and carries `lo_nhap` (one ULID minted per
// file), `dong` and `so_dong`, so the whole file is one query on the trail and every unit in it can be
// traced back to the row it came from.
func (uc *ResidentialUnitImporter) Import(ctx context.Context, rows []domain.ResidentialUnitImportRow, actor NguoiThucHien) (ResidentialUnitImportResult, error) {
	if err := actor.hopLe(); err != nil {
		return ResidentialUnitImportResult{}, err
	}
	batch, err := uc.newID()
	if err != nil {
		return ResidentialUnitImportResult{}, fmt.Errorf("thon_to_dan_pho: sinh mã lô nhập: %w", err)
	}
	var res ResidentialUnitImportResult
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		res = ResidentialUnitImportResult{}
		if err := uc.repo.LockRegister(ctx, tx); err != nil {
			return err
		}
		snap, err := uc.repo.ImportSnapshot(ctx, tx)
		if err != nil {
			return err
		}
		plan, errs := domain.PlanResidentialUnitImport(rows, snap, idstore.TranDanhSachThonToDanPho)
		if len(errs) > 0 {
			// Returned from INSIDE the closure so the transaction rolls back; nothing was written.
			return &ResidentialUnitImportRejected{Errors: errs}
		}
		for i := range plan {
			p := &plan[i]
			if p.Unit.ID, err = uc.newID(); err != nil {
				return fmt.Errorf("thon_to_dan_pho: sinh id: %w", err)
			}
			if err := uc.repo.InsertUnit(ctx, tx, p.Unit); err != nil {
				return err
			}
			if err := audit.Write(ctx, tx, audit.Entry{
				Actor:   actor.Vet,
				Action:  ActionCreateResidentialUnit,
				Subject: p.Unit.Ma,
				Delta: residentialUnitDelta(map[string]any{
					"sau":     residentialUnitTrail(p.Unit),
					"nguon":   residentialUnitImportSource,
					"lo_nhap": batch,
					"dong":    p.Row,
					"so_dong": len(plan),
				}),
			}); err != nil {
				return err
			}
		}
		res.Units = plan
		return nil
	})
	if err != nil {
		return ResidentialUnitImportResult{}, err
	}
	return res, nil
}

// TemplateChoices returns the dropdown values of the import template: the commune's types in use and
// its live, unlocked staff. Writes nothing, audits nothing.
func (uc *ResidentialUnitImporter) TemplateChoices(ctx context.Context) ([]domain.ResidentialUnitTypeChoice, []domain.HeadStaffChoice, error) {
	return uc.repo.TemplateChoices(ctx)
}
