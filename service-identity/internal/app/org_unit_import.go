package app

// The use case behind "Nhập từ Excel" on the org chart (14-cau-hinh.md §1; user decision
// 2026-09-28): org units only, ALL OR NOTHING, create only, under `admin.org`.
//
//	Preview  plan the file against the commune's chart and report — WRITES NOTHING, audits nothing
//	Import   the same plan; any error = nothing written; else every unit and one audit entry per
//	         unit, in ONE transaction (rule 6, invariant 3)
//
// THE RULES ARE domain.PlanOrgUnitImport's and are not restated here (rule 9). What this layer owns
// is the transaction: the snapshot the plan is made against is read inside the same transaction the
// inserts run in, the existing parents are locked the way SoDoToChuc.Them locks one, and a failure
// anywhere — an insert, an audit entry, the unique key refusing a code a concurrent request just
// took — rolls the WHOLE file back.

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

// OrgUnitImportRepo is the store, declared at the point of use. EVERY METHOD TAKES THE TRANSACTION.
// *idstore.BoPhanStore satisfies it; the lock and the insert are the statements POST
// /api/v1/org-units already runs, which is why their existing names are kept.
type OrgUnitImportRepo interface {
	ImportSnapshot(ctx context.Context, tx *store.ScopedTx) ([]domain.ExistingOrgUnit, error)
	KhoaBoPhan(ctx context.Context, tx *store.ScopedTx, id string) (domain.BoPhan, bool, error) // vi-name-ok: the existing idstore.BoPhanStore method, reused
	Chen(ctx context.Context, tx *store.ScopedTx, bp domain.BoPhan) error                       // vi-name-ok: the existing idstore.BoPhanStore method, reused
}

// OrgUnitImportRejected carries every error of a file that was refused. Nothing was written.
type OrgUnitImportRejected struct {
	Errors []domain.OrgUnitImportError
}

func (e *OrgUnitImportRejected) Error() string {
	return fmt.Sprintf("bo_phan: tệp nhập có %d lỗi — không ghi gì", len(e.Errors))
}

// OrgUnitImportResult is what a run planned (Preview) or created (Import), in file order. On
// Import, Unit.ID and Unit.ChaID are filled — including a ChaID that points at a unit created by a
// row above.
type OrgUnitImportResult struct {
	Units  []domain.PlannedOrgUnit
	Errors []domain.OrgUnitImportError // Preview only; Import returns *OrgUnitImportRejected instead
}

// importSource marks, in the delta, that a creation came from a file rather than the form. The
// ACTION stays HanhViThemBoPhan so "every unit ever created" is one query on the trail.
const importSource = "nhap_excel"

// errPreviewRollback ends the preview's transaction. Nothing was written, but a preview must not
// COMMIT anything even by accident, so it always rolls back.
var errPreviewRollback = errors.New("bo_phan: xem trước — huỷ giao dịch")

// OrgUnitImporter owns the import.
type OrgUnitImporter struct {
	db   *store.DB
	repo OrgUnitImportRepo

	// Injected so a test can pin it. In production: ulid.Moi.
	newID func() (string, error)
}

func NewOrgUnitImporter(db *store.DB, repo OrgUnitImportRepo) *OrgUnitImporter {
	return &OrgUnitImporter{db: db, repo: repo, newID: ulid.Moi}
}

// Preview plans the file and reports. The returned error is a system failure only; a file with
// errors is a SUCCESSFUL preview whose Errors is non-empty.
func (uc *OrgUnitImporter) Preview(ctx context.Context, rows []domain.OrgUnitImportRow) (OrgUnitImportResult, error) {
	var res OrgUnitImportResult
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		existing, err := uc.repo.ImportSnapshot(ctx, tx)
		if err != nil {
			return err
		}
		res.Units, res.Errors = domain.PlanOrgUnitImport(rows, existing, idstore.TranDanhMucBoPhan)
		return errPreviewRollback
	})
	if err != nil && !errors.Is(err, errPreviewRollback) {
		return OrgUnitImportResult{}, err
	}
	return res, nil
}

// Import writes the file, or nothing.
func (uc *OrgUnitImporter) Import(ctx context.Context, rows []domain.OrgUnitImportRow, actor NguoiThucHien) (OrgUnitImportResult, error) {
	if err := actor.hopLe(); err != nil {
		return OrgUnitImportResult{}, err
	}
	var res OrgUnitImportResult
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		res = OrgUnitImportResult{}

		existing, err := uc.repo.ImportSnapshot(ctx, tx)
		if err != nil {
			return err
		}
		plan, errs := domain.PlanOrgUnitImport(rows, existing, idstore.TranDanhMucBoPhan)
		if len(errs) > 0 {
			// Returned from INSIDE the closure so the transaction rolls back; nothing was written yet,
			// and nothing may be.
			return &OrgUnitImportRejected{Errors: errs}
		}

		// LOCK EVERY EXISTING PARENT, once, as Them locks its one: held until commit, and it re-checks
		// that the parent is still a live unit of this commune (rule 1 — another commune's id is not
		// found by the scoped lock).
		locked := make(map[string]bool)
		for _, p := range plan {
			id := p.Unit.ChaID
			if id == "" || locked[id] {
				continue
			}
			_, deleted, err := uc.repo.KhoaBoPhan(ctx, tx, id)
			if errors.Is(err, idstore.ErrKhongTimThayBoPhan) || (err == nil && deleted) {
				return idstore.ErrBoPhanChaKhongTonTai
			}
			if err != nil {
				return err
			}
			locked[id] = true
		}

		// IN FILE ORDER: a parent row is always above its children, so its id exists before any child
		// names it and the composite foreign key holds statement by statement.
		idByRow := make(map[int]string, len(plan))
		for i := range plan {
			p := &plan[i]
			if p.ParentRow != 0 {
				p.Unit.ChaID = idByRow[p.ParentRow]
				if p.Unit.ChaID == "" {
					// Cannot happen: the planner only points at rows above. Checked because a child
					// silently inserted at the root is a wrong chart nobody notices.
					return fmt.Errorf("bo_phan: dòng %d trỏ tới dòng cha %d chưa được tạo", p.Row, p.ParentRow)
				}
			}
			if p.Unit.ID, err = uc.newID(); err != nil {
				return fmt.Errorf("bo_phan: sinh id: %w", err)
			}
			if err := uc.repo.Chen(ctx, tx, p.Unit); err != nil {
				return err
			}
			// ONE ENTRY PER UNIT, IN THIS TRANSACTION (rule 6, invariant 3). Same action and subject as
			// the form's creation — the unit's code — so a unit's history reads the same whichever way
			// it was made; the delta says it came from a file, and from which row.
			if err := audit.Write(ctx, tx, audit.Entry{
				Actor:   actor.Vet,
				Action:  HanhViThemBoPhan,
				Subject: p.Unit.Ma,
				Delta: deltaBoPhan(map[string]any{
					"sau":     vetBoPhan(p.Unit),
					"nguon":   importSource,
					"dong":    p.Row,
					"so_dong": len(plan),
				}),
			}); err != nil {
				return err
			}
			idByRow[p.Row] = p.Unit.ID
		}
		res.Units = plan
		return nil
	})
	if err != nil {
		return OrgUnitImportResult{}, err
	}
	return res, nil
}
